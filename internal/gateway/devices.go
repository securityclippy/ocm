// Device pairing management via OpenClaw WebSocket RPC

package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// deviceIdentity holds the Ed25519 keypair for device auth.
type deviceIdentity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	DeviceID   string // SHA256 fingerprint of public key
}

// RPCClient is a WebSocket RPC client for OpenClaw Gateway.
type RPCClient struct {
	gatewayURL string
	token      string
	identity   *deviceIdentity
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
	identity, err := loadOrCreateIdentity()
	if err != nil {
		// Log but continue - will fail on connect if identity is required
		fmt.Fprintf(os.Stderr, "warning: failed to load device identity: %v\n", err)
	}

	return &RPCClient{
		gatewayURL: gatewayURL,
		token:      token,
		identity:   identity,
		pending:    make(map[string]chan *rpcMessage),
	}
}

// loadOrCreateIdentity loads or creates an Ed25519 keypair for device identity.
func loadOrCreateIdentity() (*deviceIdentity, error) {
	// Store identity in /data (mounted volume) or fallback to temp
	keyPath := "/data/ocm-device.key"
	if _, err := os.Stat("/data"); os.IsNotExist(err) {
		keyPath = filepath.Join(os.TempDir(), "ocm-device.key")
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == ed25519.SeedSize {
		privateKey := ed25519.NewKeyFromSeed(data)
		publicKey := privateKey.Public().(ed25519.PublicKey)
		deviceID := computeDeviceID(publicKey)
		return &deviceIdentity{
			PrivateKey: privateKey,
			PublicKey:  publicKey,
			DeviceID:   deviceID,
		}, nil
	}

	// Generate new keypair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	// Save seed for persistence
	seed := privateKey.Seed()
	if err := os.WriteFile(keyPath, seed, 0600); err != nil {
		// Non-fatal - just won't persist across restarts
		fmt.Fprintf(os.Stderr, "warning: failed to save device identity: %v\n", err)
	}

	deviceID := computeDeviceID(publicKey)
	return &deviceIdentity{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		DeviceID:   deviceID,
	}, nil
}

// computeDeviceID returns the SHA256 fingerprint of the public key.
func computeDeviceID(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)
	return hex.EncodeToString(hash[:16]) // First 16 bytes = 32 hex chars
}

// signPayload signs a payload for device auth.
func (id *deviceIdentity) signPayload(payload string) string {
	signature := ed25519.Sign(id.PrivateKey, []byte(payload))
	return base64.StdEncoding.EncodeToString(signature)
}

// buildAuthPayload builds the payload to sign for device auth.
// Format: clientId:clientMode:role:scopes:signedAt:token:nonce
func buildAuthPayload(clientID, clientMode, role string, scopes []string, signedAt int64, token, nonce string) string {
	scopesStr := ""
	for i, s := range scopes {
		if i > 0 {
			scopesStr += ","
		}
		scopesStr += s
	}
	return fmt.Sprintf("%s:%s:%s:%s:%d:%s:%s", clientID, clientMode, role, scopesStr, signedAt, token, nonce)
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

	// Extract nonce from challenge
	var challenge connectChallenge
	if payload, ok := challengeMsg.Payload.(map[string]interface{}); ok {
		if nonce, ok := payload["nonce"].(string); ok {
			challenge.Nonce = nonce
		}
	}

	// Build connect params
	clientID := "cli"
	clientMode := "cli"
	role := "operator"
	scopes := []string{"operator.admin"}
	signedAt := time.Now().UnixMilli()

	connectParams := map[string]interface{}{
		"minProtocol": 3,
		"maxProtocol": 3,
		"client": map[string]interface{}{
			"id":       clientID,
			"version":  "0.1.0",
			"platform": "linux",
			"mode":     clientMode,
		},
		"role":      role,
		"scopes":    scopes,
		"caps":      []string{},
		"userAgent": "ocm/0.1.0",
	}

	// Add auth
	if c.token != "" {
		connectParams["auth"] = map[string]string{
			"token": c.token,
		}
	}

	// Add device identity if available
	if c.identity != nil {
		payload := buildAuthPayload(clientID, clientMode, role, scopes, signedAt, c.token, challenge.Nonce)
		signature := c.identity.signPayload(payload)

		connectParams["device"] = map[string]interface{}{
			"id":        c.identity.DeviceID,
			"publicKey": base64.StdEncoding.EncodeToString(c.identity.PublicKey),
			"signature": signature,
			"signedAt":  signedAt,
			"nonce":     challenge.Nonce,
		}
	}

	// Send connect request
	connectReq := rpcMessage{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: connectParams,
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

// GetDeviceID returns OCM's device ID for pairing approval.
func (c *RPCClient) GetDeviceID() string {
	if c.identity != nil {
		return c.identity.DeviceID
	}
	return ""
}
