package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"mototaxi/location-service/internal/kafka"
	"mototaxi/location-service/internal/metrics"
	"mototaxi/location-service/internal/redisstore"
)

// locationPayload represents the POST /location request body.
// Pointer fields for lat, lng, bearing, speed_kmh distinguish "absent" from "zero" (Pitfall 2).
type locationPayload struct {
	DriverID  string   `json:"driver_id"`
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
	Bearing   *float64 `json:"bearing"`
	SpeedKmh  *float64 `json:"speed_kmh"`
	EmittedAt string   `json:"emitted_at"`
}

// validate checks all required fields and range constraints.
func (p *locationPayload) validate() error {
	if p.DriverID == "" {
		return errors.New("missing driver_id")
	}
	if p.Lat == nil {
		return errors.New("missing lat")
	}
	if *p.Lat < -90 || *p.Lat > 90 {
		return errors.New("lat out of range: must be between -90 and 90")
	}
	if p.Lng == nil {
		return errors.New("missing lng")
	}
	if *p.Lng < -180 || *p.Lng > 180 {
		return errors.New("lng out of range: must be between -180 and 180")
	}
	if p.Bearing == nil {
		return errors.New("missing bearing")
	}
	if *p.Bearing < 0 || *p.Bearing > 360 {
		return errors.New("bearing out of range: must be between 0 and 360")
	}
	if p.SpeedKmh == nil {
		return errors.New("missing speed_kmh")
	}
	if *p.SpeedKmh < 0 {
		return errors.New("speed_kmh must be >= 0")
	}
	if p.EmittedAt == "" {
		return errors.New("missing emitted_at")
	}
	if _, err := time.Parse(time.RFC3339, p.EmittedAt); err != nil {
		return errors.New("emitted_at must be a valid RFC3339 timestamp")
	}
	return nil
}

// Handler holds dependencies for HTTP route handlers.
type Handler struct {
	kafka    kafka.Producer
	redis    redisstore.Store
	metrics  *metrics.Metrics
}

// NewHandler creates a new Handler with the given Kafka producer, Redis store, and Metrics.
func NewHandler(kafka kafka.Producer, redis redisstore.Store, m *metrics.Metrics) *Handler {
	return &Handler{kafka: kafka, redis: redis, metrics: m}
}

// HandlePostLocation handles POST /location.
// Validates the driver GPS payload, publishes synchronously to Kafka, then writes to Redis.
// Returns 400 on validation failure, 503 on Kafka or Redis failure, 200 on success.
func (h *Handler) HandlePostLocation(w http.ResponseWriter, r *http.Request) {
	var payload locationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := payload.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.metrics.UpdatesReceived.Inc()

	encoded, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode payload")
		return
	}

	kafkaStart := time.Now()
	if err := h.kafka.Publish(r.Context(), payload.DriverID, encoded); err != nil {
		writeError(w, http.StatusServiceUnavailable, "kafka unavailable")
		return
	}
	h.metrics.KafkaDuration.Observe(float64(time.Since(kafkaStart).Milliseconds()))

	redisStart := time.Now()
	if err := h.redis.WriteLocation(r.Context(), payload.DriverID, *payload.Lat, *payload.Lng, encoded); err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis unavailable")
		return
	}
	h.metrics.RedisDuration.Observe(float64(time.Since(redisStart).Milliseconds()))

	w.WriteHeader(http.StatusOK)
}

// HandleGetLocation handles GET /location/{driverID}.
// Returns the stored JSON payload for the driver, 404 if not found, 503 if Redis is unavailable.
func (h *Handler) HandleGetLocation(w http.ResponseWriter, r *http.Request) {
	driverID := chi.URLParam(r, "driverID")

	val, err := h.redis.ReadLocation(r.Context(), driverID)
	if err == redisstore.ErrNotFound {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis unavailable")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(val)
}

// HandleHealth handles GET /health.
// Returns 200 OK to indicate the service is alive.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
