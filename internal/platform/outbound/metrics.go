package outbound

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Исходы вызова для метрик.
const (
	outcomeOK          = "ok"
	outcomeRejected    = "rejected"
	outcomeThrottled   = "throttled"
	outcomeError       = "error"
	outcomeCircuitOpen = "circuit_open"
)

// Metrics — внешние вызовы по системе и операции (07 §5.5). Нулевой указатель
// допустим: метрики не пишутся.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	circuit  *prometheus.GaugeVec
}

// Registerer — реестр метрик (platform/metrics.Registry или prometheus.Registry).
type Registerer interface{ MustRegister(...prometheus.Collector) }

// NewMetrics создаёт и регистрирует метрики внешних вызовов.
func NewMetrics(reg Registerer) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "maxim_outbound_requests_total", Help: "Outbound call attempts by system, operation and outcome.",
		}, []string{"system", "operation", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "maxim_outbound_request_duration_seconds", Help: "Outbound call attempt duration.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 120},
		}, []string{"system", "operation"}),
		circuit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "maxim_outbound_circuit_open", Help: "1 if calls to the system are suspended by the circuit breaker.",
		}, []string{"system"}),
	}
	reg.MustRegister(m.requests, m.duration, m.circuit)
	return m
}

func (m *Metrics) observe(system, operation, outcome string, d time.Duration) {
	if m == nil {
		return
	}
	m.requests.WithLabelValues(system, operation, outcome).Inc()
	if outcome != outcomeCircuitOpen {
		m.duration.WithLabelValues(system, operation).Observe(d.Seconds())
	}
}

func (m *Metrics) setCircuit(system string, open bool) {
	if m == nil {
		return
	}
	v := 0.0
	if open {
		v = 1
	}
	m.circuit.WithLabelValues(system).Set(v)
}

func outcomeOf(r attemptResult) string {
	switch {
	case r.err == nil:
		return outcomeOK
	case r.resp.Status == 429:
		return outcomeThrottled
	case !r.retryable:
		return outcomeRejected
	default:
		return outcomeError
	}
}
