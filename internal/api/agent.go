// Package api provides HTTP handlers for OCM.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openclaw/ocm/internal/store"
)

// NewAgentRouter creates the router for agent API (internal, :9999).
// This API has a constrained surface area - agents can only:
// - Request elevation
// - Check elevation status
// - Get credentials (if permanent or elevated)
// - List available scopes
func NewAgentRouter(db *store.Store, logger *slog.Logger) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &agentHandler{store: db, logger: logger}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/elevate", h.requestElevation)
		r.Get("/elevate/{id}", h.getElevationStatus)
		r.Get("/credentials/{service}/{scope}", h.getCredential)
		r.Get("/scopes", h.listScopes)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return r
}

type agentHandler struct {
	store  *store.Store
	logger *slog.Logger
}

// ElevationRequest is the request body for POST /elevate.
type ElevationRequest struct {
	Service      string `json:"service"`
	Scope        string `json:"scope"`
	Reason       string `json:"reason"`
	RequestedTTL string `json:"requestedTTL,omitempty"` // e.g., "30m", "1h"
}

// ElevationResponse is the response for elevation requests.
type ElevationResponse struct {
	RequestID string     `json:"requestId"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// CredentialResponse is the response for credential requests.
type CredentialResponse struct {
	Token        string     `json:"token,omitempty"`
	RefreshToken string     `json:"refreshToken,omitempty"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

// ScopesResponse is the response for listing available scopes.
type ScopesResponse struct {
	Services []ServiceScopes `json:"services"`
}

// ServiceScopes describes available scopes for a service.
type ServiceScopes struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Scopes      []string `json:"scopes"`
	Elevated    []string `json:"elevated"` // Currently elevated scopes
}

func (h *agentHandler) requestElevation(w http.ResponseWriter, r *http.Request) {
	var req ElevationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Service == "" {
		h.jsonError(w, "service is required", http.StatusBadRequest)
		return
	}
	// Scope is now always "write" or "readwrite" for the new model
	if req.Scope == "" {
		req.Scope = "write"
	}

	// Check credential exists
	cred, err := h.store.GetCredential(req.Service)
	if err != nil {
		h.logger.Error("get credential failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cred == nil {
		h.jsonError(w, "service not found", http.StatusNotFound)
		return
	}

	// Check if read-write access is configured
	if cred.ReadWrite == nil {
		h.jsonError(w, "service has no write access configured", http.StatusBadRequest)
		return
	}

	// Check if already elevated
	active, err := h.store.GetActiveElevation(req.Service, req.Scope)
	if err != nil {
		h.logger.Error("get active elevation failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if active != nil {
		h.jsonResponse(w, ElevationResponse{
			RequestID: active.ID,
			Status:    "approved",
			ExpiresAt: active.ExpiresAt,
		})
		return
	}

	// Create elevation request
	elev := &store.Elevation{
		ID:          generateID("elev"),
		Service:     req.Service,
		Scope:       req.Scope,
		Reason:      req.Reason,
		Status:      "pending",
		RequestedAt: time.Now(),
	}

	if err := h.store.CreateElevation(elev); err != nil {
		h.logger.Error("create elevation failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_requested",
		Service:   req.Service,
		Scope:     req.Scope,
		Details:   req.Reason,
		Actor:     "agent",
	})

	// TODO: Send push notification

	h.logger.Info("elevation requested",
		"request_id", elev.ID,
		"service", req.Service,
		"scope", req.Scope,
	)

	h.jsonResponse(w, ElevationResponse{
		RequestID: elev.ID,
		Status:    "pending",
	})
}

func (h *agentHandler) getElevationStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.jsonError(w, "id is required", http.StatusBadRequest)
		return
	}

	elev, err := h.store.GetElevation(id)
	if err != nil {
		h.logger.Error("get elevation failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if elev == nil {
		h.jsonError(w, "elevation not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, ElevationResponse{
		RequestID: elev.ID,
		Status:    elev.Status,
		ExpiresAt: elev.ExpiresAt,
	})
}

func (h *agentHandler) getCredential(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	scopeName := chi.URLParam(r, "scope") // "read" or "write"

	cred, err := h.store.GetCredential(service)
	if err != nil {
		h.logger.Error("get credential failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cred == nil {
		h.jsonError(w, "service not found", http.StatusNotFound)
		return
	}

	var accessLevel *store.AccessLevel

	switch scopeName {
	case "read", "r":
		// Read access is always available
		if cred.Read == nil {
			h.jsonError(w, "no read access configured", http.StatusNotFound)
			return
		}
		accessLevel = cred.Read

	case "write", "rw", "readwrite":
		// Write access requires elevation
		if cred.ReadWrite == nil {
			h.jsonError(w, "no write access configured", http.StatusNotFound)
			return
		}
		// Check for active elevation
		active, err := h.store.GetActiveElevation(service, scopeName)
		if err != nil {
			h.logger.Error("get active elevation failed", "error", err)
			h.jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}
		if active == nil {
			h.jsonError(w, "elevation required for write access", http.StatusForbidden)
			return
		}
		accessLevel = cred.ReadWrite

	default:
		h.jsonError(w, "scope must be 'read' or 'write'", http.StatusBadRequest)
		return
	}

	// Audit log
	h.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "credential_access",
		Service:   service,
		Scope:     scopeName,
		Actor:     "agent",
	})

	h.jsonResponse(w, CredentialResponse{
		Token:        accessLevel.Token,
		RefreshToken: accessLevel.RefreshToken,
		ExpiresAt:    accessLevel.ExpiresAt,
	})
}

func (h *agentHandler) listScopes(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.ListCredentials()
	if err != nil {
		h.logger.Error("list credentials failed", "error", err)
		h.jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := ScopesResponse{
		Services: make([]ServiceScopes, 0, len(creds)),
	}

	for _, cred := range creds {
		svc := ServiceScopes{
			ID:          cred.Service,
			DisplayName: cred.DisplayName,
			Scopes:      []string{},
			Elevated:    []string{},
		}

		// Add "read" if available
		if cred.Read != nil {
			svc.Scopes = append(svc.Scopes, "read")
		}

		// Add "write" if available
		if cred.ReadWrite != nil {
			svc.Scopes = append(svc.Scopes, "write")

			// Check if currently elevated
			active, _ := h.store.GetActiveElevation(cred.Service, "write")
			if active != nil {
				svc.Elevated = append(svc.Elevated, "write")
			}
		}

		resp.Services = append(resp.Services, svc)
	}

	h.jsonResponse(w, resp)
}

func (h *agentHandler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *agentHandler) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
