package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Handle --health-check flag for Docker health checks
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		if healthCheck() {
			os.Exit(0) // Success
		} else {
			os.Exit(1) // Failure
		}
	}

	// Validate configuration before starting
	if err := validateEnvironmentConfig(); err != nil {
		log.Fatalf("configuration validation failed: %v", err)
	}

	// Load configuration
	config := LoadConfig()

	// Set global variables for device monitoring
	offlineThreshold = config.OfflineThreshold
	checkFrequency = config.CheckFrequency

	// Create and connect MQTT client
	client := CreateMQTTClient(config)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect failed: %v", token.Error())
	} else {
		prettyLog(LogSuccess, "mqtt connected successfully")
	}

	// Apply rate limiting middleware
	http.HandleFunc("/", rateLimitMiddleware(geigerCounterHandler(client, config.Topic)))

	// Create server with security timeouts
	server := &http.Server{
		Addr:         ":80",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      http.DefaultServeMux,
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start background monitoring for offline devices
	go func() {
		ticker := time.NewTicker(checkFrequency)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				checkOfflineDevices(client, config.Topic)
			case <-sigChan:
				return // Stop monitoring when shutdown signal received
			}
		}
	}()

	go func() {
		prettyLog(LogInfo, "gqgmc-mqtt-bridge %s (branch: %s, commit: %s, built: %s)", Version, GitBranch, GitCommit, BuildTime)
		prettyLog(LogInfo, "starting HTTP server on %s, mqtt broker %s, topic %s", server.Addr, config.Broker, config.Topic)
		prettyLog(LogInfo, "security: rate limit 10 req/sec, max 5 params, allowed keys: GID, CPM, ACPM, uSV, AID")
		prettyLog(LogInfo, "parameter types: GID=text/numeric(max 50 chars), CPM=integer, ACPM=float, uSV=float, AID=alphanumeric(max 50 chars)")
		prettyLog(LogInfo, "monitoring: devices marked offline after %v of inactivity, checked every %v", offlineThreshold, checkFrequency)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	prettyLog(LogWarning, "shutdown signal received, shutting down gracefully...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		prettyLog(LogError, "server shutdown error: %v", err)
	}

	client.Disconnect(250)
	prettyLog(LogInfo, "server stopped")
}
