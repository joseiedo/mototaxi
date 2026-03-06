package geo

import (
	"math"
	"math/rand"
)

const (
	earthRadiusKm = 6371.0
	arrivalKm     = 0.01 // ~10 meters
)

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
	lat1 := src.Lat * math.Pi / 180
	lat2 := dst.Lat * math.Pi / 180
	dLng := (dst.Lng - src.Lng) * math.Pi / 180
	x := math.Sin(dLng) * math.Cos(lat2)
	y := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLng)
	θ := math.Atan2(x, y) * 180 / math.Pi
	return math.Mod(θ+360, 360)
}

// DistanceKm returns the great-circle distance in km between two points.
func DistanceKm(a, b Point) float64 {
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	s := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(s))
}

// StepToward returns the new position after moving from src toward dst at speedKmh over intervalSec seconds.
// Never overshoots dst; returns dst directly if the step distance would exceed the remaining distance.
func StepToward(src, dst Point, speedKmh, intervalSec float64) Point {
	distKm := DistanceKm(src, dst)
	stepKm := speedKmh * intervalSec / 3600.0
	if stepKm >= distKm {
		return dst // arrived; caller should pick new destination
	}
	fraction := stepKm / distKm
	newLat := src.Lat + fraction*(dst.Lat-src.Lat)
	newLng := src.Lng + fraction*(dst.Lng-src.Lng)
	return Point{newLat, newLng}
}

// Arrived returns true if src is within arrival threshold (~10 m) of dst.
func Arrived(src, dst Point) bool {
	return DistanceKm(src, dst) < arrivalKm
}

// RandomPoint returns a random point within the given bounding box.
func RandomPoint(bbox Bbox) Point {
	return Point{
		Lat: bbox.MinLat + rand.Float64()*(bbox.MaxLat-bbox.MinLat),
		Lng: bbox.MinLng + rand.Float64()*(bbox.MaxLng-bbox.MinLng),
	}
}

// RandomSpeed returns a random speed in km/h in [minKmh, maxKmh).
func RandomSpeed(minKmh, maxKmh float64) float64 {
	return minKmh + rand.Float64()*(maxKmh-minKmh)
}

// BboxClamp clamps p to be within the given bounding box.
func BboxClamp(p Point, bbox Bbox) Point {
	return Point{
		Lat: math.Max(bbox.MinLat, math.Min(bbox.MaxLat, p.Lat)),
		Lng: math.Max(bbox.MinLng, math.Min(bbox.MaxLng, p.Lng)),
	}
}
