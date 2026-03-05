package redisstore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned by ReadLocation when the driver key does not exist in Redis.
var ErrNotFound = errors.New("driver not found")

// Store is the Redis access interface. Implemented by redisStore; mockable in tests.
type Store interface {
	WriteLocation(ctx context.Context, driverID string, lat, lng float64, payload []byte) error
	ReadLocation(ctx context.Context, driverID string) ([]byte, error)
	Ping(ctx context.Context) error
}

type redisStore struct {
	client *redis.Client
}

// NewStore creates a new Store backed by go-redis connected to addr.
func NewStore(addr string) (Store, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	return &redisStore{client: rdb}, nil
}

// WriteLocation writes the driver location to Redis using a single pipeline round-trip.
// It performs both GeoAdd (drivers:geo) and Set (driver:{id}:latest with 30s TTL).
// On any error, no partial success is returned — the caller gets the error.
func (s *redisStore) WriteLocation(ctx context.Context, driverID string, lat, lng float64, payload []byte) error {
	pipe := s.client.Pipeline()
	pipe.GeoAdd(ctx, "drivers:geo", &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	})
	pipe.Set(ctx, "driver:"+driverID+":latest", payload, 30*time.Second)
	_, err := pipe.Exec(ctx)
	return err
}

// ReadLocation retrieves the stored location payload for the given driverID.
// Returns ErrNotFound when the key does not exist in Redis.
func (s *redisStore) ReadLocation(ctx context.Context, driverID string) ([]byte, error) {
	val, err := s.client.Get(ctx, "driver:"+driverID+":latest").Result()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return []byte(val), nil
}

// Ping checks Redis connectivity.
func (s *redisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
