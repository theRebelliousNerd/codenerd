package types

import (
	"context"
	"testing"
)

func TestWithSpawnPriority_WhenSet_ShouldBeReadableOnlyThroughTypedKey(t *testing.T) {
	t.Parallel()
	ctx := WithSpawnPriority(context.Background(), PriorityCritical)

	got, ok := SpawnPriorityFromContext(ctx)
	if !ok || got != PriorityCritical {
		t.Fatalf("SpawnPriorityFromContext() = %v, %v; want critical, true", got, ok)
	}
	// The migration is finished, so the inverse of the old assertion holds: the
	// priority must NOT be reachable under the string literal it used to share
	// with anything else in the process that wrote "spawn_priority".
	if v := ctx.Value("spawn_priority"); v != nil {
		t.Fatalf("priority is still reachable under the legacy string key: %v", v)
	}
}

func TestSpawnPriorityFromContext_WhenUnset_ShouldReportNotFound(t *testing.T) {
	t.Parallel()
	got, ok := SpawnPriorityFromContext(context.Background())
	if ok {
		t.Fatalf("expected not-found on a bare context, got %v", got)
	}
	// PriorityLow would be a silent demotion for every caller that never
	// passed through a spawn path, so the miss value is Normal.
	if got != PriorityNormal {
		t.Fatalf("miss value = %v, want normal", got)
	}
	if _, ok := SpawnPriorityFromContext(context.Background()); ok { //nolint:staticcheck // nil ctx must not panic
		t.Fatal("nil context should report not-found, not panic")
	}
}

func TestWithModelCapability_WhenSet_ShouldBeReadableOnlyThroughTypedKey(t *testing.T) {
	t.Parallel()
	ctx := WithModelCapability(context.Background(), CapabilityHighReasoning)

	got, ok := ModelCapabilityFromContext(ctx)
	if !ok || got != CapabilityHighReasoning {
		t.Fatalf("ModelCapabilityFromContext() = %q, %v", got, ok)
	}
	if v := ctx.Value("model_capability"); v != nil {
		t.Fatalf("capability is still reachable under the legacy string key: %v", v)
	}
}

func TestWithModelName_WhenSet_ShouldBeReadableOnlyThroughTypedKey(t *testing.T) {
	t.Parallel()
	ctx := WithModelName(context.Background(), "muse-spark-1.2")

	got, ok := ModelNameFromContext(ctx)
	if !ok || got != "muse-spark-1.2" {
		t.Fatalf("ModelNameFromContext() = %q, %v", got, ok)
	}
	if v := ctx.Value("model_name"); v != nil {
		t.Fatalf("model name is still reachable under the legacy string key: %v", v)
	}
}

func TestModelNameFromContext_WhenEmpty_ShouldReportNotFound(t *testing.T) {
	t.Parallel()
	// An empty override must not read as "set": a client that trusted it would
	// blank its configured default and send a request with no model at all.
	if got, ok := ModelNameFromContext(WithModelName(context.Background(), "")); ok {
		t.Fatalf("empty model name reported as set (%q)", got)
	}
}

// A typed key cannot be forged by another package writing a plain string, which
// is the collision the bare-string keys allowed.
func TestTypedContextKeys_WhenForeignStringKeyCollides_ShouldNotBeMistakenForTyped(t *testing.T) {
	t.Parallel()
	type foreignKey string
	ctx := context.WithValue(context.Background(), foreignKey("spawn_priority"), PriorityCritical)
	if got, ok := SpawnPriorityFromContext(ctx); ok {
		t.Fatalf("a foreign key typed as %T was read as a spawn priority (%v)", foreignKey(""), got)
	}
}
