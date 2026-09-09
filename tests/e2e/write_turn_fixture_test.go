//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The shared write-turn fixture.
//
// 8e9507d added checkHollowSuccess: a write-oriented intent that finishes with
// no successful write-mutation tool call is refused. The guard is right, and it
// left every orchestrator fixture in this package failing, because those
// fixtures drive "/fix" against mocks that call nothing.
//
// The tempting fix — swap the verb for a read-only one — is wrong here. In the
// orchestrator files the verb IS the mechanism under test: "/fix" routes inline
// and "/research" forces subagent isolation. Swapping it would change which
// execution path each test exercises while turning the bar green, which is the
// exact class of failure this audit exists to find.
//
// So the fixture completes a real write turn instead. Three things were
// missing, and each was a genuine gap rather than a workaround:
//
//  1. The VirtualStore never received the kernel. Every one of these fixtures
//     built a real kernel and a real VirtualStore beside each other and never
//     connected them. VirtualStore.getDreamer derives the Dreamer lazily from
//     v.kernel, so with a nil kernel there is no Dreamer, and
//     PreflightDestructiveToolCall is fail-closed on exactly that: "permission
//     and speculative safety are independent gates; an allow decision from
//     checkSafety must never compensate for a missing simulation engine."
//     Every destructive call was blocked before reaching a tool.
//  2. No write-mutation tool. The name write_file matters twice: projectdoc.
//     IsWriteMutationTool recognises it, and BuiltinEffect resolves its
//     EffectWrite without a Tool.Effect field.
//  3. Nothing in AllowedTools. The config factories returned an empty
//     EffectiveAgentRuntimeConfig, so the JIT allowlist authorised nothing.
//
// The stub must really create the file. The post-action validator checks that
// the side effect landed, so a stub returning "wrote" without writing is
// correctly judged a hollow success — which is the guard doing its job.
//
// Measured: 29 package failures before, 21 after, with eight tests fixed and
// none broken.

// registerWriteTurnTool installs the write_file stub in the process-global
// registry, once. Registration is global and shared by every test in this
// package, so it is deliberately idempotent and content-free beyond writing
// what it is asked to write.
func registerWriteTurnTool(t *testing.T) {
	t.Helper()
	if tools.Global().Get("write_file") != nil {
		return
	}
	if err := tools.Global().Register(&tools.Tool{
		Name: "write_file",
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return "", fmt.Errorf("write_file: no path argument")
			}
			// Write atomically.
			//
			// Several tests in this package drive ten or fifty goroutines
			// through one executor, and they share a single fixture path. A
			// plain os.WriteFile truncates before it writes, so a post-action
			// validator running against a concurrent writer can observe an
			// empty file, judge the side effect absent, and fail the tool call
			// — reported as "attempted=1" with nothing successful. Write to a
			// unique temp file and rename: rename is atomic, so every observer
			// sees either the previous complete file or the new one.
			tmp, err := os.CreateTemp(filepath.Dir(path), ".write_turn_*")
			if err != nil {
				return "", err
			}
			tmpName := tmp.Name()
			if _, err := tmp.WriteString(content); err != nil {
				tmp.Close()
				os.Remove(tmpName)
				return "", err
			}
			if err := tmp.Close(); err != nil {
				os.Remove(tmpName)
				return "", err
			}
			if err := os.Rename(tmpName, path); err != nil {
				os.Remove(tmpName)
				return "", err
			}
			return "wrote " + path, nil
		},
	}); err != nil {
		t.Fatalf("register write_file: %v", err)
	}
}

// writeTurnSeq makes every fixture write target a distinct file.
var writeTurnSeq atomic.Int64

// writeTurnCall returns a tool call that will satisfy checkHollowSuccess for a
// write-oriented intent, targeting a path unique to this test.
func writeTurnCall(t *testing.T) types.ToolCall {
	t.Helper()
	registerWriteTurnTool(t)
	return writeTurnCallIn(t.TempDir())
}

// writeTurnCallIn is writeTurnCall for a directory the caller already owns, and
// it returns a DIFFERENT path on every call.
//
// Several tests here drive ten or fifty goroutines through one executor. When
// they all wrote one shared path, the turn intermittently failed with
// "attempted=1" and nothing successful — not from the file write, which is
// atomic, but from the pending_edit lifecycle around it. executeToolCall
// asserts pending_edit(FilePath, Content) before a write-mutation tool and
// retracts it on every exit path. Ten goroutines writing identical path and
// content assert an identical fact, the kernel dedupes it to one, and the first
// goroutine to finish retracts it out from under the nine still running.
//
// A distinct path per call gives each concurrent turn its own fact, which is
// what the lifecycle assumes. It also keeps the fixture honest: a real turn
// does not write the same file from ten goroutines.
func writeTurnCallIn(dir string) types.ToolCall {
	n := writeTurnSeq.Add(1)
	return types.ToolCall{
		ID:   fmt.Sprintf("fixture_write_%d", n),
		Name: "write_file",
		Input: map[string]any{
			"path":    filepath.Join(dir, fmt.Sprintf("fixture_write_%d.txt", n)),
			"content": fmt.Sprintf("write-turn fixture %d", n),
		},
	}
}

// writeTurnAllowedTools is the JIT envelope a write turn needs.
func writeTurnAllowedTools() []string { return []string{"write_file"} }

// writeTurnExecutorConfig disables the post-edit verification passes. These
// fixtures write into a temp directory with no Go module in it, so a build or
// test verification would fail for reasons unrelated to anything under test.
func writeTurnExecutorConfig() session.ExecutorConfig {
	cfg := session.DefaultExecutorConfig()
	cfg.VerifyBuildAfterEdits = false
	cfg.VerifyTestsAfterEdits = false
	cfg.CriticReviewAfterEdits = false
	return cfg
}

// wireDreamer connects the kernel the fixture already built to the VirtualStore
// it already built, so the executive gate has a Dreamer to consult.
func wireDreamer(vs *core.VirtualStore, k *core.RealKernel) {
	if vs == nil || k == nil {
		return
	}
	vs.SetKernel(k)
}
