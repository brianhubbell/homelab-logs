package goutils

import (
	"os"
	"strings"
	"time"
)

// Package-level service identity, set via SetService.
var (
	serviceName    string
	serviceVersion string
)

// SetService sets the service name and version used in watermarks.
// This is the preferred way to identify a service; falls back to
// APP_NAME / APP_VERSION env vars for backward compatibility.
func SetService(name, version string) {
	serviceName = name
	serviceVersion = version
}

// ServiceName returns the service name set via SetService,
// falling back to the APP_NAME env var.
func ServiceName() string {
	if serviceName != "" {
		return serviceName
	}
	return os.Getenv("APP_NAME")
}

// ServiceVersion returns the service version set via SetService,
// falling back to the APP_VERSION env var.
func ServiceVersion() string {
	if serviceVersion != "" {
		return serviceVersion
	}
	return os.Getenv("APP_VERSION")
}

// Watermark records provenance metadata for a message.
type Watermark struct {
	Timestamp      int64      `json:"timestamp"`
	Hostname       string     `json:"hostname"`
	Type           string     `json:"type,omitempty"`
	HostType       string     `json:"hostType,omitempty"`
	ServiceName    string     `json:"serviceName,omitempty"`
	ServiceVersion string     `json:"serviceVersion,omitempty"`
	Watermark      *Watermark `json:"watermark,omitempty"`
}

// NewWatermark creates a Watermark with the current timestamp, hostname, and
// optional type and existing watermark chain.
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
	if ht := os.Getenv("HOST_TYPE"); ht != "" {
		w.HostType = ht
	}
	if name := ServiceName(); name != "" {
		w.ServiceName = name
	}
	if version := ServiceVersion(); version != "" {
		w.ServiceVersion = version
	}
	if existing != nil {
		w.Watermark = existing
	}
	return w
}
