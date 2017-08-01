package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/auth"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/egress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/ingress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/mapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	directorAddr := flag.String("director-addr", "", "The host and port of the bosh director")
	clientIdentity := flag.String("auth-client-identity", "", "The UAA client identity which has access to bosh system metrics")
	clientSecret := flag.String("auth-client-secret", "", "The UAA client password")

	metricsServerAddr := flag.String("metrics-server-addr", "", "The host and port of the metrics server")
	metronPort := flag.String("metron-port", "3458", "The GRPC port to inject metrics to")
	metronCA := flag.String("metron-ca", "", "The CA cert path for metron")
	metronCert := flag.String("metron-cert", "", "The cert path for metron")
	metronKey := flag.String("metron-key", "", "The key path for metron")

	metricsCA := flag.String("metrics-ca", "", "The CA cert path for the metrics server")
	metricsCN := flag.String("metrics-cn", "", "The common name for the metrics server")

	healthPort := flag.Int("health-port", 19111, "The port for the localhost health endpoint")
	flag.Parse()

	authClient := auth.New(*directorAddr)
	authToken, err := authClient.GetToken(*clientIdentity, *clientSecret)
	if err != nil {
		log.Fatalf("could not get token from AuthServer: %v", err)
	}

	// server setup (ingress)
	serverTLSConf := &tls.Config{
		ServerName: *metricsCN,
	}
	err = setCACert(serverTLSConf, *metricsCA)
	if err != nil {
		log.Fatal(err)
	}
	serverConn, err := grpc.Dial(
		*metricsServerAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(serverTLSConf)),
		grpc.WithPerRPCCredentials(auth.MapGRPCCreds(authToken)),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	serverClient := definitions.NewEgressClient(serverConn)

	// metron setup (egress)
	c, err := newTLSConfig(*metronCA, *metronCert, *metronKey, "metron")
	if err != nil {
		log.Fatalf("unable to read tls certs: %s", err)
	}
	metronConn, err := grpc.Dial(
		net.JoinHostPort("localhost", *metronPort),
		grpc.WithTransportCredentials(credentials.NewTLS(c)),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	metronClient := loggregator_v2.NewIngressClient(metronConn)

	metronCtx, metronCancel := context.WithCancel(context.Background())
	metronStreamClient, err := metronClient.Sender(metronCtx)
	if err != nil {
		log.Fatalf("error creating stream connection to metron: %s", err)
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/health", expvar.Handler())
		http.ListenAndServe(fmt.Sprintf("localhost:%d", *healthPort), mux)
	}()

	messages := make(chan *loggregator_v2.Envelope, 100)
	i := ingress.New(serverClient, mapper.Map, messages)
	e := egress.New(metronStreamClient, messages)

	ingressStop := i.Start()
	egressStop := e.Start()

	defer func() {
		serverConn.Close()
		ingressStop()

		close(messages)

		egressStop()
		metronCancel()
		metronConn.Close()
	}()

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM)
	<-killSignal
}

func newTLSConfig(caPath, certPath, keyPath, cn string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName:         cn,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	err = setCACert(tlsConfig, caPath)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func setCACert(tlsConfig *tls.Config, caPath string) error {
	caCertBytes, err := ioutil.ReadFile(caPath)
	if err != nil {
		return err
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
		return errors.New("cannot parse ca cert")
	}

	tlsConfig.RootCAs = caCertPool

	return nil
}
