// Package core provides the Transaction Manager for atomic multi-file edits.
// The Transaction Manager implements a Two-Phase Commit (2PC) protocol for
// coordinating edits across multiple files with safety validation.
package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// TransactionManager orchestrates atomic multi-file edits using 2PC protocol.
// It uses shadow validation to ensure edits pass safety rules before committing.
type TransactionManager struct {
	mu          sync.RWMutex
	shadowMode  *ShadowMode
	kernel      *RealKernel
	projectRoot string
	txns        map[string]*Transaction
	activeTxnID string
}

// Transaction represents an atomic unit of work spanning multiple files.
type Transaction struct {
	ID          string
	Description string
	StartTime   time.Time
	Status      TransactionStatus
	Edits       []FileEdit
	Snapshots   map[string][]byte // Original file contents for rollback
	Validation  *ShadowValidationResult
	Error       error
}

// TransactionStatus represents the state of a transaction in the 2PC protocol.
type TransactionStatus string

const (
	TxnStatusPending    TransactionStatus = "pending"    // Initial state
	TxnStatusPreparing  TransactionStatus = "preparing"  // Validating in shadow
	TxnStatusReady      TransactionStatus = "ready"      // All validations passed
	TxnStatusCommitting TransactionStatus = "committing" // Writing to filesystem
	TxnStatusCommitted  TransactionStatus = "committed"  // Successfully committed
	TxnStatusAborted    TransactionStatus = "aborted"    // Rolled back
)

// FileEdit represents a proposed edit to a file.
type FileEdit struct {
	FilePath  string
	OldHash   string // Expected hash before edit (for conflict detection)
	NewHash   string // Hash after edit
	Content   []byte // New file content
	EditType  EditType
	Timestamp time.Time
}

// EditType categorizes the type of file edit.
type EditType string

const (
	EditTypeModify EditType = "modify" // Modify existing file
	EditTypeCreate EditType = "create" // Create new file
	EditTypeDelete EditType = "delete" // Delete file
)

// ShadowValidationResult holds the outcome of shadow validation.
type ShadowValidationResult struct {
	IsValid       bool
	ParseErrors   []ParseError
	SafetyBlocks  []SafetyBlock
	Warnings      []string
	AffectedRefs  []string
	ValidatedAt   time.Time
	ValidDuration time.Duration
}

// ParseError represents a syntax error in a file.
type ParseError struct {
	FilePath string
	Line     int
	Column   int
	Message  string
}

// SafetyBlock represents a deny_edit rule that blocked the transaction.
type SafetyBlock struct {
	Ref    string
	Reason string
	Rule   string
}

// NewTransactionManager creates a new Transaction Manager.
func NewTransactionManager(kernel *RealKernel, projectRoot string) *TransactionManager {
	return &TransactionManager{
		shadowMode:  NewShadowMode(kernel),
		kernel:      kernel,
		projectRoot: projectRoot,
		txns:        make(map[string]*Transaction),
	}
}

// Begin starts a new transaction.
func (tm *TransactionManager) Begin(ctx context.Context, description string) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeTxnID != "" {
		return nil, fmt.Errorf("transaction already active: %s", tm.activeTxnID)
	}

	txnID := fmt.Sprintf("txn_%d", time.Now().UnixNano())

	txn := &Transaction{
		ID:          txnID,
		Description: description,
		StartTime:   time.Now(),
		Status:      TxnStatusPending,
		Edits:       make([]FileEdit, 0),
		Snapshots:   make(map[string][]byte),
	}

	tm.txns[txnID] = txn
	tm.activeTxnID = txnID

	logging.KernelDebug("Transaction started: %s - %s", txnID, description)

	return txn, nil
}

// AddEdit adds a file edit to the active transaction.
func (tm *TransactionManager) AddEdit(ctx context.Context, edit FileEdit) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeTxnID == "" {
		return fmt.Errorf("no active transaction")
	}

	txn := tm.txns[tm.activeTxnID]
	if txn == nil {
		return fmt.Errorf("transaction not found: %s", tm.activeTxnID)
	}

	if txn.Status != TxnStatusPending {
		return fmt.Errorf("transaction not in pending state: %s", txn.Status)
	}

	// Take snapshot of original file content for rollback
	if edit.EditType != EditTypeCreate {
		if _, exists := txn.Snapshots[edit.FilePath]; !exists {
			content, err := os.ReadFile(edit.FilePath)
			if err != nil && edit.EditType == EditTypeModify {
				return fmt.Errorf("failed to snapshot file: %s - %w", edit.FilePath, err)
			}
			txn.Snapshots[edit.FilePath] = content
		}
	}

	edit.Timestamp = time.Now()
	txn.Edits = append(txn.Edits, edit)

	logging.KernelDebug("Added edit to transaction %s: %s (%s)", txn.ID, edit.FilePath, edit.EditType)

	return nil
}

// Prepare validates the transaction in shadow mode (Phase 1 of 2PC).
// Returns true if all validations pass and the transaction is ready to commit.
func (tm *TransactionManager) Prepare(ctx context.Context) (*ShadowValidationResult, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeTxnID == "" {
		return nil, fmt.Errorf("no active transaction")
	}

	txn := tm.txns[tm.activeTxnID]
	if txn == nil {
		return nil, fmt.Errorf("transaction not found: %s", tm.activeTxnID)
	}

	if txn.Status != TxnStatusPending {
		return nil, fmt.Errorf("transaction not in pending state: %s", txn.Status)
	}

	txn.Status = TxnStatusPreparing
	startTime := time.Now()

	logging.KernelDebug("Preparing transaction: %s", txn.ID)

	// Start shadow simulation
	sim, err := tm.shadowMode.StartSimulation(ctx, fmt.Sprintf("Transaction: %s", txn.Description))
	if err != nil {
		txn.Status = TxnStatusAborted
		txn.Error = err
		return nil, fmt.Errorf("failed to start shadow simulation: %w", err)
	}

	result := &ShadowValidationResult{
		IsValid:      true,
		ParseErrors:  make([]ParseError, 0),
		SafetyBlocks: make([]SafetyBlock, 0),
		Warnings:     make([]string, 0),
		AffectedRefs: make([]string, 0),
		ValidatedAt:  time.Now(),
	}

	// Simulate each edit
	for i, edit := range txn.Edits {
		actionID := fmt.Sprintf("edit_%d", i)

		// Check for file conflicts (hash mismatch)
		if edit.OldHash != "" && edit.EditType == EditTypeModify {
			currentContent, err := os.ReadFile(edit.FilePath)
			if err != nil {
				result.IsValid = false
				result.ParseErrors = append(result.ParseErrors, ParseError{
					FilePath: edit.FilePath,
					Message:  fmt.Sprintf("failed to read file: %v", err),
				})
				continue
			}
			currentHash := computeHash(currentContent)
			if currentHash != edit.OldHash {
				result.IsValid = false
				result.SafetyBlocks = append(result.SafetyBlocks, SafetyBlock{
					Ref:    edit.FilePath,
					Reason: "file_modified_externally",
					Rule:   "conflict_detection",
				})
				continue
			}
		}

		// Simulate the file write action
		simAction := SimulatedAction{
			ID:          actionID,
			Type:        ActionTypeFileWrite,
			Target:      edit.FilePath,
			Description: fmt.Sprintf("Edit %s", filepath.Base(edit.FilePath)),
		}

		simResult, err := tm.shadowMode.SimulateAction(ctx, simAction)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("simulation warning for %s: %v", edit.FilePath, err))
		}

		if simResult != nil && !simResult.IsSafe {
			result.IsValid = false
			for _, v := range simResult.Violations {
				result.SafetyBlocks = append(result.SafetyBlocks, SafetyBlock{
					Ref:    v.ActionID,
					Reason: v.Description,
					Rule:   v.ViolationType,
				})
			}
		}
	}

	// Check for deny_edit rules in the shadow kernel
	denyEdits, _ := tm.shadowMode.GetShadowKernel().Query("deny_edit")
	for _, de := range denyEdits {
		ref := ""
		reason := ""
		if len(de.Args) > 0 {
			ref = fmt.Sprintf("%v", de.Args[0])
		}
		if len(de.Args) > 1 {
			reason = fmt.Sprintf("%v", de.Args[1])
		}
		result.IsValid = false
		result.SafetyBlocks = append(result.SafetyBlocks, SafetyBlock{
			Ref:    ref,
			Reason: reason,
			Rule:   "deny_edit",
		})
	}

	// Abort shadow simulation
	tm.shadowMode.AbortSimulation("validation complete")

	result.ValidDuration = time.Since(startTime)
	txn.Validation = result

	if result.IsValid {
		txn.Status = TxnStatusReady
		logging.KernelDebug("Transaction prepared successfully: %s (validation took %v)", txn.ID, result.ValidDuration)
	} else {
		txn.Status = TxnStatusAborted
		logging.KernelDebug("Transaction preparation failed: %s - %d safety blocks", txn.ID, len(result.SafetyBlocks))
	}

	_ = sim // Used for logging if needed

	return result, nil
}

// Commit applies the transaction to the filesystem (Phase 2 of 2PC).
// Only succeeds if the transaction is in Ready state.
func (tm *TransactionManager) Commit(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeTxnID == "" {
		return fmt.Errorf("no active transaction")
	}

	txn := tm.txns[tm.activeTxnID]
	if txn == nil {
		return fmt.Errorf("transaction not found: %s", tm.activeTxnID)
	}

	if txn.Status != TxnStatusReady {
		return fmt.Errorf("transaction not ready to commit: %s", txn.Status)
	}

	txn.Status = TxnStatusCommitting
	logging.KernelDebug("Committing transaction: %s", txn.ID)

	// Apply all edits atomically
	var committedFiles []string
	for _, edit := range txn.Edits {
		switch edit.EditType {
		case EditTypeModify, EditTypeCreate:
			// Ensure parent directory exists
			dir := filepath.Dir(edit.FilePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				tm.rollback(txn, committedFiles)
				txn.Status = TxnStatusAborted
				txn.Error = fmt.Errorf("failed to create directory: %s - %w", dir, err)
				return txn.Error
			}

			// Write the file
			if err := os.WriteFile(edit.FilePath, edit.Content, 0644); err != nil {
				tm.rollback(txn, committedFiles)
				txn.Status = TxnStatusAborted
				txn.Error = fmt.Errorf("failed to write file: %s - %w", edit.FilePath, err)
				return txn.Error
			}
			committedFiles = append(committedFiles, edit.FilePath)

		case EditTypeDelete:
			if err := os.Remove(edit.FilePath); err != nil && !os.IsNotExist(err) {
				tm.rollback(txn, committedFiles)
				txn.Status = TxnStatusAborted
				txn.Error = fmt.Errorf("failed to delete file: %s - %w", edit.FilePath, err)
				return txn.Error
			}
			committedFiles = append(committedFiles, edit.FilePath)
		}
	}

	txn.Status = TxnStatusCommitted
	tm.activeTxnID = ""

	// Emit file_written facts to kernel
	for _, edit := range txn.Edits {
		if edit.EditType != EditTypeDelete {
			tm.kernel.Assert(Fact{
				Predicate: "file_written",
				Args:      []interface{}{edit.FilePath, edit.NewHash, txn.ID, time.Now().Unix()},
			})
		}
	}

	logging.KernelDebug("Transaction committed: %s (%d files)", txn.ID, len(committedFiles))

	return nil
}

// Abort cancels the active transaction without applying changes.
func (tm *TransactionManager) Abort(ctx context.Context, reason string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeTxnID == "" {
		return fmt.Errorf("no active transaction")
	}

	txn := tm.txns[tm.activeTxnID]
	if txn == nil {
		return fmt.Errorf("transaction not found: %s", tm.activeTxnID)
	}

	if txn.Status == TxnStatusCommitted {
		return fmt.Errorf("cannot abort committed transaction")
	}

	txn.Status = TxnStatusAborted
	txn.Error = fmt.Errorf("aborted: %s", reason)
	tm.activeTxnID = ""

	logging.KernelDebug("Transaction aborted: %s - %s", txn.ID, reason)

	return nil
}

// rollback restores files to their original state on commit failure.
func (tm *TransactionManager) rollback(txn *Transaction, committedFiles []string) {
	logging.KernelDebug("Rolling back transaction: %s (%d files)", txn.ID, len(committedFiles))

	for _, filePath := range committedFiles {
		if original, exists := txn.Snapshots[filePath]; exists {
			if len(original) > 0 {
				if err := os.WriteFile(filePath, original, 0644); err != nil {
					logging.Get(logging.CategoryKernel).Error("Rollback failed for %s: %v", filePath, err)
				}
			} else {
				// Original was empty or didn't exist - delete the created file
				_ = os.Remove(filePath)
			}
		}
	}
}

// GetActiveTransaction returns the currently active transaction.
func (tm *TransactionManager) GetActiveTransaction() (*Transaction, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.activeTxnID == "" {
		return nil, false
	}

	txn, exists := tm.txns[tm.activeTxnID]
	return txn, exists
}

// GetTransaction retrieves a transaction by ID.
func (tm *TransactionManager) GetTransaction(txnID string) (*Transaction, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	txn, exists := tm.txns[txnID]
	return txn, exists
}

// IsTransactionActive returns true if a transaction is currently in progress.
func (tm *TransactionManager) IsTransactionActive() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.activeTxnID != ""
}

// computeHash computes a hash for conflict detection.
func computeHash(content []byte) string {
	if len(content) == 0 {
		return "empty"
	}
	// Use a simple hash for now (same as in scope.go)
	hash := uint64(0)
	for _, b := range content {
		hash = hash*31 + uint64(b)
	}
	return fmt.Sprintf("%016x", hash)
}

// ToFacts converts transaction state to Mangle facts.
func (tm *TransactionManager) ToFacts() []Fact {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	facts := make([]Fact, 0)

	if tm.activeTxnID == "" {
		return facts
	}

	txn := tm.txns[tm.activeTxnID]
	if txn == nil {
		return facts
	}

	// Add transaction_state fact
	facts = append(facts, Fact{
		Predicate: "transaction_state",
		Args:      []interface{}{txn.ID, string(txn.Status)},
	})

	// Add plan_edit facts for each edit
	for _, edit := range txn.Edits {
		facts = append(facts, Fact{
			Predicate: "plan_edit",
			Args:      []interface{}{edit.FilePath},
		})
	}

	return facts
}
