package core

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// factContentIdentity is the string stored where policy binds a content slot
// it never reads (pending_edit/2, pending_mutation/4). The digest is the
// same shape the session path uses for a large body (sha256:%x bytes:%d),
// so two writers name one body the same way. Empty content stays empty.
func factContentIdentity(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("sha256:%x bytes:%d", sum, len(s))
}

const PendingEditFactName = "pending_edit"

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

// NewPendingEditFact builds pending_edit(FilePath, Content). Content is a
// sha256 identity, not a prefix of the body: policy binds the slot as a
// wildcard, and two bodies that share a prefix must stay distinct.
// Caller should have validated FilePath.
func NewPendingEditFact(filePath, content string) types.Fact {
	return types.Fact{
		Predicate: PendingEditFactName,
		Args:      []any{filePath, factContentIdentity(content)},
	}
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
