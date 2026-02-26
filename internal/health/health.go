package health

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	goutils "github.com/brianhubbell/go-utils"
)

// HealthInfo holds static attributes exposed by the health endpoint.
type HealthInfo struct {
	Version  string `json:"-"`
	Hostname string `json:"-"`

	AutoUpdateInterval int    `json:"auto_update_interval"`
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

	// HTTP endpoint metrics
	httpRequests  atomic.Int64
	httpErrors    atomic.Int64
	httpLatencyNs atomic.Int64 // cumulative nanoseconds

	// Cached system metrics, refreshed in background
	systemMetricsMu     sync.RWMutex
	cachedSystemMetrics map[string]interface{}
	metricsCollector    func() (map[string]interface{}, error)
}

// NewServer creates a health server.
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

// SetMetricsCollector injects a system metrics function and starts background refresh.
func (s *Server) SetMetricsCollector(fn func() (map[string]interface{}, error)) {
	s.metricsCollector = fn
	go func() {
		s.refreshSystemMetrics()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.refreshSystemMetrics()
		}
	}()
}

func (s *Server) refreshSystemMetrics() {
	if s.metricsCollector == nil {
		return
	}
	data, err := s.metricsCollector()
	if err != nil {
		goutils.Debug("system metrics refresh error", "error", err)
		return
	}
	s.systemMetricsMu.Lock()
	s.cachedSystemMetrics = data
	s.systemMetricsMu.Unlock()
}

// GetSystemMetrics returns a copy of the cached system metrics map.
func (s *Server) GetSystemMetrics() map[string]interface{} {
	s.systemMetricsMu.RLock()
	defer s.systemMetricsMu.RUnlock()
	if s.cachedSystemMetrics == nil {
		return nil
	}
	copy := make(map[string]interface{}, len(s.cachedSystemMetrics))
	for k, v := range s.cachedSystemMetrics {
		copy[k] = v
	}
	return copy
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
		goutils.Log("health server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			goutils.Err("health server error", "error", err)
		}
	}()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.httpRequests.Add(1)

	w.Header().Set("Content-Type", "application/json")

	totalReqs := s.httpRequests.Load()
	totalLatency := s.httpLatencyNs.Load()
	var avgLatencyMs float64
	if totalReqs > 1 {
		avgLatencyMs = float64(totalLatency) / float64(totalReqs-1) / 1e6
	}

	health := map[string]interface{}{
		"status":            http.StatusOK,
		"version":           s.Info.Version,
		"hostname":          s.Info.Hostname,
		"os":                runtime.GOOS,
		"arch":              runtime.GOARCH,
		"go_version":        runtime.Version(),
		"mqtt_connected":    s.mqttConnected.Load(),
		"uptime_seconds":    s.UptimeSeconds(),
		"num_goroutine":     runtime.NumGoroutine(),
		"config":            s.Info,
		"commands_received": s.commandsRecv.Load(),
		"commands_executed": s.commandsExec.Load(),
		"commands_failed":   s.commandsFail.Load(),
		"http": map[string]interface{}{
			"requests_total":   totalReqs,
			"errors_total":     s.httpErrors.Load(),
			"avg_latency_ms":   avgLatencyMs,
		},
	}

	// Merge cached system metrics
	s.systemMetricsMu.RLock()
	sysMetrics := s.cachedSystemMetrics
	s.systemMetricsMu.RUnlock()
	if sysMetrics != nil {
		health["system"] = sysMetrics
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		s.httpErrors.Add(1)
	}

	s.httpLatencyNs.Add(time.Since(start).Nanoseconds())
}
