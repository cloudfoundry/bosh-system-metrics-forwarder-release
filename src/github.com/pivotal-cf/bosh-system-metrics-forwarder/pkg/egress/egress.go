package egress

import (
	"expvar"
	"log"
	"time"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
)

type sender interface {
	Send(*loggregator_v2.Envelope) error
	CloseAndRecv() (*loggregator_v2.IngressResponse, error)
}

type Egress struct {
	messages chan *loggregator_v2.Envelope
	snd      sender
}

var (
	sendErrCounter *expvar.Int
	sentCounter    *expvar.Int
)

func init() {
	sendErrCounter = expvar.NewInt("egress.send_err")
	sentCounter = expvar.NewInt("egress.sent")
}

func New(s sender, m chan *loggregator_v2.Envelope) *Egress {
	return &Egress{
		snd:      s,
		messages: m,
	}
}

func (e *Egress) Start() func() {
	done := make(chan struct{})

	go func() {
		log.Println("Starting forwarder...")
		for envelope := range e.messages {
			err := e.sendWithRetry(envelope)
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
		e.snd.CloseAndRecv()
	}
}

func (e *Egress) sendWithRetry(envelope *loggregator_v2.Envelope) error {
	var err error

	for i := 0; i < 3; i++ {
		err = e.snd.Send(envelope)
		if err == nil {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return err
}
