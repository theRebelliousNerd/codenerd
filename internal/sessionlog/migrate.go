package sessionlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateReport says what MigrateFlat did.
type MigrateReport struct {
	Moved int
	// Conflicts are flat files left in place because the session folder
	// already holds a session.json. Nothing is overwritten.
	Conflicts []string
}

// MigrateFlat moves every flat .nerd/sessions/<id>.json into
// .nerd/sessions/<id>/session.json. It runs once at startup; afterwards there
// is a single path and nothing reads the flat form. Calling it again is a
// no-op.
func MigrateFlat(workspace string) (MigrateReport, error) {
	var rep MigrateReport
	root := SessionsDir(workspace)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return rep, nil
		}
		return rep, err
	}
	var firstErr error
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if ValidateID(id) != nil {
			continue
		}
		src := filepath.Join(root, name)
		dir := filepath.Join(root, id)
		dst := filepath.Join(dir, HistoryFile)
		if _, err := os.Stat(dst); err == nil {
			rep.Conflicts = append(rep.Conflicts, src)
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			firstErr = firstOf(firstErr, fmt.Errorf("migrating %s: %w", name, err))
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			firstErr = firstOf(firstErr, fmt.Errorf("migrating %s: %w", name, err))
			continue
		}
		rep.Moved++
	}
	return rep, firstErr
}

func firstOf(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
