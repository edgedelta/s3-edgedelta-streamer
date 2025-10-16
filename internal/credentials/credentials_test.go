package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCredentials_AlreadyInEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_REGION")
	}()

	// This should succeed without trying to decrypt files
	err := LoadCredentials()
	if err != nil {
		t.Errorf("LoadCredentials failed when credentials already in environment: %v", err)
	}
}

func TestLoadCredentials_NoCredentials(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("AWS_REGION")

	// Set credentials dir to non-existent path
	os.Setenv("CREDENTIALS_DIR", "/nonexistent/path")
	defer os.Unsetenv("CREDENTIALS_DIR")

	// This should succeed (no encrypted credentials, rely on AWS config)
	err := LoadCredentials()
	if err != nil {
		t.Errorf("LoadCredentials failed when no credentials available: %v", err)
	}
}

func TestDecryptCredential_FileNotFound(t *testing.T) {
	credsDir := "/nonexistent"
	name := "test_cred"
	key := "test_key"

	_, err := decryptCredential(credsDir, name, key)
	if err == nil {
		t.Error("Expected error for non-existent credential file")
	}

	expectedErrSubstring := "credential file not found"
	if !strings.Contains(err.Error(), expectedErrSubstring) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedErrSubstring, err.Error())
	}
}

func TestDecryptCredential_EmptyValue(t *testing.T) {
	// Create a temporary directory and empty file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "empty_cred")

	// Create empty file
	if err := os.WriteFile(credFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to decrypt (will fail because openssl can't decrypt empty file, but that's ok for this test)
	_, err := decryptCredential(tmpDir, "empty_cred", "test_key")
	if err == nil {
		t.Error("Expected error for empty credential file")
	}
}

func TestDecryptCredential_InvalidKey(t *testing.T) {
	// Create a temporary directory and file with some content
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "invalid_cred")

	// Create file with some dummy content (not actually encrypted)
	if err := os.WriteFile(credFile, []byte("dummy content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to decrypt with wrong key (will fail)
	_, err := decryptCredential(tmpDir, "invalid_cred", "wrong_key")
	if err == nil {
		t.Error("Expected error for invalid decryption key")
	}
}
