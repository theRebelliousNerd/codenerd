package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// AmbientFile is the editor bridge's file under .nerd (AmbientContext).
const AmbientFile = "ambient.json"

// ReadAmbient reads nerdDir/ambient.json. It returns nil with no error when
// there is no file, when the file is older than maxAge (the bridge that kept
// it fresh has stopped, and a stale cursor is worse than none), or when it
// reports nothing. A file that does not parse, carries an unknown key or a
// negative cursor line is an error.
func ReadAmbient(nerdDir string, now time.Time, maxAge time.Duration) (*AmbientContext, error) {
	path := filepath.Join(nerdDir, AmbientFile)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ambient context: %w", err)
	}
	if maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ambient context: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var amb AmbientContext
	if err := dec.Decode(&amb); err != nil {
		return nil, fmt.Errorf("ambient context %s: %w", path, err)
	}
	if amb.CursorLine < 0 {
		return nil, fmt.Errorf("ambient context %s: cursor_line %d is negative", path, amb.CursorLine)
	}
	if amb.ActiveFile == "" && amb.CursorLine == 0 && amb.SelectedText == "" && len(amb.Diagnostics) == 0 {
		return nil, nil
	}
	return &amb, nil
}
