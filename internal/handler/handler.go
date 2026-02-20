package handler

import (
	"encoding/json"
	"time"

	goutils "github.com/brianhubbell/go-utils"
	paho "github.com/eclipse/paho.mqtt.golang"

	"homelab-agent/internal/executor"
	"homelab-agent/internal/health"
	"homelab-agent/internal/mqtt"
)

// Handler processes incoming MQTT commands, dispatches them to the executor,
// and publishes responses.
type Handler struct {
	exec          *executor.Executor
	client        *mqtt.Client
	met           *health.Server
	responseTopic string
}

// New creates a Handler.
func New(exec *executor.Executor, client *mqtt.Client, met *health.Server, responseTopic string) *Handler {
	return &Handler{
		exec:          exec,
		client:        client,
		met:           met,
		responseTopic: responseTopic,
	}
}

// HandleMessage is the MQTT message callback for the command topic.
func (h *Handler) HandleMessage(_ paho.Client, msg paho.Message) {
	h.met.IncReceived()

	var req executor.Request
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		goutils.Err("invalid command payload", "error", err)
		h.met.IncFailed()
		return
	}

	if req.ID == "" || req.Action == "" {
		goutils.Err("command missing id or action")
		h.met.IncFailed()
		return
	}

	goutils.Debug("command received", "id", req.ID, "action", req.Action, "args", req.Args)

	start := time.Now()
	resp := h.exec.Execute(req)
	resp.DurationMs = time.Since(start).Milliseconds()

	if resp.Status == "ok" {
		h.met.IncExecuted()
	} else {
		h.met.IncFailed()
	}

	envelope := goutils.NewMessage(resp, nil, "response")
	payload, err := json.Marshal(envelope)
	if err != nil {
		goutils.Err("marshal response", "error", err)
		return
	}

	if err := h.client.Publish(h.responseTopic, payload); err != nil {
		goutils.Err("publish response", "error", err)
	}

	goutils.Debug("response sent", "id", resp.ID, "status", resp.Status, "duration_ms", resp.DurationMs)
}
