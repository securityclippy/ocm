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
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	gatewayURL       string
	token            string
	identity         *deviceIdentity
	conn             *websocket.Conn
	mu               sync.Mutex   // Protects conn, nextID
	nextID           uint64
	pending          map[string]chan *rpcMessage
	pendingMu        sync.Mutex
	statusMu         sync.RWMutex // Protects status fields (separate to avoid blocking on Connect)
	connected        bool
	needsPairing     bool   // True if last connect failed due to pairing requirement
	tokenMismatch    bool   // True if last connect failed due to token mismatch
	pendingRequestID string // Request ID for pending pairing, if known
	readDone         chan struct{}
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(gatewayURL, token string) *RPCClient {
	identity, err := loadOrCreateIdentity()
	if err != nil {
		// Log but continue - will fail on connect if identity is required
		fmt.Fprintf(os.Stderr, "warning: failed to load device identity: %v\n", err)
	}

	client := &RPCClient{
		gatewayURL: gatewayURL,
		token:      token,
		identity:   identity,
		pending:    make(map[string]chan *rpcMessage),
	}

	// Start auto-connect in background
	go client.autoConnect()

	return client
}

// autoConnect attempts to connect on startup and retries until successful.
func (c *RPCClient) autoConnect() {
	retryInterval := 5 * time.Second
	maxRetries := 60 // Try for 5 minutes
	pairingInstructionsShown := false

	for i := 0; i < maxRetries; i++ {
		if c.IsConnected() {
			fmt.Fprintf(os.Stderr, "INFO: gateway RPC connected successfully\n")
			return
		}

		err := c.Connect()
		if err == nil {
			fmt.Fprintf(os.Stderr, "INFO: gateway RPC connected successfully\n")
			return
		}

		errStr := err.Error()
		
		// Check if pairing is required and show helpful instructions (once)
		if strings.Contains(errStr, "pairing required") && !pairingInstructionsShown {
			pairingInstructionsShown = true
			deviceID := c.GetDeviceID()
			fmt.Fprintf(os.Stderr, "\n")
			fmt.Fprintf(os.Stderr, "╔══════════════════════════════════════════════════════════════════╗\n")
			fmt.Fprintf(os.Stderr, "║  🔐 OCM Device Pairing Required                                  ║\n")
			fmt.Fprintf(os.Stderr, "╠══════════════════════════════════════════════════════════════════╣\n")
			fmt.Fprintf(os.Stderr, "║  OCM needs to be approved to connect to OpenClaw.                ║\n")
			fmt.Fprintf(os.Stderr, "║                                                                  ║\n")
			fmt.Fprintf(os.Stderr, "║  Run this command to approve:                                    ║\n")
			fmt.Fprintf(os.Stderr, "║                                                                  ║\n")
			fmt.Fprintf(os.Stderr, "║    docker exec -it openclaw node /app/dist/index.js devices list ║\n")
			fmt.Fprintf(os.Stderr, "║                                                                  ║\n")
			fmt.Fprintf(os.Stderr, "║  Then approve the pending request for device:                    ║\n")
			fmt.Fprintf(os.Stderr, "║    %.60s...  ║\n", deviceID)
			fmt.Fprintf(os.Stderr, "║                                                                  ║\n")
			fmt.Fprintf(os.Stderr, "║    docker exec -it openclaw node /app/dist/index.js devices      ║\n")
			fmt.Fprintf(os.Stderr, "║      approve <requestId>                                         ║\n")
			fmt.Fprintf(os.Stderr, "║                                                                  ║\n")
			fmt.Fprintf(os.Stderr, "║  Waiting for approval...                                         ║\n")
			fmt.Fprintf(os.Stderr, "╚══════════════════════════════════════════════════════════════════╝\n")
			fmt.Fprintf(os.Stderr, "\n")
		} else if !strings.Contains(errStr, "pairing required") {
			// Only log non-pairing errors (pairing just needs user action)
			fmt.Fprintf(os.Stderr, "INFO: gateway RPC connect attempt %d failed: %v\n", i+1, err)
		}

		time.Sleep(retryInterval)
	}

	fmt.Fprintf(os.Stderr, "ERROR: gateway RPC failed to connect after %d attempts\n", maxRetries)
}

// loadOrCreateIdentity loads or creates an Ed25519 keypair for device identity.
func loadOrCreateIdentity() (*deviceIdentity, error) {
	// Store identity in /data (mounted volume) or fallback to temp
	keyPath := "/data/ocm-device.key"
	if _, err := os.Stat("/data"); os.IsNotExist(err) {
		keyPath = filepath.Join(os.TempDir(), "ocm-device.key")
		fmt.Fprintf(os.Stderr, "INFO: /data not found, using temp path: %s\n", keyPath)
	} else {
		fmt.Fprintf(os.Stderr, "INFO: using key path: %s\n", keyPath)
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == ed25519.SeedSize {
		privateKey := ed25519.NewKeyFromSeed(data)
		publicKey := privateKey.Public().(ed25519.PublicKey)
		deviceID := computeDeviceID(publicKey)
		fmt.Fprintf(os.Stderr, "INFO: loaded existing device identity: %s\n", deviceID)
		return &deviceIdentity{
			PrivateKey: privateKey,
			PublicKey:  publicKey,
			DeviceID:   deviceID,
		}, nil
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "INFO: no existing key found (%v), generating new\n", err)
	}

	// Generate new keypair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	// Save seed for persistence
	seed := privateKey.Seed()
	if err := os.WriteFile(keyPath, seed, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to save device identity to %s: %v\n", keyPath, err)
	} else {
		fmt.Fprintf(os.Stderr, "INFO: saved new device identity to %s\n", keyPath)
	}

	deviceID := computeDeviceID(publicKey)
	fmt.Fprintf(os.Stderr, "INFO: generated new device identity: %s\n", deviceID)
	return &deviceIdentity{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		DeviceID:   deviceID,
	}, nil
}

// computeDeviceID returns the SHA256 fingerprint of the public key.
func computeDeviceID(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)
	return hex.EncodeToString(hash[:]) // Full 32 bytes = 64 hex chars
}

// sign signs a payload for device auth and returns raw bytes.
func (id *deviceIdentity) sign(payload string) []byte {
	return ed25519.Sign(id.PrivateKey, []byte(payload))
}

// buildAuthPayload builds the payload to sign for device auth.
// v2 format: v2|deviceId|clientId|clientMode|role|scopes|signedAtMs|token|nonce
func buildAuthPayload(deviceID, clientID, clientMode, role string, scopes []string, signedAtMs int64, token, nonce string) string {
	scopesStr := ""
	for i, s := range scopes {
		if i > 0 {
			scopesStr += ","
		}
		scopesStr += s
	}
	// v2 format with nonce
	return fmt.Sprintf("v2|%s|%s|%s|%s|%s|%d|%s|%s", deviceID, clientID, clientMode, role, scopesStr, signedAtMs, token, nonce)
}

// Connect establishes the WebSocket connection following OpenClaw protocol.
func (c *RPCClient) Connect() error {
	// Quick check if already connected (don't block on mu)
	c.statusMu.RLock()
	if c.connected {
		c.statusMu.RUnlock()
		return nil
	}
	c.statusMu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

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
		payload := buildAuthPayload(c.identity.DeviceID, clientID, clientMode, role, scopes, signedAt, c.token, challenge.Nonce)
		signature := c.identity.sign(payload)
		pubKeyB64 := base64.RawURLEncoding.EncodeToString(c.identity.PublicKey)
		sigB64 := base64.RawURLEncoding.EncodeToString(signature)

		// Debug logging via slog
		slog.Info("device auth debug",
			"payload", payload,
			"deviceId", c.identity.DeviceID,
			"publicKey", pubKeyB64,
			"signedAt", signedAt,
			"nonce", challenge.Nonce,
			"token", c.token,
		)

		// OpenClaw expects base64url encoding for public key and signature
		connectParams["device"] = map[string]interface{}{
			"id":        c.identity.DeviceID,
			"publicKey": pubKeyB64,
			"signature": sigB64,
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
		
		// Track connection status for UI
		c.statusMu.Lock()
		if strings.Contains(errMsg, "pairing required") {
			c.needsPairing = true
			c.tokenMismatch = false
			// Try to extract requestId from error details if available
			if helloMsg.Error != nil {
				if payload, ok := helloMsg.Payload.(map[string]interface{}); ok {
					if reqID, ok := payload["requestId"].(string); ok {
						c.pendingRequestID = reqID
					}
				}
			}
		} else if strings.Contains(errMsg, "token mismatch") || strings.Contains(errMsg, "unauthorized") {
			c.tokenMismatch = true
			c.needsPairing = false
		}
		c.statusMu.Unlock()
		
		return fmt.Errorf("connect rejected: %s", errMsg)
	}

	// Clear error flags and set connected on success
	c.statusMu.Lock()
	c.needsPairing = false
	c.tokenMismatch = false
	c.pendingRequestID = ""
	c.connected = true
	c.statusMu.Unlock()
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
		c.statusMu.Lock()
		c.connected = false
		c.statusMu.Unlock()
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
			c.statusMu.Lock()
			c.connected = false
			c.statusMu.Unlock()
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
	c.statusMu.RLock()
	connected := c.connected
	c.statusMu.RUnlock()
	
	if !connected {
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

// IsConnected returns whether the RPC client is connected to Gateway.
func (c *RPCClient) IsConnected() bool {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.connected
}

// NeedsPairing returns true if the last connection attempt failed due to pairing requirement.
func (c *RPCClient) NeedsPairing() bool {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.needsPairing
}

// TokenMismatch returns true if the last connection attempt failed due to token mismatch.
func (c *RPCClient) TokenMismatch() bool {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.tokenMismatch
}

// GetPendingRequestID returns the request ID for the pending pairing request, if known.
func (c *RPCClient) GetPendingRequestID() string {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.pendingRequestID
}

// ErrRestartDisabled is returned when Gateway restart is not enabled in OpenClaw config.
var ErrRestartDisabled = fmt.Errorf("gateway restart disabled")

// ErrRateLimited is returned when config.patch is rate limited.
// The RetryAfter field indicates how long to wait before retrying.
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("rate limited; retry after %v", e.RetryAfter)
}

// parseRateLimitError checks if an error message indicates rate limiting
// and extracts the retry-after duration if present.
// Format: "rate limit exceeded for config.patch; retry after 59s"
func parseRateLimitError(errMsg string) *ErrRateLimited {
	if !strings.Contains(errMsg, "rate limit") {
		return nil
	}
	
	// Try to extract "retry after Ns" pattern
	idx := strings.Index(errMsg, "retry after ")
	if idx == -1 {
		return &ErrRateLimited{RetryAfter: 60 * time.Second} // Default 60s
	}
	
	afterPart := errMsg[idx+len("retry after "):]
	// Parse number followed by 's'
	var seconds int
	_, err := fmt.Sscanf(afterPart, "%ds", &seconds)
	if err != nil || seconds <= 0 {
		return &ErrRateLimited{RetryAfter: 60 * time.Second}
	}
	
	return &ErrRateLimited{RetryAfter: time.Duration(seconds) * time.Second}
}

// ErrConfigFileLocked is returned when the config file is locked (EBUSY on WSL2/Windows).
// This is typically caused by file watchers and requires fixing in OpenClaw itself.
var ErrConfigFileLocked = fmt.Errorf("config file locked (EBUSY)")

// RestartGateway triggers an OpenClaw Gateway restart via RPC.
// Uses config.patch with a no-op patch to trigger restart.
// The reason/note is logged by the Gateway.
// Returns ErrRateLimited if rate limited, ErrConfigFileLocked if file is locked.
func (c *RPCClient) RestartGateway(reason string) error {
	err := c.tryRestartGateway(reason)
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Check for rate limiting
	if rl := parseRateLimitError(errStr); rl != nil {
		return rl
	}
	
	// Check for EBUSY - don't retry, this is a persistent lock on WSL2/Windows
	// Retrying just burns through rate limit tokens without helping
	if strings.Contains(errStr, "EBUSY") || strings.Contains(errStr, "resource busy") {
		return ErrConfigFileLocked
	}
	
	return err
}

// PatchConfig patches the OpenClaw config file with the given JSON5 content.
// This is used for injecting credentials that go into config (not .env).
// The patch is merged with the existing config using JSON merge patch semantics.
// Returns the new config hash on success.
func (c *RPCClient) PatchConfig(patch string, reason string) (string, error) {
	// First, get current config to get the baseHash
	getResp, err := c.call("config.get", map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("config.get failed: %w", err)
	}
	if getResp.OK != nil && !*getResp.OK {
		errMsg := "unknown error"
		if getResp.Error != nil {
			errMsg = getResp.Error.Message
		}
		return "", fmt.Errorf("config.get error: %s", errMsg)
	}

	// Extract baseHash from response
	var baseHash string
	if payload, ok := getResp.Payload.(map[string]interface{}); ok {
		if hash, ok := payload["hash"].(string); ok {
			baseHash = hash
		}
	}

	// Call config.patch
	resp, err := c.call("config.patch", map[string]interface{}{
		"raw":            patch,
		"baseHash":       baseHash,
		"note":           reason,
		"restartDelayMs": 1000,
	})
	if err != nil {
		return "", err
	}
	if resp.OK != nil && !*resp.OK {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			// Check for rate limiting
			if rl := parseRateLimitError(errMsg); rl != nil {
				return "", rl
			}
			// Check for EBUSY
			if strings.Contains(errMsg, "EBUSY") || strings.Contains(errMsg, "resource busy") {
				return "", ErrConfigFileLocked
			}
		}
		return "", fmt.Errorf("config.patch error: %s", errMsg)
	}

	// Extract new hash from response
	var newHash string
	if payload, ok := resp.Payload.(map[string]interface{}); ok {
		if hash, ok := payload["hash"].(string); ok {
			newHash = hash
		}
	}

	return newHash, nil
}

// tryRestartGateway attempts a single Gateway restart via config.patch.
func (c *RPCClient) tryRestartGateway(reason string) error {
	// First, get current config to get the baseHash
	getResp, err := c.call("config.get", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("config.get failed: %w", err)
	}
	if getResp.OK != nil && !*getResp.OK {
		errMsg := "unknown error"
		if getResp.Error != nil {
			errMsg = getResp.Error.Message
		}
		return fmt.Errorf("config.get error: %s", errMsg)
	}
	
	// Extract baseHash from response
	var baseHash string
	if payload, ok := getResp.Payload.(map[string]interface{}); ok {
		if hash, ok := payload["hash"].(string); ok {
			baseHash = hash
		}
	}
	
	// Use config.patch with empty patch to trigger restart
	// The patch is an empty object "{}" which makes no changes but still triggers restart
	resp, err := c.call("config.patch", map[string]interface{}{
		"raw":            "{}",  // Empty patch - no config changes
		"baseHash":       baseHash,
		"note":           reason,
		"restartDelayMs": 1000,
	})
	if err != nil {
		return err
	}
	if resp.OK != nil && !*resp.OK {
		errMsg := "unknown error"
		errCode := ""
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		// Detect various failure modes
		if errCode == "INVALID_REQUEST" && strings.Contains(errMsg, "unknown method") {
			return ErrRestartDisabled
		}
		if strings.Contains(errMsg, "disabled") || strings.Contains(errMsg, "not enabled") {
			return ErrRestartDisabled
		}
		return fmt.Errorf("RPC error: %s", errMsg)
	}
	return nil
}
