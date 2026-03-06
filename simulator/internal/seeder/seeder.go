package seeder

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// SeedAssignments seeds n customer-driver 1:1 assignments into Redis.
// Keys: customer:customer-{N}:driver -> "driver-{N}"
//
//	driver:driver-{N}:customer -> "customer-{N}"
func SeedAssignments(ctx context.Context, rdb *redis.Client, n int) error {
	return errors.New("not implemented")
}
