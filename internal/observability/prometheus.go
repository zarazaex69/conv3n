package observability

import (
	"fmt"
	"net/http"
	"strings"
)

type PrometheusExporter struct {
	metrics *Metrics
}

func NewPrometheusExporter(metrics *Metrics) *PrometheusExporter {
	return &PrometheusExporter{
		metrics: metrics,
	}
}

func (pe *PrometheusExporter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		var output strings.Builder

		pe.metrics.counters.Range(func(key, value any) bool {
			counter := value.(*Counter)
			metricName, labels := parseMetricKey(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s counter\n", metricName))
			output.WriteString(fmt.Sprintf("%s%s %d\n", metricName, formatLabels(labels), counter.Value()))
			return true
		})

		pe.metrics.gauges.Range(func(key, value any) bool {
			gauge := value.(*Gauge)
			metricName, labels := parseMetricKey(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s gauge\n", metricName))
			output.WriteString(fmt.Sprintf("%s%s %d\n", metricName, formatLabels(labels), gauge.Value()))
			return true
		})

		pe.metrics.histograms.Range(func(key, value any) bool {
			histogram := value.(*Histogram)
			metricName, labels := parseMetricKey(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s histogram\n", metricName))

			buckets := histogram.Buckets()
			cumulative := uint64(0)
			for _, bucket := range []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10, 30, 60} {
				if count, ok := buckets[bucket]; ok {
					cumulative += count
					output.WriteString(fmt.Sprintf("%s_bucket%s{le=\"%g\"} %d\n", metricName, formatLabels(labels), bucket, cumulative))
				}
			}

			if infCount, ok := buckets[9999999]; ok {
				cumulative += infCount
			}
			output.WriteString(fmt.Sprintf("%s_bucket%s{le=\"+Inf\"} %d\n", metricName, formatLabels(labels), cumulative))

			output.WriteString(fmt.Sprintf("%s_sum%s %f\n", metricName, formatLabels(labels), histogram.Sum()))
			output.WriteString(fmt.Sprintf("%s_count%s %d\n", metricName, formatLabels(labels), histogram.Count()))

			return true
		})

		w.Write([]byte(output.String()))
	}
}

func parseMetricKey(key string) (string, map[string]string) {
	parts := strings.Split(key, ":")
	metricName := sanitizeMetricName(parts[0])

	labels := make(map[string]string)
	for i := 1; i < len(parts); i++ {
		if kv := strings.SplitN(parts[i], "=", 2); len(kv) == 2 {
			labels[sanitizeMetricName(kv[0])] = kv[1]
		}
	}

	return metricName, labels
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var pairs []string
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", k, v))
	}

	return "{" + strings.Join(pairs, ",") + "}"
}

func sanitizeMetricName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "=", "_")
	return name
}
