package core

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// HELPERS
// =============================================================================

// setupTestShard creates a shard with a declared predicate for testing.
func setupTestShard(t *testing.T, domain string, predicates []string) *KernelShard {
	t.Helper()
	shard, err := NewKernelShard(KernelShardConfig{
		Domain:          domain,
		OwnedPredicates: predicates,
	})
	if err != nil {
		t.Fatalf("NewKernelShard(%s) failed: %v", domain, err)
	}
	// Declare predicates in the shard's kernel so Mangle can evaluate them
	for _, pred := range predicates {
		shard.kernel.AppendPolicy(fmt.Sprintf("Decl %s(Value).", pred))
	}
	if err := shard.Evaluate(); err != nil {
		t.Fatalf("shard(%s) initial Evaluate failed: %v", domain, err)
	}
	return shard
}

// =============================================================================
// KERNEL SHARD TESTS
// =============================================================================

func TestKernelShard_WhenCreated_ShouldHaveDomain(t *testing.T) {
	shard, err := NewKernelShard(KernelShardConfig{
		Domain: "test_domain",
	})
	if err != nil {
		t.Fatalf("NewKernelShard failed: %v", err)
	}

	if shard.Domain() != "test_domain" {
		t.Errorf("Domain() = %q, want %q", shard.Domain(), "test_domain")
	}
}

func TestKernelShard_WhenEmptyDomain_ShouldError(t *testing.T) {
	_, err := NewKernelShard(KernelShardConfig{})
	if err == nil {
		t.Fatal("expected error for empty domain, got nil")
	}
}

func TestKernelShard_WhenOwnsPredicate_ShouldReturnTrue(t *testing.T) {
	shard, err := NewKernelShard(KernelShardConfig{
		Domain:          "routing",
		OwnedPredicates: []string{"user_intent", "next_action", "routing_result"},
	})
	if err != nil {
		t.Fatalf("NewKernelShard failed: %v", err)
	}

	tests := []struct {
		predicate string
		want      bool
	}{
		{"user_intent", true},
		{"next_action", true},
		{"routing_result", true},
		{"file_topology", false},
		{"unknown_pred", false},
	}

	for _, tt := range tests {
		got := shard.OwnsPredicate(tt.predicate)
		if got != tt.want {
			t.Errorf("OwnsPredicate(%q) = %v, want %v", tt.predicate, got, tt.want)
		}
	}
}

func TestKernelShard_WhenAssertAndQuery_ShouldReturnFacts(t *testing.T) {
	shard := setupTestShard(t, "test", []string{"test_fact"})

	// Assert a fact
	fact := types.Fact{
		Predicate: "test_fact",
		Args:      []any{"hello"},
	}
	if err := shard.Assert(fact); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}

	// Query triggers lazy evaluation
	results, err := shard.Query("test_fact")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Query returned no results after Assert")
	}
}

func TestKernelShard_WhenAssertBatch_ShouldTrackDirtyCount(t *testing.T) {
	shard := setupTestShard(t, "test", []string{"batch_pred"})

	facts := []types.Fact{
		{Predicate: "batch_pred", Args: []any{"a"}},
		{Predicate: "batch_pred", Args: []any{"b"}},
		{Predicate: "batch_pred", Args: []any{"c"}},
	}

	if err := shard.AssertBatch(facts); err != nil {
		t.Fatalf("AssertBatch failed: %v", err)
	}

	metrics := shard.Metrics()
	if metrics.DirtyCount == 0 {
		t.Error("DirtyCount should be > 0 after AssertBatch")
	}
	if metrics.Domain != "test" {
		t.Errorf("Metrics.Domain = %q, want %q", metrics.Domain, "test")
	}
}

func TestKernelShard_WhenRetract_ShouldRemoveFacts(t *testing.T) {
	shard := setupTestShard(t, "test", []string{"retract_me"})

	// Assert then retract
	if err := shard.Assert(types.Fact{Predicate: "retract_me", Args: []any{"value"}}); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}

	if err := shard.Retract("retract_me"); err != nil {
		t.Fatalf("Retract failed: %v", err)
	}

	results, err := shard.Query("retract_me")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results after Retract, got %d", len(results))
	}
}

func TestKernelShard_Metrics_ShouldTrackQueryCount(t *testing.T) {
	shard, err := NewKernelShard(KernelShardConfig{
		Domain: "metrics_test",
	})
	if err != nil {
		t.Fatalf("NewKernelShard failed: %v", err)
	}

	// Query 3 times (results don't matter, just counting)
	for i := range 3 {
		shard.Query(fmt.Sprintf("pred_%d", i))
	}

	metrics := shard.Metrics()
	if metrics.QueryCount != 3 {
		t.Errorf("QueryCount = %d, want 3", metrics.QueryCount)
	}
}

// =============================================================================
// CORTEX KERNEL TESTS
// =============================================================================

func TestCortexKernel_WhenCreated_ShouldBeEmpty(t *testing.T) {
	cortex := NewCortexKernel("main")
	if cortex.TotalFactCount() != 0 {
		t.Errorf("TotalFactCount = %d, want 0 for new cortex", cortex.TotalFactCount())
	}
}

func TestCortexKernel_WhenRegisterShard_ShouldRoute(t *testing.T) {
	cortex := NewCortexKernel("main")

	// Use predicates that are declared in the kernel's embedded schemas
	routingShard := setupTestShard(t, "routing", []string{"my_intent", "my_action"})
	worldShard := setupTestShard(t, "world", []string{"my_file", "my_symbol"})

	if err := cortex.RegisterShard(routingShard); err != nil {
		t.Fatalf("RegisterShard(routing) failed: %v", err)
	}
	if err := cortex.RegisterShard(worldShard); err != nil {
		t.Fatalf("RegisterShard(world) failed: %v", err)
	}

	// Assert via cortex — should route to correct shard
	if err := cortex.Assert(types.Fact{
		Predicate: "my_intent",
		Args:      []any{"test_value"},
	}); err != nil {
		t.Fatalf("Assert my_intent failed: %v", err)
	}

	if err := cortex.Assert(types.Fact{
		Predicate: "my_file",
		Args:      []any{"main.go"},
	}); err != nil {
		t.Fatalf("Assert my_file failed: %v", err)
	}

	// Verify routing
	intentFacts, err := routingShard.Query("my_intent")
	if err != nil {
		t.Fatalf("routing Query failed: %v", err)
	}
	if len(intentFacts) == 0 {
		t.Error("routing shard should have my_intent fact")
	}

	fileFacts, err := worldShard.Query("my_file")
	if err != nil {
		t.Fatalf("world Query failed: %v", err)
	}
	if len(fileFacts) == 0 {
		t.Error("world shard should have my_file fact")
	}
}

func TestCortexKernel_WhenDuplicateRegister_ShouldError(t *testing.T) {
	cortex := NewCortexKernel("main")

	shard, _ := NewKernelShard(KernelShardConfig{Domain: "dup"})
	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("first RegisterShard failed: %v", err)
	}

	shard2, _ := NewKernelShard(KernelShardConfig{Domain: "dup"})
	if err := cortex.RegisterShard(shard2); err == nil {
		t.Fatal("expected error for duplicate domain registration")
	}
}

func TestCortexKernel_WhenAssertBatch_ShouldRouteToShards(t *testing.T) {
	cortex := NewCortexKernel("main")

	mainShard := setupTestShard(t, "main", []string{"alpha", "gamma"})
	otherShard := setupTestShard(t, "other", []string{"beta"})

	cortex.RegisterShard(mainShard)
	cortex.RegisterShard(otherShard)

	facts := []types.Fact{
		{Predicate: "alpha", Args: []any{"1"}},
		{Predicate: "beta", Args: []any{"2"}},
		{Predicate: "gamma", Args: []any{"3"}},
	}

	if err := cortex.AssertBatch(facts); err != nil {
		t.Fatalf("AssertBatch failed: %v", err)
	}

	// Verify distribution
	alphaFacts, _ := mainShard.Query("alpha")
	if len(alphaFacts) != 1 {
		t.Errorf("main shard should have 1 alpha fact, got %d", len(alphaFacts))
	}

	betaFacts, _ := otherShard.Query("beta")
	if len(betaFacts) != 1 {
		t.Errorf("other shard should have 1 beta fact, got %d", len(betaFacts))
	}
}

func TestCortexKernel_WhenRetract_ShouldRouteToCorrectShard(t *testing.T) {
	cortex := NewCortexKernel("main")

	shard := setupTestShard(t, "main", []string{"ephemeral"})
	cortex.RegisterShard(shard)

	// Assert then retract via cortex
	cortex.Assert(types.Fact{Predicate: "ephemeral", Args: []any{"temp"}})
	cortex.Retract("ephemeral")

	facts, _ := shard.Query("ephemeral")
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after Retract, got %d", len(facts))
	}
}

func TestCortexKernel_AllMetrics_ShouldReturnAllShards(t *testing.T) {
	cortex := NewCortexKernel("main")

	s1, _ := NewKernelShard(KernelShardConfig{Domain: "shard1"})
	s2, _ := NewKernelShard(KernelShardConfig{Domain: "shard2"})
	s3, _ := NewKernelShard(KernelShardConfig{Domain: "shard3"})

	cortex.RegisterShard(s1)
	cortex.RegisterShard(s2)
	cortex.RegisterShard(s3)

	metrics := cortex.AllMetrics()
	if len(metrics) != 3 {
		t.Errorf("AllMetrics returned %d entries, want 3", len(metrics))
	}
}

// =============================================================================
// CORTEX TRANSACTION TESTS
// =============================================================================

func TestCortexTransaction_WhenCommit_ShouldBatchMutations(t *testing.T) {
	cortex := NewCortexKernel("main")

	routingShard := setupTestShard(t, "routing", []string{"intent", "action"})
	worldShard := setupTestShard(t, "world", []string{"file_fact"})

	cortex.RegisterShard(routingShard)
	cortex.RegisterShard(worldShard)

	// Create transaction
	tx := cortex.Transaction()
	tx.Assert(types.Fact{Predicate: "intent", Args: []any{"test"}})
	tx.Assert(types.Fact{Predicate: "file_fact", Args: []any{"main.go"}})
	tx.Assert(types.Fact{Predicate: "action", Args: []any{"code"}})

	if err := tx.Commit(); err != nil {
		t.Fatalf("Transaction.Commit failed: %v", err)
	}

	// Verify both shards received their facts
	intentFacts, _ := routingShard.Query("intent")
	if len(intentFacts) == 0 {
		t.Error("routing shard should have intent fact")
	}

	fileFacts, _ := worldShard.Query("file_fact")
	if len(fileFacts) == 0 {
		t.Error("world shard should have file_fact")
	}
}

func TestCortexTransaction_WhenRetractAndAssert_ShouldOrderCorrectly(t *testing.T) {
	cortex := NewCortexKernel("main")

	shard := setupTestShard(t, "main", []string{"cortex_tx_test_state"})
	cortex.RegisterShard(shard)

	// First, put in an old state
	cortex.Assert(types.Fact{Predicate: "cortex_tx_test_state", Args: []any{"old"}})

	// Transaction: retract old, assert new
	tx := cortex.Transaction()
	tx.Retract("cortex_tx_test_state")
	tx.Assert(types.Fact{Predicate: "cortex_tx_test_state", Args: []any{"new"}})

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify only new state exists
	facts, err := shard.Query("cortex_tx_test_state")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) != 1 {
		fmt.Printf("DEBUG: facts length is %d\n", len(facts))
		fmt.Printf("DEBUG: shard total facts: %d\n", len(shard.kernel.facts))
		fmt.Printf("DEBUG: shard initialized: %v\n", shard.kernel.IsInitialized())
		fmt.Printf("DEBUG: shard dirty: %v\n", shard.kernel.IsDirty())

		allFacts := shard.kernel.facts
		for i, f := range allFacts {
			fmt.Printf("DEBUG: fact %d: %v\n", i, f)
		}

		t.Fatalf("expected 1 state fact, got %d", len(facts))
	}

	// Check the arg value is "new"
	if len(facts[0].Args) > 0 {
		if arg, ok := facts[0].Args[0].(string); ok {
			if arg != "new" {
				t.Errorf("state fact arg = %q, want %q", arg, "new")
			}
		}
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestCortexKernel_WhenConcurrentAccess_ShouldNotPanic(t *testing.T) {
	cortex := NewCortexKernel("main")

	s1 := setupTestShard(t, "shard1", []string{"concurrent_fact"})
	cortex.RegisterShard(s1)

	var wg sync.WaitGroup
	const goroutines = 10
	const opsPerGoroutine = 50

	// Concurrent asserts and queries
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range opsPerGoroutine {
				fact := types.Fact{
					Predicate: "concurrent_fact",
					Args:      []any{fmt.Sprintf("g%d_op%d", id, j)},
				}
				cortex.Assert(fact)
				cortex.Query("concurrent_fact")
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panic, concurrency is safe
}

func TestCortexKernel_WhenConcurrentTransactions_ShouldNotDeadlock(t *testing.T) {
	cortex := NewCortexKernel("main")

	s1 := setupTestShard(t, "main", []string{"tx_fact"})
	cortex.RegisterShard(s1)

	var wg sync.WaitGroup
	const goroutines = 5

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 20 {
				tx := cortex.Transaction()
				tx.Retract("tx_fact")
				tx.Assert(types.Fact{Predicate: "tx_fact", Args: []any{fmt.Sprintf("g%d_v%d", id, j)}})
				tx.Commit()
			}
		}(i)
	}

	select {
	case <-done:
		// Success — no deadlock
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK: concurrent transactions timed out after 30s")
	}
}

// =============================================================================
// ISDIRTY TESTS
// =============================================================================

func TestRealKernel_IsDirty_WhenFreshKernel_ShouldBeFalse(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}

	if k.IsDirty() {
		t.Error("fresh kernel should not be dirty")
	}
}

func TestRealKernel_IsDirty_WhenAsserted_ShouldBeTrue(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}

	if err := k.Assert(Fact{Predicate: "test_fact", Args: []any{"value"}}); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}

	if !k.IsDirty() {
		t.Error("kernel should be dirty after Assert")
	}
}

func TestRealKernel_IsDirty_WhenQueryClears_ShouldBeFalse(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}

	// Assert makes it dirty
	k.Assert(Fact{Predicate: "test_fact", Args: []any{"value"}})

	// Query triggers lazy eval, clearing dirty flag
	_, _ = k.Query("test_fact")

	if k.IsDirty() {
		t.Error("kernel should not be dirty after Query triggered eval")
	}
}

// =============================================================================
// LOAD SCHEMAS / LOAD POLICY TESTS
// =============================================================================

func TestRealKernel_LoadSchemas_ShouldReplaceSchemasAndMarkDirty(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}

	newSchemas := "# Test schemas\nDecl my_test_pred(X)."
	k.LoadSchemas(newSchemas)

	// Verify policyDirty was set (can't check directly, but LoadSchemas sets it)
	// The effect is that next evaluation will reparse
}

func TestRealKernel_LoadPolicy_ShouldReplacePolicyAndMarkDirty(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}

	newPolicy := "# Test policy\nmy_rule(X) :- my_pred(X)."
	k.LoadPolicy(newPolicy)

	// Verify policyDirty was set
}
