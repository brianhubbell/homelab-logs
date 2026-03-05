package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	osexec "os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	goutils "github.com/brianhubbell/go-utils"
	"github.com/brianhubbell/go-utils/mqtt"

	"homelab-logs/internal/config"
	"homelab-logs/internal/docker"
)

// Version is injected at build time via ldflags.
var Version string

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(Version)
		os.Exit(0)
	}

	goutils.SetService("homelab-logs", Version)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	goutils.Log("config loaded",
		"broker", cfg.MQTTBroker, "prefix", cfg.TopicPrefix,
		"logSource", cfg.LogSource)

	// 2. Resolve hostname
	fullHostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("hostname: %v", err)
	}
	hostname := fullHostname
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}

	// 3. Connect MQTT
	client, err := mqtt.NewClient(cfg.MQTTBroker, func(connected bool) {
		goutils.Debug("mqtt status", "connected", connected)
	}, nil)
	if err != nil {
		log.Fatalf("MQTT: %v", err)
	}
	defer client.Stop()

	// 4. Build log topic
	logTopic := fmt.Sprintf("control/%s/%s/logs", cfg.TopicPrefix, hostname)

	// publish helper
	publishLine := func(line string) {
		payload, _ := json.Marshal(map[string]any{
			"line": line,
			"ts":   time.Now().UnixMilli(),
		})
		if err := client.Publish(logTopic, payload); err != nil {
			goutils.Err("log stream: publish", "error", err)
		}
	}

	// 6. Start streaming based on log source
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch cfg.LogSource {
	case "docker":
		if cfg.DockerContainer == "" {
			log.Fatal("DOCKER_CONTAINER is required when LOG_SOURCE=docker")
		}
		go streamDockerLogs(ctx, cfg.DockerContainer, publishLine)
		goutils.Log("streaming docker logs", "container", cfg.DockerContainer, "topic", logTopic)
	default:
		go streamJournalLogs(ctx, cfg.JournalUnit, publishLine)
		goutils.Log("streaming journal logs", "unit", cfg.JournalUnit, "topic", logTopic)
	}

	goutils.Log("homelab-logs ready", "hostname", hostname, "logTopic", logTopic)

	// 7. Signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	cancel()
	goutils.Log("shutting down", "signal", sig)
	goutils.Log("goodbye")
	os.Exit(0)
}

func streamJournalLogs(ctx context.Context, unit string, publish func(string)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cmd := osexec.CommandContext(ctx, "journalctl", "-u", unit, "-f", "-n", "50", "--output=cat", "--no-pager")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			goutils.Err("log stream: pipe", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			goutils.Err("log stream: start", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			publish(scanner.Text())
		}
		_ = cmd.Wait()
		time.Sleep(2 * time.Second)
	}
}

func streamDockerLogs(ctx context.Context, container string, publish func(string)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := docker.StreamLogs(ctx, container, 50, publish)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			goutils.Err("docker log stream", "container", container, "error", err)
		}
		time.Sleep(2 * time.Second)
	}
}
