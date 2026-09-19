package session

import (
	"context"
	"fmt"
	"path/filepath"

	"codenerd/internal/projectdoc"
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

// guardWrite asks the context's guard, if there is one, about a write call's
// targets. A call whose targets cannot be named is refused under a guard: a
// write that cannot be checked is not let through.
func guardWrite(ctx context.Context, args map[string]any, workspace string) error {
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
