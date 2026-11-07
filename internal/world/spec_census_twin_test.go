package world

import (
	"context"
	"os/exec"
	"testing"
)

// A nerd.md beside an agents.md is that agents.md's generated twin
// (projectdoc.AgentDocTwins), so the spec census measures the agents.md only.
// A nerd.md with no agents.md beside it is a document of its own and stays.
func TestSpecCensus_SkipsAgentDocTwins(t *testing.T) {
	root := canonicalTempRoot(t)
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	writeWorkspaceFile(t, root, "pkg/agents.md", "# Pkg\n\nSee `pkg/a.go`.\n")
	writeWorkspaceFile(t, root, "pkg/nerd.md", "---\nschema: nerd/v1\n---\n\n# Pkg\n\nSee `pkg/a.go`.\n")
	writeWorkspaceFile(t, root, "other/nerd.md", "---\nschema: nerd/v1\n---\n\n# Other\n")
	writeWorkspaceFile(t, root, "pkg/a.go", "package pkg\n")

	c, err := NewScanner().specCensus(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.docs["pkg/nerd.md"]; ok {
		t.Fatal("the agents.md twin pkg/nerd.md was measured as a spec source")
	}
	for _, want := range []string{"pkg/agents.md", "other/nerd.md"} {
		if _, ok := c.docs[want]; !ok {
			t.Fatalf("census lost %s: %v", want, c.docs)
		}
	}
	if _, ok := c.abs["pkg/nerd.md"]; ok {
		t.Fatal("the twin kept an abs entry")
	}
}
