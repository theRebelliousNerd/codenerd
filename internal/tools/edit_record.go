package tools

import (
	"context"
	"sync"
)

// ExecutionRecord is what the registry reports to the kernel about one
// completed tool execution.
type ExecutionRecord struct {
	ToolName    string
	Success     bool
	DurationMs  int64
	UnixSeconds int64
	// Edits are the code elements the call changed, as the tool recorded them
	// with RecordEdit. The kernel turns them into element_modified and
	// modified_function, which the impact chain and run_impacted_tests read;
	// before 2026-09-22 no model edit produced either, so both answered
	// "nothing was edited" on the live path.
	Edits []EditedElement
}

// EditedElement is one code element a tool call changed.
type EditedElement struct {
	// File is the workspace-relative path.
	File     string
	Language string
	// Package is the Go package clause; empty for Mangle.
	Package  string
	Kind     string
	Name     string
	Receiver string
	// Key is the element's address within its file.
	Key string
	// Removed marks an element the call deleted.
	Removed bool
}

type editRecorderKey struct{}

type editRecorder struct {
	mu    sync.Mutex
	edits []EditedElement
}

// withEditRecorder gives a tool call a place to record what it edited.
func withEditRecorder(ctx context.Context) (context.Context, *editRecorder) {
	rec := &editRecorder{}
	return context.WithValue(ctx, editRecorderKey{}, rec), rec
}

func (r *editRecorder) list() []EditedElement {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]EditedElement(nil), r.edits...)
}

// RecordEdit notes code elements a tool call changed. It is a no-op outside a
// registry execution, so a tool stays callable directly (tests, apply_edits
// staging) without a recorder.
func RecordEdit(ctx context.Context, edits ...EditedElement) {
	rec, _ := ctx.Value(editRecorderKey{}).(*editRecorder)
	if rec == nil {
		return
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.edits = append(rec.edits, edits...)
}
