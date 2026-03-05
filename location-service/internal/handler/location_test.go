package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"mototaxi/location-service/internal/handler"
)

// mockProducer satisfies the kafka.Producer interface for testing.
type mockProducer struct {
	publishErr error
	published  []publishCall
}

type publishCall struct {
	key   string
	value []byte
}

func (m *mockProducer) Publish(ctx context.Context, key string, value []byte) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, publishCall{key: key, value: value})
	return nil
}

func (m *mockProducer) Ping(ctx context.Context) error { return nil }
func (m *mockProducer) Close()                         {}

func newTestRouter(prod *mockProducer) http.Handler {
	h := handler.NewHandler(prod)
	r := chi.NewRouter()
	r.Post("/location", h.HandlePostLocation)
	return r
}

func postLocation(t *testing.T, router http.Handler, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/location", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func validPayload() map[string]interface{} {
	return map[string]interface{}{
		"driver_id":  "driver-123",
		"lat":        -12.046374,
		"lng":        -77.042793,
		"bearing":    180.0,
		"speed_kmh":  50.0,
		"emitted_at": "2026-03-05T10:00:00Z",
	}
}

func TestPostLocationValid(t *testing.T) {
	prod := &mockProducer{}
	router := newTestRouter(prod)

	rr := postLocation(t, router, validPayload())

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	if len(prod.published) != 1 {
		t.Fatalf("expected 1 Kafka publish call, got %d", len(prod.published))
	}
	if prod.published[0].key != "driver-123" {
		t.Errorf("expected key 'driver-123', got '%s'", prod.published[0].key)
	}
}

func TestPostLocationMissingDriverID(t *testing.T) {
	payload := validPayload()
	delete(payload, "driver_id")

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in response body")
	}
}

func TestPostLocationMissingLat(t *testing.T) {
	payload := validPayload()
	delete(payload, "lat")

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationLatOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["lat"] = 200.0

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationLngOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["lng"] = -200.0

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationBearingOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["bearing"] = 400.0

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationNegativeSpeed(t *testing.T) {
	payload := validPayload()
	payload["speed_kmh"] = -1.0

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationMissingEmittedAt(t *testing.T) {
	payload := validPayload()
	payload["emitted_at"] = ""

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationInvalidEmittedAt(t *testing.T) {
	payload := validPayload()
	payload["emitted_at"] = "not-a-timestamp"

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationKafkaFail(t *testing.T) {
	prod := &mockProducer{publishErr: context.DeadlineExceeded}
	router := newTestRouter(prod)
	rr := postLocation(t, router, validPayload())

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in response body")
	}
}

func TestPostLocationZeroCoords(t *testing.T) {
	payload := validPayload()
	payload["lat"] = 0.0
	payload["lng"] = 0.0
	payload["bearing"] = 0.0
	payload["speed_kmh"] = 0.0

	prod := &mockProducer{}
	router := newTestRouter(prod)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for zero coords (equatorial, valid), got %d; body: %s", rr.Code, rr.Body.String())
	}
}
