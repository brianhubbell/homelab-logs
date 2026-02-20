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
	HealthPort          int
	MetricsInterval     int
	Debug               bool
	DeployEnabled       bool
	DeployDir           string
	AutoUpdateEnabled   bool
	AutoUpdateRepo      string
	AutoUpdateInterval  int
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
		HealthPort:      9110,
		MetricsInterval: 60,
		Debug:           goutils.StrToBool(os.Getenv("DEBUG")),
		DeployEnabled:   goutils.StrToBool(os.Getenv("DEPLOY_ENABLED")),
		DeployDir:       envOrDefault("DEPLOY_DIR", "/opt/homelab-services"),
		AutoUpdateEnabled:  goutils.StrToBool(os.Getenv("AUTO_UPDATE_ENABLED")),
		AutoUpdateRepo:     os.Getenv("AUTO_UPDATE_REPO"),
		AutoUpdateInterval: 3600,
	}

	if v := os.Getenv("ALLOWED_COMPOSE_PATHS"); v != "" {
		cfg.AllowedComposePaths = splitCSV(v)
	}

	if v := os.Getenv("HEALTH_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid HEALTH_PORT %q: %w", v, err)
		}
		cfg.HealthPort = n
	}

	if v := os.Getenv("METRICS_INTERVAL_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_INTERVAL_SECONDS %q: %w", v, err)
		}
		cfg.MetricsInterval = n
	}

	if v := os.Getenv("AUTO_UPDATE_INTERVAL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid AUTO_UPDATE_INTERVAL %q: %w", v, err)
		}
		cfg.AutoUpdateInterval = n
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
