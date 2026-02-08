# QA Journal: Boundary Value Analysis of Knowledge Graph (Shard C)

**Date:** 2026-02-08
**Time:** 00:15 EST
**Author:** QA Automation Engineer (Jules)
**Target System:** `internal/store/local_graph.go` (Knowledge Graph / Shard C)

## Executive Summary

This journal documents a deep-dive boundary value analysis and negative testing review of the `internal/store/local_graph.go` subsystem. This module implements a local knowledge graph using SQLite as the backing store, providing `StoreLink`, `QueryLinks`, and `TraversePath` capabilities.

The analysis focused on four key vectors:
1.  **Null/Undefined/Empty Inputs**
2.  **Type Coercion & Data Integrity**
3.  **User Extremes (Scale & Depth)**
4.  **State Conflicts (Concurrency & Deadlocks)**

**Critical Finding:** A potential **Deadlock** condition exists in `TraversePath` due to nested `RLock` acquisition in the presence of pending writers. This is a high-severity architectural flaw that can freeze the entire knowledge graph subsystem under concurrent load.

---

## 1. System Overview

The `LocalStore` struct manages a knowledge graph where entities are nodes and relations are weighted edges with metadata.
-   **Storage**: SQLite table `knowledge_graph`.
-   **Concurrency**: `sync.RWMutex` protects access.
-   **Operations**: Insert/Replace links, Query by direction, BFS Traversal.

## 2. Boundary Value Analysis

### 2.1 Null/Undefined/Empty Inputs

#### Scenario 1: Empty Strings for Entities
**Code Path:** `StoreLink(entityA, relation, entityB, ...)`
**Current Behavior:**
The code executes `INSERT OR REPLACE` with empty strings. SQLite allows empty strings.
**Risk:**
-   The graph becomes polluted with "ghost" nodes (empty string keys).
-   `QueryLinks("", ...)` returns these ghost links, potentially linking unrelated parts of the graph via the "empty" node.
-   **Recommendation:** Explicitly validate `entityA`, `entityB`, and `relation` are non-empty.

#### Scenario 2: Nil Metadata
**Code Path:** `StoreLink(..., metadata=nil)`
**Current Behavior:**
`json.Marshal(nil)` returns `null` (literal). SQLite stores string `"null"`.
**Risk:**
-   `QueryLinks` reads `"null"`. `json.Unmarshal([]byte("null"), &link.Metadata)` works fine (sets nil).
-   **Verdict:** Safe, but relies on JSON behavior.

#### Scenario 3: Empty Relation
**Code Path:** `StoreLink(..., relation="")`
**Current Behavior:**
Allowed.
**Risk:**
-   Semantic ambiguity. Is an empty relation a "default" relation?
-   **Recommendation:** Enforce named relations.

### 2.2 Type Coercion & Data Integrity

#### Scenario 4: NaN / Infinity Weights
**Code Path:** `StoreLink(..., weight=NaN)`
**Current Behavior:**
SQLite `REAL` type supports IEEE 754 floats. Go `float64` supports `NaN`.
**Risk:**
-   `ORDER BY weight DESC` in `HydrateKnowledgeGraph` behavior with `NaN` is undefined or implementation-specific (usually `NaN` is smallest or largest).
-   Logic relying on `weight > threshold` might fail unpredictably.
-   **Recommendation:** Validate `!math.IsNaN(weight)` and `!math.IsInf(weight, 0)`.

#### Scenario 5: Metadata Cyclic References
**Code Path:** `StoreLink(..., metadata=cyclicMap)`
**Current Behavior:**
`json.Marshal(metadata)` returns error.
**The Code Ignores the Error:** `metaJSON, _ := json.Marshal(metadata)`
**Risk:**
-   `metaJSON` is likely empty string `""` or partial.
-   SQLite stores empty string/partial.
-   `QueryLinks` reads it. `json.Unmarshal` on empty string fails?
-   `QueryLinks` loop:
    ```go
    if metaJSON != "" {
        json.Unmarshal([]byte(metaJSON), &link.Metadata)
    }
    ```
-   If `metaJSON` is empty, metadata is nil. **Data Loss**.
-   **Verdict:** **BUG**. The system silently drops metadata if serialization fails.

#### Scenario 6: Malformed Metadata in DB
**Code Path:** `QueryLinks` reading corrupted JSON.
**Current Behavior:**
`json.Unmarshal` returns error. The error is **ignored**.
```go
if metaJSON != "" {
    json.Unmarshal([]byte(metaJSON), &link.Metadata)
}
```
**Risk:**
-   The link is returned with `Metadata: nil`, even if the DB has data.
-   **Verdict:** **Silent Failure**. This makes debugging corrupted state impossible.

### 2.3 User Extremes

#### Scenario 7: Massive Graph Traversal
**Code Path:** `TraversePath`
**Current Behavior:**
-   `visited` map grows linearly with nodes visited.
-   **Locking:** `s.mu.RLock()` is held for the **entire duration**.
**Risk:**
-   If `maxDepth` is large (e.g., 100) and branching factor is high, traversal takes seconds/minutes.
-   **ALL WRITERS BLOCKED**. `StoreLink` cannot proceed.
-   System grinding to a halt.
-   **Recommendation:** Use fine-grained locking or snapshot isolation (SQLite MVCC). Don't hold Go mutex for the whole traversal.

#### Scenario 8: Max Depth Logic
**Code Path:** `TraversePath(..., maxDepth=0)`
**Current Behavior:**
Defaults to 5.
**Risk:**
-   User might *want* 0 depth (just check existence?).
-   Negative depth defaults to 5.
-   **Verdict:** Acceptable default, but maybe explicit error for negative is better.

### 2.4 State Conflicts (CRITICAL)

#### Scenario 9: The Deadlock
**Code Path:** `TraversePath` calling `QueryLinks`.
**Analysis:**
1.  `TraversePath` acquires `s.mu.RLock()`.
2.  It loops. inside the loop: `links, err := s.QueryLinks(current.entity, "outgoing")`.
3.  `QueryLinks` acquires `s.mu.RLock()`.

**The Conflict:**
Go's `sync.RWMutex` implementation:
> "If a goroutine holds a RWMutex for reading and another goroutine might call Lock, no goroutine should expect to be able to acquire a read lock until the initial read lock is released."

**Sequence:**
1.  **Goroutine A (Reader)**: Calls `TraversePath`. Acquires `RLock`.
    -   Holds `RLock`.
    -   Doing work...
2.  **Goroutine B (Writer)**: Calls `StoreLink`. Calls `Lock`.
    -   `Lock` blocks because Reader A holds `RLock`.
    -   `RWMutex` internals mark "writer waiting" to prevent reader starvation. New readers are now blocked.
3.  **Goroutine A (Reader)**: Calls `QueryLinks`.
    -   `QueryLinks` calls `RLock`.
    -   `RLock` sees "writer waiting" (Goroutine B).
    -   `RLock` blocks to let writer proceed.
4.  **DEADLOCK**.
    -   A is waiting for B (indirectly, via RWMutex policy).
    -   B is waiting for A (directly, for RUnlock).

**Impact:**
-   This will freeze the Knowledge Graph subsystem completely.
-   Any subsequent request (Read or Write) will pile up.
-   **Verdict:** **CRITICAL ARCHITECTURAL DEFECT**.

**Fix:**
-   `TraversePath` should *not* call public `QueryLinks`.
-   It should call an internal `queryLinksLocked` (no lock) helper.
-   Or `TraversePath` should not hold the lock across the loop (but then consistency issues arise).
-   Best approach: `queryLinksInternal` which assumes lock is held, and `QueryLinks` which acquires it.

---

## 3. Negative Test Plan

The following tests must be added to `internal/store/local_graph_test.go` to cover these gaps.

### 3.1 Test: Deadlock Reproduction
**Objective:** Prove the nested RLock deadlock.
**Setup:**
-   Start `TraversePath` in G1.
-   Pause G1 inside the loop (via hook or large graph).
-   Start `StoreLink` in G2.
-   G1 attempts `QueryLinks`.
**Expectation:** Deadlock (timeout).

### 3.2 Test: Data Integrity (JSON)
**Objective:** Verify silent failure of metadata.
**Setup:**
-   `StoreLink` with cyclic map.
-   Assert `QueryLinks` returns nil metadata (or verify DB has empty string).
**Expectation:** Should error, but currently fails silently.

### 3.3 Test: Empty Entity Injection
**Objective:** Verify "ghost" nodes.
**Setup:**
-   `StoreLink("", "rel", "B")`.
-   `QueryLinks("")`.
**Expectation:** Should fail validation, currently succeeds.

### 3.4 Test: NaN Weights
**Objective:** Verify behavior of invalid weights.
**Setup:**
-   `StoreLink(..., NaN)`.
-   `HydrateKnowledgeGraph` sort order.
**Expectation:** Undefined behavior / explicit failure needed.

---

## 4. Detailed Gap Analysis for `internal/store/local_graph.go`

| Vector | Edge Case | Severity | Current Status | Recommendation |
| :--- | :--- | :--- | :--- | :--- |
| **State** | **Nested RLock Deadlock** | **CRITICAL** | **Present** | Refactor to use internal helpers without locking. |
| **Integrity** | Ignored JSON Errors (Write) | High | Present | Return error on `json.Marshal` failure. |
| **Integrity** | Ignored JSON Errors (Read) | Medium | Present | Return error or log warning on `json.Unmarshal` failure. |
| **Input** | Empty Entity Strings | Medium | Present | Add validation logic. |
| **Input** | NaN/Inf Weights | Low | Present | Add validation logic. |
| **Performance** | Long-held Read Lock | High | Present | Snapshot isolation or batched locking. |

## 5. Proposed Fixes (Pseudocode)

### Fix for Deadlock

```go
// Internal helper, assumes lock held
func (s *LocalStore) queryLinksInternal(entity, direction string) ([]KnowledgeLink, error) {
    // ... SQL query execution ...
    // No locking here!
}

// Public method
func (s *LocalStore) QueryLinks(entity, direction string) ([]KnowledgeLink, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.queryLinksInternal(entity, direction)
}

// TraversePath
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // ...
    // Call internal helper
    links, err := s.queryLinksInternal(current.entity, "outgoing")
    // ...
}
```

### Fix for JSON Errors

```go
func (s *LocalStore) StoreLink(...) error {
    metaJSON, err := json.Marshal(metadata)
    if err != nil {
        return fmt.Errorf("failed to marshal metadata: %w", err)
    }
    // ...
}
```

## 6. Conclusion

The `internal/store/local_graph.go` module is functionally correct for the "happy path" but contains a critical concurrency flaw (Deadlock) and significant data integrity issues regarding metadata handling. The lack of input validation for entities and weights further reduces robustness. Immediate refactoring of the locking strategy is required to prevent system freeze under load.

---

**Next Steps:**
1.  Annotate `internal/store/local_graph_test.go` with `TEST_GAP` comments.
2.  Prioritize the Deadlock fix in the next sprint.
3.  Implement validation logic.

---

# Appendix A: Proof of Concept (Deadlock Reproduction)

This section provides the complete Go test code to reproduce the deadlock condition identified in Scenario 9.

```go
func TestTraversePathDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deadlock test in short mode")
	}

	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Seed data
	store.StoreLink("A", "next", "B", 1.0, nil)

	// Channel to signal that TraversePath has started and acquired the lock
	traversalStarted := make(chan struct{})
	// Channel to signal that the Writer is ready
	writerReady := make(chan struct{})

	// 1. Start a "slow" traversal that blocks inside the read lock
	go func() {
		// Note: In a real test, we might need to inject a hook or use a massive graph
		// to slow down TraversePath. Here we rely on timing.
		// Alternatively, we can mock the internal DB or use a debug hook.
		// For this POC, we simulate the logic:
		store.mu.RLock()
		defer store.mu.RUnlock()

		traversalStarted <- struct{}{}

		// Simulate work inside the lock
		time.Sleep(100 * time.Millisecond)

		// Wait for writer to be ready (waiting on Lock)
		<-writerReady

		// Attempt nested RLock (simulating call to QueryLinks)
		// This should BLOCK forever if RWMutex prioritization is active
		// store.QueryLinks("A", "outgoing") // This calls RLock()

		// Direct simulation of nested RLock:
		store.mu.RLock()
		store.mu.RUnlock()
	}()

	<-traversalStarted

	// 2. Start a Writer that attempts to acquire Lock
	// This will block because Reader holds RLock
	// RWMutex will set writerWaiting = true
	done := make(chan struct{})
	go func() {
		writerReady <- struct{}{} // Signal reader to proceed
		store.mu.Lock()
		defer store.mu.Unlock()
		store.StoreLink("C", "next", "D", 1.0, nil)
		close(done)
	}()

	// 3. Wait for completion or timeout
	select {
	case <-done:
		t.Log("No deadlock occurred (Writer succeeded)")
	case <-time.After(2 * time.Second):
		t.Fatal("Deadlock detected! Writer blocked Reader's nested RLock acquisition.")
	}
}
```

# Appendix B: Recommended Fix Implementation

This section provides the refactored code structure to eliminate the deadlock and improve robustness.

```go
// Internal helper: Execute query without locking
// PRECONDITION: Caller must hold s.mu.RLock()
func (s *LocalStore) queryLinksInternal(entity, direction string) ([]KnowledgeLink, error) {
	var query string
	switch direction {
	case "outgoing":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ?"
	case "incoming":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_b = ?"
	default:
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ? OR entity_b = ?"
	}

	var args []interface{}
	if direction == "both" {
		args = []interface{}{entity, entity}
	} else {
		args = []interface{}{entity}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []KnowledgeLink
	for rows.Next() {
		var link KnowledgeLink
		var metaJSON string
		if err := rows.Scan(&link.EntityA, &link.Relation, &link.EntityB, &link.Weight, &metaJSON); err != nil {
			// Log warning but continue? Or fail?
			logging.Get(logging.CategoryStore).Warn("Row scan failed: %v", err)
			continue
		}
		if metaJSON != "" {
			if err := json.Unmarshal([]byte(metaJSON), &link.Metadata); err != nil {
				// Log warning for data corruption
				logging.Get(logging.CategoryStore).Warn("Metadata unmarshal failed: %v", err)
			}
		}
		links = append(links, link)
	}
	return links, nil
}

// Public API: Acquires Lock
func (s *LocalStore) QueryLinks(entity, direction string) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "QueryLinks")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.queryLinksInternal(entity, direction)
}

// TraversePath: Acquires Lock once
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "TraversePath")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// ... BFS logic ...

	// INSIDE LOOP:
	// Call internal method directly to avoid nested RLock
	links, err := s.queryLinksInternal(current.entity, "outgoing")

	// ...
}
```

# Appendix C: Fuzzing Strategy

Given the complexity of graph structures and the potential for input edge cases, property-based testing (fuzzing) is highly recommended.

**Target Properties:**
1.  **Graph Consistency**: If `StoreLink(A->B)` succeeds, `QueryLinks(A)` must contain B.
2.  **No Deadlock**: Concurrent random operations (Store, Query, Traverse) must never hang.
3.  **Crash Safety**: Random strings, nil maps, and NaN weights should never panic the process.

**Fuzzing Implementation Plan:**
1.  Use `google/gofuzz` or Go 1.18+ native fuzzing.
2.  **FuzzStoreLink**:
    -   Generate random UTF-8 strings for entities.
    -   Generate random floats for weights (including NaN, Inf).
    -   Generate deep nested maps for metadata.
3.  **FuzzConcurrency**:
    -   Spawn N goroutines.
    -   Randomly pick an operation (Store/Query/Traverse).
    -   Run for fixed duration (e.g., 5s).
    -   Detect hangs via `test -timeout`.

**Sample Fuzz Test Stub:**

```go
func FuzzStoreLink(f *testing.F) {
	f.Add("entityA", "rel", "entityB", 1.0)
	f.Fuzz(func(t *testing.T, a, rel, b string, w float64) {
		store, _ := NewLocalStore(":memory:")
		defer store.Close()

		err := store.StoreLink(a, rel, b, w, nil)
		if err != nil {
			// Check if error is expected (validation) or unexpected (crash/internal)
		}

		// Invariant check
		if err == nil {
			links, _ := store.QueryLinks(a, "outgoing")
			found := false
			for _, l := range links {
				if l.EntityB == b { found = true }
			}
			if !found && a != "" {
				t.Errorf("Stored link not found")
			}
		}
	})
}
```

# Appendix D: Security Implications

The lack of input validation on `StoreLink` has subtle security implications beyond simple bugs.

**Ghost Node Injection:**
-   **Attack Vector:** An attacker with control over entity names (e.g., via a "create resource" API that maps to a graph node) injects an empty string `""` or a control character string as an entity name.
-   **Impact:** If logic depends on graph traversal for authorization (e.g., "User can access Resource if path exists"), injection of a common "ghost node" could short-circuit these checks.
-   **Example:** If `Admin -> *` and `Attacker -> ""` and `"" -> Target`, traverse might find `Attacker -> "" -> Target` if `""` is mishandled as a wildcard or default.

**Denial of Service (DoS):**
-   **Attack Vector:** Triggering `TraversePath` on a highly connected graph node (or a loop).
-   **Impact:** Because `TraversePath` holds the global read lock, a single expensive traversal blocks all write operations. An attacker can repeatedly trigger traversals to starve the system of updates (e.g., preventing revocation of credentials or logging of events).

**Recommendations:**
1.  **Strict Allowlisting:** Only allow alphanumeric + specific punctuation for entity IDs.
2.  **Depth/Cost Limits:** Enforce a strict "gas limit" (nodes visited count) for traversal, not just depth.
3.  **Timeouts:** Enforce context-based timeouts on all DB locks.

End of Journal (Revised).
