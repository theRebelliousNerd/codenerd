package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/sessionlog"
)

// nerd sessions wire prints the captured request body of an exchange file.
func TestSessionsWire_PrintsTheCapturedRequestBody(t *testing.T) {
	ex := map[string]any{
		"schema": sessionlog.ExchangeSchema,
		"wire": []any{map[string]any{
			"attempt": 1,
			"request": map[string]any{"body": map[string]any{"prompt": []any{"a", "b"}}, "lines_paths": []any{"/prompt"}},
		}},
	}
	data, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "0001-turn.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sessionsWireCmd.SetOut(&out)
	t.Cleanup(func() { sessionsWireCmd.SetOut(nil) })
	if err := runSessionsWire(sessionsWireCmd, []string{path}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != `{"prompt":"a\nb"}` {
		t.Fatalf("printed %q", got)
	}
	found := false
	for _, c := range sessionsCmd.Commands() {
		if c == sessionsWireCmd {
			found = true
		}
	}
	if !found {
		t.Fatal("wire is not a subcommand of nerd sessions")
	}
}
