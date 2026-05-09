package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TODO: TEST_GAP: Null/Undefined/Empty: Check `Validate` behavior in `SyntaxValidator` and `MangleSyntaxValidator` when `ActionRequest` has an empty target path or when `content` read from file is completely empty. Check `validateTOMLSyntax` behavior with empty byte arrays or arrays containing only empty strings.
// TODO: TEST_GAP: Type Coercion: Check `validateJSONSyntax` and `validateYAMLSyntax` behavior when parsing content containing only numbers, booleans, or null values instead of typical object/array structures. Check how `validateMangleSyntax` behaves with pure integer or boolean strings.
// TODO: TEST_GAP: User Request Extremes: Check `Validate` performance and memory consumption in `SyntaxValidator` with massive files (e.g., 500MB `.json` or `.go` files) to ensure `os.ReadFile` or `parserFunc` don't OOM or freeze. Check `validateMangleSyntax` with extremely long single-line strings.
// TODO: TEST_GAP: State Conflicts: Check concurrent execution of `RegisterParser` while `Validate` is actively being called by another goroutine, leading to potential data races on the `parsers` map. Check TOCTOU (Time of Check to Time of Use) file write race condition during the `os.ReadFile` check in `Validate`.

func TestSyntaxValidator_New(t *testing.T) {
	v := NewSyntaxValidator()
	if v == nil {
		t.Fatal("NewSyntaxValidator returned nil")
	}
	if v.parsers == nil {
		t.Fatal("Parsers map is nil")
	}
	// Check registered parsers
	if _, ok := v.parsers[".go"]; !ok {
		t.Error("Missing .go parser")
	}
	if _, ok := v.parsers[".json"]; !ok {
		t.Error("Missing .json parser")
	}
}

func TestSyntaxValidator_CanValidate(t *testing.T) {
	v := NewSyntaxValidator()

	testCases := []struct {
		action ActionType
		want   bool
	}{
		{ActionWriteFile, true},
		{ActionEditFile, true},
		{ActionListFiles, false},
	}

	for _, tc := range testCases {
		got := v.CanValidate(tc.action)
		if got != tc.want {
			t.Errorf("CanValidate(%v) = %v, want %v", tc.action, got, tc.want)
		}
	}
}

func TestSyntaxValidator_Go_Valid(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.go")
	content := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if !vr.Verified {
		t.Errorf("Expected valid Go to pass, got error: %s", vr.Error)
	}
}

func TestSyntaxValidator_Go_Invalid(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.go")
	content := `package main

func main() {
	println("unclosed
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if vr.Verified {
		t.Error("Expected invalid Go to fail")
	}
}

func TestSyntaxValidator_JSON_Valid(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")
	content := `{"key": "value", "num": 123}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if !vr.Verified {
		t.Errorf("Expected valid JSON to pass, got error: %s", vr.Error)
	}
}

func TestSyntaxValidator_JSON_Invalid(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	content := `{"key": "value",}` // trailing comma

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if vr.Verified {
		t.Error("Expected invalid JSON to fail")
	}
}

func TestSyntaxValidator_YAML_Valid(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")
	content := `key: value
list:
  - item1
  - item2
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if !vr.Verified {
		t.Errorf("Expected valid YAML to pass, got error: %s", vr.Error)
	}
}

func TestMangleSyntaxValidator_Valid(t *testing.T) {
	v := NewMangleSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.mg")
	content := `# Comment
Decl foo(Name).
foo("bar").
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if !vr.Verified {
		t.Errorf("Expected valid Mangle to pass, got error: %s", vr.Error)
	}
}

func TestMangleSyntaxValidator_MissingPeriod(t *testing.T) {
	v := NewMangleSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.mg")
	content := `Decl foo(Name)` // Missing period

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if vr.Verified {
		t.Error("Expected Mangle missing period to fail")
	}
}
