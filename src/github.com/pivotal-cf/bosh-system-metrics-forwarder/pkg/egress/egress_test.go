package egress_test

import (
	"errors"
	"io/ioutil"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/egress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestStartProcessesEvents(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	sender := newSpySender()
	client := newSpyEgressClient(sender, nil)
	messages := make(chan *loggregator_v2.Envelope)

	egress := egress.New(client, messages)
	egress.Start()

	messages <- envelope

	Eventually(sender.SentEnvelopes).Should(Receive(Equal(envelope)))
}

func TestStartDoesNotDropMessageWhenConnectionDies(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	sender := newSpySender()
	sender.SendError(errors.New("some error"))
	client := newSpyEgressClient(sender, nil)
	messages := make(chan *loggregator_v2.Envelope)

	egress := egress.New(client, messages)
	egress.Start()

	messages <- envelope

	Eventually(sender.SendCallCount, "2s", "10ms").Should(BeNumerically(">", 0))

	sender.SendError(nil)

	Eventually(sender.SentEnvelopes).Should(Receive(Equal(envelope)))
}

func TestStopDrainsMessagesBeforeClosing(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	sender := newSpySender()
	client := newSpyEgressClient(sender, nil)
	messages := make(chan *loggregator_v2.Envelope, 100)
	egress := egress.New(client, messages)

	for i := 0; i < 100; i++ {
		messages <- envelope
	}

	stop := egress.Start()

	Eventually(client.SenderCallCount).Should(BeNumerically(">", 0))
	Expect(len(messages)).To(BeNumerically(">", 0))

	close(messages)
	stop()

	Expect(messages).To(HaveLen(0))
	Expect(sender.CloseAndRecvCallCount()).To(BeNumerically("==", 1))
}

func TestStartReconnectsWhenClientUnableToCreateSender(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	client := newSpyEgressClient(nil, errors.New("metron is down"))
	messages := make(chan *loggregator_v2.Envelope, 100)
	egress := egress.New(client, messages)

	egress.Start()

	messages <- envelope

	Eventually(client.SenderCallCount).Should(BeNumerically(">", 1))
}

func TestStartReconnectsOnSendError(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	sender := newSpySender()
	sender.SendError(errors.New("some error"))

	client := newSpyEgressClient(sender, nil)
	messages := make(chan *loggregator_v2.Envelope, 100)
	egress := egress.New(client, messages)

	egress.Start()

	messages <- envelope

	Eventually(client.SenderCallCount).Should(BeNumerically(">", 1))
}

type spyEgressClient struct {
	senderCallCount int32
	spySender       *spySender
	err             error
}

func newSpyEgressClient(s *spySender, e error) *spyEgressClient {
	return &spyEgressClient{
		spySender: s,
		err:       e,
	}
}

func (s *spyEgressClient) Sender(ctx context.Context, opts ...grpc.CallOption) (loggregator_v2.Ingress_SenderClient, error) {
	atomic.AddInt32(&s.senderCallCount, 1)

	return s.spySender, s.err
}

func (s *spyEgressClient) SenderCallCount() int32 {
	return atomic.LoadInt32(&s.senderCallCount)
}

type spySender struct {
	mu            sync.Mutex
	sendCallCount int
	sendError     error

	SentEnvelopes         chan *loggregator_v2.Envelope
	closeAndRecvCallCount int32

	grpc.ClientStream
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

	s.sendCallCount++

	if s.sendError != nil {
		return s.sendError
	}

	time.Sleep(10 * time.Millisecond)

	s.SentEnvelopes <- e
	return nil
}

func (s *spySender) SendCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sendCallCount
}

func (s *spySender) CloseAndRecv() (*loggregator_v2.IngressResponse, error) {
	atomic.AddInt32(&s.closeAndRecvCallCount, 1)
	return nil, nil
}

func (s *spySender) CloseAndRecvCallCount() int32 {
	return atomic.LoadInt32(&s.closeAndRecvCallCount)
}

var envelope = &loggregator_v2.Envelope{
	Timestamp: 1499293724,
	Tags: map[string]string{
		"job": "consul",
		"index": "4",
		"id": "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
		"origin": "bosh-system-metrics-forwarder",
		"deployment": "loggregator",
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"system.healthy": {Value: 1, Unit: "b"},
			},
		},
	},
}
