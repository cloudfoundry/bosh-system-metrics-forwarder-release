package egress_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/egress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"sync"
)

func TestStartProcessesEvents(t *testing.T) {
	RegisterTestingT(t)

	sender := newSpySender()
	messages := make(chan *loggregator_v2.Envelope)

	egress := egress.New(sender, messages)
	defer egress.Start()()

	messages <- envelope

	Eventually(sender.SentEnvelopes).Should(Receive(Equal(envelope)))
}

func TestStartRetriesUponSendError(t *testing.T) {
	RegisterTestingT(t)
	sender := newSpySender()
	sender.SendError(errors.New("some error"))
	messages := make(chan *loggregator_v2.Envelope)

	egress := egress.New(sender, messages)
	defer egress.Start()()

	messages <- envelope

	Consistently(sender.SentEnvelopes).ShouldNot(Receive())

	sender.SendError(nil)

	Eventually(sender.SentEnvelopes).Should(Receive(Equal(envelope)))
}

type spySender struct {
	mu sync.Mutex
	sendError     error
	SentEnvelopes chan *loggregator_v2.Envelope

}

func newSpySender() *spySender {
	return &spySender{
		SentEnvelopes: make(chan *loggregator_v2.Envelope, 100),
	}
}

func (s *spySender) SendError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendError = err
}

func (s *spySender) Send(e *loggregator_v2.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sendError != nil {
		return s.sendError
	}

	s.SentEnvelopes <- e
	return nil
}

var envelope = &loggregator_v2.Envelope{
	Timestamp: 1499293724,
	Tags: map[string]*loggregator_v2.Value{
		"job": {Data: &loggregator_v2.Value_Text{
			Text: "consul",
		}},
		"index": {Data: &loggregator_v2.Value_Integer{
			Integer: 4,
		}},
		"id": {Data: &loggregator_v2.Value_Text{
			Text: "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
		}},
		"origin": {Data: &loggregator_v2.Value_Text{
			Text: "bosh-system-metrics-forwarder",
		}},
		"deployment": {Data: &loggregator_v2.Value_Text{
			Text: "loggregator",
		}},
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"system.healthy": {Value: 1, Unit: "b"},
			},
		},
	},
}

var heartbeatEvent = &definitions.Event{
	Id:         "55b68400-f984-4f76-b341-cf849e07d4f9",
	Timestamp:  1499293724,
	Deployment: "loggregator",
	Message: &definitions.Event_Heartbeat{
		Heartbeat: &definitions.Heartbeat{
			AgentId:    "2accd102-37e7-4dd6-b337-b3f87da97914",
			Job:        "consul",
			Index:      4,
			InstanceId: "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
			JobState:   "running",
			Vitals:     &definitions.Heartbeat_Vitals{},
			Metrics: []*definitions.Heartbeat_Metric{
				{
					Name:      "system.healthy",
					Value:     1,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
			},
		},
	},
}
