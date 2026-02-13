package autopoiesis

import (
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
	// TODO: TEST_GAP: Verify behavior when input exceeds the scanner's 10MB buffer (scanner.Scan() returns false).
	// TODO: TEST_GAP: Verify that inputs with newlines are not truncated by scanner.Scan().
	// TODO: TEST_GAP: Verify OOM detection reliability with a tool that allocates memory rapidly > 100ms interval.
	// TODO: TEST_GAP: Verify environment isolation (host env vars should not leak to tool).
	t.Skip("This test marks missing coverage for Thunderdome edge cases.")
}
