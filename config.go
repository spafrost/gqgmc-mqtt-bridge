package main

import (
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config holds all configuration values
type Config struct {
	Broker           string
	Topic            string
	Username         string
	Password         string
	OfflineThreshold time.Duration
	CheckFrequency   time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := &Config{
		Broker:   os.Getenv("MQTT_BROKER"),
		Topic:    os.Getenv("MQTT_TOPIC"),
		Username: os.Getenv("MQTT_USERNAME"),
		Password: os.Getenv("MQTT_PASSWORD"),
	}

	// Set defaults
	if config.Broker == "" {
		config.Broker = "tcp://localhost:1883"
	}
	if config.Topic == "" {
		config.Topic = "params"
	}

	// Device monitoring configuration
	offlineThresholdStr := os.Getenv("OFFLINE_THRESHOLD_MINUTES")
	if offlineThresholdStr == "" {
		config.OfflineThreshold = 30 * time.Minute // default 30 minutes
	} else {
		if minutes, err := time.ParseDuration(offlineThresholdStr + "m"); err != nil {
			prettyLog(LogWarning, "Invalid OFFLINE_THRESHOLD_MINUTES '%s', using default 30 minutes", offlineThresholdStr)
			config.OfflineThreshold = 30 * time.Minute
		} else {
			config.OfflineThreshold = minutes
		}
	}

	checkFrequencyStr := os.Getenv("CHECK_FREQUENCY_MINUTES")
	if checkFrequencyStr == "" {
		config.CheckFrequency = 5 * time.Minute // default 5 minutes
	} else {
		if minutes, err := time.ParseDuration(checkFrequencyStr + "m"); err != nil {
			prettyLog(LogWarning, "Invalid CHECK_FREQUENCY_MINUTES '%s', using default 5 minutes", checkFrequencyStr)
			config.CheckFrequency = 5 * time.Minute
		} else {
			config.CheckFrequency = minutes
		}
	}

	return config
}

// CreateMQTTClient creates and configures an MQTT client
func CreateMQTTClient(config *Config) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetClientID("gqgmc-mqtt-bridge-publisher")
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)

	// Set authentication if provided
	if config.Username != "" {
		opts.SetUsername(config.Username)
		if config.Password != "" {
			opts.SetPassword(config.Password)
		}
	}

	return mqtt.NewClient(opts)
}
