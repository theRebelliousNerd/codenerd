package sessionlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
)

// installStore installs a store over a temp workspace for the test, with the
// exchange switch under the test's control.
func installStore(t *testing.T, session string, enabled bool) (*Store, *atomic.Bool) {
	t.Helper()
	var on atomic.Bool
	on.Store(enabled)
	s := NewStore(Options{Workspace: t.TempDir(), Session: session, Enabled: on.Load})
	Install(s)
	t.Cleanup(func() { Install(nil) })
	return s, &on
}

func sessionPath(s *Store, session string, parts ...string) string {
	return filepath.Join(append([]string{SessionsDir(s.workspace), session}, parts...)...)
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s is not valid JSON: %v\n%s", path, err, data)
	}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFileBytes(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }
