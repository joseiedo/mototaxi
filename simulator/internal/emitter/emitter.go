package emitter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"mototaxi/simulator/internal/geo"
)

// saoPauloBbox is the geographic bounding box used for generating random driver positions.
var saoPauloBbox = geo.Bbox{
	MinLat: -23.65,
	MaxLat: -23.45,
	MinLng: -46.75,
	MaxLng: -46.55,
}

const (
	minSpeedKmh = 20.0
	maxSpeedKmh = 60.0
)

// locationPayload is the JSON body sent to the Location Service.
type locationPayload struct {
	DriverID  string  `json:"driver_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Bearing   float64 `json:"bearing"`
	SpeedKmh  float64 `json:"speed_kmh"`
	EmittedAt string  `json:"emitted_at"`
}

// emitLocation sends a location update to the given URL using the provided client.
// Non-2xx responses are logged but not returned as errors (fire-and-forget).
// Network errors are also logged and return nil.
func emitLocation(client *http.Client, locationURL string, payload locationPayload) error {
	body, _ := json.Marshal(payload)
	resp, err := client.Post(locationURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[%s] emit error: %v", payload.DriverID, err)
		return nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] emit got HTTP %d", payload.DriverID, resp.StatusCode)
	}
	return nil
}

// RunDriver runs the driver simulation loop for a single driver goroutine.
// It ticks at intervalMs, computes the new position, and POSTs a location payload.
// The loop exits cleanly when ctx is cancelled.
func RunDriver(ctx context.Context, id int, client *http.Client, locationURL string, intervalMs int) {
	driverID := fmt.Sprintf("driver-%d", id)
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	cur := geo.RandomPoint(saoPauloBbox)
	dst := geo.RandomPoint(saoPauloBbox)
	speed := geo.RandomSpeed(minSpeedKmh, maxSpeedKmh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bearing := geo.Bearing(cur, dst)
			cur = geo.StepToward(cur, dst, speed, float64(intervalMs)/1000.0)
			if geo.Arrived(cur, dst) {
				dst = geo.RandomPoint(saoPauloBbox)
				speed = geo.RandomSpeed(minSpeedKmh, maxSpeedKmh)
			}
			emitLocation(client, locationURL+"/location", locationPayload{ //nolint:errcheck
				DriverID:  driverID,
				Lat:       cur.Lat,
				Lng:       cur.Lng,
				Bearing:   bearing,
				SpeedKmh:  speed,
				EmittedAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}
