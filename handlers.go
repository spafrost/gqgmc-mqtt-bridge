package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/time/rate"
)

// Rate limiter: 10 requests per second, burst of 100
var limiter = rate.NewLimiter(10, 100)

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

// Rate limiting middleware
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			prettyLog(LogSecurity, "Rate limit exceeded from %s", r.RemoteAddr)
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
			prettyLog(LogSecurity, "Invalid method '%s' from %s", r.Method, r.RemoteAddr)
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
		prettyLog(LogDebug, "connection from %s to %s with %d params",
			r.RemoteAddr, r.URL.Path, len(params))

		// Validate number of parameters
		if len(params) > 5 {
			prettyLog(LogSecurity, "Too many params (%d) from %s", len(params), r.RemoteAddr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// First pass: validate all parameters before processing any
		for paramName, values := range params {
			// Validate parameter key - only allow GID, CPM, ACPM, uSV, AID
			if !isValidGeigerParameter(paramName) {
				prettyLog(LogSecurity, "Invalid parameter key '%s' from %s (only GID, CPM, ACPM, uSV, AID allowed)", paramName, r.RemoteAddr)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			for _, value := range values {
				// Validate parameter value based on key type
				if !isValidGeigerValue(paramName, value) {
					prettyLog(LogSecurity, "Invalid value '%s' for key '%s' from %s", value, paramName, r.RemoteAddr)
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
					prettyLog(LogSecurity, "Invalid topic format '%s' from %s", subTopic, r.RemoteAddr)
					continue
				}

				token := client.Publish(subTopic, 0, false, value)
				// wait up to 5s for publish
				token.WaitTimeout(5 * time.Second)
				if token.Error() != nil {
					prettyLog(LogError, "failed to publish to mqtt topic %s: %v", subTopic, token.Error())
				} else {
					prettyLog(LogMQTT, "published to topic %s: %s", subTopic, value)
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
				prettyLog(LogError, "failed to publish timestamp to mqtt topic %s: %v", timestampTopic, token.Error())
			} else {
				prettyLog(LogMQTT, "published timestamp to topic %s: %s", timestampTopic, timestampValue)
			}
		}

		// Always return the original response text
		fmt.Fprint(w, "OK.ERR0")
	}
}
