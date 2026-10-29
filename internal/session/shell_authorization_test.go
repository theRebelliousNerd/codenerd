package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

// A request that names a step ("fix X, then commit it") authorizes that step
// for the turn's tasks, narrowly: staging and committing the files the turn
// wrote, and nothing beside it. These drive the registered executor path with
// the production constitution and a real kernel (the decision is
// policy/constitution.mg, section 7E), a real git repository for the staged
// set, and a counting tool: the handler count is the evidence of dispatch.

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func touch(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte("package x\n// "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

type shellAuthRig struct {
	*restrictionRig
	runs *atomic.Int64
}

func newShellAuthRig(t *testing.T) *shellAuthRig {
	t.Helper()
	base := newRestrictionRig(t)
	rig := &shellAuthRig{restrictionRig: base, runs: &atomic.Int64{}}
	for _, name := range []string{"run_command", "git_operation"} {
		registerTestTool(t, &tools.Tool{Name: name, Effect: tools.EffectExecute, Description: "counting " + name,
			Execute: func(context.Context, map[string]any) (string, error) { rig.runs.Add(1); return "ran", nil }})
	}
	initGitRepo(t, base.workspace)
	return rig
}

// turn builds the context a turn runs under the way ProcessWithIntent does:
// the request's authorization and restriction told to the kernel, the turn's
// identity, and the shell scope with the files the turn has written.
func (r *shellAuthRig) turn(t *testing.T, auth perception.Authorization, u perception.Understanding, written ...string) context.Context {
	t.Helper()
	restriction := perception.RestrictionFrom(&u)
	turn := newTurnAtom()
	result := &ExecutionResult{WrittenPaths: written}
	ctx := withCatalogTurn(perception.WithAuthorization(perception.WithRestriction(context.Background(), restriction), auth), turn)
	if err := r.exec.assertTurnRestriction(turn, restriction); err != nil {
		t.Fatal(err)
	}
	if err := r.exec.assertTurnAuthorization(turn, auth); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.exec.cleanupTurnFacts)
	return r.exec.withShellScope(ctx, turn, result)
}

func (r *shellAuthRig) run(ctx context.Context, id, command string) error {
	_, err := r.exec.executeToolCall(ctx, ToolCall{ID: id, Name: "run_command", Args: map[string]any{"command": command}},
		validCapabilityTestConfig("run_command", "git_operation"))
	return err
}

var commitAuth = perception.AuthorizationOf(perception.AuthorizeCommit)

func TestShellAuthorization_CommitRequestAdmitsStagingAndCommittingTheTurnsFiles(t *testing.T) {
	rig := newShellAuthRig(t)
	touch(t, rig.workspace, "a.go")
	ctx := rig.turn(t, commitAuth, perception.Understanding{}, "a.go")

	if err := rig.run(ctx, "add", "git add -- a.go"); err != nil {
		t.Fatalf("staging the turn's own file was refused: %v", err)
	}
	if got := rig.runs.Load(); got != 1 {
		t.Fatalf("handler ran %d times after the add, want 1", got)
	}
	// The probe reads the real index, so stage for real, as the tool would.
	gitOut(t, rig.workspace, "add", "a.go")
	if err := rig.run(ctx, "commit", `git commit -m "fix: the thing"`); err != nil {
		t.Fatalf("committing the turn's own staged file was refused: %v", err)
	}
	if got := rig.runs.Load(); got != 2 {
		t.Fatalf("handler ran %d times after the commit, want 2", got)
	}
	// The typed git tool is the same decision.
	if _, err := rig.exec.executeToolCall(ctx, ToolCall{ID: "t1", Name: "git_operation", Args: map[string]any{"operation": "commit", "message": "fix"}},
		validCapabilityTestConfig("git_operation")); err != nil {
		t.Fatalf("typed git commit was refused: %v", err)
	}
	rig.requireNoAuthorityLeft(t)
	for _, p := range []string{"shell_call", "shell_call_in_write_set"} {
		if rows, _ := rig.kernel.Query(p); len(rows) != 0 {
			t.Fatalf("%s outlived the call: %v", p, rows)
		}
	}
}

func TestShellAuthorization_NoCommitWordMeansTheGateStillRefuses(t *testing.T) {
	rig := newShellAuthRig(t)
	touch(t, rig.workspace, "a.go")
	ctx := rig.turn(t, perception.Authorization{}, perception.Understanding{}, "a.go")
	for _, command := range []string{"git add -- a.go", `git commit -m "x"`} {
		err := rig.run(ctx, "c", command)
		if err == nil || !strings.HasPrefix(err.Error(), "blocked by shell-effect gate:") {
			t.Fatalf("%q without an authorization = %v, want the shell-effect gate's refusal", command, err)
		}
		if strings.Contains(err.Error(), "authorized to commit") {
			t.Fatalf("a turn with no authorization was told it holds one: %v", err)
		}
	}
	if rig.runs.Load() != 0 {
		t.Fatal("a refused call reached its handler")
	}
}

func TestShellAuthorization_AuthorizationReachesOnlyItsOwnTurn(t *testing.T) {
	rig := newShellAuthRig(t)
	touch(t, rig.workspace, "a.go")
	rig.turn(t, commitAuth, perception.Understanding{}, "a.go")
	other := rig.turn(t, perception.Authorization{}, perception.Understanding{}, "a.go")
	if err := rig.run(other, "x", "git add -- a.go"); err == nil {
		t.Fatal("a sibling turn borrowed the authorization")
	}
	// No turn on the context at all: nothing to authorize.
	if err := rig.run(perception.WithAuthorization(context.Background(), commitAuth), "y", "git add -- a.go"); err == nil {
		t.Fatal("an authorization with no turn scope admitted a mutation")
	}
	if rig.runs.Load() != 0 {
		t.Fatal("a refused call reached its handler")
	}
}

// "Commit" is not "push", not "everything", and not another step.
func TestShellAuthorization_CommitNeverCoversWiderGitWork(t *testing.T) {
	rig := newShellAuthRig(t)
	touch(t, rig.workspace, "a.go")
	touch(t, rig.workspace, "b.go")
	ctx := rig.turn(t, commitAuth, perception.Understanding{}, "a.go")
	for _, command := range []string{
		"git push", "git push origin main", "git push --force", "git push -f origin main",
		"git reset --hard HEAD~1", "git checkout -b other", "git switch main", "git stash",
		"git add -A", "git add .", "git add --all", "git add -- a.go b.go", "git add b.go", "git add *.go",
		"git commit -a -m x", "git commit --amend -m x", "git commit --no-verify -m x", "git commit -m x b.go",
		"git add a.go && git commit -m x", "git add a.go; git push", "git -c core.hooksPath=/x commit -m x",
		"rm a.go", "git rm a.go", "git clean -fd",
	} {
		if err := rig.run(ctx, "w", command); err == nil {
			t.Errorf("%q was admitted on a commit request", command)
		}
	}
	if rig.runs.Load() != 0 {
		t.Fatalf("a refused call reached its handler %d time(s)", rig.runs.Load())
	}
	// The refusal names the unwritten file.
	err := rig.run(ctx, "w", "git add -- b.go")
	if err == nil || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("add of a file the turn did not write = %v, want b.go named", err)
	}
	// A commit with a file the turn did not write staged is refused too.
	gitOut(t, rig.workspace, "add", "b.go")
	if err := rig.run(ctx, "w", `git commit -m "x"`); err == nil || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("commit carrying an unwritten staged file = %v, want it refused naming b.go", err)
	}
	// And a typed push.
	if _, err := rig.exec.executeToolCall(ctx, ToolCall{ID: "p", Name: "git_operation", Args: map[string]any{"operation": "push"}},
		validCapabilityTestConfig("git_operation")); err == nil {
		t.Fatal("typed git push was admitted on a commit request")
	}
}

// The user's limits win over the user's authorization.
func TestShellAuthorization_AReadOnlyRestrictionAlwaysWins(t *testing.T) {
	for _, u := range []perception.Understanding{
		{UserConstraints: []string{"read_only"}},
		{UserConstraints: []string{"no_changes"}},
		{UserConstraints: []string{"dry_run"}},
		{Signals: perception.Signals{IsHypothetical: true}},
	} {
		rig := newShellAuthRig(t)
		touch(t, rig.workspace, "a.go")
		ctx := rig.turn(t, commitAuth, u, "a.go")
		for _, command := range []string{"git add -- a.go", `git commit -m "x"`} {
			if err := rig.run(ctx, "r", command); err == nil {
				t.Errorf("%+v: %q was admitted under a restriction", u, command)
			}
		}
		if rig.runs.Load() != 0 {
			t.Errorf("%+v: a refused call reached its handler", u)
		}
	}
}

// The wiring from the turn: ProcessWithIntent merges the authorization from
// the intent and from the context it runs under (a task spawned from a chat
// turn), tells the kernel, and puts the scope on the context. A model-built
// intent field is not what carries it: only the merge of the two sources is.
func TestShellAuthorization_ProcessWithIntentCarriesTheRequestsAuthorization(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs a throwaway module turn")
	}
	for _, tc := range []struct {
		name       string
		ctxAuth    perception.Authorization
		intentAuth perception.Authorization
		wantCommit bool
	}{
		{"from the spawning turn's context", commitAuth, perception.Authorization{}, true},
		{"from the intent", perception.Authorization{}, commitAuth, true},
		{"from neither", perception.Authorization{}, perception.Authorization{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRepairHarness(t, func(e *Executor) {
				e.config.EnableSafetyGate = true
				e.config.VerifyBuildAfterEdits = false
				e.config.VerifyTestsAfterEdits = false
				allowed := []string{"write_file", "read_file", "edit_file", "run_command"}
				e.configFactory = &MockConfigFactory{GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
					return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}, nil
				}}
				e.EffectiveAgentRuntimeConfig = &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}
			})
			if err := shell.RegisterAll(h.reg); err != nil {
				t.Fatal(err)
			}
			initGitRepo(t, h.ws)
			remote := t.TempDir()
			gitOut(t, remote, "init", "-q", "--bare")
			gitOut(t, h.ws, "remote", "add", "origin", remote)
			mustWrite(t, filepath.Join(h.ws, "other.txt"), "not the turn's\n")
			a := filepath.Join(h.ws, "a.go")
			h.initial = func() *types.LLMToolResponse {
				return &types.LLMToolResponse{Text: "writing and committing", ToolCalls: []types.ToolCall{
					h.writeCall("w1", "write_file", a, "package main\n\nfunc main() {}\n"),
					{ID: "g1", Name: "run_command", Input: map[string]any{"command": "git add -- a.go"}},
					{ID: "g2", Name: "run_command", Input: map[string]any{"command": "git commit -m \"add a\""}},
					{ID: "g3", Name: "run_command", Input: map[string]any{"command": "git push origin main"}},
				}}
			}
			preset := presetIntentForTask("/fix", "fix it, then commit it", "")
			preset.Authorization = tc.intentAuth
			ctx, cancel := context.WithTimeout(perception.WithAuthorization(context.Background(), tc.ctxAuth), 3*time.Minute)
			defer cancel()
			if _, err := h.executor.ProcessWithIntent(ctx, "fix it, then commit it", preset); err != nil {
				t.Logf("ProcessWithIntent: %v", err)
			}
			// A repository with no commit yet has no log: that is the empty answer.
			logOut, _ := exec.Command("git", "-C", h.ws, "log", "--name-only", "--format=%s").CombinedOutput()
			log := string(logOut)
			if refs := gitOut(t, remote, "for-each-ref"); strings.TrimSpace(refs) != "" {
				t.Fatalf("the push the request never named reached the remote:\n%s", refs)
			}
			if !tc.wantCommit {
				if strings.Contains(log, "add a") {
					t.Fatalf("a turn with no authorization committed:\n%s", log)
				}
				return
			}
			if !strings.Contains(log, "add a") || !strings.Contains(log, "a.go") {
				t.Fatalf("the request's commit did not land:\n%s", log)
			}
			if strings.Contains(log, "other.txt") {
				t.Fatalf("the commit took a file the turn did not write:\n%s", log)
			}
		})
	}
}
