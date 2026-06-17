package mangle

import (
	"strings"
	"sync"
	"testing"
)

func TestParseUnit_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// Valid mangle unit string
	unitStr := `
		Decl test_pred(Int).
		test_pred(1).
	`

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh // Wait for the green light
			reader := strings.NewReader(unitStr)
			_, err := ParseUnit(reader)
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
			}
		}()
	}

	close(startCh) // Unleash all goroutines
	wg.Wait()
}

func TestParseAtom_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// ParseAtom expects arguments to have types or be variables/strings
	// Use an atom that is valid for ParseAtom
	atomStr := "test_pred(1)"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh // Wait for the green light
			_, err := ParseAtom(atomStr)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
			}
		}()
	}

	close(startCh) // Unleash all goroutines
	wg.Wait()
}

func TestParseMixed_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-startCh
			source := `
				Decl test_pred(Int).
				test_pred(1).
			`
			_, err := ParseUnit(strings.NewReader(source))
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			<-startCh
			source := "test_pred(1)"
			_, err := ParseAtom(source)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
			}
		}()
	}

	close(startCh)
	wg.Wait()
}

func TestParseUnit_Error(t *testing.T) {
	// Test parsing invalid syntax
	invalidStr := "p(X) :- "
	_, err := ParseUnit(strings.NewReader(invalidStr))
	if err == nil {
		t.Errorf("Expected ParseUnit to return an error for invalid syntax")
	}
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
