package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAmbient(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AmbientFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The editor bridge's file is read while fresh, left out when absent, stale
// or empty, and refused when it breaks the contract.
func TestReadAmbient_Contract(t *testing.T) {
	now := time.Now()
	maxAge := 15 * time.Minute

	if amb, err := ReadAmbient(t.TempDir(), now, maxAge); amb != nil || err != nil {
		t.Fatalf("no bridge gave %+v, %v", amb, err)
	}

	dir := writeAmbient(t, `{"active_file":"internal/core/kernel.go","cursor_line":120,"selected_text":"func Query","diagnostics":["kernel.go:122:9: undefined: rows"]}`)
	amb, err := ReadAmbient(dir, now, maxAge)
	if err != nil || amb == nil {
		t.Fatalf("fresh file: %+v, %v", amb, err)
	}
	if amb.ActiveFile != "internal/core/kernel.go" || amb.CursorLine != 120 || amb.SelectedText != "func Query" || len(amb.Diagnostics) != 1 {
		t.Fatalf("fields = %+v", amb)
	}

	old := now.Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, AmbientFile), old, old); err != nil {
		t.Fatal(err)
	}
	if amb, err := ReadAmbient(dir, now, maxAge); amb != nil || err != nil {
		t.Fatalf("a stale file gave %+v, %v", amb, err)
	}

	if amb, err := ReadAmbient(writeAmbient(t, `{}`), now, maxAge); amb != nil || err != nil {
		t.Fatalf("an empty report gave %+v, %v", amb, err)
	}
	for body, want := range map[string]string{
		`{"active_file":"a.go","cursor":3}`: "unknown field",
		`{"cursor_line":-2}`:                "negative",
		`not json`:                          "ambient context",
	} {
		if _, err := ReadAmbient(writeAmbient(t, body), now, maxAge); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: err = %v, want %q", body, err, want)
		}
	}
}
