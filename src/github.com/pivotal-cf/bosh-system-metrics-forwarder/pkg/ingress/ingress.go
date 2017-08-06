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
	convert  mapper
	messages chan *loggregator_v2.Envelope
	client   definitions.EgressClient
	auth     tokener

	mu                  sync.Mutex
	metricsServerCancel context.CancelFunc
	subscriptionID      string
}

var (
	connErrCounter    *expvar.Int
	receiveErrCounter *expvar.Int
	convertErrCounter *expvar.Int
	receivedCounter   *expvar.Int
	droppedCounter    *expvar.Int
)

func init() {
	connErrCounter = expvar.NewInt("ingress.stream_conn_err")
	receiveErrCounter = expvar.NewInt("ingress.stream_receive_err")
	convertErrCounter = expvar.NewInt("ingress.stream_convert_err")
	receivedCounter = expvar.NewInt("ingress.received")
	droppedCounter = expvar.NewInt("ingress.dropped")
}

func New(
	s definitions.EgressClient,
	m mapper,
	messages chan *loggregator_v2.Envelope,
	auth tokener,
	sID string,
) *Ingress {
	return &Ingress{
		client:              s,
		convert:             m,
		messages:            messages,
		auth:                auth,
		subscriptionID:      sID,
		metricsServerCancel: func() {},
	}
}

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
				s, ok := status.FromError(err)
				if ok && s.Code() == codes.PermissionDenied {
					log.Printf("authorization failure, retrieving token: %s", err)
					token, err = i.auth.Token()
					if err != nil {
						log.Fatalf("unable to refresh token: %s", err)
					}
				} else {
					connErrCounter.Add(1)
					log.Printf("error creating stream connection to metrics server: %s", err)
					time.Sleep(250 * time.Millisecond)
					continue
				}

				connErrCounter.Add(1)
				log.Printf("error creating stream connection to metrics server: %s", err)
				time.Sleep(time.Second)
				continue
			}

			err = i.processMessages(metricsStreamClient)
			if err != nil {
				receiveErrCounter.Add(1)
				log.Printf("error receiving from metrics server: %s", err)
				time.Sleep(250 * time.Millisecond)
				continue
			}
		}
	}()

	return func() {
		log.Println("closing connection to metrics server")

		i.mu.Lock()
		defer i.mu.Unlock()

		i.metricsServerCancel()
		close(stop)
		<-done
	}
}

func (i *Ingress) processMessages(client definitions.Egress_BoshMetricsClient) error {
	for {
		event, err := client.Recv()
		if err != nil {
			return err
		}

		envelope, err := i.convert(event)
		if err != nil {
			convertErrCounter.Add(1)
			continue
		}

		select {
		case i.messages <- envelope:
			receivedCounter.Add(1)
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
