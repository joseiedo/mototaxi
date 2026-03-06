package seeder_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"mototaxi/simulator/internal/seeder"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// TestSeedAssignments verifies that n=2 seeds the correct bidirectional keys.
func TestSeedAssignments(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	if err := seeder.SeedAssignments(ctx, rdb, 2); err != nil {
		t.Fatalf("SeedAssignments: %v", err)
	}

	cases := []struct {
		key  string
		want string
	}{
		{"customer:customer-1:driver", "driver-1"},
		{"driver:driver-1:customer", "customer-1"},
		{"customer:customer-2:driver", "driver-2"},
		{"driver:driver-2:customer", "customer-2"},
	}

	for _, tc := range cases {
		got, err := rdb.Get(ctx, tc.key).Result()
		if err != nil {
			t.Errorf("GET %q: %v", tc.key, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GET %q = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// TestSeedIdempotent verifies that seeding n=1 twice produces no error and same values.
func TestSeedIdempotent(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	if err := seeder.SeedAssignments(ctx, rdb, 1); err != nil {
		t.Fatalf("first SeedAssignments: %v", err)
	}

	// Second call must not error.
	if err := seeder.SeedAssignments(ctx, rdb, 1); err != nil {
		t.Fatalf("second SeedAssignments (idempotent): %v", err)
	}

	got, err := rdb.Get(ctx, "customer:customer-1:driver").Result()
	if err != nil {
		t.Fatalf("GET after second seed: %v", err)
	}
	if got != "driver-1" {
		t.Errorf("customer:customer-1:driver = %q, want %q", got, "driver-1")
	}
}
