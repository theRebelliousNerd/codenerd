package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// WriteGuard is asked before a write-mutation tool touches the workspace,
// with the call's target paths, absolute. A refusal is the tool call's error:
// the model reads why, and nothing is written or snapshotted.
//
// A campaign puts one on each turn it runs (external audit F4): leases cover
// the write set a task declared, and a turn can write outside it, so two tasks
// running side by side could interleave writes to one undeclared file. The
// guard takes that file's lease for the task at the moment of the write.
type WriteGuard func(ctx context.Context, paths []string) error

type writeGuardKey struct{}

// WithWriteGuard returns ctx carrying guard for every tool call made under it.
func WithWriteGuard(ctx context.Context, guard WriteGuard) context.Context {
	return context.WithValue(ctx, writeGuardKey{}, guard)
}

// parentWriteGuard returns the guard already on the context, if there is
// one, so a narrower guard can chain it instead of replacing it. A repair
// episode runs under the campaign turn's guard in campaigns (leases); its
// own confinement wraps that one, and the outer refusal wins.
func parentWriteGuard(ctx context.Context) WriteGuard {
	guard, _ := ctx.Value(writeGuardKey{}).(WriteGuard)
	return guard
}

type turnWriteSetKey struct{}

// WithTurnWriteSet freezes the files the turn had written when the first
// post-edit gate was about to run. verifyCompletedToolTurn calls it once,
// before the rounds map, so a later round that appends to WrittenPaths does
// not move the set a repair prompt or a refusal names.
func WithTurnWriteSet(ctx context.Context, paths []string) context.Context {
	// A non-nil empty slice stays distinguishable from "no snapshot": a nil
	// value stored in a context comes back as a missing value.
	frozen := append([]string{}, paths...)
	return context.WithValue(ctx, turnWriteSetKey{}, frozen)
}

// namedWriteSet is the pre-gate write set when one was snapshotted, and the
// files written so far otherwise. Callers that invoke a round directly
// (tests, a gate with no turn around it) have no snapshot; naming
// WrittenPaths there is the set the round can see.
func namedWriteSet(ctx context.Context, result *ExecutionResult) []string {
	if ctx != nil {
		if frozen, ok := ctx.Value(turnWriteSetKey{}).([]string); ok {
			return append([]string{}, frozen...)
		}
	}
	if result == nil {
		return nil
	}
	return append([]string{}, result.WrittenPaths...)
}

// turnOwnedPaths is the pre-gate set plus every path the turn has written
// since, including a file an earlier repair attempt created. A compile
// failure in that file is still this turn's. Membership for the foreign
// skip uses this; the prompt and the refusal name namedWriteSet, the set
// the turn had when the gates started.
func turnOwnedPaths(ctx context.Context, result *ExecutionResult) []string {
	owned := namedWriteSet(ctx, result)
	if result == nil {
		return owned
	}
	seen := make(map[string]bool, len(owned)+len(result.WrittenPaths))
	for _, p := range owned {
		seen[p] = true
	}
	for _, p := range result.WrittenPaths {
		if seen[p] {
			continue
		}
		seen[p] = true
		owned = append(owned, p)
	}
	return owned
}

// RepairWriteGuard confines a repair episode to the turn's write set: the
// files the turn wrote before the first gate ran. Dogfood run 5 (2026-09-29):
// a repair loop added //go:build ignore to another agent's files to make a
// workspace build pass. A write to an existing file outside the set is
// refused with the set named, so the model reads where it may edit; files
// the episode itself creates are allowed, because the coverage and pinning
// rounds write their tests inside the episode (change_gates.go,
// pin_gate.go) and a static set would refuse the round's own work. Creating
// cannot destroy another agent's bytes -- the file did not exist -- while an
// overwrite outside the set is exactly run 5. parent, when non-nil, is asked
// first and its refusal wins.
func RepairWriteGuard(workspace string, writeSet []string, parent WriteGuard) WriteGuard {
	return repairWriteGuard(workspace, writeSet, nil, parent)
}

// repairWriteGuard is RepairWriteGuard plus the files the turn has written
// since the set was frozen. live is read on each call: the tool batch is
// sequential, so a file created earlier in the episode is in
// result.WrittenPaths before the next edit, and a guard built once from the
// frozen set would refuse that edit (the file exists, and it was not in the
// pre-gate set). The refusal still names the frozen set.
func repairWriteGuard(workspace string, writeSet []string, live func() []string, parent WriteGuard) WriteGuard {
	allowed, set := indexWriteSet(workspace, writeSet)
	return func(ctx context.Context, paths []string) error {
		if parent != nil {
			if err := parent(ctx, paths); err != nil {
				return err
			}
		}
		current := allowed
		if live != nil {
			extra, _ := indexWriteSet(workspace, live())
			if len(extra) > 0 {
				merged := make(map[string]bool, len(allowed)+len(extra))
				for k, v := range allowed {
					merged[k] = v
				}
				for k, v := range extra {
					merged[k] = v
				}
				current = merged
			}
		}
		for _, path := range paths {
			if current[filepath.Clean(path)] {
				continue
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("repair may only edit the turn's files (%s); %s is outside that set -- failures there are not this turn's to fix",
				set, filepath.ToSlash(path))
		}
		return nil
	}
}

// withRepairWriteGuard installs one guard for this repair attempt. The
// frozen name is the pre-gate set (namedWriteSet); files the turn has
// written since stay writable. Installed per attempt, in repairRound, so
// the parent it chains is the campaign guard already on the context and not
// a previous attempt's own guard.
func withRepairWriteGuard(ctx context.Context, workspace string, result *ExecutionResult) context.Context {
	if result == nil {
		return ctx
	}
	live := func() []string { return result.WrittenPaths }
	return WithWriteGuard(ctx, repairWriteGuard(workspace, namedWriteSet(ctx, result), live, parentWriteGuard(ctx)))
}

func indexWriteSet(workspace string, writeSet []string) (map[string]bool, string) {
	allowed := make(map[string]bool, len(writeSet))
	var names []string
	for _, p := range writeSet {
		normalized := canonicalizeWrittenPath(p, workspace)
		if normalized == "" {
			continue
		}
		abs := normalized
		if workspace != "" && !filepath.IsAbs(filepath.FromSlash(normalized)) {
			abs = filepath.Join(workspace, filepath.FromSlash(normalized))
		}
		allowed[filepath.Clean(abs)] = true
		names = append(names, normalized)
	}
	sort.Strings(names)
	set := strings.Join(names, ", ")
	if set == "" {
		set = "(none yet)"
	}
	return allowed, set
}

// guardWrite asks the context's guard, if there is one, about a write call's
// targets, then refuses a Go write that would hide its file from the build.
// A call whose targets cannot be named is refused under a guard: a write
// that cannot be checked is not let through. The path guard is asked first,
// so a repair's boundary error is the one the model reads; the exclusion
// check runs even when no path guard is installed, because every session
// write mutation comes through here (executeAndRecordToolCall) and hiding a
// file is not a fix whoever the path belongs to.
func guardWrite(ctx context.Context, toolName string, args map[string]any, workspace string) error {
	if err := guardWritePaths(ctx, args, workspace); err != nil {
		return err
	}
	return tools.RefuseAddedBuildExclusion(toolName, args, workspace)
}

func guardWritePaths(ctx context.Context, args map[string]any, workspace string) error {
	guard, _ := ctx.Value(writeGuardKey{}).(WriteGuard)
	if guard == nil {
		return nil
	}
	targets, err := projectdoc.TargetPaths(args)
	if err != nil {
		return fmt.Errorf("the write's target could not be named, so it could not be checked: %w", err)
	}
	var paths []string
	for _, target := range targets {
		normalized := canonicalizeWrittenPath(target, workspace)
		if normalized == "" {
			continue
		}
		if workspace != "" && !filepath.IsAbs(filepath.FromSlash(normalized)) {
			normalized = filepath.Join(workspace, filepath.FromSlash(normalized))
		}
		paths = append(paths, normalized)
	}
	if len(paths) == 0 {
		return nil
	}
	return guard(ctx, paths)
}
