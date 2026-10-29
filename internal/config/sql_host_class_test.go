package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// core_limits.sql_host_class loads, and a value that is not a host class is a
// load error naming the choices rather than a silent fall-back to the default.
func TestCoreLimits_SQLHostClass_ShouldLoadAndRefuseUnknownClasses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	cfg, err := LoadUserConfig(write("ok.json", `{"core_limits":{"sql_host_class":"laptop"}}`))
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got := cfg.GetCoreLimits().SQLHostClass; got != "laptop" {
		t.Errorf("sql_host_class = %q, want laptop", got)
	}

	_, err = LoadUserConfig(write("bad.json", `{"core_limits":{"sql_host_class":"mainframe"}}`))
	if err == nil || !strings.Contains(err.Error(), "sql_host_class") {
		t.Fatalf("an unknown host class loaded: err = %v", err)
	}
}
