package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	goutils "github.com/brianhubbell/go-utils"
)

// HealthInfo holds static attributes exposed by the health endpoint.
type HealthInfo struct {
	Version  string `json:"-"`
	Hostname string `json:"-"`

	AutoUpdateEnabled  bool   `json:"auto_update_enabled"`
	AutoUpdateRepo     string `json:"auto_update_repo,omitempty"`
	AutoUpdateInterval int    `json:"auto_update_interval"`
	DeployEnabled      bool   `json:"deploy_enabled"`
	DeployDir          string `json:"deploy_dir"`
}

// Server exposes a health check on /health and tracks operational metrics.
type Server struct {
	startTime time.Time
	Info      HealthInfo

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
		Status        int        `json:"status"`
		Version       string     `json:"version"`
		Hostname      string     `json:"hostname"`
		OS            string     `json:"os"`
		Arch          string     `json:"arch"`
		GoVersion     string     `json:"go_version"`
		MQTTConnected bool       `json:"mqtt_connected"`
		UptimeSeconds int64      `json:"uptime_seconds"`
		NumGoroutine  int        `json:"num_goroutine"`
		Config        HealthInfo `json:"config"`
	}{
		Status:        http.StatusOK,
		Version:       s.Info.Version,
		Hostname:      s.Info.Hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		MQTTConnected: s.mqttConnected.Load(),
		UptimeSeconds: s.UptimeSeconds(),
		NumGoroutine:  runtime.NumGoroutine(),
		Config:        s.Info,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(health)
}
