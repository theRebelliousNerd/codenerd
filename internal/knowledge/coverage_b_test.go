package knowledge

import (
	"context"
	"testing"
)

type coverCancelSrcB struct {
	cancel context.CancelFunc
}

func (s coverCancelSrcB) Name() string { return "cover-cancel-b" }

func (s coverCancelSrcB) Acquire(_ context.Context, _ Subject) ([]Atom, error) {
	s.cancel()
	return []Atom{{Concept: "c", Content: "v"}}, nil
}

func TestCoverAcquireCtxErrAfterCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := Acquirer{Sources: []Source{coverCancelSrcB{cancel: cancel}}}
	_, outcomes := a.Acquire(ctx, Subject{Topic: "t"})
	if len(outcomes) != 1 {
		t.Fatalf("want 1 got %d", len(outcomes))
	}
	if outcomes[0].Status != StatusFailed {
		t.Fatalf("want failed got %s", outcomes[0].Status)
	}
}
