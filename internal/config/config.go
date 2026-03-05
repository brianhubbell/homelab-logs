package config

import (
	"fmt"
	"os"

	goutils "github.com/brianhubbell/go-utils"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	MQTTBroker      string
	TopicPrefix     string
	Debug           bool
	JournalUnit     string
	LogSource       string
	DockerContainer string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		return nil, fmt.Errorf("MQTT_BROKER environment variable is required")
	}

	cfg := &Config{
		MQTTBroker:      broker,
		TopicPrefix:     envOrDefault("TOPIC_PREFIX", "agent"),
		Debug:           goutils.StrToBool(os.Getenv("DEBUG")),
		JournalUnit:     envOrDefault("JOURNAL_UNIT", "homelab-logs"),
		LogSource:       envOrDefault("LOG_SOURCE", "journal"),
		DockerContainer: os.Getenv("DOCKER_CONTAINER"),
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
