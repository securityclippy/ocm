// Package store provides encrypted credential storage using SQLite.
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages encrypted credential storage.
type Store struct {
	db        *sql.DB
	masterKey []byte
	gcm       cipher.AEAD
	mu        sync.RWMutex
}

// Credential represents a stored credential with read and optional read-write access.
type Credential struct {
	ID          string       `json:"id"`
	Service     string       `json:"service"`
	DisplayName string       `json:"displayName"`
	Type        string       `json:"type"` // oauth2, token, pat, api_key

	// Read access - always available, injected permanently
	Read *AccessLevel `json:"read"`

	// ReadWrite access - requires elevation, injected temporarily (optional)
	ReadWrite *AccessLevel `json:"readWrite,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// InjectionType specifies where a credential gets written.
type InjectionType string

const (
	InjectionEnv    InjectionType = "env"    // Write to .env file
	InjectionConfig InjectionType = "config" // Patch OpenClaw config file
)

// AccessLevel represents a single access level (read or read-write).
type AccessLevel struct {
	// Primary injection target - where to write the main credential
	InjectionType InjectionType `json:"injectionType,omitempty"` // "env" (default) or "config"
	EnvVar        string        `json:"envVar,omitempty"`        // For env injection: var name (e.g., "GITHUB_TOKEN")
	ConfigPath    string        `json:"configPath,omitempty"`    // For config injection: JSON path (e.g., "channels.slack.userToken")

	Token        string        `json:"token,omitempty"`        // The credential value (encrypted at rest)
	RefreshToken string        `json:"refreshToken,omitempty"` // For OAuth refresh
	ExpiresAt    *time.Time    `json:"expiresAt,omitempty"`    // Token expiration (not elevation)
	MaxTTL       time.Duration `json:"maxTTL,omitempty"`       // Max elevation duration (only for ReadWrite)

	// Additional fields that get injected alongside the primary token
	// Used for multi-field credentials like Slack (userToken + cookie)
	AdditionalFields []AdditionalField `json:"additionalFields,omitempty"`
}

// AdditionalField is an extra field that gets injected with the primary token.
type AdditionalField struct {
	Name          string        `json:"name"`                    // Field identifier (e.g., "cookie")
	InjectionType InjectionType `json:"injectionType,omitempty"` // "env" or "config"
	EnvVar        string        `json:"envVar,omitempty"`        // For env injection
	ConfigPath    string        `json:"configPath,omitempty"`    // For config injection
	Value         string        `json:"value"`                   // The field value (encrypted at rest)
}

// GetInjectionType returns the injection type, defaulting to "env" for backwards compat.
func (a *AccessLevel) GetInjectionType() InjectionType {
	if a.InjectionType == "" {
		return InjectionEnv
	}
	return a.InjectionType
}

// GetInjectionKey returns the env var or config path depending on injection type.
func (a *AccessLevel) GetInjectionKey() string {
	if a.GetInjectionType() == InjectionConfig {
		return a.ConfigPath
	}
	return a.EnvVar
}

// Legacy Scope for migration compatibility
type Scope struct {
	Name             string        `json:"name"`
	EnvVar           string        `json:"envVar"`
	Permanent        bool          `json:"permanent"`
	RequiresApproval bool          `json:"requiresApproval"`
	MaxTTL           time.Duration `json:"maxTTL"`
	Token            string        `json:"token,omitempty"`
	RefreshToken     string        `json:"refreshToken,omitempty"`
	ExpiresAt        *time.Time    `json:"expiresAt,omitempty"`
}

// Elevation represents an active elevation grant.
type Elevation struct {
	ID          string    `json:"id"`
	Service     string    `json:"service"`
	Scope       string    `json:"scope"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"` // pending, approved, denied, expired, revoked
	RequestedAt time.Time `json:"requestedAt"`
	ApprovedAt  *time.Time `json:"approvedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	ApprovedBy  string    `json:"approvedBy,omitempty"`
}

// AuditEntry represents an audit log entry.
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // credential_access, elevation_request, elevation_approved, etc.
	Service   string    `json:"service,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	Details   string    `json:"details,omitempty"`
	Actor     string    `json:"actor"` // agent, admin:<user>, system
}

// New creates a new Store with the given database path and master key.
func New(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes")
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{
		db:        db,
		masterKey: masterKey,
		gcm:       gcm,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs database migrations.
func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS credentials (
			id TEXT PRIMARY KEY,
			service TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			type TEXT NOT NULL,
			scopes_encrypted BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS elevations (
			id TEXT PRIMARY KEY,
			service TEXT NOT NULL,
			scope TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			approved_at DATETIME,
			expires_at DATETIME,
			approved_by TEXT,
			FOREIGN KEY (service) REFERENCES credentials(service)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_elevations_status ON elevations(status)`,
		`CREATE INDEX IF NOT EXISTS idx_elevations_service_scope ON elevations(service, scope)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			action TEXT NOT NULL,
			service TEXT,
			scope TEXT,
			details TEXT,
			actor TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_service ON audit_log(service)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}
	return nil
}

// encrypt encrypts data using AES-GCM.
func (s *Store) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return s.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts data using AES-GCM.
func (s *Store) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < s.gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:s.gcm.NonceSize()], ciphertext[s.gcm.NonceSize():]
	return s.gcm.Open(nil, nonce, ciphertext, nil)
}

// credentialData is the internal storage format for credentials.
type credentialData struct {
	Read      *AccessLevel `json:"read"`
	ReadWrite *AccessLevel `json:"readWrite,omitempty"`
}

// SaveCredential saves or updates a credential.
func (s *Store) SaveCredential(cred *Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize and encrypt access levels
	data := credentialData{
		Read:      cred.Read,
		ReadWrite: cred.ReadWrite,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal credential data: %w", err)
	}
	encrypted, err := s.encrypt(dataJSON)
	if err != nil {
		return fmt.Errorf("encrypt credential data: %w", err)
	}

	now := time.Now()
	_, err = s.db.Exec(`
		INSERT INTO credentials (id, service, display_name, type, scopes_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(service) DO UPDATE SET
			display_name = excluded.display_name,
			type = excluded.type,
			scopes_encrypted = excluded.scopes_encrypted,
			updated_at = excluded.updated_at
	`, cred.ID, cred.Service, cred.DisplayName, cred.Type, encrypted, now, now)
	if err != nil {
		return fmt.Errorf("save credential: %w", err)
	}

	return nil
}

// GetCredential retrieves a credential by service name.
func (s *Store) GetCredential(service string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cred Credential
	var encrypted []byte
	err := s.db.QueryRow(`
		SELECT id, service, display_name, type, scopes_encrypted, created_at, updated_at
		FROM credentials WHERE service = ?
	`, service).Scan(&cred.ID, &cred.Service, &cred.DisplayName, &cred.Type, &encrypted, &cred.CreatedAt, &cred.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query credential: %w", err)
	}

	// Decrypt and deserialize
	decrypted, err := s.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential data: %w", err)
	}

	// Try new format first
	var data credentialData
	if err := json.Unmarshal(decrypted, &data); err == nil && data.Read != nil {
		cred.Read = data.Read
		cred.ReadWrite = data.ReadWrite
		return &cred, nil
	}

	// Fall back to legacy scopes format for migration
	var scopes map[string]*Scope
	if err := json.Unmarshal(decrypted, &scopes); err != nil {
		return nil, fmt.Errorf("unmarshal credential data: %w", err)
	}

	// Convert legacy scopes to new format
	for name, scope := range scopes {
		if scope.Permanent || !scope.RequiresApproval {
			// This was a permanent/read scope
			if cred.Read == nil {
				cred.Read = &AccessLevel{
					EnvVar:       scope.EnvVar,
					Token:        scope.Token,
					RefreshToken: scope.RefreshToken,
					ExpiresAt:    scope.ExpiresAt,
				}
			}
		} else {
			// This was an elevation-required scope
			if cred.ReadWrite == nil {
				cred.ReadWrite = &AccessLevel{
					EnvVar:       scope.EnvVar,
					Token:        scope.Token,
					RefreshToken: scope.RefreshToken,
					ExpiresAt:    scope.ExpiresAt,
					MaxTTL:       scope.MaxTTL,
				}
			}
		}
		_ = name // Silence unused warning
	}

	return &cred, nil
}

// ListCredentials returns all credentials (without decrypted tokens).
func (s *Store) ListCredentials() ([]*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, service, display_name, type, scopes_encrypted, created_at, updated_at
		FROM credentials ORDER BY service
	`)
	if err != nil {
		return nil, fmt.Errorf("query credentials: %w", err)
	}
	defer rows.Close()

	var creds []*Credential
	for rows.Next() {
		var cred Credential
		var encrypted []byte
		if err := rows.Scan(&cred.ID, &cred.Service, &cred.DisplayName, &cred.Type, &encrypted, &cred.CreatedAt, &cred.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}

		decrypted, err := s.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt credential data: %w", err)
		}

		// Try new format first
		var data credentialData
		if err := json.Unmarshal(decrypted, &data); err == nil && data.Read != nil {
			cred.Read = data.Read
			cred.ReadWrite = data.ReadWrite
		} else {
			// Fall back to legacy format
			var scopes map[string]*Scope
			if err := json.Unmarshal(decrypted, &scopes); err == nil {
				for _, scope := range scopes {
					if scope.Permanent || !scope.RequiresApproval {
						if cred.Read == nil {
							cred.Read = &AccessLevel{
								EnvVar: scope.EnvVar,
							}
						}
					} else {
						if cred.ReadWrite == nil {
							cred.ReadWrite = &AccessLevel{
								EnvVar: scope.EnvVar,
								MaxTTL: scope.MaxTTL,
							}
						}
					}
				}
			}
		}

		// Clear tokens for list view (security)
		if cred.Read != nil {
			cred.Read.Token = ""
			cred.Read.RefreshToken = ""
		}
		if cred.ReadWrite != nil {
			cred.ReadWrite.Token = ""
			cred.ReadWrite.RefreshToken = ""
		}

		creds = append(creds, &cred)
	}

	return creds, rows.Err()
}

// DeleteCredential removes a credential.
func (s *Store) DeleteCredential(service string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM credentials WHERE service = ?`, service)
	return err
}

// CreateElevation creates a new elevation request.
func (s *Store) CreateElevation(elev *Elevation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO elevations (id, service, scope, reason, status, requested_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, elev.ID, elev.Service, elev.Scope, elev.Reason, elev.Status, elev.RequestedAt)
	return err
}

// GetElevation retrieves an elevation by ID.
func (s *Store) GetElevation(id string) (*Elevation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var elev Elevation
	var approvedAt, expiresAt sql.NullTime
	var approvedBy sql.NullString
	err := s.db.QueryRow(`
		SELECT id, service, scope, reason, status, requested_at, approved_at, expires_at, approved_by
		FROM elevations WHERE id = ?
	`, id).Scan(&elev.ID, &elev.Service, &elev.Scope, &elev.Reason, &elev.Status,
		&elev.RequestedAt, &approvedAt, &expiresAt, &approvedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if approvedAt.Valid {
		elev.ApprovedAt = &approvedAt.Time
	}
	if expiresAt.Valid {
		elev.ExpiresAt = &expiresAt.Time
	}
	if approvedBy.Valid {
		elev.ApprovedBy = approvedBy.String
	}
	return &elev, nil
}

// GetActiveElevation returns an active (approved, not expired) elevation for a service/scope.
func (s *Store) GetActiveElevation(service, scope string) (*Elevation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var elev Elevation
	var approvedAt, expiresAt sql.NullTime
	var approvedBy sql.NullString
	err := s.db.QueryRow(`
		SELECT id, service, scope, reason, status, requested_at, approved_at, expires_at, approved_by
		FROM elevations 
		WHERE service = ? AND scope = ? AND status = 'approved' AND expires_at > datetime('now')
		ORDER BY expires_at DESC LIMIT 1
	`, service, scope).Scan(&elev.ID, &elev.Service, &elev.Scope, &elev.Reason, &elev.Status,
		&elev.RequestedAt, &approvedAt, &expiresAt, &approvedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if approvedAt.Valid {
		elev.ApprovedAt = &approvedAt.Time
	}
	if expiresAt.Valid {
		elev.ExpiresAt = &expiresAt.Time
	}
	if approvedBy.Valid {
		elev.ApprovedBy = approvedBy.String
	}
	return &elev, nil
}

// UpdateElevation updates an elevation's status.
func (s *Store) UpdateElevation(id string, status string, approvedBy string, expiresAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE elevations 
		SET status = ?, approved_at = ?, expires_at = ?, approved_by = ?
		WHERE id = ?
	`, status, now, expiresAt, approvedBy, id)
	return err
}

// ListPendingElevations returns all pending elevation requests.
func (s *Store) ListPendingElevations() ([]*Elevation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, service, scope, reason, status, requested_at, approved_at, expires_at, approved_by
		FROM elevations WHERE status = 'pending' ORDER BY requested_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var elevs []*Elevation
	for rows.Next() {
		var elev Elevation
		var approvedAt, expiresAt sql.NullTime
		var approvedBy sql.NullString
		if err := rows.Scan(&elev.ID, &elev.Service, &elev.Scope, &elev.Reason, &elev.Status,
			&elev.RequestedAt, &approvedAt, &expiresAt, &approvedBy); err != nil {
			return nil, err
		}
		if approvedAt.Valid {
			elev.ApprovedAt = &approvedAt.Time
		}
		if expiresAt.Valid {
			elev.ExpiresAt = &expiresAt.Time
		}
		if approvedBy.Valid {
			elev.ApprovedBy = approvedBy.String
		}
		elevs = append(elevs, &elev)
	}
	return elevs, rows.Err()
}

// AddAuditEntry adds an entry to the audit log.
func (s *Store) AddAuditEntry(entry *AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO audit_log (id, timestamp, action, service, scope, details, actor)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, entry.Timestamp, entry.Action, entry.Service, entry.Scope, entry.Details, entry.Actor)
	return err
}

// ListAuditEntries returns recent audit entries.
func (s *Store) ListAuditEntries(limit int, service string) ([]*AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, timestamp, action, service, scope, details, actor FROM audit_log`
	args := []interface{}{}
	if service != "" {
		query += ` WHERE service = ?`
		args = append(args, service)
	}
	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Action, &entry.Service,
			&entry.Scope, &entry.Details, &entry.Actor); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}
