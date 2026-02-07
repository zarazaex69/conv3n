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
			metricName := sanitizeMetricName(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s counter\n", metricName))
			output.WriteString(fmt.Sprintf("%s %d\n", metricName, counter.Value()))
			return true
		})

		pe.metrics.gauges.Range(func(key, value any) bool {
			gauge := value.(*Gauge)
			metricName := sanitizeMetricName(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s gauge\n", metricName))
			output.WriteString(fmt.Sprintf("%s %d\n", metricName, gauge.Value()))
			return true
		})

		pe.metrics.histograms.Range(func(key, value any) bool {
			histogram := value.(*Histogram)
			metricName := sanitizeMetricName(key.(string))

			output.WriteString(fmt.Sprintf("# TYPE %s histogram\n", metricName))
			output.WriteString(fmt.Sprintf("%s_count %d\n", metricName, histogram.Count()))
			output.WriteString(fmt.Sprintf("%s_sum %f\n", metricName, histogram.Sum()))

			buckets := histogram.Buckets()
			for bucket, count := range buckets {
				if bucket == 9999999 {
					output.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", metricName, count))
				} else {
					output.WriteString(fmt.Sprintf("%s_bucket{le=\"%f\"} %d\n", metricName, bucket, count))
				}
			}

			return true
		})

		w.Write([]byte(output.String()))
	}
}

func sanitizeMetricName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "=", "_")
	return name
}
