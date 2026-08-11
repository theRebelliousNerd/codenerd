package shell

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestWaitDelay_AllPaths(t *testing.T) {
	origExec := execCommandContext
	origLookPath := execLookPath
	defer func() {
		execCommandContext = origExec
		execLookPath = origLookPath
	}()

	var cmds []*exec.Cmd
	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmds = append(cmds, cmd)
		return cmd
	}
	// Pin lookups so the simple path does not divert into the Go builtin
	// fallback (executeRunCommand serves builtins when the binary is absent
	// and returns before execCommandContext is ever called). The same pin
	// ensures the compound and bash branches find a shell and exercise
	// their exec path rather than returning "interpreter not found".
	execLookPath = func(file string) (string, error) { return file, nil }

	if err := os.Setenv("GO_WANT_HELPER_PROCESS", "1"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")
	if err := os.Setenv("MOCK_OUTPUT", "ok"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	defer os.Unsetenv("MOCK_OUTPUT")

	assertWaitDelay := func(path string) {
		if len(cmds) == 0 {
			t.Fatalf("%s: no commands recorded; WaitDelay not exercised", path)
		}
		for i, cmd := range cmds {
			if cmd.WaitDelay == 0 {
				t.Errorf("%s cmd %d (%v %v): WaitDelay is 0, want %v", path, i, cmd.Path, cmd.Args, commandWaitDelay)
			}
			if cmd.WaitDelay != commandWaitDelay {
				t.Errorf("%s cmd %d (%v %v): WaitDelay=%v, want %v", path, i, cmd.Path, cmd.Args, cmd.WaitDelay, commandWaitDelay)
			}
		}
	}

	// 1. Compound path: isCompoundCommand true -> sh -c / pwsh -Command branch
	//    This exercises the first cmd.Run site (compound routing).
	cmds = nil
	if _, err := executeRunCommand(context.Background(), map[string]any{"command": "echo a && echo b"}); err != nil {
		t.Fatalf("compound executeRunCommand: %v", err)
	}
	assertWaitDelay("compound")

	// 2. Simple path: isCompoundCommand false -> direct exec branch
	//    This exercises the second cmd.Run site.
	cmds = nil
	if _, err := executeRunCommand(context.Background(), map[string]any{"command": "echo hello"}); err != nil {
		t.Fatalf("simple executeRunCommand: %v", err)
	}
	assertWaitDelay("simple")

	// 3. Bash path: executeBash -> bash branch
	//    This exercises the third cmd.Run site.
	cmds = nil
	if _, err := executeBash(context.Background(), map[string]any{"script": "echo hello"}); err != nil {
		t.Fatalf("executeBash: %v", err)
	}
	assertWaitDelay("bash")
}
