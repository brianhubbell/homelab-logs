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
	HostType       string `json:"hostType,omitempty"`
	ServiceType    string `json:"serviceType,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
	ServiceVersion string `json:"serviceVersion,omitempty"`
	Watermark  *Watermark `json:"watermark,omitempty"`
}

// NewWatermark creates a Watermark with the current timestamp, hostname, and
// optional type and existing watermark chain. SERVICE_NAME and SERVICE_VERSION
// env vars are read to populate ServiceName and ServiceVersion.
func NewWatermark(existing *Watermark, typ string) *Watermark {
	hostname, _ := os.Hostname()
	w := &Watermark{
		Timestamp: time.Now().UnixMilli(),
		Hostname:  hostname,
	}
	if typ != "" {
		w.Type = typ
	}
	if ht := os.Getenv("HOST_TYPE"); ht != "" {
		w.HostType = ht
	}
	if st := os.Getenv("SERVICE_TYPE"); st != "" {
		w.ServiceType = st
	}
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		w.ServiceName = name
	}
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		w.ServiceVersion = version
	}
	if existing != nil {
		w.Watermark = existing
	}
	return w
}
