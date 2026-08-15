package types

import (
	"context"
	"testing"
)

func TestWithSpawnPriority_WhenSet_ShouldBeReadableByTypedAndLegacyKey(t *testing.T) {
	t.Parallel()
	ctx := WithSpawnPriority(context.Background(), PriorityCritical)

	got, ok := SpawnPriorityFromContext(ctx)
	if !ok || got != PriorityCritical {
		t.Fatalf("SpawnPriorityFromContext() = %v, %v; want critical, true", got, ok)
	}
	// The dual write is the whole point of the migration: api_scheduler.go and
	// session/task_executor.go still read the bare string key. A typed-only
	// write would make every priority set through this helper invisible to the
	// scheduler, which fails open at PriorityNormal and would look like nothing
	// more than "preemption stopped working".
	legacy, ok := ctx.Value(CtxKeyPriority).(SpawnPriority)
	if !ok || legacy != PriorityCritical {
		t.Fatalf("legacy string key lost the priority: %v, %v", legacy, ok)
	}
}

func TestSpawnPriorityFromContext_WhenSetByLegacyStringKey_ShouldStillRead(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), CtxKeyPriority, PriorityHigh) //nolint:staticcheck // exercising the legacy path on purpose
	got, ok := SpawnPriorityFromContext(ctx)
	if !ok || got != PriorityHigh {
		t.Fatalf("SpawnPriorityFromContext() = %v, %v; want high, true", got, ok)
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
	if _, ok := SpawnPriorityFromContext(nil); ok { //nolint:staticcheck // nil ctx must not panic
		t.Fatal("nil context should report not-found, not panic")
	}
}

func TestWithModelCapability_WhenSet_ShouldBeReadableByTypedAndLegacyKey(t *testing.T) {
	t.Parallel()
	ctx := WithModelCapability(context.Background(), CapabilityHighReasoning)

	got, ok := ModelCapabilityFromContext(ctx)
	if !ok || got != CapabilityHighReasoning {
		t.Fatalf("ModelCapabilityFromContext() = %q, %v", got, ok)
	}
	legacy, ok := ctx.Value(CtxKeyModelCapability).(ModelCapability)
	if !ok || legacy != CapabilityHighReasoning {
		t.Fatalf("legacy string key lost the capability: %q, %v", legacy, ok)
	}
}

func TestModelCapabilityFromContext_WhenSetByLegacyStringKey_ShouldStillRead(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), CtxKeyModelCapability, CapabilityHighSpeed) //nolint:staticcheck // legacy path
	got, ok := ModelCapabilityFromContext(ctx)
	if !ok || got != CapabilityHighSpeed {
		t.Fatalf("ModelCapabilityFromContext() = %q, %v", got, ok)
	}
}

func TestWithModelName_WhenSet_ShouldBeReadableByTypedAndLegacyKey(t *testing.T) {
	t.Parallel()
	ctx := WithModelName(context.Background(), "muse-spark-1.2")

	got, ok := ModelNameFromContext(ctx)
	if !ok || got != "muse-spark-1.2" {
		t.Fatalf("ModelNameFromContext() = %q, %v", got, ok)
	}
	legacy, ok := ctx.Value(CtxKeyModelName).(string)
	if !ok || legacy != "muse-spark-1.2" {
		t.Fatalf("legacy string key lost the model name: %q, %v", legacy, ok)
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
