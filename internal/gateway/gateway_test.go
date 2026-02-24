package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteEnvFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ocm-gateway-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, ".env")
	client := NewClient("http://localhost:18789", envPath)

	// Write some credentials
	env := map[string]string{
		"GMAIL_TOKEN":    "ya29.test-token",
		"LINEAR_API_KEY": "lin_api_xxx",
	}
	if err := client.writeEnvFile(env); err != nil {
		t.Fatalf("writeEnvFile() error = %v", err)
	}

	// Read back
	got, err := client.readEnvFile()
	if err != nil {
		t.Fatalf("readEnvFile() error = %v", err)
	}

	if got["GMAIL_TOKEN"] != "ya29.test-token" {
		t.Errorf("GMAIL_TOKEN = %s, want ya29.test-token", got["GMAIL_TOKEN"])
	}
	if got["LINEAR_API_KEY"] != "lin_api_xxx" {
		t.Errorf("LINEAR_API_KEY = %s, want lin_api_xxx", got["LINEAR_API_KEY"])
	}

	// Verify file permissions
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestSetCredentials(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ocm-gateway-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock Gateway server
	restartCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/gateway" {
			restartCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	envPath := filepath.Join(tmpDir, ".env")
	client := NewClient(server.URL, envPath)

	// Set credentials
	err = client.SetCredentials([]CredentialEnv{
		{Name: "TEST_TOKEN", Value: "secret123"},
	})
	if err != nil {
		t.Fatalf("SetCredentials() error = %v", err)
	}

	// Verify restart was called
	if !restartCalled {
		t.Error("Gateway restart was not called")
	}

	// Verify credential was written
	got, err := client.readEnvFile()
	if err != nil {
		t.Fatal(err)
	}
	if got["TEST_TOKEN"] != "secret123" {
		t.Errorf("TEST_TOKEN = %s, want secret123", got["TEST_TOKEN"])
	}
}

func TestClearCredentials(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ocm-gateway-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock Gateway server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	envPath := filepath.Join(tmpDir, ".env")
	client := NewClient(server.URL, envPath)

	// Set initial credentials
	client.writeEnvFile(map[string]string{
		"KEEP_THIS":   "value1",
		"REMOVE_THIS": "value2",
	})

	// Clear one credential
	err = client.ClearCredentials([]string{"REMOVE_THIS"})
	if err != nil {
		t.Fatalf("ClearCredentials() error = %v", err)
	}

	// Verify
	got, _ := client.readEnvFile()
	if _, exists := got["REMOVE_THIS"]; exists {
		t.Error("REMOVE_THIS should have been removed")
	}
	if got["KEEP_THIS"] != "value1" {
		t.Errorf("KEEP_THIS = %s, want value1", got["KEEP_THIS"])
	}
}

func TestEnvFileWithQuotes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ocm-gateway-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, ".env")
	client := NewClient("http://localhost:18789", envPath)

	// Write value with spaces
	env := map[string]string{
		"TOKEN_WITH_SPACE": "has some spaces",
	}
	client.writeEnvFile(env)

	// Read file content directly
	content, _ := os.ReadFile(envPath)
	if !strings.Contains(string(content), `"has some spaces"`) {
		t.Errorf("value with spaces should be quoted, got: %s", content)
	}

	// Read back via parser
	got, _ := client.readEnvFile()
	if got["TOKEN_WITH_SPACE"] != "has some spaces" {
		t.Errorf("TOKEN_WITH_SPACE = %s, want 'has some spaces'", got["TOKEN_WITH_SPACE"])
	}
}
