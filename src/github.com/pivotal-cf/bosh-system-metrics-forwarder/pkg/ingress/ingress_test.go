package ingress_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"errors"
	"io/ioutil"
	"log"
	"sync"

	"time"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/ingress"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestStartProcessesEvents(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id")
	i.Start()

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartRetriesUponReceiveError(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	receiver.RecvError(errors.New("some error"))
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id", ingress.WithReconnectWait(time.Millisecond))
	i.Start()

	Eventually(client.BoshMetricsCallCount).Should(BeNumerically(">", 1))

	receiver.RecvError(nil)

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartGetsToken(t *testing.T) {
	RegisterTestingT(t)

	receiver := newSpyReceiver()
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id", ingress.WithReconnectWait(time.Millisecond))
	i.Start()

	Eventually(tokener.TokenCallCount).Should(Equal(int32(1)))
	Eventually(client.BoshMetricsCallCount).Should(Equal(int32(1)))
	md, ok := metadata.FromOutgoingContext(client.LatestContext())
	Expect(ok).To(BeTrue())
	Expect(md["authorization"][0]).To(Equal("token0"))
}

func TestStartRefreshesTokenUponPermissionDeniedError(t *testing.T) {
	RegisterTestingT(t)

	receiver := newSpyReceiver()
	client := newSpyEgressClient(
		receiver,
		status.Error(codes.PermissionDenied, "some-error"),
	)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 1)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id")
	i.Start()

	Eventually(tokener.TokenCallCount).Should(BeNumerically(">", 1))
	Eventually(client.BoshMetricsCallCount, "2s").Should(BeNumerically(">", 1))
	md, ok := metadata.FromOutgoingContext(client.LatestContext())
	Expect(ok).To(BeTrue())
	// token0 is the first token generated. We want a different token.
	Expect(md["authorization"][0]).ToNot(Equal("token0"))
}

func TestStartContinuesUponConversionError(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, errors.New("conversion error"))
	messages := make(chan *loggregator_v2.Envelope, 1)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id", ingress.WithReconnectWait(time.Millisecond))
	i.Start()

	Consistently(messages).ShouldNot(Receive())

	mapper.ConvertError(nil)

	Eventually(messages).Should(Receive(Equal(envelope)))
}

func TestStartDoesNotBlockSendingEnvelopes(t *testing.T) {
	RegisterTestingT(t)

	receiver := newSpyReceiver()
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 2)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id", ingress.WithReconnectWait(time.Millisecond))
	i.Start()

	Eventually(receiver.RecvCallCount).Should(BeNumerically(">", 3))
}

func TestStartDoesNotReconnectAfterStopping(t *testing.T) {
	RegisterTestingT(t)
	log.SetOutput(ioutil.Discard)

	receiver := newSpyReceiver()
	receiver.RecvError(grpc.ErrClientConnClosing)
	client := newSpyEgressClient(receiver, nil)
	mapper := newSpyMapper(envelope, nil)
	messages := make(chan *loggregator_v2.Envelope, 2)
	tokener := newSpyTokener()

	i := ingress.New(client, mapper.F, messages, tokener, "sub-id", ingress.WithReconnectWait(250*time.Millisecond))
	stop := i.Start()

	Eventually(client.BoshMetricsCallCount).Should(Equal(int32(1)))

	stop()

	Consistently(client.BoshMetricsCallCount).Should(Equal(int32(1)))
}

type spyEgressClient struct {
	boshMetricsCallCount int32
	receiver             definitions.Egress_BoshMetricsClient
	err                  error

	mu  sync.Mutex
	ctx context.Context
}

func newSpyEgressClient(recv definitions.Egress_BoshMetricsClient, err error) *spyEgressClient {
	return &spyEgressClient{
		receiver: recv,
		err:      err,
	}
}

func (c *spyEgressClient) BoshMetrics(ctx context.Context, r *definitions.EgressRequest, opts ...grpc.CallOption) (definitions.Egress_BoshMetricsClient, error) {
	atomic.AddInt32(&c.boshMetricsCallCount, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctx = ctx
	return c.receiver, c.err
}

func (c *spyEgressClient) LatestContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx
}

func (c *spyEgressClient) BoshMetricsCallCount() int32 {
	return atomic.LoadInt32(&c.boshMetricsCallCount)
}

type spyReceiver struct {
	mu            sync.Mutex
	recvError     error
	recvCallCount int32
	grpc.ClientStream
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

	atomic.AddInt32(&r.recvCallCount, 1)
	if r.recvError != nil {
		return nil, r.recvError
	}
	return &definitions.Event{}, nil
}

func (r *spyReceiver) RecvCallCount() int32 {
	return atomic.LoadInt32(&r.recvCallCount)
}

type spyMapper struct {
	mu           sync.Mutex
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

type spyTokener struct {
	tokenCallCount int32
	err            error
}

type spyTokenOpt func(*spyTokener)

func WithError(e string) spyTokenOpt {
	return func(t *spyTokener) {
		t.err = errors.New(e)
	}
}

func newSpyTokener(opts ...spyTokenOpt) *spyTokener {
	t := &spyTokener{}

	for _, o := range opts {
		o(t)
	}

	return t
}

func (t *spyTokener) Token() (string, error) {
	token := fmt.Sprintf("token%d", atomic.LoadInt32(&t.tokenCallCount))
	atomic.AddInt32(&t.tokenCallCount, 1)
	return token, nil
}

func (t *spyTokener) TokenCallCount() int32 {
	return atomic.LoadInt32(&t.tokenCallCount)
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
