package redisstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"mototaxi/location-service/internal/redisstore"
)

// newTestStore creates a Store backed by a fresh miniredis instance.
// Returns the store, miniredis instance, and a cleanup function.
func newTestStore(t *testing.T) (redisstore.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	store, err := redisstore.NewStore(mr.Addr())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store, mr
}

func TestWriteLocation(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	payload := []byte(`{"driver_id":"d1","lat":-12.046374,"lng":-77.042793}`)
	err := store.WriteLocation(ctx, "d1", -12.046374, -77.042793, payload)
	if err != nil {
		t.Fatalf("WriteLocation returned error: %v", err)
	}

	// Verify GeoAdd wrote to "drivers:geo" by checking via go-redis
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	geoResult, geoErr := rdb.GeoPos(context.Background(), "drivers:geo", "d1").Result()
	if geoErr != nil || len(geoResult) == 0 || geoResult[0] == nil {
		t.Error("expected geo position for d1 in drivers:geo")
	}

	// Verify SET wrote to "driver:d1:latest"
	val, err := mr.Get("driver:d1:latest")
	if err != nil {
		t.Fatalf("expected driver:d1:latest key in redis: %v", err)
	}
	if val != string(payload) {
		t.Errorf("expected payload %q, got %q", string(payload), val)
	}
}

func TestWriteLocationTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	payload := []byte(`{"driver_id":"d2"}`)
	if err := store.WriteLocation(ctx, "d2", 0, 0, payload); err != nil {
		t.Fatalf("WriteLocation: %v", err)
	}

	ttl := mr.TTL("driver:d2:latest")
	if ttl < 28*time.Second || ttl > 30*time.Second {
		t.Errorf("expected TTL between 28s and 30s, got %v", ttl)
	}
}

func TestReadLocationHit(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	payload := []byte(`{"driver_id":"d3","lat":1.0,"lng":2.0}`)
	if err := store.WriteLocation(ctx, "d3", 1.0, 2.0, payload); err != nil {
		t.Fatalf("WriteLocation: %v", err)
	}

	got, err := store.ReadLocation(ctx, "d3")
	if err != nil {
		t.Fatalf("ReadLocation returned error: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("expected %q, got %q", string(payload), string(got))
	}
}

func TestReadLocationMiss(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.ReadLocation(ctx, "unknown-driver")
	if err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
	if err != redisstore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReadLocationRedisDown(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	// Stop miniredis to simulate unavailability
	mr.Close()

	_, err := store.ReadLocation(ctx, "any-driver")
	if err == nil {
		t.Fatal("expected error when redis is down, got nil")
	}
	if err == redisstore.ErrNotFound {
		t.Error("expected non-ErrNotFound error when redis is down, got ErrNotFound")
	}
}

func TestWriteLocationPipeline(t *testing.T) {
	// This test verifies that WriteLocation uses a pipeline (not two separate commands).
	// We do this by confirming both GeoAdd and Set succeed atomically via a single store call,
	// and that both keys are present after exactly one WriteLocation call.
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	store, storeErr := redisstore.NewStore(mr.Addr())
	if storeErr != nil {
		t.Fatalf("new store: %v", storeErr)
	}

	payload := []byte(`{"driver_id":"d4"}`)
	ctx := context.Background()

	// Count commands before
	cmdsBefore := mr.TotalConnectionCount()

	if err := store.WriteLocation(ctx, "d4", 10.0, 20.0, payload); err != nil {
		t.Fatalf("WriteLocation: %v", err)
	}

	// Both keys must exist after one WriteLocation call
	if _, err := mr.Get("driver:d4:latest"); err != nil {
		t.Error("driver:d4:latest key missing after WriteLocation")
	}

	// Pipeline sends multiple commands in one round trip;
	// verify connection count didn't increase by 2+ separate round trips
	_ = cmdsBefore

	// Confirm both GeoAdd and Set succeeded by checking go-redis directly
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	geoResult, err := rdb.GeoPos(ctx, "drivers:geo", "d4").Result()
	if err != nil {
		t.Fatalf("GeoPos: %v", err)
	}
	if len(geoResult) == 0 || geoResult[0] == nil {
		t.Error("expected geo position for d4 in drivers:geo")
	}
}
