package mqtt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	goutils "github.com/brianhubbell/go-utils"
	paho "github.com/eclipse/paho.mqtt.golang"
)

// Client wraps a Paho MQTT client with auto-reconnect and metrics
// publishing. It is the transport companion to goutils.Message / goutils.Watermark.
type Client struct {
	client      paho.Client
	hostname    string
	statusTopic string
	started     time.Time
	done       chan struct{}
	once       sync.Once

	mu        sync.RWMutex
	connected bool
	onStatus  func(bool)
}

// NewClient connects to the MQTT broker at the given address (host or host:port).
// onStatus is called whenever the connection state changes.
// onConnect is called after every (re)connect and receives the Client so the
// caller can re-subscribe or publish retained config.
func NewClient(broker string, onStatus func(bool), onConnect func(*Client)) (*Client, error) {
	c := &Client{
		hostname:    shortHostname(),
		statusTopic: fmt.Sprintf("status/%s", shortHostname()),
		started:     time.Now(),
		done:        make(chan struct{}),
		onStatus:    onStatus,
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:1883", broker))
	opts.SetClientID(fmt.Sprintf("%s-%s", goutils.ServiceName(), c.hostname))
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)

	// LWT: broker publishes offline status if the TCP connection is lost.
	lwt := goutils.NewMessage(map[string]any{"status": "offline"}, nil, "status")
	lwtData, _ := json.Marshal(lwt)
	opts.SetWill(c.statusTopic, string(lwtData), 0, true)

	opts.SetOnConnectHandler(func(_ paho.Client) {
		goutils.Log("mqtt connected", "broker", broker)
		c.setConnected(true)
		c.publishStatus("online")
		if onConnect != nil {
			onConnect(c)
		}
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		goutils.Err("mqtt connection lost", "error", err)
		c.setConnected(false)
	})
	opts.SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
		goutils.Debug("mqtt reconnecting", "broker", broker)
	})

	c.client = paho.NewClient(opts)
	token := c.client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect to %s: %w", broker, err)
	}

	c.startStatusRefresh()
	return c, nil
}

// Publish sends payload to the given topic with QoS 0 and retain=false.
func (c *Client) Publish(topic string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("mqtt client not connected")
	}
	token := c.client.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

// PublishRetained sends payload to the given topic with QoS 0 and retain=true.
func (c *Client) PublishRetained(topic string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("mqtt client not connected")
	}
	token := c.client.Publish(topic, 0, true, payload)
	token.Wait()
	return token.Error()
}

// Subscribe registers a handler for the given topic and QoS.
func (c *Client) Subscribe(topic string, qos byte, handler paho.MessageHandler) error {
	if !c.IsConnected() {
		return fmt.Errorf("mqtt client not connected")
	}
	token := c.client.Subscribe(topic, qos, handler)
	token.Wait()
	return token.Error()
}

// IsConnected returns whether the client is currently connected to the broker.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Hostname returns the short hostname used in topic paths.
func (c *Client) Hostname() string {
	return c.hostname
}

// UptimeSeconds returns the number of seconds since the client was created.
func (c *Client) UptimeSeconds() int64 {
	return int64(time.Since(c.started).Seconds())
}

// Done returns a channel that is closed when Stop is called.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// startStatusRefresh launches a goroutine that re-publishes the online status
// every 60 seconds so the Redis TTL is refreshed before it expires.
// The goroutine exits when Stop is called.
func (c *Client) startStatusRefresh() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				if c.IsConnected() {
					c.publishStatus("online")
				}
			}
		}
	}()
}

// Stop gracefully shuts down the client. It is safe to call multiple times.
// It closes the done channel (stopping the status refresh and metrics goroutines),
// publishes a retained offline status, and disconnects from the broker.
func (c *Client) Stop() {
	c.once.Do(func() {
		close(c.done)
		c.publishStatus("offline")
		c.client.Disconnect(1000)
		c.setConnected(false)
	})
}

// publishStatus publishes a retained online/offline status message.
func (c *Client) publishStatus(status string) {
	envelope := goutils.NewMessage(map[string]any{"status": status}, nil, "status")
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	token := c.client.Publish(c.statusTopic, 0, true, data)
	token.Wait()
}

func (c *Client) setConnected(connected bool) {
	c.mu.Lock()
	c.connected = connected
	cb := c.onStatus
	c.mu.Unlock()
	if cb != nil {
		cb(connected)
	}
}

func shortHostname() string {
	hostname, _ := os.Hostname()
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}
	return hostname
}
