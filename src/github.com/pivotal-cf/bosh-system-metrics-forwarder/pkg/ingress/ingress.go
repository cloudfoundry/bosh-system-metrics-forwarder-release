package ingress

import (
	"log"
	"time"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"context"
	"sync"
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
}

func New(s definitions.EgressClient, m mapper, messages chan *loggregator_v2.Envelope) *Ingress {
	return &Ingress{
		client:   s,
		convert:  m,
		messages: messages,
	}
}

func (i *Ingress) Start() func() {

	log.Println("Starting ingestor...")

	go func() {
		for {
			metricsStreamClient, err := i.establishStream()
			if err != nil {
				// TODO: counter metric
				log.Printf("error creating stream connection to metrics server: %s", err)
				time.Sleep(250 * time.Millisecond)
				continue
			}

			err = i.processMessages(metricsStreamClient)
			if err != nil {
				// TODO: counter metric
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
			// TODO: counter metric
			continue
		}

		select {
		case i.messages <- envelope:
		default:
			// TODO: counter metric
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
			SubscriptionId: "bosh-system-metrics-forwarder",
		},
	)
}
