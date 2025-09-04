package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/time/rate"
)

// Health check function
func healthCheck() bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://localhost:80/")
	if err != nil {
		// Don't log health check failures to avoid spam
		// Docker will handle the failure status
		return false
	}
	defer resp.Body.Close()

	// Accept any HTTP status code as long as server responds
	// Don't log success either to avoid log spam every 30 seconds
	return true
}

// Rate limiter: 10 requests per second, burst of 100
var limiter = rate.NewLimiter(10, 100)

// Device health monitoring - will be configured from environment variables
var (
	offlineThreshold time.Duration
	checkFrequency   time.Duration
)

type deviceStatus struct {
	lastSeen time.Time
	isOnline bool
}

var (
	deviceStates = make(map[string]*deviceStatus)
	deviceMutex  sync.RWMutex
)

// Update device status and publish status changes
func updateDeviceStatus(client mqtt.Client, deviceID, baseTopic string) {
	now := time.Now()

	deviceMutex.Lock()
	defer deviceMutex.Unlock()

	// Get or create device status
	status, exists := deviceStates[deviceID]
	if !exists {
		status = &deviceStatus{
			lastSeen: now,
			isOnline: true,
		}
		deviceStates[deviceID] = status

		// Publish initial online status
		statusTopic := baseTopic + "/" + deviceID + "/status"
		if isValidMqttTopic(statusTopic) {
			token := client.Publish(statusTopic, 0, true, "online") // retained message
			token.WaitTimeout(5 * time.Second)
			if token.Error() != nil {
				log.Printf("failed to publish online status for device %s: %v", deviceID, token.Error())
			} else {
				log.Printf("device %s marked as online", deviceID)
			}
		}
	} else {
		// Update last seen time
		status.lastSeen = now

		// If device was offline, mark it online and publish status
		if !status.isOnline {
			status.isOnline = true
			statusTopic := baseTopic + "/" + deviceID + "/status"
			if isValidMqttTopic(statusTopic) {
				token := client.Publish(statusTopic, 0, true, "online") // retained message
				token.WaitTimeout(5 * time.Second)
				if token.Error() != nil {
					log.Printf("failed to publish online status for device %s: %v", deviceID, token.Error())
				} else {
					log.Printf("device %s came back online", deviceID)
				}
			}
		}
	}
}

// Check for offline devices and publish status updates
func checkOfflineDevices(client mqtt.Client, baseTopic string) {
	deviceMutex.Lock()
	defer deviceMutex.Unlock()

	now := time.Now()
	for deviceID, status := range deviceStates {
		if status.isOnline && now.Sub(status.lastSeen) > offlineThreshold {
			status.isOnline = false
			statusTopic := baseTopic + "/" + deviceID + "/status"
			if isValidMqttTopic(statusTopic) {
				token := client.Publish(statusTopic, 0, true, "offline") // retained message
				token.WaitTimeout(5 * time.Second)
				if token.Error() != nil {
					log.Printf("failed to publish offline status for device %s: %v", deviceID, token.Error())
				} else {
					log.Printf("device %s marked as offline (last seen: %v)", deviceID, status.lastSeen.Format(time.RFC3339))
				}
			}
		}
	}
}

// Validate MQTT topic names to prevent injection
func isValidMqttTopic(topic string) bool {
	// Allow alphanumeric, underscore, hyphen, and forward slash
	return regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`).MatchString(topic)
}

// Validate GMC geiger counter parameter keys - only allow specific keys
func isValidGeigerParameter(key string) bool {
	validKeys := map[string]bool{
		"GID":  true, // text and numeric value
		"CPM":  true, // integer value
		"ACPM": true, // floating point value
		"uSV":  true, // floating point value
		"AID":  true, // alphanumeric identifier
	}
	return validKeys[key]
}

// Validate GMC geiger counter parameter values based on key type
func isValidGeigerValue(key, value string) bool {
	if len(value) == 0 {
		return false // Empty values not allowed
	}

	switch key {
	case "GID":
		// Allow alphanumeric characters (text and numeric)
		return regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(value) && len(value) <= 50
	case "CPM":
		// Must be a valid integer
		return regexp.MustCompile(`^\d+$`).MatchString(value)
	case "ACPM":
		// Must be a valid floating point number
		return regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(value)
	case "uSV":
		// Must be a valid floating point number
		return regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(value)
	case "AID":
		// Allow alphanumeric characters (similar to GID)
		return regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(value) && len(value) <= 50
	default:
		return false
	}
}

// Validate environment variables and configuration
func validateEnvironmentConfig() error {
	broker := os.Getenv("MQTT_BROKER")
	if broker != "" && !strings.HasPrefix(broker, "tcp://") && !strings.HasPrefix(broker, "ssl://") {
		return fmt.Errorf("invalid broker protocol - must start with tcp:// or ssl://")
	}

	topic := os.Getenv("MQTT_TOPIC")
	if topic != "" && !isValidMqttTopic(topic) {
		return fmt.Errorf("invalid topic format - only alphanumeric, underscore, hyphen, and slash allowed")
	}

	return nil
}

// Rate limiting middleware
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			log.Printf("SECURITY: Rate limit exceeded from %s", r.RemoteAddr)
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func geigerCounterHandler(client mqtt.Client, topic string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET and HEAD methods
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			log.Printf("SECURITY: Invalid method '%s' from %s", r.Method, r.RemoteAddr)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Limit request size to 128KB
		r.Body = http.MaxBytesReader(w, r.Body, 131072)

		// Set comprehensive security headers
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Server", "") // Hide server information

		// collect GET params
		params := r.URL.Query()

		// Early return for health checks (localhost with no params) - don't process MQTT publishing or logging
		if (strings.Contains(r.RemoteAddr, "127.0.0.1") || strings.Contains(r.RemoteAddr, "[::1]")) && len(params) == 0 {
			fmt.Fprint(w, "HEALTHY")
			return
		}

		// Log connection for monitoring (health checks already filtered out above)
		log.Printf("connection from %s to %s with %d params",
			r.RemoteAddr, r.URL.Path, len(params))

		// Validate number of parameters
		if len(params) > 5 {
			log.Printf("SECURITY: Too many params (%d) from %s", len(params), r.RemoteAddr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// First pass: validate all parameters before processing any
		for paramName, values := range params {
			// Validate parameter key - only allow GID, CPM, ACPM, uSV, AID
			if !isValidGeigerParameter(paramName) {
				log.Printf("SECURITY: Invalid parameter key '%s' from %s (only GID, CPM, ACPM, uSV, AID allowed)", paramName, r.RemoteAddr)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			for _, value := range values {
				// Validate parameter value based on key type
				if !isValidGeigerValue(paramName, value) {
					log.Printf("SECURITY: Invalid value '%s' for key '%s' from %s", value, paramName, r.RemoteAddr)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}
		}

		// Second pass: publish each parameter to its own sub-topic organized by device
		// Extract GID (device ID) for topic organization
		gidValues, hasGID := params["GID"]
		deviceID := "unknown"
		if hasGID && len(gidValues) > 0 && gidValues[0] != "" {
			deviceID = gidValues[0]
		}

		// Update device status (online/offline tracking)
		updateDeviceStatus(client, deviceID, topic)

		// Get current timestamp for this update
		now := time.Now()

		for paramName, values := range params {
			for _, value := range values {
				// Create device-specific topic structure: base_topic/device_id/parameter
				subTopic := topic + "/" + deviceID + "/" + paramName
				// Additional topic validation
				if !isValidMqttTopic(subTopic) {
					log.Printf("SECURITY: Invalid topic format '%s' from %s", subTopic, r.RemoteAddr)
					continue
				}

				token := client.Publish(subTopic, 0, false, value)
				// wait up to 5s for publish
				token.WaitTimeout(5 * time.Second)
				if token.Error() != nil {
					log.Printf("failed to publish to mqtt topic %s: %v", subTopic, token.Error())
				} else {
					log.Printf("published to topic %s: %s", subTopic, value)
				}
			}
		}

		// Publish the current timestamp for this device update
		timestampTopic := topic + "/" + deviceID + "/last_update"
		if isValidMqttTopic(timestampTopic) {
			// Format timestamp as RFC3339 (ISO 8601) for better readability
			timestampValue := now.Format(time.RFC3339)
			token := client.Publish(timestampTopic, 0, false, timestampValue)
			token.WaitTimeout(5 * time.Second)
			if token.Error() != nil {
				log.Printf("failed to publish timestamp to mqtt topic %s: %v", timestampTopic, token.Error())
			} else {
				log.Printf("published timestamp to topic %s: %s", timestampTopic, timestampValue)
			}
		}

		// Always return the original response text
		fmt.Fprint(w, "OK.ERR0")
	}
}

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

	// MQTT configuration via env vars
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	topic := os.Getenv("MQTT_TOPIC")
	if topic == "" {
		topic = "params"
	}
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")

	// Device monitoring configuration via env vars
	offlineThresholdStr := os.Getenv("OFFLINE_THRESHOLD_MINUTES")
	if offlineThresholdStr == "" {
		offlineThreshold = 30 * time.Minute // default 30 minutes
	} else {
		if minutes, err := time.ParseDuration(offlineThresholdStr + "m"); err != nil {
			log.Printf("Invalid OFFLINE_THRESHOLD_MINUTES '%s', using default 30 minutes", offlineThresholdStr)
			offlineThreshold = 30 * time.Minute
		} else {
			offlineThreshold = minutes
		}
	}

	checkFrequencyStr := os.Getenv("CHECK_FREQUENCY_MINUTES")
	if checkFrequencyStr == "" {
		checkFrequency = 5 * time.Minute // default 5 minutes
	} else {
		if minutes, err := time.ParseDuration(checkFrequencyStr + "m"); err != nil {
			log.Printf("Invalid CHECK_FREQUENCY_MINUTES '%s', using default 5 minutes", checkFrequencyStr)
			checkFrequency = 5 * time.Minute
		} else {
			checkFrequency = minutes
		}
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("gqgmc-mqtt-bridge-publisher")
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)

	// Set authentication if provided
	if username != "" {
		opts.SetUsername(username)
		if password != "" {
			opts.SetPassword(password)
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("mqtt connect failed: %v", token.Error())
	} else {
		log.Printf("mqtt connected successfully")
	}

	// Apply rate limiting middleware
	http.HandleFunc("/", rateLimitMiddleware(geigerCounterHandler(client, topic)))

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
				checkOfflineDevices(client, topic)
			case <-sigChan:
				return // Stop monitoring when shutdown signal received
			}
		}
	}()

	go func() {
		log.Printf("starting HTTP server on %s, mqtt broker %s, topic %s", server.Addr, broker, topic)
		log.Printf("security: rate limit 10 req/sec, max 5 params, allowed keys: GID, CPM, ACPM, uSV, AID")
		log.Printf("parameter types: GID=text/numeric(max 50 chars), CPM=integer, ACPM=float, uSV=float, AID=alphanumeric(max 50 chars)")
		log.Printf("monitoring: devices marked offline after %v of inactivity, checked every %v", offlineThreshold, checkFrequency)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("shutdown signal received, shutting down gracefully...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	client.Disconnect(250)
	log.Println("server stopped")
}
