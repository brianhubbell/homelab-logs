package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	goutils "github.com/brianhubbell/go-utils"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	MQTTBroker          string
	TopicPrefix         string
	AllowedServices     []string
	AllowedComposePaths []string
	MetricsPort         int
	MetricsInterval     int
	Debug               bool
	DeployEnabled       bool
	DeployDir           string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		return nil, fmt.Errorf("MQTT_BROKER environment variable is required")
	}

	allowedRaw := os.Getenv("ALLOWED_SERVICES")
	if allowedRaw == "" {
		return nil, fmt.Errorf("ALLOWED_SERVICES environment variable is required")
	}

	cfg := &Config{
		MQTTBroker:      broker,
		TopicPrefix:     envOrDefault("TOPIC_PREFIX", "agent"),
		AllowedServices: splitCSV(allowedRaw),
		MetricsPort:     9110,
		MetricsInterval: 60,
		Debug:           goutils.StrToBool(os.Getenv("DEBUG")),
		DeployEnabled:   goutils.StrToBool(os.Getenv("DEPLOY_ENABLED")),
		DeployDir:       envOrDefault("DEPLOY_DIR", "/opt/homelab-services"),
	}

	if v := os.Getenv("ALLOWED_COMPOSE_PATHS"); v != "" {
		cfg.AllowedComposePaths = splitCSV(v)
	}

	if v := os.Getenv("METRICS_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_PORT %q: %w", v, err)
		}
		cfg.MetricsPort = n
	}

	if v := os.Getenv("METRICS_INTERVAL_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_INTERVAL_SECONDS %q: %w", v, err)
		}
		cfg.MetricsInterval = n
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
