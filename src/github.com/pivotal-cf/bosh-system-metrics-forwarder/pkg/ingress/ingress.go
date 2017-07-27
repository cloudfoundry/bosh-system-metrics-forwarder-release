package ingress

import (
	"log"
	"time"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
)

type receiver interface {
	Recv() (*definitions.Event, error)
}

type sender interface {
	Send(*loggregator_v2.Envelope) error
}

type mapper func(event *definitions.Event) (*loggregator_v2.Envelope, error)

type Ingress struct {
	rcv      receiver
	convert  mapper
	messages chan *loggregator_v2.Envelope
}

func New(r receiver, m mapper, messages chan *loggregator_v2.Envelope) *Ingress {
	return &Ingress{
		rcv:      r,
		convert:  m,
		messages: messages,
	}
}

func (i *Ingress) Start() func() {

	log.Println("Starting ingestor...")
	go func() {
		for {
			event, err := i.rcv.Recv()
			if err != nil {
				// TODO: counter metric
				log.Printf("Error receiving from metrics server: %s", err)
				time.Sleep(250 * time.Millisecond)
				continue
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
	}()

	return func() {}
}
