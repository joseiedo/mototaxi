package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"mototaxi/simulator/internal/emitter"
	"mototaxi/simulator/internal/seeder"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustInt(key, val string) int {
	if val == "" {
		log.Fatalf("config: %s must not be empty", key)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("config: %s must be a valid integer, got %q: %v", key, val, err)
	}
	if n <= 0 {
		log.Fatalf("config: %s must be a positive integer, got %d", key, n)
	}
	return n
}

func pingWithRetry(rdb *redis.Client) {
	const maxAttempts = 10
	for i := 1; i <= maxAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := rdb.Ping(ctx).Err()
		cancel()
		if err == nil {
			log.Printf("redis: connected on attempt %d", i)
			return
		}
		log.Printf("redis: ping attempt %d/%d failed: %v", i, maxAttempts, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("redis: failed to connect after %d attempts", maxAttempts)
}

func main() {
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	locationURL := envOr("LOCATION_SERVICE_URL", "http://localhost:8080")
	driverCount := mustInt("DRIVER_COUNT", envOr("DRIVER_COUNT", "10"))
	emitIntervalMs := mustInt("EMIT_INTERVAL_MS", envOr("EMIT_INTERVAL_MS", "1000"))

	log.Printf("simulator starting: drivers=%d interval=%dms redis=%s location=%s",
		driverCount, emitIntervalMs, redisAddr, locationURL)

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	pingWithRetry(rdb)

	if err := seeder.SeedAssignments(context.Background(), rdb, driverCount); err != nil {
		log.Fatalf("seeder: SeedAssignments failed: %v", err)
	}
	log.Printf("seeder: seeded %d driver/customer assignment pairs", driverCount)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 5 * time.Second}

	var wg sync.WaitGroup
	for i := 1; i <= driverCount; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			emitter.RunDriver(ctx, i, client, locationURL, emitIntervalMs)
		}()
	}

	wg.Wait()
	log.Println("simulator stopped cleanly")
}
