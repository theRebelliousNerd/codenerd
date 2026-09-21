package session

import (
	"context"
	"strings"

	"codenerd/internal/types"
)

// ShardProfileContext attaches a persona's configured profile -- its model,
// the provider that serves it, its sampling -- to the context a turn's model
// calls are made under.
type ShardProfileContext func(ctx context.Context, shardType string) context.Context

// SetShardProfileContext installs the per-persona profile on the executor.
//
// Until 2026-09-21 the profile was applied in one place, the chat delegation
// path. `nerd fix`, campaigns and every delegated task ran the serving
// client's own model whatever shard_profiles said: the setting was live in the
// TUI and inert everywhere else, and nothing reported the difference.
func (e *Executor) SetShardProfileContext(apply ShardProfileContext) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shardProfileContext = apply
}

// withShardProfile applies the persona's profile to ctx. A context that
// already names a model keeps it: the caller that delegated the turn chose.
func (e *Executor) withShardProfile(ctx context.Context, shardType string) context.Context {
	e.mu.RLock()
	apply := e.shardProfileContext
	e.mu.RUnlock()
	shardType = strings.TrimPrefix(strings.TrimSpace(shardType), "/")
	if apply == nil || shardType == "" {
		return ctx
	}
	if _, set := types.ModelNameFromContext(ctx); set {
		return ctx
	}
	if _, set := types.ProviderFromContext(ctx); set {
		return ctx
	}
	return apply(ctx, shardType)
}
