package handler

import (
	"encoding/json"
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"homelab-agent/internal/executor"
	"homelab-agent/internal/message"
	"homelab-agent/internal/metrics"
	"homelab-agent/internal/mqtt"
)

// Handler processes incoming MQTT commands, dispatches them to the executor,
// and publishes responses.
type Handler struct {
	exec          *executor.Executor
	client        *mqtt.Client
	met           *metrics.Server
	responseTopic string
	debug         bool
}

// New creates a Handler.
func New(exec *executor.Executor, client *mqtt.Client, met *metrics.Server, responseTopic string, debug bool) *Handler {
	return &Handler{
		exec:          exec,
		client:        client,
		met:           met,
		responseTopic: responseTopic,
		debug:         debug,
	}
}

// HandleMessage is the MQTT message callback for the command topic.
func (h *Handler) HandleMessage(_ paho.Client, msg paho.Message) {
	h.met.IncReceived()

	var req executor.Request
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		log.Printf("ERROR invalid command payload: %v", err)
		h.met.IncFailed()
		return
	}

	if req.ID == "" || req.Action == "" {
		log.Printf("ERROR command missing id or action")
		h.met.IncFailed()
		return
	}

	if h.debug {
		log.Printf("DEBUG command id=%s action=%s args=%v", req.ID, req.Action, req.Args)
	}

	start := time.Now()
	resp := h.exec.Execute(req)
	resp.DurationMs = time.Since(start).Milliseconds()

	if resp.Status == "ok" {
		h.met.IncExecuted()
	} else {
		h.met.IncFailed()
	}

	envelope := message.NewMessage(resp, nil, "response")
	payload, err := json.Marshal(envelope)
	if err != nil {
		log.Printf("ERROR marshal response: %v", err)
		return
	}

	if err := h.client.Publish(h.responseTopic, payload); err != nil {
		log.Printf("ERROR publish response: %v", err)
	}

	if h.debug {
		log.Printf("DEBUG response id=%s status=%s duration=%dms", resp.ID, resp.Status, resp.DurationMs)
	}
}
