package credentials

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
)

// LoadCredentials decrypts and loads AWS credentials from encrypted files
// If credentials are already in environment, skips decryption
func LoadCredentials() error {
	logger := logging.GetDefaultLogger()

	// Check if credentials are already in environment
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" &&
		os.Getenv("AWS_SECRET_ACCESS_KEY") != "" &&
		os.Getenv("AWS_REGION") != "" {
		logger.Info("AWS credentials loaded from environment variables")
		return nil // Already loaded
	}

	// Get credentials directory from environment or use default
	credsDir := os.Getenv("CREDENTIALS_DIR")
	if credsDir == "" {
		credsDir = "/etc/systemd/creds/s3-streamer"
	}

	// Check if credentials directory exists
	if _, err := os.Stat(credsDir); os.IsNotExist(err) {
		// No encrypted credentials, rely on environment or AWS config
		logger.Warn("No encrypted credentials found, relying on environment or AWS config",
			"credentials_dir", credsDir)
		return nil
	}

	logger.Info("Loading encrypted credentials",
		"credentials_dir", credsDir)

	// Generate machine-specific decryption key
	machineID, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return fmt.Errorf("failed to read machine-id: %w", err)
	}

	// Create encryption key from machine-id + salt
	salt := "s3-edgedelta-streamer-v1"
	keyData := string(machineID) + salt
	keyHash := sha256.Sum256([]byte(keyData))
	encKey := fmt.Sprintf("%x", keyHash)

	logger.Debug("Generated decryption key from machine-id")

	// Decrypt and load each credential
	accessKey, err := decryptCredential(credsDir, "aws_access_key_id", encKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt access key: %w", err)
	}

	secretKey, err := decryptCredential(credsDir, "aws_secret_access_key", encKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret key: %w", err)
	}

	region, err := decryptCredential(credsDir, "aws_region", encKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt region: %w", err)
	}

	// Set environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
	os.Setenv("AWS_REGION", region)

	logger.Info("Successfully loaded encrypted AWS credentials")

	return nil
}

// decryptCredential decrypts a single credential file using OpenSSL
func decryptCredential(credsDir, name, key string) (string, error) {
	credFile := fmt.Sprintf("%s/%s", credsDir, name)

	// Check if file exists
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		return "", fmt.Errorf("credential file not found: %s", credFile)
	}

	// Decrypt using openssl
	cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-d", "-pbkdf2",
		"-pass", fmt.Sprintf("pass:%s", key),
		"-in", credFile)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Trim whitespace and newlines
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("decrypted value is empty")
	}

	return value, nil
}
