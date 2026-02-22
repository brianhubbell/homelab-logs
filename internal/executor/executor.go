package executor

import (
	"fmt"
	"strconv"
	"strings"

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
	OnConfigChange     func(key, value string)
	CurrentVersion     string

	// AutoUpdateIntervalChanged is signalled when the auto_update_interval is
	// changed via config.set so the ticker goroutine can reset itself.
	AutoUpdateIntervalChanged chan struct{}

	// ShutdownCh is sent on after a successful self-update so main can
	// publish the MQTT response before exiting and letting the service
	// manager restart with the new binary.
	ShutdownCh chan string
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
		}

	case "config.set":
		key := req.Args["key"]
		value := req.Args["value"]
		if key == "" {
			resp.Status = "error"
			resp.Error = "missing required arg: key"
			return resp
		}
		switch key {
		case "auto_update_interval":
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				e.AutoUpdateInterval = n
			} else {
				resp.Status = "error"
				resp.Error = fmt.Sprintf("invalid interval %q", value)
				return resp
			}
		default:
			resp.Status = "error"
			resp.Error = fmt.Sprintf("unknown config key %q", key)
			return resp
		}
		resp.Status = "ok"
		resp.Data = map[string]interface{}{"key": key, "value": value}

		// Persist the change to disk so it survives restarts.
		if err := config.SaveOverride(key, value); err != nil {
			goutils.Err("persisting config override", "key", key, "error", err)
		}

		// Signal the auto-update goroutine if interval changed.
		if key == "auto_update_interval" {
			select {
			case e.AutoUpdateIntervalChanged <- struct{}{}:
			default:
			}
		}

		if e.OnConfigChange != nil {
			e.OnConfigChange(key, value)
		}

	default:
		resp.Status = "error"
		resp.Error = fmt.Sprintf("unknown action %q", req.Action)
	}

	return resp
}
