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
// The migration completed on 2026-09-10. It ran as a dual write — typed key
// first, legacy string key second — with getters reading typed first and legacy
// second, so readers could move in any order. Every production reader and every
// test now goes through the accessors below, so the legacy writes, the legacy
// reads, and the CtxKeyPriority / CtxKeyModelCapability / CtxKeyModelName
// constants are gone.
//
// What that buys: a string key is writable by any package that happens to use
// the same literal, and neither the compiler nor go vet can see the collision
// because the key type is string on both sides. These keys are now unexported
// zero-width struct types, so a foreign package cannot construct one at all.
// TestTypedContextKeys_WhenForeignStringKeyCollides_ShouldNotBeMistakenForTyped
// pins that property.

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
	return context.WithValue(ctx, spawnPriorityKey, p)
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
	return PriorityNormal, false
}

// WithModelCapability attaches a per-call reasoning-class hint to ctx. A shared
// LLM client reads it to pick a model tier without a per-shard client.
func WithModelCapability(ctx context.Context, c ModelCapability) context.Context {
	return context.WithValue(ctx, modelCapabilityKey, c)
}

// ModelCapabilityFromContext returns the capability hint attached to ctx, if any.
func ModelCapabilityFromContext(ctx context.Context) (ModelCapability, bool) {
	if ctx == nil {
		return "", false
	}
	if v, ok := ctx.Value(modelCapabilityKey).(ModelCapability); ok && v != "" {
		return v, true
	}
	return "", false
}

// WithModelName attaches a concrete model override to ctx. It wins over the
// capability hint at the client, so set it only when a profile names a model.
func WithModelName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, modelNameKey, name)
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
	return "", false
}

// Sampling carries the per-call sampling a shard profile configured. A zero
// field means "not configured": the client keeps its own value for it. Shard
// profiles carried temperature and top_p for months without any client
// reading them; this is the seam that makes them live, the same way the
// model override reaches a shared client without a per-shard client.
type Sampling struct {
	Temperature float64
	TopP        float64
}

type samplingKeyType struct{}

var samplingKey = samplingKeyType{}

// WithSampling attaches s to ctx. A Sampling with nothing configured attaches
// nothing, so callers can pass a profile's values through unconditionally.
func WithSampling(ctx context.Context, s Sampling) context.Context {
	if s.Temperature <= 0 && s.TopP <= 0 {
		return ctx
	}
	return context.WithValue(ctx, samplingKey, s)
}

// SamplingFromContext returns the sampling attached to ctx, if any.
func SamplingFromContext(ctx context.Context) (Sampling, bool) {
	if ctx == nil {
		return Sampling{}, false
	}
	if v, ok := ctx.Value(samplingKey).(Sampling); ok && (v.Temperature > 0 || v.TopP > 0) {
		return v, true
	}
	return Sampling{}, false
}

// TemperatureFor is the temperature a request builder should send: the
// profile's, when one is attached to ctx, else fallback (the client's own
// default, which may be zero to leave the vendor default in force).
func TemperatureFor(ctx context.Context, fallback float64) float64 {
	if s, ok := SamplingFromContext(ctx); ok && s.Temperature > 0 {
		return s.Temperature
	}
	return fallback
}

// TopPFor is the top_p counterpart of TemperatureFor.
func TopPFor(ctx context.Context, fallback float64) float64 {
	if s, ok := SamplingFromContext(ctx); ok && s.TopP > 0 {
		return s.TopP
	}
	return fallback
}
