package client

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var ErrClientClosed = errors.New("client is closed")

// Client sends bundle requests to a bundle-broadcaster WebSocket endpoint.
// It connects on creation and reconnects automatically when the connection is closed.
type Client struct {
	url    string
	conn   *websocket.Conn
	closed bool
	mu     sync.Mutex
	dialer *websocket.Dialer
}

// New creates a client for the given WebSocket URL (e.g. "ws://localhost:8585/ws"),
// stores the URL, and establishes the connection immediately.
func New(wsURL string) (*Client, error) {
	c := &Client{
		url: wsURL,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// connect establishes the WebSocket connection. Does nothing if Close was called.
func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.conn != nil {
		return nil
	}
	conn, _, err := c.dialer.Dial(c.url, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// disconnect closes the connection and clears it so the next Send will reconnect.
func (c *Client) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Send sends a bundle request to the broadcaster.
// If the connection is closed, it reconnects automatically and retries.
func (c *Client) Send(b *BundleRequest) error {
	msg, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return c.sendBytes(msg)
}

func (c *Client) sendBytes(msg []byte) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.mu.Lock()
		closed := c.closed
		conn := c.conn
		c.mu.Unlock()

		if closed {
			return ErrClientClosed
		}
		if conn == nil {
			if err := c.connect(); err != nil {
				return err
			}
			c.mu.Lock()
			conn = c.conn
			c.mu.Unlock()
		}

		lastErr = conn.WriteMessage(websocket.TextMessage, msg)
		if lastErr == nil {
			return nil
		}

		c.disconnect()
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return lastErr
		}
		c.mu.Unlock()
	}
	return lastErr
}

// Close closes the WebSocket connection and prevents reconnection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
