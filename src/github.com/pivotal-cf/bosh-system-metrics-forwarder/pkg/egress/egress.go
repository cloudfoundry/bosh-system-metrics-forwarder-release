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
	messages chan *loggregator_v2.Envelope
	client   client
}

var (
	sendErrCounter *expvar.Int
	sentCounter    *expvar.Int
)

func init() {
	sendErrCounter = expvar.NewInt("egress.send_err")
	sentCounter = expvar.NewInt("egress.sent")
}

func New(c client, m chan *loggregator_v2.Envelope) *Egress {
	return &Egress{
		client:   c,
		messages: m,
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

			ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
			snd, err = e.client.Sender(ctx)
			if err != nil {
				log.Fatalf("error creating stream connection to metron: %s", err)
			}

			err = e.processMessages(snd)
			if err != nil {
				log.Printf("Error sending to log agent: %s", err)
				sendErrCounter.Add(1)
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func (e *Egress) processMessages(snd loggregator_v2.Ingress_SenderClient) error {
	for envelope := range e.messages {
		err := e.sendWithRetry(snd, envelope)
		if err != nil {
			return err
		}

		sentCounter.Add(1)
	}

	return nil
}

func (e *Egress) sendWithRetry(snd sender, envelope *loggregator_v2.Envelope) error {
	var err error

	for i := 0; i < 3; i++ {
		err = snd.Send(envelope)
		if err == nil {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return err
}
