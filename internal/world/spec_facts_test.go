package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

// specWorkspace writes files (path -> body) under a temp root, giving each a
// modification time: docs at docTime, everything else at codeTime.
func specWorkspace(t *testing.T, files map[string]string, docTime, codeTime time.Time) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		writeSpecFile(t, root, rel, body, docTime, codeTime)
	}
	return root
}

func writeSpecFile(t *testing.T, root, rel, body string, docTime, codeTime time.Time) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := codeTime
	if strings.HasSuffix(rel, ".md") {
		stamp = docTime
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

const specGoFile = `package core

// Kernel evaluates facts.
type Kernel struct{}

// Eval derives the fixpoint.
//
// spec: Docs/guide.md#Guide > Evaluation
func (k *Kernel) Eval() { helper() }

// Func is documented and tested.
func Func() {}

func helper() {}

// Unspecced is exported and no section names it.
func Unspecced() {}
`

const specGuide = "# Guide\n\n## Evaluation\nThe kernel is `internal/core/foo.go:Kernel` and `Func` in `internal/core/foo.go`.\n\n## Stale\nThis cites `internal/core/foo.go:Removed`.\n"

const specRequirements = `
spec_requirement(/eval, "Eval is exported, documented, tested and calls helper").
requires(/eval, "internal/core.Kernel.Eval", /exported).
requires(/eval, "internal/core.Kernel.Eval", /has_doc_comment).
requires(/eval, "internal/core.Kernel.Eval", /has_test).
requires_calls(/eval, "internal/core.Kernel.Eval", "internal/core.helper").
requires(/eval, "internal/core.Func", /has_test).
requires(/eval, "internal/core.Gone", /exists).
`

func factRows(t *testing.T, k *core.RealKernel, pred string) []string {
	t.Helper()
	facts, err := k.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	var out []string
	for _, f := range facts {
		parts := make([]string, len(f.Args))
		for i, a := range f.Args {
			parts[i] = fmt.Sprint(a)
		}
		out = append(out, strings.Join(parts, "|"))
	}
	sort.Strings(out)
	return out
}

func hasRow(rows []string, want string) bool {
	for _, r := range rows {
		if r == want {
			return true
		}
	}
	return false
}

// TestSpecPairingThroughTheScan is the production path: a scan measures the
// pairing, applying the result loads it into a real kernel, and the kernel's
// own policy judges drift, dangling links, coverage and requirements. A second
// scan after an edit re-measures; a scan after nothing changed does not.
func TestSpecPairingThroughTheScan(t *testing.T) {
	ctx := context.Background()
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	recent := time.Now().Add(-time.Hour).Truncate(time.Second)
	root := specWorkspace(t, map[string]string{
		"go.mod":                    "module example.com/ws\n\ngo 1.26\n",
		"internal/core/foo.go":      specGoFile,
		"internal/core/foo_test.go": "package core\n\nimport \"testing\"\n\nfunc TestEval(t *testing.T) { var k Kernel; k.Eval(); Func() }\n",
		"Docs/guide.md":             specGuide,
		"Docs/specs/kernel.mg":      specRequirements,
	}, old, recent)

	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner()
	res, err := scanner.Refresh(ctx, root, nil, kernel, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.SpecMeasured || len(res.SpecWarnings) != 0 {
		t.Fatalf("spec pairing not measured: measured=%v warnings=%v", res.SpecMeasured, res.SpecWarnings)
	}

	eval := "Docs/guide.md:Guide > Evaluation"
	stale := "Docs/guide.md:Guide > Stale"
	links := factRows(t, kernel, "spec_link")
	for _, want := range []string{
		eval + "|internal/core.Kernel|/cites|/doc_citation",
		eval + "|internal/core.Func|/cites|/doc_citation",
		eval + "|internal/core.Kernel.Eval|/specifies|/code_comment",
		stale + "|internal/core/foo.go:Removed|/cites|/doc_citation",
	} {
		if !hasRow(links, want) {
			t.Errorf("missing spec_link %s in %v", want, links)
		}
	}

	// The docs are two days old and foo.go an hour: every linked element moved after its section.
	drift := factRows(t, kernel, "spec_drift")
	if !hasRow(drift, eval+"|internal/core.Func") || !hasRow(drift, eval+"|internal/core.Kernel.Eval") {
		t.Errorf("drift: %v", drift)
	}
	if dangling := factRows(t, kernel, "dangling_spec_link"); !hasRow(dangling, stale+"|internal/core/foo.go:Removed") || len(dangling) != 1 {
		t.Errorf("dangling: %v", dangling)
	}
	if unspecced := factRows(t, kernel, "unspecced_element"); !hasRow(unspecced, "internal/core.Unspecced") || hasRow(unspecced, "internal/core.Func") {
		t.Errorf("unspecced: %v", unspecced)
	}
	if cov := factRows(t, kernel, "spec_coverage"); len(cov) != 1 || !strings.HasPrefix(cov[0], "internal/core|") {
		t.Errorf("coverage: %v", cov)
	}
	// Eval is exported, documented, tested (TestEval calls it) and calls helper; Func is tested; Gone is not there.
	violated := factRows(t, kernel, "requirement_violated")
	if len(violated) != 1 || violated[0] != "/eval|internal/core.Gone|/exists" {
		t.Errorf("requirement_violated: %v", violated)
	}

	// Nothing changed: nothing is measured, the kernel keeps its rows.
	again, err := scanner.Refresh(ctx, root, nil, kernel, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil && again.SpecMeasured {
		t.Fatal("an unchanged workspace was measured again")
	}
	if n := len(factRows(t, kernel, "spec_link")); n != len(links) {
		t.Fatalf("links changed without a change: %d -> %d", len(links), n)
	}

	// Edit the code: Func goes away. The section that cited it now cites nothing.
	writeSpecFile(t, root, "internal/core/foo.go", strings.Replace(specGoFile, "func Func() {}", "", 1), old, time.Now().Truncate(time.Second))
	edited, err := scanner.Refresh(ctx, root, nil, kernel, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if edited == nil || !edited.SpecMeasured {
		t.Fatal("an edited code file was not re-measured")
	}
	dangling := factRows(t, kernel, "dangling_spec_link")
	if hasRow(dangling, eval+"|internal/core.Func") {
		t.Errorf("Func is gone; the link to its ref cannot stay a real one: %v", dangling)
	}
	if !hasRow(factRows(t, kernel, "requirement_violated"), "/eval|internal/core.Func|/has_test") {
		t.Errorf("a requirement on an element that is gone must be violated: %v", factRows(t, kernel, "requirement_violated"))
	}
}

func TestSpecFileThatIsNotDataIsRefusedAndNamed(t *testing.T) {
	ctx := context.Background()
	root := specWorkspace(t, map[string]string{
		"go.mod":               "module example.com/ws\n\ngo 1.26\n",
		"a.go":                 "package main\nfunc main() {}\n",
		"Docs/specs/bad.mg":    "spec_requirement(/r, \"x\").\npermitted(/anything) :- spec_requirement(/r, _).\n",
		"Docs/specs/others.mg": "spec_requirement(/ok, \"fine\").\nrequires(/ok, \"main.main\", /exists).\n",
	}, time.Now(), time.Now())
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewScanner().Refresh(ctx, root, nil, kernel, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SpecWarnings) != 1 || !strings.Contains(res.SpecWarnings[0], "bad.mg") {
		t.Fatalf("warnings: %v", res.SpecWarnings)
	}
	if reqs := factRows(t, kernel, "spec_requirement"); len(reqs) != 1 || !strings.HasPrefix(reqs[0], "/ok") {
		t.Fatalf("the refused file must contribute nothing and the good one must load: %v", reqs)
	}
}
