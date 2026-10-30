package internal

import (
	"fmt"
	"httpServer/internal/http1"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func TrackMetrics(route string, next http.HandlerFunc) http1.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestsTotalName := fmt.Sprintf(`http_requests_total{route="%s"}`, route)
		metrics.GetOrCreateCounter(requestsTotalName).Inc()
		inFlight := metrics.GetOrCreateGauge(`http_requests_in_flight`, nil)
		inFlight.Inc()
		defer inFlight.Dec()

		startTime := time.Now()

		next.ServeHTTP(w, req)

		durationName := fmt.Sprintf(`http_request_duration_seconds{method="%s", route="%s"}`, req.Method, route)
		metrics.GetOrCreateSummary(durationName).UpdateDuration(startTime)
	}
}
