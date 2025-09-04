package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

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
