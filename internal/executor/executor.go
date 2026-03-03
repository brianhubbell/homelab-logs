package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	exec "os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"homelab-agent/internal/config"

	goutils "github.com/brianhubbell/go-utils"
)

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

// Request represents an incoming command.
type Request struct {
	ID     string            `json:"id"`
	Action string            `json:"action"`
	Args   map[string]string `json:"args"`
}

// Response represents the result of an executed command.
type Response struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	Action     string                 `json:"action"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
}

// Executor dispatches commands.
type Executor struct {
	Services           []string
	DeployDir          string
	AutoUpdateInterval int
	MetricsInterval    int
	OnConfigChange     func(key, value string)
	CurrentVersion     string

	// AutoUpdateIntervalChanged is signalled when the auto_update_interval is
	// changed via config.set so the ticker goroutine can reset itself.
	AutoUpdateIntervalChanged chan struct{}

	// ShutdownCh is sent on after a successful self-update so main can
	// publish the MQTT response before exiting and letting the service
	// manager restart with the new binary.
	ShutdownCh chan string

	// Log streaming
	Publish    func(topic string, payload []byte) error
	LogTopic   string
	logMu      sync.Mutex
	logCancel  context.CancelFunc
	logGeneration int
}

// New creates an Executor.
func New(services []string) *Executor {
	return &Executor{
		Services:                  services,
		AutoUpdateIntervalChanged: make(chan struct{}, 1),
		ShutdownCh:                make(chan string, 1),
	}
}

// Execute runs the given action with args.
func (e *Executor) Execute(req Request) Response {
	resp := Response{
		ID:     req.ID,
		Action: req.Action,
	}

	switch req.Action {
	case "ping":
		resp.Status = "ok"
		resp.Data = map[string]interface{}{"pong": true}

	case "service.status", "service.start", "service.stop", "service.restart":
		svc := req.Args["service"]
		if svc == "" {
			resp.Status = "error"
			resp.Error = "missing required arg: service"
			return resp
		}
		op := strings.TrimPrefix(req.Action, "service.")
		data, err := serviceControl(svc, op)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	case "compose.status", "compose.start", "compose.stop", "compose.restart":
		path := req.Args["path"]
		if path == "" {
			resp.Status = "error"
			resp.Error = "missing required arg: path"
			return resp
		}
		svc := req.Args["service"] // optional: specific service within compose
		op := strings.TrimPrefix(req.Action, "compose.")
		data, err := composeControl(path, svc, op)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	case "service.check":
		svc := req.Args["service"]
		if svc == "" {
			resp.Status = "error"
			resp.Error = "missing required arg: service"
			return resp
		}
		data, err := e.serviceCheck(svc)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	case "agent.update":
		data, err := e.SelfUpdate()
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	case "system.metrics":
		data, err := SystemMetrics()
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	case "config.get":
		resp.Status = "ok"
		resp.Data = map[string]interface{}{
			"deploy_dir":           e.DeployDir,
			"auto_update_interval": e.AutoUpdateInterval,
			"metrics_interval":     e.MetricsInterval,
		}

	case "config.set":
		key := req.Args["key"]
		value := req.Args["value"]
		if key == "" {
			resp.Status = "error"
			resp.Error = "missing required arg: key"
			return resp
		}
		if err := e.applyConfigKey(key, value); err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = map[string]interface{}{"key": key, "value": value}

	case "log.stream":
		go e.streamAgentLogs()
		resp.Status = "ok"
		resp.Data = map[string]interface{}{"message": "log streaming started"}

	case "log.stop":
		e.stopLogStream()
		resp.Status = "ok"
		resp.Data = map[string]interface{}{"message": "log streaming stopped"}

	default:
		resp.Status = "error"
		resp.Error = fmt.Sprintf("unknown action %q", req.Action)
	}

	return resp
}

// applyConfigKey validates, applies, persists, and notifies a single config change.
func (e *Executor) applyConfigKey(key, value string) error {
	switch key {
	case "auto_update_interval":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid interval %q", value)
		}
		e.AutoUpdateInterval = n
		select {
		case e.AutoUpdateIntervalChanged <- struct{}{}:
		default:
		}
	case "metrics_interval":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid interval %q", value)
		}
		e.MetricsInterval = n
	case "deploy_dir":
		if value == "" {
			return fmt.Errorf("deploy_dir cannot be empty")
		}
		e.DeployDir = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}

	if err := config.SaveOverride(key, value); err != nil {
		goutils.Err("persisting config override", "key", key, "error", err)
	}
	if e.OnConfigChange != nil {
		e.OnConfigChange(key, value)
	}
	return nil
}

func (e *Executor) streamAgentLogs() {
	e.logMu.Lock()
	if e.logCancel != nil {
		e.logCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	e.logCancel = cancel
	e.logGeneration++
	gen := e.logGeneration
	e.logMu.Unlock()

	defer func() {
		e.logMu.Lock()
		if e.logGeneration == gen {
			e.logCancel = nil
		}
		e.logMu.Unlock()
		cancel()
	}()

	cmd := exec.CommandContext(ctx, "journalctl", "-u", "homelab-agent", "-f", "-n", "100", "--output=cat", "--no-pager")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		goutils.Err("log.stream: stdout pipe", "error", err)
		return
	}
	if err := cmd.Start(); err != nil {
		goutils.Err("log.stream: start journalctl", "error", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if e.Publish == nil || e.LogTopic == "" {
			continue
		}
		// Filter noisy redis log lines
		if strings.Contains(strings.ToLower(line), "redis") {
			continue
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"line": line,
			"ts":   time.Now().UnixMilli(),
		})
		if err := e.Publish(e.LogTopic, payload); err != nil {
			goutils.Err("log.stream: publish", "error", err)
		}
	}
	_ = cmd.Wait()
}

func (e *Executor) stopLogStream() {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	if e.logCancel != nil {
		e.logCancel()
		e.logCancel = nil
	}
}

// ApplyDesiredState applies a map of config key-value pairs, returning any
// per-key error strings. Unknown keys are skipped with a logged warning.
func (e *Executor) ApplyDesiredState(state map[string]string) []string {
	var errs []string
	for key, value := range state {
		if err := e.applyConfigKey(key, value); err != nil {
			goutils.Err("desired_state: applying key", "key", key, "error", err)
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
		} else {
			goutils.Log("desired_state: applied", "key", key, "value", value)
		}
	}
	return errs
}
