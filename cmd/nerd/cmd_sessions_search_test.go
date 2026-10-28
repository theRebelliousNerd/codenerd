package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/sessionlog"
)

func seedSessions(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	root := filepath.Join(ws, ".nerd", "sessions")
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("session-1/session.json", `{"created_at":"2026-10-27T10:00:00Z","messages":[{"role":"user","content":"run go test please"},{"role":"assistant","content":"ok"}]}`)
	write("session-1/coder-7/transcript.json", `{"schema":"codenerd.transcript/1","events":[{"seq":1,"turn":1,"kind":"tool_result","text":["FAIL: undefined: Frob","exit 2"]}]}`)
	write("session-2.json", `{"created_at":"2026-10-28T10:00:00Z","messages":[]}`) // flat: migrated on first use
	return ws
}

func TestSessionsListShowsIDStartAgentsAndSize(t *testing.T) {
	ws := seedSessions(t)
	if _, err := sessionlog.MigrateFlat(ws); err != nil {
		t.Fatal(err)
	}
	infos, err := sessionlog.List(ws)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeSessionList(&out, infos); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want a header and two sessions, got:\n%s", out.String())
	}
	for _, col := range []string{"SESSION", "START", "AGENTS", "SIZE"} {
		if !strings.Contains(lines[0], col) {
			t.Errorf("header lacks %s: %q", col, lines[0])
		}
	}
	if !strings.HasPrefix(lines[1], "session-1") || !strings.Contains(lines[1], "coder-7") || !strings.Contains(lines[1], " B") {
		t.Errorf("session-1 row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "session-2") || !strings.Contains(lines[2], "2026-10-28") {
		t.Errorf("session-2 row (migrated flat file): %q", lines[2])
	}
}

func TestSessionsSearchPrintsSessionAgentFileAndTurn(t *testing.T) {
	ws := seedSessions(t)
	hits, err := sessionlog.Search(ws, "undefined", sessionlog.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeSearchHits(&out, hits); err != nil {
		t.Fatal(err)
	}
	want := "session-1  coder-7  transcript.json  turn 1: FAIL: undefined: Frob\n"
	if out.String() != want {
		t.Fatalf("got %q\nwant %q", out.String(), want)
	}

	hits, _ = sessionlog.Search(ws, "go test", sessionlog.SearchOptions{Agent: "chat"})
	out.Reset()
	_ = writeSearchHits(&out, hits)
	if out.String() != "session-1  chat  session.json  turn 1: run go test please\n" {
		t.Fatalf("--agent chat: %q", out.String())
	}
	hits, _ = sessionlog.Search(ws, "go test", sessionlog.SearchOptions{Agent: "coder"})
	if len(hits) != 0 {
		t.Fatalf("--agent coder must not match the chat: %+v", hits)
	}
}

func TestSessionsSearchCommandIsRegisteredWithItsFlags(t *testing.T) {
	cmd, _, err := sessionsCmd.Find([]string{"search"})
	if err != nil || cmd.Name() != "search" {
		t.Fatalf("search subcommand missing: %v", err)
	}
	for _, f := range []string{"agent", "session"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("--%s missing", f)
		}
	}
}
