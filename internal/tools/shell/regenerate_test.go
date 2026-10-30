package shell

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// regenWorkspace is a git work tree whose nerd.md declares generators that
// copy a source file onto their outputs.
func regenWorkspace(t *testing.T) (string, context.Context) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not installed")
	}
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("nerd.md", `---
schema: nerd/v1
generators:
  - id: corpus
    run: cp src.txt gen/out.txt
    outputs: [gen/out.txt]
  - id: sprawling
    run: cp src.txt other.txt
    outputs: [gen/out.txt]
  - id: deleting
    run: rm other.txt
    outputs: [gen/out.txt]
---
`)
	write("src.txt", "new content\n")
	write("gen/out.txt", "old content\n")
	write("other.txt", "untouched\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root, context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, root)
}

// The call's paths are the generator's declared outputs, so every write
// guard and the preimage capture see exactly what the run will rewrite.
func TestRegenerate_ExpandsTheDeclaredOutputsIntoPaths(t *testing.T) {
	_, ctx := regenWorkspace(t)
	reg := tools.NewRegistry()
	if err := reg.Register(RegenerateTool()); err != nil {
		t.Fatal(err)
	}
	args, err := reg.NormalizedArgs(ctx, "regenerate", map[string]any{"id": "corpus"})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := projectdoc.TargetPaths(args)
	if err != nil || len(targets) != 1 || targets[0] != "gen/out.txt" {
		t.Fatalf("write targets = %v (%v), want the declared output", targets, err)
	}
	if !projectdoc.IsWriteMutationTool("regenerate") {
		t.Fatal("regenerate is not classified as a write: its outputs would bypass the write guards")
	}
	if _, err := reg.NormalizedArgs(ctx, "regenerate", map[string]any{"id": "corpus", "paths": []any{"other.txt"}}); err == nil {
		t.Fatal("paths other than the declared outputs were accepted")
	}
	if _, err := reg.NormalizedArgs(ctx, "regenerate", map[string]any{"id": "missing"}); err == nil || !strings.Contains(err.Error(), "corpus") {
		t.Fatalf("an undeclared generator: err = %v, want the declared ids named", err)
	}
}

func TestRegenerate_RunsTheDeclaredGenerator(t *testing.T) {
	root, ctx := regenWorkspace(t)
	out, err := executeRegenerate(ctx, map[string]any{"id": "corpus"})
	if err != nil {
		t.Fatalf("regenerate: %v (%s)", err, out)
	}
	var res RegenerateResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Changed) != 1 || res.Changed[0] != "gen/out.txt" || len(res.Undeclared) != 0 || res.UndeclaredUnchecked != "" {
		t.Fatalf("report = %+v", res)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "gen", "out.txt")); string(got) != "new content\n" {
		t.Fatalf("output = %q", got)
	}
}

// A generator that writes a file it does not declare fails by name: its
// writes could not be recorded or undone.
func TestRegenerate_AnUndeclaredWriteFails(t *testing.T) {
	_, ctx := regenWorkspace(t)
	out, err := executeRegenerate(ctx, map[string]any{"id": "sprawling"})
	if err == nil || !strings.Contains(err.Error(), "other.txt") {
		t.Fatalf("err = %v (%s), want the undeclared write named", err, out)
	}
}

// A generator that deletes a clean tracked file it does not declare fails by
// name: the deletion is a write the turn could not record or undo.
func TestRegenerate_AnUndeclaredDeletionFails(t *testing.T) {
	if _, err := exec.LookPath("rm"); err != nil {
		t.Skip("rm not installed")
	}
	_, ctx := regenWorkspace(t)
	out, err := executeRegenerate(ctx, map[string]any{"id": "deleting"})
	if err == nil || !strings.Contains(err.Error(), "other.txt") {
		t.Fatalf("err = %v (%s), want the undeclared deletion named", err, out)
	}
}
