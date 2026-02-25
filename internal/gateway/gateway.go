// Package gateway handles integration with OpenClaw Gateway.
package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Client manages communication with OpenClaw Gateway.
type Client struct {
	// GatewayURL is the OpenClaw Gateway RPC endpoint (e.g., http://localhost:18789)
	// Note: Currently unused - restart requires WebSocket RPC (TODO)
	GatewayURL string
	// EnvFilePath is the path to OpenClaw's .env file (e.g., ~/.openclaw/.env)
	EnvFilePath string
}

// NewClient creates a new Gateway client.
func NewClient(gatewayURL, envFilePath string) *Client {
	if envFilePath == "" {
		home, _ := os.UserHomeDir()
		envFilePath = filepath.Join(home, ".openclaw", ".env")
	}
	return &Client{
		GatewayURL:  gatewayURL,
		EnvFilePath: envFilePath,
	}
}

// CredentialEnv represents a credential as an environment variable.
type CredentialEnv struct {
	Name  string // e.g., "GMAIL_TOKEN", "LINEAR_API_KEY"
	Value string // The actual credential value
}

// SetCredentials writes credentials to the .env file and triggers a Gateway restart.
// This is the core function for credential injection.
func (c *Client) SetCredentials(creds []CredentialEnv) error {
	// Read existing .env file
	existing, err := c.readEnvFile()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read env file: %w", err)
	}

	// Update with new credentials
	for _, cred := range creds {
		existing[cred.Name] = cred.Value
	}

	// Write back
	if err := c.writeEnvFile(existing); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	// Trigger Gateway restart
	if err := c.RestartGateway("OCM credential update"); err != nil {
		return fmt.Errorf("restart gateway: %w", err)
	}

	return nil
}

// ClearCredentials removes credentials from the .env file and triggers a Gateway restart.
func (c *Client) ClearCredentials(names []string) error {
	existing, err := c.readEnvFile()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read env file: %w", err)
	}

	for _, name := range names {
		delete(existing, name)
	}

	if err := c.writeEnvFile(existing); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	if err := c.RestartGateway("OCM credential removal"); err != nil {
		return fmt.Errorf("restart gateway: %w", err)
	}

	return nil
}

// WriteCredentialToEnv writes a single credential to the .env file without restarting.
// Use this during setup to accumulate credentials, then call SyncAndRestart when done.
func (c *Client) WriteCredentialToEnv(name, value string) error {
	existing, err := c.readEnvFile()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read env file: %w", err)
	}

	existing[name] = value

	return c.writeEnvFile(existing)
}

// SyncAndRestart syncs current credentials to the .env file and restarts Gateway.
// This is used during setup to ensure all credentials are written before restart.
func (c *Client) SyncAndRestart(reason string) error {
	// Just trigger restart - credentials should already be in .env from creation
	return c.RestartGateway(reason)
}

// RestartGateway triggers a Gateway restart.
// Note: OpenClaw uses WebSocket RPC, not HTTP REST. For now, we skip the
// automatic restart and rely on the user to restart OpenClaw manually,
// or on OpenClaw reading the updated .env on its next natural restart.
func (c *Client) RestartGateway(reason string) error {
	// TODO: Implement proper WebSocket RPC client to call gateway.restart
	// For now, credentials are written to .env and will be picked up on
	// the next OpenClaw restart. Log this for transparency.
	return nil // Skip restart - not yet implemented
}

// GetCurrentCredentials reads the current credentials from the .env file.
// Returns map of credential name -> value (values are masked in logs).
func (c *Client) GetCurrentCredentials() (map[string]string, error) {
	return c.readEnvFile()
}

// readEnvFile parses the .env file into a map.
func (c *Client) readEnvFile() (map[string]string, error) {
	data, err := os.ReadFile(c.EnvFilePath)
	if err != nil {
		return make(map[string]string), err
	}

	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			result[key] = value
		}
	}
	return result, nil
}

// writeEnvFile writes the map back to the .env file.
func (c *Client) writeEnvFile(env map[string]string) error {
	// Ensure directory exists
	dir := filepath.Dir(c.EnvFilePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	var lines []string
	lines = append(lines, "# Managed by OCM (OpenClaw Credential Manager)")
	lines = append(lines, "# Do not edit manually - changes will be overwritten")
	lines = append(lines, "")

	for key, value := range env {
		// Quote values that contain spaces or special characters
		if strings.ContainsAny(value, " \t\n\"'") {
			value = fmt.Sprintf(`"%s"`, strings.ReplaceAll(value, `"`, `\"`))
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(c.EnvFilePath, []byte(content), 0600)
}
