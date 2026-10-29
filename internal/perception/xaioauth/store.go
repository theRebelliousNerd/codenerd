package xaioauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadCredentials reads the codeNERD OAuth credential file.
func LoadCredentials(path string) (*Credentials, error) {
	if path == "" {
		return nil, fmt.Errorf("credential path empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoCredentials
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if creds.AccessToken == "" && creds.RefreshToken == "" {
		return nil, ErrNoCredentials
	}
	return &creds, nil
}

// SaveCredentials writes credentials atomically with restrictive permissions.
func SaveCredentials(path string, creds *Credentials) error {
	if path == "" {
		return fmt.Errorf("credential path empty")
	}
	if creds == nil {
		return fmt.Errorf("credentials nil")
	}
	creds.UpdatedAt = time.Now().UTC()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename credentials: %w", err)
	}
	return nil
}

// DeleteCredentials removes the credential file if present.
func DeleteCredentials(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
