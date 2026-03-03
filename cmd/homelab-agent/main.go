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
	"github.com/brianhubbell/go-utils/mqtt"
	paho "github.com/eclipse/paho.mqtt.golang"

	"homelab-agent/internal/config"
	"homelab-agent/internal/executor"
	"homelab-agent/internal/handler"
	"homelab-agent/internal/health"
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
	goutils.SetService("homelab-agent", Version)
	if os.Getenv("HOST_TYPE") == "" {
		os.Setenv("HOST_TYPE", "physical")
	}

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	goutils.Log("config loaded",
		"broker", cfg.MQTTBroker, "prefix", cfg.TopicPrefix,
		"services", cfg.Services, "deployDir", cfg.DeployDir)

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
		AutoUpdateInterval: cfg.AutoUpdateInterval,
		DeployDir:          cfg.DeployDir,
	}
	met.SetMetricsCollector(executor.SystemMetrics)

	// 4. Build MQTT topics
	configTopic := fmt.Sprintf("control/%s/%s/config", cfg.TopicPrefix, hostname)
	commandTopic := fmt.Sprintf("control/%s/%s/command", cfg.TopicPrefix, hostname)
	responseTopic := fmt.Sprintf("control/%s/%s/response", cfg.TopicPrefix, hostname)
	desiredStateTopic := fmt.Sprintf("control/%s/%s/desired_state", cfg.TopicPrefix, hostname)

	// 5. Build node config for self-registration
	hostType := os.Getenv("HOST_TYPE")
	if hostType == "" {
		hostType = "physical"
	}
	address := os.Getenv("AGENT_ADDRESS")
	if address == "" {
		address = hostname + ".lan"
	}
	nodeConfig := map[string]interface{}{
		"hostname":  hostname,
		"label":     hostname,
		"type":      "agent",
		"host_type": hostType,
		"scope":     hostType,
		"address":   address,
		"services":  cfg.Services,
		"version":   Version,
	}

	// 6. Create executor
	exec := executor.New(cfg.Services)
	exec.DeployDir = cfg.DeployDir
	exec.CurrentVersion = Version
	exec.AutoUpdateInterval = cfg.AutoUpdateInterval
	logTopic := fmt.Sprintf("control/%s/%s/logs", cfg.TopicPrefix, hostname)
	exec.LogTopic = logTopic

	// Add service versions to node config
	nodeConfig["serviceVersions"] = exec.ServiceVersions()
	configPayload, err := json.Marshal(goutils.NewMessage(nodeConfig, nil, "config"))
	if err != nil {
		log.Fatalf("marshal node config: %v", err)
	}

	// desiredStateHandler applies a retained desired-state payload on connect.
	desiredStateHandler := func(_ paho.Client, msg paho.Message) {
		var envelope goutils.Message[map[string]string]
		if err := json.Unmarshal(msg.Payload(), &envelope); err != nil {
			goutils.Err("desired_state: unmarshal", "error", err)
			return
		}
		if errs := exec.ApplyDesiredState(envelope.Payload); len(errs) > 0 {
			goutils.Err("desired_state: some keys failed", "errors", errs)
		}
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
			if err := c.Subscribe(commandTopic, 1, h.HandleMessage); err != nil {
				goutils.Err("subscribe command topic", "error", err)
			}
		}
		if err := c.Subscribe(desiredStateTopic, 1, desiredStateHandler); err != nil {
			goutils.Err("subscribe desired_state topic", "error", err)
		}
	})
	if err != nil {
		log.Fatalf("MQTT: %v", err)
	}
	defer client.Stop()
	exec.Publish = func(topic string, payload []byte) error {
		return client.Publish(topic, payload)
	}

	// 8. Wire config change callback to update health info and re-publish node config
	exec.OnConfigChange = func(key, value string) {
		met.Info = health.HealthInfo{
			Version:            Version,
			Hostname:           hostname,
			AutoUpdateInterval: exec.AutoUpdateInterval,
			DeployDir:          exec.DeployDir,
		}
		goutils.Log("config updated via command", "key", key, "value", value)

		// Apply runtime-adjustable settings immediately without restart.
		if key == "metrics_interval" {
			client.ResetMetrics(time.Duration(exec.MetricsInterval) * time.Second)
		}

		// Re-publish node config so the control plane sees the change immediately.
		nodeConfig["auto_update_interval"] = exec.AutoUpdateInterval
		nodeConfig["metrics_interval"] = exec.MetricsInterval
		payload, err := json.Marshal(goutils.NewMessage(nodeConfig, nil, "config"))
		if err != nil {
			goutils.Err("marshal updated node config after config.set", "error", err)
			return
		}
		if err := client.PublishRetained(configTopic, payload); err != nil {
			goutils.Err("publish updated node config after config.set", "error", err)
		} else {
			goutils.Log("published updated node config after config.set", "topic", configTopic, "key", key, "value", value)
		}
	}

	// 9. Create handler — subscribe now if connected, otherwise onConnect callback handles it
	h = handler.New(exec, client, met, responseTopic)
	if err := client.Subscribe(commandTopic, 1, h.HandleMessage); err != nil {
		goutils.Log("initial subscribe deferred to onConnect", "reason", err)
	}
	if err := client.Subscribe(desiredStateTopic, 1, desiredStateHandler); err != nil {
		goutils.Log("desired_state subscribe deferred to onConnect", "reason", err)
	}

	// 10. Start metrics publishing via shared mqtt client
	client.StartMetrics(time.Duration(cfg.MetricsInterval)*time.Second, func(_ int64) any {
		return met.GetSystemMetrics() // returns nil when not yet available, skipping the tick
	})

	// 12. Start auto-update goroutine with resettable timer
	{
		go func() {
			timer := time.NewTimer(time.Duration(exec.AutoUpdateInterval) * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-timer.C:
					goutils.Log("auto-update check starting")
					result, err := exec.SelfUpdate()
					if err != nil {
						goutils.Err("auto-update failed", "error", err)
					} else {
						goutils.Log("auto-update check complete", "result", result)
					}
					timer.Reset(time.Duration(exec.AutoUpdateInterval) * time.Second)
				case <-exec.AutoUpdateIntervalChanged:
					// Interval was changed via config.set — reset the timer.
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					goutils.Log("auto-update interval changed", "newInterval", exec.AutoUpdateInterval)
					timer.Reset(time.Duration(exec.AutoUpdateInterval) * time.Second)
				}
			}
		}()
	}

	goutils.Log("homelab-agent ready", "hostname", hostname, "commandTopic", commandTopic)

	// 13. Signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		goutils.Log("shutting down", "signal", sig)
	case newVer := <-exec.ShutdownCh:
		goutils.Log("restarting for update", "new_version", newVer)
		// Give MQTT time to flush the response before we disconnect.
		time.Sleep(500 * time.Millisecond)
	}

	if err := client.PublishRetained(configTopic, []byte{}); err != nil {
		goutils.Err("clearing node config", "error", err)
	} else {
		goutils.Log("cleared node config", "topic", configTopic)
	}
	// client.Stop() is called via defer above.
	goutils.Log("goodbye")
	os.Exit(0)
}
