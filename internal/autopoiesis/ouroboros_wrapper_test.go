package autopoiesis

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRuntimeWrapper_LargeInput verifies that the runtime wrapper correctly handles inputs
// larger than the default bufio.Scanner buffer (64KB) and enforces a 10MB limit.
func TestRuntimeWrapper_LargeInput(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	config := OuroborosConfig{
		ToolsDir:       tmpDir,
		CompiledDir:    tmpDir,
		TargetOS:       "", // Use host defaults
		TargetArch:     "",
		CompileTimeout: 30 * time.Second,
	}

	tc := NewToolCompiler(config)

	// Create a dummy tool source that echoes input
	toolCode := `package main

import (
	"context"
)

// Echo echoes the input string
func Echo(ctx context.Context, input string) (string, error) {
	return input, nil
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "tool.go"), []byte(toolCode), 0644); err != nil {
		t.Fatalf("Failed to write tool.go: %v", err)
	}

	// Generate wrapper
	if err := tc.writeWrapper(tmpDir, "Echo"); err != nil {
		t.Fatalf("Failed to write wrapper: %v", err)
	}

	// Create go.mod to ensure build works in module mode
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module echo_tool\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Compile the tool
	outputPath := filepath.Join(tmpDir, "echo_tool")
	// Using "go build" directly as we are testing the generated code, not the Ouroboros build process itself
	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Compilation failed: %v\nOutput: %s", err, output)
	}

	// Helper to run the tool and return its output field
	runTool := func(inputPayload string) (string, error) {
		cmd := exec.Command(outputPath)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", err
		}

		go func() {
			defer stdin.Close()
			// Write input to stdin
			io.WriteString(stdin, inputPayload)
		}()

		// Capture stdout/stderr
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("run failed: %v\nOutput: %s", err, output)
		}

		// Parse JSON output from wrapper
		var res struct {
			Output string `json:"output"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(output, &res); err != nil {
			return "", fmt.Errorf("failed to parse output JSON: %v\nRaw output: %s", err, output)
		}

		if res.Error != "" {
			return "", fmt.Errorf("tool returned error: %s", res.Error)
		}

		return res.Output, nil
	}

	// Test Case 1: Small input (Sanity check)
	t.Run("SmallInput", func(t *testing.T) {
		payload := "hello world"
		// Wrapper expects raw string if not JSON, or JSON object with "input" field
		// Let's test JSON input format
		jsonInput := fmt.Sprintf(`{"input": "%s"}`, payload)
		out, err := runTool(jsonInput)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if out != payload {
			t.Errorf("Got %q, want %q", out, payload)
		}
	})

	// Test Case 2: Large input (> 64KB)
	// This verifies that we can handle inputs larger than scanner buffer
	t.Run("LargeInput_70KB", func(t *testing.T) {
		size := 70 * 1024 // 70KB
		payload := strings.Repeat("a", size)
		jsonInput := fmt.Sprintf(`{"input": "%s"}`, payload)

		out, err := runTool(jsonInput)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}
		if len(out) != size {
			t.Errorf("Got length %d, want %d", len(out), size)
		}
	})

	// Test Case 3: Excessive input (> 10MB)
	// This verifies that we enforce the 10MB limit
	t.Run("ExcessiveInput_Over10MB", func(t *testing.T) {
		limit := 10 * 1024 * 1024 // 10MB
		excess := 1024            // +1KB
		size := limit + excess
		// Use raw string to simplify check (JSON unmarshal would fail on truncated input, falling back to raw string)
		// The input will be truncated at 10MB
		payload := strings.Repeat("b", size)

		out, err := runTool(payload)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		// The input received by tool should be exactly 10MB
		if len(out) != limit {
			t.Errorf("Got length %d, want %d (10MB limit)", len(out), limit)
		}
	})
}
