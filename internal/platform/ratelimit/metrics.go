package ratelimit

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics — ожидание токена по виду корзины и приоритету (07 §5.5). Нулевой
// указатель допустим.
type Metrics struct {
	wait *prometheus.HistogramVec
}

// Registerer — реестр метрик.
type Registerer interface{ MustRegister(...prometheus.Collector) }

// NewMetrics создаёт и регистрирует метрики ограничителя.
func NewMetrics(reg Registerer) *Metrics {
	m := &Metrics{wait: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "maxim_ratelimit_wait_seconds", Help: "Time spent waiting for a rate limiter token.",
		Buckets: []float64{0, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
	}, []string{"key_kind", "priority"})}
	reg.MustRegister(m.wait)
	return m
}

func (m *Metrics) observe(key string, p Priority, d time.Duration) {
	if m == nil {
		return
	}
	m.wait.WithLabelValues(keyKind(key), p.String()).Observe(d.Seconds())
}

// keyKind — вид корзины без идентификатора чата: метка не должна разрастаться.
func keyKind(key string) string {
	if strings.Contains(key, ":chat:") {
		return "chat"
	}
	if i := strings.LastIndex(key, ":"); i >= 0 {
		return key[i+1:]
	}
	return key
}
