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

	gaps, err := auditShardKBs(context.Background(), ws)
	if err != nil {
		t.Fatalf("auditShardKBs: %v", err)
	}
	var out bytes.Buffer
	n := printKBGaps(&out, gaps)
	if n == 0 || !strings.Contains(out.String(), "shard_agent_kb knowledge_atoms.source never_written") {
		t.Errorf("check-kb output (%d gaps):\n%s", n, out.String())
	}
}
