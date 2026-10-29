package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const privateConfigMode os.FileMode = 0o600

// decodeStrictJSON rejects unknown fields and trailing JSON values. Configuration
// is executable policy; accepting misspelled or obsolete keys would silently run
// with a different policy than the operator intended.
func decodeStrictJSON(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

// writePrivateFileAtomically replaces path only after a complete, synced write
// in the same directory. The existing config remains intact on every pre-rename
// failure, and the replacement is owner-readable/writable because it can contain
// API keys and OAuth material.
func writePrivateFileAtomically(path string, data []byte) error {
	return writePrivateFileAtomicallyWithReplace(path, data, replacePrivateFile)
}

func writePrivateFileAtomicallyWithReplace(path string, data []byte, replace func(string, string) error) error {
	if replace == nil {
		return fmt.Errorf("replace function is nil")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".codenerd-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(privateConfigMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := replace(tmpPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	keepTemp = false
	if err := os.Chmod(path, privateConfigMode); err != nil {
		return fmt.Errorf("secure config: %w", err)
	}
	return nil
}
