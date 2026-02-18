package mqtt

import (
	"fmt"
	"log"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Client manages an MQTT connection with publish and subscribe capabilities.
type Client struct {
	client    paho.Client
	mu        sync.RWMutex
	connected bool
	onStatus  func(connected bool)
}

// NewClient creates and connects an MQTT client to tcp://{broker}:1883.
// Auto-reconnect is enabled. The onStatus callback fires on connection changes.
// The onConnect callback fires on every (re)connect — use it to re-subscribe and republish retained messages.
func NewClient(broker string, onStatus func(connected bool), onConnect func(*Client)) (*Client, error) {
	c := &Client{
		onStatus: onStatus,
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:1883", broker))
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)

	opts.SetOnConnectHandler(func(_ paho.Client) {
		log.Printf("MQTT connected to %s", broker)
		c.setConnected(true)
		if onConnect != nil {
			onConnect(c)
		}
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
		c.setConnected(false)
	})
	opts.SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
		log.Printf("MQTT reconnecting to %s", broker)
	})

	c.client = paho.NewClient(opts)
	token := c.client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT connect to %s: %w", broker, err)
	}

	return c, nil
}

// Publish sends a message to the given topic with QoS 0 and retain=false.
func (c *Client) Publish(topic string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}
	token := c.client.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

// PublishRetained sends a message to the given topic with QoS 1 and retain=true.
func (c *Client) PublishRetained(topic string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}
	token := c.client.Publish(topic, 1, true, payload)
	token.Wait()
	return token.Error()
}

// Subscribe registers a handler for the given topic with QoS 1.
func (c *Client) Subscribe(topic string, handler paho.MessageHandler) error {
	token := c.client.Subscribe(topic, 1, handler)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	log.Printf("subscribed to %s", topic)
	return nil
}

// IsConnected returns the current connection status.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Disconnect cleanly shuts down the MQTT connection.
func (c *Client) Disconnect() {
	c.client.Disconnect(1000)
	c.setConnected(false)
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
