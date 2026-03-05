package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the custom Prometheus registry and all application metrics.
// Use NewMetrics() to construct — it registers Go runtime and process collectors
// on a fresh isolated registry so multiple instances can coexist in the same
// process without "already registered" panics.
type Metrics struct {
	UpdatesReceived prometheus.Counter
	KafkaDuration   prometheus.Histogram
	RedisDuration   prometheus.Histogram
	Registry        *prometheus.Registry // exposed for promhttp.HandlerFor in main.go
}

// NewMetrics creates a new isolated Prometheus registry, registers Go runtime
// and process collectors, and creates the three custom application metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	factory := promauto.With(reg)
	return &Metrics{
		Registry: reg,
		UpdatesReceived: factory.NewCounter(prometheus.CounterOpts{
			Name: "location_updates_received_total",
			Help: "Total number of location updates received via POST /location",
		}),
		KafkaDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "kafka_publish_duration_ms",
			Help:    "Kafka sync produce round-trip duration in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
		}),
		RedisDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "redis_write_duration_ms",
			Help:    "Redis pipeline write duration in milliseconds",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 25, 50, 100},
		}),
	}
}
