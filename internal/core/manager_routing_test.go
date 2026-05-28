package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// setupMockCortexKernel registers a test shard with predicates required by tool routing.
func setupMockCortexKernel(t *testing.T) (*CortexKernel, *KernelShard) {
	t.Helper()

	cortex := NewCortexKernel("main")

	// Pre-declared predicates needed for tool routing and intent assertions
	ownedPreds := []string{
		"current_shard_type",
		"current_intent",
		"current_time",
		"user_intent",
		"tool_registered",
		"tool_description",
		"tool_binary_path",
		"relevant_tool",
		"tool_base_relevance",
	}

	shard, err := NewKernelShard(KernelShardConfig{
		Domain:          "main",
		OwnedPredicates: ownedPreds,
	})
	if err != nil {
		t.Fatalf("Failed to create main shard: %v", err)
	}
	if err := shard.Evaluate(); err != nil {
		t.Fatalf("Shard evaluation failed: %v", err)
	}

	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("RegisterShard failed: %v", err)
	}

	return cortex, shard
}

func TestShardManager_AssertToolRoutingContext(t *testing.T) {
	cortex, shard := setupMockCortexKernel(t)

	sm := shards.NewShardManager()
	sm.SetParentKernel(cortex)

	// Inject tool registrations into the mock kernel to simulate real workspace facts
	cortex.Assert(types.Fact{Predicate: "tool_registered", Args: []any{"git_diff", int64(0)}})
	cortex.Assert(types.Fact{Predicate: "tool_registered", Args: []any{"ripgrep", int64(0)}})
	cortex.Assert(types.Fact{Predicate: "tool_description", Args: []any{"git_diff", "Computes changes"}})
	cortex.Assert(types.Fact{Predicate: "tool_binary_path", Args: []any{"git_diff", "/usr/bin/git"}})

	// Run assertToolRoutingContext via ShardManager
	query := shards.ToolRelevanceQuery{
		ShardType:   "coder",
		IntentVerb:  "refactor",
		TargetFile:  "main.go",
		TokenBudget: 1000,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("AssertToolRoutingContext panicked: %v", r)
			}
		}()
		sm.AssertToolRoutingContext(query)
	}()

	// Query Mangle facts from shard to verify atomic transactions applied correctly
	shardTypeFacts, err := shard.Query("current_shard_type")
	if err != nil {
		t.Fatalf("Query current_shard_type failed: %v", err)
	}
	if len(shardTypeFacts) != 1 || shardTypeFacts[0].Args[0].(string) != "/coder" {
		t.Errorf("Expected current_shard_type /coder, got %v", shardTypeFacts)
	}

	intentFacts, err := shard.Query("current_intent")
	if err != nil {
		t.Fatalf("Query current_intent failed: %v", err)
	}
	if len(intentFacts) != 1 || intentFacts[0].Args[0].(string) != "/tool_routing_context" {
		t.Errorf("Expected current_intent /tool_routing_context, got %v", intentFacts)
	}

	userIntentFacts, err := shard.Query("user_intent")
	if err != nil {
		t.Fatalf("Query user_intent failed: %v", err)
	}
	if len(userIntentFacts) != 1 {
		t.Errorf("Expected 1 user_intent fact, got %v", userIntentFacts)
	} else {
		args := userIntentFacts[0].Args
		if args[0].(string) != "/tool_routing_context" ||
			args[1].(string) != "/routing" ||
			args[2].(string) != "/refactor" ||
			args[3].(string) != "main.go" {
			t.Errorf("Unexpected user_intent args: %v", args)
		}
	}

	timeFacts, _ := shard.Query("current_time")
	if len(timeFacts) != 1 {
		t.Errorf("Expected current_time assertion, got %v", timeFacts)
	}
}

func TestShardManager_ConcurrentToolQueries(t *testing.T) {
	cortex, _ := setupMockCortexKernel(t)

	sm := shards.NewShardManager()
	sm.SetParentKernel(cortex)

	// Pre-populate some tools and relevance scores
	cortex.Assert(types.Fact{Predicate: "tool_registered", Args: []any{"tool_alpha", int64(0)}})
	cortex.Assert(types.Fact{Predicate: "tool_registered", Args: []any{"tool_beta", int64(0)}})
	cortex.Assert(types.Fact{Predicate: "tool_base_relevance", Args: []any{"/coder", "tool_alpha", int64(95)}})
	cortex.Assert(types.Fact{Predicate: "tool_base_relevance", Args: []any{"/coder", "tool_beta", int64(40)}})

	var wg sync.WaitGroup
	const concurrentUsers = 30
	const queriesPerUser = 20

	errs := make(chan error, concurrentUsers*queriesPerUser)

	for i := range concurrentUsers {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			for j := range queriesPerUser {
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				_ = ctx // Context can be used if we spawn async, here we query relevance
				query := shards.ToolRelevanceQuery{
					ShardType:   "coder",
					IntentVerb:  fmt.Sprintf("verb_%d", j),
					TargetFile:  fmt.Sprintf("file_%d_%d.go", userID, j),
					TokenBudget: 2000,
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							errs <- fmt.Errorf("User %d query %d panicked: %v", userID, j, r)
						}
					}()
					relevant := sm.QueryRelevantTools(query)
					if len(relevant) == 0 {
						// There are no registered relevant_tool facts, so it falls back to all registered tools, which should return at least 2 tools
						errs <- fmt.Errorf("Expected at least 2 tools, got 0")
					}
				}()
				cancel()
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("Concurrent tool query boundary test failed: %v", err)
	}
}
