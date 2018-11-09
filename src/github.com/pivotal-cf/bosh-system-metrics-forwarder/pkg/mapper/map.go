package mapper

import (
	"errors"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
)

// New returns a function that converts a bosh Event to an envelope.
// It only process heartbeat events.
// It returns an error if it receives a message type isn't a heartbeat type.
// It takes an IP tag which overrides the `ip` tag on the envelope.
func New(ipTag string) func(event *definitions.Event) (*loggregator_v2.Envelope, error) {
	return func(event *definitions.Event) (*loggregator_v2.Envelope, error) {
		switch event.Message.(type) {
		case *definitions.Event_Heartbeat:
			return mapHeartbeat(event, ipTag), nil
		default:
			return nil, errors.New("metric type not supported")
		}
	}
}

func mapHeartbeat(event *definitions.Event, ipTag string) *loggregator_v2.Envelope {

	gaugeMetrics := make(map[string]*loggregator_v2.GaugeValue, len(event.GetHeartbeat().GetMetrics()))

	for _, v := range event.GetHeartbeat().GetMetrics() {
		gaugeMetrics[v.Name] = &loggregator_v2.GaugeValue{
			Value: v.Value,
			Unit:  eventNameToUnit[v.Name],
		}

	}

	return &loggregator_v2.Envelope{
		Timestamp: event.Timestamp,
		Tags: map[string]string{
			"job": event.GetHeartbeat().GetJob(),
			"index": event.GetHeartbeat().GetInstanceId(),
			"id": event.GetHeartbeat().GetInstanceId(),
			"origin": "bosh-system-metrics-forwarder",
			"deployment": event.GetDeployment(),
			"ip": ipTag,
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: gaugeMetrics,
			},
		},
	}
}

var eventNameToUnit = map[string]string{
	"system.healthy":                       "b",
	"system.load.1m":                       "Load",
	"system.cpu.user":                      "Load",
	"system.cpu.sys":                       "Load",
	"system.cpu.wait":                      "Load",
	"system.disk.system.percent":           "Percent",
	"system.disk.system.inode_percent":     "Percent",
	"system.mem.percent":                   "Percent",
	"system.swap.percent":                  "Percent",
	"system.disk.ephemeral.percent":        "Percent",
	"system.disk.ephemeral.inode_percent":  "Percent",
	"system.disk.persistent.percent":       "Percent",
	"system.disk.persistent.inode_percent": "Percent",
	"system.mem.kb":                        "Kb",
	"system.swap.kb":                       "Kb",
}
