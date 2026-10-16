package postgres

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

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
