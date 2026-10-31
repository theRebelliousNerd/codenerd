package world

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
)

type holographicScopeKey struct{}

// A render shares one refreshed membership view. A constructed provider's
// workspace is already canonical; file lookups are independent of process CWD.
type holographicReadScope struct {
	provider *HolographicProvider
	root     string
	members  *workspace.Membership
}

func (h *HolographicProvider) readScope(ctx context.Context, target string) (context.Context, *holographicReadScope, error) {
	if ctx == nil {
		return ctx, nil, fmt.Errorf("context cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if h.rootError != nil {
		return ctx, nil, h.rootError
	}
	if scope, _ := ctx.Value(holographicScopeKey{}).(*holographicReadScope); scope != nil && scope.provider == h {
		return ctx, scope, nil
	}
	root := h.workDir
	if root == "" {
		// Keep the legacy zero-value provider's direct absolute parser use.
		// Constructors capture a root, even for an empty root argument.
		root = workspaceRootOrCwd("")
		if filepath.IsAbs(target) {
			root = filepath.Dir(target)
		}
	}
	root, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return ctx, nil, err
	}
	members, err := workspace.For(root)
	if err != nil {
		return ctx, nil, err
	}
	if err := members.Refresh(); err != nil {
		return ctx, nil, err
	}
	scope := &holographicReadScope{provider: h, root: members.Root(), members: members}
	return context.WithValue(ctx, holographicScopeKey{}, scope), scope, nil
}

func (s *holographicReadScope) resolve(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	abs, err := tools.ResolveWorkspacePath(ctx, s.root, path)
	if err != nil {
		return "", err
	}
	lexical := types.ResolveWorkspacePath(s.root, strings.ReplaceAll(path, "\\", "/"))
	for _, candidate := range []string{lexical, abs} {
		if err := s.members.Gate(candidate, false); err != nil {
			return "", err
		}
		if tools.IsSecretPath(types.CanonicalPath(s.root, candidate)) {
			return "", fmt.Errorf("%w: secret path", workspace.ErrNotMember)
		}
	}
	return abs, nil
}

func (s *holographicReadScope) includes(ctx context.Context, path string) (bool, error) {
	_, err := s.resolve(ctx, path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, workspace.ErrNotMember) || errors.Is(err, tools.ErrPathOutsideWorkspace) {
		return false, nil
	}
	return false, err
}

func (h *HolographicProvider) contextTarget(ctx context.Context, path string) (context.Context, *holographicReadScope, string, error) {
	ctx, scope, err := h.readScope(ctx, path)
	if err != nil {
		return ctx, nil, "", err
	}
	abs, err := scope.resolve(ctx, path)
	if err != nil {
		return ctx, nil, "", err
	}
	info, err := os.Stat(abs)
	if err != nil && !os.IsNotExist(err) {
		return ctx, nil, "", err
	}
	// A deleted target has always yielded partial package context. Admission
	// still precedes this check, so missing targets cannot escape the root or
	// bypass exclusions; only an admitted missing file keeps that behavior.
	if err == nil && !info.Mode().IsRegular() {
		return ctx, nil, "", fmt.Errorf("holographic target is not a regular file: %s", path)
	}
	return ctx, scope, abs, nil
}
