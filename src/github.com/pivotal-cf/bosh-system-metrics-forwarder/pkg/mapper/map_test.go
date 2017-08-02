package mapper_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/definitions"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/loggregator_v2"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/mapper"
)

func TestMapHeartbeat(t *testing.T) {
	RegisterTestingT(t)

	envelope, err := mapper.Map(heartbeatEvent)
	Expect(err).ToNot(HaveOccurred())

	Expect(envelope).To(Equal(&loggregator_v2.Envelope{
		Timestamp: 1499293724,
		Tags: map[string]*loggregator_v2.Value{
			"job": {Data: &loggregator_v2.Value_Text{
				Text: "consul",
			}},
			"index": {Data: &loggregator_v2.Value_Text{
				Text: "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
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
					"system.load.1m":                       {Value: 0.18, Unit: "Load"},
					"system.cpu.user":                      {Value: 2.5, Unit: "Load"},
					"system.cpu.sys":                       {Value: 3.2, Unit: "Load"},
					"system.cpu.wait":                      {Value: 0.0, Unit: "Load"},
					"system.mem.percent":                   {Value: 28, Unit: "Percent"},
					"system.mem.kb":                        {Value: 1139140, Unit: "Kb"},
					"system.swap.percent":                  {Value: 0, Unit: "Percent"},
					"system.swap.kb":                       {Value: 9788, Unit: "Kb"},
					"system.disk.system.percent":           {Value: 23, Unit: "Percent"},
					"system.disk.system.inode_percent":     {Value: 14, Unit: "Percent"},
					"system.disk.ephemeral.percent":        {Value: 4, Unit: "Percent"},
					"system.disk.ephemeral.inode_percent":  {Value: 2, Unit: "Percent"},
					"system.disk.persistent.percent":       {Value: 4, Unit: "Percent"},
					"system.disk.persistent.inode_percent": {Value: 2, Unit: "Percent"},
					"system.healthy":                       {Value: 1, Unit: "b"},
				},
			},
		},
	}))
}

func TestMapIgnoresAlerts(t *testing.T) {
	RegisterTestingT(t)

	_, err := mapper.Map(alertEvent)
	Expect(err).To(HaveOccurred())
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
			Metrics: []*definitions.Heartbeat_Metric{
				{
					Name:      "system.load.1m",
					Value:     0.18,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.cpu.user",
					Value:     2.5,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.cpu.sys",
					Value:     3.2,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.cpu.wait",
					Value:     0.0,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.mem.percent",
					Value:     28,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.mem.kb",
					Value:     1139140,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.swap.percent",
					Value:     0,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.swap.kb",
					Value:     9788,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.system.percent",
					Value:     23,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.system.inode_percent",
					Value:     14,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.ephemeral.percent",
					Value:     4,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.ephemeral.inode_percent",
					Value:     2,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.persistent.percent",
					Value:     4,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
				{
					Name:      "system.disk.persistent.inode_percent",
					Value:     2,
					Timestamp: 1499293724,
					Tags: map[string]string{
						"job":   "consul",
						"index": "0",
						"id":    "6f60a3ce-9e4d-477f-ba45-7d29bcfab5b9",
					},
				},
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

var alertEvent = &definitions.Event{
	Id:         "93eb25a4-9348-4232-6f71-69e1e01081d7",
	Timestamp:  1499359162,
	Deployment: "loggregator",
	Message: &definitions.Event_Alert{
		Alert: &definitions.Alert{
			Severity: 4,
			Category: "",
			Title:    "SSH Access Denied",
			Summary:  "Failed password for vcap from 10.244.0.1 port 38732 ssh2",
			Source:   "loggregator: log-api(6f721317-2399-4e38-b38c-9d1b213c2d67) [id=130a69f5-6da1-45ce-830e-31e9c856085a, index=0, cid=b5df1c77-2c91-4093-6fc5-1cf2cba72471]",
		},
	},
}
