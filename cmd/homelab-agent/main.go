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

	"homelab-agent/internal/config"
	"homelab-agent/internal/executor"
	"homelab-agent/internal/handler"
	"homelab-agent/internal/message"
	"homelab-agent/internal/metrics"
	"homelab-agent/internal/mqtt"
)

func main() {
	platformMain()
}

func run() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("CONFIG mqttBroker=%s topicPrefix=%s allowedServices=%v composePaths=%v debug=%v metricsInterval=%ds metricsPort=%d",
		cfg.MQTTBroker, cfg.TopicPrefix, cfg.AllowedServices, cfg.AllowedComposePaths,
		cfg.Debug, cfg.MetricsInterval, cfg.MetricsPort)

	// 2. Start metrics server
	met := metrics.NewServer()
	met.Start(cfg.MetricsPort)

	// 3. Resolve hostname
	fullHostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("hostname: %v", err)
	}
	hostname := fullHostname
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}

	// 4. Build MQTT topics
	configTopic := fmt.Sprintf("%s/node/%s/config", cfg.TopicPrefix, hostname)
	commandTopic := fmt.Sprintf("%s/node/%s/command", cfg.TopicPrefix, hostname)
	responseTopic := fmt.Sprintf("%s/node/%s/response", cfg.TopicPrefix, hostname)
	metricsTopic := fmt.Sprintf("%s/node/%s/metrics", cfg.TopicPrefix, hostname)

	// 5. Build node config for self-registration
	nodeConfig := map[string]interface{}{
		"hostname":         hostname,
		"label":            hostname,
		"type":             "agent",
		"address":          hostname + ".lan",
		"port":             cfg.MetricsPort,
		"allowedServices":  cfg.AllowedServices,
		"allowedComposePaths": cfg.AllowedComposePaths,
		"version":          message.Version,
	}
	configPayload, err := json.Marshal(message.NewMessage(nodeConfig, nil, "config"))
	if err != nil {
		log.Fatalf("marshal node config: %v", err)
	}

	// 6. Create executor
	exec := executor.New(cfg.AllowedServices, cfg.AllowedComposePaths)

	// 7. Connect MQTT
	var h *handler.Handler
	client, err := mqtt.NewClient(cfg.MQTTBroker, func(connected bool) {
		met.SetMQTTConnected(connected)
		if cfg.Debug {
			log.Printf("DEBUG mqttConnected=%v", connected)
		}
	}, func(c *mqtt.Client) {
		// On every (re)connect: publish config and re-subscribe
		if err := c.PublishRetained(configTopic, configPayload); err != nil {
			log.Printf("ERROR publishing node config: %v", err)
		} else {
			log.Printf("published node config to %s", configTopic)
		}
		if h != nil {
			if err := c.Subscribe(commandTopic, h.HandleMessage); err != nil {
				log.Printf("ERROR subscribing to command topic: %v", err)
			}
		}
	})
	if err != nil {
		log.Fatalf("MQTT: %v", err)
	}

	// 8. Create handler and subscribe to command topic
	h = handler.New(exec, client, met, responseTopic, cfg.Debug)
	if err := client.Subscribe(commandTopic, h.HandleMessage); err != nil {
		log.Fatalf("subscribe command topic: %v", err)
	}

	// 9. Start metrics publishing ticker
	if cfg.MetricsInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.MetricsInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				data, err := executor.SystemMetrics()
				if err != nil {
					if cfg.Debug {
						log.Printf("DEBUG system metrics error: %v", err)
					}
					continue
				}
				envelope := message.NewMessage(data, nil, "metrics")
				payload, err := json.Marshal(envelope)
				if err != nil {
					continue
				}
				if err := client.Publish(metricsTopic, payload); err != nil {
					if cfg.Debug {
						log.Printf("DEBUG publish metrics error: %v", err)
					}
				}
			}
		}()
	}

	log.Printf("homelab-agent ready on %s (listening on %s)", hostname, commandTopic)

	// 10. Signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)
	if err := client.PublishRetained(configTopic, []byte{}); err != nil {
		log.Printf("ERROR clearing node config: %v", err)
	} else {
		log.Printf("cleared node config from %s", configTopic)
	}
	client.Disconnect()
	log.Printf("goodbye")
	os.Exit(0)
}
