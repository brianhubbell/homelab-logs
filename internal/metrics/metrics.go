package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server exposes Prometheus metrics on /metrics and a health check on /health.
type Server struct {
	startTime time.Time

	UptimeGauge      prometheus.Gauge
	MQTTConnected    prometheus.Gauge
	CommandsReceived prometheus.Counter
	CommandsExecuted prometheus.Counter
	CommandsFailed   prometheus.Counter

	mqttConnected atomic.Bool
	commandsRecv  atomic.Int64
	commandsExec  atomic.Int64
	commandsFail  atomic.Int64
}

// NewServer creates a metrics server with all Prometheus collectors registered.
func NewServer() *Server {
	s := &Server{
		startTime: time.Now(),
	}

	s.UptimeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "agent_uptime_seconds",
		Help: "Application uptime in seconds",
	})
	s.MQTTConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "agent_mqtt_connected",
		Help: "MQTT connection status (1 = connected, 0 = disconnected)",
	})
	s.CommandsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "agent_commands_received_total",
		Help: "Total number of commands received",
	})
	s.CommandsExecuted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "agent_commands_executed_total",
		Help: "Total number of commands executed successfully",
	})
	s.CommandsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "agent_commands_failed_total",
		Help: "Total number of commands that failed",
	})

	prometheus.MustRegister(
		s.UptimeGauge,
		s.MQTTConnected,
		s.CommandsReceived,
		s.CommandsExecuted,
		s.CommandsFailed,
	)

	return s
}

func (s *Server) IncReceived() {
	s.commandsRecv.Add(1)
	s.CommandsReceived.Inc()
}

func (s *Server) IncExecuted() {
	s.commandsExec.Add(1)
	s.CommandsExecuted.Inc()
}

func (s *Server) IncFailed() {
	s.commandsFail.Add(1)
	s.CommandsFailed.Inc()
}

func (s *Server) SetMQTTConnected(connected bool) {
	s.mqttConnected.Store(connected)
	if connected {
		s.MQTTConnected.Set(1)
	} else {
		s.MQTTConnected.Set(0)
	}
}

func (s *Server) UptimeSeconds() int64 {
	return int64(math.Round(time.Since(s.startTime).Seconds()))
}

func (s *Server) Start(port int) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.UptimeGauge.Set(float64(s.UptimeSeconds()))
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", s.healthHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("metrics server listening on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
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
