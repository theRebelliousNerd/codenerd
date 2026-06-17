package mangle

import (
	"strings"
	"sync"
	"testing"
)

func TestParseUnit_Concurrent(t *testing.T) {
	const numGoroutines = 50
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// Valid mangle unit string
	unitStr := "p(X) :- q(X)."

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
	const numGoroutines = 50
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// ParseAtom expects arguments to have types or be variables/strings
	// Use an atom that is valid for ParseAtom
	atomStr := "foo('bar')"

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

func TestParseUnit_Error(t *testing.T) {
    // Test parsing invalid syntax
    invalidStr := "p(X) :- "
    _, err := ParseUnit(strings.NewReader(invalidStr))
    if err == nil {
        t.Errorf("Expected ParseUnit to return an error for invalid syntax")
    }
}

func TestParseAtom_Error(t *testing.T) {
    // Test parsing invalid syntax
    invalidStr := "foo("
    _, err := ParseAtom(invalidStr)
    if err == nil {
        t.Errorf("Expected ParseAtom to return an error for invalid syntax")
    }
}
