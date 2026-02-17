package store

import (
	"fmt"
	"testing"
	"time"
)

func TestCleanupEdgeCases(t *testing.T) {
	// Setup
	ts, err := NewToolStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create tool store: %v", err)
	}
	defer ts.Close()

	// 1. Test Empty Store
	stats, err := ts.CleanupByRuntimeBudget(10.0)
	if err != nil {
		t.Errorf("Cleanup on empty store failed: %v", err)
	}
	if stats.ExecutionsDeleted != 0 {
		t.Errorf("Expected 0 deletions, got %d", stats.ExecutionsDeleted)
	}

	// 2. Test Exact Budget (No deletion needed)
	// Insert one session with 1 hour runtime
	_, err = ts.db.Exec(`INSERT INTO tool_executions (call_id, session_id, tool_name, result, success, duration_ms, result_size, session_runtime_ms, created_at) VALUES ('1', 's1', 't1', 'res', 1, 100, 100, 3600000, ?)`, time.Now())
    if err != nil {
        t.Fatalf("Insert failed: %v", err)
    }

	stats, err = ts.CleanupByRuntimeBudget(1.0) // Budget matches usage
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
	if stats.ExecutionsDeleted != 0 {
		t.Errorf("Expected 0 deletions for exact budget, got %d", stats.ExecutionsDeleted)
	}

	// 3. Test Budget slightly exceeded (Should delete the session)
	stats, err = ts.CleanupByRuntimeBudget(0.99)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
	if stats.ExecutionsDeleted != 1 {
		t.Errorf("Expected 1 deletion, got %d", stats.ExecutionsDeleted)
	}

    // Verify store is empty
    var count int
    ts.db.QueryRow("SELECT COUNT(*) FROM tool_executions").Scan(&count)
    if count != 0 {
        t.Errorf("Store should be empty, got %d rows", count)
    }

    // 4. Test Large Batch Deletion (Batch size edge case)
    // Batch size is 500. Let's insert 550 sessions.
    tx, _ := ts.db.Begin()
    stmt, _ := tx.Prepare(`INSERT INTO tool_executions (call_id, session_id, tool_name, result, success, duration_ms, result_size, session_runtime_ms, created_at) VALUES (?, ?, 't1', 'res', 1, 100, 100, 3600000, ?)`) // 1hr runtime

    startTime := time.Now().Add(-10 * time.Hour)
    for i := 0; i < 550; i++ {
        _, err := stmt.Exec(fmt.Sprintf("call-%d", i), fmt.Sprintf("sess-%d", i), startTime.Add(time.Duration(i)*time.Second))
        if err != nil {
             t.Fatalf("Batch insert failed: %v", err)
        }
    }
    tx.Commit()

    // Total runtime: 550 * 1 = 550 hours.
    // Set budget to 0.
    stats, err = ts.CleanupByRuntimeBudget(0.0)
    if err != nil {
        t.Errorf("Cleanup large batch failed: %v", err)
    }
    if stats.ExecutionsDeleted != 550 {
        t.Errorf("Expected 550 deletions, got %d", stats.ExecutionsDeleted)
    }
}

func BenchmarkCleanupByRuntimeBudget(b *testing.B) {
	// Setup
	ts, err := NewToolStore(":memory:")
	if err != nil {
		b.Fatalf("Failed to create tool store: %v", err)
	}
	defer ts.Close()

    // Parameters
	numSessions := 200
	executionsPerSession := 20

    b.ResetTimer()

    for n := 0; n < b.N; n++ {
        b.StopTimer()
        // Repopulate DB
        ts.db.Exec("DELETE FROM tool_executions")
        tx, _ := ts.db.Begin()
        stmt, _ := tx.Prepare(`
            INSERT INTO tool_executions
            (call_id, session_id, tool_name, result, success, duration_ms, result_size, session_runtime_ms, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `)

        startTime := time.Now().Add(-24 * time.Hour)
        for i := 0; i < numSessions; i++ {
            sessionID := fmt.Sprintf("session-%d-%d", n, i) // Unique per benchmark iteration
            sessionStart := startTime.Add(time.Duration(i) * time.Minute)
            for j := 0; j < executionsPerSession; j++ {
                callID := fmt.Sprintf("%s-%d", sessionID, j)
                runtimeMs := int64((j + 1) * 1000)
                created := sessionStart.Add(time.Duration(j) * time.Second)
                _, err := stmt.Exec(callID, sessionID, "bench-tool", "res", 1, 100, 100, runtimeMs, created)
                if err != nil {
                    b.Fatalf("Insert failed: %v", err)
                }
            }
        }
        stmt.Close()
        tx.Commit()

        b.StartTimer()
        // Target 0.5 hours (initial ~1.1 hours)
        _, err := ts.CleanupByRuntimeBudget(0.5)
        if err != nil {
            b.Fatalf("Cleanup failed: %v", err)
        }
    }
}
