package emitter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestEmitPayload verifies that emitLocation sends the correct JSON body to the server.
func TestEmitPayload(t *testing.T) {
	var (
		gotBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	now := time.Now().UTC()
	payload := locationPayload{
		DriverID:  "driver-1",
		Lat:       -23.55,
		Lng:       -46.65,
		Bearing:   45.0,
		SpeedKmh:  40.0,
		EmittedAt: now.Format(time.RFC3339),
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if err := emitLocation(client, srv.URL+"/location", payload); err != nil {
		t.Fatalf("emitLocation: %v", err)
	}

	var received locationPayload
	if err := json.Unmarshal(gotBody, &received); err != nil {
		t.Fatalf("unmarshal body: %v, body=%q", err, gotBody)
	}

	if received.DriverID != payload.DriverID {
		t.Errorf("driver_id = %q, want %q", received.DriverID, payload.DriverID)
	}
	if received.Lat != payload.Lat {
		t.Errorf("lat = %v, want %v", received.Lat, payload.Lat)
	}
	if received.Lng != payload.Lng {
		t.Errorf("lng = %v, want %v", received.Lng, payload.Lng)
	}
	if received.Bearing != payload.Bearing {
		t.Errorf("bearing = %v, want %v", received.Bearing, payload.Bearing)
	}
	if received.SpeedKmh != payload.SpeedKmh {
		t.Errorf("speed_kmh = %v, want %v", received.SpeedKmh, payload.SpeedKmh)
	}
	// Validate emitted_at is RFC3339.
	if _, err := time.Parse(time.RFC3339, received.EmittedAt); err != nil {
		t.Errorf("emitted_at %q is not RFC3339: %v", received.EmittedAt, err)
	}
}

// TestEmitNon200 verifies that a 503 response does not cause a panic or error return.
func TestEmitNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	payload := locationPayload{
		DriverID:  "driver-1",
		Lat:       -23.55,
		Lng:       -46.65,
		Bearing:   0,
		SpeedKmh:  30.0,
		EmittedAt: time.Now().UTC().Format(time.RFC3339),
	}

	client := &http.Client{Timeout: 5 * time.Second}
	// Must not panic, must not return error (503 is logged only).
	if err := emitLocation(client, srv.URL+"/location", payload); err != nil {
		t.Errorf("emitLocation returned error on 503, want nil: %v", err)
	}
}
