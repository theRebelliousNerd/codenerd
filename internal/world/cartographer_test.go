package world

import (
	"codenerd/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCartographer(t *testing.T) {
	c := NewCartographer()
	if c == nil {
		t.Fatal("NewCartographer() returned nil")
	}
	if c.dataFlowExtractor == nil {
		t.Error("NewCartographer() did not initialize dataFlowExtractor")
	}
}

func TestCartographer_MapFile_Unsupported(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	facts, err := c.MapFile("test.txt")
	if err != nil {
		t.Errorf("Expected nil error for unsupported file, got %v", err)
	}
	if facts != nil {
		t.Errorf("Expected nil facts for unsupported file, got %v", facts)
	}
}

func TestCartographer_MapFile_Go(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	// Create a temporary Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	// A simple program that should emit some predictable facts
	content := []byte(`package main

type MyStruct struct {}

func hello() {
	println("hello")
}

func (m *MyStruct) method() {
	hello()
}
`)
	if err := os.WriteFile(goFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	facts, err := c.MapFile(goFile)
	if err != nil {
		t.Errorf("Expected nil error for Go file, got %v", err)
	}
	if len(facts) == 0 {
		t.Error("Expected facts for Go file, got empty")
	}

	// Verify we got the expected facts
	foundHello := false
	foundStruct := false
	foundMethod := false
	foundCall := false
	foundDualCall := false

	for _, f := range facts {
		if f.Predicate == "code_defines" {
			id, ok1 := f.Args[1].(string)
			typeAtom, ok2 := f.Args[2].(core.MangleAtom)

			if ok1 && ok2 {
				typeStr := string(typeAtom)

				if id == "main.hello" && typeStr == "/function" {
					foundHello = true
				} else if id == "main.MyStruct" && typeStr == "/struct" {
					foundStruct = true
				} else if id == "main.MyStruct.method" && typeStr == "/function" {
					foundMethod = true
				}
			}
		} else if f.Predicate == "code_calls" {
			caller, ok1 := f.Args[0].(string)
			callee, ok2 := f.Args[1].(string)

			if ok1 && ok2 {
				if caller == "main.MyStruct.method" && callee == "main.hello" {
					foundCall = true
				}
				if caller == "fn:main.MyStruct.method" && callee == "fn:main.hello" {
					foundDualCall = true
				}
			}
		}
	}

	if !foundHello {
		t.Error("Expected to find code_defines fact for main.hello")
	}
	if !foundStruct {
		t.Error("Expected to find code_defines fact for main.MyStruct")
	}
	if !foundMethod {
		t.Error("Expected to find code_defines fact for main.MyStruct.method")
	}
	if !foundCall {
		t.Error("Expected to find code_calls fact from main.MyStruct.method to main.hello")
	}
	if !foundDualCall {
		t.Error("Expected to find dual code_calls fact from fn:main.MyStruct.method to fn:main.hello")
	}
}

func TestCartographer_MapFile_Go_Error(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	// Parse non-existent file
	_, err := c.MapFile("does_not_exist.go")
	if err == nil {
		t.Error("Expected error for non-existent Go file, got nil")
	}
}
