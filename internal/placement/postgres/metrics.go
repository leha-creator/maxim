package postgres

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/leha-creator/maxim/internal/placement/domain"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// Collector отдаёт число размещений по состояниям (07 §5.5): maxim_placements{status}.
// Значения читаются из базы при сборе метрик.
type Collector struct {
	s     *Store
	count *prometheus.Desc
}

// NewCollector создаёт сборщик метрик размещений.
func NewCollector(s *Store) *Collector {
	return &Collector{s: s, count: prometheus.NewDesc("maxim_placements", "Placements by status.", []string{"status"}, nil)}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) { ch <- c.count }

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	counts, err := c.s.Placements().CountByStatus(ctx)
	if err != nil {
		c.s.log.WarnContext(ctx, "placement metrics query failed", logx.KeyOp, "Placement.Metrics", logx.KeyError, err.Error())
		return
	}
	for status, n := range counts {
		ch <- prometheus.MustNewConstMetric(c.count, prometheus.GaugeValue, float64(n), string(status))
	}
}

// ControlMetrics — метрики публикации и контроля (07 §5.5, R-15): задержка публикации и снятия
// относительно плана, число нарушений по виду и источнику. Реализует app.Observer.
type ControlMetrics struct {
	publishDelay prometheus.Histogram
	removeDelay  prometheus.Histogram
	violations   *prometheus.CounterVec
}

// delayBuckets — от секунды до часа: требование точности — 10 с (07 §2).
var delayBuckets = []float64{1, 2, 5, 10, 30, 60, 300, 900, 1800, 3600}

// NewControlMetrics создаёт и регистрирует метрики в reg.
func NewControlMetrics(reg interface{ MustRegister(...prometheus.Collector) }) *ControlMetrics {
	m := &ControlMetrics{
		publishDelay: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "maxim_placement_publish_delay_seconds",
			Help: "Publication delay relative to publish_at.", Buckets: delayBuckets}),
		removeDelay: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "maxim_placement_remove_delay_seconds",
			Help: "Removal delay relative to remove_at.", Buckets: delayBuckets}),
		violations: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "maxim_placement_violations_total",
			Help: "Placement violations found by control."}, []string{"kind", "source"}),
	}
	reg.MustRegister(m.publishDelay, m.removeDelay, m.violations)
	return m
}

func (m *ControlMetrics) PublishDelay(d time.Duration) { m.publishDelay.Observe(max(d.Seconds(), 0)) }
func (m *ControlMetrics) RemoveDelay(d time.Duration)  { m.removeDelay.Observe(max(d.Seconds(), 0)) }

func (m *ControlMetrics) Violation(kind domain.ViolationKind, source domain.ViolationSource) {
	m.violations.WithLabelValues(string(kind), string(source)).Inc()
}
