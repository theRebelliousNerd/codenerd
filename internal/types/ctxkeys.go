package types

import "context"

// =============================================================================
// TYPED CONTEXT KEYS
// =============================================================================
//
// Spawn priority and the per-call model hints travel through context.Context.
// They used to travel under bare string keys (CtxKeyPriority = "spawn_priority"
// and friends below), which context.WithValue explicitly warns against: any
// package can write "spawn_priority" into a context and silently outrank the
// scheduler, and `go vet` cannot see the collision because the key type is
// string on both sides. WithSessionContext has always used a private
// zero-width struct key; these now match it.
//
// Migration shape (same as internal/usage took this pass): the setters
// dual-write — typed key first, legacy string key second — and the getters read
// typed first, legacy second. Dual-write is what makes the migration safe in
// either order: internal/core/api_scheduler.go, internal/perception's clients,
// and internal/session still read ctx.Value(types.CtxKeyPriority) directly, and
// a typed-only write would be invisible to them. When every reader has moved to
// the accessors below, drop the legacy writes, then the constants.

type spawnPriorityKeyType struct{}
type modelCapabilityKeyType struct{}
type modelNameKeyType struct{}

var (
	spawnPriorityKey   = spawnPriorityKeyType{}
	modelCapabilityKey = modelCapabilityKeyType{}
	modelNameKey       = modelNameKeyType{}
)

// WithSpawnPriority attaches a scheduling priority to ctx for the spawn/API
// scheduler to honor.
func WithSpawnPriority(ctx context.Context, p SpawnPriority) context.Context {
	ctx = context.WithValue(ctx, spawnPriorityKey, p)
	return context.WithValue(ctx, CtxKeyPriority, p) //nolint:staticcheck // legacy string key, read by api_scheduler/session until they migrate
}

// SpawnPriorityFromContext returns the priority attached to ctx, if any.
// The bool distinguishes "not set" from an explicit PriorityLow, which matters:
// callers that treat absence as PriorityLow would demote every context that
// simply never passed through a spawn path.
func SpawnPriorityFromContext(ctx context.Context) (SpawnPriority, bool) {
	if ctx == nil {
		return PriorityNormal, false
	}
	if v, ok := ctx.Value(spawnPriorityKey).(SpawnPriority); ok {
		return v, true
	}
	if v, ok := ctx.Value(CtxKeyPriority).(SpawnPriority); ok { //nolint:staticcheck // back-compat with string-keyed call sites
		return v, true
	}
	return PriorityNormal, false
}

// WithModelCapability attaches a per-call reasoning-class hint to ctx. A shared
// LLM client reads it to pick a model tier without a per-shard client.
func WithModelCapability(ctx context.Context, c ModelCapability) context.Context {
	ctx = context.WithValue(ctx, modelCapabilityKey, c)
	return context.WithValue(ctx, CtxKeyModelCapability, c) //nolint:staticcheck // legacy string key, read by perception clients until they migrate
}

// ModelCapabilityFromContext returns the capability hint attached to ctx, if any.
func ModelCapabilityFromContext(ctx context.Context) (ModelCapability, bool) {
	if ctx == nil {
		return "", false
	}
	if v, ok := ctx.Value(modelCapabilityKey).(ModelCapability); ok && v != "" {
		return v, true
	}
	if v, ok := ctx.Value(CtxKeyModelCapability).(ModelCapability); ok && v != "" { //nolint:staticcheck // back-compat with string-keyed call sites
		return v, true
	}
	return "", false
}

// WithModelName attaches a concrete model override to ctx. It wins over the
// capability hint at the client, so set it only when a profile names a model.
func WithModelName(ctx context.Context, name string) context.Context {
	ctx = context.WithValue(ctx, modelNameKey, name)
	return context.WithValue(ctx, CtxKeyModelName, name) //nolint:staticcheck // legacy string key, read by perception clients until they migrate
}

// ModelNameFromContext returns the model override attached to ctx, if any.
// An empty override reports false: an empty model name is never actionable and
// treating it as set would blank the client's configured default.
func ModelNameFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if v, ok := ctx.Value(modelNameKey).(string); ok && v != "" {
		return v, true
	}
	if v, ok := ctx.Value(CtxKeyModelName).(string); ok && v != "" { //nolint:staticcheck // back-compat with string-keyed call sites
		return v, true
	}
	return "", false
}
