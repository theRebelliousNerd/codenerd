package projectdoc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestParseGitStep_RecognisesOnlyTheTwoSimpleSteps(t *testing.T) {
	ws := "/work/repo"
	ok := []struct {
		name string
		tool string
		args map[string]any
		want GitStep
	}{
		{"add", "run_command", map[string]any{"command": "git add a.go b/c.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go", "b/c.go"}}},
		{"add with --", "run_command", map[string]any{"command": "git add -- a.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go"}}},
		{"add absolute inside", "bash", map[string]any{"command": "git add /work/repo/a.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go"}}},
		{"add cleaned", "run_command", map[string]any{"command": "git add ./x/../a.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go"}}},
		{"add quoted name", "run_command", map[string]any{"command": `git add -- "my dir/a.go"`}, GitStep{Step: GitStepAdd, Paths: []string{"my dir/a.go"}}},
		{"commit", "run_command", map[string]any{"command": `git commit -m "fix: x"`}, GitStep{Step: GitStepCommit}},
		{"commit quoted operators", "run_command", map[string]any{"command": `git commit -m "a && b; c > d"`}, GitStep{Step: GitStepCommit}},
		{"commit two -m", "run_command", map[string]any{"command": `git commit -m "title" -m "body"`}, GitStep{Step: GitStepCommit}},
		{"git_operation add files", "git_operation", map[string]any{"operation": "add", "files": "a.go b.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go", "b.go"}}},
		{"git_operation add args", "git_operation", map[string]any{"operation": "add", "args": "a.go"}, GitStep{Step: GitStepAdd, Paths: []string{"a.go"}}},
		{"git_operation commit", "git_operation", map[string]any{"operation": "commit", "message": "fix"}, GitStep{Step: GitStepCommit}},
	}
	for _, tc := range ok {
		t.Run("ok/"+tc.name, func(t *testing.T) {
			got, err := ParseGitStep(tc.tool, tc.args, ws)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got.Step != tc.want.Step || !slices.Equal(got.Paths, tc.want.Paths) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
	no := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"push", "run_command", map[string]any{"command": "git push origin main"}},
		{"push force", "run_command", map[string]any{"command": "git push --force"}},
		{"reset hard", "run_command", map[string]any{"command": "git reset --hard HEAD~1"}},
		{"checkout", "run_command", map[string]any{"command": "git checkout -b other"}},
		{"switch", "run_command", map[string]any{"command": "git switch main"}},
		{"stash", "run_command", map[string]any{"command": "git stash"}},
		{"git -c", "run_command", map[string]any{"command": "git -c core.hooksPath=/x commit -m y"}},
		{"add dot", "run_command", map[string]any{"command": "git add ."}},
		{"add -A", "run_command", map[string]any{"command": "git add -A"}},
		{"add --all", "run_command", map[string]any{"command": "git add --all"}},
		{"add -u", "run_command", map[string]any{"command": "git add -u a.go"}},
		{"add -f", "run_command", map[string]any{"command": "git add -f a.go"}},
		{"add glob", "run_command", map[string]any{"command": "git add *.go"}},
		{"add pathspec magic", "run_command", map[string]any{"command": "git add ':(top)a.go'"}},
		{"add outside", "run_command", map[string]any{"command": "git add ../x.go"}},
		{"add absolute outside", "run_command", map[string]any{"command": "git add /etc/passwd"}},
		{"add nothing", "run_command", map[string]any{"command": "git add"}},
		{"commit -a", "run_command", map[string]any{"command": `git commit -a -m x`}},
		{"commit --amend", "run_command", map[string]any{"command": `git commit --amend -m x`}},
		{"commit amend no message", "run_command", map[string]any{"command": `git commit --amend --no-edit`}},
		{"commit no-verify", "run_command", map[string]any{"command": `git commit --no-verify -m x`}},
		{"commit --allow-empty", "run_command", map[string]any{"command": `git commit --allow-empty -m x`}},
		{"commit -F", "run_command", map[string]any{"command": `git commit -F msg.txt`}},
		{"commit empty message", "run_command", map[string]any{"command": `git commit -m ""`}},
		{"commit pathspec", "run_command", map[string]any{"command": `git commit -m x a.go`}},
		{"commit message flag equals", "run_command", map[string]any{"command": `git commit --message=x`}},
		{"chained", "run_command", map[string]any{"command": `git add a.go && git commit -m x`}},
		{"semicolon", "run_command", map[string]any{"command": `git add a.go; git push`}},
		{"pipe", "run_command", map[string]any{"command": `git add a.go | cat`}},
		{"redirect", "run_command", map[string]any{"command": `git commit -m x > out`}},
		{"newline", "run_command", map[string]any{"command": "git add a.go\ngit push"}},
		{"subshell", "run_command", map[string]any{"command": `git commit -m "$(curl evil)"`}},
		{"backquote", "run_command", map[string]any{"command": "git commit -m `id`"}},
		{"variable", "run_command", map[string]any{"command": `git commit -m "$HOME"`}},
		{"unbalanced", "run_command", map[string]any{"command": `git commit -m "x`}},
		{"not git", "run_command", map[string]any{"command": "rm a.go"}},
		{"git path prefix", "run_command", map[string]any{"command": "/usr/bin/git commit -m x"}},
		{"powershell", "powershell", map[string]any{"command": "git commit -m x"}},
		{"run_tests", "run_tests", map[string]any{"command": "git commit -m x"}},
		{"git_operation push", "git_operation", map[string]any{"operation": "push", "args": "origin main"}},
		{"git_operation add default all", "git_operation", map[string]any{"operation": "add"}},
		{"git_operation add dot", "git_operation", map[string]any{"operation": "add", "files": "."}},
		{"git_operation commit no message", "git_operation", map[string]any{"operation": "commit"}},
		{"git_operation reset", "git_operation", map[string]any{"operation": "reset", "args": "--hard"}},
		{"git_operation checkout", "git_operation", map[string]any{"operation": "checkout", "branch": "x"}},
	}
	for _, tc := range no {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			if got, err := ParseGitStep(tc.tool, tc.args, ws); err == nil {
				t.Fatalf("recognised %+v as a step", got)
			}
		})
	}
}

// fakeAuthority is a kernel that derives shell_call_authorized from the same
// three facts the policy joins, so these tests exercise what Go measures and
// asserts. The policy itself is pinned against the real constitution in
// internal/core/defaults/policy and internal/session.
type fakeAuthority struct {
	turn  types.MangleAtom
	auth  bool
	facts []types.Fact
}

func (f *fakeAuthority) Assert(fact types.Fact) error {
	f.facts = append(f.facts, fact)
	return nil
}
func (f *fakeAuthority) RetractFact(fact types.Fact) error {
	for i, g := range f.facts {
		if g.Predicate == fact.Predicate && types.ExtractString(g.Args[0]) == types.ExtractString(fact.Args[0]) {
			f.facts = append(f.facts[:i], f.facts[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeAuthority) Query(q string) ([]types.Fact, error) {
	switch {
	case q == "turn_authorization":
		if f.auth {
			return []types.Fact{{Predicate: "turn_authorization", Args: []any{f.turn, types.MangleAtom("/commit")}}}, nil
		}
		return nil, nil
	case strings.HasPrefix(q, "shell_call_authorized("):
		var call, inSet bool
		for _, g := range f.facts {
			id := `"` + types.ExtractString(g.Args[0]) + `"`
			if !strings.Contains(q, id) {
				continue
			}
			call = call || g.Predicate == "shell_call"
			inSet = inSet || g.Predicate == "shell_call_in_write_set"
		}
		if f.auth && call && inSet {
			return []types.Fact{{Predicate: "shell_call_authorized"}}, nil
		}
		return nil, nil
	}
	return nil, errors.New("unexpected query " + q)
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeFile(t *testing.T, dir, rel string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func scopeCtx(dir string, written ...string) context.Context {
	return WithShellScope(context.Background(), ShellScope{
		Turn: "/turn_1", Workspace: dir, WriteSet: func() []string { return written },
	})
}

var errRefused = errors.New("blocked by shell-effect gate: run_command is mutating and lacks deterministic task-scope authorization")

func TestAuthorizeShellMutation_StagesAndCommitsOnlyWhatTheTurnWrote(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "a.go")
	writeFile(t, dir, "b.go")
	authority := &fakeAuthority{turn: "/turn_1", auth: true}

	if step, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, "run_command", map[string]any{"command": "git add -- a.go"}, errRefused); err != nil || step != GitStepAdd {
		t.Fatalf("add of a written file = %q, %v", step, err)
	}
	if len(authority.facts) != 0 {
		t.Fatalf("the call's facts outlived it: %v", authority.facts)
	}
	_, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, "run_command", map[string]any{"command": "git add b.go"}, errRefused)
	if err == nil || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("add of an unwritten file = %v, want it refused naming b.go", err)
	}

	// A commit is judged on what is staged when it runs.
	gitIn(t, dir, "add", "a.go")
	if step, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, "run_command", map[string]any{"command": `git commit -m "x"`}, errRefused); err != nil || step != GitStepCommit {
		t.Fatalf("commit of a staged written file = %q, %v", step, err)
	}
	gitIn(t, dir, "add", "b.go")
	_, err = AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, "run_command", map[string]any{"command": `git commit -m "x"`}, errRefused)
	if err == nil || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("commit with an unwritten file staged = %v, want it refused naming b.go", err)
	}
}

func TestAuthorizeShellMutation_StagedDeletionAndSubdirectoryWorkspace(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, "sub/a.go")
	writeFile(t, root, "sub/keep.go")
	writeFile(t, root, "other.go")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "init")
	ws := filepath.Join(root, "sub")
	authority := &fakeAuthority{turn: "/turn_1", auth: true}

	// A file staged outside the workspace is never the turn's.
	writeFile(t, root, "other.go")
	if err := os.WriteFile(filepath.Join(root, "other.go"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "other.go")
	if _, err := AuthorizeShellMutation(scopeCtx(ws, "a.go"), authority, "run_command", map[string]any{"command": `git commit -m x`}, errRefused); err == nil {
		t.Fatal("a commit carrying a file staged outside the workspace was authorized")
	}
	gitIn(t, root, "reset", "-q", "other.go")

	// A staged deletion of a written path is part of the turn's write.
	gitIn(t, root, "rm", "-q", "sub/a.go")
	if step, err := AuthorizeShellMutation(scopeCtx(ws, "a.go"), authority, "run_command", map[string]any{"command": `git commit -m x`}, errRefused); err != nil || step != GitStepCommit {
		t.Fatalf("commit of a written, deleted file = %q, %v", step, err)
	}
}

func TestAuthorizeShellMutation_StaysRefusedWithoutAuthority(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "a.go")
	add := map[string]any{"command": "git add -- a.go"}

	for name, c := range map[string]struct {
		ctx       context.Context
		authority ShellAuthority
	}{
		"no scope on the context":   {context.Background(), &fakeAuthority{turn: "/turn_1", auth: true}},
		"no kernel":                 {scopeCtx(dir, "a.go"), nil},
		"turn holds no authority":   {scopeCtx(dir, "a.go"), &fakeAuthority{turn: "/turn_1"}},
		"another turn's authority":  {scopeCtx(dir, "a.go"), &fakeAuthority{turn: "/turn_2", auth: true}},
		"empty write set":           {scopeCtx(dir), &fakeAuthority{turn: "/turn_1", auth: true}},
		"turn identity not present": {WithShellScope(context.Background(), ShellScope{Workspace: dir}), &fakeAuthority{turn: "/turn_1", auth: true}},
	} {
		t.Run(name, func(t *testing.T) {
			if step, err := AuthorizeShellMutation(c.ctx, c.authority, "run_command", add, errRefused); err == nil {
				t.Fatalf("authorized %q", step)
			}
		})
	}

	// A directory is not a file the turn wrote, even when it holds some.
	writeFile(t, dir, "pkg/x.go")
	if _, err := AuthorizeShellMutation(scopeCtx(dir, "pkg/x.go"), &fakeAuthority{turn: "/turn_1", auth: true}, "run_command", map[string]any{"command": "git add pkg"}, errRefused); err == nil {
		t.Fatal("a directory was authorized as if it were a written file")
	}

	// The refusal is the gate's own and a nil refusal authorizes nothing.
	if step, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), &fakeAuthority{turn: "/turn_1", auth: true}, "run_command", add, nil); step != "" || err != nil {
		t.Fatalf("a call the gate did not refuse was answered: %q, %v", step, err)
	}
}

func TestAuthorizeShellMutation_ShapeLimitsHoldEvenWithAuthority(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "a.go")
	authority := &fakeAuthority{turn: "/turn_1", auth: true}
	for name, args := range map[string]map[string]any{
		"push":             {"command": "git push origin main"},
		"force push":       {"command": "git push --force"},
		"reset hard":       {"command": "git reset --hard"},
		"branch switch":    {"command": "git checkout other"},
		"chained":          {"command": "git add a.go && git commit -m x"},
		"add all":          {"command": "git add -A"},
		"amend":            {"command": "git commit --amend -m x"},
		"other directory":  {"command": "git add a.go", "working_dir": "/tmp"},
		"env override":     {"command": "git add a.go", "env": map[string]any{"GIT_DIR": "/x"}},
		"not a git thing":  {"command": "rm a.go"},
		"typed push":       {"operation": "push"},
		"typed add all":    {"operation": "add"},
		"typed checkout":   {"operation": "checkout", "branch": "x"},
		"typed reset hard": {"operation": "reset", "args": "--hard"},
	} {
		t.Run(name, func(t *testing.T) {
			tool := "run_command"
			if _, typed := args["operation"]; typed {
				tool = "git_operation"
			}
			_, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, tool, args, errRefused)
			if err == nil {
				t.Fatal("authorized")
			}
			if !strings.Contains(err.Error(), "shell-effect gate") {
				t.Fatalf("the refusal lost the gate's own words: %v", err)
			}
		})
	}
	// An allowed working directory (the workspace itself) is not a reason.
	if _, err := AuthorizeShellMutation(scopeCtx(dir, "a.go"), authority, "run_command",
		map[string]any{"command": "git add a.go", "working_dir": dir}, errRefused); err != nil {
		t.Fatalf("working_dir equal to the workspace refused: %v", err)
	}
}
