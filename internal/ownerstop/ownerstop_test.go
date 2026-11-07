package ownerstop

import (
	"context"
	"testing"
	"time"
)

// The owner's stop outlives the work context's soft deadline: the deadline is
// the context's permanent first error, and Done is still the channel that
// reports the owner's cancel afterwards.
func TestDone_ReportsTheOwnersStopAfterTheWorkDeadlineFired(t *testing.T) {
	t.Parallel()
	owner, stop := context.WithCancel(context.Background())
	defer stop()
	work, cancel := context.WithTimeout(owner, time.Nanosecond)
	defer cancel()
	work = With(work, owner)

	<-work.Done()
	if work.Err() != context.DeadlineExceeded {
		t.Fatalf("work error = %v, want its deadline", work.Err())
	}
	select {
	case <-Done(work):
		t.Fatal("owner reported stopped before it was")
	default:
	}
	stop()
	select {
	case <-Done(work):
	case <-time.After(time.Second):
		t.Fatal("owner stop not visible through the work context after its deadline")
	}
}

// A context with no recorded owner has no owner channel, and a nil owner
// records nothing.
func TestDone_NilWithoutAnOwner(t *testing.T) {
	t.Parallel()
	if Done(context.Background()) != nil {
		t.Fatal("a context with no owner reported a done channel")
	}
	ctx := context.Background()
	if got := With(ctx, nil); got != ctx || Done(got) != nil {
		t.Fatal("a nil owner must leave the context unchanged")
	}
}
