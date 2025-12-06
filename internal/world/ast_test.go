package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestASTParser_ParsePython(t *testing.T) {
	// Create a temporary Python file
	tmpDir := t.TempDir()
	pythonFile := filepath.Join(tmpDir, "test.py")

	pythonCode := `import os
import sys

class MyClass:
    def __init__(self):
        pass

    def public_method(self):
        pass

    def _protected_method(self):
        pass

def standalone_function():
    pass
`

	err := os.WriteFile(pythonFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewASTParser()
	defer parser.Close()

	facts, err := parser.Parse(pythonFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("Expected facts, got none")
	}

	// Verify we found the class and functions
	foundClass := false
	foundFunc := false
	foundImport := false

	for _, fact := range facts {
		if fact.Predicate == "symbol_graph" {
			if len(fact.Args) > 1 {
				symbolType := fact.Args[1].(string)
				if symbolType == "class" {
					foundClass = true
				} else if symbolType == "function" {
					foundFunc = true
				}
			}
		} else if fact.Predicate == "dependency_link" {
			foundImport = true
		}
	}

	if !foundClass {
		t.Error("Expected to find class definition")
	}
	if !foundFunc {
		t.Error("Expected to find function definition")
	}
	if !foundImport {
		t.Error("Expected to find import statement")
	}

	t.Logf("Found %d facts", len(facts))
}

func TestASTParser_ParseRust(t *testing.T) {
	tmpDir := t.TempDir()
	rustFile := filepath.Join(tmpDir, "test.rs")

	rustCode := `use std::io;

pub struct MyStruct {
    field: i32,
}

pub enum MyEnum {
    Variant1,
    Variant2,
}

pub fn public_function() {
    println!("Hello");
}

fn private_function() {
    println!("Private");
}

pub mod my_module {
    pub fn nested_function() {}
}
`

	err := os.WriteFile(rustFile, []byte(rustCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewASTParser()
	defer parser.Close()

	facts, err := parser.Parse(rustFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("Expected facts, got none")
	}

	// Verify we found structs, enums, functions
	foundStruct := false
	foundEnum := false
	foundFunc := false

	for _, fact := range facts {
		if fact.Predicate == "symbol_graph" && len(fact.Args) > 1 {
			symbolType := fact.Args[1].(string)
			if symbolType == "struct" {
				foundStruct = true
			} else if symbolType == "enum" {
				foundEnum = true
			} else if symbolType == "function" {
				foundFunc = true
			}
		}
	}

	if !foundStruct {
		t.Error("Expected to find struct definition")
	}
	if !foundEnum {
		t.Error("Expected to find enum definition")
	}
	if !foundFunc {
		t.Error("Expected to find function definition")
	}

	t.Logf("Found %d facts", len(facts))
}

func TestASTParser_ParseTypeScript(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "test.ts")

	tsCode := `import { Something } from './module';

export interface MyInterface {
    field: string;
}

export class MyClass {
    constructor() {}

    method(): void {}
}

export function publicFunction(): void {
    console.log("Hello");
}

const arrowFunction = () => {
    console.log("Arrow");
};

export type MyType = string | number;
`

	err := os.WriteFile(tsFile, []byte(tsCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewASTParser()
	defer parser.Close()

	facts, err := parser.Parse(tsFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("Expected facts, got none")
	}

	// Verify we found interfaces, classes, functions
	foundInterface := false
	foundClass := false
	foundFunc := false

	for _, fact := range facts {
		if fact.Predicate == "symbol_graph" && len(fact.Args) > 1 {
			symbolType := fact.Args[1].(string)
			if symbolType == "interface" {
				foundInterface = true
			} else if symbolType == "class" {
				foundClass = true
			} else if symbolType == "function" {
				foundFunc = true
			}
		}
	}

	if !foundInterface {
		t.Error("Expected to find interface definition")
	}
	if !foundClass {
		t.Error("Expected to find class definition")
	}
	if !foundFunc {
		t.Error("Expected to find function definition")
	}

	t.Logf("Found %d facts", len(facts))
}

func TestASTParser_ParseJavaScript(t *testing.T) {
	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")

	jsCode := `import something from './module';

export class MyClass {
    constructor() {}

    method() {}
}

export function publicFunction() {
    console.log("Hello");
}

const arrowFunction = () => {
    console.log("Arrow");
};

export const exportedArrow = () => {
    console.log("Exported arrow");
};
`

	err := os.WriteFile(jsFile, []byte(jsCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewASTParser()
	defer parser.Close()

	facts, err := parser.Parse(jsFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("Expected facts, got none")
	}

	// Verify we found classes and functions
	foundClass := false
	foundFunc := false

	for _, fact := range facts {
		if fact.Predicate == "symbol_graph" && len(fact.Args) > 1 {
			symbolType := fact.Args[1].(string)
			if symbolType == "class" {
				foundClass = true
			} else if symbolType == "function" {
				foundFunc = true
			}
		}
	}

	if !foundClass {
		t.Error("Expected to find class definition")
	}
	if !foundFunc {
		t.Error("Expected to find function definition")
	}

	t.Logf("Found %d facts", len(facts))
}
