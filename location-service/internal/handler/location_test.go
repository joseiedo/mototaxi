package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"mototaxi/location-service/internal/handler"
	"mototaxi/location-service/internal/redisstore"
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

// mockStore satisfies the redisstore.Store interface for testing.
type mockStore struct {
	data         map[string][]byte
	writeErr     error
	readErr      error
	readNotFound bool
}

func (m *mockStore) WriteLocation(ctx context.Context, driverID string, lat, lng float64, payload []byte) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[driverID] = payload
	return nil
}

func (m *mockStore) ReadLocation(ctx context.Context, driverID string) ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	if m.readNotFound {
		return nil, redisstore.ErrNotFound
	}
	if m.data != nil {
		if val, ok := m.data[driverID]; ok {
			return val, nil
		}
	}
	return nil, redisstore.ErrNotFound
}

func (m *mockStore) Ping(ctx context.Context) error { return nil }

func newTestRouter(prod *mockProducer, store *mockStore) http.Handler {
	h := handler.NewHandler(prod, store)
	r := chi.NewRouter()
	r.Post("/location", h.HandlePostLocation)
	r.Get("/location/{driverID}", h.HandleGetLocation)
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

func getLocation(t *testing.T, router http.Handler, driverID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/location/"+driverID, nil)
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

// --- Existing POST tests (now with mock redis that succeeds by default) ---

func TestPostLocationValid(t *testing.T) {
	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)

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
	store := &mockStore{}
	router := newTestRouter(prod, store)
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
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationLatOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["lat"] = 200.0

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationLngOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["lng"] = -200.0

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationBearingOutOfRange(t *testing.T) {
	payload := validPayload()
	payload["bearing"] = 400.0

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationNegativeSpeed(t *testing.T) {
	payload := validPayload()
	payload["speed_kmh"] = -1.0

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationMissingEmittedAt(t *testing.T) {
	payload := validPayload()
	payload["emitted_at"] = ""

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationInvalidEmittedAt(t *testing.T) {
	payload := validPayload()
	payload["emitted_at"] = "not-a-timestamp"

	prod := &mockProducer{}
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPostLocationKafkaFail(t *testing.T) {
	prod := &mockProducer{publishErr: context.DeadlineExceeded}
	store := &mockStore{}
	router := newTestRouter(prod, store)
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
	store := &mockStore{}
	router := newTestRouter(prod, store)
	rr := postLocation(t, router, payload)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for zero coords (equatorial, valid), got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// --- New POST tests for Redis failure ---

func TestPostLocationRedisFailure(t *testing.T) {
	prod := &mockProducer{}
	store := &mockStore{writeErr: errors.New("redis connection refused")}
	router := newTestRouter(prod, store)

	rr := postLocation(t, router, validPayload())

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when redis write fails, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in response body")
	}
}

// --- GET /location/{driverID} tests ---

func TestGetLocationHit(t *testing.T) {
	store := &mockStore{
		data: map[string][]byte{
			"driver-42": []byte(`{"driver_id":"driver-42","lat":-12.0,"lng":-77.0}`),
		},
	}
	prod := &mockProducer{}
	router := newTestRouter(prod, store)

	rr := getLocation(t, router, "driver-42")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["driver_id"] != "driver-42" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestGetLocationMiss(t *testing.T) {
	store := &mockStore{readNotFound: true}
	prod := &mockProducer{}
	router := newTestRouter(prod, store)

	rr := getLocation(t, router, "ghost-driver")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "not found" {
		t.Errorf("expected error 'not found', got %q", body["error"])
	}
}

func TestGetLocationRedisDown(t *testing.T) {
	store := &mockStore{readErr: errors.New("connection refused")}
	prod := &mockProducer{}
	router := newTestRouter(prod, store)

	rr := getLocation(t, router, "any-driver")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
