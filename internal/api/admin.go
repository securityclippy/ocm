package api

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openclaw/ocm/internal/store"
)

// WebAssets holds the embedded SvelteKit build.
// This is set from the main package.
var WebAssets embed.FS

// NewAdminRouter creates the router for admin API/UI (:8080).
// This API is for human administrators and includes:
// - Credential management
// - Elevation approval/denial
// - Audit log viewing
// - Web UI serving
func NewAdminRouter(db *store.Store, logger *slog.Logger) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &adminHandler{store: db, logger: logger}

	// API routes (protected by auth middleware)
	r.Route("/admin/api", func(r chi.Router) {
		// TODO: Add auth middleware
		// r.Use(adminAuthMiddleware)

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
	store  *store.Store
	logger *slog.Logger
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

	// Get the elevation request
	elev, err := h.store.GetElevation(id)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if elev == nil {
		h.jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if elev.Status != "pending" {
		h.jsonError(w, "request not pending", http.StatusBadRequest)
		return
	}

	// Check TTL against maxTTL
	cred, _ := h.store.GetCredential(elev.Service)
	if cred != nil {
		if scope, ok := cred.Scopes[elev.Scope]; ok && scope.MaxTTL > 0 {
			if ttl > scope.MaxTTL {
				ttl = scope.MaxTTL
			}
		}
	}

	expiresAt := time.Now().Add(ttl)
	if err := h.store.UpdateElevation(id, "approved", "admin", &expiresAt); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_approved",
		Service:   elev.Service,
		Scope:     elev.Scope,
		Details:   req.TTL,
		Actor:     "admin",
	})

	h.logger.Info("elevation approved",
		"request_id", id,
		"service", elev.Service,
		"scope", elev.Scope,
		"ttl", ttl,
	)

	// TODO: Send callback to agent

	h.jsonResponse(w, map[string]interface{}{
		"status":    "approved",
		"expiresAt": expiresAt,
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

	active, err := h.store.GetActiveElevation(service, scope)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if active == nil {
		h.jsonError(w, "no active elevation", http.StatusNotFound)
		return
	}

	if err := h.store.UpdateElevation(active.ID, "revoked", "admin", nil); err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_revoked",
		Service:   service,
		Scope:     scope,
		Actor:     "admin",
	})

	h.logger.Info("elevation revoked", "service", service, "scope", scope)

	h.jsonResponse(w, map[string]string{"status": "revoked"})
}

func (h *adminHandler) listAuditEntries(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	entries, err := h.store.ListAuditEntries(100, service)
	if err != nil {
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
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
func spaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to serve from embedded assets
		webRoot, err := fs.Sub(WebAssets, "web/build")
		if err != nil {
			// No embedded assets yet - serve placeholder
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
			return
		}

		// Serve static file or fall back to index.html
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Try to open the file
		f, err := webRoot.Open(path[1:]) // Remove leading /
		if err != nil {
			// Fall back to index.html for SPA routing
			f, err = webRoot.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}
		defer f.Close()

		// Get file info for content type
		stat, _ := f.Stat()
		if stat.IsDir() {
			f, _ = webRoot.Open("index.html")
			stat, _ = f.Stat()
		}

		// Serve the file
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(http.File))
	}
}
