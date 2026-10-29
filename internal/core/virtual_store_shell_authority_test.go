package core

import (
	"context"
	"os/exec"
	"testing"

	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The tool-layer guard is the lowest chokepoint every shell call reaches, so
// it must reach the same decision as the session executor's shell-effect gate
// for a turn whose own request authorized a commit: the shell scope on the
// call's context and the turn's authorization in the kernel, decided by
// constitution.mg section 7E. A direct call with no turn scope stays refused.
func TestVirtualStore_ToolWriteGuardHonoursTheTurnsCommitAuthorization(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", ws}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	kernel := setupMockKernel(t)
	v := &VirtualStore{kernel: kernel, dreamer: NewDreamer(kernel)}
	registry := tools.NewRegistry()
	v.installToolWriteGuard(registry)

	runs := 0
	if err := registry.Register(&tools.Tool{
		Effect: tools.EffectExecute, Name: "run_command", Category: tools.CategoryCode,
		Schema:  tools.ToolSchema{Required: []string{"command"}},
		Execute: func(context.Context, map[string]any) (string, error) { runs++; return "ran", nil },
	}); err != nil {
		t.Fatal(err)
	}

	const turn = types.MangleAtom("/turn_guard_1")
	scoped := projectdoc.WithShellScope(context.Background(), projectdoc.ShellScope{
		Turn: turn, Workspace: ws, WriteSet: func() []string { return []string{"a.go"} },
	})
	call := func(ctx context.Context, command string) error {
		_, err := registry.Execute(ctx, "run_command", map[string]any{"command": command})
		return err
	}

	// Before the kernel is told the request authorized anything: refused.
	if err := call(scoped, "git add -- a.go"); err == nil {
		t.Fatal("a mutation was admitted by a turn holding no authorization")
	}
	if err := kernel.Assert(types.Fact{Predicate: "turn_authorization", Args: []any{turn, types.MangleAtom("/commit")}}); err != nil {
		t.Fatal(err)
	}
	if err := call(scoped, "git add -- a.go"); err != nil {
		t.Fatalf("the turn's own file was refused at the registry: %v", err)
	}
	if runs != 1 {
		t.Fatalf("handler ran %d times, want 1", runs)
	}
	for _, command := range []string{"git add -- b.go", "git add -A", "git push", "git push --force", "git reset --hard", "git add a.go && git push"} {
		if err := call(scoped, command); err == nil {
			t.Errorf("%q was admitted at the registry on a commit authorization", command)
		}
	}
	// No scope on the context: a direct registry call is never authorized.
	if err := call(context.Background(), "git add -- a.go"); err == nil {
		t.Error("a direct registry call with no turn scope was authorized")
	}
	// A restriction on the turn wins.
	if err := kernel.Assert(types.Fact{Predicate: "turn_restriction", Args: []any{turn, types.MangleAtom("/read_only")}}); err != nil {
		t.Fatal(err)
	}
	if err := call(scoped, "git add -- a.go"); err == nil {
		t.Error("a read-only turn's commit authorization was honoured")
	}
	if runs != 1 {
		t.Fatalf("a refused call reached the handler: runs=%d", runs)
	}
}
