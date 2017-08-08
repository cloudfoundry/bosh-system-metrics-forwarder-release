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
	client client
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
	done := make(chan struct{})

	snd, _ := e.client.Sender(context.Background())

	metronCtx, metronCancel := context.WithCancel(context.Background())
	snd, err := e.client.Sender(metronCtx)
	if err != nil {
		log.Fatalf("error creating stream connection to metron: %s", err)
	}

	go func() {
		log.Println("Starting forwarder...")
		for envelope := range e.messages {
			err := e.sendWithRetry(snd, envelope)
			if err != nil {
				log.Printf("Error sending to log agent: %s", err)
				sendErrCounter.Add(1)
				continue
			}
			sentCounter.Add(1)
		}
		close(done)
	}()

	return func() {
		<-done
		snd.CloseAndRecv()
		metronCancel()
	}
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
