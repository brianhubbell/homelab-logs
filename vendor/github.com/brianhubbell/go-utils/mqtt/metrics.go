package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	goutils "github.com/brianhubbell/go-utils"
)

// StartMetrics launches a goroutine that publishes metrics at the given
// interval to metrics/{ServiceName}/{hostname}.
//
// collect is called each tick with the client's uptime in seconds.  It should
// return a value to publish as the message payload.  Return nil to skip a tick
// (e.g. when a collector encounters an error).
//
// The goroutine exits when Stop is called.
func (c *Client) StartMetrics(interval time.Duration, collect func(uptimeSeconds int64) any) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		topic := fmt.Sprintf("metrics/%s/%s", goutils.ServiceName(), c.hostname)

		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				payload := collect(c.UptimeSeconds())
				if payload == nil {
					continue
				}
				envelope := goutils.NewMessage(payload, nil, "metrics")
				data, err := json.Marshal(envelope)
				if err != nil {
					goutils.Err("mqtt metrics marshal error", "error", err)
					continue
				}
				if err := c.Publish(topic, data); err != nil {
					goutils.Debug("mqtt metrics publish skipped", "error", err)
				}
			}
		}
	}()
}
