package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func openSourceGenerationDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustToken(t *testing.T, db *sql.DB, tables ...string) string {
	t.Helper()
	token, err := ReadSourceGeneration(context.Background(), db, tables...)
	if err != nil {
		t.Fatalf("ReadSourceGeneration: %v", err)
	}
	return token
}

func TestSourceGeneration_EveryWriteFromAnyConnectionMovesTheToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "src.db")
	reader := openSourceGenerationDB(t, path)
	writer := openSourceGenerationDB(t, path) // a second pool: a different "process"
	if _, err := writer.Exec(`CREATE TABLE atoms (id TEXT PRIMARY KEY, body TEXT)`); err != nil {
		t.Fatal(err)
	}

	start := mustToken(t, reader, "atoms")
	if again := mustToken(t, reader, "atoms"); again != start {
		t.Fatalf("token moved without a write: %q -> %q", start, again)
	}

	steps := []string{
		`INSERT INTO atoms VALUES ('a', 'one')`,
		`UPDATE atoms SET body = 'two' WHERE id = 'a'`,
		`DELETE FROM atoms WHERE id = 'a'`,
	}
	prev := start
	for _, stmt := range steps {
		if _, err := writer.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
		next := mustToken(t, reader, "atoms")
		if next == prev {
			t.Fatalf("token did not move after %q (%q)", stmt, next)
		}
		prev = next
	}
}

func TestSourceGeneration_RecreatedTableIsRetrackedBeforeTrust(t *testing.T) {
	db := openSourceGenerationDB(t, filepath.Join(t.TempDir(), "src.db"))
	if _, err := db.Exec(`CREATE TABLE atoms (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	before := mustToken(t, db, "atoms")

	// Dropping the table drops its triggers. The schema change alone must
	// change the token, and the next read must re-install the triggers so a
	// later write is seen again.
	if _, err := db.Exec(`DROP TABLE atoms; CREATE TABLE atoms (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	recreated := mustToken(t, db, "atoms")
	if recreated == before {
		t.Fatalf("token survived drop/recreate: %q", recreated)
	}
	if _, err := db.Exec(`INSERT INTO atoms VALUES ('x')`); err != nil {
		t.Fatal(err)
	}
	if after := mustToken(t, db, "atoms"); after == recreated {
		t.Fatalf("write after recreate not tracked: %q", after)
	}
}

func TestSourceGeneration_AbsentTableIsPartOfTheToken(t *testing.T) {
	db := openSourceGenerationDB(t, filepath.Join(t.TempDir(), "src.db"))
	absent := mustToken(t, db, "atoms")
	if _, err := db.Exec(`CREATE TABLE atoms (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	if present := mustToken(t, db, "atoms"); present == absent {
		t.Fatalf("creating the table did not change the token: %q", present)
	}
}

func TestSourceGeneration_ReadOnlyUntrackedDatabaseIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.db")
	rw := openSourceGenerationDB(t, path)
	if _, err := rw.Exec(`CREATE TABLE atoms (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	_ = rw.Close()
	ro := openSourceGenerationDB(t, "file:"+path+"?mode=ro")
	if _, err := ReadSourceGeneration(context.Background(), ro, "atoms"); err == nil {
		t.Fatal("an untracked table on a read-only database must be reported, not trusted")
	}
}
