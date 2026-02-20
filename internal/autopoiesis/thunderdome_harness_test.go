package autopoiesis

import (
	"context"
	"strings"
	"testing"
)

func TestThunderdome_GenerateTestHarness_PhantomPunchFix(t *testing.T) {
	// Setup
	td := NewThunderdome()
	entryPoint := "MySpecialFunction"

	// Execution
	// We can pass nil for GeneratedTool because the method signature uses _ for it
	harnessCode := td.generateTestHarness(nil, entryPoint)

	// Verification

	// 1. Check that the input variable is actually used in the function call
	expectedCall := "_, toolErr = MySpecialFunction(ctx, input)"
	if !strings.Contains(harnessCode, expectedCall) {
		t.Errorf("Harness code does not contain expected function call.\nExpected to contain: %s\nActual code snippet:\n%s",
			expectedCall, extractFunctionCall(harnessCode))
	}

	// 2. Check that input is NOT discarded (the bug was doing _ = input)
	if strings.Contains(harnessCode, "_ = input") {
		t.Error("Harness code explicitly discards input ('_ = input'), which is the Phantom Punch bug.")
	}

	// 3. Ensure stdin is read into input
	if !strings.Contains(harnessCode, "io.ReadAll(os.Stdin)") {
		t.Error("Harness code does not read stdin via io.ReadAll(os.Stdin).")
	}
}

func TestThunderdome_Behavioral_NewlineHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping behavioral test in short mode")
	}

	// Setup Thunderdome
	td := NewThunderdome()
	td.config.WorkDir = t.TempDir()

	// Create a tool that fails if input doesn't contain a newline
	toolCode := `package tools
import (
	"context"
	"fmt"
	"strings"
)

func CheckNewline(ctx context.Context, input string) (string, error) {
	if !strings.Contains(input, "\n") {
		return "", fmt.Errorf("input truncated, missing newline. Input: %q", input)
	}
	return "ok", nil
}
`
	tool := &GeneratedTool{
		Name: "newline_checker",
		Code: toolCode,
	}

	// Create an attack vector with a newline
	attacks := []AttackVector{
		{
			Name:     "Newline Input",
			Category: "formatting",
			Input:    "line1\nline2",
		},
	}

	// Run Battle
	ctx := context.Background()
	result, err := td.Battle(ctx, tool, attacks)
	if err != nil {
		t.Fatalf("Battle failed: %v", err)
	}

	// Verify survival
	if !result.Survived {
		t.Errorf("Tool failed behavioral test handling newlines. Result: %+v", result)
		for _, r := range result.Results {
			if !r.Survived {
				t.Logf("Failure: %s", r.Failure)
			}
		}
	}
}

// extractFunctionCall helps debug by finding the line where the tool is called
func extractFunctionCall(code string) string {
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		if strings.Contains(line, "toolErr =") {
			return strings.TrimSpace(line)
		}
	}
	return "Function call not found"
}

func TestThunderdome_Gaps(t *testing.T) {
	// TODO: TEST_GAP: Verify OOM detection reliability with a tool that allocates memory rapidly > 100ms interval.
	// TODO: TEST_GAP: Verify environment isolation (host env vars should not leak to tool).
	t.Skip("This test marks missing coverage for Thunderdome edge cases.")
}
