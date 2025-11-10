package metrics

import (
	"fmt"
	"net/http"

	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DefaultMetricsPort = 9090

func StartMetricsServer(port int) {
	if port == 0 {
		port = DefaultMetricsPort
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	logger.Infof("Starting Prometheus metrics server on %s", addr)

	go func() {
		logger.Infof("Prometheus metrics server listening on http://%s/metrics", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Errorf("Metrics server failed: %v", err)
		}
	}()
}
