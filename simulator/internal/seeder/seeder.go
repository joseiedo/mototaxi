package seeder

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// SeedAssignments writes customer→driver and driver→customer assignment keys
// into Redis using a single MSet call (one round-trip for all 2*n keys).
// Keys have no TTL — they persist for the stack lifetime.
// Calling SeedAssignments again is idempotent: MSet simply overwrites existing keys.
func SeedAssignments(ctx context.Context, rdb *redis.Client, n int) error {
	if n <= 0 {
		return nil
	}
	// Preallocate: 2 keys × 2 (key + value) per driver = 4 elements per driver
	pairs := make([]interface{}, 0, n*4)
	for i := 1; i <= n; i++ {
		driverID := fmt.Sprintf("driver-%d", i)
		customerID := fmt.Sprintf("customer-%d", i)
		pairs = append(pairs,
			fmt.Sprintf("customer:%s:driver", customerID), driverID,
			fmt.Sprintf("driver:%s:customer", driverID), customerID,
		)
	}
	return rdb.MSet(ctx, pairs...).Err()
}
