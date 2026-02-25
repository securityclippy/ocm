package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openclaw/ocm/internal"
	"github.com/openclaw/ocm/internal/elevation"
	"github.com/openclaw/ocm/internal/store"
)

// NewAdminRouter creates the router for admin API/UI (:8080).
// This API is for human administrators and includes:
// - Credential management
// - Elevation approval/denial
// - Audit log viewing
// - Web UI serving
func NewAdminRouter(db *store.Store, elevSvc *elevation.Service, logger *slog.Logger) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &adminHandler{store: db, elevation: elevSvc, logger: logger}

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
	Service     string                 `json:"service"`
	DisplayName string                 `json:"displayName"`
	Type        string                 `json:"type"`
	Scopes      map[string]ScopeConfig `json:"scopes"`
}

// ScopeConfig is the configuration for a single scope.
type ScopeConfig struct {
	EnvVar           string `json:"envVar"`           // e.g., "GMAIL_TOKEN"
	Permanent        bool   `json:"permanent"`
	RequiresApproval bool   `json:"requiresApproval"`
	MaxTTL           string `json:"maxTTL"` // e.g., "1h", "30m"
	Token            string `json:"token"`
	RefreshToken     string `json:"refreshToken,omitempty"`
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
var requiredModelProviders = []string{"anthropic", "openai", "google", "azure-openai"}

func (h *adminHandler) getSetupStatus(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.ListCredentials()
	if err != nil {
		h.jsonError(w, "failed to check credentials", http.StatusInternalServerError)
		return
	}

	// Check if any model provider is configured with a permanent API key
	var configuredKeys []string
	hasModelProvider := false

	for _, cred := range creds {
		configuredKeys = append(configuredKeys, cred.Service)
		for _, provider := range requiredModelProviders {
			if cred.Service == provider {
				// Check if it has a permanent scope with a token
				for _, scope := range cred.Scopes {
					if scope.Permanent && scope.Token != "" {
						hasModelProvider = true
						break
					}
				}
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

	// Convert to store.Credential
	cred := &store.Credential{
		ID:          generateID("cred"),
		Service:     req.Service,
		DisplayName: req.DisplayName,
		Type:        req.Type,
		Scopes:      make(map[string]*store.Scope),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for name, cfg := range req.Scopes {
		var maxTTL time.Duration
		if cfg.MaxTTL != "" {
			var err error
			maxTTL, err = time.ParseDuration(cfg.MaxTTL)
			if err != nil {
				h.jsonError(w, "invalid maxTTL format", http.StatusBadRequest)
				return
			}
		}

		cred.Scopes[name] = &store.Scope{
			Name:             name,
			EnvVar:           cfg.EnvVar,
			Permanent:        cfg.Permanent,
			RequiresApproval: cfg.RequiresApproval,
			MaxTTL:           maxTTL,
			Token:            cfg.Token,
			RefreshToken:     cfg.RefreshToken,
		}
	}

	if err := h.store.SaveCredential(cred); err != nil {
		h.logger.Error("save credential failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sync permanent credentials to Gateway .env file (without restart yet)
	// The setup wizard will trigger restart after all credentials are added
	if h.elevation != nil && h.elevation.Gateway() != nil {
		for _, scope := range cred.Scopes {
			if scope.Permanent && scope.EnvVar != "" && scope.Token != "" {
				h.elevation.Gateway().WriteCredentialToEnv(scope.EnvVar, scope.Token)
			}
		}
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_created",
		Service:   req.Service,
		Actor:     "admin", // TODO: get from auth
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

	for name, cfg := range req.Scopes {
		var maxTTL time.Duration
		if cfg.MaxTTL != "" {
			maxTTL, _ = time.ParseDuration(cfg.MaxTTL)
		}

		existing.Scopes[name] = &store.Scope{
			Name:             name,
			EnvVar:           cfg.EnvVar,
			Permanent:        cfg.Permanent,
			RequiresApproval: cfg.RequiresApproval,
			MaxTTL:           maxTTL,
			Token:            cfg.Token,
			RefreshToken:     cfg.RefreshToken,
		}
	}

	if err := h.store.SaveCredential(existing); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
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

	if err := h.store.DeleteCredential(service); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_deleted",
		Service:   service,
		Actor:     "admin",
	})

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
