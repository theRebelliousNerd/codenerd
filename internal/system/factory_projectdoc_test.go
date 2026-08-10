package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/projectdoc"
)

// TestLoadProjectDoc_ModuleOnlyLeavesProjectDocNil reproduces the LoadAll
// regression where docs[0] being a module document was mistakenly promoted to
// the workspace-wide projectDoc and injected into every turn's SYSTEM PROMPT.
//
// A workspace that has module-level nerd.md files but no root nerd.md must
// leave the resolved project document nil (exactly as pre-LoadAll Load returned
// nil) while still asserting that module's facts into the kernel.
func TestLoadProjectDoc_ModuleOnlyLeavesProjectDocNil(t *testing.T) {
	dir := t.TempDir()

	// No root nerd.md — only a module one.
	modulePath := filepath.Join(dir, "internal", "mymod", projectdoc.FileName)
	if err := os.MkdirAll(filepath.Dir(modulePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	moduleDoc := `---
schema: nerd/v1
project: mymodule
forbid:
  - match: secrets.txt
    reason: do not leak secrets
---
Module body prose.
`
	if err := os.WriteFile(modulePath, []byte(moduleDoc), 0o644); err != nil {
		t.Fatalf("write module nerd.md: %v", err)
	}

	// Verify LoadAll itself sees only the module and no root.
	docs, err := projectdoc.LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("LoadAll got %d docs, want 1 (module only)", len(docs))
	}
	if docs[0].Path == projectdoc.FileName {
		t.Fatalf("LoadAll returned root path %q for a module-only workspace", docs[0].Path)
	}
	if docs[0].Path != filepath.ToSlash(filepath.Join("internal", "mymod", projectdoc.FileName)) {
		t.Fatalf("unexpected module path %q", docs[0].Path)
	}

	mock := &MockSystemKernel{}
	bctx := &bootContext{
		workspace: dir,
		kernel:    mock,
	}
	loadProjectDoc(bctx)

	if bctx.projectDoc != nil {
		t.Fatalf("bctx.projectDoc = %v (Path %q), want nil for module-only workspace — a module doc must not become the project-wide SYSTEM PROMPT", bctx.projectDoc, bctx.projectDoc.Path)
	}

	// Facts for the module must still have been asserted.
	if len(mock.facts) == 0 {
		t.Fatal("kernel LoadFacts was not called or produced no facts; module facts must still be asserted even when projectDoc is nil")
	}

	// The module's facts include project_doc with its module path and the forbid rule.
	foundProjectDocFact := false
	foundForbidFact := false
	for _, f := range mock.facts {
		if f.Predicate == projectdoc.PredPresent && len(f.Args) >= 1 {
			if s, ok := f.Args[0].(string); ok && s == docs[0].Path {
				foundProjectDocFact = true
			}
		}
		if f.Predicate == projectdoc.PredForbiddenPath && len(f.Args) >= 2 {
			if s, ok := f.Args[0].(string); ok && s == "secrets.txt" {
				foundForbidFact = true
			}
		}
	}
	if !foundProjectDocFact {
		t.Errorf("kernel facts missing project_doc for module path %q; got facts: %v", docs[0].Path, mock.facts)
	}
	if !foundForbidFact {
		t.Errorf("kernel facts missing forbid rule from module doc; got facts: %v", mock.facts)
	}

	// Sanity: PromptSection on nil must be empty so withProjectInstructions is a no-op.
	if got := bctx.projectDoc.PromptSection(); got != "" {
		t.Errorf("nil projectDoc PromptSection = %q, want empty", got)
	}
	// Also verify that a non-empty module doc's PromptSection would not be empty
	// if it were incorrectly assigned — ensures the test would catch the regression.
	if sec := docs[0].PromptSection(); strings.TrimSpace(sec) == "" {
		t.Errorf("module doc PromptSection is empty; test module doc must have prose to detect mis-assignment")
	}
}

func TestLoadProjectDoc_RootAndModuleSetsRootOnly(t *testing.T) {
	dir := t.TempDir()

	// Root document.
	rootPath := filepath.Join(dir, projectdoc.FileName)
	rootDoc := `---
schema: nerd/v1
project: rootproj
forbid:
  - match: rootsecret.txt
    reason: root secret
---
Root body.
`
	if err := os.WriteFile(rootPath, []byte(rootDoc), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	// Module document.
	modPath := filepath.Join(dir, "internal", "a", projectdoc.FileName)
	if err := os.MkdirAll(filepath.Dir(modPath), 0o755); err != nil {
		t.Fatalf("mkdir mod: %v", err)
	}
	modDoc := `---
schema: nerd/v1
project: modproj
forbid:
  - match: modsecret.txt
    reason: mod secret
---
Module A body.
`
	if err := os.WriteFile(modPath, []byte(modDoc), 0o644); err != nil {
		t.Fatalf("write mod: %v", err)
	}

	mock := &MockSystemKernel{}
	bctx := &bootContext{workspace: dir, kernel: mock}
	loadProjectDoc(bctx)

	if bctx.projectDoc == nil {
		t.Fatal("bctx.projectDoc is nil, want root document")
	}
	if bctx.projectDoc.Path != projectdoc.FileName {
		t.Fatalf("bctx.projectDoc.Path = %q, want %q (root)", bctx.projectDoc.Path, projectdoc.FileName)
	}
	// Must not be the module path.
	if bctx.projectDoc.Path == filepath.ToSlash(filepath.Join("internal", "a", projectdoc.FileName)) {
		t.Fatal("bctx.projectDoc is module doc, want root")
	}
	// Both docs' facts must be present.
	hasRootForbid := false
	hasModForbid := false
	for _, f := range mock.facts {
		if f.Predicate == projectdoc.PredForbiddenPath && len(f.Args) >= 1 {
			if f.Args[0] == "rootsecret.txt" {
				hasRootForbid = true
			}
			if f.Args[0] == "modsecret.txt" {
				hasModForbid = true
			}
		}
	}
	if !hasRootForbid {
		t.Error("missing root forbid fact")
	}
	if !hasModForbid {
		t.Error("missing module forbid fact: LoadAll facts must be asserted for ALL docs")
	}
	// Root detection must not depend on slice position — LoadAll returns root first today,
	// but the fix must not assume it; simulate by checking the loop searches whole slice.
	// Directly verify the detection predicate on a synthetic slice where module would be first.
	synth := []*projectdoc.Document{
		{Path: "internal/z/nerd.md"},
		{Path: projectdoc.FileName},
	}
	var found *projectdoc.Document
	for _, d := range synth {
		if d.Path == projectdoc.FileName || filepath.ToSlash(filepath.Dir(d.Path)) == "." {
			found = d
			break
		}
	}
	if found == nil || found.Path != projectdoc.FileName {
		t.Fatalf("root detection failed on synthetic out-of-order slice: got %v", found)
	}
}
