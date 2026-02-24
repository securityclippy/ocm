package store

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "ocm-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Valid master key (32 bytes)
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	// Test successful creation
	s, err := New(tmpFile.Name(), masterKey)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// Test invalid key length
	_, err = New(tmpFile.Name(), []byte("short"))
	if err == nil {
		t.Error("New() with short key should error")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ocm-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	s, err := New(tmpFile.Name(), masterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Test encrypt/decrypt roundtrip
	plaintext := []byte("super secret token value")
	encrypted, err := s.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	// Encrypted should be longer (nonce + ciphertext + tag)
	if len(encrypted) <= len(plaintext) {
		t.Error("encrypted should be longer than plaintext")
	}

	decrypted, err := s.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypt() = %s, want %s", decrypted, plaintext)
	}
}

func TestCredentialCRUD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ocm-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	s, err := New(tmpFile.Name(), masterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create credential
	cred := &Credential{
		ID:          "cred-1",
		Service:     "gmail",
		DisplayName: "Gmail (Personal)",
		Type:        "oauth2",
		Scopes: map[string]*Scope{
			"read": {
				Name:      "read",
				Permanent: true,
				Token:     "read-token-123",
			},
			"write": {
				Name:             "write",
				Permanent:        false,
				RequiresApproval: true,
				MaxTTL:           time.Hour,
				Token:            "write-token-456",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save
	if err := s.SaveCredential(cred); err != nil {
		t.Fatalf("SaveCredential() error = %v", err)
	}

	// Get
	got, err := s.GetCredential("gmail")
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetCredential() returned nil")
	}
	if got.Service != "gmail" {
		t.Errorf("GetCredential().Service = %s, want gmail", got.Service)
	}
	if got.Scopes["read"].Token != "read-token-123" {
		t.Errorf("GetCredential().Scopes[read].Token = %s, want read-token-123", got.Scopes["read"].Token)
	}
	if got.Scopes["write"].RequiresApproval != true {
		t.Error("GetCredential().Scopes[write].RequiresApproval should be true")
	}

	// List
	list, err := s.ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListCredentials() len = %d, want 1", len(list))
	}
	// Tokens should be cleared in list
	if list[0].Scopes["read"].Token != "" {
		t.Error("ListCredentials() should not include tokens")
	}

	// Delete
	if err := s.DeleteCredential("gmail"); err != nil {
		t.Fatalf("DeleteCredential() error = %v", err)
	}
	got, err = s.GetCredential("gmail")
	if err != nil {
		t.Fatalf("GetCredential() after delete error = %v", err)
	}
	if got != nil {
		t.Error("GetCredential() after delete should return nil")
	}
}

func TestElevationWorkflow(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ocm-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	s, err := New(tmpFile.Name(), masterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// First create a credential (required for foreign key)
	cred := &Credential{
		ID:          "cred-1",
		Service:     "gmail",
		DisplayName: "Gmail",
		Type:        "oauth2",
		Scopes:      map[string]*Scope{"write": {Name: "write", RequiresApproval: true}},
	}
	if err := s.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}

	// Create elevation request
	elev := &Elevation{
		ID:          "elev-1",
		Service:     "gmail",
		Scope:       "write",
		Reason:      "Send project update",
		Status:      "pending",
		RequestedAt: time.Now(),
	}
	if err := s.CreateElevation(elev); err != nil {
		t.Fatalf("CreateElevation() error = %v", err)
	}

	// Get pending
	pending, err := s.ListPendingElevations()
	if err != nil {
		t.Fatalf("ListPendingElevations() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("ListPendingElevations() len = %d, want 1", len(pending))
	}

	// Approve
	expiresAt := time.Now().Add(30 * time.Minute)
	if err := s.UpdateElevation("elev-1", "approved", "admin:testuser", &expiresAt); err != nil {
		t.Fatalf("UpdateElevation() error = %v", err)
	}

	// Check active
	active, err := s.GetActiveElevation("gmail", "write")
	if err != nil {
		t.Fatalf("GetActiveElevation() error = %v", err)
	}
	if active == nil {
		t.Fatal("GetActiveElevation() should return approved elevation")
	}
	if active.Status != "approved" {
		t.Errorf("GetActiveElevation().Status = %s, want approved", active.Status)
	}

	// Pending should now be empty
	pending, err = s.ListPendingElevations()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("ListPendingElevations() after approval len = %d, want 0", len(pending))
	}
}

func TestAuditLog(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ocm-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	s, err := New(tmpFile.Name(), masterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Add entries
	entry1 := &AuditEntry{
		ID:        "audit-1",
		Timestamp: time.Now(),
		Action:    "credential_access",
		Service:   "gmail",
		Scope:     "read",
		Actor:     "agent",
	}
	entry2 := &AuditEntry{
		ID:        "audit-2",
		Timestamp: time.Now(),
		Action:    "elevation_approved",
		Service:   "gmail",
		Scope:     "write",
		Actor:     "admin:testuser",
	}

	if err := s.AddAuditEntry(entry1); err != nil {
		t.Fatalf("AddAuditEntry() error = %v", err)
	}
	if err := s.AddAuditEntry(entry2); err != nil {
		t.Fatalf("AddAuditEntry() error = %v", err)
	}

	// List all
	entries, err := s.ListAuditEntries(10, "")
	if err != nil {
		t.Fatalf("ListAuditEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("ListAuditEntries() len = %d, want 2", len(entries))
	}

	// List filtered by service
	entries, err = s.ListAuditEntries(10, "gmail")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("ListAuditEntries(gmail) len = %d, want 2", len(entries))
	}
}
