package emitter

import (
	"errors"
	"net/http"
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
// Non-2xx responses are logged and not returned as errors.
func emitLocation(client *http.Client, url string, payload locationPayload) error {
	return errors.New("not implemented")
}
