package message

import (
	"os"
	"time"
)

// Version is injected at build time via ldflags.
var Version string

// Watermark records provenance metadata for a message.
type Watermark struct {
	Timestamp  int64      `json:"timestamp"`
	Hostname   string     `json:"hostname"`
	Type       string     `json:"type,omitempty"`
	AppName    string     `json:"goModule,omitempty"`
	AppVersion string     `json:"goVersion,omitempty"`
	Watermark  *Watermark `json:"watermark,omitempty"`
}

// Message wraps a payload with a Watermark.
type Message struct {
	Payload   interface{} `json:"payload"`
	Watermark Watermark   `json:"watermark"`
}

// NewWatermark creates a Watermark with the current timestamp, hostname,
// optional type, and optional parent watermark.
func NewWatermark(existing *Watermark, msgType string) Watermark {
	hostname, _ := os.Hostname()

	w := Watermark{
		Timestamp: time.Now().UnixMilli(),
		Hostname:  hostname,
	}

	if msgType != "" {
		w.Type = msgType
	}

	if Version != "" {
		w.AppName = "homelab-agent"
		w.AppVersion = Version
	}

	if existing != nil {
		w.Watermark = existing
	}

	return w
}

// NewMessage creates a Message wrapping the given payload with a fresh Watermark.
func NewMessage(payload interface{}, existing *Watermark, msgType string) Message {
	return Message{
		Payload:   payload,
		Watermark: NewWatermark(existing, msgType),
	}
}
