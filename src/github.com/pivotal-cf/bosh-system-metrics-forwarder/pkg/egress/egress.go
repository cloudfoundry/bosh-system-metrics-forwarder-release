package egress

import (
	"expvar"
	"log"
	"time"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type client interface {
	Sender(ctx context.Context, opts ...grpc.CallOption) (loggregator_v2.Ingress_SenderClient, error)
}

type sender interface {
	Send(*loggregator_v2.Envelope) error
	CloseAndRecv() (*loggregator_v2.IngressResponse, error)
}

type Egress struct {
	messages <-chan *loggregator_v2.Envelope
	client   client
	retry    chan *loggregator_v2.Envelope
}

var (
	sendErrCounter *expvar.Int
	droppedCounter *expvar.Int
	sentCounter    *expvar.Int
)

func init() {
	sendErrCounter = expvar.NewInt("egress.send_err")
	droppedCounter = expvar.NewInt("egress.dropped")
	sentCounter = expvar.NewInt("egress.sent")
}

func New(c client, m <-chan *loggregator_v2.Envelope) *Egress {
	return &Egress{
		client:   c,
		messages: m,
		retry:    make(chan *loggregator_v2.Envelope, 1),
	}
}

func (e *Egress) Start() func() {
	log.Println("Starting forwarder...")

	done := make(chan struct{})
	stop := make(chan struct{})

	go func() {
		defer close(done)

		var (
			snd loggregator_v2.Ingress_SenderClient
			err error
		)

		for {
			select {
			case <-stop:
				if snd != nil {
					snd.CloseAndRecv()
				}
				return
			default:
			}

			snd, err = e.client.Sender(context.Background())
			if err != nil {
				log.Fatalf("error creating stream connection to metron: %s", err)
			}

			log.Println("metron stream created")

			err = e.processMessages(snd)
			if err != nil {
				log.Printf("error sending to log agent: %s\n", err)
				sendErrCounter.Add(1)
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func (e *Egress) processMessages(snd loggregator_v2.Ingress_SenderClient) error {
	err := e.processRetries(snd)
	if err != nil {
		return err
	}

	for envelope := range e.messages {
		err := snd.Send(envelope)
		if err != nil {
			e.retryLater(envelope)
			return err
		}

		sentCounter.Add(1)
	}

	return nil
}

func (e *Egress) retryLater(envelope *loggregator_v2.Envelope) {
	select {
	case e.retry <- envelope:
	default:
		droppedCounter.Add(1)
	}
}

func (e *Egress) processRetries(snd sender) error {
	for {
		select {
		case envelope := <-e.retry:
			err := snd.Send(envelope)
			if err != nil {
				droppedCounter.Add(1)
				return err
			}

			sentCounter.Add(1)
		default:
			return nil
		}
	}
}
