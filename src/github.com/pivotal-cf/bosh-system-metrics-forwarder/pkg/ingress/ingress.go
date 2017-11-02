package ingress

import (
	"expvar"
	"log"
	"time"

	"sync"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	connErrCounter    *expvar.Int
	receiveErrCounter *expvar.Int
	convertErrCounter *expvar.Int
	receivedCounter   *expvar.Int
	droppedCounter    *expvar.Int
)

func init() {
	connErrCounter = expvar.NewInt("ingress.stream_conn_err")       // Tracks errors when a stream needs to be established
	receiveErrCounter = expvar.NewInt("ingress.stream_receive_err") // Tracks errors when receiving events from metrics server
	convertErrCounter = expvar.NewInt("ingress.stream_convert_err") // Tracks errors when converting an event to an envelope
	receivedCounter = expvar.NewInt("ingress.received")             // Tracks total number of events received
	droppedCounter = expvar.NewInt("ingress.dropped")               // Tracks the number of envelopes dropped if unable to queue the msg
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
	subscriptionID string

	mu                  sync.Mutex
	metricsServerCancel context.CancelFunc
}

type IngressOpt func(*Ingress)

func WithReconnectWait(d time.Duration) IngressOpt {
	return func(i *Ingress) {
		i.reconnectWait = d
	}
}

// New returns a new Ingress.
func New(
	s definitions.EgressClient,
	m mapper,
	messages chan *loggregator_v2.Envelope,
	auth tokener,
	sID string,
	opts ...IngressOpt,
) *Ingress {
	i := &Ingress{
		client:              s,
		convert:             m,
		messages:            messages,
		auth:                auth,
		subscriptionID:      sID,
		metricsServerCancel: func() {},
		reconnectWait:       time.Second,
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
	log.Println("Starting ingestor...")
	done := make(chan struct{})
	stop := make(chan struct{})

	go func() {
		defer close(done)

		token, err := i.auth.Token()
		if err != nil {
			log.Fatalf("unable to get token: %s", err)
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

				connErrCounter.Add(1)
				log.Printf("error creating stream connection to metrics server: %s\n", err)
				time.Sleep(i.reconnectWait)
				continue
			}

			log.Println("metrics server stream created")
			err = i.processMessages(metricsStreamClient)
			if err != nil {
				newToken, tokenWasFetched := i.checkPermissionDeniedError(err)

				if tokenWasFetched {
					token = newToken
				}

				receiveErrCounter.Add(1)
				log.Printf("error receiving from metrics server: %s\n", err)
				time.Sleep(i.reconnectWait)
				continue
			}
		}
	}()

	return func() {
		log.Println("closing connection to metrics server")

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
		log.Printf("authorization failure, retrieving token: %s\n", sourceError)
		token, err := i.auth.Token()
		if err != nil {
			log.Fatalf("unable to refresh token: %s", err)
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
		receivedCounter.Add(1)

		envelope, err := i.convert(event)
		if err != nil {
			convertErrCounter.Add(1)
			continue
		}

		select {
		case i.messages <- envelope:
		default:
			droppedCounter.Add(1)
		}
	}
}

func (i *Ingress) establishStream(token string) (definitions.Egress_BoshMetricsClient, error) {
	md := metadata.Pairs("authorization", token)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	metricsServerCtx, metricsServerCancel := context.WithCancel(ctx)

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
