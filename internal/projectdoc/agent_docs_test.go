package projectdoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Every agents.md in this repository has current twins: the tracked nerd.md
// bodies always, and a local CLAUDE.md when one exists.
func TestAgentDocTwinsAreCurrent(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := StaleAgentDocTwins(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	gen, ok := doc.Spec.Generator(AgentDocGenerator)
	if !ok {
		t.Fatalf("the root nerd.md declares no %q generator", AgentDocGenerator)
	}
	twins, err := AgentDocTwins(root)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, tw := range twins {
		want = append(want, tw.Path)
	}
	if strings.Join(gen.Outputs, ",") != strings.Join(want, ",") {
		t.Fatalf("generator %q outputs\n  %v\nwant the twins\n  %v", AgentDocGenerator, gen.Outputs, want)
	}
	if len(stale) > 0 {
		t.Fatalf("stale agent-doc twins (generator %q: go run ./cmd/tools/agent_docs):\n  %s",
			AgentDocGenerator, strings.Join(stale, "\n  "))
	}
}

func gitWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	return root
}

func writeRel(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAgentDocTwins_GenerateKeepFrontmatterAndSkipMirrors(t *testing.T) {
	root := gitWorkspace(t)
	writeRel(t, root, "pkg/a/agents.md", "# A\r\n\r\nGuidance for a.\r\n")
	writeRel(t, root, "pkg/b/AGENTS.md", "# B\n")
	writeRel(t, root, "pkg/b/nerd.md", "---\nschema: nerd/v1\nforbid:\n  - match: secret.txt\n    reason: keep out\n---\n\nold body\n")
	writeRel(t, root, ".codex/skills/x/AGENTS.md", "# harness mirror\n")
	writeRel(t, root, ".gitignore", "**/CLAUDE.md\n")

	written, err := WriteAgentDocTwins(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pkg/a/CLAUDE.md", "pkg/a/nerd.md", "pkg/b/CLAUDE.md", "pkg/b/nerd.md"}
	if strings.Join(written, ",") != strings.Join(want, ",") {
		t.Fatalf("written = %v, want %v", written, want)
	}
	if got := readRel(t, root, "pkg/a/CLAUDE.md"); got != "# A\r\n\r\nGuidance for a.\r\n" {
		t.Fatalf("CLAUDE.md is not a byte copy: %q", got)
	}
	nerdA := readRel(t, root, "pkg/a/nerd.md")
	if nerdA != "---\nschema: nerd/v1\n---\n\n# A\r\n\r\nGuidance for a.\r\n" {
		t.Fatalf("new nerd.md = %q", nerdA)
	}
	nerdB := readRel(t, root, "pkg/b/nerd.md")
	if !strings.HasPrefix(nerdB, "---\nschema: nerd/v1\nforbid:\n  - match: secret.txt\n    reason: keep out\n---\n\n# B\n") || strings.Contains(nerdB, "old body") {
		t.Fatalf("existing nerd.md lost its frontmatter or kept the old body: %q", nerdB)
	}
	doc, err := Parse([]byte(nerdB))
	if err != nil || len(doc.Spec.Forbid) != 1 || doc.Body != "# B" {
		t.Fatalf("regenerated nerd.md parse = %+v, %v", doc, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex/skills/x/CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("a harness mirror AGENTS.md got a twin")
	}
	if stale, err := StaleAgentDocTwins(root); err != nil || len(stale) != 0 {
		t.Fatalf("fresh twins reported stale: %v %v", stale, err)
	}
	if again, err := WriteAgentDocTwins(root); err != nil || len(again) != 0 {
		t.Fatalf("a second run rewrote %v (%v)", again, err)
	}

	writeRel(t, root, "pkg/a/agents.md", "# A\n\nChanged.\n")
	stale, err := StaleAgentDocTwins(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(stale, ",") != "pkg/a/CLAUDE.md,pkg/a/nerd.md" {
		t.Fatalf("stale = %v", stale)
	}
	if err := os.Remove(filepath.Join(root, "pkg/a/CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if stale, _ := StaleAgentDocTwins(root); strings.Join(stale, ",") != "pkg/a/nerd.md" {
		t.Fatalf("a missing local CLAUDE.md counted as stale: %v", stale)
	}
}

func TestAgentDocTwins_RefusesBrokenFrontmatter(t *testing.T) {
	root := gitWorkspace(t)
	writeRel(t, root, "pkg/agents.md", "# P\n")
	writeRel(t, root, "pkg/nerd.md", "---\nschema: nerd/v1\nnot_a_key: 1\n---\nbody\n")
	if _, err := WriteAgentDocTwins(root); err == nil || !strings.Contains(err.Error(), "pkg/nerd.md") {
		t.Fatalf("broken frontmatter accepted: %v", err)
	}
	if got := readRel(t, root, "pkg/nerd.md"); !strings.Contains(got, "not_a_key") {
		t.Fatal("the broken nerd.md was overwritten")
	}
}
