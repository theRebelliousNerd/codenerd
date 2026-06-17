package mangle

import (
	"strings"
	"sync"
	"testing"
)

func TestParseUnit_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			source := `
				Decl test_pred(Int).
				test_pred(1).
			`
			_, err := ParseUnit(strings.NewReader(source))
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func TestParseAtom_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			source := "test_pred(1)"
			_, err := ParseAtom(source)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func TestParseMixed_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			source := `
				Decl test_pred(Int).
				test_pred(1).
			`
			_, err := ParseUnit(strings.NewReader(source))
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
				return
			}
		}()

		go func() {
			defer wg.Done()
			source := "test_pred(1)"
			_, err := ParseAtom(source)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func TestParseAtom_Success(t *testing.T) {
	atom, err := ParseAtom("test_pred(1)")
	if err != nil {
		t.Fatalf("ParseAtom failed: %v", err)
	}
	if atom.Predicate.Symbol != "test_pred" {
		t.Errorf("Expected predicate 'test_pred', got '%v'", atom.Predicate.Symbol)
	}
}

func TestParseAtom_Error(t *testing.T) {
	_, err := ParseAtom("invalid syntax (((")
	if err == nil {
		t.Fatalf("ParseAtom should fail on invalid syntax")
	}
}
