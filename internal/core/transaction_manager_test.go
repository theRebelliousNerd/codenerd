package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestTransactionManager_Begin tests starting a new transaction.
func TestTransactionManager_Begin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal kernel for testing
	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Begin a transaction
	txn, err := tm.Begin(context.Background(), "Test transaction")
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if txn == nil {
		t.Fatal("Expected non-nil transaction")
	}

	if txn.ID == "" {
		t.Error("Expected transaction ID to be set")
	}

	if txn.Description != "Test transaction" {
		t.Errorf("Expected description 'Test transaction', got %q", txn.Description)
	}

	if txn.Status != TxnStatusPending {
		t.Errorf("Expected status %s, got %s", TxnStatusPending, txn.Status)
	}

	// Should not be able to begin another transaction
	_, err = tm.Begin(context.Background(), "Second transaction")
	if err == nil {
		t.Error("Expected error when beginning second transaction")
	}
}

// TestTransactionManager_AddEdit tests adding edits to a transaction.
func TestTransactionManager_AddEdit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file to modify
	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := []byte("package test\n\nfunc Original() {}\n")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Try adding edit without transaction
	err := tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		Content:  []byte("new content"),
		EditType: EditTypeModify,
	})
	if err == nil {
		t.Error("Expected error when adding edit without transaction")
	}

	// Begin transaction
	_, err = tm.Begin(context.Background(), "Test edits")
	if err != nil {
		t.Fatal(err)
	}

	// Add a modify edit
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		Content:  []byte("package test\n\nfunc Modified() {}\n"),
		EditType: EditTypeModify,
	})
	if err != nil {
		t.Fatalf("AddEdit failed: %v", err)
	}

	// Add a create edit
	newFile := filepath.Join(tmpDir, "new.go")
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: newFile,
		Content:  []byte("package test\n\nfunc New() {}\n"),
		EditType: EditTypeCreate,
	})
	if err != nil {
		t.Fatalf("AddEdit (create) failed: %v", err)
	}

	// Verify edits were added
	txn, _ := tm.GetActiveTransaction()
	if len(txn.Edits) != 2 {
		t.Errorf("Expected 2 edits, got %d", len(txn.Edits))
	}

	// Verify snapshot was taken
	if _, exists := txn.Snapshots[testFile]; !exists {
		t.Error("Expected snapshot of modified file")
	}
}

// TestTransactionManager_Abort tests aborting a transaction.
func TestTransactionManager_Abort(t *testing.T) {
	tmpDir := t.TempDir()

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Begin transaction
	txn, err := tm.Begin(context.Background(), "Test abort")
	if err != nil {
		t.Fatal(err)
	}

	txnID := txn.ID

	// Abort the transaction
	err = tm.Abort(context.Background(), "User cancelled")
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	// Verify transaction was aborted
	abortedTxn, exists := tm.GetTransaction(txnID)
	if !exists {
		t.Error("Expected transaction to still exist after abort")
	}

	if abortedTxn.Status != TxnStatusAborted {
		t.Errorf("Expected status %s, got %s", TxnStatusAborted, abortedTxn.Status)
	}

	// Should be able to begin a new transaction now
	_, err = tm.Begin(context.Background(), "New transaction")
	if err != nil {
		t.Errorf("Expected to be able to begin new transaction after abort: %v", err)
	}
}

// TestTransactionManager_IsTransactionActive tests active transaction check.
func TestTransactionManager_IsTransactionActive(t *testing.T) {
	tmpDir := t.TempDir()

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Initially no active transaction
	if tm.IsTransactionActive() {
		t.Error("Expected no active transaction initially")
	}

	// Begin transaction
	_, err := tm.Begin(context.Background(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Now should be active
	if !tm.IsTransactionActive() {
		t.Error("Expected active transaction after Begin")
	}

	// Abort
	tm.Abort(context.Background(), "test")

	// Should not be active
	if tm.IsTransactionActive() {
		t.Error("Expected no active transaction after Abort")
	}
}

// TestTransactionManager_GetActiveTransaction tests getting active transaction.
func TestTransactionManager_GetActiveTransaction(t *testing.T) {
	tmpDir := t.TempDir()

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// No active transaction initially
	txn, exists := tm.GetActiveTransaction()
	if exists || txn != nil {
		t.Error("Expected no active transaction initially")
	}

	// Begin transaction
	created, err := tm.Begin(context.Background(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Get active transaction
	txn, exists = tm.GetActiveTransaction()
	if !exists || txn == nil {
		t.Error("Expected active transaction to exist")
	}

	if txn.ID != created.ID {
		t.Error("Expected same transaction ID")
	}
}

// TestFileEdit_EditTypes tests different edit types.
func TestFileEdit_EditTypes(t *testing.T) {
	tests := []struct {
		editType    EditType
		description string
	}{
		{EditTypeModify, "modify"},
		{EditTypeCreate, "create"},
		{EditTypeDelete, "delete"},
	}

	for _, tt := range tests {
		if string(tt.editType) != tt.description {
			t.Errorf("EditType %v should equal %q", tt.editType, tt.description)
		}
	}
}

// TestTransactionStatus tests transaction status values.
func TestTransactionStatus(t *testing.T) {
	statuses := []TransactionStatus{
		TxnStatusPending,
		TxnStatusPreparing,
		TxnStatusReady,
		TxnStatusCommitting,
		TxnStatusCommitted,
		TxnStatusAborted,
	}

	statusSet := make(map[TransactionStatus]bool)
	for _, status := range statuses {
		if statusSet[status] {
			t.Errorf("Duplicate status: %s", status)
		}
		statusSet[status] = true

		if status == "" {
			t.Error("Status should not be empty")
		}
	}
}

// TestComputeHash tests the hash computation function.
func TestComputeHash(t *testing.T) {
	// Test empty
	hash := computeHash([]byte{})
	if hash != "empty" {
		t.Errorf("Expected 'empty' for empty content, got %s", hash)
	}

	// Test that same content produces same hash
	hash1 := computeHash([]byte("test content"))
	hash2 := computeHash([]byte("test content"))
	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}

	// Test that different content produces different hash
	hash3 := computeHash([]byte("different content"))
	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}
}

// TestTransactionManager_ToFacts tests fact generation.
func TestTransactionManager_ToFacts(t *testing.T) {
	tmpDir := t.TempDir()

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// No facts when no transaction
	facts := tm.ToFacts()
	if len(facts) != 0 {
		t.Errorf("Expected 0 facts without transaction, got %d", len(facts))
	}

	// Begin transaction
	txn, err := tm.Begin(context.Background(), "Test facts")
	if err != nil {
		t.Fatal(err)
	}

	// Add edit
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("test"), 0644)
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		EditType: EditTypeModify,
		Content:  []byte("modified"),
	})

	// Get facts
	facts = tm.ToFacts()

	// Should have transaction_state and plan_edit facts
	var hasTransactionState, hasPlanEdit bool
	for _, f := range facts {
		if f.Predicate == "transaction_state" {
			hasTransactionState = true
			if len(f.Args) < 2 {
				t.Error("transaction_state should have 2 args")
			}
		}
		if f.Predicate == "plan_edit" {
			hasPlanEdit = true
		}
	}

	if !hasTransactionState {
		t.Error("Expected transaction_state fact")
	}
	if !hasPlanEdit {
		t.Error("Expected plan_edit fact")
	}

	_ = txn
}

// TestValidationResult_Fields tests ValidationResult structure.
func TestValidationResult_Fields(t *testing.T) {
	result := &ValidationResult{
		IsValid:       true,
		ParseErrors:   []ParseError{{FilePath: "test.go", Line: 1, Column: 0, Message: "error"}},
		SafetyBlocks:  []SafetyBlock{{Ref: "ref", Reason: "reason", Rule: "rule"}},
		Warnings:      []string{"warning"},
		AffectedRefs:  []string{"ref1", "ref2"},
	}

	if !result.IsValid {
		t.Error("Expected IsValid to be true")
	}

	if len(result.ParseErrors) != 1 {
		t.Errorf("Expected 1 parse error, got %d", len(result.ParseErrors))
	}

	if result.ParseErrors[0].FilePath != "test.go" {
		t.Error("Expected parse error file path to be test.go")
	}

	if len(result.SafetyBlocks) != 1 {
		t.Errorf("Expected 1 safety block, got %d", len(result.SafetyBlocks))
	}

	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}

	if len(result.AffectedRefs) != 2 {
		t.Errorf("Expected 2 affected refs, got %d", len(result.AffectedRefs))
	}
}

// TestTransaction_InitialState tests transaction initial state.
func TestTransaction_InitialState(t *testing.T) {
	txn := &Transaction{
		ID:          "test",
		Description: "test desc",
		Status:      TxnStatusPending,
		Edits:       make([]FileEdit, 0),
		Snapshots:   make(map[string][]byte),
	}

	if txn.ID != "test" {
		t.Error("ID not set correctly")
	}

	if txn.Description != "test desc" {
		t.Error("Description not set correctly")
	}

	if txn.Status != TxnStatusPending {
		t.Error("Status should be pending")
	}

	if txn.Edits == nil {
		t.Error("Edits should not be nil")
	}

	if txn.Snapshots == nil {
		t.Error("Snapshots should not be nil")
	}
}

// TestTransactionManager_Concurrency tests thread safety.
func TestTransactionManager_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Start a transaction
	_, err := tm.Begin(context.Background(), "Concurrent test")
	if err != nil {
		t.Fatal(err)
	}

	// Add a test file
	testFile := filepath.Join(tmpDir, "concurrent.go")
	os.WriteFile(testFile, []byte("test"), 0644)
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: testFile,
		EditType: EditTypeModify,
		Content:  []byte("modified"),
	})

	// Run concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = tm.IsTransactionActive()
			_, _ = tm.GetActiveTransaction()
			_ = tm.ToFacts()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestSafetyBlock_Fields tests SafetyBlock structure.
func TestSafetyBlock_Fields(t *testing.T) {
	block := SafetyBlock{
		Ref:    "go:test.go:Func",
		Reason: "/auth_removed",
		Rule:   "deny_edit",
	}

	if block.Ref != "go:test.go:Func" {
		t.Error("Ref not set correctly")
	}

	if block.Reason != "/auth_removed" {
		t.Error("Reason not set correctly")
	}

	if block.Rule != "deny_edit" {
		t.Error("Rule not set correctly")
	}
}

// TestParseError_Fields tests ParseError structure.
func TestParseError_Fields(t *testing.T) {
	pe := ParseError{
		FilePath: "test.go",
		Line:     42,
		Column:   10,
		Message:  "syntax error",
	}

	if pe.FilePath != "test.go" {
		t.Error("FilePath not set correctly")
	}

	if pe.Line != 42 {
		t.Error("Line not set correctly")
	}

	if pe.Column != 10 {
		t.Error("Column not set correctly")
	}

	if pe.Message != "syntax error" {
		t.Error("Message not set correctly")
	}
}
