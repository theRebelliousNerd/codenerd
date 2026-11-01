package mangle

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestParseAtomContextOwnership(t *testing.T) {
	if err := parseMu.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	owned := true
	defer func() {
		if owned {
			parseMu.Release(1)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 8 {
		if _, err := ParseAtomContext(ctx, "waiting(/owner)"); !errors.Is(err, context.Canceled) {
			t.Fatalf("withdrawn parser = %v", err)
		}
		if parseMu.TryAcquire(1) {
			parseMu.Release(1)
			t.Fatal("withdrawn waiter released another parser's ownership")
		}
	}
	parseMu.Release(1)
	owned = false
	// Pre-cancellation also refuses when the semaphore is immediately free.
	if _, err := ParseAtomContext(ctx, "free(/owner)"); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled parser = %v", err)
	}
	if _, err := ParseUnit(strings.NewReader("healthy(/owner).")); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAtomContext(context.Background(), "healthy(/owner)"); err != nil {
		t.Fatal(err)
	}
}

func TestParseAtomContextInputSemantics(t *testing.T) {
	for _, tc := range []struct {
		input, spelling string
		bad             bool
	}{
		{`probe(/alice, "value", 7)`, `probe(/alice,"value",7)`, false},
		{`probe("/alice", Value)`, `probe("/alice",Value)`, false},
		{`probe(7).`, `probe(7)`, false},
		{`probe(7) ignored`, "", true},
		{`probe(`, "", true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			atom, err := ParseAtomContext(context.Background(), tc.input)
			if (err != nil) != tc.bad {
				t.Fatalf("parse error = %v, bad=%v", err, tc.bad)
			}
			if err == nil && atom.String() != tc.spelling {
				t.Fatalf("atom = %s, want %s", atom.String(), tc.spelling)
			}
		})
	}
}

func TestParseAtomContextConcurrentWithOrdinaryParsers(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 48 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			switch i % 3 {
			case 0:
				_, err = ParseAtomContext(context.Background(), `parallel(/name, "text")`)
			case 1:
				_, err = ParseAtom(`parallel(/name, "text")`)
			case 2:
				_, err = ParseUnit(strings.NewReader(`parallel(/name, "text").`))
			}
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
