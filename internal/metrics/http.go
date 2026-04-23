package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	dependencyUp    *prometheus.GaugeVec
}

func NewRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return registry
}

func NewHTTPMetrics(registry prometheus.Registerer) *HTTPMetrics {
	httpMetrics := &HTTPMetrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of processed HTTP requests.",
			},
			[]string{"scope", "method", "route", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"scope", "method", "route", "status"},
		),
		dependencyUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dependency_up",
				Help: "Dependency health state. 1 means healthy, 0 means unhealthy.",
			},
			[]string{"dependency"},
		),
	}

	registry.MustRegister(
		httpMetrics.requestsTotal,
		httpMetrics.requestDuration,
		httpMetrics.dependencyUp,
	)

	return httpMetrics
}

func (m *HTTPMetrics) ObserveRequest(scope, method, route string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)

	m.requestsTotal.WithLabelValues(scope, method, route, statusLabel).Inc()
	m.requestDuration.WithLabelValues(scope, method, route, statusLabel).Observe(duration.Seconds())
}

func (m *HTTPMetrics) SetDependencyUp(name string, up bool) {
	value := 0.0
	if up {
		value = 1
	}

	m.dependencyUp.WithLabelValues(name).Set(value)
}

func Handler(gatherer prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
}
