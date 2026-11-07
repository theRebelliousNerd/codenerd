package knowledge

import (
	"context"
	"testing"
)

type coverOKSource struct {
	name  string
	atoms []Atom
	err   error
}

func (s coverOKSource) Name() string { return s.name }

func (s coverOKSource) Acquire(_ context.Context, _ Subject) ([]Atom, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.atoms, nil
}

func TestCoverAcquireNilCtx(t *testing.T) {
	a := Acquirer{Sources: []Source{nil, coverOKSource{name: "b"}}}
	_, outcomes := a.Acquire(nil, Subject{Topic: "t"})
	if len(outcomes) != 1 {
		t.Fatalf("want 1 got %d", len(outcomes))
	}
}

func TestCoverAcquireCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := Acquirer{Sources: []Source{coverOKSource{name: "a"}}}
	_, outcomes := a.Acquire(ctx, Subject{Topic: "t"})
	if len(outcomes) != 1 || outcomes[0].Status != StatusFailed {
		t.Fatalf("want failed %+v", outcomes)
	}
}
