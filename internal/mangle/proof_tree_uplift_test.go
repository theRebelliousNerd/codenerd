package mangle

import (
	"context"
	"testing"
	"time"
)

// Behavioral proofs for the tracer uplift: cache identity, FIFO eviction,
// and fail-closed nil handling.

func upliftTracerEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	schema := `
	Decl dependency_link(File, Dep, Type) descr [mode("-", "-", "-")].
	Decl impacted(File) descr [mode("-")].
	impacted(X) :- dependency_link(X, _, _).
	`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString failed: %v", err)
	}
	if err := engine.AddFact("dependency_link", "main.go", "lib.go", "import"); err != nil {
		t.Fatalf("AddFact failed: %v", err)
	}
	return engine
}

func upliftTraceCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// A repeated identical query is served the cached derivation, not recomputed.
func TestTracerCacheHitReturnsIdentity(t *testing.T) {
	tracer := NewProofTreeTracer(upliftTracerEngine(t))
	tracer.IndexRules()
	ctx, cancel := upliftTraceCtx()
	defer cancel()
	first, err := tracer.TraceQuery(ctx, "impacted(X)")
	if err != nil {
		t.Fatalf("first trace failed: %v", err)
	}
	second, err := tracer.TraceQuery(ctx, "impacted(X)")
	if err != nil {
		t.Fatalf("second trace failed: %v", err)
	}
	if first != second {
		t.Fatal("cache miss: identical query returned a different trace object")
	}
}

// Eviction is FIFO: the oldest query goes first and the cache never exceeds
// its bound.
func TestTracerEvictionIsFIFO(t *testing.T) {
	tracer := NewProofTreeTracer(upliftTracerEngine(t))
	tracer.IndexRules()
	tracer.maxCache = 3
	ctx, cancel := upliftTraceCtx()
	defer cancel()
	queries := []string{"impacted(X)", "impacted(Y)", "impacted(Z)", "impacted(W)"}
	for _, q := range queries {
		if _, err := tracer.TraceQuery(ctx, q); err != nil {
			t.Fatalf("trace %q failed: %v", q, err)
		}
	}
	if n := len(tracer.traces); n != 3 {
		t.Fatalf("cache holds %d traces, want exactly 3", n)
	}
	if _, ok := tracer.traces["impacted(X)"]; ok {
		t.Fatal("oldest query survived eviction; eviction is not FIFO")
	}
	for _, q := range queries[1:] {
		if _, ok := tracer.traces[q]; !ok {
			t.Fatalf("recent query %q was evicted; eviction is not FIFO", q)
		}
	}
}

// ClearCache empties both the map and the order list, so the next fill
// evicts from a clean slate.
func TestTracerClearCacheResetsOrder(t *testing.T) {
	tracer := NewProofTreeTracer(upliftTracerEngine(t))
	tracer.IndexRules()
	ctx, cancel := upliftTraceCtx()
	defer cancel()
	if _, err := tracer.TraceQuery(ctx, "impacted(X)"); err != nil {
		t.Fatalf("trace failed: %v", err)
	}
	tracer.ClearCache()
	if len(tracer.traces) != 0 || len(tracer.traceOrder) != 0 {
		t.Fatalf("cache not empty after clear: %d traces, %d order entries",
			len(tracer.traces), len(tracer.traceOrder))
	}
}

// Nil tracer and nil engine fail closed with errors, never panics.
func TestTracerNilHandling(t *testing.T) {
	ctx, cancel := upliftTraceCtx()
	defer cancel()
	var nilTracer *ProofTreeTracer
	if _, err := nilTracer.TraceQuery(ctx, "impacted(X)"); err == nil {
		t.Fatal("nil tracer TraceQuery returned nil error")
	}
	nilTracer.ClearCache() // must not panic
	nilTracer.IndexRules() // must not panic
	empty := NewProofTreeTracer(nil)
	empty.IndexRules() // must not panic
	if _, err := empty.TraceQuery(ctx, "impacted(X)"); err == nil {
		t.Fatal("nil-engine TraceQuery returned nil error")
	}
}
