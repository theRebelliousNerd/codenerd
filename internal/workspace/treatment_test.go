package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func treatmentWorkspace(t *testing.T) *Membership {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{"data/a.go", "data/x/b.go", "database/c.go", "fx/d.py", "fx/in/e.py", "top.go"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func writeArtifact(t *testing.T, m *Membership, a TreatmentArtifact) {
	t.Helper()
	body, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	p := TreatmentArtifactPath(m.Root())
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleArtifact(m *Membership) TreatmentArtifact {
	return TreatmentArtifact{
		SchemaVersion: TreatmentArtifactSchema,
		Workspace:     m.Root(),
		Generation:    m.Generation(),
		Treatments: []TreatmentRow{
			{Dir: "data/x", Treatment: TreatmentIndexNamesOnly, Authority: "/operator", Scope: "data", Reasons: []string{"/b", "/a"}},
			{Dir: "data", Treatment: TreatmentIndexNamesOnly, Authority: "/operator", Scope: "data", Reasons: []string{"/operator_override"}},
		},
		Conflicts: []TreatmentConflict{
			{Dir: "fx", Scope: "fx", Treatments: []Treatment{TreatmentIndexNamesOnly, TreatmentIndexAndParse}},
		},
	}
}

func TestTreatmentViewLooksUpBySegment(t *testing.T) {
	m := treatmentWorkspace(t)
	writeArtifact(t, m, sampleArtifact(m))
	v, err := m.TreatmentView()
	if err != nil || v == nil {
		t.Fatalf("view %v, %v", v, err)
	}
	if !v.Current() || v.Generation() != m.Generation() {
		t.Fatalf("view not bound to the current generation")
	}
	for rel, want := range map[string]TreatmentDecision{
		"data/a.go":      {Treatment: TreatmentIndexNamesOnly, Dir: "data", Authority: "/operator"},
		"data/x/b.go":    {Treatment: TreatmentIndexNamesOnly, Dir: "data/x", Authority: "/operator"},
		"data/x/deep/z":  {Treatment: TreatmentIndexNamesOnly, Dir: "data/x", Authority: "/operator"},
		"database/c.go":  {},
		"datab":          {},
		"top.go":         {},
		"fx/d.py":        {Dir: "fx", Conflict: true},
		"fx/in/e.py":     {Dir: "fx", Conflict: true},
		"../data/a.go":   {},
		"/abs/data/a.go": {},
	} {
		if got := v.Treatment(rel); got != want {
			t.Errorf("Treatment(%q) = %+v, want %+v", rel, got, want)
		}
	}
	var nilView *TreatmentView
	if got := nilView.Treatment("data/a.go"); got != (TreatmentDecision{}) {
		t.Errorf("nil view decided %+v", got)
	}
}

func TestTreatmentViewDoesNotChangeMembership(t *testing.T) {
	m := treatmentWorkspace(t)
	beforeFiles := m.Files()
	writeArtifact(t, m, sampleArtifact(m))
	if _, err := m.TreatmentView(); err != nil {
		t.Fatal(err)
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := m.Files(); !reflect.DeepEqual(got, beforeFiles) {
		t.Fatalf("Files changed with a treatment artifact present: %v vs %v", got, beforeFiles)
	}
	for _, rel := range []string{"data/a.go", "data/x/b.go", "fx/d.py"} {
		if !m.Includes(rel) {
			t.Errorf("%s left membership", rel)
		}
		ok, err := m.Admit(filepath.Join(m.Root(), filepath.FromSlash(rel)), false)
		if err != nil || !ok {
			t.Errorf("Admit(%s) = %v, %v", rel, ok, err)
		}
	}
	if !m.IncludesDir("data") || !m.IncludesDir("data/x") {
		t.Error("a names-only directory stopped being walkable")
	}
}

func TestTreatmentViewStaleMissingAndForeign(t *testing.T) {
	m := treatmentWorkspace(t)
	if v, err := m.TreatmentView(); v != nil || err != nil {
		t.Fatalf("missing artifact: %v, %v; want nil, nil", v, err)
	}
	a := sampleArtifact(m)
	a.Generation = "sha256:other"
	writeArtifact(t, m, a)
	v, err := m.TreatmentView()
	if err != nil {
		t.Fatal(err)
	}
	if v.Current() || !v.Treatment("data/a.go").Stale {
		t.Fatal("a generation mismatch must be visible as stale")
	}
	a = sampleArtifact(m)
	a.Workspace = "/some/other/checkout"
	writeArtifact(t, m, a)
	if _, err := m.TreatmentView(); !errors.Is(err, ErrTreatmentIdentity) {
		t.Fatalf("foreign artifact: %v", err)
	}
	a = sampleArtifact(m)
	a.SchemaVersion = TreatmentArtifactSchema + 1
	writeArtifact(t, m, a)
	if _, err := m.TreatmentView(); !errors.Is(err, ErrTreatmentIdentity) {
		t.Fatalf("future schema: %v", err)
	}
	if err := os.WriteFile(TreatmentArtifactPath(m.Root()), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.TreatmentView(); err == nil {
		t.Fatal("malformed artifact accepted")
	}
}

func TestTreatmentArtifactEncodeIsDeterministic(t *testing.T) {
	m := treatmentWorkspace(t)
	a := sampleArtifact(m)
	first, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	a.Treatments[0], a.Treatments[1] = a.Treatments[1], a.Treatments[0]
	a.Treatments[1].Reasons = []string{"/a", "/b"}
	second, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("encode depends on input order:\n%s\n%s", first, second)
	}
}

func TestGenerationTracksMemberSet(t *testing.T) {
	m := treatmentWorkspace(t)
	g1 := m.Generation()
	if g1 != GenerationOf([]string{"top.go", "fx/in/e.py", "fx/d.py", "database/c.go", "data/x/b.go", "data/a.go"}) {
		t.Fatal("generation is not the digest of the member list")
	}
	if err := os.WriteFile(filepath.Join(m.Root(), "data", "a.go"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if m.Generation() != g1 {
		t.Fatal("a content edit moved the member generation")
	}
	if err := os.WriteFile(filepath.Join(m.Root(), "new.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if m.Generation() == g1 {
		t.Fatal("a new member did not move the generation")
	}
}
