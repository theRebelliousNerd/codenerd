package sessionlog

import (
	"context"
	"testing"
	"time"
)

// manifest mirrors prompt.PromptManifest's JSON shape without importing the
// compiler.
type manifest struct {
	Timestamp   time.Time `json:"timestamp"`
	ContextHash string    `json:"context_hash"`
	TokenUsage  int       `json:"token_usage"`
	BudgetLimit int       `json:"budget_limit"`
	Selected    []struct {
		ID          string  `json:"id"`
		Category    string  `json:"category"`
		Source      string  `json:"source"`
		Importance  string  `json:"importance"`
		LogicScore  float64 `json:"logic_score"`
		VectorScore float64 `json:"vector_score"`
		RenderMode  string  `json:"render_mode"`
	} `json:"selected"`
	Dropped []struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	} `json:"dropped"`
}

func sampleManifest() manifest {
	m := manifest{Timestamp: time.Date(2026, 10, 28, 1, 2, 3, 0, time.UTC), ContextHash: "h1", TokenUsage: 1200, BudgetLimit: 4000}
	m.Selected = append(m.Selected, struct {
		ID          string  `json:"id"`
		Category    string  `json:"category"`
		Source      string  `json:"source"`
		Importance  string  `json:"importance"`
		LogicScore  float64 `json:"logic_score"`
		VectorScore float64 `json:"vector_score"`
		RenderMode  string  `json:"render_mode"`
	}{"identity/core", "identity", "embedded", "mandatory", 1, 0.5, "standard"},
		struct {
			ID          string  `json:"id"`
			Category    string  `json:"category"`
			Source      string  `json:"source"`
			Importance  string  `json:"importance"`
			LogicScore  float64 `json:"logic_score"`
			VectorScore float64 `json:"vector_score"`
			RenderMode  string  `json:"render_mode"`
		}{"tools/go", "tools", "project", "high", 0.8, 0.9, "concise"})
	m.Dropped = append(m.Dropped, struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}{"extra/verbose", "Budget: exceeded"})
	return m
}

func TestJITReceiptKeepsRenderOrderSourceReasonDropsAndBudget(t *testing.T) {
	r, err := NewJITReceipt(sampleManifest())
	if err != nil {
		t.Fatal(err)
	}
	if r.ContextHash != "h1" || r.BudgetLimit != 4000 || r.TokenUsage != 1200 || r.Compiled == nil {
		t.Fatalf("receipt header: %+v", r)
	}
	if len(r.Selected) != 2 || r.Selected[0].ID != "identity/core" || r.Selected[0].Order != 1 || r.Selected[1].Order != 2 {
		t.Fatalf("selected must keep render order: %+v", r.Selected)
	}
	if r.Selected[1].Source != "project" || r.Selected[1].RenderMode != "concise" || r.Selected[1].Reason == "" {
		t.Fatalf("source, mode and reason must be present: %+v", r.Selected[1])
	}
	if len(r.Dropped) != 1 || r.Dropped[0].Reason != "Budget: exceeded" {
		t.Fatalf("dropped: %+v", r.Dropped)
	}
}

func TestExchangeCarriesDecisionsFirstWholeThenByReference(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	ctx := WithAgent(context.Background(), "coder", "coder-1")

	NoteJIT(ctx, sampleManifest())
	NoteContext(ctx, ContextReceipt{Source: "compressor", Included: []ContextItem{{ID: "recent_turns", Reason: "window"}}})
	NoteWorkingSet(ctx, WorkingSetNote{Round: 3, Evicted: []string{"call-1"}, Recalled: []string{"obs-9"}})
	for i := 0; i < 2; i++ {
		_, call := Begin(ctx, CallMeta{Purpose: "session"})
		call.End(Result{})
	}
	var first, second Exchange
	readJSON(t, sessionPath(s, "session-1", "coder-1", "0001-session.json"), &first)
	readJSON(t, sessionPath(s, "session-1", "coder-1", "0002-session.json"), &second)

	d := first.Decisions
	if d == nil || d.JIT == nil || len(d.JIT.Selected) != 2 || d.Context == nil || len(d.Context.Included) != 1 {
		t.Fatalf("first exchange must carry the receipts whole: %+v", d)
	}
	if d.WorkingSet == nil || d.WorkingSet.Evicted[0] != "call-1" || d.WorkingSet.Recalled[0] != "obs-9" {
		t.Fatalf("working-set evictions and recalls: %+v", d.WorkingSet)
	}
	d2 := second.Decisions
	if d2 == nil || d2.JIT == nil || d2.JIT.SameAs != "0001-session.json" || len(d2.JIT.Selected) != 0 || d2.JIT.ContextHash != "h1" {
		t.Fatalf("a repeated receipt must point at the file that holds it: %+v", d2)
	}
	if d2.Context == nil || d2.Context.SameAs != "0001-session.json" {
		t.Fatalf("context receipt must point at the first: %+v", d2)
	}
	if d2.WorkingSet != nil {
		t.Fatalf("working-set notes are about one call and must not repeat: %+v", d2.WorkingSet)
	}

	// A new compile replaces the receipt and is carried whole again.
	m := sampleManifest()
	m.ContextHash = "h2"
	NoteJIT(ctx, m)
	_, call := Begin(ctx, CallMeta{Purpose: "session"})
	call.End(Result{})
	var third Exchange
	readJSON(t, sessionPath(s, "session-1", "coder-1", "0003-session.json"), &third)
	if third.Decisions.JIT.SameAs != "" || third.Decisions.JIT.ContextHash != "h2" || len(third.Decisions.JIT.Selected) != 2 {
		t.Fatalf("new compile: %+v", third.Decisions.JIT)
	}
}

func TestNotesAreNoOpsWhenDebugIsOffOrThereIsNoScope(t *testing.T) {
	_, on := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "c")
	NoteJIT(ctx, sampleManifest())
	NoteContext(ctx, ContextReceipt{Source: "x"})
	NoteWorkingSet(ctx, WorkingSetNote{Round: 1})
	on.Store(true)
	_, call := Begin(ctx, CallMeta{Purpose: "session"})
	if d := takeDecisions(call.scope, call.File()); d != nil {
		t.Fatalf("notes taken while debug was off must not appear: %+v", d)
	}
	NoteJIT(context.Background(), sampleManifest()) // no scope: must not panic
}
