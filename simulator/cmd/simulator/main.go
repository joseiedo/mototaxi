package main

import (
	"log"
	"os"
	"strconv"
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

func main() {
	// Placeholder — implementation comes in later plans.
	log.Println("simulator starting")
}
