package logging

import (
	"fmt"
	"sync"
	"testing"
)

func TestRequestLogger_WithField_ShouldNotMutateTheReceiver(t *testing.T) {
	base := WithRequestID(CategoryKernel, "req-test")

	a := base.WithField("a", 1)
	b := base.WithField("b", 2)

	if len(base.fields) != 0 {
		t.Fatalf("base should still have 0 fields, got %d: %v", len(base.fields), base.fields)
	}

	if v, ok := a.fields["a"]; !ok || v != 1 {
		t.Errorf("derived a should have field 'a'=1, got %v (present=%v) fields=%v", v, ok, a.fields)
	}
	if _, ok := a.fields["b"]; ok {
		t.Errorf("derived a should not have field 'b', got fields=%v", a.fields)
	}

	if v, ok := b.fields["b"]; !ok || v != 2 {
		t.Errorf("derived b should have field 'b'=2, got %v (present=%v) fields=%v", v, ok, b.fields)
	}
	if _, ok := b.fields["a"]; ok {
		t.Errorf("derived b should not have field 'a', got fields=%v", b.fields)
	}

	if a == base {
		t.Error("WithField should return a new RequestLogger, got same pointer as base for a")
	}
	if b == base {
		t.Error("WithField should return a new RequestLogger, got same pointer as base for b")
	}
	if a == b {
		t.Error("siblings derived from same base should be distinct pointers")
	}
}

func TestRequestLogger_WithField_WhenDerivedConcurrently_ShouldNotRace(t *testing.T) {
	base := WithRequestID(CategoryKernel, "req-concurrent")

	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		i := i
		go func() {
			defer wg.Done()
			derived := base.WithField(fmt.Sprintf("k%d", i), i)
			derived.Info("msg %d", i)
		}()
	}
	wg.Wait()

	if len(base.fields) != 0 {
		t.Errorf("base fields should remain 0 after concurrent derivations, got %d: %v", len(base.fields), base.fields)
	}
}
