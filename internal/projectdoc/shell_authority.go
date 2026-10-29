package projectdoc

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"codenerd/internal/types"
)

// A shell mutation is refused by ValidateShellToolInvocation because command
// text can never authorize itself. The user's request can: "fix X, then commit
// it" names the commit. This file is the one place that turns that into an
// admitted call, and it decides nothing itself: Go measures the call (is it
// exactly one recognised git step, are its files ones the turn wrote) and the
// kernel derives whether the turn's authorization covers it
// (policy/constitution.mg, section 7E: shell_call_authorized/1). Both the
// session executor's shell-effect gate and the tool-layer write guard call it,
// so the two gates cannot disagree.

// ShellScope is what a gate knows about the turn a shell call belongs to. The
// session executor puts it on the turn's context; a call with no scope (a
// direct registry call outside any turn) is never authorized.
type ShellScope struct {
	// Turn is the turn atom the kernel keys turn_authorization to.
	Turn types.MangleAtom
	// Workspace is the root the turn's relative paths are relative to.
	Workspace string
	// WriteSet reports, live, the workspace-relative files the turn has
	// written so far.
	WriteSet func() []string
}

type shellScopeKey struct{}

// WithShellScope returns ctx carrying the turn's scope.
func WithShellScope(ctx context.Context, scope ShellScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, shellScopeKey{}, scope)
}

// ShellScopeFrom is the scope the context's turn carries.
func ShellScopeFrom(ctx context.Context) (ShellScope, bool) {
	if ctx == nil {
		return ShellScope{}, false
	}
	scope, ok := ctx.Value(shellScopeKey{}).(ShellScope)
	return scope, ok && scope.Turn != ""
}

// ShellAuthority is the slice of a kernel the decision needs.
type ShellAuthority interface {
	Query(predicate string) ([]types.Fact, error)
	Assert(fact types.Fact) error
	RetractFact(fact types.Fact) error
}

var shellCallSeq atomic.Uint64

// stagedProbeTimeout bounds the git probe that reads what a commit would
// contain.
const stagedProbeTimeout = 15 * time.Second

// AuthorizeShellMutation answers a refusal: when the turn's own request
// authorizes the call, it returns the authorized step and a nil error;
// otherwise it returns refusal (with what the request did authorize, if
// anything, so the model does not guess at the shape). It never widens a
// refusal that is not a mutation: the caller passes only the error
// ValidateShellToolInvocation returned.
func AuthorizeShellMutation(ctx context.Context, authority ShellAuthority, toolName string, args map[string]any, refusal error) (string, error) {
	if refusal == nil {
		return "", nil
	}
	scope, ok := ShellScopeFrom(ctx)
	if !ok || authority == nil {
		return "", refusal
	}
	held := turnHolds(authority, scope.Turn)
	if !held {
		return "", refusal
	}
	if wd := stringArg(args, "working_dir"); wd != "" && !sameDirectory(wd, scope.Workspace) {
		return "", fmt.Errorf("%w; the turn is authorized to commit, but only from the workspace root", refusal)
	}
	if _, hasEnv := args["env"]; hasEnv {
		return "", fmt.Errorf("%w; the turn is authorized to commit, but not with an environment override", refusal)
	}
	step, err := ParseGitStep(toolName, args, scope.Workspace)
	if err != nil {
		return "", fmt.Errorf("%w; the turn is authorized to commit its own work, only as separate simple commands `git add -- <files you wrote>` and `git commit -m <message>` (%v)", refusal, err)
	}
	paths, err := stepPaths(ctx, step, scope.Workspace)
	if err != nil {
		return "", fmt.Errorf("%w; the turn is authorized to commit, but what it would touch could not be read (%v)", refusal, err)
	}
	written := make(map[string]bool)
	if scope.WriteSet != nil {
		for _, p := range scope.WriteSet() {
			if rel, ok := workspaceRelative(p, scope.Workspace); ok {
				written[rel] = true
			}
		}
	}
	var outside []string
	for _, p := range paths {
		if !written[p] {
			outside = append(outside, p)
		}
	}
	id := "shell-" + strconv.FormatUint(shellCallSeq.Add(1), 10) + "-" + strings.TrimPrefix(string(scope.Turn), "/")
	facts := []types.Fact{{Predicate: "shell_call", Args: []any{id, scope.Turn, types.MangleAtom("/" + step.Step)}}}
	if len(outside) == 0 {
		facts = append(facts, types.Fact{Predicate: "shell_call_in_write_set", Args: []any{id}})
	}
	var asserted []types.Fact
	defer func() {
		for _, f := range asserted {
			_ = authority.RetractFact(f)
		}
	}()
	for _, f := range facts {
		if err := authority.Assert(f); err != nil {
			return "", fmt.Errorf("%w; the authorization could not be checked (%v)", refusal, err)
		}
		asserted = append(asserted, f)
	}
	rows, err := authority.Query(fmt.Sprintf("shell_call_authorized(%s)", strconv.Quote(id)))
	if err != nil {
		return "", fmt.Errorf("%w; the authorization could not be checked (%v)", refusal, err)
	}
	if len(rows) > 0 {
		return step.Step, nil
	}
	if len(outside) > 0 {
		return "", fmt.Errorf("%w; the turn is authorized to commit only files it wrote, and these are not among them: %s", refusal, strings.Join(outside, ", "))
	}
	return "", refusal
}

// turnHolds reports whether the kernel holds any authorization for the turn.
// It only shapes the message and skips the measurement for turns that were
// given none; the decision is shell_call_authorized.
func turnHolds(authority ShellAuthority, turn types.MangleAtom) bool {
	rows, err := authority.Query("turn_authorization")
	if err != nil {
		return false
	}
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == string(turn) {
			return true
		}
	}
	return false
}

// stepPaths are the workspace-relative files the step would put in or take
// from a commit: what an add names, or what is staged for a commit.
func stepPaths(ctx context.Context, step GitStep, workspace string) ([]string, error) {
	if step.Step == GitStepAdd {
		return step.Paths, nil
	}
	return stagedPaths(ctx, workspace)
}

// stagedPaths reads the index: the files `git commit` would commit. A rename
// shows as its removal and its addition. A staged path outside the workspace
// is reported with a leading "../" and so is never a file the turn wrote.
func stagedPaths(ctx context.Context, workspace string) ([]string, error) {
	if workspace == "" {
		return nil, fmt.Errorf("no workspace")
	}
	ctx, cancel := context.WithTimeout(ctx, stagedProbeTimeout)
	defer cancel()
	prefixOut, err := exec.CommandContext(ctx, "git", "-C", workspace, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	prefix := strings.TrimSpace(string(prefixOut))
	out, err := exec.CommandContext(ctx, "git", "-C", workspace, "diff", "--cached", "--name-only", "--no-renames", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}
	var paths []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(name, prefix) {
				name = "../" + name
			} else {
				name = strings.TrimPrefix(name, prefix)
			}
		}
		paths = append(paths, name)
	}
	return paths, nil
}

func sameDirectory(dir, workspace string) bool {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "." {
		return true
	}
	return workspace != "" && filepath.IsAbs(dir) && dir == filepath.Clean(workspace)
}
