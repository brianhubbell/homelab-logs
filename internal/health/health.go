package health

import (
	"math"
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

