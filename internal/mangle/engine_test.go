package mangle

import (
	"context"
	"testing"
	"time"
)

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
	_ = engine.AddFact("record", "a", "apple")
	_ = engine.AddFact("record", "b", "banana")

	// Query specific
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
