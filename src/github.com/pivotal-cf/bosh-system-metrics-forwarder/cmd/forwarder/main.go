package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/auth"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/egress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/ingress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/mapper"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/monitor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	directorURL := flag.String("director-url", "", "The url of the bosh director")
	directorCA := flag.String("director-ca", "", "The CA cert path for the bosh director")

	clientIdentity := flag.String("auth-client-identity", "", "The UAA client identity which has access to bosh system metrics")
	clientSecret := flag.String("auth-client-secret", "", "The UAA client password")

	metronPort := flag.Int("metron-port", 3458, "The GRPC port to inject metrics to")
	metronCA := flag.String("metron-ca", "", "The CA cert path for metron")
	metronCert := flag.String("metron-cert", "", "The cert path for metron")
	metronKey := flag.String("metron-key", "", "The key path for metron")

	metricsServerAddr := flag.String("metrics-server-addr", "", "The host and port of the metrics server")
	metricsCA := flag.String("metrics-ca", "", "The CA cert path for the metrics server")
	metricsCN := flag.String("metrics-cn", "", "The common name for the metrics server")

	subscriptionID := flag.String("subscription-id", "bosh-system-metrics-forwarder", "The subscription id to use for the metrics server")

	healthPort := flag.Int("health-port", 0, "The port for the localhost health endpoint")
	pprofPort := flag.Int("pprof-port", 0, "The port for the localhost pprof endpoint")

	flag.Parse()

	validateCredentials(*clientIdentity, *clientSecret)

	directorTLSConf := &tls.Config{}
	err := setCACert(directorTLSConf, *directorCA)
	if err != nil {
		log.Fatal(err)
	}

	addressProvider := auth.NewAddressProvider(*directorURL, directorTLSConf)
	authClient := auth.New(addressProvider, *clientIdentity, *clientSecret, directorTLSConf)

	messages := make(chan *loggregator_v2.Envelope, 1024)

	// server setup (ingress)
	serverClient, serverConnClose := setupConnToMetricsServer(*metricsServerAddr, *metricsCN, *metricsCA)
	i := ingress.New(serverClient, mapper.Map, messages, authClient, *subscriptionID)

	// metron setup (egress)
	metronClient, metronConnClose := setupConnToMetron(*metronPort, *metronCA, *metronCert, *metronKey)
	e := egress.New(metronClient, messages)

	ingressStop := i.Start()
	egressStop := e.Start()

	go monitor.NewHealth(uint32(*healthPort)).Start()
	go monitor.NewProfiler(uint32(*pprofPort)).Start()

	defer func() {
		fmt.Println("process shutting down, stop accepting messages from system metrics server...")
		serverConnClose()
		ingressStop()

		close(messages)

		fmt.Println("drain remaining messages...")
		egressStop()
		metronConnClose()

		fmt.Println("DONE")
	}()

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM)
	<-killSignal
}

func validateCredentials(id, secret string) {
	if id == "" || secret == "" {
		log.Fatalf("UAA System Metrics Client Credentials are required. Please see Bosh System Metrics Forwarder configuration")
	}
}

func setupConnToMetron(metronPort int, metronCA, metronCert, metronKey string) (loggregator_v2.IngressClient, func() error) {
	c, err := newTLSConfig(metronCA, metronCert, metronKey, "metron")
	if err != nil {
		log.Fatalf("unable to read tls certs: %s", err)
	}
	metronConn, err := grpc.Dial(
		fmt.Sprintf("localhost:%d", metronPort),
		grpc.WithTransportCredentials(credentials.NewTLS(c)),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return loggregator_v2.NewIngressClient(metronConn), metronConn.Close
}

func setupConnToMetricsServer(addr, cn, ca string) (definitions.EgressClient, func() error) {
	serverTLSConf := &tls.Config{
		ServerName: cn,
	}
	err := setCACert(serverTLSConf, ca)
	if err != nil {
		log.Fatal(err)
	}
	serverConn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(credentials.NewTLS(serverTLSConf)),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	return definitions.NewEgressClient(serverConn), serverConn.Close
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
		return fmt.Errorf("cannot parse ca cert from %s", caPath)
	}

	tlsConfig.RootCAs = caCertPool

	return nil
}
