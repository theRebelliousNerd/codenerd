package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// check-kb names the unfilled fields of every shard-agent KB in the
// workspace: here a knowledge row with no source.
func TestCheckKBNamesAnUnfilledField(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "shards")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "probe_knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE knowledge_atoms (concept TEXT, content TEXT, source TEXT)`,
		`INSERT INTO knowledge_atoms VALUES ('c', 'body', '')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	reports, err := auditShardKBs(context.Background(), ws)
	if err != nil {
		t.Fatalf("auditShardKBs: %v", err)
	}
	var out bytes.Buffer
	n := printKBReports(&out, reports)
	if n == 0 || !strings.Contains(out.String(), "shard_agent_kb knowledge_atoms.source never_written") {
		t.Errorf("check-kb output (%d gaps):\n%s", n, out.String())
	}
}

// check-kb says what each KB knows by source, calls placeholder rows what
// they are, and shows every source attempt that did not acquire.
func TestCheckKBReportsKnowledgeBySourceAndSourceOutcomes(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "shards")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "goexpert_knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE knowledge_atoms (concept TEXT, content TEXT, source TEXT)`,
		`INSERT INTO knowledge_atoms VALUES ('agent_identity', 'You are goexpert, a specialist agent in the codeNERD ecosystem.', 'baseline_seed')`,
		`INSERT INTO knowledge_atoms VALUES ('godoc/context@std', 'context at std ...', 'godoc/context@std')`,
		`INSERT INTO knowledge_atoms VALUES ('web/go/section_0', 'Use errgroup ...', 'web/openai')`,
		`CREATE TABLE knowledge_acquisitions (source TEXT, subject TEXT, status TEXT, atoms INTEGER, detail TEXT, attempted_at TEXT)`,
		`INSERT INTO knowledge_acquisitions VALUES ('web', 'go', 'acquired', 1, '', '2026-10-29T00:00:00Z')`,
		`INSERT INTO knowledge_acquisitions VALUES ('context7', 'go', 'failed', 0, 'context7 answered 429', '2026-10-29T00:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	reports, err := auditShardKBs(context.Background(), ws)
	if err != nil {
		t.Fatalf("auditShardKBs: %v", err)
	}
	var out bytes.Buffer
	printKBReports(&out, reports)
	got := out.String()
	for _, want := range []string{
		"knowledge: godoc 1, placeholder 1, web/openai 1",
		"sources: 1 acquired, 1 failed",
		"context7 on go: failed: context7 answered 429",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("check-kb output lacks %q:\n%s", want, got)
		}
	}
}
