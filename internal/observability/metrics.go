package observability

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

type Metrics struct {
	counters   sync.Map
	gauges     sync.Map
	histograms sync.Map
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

type Counter struct {
	value  atomic.Uint64
	labels map[string]string
}

func (m *Metrics) Counter(name string, labels map[string]string) *Counter {
	key := metricKey(name, labels)

	if c, ok := m.counters.Load(key); ok {
		return c.(*Counter)
	}

	counter := &Counter{labels: labels}
	actual, _ := m.counters.LoadOrStore(key, counter)
	return actual.(*Counter)
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Add(delta uint64) {
	c.value.Add(delta)
}

func (c *Counter) Value() uint64 {
	return c.value.Load()
}

type Gauge struct {
	value  atomic.Int64
	labels map[string]string
}

func (m *Metrics) Gauge(name string, labels map[string]string) *Gauge {
	key := metricKey(name, labels)

	if g, ok := m.gauges.Load(key); ok {
		return g.(*Gauge)
	}

	gauge := &Gauge{labels: labels}
	actual, _ := m.gauges.LoadOrStore(key, gauge)
	return actual.(*Gauge)
}

func (g *Gauge) Set(value int64) {
	g.value.Store(value)
}

func (g *Gauge) Inc() {
	g.value.Add(1)
}

func (g *Gauge) Dec() {
	g.value.Add(-1)
}

func (g *Gauge) Add(delta int64) {
	g.value.Add(delta)
}

func (g *Gauge) Value() int64 {
	return g.value.Load()
}

type Histogram struct {
	buckets []float64
	counts  []atomic.Uint64
	sum     atomic.Uint64
	count   atomic.Uint64
	labels  map[string]string
	mu      sync.RWMutex
}

func (m *Metrics) Histogram(name string, buckets []float64, labels map[string]string) *Histogram {
	key := metricKey(name, labels)

	if h, ok := m.histograms.Load(key); ok {
		return h.(*Histogram)
	}

	if buckets == nil {
		buckets = []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10, 30, 60}
	}

	histogram := &Histogram{
		buckets: buckets,
		counts:  make([]atomic.Uint64, len(buckets)+1),
		labels:  labels,
	}

	actual, _ := m.histograms.LoadOrStore(key, histogram)
	return actual.(*Histogram)
}

func (h *Histogram) Observe(value float64) {
	h.count.Add(1)
	h.sum.Add(uint64(value * 1000000))

	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i].Add(1)
			return
		}
	}

	h.counts[len(h.buckets)].Add(1)
}

func (h *Histogram) ObserveDuration(start time.Time) {
	duration := time.Since(start).Seconds()
	h.Observe(duration)
}

func (h *Histogram) Count() uint64 {
	return h.count.Load()
}

func (h *Histogram) Sum() float64 {
	return float64(h.sum.Load()) / 1000000
}

func (h *Histogram) Buckets() map[float64]uint64 {
	result := make(map[float64]uint64)
	for i, bucket := range h.buckets {
		result[bucket] = h.counts[i].Load()
	}
	result[9999999] = h.counts[len(h.buckets)].Load()
	return result
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	var b strings.Builder
	b.WriteString(name)
	for k, v := range labels {
		b.WriteString(":")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
	}
	return b.String()
}

func (m *Metrics) Snapshot() map[string]any {
	snapshot := map[string]any{
		"counters":   make(map[string]uint64),
		"gauges":     make(map[string]int64),
		"histograms": make(map[string]map[string]any),
	}

	m.counters.Range(func(key, value any) bool {
		counter := value.(*Counter)
		snapshot["counters"].(map[string]uint64)[key.(string)] = counter.Value()
		return true
	})

	m.gauges.Range(func(key, value any) bool {
		gauge := value.(*Gauge)
		snapshot["gauges"].(map[string]int64)[key.(string)] = gauge.Value()
		return true
	})

	m.histograms.Range(func(key, value any) bool {
		histogram := value.(*Histogram)
		snapshot["histograms"].(map[string]map[string]any)[key.(string)] = map[string]any{
			"count":   histogram.Count(),
			"sum":     histogram.Sum(),
			"buckets": histogram.Buckets(),
		}
		return true
	})

	return snapshot
}

var globalMetrics = NewMetrics()

func GetMetrics() *Metrics {
	return globalMetrics
}
