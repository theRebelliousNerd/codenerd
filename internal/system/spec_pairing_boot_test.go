package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tools/codedom"
	"codenerd/internal/types"
)

// Specs are paired with code at boot: the world scan measures which Markdown
// section cites or specifies which element and loads the machine-checkable
// spec files, and the sharded kernel's own policy derives drift, dangling
// links and violated requirements. Nothing here is asserted by the test but
// the files on disk.
func TestSpecPairingIsDerivedOnTheBootedShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	old := time.Now().Add(-72 * time.Hour)
	write := func(rel, body string, mtime time.Time) {
		t.Helper()
		path := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/ws\n\ngo 1.26\n", old)
	write("internal/core/foo.go", `package core

// Eval derives the fixpoint.
//
// spec: Docs/guide.md#Guide > Evaluation
func Eval() {}

// Plan has no test and no section.
func Plan() {}
`, time.Now().Add(-time.Hour))
	write("Docs/guide.md", "# Guide\n\n## Evaluation\nThe entry is `internal/core/foo.go:Eval`, and `internal/core/foo.go:Gone` was removed.\n", old)
	write("Docs/specs/core.mg", `
spec_requirement(/plan_tested, "Plan is covered by a test").
requires(/plan_tested, "internal/core.Plan", /has_test).
requires(/plan_tested, "internal/core.Eval", /has_doc_comment).
`, old)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	rows := func(pred string) []string {
		t.Helper()
		facts, err := cortex.Kernel.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		return factStrings(facts)
	}
	has := func(pred, sub string) bool {
		for _, r := range rows(pred) {
			if strings.Contains(r, sub) {
				return true
			}
		}
		return false
	}

	section := "Docs/guide.md:Guide > Evaluation"
	if !has("spec_link", section+"|internal/core.Eval|/specifies|/code_comment") || !has("spec_link", section+"|internal/core.Eval|/cites|/doc_citation") {
		t.Errorf("the booted kernel holds no link between the section and Eval: %v", rows("spec_link"))
	}
	// Drift is content, not file dates: Eval's file is newer than the section, and a
	// first boot confirms every link as it stands.
	if got := rows("spec_drift"); len(got) != 0 {
		t.Errorf("a first boot confirms every link and has no drift: %v", got)
	}
	if !has("spec_confirmed", section+"|internal/core.Eval|") {
		t.Errorf("the booted kernel holds no confirmation for the Eval link: %v", rows("spec_confirmed"))
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "specs", "links.json")); err != nil {
		t.Errorf("the confirmations are persisted under .nerd: %v", err)
	}
	if !has("dangling_spec_link", section+"|internal/core/foo.go:Gone") {
		t.Errorf("the citation of a removed function must dangle: %v", rows("dangling_spec_link"))
	}
	if !has("unspecced_element", "internal/core.Plan") {
		t.Errorf("Plan is exported in a package a spec reaches and no spec names it: %v", rows("unspecced_element"))
	}
	// confirm_spec_link reaches the booted kernel: the production wiring, not a test double.
	out, err := codedom.ConfirmSpecLinkTool().Execute(ctx, map[string]any{"spec_ref": section, "code_ref": "internal/core.Eval"})
	if err != nil || !strings.Contains(out, "Confirmed") {
		t.Errorf("confirm_spec_link through the booted wiring: %q %v", out, err)
	}
	if got := rows("requirement_violated"); len(got) != 1 || !strings.Contains(got[0], "/plan_tested|internal/core.Plan|/has_test") {
		t.Errorf("only the missing test violates a requirement: %v", got)
	}
}

func factStrings(facts []types.Fact) []string {
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		parts := make([]string, len(f.Args))
		for i, a := range f.Args {
			parts[i] = fmt.Sprint(a)
		}
		out = append(out, strings.Join(parts, "|"))
	}
	return out
}
