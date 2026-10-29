package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

func openSeedFixture(test *testing.T, databasePath string) *sql.DB {
	test.Helper()
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		test.Fatal(err)
	}
	uriPath := filepath.ToSlash(absolutePath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	databaseURL := url.URL{Scheme: "file", Path: uriPath}
	database, err := sql.Open("sqlite3", databaseURL.String())
	if err != nil {
		test.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	test.Cleanup(func() { _ = database.Close() })
	return database
}

func newSeedFixture(test *testing.T) (string, *sql.DB) {
	test.Helper()
	filename := "seed # % ü.db"
	if runtime.GOOS != "windows" {
		filename = "seed ?# % ü.db"
	}
	databasePath := filepath.Join(test.TempDir(), filename)
	database := openSeedFixture(test, databasePath)
	if err := prompt.NewAtomLoader(nil).EnsureSchema(context.Background(), database); err != nil {
		test.Fatal(err)
	}
	if _, err := database.Exec("CREATE TABLE vec_prompt_atoms (atom_id TEXT PRIMARY KEY, embedding BLOB)"); err != nil {
		test.Fatal(err)
	}
	return databasePath, database
}

func insertSeedFixtureAtom(test *testing.T, database *sql.DB, atom *prompt.PromptAtom, source any, vector []byte, task any) {
	test.Helper()
	if _, err := database.Exec(`INSERT INTO prompt_atoms
		(atom_id, version, content, token_count, content_hash, category, description, source_file, embedding, embedding_task)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		atom.ID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash, string(atom.Category), atom.Description, source, vector, task); err != nil {
		test.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO vec_prompt_atoms (atom_id, embedding) VALUES (?, ?)", atom.ID, vector); err != nil {
		test.Fatal(err)
	}
}

func seedFixtureBytes(test *testing.T, databasePath string) []byte {
	test.Helper()
	content, err := os.ReadFile(databasePath)
	if err != nil {
		test.Fatal(err)
	}
	return content
}

func TestReconcileSeed_ExistingWritableProfilePersistsWAL(test *testing.T) {
	databasePath, database := newSeedFixture(test)
	stored := prompt.NewPromptAtom("fixture/profile", prompt.CategoryMethodology, "retained body")
	vector := encodeFloat32Slice([]float32{1.25, -0.5})
	insertSeedFixtureAtom(test, database, stored, "embedded", vector, " \t ")
	var beforeMode string
	if err := database.QueryRow("PRAGMA journal_mode = DELETE").Scan(&beforeMode); err != nil {
		test.Fatal(err)
	}
	if beforeMode != "delete" {
		test.Fatalf("untuned control journal mode = %q, want delete", beforeMode)
	}
	if err := database.Close(); err != nil {
		test.Fatal(err)
	}
	before := seedFixtureBytes(test, databasePath)
	if len(before) < 20 || before[18] != 1 || before[19] != 1 {
		test.Fatal("untuned control lacks rollback-journal read/write header versions")
	}
	counts, err := reconcilePromptSeed(context.Background(), databasePath, []*prompt.PromptAtom{stored})
	if err != nil || counts != (prompt.ReconcileCounts{Upserted: 1, RetainedEmbeddings: 1}) {
		test.Fatalf("profile reconciliation = %+v (%v)", counts, err)
	}
	after := seedFixtureBytes(test, databasePath)
	if len(after) < 20 || after[18] != 2 || after[19] != 2 {
		test.Fatal("reconciled corpus lacks durable WAL read/write header versions")
	}
	reopened := openSeedFixture(test, databasePath)
	var afterMode string
	if err := reopened.QueryRow("PRAGMA journal_mode").Scan(&afterMode); err != nil {
		test.Fatal(err)
	}
	if afterMode != "wal" {
		test.Fatalf("reopened journal mode = %q, want wal", afterMode)
	}
	var retainedVector []byte
	var retainedTask sql.NullString
	if err := reopened.QueryRow("SELECT embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?", stored.ID).Scan(&retainedVector, &retainedTask); err != nil {
		test.Fatal(err)
	}
	if !bytes.Equal(retainedVector, vector) || !retainedTask.Valid || retainedTask.String != " \t " {
		test.Fatalf("profile changed retained vector/task: %v/%v", retainedVector, retainedTask)
	}
}

func TestReconcileSeed_EmbeddingInputControlsExactRetention(test *testing.T) {
	controls := []struct {
		name           string
		oldDescription string
		newDescription string
		oldContent     string
		newContent     string
		task           any
		retained       bool
	}{
		{"unchanged content input", "", "", "same", "same", "RETRIEVAL_DOCUMENT", true},
		{"content change with unchanged description", "same description", "same description", "old body", "new body", " RETRIEVAL_DOCUMENT ", true},
		{"unchanged input with null task", "same description", "same description", "same", "same", nil, true},
		{"unchanged input with empty task", "same description", "same description", "same", "same", "", true},
		{"unchanged input with whitespace task", "same description", "same description", "same", "same", " \t ", true},
		{"description change", "old description", "new description", "same", "same", "RETRIEVAL_DOCUMENT", false},
		{"content change without description", "", "", "old body", "new body", "RETRIEVAL_DOCUMENT", false},
		{"blank description uses content", "   ", "\t", "old body", "new body", "RETRIEVAL_DOCUMENT", false},
	}
	for _, control := range controls {
		test.Run(control.name, func(subtest *testing.T) {
			databasePath, database := newSeedFixture(subtest)
			stored := prompt.NewPromptAtom("fixture/retention", prompt.CategoryMethodology, control.oldContent)
			stored.Description = control.oldDescription
			vector := encodeFloat32Slice([]float32{1.25, -0.5})
			insertSeedFixtureAtom(subtest, database, stored, "legacy/atoms.yaml", vector, control.task)
			canonical := prompt.NewPromptAtom(stored.ID, prompt.CategoryMethodology, control.newContent)
			canonical.Description = control.newDescription
			counts, err := reconcilePromptSeed(context.Background(), databasePath, []*prompt.PromptAtom{canonical})
			if err != nil {
				subtest.Fatal(err)
			}
			var content, contentHash, source string
			var retainedVector []byte
			var retainedTask sql.NullString
			if err := database.QueryRow("SELECT content, content_hash, source_file, embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?", stored.ID).Scan(&content, &contentHash, &source, &retainedVector, &retainedTask); err != nil {
				subtest.Fatal(err)
			}
			if content != canonical.Content || contentHash != canonical.ContentHash || source != "embedded" {
				subtest.Fatalf("canonical row = %q/%q/%q", content, contentHash, source)
			}
			var vectorRows int
			if err := database.QueryRow("SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?", stored.ID).Scan(&vectorRows); err != nil {
				subtest.Fatal(err)
			}
			expected := prompt.ReconcileCounts{Upserted: 1}
			if control.retained {
				expected.RetainedEmbeddings = 1
				if !bytes.Equal(retainedVector, vector) || vectorRows != 1 {
					subtest.Fatalf("retained bytes/rows = %v/%d, want %v/1", retainedVector, vectorRows, vector)
				}
				if control.task == nil {
					if retainedTask.Valid {
						subtest.Fatalf("NULL task changed to %v", retainedTask)
					}
				} else if !retainedTask.Valid || retainedTask.String != control.task.(string) {
					subtest.Fatalf("task metadata changed: %v, want %q", retainedTask, control.task)
				}
				var vectorTableBytes []byte
				if err := database.QueryRow("SELECT embedding FROM vec_prompt_atoms WHERE atom_id = ?", stored.ID).Scan(&vectorTableBytes); err != nil || !bytes.Equal(vectorTableBytes, vector) {
					subtest.Fatalf("vector table bytes changed: %v (%v)", vectorTableBytes, err)
				}
			} else {
				expected.ClearedEmbeddings = 1
				if retainedVector != nil || retainedTask.Valid || vectorRows != 0 {
					subtest.Fatalf("changed input retained vector/task/rows: %v/%v/%d", retainedVector, retainedTask, vectorRows)
				}
			}
			if counts != expected {
				subtest.Fatalf("counts = %+v, want %+v", counts, expected)
			}
		})
	}
}

func TestReconcileSeed_CanonicalTagsAndOwnership(test *testing.T) {
	databasePath, database := newSeedFixture(test)
	for _, row := range []struct {
		identity string
		source   any
	}{
		{"fixture/winner", nil}, {"fixture/obsolete", "atoms/legacy.yaml"},
		{"project/null", nil}, {"project/empty", ""}, {"project/blank", " \t "},
	} {
		insertSeedFixtureAtom(test, database, prompt.NewPromptAtom(row.identity, prompt.CategoryMethodology, "original "+row.identity), row.source, nil, nil)
	}
	if _, err := database.Exec("INSERT INTO atom_context_tags (atom_id, dimension, tag) VALUES ('fixture/winner', 'mode', 'stale')"); err != nil {
		test.Fatal(err)
	}
	winner := prompt.NewPromptAtom("fixture/winner", prompt.CategoryMethodology, "canonical body")
	winner.OperationalModes = []string{"active"}
	winner.Languages = []string{"go"}
	winner.RequiresTools = []string{"run_tests"}
	added := prompt.NewPromptAtom("fixture/added", prompt.CategoryProtocol, "new body")
	counts, err := reconcilePromptSeed(context.Background(), databasePath, []*prompt.PromptAtom{winner, added})
	if err != nil || counts != (prompt.ReconcileCounts{Upserted: 2, Deleted: 1}) {
		test.Fatalf("counts/error = %+v/%v", counts, err)
	}
	rows, err := database.Query("SELECT dimension || ':' || tag FROM atom_context_tags WHERE atom_id = ? ORDER BY dimension, tag", winner.ID)
	if err != nil {
		test.Fatal(err)
	}
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			_ = rows.Close()
			test.Fatal(err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		test.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		test.Fatal(err)
	}
	if !reflect.DeepEqual(tags, []string{"lang:go", "mode:active", "requires_tool:run_tests"}) {
		test.Fatalf("canonical tags = %v", tags)
	}
	for _, identity := range []string{"project/null", "project/empty", "project/blank"} {
		var content string
		if err := database.QueryRow("SELECT content FROM prompt_atoms WHERE atom_id = ?", identity).Scan(&content); err != nil || content != "original "+identity {
			test.Fatalf("project atom %s changed: %q (%v)", identity, content, err)
		}
	}
	var obsoleteAtoms, obsoleteVectors int
	if err := database.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = 'fixture/obsolete'").Scan(&obsoleteAtoms); err != nil {
		test.Fatal(err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = 'fixture/obsolete'").Scan(&obsoleteVectors); err != nil {
		test.Fatal(err)
	}
	if obsoleteAtoms != 0 || obsoleteVectors != 0 {
		test.Fatalf("obsolete rows remain: atoms=%d vectors=%d", obsoleteAtoms, obsoleteVectors)
	}
	var newVector []byte
	var newTask sql.NullString
	if err := database.QueryRow("SELECT embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?", added.ID).Scan(&newVector, &newTask); err != nil || newVector != nil || newTask.Valid {
		test.Fatalf("new atom received fabricated vector/task: %v/%v (%v)", newVector, newTask, err)
	}
}

func TestReconcileSeed_RejectsInvalidTargetsWithoutReplacement(test *testing.T) {
	for _, kind := range []string{"missing", "directory", "garbage", "empty", "unrelated SQLite", "invalid corpus schema", "corpus without unique identity"} {
		test.Run(kind, func(subtest *testing.T) {
			databasePath := filepath.Join(subtest.TempDir(), "candidate.db")
			switch kind {
			case "directory":
				if err := os.Mkdir(databasePath, 0700); err != nil {
					subtest.Fatal(err)
				}
			case "garbage", "empty":
				content := []byte("not a SQLite corpus")
				if kind == "empty" {
					content = nil
				}
				if err := os.WriteFile(databasePath, content, 0600); err != nil {
					subtest.Fatal(err)
				}
			case "unrelated SQLite", "invalid corpus schema":
				database := openSeedFixture(subtest, databasePath)
				statement := "CREATE TABLE unrelated (value TEXT)"
				if kind == "invalid corpus schema" {
					statement = "CREATE TABLE prompt_atoms (atom_id TEXT); CREATE TABLE atom_context_tags (atom_id TEXT)"
				}
				if _, err := database.Exec(statement); err != nil {
					subtest.Fatal(err)
				}
				if err := database.Close(); err != nil {
					subtest.Fatal(err)
				}
			case "corpus without unique identity":
				database := openSeedFixture(subtest, databasePath)
				if err := prompt.NewAtomLoader(nil).EnsureSchema(context.Background(), database); err != nil {
					subtest.Fatal(err)
				}
				if _, err := database.Exec(`ALTER TABLE prompt_atoms RENAME TO original_atoms;
					CREATE TABLE prompt_atoms AS SELECT * FROM original_atoms; DROP TABLE original_atoms`); err != nil {
					subtest.Fatal(err)
				}
				if err := database.Close(); err != nil {
					subtest.Fatal(err)
				}
			}
			var before []byte
			if kind != "missing" && kind != "directory" {
				before = seedFixtureBytes(subtest, databasePath)
			}
			counts, err := reconcileEmbeddedSeed(context.Background(), "internal/prompt/atoms", databasePath)
			if err == nil || counts != (prompt.ReconcileCounts{}) {
				subtest.Fatalf("invalid target succeeded: %+v (%v)", counts, err)
			}
			switch kind {
			case "missing":
				if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
					subtest.Fatalf("missing target was created: %v", err)
				}
			case "directory":
				info, err := os.Stat(databasePath)
				if err != nil || !info.IsDir() {
					subtest.Fatalf("directory was replaced: %v", err)
				}
			default:
				if !bytes.Equal(before, seedFixtureBytes(subtest, databasePath)) {
					subtest.Fatal("invalid target bytes changed")
				}
			}
		})
	}
}

func TestReconcileSeed_RejectsConflictingInputAndCancellation(test *testing.T) {
	databasePath, database := newSeedFixture(test)
	if err := database.Close(); err != nil {
		test.Fatal(err)
	}
	before := seedFixtureBytes(test, databasePath)
	if _, err := reconcileEmbeddedSeed(context.Background(), "custom/atoms", databasePath); err == nil || !strings.Contains(err.Error(), "custom -input") {
		test.Fatalf("conflicting input error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	counts, err := reconcileEmbeddedSeed(cancelled, "internal/prompt/atoms", databasePath)
	if !errors.Is(err, context.Canceled) || counts != (prompt.ReconcileCounts{}) {
		test.Fatalf("cancelled reconciliation = %+v (%v)", counts, err)
	}
	if !bytes.Equal(before, seedFixtureBytes(test, databasePath)) {
		test.Fatal("conflict or cancellation modified the target")
	}
}

func TestReconcileSeed_SQLFailureRollsBack(test *testing.T) {
	databasePath, database := newSeedFixture(test)
	stored := prompt.NewPromptAtom("fixture/reject", prompt.CategoryMethodology, "old body")
	stored.Description = "old description"
	vector := encodeFloat32Slice([]float32{0.25, -1})
	insertSeedFixtureAtom(test, database, stored, "embedded", vector, "RETRIEVAL_DOCUMENT")
	if _, err := database.Exec(`CREATE TRIGGER reject_vector_delete BEFORE DELETE ON vec_prompt_atoms
		WHEN OLD.atom_id = 'fixture/reject' BEGIN SELECT RAISE(ABORT, 'retention control'); END`); err != nil {
		test.Fatal(err)
	}
	changed := prompt.NewPromptAtom(stored.ID, prompt.CategoryMethodology, "new body")
	changed.Description = "new description"
	added := prompt.NewPromptAtom("fixture/rollback", prompt.CategoryMethodology, "not committed")
	counts, err := reconcilePromptSeed(context.Background(), databasePath, []*prompt.PromptAtom{added, changed})
	if err == nil || !strings.Contains(err.Error(), "retention control") || counts != (prompt.ReconcileCounts{}) {
		test.Fatalf("failure did not propagate: %+v (%v)", counts, err)
	}
	var addedRows int
	if err := database.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = ?", added.ID).Scan(&addedRows); err != nil || addedRows != 0 {
		test.Fatalf("partial upsert survived rollback: %d (%v)", addedRows, err)
	}
	var body, description string
	var retained []byte
	if err := database.QueryRow("SELECT content, description, embedding FROM prompt_atoms WHERE atom_id = ?", stored.ID).Scan(&body, &description, &retained); err != nil || body != stored.Content || description != stored.Description || !bytes.Equal(retained, vector) {
		test.Fatalf("rollback changed original row: %q/%q/%v (%v)", body, description, retained, err)
	}
}

func TestReconcileSeed_MainIsCredentialFreeAndUsesCanonicalCorpus(test *testing.T) {
	if databasePath := os.Getenv("PROMPT_BUILDER_RECONCILE_CHILD"); databasePath != "" {
		flag.CommandLine = flag.NewFlagSet("prompt_builder", flag.ExitOnError)
		os.Args = []string{os.Args[0], "-reconcile-embedded", "-output", databasePath}
		main()
		return
	}
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		test.Fatal(err)
	}
	canonical := corpus.All()
	if len(canonical) < 2 {
		test.Fatal("canonical corpus lacks retention and invalidation controls")
	}
	databasePath, database := newSeedFixture(test)
	vector := encodeFloat32Slice([]float32{1, 2})
	insertSeedFixtureAtom(test, database, canonical[0], "legacy/atoms.yaml", vector, "RETRIEVAL_DOCUMENT")
	stale := *canonical[1]
	stale.Description = "stale embedding input control"
	if stale.Description == canonical[1].Description {
		test.Fatal("invalidation control unexpectedly matches canonical description")
	}
	insertSeedFixtureAtom(test, database, &stale, "legacy/atoms.yaml", vector, "RETRIEVAL_DOCUMENT")
	insertSeedFixtureAtom(test, database, prompt.NewPromptAtom("fixture/obsolete", prompt.CategoryMethodology, "obsolete"), "legacy/atoms.yaml", nil, nil)
	if err := database.Close(); err != nil {
		test.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	watchdog, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(watchdog, executable, "-test.run=^TestReconcileSeed_MainIsCredentialFreeAndUsesCanonicalCorpus$")
	for _, environment := range os.Environ() {
		if strings.HasPrefix(environment, "GEMINI_API_KEY=") || strings.HasPrefix(environment, "GENAI_EMBEDDING_MODEL=") || strings.HasPrefix(environment, "PROMPT_BUILDER_RECONCILE_CHILD=") {
			continue
		}
		child.Env = append(child.Env, environment)
	}
	child.Env = append(child.Env, "GEMINI_API_KEY=", "GENAI_EMBEDDING_MODEL=", "PROMPT_BUILDER_RECONCILE_CHILD="+databasePath)
	output, err := child.CombinedOutput()
	if err != nil {
		test.Fatalf("credential-free main failed: %v\n%s", err, output)
	}
	var reported prompt.ReconcileCounts
	reportFound := false
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			if err := json.Unmarshal([]byte(line), &reported); err != nil {
				test.Fatal(err)
			}
			reportFound = true
		}
	}
	expected := prompt.ReconcileCounts{Upserted: len(canonical), Deleted: 1, RetainedEmbeddings: 1, ClearedEmbeddings: 1}
	if !reportFound || reported != expected {
		test.Fatalf("reported counts = %+v, want %+v\n%s", reported, expected, output)
	}
	if strings.Contains(string(output), "API key found") || strings.Contains(string(output), "Embedding engine created") || strings.Contains(string(output), "Database created") {
		test.Fatalf("reconciliation entered generation: %s", output)
	}
	reconciled := openSeedFixture(test, databasePath)
	for _, atom := range canonical {
		var content, contentHash, source string
		if err := reconciled.QueryRow("SELECT content, content_hash, source_file FROM prompt_atoms WHERE atom_id = ?", atom.ID).Scan(&content, &contentHash, &source); err != nil || content != atom.Content || contentHash != atom.ContentHash || source != "embedded" {
			test.Fatalf("canonical atom %s drifted: %q/%q/%q (%v)", atom.ID, content, contentHash, source, err)
		}
	}
}
