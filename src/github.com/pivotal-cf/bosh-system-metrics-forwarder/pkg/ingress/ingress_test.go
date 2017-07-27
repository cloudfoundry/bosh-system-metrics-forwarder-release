package ingress_test

import (
	"errors"
	"io/ioutil"
	"log"
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/ingress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"sync"
)

func TestStartProcessesEvents(t *testing.T) {
	RegisterTestingT(t)

	receiver := newSpyReceiver()
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)

	i := ingress.New(receiver, mapper.F, messages)
	defer i.Start()()

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartRetriesUponReceiveError(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	receiver.RecvError(errors.New("some error"))
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)

	i := ingress.New(receiver, mapper.F, messages)
	defer i.Start()()

	Consistently(messages).ShouldNot(Receive())

	receiver.RecvError(nil)

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartContinuesUponConversionError(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	mapper := newSpyMapper(envelope, errors.New("conversion error"))
	messages := make(chan *loggregator_v2.Envelope, 1)

	i := ingress.New(receiver, mapper.F, messages)
	defer i.Start()()

	Consistently(messages).ShouldNot(Receive())

	mapper.ConvertError(nil)

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartDoesNotBlockSendingEnvelopes(t *testing.T) {
	RegisterTestingT(t)

	receiver := newSpyReceiver()
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 2)

	i := ingress.New(receiver, mapper.F, messages)
	defer i.Start()()

	Eventually(receiver.RecvCallCount).Should(BeNumerically(">", 3))

}

type spyReceiver struct {
	mu sync.Mutex
	recvError     error
	recvCallCount int64
}

func newSpyReceiver() *spyReceiver {
	return &spyReceiver{}
}

func (r *spyReceiver) RecvError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recvError = err
}

func (r *spyReceiver) Recv() (*definitions.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	atomic.AddInt64(&r.recvCallCount, 1)
	if r.recvError != nil {
		return nil, r.recvError
	}
	return &definitions.Event{}, nil
}

func (r *spyReceiver) RecvCallCount() int64 {
	return atomic.LoadInt64(&r.recvCallCount)
}

type spyMapper struct {
	mu sync.Mutex
	convertError error
	Envelope     *loggregator_v2.Envelope
}

func newSpyMapper(envelope *loggregator_v2.Envelope, err error) *spyMapper {
	return &spyMapper{
		Envelope:     envelope,
		convertError: err,
	}
}

func (s *spyMapper) ConvertError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convertError = err
}

func (s *spyMapper) F(event *definitions.Event) (*loggregator_v2.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Envelope, s.convertError
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
