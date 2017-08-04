package ingress

import (
	"expvar"
	"log"
	"time"

	"context"
	"sync"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
)

type receiver interface {
	Recv() (*definitions.Event, error)
}

type sender interface {
	Send(*loggregator_v2.Envelope) error
}

type mapper func(event *definitions.Event) (*loggregator_v2.Envelope, error)

type Ingress struct {
	convert  mapper
	messages chan *loggregator_v2.Envelope
	client   definitions.EgressClient

	mu                  sync.Mutex
	metricsServerCancel context.CancelFunc
	subscriptionID string
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

func New(s definitions.EgressClient, m mapper, messages chan *loggregator_v2.Envelope, sID string) *Ingress {
	return &Ingress{
		client:   s,
		convert:  m,
		messages: messages,
		subscriptionID: sID,
	}
}

func (i *Ingress) Start() func() {
	log.Println("Starting ingestor...")
	go func() {
		for {
			metricsStreamClient, err := i.establishStream()
			if err != nil {

				s, ok := status.FromError(err)
				if ok && s.Code() == codes.PermissionDenied {
					log.Fatalf("Authorization failure: %s", err)
				}

				connErrCounter.Add(1)
				log.Printf("error creating stream connection to metrics server: %s", err)
				time.Sleep(250 * time.Millisecond)
				continue
			}

			err = i.processMessages(metricsStreamClient)
			if err != nil {
				receiveErrCounter.Add(1)
				log.Printf("Error receiving from metrics server: %s", err)
				time.Sleep(250 * time.Millisecond)
				continue
			}
		}
	}()

	return func() {
		i.mu.Lock()
		defer i.mu.Unlock()

		i.metricsServerCancel()
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

func (i *Ingress) establishStream() (definitions.Egress_BoshMetricsClient, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	metricsServerCtx, metricsServerCancel := context.WithCancel(context.Background())

	i.metricsServerCancel = metricsServerCancel

	return i.client.BoshMetrics(
		metricsServerCtx,
		&definitions.EgressRequest{
			SubscriptionId: i.subscriptionID,
		},
	)
}
