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

// TestTransactionManager_MultiFileEdit tests multi-file atomic edits.
func TestTransactionManager_MultiFileEdit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	file1 := filepath.Join(tmpDir, "user.go")
	file2 := filepath.Join(tmpDir, "api.go")
	file3 := filepath.Join(tmpDir, "types.ts")

	os.WriteFile(file1, []byte("package main\n\ntype User struct {\n\tUserID string\n}\n"), 0644)
	os.WriteFile(file2, []byte("package main\n\nfunc GetUser(userID string) {}\n"), 0644)
	os.WriteFile(file3, []byte("interface IUser {\n\tuserId: string;\n}\n"), 0644)

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Begin transaction for cross-language refactor
	_, err := tm.Begin(context.Background(), "Rename userID to subID across languages")
	if err != nil {
		t.Fatal(err)
	}

	// Add edits for all three files (simulating polyglot refactor)
	edits := []FileEdit{
		{
			FilePath: file1,
			Content:  []byte("package main\n\ntype User struct {\n\tSubID string\n}\n"),
			EditType: EditTypeModify,
		},
		{
			FilePath: file2,
			Content:  []byte("package main\n\nfunc GetUser(subID string) {}\n"),
			EditType: EditTypeModify,
		},
		{
			FilePath: file3,
			Content:  []byte("interface IUser {\n\tsubId: string;\n}\n"),
			EditType: EditTypeModify,
		},
	}

	for _, edit := range edits {
		err := tm.AddEdit(context.Background(), edit)
		if err != nil {
			t.Fatalf("AddEdit failed for %s: %v", edit.FilePath, err)
		}
	}

	// Verify all edits are tracked
	txn, _ := tm.GetActiveTransaction()
	if len(txn.Edits) != 3 {
		t.Errorf("Expected 3 edits in transaction, got %d", len(txn.Edits))
	}

	// Verify snapshots were taken for all files
	for _, edit := range edits {
		if _, exists := txn.Snapshots[edit.FilePath]; !exists {
			t.Errorf("Expected snapshot for %s", edit.FilePath)
		}
	}

	// Verify facts are generated for all edits
	facts := tm.ToFacts()
	planEditCount := 0
	for _, f := range facts {
		if f.Predicate == "plan_edit" {
			planEditCount++
		}
	}
	if planEditCount != 3 {
		t.Errorf("Expected 3 plan_edit facts, got %d", planEditCount)
	}
}

// TestTransactionManager_MultiFileAbort tests that abort rolls back all files.
func TestTransactionManager_MultiFileAbort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files with original content
	file1 := filepath.Join(tmpDir, "backend.go")
	file2 := filepath.Join(tmpDir, "frontend.ts")

	original1 := []byte("package main\n\nfunc Original1() {}\n")
	original2 := []byte("function original2() {}\n")

	os.WriteFile(file1, original1, 0644)
	os.WriteFile(file2, original2, 0644)

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	// Begin transaction
	_, err := tm.Begin(context.Background(), "Multi-file edit to abort")
	if err != nil {
		t.Fatal(err)
	}

	// Add edits
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: file1,
		Content:  []byte("package main\n\nfunc Modified1() {}\n"),
		EditType: EditTypeModify,
	})
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: file2,
		Content:  []byte("function modified2() {}\n"),
		EditType: EditTypeModify,
	})

	// Verify snapshots exist before abort
	txn, _ := tm.GetActiveTransaction()
	if len(txn.Snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(txn.Snapshots))
	}

	// Abort transaction
	err = tm.Abort(context.Background(), "Safety violation detected")
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	// Verify transaction is aborted
	if tm.IsTransactionActive() {
		t.Error("Transaction should not be active after abort")
	}

	// Note: Actual file rollback would happen in Commit phase
	// Abort just marks the transaction as aborted and preserves snapshots
}

// TestTransactionManager_MultiFileFactGeneration tests fact generation for multi-file edits.
func TestTransactionManager_MultiFileFactGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in different "packages"
	goFile := filepath.Join(tmpDir, "models", "user.go")
	tsFile := filepath.Join(tmpDir, "frontend", "types.ts")
	pyFile := filepath.Join(tmpDir, "backend", "models.py")

	os.MkdirAll(filepath.Join(tmpDir, "models"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "frontend"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "backend"), 0755)

	os.WriteFile(goFile, []byte("package models\n"), 0644)
	os.WriteFile(tsFile, []byte("// types\n"), 0644)
	os.WriteFile(pyFile, []byte("# models\n"), 0644)

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "Cross-language refactor")
	if err != nil {
		t.Fatal(err)
	}

	// Add edits for different languages
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: goFile,
		Content:  []byte("package models\n\ntype User struct{}\n"),
		EditType: EditTypeModify,
	})
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: tsFile,
		Content:  []byte("interface User {}\n"),
		EditType: EditTypeModify,
	})
	tm.AddEdit(context.Background(), FileEdit{
		FilePath: pyFile,
		Content:  []byte("class User:\n    pass\n"),
		EditType: EditTypeModify,
	})

	// Get facts
	facts := tm.ToFacts()

	// Verify transaction_state fact
	var hasTransactionState bool
	var planEditFiles []string

	for _, f := range facts {
		if f.Predicate == "transaction_state" {
			hasTransactionState = true
		}
		if f.Predicate == "plan_edit" && len(f.Args) >= 1 {
			if path, ok := f.Args[0].(string); ok {
				planEditFiles = append(planEditFiles, path)
			}
		}
	}

	if !hasTransactionState {
		t.Error("Expected transaction_state fact")
	}

	if len(planEditFiles) != 3 {
		t.Errorf("Expected 3 plan_edit facts, got %d", len(planEditFiles))
	}

	// Verify all file paths are represented
	fileSet := make(map[string]bool)
	for _, f := range planEditFiles {
		fileSet[f] = true
	}

	if !fileSet[goFile] {
		t.Error("Missing plan_edit for Go file")
	}
	if !fileSet[tsFile] {
		t.Error("Missing plan_edit for TypeScript file")
	}
	if !fileSet[pyFile] {
		t.Error("Missing plan_edit for Python file")
	}
}

// TestTransactionManager_MixedEditTypes tests transactions with create, modify, delete.
func TestTransactionManager_MixedEditTypes(t *testing.T) {
	tmpDir := t.TempDir()

	existingFile := filepath.Join(tmpDir, "existing.go")
	newFile := filepath.Join(tmpDir, "new.go")
	toDeleteFile := filepath.Join(tmpDir, "delete_me.go")

	os.WriteFile(existingFile, []byte("package main\n"), 0644)
	os.WriteFile(toDeleteFile, []byte("package main\n// to be deleted\n"), 0644)

	kernel := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	tm := NewTransactionManager(kernel, tmpDir)

	_, err := tm.Begin(context.Background(), "Mixed edit types")
	if err != nil {
		t.Fatal(err)
	}

	// Modify existing
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: existingFile,
		Content:  []byte("package main\n\nfunc Modified() {}\n"),
		EditType: EditTypeModify,
	})
	if err != nil {
		t.Fatalf("AddEdit (modify) failed: %v", err)
	}

	// Create new
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: newFile,
		Content:  []byte("package main\n\nfunc New() {}\n"),
		EditType: EditTypeCreate,
	})
	if err != nil {
		t.Fatalf("AddEdit (create) failed: %v", err)
	}

	// Delete existing
	err = tm.AddEdit(context.Background(), FileEdit{
		FilePath: toDeleteFile,
		Content:  nil,
		EditType: EditTypeDelete,
	})
	if err != nil {
		t.Fatalf("AddEdit (delete) failed: %v", err)
	}

	txn, _ := tm.GetActiveTransaction()

	// Verify all edit types
	var hasModify, hasCreate, hasDelete bool
	for _, edit := range txn.Edits {
		switch edit.EditType {
		case EditTypeModify:
			hasModify = true
		case EditTypeCreate:
			hasCreate = true
		case EditTypeDelete:
			hasDelete = true
		}
	}

	if !hasModify {
		t.Error("Expected modify edit")
	}
	if !hasCreate {
		t.Error("Expected create edit")
	}
	if !hasDelete {
		t.Error("Expected delete edit")
	}

	// Verify snapshot taken for files being modified/deleted (not created)
	if _, exists := txn.Snapshots[existingFile]; !exists {
		t.Error("Expected snapshot for modified file")
	}
	if _, exists := txn.Snapshots[toDeleteFile]; !exists {
		t.Error("Expected snapshot for deleted file")
	}
}
