package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
)

var (
	loggregatorAddr = os.Getenv("LOGGREGATOR_ADDR")
	authToken      = os.Getenv("CF_ACCESS_TOKEN")
)

func main() {
	cnsmr := consumer.New(loggregatorAddr, &tls.Config{InsecureSkipVerify: true}, nil)
	msgs, errs := cnsmr.FilteredFirehose("bosh-system-metrics-smoke-tests", authToken, consumer.Metrics)

	go func() {
		for err := range errs {
			fmt.Fprintf(os.Stderr, "Error from the firehose: %v\n", err.Error())
		}
	}()

	t := time.NewTimer(time.Minute)

	verifiedEvents := make(map[string]bool)
	for k, _ := range eventNameToUnit {
		verifiedEvents[k] = false
	}

	fmt.Println("Starting Smoke Tests...")
	for {
		select {
		case msg := <-msgs:
			unit, ok := eventNameToUnit[msg.GetValueMetric().GetName()]
			if ok {
				name := msg.GetValueMetric().GetName()
				if msg.GetValueMetric().GetUnit() == unit {
					verifiedEvents[name] = true
				}
			}
		case <-t.C:
			failed := false
			for k, verified := range verifiedEvents {
				if !verified {
					fmt.Fprintf(os.Stderr, "Unverified event: %s\n", k)
					failed = true
				}
			}
			if failed {
				fmt.Println("SMOKE TESTS FAILED")
				os.Exit(1)
			}
			fmt.Println("SMOKE TESTS PASSED")
			os.Exit(0)
		}
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
