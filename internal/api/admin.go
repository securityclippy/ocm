package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openclaw/ocm/internal"
	"github.com/openclaw/ocm/internal/elevation"
	"github.com/openclaw/ocm/internal/gateway"
	"github.com/openclaw/ocm/internal/store"
)

// NewAdminRouter creates the router for admin API/UI (:8080).
// This API is for human administrators and includes:
// - Credential management
// - Elevation approval/denial
// - Device pairing management
// - Audit log viewing
// - Web UI serving
func NewAdminRouter(db *store.Store, elevSvc *elevation.Service, rpcClient *gateway.RPCClient, logger *slog.Logger) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &adminHandler{store: db, elevation: elevSvc, rpc: rpcClient, logger: logger}

	// API routes (protected by auth middleware)
	r.Route("/admin/api", func(r chi.Router) {
		// TODO: Add auth middleware
		// r.Use(adminAuthMiddleware)

		// Setup (bootstrap flow)
		r.Get("/setup/status", h.getSetupStatus)
		r.Post("/setup/complete", h.completeSetup)

		// Dashboard
		r.Get("/dashboard", h.getDashboard)

		// Credentials
		r.Get("/credentials", h.listCredentials)
		r.Post("/credentials", h.createCredential)
		r.Get("/credentials/{service}", h.getCredential)
		r.Put("/credentials/{service}", h.updateCredential)
		r.Delete("/credentials/{service}", h.deleteCredential)

		// Elevations
		r.Get("/requests", h.listPendingRequests)
		r.Post("/requests/{id}/approve", h.approveRequest)
		r.Post("/requests/{id}/deny", h.denyRequest)
		r.Post("/revoke/{service}/{scope}", h.revokeElevation)

		// Audit
		r.Get("/audit", h.listAuditEntries)

		// Device pairing (OpenClaw integration)
		r.Get("/devices", h.listDevices)
		r.Post("/devices/{requestId}/approve", h.approveDevice)
		r.Post("/devices/{requestId}/reject", h.rejectDevice)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Serve SPA (fallback to index.html for client-side routing)
	r.Handle("/*", spaHandler())

	return r
}

type adminHandler struct {
	store     *store.Store
	elevation *elevation.Service
	rpc       *gateway.RPCClient
	logger    *slog.Logger
}

// DashboardResponse contains summary data for the admin dashboard.
type DashboardResponse struct {
	TotalCredentials   int                   `json:"totalCredentials"`
	PendingRequests    int                   `json:"pendingRequests"`
	ActiveElevations   int                   `json:"activeElevations"`
	RecentAuditEntries []*store.AuditEntry   `json:"recentAudit"`
	Pending            []*store.Elevation    `json:"pending"`
}

// CreateCredentialRequest is the request body for creating credentials.
type CreateCredentialRequest struct {
	Service     string             `json:"service"`
	DisplayName string             `json:"displayName"`
	Type        string             `json:"type"`
	Read        *AccessLevelConfig `json:"read"`               // Required - always available
	ReadWrite   *AccessLevelConfig `json:"readWrite,omitempty"` // Optional - requires elevation
}

// AccessLevelConfig is the configuration for a single access level.
type AccessLevelConfig struct {
	EnvVar       string `json:"envVar"`                 // e.g., "GITHUB_TOKEN"
	Token        string `json:"token"`                  // The credential value
	RefreshToken string `json:"refreshToken,omitempty"` // For OAuth
	MaxTTL       string `json:"maxTTL,omitempty"`       // e.g., "1h" - only for ReadWrite
}

// ApproveRequest is the request body for approving an elevation.
type ApproveRequest struct {
	TTL string `json:"ttl"` // e.g., "30m", "1h"
}

// SetupStatusResponse indicates whether initial setup is complete.
type SetupStatusResponse struct {
	SetupComplete bool     `json:"setupComplete"`
	MissingKeys   []string `json:"missingKeys"`   // Required credentials not yet configured
	ConfiguredKeys []string `json:"configuredKeys"` // Already configured credentials
}

// requiredModelProviders lists the services that provide LLM API keys.
// At least one must be configured for OpenClaw to work.
// NOTE: Keep in sync with modelProviderIds in SetupWizard.svelte
var requiredModelProviders = []string{"anthropic", "openai", "openrouter", "groq", "google", "azure-openai"}

func (h *adminHandler) getSetupStatus(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.ListCredentials()
	if err != nil {
		h.logger.Error("setup status: failed to list credentials", "error", err)
		h.jsonError(w, "failed to check credentials", http.StatusInternalServerError)
		return
	}

	// Check if any model provider is configured
	// Note: ListCredentials() clears tokens for security, so we just check
	// if a provider credential exists (if it was created, it has a token)
	var configuredKeys []string
	hasModelProvider := false

	h.logger.Info("setup status check", "credentialCount", len(creds))
	for _, cred := range creds {
		configuredKeys = append(configuredKeys, cred.Service)
		h.logger.Info("checking credential", "service", cred.Service, "hasRead", cred.Read != nil, "hasReadWrite", cred.ReadWrite != nil)
		for _, provider := range requiredModelProviders {
			if cred.Service == provider {
				// Provider credential exists - setup is complete
				h.logger.Info("found model provider", "provider", provider)
				hasModelProvider = true
				break
			}
		}
	}

	// Ensure empty slice instead of nil
	if configuredKeys == nil {
		configuredKeys = []string{}
	}

	resp := SetupStatusResponse{
		SetupComplete:  hasModelProvider,
		ConfiguredKeys: configuredKeys,
	}

	if !hasModelProvider {
		resp.MissingKeys = []string{"anthropic OR openai OR google OR azure-openai"}
	} else {
		resp.MissingKeys = []string{}
	}

	h.jsonResponse(w, resp)
}

func (h *adminHandler) completeSetup(w http.ResponseWriter, r *http.Request) {
	// Trigger Gateway restart to pick up any new credentials
	if h.elevation != nil && h.elevation.Gateway() != nil {
		if err := h.elevation.Gateway().SyncAndRestart("setup complete"); err != nil {
			h.logger.Error("failed to restart gateway after setup", "error", err)
			// Don't fail the request - credentials are saved, restart can be done manually
		}
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "setup_completed",
		Actor:     "admin",
	})

	h.logger.Info("setup completed, gateway restart triggered")

	h.jsonResponse(w, map[string]interface{}{
		"status":  "complete",
		"message": "Setup complete. OpenClaw will restart to load credentials.",
	})
}

func (h *adminHandler) getDashboard(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.ListCredentials()
	if err != nil {
		h.jsonError(w, "failed to list credentials", http.StatusInternalServerError)
		return
	}

	pending, err := h.store.ListPendingElevations()
	if err != nil {
		h.jsonError(w, "failed to list pending", http.StatusInternalServerError)
		return
	}

	audit, err := h.store.ListAuditEntries(10, "")
	if err != nil {
		h.jsonError(w, "failed to list audit", http.StatusInternalServerError)
		return
	}

	// Count active elevations (simplified - would need a real query)
	activeCount := 0

	// Ensure we return empty slices instead of nil (JSON: [] not null)
	if audit == nil {
		audit = []*store.AuditEntry{}
	}
	if pending == nil {
		pending = []*store.Elevation{}
	}

	h.jsonResponse(w, DashboardResponse{
		TotalCredentials:   len(creds),
		PendingRequests:    len(pending),
		ActiveElevations:   activeCount,
		RecentAuditEntries: audit,
		Pending:            pending,
	})
}

func (h *adminHandler) listCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.ListCredentials()
	if err != nil {
		h.logger.Error("list credentials failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Ensure empty slice instead of nil
	if creds == nil {
		creds = []*store.Credential{}
	}
	h.jsonResponse(w, creds)
}

func (h *adminHandler) createCredential(w http.ResponseWriter, r *http.Request) {
	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Service == "" || req.DisplayName == "" {
		h.jsonError(w, "service and displayName are required", http.StatusBadRequest)
		return
	}

	if req.Read == nil || req.Read.EnvVar == "" {
		h.jsonError(w, "read access with envVar is required", http.StatusBadRequest)
		return
	}

	// Convert to store.Credential with new Read/ReadWrite model
	cred := &store.Credential{
		ID:          generateID("cred"),
		Service:     req.Service,
		DisplayName: req.DisplayName,
		Type:        req.Type,
		Read: &store.AccessLevel{
			EnvVar:       req.Read.EnvVar,
			Token:        req.Read.Token,
			RefreshToken: req.Read.RefreshToken,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add ReadWrite access if provided
	if req.ReadWrite != nil && req.ReadWrite.EnvVar != "" {
		var maxTTL time.Duration
		if req.ReadWrite.MaxTTL != "" {
			var err error
			maxTTL, err = time.ParseDuration(req.ReadWrite.MaxTTL)
			if err != nil {
				h.jsonError(w, "invalid maxTTL format for readWrite", http.StatusBadRequest)
				return
			}
		} else {
			maxTTL = 30 * time.Minute // Default max TTL
		}

		cred.ReadWrite = &store.AccessLevel{
			EnvVar:       req.ReadWrite.EnvVar,
			Token:        req.ReadWrite.Token,
			RefreshToken: req.ReadWrite.RefreshToken,
			MaxTTL:       maxTTL,
		}
	}

	if err := h.store.SaveCredential(cred); err != nil {
		h.logger.Error("save credential failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sync read credentials to Gateway .env file and restart
	if h.elevation != nil && h.elevation.Gateway() != nil {
		if cred.Read != nil && cred.Read.EnvVar != "" && cred.Read.Token != "" {
			if err := h.elevation.Gateway().WriteCredentialToEnv(cred.Read.EnvVar, cred.Read.Token); err != nil {
				h.logger.Error("failed to write credential to env", "error", err)
			}
			// Trigger Gateway restart to pick up new credential
			if err := h.elevation.Gateway().RestartGateway("credential created: " + req.Service); err != nil {
				h.logger.Error("failed to restart gateway", "error", err)
				// Don't fail the request - credential is saved, just needs manual restart
			}
		}
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_created",
		Service:   req.Service,
		Actor:     "admin",
	})

	h.logger.Info("credential created", "service", req.Service)
	w.WriteHeader(http.StatusCreated)
	h.jsonResponse(w, cred)
}

func (h *adminHandler) getCredential(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	cred, err := h.store.GetCredential(service)
	if err != nil {
		h.logger.Error("get credential failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cred == nil {
		h.jsonError(w, "not found", http.StatusNotFound)
		return
	}
	h.jsonResponse(w, cred)
}

func (h *adminHandler) updateCredential(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")

	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing
	existing, err := h.store.GetCredential(service)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		h.jsonError(w, "not found", http.StatusNotFound)
		return
	}

	// Update fields
	existing.DisplayName = req.DisplayName
	existing.Type = req.Type
	existing.UpdatedAt = time.Now()

	// Update Read access
	if req.Read != nil {
		existing.Read = &store.AccessLevel{
			EnvVar:       req.Read.EnvVar,
			Token:        req.Read.Token,
			RefreshToken: req.Read.RefreshToken,
		}
	}

	// Update ReadWrite access
	if req.ReadWrite != nil && req.ReadWrite.EnvVar != "" {
		var maxTTL time.Duration
		if req.ReadWrite.MaxTTL != "" {
			maxTTL, _ = time.ParseDuration(req.ReadWrite.MaxTTL)
		} else {
			maxTTL = 30 * time.Minute
		}
		existing.ReadWrite = &store.AccessLevel{
			EnvVar:       req.ReadWrite.EnvVar,
			Token:        req.ReadWrite.Token,
			RefreshToken: req.ReadWrite.RefreshToken,
			MaxTTL:       maxTTL,
		}
	} else {
		existing.ReadWrite = nil // Clear if not provided
	}

	if err := h.store.SaveCredential(existing); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sync read credentials to Gateway .env file and restart
	if h.elevation != nil && h.elevation.Gateway() != nil {
		if existing.Read != nil && existing.Read.EnvVar != "" && existing.Read.Token != "" {
			if err := h.elevation.Gateway().WriteCredentialToEnv(existing.Read.EnvVar, existing.Read.Token); err != nil {
				h.logger.Error("failed to write credential to env", "error", err)
			}
			// Trigger Gateway restart to pick up updated credential
			if err := h.elevation.Gateway().RestartGateway("credential updated: " + service); err != nil {
				h.logger.Error("failed to restart gateway", "error", err)
			}
		}
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_updated",
		Service:   service,
		Actor:     "admin",
	})

	h.jsonResponse(w, existing)
}

func (h *adminHandler) deleteCredential(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")

	// Get credential first to know which env vars to clear
	cred, err := h.store.GetCredential(service)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Collect env vars to clear
	var envVarsToClear []string
	if cred != nil {
		if cred.Read != nil && cred.Read.EnvVar != "" {
			envVarsToClear = append(envVarsToClear, cred.Read.EnvVar)
		}
		if cred.ReadWrite != nil && cred.ReadWrite.EnvVar != "" && 
		   (cred.Read == nil || cred.ReadWrite.EnvVar != cred.Read.EnvVar) {
			envVarsToClear = append(envVarsToClear, cred.ReadWrite.EnvVar)
		}
	}

	// Delete from database
	if err := h.store.DeleteCredential(service); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Clear from .env and restart Gateway
	if h.elevation != nil && h.elevation.Gateway() != nil && len(envVarsToClear) > 0 {
		if err := h.elevation.Gateway().ClearCredentials(envVarsToClear); err != nil {
			h.logger.Error("failed to clear credentials from env", "error", err, "envVars", envVarsToClear)
			// Don't fail - credential is deleted from DB, env cleanup is best-effort
		}
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_deleted",
		Service:   service,
		Actor:     "admin",
	})

	h.logger.Info("credential deleted", "service", service, "clearedEnvVars", envVarsToClear)
	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listPendingRequests(w http.ResponseWriter, r *http.Request) {
	pending, err := h.store.ListPendingElevations()
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if pending == nil {
		pending = []*store.Elevation{}
	}
	h.jsonResponse(w, pending)
}

func (h *adminHandler) approveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to 30 minutes if not specified
		req.TTL = "30m"
	}

	ttl, err := time.ParseDuration(req.TTL)
	if err != nil {
		ttl = 30 * time.Minute
	}

	// Use elevation service to approve and inject credential
	if err := h.elevation.ApproveElevation(id, ttl, "admin"); err != nil {
		h.logger.Error("approve elevation failed", "error", err)
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get updated elevation for response
	elev, _ := h.store.GetElevation(id)
	
	h.logger.Info("elevation approved via admin API",
		"request_id", id,
		"ttl", ttl,
	)

	h.jsonResponse(w, map[string]interface{}{
		"status":    "approved",
		"expiresAt": elev.ExpiresAt,
	})
}

func (h *adminHandler) denyRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	elev, err := h.store.GetElevation(id)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if elev == nil {
		h.jsonError(w, "not found", http.StatusNotFound)
		return
	}

	if err := h.store.UpdateElevation(id, "denied", "admin", nil); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_denied",
		Service:   elev.Service,
		Scope:     elev.Scope,
		Actor:     "admin",
	})

	h.logger.Info("elevation denied", "request_id", id)

	h.jsonResponse(w, map[string]string{"status": "denied"})
}

func (h *adminHandler) revokeElevation(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	scope := chi.URLParam(r, "scope")

	// Use elevation service to revoke and remove credential from Gateway
	if err := h.elevation.RevokeElevation(service, scope, "admin revocation"); err != nil {
		h.logger.Error("revoke elevation failed", "error", err)
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("elevation revoked via admin API", "service", service, "scope", scope)

	h.jsonResponse(w, map[string]string{"status": "revoked"})
}

func (h *adminHandler) listAuditEntries(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	entries, err := h.store.ListAuditEntries(100, service)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []*store.AuditEntry{}
	}
	h.jsonResponse(w, entries)
}

func (h *adminHandler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *adminHandler) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Device pairing handlers

func (h *adminHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	if h.rpc == nil {
		h.jsonResponse(w, map[string]interface{}{
			"pending": []gateway.PendingDevice{},
			"paired":  []gateway.PairedDevice{},
			"error":   "OPENCLAW_GATEWAY_TOKEN not configured - set it in docker-compose.yml",
		})
		return
	}

	devices, err := h.rpc.ListDevices()
	if err != nil {
		h.logger.Error("list devices failed", "error", err)
		h.jsonResponse(w, map[string]interface{}{
			"pending": []gateway.PendingDevice{},
			"paired":  []gateway.PairedDevice{},
			"error":   fmt.Sprintf("Gateway RPC error: %v", err),
		})
		return
	}

	// Ensure empty slices instead of nil
	if devices.Pending == nil {
		devices.Pending = []gateway.PendingDevice{}
	}
	if devices.Paired == nil {
		devices.Paired = []gateway.PairedDevice{}
	}

	h.jsonResponse(w, map[string]interface{}{
		"pending": devices.Pending,
		"paired":  devices.Paired,
	})
}

func (h *adminHandler) approveDevice(w http.ResponseWriter, r *http.Request) {
	if h.rpc == nil {
		h.jsonError(w, "Gateway RPC not configured", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestId")
	if requestID == "" {
		h.jsonError(w, "requestId is required", http.StatusBadRequest)
		return
	}

	if err := h.rpc.ApproveDevice(requestID); err != nil {
		h.logger.Error("approve device failed", "error", err, "requestId", requestID)
		h.jsonError(w, fmt.Sprintf("failed to approve device: %v", err), http.StatusBadGateway)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "device_approved",
		Details:   fmt.Sprintf("requestId: %s", requestID),
		Actor:     "admin",
	})

	h.logger.Info("device pairing approved", "requestId", requestID)
	h.jsonResponse(w, map[string]string{"status": "approved", "requestId": requestID})
}

func (h *adminHandler) rejectDevice(w http.ResponseWriter, r *http.Request) {
	if h.rpc == nil {
		h.jsonError(w, "Gateway RPC not configured", http.StatusServiceUnavailable)
		return
	}

	requestID := chi.URLParam(r, "requestId")
	if requestID == "" {
		h.jsonError(w, "requestId is required", http.StatusBadRequest)
		return
	}

	if err := h.rpc.RejectDevice(requestID); err != nil {
		h.logger.Error("reject device failed", "error", err, "requestId", requestID)
		h.jsonError(w, fmt.Sprintf("failed to reject device: %v", err), http.StatusBadGateway)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "device_rejected",
		Details:   fmt.Sprintf("requestId: %s", requestID),
		Actor:     "admin",
	})

	h.logger.Info("device pairing rejected", "requestId", requestID)
	h.jsonResponse(w, map[string]string{"status": "rejected", "requestId": requestID})
}

// spaHandler serves the SvelteKit SPA with fallback to index.html.
func spaHandler() http.Handler {
	// Try to get embedded assets
	webRoot, err := fs.Sub(internal.WebAssets, "web/build")
	if err != nil {
		// No embedded assets - serve placeholder
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>OCM - OpenClaw Credential Manager</title></head>
<body>
<h1>OCM Admin UI</h1>
<p>Frontend not yet built. Run <code>make web</code> to build the SvelteKit app.</p>
<p><a href="/admin/api/dashboard">API Dashboard</a></p>
</body>
</html>`))
		})
	}

	// Create file server with SPA fallback
	fileServer := http.FileServer(http.FS(webRoot))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Check if file exists
		if path != "/" {
			_, err := fs.Stat(webRoot, path[1:]) // Remove leading /
			if err == nil {
				// File exists, serve it
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fall back to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
