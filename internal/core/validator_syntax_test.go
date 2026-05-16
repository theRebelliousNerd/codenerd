package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
