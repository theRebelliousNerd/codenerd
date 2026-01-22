package autopoiesis

import (
	"codenerd/internal/mangle"
	"testing"
	"time"
)

func TestOuroboros_PanicPersistence(t *testing.T) {
	// 1. Setup Mangle Engine
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = false // Disable auto-eval to avoid needing full logic
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create Mangle engine: %v", err)
	}

	// 2. Load Schema
	// We need to declare the predicates used in handlePanic
	// Using simple declaration syntax as seen in other .mg files
	schema := `
	Decl error_event(EventType).
	Decl error_history(StepID, EventType, Timestamp).
	`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	// 3. Create OuroborosLoop
	// We only need the engine and the mutex (zero value is fine)
	loop := &OuroborosLoop{
		engine: engine,
	}

	// 4. Invoke handlePanic
	stepID := "/step_test_panic"
	panicVal := "simulated panic"
	result := &LoopResult{}

	// We are testing the private method handlePanic directly since we are in the same package
	loop.handlePanic(stepID, panicVal, result)

	// 5. Assertions

	// Check result
	if result.Success {
		t.Error("Expected result.Success to be false")
	}
	if result.Stage != StagePanic {
		t.Errorf("Expected result.Stage to be StagePanic, got %v", result.Stage)
	}
	expectedError := "PANIC recovered in Ouroboros: simulated panic"
	if result.Error != expectedError {
		t.Errorf("Expected result.Error to be %q, got %q", expectedError, result.Error)
	}

	// Check Stats
	stats := loop.GetStats()
	if stats.Panics != 1 {
		t.Errorf("Expected 1 panic in stats, got %d", stats.Panics)
	}

	// Check Mangle Facts
	// Check error_event("/panic")
	facts, err := engine.GetFacts("error_event")
	if err != nil {
		t.Fatalf("Failed to get facts for error_event: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("Expected 1 error_event fact, got %d", len(facts))
	} else {
		// Verify args: ["/panic"]
		arg := facts[0].Args[0]
		if arg != "/panic" {
			t.Errorf("Expected error_event arg to be '/panic', got %v", arg)
		}
	}

	// Check error_history(stepID, "/panic", timestamp)
	facts, err = engine.GetFacts("error_history")
	if err != nil {
		t.Fatalf("Failed to get facts for error_history: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("Expected 1 error_history fact, got %d", len(facts))
	} else {
		// Verify args: [stepID, "/panic", timestamp]
		if len(facts[0].Args) != 3 {
			t.Errorf("Expected 3 args for error_history, got %d", len(facts[0].Args))
		}
		if facts[0].Args[0] != stepID {
			t.Errorf("Expected error_history arg 0 to be %q, got %v", stepID, facts[0].Args[0])
		}
		if facts[0].Args[1] != "/panic" {
			t.Errorf("Expected error_history arg 1 to be '/panic', got %v", facts[0].Args[1])
		}
		// Arg 2 is timestamp (int64), just check it's recent
		ts, ok := facts[0].Args[2].(int64)
		if !ok {
			// Mangle might store it as int or int64 depending on how it was passed
			// In AddFacts it's passed as time.Now().Unix() which is int64
			// mangle.Fact stores []interface{}, so it should be int64
			// But let's check if it's convertible
			t.Errorf("Expected error_history arg 2 to be int64, got %T: %v", facts[0].Args[2], facts[0].Args[2])
		} else {
			now := time.Now().Unix()
			if now-ts > 5 || ts-now > 5 {
				t.Errorf("Expected timestamp close to %d, got %d", now, ts)
			}
		}
	}
}
