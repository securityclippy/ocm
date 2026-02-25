// Device pairing management via OpenClaw WebSocket RPC

package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// PendingDevice represents a device waiting for pairing approval.
type PendingDevice struct {
	RequestID string `json:"requestId"`
	DeviceID  string `json:"deviceId"`
	Role      string `json:"role"`
	Origin    string `json:"origin"`
	UserAgent string `json:"userAgent"`
	CreatedAt int64  `json:"createdAt"`
}

// PairedDevice represents an approved device.
type PairedDevice struct {
	DeviceID  string `json:"deviceId"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"createdAt"`
}

// DeviceListResponse is the response from device.pair.list.
type DeviceListResponse struct {
	Pending []PendingDevice `json:"pending"`
	Paired  []PairedDevice  `json:"paired"`
}

// rpcMessage is the WebSocket RPC message format.
type rpcMessage struct {
	ID     int64       `json:"id"`
	Method string      `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RPCClient is a WebSocket RPC client for OpenClaw Gateway.
type RPCClient struct {
	gatewayURL string
	token      string
	conn       *websocket.Conn
	mu         sync.Mutex
	nextID     int64
	pending    map[int64]chan *rpcMessage
	pendingMu  sync.Mutex
	connected  bool
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(gatewayURL, token string) *RPCClient {
	return &RPCClient{
		gatewayURL: gatewayURL,
		token:      token,
		pending:    make(map[int64]chan *rpcMessage),
	}
}

// Connect establishes the WebSocket connection.
func (c *RPCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(c.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}

	// Add auth token as query param
	q := u.Query()
	q.Set("token", c.token)
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), http.Header{})
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Start reading responses
	go c.readLoop()

	// Send connect message
	connectMsg := map[string]interface{}{
		"method": "connect",
		"params": map[string]interface{}{
			"auth": map[string]string{
				"token": c.token,
			},
		},
	}
	if err := c.conn.WriteJSON(connectMsg); err != nil {
		c.conn.Close()
		c.connected = false
		return fmt.Errorf("connect message failed: %w", err)
	}

	return nil
}

// Close closes the WebSocket connection.
func (c *RPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.connected = false
		return c.conn.Close()
	}
	return nil
}

// readLoop reads messages from the WebSocket.
func (c *RPCClient) readLoop() {
	for {
		var msg rpcMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return
		}

		// Route response to waiting caller
		c.pendingMu.Lock()
		if ch, ok := c.pending[msg.ID]; ok {
			ch <- &msg
			delete(c.pending, msg.ID)
		}
		c.pendingMu.Unlock()
	}
}

// call makes an RPC call and waits for response.
func (c *RPCClient) call(method string, params interface{}) (*rpcMessage, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan *rpcMessage, 1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	msg := rpcMessage{
		ID:     id,
		Method: method,
		Params: params,
	}

	c.mu.Lock()
	err := c.conn.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write failed: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(30 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// ListDevices returns pending and paired devices.
func (c *RPCClient) ListDevices() (*DeviceListResponse, error) {
	resp, err := c.call("device.pair.list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", resp.Error.Message)
	}

	// Parse result
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var result DeviceListResponse
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &result, nil
}

// ApproveDevice approves a pending device pairing request.
func (c *RPCClient) ApproveDevice(requestID string) error {
	resp, err := c.call("device.pair.approve", map[string]string{
		"requestId": requestID,
	})
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("RPC error: %s", resp.Error.Message)
	}
	return nil
}

// RejectDevice rejects a pending device pairing request.
func (c *RPCClient) RejectDevice(requestID string) error {
	resp, err := c.call("device.pair.reject", map[string]string{
		"requestId": requestID,
	})
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("RPC error: %s", resp.Error.Message)
	}
	return nil
}
