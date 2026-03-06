package geo

import "errors"

// Point represents a geographic coordinate.
type Point struct {
	Lat float64
	Lng float64
}

// Bbox defines a geographic bounding box.
type Bbox struct {
	MinLat, MaxLat float64
	MinLng, MaxLng float64
}

// Bearing returns the initial bearing (degrees, 0-360) from src to dst.
func Bearing(src, dst Point) float64 {
	_ = errors.New("not implemented")
	return 0
}

// DistanceKm returns the great-circle distance in km between two points.
func DistanceKm(a, b Point) float64 {
	return 0
}

// StepToward moves from src toward dst at speedKmh over intervalSec seconds.
// Returns the new position, which will not overshoot dst.
func StepToward(src, dst Point, speedKmh, intervalSec float64) Point {
	return src
}

// Arrived returns true if src is within arrival threshold of dst.
func Arrived(src, dst Point) bool {
	return false
}

// RandomPoint returns a random point within the given bounding box.
func RandomPoint(bbox Bbox) Point {
	return Point{}
}

// RandomSpeed returns a random speed in km/h between minKmh and maxKmh.
func RandomSpeed(minKmh, maxKmh float64) float64 {
	return minKmh
}

// BboxClamp clamps p to be within the given bounding box.
func BboxClamp(p Point, bbox Bbox) Point {
	return p
}
