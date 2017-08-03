package ingress

import (
	"expvar"
	"log"
	"time"

	"context"
	"sync"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
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
	GetToken() (string, error)
}

type mapper func(event *definitions.Event) (*loggregator_v2.Envelope, error)

type Ingress struct {
	convert  mapper
	messages chan *loggregator_v2.Envelope
	client   definitions.EgressClient
	auth     tokener

	mu                  sync.Mutex
	metricsServerCancel context.CancelFunc
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
) *Ingress {
	return &Ingress{
		client:   s,
		convert:  m,
		messages: messages,
		auth:     auth,
	}
}

func (i *Ingress) Start() func() {
	log.Println("Starting ingestor...")
	go func() {
		for {
			token, err := i.auth.GetToken()
			// if err != nil {
			// 	log.Fatalf("unable to get token, cannot establish stream: %s", err)
			// }
			metricsStreamClient, err := i.establishStream(token)
			if err != nil {

				s, ok := status.FromError(err)
				if ok && s.Code() == codes.PermissionDenied {
					log.Printf("Authorization failure, retrieving token: %s", err)
					token, _ := i.auth.RefreshToken()
					// TODO: handle err
					i.establishStream(token)
				} else {
					connErrCounter.Add(1)
					log.Printf("error creating stream connection to metrics server: %s", err)
					time.Sleep(250 * time.Millisecond)
					continue
				}
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

func (i *Ingress) establishStream(token string) (definitions.Egress_BoshMetricsClient, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	md := metadata.Pairs("authorization", token)
	ctx := metadata.NewContext(context.Background())
	metricsServerCtx, metricsServerCancel := context.WithCancel(ctx)

	i.metricsServerCancel = metricsServerCancel

	return i.client.BoshMetrics(
		metricsServerCtx,
		&definitions.EgressRequest{
			SubscriptionId: "bosh-system-metrics-forwarder",
		},
	)
}
