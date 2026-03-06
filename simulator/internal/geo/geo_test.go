package geo_test

import (
	"math"
	"testing"

	"mototaxi/simulator/internal/geo"
)

const (
	// São Paulo bounding box.
	bboxMinLat = -23.65
	bboxMaxLat = -23.45
	bboxMinLng = -46.75
	bboxMaxLng = -46.55
)

var saoPauloBbox = geo.Bbox{
	MinLat: bboxMinLat,
	MaxLat: bboxMaxLat,
	MinLng: bboxMinLng,
	MaxLng: bboxMaxLng,
}

// TestBearing verifies bearing calculation for cardinal directions.
func TestBearing(t *testing.T) {
	origin := geo.Point{Lat: -23.55, Lng: -46.65}

	// North: same lng, higher lat (less negative in Southern Hemisphere)
	northDst := geo.Point{Lat: -23.45, Lng: -46.65}
	northBearing := geo.Bearing(origin, northDst)
	if math.Abs(northBearing) > 1 && math.Abs(northBearing-360) > 1 {
		t.Errorf("North bearing = %.2f, want ~0 (±1°)", northBearing)
	}

	// East: same lat, higher lng (less negative)
	eastDst := geo.Point{Lat: -23.55, Lng: -46.55}
	eastBearing := geo.Bearing(origin, eastDst)
	if math.Abs(eastBearing-90) > 1 {
		t.Errorf("East bearing = %.2f, want ~90 (±1°)", eastBearing)
	}
}

// TestStepToward verifies that stepping at 60 km/h for 1 sec moves ~0.01667 km.
func TestStepToward(t *testing.T) {
	src := geo.Point{Lat: -23.55, Lng: -46.65}
	dst := geo.Point{Lat: -23.45, Lng: -46.65} // ~11 km north

	const speedKmh = 60.0
	const intervalSec = 1.0

	result := geo.StepToward(src, dst, speedKmh, intervalSec)

	// Should move ~0.01667 km (60 km/h / 3600 s).
	dist := geo.DistanceKm(src, result)
	expectedDist := speedKmh / 3600.0 * intervalSec
	if math.Abs(dist-expectedDist) > 0.001 {
		t.Errorf("StepToward moved %.5f km, want ~%.5f km", dist, expectedDist)
	}

	// Result must be between src and dst (not past dst).
	totalDist := geo.DistanceKm(src, dst)
	distToDst := geo.DistanceKm(result, dst)
	if distToDst > totalDist+0.001 {
		t.Errorf("StepToward overshot: dist to dst = %.5f km, total = %.5f km", distToDst, totalDist)
	}
}

// TestBboxClamp verifies that StepToward result for a destination outside bbox is clamped.
func TestBboxClamp(t *testing.T) {
	// Start inside bbox, destination outside bbox.
	src := geo.Point{Lat: -23.46, Lng: -46.56} // near NE corner, inside
	dst := geo.Point{Lat: -23.40, Lng: -46.50} // outside bbox

	// Take a large step toward dst.
	result := geo.StepToward(src, dst, 1000.0, 10.0) // very fast, large step
	clamped := geo.BboxClamp(result, saoPauloBbox)

	if clamped.Lat < bboxMinLat || clamped.Lat > bboxMaxLat {
		t.Errorf("clamped.Lat = %.5f, want in [%.2f, %.2f]", clamped.Lat, bboxMinLat, bboxMaxLat)
	}
	if clamped.Lng < bboxMinLng || clamped.Lng > bboxMaxLng {
		t.Errorf("clamped.Lng = %.5f, want in [%.2f, %.2f]", clamped.Lng, bboxMinLng, bboxMaxLng)
	}
}

// TestArrived verifies arrival detection.
func TestArrived(t *testing.T) {
	p := geo.Point{Lat: -23.55, Lng: -46.65}

	// Same point must be considered arrived.
	if !geo.Arrived(p, p) {
		t.Error("Arrived(p, p) = false, want true")
	}

	// Points ~0.5 km apart must not be considered arrived.
	far := geo.Point{Lat: -23.555, Lng: -46.65} // ~0.5 km south
	if geo.Arrived(p, far) {
		t.Error("Arrived(p, far) = true, want false (0.5 km distance)")
	}
}

// TestRandomPoint verifies that RandomPoint always returns a point within the bbox (property test).
func TestRandomPoint(t *testing.T) {
	for i := 0; i < 100; i++ {
		p := geo.RandomPoint(saoPauloBbox)
		if p.Lat < bboxMinLat || p.Lat > bboxMaxLat {
			t.Errorf("iteration %d: RandomPoint().Lat = %.6f, want in [%.2f, %.2f]", i, p.Lat, bboxMinLat, bboxMaxLat)
		}
		if p.Lng < bboxMinLng || p.Lng > bboxMaxLng {
			t.Errorf("iteration %d: RandomPoint().Lng = %.6f, want in [%.2f, %.2f]", i, p.Lng, bboxMinLng, bboxMaxLng)
		}
	}
}

// TestRandomSpeed verifies that RandomSpeed always returns a value in [20.0, 60.0) (property test).
func TestRandomSpeed(t *testing.T) {
	for i := 0; i < 100; i++ {
		speed := geo.RandomSpeed(20.0, 60.0)
		if speed < 20.0 || speed >= 60.0 {
			t.Errorf("iteration %d: RandomSpeed() = %.4f, want in [20.0, 60.0)", i, speed)
		}
	}
}
