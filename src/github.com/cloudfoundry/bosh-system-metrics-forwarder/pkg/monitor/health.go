package monitor

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// New creates a health metrics server
func NewHealth(port uint32) Starter {
	return &health{port}
}

type health struct {
	port uint32
}

// Start initializes a monitor health server
func (s *health) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", s.port))
	if err != nil {
		log.Printf("unable to start monitor endpoint: %s", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	log.Printf("starting monitor endpoint on http://%s/metrics\n", lis.Addr().String())
	err = http.Serve(lis, mux)
	log.Printf("error starting the monitor server: %s", err)
}
