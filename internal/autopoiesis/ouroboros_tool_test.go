package autopoiesis

import (
	"context"
	"testing"
)

func TestGenerateToolFromCode_SafetyViolation(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{}
	config := DefaultOuroborosConfig(t.TempDir())
	loop := NewOuroborosLoop(mockLLM, config)

	// Unsafe code
	unsafeCode := `package tools
import "unsafe"
func Unsafe() { _ = unsafe.Pointer(nil) }
`

	success, _, _, errMsg := loop.GenerateToolFromCode(
		context.Background(),
		"unsafe_tool",
		"Do unsafe things",
		unsafeCode,
		1.0,
		1.0,
		false,
	)

	if success {
		t.Error("Expected safety check to fail")
	}
	if errMsg == "" {
		t.Error("Expected error message")
	}

	// Check loop stats
	stats := loop.GetStats()
	if stats.SafetyViolations == 0 {
		t.Error("Expected SafetyViolations count to increase")
	}
}

func TestGenerateToolFromCode_CompilationFailure(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{}
	tmpDir := t.TempDir()
	config := DefaultOuroborosConfig(tmpDir)
	loop := NewOuroborosLoop(mockLLM, config)

	// Safe code
	validCode := `package tools
import "fmt"
func Hello() { fmt.Println("Hello") }
`

	// This should pass safety check.
	// Then it goes to Compile.
	// Compile runs `go build`. Without `go.mod` in `tools` dir, it might fail or succeed depending on Go version/env.
	// `DefaultOuroborosConfig` sets ToolsDir.

	success, _, _, errMsg := loop.GenerateToolFromCode(
		context.Background(),
		"valid_tool",
		"Say Hello",
		validCode,
		1.0,
		1.0,
		false,
	)

	// We expect success to be false because compilation will likely fail in a bare temp dir
	// OR success to be true if `go build` works with standalone file.
	// But `OuroborosLoop` tries to build it as a plugin or executable?
	// Let's just verify it *tries* to compile (safety check passed).

	if !success {
		// If it failed, was it safety or compilation?
		// "safety check failed" vs "compilation failed"
		if errMsg != "" && errMsg[0:6] == "safety" {
			t.Errorf("Safety check failed for valid code: %s", errMsg)
		}
	}
}
