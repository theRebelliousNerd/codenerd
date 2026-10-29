package world

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codenerd/internal/workspace"
)

func shadowFixture(t *testing.T) (string, *workspace.Membership) {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"src/main.go":          "package main\n\nfunc main() {}\n",
		"data/rows.go":         "package data\n\nfunc Rows() {}\n",
		"data/big/sample.json": `{"a": 1}`,
		"database/conn.go":     "package database\n\nfunc Open() {}\n",
		"contested/x.py":       "def f():\n    pass\n",
		"contested/inner/y.py": "def g():\n    pass\n",
		"data/helper_test.go":  "package data\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := workspace.Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	return root, m
}

func writeShadowArtifact(t *testing.T, m *workspace.Membership, generation string) {
	writeTreatmentArtifact(t, m, generation, workspace.AuthorityInferred)
}

// writeTreatmentArtifact writes the fixture artifact with every names-only row
// at the given authority.
func writeTreatmentArtifact(t *testing.T, m *workspace.Membership, generation, authority string) {
	t.Helper()
	art := workspace.TreatmentArtifact{
		SchemaVersion: workspace.TreatmentArtifactSchema,
		Workspace:     m.Root(),
		Generation:    generation,
		Treatments: []workspace.TreatmentRow{
			{Dir: "data", Treatment: workspace.TreatmentIndexNamesOnly, Authority: authority, Scope: "data", Reasons: []string{"/operator_override"}},
			{Dir: "data/big", Treatment: workspace.TreatmentIndexNamesOnly, Authority: authority, Scope: "data", Reasons: []string{"/operator_override"}},
		},
		Conflicts: []workspace.TreatmentConflict{
			{Dir: "contested", Scope: "contested", Treatments: []workspace.Treatment{workspace.TreatmentIndexAndParse, workspace.TreatmentIndexNamesOnly}},
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
}

func shadowScanner() *Scanner {
	cfg := DefaultScannerConfig()
	cfg.IgnorePatterns = []string{}
	cfg.MaxConcurrency = 2
	return NewScannerWithConfig(cfg)
}

func sortedFactStrings(r *ScanResult) []string {
	out := factStrings(r.Facts)
	sort.Strings(out)
	return out
}

func TestScanRecordsTreatmentShadowWithoutChangingFacts(t *testing.T) {
	root, m := shadowFixture(t)
	writeShadowArtifact(t, m, m.Generation())

	withShadow, err := shadowScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	sh := withShadow.TreatmentShadow
	if sh == nil || !sh.Active || !sh.Current {
		t.Fatalf("shadow %+v, want an active current view", sh)
	}
	// data/rows.go, data/helper_test.go, data/big/sample.json: names-only.
	// database/ shares a prefix with data/ but no segment, so it is not counted.
	if sh.Files != 3 {
		t.Fatalf("shadowed files %d, want 3 (%+v)", sh.Files, sh.Dirs)
	}
	if sh.Hashes != 3 {
		t.Errorf("hashes that would be avoided %d, want 3 cold-cache hashes", sh.Hashes)
	}
	// rows.go and sample.json are read for parsing; helper_test.go for its header.
	if sh.BodyReads != 3 {
		t.Errorf("body reads that would be avoided %d, want 3", sh.BodyReads)
	}
	if sh.ConflictedFiles != 2 {
		t.Errorf("conflicted files %d, want 2", sh.ConflictedFiles)
	}
	if len(sh.Dirs) != 2 || sh.Dirs[0].Dir != "data" || sh.Dirs[0].Files != 2 || sh.Dirs[1].Dir != "data/big" || sh.Dirs[1].Files != 1 {
		t.Errorf("per-directory shadow %+v", sh.Dirs)
	}
	if sh.Bytes <= 0 {
		t.Errorf("shadowed bytes %d", sh.Bytes)
	}

	// The same tree scanned with no artifact yields the same facts: shadow
	// mode changes no hashing, parsing or fact.
	if err := os.Remove(workspace.TreatmentArtifactPath(m.Root())); err != nil {
		t.Fatal(err)
	}
	without, err := shadowScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if without.TreatmentShadow == nil || without.TreatmentShadow.Active || without.TreatmentShadow.Unavailable != "" {
		t.Fatalf("no artifact must be an inactive shadow, got %+v", without.TreatmentShadow)
	}
	a, b := sortedFactStrings(withShadow), sortedFactStrings(without)
	if len(a) != len(b) {
		t.Fatalf("facts differ in count: %d with shadow, %d without", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("facts differ:\n%s\n%s", a[i], b[i])
		}
	}
	if withShadow.FileCount != without.FileCount || withShadow.DirectoryCount != without.DirectoryCount {
		t.Fatalf("counts differ: %d/%d vs %d/%d", withShadow.FileCount, withShadow.DirectoryCount, without.FileCount, without.DirectoryCount)
	}
}

func TestScanTreatmentShadowRefusesForeignAndMarksStale(t *testing.T) {
	root, m := shadowFixture(t)
	writeShadowArtifact(t, m, "sha256:older-generation")
	res, err := shadowScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if sh := res.TreatmentShadow; !sh.Active || sh.Current {
		t.Fatalf("stale artifact shadow %+v, want active and not current", sh)
	}

	other, otherMem := shadowFixture(t)
	writeShadowArtifact(t, otherMem, otherMem.Generation())
	copied, err := os.ReadFile(workspace.TreatmentArtifactPath(otherMem.Root()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspace.TreatmentArtifactPath(m.Root()), copied, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = shadowScanner().ScanDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if sh := res.TreatmentShadow; sh.Active || sh.Unavailable == "" || sh.Files != 0 {
		t.Fatalf("artifact copied from %s was used: %+v", other, sh)
	}
}
