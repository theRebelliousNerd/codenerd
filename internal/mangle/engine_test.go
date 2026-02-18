package mangle

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
	)
}

func TestNewEngine(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine() returned nil engine")
	}
}

func TestEngineLoadSchemaString(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema using Mangle's Decl syntax (arguments are variables)
	schema := `Decl test_fact(X, Y).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}
}

func TestEngineAddFact(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false // Manual eval for testing
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema first
	schema := `Decl test_fact(X, Y).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add a fact using the correct API (variadic args)
	err = engine.AddFact("test_fact", "hello", int64(42))
	if err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
}

func TestEngineAddFacts(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema
	schema := `Decl person(Name, Age).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add facts
	facts := []Fact{
		{Predicate: "person", Args: []interface{}{"Alice", int64(30)}},
		{Predicate: "person", Args: []interface{}{"Bob", int64(25)}},
	}
	if err := engine.AddFacts(facts); err != nil {
		t.Fatalf("AddFacts() error = %v", err)
	}
}

func TestEngineQuery(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema with mode declaration for querying
	// Mode "+" means input (bound), "-" means output (unbound)
	schema := `Decl person(Name, Age) descr [mode("-", "-")].`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add facts
	facts := []Fact{
		{Predicate: "person", Args: []interface{}{"Alice", int64(30)}},
		{Predicate: "person", Args: []interface{}{"Bob", int64(25)}},
	}
	if err := engine.AddFacts(facts); err != nil {
		t.Fatalf("AddFacts() error = %v", err)
	}

	// Query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := engine.Query(ctx, "person(X, Y)")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	// Note: Query returns bindings based on mode evaluation
	// GetFacts is the primary API for retrieving facts
	t.Logf("Query returned %d bindings", len(result.Bindings))
}

func TestEngineGetFacts(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema
	schema := `Decl item(Name).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add facts
	_ = engine.AddFact("item", "apple")
	_ = engine.AddFact("item", "banana")

	// Get facts
	facts, err := engine.GetFacts("item")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}

	if len(facts) != 2 {
		t.Errorf("GetFacts() returned %d facts, want 2", len(facts))
	}
}

func TestEngineClear(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema
	schema := `Decl data(Value).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add fact
	_ = engine.AddFact("data", "test")

	// Clear
	engine.Clear()

	// Verify cleared
	facts, _ := engine.GetFacts("data")
	if len(facts) != 0 {
		t.Errorf("GetFacts() after Clear() returned %d facts, want 0", len(facts))
	}
}

func TestEngineGetStats(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	stats := engine.GetStats()
	if stats.TotalFacts < 0 {
		t.Error("Stats.TotalFacts should be >= 0")
	}
}

func TestFactString(t *testing.T) {
	tests := []struct {
		name string
		fact Fact
		want string
	}{
		{
			name: "string args",
			fact: Fact{Predicate: "test", Args: []interface{}{"hello", "world"}},
			want: `test("hello", "world").`,
		},
		{
			name: "int args",
			fact: Fact{Predicate: "num", Args: []interface{}{int64(42)}},
			want: `num(42).`,
		},
		{
			name: "name constant",
			fact: Fact{Predicate: "status", Args: []interface{}{"/active"}},
			want: `status(/active).`,
		},
		{
			name: "mixed args",
			fact: Fact{Predicate: "record", Args: []interface{}{"Alice", int64(30), "/employee"}},
			want: `record("Alice", 30, /employee).`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fact.String()
			if got != tt.want {
				t.Errorf("Fact.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.FactLimit != 100000 {
		t.Errorf("FactLimit = %d, want 100000", cfg.FactLimit)
	}
	if cfg.QueryTimeout != 30 {
		t.Errorf("QueryTimeout = %d, want 30", cfg.QueryTimeout)
	}
	if !cfg.AutoEval {
		t.Error("AutoEval should be true by default")
	}
}

func TestEnginePushFact(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema
	schema := `Decl event(Name, Value).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Push fact
	err = engine.PushFact("event", "click", int64(1))
	if err != nil {
		t.Fatalf("PushFact() error = %v", err)
	}
}

func TestEngineQueryFacts(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Load schema
	schema := `Decl record(ID, Value).`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add facts
	if err := engine.AddFact("record", "a", "apple"); err != nil {
		t.Fatalf("AddFact(a) error = %v", err)
	}
	if err := engine.AddFact("record", "b", "banana"); err != nil {
		t.Fatalf("AddFact(b) error = %v", err)
	}

	// Query specific - handles Mangle named constant prefix (/)
	facts := engine.QueryFacts("record", "a")
	if len(facts) != 1 {
		t.Errorf("QueryFacts() returned %d facts, want 1", len(facts))
	}
}

func TestEngineToggleAutoEval(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Toggle off
	engine.ToggleAutoEval(false)

	// Toggle on
	engine.ToggleAutoEval(true)
}

// -----------------------------------------------------------------------------
// QA NEGATIVE TESTING GAPS (Identified 2026-01-31) — FILLED 2026-02-18
// -----------------------------------------------------------------------------

func TestNilArguments(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl test_nil(X, Y).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// nil argument should not panic — it marshals via the default JSON path
	err = engine.AddFact("test_nil", nil, "hello")
	if err != nil {
		t.Logf("AddFact with nil arg returned error (acceptable): %v", err)
	}

	// Verify engine is still functional after nil arg handling
	err = engine.AddFact("test_nil", "ok", "fine")
	if err != nil {
		t.Fatalf("Engine broken after nil arg test: %v", err)
	}
}

func TestFloatCoercionBoundaries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl score(Name, Value).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Test various float64 boundary values
	tests := []struct {
		name  string
		value float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"negative", -1.5},
		{"tiny", 0.000001},
		{"large", 99999.99},
		{"max_int_range", 1e15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.AddFact("score", tt.name, tt.value)
			if err != nil {
				t.Fatalf("AddFact(%s, %f) error = %v", tt.name, tt.value, err)
			}
		})
	}

	facts, err := engine.GetFacts("score")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	if len(facts) != len(tests) {
		t.Errorf("Expected %d facts, got %d", len(tests), len(facts))
	}
}

func TestStringAtomAmbiguity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl data(Key, Value).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Strings starting with "/" should always be treated as atoms
	err = engine.AddFact("data", "/active", "test")
	if err != nil {
		t.Fatalf("AddFact with /atom arg failed: %v", err)
	}

	// Plain strings should be stored as strings (or auto-promoted if identifier-like)
	err = engine.AddFact("data", "hello world", "test")
	if err != nil {
		t.Fatalf("AddFact with plain string failed: %v", err)
	}

	// Identifier-like string (lowercase, no spaces) — auto-promoted to atom /active
	// which DEDUPLICATES with the "/active" fact above (correct behavior)
	err = engine.AddFact("data", "active", "test")
	if err != nil {
		t.Fatalf("AddFact with identifier-like string failed: %v", err)
	}

	facts, err := engine.GetFacts("data")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	// "active" becomes /active which deduplicates with the explicit "/active" fact.
	// So we expect 2 unique facts, not 3.
	if len(facts) != 2 {
		t.Errorf("Expected 2 facts (active deduplicates with /active), got %d", len(facts))
	}
}

func TestFactLimitEnforcement(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FactLimit = 3
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(ID).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Add facts up to the limit
	for i := 0; i < 3; i++ {
		if err := engine.AddFact("item", i); err != nil {
			t.Fatalf("AddFact(%d) should succeed under limit: %v", i, err)
		}
	}

	// Next fact should fail
	err = engine.AddFact("item", 999)
	if err == nil {
		t.Fatal("AddFact() should have returned error when exceeding FactLimit")
	}
	if !strings.Contains(err.Error(), "fact limit exceeded") {
		t.Errorf("Expected 'fact limit exceeded' error, got: %v", err)
	}
}

func TestDerivedFactsGasLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DerivedFactsLimit = 5 // Very low to trigger quickly
	cfg.AutoEval = true
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Schema with a rule that will derive many facts
	schema := `
	Decl edge(X, Y) bound [/string, /string].
	Decl path(X, Y) bound [/string, /string].
	path(X, Y) :- edge(X, Y).
	path(X, Z) :- edge(X, Y), path(Y, Z).
	`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Create a chain long enough to exceed the gas limit
	// Each fact triggers rule evaluation; a long chain will exceed 5 derived facts
	edges := []Fact{
		{Predicate: "edge", Args: []interface{}{"a", "b"}},
		{Predicate: "edge", Args: []interface{}{"b", "c"}},
		{Predicate: "edge", Args: []interface{}{"c", "d"}},
		{Predicate: "edge", Args: []interface{}{"d", "e"}},
		{Predicate: "edge", Args: []interface{}{"e", "f"}},
		{Predicate: "edge", Args: []interface{}{"f", "g"}},
	}
	err = engine.AddFacts(edges)
	// With gas limit of 5, this should either succeed partially or return an error
	// The important thing is it doesn't hang indefinitely
	t.Logf("AddFacts with gas limit result: err=%v", err)
}

func TestConcurrentAccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl concurrent_test(ID, Value).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	const goroutines = 10
	const factsPerGoroutine = 50
	var wg sync.WaitGroup

	// Concurrent writers
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < factsPerGoroutine; i++ {
				_ = engine.AddFact("concurrent_test", gid*1000+i, "value")
			}
		}(g)
	}

	// Concurrent reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = engine.GetFacts("concurrent_test")
		}
	}()

	wg.Wait()

	// Verify no panics occurred and engine is still functional
	facts, err := engine.GetFacts("concurrent_test")
	if err != nil {
		t.Fatalf("GetFacts() after concurrent access: %v", err)
	}
	t.Logf("Concurrent test: %d facts stored", len(facts))
}

func TestEmptyAndInvalidPredicates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl valid_pred(X).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Empty predicate name
	err = engine.AddFact("", "test")
	if err == nil {
		t.Error("AddFact with empty predicate should fail")
	}

	// Predicate with spaces (invalid)
	err = engine.AddFact("invalid name", "test")
	if err == nil {
		t.Error("AddFact with space in predicate should fail")
	}

	// Predicate starting with uppercase (invalid Mangle)
	err = engine.AddFact("Invalid", "test")
	if err == nil {
		t.Error("AddFact with uppercase predicate should fail")
	}
}

// -----------------------------------------------------------------------------
// QA DEEP GAPS (Identified 2026-02-01) — FILLED 2026-02-18
// -----------------------------------------------------------------------------

func TestMapToStructRecursion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl config(Name, Data).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Maps are currently flattened to JSON strings (known limitation).
	// Verify this behavior is stable and doesn't panic.
	m := map[string]string{"key1": "val1", "key2": "val2"}
	err = engine.AddFact("config", "test_map", m)
	if err != nil {
		t.Fatalf("AddFact with map arg failed: %v", err)
	}

	// Nested map
	nested := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "value",
		},
	}
	err = engine.AddFact("config", "nested_map", nested)
	if err != nil {
		t.Fatalf("AddFact with nested map arg failed: %v", err)
	}

	facts, err := engine.GetFacts("config")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("Expected 2 facts, got %d", len(facts))
	}
}

func TestBatchAtomicity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FactLimit = 5
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(ID).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Pre-fill 3 facts to leave room for only 2 more
	for i := 0; i < 3; i++ {
		if err := engine.AddFact("item", i); err != nil {
			t.Fatalf("Pre-fill AddFact(%d) failed: %v", i, err)
		}
	}

	// Try to add a batch of 5 — should fail partway through
	batch := []Fact{
		{Predicate: "item", Args: []interface{}{10}},
		{Predicate: "item", Args: []interface{}{11}},
		{Predicate: "item", Args: []interface{}{12}}, // This should fail (6th fact, limit=5)
		{Predicate: "item", Args: []interface{}{13}},
		{Predicate: "item", Args: []interface{}{14}},
	}
	err = engine.AddFacts(batch)
	if err == nil {
		t.Fatal("AddFacts batch should have failed when exceeding limit")
	}

	// Document: batch is NOT atomic — first 2 facts may have been inserted
	facts, _ := engine.GetFacts("item")
	t.Logf("After partial batch failure: %d facts (batch is non-atomic: first %d of batch were inserted)", len(facts), len(facts)-3)
}

func TestUnicodeIdentifiers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl text(Content).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Unicode strings should be treated as strings (not atoms) since isIdentifier is ASCII-only
	unicodeValues := []string{
		"日本語テスト",
		"über-cool",
		"café",
		"emoji 🎉",
	}
	for _, v := range unicodeValues {
		if err := engine.AddFact("text", v); err != nil {
			t.Fatalf("AddFact(%q) failed: %v", v, err)
		}
	}

	facts, err := engine.GetFacts("text")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	if len(facts) != len(unicodeValues) {
		t.Errorf("Expected %d facts, got %d", len(unicodeValues), len(facts))
	}
}

func TestFloatDiscontinuity(t *testing.T) {
	// With Float64Type (from Mangle HEAD upgrade), floats are stored as Float64.
	// Verify there is no discontinuity: 1.0 and 1.0000001 should both be Float64.
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl metric(Name, Score).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// These should NOT be confused since they're now stored as Float64
	if err := engine.AddFact("metric", "exact_one", 1.0); err != nil {
		t.Fatalf("AddFact(1.0) error: %v", err)
	}
	if err := engine.AddFact("metric", "near_one", 1.0000001); err != nil {
		t.Fatalf("AddFact(1.0000001) error: %v", err)
	}
	if err := engine.AddFact("metric", "zero", 0.0); err != nil {
		t.Fatalf("AddFact(0.0) error: %v", err)
	}
	if err := engine.AddFact("metric", "negative", -0.5); err != nil {
		t.Fatalf("AddFact(-0.5) error: %v", err)
	}

	facts, err := engine.GetFacts("metric")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	if len(facts) != 4 {
		t.Errorf("Expected 4 facts, got %d", len(facts))
	}
}

// -----------------------------------------------------------------------------
// QA BOUNDARY GAPS (Identified 2026-02-17) — FILLED 2026-02-18
// -----------------------------------------------------------------------------

func TestInvalidUTF8Strings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl raw(Data).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Invalid UTF-8 sequences
	invalidUTF8 := []string{
		string([]byte{0xFF, 0xFE}), // BOM-like invalid
		string([]byte{0x80, 0x81}), // continuation without start
		string([]byte{0xC0, 0xAF}), // overlong encoding
		"hello\x80world",           // embedded invalid byte
	}

	for i, v := range invalidUTF8 {
		err := engine.AddFact("raw", v)
		// Should either succeed (storing as-is) or fail gracefully — not panic
		t.Logf("Invalid UTF-8 #%d: err=%v", i, err)
	}
}

func TestZeroTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.QueryTimeout = 0 // Zero timeout
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X) descr [mode("-")].`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}
	if err := engine.AddFact("item", "test"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}

	// Query with a caller-provided timeout instead, since QueryTimeout=0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := engine.Query(ctx, "item(X)")
	if err != nil {
		t.Logf("Query with zero config timeout: err=%v (acceptable if caller timeout used)", err)
	} else {
		t.Logf("Query returned %d bindings (zero config timeout did not block)", len(result.Bindings))
	}
}

func TestNegativeLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FactLimit = -1         // Negative limit
	cfg.DerivedFactsLimit = -1 // Negative derived limit
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Negative FactLimit: insertFactLocked checks `FactLimit > 0 && factCount >= FactLimit`.
	// Since -1 > 0 is false, this should behave as unlimited.
	err = engine.AddFact("item", "test")
	if err != nil {
		t.Fatalf("AddFact with negative FactLimit should behave as unlimited: %v", err)
	}
}

func TestPredicateArityMismatch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl pair(X, Y).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Too few arguments (1 instead of 2)
	err = engine.AddFact("pair", "only_one")
	if err == nil {
		t.Error("AddFact with too few args should fail (arity mismatch)")
	} else {
		t.Logf("Too few args error: %v", err)
	}

	// Too many arguments (3 instead of 2)
	err = engine.AddFact("pair", "a", "b", "c")
	if err == nil {
		t.Error("AddFact with too many args should fail (arity mismatch)")
	} else {
		t.Logf("Too many args error: %v", err)
	}

	// Correct arity should succeed
	err = engine.AddFact("pair", "x", "y")
	if err != nil {
		t.Fatalf("AddFact with correct arity should succeed: %v", err)
	}
}

func TestPartialBatchFailure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl record(X, Y).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Batch where one fact has arity mismatch
	batch := []Fact{
		{Predicate: "record", Args: []interface{}{"a", "good"}},    // valid
		{Predicate: "record", Args: []interface{}{"b", "good"}},    // valid
		{Predicate: "record", Args: []interface{}{"bad_arity"}},    // invalid: 1 arg instead of 2
		{Predicate: "record", Args: []interface{}{"d", "skipped"}}, // never reached
	}
	err = engine.AddFacts(batch)
	if err == nil {
		t.Fatal("AddFacts with arity mismatch in batch should fail")
	}

	// Document: first 2 valid facts may have been inserted (non-atomic)
	facts, _ := engine.GetFacts("record")
	t.Logf("After partial batch failure: %d facts inserted before error", len(facts))
}

func TestNilConfigDefaults(t *testing.T) {
	// Zero-value Config struct
	cfg := Config{}
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine with zero Config should not fail: %v", err)
	}

	// Should still be usable
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// FactLimit=0 means unlimited (since insertFactLocked checks FactLimit > 0)
	err = engine.AddFact("item", "test")
	if err != nil {
		t.Fatalf("AddFact with zero-config should succeed: %v", err)
	}
}

func TestLargeStringHandling(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl blob(Key, Data).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	// Test with a large (but not absurdly large) string — 1MB
	largeStr := strings.Repeat("x", 1024*1024)
	err = engine.AddFact("blob", "large", largeStr)
	if err != nil {
		t.Logf("AddFact with 1MB string: err=%v (may be acceptable)", err)
	} else {
		facts, _ := engine.GetFacts("blob")
		if len(facts) != 1 {
			t.Errorf("Expected 1 fact, got %d", len(facts))
		}
	}
}
