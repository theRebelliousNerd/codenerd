package sessionlog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DecisionSet is what shaped one request: which prompt atoms the compiler
// kept and why, which context the selector kept and cut, and what the working
// set moved out or brought back for this call.
type DecisionSet struct {
	JIT        *JITReceipt     `json:"jit,omitempty"`
	Context    *ContextReceipt `json:"context,omitempty"`
	WorkingSet *WorkingSetNote `json:"working_set,omitempty"`
}

// latest is a receipt that persists across calls until replaced. The first
// exchange after it was noted carries it whole; later exchanges name that
// file instead of repeating it.
type latest[T any] struct {
	value      T
	attachedIn string
}

// JITReceipt is the prompt compiler's receipt for the system prompt a call
// carried: the atoms selected in render order, the atoms dropped, the budget.
// It is built from the compiler's own manifest (prompt.PromptManifest), not
// recomputed.
type JITReceipt struct {
	ContextHash string       `json:"context_hash,omitempty"`
	Compiled    *time.Time   `json:"compiled,omitempty"`
	TokenUsage  int          `json:"token_usage"`
	BudgetLimit int          `json:"budget_limit"`
	Selected    []JITAtom    `json:"selected_in_render_order,omitempty"`
	Dropped     []JITDropped `json:"dropped,omitempty"`
	// SameAs names the exchange file that carried this receipt whole.
	SameAs string `json:"same_as,omitempty"`
}

// JITAtom is one selected atom.
type JITAtom struct {
	Order       int     `json:"order"`
	ID          string  `json:"id"`
	Category    string  `json:"category,omitempty"`
	Source      string  `json:"source,omitempty"`
	Reason      string  `json:"reason"`
	Importance  string  `json:"importance,omitempty"`
	LogicScore  float64 `json:"logic_score"`
	VectorScore float64 `json:"vector_score"`
	RenderMode  string  `json:"render_mode,omitempty"`
}

// JITDropped is one atom considered and rejected.
type JITDropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// manifestShape reads the compiler's manifest by its JSON field names, so this
// package needs no import of the compiler. A Reason the manifest grows is used
// as is.
type manifestShape struct {
	Timestamp   time.Time `json:"timestamp"`
	ContextHash string    `json:"context_hash"`
	TokenUsage  int       `json:"token_usage"`
	BudgetLimit int       `json:"budget_limit"`
	Selected    []struct {
		ID          string  `json:"id"`
		Category    string  `json:"category"`
		Source      string  `json:"source"`
		Reason      string  `json:"reason"`
		Importance  string  `json:"importance"`
		LogicScore  float64 `json:"logic_score"`
		VectorScore float64 `json:"vector_score"`
		RenderMode  string  `json:"render_mode"`
	} `json:"selected"`
	Dropped []JITDropped `json:"dropped"`
}

// NewJITReceipt converts a prompt manifest (anything with the manifest's JSON
// shape) into a receipt.
func NewJITReceipt(manifest any) (*JITReceipt, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var m manifestShape
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	r := &JITReceipt{
		ContextHash: m.ContextHash,
		TokenUsage:  m.TokenUsage,
		BudgetLimit: m.BudgetLimit,
		Dropped:     m.Dropped,
	}
	if !m.Timestamp.IsZero() {
		ts := m.Timestamp
		r.Compiled = &ts
	}
	for i, s := range m.Selected {
		reason := s.Reason
		if reason == "" {
			reason = fmt.Sprintf("selected: importance=%s logic_score=%.3f vector_score=%.3f render=%s",
				s.Importance, s.LogicScore, s.VectorScore, s.RenderMode)
		}
		r.Selected = append(r.Selected, JITAtom{
			Order: i + 1, ID: s.ID, Category: s.Category, Source: s.Source, Reason: reason,
			Importance: s.Importance, LogicScore: s.LogicScore, VectorScore: s.VectorScore, RenderMode: s.RenderMode,
		})
	}
	return r, nil
}

// ContextReceipt is the context selector's receipt for a call: what entered
// the prompt and why, and what was left out, masked, evicted or retracted and
// why.
type ContextReceipt struct {
	Source       string        `json:"source"`
	SectionOrder []string      `json:"section_order,omitempty"`
	Included     []ContextItem `json:"included,omitempty"`
	Excluded     []ContextItem `json:"excluded,omitempty"`
	// Stats is the selector's own statistics (context.SelectionStats).
	Stats  any    `json:"stats,omitempty"`
	SameAs string `json:"same_as,omitempty"`
}

// ContextItem is one context item and the reason for its fate.
type ContextItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Action string `json:"action,omitempty"` // excluded | masked | evicted | retracted
	Reason string `json:"reason"`
}

// WorkingSetNote is what the working set did for one request: results it
// carried whole, results it moved out behind recall handles, observations it
// restated because their file changed, and observations a recall brought back.
type WorkingSetNote struct {
	Round    int            `json:"round"`
	Carried  []WorkingEntry `json:"carried,omitempty"`
	Evicted  []string       `json:"evicted,omitempty"`
	Restated []string       `json:"restated,omitempty"`
	Recalled []string       `json:"recalled,omitempty"`
}

// WorkingEntry is one tool result a request carries whole.
type WorkingEntry struct {
	Call  string `json:"call"`
	ID    string `json:"id"`
	Bytes int    `json:"bytes"`
	Round int    `json:"round"`
}

// NoteJIT records the prompt manifest of the compile that produced the system
// prompt the agent under ctx is about to send. A no-op unless exchange logging
// is active, so call sites need no guard.
func NoteJIT(ctx context.Context, manifest any) {
	s := ScopeFrom(ctx)
	if s == nil || manifest == nil || !Active() {
		return
	}
	r, err := NewJITReceipt(manifest)
	if err != nil {
		Current().fail(fmt.Errorf("jit receipt: %w", err))
		return
	}
	s.state.mu.Lock()
	s.state.jit = &latest[JITReceipt]{value: *r}
	s.state.mu.Unlock()
}

// NoteContext records the context selection behind the next calls.
func NoteContext(ctx context.Context, r ContextReceipt) {
	s := ScopeFrom(ctx)
	if s == nil || !Active() {
		return
	}
	s.state.mu.Lock()
	s.state.context = &latest[ContextReceipt]{value: r}
	s.state.mu.Unlock()
}

// NoteWorkingSet records what the working set did for the next call. It is
// consumed by that call.
func NoteWorkingSet(ctx context.Context, n WorkingSetNote) {
	s := ScopeFrom(ctx)
	if s == nil || !Active() {
		return
	}
	s.state.mu.Lock()
	s.state.working = &n
	s.state.mu.Unlock()
}

// takeDecisions returns the decisions for the exchange named file. A receipt
// already carried whole by an earlier exchange is replaced by a pointer to it.
func takeDecisions(s *Scope, file string) *DecisionSet {
	if s == nil {
		return nil
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	var d DecisionSet
	if l := s.state.jit; l != nil {
		v := l.value
		if l.attachedIn == "" {
			l.attachedIn = file
		} else {
			v = JITReceipt{ContextHash: v.ContextHash, SameAs: l.attachedIn}
		}
		d.JIT = &v
	}
	if l := s.state.context; l != nil {
		v := l.value
		if l.attachedIn == "" {
			l.attachedIn = file
		} else {
			v = ContextReceipt{Source: v.Source, SameAs: l.attachedIn}
		}
		d.Context = &v
	}
	if w := s.state.working; w != nil {
		d.WorkingSet = w
		s.state.working = nil
	}
	if d.JIT == nil && d.Context == nil && d.WorkingSet == nil {
		return nil
	}
	return &d
}
