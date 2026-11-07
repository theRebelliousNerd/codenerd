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

	// A first scan confirms every link as it stands: the docs are two days old and
	// foo.go an hour, and that is not drift (drift is content, not file dates).
	if drift := factRows(t, kernel, "spec_drift"); len(drift) != 0 {
		t.Errorf("a fresh checkout has no drift: %v", drift)
	}
	if confirmed := factRows(t, kernel, "spec_confirmed"); len(confirmed) != 3 {
		t.Errorf("the three resolvable links are confirmed on first sight: %v", confirmed)
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

// TestSpecDriftIsJudgedPerElementFromContent is the production path end to end:
// the scan fingerprints each linked element and section and confirms a link when
// it first sees it; an edit to one function's body raises drift for that
// function's link only, a reformat or an edit to a neighbour raises none,
// confirming through the review tool's confirmer clears it, a changed section
// makes the unchanged code stale, and both sides changing confirms the pair again.
func TestSpecDriftIsJudgedPerElementFromContent(t *testing.T) {
	ctx := context.Background()
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	root := specWorkspace(t, map[string]string{
		"go.mod":               "module example.com/ws\n\ngo 1.26\n",
		"internal/core/foo.go": specGoFile,
		"Docs/guide.md":        specGuide,
	}, stamp, stamp)
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	var clock int64 = 1000
	scanner := NewScanner()
	scanner.specMeasure.now = func() int64 { return clock }
	scan := func(label string) {
		t.Helper()
		clock += 1000
		res, err := scanner.Refresh(ctx, root, nil, kernel, IncrementalOptions{})
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if res == nil || !res.SpecMeasured {
			t.Fatalf("%s: the pairing was not measured", label)
		}
	}
	edit := func(rel, from, to string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !strings.Contains(string(data), from) {
			t.Fatalf("edit %s: %q not found (%v)", rel, from, err)
		}
		writeSpecFile(t, root, rel, strings.Replace(string(data), from, to, 1), stamp, stamp)
	}
	judged := func(pred string) []string { return factRows(t, kernel, pred) }

	eval := "Docs/guide.md:Guide > Evaluation"
	funcLink, evalLink, kernelLink := eval+"|internal/core.Func", eval+"|internal/core.Kernel.Eval", eval+"|internal/core.Kernel"

	scan("first scan")
	if got := judged("spec_drift"); len(got) != 0 {
		t.Fatalf("first scan: %v", got)
	}

	// A reformat of Func and an edit to a function no link names change no fingerprint
	// a link depends on.
	edit("internal/core/foo.go", "func Func() {}", "func Func() {\n}")
	edit("internal/core/foo.go", "func Unspecced() {}", "func Unspecced() { helper() }")
	scan("reformat and neighbour edit")
	if got := judged("spec_drift"); len(got) != 0 {
		t.Fatalf("a reformat and an edit to another function must not drift any link: %v", got)
	}

	// Func's body changes: the link to Func drifts, its neighbours' do not.
	edit("internal/core/foo.go", "func Func() {\n}", "func Func() { helper() }")
	scan("edit Func")
	if got := judged("spec_drift"); len(got) != 1 || got[0] != funcLink {
		t.Fatalf("only the link to the edited function drifts: %v", got)
	}
	firstSeen := clock
	if got := judged("spec_drift_since"); len(got) != 1 || got[0] != funcLink+"|"+fmt.Sprint(firstSeen) {
		t.Fatalf("drift is dated by the scan that first saw it: %v", got)
	}
	if got := judged("spec_stale_code"); len(got) != 0 {
		t.Fatalf("the section did not move: %v", got)
	}
	// Git history, when it is loaded, names the commit: a commit after the link was
	// confirmed (the first scan, at 2000) is attached, one before it is not.
	for _, c := range []struct {
		hash string
		at   int64
	}{{"before1", 500}, {"after01", 2500}} {
		if err := kernel.Assert(core.Fact{Predicate: "git_history", Args: []any{"internal/core/foo.go", c.hash, "ann", c.at, "change"}}); err != nil {
			t.Fatal(err)
		}
	}
	if got := judged("spec_drift_commit"); len(got) != 1 || got[0] != funcLink+"|after01|2500" {
		t.Fatalf("drift_commit: %v", got)
	}

	// The same body again, rescanned: still drifted, and still dated by the first sighting.
	edit("internal/core/foo.go", "func Func() { helper() }", "func Func() { helper(); helper() }")
	scan("edit Func again")
	if got := judged("spec_drift_since"); len(got) != 1 || got[0] != funcLink+"|"+fmt.Sprint(firstSeen) {
		t.Fatalf("drift keeps the first sighting: %v", got)
	}

	// Confirming (the confirm_spec_link tool's confirmer) clears that link, in the
	// kernel at once and in the store for the next process.
	confirmer := NewSpecLinkConfirmer(kernel, root)
	confirmer.now = func() int64 { return clock + 1 }
	out, err := confirmer.ConfirmSpecLink(eval, "internal/core.Func")
	if err != nil || !strings.Contains(out, "drifted") {
		t.Fatalf("confirm: %q %v", out, err)
	}
	if got := judged("spec_drift"); len(got) != 0 {
		t.Fatalf("a confirmed link no longer drifts: %v", got)
	}
	if _, err := confirmer.ConfirmSpecLink(eval, "internal/core.Nope"); err == nil || !strings.Contains(err.Error(), "no link between") {
		t.Fatalf("confirming a link that does not exist must say so: %v", err)
	}
	fresh := NewScanner()
	fresh.specMeasure.now = func() int64 { return clock + 5 }
	freshKernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.Refresh(ctx, root, nil, freshKernel, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := factRows(t, freshKernel, "spec_drift"); len(got) != 0 {
		t.Fatalf("the confirmation outlives the process: %v", got)
	}

	// The section changes: the code behind it did not, so it is stale.
	clock += 10
	edit("Docs/guide.md", "The kernel is", "The kernel was")
	scan("edit the guide")
	stale := judged("spec_stale_code")
	if !hasRow(stale, evalLink) || !hasRow(stale, kernelLink) || !hasRow(stale, funcLink) {
		t.Fatalf("every link into the edited section is stale: %v", stale)
	}
	if got := judged("spec_drift"); len(got) != 0 {
		t.Fatalf("the code did not move: %v", got)
	}

	// Func then follows the section: both sides moved, so that pair is confirmed
	// again. The links whose code did not move are still stale.
	edit("internal/core/foo.go", "func Func() { helper(); helper() }", "func Func() { helper(); helper(); helper() }")
	scan("edit Func after the guide")
	stale = judged("spec_stale_code")
	if hasRow(stale, funcLink) || !hasRow(stale, evalLink) || !hasRow(stale, kernelLink) || len(judged("spec_drift")) != 0 {
		t.Fatalf("Func followed the section and is confirmed again; the others stay stale: stale=%v drift=%v", stale, judged("spec_drift"))
	}
}

// A confirmations file that cannot be read stops the measurement and says why,
// rather than judging every link as first seen over a reviewer's decisions.
func TestSpecPairingRefusesAnUnreadableConfirmationFile(t *testing.T) {
	ctx := context.Background()
	stamp := time.Now().Add(-time.Hour)
	root := specWorkspace(t, map[string]string{
		"go.mod":                 "module example.com/ws\n\ngo 1.26\n",
		"internal/core/foo.go":   specGoFile,
		"Docs/guide.md":          specGuide,
		".nerd/specs/links.json": "{broken",
	}, stamp, stamp)
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewScanner().Refresh(ctx, root, nil, kernel, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.SpecMeasured || len(res.SpecWarnings) != 1 || !strings.Contains(res.SpecWarnings[0], "links.json") {
		t.Fatalf("measured=%v warnings=%v", res.SpecMeasured, res.SpecWarnings)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".nerd", "specs", "links.json")); string(data) != "{broken" {
		t.Fatalf("the unreadable file was overwritten: %q", data)
	}
}
