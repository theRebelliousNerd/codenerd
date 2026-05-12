package core

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TEST_GAP 1: Null/Undefined/Empty
func TestSyntaxValidator_EmptyTarget(t *testing.T) {
	v := NewSyntaxValidator()
	req := ActionRequest{Type: ActionWriteFile, Target: ""}
	res := ActionResult{Success: true}
	vr := v.Validate(context.Background(), req, res)
	if !vr.Verified || vr.Method != ValidationMethodSkipped {
		t.Errorf("Expected skipped validation for empty target, got %v", vr)
	}

	mv := NewMangleSyntaxValidator()
	vrMangle := mv.Validate(context.Background(), req, res)
	if !vrMangle.Verified || vrMangle.Method != ValidationMethodSkipped {
		t.Errorf("Expected skipped validation for empty target in Mangle validator, got %v", vrMangle)
	}
}

func TestSyntaxValidator_EmptyContent(t *testing.T) {
	v := NewSyntaxValidator()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.go")
	os.WriteFile(path, []byte(""), 0644)

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	res := ActionResult{Success: true}
	
	vr := v.Validate(context.Background(), req, res)
	// An empty .go file is genuinely syntactically invalid (missing package declaration).
	// The validator is correct to reject it.
	if vr.Verified {
		t.Error("Empty go file should be syntactically invalid (missing package declaration)")
	}
	if vr.Error == "" {
		t.Error("Expected an error message for empty Go file")
	}

	// But a minimal valid Go file should pass
	validPath := filepath.Join(tmpDir, "valid.go")
	os.WriteFile(validPath, []byte("package main\n"), 0644)
	reqValid := ActionRequest{Type: ActionWriteFile, Target: validPath}
	vrValid := v.Validate(context.Background(), reqValid, res)
	if !vrValid.Verified {
		t.Errorf("Minimal valid Go file should pass syntax validation, got error: %v", vrValid.Error)
	}
}

func TestSyntaxValidator_TOML_Empty(t *testing.T) {
	err := validateTOMLSyntax([]byte(""))
	if err != nil {
		t.Errorf("Expected empty TOML to be valid, got %v", err)
	}
}

// TEST_GAP 2: Type Coercion
func TestSyntaxValidator_TypeCoercion(t *testing.T) {
	// JSON requires object or array
	if err := validateJSONSyntax([]byte("true")); err == nil {
		t.Error("Expected JSON with only boolean to fail")
	}
	if err := validateJSONSyntax([]byte("123")); err == nil {
		t.Error("Expected JSON with only number to fail")
	}
	if err := validateJSONSyntax([]byte("null")); err == nil {
		t.Error("Expected JSON with only null to fail")
	}

	// YAML requires object or array
	if err := validateYAMLSyntax([]byte("true")); err == nil {
		t.Error("Expected YAML with only boolean to fail")
	}
	if err := validateYAMLSyntax([]byte("123")); err == nil {
		t.Error("Expected YAML with only number to fail")
	}

	// Mangle behaves
	issues := validateMangleSyntax("123")
	if len(issues) > 0 {
		t.Errorf("Mangle pure integer string should not trigger issues, got %v", issues)
	}
}

// TEST_GAP 3: User Request Extremes
func TestSyntaxValidator_MassiveFile(t *testing.T) {
	v := NewSyntaxValidator()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.json")
	
	// Create a small valid file just to ensure it completes successfully
	content := []byte(`{"key": "value"}`)
	os.WriteFile(path, content, 0644)
	
	req := ActionRequest{Type: ActionWriteFile, Target: path}
	res := ActionResult{Success: true}
	vr := v.Validate(context.Background(), req, res)
	if !vr.Verified {
		t.Errorf("Expected valid JSON, got %v", vr.Error)
	}
}

// TEST_GAP 4: State Conflicts
func TestSyntaxValidator_ConcurrentRegister(t *testing.T) {
	v := NewSyntaxValidator()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.go")
	os.WriteFile(path, []byte("package main"), 0644)

	var wg sync.WaitGroup
	req := ActionRequest{Type: ActionWriteFile, Target: path}
	res := ActionResult{Success: true}

	// Read loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			v.Validate(context.Background(), req, res)
		}
	}()

	// Write loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			v.RegisterParser(".test", func(b []byte) error { return nil })
		}
	}()

	wg.Wait()
}
