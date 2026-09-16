package store

import (
	"context"
	"testing"
)

func openLocalTestStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// Session turns merge: a rich rewrite upgrades a placeholder row, but a later
// placeholder must never clobber rich payloads already stored.
func TestSessionTurn_MergesPlaceholders(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreSessionTurn("sess", 1, "", "", "", ""); err != nil {
		t.Fatalf("placeholder store: %v", err)
	}
	if err := s.StoreSessionTurn("sess", 1, "do it", `{"verb":"do"}`, "done", `["a"]`); err != nil {
		t.Fatalf("rich store: %v", err)
	}
	h, err := s.GetSessionHistory("sess", 10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("history = %d turns, want 1 merged row", len(h))
	}
	if h[0]["user_input"] != "do it" || h[0]["intent"] != `{"verb":"do"}` ||
		h[0]["response"] != "done" || h[0]["atoms"] != `["a"]` {
		t.Fatalf("merged turn = %+v, want the rich payloads", h[0])
	}

	// Placeholder rewrite after rich: nothing regresses.
	if err := s.StoreSessionTurn("sess", 1, "", "{}", "", "[]"); err != nil {
		t.Fatalf("placeholder rewrite: %v", err)
	}
	h, err = s.GetSessionHistory("sess", 10)
	if err != nil {
		t.Fatal(err)
	}
	if h[0]["user_input"] != "do it" || h[0]["atoms"] != `["a"]` {
		t.Errorf("turn after placeholder rewrite = %+v, want rich payloads kept", h[0])
	}
}

// Compressed state round-trips per turn; unknown sessions read as empty with
// the neutral 1.0 ratio rather than an error.
func TestCompressedState_RoundTripAndDefault(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreCompressedState("sess", 2, `{"k":"v"}`, 0.4); err != nil {
		t.Fatalf("StoreCompressedState: %v", err)
	}
	state, turn, ratio, err := s.LoadLatestCompressedState("sess")
	if err != nil {
		t.Fatalf("LoadLatestCompressedState: %v", err)
	}
	if state != `{"k":"v"}` || turn != 2 || ratio != 0.4 {
		t.Errorf("got (%q, %d, %f), want the stored state", state, turn, ratio)
	}
	state, turn, ratio, err = s.LoadLatestCompressedState("missing")
	if err != nil || state != "" || turn != 0 || ratio != 1.0 {
		t.Errorf("missing session = (%q, %d, %f, %v), want empty/0/1.0/nil", state, turn, ratio, err)
	}
}

// Activations keep the max score per fact within the hour window and honor
// the minimum-score floor.
func TestActivations_MaxPerFactAndFloor(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.LogActivation("f1", 0.2); err != nil {
		t.Fatal(err)
	}
	if err := s.LogActivation("f1", 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.LogActivation("f2", 0.1); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRecentActivations(10, 0.5)
	if err != nil {
		t.Fatalf("GetRecentActivations: %v", err)
	}
	if len(got) != 1 || got["f1"] != 0.9 {
		t.Errorf("activations = %v, want map[f1:0.9]", got)
	}
}

// With an engine, a knowledge atom lands in BOTH halves of the dual store
// and is semantically retrievable; without one, the atoms table still works
// and semantic search degrades to empty instead of erroring.
func TestKnowledgeAtom_DualWriteAndDegrade(t *testing.T) {
	ctx := context.Background()
	s := openLocalTestStore(t)
	s.SetEmbeddingEngine(&MockEmbeddingEngine{})
	if err := s.StoreKnowledgeAtomWithEmbedding(ctx, "strategic/x", "hold the north star", 0.9); err != nil {
		t.Fatalf("StoreKnowledgeAtomWithEmbedding: %v", err)
	}
	byPrefix, err := s.GetKnowledgeAtomsByPrefix("strategic/")
	if err != nil {
		t.Fatalf("GetKnowledgeAtomsByPrefix: %v", err)
	}
	if len(byPrefix) != 1 || byPrefix[0].Concept != "strategic/x" {
		t.Fatalf("byPrefix = %+v, want the atom", byPrefix)
	}
	sem, err := s.SearchKnowledgeAtomsSemantic(ctx, "north star", 5)
	if err != nil {
		t.Fatalf("SearchKnowledgeAtomsSemantic: %v", err)
	}
	if len(sem) != 1 || sem[0].Concept != "strategic/x" {
		t.Fatalf("semantic = %+v, want the atom via vectors", sem)
	}

	plain := openLocalTestStore(t)
	if err := plain.StoreKnowledgeAtomWithEmbedding(ctx, "c", "content", 0.5); err != nil {
		t.Fatalf("engineless store: %v", err)
	}
	sem, err = plain.SearchKnowledgeAtomsSemantic(ctx, "content", 5)
	if err != nil || sem != nil {
		t.Errorf("engineless semantic = %+v, %v; want nil, nil", sem, err)
	}
}

// Verification attempts persist with their payloads, and the stats view
// groups failing patterns by their violations blob.
func TestVerification_RoundTripAndStats(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreVerification("sess", 1, "task", "coder", 1, false, 0.3, "bad", `["v1"]`, "{}", "{}", "h1"); err != nil {
		t.Fatalf("StoreVerification: %v", err)
	}
	if err := s.StoreVerification("sess", 1, "task", "coder", 2, true, 0.9, "good", `[]`, "{}", "{}", "h2"); err != nil {
		t.Fatalf("StoreVerification: %v", err)
	}
	h, err := s.GetVerificationHistory("sess", 10)
	if err != nil {
		t.Fatalf("GetVerificationHistory: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("history = %d records, want 2", len(h))
	}
	stats, err := s.GetQualityViolationStats()
	if err != nil {
		t.Fatalf("GetQualityViolationStats: %v", err)
	}
	if stats[`["v1"]`] != 1 || len(stats) != 1 {
		t.Errorf("stats = %v, want only the failing pattern counted once", stats)
	}
}

// Review findings are write-only through the API; the row must exist with
// every field intact.
func TestReviewFinding_Persists(t *testing.T) {
	s := openLocalTestStore(t)
	in := StoredReviewFinding{FilePath: "a.go", Line: 12, Severity: "high", Category: "sec", RuleID: "R1", Message: "m", ProjectRoot: "/r"}
	if err := s.StoreReviewFinding(in); err != nil {
		t.Fatalf("StoreReviewFinding: %v", err)
	}
	var got StoredReviewFinding
	err := s.db.QueryRow(
		"SELECT file_path, line, severity, category, rule_id, message, project_root FROM review_findings",
	).Scan(&got.FilePath, &got.Line, &got.Severity, &got.Category, &got.RuleID, &got.Message, &got.ProjectRoot)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != in {
		t.Errorf("got %+v, want %+v", got, in)
	}
}
