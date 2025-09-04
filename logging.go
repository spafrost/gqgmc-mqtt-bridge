package main

import "log"

// LogLevel represents a log level with its name and color
type LogLevel struct {
	Name  string
	Color string
}

// Available log levels with embedded ANSI color codes
var (
	LogInfo     = LogLevel{"INFO", "\033[34m"}     // Blue
	LogSuccess  = LogLevel{"SUCCESS", "\033[32m"}  // Green
	LogWarning  = LogLevel{"WARNING", "\033[33m"}  // Yellow
	LogError    = LogLevel{"ERROR", "\033[31m"}    // Red
	LogSecurity = LogLevel{"SECURITY", "\033[31m"} // Red
	LogMQTT     = LogLevel{"MQTT", "\033[36m"}     // Cyan
	LogDevice   = LogLevel{"DEVICE", "\033[35m"}   // Purple
	LogDebug    = LogLevel{"DEBUG", "\033[90m"}    // Gray
)

// Colored logging function
func prettyLog(level LogLevel, format string, args ...interface{}) {
	log.Printf(level.Color+"[%s] "+format+"\033[0m", append([]interface{}{level.Name}, args...)...)
}

// Build information - set via ldflags during build
var (
	GitBranch = "unknown"
	GitCommit = "unknown"
	BuildTime = "unknown"
	Version   = "dev"
)
