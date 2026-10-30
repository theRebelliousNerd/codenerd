package store

import (
	"context"
	"testing"
)

// The first-recall heal claimed its space before it ran and never released
// the claim, so "attempted once" passed for "healed": a heal stopped by its
// caller, or one that found nothing before legacy rows arrived, left every
// later recall on that store skipping the rows it never reached (audit
// legacy-vector-repair-attempt-lifecycle).

// A heal its caller canceled is incomplete, not done: the next recall
// finishes it.
func TestHealUnattributed_ACanceledHealIsResumedByTheNextRecall(t *testing.T) {
	s := newSpaceStore(t)
	if _, err := s.db.Exec(`INSERT INTO vectors (content, embedding, metadata) VALUES ('alpha', '[0,1,0,0]', '{"path":"alpha","kind":"unit"}')`); err != nil {
		t.Fatal(err)
	}
	engine := &spaceEngine{name: "test:model-a", dim: 4}
	attach(t, s, engine)
	s.mu.RLock()
	space := s.currentSpaceLocked()
	s.mu.RUnlock()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	s.maybeHealUnattributed(canceled, engine, space)
	if cov, err := s.VectorCoverage(); err != nil || cov.Unattributed != 1 {
		t.Fatalf("precondition: the canceled heal left alpha unattributed, coverage %+v (%v)", cov, err)
	}

	if _, err := s.VectorRecallSemantic(context.Background(), "alpha", 5); err != nil {
		t.Fatal(err)
	}
	cov, err := s.VectorCoverage()
	if err != nil {
		t.Fatal(err)
	}
	if cov.Unattributed != 0 || cov.Eligible != 1 {
		t.Fatalf("after a healthy recall coverage = %+v: the canceled heal was taken for a finished one", cov)
	}
}

// A heal that found nothing has healed what was there, not what comes later:
// a legacy row that arrives afterwards (an import, a restored database) is
// healed by the next recall.
func TestHealUnattributed_LegacyRowsArrivingLaterAreHealed(t *testing.T) {
	s := newSpaceStore(t)
	engine := &spaceEngine{name: "test:model-a", dim: 4}
	attach(t, s, engine)
	ctx := context.Background()
	if _, err := s.VectorRecallSemantic(ctx, "alpha", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO vectors (content, embedding, metadata) VALUES ('alpha', '[0,1,0,0]', '{"path":"alpha","kind":"unit"}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VectorRecallSemantic(ctx, "alpha", 5); err != nil {
		t.Fatal(err)
	}
	cov, err := s.VectorCoverage()
	if err != nil {
		t.Fatal(err)
	}
	if cov.Unattributed != 0 || cov.Eligible != 1 {
		t.Fatalf("coverage = %+v: the row that arrived after an empty heal was never healed", cov)
	}

	// With nothing new, a recall pays only for its query.
	before := len(engine.embeddedTexts())
	if _, err := s.VectorRecallSemantic(ctx, "alpha", 5); err != nil {
		t.Fatal(err)
	}
	if after := engine.embeddedTexts(); len(after)-before != 1 {
		t.Fatalf("a recall with nothing to heal embedded %v, want just the query", after[before:])
	}
}
