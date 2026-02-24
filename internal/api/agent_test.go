package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"log/slog"

	"github.com/openclaw/ocm/internal/store"
)

func setupTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	
	tmpFile, err := os.CreateTemp("", "ocm-api-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	s, err := store.New(tmpFile.Name(), masterKey)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatal(err)
	}

	cleanup := func() {
		s.Close()
		os.Remove(tmpFile.Name())
	}

	return s, cleanup
}

func TestAgentAPI_ListScopes(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	// Add a test credential
	cred := &store.Credential{
		ID:          "test-cred",
		Service:     "gmail",
		DisplayName: "Gmail Test",
		Type:        "oauth2",
		Scopes: map[string]*store.Scope{
			"read":  {Name: "read", Permanent: true, Token: "read-token"},
			"write": {Name: "write", RequiresApproval: true, Token: "write-token"},
		},
	}
	if err := db.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/v1/scopes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListScopes status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp ScopesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Services) != 1 {
		t.Errorf("ListScopes services = %d, want 1", len(resp.Services))
	}
	if resp.Services[0].ID != "gmail" {
		t.Errorf("ListScopes service ID = %s, want gmail", resp.Services[0].ID)
	}
}

func TestAgentAPI_GetCredential_Permanent(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	// Add a test credential with permanent scope
	cred := &store.Credential{
		ID:          "test-cred",
		Service:     "gmail",
		DisplayName: "Gmail Test",
		Type:        "oauth2",
		Scopes: map[string]*store.Scope{
			"read": {Name: "read", Permanent: true, Token: "secret-read-token"},
		},
	}
	if err := db.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/v1/credentials/gmail/read", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetCredential status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp CredentialResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Token != "secret-read-token" {
		t.Errorf("GetCredential token = %s, want secret-read-token", resp.Token)
	}
}

func TestAgentAPI_GetCredential_RequiresElevation(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	// Add a test credential with non-permanent scope
	cred := &store.Credential{
		ID:          "test-cred",
		Service:     "gmail",
		DisplayName: "Gmail Test",
		Type:        "oauth2",
		Scopes: map[string]*store.Scope{
			"write": {Name: "write", Permanent: false, RequiresApproval: true, Token: "secret-write-token"},
		},
	}
	if err := db.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	// Should be forbidden without elevation
	req := httptest.NewRequest("GET", "/api/v1/credentials/gmail/write", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("GetCredential without elevation status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAgentAPI_RequestElevation(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	// Add a test credential
	cred := &store.Credential{
		ID:          "test-cred",
		Service:     "gmail",
		DisplayName: "Gmail Test",
		Type:        "oauth2",
		Scopes: map[string]*store.Scope{
			"write": {Name: "write", Permanent: false, RequiresApproval: true, Token: "write-token"},
		},
	}
	if err := db.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	// Request elevation
	body := ElevationRequest{
		Service: "gmail",
		Scope:   "write",
		Reason:  "Send test email",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/elevate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequestElevation status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp ElevationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "pending" {
		t.Errorf("RequestElevation status = %s, want pending", resp.Status)
	}
	if resp.RequestID == "" {
		t.Error("RequestElevation should return requestId")
	}
}

func TestAgentAPI_GetCredential_WithElevation(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	// Add a test credential
	cred := &store.Credential{
		ID:          "test-cred",
		Service:     "gmail",
		DisplayName: "Gmail Test",
		Type:        "oauth2",
		Scopes: map[string]*store.Scope{
			"write": {Name: "write", Permanent: false, RequiresApproval: true, Token: "secret-write-token"},
		},
	}
	if err := db.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	// Create an approved elevation
	expiresAt := time.Now().Add(30 * time.Minute)
	elev := &store.Elevation{
		ID:          "test-elev",
		Service:     "gmail",
		Scope:       "write",
		Reason:      "Test",
		Status:      "pending",
		RequestedAt: time.Now(),
	}
	if err := db.CreateElevation(elev); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateElevation("test-elev", "approved", "admin", &expiresAt); err != nil {
		t.Fatal(err)
	}

	// Now should be able to get credential
	req := httptest.NewRequest("GET", "/api/v1/credentials/gmail/write", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetCredential with elevation status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp CredentialResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Token != "secret-write-token" {
		t.Errorf("GetCredential token = %s, want secret-write-token", resp.Token)
	}
}

func TestAgentAPI_Health(t *testing.T) {
	db, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewAgentRouter(db, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health status = %d, want %d", w.Code, http.StatusOK)
	}
}
