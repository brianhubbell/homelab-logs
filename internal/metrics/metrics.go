package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	goutils "github.com/brianhubbell/go-utils"
)

// Server exposes a health check on /health and tracks operational metrics.
type Server struct {
	startTime time.Time

	mqttConnected atomic.Bool
	commandsRecv  atomic.Int64
	commandsExec  atomic.Int64
	commandsFail  atomic.Int64
}

// NewServer creates a metrics server.
func NewServer() *Server {
	s := &Server{
		startTime: time.Now(),
	}
	return s
}

func (s *Server) IncReceived() {
	s.commandsRecv.Add(1)
}

func (s *Server) IncExecuted() {
	s.commandsExec.Add(1)
}

func (s *Server) IncFailed() {
	s.commandsFail.Add(1)
}

func (s *Server) SetMQTTConnected(connected bool) {
	s.mqttConnected.Store(connected)
}

func (s *Server) UptimeSeconds() int64 {
	return int64(math.Round(time.Since(s.startTime).Seconds()))
}

func (s *Server) GetMetricsPayload() map[string]interface{} {
	return map[string]interface{}{
		"uptime_seconds":    s.UptimeSeconds(),
		"mqtt_connected":    s.mqttConnected.Load(),
		"commands_received": s.commandsRecv.Load(),
		"commands_executed": s.commandsExec.Load(),
		"commands_failed":   s.commandsFail.Load(),
	}
}

func (s *Server) Start(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	go func() {
		goutils.Log("metrics server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			goutils.Err("metrics server error", "error", err)
		}
	}()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := struct {
		Status        string `json:"status"`
		MQTTConnected bool   `json:"mqtt_connected"`
		UptimeSeconds int64  `json:"uptime_seconds"`
		Commands      struct {
			Received int64 `json:"received"`
			Executed int64 `json:"executed"`
			Failed   int64 `json:"failed"`
		} `json:"commands"`
	}{
		Status:        "ok",
		MQTTConnected: s.mqttConnected.Load(),
		UptimeSeconds: s.UptimeSeconds(),
	}
	health.Commands.Received = s.commandsRecv.Load()
	health.Commands.Executed = s.commandsExec.Load()
	health.Commands.Failed = s.commandsFail.Load()

	_ = json.NewEncoder(w).Encode(health)
}
