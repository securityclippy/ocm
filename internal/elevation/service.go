// Package elevation manages the credential elevation workflow.
package elevation

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/openclaw/ocm/internal/gateway"
	"github.com/openclaw/ocm/internal/store"
)

// Service manages credential elevation and injection.
type Service struct {
	store   *store.Store
	gateway *gateway.Client
	logger  *slog.Logger

	// expiryTimers tracks active elevation expiry timers
	expiryTimers map[string]*time.Timer
	mu           sync.Mutex
}

// NewService creates a new elevation service.
func NewService(s *store.Store, g *gateway.Client, logger *slog.Logger) *Service {
	svc := &Service{
		store:        s,
		gateway:      g,
		logger:       logger,
		expiryTimers: make(map[string]*time.Timer),
	}
	
	// On startup, sync current state to Gateway
	svc.syncCredentialsToGateway()
	
	return svc
}

// ApproveElevation approves an elevation request and injects the credential.
func (s *Service) ApproveElevation(elevationID string, ttl time.Duration, approvedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get the elevation request
	elev, err := s.store.GetElevation(elevationID)
	if err != nil {
		return fmt.Errorf("get elevation: %w", err)
	}
	if elev == nil {
		return fmt.Errorf("elevation not found")
	}
	if elev.Status != "pending" {
		return fmt.Errorf("elevation not pending (status: %s)", elev.Status)
	}

	// Get the credential
	cred, err := s.store.GetCredential(elev.Service)
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}
	if cred == nil {
		return fmt.Errorf("credential not found")
	}

	scope, ok := cred.Scopes[elev.Scope]
	if !ok {
		return fmt.Errorf("scope not found")
	}

	// Enforce maxTTL
	if scope.MaxTTL > 0 && ttl > scope.MaxTTL {
		ttl = scope.MaxTTL
	}

	// Update elevation status
	expiresAt := time.Now().Add(ttl)
	if err := s.store.UpdateElevation(elevationID, "approved", approvedBy, &expiresAt); err != nil {
		return fmt.Errorf("update elevation: %w", err)
	}

	// Inject credential into Gateway
	if err := s.injectCredential(cred, scope); err != nil {
		// Rollback elevation status on failure
		s.store.UpdateElevation(elevationID, "pending", "", nil)
		return fmt.Errorf("inject credential: %w", err)
	}

	// Set up expiry timer
	s.setExpiryTimer(elevationID, elev.Service, elev.Scope, ttl)

	// Audit log
	s.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_approved",
		Service:   elev.Service,
		Scope:     elev.Scope,
		Details:   fmt.Sprintf("TTL: %s, approved by: %s", ttl, approvedBy),
		Actor:     approvedBy,
	})

	s.logger.Info("elevation approved",
		"elevation_id", elevationID,
		"service", elev.Service,
		"scope", elev.Scope,
		"ttl", ttl,
	)

	return nil
}

// RevokeElevation revokes an active elevation and removes the credential.
func (s *Service) RevokeElevation(service, scope string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get active elevation
	active, err := s.store.GetActiveElevation(service, scope)
	if err != nil {
		return fmt.Errorf("get active elevation: %w", err)
	}
	if active == nil {
		return fmt.Errorf("no active elevation")
	}

	// Cancel expiry timer
	timerKey := fmt.Sprintf("%s:%s", service, scope)
	if timer, ok := s.expiryTimers[timerKey]; ok {
		timer.Stop()
		delete(s.expiryTimers, timerKey)
	}

	// Update elevation status
	if err := s.store.UpdateElevation(active.ID, "revoked", "admin", nil); err != nil {
		return fmt.Errorf("update elevation: %w", err)
	}

	// Remove credential from Gateway (or downgrade to permanent scope)
	if err := s.removeOrDowngradeCredential(service, scope); err != nil {
		return fmt.Errorf("remove credential: %w", err)
	}

	// Audit log
	s.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_revoked",
		Service:   service,
		Scope:     scope,
		Details:   reason,
		Actor:     "admin",
	})

	s.logger.Info("elevation revoked", "service", service, "scope", scope)

	return nil
}

// syncCredentialsToGateway syncs all permanent credentials to the Gateway on startup.
func (s *Service) syncCredentialsToGateway() {
	creds, err := s.store.ListCredentials()
	if err != nil {
		s.logger.Error("failed to list credentials for sync", "error", err)
		return
	}

	var envCreds []gateway.CredentialEnv
	for _, cred := range creds {
		fullCred, _ := s.store.GetCredential(cred.Service)
		if fullCred == nil {
			continue
		}
		for _, scope := range fullCred.Scopes {
			if scope.Permanent && scope.EnvVar != "" && scope.Token != "" {
				envCreds = append(envCreds, gateway.CredentialEnv{
					Name:  scope.EnvVar,
					Value: scope.Token,
				})
			}
		}
	}

	if len(envCreds) > 0 {
		if err := s.gateway.SetCredentials(envCreds); err != nil {
			s.logger.Error("failed to sync credentials to gateway", "error", err)
		} else {
			s.logger.Info("synced permanent credentials to gateway", "count", len(envCreds))
		}
	}
}

// injectCredential injects a credential into the Gateway.
func (s *Service) injectCredential(cred *store.Credential, scope *store.Scope) error {
	if scope.EnvVar == "" {
		return fmt.Errorf("scope has no env var configured")
	}
	if scope.Token == "" {
		return fmt.Errorf("scope has no token")
	}

	return s.gateway.SetCredentials([]gateway.CredentialEnv{
		{Name: scope.EnvVar, Value: scope.Token},
	})
}

// removeOrDowngradeCredential removes a credential or downgrades to a permanent scope.
func (s *Service) removeOrDowngradeCredential(service, scopeName string) error {
	cred, err := s.store.GetCredential(service)
	if err != nil || cred == nil {
		return err
	}

	scope, ok := cred.Scopes[scopeName]
	if !ok {
		return nil
	}

	// Check if there's a permanent (read-only) scope we can downgrade to
	// Convention: "read" scope is permanent, "write" scope requires elevation
	if readScope, ok := cred.Scopes["read"]; ok && readScope.Permanent && readScope.EnvVar == scope.EnvVar {
		// Downgrade to read-only token
		return s.gateway.SetCredentials([]gateway.CredentialEnv{
			{Name: readScope.EnvVar, Value: readScope.Token},
		})
	}

	// No downgrade available - clear the credential
	return s.gateway.ClearCredentials([]string{scope.EnvVar})
}

// setExpiryTimer sets a timer to auto-expire an elevation.
func (s *Service) setExpiryTimer(elevationID, service, scope string, ttl time.Duration) {
	timerKey := fmt.Sprintf("%s:%s", service, scope)

	// Cancel existing timer
	if existing, ok := s.expiryTimers[timerKey]; ok {
		existing.Stop()
	}

	// Set new timer
	s.expiryTimers[timerKey] = time.AfterFunc(ttl, func() {
		s.handleExpiry(elevationID, service, scope)
	})
}

// handleExpiry handles elevation expiry.
func (s *Service) handleExpiry(elevationID, service, scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update status
	s.store.UpdateElevation(elevationID, "expired", "", nil)

	// Remove/downgrade credential
	if err := s.removeOrDowngradeCredential(service, scope); err != nil {
		s.logger.Error("failed to remove credential on expiry", "error", err, "service", service, "scope", scope)
	}

	// Cleanup timer reference
	timerKey := fmt.Sprintf("%s:%s", service, scope)
	delete(s.expiryTimers, timerKey)

	// Audit log
	s.store.AddAuditEntry(&store.AuditEntry{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		Action:    "elevation_expired",
		Service:   service,
		Scope:     scope,
		Actor:     "system",
	})

	s.logger.Info("elevation expired", "service", service, "scope", scope)
}

// generateID creates a unique ID with prefix.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
