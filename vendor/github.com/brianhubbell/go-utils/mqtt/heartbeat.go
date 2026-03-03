package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	goutils "github.com/brianhubbell/go-utils"
)

// StartHeartbeat launches a goroutine that publishes a retained heartbeat at
// the given interval to heartbeat/{ServiceName}/{hostname}.
//
// An initial heartbeat is published immediately.  On Stop, a final offline
// heartbeat is published (see Client.Stop).
//
// The goroutine exits when Stop is called.
func (c *Client) StartHeartbeat(interval time.Duration) {
	go func() {
		topic := fmt.Sprintf("heartbeat/%s/%s", goutils.ServiceName(), c.hostname)

		publish := func() {
			payload := map[string]any{
				"uptime_seconds": c.UptimeSeconds(),
				"status":         "online",
			}
			envelope := goutils.NewMessage(payload, nil, "heartbeat")
			data, err := json.Marshal(envelope)
			if err != nil {
				goutils.Err("mqtt heartbeat marshal error", "error", err)
				return
			}
			if err := c.PublishRetained(topic, data); err != nil {
				goutils.Debug("mqtt heartbeat publish skipped", "error", err)
			}
		}

		// Publish immediately on start.
		publish()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				publish()
			}
		}
	}()
}
