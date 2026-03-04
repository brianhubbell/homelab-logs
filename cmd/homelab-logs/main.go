package main

import (
	"bufio"
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
		"journalUnit", cfg.JournalUnit)

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

	// 4. Start heartbeat
	client.StartHeartbeat(10 * time.Second)

	// 5. Stream journalctl to MQTT
	logTopic := fmt.Sprintf("control/%s/%s/logs", cfg.TopicPrefix, hostname)
	go func() {
		for {
			cmd := osexec.Command("journalctl", "-u", cfg.JournalUnit, "-f", "-n", "50", "--output=cat", "--no-pager")
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
				line := scanner.Text()
				if strings.Contains(strings.ToLower(line), "redis") {
					continue
				}
				payload, _ := json.Marshal(map[string]any{
					"line": line,
					"ts":   time.Now().UnixMilli(),
				})
				if err := client.Publish(logTopic, payload); err != nil {
					goutils.Err("log stream: publish", "error", err)
				}
			}
			_ = cmd.Wait()
			time.Sleep(2 * time.Second)
		}
	}()

	goutils.Log("homelab-logs ready", "hostname", hostname, "logTopic", logTopic)

	// 6. Signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	goutils.Log("shutting down", "signal", sig)
	goutils.Log("goodbye")
	os.Exit(0)
}
