package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"mototaxi/location-service/internal/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	h := promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Body.String()
}

func TestNewMetrics(t *testing.T) {
	m := metrics.NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if m.UpdatesReceived == nil {
		t.Error("UpdatesReceived is nil")
	}
	if m.KafkaDuration == nil {
		t.Error("KafkaDuration is nil")
	}
	if m.RedisDuration == nil {
		t.Error("RedisDuration is nil")
	}
	if m.Registry == nil {
		t.Error("Registry is nil")
	}
}

func TestCounterIncrement(t *testing.T) {
	m := metrics.NewMetrics()
	m.UpdatesReceived.Inc()
	m.UpdatesReceived.Inc()

	body := scrape(t, m)
	if !strings.Contains(body, "location_updates_received_total 2") {
		t.Errorf("expected 'location_updates_received_total 2' in scrape output, got:\n%s", body)
	}
}

func TestKafkaHistogramObserve(t *testing.T) {
	m := metrics.NewMetrics()
	m.KafkaDuration.Observe(42.0)

	body := scrape(t, m)
	if !strings.Contains(body, "kafka_publish_duration_ms") {
		t.Errorf("expected 'kafka_publish_duration_ms' in scrape output, got:\n%s", body)
	}
}

func TestRedisHistogramObserve(t *testing.T) {
	m := metrics.NewMetrics()
	m.RedisDuration.Observe(5.0)

	body := scrape(t, m)
	if !strings.Contains(body, "redis_write_duration_ms") {
		t.Errorf("expected 'redis_write_duration_ms' in scrape output, got:\n%s", body)
	}
}

func TestGoRuntimeMetrics(t *testing.T) {
	m := metrics.NewMetrics()

	body := scrape(t, m)
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("expected 'go_goroutines' in scrape output (GoCollector not registered), got:\n%s", body)
	}
}

func TestRegistryIsolated(t *testing.T) {
	// Calling NewMetrics twice must not panic with "already registered".
	// This proves a custom registry is used instead of the global DefaultRegisterer.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewMetrics() panicked on second call: %v", r)
		}
	}()
	_ = metrics.NewMetrics()
	_ = metrics.NewMetrics()
}
