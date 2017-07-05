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
)

func main() {
	cnsmr := consumer.New(loggregatorAddr, &tls.Config{InsecureSkipVerify: true}, nil)

	msgs, errs := cnsmr.FilteredFirehose("bosh-system-metrics-smoke-tests", "", consumer.Metrics)

	go func() {
		for err := range errs {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		}
	}()

	t := time.NewTimer(time.Minute)

	for {
		select {
		case msg := <-msgs:
			fmt.Printf("%v \n", msg)
			if msg.GetValueMetric().GetName() == "bosh.healthmonitor.system.cpu.user" {
				os.Exit(0)
			}
		case <-t.C:
			fmt.Fprintf(os.Stderr, "Could not find all expected Bosh HM Metrics")
			os.Exit(1)
		}
	}
}
