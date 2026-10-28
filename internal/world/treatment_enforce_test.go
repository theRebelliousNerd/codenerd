package world

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
)

// enforceScanner ignores .nerd so the scan cache and the treatment artifact
// are not members of the generation they describe.
func enforceScanner() *Scanner {
	cfg := DefaultScannerConfig()
	cfg.IgnorePatterns = []string{".nerd"}
	cfg.MaxConcurrency = 2
	return NewScannerWithConfig(cfg)
}

func enforceMembership(t *testing.T, root string) *workspace.Membership {
	t.Helper()
	m, err := workspace.Open(root, []string{".nerd"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	return m
}

func enforceFixture(t *testing.T) (string, *workspace.Membership) {
	t.Helper()
	root, _ := shadowFixture(t)
	return root, enforceMembership(t, root)
}

func hasFirstArg(facts []core.Fact, pred string, first string) bool {
	for _, f := range facts {
		if f.Predicate == pred && len(f.Args) > 0 && f.Args[0] == first {
			return true
		}
	}
	return false
}

func TestScanEnforcesOperatorConfirmedNamesOnlyScope(t *testing.T) {
	root, m := enforceFixture(t)
	writeTreatmentArtifact(t, m, m.Generation(), workspace.AuthorityOperator)

	res, err := enforceScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"data/rows.go", "data/helper_test.go", "data/big/sample.json"} {
		if hasFirstArg(res.Facts, "file_topology", p) {
			t.Errorf("%s was hashed into file_topology under an enforced names-only scope", p)
		}
		if !hasFirstArg(res.Facts, "file_metadata_only", p) {
			t.Errorf("%s lost its name: no file_metadata_only row", p)
		}
		for _, f := range res.Facts {
			if f.Predicate != "file_topology" && f.Predicate != "file_metadata_only" && len(f.Args) > 0 && f.Args[0] == p {
				t.Errorf("%s was parsed: %s row", p, f.Predicate)
			}
		}
	}
	// A shared name prefix is not a shared segment, and a conflicted scope
	// keeps current behavior.
	for _, p := range []string{"src/main.go", "database/conn.go", "contested/x.py", "contested/inner/y.py"} {
		if !hasFirstArg(res.Facts, "file_topology", p) {
			t.Errorf("%s is outside every enforced scope but was not scanned", p)
		}
	}
	scopes := map[string][2]int64{}
	for _, f := range res.Facts {
		if f.Predicate == "treatment_scope" {
			scopes[f.Args[0].(string)] = [2]int64{f.Args[2].(int64), f.Args[3].(int64)}
		}
	}
	if len(scopes) != 2 || scopes["data"][0] != 2 || scopes["data/big"][0] != 1 || scopes["data"][1] <= 0 {
		t.Errorf("treatment_scope coverage %v, want data=2 files and data/big=1 file", scopes)
	}
	sh := res.TreatmentShadow
	if sh.EnforcedFiles != 3 || sh.Files != 0 || sh.Hashes != 0 || sh.BodyReads != 0 {
		t.Errorf("report %+v, want 3 enforced files and no shadow work", sh)
	}
	if res.FileCount != 4 {
		t.Errorf("FileCount %d counts unread files; want the 4 scanned ones", res.FileCount)
	}
}

// TestScanNeverOpensAnEnforcedBody is the audit's boundary test: zero ordinary
// body opens, hashing included. An unreadable file inside the scope fails a
// scan that reads it and not one that enforces the scope.
func TestScanNeverOpensAnEnforcedBody(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not stop root")
	}
	root, m := enforceFixture(t)
	locked := filepath.Join(root, "data", "rows.go")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	writeTreatmentArtifact(t, m, m.Generation(), workspace.AuthorityInferred)
	if _, err := enforceScanner().ScanDirectory(context.Background(), root); err == nil {
		t.Fatal("an inferred scope is shadow only: the scan must still read the file and fail on it")
	}
	writeTreatmentArtifact(t, m, m.Generation(), workspace.AuthorityOperator)
	if _, err := enforceScanner().ScanDirectory(context.Background(), root); err != nil {
		t.Fatalf("enforced names-only scope still opened a body: %v", err)
	}
}

func TestScanDoesNotEnforceAStaleArtifact(t *testing.T) {
	root, m := enforceFixture(t)
	writeTreatmentArtifact(t, m, "sha256:an-older-member-set", workspace.AuthorityOperator)
	res, err := enforceScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFirstArg(res.Facts, "file_topology", "data/rows.go") || hasFirstArg(res.Facts, "file_metadata_only", "data/rows.go") {
		t.Error("a stale artifact governs nothing: the file must be scanned as before")
	}
	if res.TreatmentShadow.EnforcedFiles != 0 {
		t.Errorf("stale artifact enforced %d files", res.TreatmentShadow.EnforcedFiles)
	}
}

// TestExcludeScopeLeavesCoverageAndNoFileRows pins /exclude: no per-file row,
// but the scope's coverage is recorded so the omission is visible.
func TestExcludeScopeLeavesCoverageAndNoFileRows(t *testing.T) {
	root, m := enforceFixture(t)
	art := workspace.TreatmentArtifact{
		SchemaVersion: workspace.TreatmentArtifactSchema,
		Workspace:     m.Root(),
		Generation:    m.Generation(),
		Treatments: []workspace.TreatmentRow{
			{Dir: "database", Treatment: workspace.TreatmentExclude, Authority: workspace.AuthorityAccessRestriction, Scope: "database", Reasons: []string{"/operator_access_restriction"}},
		},
	}
	body, err := art.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := workspace.TreatmentArtifactPath(m.Root())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := enforceScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if hasFirstArg(res.Facts, "file_topology", "database/conn.go") || hasFirstArg(res.Facts, "file_metadata_only", "database/conn.go") {
		t.Error("an excluded file has a per-file row")
	}
	if !hasFirstArg(res.Facts, "treatment_scope", "database") {
		t.Error("an excluded scope left no coverage record")
	}
	if !hasFirstArg(res.Facts, "file_topology", "data/rows.go") {
		t.Error("data/ shares a name prefix with database/ and must still be scanned")
	}
}

func coreFacts(in []types.Fact) []core.Fact {
	out := make([]core.Fact, len(in))
	for i, f := range in {
		out[i] = core.Fact(f)
	}
	return out
}

func openEnforceStore(t *testing.T) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestIncrementalScanRetiresDemotedFilesAndKeepsCoverageCurrent walks one
// workspace through demotion, a new file in the enforced scope, and
// promotion. Each step is checked on the result the kernel would apply.
func TestIncrementalScanRetiresDemotedFilesAndKeepsCoverageCurrent(t *testing.T) {
	root, m := enforceFixture(t)
	db := openEnforceStore(t)
	sc := enforceScanner()
	ctx := context.Background()

	first, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil || !first.Full {
		t.Fatalf("first scan: full=%v err=%v", first != nil && first.Full, err)
	}
	if !hasFirstArg(first.NewFacts, "file_topology", "data/rows.go") {
		t.Fatal("setup: data/rows.go should be indexed before demotion")
	}

	// Demotion: the scope becomes operator names-only.
	writeTreatmentArtifact(t, m, m.Generation(), workspace.AuthorityOperator)
	demoted, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"data/rows.go", "data/helper_test.go", "data/big/sample.json"} {
		if !slices.Contains(demoted.DeletedFiles, filepath.Join(root, filepath.FromSlash(p))) {
			t.Errorf("%s was not retired when its scope became names-only: deleted=%v", p, demoted.DeletedFiles)
		}
		if !hasFirstArg(demoted.RetractFacts, "file_topology", p) {
			t.Errorf("%s keeps its body-derived file_topology row after demotion", p)
		}
		if !hasFirstArg(demoted.TreatmentFacts, "file_metadata_only", p) {
			t.Errorf("%s has no metadata-only row after demotion", p)
		}
	}
	paths, err := db.ListWorldFilePaths()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(paths, "data/rows.go") {
		t.Error("the world cache still holds a row for a demoted file")
	}

	// A new file in the enforced scope changes nothing the cache can see, so
	// the scan is Unchanged, and still reports the file.
	if err := os.WriteFile(filepath.Join(root, "data", "added.go"), []byte("package data\n\nfunc Added() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTreatmentArtifact(t, m, enforceMembership(t, root).Generation(), workspace.AuthorityOperator)
	quiet, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !quiet.Unchanged {
		t.Errorf("a file under an enforced scope must not make the scan a delta: new=%v", quiet.NewFiles)
	}
	if !hasFirstArg(quiet.TreatmentFacts, "file_metadata_only", "data/added.go") {
		t.Error("an unchanged scan lost the new file's name")
	}
	k := &recordingKernel{}
	if err := ApplyIncrementalResult(k, quiet); err != nil {
		t.Fatal(err)
	}
	if !hasFirstArg(coreFacts(k.loaded), "file_metadata_only", "data/added.go") {
		t.Error("applying an unchanged result did not publish the coverage rows")
	}

	// Promotion: the override goes away, the files are read again.
	if err := os.Remove(workspace.TreatmentArtifactPath(root)); err != nil {
		t.Fatal(err)
	}
	promoted, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted.NewFiles) != 4 || len(promoted.TreatmentFacts) != 0 {
		t.Errorf("promotion: new=%d treatment rows=%d, want the 3 data files and added.go read again with no coverage rows", len(promoted.NewFiles), len(promoted.TreatmentFacts))
	}
	if !hasFirstArg(promoted.NewFacts, "file_topology", "data/rows.go") {
		t.Error("a promoted file was not hashed again")
	}
}
