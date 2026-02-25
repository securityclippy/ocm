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
	Type    string      `json:"type"`              // "req", "res", "event"
	ID      string      `json:"id,omitempty"`      // request/response correlation (string!)
	Method  string      `json:"method,omitempty"`  // for requests
	Params  interface{} `json:"params,omitempty"`  // for requests
	Event   string      `json:"event,omitempty"`   // for events
	Payload interface{} `json:"payload,omitempty"` // for events/responses
	OK      *bool       `json:"ok,omitempty"`      // for responses
	Error   *rpcError   `json:"error,omitempty"`   // for error responses
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// connectChallenge is sent by Gateway before connect.
type connectChallenge struct {
	Nonce string `json:"nonce"`
	Ts    int64  `json:"ts"`
}

// RPCClient is a WebSocket RPC client for OpenClaw Gateway.
type RPCClient struct {
	gatewayURL string
	token      string
	conn       *websocket.Conn
	mu         sync.Mutex
	nextID     uint64
	pending    map[string]chan *rpcMessage
	pendingMu  sync.Mutex
	connected  bool
	readDone   chan struct{}
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(gatewayURL, token string) *RPCClient {
	return &RPCClient{
		gatewayURL: gatewayURL,
		token:      token,
		pending:    make(map[string]chan *rpcMessage),
	}
}

// Connect establishes the WebSocket connection following OpenClaw protocol.
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

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), http.Header{})
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.conn = conn
	c.readDone = make(chan struct{})

	// Wait for connect.challenge event from Gateway
	var challengeMsg rpcMessage
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&challengeMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read challenge: %w", err)
	}
	conn.SetReadDeadline(time.Time{}) // Clear deadline

	if challengeMsg.Type != "event" || challengeMsg.Event != "connect.challenge" {
		conn.Close()
		return fmt.Errorf("expected connect.challenge event, got type=%s event=%s", challengeMsg.Type, challengeMsg.Event)
	}

	// Send connect request with proper protocol structure
	// Valid client IDs: cli, gateway-client, webchat, etc.
	// Valid modes: cli, backend, ui, node, etc.
	connectReq := rpcMessage{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: map[string]interface{}{
			"minProtocol": 3,
			"maxProtocol": 3,
			"client": map[string]interface{}{
				"id":       "cli",
				"version":  "0.1.0",
				"platform": "linux",
				"mode":     "cli",
			},
			"role":   "operator",
			"scopes": []string{"operator.read", "operator.pairing"},
			"caps":   []string{},
			"auth": map[string]string{
				"token": c.token,
			},
			"userAgent": "ocm/0.1.0",
		},
	}

	if err := conn.WriteJSON(connectReq); err != nil {
		conn.Close()
		return fmt.Errorf("connect request failed: %w", err)
	}

	// Wait for hello-ok response
	var helloMsg rpcMessage
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&helloMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read hello response: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	if helloMsg.Type != "res" {
		conn.Close()
		return fmt.Errorf("expected response, got type=%s", helloMsg.Type)
	}

	if helloMsg.OK == nil || !*helloMsg.OK {
		errMsg := "unknown error"
		if helloMsg.Error != nil {
			errMsg = helloMsg.Error.Message
		}
		conn.Close()
		return fmt.Errorf("connect rejected: %s", errMsg)
	}

	c.connected = true
	c.nextID = 1 // Start from 2 for subsequent calls (1 used for connect)

	// Start reading responses
	go c.readLoop()

	return nil
}

// Close closes the WebSocket connection.
func (c *RPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.connected = false
		err := c.conn.Close()
		if c.readDone != nil {
			<-c.readDone // Wait for read loop to exit
		}
		return err
	}
	return nil
}

// readLoop reads messages from the WebSocket.
func (c *RPCClient) readLoop() {
	defer close(c.readDone)
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
		if msg.Type == "res" && msg.ID != "" {
			c.pendingMu.Lock()
			if ch, ok := c.pending[msg.ID]; ok {
				ch <- &msg
				delete(c.pending, msg.ID)
			}
			c.pendingMu.Unlock()
		}
		// Ignore events for now (could handle node.pair.requested etc.)
	}
}

// call makes an RPC call and waits for response.
func (c *RPCClient) call(method string, params interface{}) (*rpcMessage, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	id := fmt.Sprintf("%d", atomic.AddUint64(&c.nextID, 1))
	ch := make(chan *rpcMessage, 1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	msg := rpcMessage{
		Type:   "req",
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
	if resp.OK != nil && !*resp.OK {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return nil, fmt.Errorf("RPC error: %s", errMsg)
	}

	// Parse result from payload
	resultBytes, err := json.Marshal(resp.Payload)
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
	if resp.OK != nil && !*resp.OK {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return fmt.Errorf("RPC error: %s", errMsg)
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
	if resp.OK != nil && !*resp.OK {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return fmt.Errorf("RPC error: %s", errMsg)
	}
	return nil
}
