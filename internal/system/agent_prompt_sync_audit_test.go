package system

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openAgentKB(t *testing.T, ws, agent string) *sql.DB {
	t.Helper()
	path := filepath.Join(ws, ".nerd", "shards", strings.ToLower(agent)+"_knowledge.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// agentAtomContents is every prompt atom's content in the agent's KB, joined.
func agentAtomContents(t *testing.T, ws, agent string) string {
	t.Helper()
	db := openAgentKB(t, ws, agent)
	defer db.Close()
	rows, err := db.Query(`SELECT content FROM prompt_atoms`)
	if err != nil {
		t.Fatalf("read prompt_atoms: %v", err)
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, c)
	}
	return strings.Join(parts, "\n")
}

// auditAtomWrites installs triggers that log every insert, update and delete
// of a prompt atom into atom_write_audit.
func auditAtomWrites(t *testing.T, ws, agent string) {
	t.Helper()
	db := openAgentKB(t, ws, agent)
	defer db.Close()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS atom_write_audit (op TEXT)`,
		`CREATE TRIGGER IF NOT EXISTS atom_audit_ins AFTER INSERT ON prompt_atoms BEGIN INSERT INTO atom_write_audit VALUES ('insert'); END`,
		`CREATE TRIGGER IF NOT EXISTS atom_audit_upd AFTER UPDATE ON prompt_atoms BEGIN INSERT INTO atom_write_audit VALUES ('update'); END`,
		`CREATE TRIGGER IF NOT EXISTS atom_audit_del AFTER DELETE ON prompt_atoms BEGIN INSERT INTO atom_write_audit VALUES ('delete'); END`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("install audit (%s): %v", s, err)
		}
	}
}

func auditedAtomWrites(t *testing.T, ws, agent string) int {
	t.Helper()
	db := openAgentKB(t, ws, agent)
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM atom_write_audit`).Scan(&n); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	return n
}
