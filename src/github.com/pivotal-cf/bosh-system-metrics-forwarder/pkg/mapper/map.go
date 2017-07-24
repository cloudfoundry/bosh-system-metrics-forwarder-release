package mapper

import (
	"errors"

	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
)

func Map(event *definitions.Event) (*loggregator_v2.Envelope, error) {
	switch event.Message.(type) {
	case *definitions.Event_Heartbeat:
		return mapHeartbeat(event), nil
	default:
		return nil, errors.New("metric type not supported")
	}
}

func mapHeartbeat(event *definitions.Event) *loggregator_v2.Envelope {

	gaugeMetrics := make(map[string]*loggregator_v2.GaugeValue, len(event.GetHeartbeat().GetMetrics()))

	for _, v := range event.GetHeartbeat().GetMetrics() {
		gaugeMetrics[v.Name] = &loggregator_v2.GaugeValue{
			Value: v.Value,
			Unit:  eventNameToUnit[v.Name],
		}

	}

	return &loggregator_v2.Envelope{
		Timestamp: event.Timestamp,
		Tags: map[string]*loggregator_v2.Value{
			"job": {Data: &loggregator_v2.Value_Text{
				Text: event.GetHeartbeat().GetJob(),
			}},
			"index": {Data: &loggregator_v2.Value_Integer{
				Integer: int64(event.GetHeartbeat().GetIndex()),
			}},
			"id": {Data: &loggregator_v2.Value_Text{
				Text: event.GetHeartbeat().GetInstanceId(),
			}},
			"origin": {Data: &loggregator_v2.Value_Text{
				Text: "bosh-system-metrics-forwarder",
			}},
			"deployment": {Data: &loggregator_v2.Value_Text{
				Text: event.GetDeployment(),
			}},
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
