package executor

import (
	"fmt"
	"strings"
)

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

// Executor dispatches commands with whitelist enforcement.
type Executor struct {
	allowedServices     map[string]bool
	allowedComposePaths map[string]bool
	DeployEnabled       bool
	DeployDir           string
	OnWhitelistChange   func([]string)
}

// New creates an Executor with the given allowed services and compose paths.
func New(services []string, composePaths []string) *Executor {
	e := &Executor{
		allowedServices:     make(map[string]bool),
		allowedComposePaths: make(map[string]bool),
	}
	for _, s := range services {
		e.allowedServices[s] = true
	}
	for _, p := range composePaths {
		e.allowedComposePaths[p] = true
	}
	return e
}

// AllowedServicesList returns the current allowed services as a sorted slice.
func (e *Executor) AllowedServicesList() []string {
	var list []string
	for s := range e.allowedServices {
		list = append(list, s)
	}
	return list
}

// Execute runs the given action with args, enforcing the whitelist.
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
		if !e.allowedServices[svc] {
			resp.Status = "error"
			resp.Error = fmt.Sprintf("service %q not in whitelist", svc)
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
		if !e.allowedComposePaths[path] {
			resp.Status = "error"
			resp.Error = fmt.Sprintf("compose path %q not in whitelist", path)
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

	case "service.deploy":
		if !e.DeployEnabled {
			resp.Status = "error"
			resp.Error = "deploy functionality is not enabled"
			return resp
		}
		data, err := e.serviceDeploy(req.Args)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data
		if e.OnWhitelistChange != nil {
			e.OnWhitelistChange(e.AllowedServicesList())
		}

	case "system.metrics":
		data, err := SystemMetrics()
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return resp
		}
		resp.Status = "ok"
		resp.Data = data

	default:
		resp.Status = "error"
		resp.Error = fmt.Sprintf("unknown action %q", req.Action)
	}

	return resp
}
