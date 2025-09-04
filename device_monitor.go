package main

import (
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

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
				prettyLog(LogError, "failed to publish online status for device %s: %v", deviceID, token.Error())
			} else {
				prettyLog(LogDevice, "device %s marked as online", deviceID)
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
					prettyLog(LogError, "failed to publish online status for device %s: %v", deviceID, token.Error())
				} else {
					prettyLog(LogDevice, "device %s came back online", deviceID)
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
					prettyLog(LogError, "failed to publish offline status for device %s: %v", deviceID, token.Error())
				} else {
					prettyLog(LogWarning, "device %s marked as offline (last seen: %v)", deviceID, status.lastSeen.Format(time.RFC3339))
				}
			}
		}
	}
}
