package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Remediation for: TEST_GAP: Null/Empty Inputs
// QA: 2026-05-04_04-07-20-EST_transaction_manager_boundary_analysis.md
// ============================================================================

// TestTransactionManagerGap_Begin_EmptyDescription verifies that Begin
// accepts or rejects an empty description string. Currently the system
// accepts it (producing "aborted: " on abort). We verify it doesn't panic
// and the transaction is usable.
func TestTransactionManagerGap_Begin_EmptyDescription(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}
	tm := NewTransactionManager(kernel, tmpDir)

	txn, err := tm.Begin(context.Background(), "")
	if err != nil {
		t.Fatalf("Begin with empty description should not error, got: %v", err)
	}
	if txn == nil {
		t.Fatal("Expected non-nil transaction")
	}
	if txn.Description != "" {
		t.Errorf("Expected empty description, got %q", txn.Description)
	}
	// Verify it's usable — abort should work
	err = tm.Abort(context.Background(), "cleanup")
	if err != nil {
		t.Fatalf("Abort after empty-desc Begin failed: %v", err)
	}
}

// TestTransactionManagerGap_AddEdit_EmptyFilePath verifies that AddEdit
// with an empty FilePath returns an error for EditTypeModify (since os.ReadFile("")
// will fail), and doesn't panic for EditTypeCreate.
func TestTransactionManagerGap_AddEdit_EmptyFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "empty path test")
	if err != nil {
		t.Fatal(err)
	}

	// EditTypeModify with empty path should fail on snapshot (os.ReadFile(""))
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: "",
		Content:  []byte("content"),
		EditType: EditTypeModify,
	})
	if err == nil {
		t.Error("Expected error for AddEdit with empty FilePath and EditTypeModify")
	}

	// EditTypeCreate with empty path should succeed (no snapshot needed)
	// but the path is logically invalid — the system currently accepts it
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: "",
		Content:  []byte("content"),
		EditType: EditTypeCreate,
	})
	// We document that this succeeds without panic; the failure happens at Commit time
	if err != nil {
		t.Logf("AddEdit(create) with empty path returned error (acceptable): %v", err)
	}
}

// TestTransactionManagerGap_AddEdit_NilContent verifies behavior when
// Content is nil for EditTypeCreate and EditTypeModify.
func TestTransactionManagerGap_AddEdit_NilContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file to modify
	testFile := filepath.Join(tmpDir, "exists.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "nil content test")
	if err != nil {
		t.Fatal(err)
	}

	// nil content for create — accepted, results in 0-byte file at commit
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: filepath.Join(tmpDir, "nil_create.go"),
		Content:  nil,
		EditType: EditTypeCreate,
	})
	if err != nil {
		t.Fatalf("AddEdit(create) with nil content should not error: %v", err)
	}

	// nil content for modify — accepted, will write 0-byte file
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		Content:  nil,
		EditType: EditTypeModify,
	})
	if err != nil {
		t.Fatalf("AddEdit(modify) with nil content should not error: %v", err)
	}

	// Verify edits were tracked
	txn, _ := tm.GetActiveTransaction()
	if len(txn.Edits) != 2 {
		t.Errorf("Expected 2 edits, got %d", len(txn.Edits))
	}
}

// TestTransactionManagerGap_Operations_NoActiveTransaction verifies that
// Prepare, Commit, and Abort all return clean errors when no transaction is active.
func TestTransactionManagerGap_Operations_NoActiveTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}
	tm := NewTransactionManager(kernel, tmpDir)

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Prepare", func() error { _, err := tm.Prepare(context.Background()); return err }},
		{"Commit", func() error { return tm.Commit(context.Background()) }},
		{"Abort", func() error { return tm.Abort(context.Background(), "no txn") }},
		{"AddEdit", func() error {
			return tm.AddEdit(context.Background(), FileEdit{FilePath: "test.go", EditType: EditTypeCreate})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s without active transaction should return error", tt.name)
			}
			if !strings.Contains(err.Error(), "no active transaction") {
				t.Errorf("Expected 'no active transaction' error, got: %v", err)
			}
		})
	}
}

// ============================================================================
// Remediation for: TEST_GAP: State Conflicts / Invalid Transitions
// ============================================================================

// TestTransactionManagerGap_InvalidStateTransitions verifies that methods
// reject calls when the transaction is in an incompatible state.
func TestTransactionManagerGap_InvalidStateTransitions(t *testing.T) {
	t.Run("Begin_WhenAlreadyActive", func(t *testing.T) {
		tmpDir := t.TempDir()
		kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
		tm := NewTransactionManager(kernel, tmpDir)

		_, err := tm.Begin(context.Background(), "first")
		if err != nil {
			t.Fatal(err)
		}

		_, err = tm.Begin(context.Background(), "second")
		if err == nil {
			t.Error("Expected error when beginning second transaction")
		}
		if !strings.Contains(err.Error(), "transaction already active") {
			t.Errorf("Expected 'transaction already active' error, got: %v", err)
		}
	})

	t.Run("Commit_WhenNotReady", func(t *testing.T) {
		tmpDir := t.TempDir()
		kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
		tm := NewTransactionManager(kernel, tmpDir)

		_, err := tm.Begin(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}

		// Transaction is in Pending state, not Ready — Commit should fail
		err = tm.Commit(context.Background())
		if err == nil {
			t.Error("Expected error when committing non-ready transaction")
		}
		if !strings.Contains(err.Error(), "not ready to commit") {
			t.Errorf("Expected 'not ready to commit' error, got: %v", err)
		}
	})

	t.Run("Abort_WhenCommitted", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.go")
		os.WriteFile(testFile, []byte("package main\n"), 0644)

		kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
		tm := NewTransactionManager(kernel, tmpDir)

		_, err := tm.Begin(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}

		// Add an edit and manually set state to Ready to enable commit
		tm.AddEdit(context.Background(), FileEdit{
			FilePath: testFile,
			Content:  []byte("package main\n// modified\n"),
			EditType: EditTypeModify,
		})

		// Manually transition to Ready (bypassing Prepare which needs ShadowMode)
		tm.mu.Lock()
		txn := tm.txns[tm.activeTxnID]
		txn.Status = TxnStatusReady
		tm.mu.Unlock()

		// Commit
		err = tm.Commit(context.Background())
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Now try to abort the committed transaction — should fail
		// Note: activeTxnID is cleared after commit, so Abort says "no active transaction"
		err = tm.Abort(context.Background(), "too late")
		if err == nil {
			t.Error("Expected error when aborting after commit")
		}
	})

	t.Run("AddEdit_WhenNotPending", func(t *testing.T) {
		tmpDir := t.TempDir()
		kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
		tm := NewTransactionManager(kernel, tmpDir)

		_, err := tm.Begin(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}

		// Manually transition to Preparing
		tm.mu.Lock()
		txn := tm.txns[tm.activeTxnID]
		txn.Status = TxnStatusPreparing
		tm.mu.Unlock()

		err = tm.AddEdit(context.Background(), FileEdit{
			FilePath: filepath.Join(tmpDir, "new.go"),
			Content:  []byte("package main\n"),
			EditType: EditTypeCreate,
		})
		if err == nil {
			t.Error("Expected error when adding edit to non-pending transaction")
		}
		if !strings.Contains(err.Error(), "not in pending state") {
			t.Errorf("Expected 'not in pending state' error, got: %v", err)
		}
	})

	t.Run("Prepare_WhenNotPending", func(t *testing.T) {
		tmpDir := t.TempDir()
		kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
		tm := NewTransactionManager(kernel, tmpDir)

		_, err := tm.Begin(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}

		// Manually transition to Ready
		tm.mu.Lock()
		txn := tm.txns[tm.activeTxnID]
		txn.Status = TxnStatusReady
		tm.mu.Unlock()

		_, err = tm.Prepare(context.Background())
		if err == nil {
			t.Error("Expected error when preparing already-ready transaction")
		}
		if !strings.Contains(err.Error(), "not in pending state") {
			t.Errorf("Expected 'not in pending state' error, got: %v", err)
		}
	})
}

// ============================================================================
// Remediation for: TEST_GAP: Rollback Resilience and System Errors
// ============================================================================

// TestTransactionManagerGap_Commit_RollbackOnWriteFailure verifies that
// when Commit fails partway through writing files (e.g., read-only directory),
// the rollback function restores previously committed files.
func TestTransactionManagerGap_Commit_RollbackOnWriteFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	original1 := []byte("package main\n// original1\n")
	original2 := []byte("package main\n// original2\n")
	os.WriteFile(file1, original1, 0644)
	os.WriteFile(file2, original2, 0644)

	// Create a read-only directory for the third edit to fail
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readOnlyDir, 0755)
	failFile := filepath.Join(readOnlyDir, "sub", "fail.go")

	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "rollback test")
	if err != nil {
		t.Fatal(err)
	}

	// Add edits — first two will succeed, third will fail
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: file1, Content: []byte("package main\n// modified1\n"), EditType: EditTypeModify,
	})
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: file2, Content: []byte("package main\n// modified2\n"), EditType: EditTypeModify,
	})
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: failFile, Content: []byte("package main\n"), EditType: EditTypeCreate,
	})

	// Manually set to Ready (bypassing ShadowMode)
	tm.mu.Lock()
	txn := tm.txns[tm.activeTxnID]
	txn.Status = TxnStatusReady
	tm.mu.Unlock()

	// Make the readonly dir actually read-only AFTER adding edits
	// On Windows, os.Chmod is limited, so we use a different approach:
	// Remove the directory entirely so MkdirAll fails for the nested path
	os.RemoveAll(readOnlyDir)
	// Create a FILE at readOnlyDir path so MkdirAll("readonly/sub") fails
	os.WriteFile(readOnlyDir, []byte("not a directory"), 0644)

	// Commit should fail on the third edit
	err = tm.Commit(context.Background())
	if err == nil {
		t.Error("Expected Commit to fail on the third edit")
	} else {
		t.Logf("Commit correctly failed: %v", err)
	}

	// Verify file1 was rolled back to original
	content1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("Failed to read file1 after rollback: %v", err)
	}
	if string(content1) != string(original1) {
		t.Errorf("file1 not rolled back.\n  Expected: %q\n  Got: %q", string(original1), string(content1))
	}

	// Verify file2 was rolled back to original
	content2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("Failed to read file2 after rollback: %v", err)
	}
	if string(content2) != string(original2) {
		t.Errorf("file2 not rolled back.\n  Expected: %q\n  Got: %q", string(original2), string(content2))
	}
}

// TestTransactionManagerGap_ExternalModification_HashMismatch verifies that
// Prepare detects when a file has been modified externally since the snapshot
// was taken (TOCTOU vulnerability).
func TestTransactionManagerGap_ExternalModification_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "target.go")
	original := []byte("package main\n// v1\n")
	os.WriteFile(testFile, original, 0644)

	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "conflict test")
	if err != nil {
		t.Fatal(err)
	}

	// Add edit WITH OldHash set (enables conflict detection)
	originalHash := computeHash(original)
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		Content:  []byte("package main\n// v2\n"),
		EditType: EditTypeModify,
		OldHash:  originalHash,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Externally modify the file AFTER snapshot
	os.WriteFile(testFile, []byte("package main\n// externally modified\n"), 0644)

	// Prepare should detect the hash mismatch
	result, err := tm.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare returned error (should return result with IsValid=false): %v", err)
	}

	if result.IsValid {
		t.Error("Expected IsValid=false due to external file modification")
	}

	foundConflict := false
	for _, block := range result.SafetyBlocks {
		if block.Reason == "file_modified_externally" {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Error("Expected a SafetyBlock with reason 'file_modified_externally'")
	}
}

// ============================================================================
// Remediation for: TEST_GAP: Type Coercion / Unknown Values
// ============================================================================

// TestTransactionManagerGap_AddEdit_InvalidEditType verifies behavior when
// an unknown EditType is used. The system currently accepts it during AddEdit
// but silently skips it during Commit.
func TestTransactionManagerGap_AddEdit_InvalidEditType(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "invalid type test")
	if err != nil {
		t.Fatal(err)
	}

	// Use an unknown EditType
	unknownType := EditType("teleport")
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: filepath.Join(tmpDir, "ghost.go"),
		Content:  []byte("package main\n"),
		EditType: unknownType,
	})
	// AddEdit might succeed (no EditType validation) or fail on snapshot
	// The key assertion is: no panic occurs
	t.Logf("AddEdit with unknown EditType returned: %v", err)

	if err == nil {
		// If AddEdit succeeded, verify the edit was added
		txn, _ := tm.GetActiveTransaction()
		if len(txn.Edits) == 0 {
			t.Error("Edit should be added even with unknown type")
		}

		// Now test Commit — it should skip the unknown type silently
		tm.mu.Lock()
		txnInner := tm.txns[tm.activeTxnID]
		txnInner.Status = TxnStatusReady
		tm.mu.Unlock()

		err = tm.Commit(context.Background())
		if err != nil {
			t.Logf("Commit with unknown EditType: %v (acceptable)", err)
		}

		// The file should NOT exist since EditType("teleport") is not handled
		if _, statErr := os.Stat(filepath.Join(tmpDir, "ghost.go")); statErr == nil {
			t.Error("Unknown EditType should not create a file on disk")
		}
	}
}

// ============================================================================
// Remediation for: TEST_GAP: User Request Extremes
// (Massive file and thousands of edits are deferred as unsafe for CI;
//  we test reasonable bounds instead)
// ============================================================================

// TestTransactionManagerGap_ManyEdits_Stress verifies the transaction system
// handles a moderate number of edits (500) without panicking or corrupting state.
func TestTransactionManagerGap_ManyEdits_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	tmpDir := t.TempDir()
	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "stress test")
	if err != nil {
		t.Fatal(err)
	}

	const editCount = 500
	for i := range editCount {
		filePath := filepath.Join(tmpDir, "gen", "file_"+strings.Replace(
			time.Now().Format("150405.000000"), ".", "_", -1)+"_"+
			strings.Repeat("x", 5)+".go")
		// Use unique paths
		filePath = filepath.Join(tmpDir, "gen", filepath.Base(
			filepath.Join(tmpDir, "gen", "file_"+string(rune('a'+i%26))+
				"_"+strings.Repeat("x", i%10)+".go")))

		err = tm.AddEdit(context.Background(), FileEdit{
			FilePath: filePath,
			Content:  []byte("package gen\n"),
			EditType: EditTypeCreate,
		})
		if err != nil {
			t.Fatalf("AddEdit %d failed: %v", i, err)
		}
	}

	txn, _ := tm.GetActiveTransaction()
	if len(txn.Edits) != editCount {
		t.Errorf("Expected %d edits, got %d", editCount, len(txn.Edits))
	}

	// Verify ToFacts handles large edit count without panic
	facts := tm.ToFacts()
	planEditCount := 0
	for _, f := range facts {
		if f.Predicate == "plan_edit" {
			planEditCount++
		}
	}
	if planEditCount != editCount {
		t.Errorf("Expected %d plan_edit facts, got %d", editCount, planEditCount)
	}
}

// ============================================================================
// Remediation for: TEST_GAP: State Conflicts — Context cancellation
// ============================================================================

// TestTransactionManagerGap_Prepare_ContextCancellation verifies that
// Prepare respects context cancellation. The current implementation
// treats SimulateAction errors as warnings, which is a known bug.
// This test documents the behavior.
func TestTransactionManagerGap_Prepare_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cancel_test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "cancel test")
	if err != nil {
		t.Fatal(err)
	}

	tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		Content:  []byte("package main\n// modified\n"),
		EditType: EditTypeModify,
	})

	// Cancel the context before Prepare
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	result, err := tm.Prepare(ctx)
	// The system may return an error from StartSimulation if context is cancelled
	// or it may proceed and surface warnings
	if err != nil {
		t.Logf("Prepare correctly propagated context cancellation: %v", err)
		// Verify transaction was aborted
		txn, _ := tm.GetActiveTransaction()
		if txn != nil && txn.Status != TxnStatusAborted {
			t.Errorf("Expected TxnStatusAborted after context cancellation, got %s", txn.Status)
		}
		return
	}

	// Verify that if a result is returned, it should not happen when context is canceled
	if result != nil {
		t.Errorf("Expected Prepare to fail with context cancellation, but it returned a result")
	}
}

// ============================================================================
// Remediation for: TEST_GAP: State Conflicts — Concurrency
// ============================================================================

// TestTransactionManagerGap_Concurrency_InterleavedOperations verifies
// thread safety when multiple goroutines call different TM methods concurrently.
func TestTransactionManagerGap_Concurrency_InterleavedOperations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	for i := range 10 {
		f := filepath.Join(tmpDir, filepath.Base(filepath.Join(tmpDir, "file_"+string(rune('a'+i))+".go")))
		os.WriteFile(f, []byte("package main\n"), 0644)
	}

	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "concurrency test")
	if err != nil {
		t.Fatal(err)
	}

	// Seed with one edit
	seedFile := filepath.Join(tmpDir, "file_a.go")
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: seedFile,
		Content:  []byte("package main\n// seed\n"),
		EditType: EditTypeModify,
	})

	var wg sync.WaitGroup
	const goroutines = 50

	// Launch readers
	for range goroutines {
		wg.Go(func() {
			_ = tm.IsTransactionActive()
			_, _ = tm.GetActiveTransaction()
			_ = tm.ToFacts()
		})
	}

	// Launch writers (AddEdit for create operations — no snapshot needed)
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newFile := filepath.Join(tmpDir, "concurrent_"+string(rune('a'+idx))+".go")
			_ = tm.AddEdit(context.Background(), FileEdit{
				FilePath: newFile,
				Content:  []byte("package main\n"),
				EditType: EditTypeCreate,
			})
		}(i)
	}

	wg.Wait()

	// No panics or data races should have occurred (run with -race to verify)
	txn, _ := tm.GetActiveTransaction()
	if txn == nil {
		t.Fatal("Transaction should still be active")
	}
	t.Logf("Total edits after concurrent operations: %d", len(txn.Edits))
}

// TestTransactionManagerGap_Concurrency_MultipleBeginAttempts verifies
// that concurrent Begin calls are serialized correctly by the mutex.
func TestTransactionManagerGap_Concurrency_MultipleBeginAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, tmpDir)

	const attempts = 20
	results := make(chan error, attempts)

	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := tm.Begin(context.Background(), "attempt_"+string(rune('a'+idx)))
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	errorCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful Begin, got %d", successCount)
	}
	if errorCount != attempts-1 {
		t.Errorf("Expected %d rejected Begin attempts, got %d", attempts-1, errorCount)
	}
}
