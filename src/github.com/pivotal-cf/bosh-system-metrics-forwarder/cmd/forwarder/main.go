package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/egress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/ingress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/mapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {

	metricsServerAddr := flag.String("metrics-server-addr", "", "The host and port of the metrics server")
	metronPort := flag.String("metron-port", "3458", "The GRPC port to inject metrics to")
	metronCA := flag.String("metron-ca", "", "The CA cert path for metron")
	metronCert := flag.String("metron-cert", "", "The cert path for metron")
	metronKey := flag.String("metron-key", "", "The key path for metron")
	flag.Parse()

	metricsConn, err := grpc.Dial(*metricsServerAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	directorClient := definitions.NewEgressClient(metricsConn)

	metricsServerCtx, metricsServerCancel := context.WithCancel(context.Background())
	metricsStreamClient, err := directorClient.BoshMetrics(
		metricsServerCtx,
		&definitions.EgressRequest{
			SubscriptionId: "bosh-system-metrics-forwarder",
		},
	)
	if err != nil {
		log.Fatalf("error creating stream connection to metrics server: %s", err)
	}

	c, err := newTLSConfig(*metronCA, *metronCert, *metronKey, "metron")
	if err != nil {
		log.Fatalf("unable to read tls certs: %s", err)
	}
	metronConn, err := grpc.Dial(net.JoinHostPort("localhost", *metronPort), grpc.WithTransportCredentials(credentials.NewTLS(c)))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	metronClient := loggregator_v2.NewIngressClient(metronConn)

	metronCtx, metronCancel := context.WithCancel(context.Background())
	metronStreamClient, err := metronClient.Sender(metronCtx)
	if err != nil {
		log.Fatalf("error creating stream connection to metron: %s", err)
	}

	messages := make(chan *loggregator_v2.Envelope, 100)
	i := ingress.New(metricsStreamClient, mapper.Map, messages)
	e := egress.New(metronStreamClient, messages)

	ingressStop := i.Start()
	egressStop := e.Start()

	defer func() {
		metricsConn.Close()
		metricsServerCancel()
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

	caCertBytes, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, errors.New("cannot parse ca cert")
	}

	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}
