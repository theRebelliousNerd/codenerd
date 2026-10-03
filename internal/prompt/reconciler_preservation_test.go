package prompt

import (
	"bytes"
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func openReconcilePreservationFixture(test *testing.T) *sql.DB {
	test.Helper()
	database := openReconcileTestDB(test)
	database.SetMaxOpenConns(1)
	test.Cleanup(func() {
		if err := database.Close(); err != nil {
			test.Errorf("close preservation database: %v", err)
		}
	})
	return database
}

func assertReconcilePreservationVector(test *testing.T, database *sql.DB, atomID string, expectedVector []byte, expectedTask sql.NullString) {
	test.Helper()
	var actualVector []byte
	var actualTask sql.NullString
	if err := database.QueryRow("SELECT embedding, embedding_task FROM prompt_atoms WHERE atom_id = ?", atomID).Scan(&actualVector, &actualTask); err != nil {
		test.Fatalf("read vector/task for %s: %v", atomID, err)
	}
	if !bytes.Equal(actualVector, expectedVector) || actualTask != expectedTask {
		test.Errorf("vector/task for %s differs: vector_equal=%t task=%+v want=%+v", atomID, bytes.Equal(actualVector, expectedVector), actualTask, expectedTask)
	}
	if expectedVector == nil && actualVector != nil {
		test.Errorf("embedding for %s is not SQL NULL", atomID)
	}
	var indexVector []byte
	err := database.QueryRow("SELECT embedding FROM vec_prompt_atoms WHERE atom_id = ?", atomID).Scan(&indexVector)
	if expectedVector == nil {
		if err != sql.ErrNoRows {
			test.Fatalf("invalidated index row for %s remains or could not be inspected: %v", atomID, err)
		}
	} else if err != nil || !bytes.Equal(indexVector, expectedVector) {
		test.Fatalf("retained index vector for %s differs: %v", atomID, err)
	}
}

func TestReconcilePreservation_ExactNullableTasks(test *testing.T) {
	vector := []byte{0, 0, 128, 63, 0, 0, 0, 64}
	cases := []struct {
		name string
		task sql.NullString
	}{
		{name: "null"},
		{name: "empty", task: sql.NullString{String: "", Valid: true}},
		{name: "space", task: sql.NullString{String: " ", Valid: true}},
		{name: "tab", task: sql.NullString{String: "\t", Valid: true}},
		{name: "unicode whitespace", task: sql.NullString{String: "\u00a0\u2003", Valid: true}},
		{name: "mixed whitespace", task: sql.NullString{String: " \t\r\n\u2003 ", Valid: true}},
		{name: "document task", task: sql.NullString{String: "RETRIEVAL_DOCUMENT", Valid: true}},
		{name: "padded task", task: sql.NullString{String: " RETRIEVAL_DOCUMENT\t", Valid: true}},
		{name: "other legacy task", task: sql.NullString{String: "SEMANTIC_SIMILARITY", Valid: true}},
	}
	for _, fixture := range cases {
		test.Run(fixture.name, func(test *testing.T) {
			database := openReconcilePreservationFixture(test)
			mustInsertAtom(test, database, "task/retained", "old body", "unchanged input", "embedded", vector)
			mustInsertVecRow(test, database, "task/retained", vector)
			if _, err := database.Exec("UPDATE prompt_atoms SET embedding_task = ? WHERE atom_id = ?", fixture.task, "task/retained"); err != nil {
				test.Fatalf("set task fixture: %v", err)
			}
			canonical := NewPromptAtom("task/retained", CategoryMethodology, "new canonical body")
			canonical.Description = "unchanged input"
			counts, err := ReconcilePromptCorpus(context.Background(), database, []*PromptAtom{canonical})
			if err != nil {
				test.Fatalf("reconcile: %v", err)
			}
			if counts != (ReconcileCounts{Upserted: 1, RetainedEmbeddings: 1}) {
				test.Fatalf("unexpected retention counts: %+v", counts)
			}
			assertReconcilePreservationVector(test, database, canonical.ID, vector, fixture.task)
		})
	}
}

func TestReconcilePreservation_EffectiveInputAndInvalidation(test *testing.T) {
	vector := []byte{0, 0, 128, 63, 0, 0, 0, 64}
	cases := []struct {
		name           string
		oldDescription string
		newDescription string
		oldBody        string
		newBody        string
		retain         bool
	}{
		{"description selects input", "same description", "same description", "old", "new", true},
		{"blank description selects body", " \t\u2003", "\u00a0", "same body", "same body", true},
		{"description replaces identical body input", "", "old body", "old body", "new body", true},
		{"description change invalidates", "old description", "new description", "same body", "same body", false},
		{"description padding is significant", " description ", "description", "same body", "same body", false},
		{"body change invalidates", "\t", "\u2003", "old body", "new body", false},
		{"body padding is significant", "", "", "body ", "body", false},
	}
	for _, fixture := range cases {
		test.Run(fixture.name, func(test *testing.T) {
			database := openReconcilePreservationFixture(test)
			mustInsertAtom(test, database, "input/controlled", fixture.oldBody, fixture.oldDescription, "embedded", vector)
			mustInsertVecRow(test, database, "input/controlled", vector)
			task := sql.NullString{String: "\t\u2003", Valid: true}
			if _, err := database.Exec("UPDATE prompt_atoms SET embedding_task = ? WHERE atom_id = ?", task, "input/controlled"); err != nil {
				test.Fatalf("set task fixture: %v", err)
			}
			canonical := NewPromptAtom("input/controlled", CategoryMethodology, fixture.newBody)
			canonical.Description = fixture.newDescription
			counts, err := ReconcilePromptCorpus(context.Background(), database, []*PromptAtom{canonical})
			if err != nil {
				test.Fatalf("reconcile: %v", err)
			}
			expectedCounts := ReconcileCounts{Upserted: 1}
			if fixture.retain {
				expectedCounts.RetainedEmbeddings = 1
				assertReconcilePreservationVector(test, database, canonical.ID, vector, task)
			} else {
				expectedCounts.ClearedEmbeddings = 1
				assertReconcilePreservationVector(test, database, canonical.ID, nil, sql.NullString{})
			}
			if counts != expectedCounts {
				test.Errorf("counts=%+v want=%+v", counts, expectedCounts)
			}
		})
	}
}

func TestReconcilePreservation_ProjectOwnershipAndLegacyDeletion(test *testing.T) {
	database := openReconcilePreservationFixture(test)
	vector := []byte{0, 0, 128, 63, 0, 0, 0, 64}
	cases := []struct {
		atomID string
		source sql.NullString
		keep   bool
	}{
		{atomID: "project/null", keep: true},
		{atomID: "project/empty", source: sql.NullString{String: "", Valid: true}, keep: true},
		{atomID: "project/spaces", source: sql.NullString{String: "   ", Valid: true}, keep: true},
		{atomID: "project/tab", source: sql.NullString{String: "\t", Valid: true}, keep: true},
		{atomID: "project/newlines", source: sql.NullString{String: "\r\n", Valid: true}, keep: true},
		{atomID: "project/unicode", source: sql.NullString{String: "\u00a0\u2003", Valid: true}, keep: true},
		{atomID: "project/mixed", source: sql.NullString{String: " \t\u2003 ", Valid: true}, keep: true},
		{atomID: "legacy/embedded", source: sql.NullString{String: "embedded", Valid: true}},
		{atomID: "legacy/yaml", source: sql.NullString{String: "atoms/obsolete.yaml", Valid: true}},
		{atomID: "legacy/padded", source: sql.NullString{String: "\t atoms/obsolete.yaml\u2003", Valid: true}},
	}
	for _, fixture := range cases {
		mustInsertAtom(test, database, fixture.atomID, "original body", "original description", fixture.source, vector)
		mustInsertTag(test, database, fixture.atomID, "mode", "original")
		mustInsertVecRow(test, database, fixture.atomID, vector)
	}
	counts, err := ReconcilePromptCorpus(context.Background(), database, []*PromptAtom{})
	if err != nil {
		test.Fatalf("reconcile: %v", err)
	}
	if counts != (ReconcileCounts{Deleted: 3}) {
		test.Fatalf("unexpected ownership counts: %+v", counts)
	}
	for _, fixture := range cases {
		if !fixture.keep {
			for _, table := range []string{"prompt_atoms", "atom_context_tags", "vec_prompt_atoms"} {
				if countRows(test, database, "SELECT COUNT(*) FROM "+table+" WHERE atom_id = ?", fixture.atomID) != 0 {
					test.Errorf("obsolete atom %s remains in %s", fixture.atomID, table)
				}
			}
			continue
		}
		var content, description string
		var source sql.NullString
		if err := database.QueryRow("SELECT content, description, source_file FROM prompt_atoms WHERE atom_id = ?", fixture.atomID).Scan(&content, &description, &source); err != nil {
			test.Fatalf("read project atom %s: %v", fixture.atomID, err)
		}
		if content != "original body" || description != "original description" || source != fixture.source {
			test.Errorf("project row %s changed", fixture.atomID)
		}
		if countRows(test, database, "SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ?", fixture.atomID) != 1 || countRows(test, database, "SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ? AND dimension = 'mode' AND tag = 'original' AND is_exclusion = 0", fixture.atomID) != 1 {
			test.Errorf("project tag for %s changed", fixture.atomID)
		}
		assertReconcilePreservationVector(test, database, fixture.atomID, vector, sql.NullString{})
	}
}

func TestReconcilePreservation_CanonicalIDOverridesProjectOwnership(test *testing.T) {
	database := openReconcilePreservationFixture(test)
	vector := []byte{0, 0, 128, 63, 0, 0, 0, 64}
	mustInsertAtom(test, database, "canonical/winner", "project body", "same input", "\t\u2003", vector)
	mustInsertTag(test, database, "canonical/winner", "mode", "project")
	mustInsertVecRow(test, database, "canonical/winner", vector)
	if _, err := database.Exec("UPDATE prompt_atoms SET embedding_task = '' WHERE atom_id = 'canonical/winner'"); err != nil {
		test.Fatalf("set empty task: %v", err)
	}
	canonical := NewPromptAtom("canonical/winner", CategoryIdentity, "canonical body")
	canonical.Description = "same input"
	canonical.Priority = 71
	canonical.OperationalModes = []string{"active"}
	canonical.Languages = []string{"go"}
	counts, err := ReconcilePromptCorpus(context.Background(), database, []*PromptAtom{canonical})
	if err != nil {
		test.Fatalf("reconcile: %v", err)
	}
	if counts != (ReconcileCounts{Upserted: 1, RetainedEmbeddings: 1}) {
		test.Fatalf("unexpected canonical counts: %+v", counts)
	}
	var content, contentHash, source, category string
	var priority int
	if err := database.QueryRow("SELECT content, content_hash, source_file, category, priority FROM prompt_atoms WHERE atom_id = ?", canonical.ID).Scan(&content, &contentHash, &source, &category, &priority); err != nil {
		test.Fatalf("read canonical winner: %v", err)
	}
	if content != canonical.Content || contentHash != canonical.ContentHash || source != "embedded" || category != string(CategoryIdentity) || priority != canonical.Priority {
		test.Fatal("project ownership displaced canonical fields")
	}
	if countRows(test, database, "SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ?", canonical.ID) != 2 {
		test.Fatal("canonical tags did not replace project tags exactly")
	}
	for _, tag := range []struct{ dimension, value string }{{"mode", "active"}, {"lang", "go"}} {
		if countRows(test, database, "SELECT COUNT(*) FROM atom_context_tags WHERE atom_id = ? AND dimension = ? AND tag = ?", canonical.ID, tag.dimension, tag.value) != 1 {
			test.Errorf("missing canonical tag %s=%s", tag.dimension, tag.value)
		}
	}
	assertReconcilePreservationVector(test, database, canonical.ID, vector, sql.NullString{String: "", Valid: true})
}

func snapshotReconcilePreservation(test *testing.T, database *sql.DB) map[string][]string {
	test.Helper()
	queries := map[string]string{
		"atoms":   `SELECT quote(atom_id)||'|'||quote(version)||'|'||quote(content)||'|'||quote(token_count)||'|'||quote(content_hash)||'|'||quote(description)||'|'||quote(content_concise)||'|'||quote(content_min)||'|'||quote(category)||'|'||quote(subcategory)||'|'||quote(priority)||'|'||quote(is_mandatory)||'|'||quote(is_exclusive)||'|'||quote(source_file)||'|'||quote(embedding)||'|'||quote(embedding_task) FROM prompt_atoms ORDER BY atom_id`,
		"tags":    `SELECT quote(atom_id)||'|'||quote(dimension)||'|'||quote(tag)||'|'||quote(is_exclusion) FROM atom_context_tags ORDER BY atom_id, dimension, tag`,
		"vectors": `SELECT quote(atom_id)||'|'||quote(embedding) FROM vec_prompt_atoms ORDER BY atom_id`,
	}
	snapshot := make(map[string][]string)
	for table, query := range queries {
		rows, err := database.Query(query)
		if err != nil {
			test.Fatalf("snapshot %s: %v", table, err)
		}
		for rows.Next() {
			var record string
			if err := rows.Scan(&record); err != nil {
				rows.Close()
				test.Fatalf("snapshot row in %s: %v", table, err)
			}
			snapshot[table] = append(snapshot[table], record)
		}
		iterationErr := rows.Err()
		closeErr := rows.Close()
		if iterationErr != nil || closeErr != nil {
			test.Fatalf("finish snapshot %s: iteration=%v close=%v", table, iterationErr, closeErr)
		}
	}
	return snapshot
}

func TestReconcilePreservation_LateFailureRollsBackAllTables(test *testing.T) {
	database := openReconcilePreservationFixture(test)
	vector := []byte{0, 0, 128, 63, 0, 0, 0, 64}
	for _, atomID := range []string{"canonical/changed", "legacy/obsolete", "project/keep"} {
		source := "atoms/legacy.yaml"
		if atomID == "project/keep" {
			source = "\t\u2003"
		}
		mustInsertAtom(test, database, atomID, "old body", "old description", source, vector)
		mustInsertTag(test, database, atomID, "mode", "original")
		mustInsertVecRow(test, database, atomID, vector)
	}
	if _, err := database.Exec("UPDATE prompt_atoms SET embedding_task = '\t' WHERE atom_id = 'canonical/changed'"); err != nil {
		test.Fatalf("set rollback task: %v", err)
	}
	if _, err := database.Exec(`CREATE TRIGGER reject_obsolete_deletion BEFORE DELETE ON prompt_atoms WHEN OLD.atom_id = 'legacy/obsolete' BEGIN SELECT RAISE(ABORT, 'forced late preservation failure'); END;`); err != nil {
		test.Fatalf("create rollback trigger: %v", err)
	}
	before := snapshotReconcilePreservation(test, database)
	added := NewPromptAtom("canonical/added", CategoryIdentity, "new body")
	added.OperationalModes = []string{"active"}
	changed := NewPromptAtom("canonical/changed", CategoryMethodology, "new body")
	changed.Description = "new description"
	changed.Languages = []string{"go"}
	counts, err := ReconcilePromptCorpus(context.Background(), database, []*PromptAtom{added, changed})
	if err == nil || !strings.Contains(err.Error(), "forced late preservation failure") {
		test.Fatalf("expected late deletion failure, got %v", err)
	}
	if counts != (ReconcileCounts{}) {
		test.Errorf("failed transaction returned committed counts: %+v", counts)
	}
	after := snapshotReconcilePreservation(test, database)
	for _, table := range []string{"atoms", "tags", "vectors"} {
		if !reflect.DeepEqual(before[table], after[table]) {
			test.Errorf("late failure did not restore %s", table)
		}
	}
}
