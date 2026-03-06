package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"analytics-ir/event-collector/internal/api"
	"analytics-ir/event-collector/internal/clickhouse"

	"github.com/gin-gonic/gin"
)

func main() {
	chAddr := getenv("CLICKHOUSE_ADDR", "clickhouse.analytics.svc.cluster.local:9000")
	chUser := getenv("CLICKHOUSE_USER", "default")
	chPass := getenv("CLICKHOUSE_PASSWORD", "")
	chDB := getenv("CLICKHOUSE_DB", "analytics")
	flushSize := getenvInt("FLUSH_SIZE", 500)
	flushInterval := getenvDuration("FLUSH_INTERVAL", 2*time.Second)
	sessionWindow := getenvDuration("SESSION_WINDOW", 30*time.Minute)
	insertTimeout := getenvDuration("INSERT_TIMEOUT", 2*time.Second)

	client, err := clickhouse.NewClient(chAddr, chUser, chPass, chDB)
	if err != nil {
		log.Fatalf("clickhouse connect failed: %v", err)
	}

	server := api.NewServer(client, flushSize, flushInterval, sessionWindow, insertTimeout)
	r := gin.Default()
	server.RegisterRoutes(r)

	addr := getenv("HTTP_ADDR", ":8080")
	log.Printf("event-collector listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		parsed, err := time.ParseDuration(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
