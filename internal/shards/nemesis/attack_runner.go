// Package nemesis implements the attack_runner - a sandboxed executor for
// generated chaos scripts that actively try to break code.
package nemesis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
)

// AttackRunner executes generated attack scripts in a sandboxed environment.
// It compiles and runs Go test files that probe for weaknesses.
type AttackRunner struct {
	workDir     string        // Temporary directory for attack scripts
	timeout     time.Duration // Max time per attack
	maxMemoryMB int           // Memory limit for attacks
	mu          sync.Mutex
	stats       AttackRunnerStats
}

// AttackRunnerStats tracks execution metrics.
type AttackRunnerStats struct {
	TotalRuns         int
	SuccessfulBreaks  int // Times the attack found a bug
	Timeouts          int
	Panics            int
	CleanPasses       int
	CompilationErrors int // Attacks that failed to compile (false negatives!)
}

// AttackScript represents a generated attack to execute.
type AttackScript struct {
	Name           string   `json:"name"`
	Category       string   `json:"category"` // concurrency, boundary, resource, nil_pointer
	TargetFile     string   `json:"target_file"`
	TargetFunction string   `json:"target_function"`
	TestCode       string   `json:"test_code"` // Go test code to execute
	Hypothesis     string   `json:"hypothesis"` // What we expect to break
	Inputs         []string `json:"inputs"`     // Malicious inputs to try
}

// AttackExecution is the result of running an attack script.
type AttackExecution struct {
	Script       *AttackScript `json:"script"`
	Success      bool          `json:"success"`       // true = attack found a bug
	BreakageType string        `json:"breakage_type"` // panic, timeout, assertion, race
	Output       string        `json:"output"`        // stdout/stderr
	Duration     time.Duration `json:"duration"`
	ExitCode     int           `json:"exit_code"`
	MemoryUsedKB int64         `json:"memory_used_kb"`
}

// NewAttackRunner creates a new attack runner with the given configuration.
func NewAttackRunner(timeout time.Duration, maxMemoryMB int) (*AttackRunner, error) {
	workDir, err := os.MkdirTemp("", "nemesis-attacks-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create attack workdir: %w", err)
	}

	return &AttackRunner{
		workDir:     workDir,
		timeout:     timeout,
		maxMemoryMB: maxMemoryMB,
	}, nil
}

// RunAttack executes a single attack script and returns the result.
func (r *AttackRunner) RunAttack(ctx context.Context, script *AttackScript) (*AttackExecution, error) {
	r.mu.Lock()
	r.stats.TotalRuns++
	r.mu.Unlock()

	logging.Shards("AttackRunner: Executing %s attack on %s", script.Category, script.TargetFunction)

	execution := &AttackExecution{
		Script:  script,
		Success: false,
	}

	startTime := time.Now()

	// Create attack directory
	attackDir := filepath.Join(r.workDir, fmt.Sprintf("attack_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(attackDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create attack dir: %w", err)
	}
	defer os.RemoveAll(attackDir) // Cleanup after execution

	// Write the test file
	testFile := filepath.Join(attackDir, "nemesis_attack_test.go")
	testCode := r.wrapTestCode(script)
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write test file: %w", err)
	}

	// Create go.mod for the attack
	goMod := fmt.Sprintf("module nemesis_attack\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(attackDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// FIX 1: Robust Import Management - run go mod tidy to resolve dependencies
	// This prevents false negatives from attacks that fail to compile due to missing imports
	tidyCtx, tidyCancel := context.WithTimeout(ctx, 30*time.Second)
	tidyCmd := exec.CommandContext(tidyCtx, "go", "mod", "tidy")
	tidyCmd.Dir = attackDir
	tidyCmd.Env = build.GetBuildEnvForTest(nil, attackDir)
	if tidyErr := tidyCmd.Run(); tidyErr != nil {
		logging.ShardsDebug("AttackRunner: go mod tidy failed (continuing anyway): %v", tidyErr)
	}
	tidyCancel()

	// Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// FIX 3: Probabilistic Race Detection - run test multiple times to catch flaky races
	// A single run of -race often misses subtle race conditions
	cmd := exec.CommandContext(execCtx, "go", "test", "-v", "-race", "-count=5", "-timeout", r.timeout.String())
	cmd.Dir = attackDir
	cmd.Env = build.GetBuildEnvForTest(nil, attackDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	execution.Duration = time.Since(startTime)
	execution.Output = stdout.String() + "\n" + stderr.String()

	// Analyze the result
	if execCtx.Err() == context.DeadlineExceeded {
		execution.Success = true
		execution.BreakageType = "timeout"
		execution.ExitCode = -1
		r.mu.Lock()
		r.stats.Timeouts++
		r.mu.Unlock()
		logging.Shards("AttackRunner: TIMEOUT - attack %s caused hang", script.Name)
	} else if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			execution.ExitCode = exitErr.ExitCode()
		}

		// Check what kind of failure
		output := execution.Output

		// FIX 2: Distinguish Compilation Failure from Test Failure
		// A compilation error is NOT a clean pass - it's a "fizzle" where the attack
		// couldn't even run. This is a false negative risk that must be tracked.
		if strings.Contains(output, "[build failed]") ||
			strings.Contains(output, "undefined:") ||
			strings.Contains(output, "cannot find package") ||
			strings.Contains(output, "could not import") ||
			strings.Contains(output, "syntax error") ||
			strings.Contains(output, "expected ") && strings.Contains(output, "found ") {
			execution.Success = false
			execution.BreakageType = "compilation_error"
			r.mu.Lock()
			r.stats.CompilationErrors++
			r.mu.Unlock()
			logging.Shards("AttackRunner: COMPILATION FAILED - attack %s could not compile (FALSE NEGATIVE RISK)", script.Name)
			// Do NOT count this as clean pass - the attack never actually ran
			return execution, nil
		}

		if strings.Contains(output, "panic:") {
			execution.Success = true
			execution.BreakageType = "panic"
			r.mu.Lock()
			r.stats.Panics++
			r.mu.Unlock()
			logging.Shards("AttackRunner: PANIC - attack %s triggered panic", script.Name)
		} else if strings.Contains(output, "DATA RACE") {
			execution.Success = true
			execution.BreakageType = "race"
			logging.Shards("AttackRunner: RACE - attack %s found data race", script.Name)
		} else if strings.Contains(output, "FAIL") {
			execution.Success = true
			execution.BreakageType = "assertion"
			logging.Shards("AttackRunner: ASSERTION - attack %s found bug", script.Name)
		} else {
			// Test failed but not in an interesting way
			execution.Success = false
			execution.BreakageType = "unknown_failure"
		}

		if execution.Success {
			r.mu.Lock()
			r.stats.SuccessfulBreaks++
			r.mu.Unlock()
		}
	} else {
		// Clean pass - attack didn't find anything
		execution.Success = false
		execution.BreakageType = "clean_pass"
		r.mu.Lock()
		r.stats.CleanPasses++
		r.mu.Unlock()
	}

	return execution, nil
}

// RunAttackBattery executes multiple attacks and returns all results.
func (r *AttackRunner) RunAttackBattery(ctx context.Context, scripts []*AttackScript) ([]*AttackExecution, error) {
	results := make([]*AttackExecution, 0, len(scripts))

	for _, script := range scripts {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		exec, err := r.RunAttack(ctx, script)
		if err != nil {
			logging.Shards("AttackRunner: Failed to run %s: %v", script.Name, err)
			continue
		}
		results = append(results, exec)

		// Fail fast on first successful break (optional)
		if exec.Success {
			logging.Shards("AttackRunner: Early exit - found breakage with %s", script.Name)
			// Continue to find more bugs rather than stopping
		}
	}

	return results, nil
}

// wrapTestCode wraps the attack script in a proper Go test structure.
func (r *AttackRunner) wrapTestCode(script *AttackScript) string {
	// If the script already has full test code, use it
	if strings.Contains(script.TestCode, "func Test") {
		return script.TestCode
	}

	// Otherwise, wrap it in a test function
	var sb strings.Builder
	sb.WriteString("package nemesis_attack\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"testing\"\n")
	sb.WriteString("\t\"time\"\n")
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("// %s\n", script.Hypothesis))
	sb.WriteString(fmt.Sprintf("// Category: %s\n", script.Category))
	sb.WriteString(fmt.Sprintf("// Target: %s\n\n", script.TargetFunction))

	sb.WriteString(fmt.Sprintf("func Test%s(t *testing.T) {\n", sanitizeFuncName(script.Name)))
	sb.WriteString("\t_ = time.Now() // prevent unused import\n\n")
	sb.WriteString("\t// Attack code:\n")

	// Indent each line of the test code
	for _, line := range strings.Split(script.TestCode, "\n") {
		sb.WriteString("\t" + line + "\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// GenerateAttackScripts uses the LLM to generate attack scripts for given targets.
func (r *AttackRunner) GenerateAttackScripts(
	ctx context.Context,
	llmComplete func(ctx context.Context, prompt string) (string, error),
	targetFile string,
	targetFunctions []string,
	sourceCode string,
) ([]*AttackScript, error) {
	logging.Shards("AttackRunner: Generating attacks for %s (%d functions)", targetFile, len(targetFunctions))

	prompt := r.buildAttackGenerationPrompt(targetFile, targetFunctions, sourceCode)

	response, err := llmComplete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM attack generation failed: %w", err)
	}

	scripts, err := r.parseAttackScripts(response)
	if err != nil {
		logging.Shards("AttackRunner: Failed to parse attack scripts: %v", err)
		return nil, err
	}

	logging.Shards("AttackRunner: Generated %d attack scripts", len(scripts))
	return scripts, nil
}

// buildAttackGenerationPrompt creates a prompt for the LLM to generate attack scripts.
func (r *AttackRunner) buildAttackGenerationPrompt(targetFile string, functions []string, sourceCode string) string {
	var sb strings.Builder

	sb.WriteString("# Nemesis Attack Script Generation\n\n")
	sb.WriteString("You are the Nemesis - a chaos engineer whose job is to BREAK code.\n")
	sb.WriteString("Generate Go test code that will expose bugs, crashes, and edge cases.\n\n")

	sb.WriteString("## Target File\n")
	sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", targetFile))

	sb.WriteString("## Functions to Attack\n")
	for _, fn := range functions {
		sb.WriteString(fmt.Sprintf("- %s\n", fn))
	}
	sb.WriteString("\n")

	sb.WriteString("## Source Code\n")
	sb.WriteString(fmt.Sprintf("```go\n%s\n```\n\n", sourceCode))

	sb.WriteString("## Attack Categories to Try\n")
	sb.WriteString("1. **nil_pointer**: Pass nil where non-nil is expected\n")
	sb.WriteString("2. **boundary**: Empty slices, max int, negative numbers, huge strings\n")
	sb.WriteString("3. **concurrency**: Race conditions, deadlocks (call from goroutines)\n")
	sb.WriteString("4. **resource**: Memory exhaustion, file handle leaks\n")
	sb.WriteString("5. **format**: Invalid UTF-8, special chars, injection attempts\n\n")

	sb.WriteString("## Output Format\n")
	sb.WriteString("Return a JSON array of attack scripts:\n")
	sb.WriteString("```json\n")
	sb.WriteString(`[
  {
    "name": "attack_name",
    "category": "nil_pointer|boundary|concurrency|resource|format",
    "target_function": "FunctionName",
    "hypothesis": "What we expect to break",
    "test_code": "// Go code that calls the function with malicious inputs\n..."
  }
]
`)
	sb.WriteString("```\n\n")

	sb.WriteString("Generate 3-5 attack scripts targeting the most suspicious code patterns.\n")
	sb.WriteString("Focus on: error handling gaps, unchecked inputs, race-prone patterns, implicit assumptions.\n")

	return sb.String()
}

// parseAttackScripts parses the LLM response into AttackScript structs.
func (r *AttackRunner) parseAttackScripts(response string) ([]*AttackScript, error) {
	// Clean up response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Find JSON array bounds
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	response = response[start : end+1]

	var scripts []*AttackScript
	if err := json.Unmarshal([]byte(response), &scripts); err != nil {
		return nil, fmt.Errorf("failed to parse attack scripts JSON: %w", err)
	}

	return scripts, nil
}

// GetStats returns the current runner statistics.
func (r *AttackRunner) GetStats() AttackRunnerStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

// Cleanup removes the work directory.
func (r *AttackRunner) Cleanup() error {
	return os.RemoveAll(r.workDir)
}

// sanitizeFuncName converts a name to a valid Go function name.
func sanitizeFuncName(name string) string {
	result := strings.Builder{}
	capitalize := true

	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if capitalize {
				result.WriteRune(r &^ 0x20) // Convert to uppercase
				capitalize = false
			} else {
				result.WriteRune(r)
			}
		} else {
			capitalize = true // Capitalize next letter after separator
		}
	}

	return result.String()
}

// FormatAttackReport creates a markdown report of attack results.
func FormatAttackReport(executions []*AttackExecution) string {
	var sb strings.Builder

	sb.WriteString("## Nemesis Attack Report\n\n")

	// Summary
	breaksFound := 0
	compilationErrors := 0
	for _, e := range executions {
		if e.Success {
			breaksFound++
		}
		if e.BreakageType == "compilation_error" {
			compilationErrors++
		}
	}

	if breaksFound > 0 {
		sb.WriteString(fmt.Sprintf("**VULNERABILITIES FOUND: %d**\n\n", breaksFound))
	} else {
		sb.WriteString("**No vulnerabilities found** - code survived the gauntlet.\n\n")
	}

	// Warn about compilation errors - these are false negative risks!
	if compilationErrors > 0 {
		sb.WriteString(fmt.Sprintf("**WARNING: %d attack(s) failed to compile** - these are potential false negatives!\n", compilationErrors))
		sb.WriteString("Consider reviewing the generated attack code for missing imports or syntax issues.\n\n")
	}

	// Details
	sb.WriteString("### Attack Results\n\n")
	for i, exec := range executions {
		status := "SURVIVED"
		if exec.Success {
			status = fmt.Sprintf("**BROKEN** (%s)", exec.BreakageType)
		} else if exec.BreakageType == "compilation_error" {
			status = "**FIZZLED** (compilation failed)"
		}

		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, exec.Script.Name, status))
		sb.WriteString(fmt.Sprintf("   - Category: %s\n", exec.Script.Category))
		sb.WriteString(fmt.Sprintf("   - Hypothesis: %s\n", exec.Script.Hypothesis))
		sb.WriteString(fmt.Sprintf("   - Duration: %v\n", exec.Duration.Round(time.Millisecond)))

		if exec.Success && exec.Output != "" {
			// Show relevant output for breaks
			sb.WriteString("   - Output:\n")
			lines := strings.Split(exec.Output, "\n")
			for _, line := range lines {
				if strings.Contains(line, "panic") || strings.Contains(line, "FAIL") ||
					strings.Contains(line, "race") || strings.Contains(line, "Error") {
					sb.WriteString(fmt.Sprintf("     ```\n     %s\n     ```\n", line))
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
