package goutils

import (
	"os"
	"time"
)

// Watermark records provenance metadata for a message.
type Watermark struct {
	Timestamp  int64      `json:"timestamp"`
	Hostname   string     `json:"hostname"`
	Type       string     `json:"type,omitempty"`
	AppName    string     `json:"appName,omitempty"`
	AppVersion string     `json:"appVersion,omitempty"`
	Watermark  *Watermark `json:"watermark,omitempty"`
}

// NewWatermark creates a Watermark with the current timestamp, hostname, and
// optional type and existing watermark chain. APP_NAME and APP_VERSION env vars
// are read to populate AppName and AppVersion.
func NewWatermark(existing *Watermark, typ string) *Watermark {
	hostname, _ := os.Hostname()
	w := &Watermark{
		Timestamp: time.Now().UnixMilli(),
		Hostname:  hostname,
	}
	if typ != "" {
		w.Type = typ
	}
	if name := os.Getenv("APP_NAME"); name != "" {
		w.AppName = name
	}
	if version := os.Getenv("APP_VERSION"); version != "" {
		w.AppVersion = version
	}
	if existing != nil {
		w.Watermark = existing
	}
	return w
}
