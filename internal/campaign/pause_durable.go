package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A separate control file cannot be erased by an active snapshot autosave.
func pauseRequestPath(workspace, id string) (string, error) {
	name := strings.TrimPrefix(id, "/")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:") {
		return "", fmt.Errorf("invalid campaign ID %q", id)
	}
	return filepath.Join(workspace, ".nerd", "campaigns", name+".pause"), nil
}

// RequestPause signals the owner process. It deliberately never writes a stale
// copy of the campaign snapshot over tasks completed by that process.
func RequestPause(workspace, id string) error {
	path, err := pauseRequestPath(workspace, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0600)
}

// ClearPauseRequest is called for an explicit resume, before starting work.
func ClearPauseRequest(workspace, id string) error {
	path, err := pauseRequestPath(workspace, id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func watchPauseRequest(ctx context.Context, workspace, id string, cancel context.CancelFunc) {
	path, err := pauseRequestPath(workspace, id)
	if err != nil {
		cancel()
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			cancel()
			return
		} else if !os.IsNotExist(err) {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
