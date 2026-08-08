package core

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

const PendingEditFactName = "pending_edit"

// PendingEdit mirrors Decl pending_edit(FilePath, Content) bound [/string, /string].
// FilePath: repo-relative path (e.g. "internal/core/foo.go"), no leading "/", no "..", no "\".
// Content: file content preview (opaque string); empty allowed for create/delete edge.
// For kernel EDB use PendingEditContentPreview to truncate to 200+... before Assert.
type PendingEdit struct {
	FilePath string
	Content  string
}

// FactName returns the Mangle predicate name for this type.
func (PendingEdit) FactName() string {
	return PendingEditFactName
}

// ---------------------------------------------------------------------------
// In-memory store (kept for backward compatibility with pending_edit_store_test.go)
// ---------------------------------------------------------------------------

// RetractPendingEdit removes from the default in-memory store.
// ---------------------------------------------------------------------------
// Kernel fact lifecycle helpers — pending_edit(FilePath, Content)
// ---------------------------------------------------------------------------

// ValidatePendingEditFilePath validates FilePath shape expected by policy and
// VirtualStore.resolvePath: non-empty (trimmed), repo-relative (no leading "/"),
// no ".." traversal, no "\" separators. Content is opaque and not validated.
func ValidatePendingEditFilePath(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("pending_edit: FilePath must not be empty")
	}
	if strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("pending_edit: FilePath must be repo-relative, got %q", filePath)
	}
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("pending_edit: FilePath must not contain \"..\", got %q", filePath)
	}
	if strings.Contains(filePath, "\\") {
		return fmt.Errorf("pending_edit: FilePath must use '/' separators, got %q", filePath)
	}
	return nil
}

// PendingEditContentPreview truncates content to a 200-char preview + "..." for
// Mangle EDB storage, mirroring transaction_manager.go pending_mutation logic
// (string(snapshot[:200])+"..." when len > 200). Short content is returned unchanged.
// The full content is still written to disk; only the fact arg is previewed.
func PendingEditContentPreview(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// pendingEditPreview is an unexported alias retained for internal use and to
// document the analog name trunc200 from the inventory.
func pendingEditPreview(s string) string { return PendingEditContentPreview(s) }

// NewPendingEditFact builds the kernel Fact for pending_edit(FilePath, Content)
// with Content already preview-truncated. Caller should have validated FilePath.
func NewPendingEditFact(filePath, content string) types.Fact {
	return types.Fact{
		Predicate: PendingEditFactName,
		Args:      []any{filePath, PendingEditContentPreview(content)},
	}
}

// AssertPendingEditFact validates FilePath and asserts
// pending_edit(FilePath, ContentPreview) via the existing Go fact API
// Kernel.Assert. Content may be empty (create/delete edge); FilePath must
// satisfy ValidatePendingEditFilePath. Uses the canonical Decl shape
// pending_edit(FilePath, Content) bound [/string, /string] from schemas_shards.mg:132.
func AssertPendingEditFact(kernel types.Kernel, filePath, content string) error {
	if kernel == nil {
		return fmt.Errorf("pending_edit: kernel is nil")
	}
	if err := ValidatePendingEditFilePath(filePath); err != nil {
		return err
	}
	fact := NewPendingEditFact(filePath, content)
	if err := kernel.Assert(fact); err != nil {
		return fmt.Errorf("pending_edit: failed to assert pending_edit(%q, len=%d): %w", filePath, len(content), err)
	}
	return nil
}

// AssertPendingEditBatch validates each FilePath and asserts all pending_edit
// facts in a single AssertBatch call (one dirty flag, one evaluate cycle).
func AssertPendingEditBatch(kernel types.Kernel, edits []PendingEdit) error {
	if kernel == nil {
		return fmt.Errorf("pending_edit: kernel is nil")
	}
	if len(edits) == 0 {
		return nil
	}
	facts := make([]types.Fact, 0, len(edits))
	for _, e := range edits {
		if err := ValidatePendingEditFilePath(e.FilePath); err != nil {
			return err
		}
		facts = append(facts, NewPendingEditFact(e.FilePath, e.Content))
	}
	if err := kernel.AssertBatch(facts); err != nil {
		return fmt.Errorf("pending_edit: AssertBatch failed for %d edits: %w", len(facts), err)
	}
	return nil
}

// RetractPendingEditFact retracts the pending_edit fact(s) for filePath using
// the existing Go fact API Kernel.RetractFact. RetractFact matches on predicate
// + first arg (FilePath) per kernel_facts.go:750 (argsEqual on Args[0]), so
// the Content arg is not needed to identify the fact. This is the primary
// per-file abandonment path (commit/abort/error cleanup).
func RetractPendingEditFact(kernel types.Kernel, filePath string) error {
	if kernel == nil {
		return fmt.Errorf("pending_edit: kernel is nil")
	}
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("pending_edit: FilePath must not be empty for retract")
	}
	fact := types.Fact{
		Predicate: PendingEditFactName,
		Args:      []any{filePath},
	}
	if err := kernel.RetractFact(fact); err != nil {
		return fmt.Errorf("pending_edit: failed to retract pending_edit(%q): %w", filePath, err)
	}
	return nil
}

// RetractPendingEditExact retracts the exact pending_edit(FilePath, ContentPreview)
// fact using RetractExactFact when available, falling back to RetractFact or
// RetractExactFactsBatch. Prefer RetractPendingEditFact for per-file abandonment
// (first-arg match); use this only when the exact Content preview must match.
func RetractPendingEditExact(kernel types.Kernel, filePath, content string) error {
	if kernel == nil {
		return fmt.Errorf("pending_edit: kernel is nil")
	}
	if err := ValidatePendingEditFilePath(filePath); err != nil {
		return err
	}
	preview := PendingEditContentPreview(content)
	fact := types.Fact{
		Predicate: PendingEditFactName,
		Args:      []any{filePath, preview},
	}
	// Prefer RealKernel.RetractExactFact if present.
	if rk, ok := kernel.(interface {
		RetractExactFact(types.Fact) error
	}); ok {
		if err := rk.RetractExactFact(fact); err != nil {
			return fmt.Errorf("pending_edit: RetractExactFact failed for %q: %w", filePath, err)
		}
		return nil
	}
	// Fallback: batch exact retract (part of Kernel interface).
	if batch, ok := kernel.(interface {
		RetractExactFactsBatch([]types.Fact) error
	}); ok {
		if err := batch.RetractExactFactsBatch([]types.Fact{fact}); err != nil {
			return fmt.Errorf("pending_edit: RetractExactFactsBatch failed for %q: %w", filePath, err)
		}
		return nil
	}
	// Last resort: first-arg retract (predicate + FilePath).
	if err := kernel.RetractFact(types.Fact{Predicate: PendingEditFactName, Args: []any{filePath}}); err != nil {
		return fmt.Errorf("pending_edit: fallback retract failed for %q: %w", filePath, err)
	}
	return nil
}

// ClearPendingEdits retracts ALL pending_edit facts via Kernel.Retract(predicate).
// Use on commit, abort, or any error path to avoid orphaned pending_edit facts
// that would keep dormant safety blocks (coder_safety.mg, coder_workflow.mg, etc.) alive.
// Idempotent — retracting a non-existent predicate is a no-op in RealKernel.
func ClearPendingEdits(kernel types.Kernel) error {
	if kernel == nil {
		return fmt.Errorf("pending_edit: kernel is nil")
	}
	if err := kernel.Retract(PendingEditFactName); err != nil {
		return fmt.Errorf("pending_edit: failed to clear pending_edit: %w", err)
	}
	return nil
}

// AbandonPendingEdits is an alias for ClearPendingEdits — semantic name for
// error/abort cleanup paths. Both retract all pending_edit facts for the kernel.
func AbandonPendingEdits(kernel types.Kernel) error {
	return ClearPendingEdits(kernel)
}

// QueryPendingEdits returns all pending_edit(FilePath, ContentPreview) facts
// currently in the kernel EDB, sorted by FilePath. Uses Kernel.Query.
func QueryPendingEdits(kernel types.Kernel) ([]PendingEdit, error) {
	if kernel == nil {
		return nil, fmt.Errorf("pending_edit: kernel is nil")
	}
	facts, err := kernel.Query(PendingEditFactName)
	if err != nil {
		return nil, fmt.Errorf("pending_edit: query failed: %w", err)
	}
	edits := make([]PendingEdit, 0, len(facts))
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		fp, _ := f.Args[0].(string)
		if strings.TrimSpace(fp) == "" {
			continue
		}
		var content string
		if f.Args[1] != nil {
			if s, ok := f.Args[1].(string); ok {
				content = s
			} else {
				content = fmt.Sprintf("%v", f.Args[1])
			}
		}
		edits = append(edits, PendingEdit{FilePath: fp, Content: content})
	}
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].FilePath < edits[j].FilePath
	})
	return edits, nil
}

// HasPendingEdit reports whether a pending_edit fact exists for filePath.
func HasPendingEdit(kernel types.Kernel, filePath string) (bool, error) {
	if kernel == nil {
		return false, fmt.Errorf("pending_edit: kernel is nil")
	}
	if err := ValidatePendingEditFilePath(filePath); err != nil {
		return false, err
	}
	facts, err := kernel.Query(PendingEditFactName)
	if err != nil {
		return false, fmt.Errorf("pending_edit: query failed: %w", err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 {
			if fp, ok := f.Args[0].(string); ok && fp == filePath {
				return true, nil
			}
		}
	}
	return false, nil
}

// assertPendingEdit marks a write action as in flight for the policy layer.
//
// The fact lives in the kernel and nowhere else. An in-memory mirror would be a
// second source of truth for something the rules already read from the fact
// store, and the two would drift the first time a write took an unexpected exit
// path.
//
// Returns the asserted fact and true only when an assertion actually landed, so
// the caller can defer exactly the matching retraction.
func (v *VirtualStore) assertPendingEdit(req ActionRequest) (types.Fact, bool) {
	if _, isWrite := writeMutationActions[req.Type]; !isWrite {
		return types.Fact{}, false
	}

	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()
	if kernel == nil {
		return types.Fact{}, false
	}

	content, _ := req.Payload["content"].(string)
	fact := NewPendingEditFact(req.Target, content)
	if err := kernel.Assert(fact); err != nil {
		logging.VirtualStoreWarn("failed to assert pending_edit for %s: %v", req.Target, err)
		return types.Fact{}, false
	}
	return fact, true
}

// retractPendingEdit clears the in-flight marker. Deferred by the caller so it
// runs on success, failure, validator refusal and panic alike -- pending_edit
// means "an edit is happening right now", so any path that leaves it behind
// makes every rule reading it reason about work that already finished.
func (v *VirtualStore) retractPendingEdit(fact types.Fact) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()
	if kernel == nil {
		return
	}
	if err := kernel.RetractFact(fact); err != nil {
		logging.VirtualStoreWarn("failed to retract pending_edit for %v: %v", fact.Args, err)
	}
}
