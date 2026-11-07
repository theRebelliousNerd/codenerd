package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

// IsDebugSelf returns whether debug-self mode is enabled in the effective config.
func IsDebugSelf() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.DebugSelf
}

// WriteGoroutineDump writes the stack of every goroutine to a new file in the
// logs directory and returns its path. It does not stop the process: a stalled
// turn is diagnosed where it stands, not killed for a SIGQUIT dump that a TUI's
// raw terminal mangles and its pane loses (2026-11-07). reason heads the file.
func WriteGoroutineDump(reason string) (string, error) {
	configMu.RLock()
	dir := logsDir
	configMu.RUnlock()
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("goroutines-%s.txt", time.Now().Format("20060102-150405.000")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "goroutine dump, pid %d, %s\nreason: %s\n\n", os.Getpid(), time.Now().Format(time.RFC3339), reason); err != nil {
		return "", err
	}
	if err := pprof.Lookup("goroutine").WriteTo(f, 2); err != nil {
		return "", err
	}
	return path, nil
}
