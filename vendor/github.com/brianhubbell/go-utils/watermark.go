package goutils

import (
	"os"
	"strings"
	"time"
)

// Watermark records provenance metadata for a message.
type Watermark struct {
	Timestamp  int64      `json:"timestamp"`
	Hostname   string     `json:"hostname"`
	Type       string     `json:"type,omitempty"`
	HostType   string     `json:"host_type,omitempty"`
	ServiceName    string     `json:"serviceName,omitempty"`
	ServiceVersion string     `json:"serviceVersion,omitempty"`
	Watermark  *Watermark `json:"watermark,omitempty"`
}

// NewWatermark creates a Watermark with the current timestamp, hostname, and
// optional type and existing watermark chain. APP_NAME, APP_VERSION, and
// HOST_TYPE env vars are read to populate the corresponding fields.
func NewWatermark(existing *Watermark, typ string) *Watermark {
	hostname, _ := os.Hostname()
	// Sanitize hostname: strip null bytes and non-printable characters
	hostname = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, hostname)
	w := &Watermark{
		Timestamp: time.Now().UnixMilli(),
		Hostname:  hostname,
	}
	if typ != "" {
		w.Type = typ
	}
	if hostType := os.Getenv("HOST_TYPE"); hostType != "" {
		w.HostType = hostType
	}
	if name := os.Getenv("APP_NAME"); name != "" {
		w.ServiceName = name
	}
	if version := os.Getenv("APP_VERSION"); version != "" {
		w.ServiceVersion = version
	}
	if existing != nil {
		w.Watermark = existing
	}
	return w
}
