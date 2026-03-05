package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mototaxi/location-service/internal/handler"
	"mototaxi/location-service/internal/kafka"
	"mototaxi/location-service/internal/metrics"
	"mototaxi/location-service/internal/redisstore"
)

func main() {
	redisAddr := envOr("REDIS_ADDR", "redis:6379")
	kafkaAddr := envOr("KAFKA_ADDR", "redpanda:9092")
	kafkaTopic := envOr("KAFKA_TOPIC", "driver.location")

	// Build dependencies
	m := metrics.NewMetrics()
	store, err := redisstore.NewStore(redisAddr)
	if err != nil {
		log.Fatalf("redis init: %v", err)
	}

	producer, err := kafka.NewProducer(kafkaAddr, kafkaTopic)
	if err != nil {
		log.Fatalf("kafka init: %v", err)
	}
	defer producer.Close()

	// Startup pings — fail fast before accepting traffic
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		log.Fatalf("redis ping: %v", err)
	}
	if err := producer.Ping(ctx); err != nil {
		log.Fatalf("kafka ping: %v", err)
	}

	// Wire handler
	h := handler.NewHandler(producer, store, m)

	// Register routes
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Post("/location", h.HandlePostLocation)
	r.Get("/location/{driverID}", h.HandleGetLocation)
	r.Get("/health", h.HandleHealth)
	r.Handle("/metrics", promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}))

	log.Println("location-service listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
