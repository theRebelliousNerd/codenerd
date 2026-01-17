package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// CODE ELEMENT TESTS
// =============================================================================

func TestCodeElement_Fields(t *testing.T) {
	t.Parallel()

	elem := CodeElement{
		Name:      "TestFunc",
		Type:      "function",
		File:      "test.go",
		StartLine: 10,
		EndLine:   20,
		Signature: "func TestFunc()",
	}

	if elem.Name != "TestFunc" {
		t.Errorf("Name mismatch: got %q", elem.Name)
	}
	if elem.Type != "function" {
		t.Errorf("Type mismatch: got %q", elem.Type)
	}
}

// =============================================================================
// GET ELEMENTS TOOL TESTS
// =============================================================================

func TestGetElementsTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GetElementsTool()

	if tool.Name != "get_elements" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Description should not be empty")
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
	if len(tool.Schema.Required) == 0 {
		t.Error("Required fields should be specified")
	}
}

func TestGetElementsTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeGetElements(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetElementsTool_Execute_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := executeGetElements(context.Background(), map[string]any{
		"path": "/nonexistent/file.go",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGetElementsTool_Execute_GoFile(t *testing.T) {
	t.Parallel()

	// Create a temp Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func Hello() {
    fmt.Println("hello")
}

type MyStruct struct {
    Name string
}

func (m *MyStruct) Method() {
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := executeGetElements(context.Background(), map[string]any{
		"path": goFile,
	})
	if err != nil {
		t.Fatalf("executeGetElements error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("expected to find Hello function")
	}
	if !strings.Contains(result, "MyStruct") {
		t.Error("expected to find MyStruct")
	}
}

func TestGetElementsTool_Execute_WithTypeFilter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func Func1() {}
func Func2() {}

type Struct1 struct {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := executeGetElements(context.Background(), map[string]any{
		"path": goFile,
		"type": "function",
	})
	if err != nil {
		t.Fatalf("executeGetElements error: %v", err)
	}

	if !strings.Contains(result, "Func1") {
		t.Error("expected to find Func1")
	}
	// Should not contain struct when filtering for functions
	if strings.Contains(result, `"type": "struct"`) {
		t.Error("should not find struct when filtering for function")
	}
}

// =============================================================================
// EXTRACT CODE ELEMENTS TESTS
// =============================================================================

func TestExtractCodeElements_Go(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func Standalone() {}

type MyInterface interface {
    Method()
}

type MyStruct struct {}

func (m *MyStruct) Method() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	elements, err := extractCodeElements(goFile)
	if err != nil {
		t.Fatalf("extractCodeElements error: %v", err)
	}

	if len(elements) < 3 {
		t.Errorf("expected at least 3 elements, got %d", len(elements))
	}

	// Check for expected types
	foundFunc := false
	foundStruct := false
	foundInterface := false
	for _, e := range elements {
		switch e.Type {
		case "function":
			foundFunc = true
		case "struct":
			foundStruct = true
		case "interface":
			foundInterface = true
		}
	}

	if !foundFunc {
		t.Error("expected to find function")
	}
	if !foundStruct {
		t.Error("expected to find struct")
	}
	if !foundInterface {
		t.Error("expected to find interface")
	}
}

func TestExtractCodeElements_Python(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	content := `class MyClass:
    def method(self):
        pass

def standalone_func():
    pass
`
	if err := os.WriteFile(pyFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	elements, err := extractCodeElements(pyFile)
	if err != nil {
		t.Fatalf("extractCodeElements error: %v", err)
	}

	if len(elements) < 2 {
		t.Errorf("expected at least 2 elements, got %d", len(elements))
	}
}

func TestExtractCodeElements_JavaScript(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")
	content := `function hello() {
    console.log("hello");
}

class MyClass {
    constructor() {}
}

const arrowFunc = (x) => x * 2;
`
	if err := os.WriteFile(jsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	elements, err := extractCodeElements(jsFile)
	if err != nil {
		t.Fatalf("extractCodeElements error: %v", err)
	}

	if len(elements) < 2 {
		t.Errorf("expected at least 2 elements, got %d", len(elements))
	}
}

// =============================================================================
// GET ELEMENT TOOL TESTS
// =============================================================================

func TestGetElementTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GetElementTool()

	if tool.Name != "get_element" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if len(tool.Schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(tool.Schema.Required))
	}
}

func TestGetElementTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeGetElement(context.Background(), map[string]any{
		"name": "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestGetElementTool_Execute_MissingName(t *testing.T) {
	t.Parallel()

	_, err := executeGetElement(context.Background(), map[string]any{
		"path": "/some/file.go",
	})
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestGetElementTool_Execute_Found(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func TargetFunc() {}
func OtherFunc() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := executeGetElement(context.Background(), map[string]any{
		"path": goFile,
		"name": "TargetFunc",
	})
	if err != nil {
		t.Fatalf("executeGetElement error: %v", err)
	}

	if !strings.Contains(result, "TargetFunc") {
		t.Error("expected result to contain TargetFunc")
	}
}

func TestGetElementTool_Execute_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func Existing() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := executeGetElement(context.Background(), map[string]any{
		"path": goFile,
		"name": "NonExistent",
	})
	if err == nil {
		t.Error("expected error for not found element")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("unexpected error: %v", err)
	}
}
