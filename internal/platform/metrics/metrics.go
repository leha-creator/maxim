// Package metrics — метрики приложения в формате Prometheus (07 §5.5).
// Реестр создаётся в bootstrap и передаётся конструкторами; глобального нет.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry — реестр метрик процесса.
type Registry struct {
	reg          *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

// New создаёт реестр со стандартными метриками процесса и Go.
func New() *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	r := &Registry{
		reg: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "maxim_http_requests_total",
			Help: "HTTP requests by route, method and status code.",
		}, []string{"route", "method", "code"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "maxim_http_request_duration_seconds",
			Help:    "HTTP request duration by route and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
	}
	reg.MustRegister(r.httpRequests, r.httpDuration)
	return r
}

// MustRegister добавляет метрики модуля.
func (r *Registry) MustRegister(cs ...prometheus.Collector) { r.reg.MustRegister(cs...) }

// ObserveHTTP учитывает завершённый запрос.
func (r *Registry) ObserveHTTP(route, method string, status int, d time.Duration) {
	r.httpRequests.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
	r.httpDuration.WithLabelValues(route, method).Observe(d.Seconds())
}

// Handler отдаёт метрики по /metrics.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{Registry: r.reg})
}
