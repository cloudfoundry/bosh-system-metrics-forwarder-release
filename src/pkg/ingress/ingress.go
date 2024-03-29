package ingress

import (
	"errors"
	"log"
	"time"

	"sync"

	"github.com/cloudfoundry/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/cloudfoundry/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	connErrCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "ingress",
		Name:      "stream_conn_err",
		Help:      "Tracks errors when a stream needs to be established",
	})
	receiveErrCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "ingress",
		Name:      "stream_receive_err",
		Help:      "Tracks errors when receiving events from metrics server",
	})
	convertErrCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "ingress",
		Name:      "stream_convert_err",
		Help:      "Tracks errors when converting an event to an envelope",
	})
	receivedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "ingress",
		Name:      "received",
		Help:      "Tracks total number of events received",
	})
	droppedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "ingress",
		Name:      "dropped",
		Help:      "Tracks the number of envelopes dropped if unable to queue the msg",
	})
)

func init() {
	prometheus.MustRegister(connErrCounter)
	prometheus.MustRegister(receiveErrCounter)
	prometheus.MustRegister(convertErrCounter)
	prometheus.MustRegister(receivedCounter)
	prometheus.MustRegister(droppedCounter)
}

type receiver interface {
	Recv() (*definitions.Event, error)
}

type sender interface {
	Send(*loggregator_v2.Envelope) error
}

type tokener interface {
	Token() (string, error)
}

type mapper func(event *definitions.Event) (*loggregator_v2.Envelope, error)

type Ingress struct {
	auth           tokener
	convert        mapper
	messages       chan *loggregator_v2.Envelope
	client         definitions.EgressClient
	reconnectWait  time.Duration
	streamTimeout  time.Duration
	subscriptionID string
	logger         *log.Logger

	mu                  sync.Mutex
	metricsServerCancel context.CancelFunc
}

type IngressOpt func(*Ingress)

func WithReconnectWait(d time.Duration) IngressOpt {
	return func(i *Ingress) {
		i.reconnectWait = d
	}
}

func WithStreamTimeout(d time.Duration) IngressOpt {
	return func(i *Ingress) {
		i.streamTimeout = d
	}
}

// New returns a new Ingress.
func New(
	s definitions.EgressClient,
	m mapper,
	messages chan *loggregator_v2.Envelope,
	auth tokener,
	sID string,
	l *log.Logger,
	opts ...IngressOpt,
) *Ingress {
	i := &Ingress{
		client:              s,
		convert:             m,
		messages:            messages,
		auth:                auth,
		subscriptionID:      sID,
		logger:              l,
		metricsServerCancel: func() {},
		reconnectWait:       time.Second,
		streamTimeout:       45 * time.Second,
	}

	for _, o := range opts {
		o(i)
	}

	return i
}

// Start spins a new go routine that establishes a connection to
// the Bosh System metrics Server.
// It returns a shutdown function that blocks until the grpc stream client
// has been successfully closed.
func (i *Ingress) Start() func() {
	i.logger.Println("Starting ingestor...")
	done := make(chan struct{})
	stop := make(chan struct{})

	go func() {
		defer close(done)

		token, err := i.auth.Token()
		if err != nil {
			i.logger.Fatalf("unable to get token: %s", err)
		}
		for {
			select {
			case <-stop:
				return
			default:
			}

			metricsStreamClient, err := i.establishStream(token)
			if err != nil {
				newToken, tokenWasFetched := i.checkPermissionDeniedError(err)

				if tokenWasFetched {
					token = newToken
				}

				connErrCounter.Inc()
				i.logger.Printf("error creating stream connection to metrics server: %s\n", err)
				time.Sleep(i.reconnectWait)
				continue
			}

			err = i.processMessages(metricsStreamClient)
			if err != nil {
				newToken, tokenWasFetched := i.checkPermissionDeniedError(err)

				if tokenWasFetched {
					token = newToken
				}

				if !errors.Is(err, context.DeadlineExceeded) {
					receiveErrCounter.Inc()
					i.logger.Printf("error receiving from metrics server: %s\n", err)
				}
				time.Sleep(i.reconnectWait)
			}
		}
	}()

	return func() {
		i.logger.Println("closing connection to metrics server")

		i.mu.Lock()
		defer i.mu.Unlock()

		close(stop)
		i.metricsServerCancel()
		<-done
	}
}

func (i *Ingress) checkPermissionDeniedError(sourceError error) (newToken string, tokenWasFetched bool) {
	s, ok := status.FromError(sourceError)
	if ok && s.Code() == codes.PermissionDenied {
		i.logger.Printf("authorization failure, retrieving token: %s\n", sourceError)
		token, err := i.auth.Token()
		if err != nil {
			i.logger.Fatalf("unable to refresh token: %s", err)
		}

		return token, true
	}

	return "", false
}

func (i *Ingress) processMessages(client definitions.Egress_BoshMetricsClient) error {
	for {
		event, err := client.Recv()
		if err != nil {
			return err
		}
		receivedCounter.Inc()

		envelope, err := i.convert(event)
		if err != nil {
			convertErrCounter.Inc()
			continue
		}

		select {
		case i.messages <- envelope:
		default:
			droppedCounter.Inc()
		}
	}
}

func (i *Ingress) establishStream(token string) (definitions.Egress_BoshMetricsClient, error) {
	md := metadata.Pairs("authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	metricsServerCtx, metricsServerCancel := context.WithTimeout(ctx, i.streamTimeout)

	i.mu.Lock()
	defer i.mu.Unlock()
	i.metricsServerCancel = metricsServerCancel

	return i.client.BoshMetrics(
		metricsServerCtx,
		&definitions.EgressRequest{
			SubscriptionId: i.subscriptionID,
		},
	)
}
