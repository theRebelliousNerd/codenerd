package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// docsEvidenceWorkspace builds a one-package docs tree in a temp git repo.
func docsEvidenceWorkspace(t *testing.T, witness string) string {
	t.Helper()
	root := t.TempDir()
	adr := "---\ndoc-class: governance\nimplementation-status: shipped\nlast-verified: 2026-10-28\nverified-against: abc\n---\n\n# ADR\n\n**Witness:** " + witness + "\n"
	for rel, body := range map[string]string{
		"Docs/architecture/pkg/adr/ADR-001.md": adr,
		"p/p.go":                               "package p\n\nfunc Declared() {}\n\n// OnlyComment is a comment.\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

// The command reports clean only for a complete scan whose required
// references reached a satisfying grade, and prints the manifest either way.
func TestDocsEvidenceInto(t *testing.T) {
	var out bytes.Buffer
	clean, err := docsEvidenceInto(&out, docsEvidenceWorkspace(t, "symbol:Declared"), []string{"pkg"}, false)
	if err != nil || !clean {
		t.Fatalf("declared witness: clean=%v err=%v\n%s", clean, err, out.String())
	}
	for _, want := range []string{"-> declared", "manifest: status=complete scope=subset", "not observed by this view"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	clean, err = docsEvidenceInto(&out, docsEvidenceWorkspace(t, "symbol:OnlyComment"), []string{"pkg"}, true)
	if err != nil || clean {
		t.Fatalf("a comment-only witness must not be clean: clean=%v err=%v", clean, err)
	}
	if !strings.Contains(out.String(), `"grade":"occurrence_only"`) || !strings.Contains(out.String(), `"record":"manifest"`) {
		t.Fatalf("json report:\n%s", out.String())
	}

	if _, err := docsEvidenceInto(&out, docsEvidenceWorkspace(t, "symbol:Declared"), []string{"missing"}, false); err == nil {
		t.Fatal("unknown package must error")
	}
}
