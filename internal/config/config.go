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
	MQTTBroker         string
	TopicPrefix        string
	Services           []string
	HealthPort         int
	MetricsInterval    int
	Debug              bool
	DeployDir          string
	AutoUpdateInterval int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		return nil, fmt.Errorf("MQTT_BROKER environment variable is required")
	}

	cfg := &Config{
		MQTTBroker:         broker,
		TopicPrefix:        envOrDefault("TOPIC_PREFIX", "agent"),
		Services:           splitCSV(os.Getenv("SERVICES")),
		HealthPort:         9110,
		MetricsInterval:    60,
		Debug:              goutils.StrToBool(os.Getenv("DEBUG")),
		DeployDir:          envOrDefault("DEPLOY_DIR", "/opt/homelab-services"),
		AutoUpdateInterval: 3600,
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

	// Apply persisted overrides (from config.set commands).
	// Overrides take precedence since they represent explicit user changes.
	overrides, err := LoadOverrides()
	if err != nil {
		goutils.Err("loading config overrides", "error", err)
	} else {
		applyOverrides(cfg, overrides)
	}

	return cfg, nil
}

// applyOverrides applies persisted key-value overrides to the config.
func applyOverrides(cfg *Config, overrides map[string]string) {
	for key, value := range overrides {
		switch key {
		case "auto_update_interval":
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				cfg.AutoUpdateInterval = n
			}
		}
	}
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
