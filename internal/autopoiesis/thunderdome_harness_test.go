package autopoiesis

import (
	"context"
	"strings"
	"testing"
	"time"
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
		panic(fmt.Sprintf("input truncated, missing newline. Input: %q", input))
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
		{
			Name:     "Trailing Newline",
			Category: "formatting",
			Input:    "line1\n",
		},
		{
			Name:     "Multiple Newlines",
			Category: "formatting",
			Input:    "line1\nline2\nline3\n",
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
	// TODO: TEST_GAP: Verify behavior when input exceeds the scanner's 10MB buffer (scanner.Scan() returns false).
	t.Skip("This test marks missing coverage for Thunderdome edge cases.")
}

func TestThunderdome_OOM_Detection(t *testing.T) {
	// 1. Setup Thunderdome with strict memory limit (50MB) and sufficient timeout
	config := DefaultThunderdomeConfig()
	config.MaxMemoryMB = 50
	config.Timeout = 5 * time.Second
	// Use sequential attacks to ensure clean failure analysis
	config.ParallelAttacks = 1
	td := NewThunderdomeWithConfig(config)

	// 2. Define a tool that allocates memory rapidly
	// It allocates 10MB chunks every 10ms.
	// In 100ms (monitor interval), it should allocate ~100MB > 50MB.
	// The function signature matches what Thunderdome expects: (ctx, input) (string, error)
	toolCode := `package tools

import (
	"context"
	"time"
)

// RapidAllocator allocates memory until it crashes
func RapidAllocator(ctx context.Context, input string) (string, error) {
	var data [][]byte
	// Allocation loop
	for {
		// Allocate 10MB chunk
		chunk := make([]byte, 10*1024*1024)
		data = append(data, chunk)

		// Small sleep to allow monitor to catch it, but fast enough to OOM quickly
		time.Sleep(10 * time.Millisecond)

		// Check context
		select {
		case <-ctx.Done():
			return "cancelled", ctx.Err()
		default:
		}
	}
	return "done", nil
}
`
	// Create a dummy generated tool structure
	tool := &GeneratedTool{
		Name: "rapid_allocator",
		Code: toolCode,
	}

	// 3. Define attack
	attacks := []AttackVector{
		{
			Name:            "Memory Stress Test",
			Category:        "resource",
			Input:           "start",
			Description:     "Forces rapid memory allocation",
			ExpectedFailure: "oom",
		},
	}

	// 4. Run Battle
	ctx := context.Background()
	// Battle compiles the tool and runs the attack
	result, err := td.Battle(ctx, tool, attacks)
	if err != nil {
		t.Fatalf("Battle failed unexpectedly: %v", err)
	}

	// 5. Verification
	if result.Survived {
		t.Error("Tool should have been defeated by OOM, but it survived.")
	}

	if len(result.Results) == 0 {
		t.Fatal("No attack results returned.")
	}

	attackResult := result.Results[0]
	if attackResult.Failure != "oom" {
		t.Errorf("Expected failure mode 'oom', got '%s'. Output: %s", attackResult.Failure, attackResult.StackDump)
	}
}

func TestThunderdome_EnvironmentIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping isolation test in short mode")
	}

	// 1. Setup Host Environment with a secret
	secretKey := "THUNDERDOME_SECRET_KEY"
	secretVal := "super_secret_value_that_must_not_leak"
	t.Setenv(secretKey, secretVal)

	// 2. Setup Thunderdome
	td := NewThunderdome()
	td.config.WorkDir = t.TempDir()

	// 3. Create a tool that attempts to read the secret
	// Note: We panic if we find it, which causes a "failure" in Thunderdome terms.
	// So "Survival" means success (isolation worked).
	toolCode := `package tools

import (
	"context"
	"fmt"
	"os"
)

func CheckIsolation(ctx context.Context, input string) (string, error) {
	val := os.Getenv("THUNDERDOME_SECRET_KEY")
	if val != "" {
		panic(fmt.Sprintf("LEAK DETECTED: Found %s=%s", "THUNDERDOME_SECRET_KEY", val))
	}
	// Also verify that basic env vars like PATH are present (sanity check)
	// if os.Getenv("PATH") == "" {
	// 	return "error", fmt.Errorf("PATH is missing, environment is too sterile")
	// }
	return "secure", nil
}
`
	tool := &GeneratedTool{
		Name: "isolation_checker",
		Code: toolCode,
	}

	// 4. Define attack (dummy input)
	attacks := []AttackVector{
		{
			Name:     "Env Check",
			Category: "security",
			Input:    "check",
		},
	}

	// 5. Run Battle
	ctx := context.Background()
	result, err := td.Battle(ctx, tool, attacks)
	if err != nil {
		t.Fatalf("Battle failed: %v", err)
	}

	// 6. Verify Survival (Isolation)
	if !result.Survived {
		t.Errorf("Isolation test failed: Tool was able to access host environment variable.")
		for _, r := range result.Results {
			if !r.Survived {
				t.Logf("Failure Details: %s\nStack: %s", r.Failure, r.StackDump)
			}
		}
	}
}
