package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	goutils "github.com/brianhubbell/go-utils"

	"homelab-agent/internal/config"
	"homelab-agent/internal/executor"
	"homelab-agent/internal/handler"
	"homelab-agent/internal/health"
	"homelab-agent/internal/mqtt"
)

// Version is injected at build time via ldflags.
var Version string

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(Version)
		os.Exit(0)
	}
	platformMain()
}

func run() {
	// Set app metadata for watermarks
	os.Setenv("APP_NAME", "homelab-agent")
	os.Setenv("APP_VERSION", Version)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	goutils.Log("config loaded",
		"broker", cfg.MQTTBroker, "prefix", cfg.TopicPrefix,
		"services", cfg.AllowedServices, "composePaths", cfg.AllowedComposePaths,
		"metricsInterval", cfg.MetricsInterval, "healthPort", cfg.HealthPort,
		"deployEnabled", cfg.DeployEnabled, "deployDir", cfg.DeployDir)

	// 2. Resolve hostname
	fullHostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("hostname: %v", err)
	}
	hostname := fullHostname
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}

	// 3. Start health server
	met := health.NewServer()
	met.Info = health.HealthInfo{
		Version:            Version,
		Hostname:           hostname,
		AutoUpdateEnabled:  cfg.AutoUpdateEnabled,
		AutoUpdateRepo:     cfg.AutoUpdateRepo,
		AutoUpdateInterval: cfg.AutoUpdateInterval,
		DeployEnabled:      cfg.DeployEnabled,
		DeployDir:          cfg.DeployDir,
	}
	met.Start(cfg.HealthPort)

	// 4. Build MQTT topics
	configTopic := fmt.Sprintf("control/%s/%s/config", cfg.TopicPrefix, hostname)
	commandTopic := fmt.Sprintf("control/%s/%s/command", cfg.TopicPrefix, hostname)
	responseTopic := fmt.Sprintf("control/%s/%s/response", cfg.TopicPrefix, hostname)
	metricsTopic := fmt.Sprintf("observability/homelab-agent/%s/metrics", hostname)

	// 5. Build node config for self-registration
	hostType := os.Getenv("HOST_TYPE")
	if hostType == "" {
		hostType = "physical"
	}
	nodeConfig := map[string]interface{}{
		"hostname":            hostname,
		"label":               hostname,
		"type":                "agent",
		"host_type":           hostType,
		"address":             hostname + ".lan",
		"port":                cfg.HealthPort,
		"allowedServices":     cfg.AllowedServices,
		"allowedComposePaths": cfg.AllowedComposePaths,
		"version":             Version,
	}
	configPayload, err := json.Marshal(goutils.NewMessage(nodeConfig, nil, "config"))
	if err != nil {
		log.Fatalf("marshal node config: %v", err)
	}

	// 6. Create executor
	exec := executor.New(cfg.AllowedServices, cfg.AllowedComposePaths)
	exec.DeployEnabled = cfg.DeployEnabled
	exec.DeployDir = cfg.DeployDir
	exec.CurrentVersion = Version
	exec.AutoUpdateEnabled = cfg.AutoUpdateEnabled
	exec.AutoUpdateRepo = cfg.AutoUpdateRepo
	exec.AutoUpdateInterval = cfg.AutoUpdateInterval

	// Add service versions to node config
	nodeConfig["serviceVersions"] = exec.ServiceVersions()
	configPayload, err = json.Marshal(goutils.NewMessage(nodeConfig, nil, "config"))
	if err != nil {
		log.Fatalf("marshal node config: %v", err)
	}

	// 7. Connect MQTT
	var h *handler.Handler
	client, err := mqtt.NewClient(cfg.MQTTBroker, func(connected bool) {
		met.SetMQTTConnected(connected)
		goutils.Debug("mqtt status", "connected", connected)
	}, func(c *mqtt.Client) {
		if err := c.PublishRetained(configTopic, configPayload); err != nil {
			goutils.Err("publish node config", "error", err)
		} else {
			goutils.Log("published node config", "topic", configTopic)
		}
		if h != nil {
			if err := c.Subscribe(commandTopic, h.HandleMessage); err != nil {
				goutils.Err("subscribe command topic", "error", err)
			}
		}
	})
	if err != nil {
		log.Fatalf("MQTT: %v", err)
	}

	// 8. Wire whitelist change callback to re-publish node config
	exec.OnWhitelistChange = func(services []string) {
		nodeConfig["allowedServices"] = services
		nodeConfig["serviceVersions"] = exec.ServiceVersions()
		payload, err := json.Marshal(goutils.NewMessage(nodeConfig, nil, "config"))
		if err != nil {
			goutils.Err("marshal updated node config", "error", err)
			return
		}
		if err := client.PublishRetained(configTopic, payload); err != nil {
			goutils.Err("publish updated node config", "error", err)
		} else {
			goutils.Log("published updated node config", "topic", configTopic, "allowedServices", services)
		}
	}

	// 8b. Wire config change callback to update health info
	exec.OnConfigChange = func(key, value string) {
		met.Info = health.HealthInfo{
			Version:            Version,
			Hostname:           hostname,
			AutoUpdateEnabled:  exec.AutoUpdateEnabled,
			AutoUpdateRepo:     exec.AutoUpdateRepo,
			AutoUpdateInterval: exec.AutoUpdateInterval,
			DeployEnabled:      exec.DeployEnabled,
			DeployDir:          exec.DeployDir,
		}
		goutils.Log("config updated via command", "key", key, "value", value)
	}

	// 9. Create handler and subscribe to command topic
	h = handler.New(exec, client, met, responseTopic)
	if err := client.Subscribe(commandTopic, h.HandleMessage); err != nil {
		log.Fatalf("subscribe command topic: %v", err)
	}

	// 10. Start metrics publishing ticker
	if cfg.MetricsInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.MetricsInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				data, err := executor.SystemMetrics()
				if err != nil {
					goutils.Debug("system metrics error", "error", err)
					continue
				}
				data["host_type"] = hostType
				for k, v := range met.GetMetricsPayload() {
					data[k] = v
				}
				envelope := goutils.NewMessage(data, nil, "metrics")
				payload, err := json.Marshal(envelope)
				if err != nil {
					continue
				}
				if err := client.Publish(metricsTopic, payload); err != nil {
					goutils.Debug("publish metrics error", "error", err)
				}
			}
		}()
	}

	// 11. Start auto-update ticker
	if cfg.AutoUpdateEnabled && cfg.AutoUpdateRepo != "" {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.AutoUpdateInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				goutils.Log("auto-update check starting")
				result, err := exec.SelfUpdate()
				if err != nil {
					goutils.Err("auto-update failed", "error", err)
					continue
				}
				goutils.Log("auto-update check complete", "result", result)
			}
		}()
		goutils.Log("auto-update enabled", "repo", cfg.AutoUpdateRepo, "interval", cfg.AutoUpdateInterval)
	}

	goutils.Log("homelab-agent ready", "hostname", hostname, "commandTopic", commandTopic)

	// 12. Signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	goutils.Log("shutting down", "signal", sig)
	if err := client.PublishRetained(configTopic, []byte{}); err != nil {
		goutils.Err("clearing node config", "error", err)
	} else {
		goutils.Log("cleared node config", "topic", configTopic)
	}
	client.Disconnect()
	goutils.Log("goodbye")
	os.Exit(0)
}
