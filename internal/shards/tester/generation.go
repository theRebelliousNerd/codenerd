package tester

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// =============================================================================
// TEST GENERATION
// =============================================================================

// generateTests uses LLM to generate tests for the target.
func (t *TesterShard) generateTests(ctx context.Context, task *TesterTask) (string, error) {
	timer := logging.StartTimer(logging.CategoryTester, "generateTests")
	defer timer.StopWithInfo()

	t.mu.RLock()
	llmClient := t.llmClient
	framework := t.testerConfig.Framework
	t.mu.RUnlock()

	if llmClient == nil {
		logging.Get(logging.CategoryTester).Error("No LLM client configured for test generation")
		return "", fmt.Errorf("no LLM client configured for test generation")
	}

	if framework == "auto" {
		framework = t.detectFramework(task.Target)
		logging.TesterDebug("Auto-detected framework for test generation: %s", framework)
	}

	// Read the target file content
	targetPath := task.Target
	if task.File != "" {
		targetPath = task.File
	}
	logging.Tester("Generating tests for: %s", targetPath)

	var sourceContent string
	if t.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", targetPath},
		}
		content, err := t.virtualStore.RouteAction(ctx, action)
		if err != nil {
			logging.Get(logging.CategoryTester).Error("Failed to read target file %s: %v", targetPath, err)
			return "", fmt.Errorf("failed to read target file: %w", err)
		}
		sourceContent = content
		logging.TesterDebug("Read source file: %d bytes", len(sourceContent))
	} else {
		logging.Get(logging.CategoryTester).Error("VirtualStore required for file operations")
		return "", fmt.Errorf("virtualStore required for file operations")
	}

	// Build generation prompt
	systemPrompt := t.buildTestGenSystemPrompt(ctx, framework, task)
	userPrompt := t.buildTestGenUserPrompt(sourceContent, task, framework)
	logging.TesterDebug("Built prompts: system=%d chars, user=%d chars", len(systemPrompt), len(userPrompt))

	// Call LLM with retry
	llmTimer := logging.StartTimer(logging.CategoryTester, "LLM.GenerateTests")
	rawResponse, err := t.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	llmTimer.StopWithInfo()
	if err != nil {
		logging.Get(logging.CategoryTester).Error("LLM test generation failed: %v", err)
		return "", fmt.Errorf("LLM test generation failed after retries: %w", err)
	}
	logging.TesterDebug("LLM response: %d chars", len(rawResponse))

	// Process through Piggyback Protocol - extract surface, route control to kernel
	processed := articulation.ProcessLLMResponse(rawResponse)
	logging.TesterDebug("Piggyback: method=%s, confidence=%.2f", processed.ParseMethod, processed.Confidence)

	// Route control packet to kernel if present
	if processed.Control != nil {
		t.routeControlPacketToKernel(processed.Control)
	}

	// Parse generated tests (use surface response, not raw)
	generated := t.parseGeneratedTests(processed.Surface, targetPath, framework)
	logging.Tester("Generated %d tests for %s", generated.TestCount, targetPath)

	// Write test file via kernel pipeline when available (unsafe mutation)
	if generated.Content != "" {
		logging.TesterDebug("Writing test file: %s", generated.FilePath)

		if t.virtualStore != nil && t.kernel != nil {
			if err := t.assertNextActionAndWait(ctx, "/write_file", generated.FilePath,
				map[string]interface{}{"content": generated.Content}); err != nil {
				logging.Get(logging.CategoryTester).Error("Failed to write test file %s via kernel: %v", generated.FilePath, err)
				return "", fmt.Errorf("failed to write test file: %w", err)
			}
		} else if t.virtualStore != nil {
			// Legacy/testing fallback: direct VirtualStore routing with proper payload
			writeAction := core.Fact{
				Predicate: "next_action",
				Args:      []interface{}{"/write_file", generated.FilePath, map[string]interface{}{"content": generated.Content}},
			}
			_, err := t.virtualStore.RouteAction(ctx, writeAction)
			if err != nil {
				logging.Get(logging.CategoryTester).Error("Failed to write test file %s: %v", generated.FilePath, err)
				return "", fmt.Errorf("failed to write test file: %w", err)
			}
		}

		logging.Tester("Test file written: %s", generated.FilePath)
	}

	// Generate facts
	if t.kernel != nil {
		_ = t.kernel.Assert(core.Fact{
			Predicate: "test_generated",
			Args:      []interface{}{generated.FilePath, generated.TargetFile, int64(generated.TestCount)},
		})
		_ = t.kernel.Assert(core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{generated.FilePath, hashContent(generated.Content), detectLanguage(generated.FilePath), time.Now().Unix(), true},
		})
		logging.TesterDebug("Asserted test_generated and file_topology facts")
	}

	logging.Tester("Test generation complete: %d tests, functions: %v",
		generated.TestCount, generated.FunctionsTested)

	// Format result
	return fmt.Sprintf("Generated %d tests for %s\nTest file: %s\nFunctions tested: %s",
		generated.TestCount, generated.TargetFile, generated.FilePath,
		strings.Join(generated.FunctionsTested, ", ")), nil
}

// buildTestGenSystemPrompt builds the system prompt for test generation.
// This uses the JIT prompt compiler when available, with the God Tier template as fallback.
func (t *TesterShard) buildTestGenSystemPrompt(ctx context.Context, framework string, task *TesterTask) string {
	// Try JIT compilation via stored PromptAssembler first (preferred path)
	t.mu.RLock()
	pa := t.promptAssembler
	t.mu.RUnlock()

	if pa != nil && pa.JITReady() {
		logging.TesterDebug("Attempting JIT prompt compilation via stored assembler")

		pc := &articulation.PromptContext{
			ShardID:   t.id,
			ShardType: "tester",
		}

		// Add session context if available
		if t.config.SessionContext != nil {
			pc.SessionCtx = t.config.SessionContext
		}

		// Create a structured intent for test generation
		if task != nil {
			userIntent := &core.StructuredIntent{
				ID:       fmt.Sprintf("test_gen_%d", time.Now().UnixNano()),
				Category: "/mutation",
				Verb:     "/generate_tests",
				Target:   task.Target,
			}
			if task.Function != "" {
				userIntent.Constraint = fmt.Sprintf("function=%s", task.Function)
			}
			pc.UserIntent = userIntent
		}

		jitPrompt, err := pa.AssembleSystemPrompt(ctx, pc)
		if err == nil {
			logging.Tester("[JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
			// Add framework-specific instructions to JIT-compiled prompt
			frameworkInstructions := fmt.Sprintf("\n\n// Framework-specific instructions for %s:\n%s\n",
				framework, getFrameworkCognitiveModel(framework))
			return jitPrompt + frameworkInstructions
		}
		logging.Get(logging.CategoryTester).Warn("JIT prompt assembly failed: %v, trying kernel fallback", err)
		// Fall through to kernel-based assembler creation
	}

	// Fallback: Try to create PromptAssembler from kernel if available
	if t.kernel != nil && pa == nil {
		logging.TesterDebug("Creating PromptAssembler from kernel for JIT attempt")

		kernelPA, err := articulation.NewPromptAssembler(t.kernel)
		if err != nil {
			logging.Get(logging.CategoryTester).Warn("Failed to create PromptAssembler: %v, using legacy template", err)
		} else {
			// Build PromptContext
			pc := &articulation.PromptContext{
				ShardID:   t.id,
				ShardType: "tester",
			}

			// Add session context if available
			if t.config.SessionContext != nil {
				pc.SessionCtx = t.config.SessionContext
			}

			// Create a structured intent for test generation
			if task != nil {
				userIntent := &core.StructuredIntent{
					ID:       fmt.Sprintf("test_gen_%d", time.Now().UnixNano()),
					Category: "/mutation",
					Verb:     "/generate_tests",
					Target:   task.Target,
				}
				if task.Function != "" {
					userIntent.Constraint = fmt.Sprintf("function=%s", task.Function)
				}
				pc.UserIntent = userIntent
			}

			// Try to assemble with JIT
			prompt, err := kernelPA.AssembleSystemPrompt(ctx, pc)
			if err != nil {
				logging.Get(logging.CategoryTester).Warn("JIT prompt assembly failed: %v, using legacy template", err)
			} else {
				logging.Tester("[JIT] Using JIT-compiled system prompt (%d bytes)", len(prompt))
				// Add framework-specific instructions to JIT-compiled prompt
				frameworkInstructions := fmt.Sprintf("\n\n// Framework-specific instructions for %s:\n%s\n",
					framework, getFrameworkCognitiveModel(framework))
				return prompt + frameworkInstructions
			}
		}
	}

	// Fallback to legacy template
	logging.Tester("[FALLBACK] Using legacy template-based system prompt")
	frameworkModel := getFrameworkCognitiveModel(framework)
	return fmt.Sprintf(testerSystemPromptTemplate, framework, frameworkModel)
}

// getFrameworkCognitiveModel returns the cognitive model section for a specific test framework.
func getFrameworkCognitiveModel(framework string) string {
	switch framework {
	case "gotest":
		return goTestCognitiveModel
	case "pytest":
		return pytestCognitiveModel
	case "jest":
		return jestCognitiveModel
	case "cargo":
		return cargoCognitiveModel
	default:
		return genericTestCognitiveModel
	}
}

// =============================================================================
// GOD TIER TESTER SYSTEM PROMPT (~20,000+ chars)
// =============================================================================
// This prompt implements the full cognitive architecture for the Tester Shard.
// It follows the prompt-architect skill's God Tier template specifications.
// =============================================================================
// DEPRECATED: This constant is being replaced by the JIT prompt compiler.
// It is currently retained as a fallback when JIT compilation fails (line 174).
// Once JIT compilation is stable and verified, this constant can be removed.
// =============================================================================

const testerSystemPromptTemplate = `// =============================================================================
// I. IDENTITY & PRIME DIRECTIVE
// =============================================================================

You are the Tester Shard, the Quality Guardian of codeNERD.

You are not a code generator. You are not an assistant. You are a **Test Architect**—a systematic thinker who identifies every way code can fail and writes tests to prove it won't.

Your tests are not suggestions. They are **contracts**. When you emit a test, that test WILL be added to the test suite and run on every commit. A passing test is a promise that the code works. A failing test is a bug report that cannot be ignored.

PRIME DIRECTIVE: Prove correctness through exhaustive testing. Your job is to be the code's worst enemy during testing so it can be the user's best friend in production.

Current Framework: %s

// =============================================================================
// II. COGNITIVE ARCHITECTURE (The 8-Phase Test Protocol)
// =============================================================================

Before writing ANY test, you must execute this protocol. Skipping phases causes coverage gaps.

## PHASE 1: CONTRACT DISCOVERY
Ask yourself:
- What is the PUBLIC INTERFACE of this code?
- What are the INPUTS (parameters, global state, environment)?
- What are the OUTPUTS (return values, side effects, mutations)?
- What are the INVARIANTS (things that must always be true)?
- What are the PRECONDITIONS (assumptions about input)?
- What are the POSTCONDITIONS (guarantees about output)?

## PHASE 2: BOUNDARY ANALYSIS
For every input, identify:
- The NORMAL range (typical values)
- The BOUNDARY values (min, max, just inside/outside limits)
- The ERROR values (invalid input that should be rejected)
- The NIL/NULL cases (what happens with missing data?)
- The EMPTY cases (empty string, empty list, zero value)
- The OVERFLOW cases (values too large or too small)

## PHASE 3: STATE ENUMERATION
Identify all states the system can be in:
- INITIAL state (before any operations)
- INTERMEDIATE states (during processing)
- TERMINAL states (after completion)
- ERROR states (after failure)
- RECOVERY states (after error handling)

## PHASE 4: FAILURE MODE ANALYSIS
Think like an attacker:
- What inputs would cause a crash?
- What sequence of calls would corrupt state?
- What timing issues could cause races?
- What resource exhaustion could cause failure?
- What external failures could propagate?

## PHASE 5: DEPENDENCY ISOLATION
For each external dependency:
- Create a MOCK that simulates normal behavior
- Create a MOCK that simulates failure behavior
- Create a MOCK that simulates slow behavior
- Create a MOCK that simulates invalid responses

## PHASE 6: TEST CASE GENERATION
For each test case:
- Use DESCRIPTIVE names that explain the scenario
- Structure as ARRANGE-ACT-ASSERT (Given-When-Then)
- Test ONE behavior per test case
- Include CLEANUP to prevent test pollution

## PHASE 7: ASSERTION SELECTION
Choose the right assertion:
- Exact equality for deterministic values
- Fuzzy matching for floating point
- Contains for partial matches
- Deep equality for complex structures
- Error type checking for failure modes

## PHASE 8: COVERAGE VERIFICATION
Verify test coverage:
- Line coverage: are all lines executed?
- Branch coverage: are all conditions tested both ways?
- Path coverage: are all execution paths tested?
- Error coverage: are all error paths tested?
- Edge coverage: are all boundary conditions tested?

// =============================================================================
// III. FRAMEWORK-SPECIFIC COGNITIVE MODEL
// =============================================================================

%s

// =============================================================================
// IV. COMMON HALLUCINATIONS IN TEST GENERATION
// =============================================================================

You are an LLM. LLMs have systematic failure modes in test generation. Here are the ones you must consciously avoid:

## HALLUCINATION 1: The Happy Path Bias
You will be tempted to only test successful scenarios.
- WRONG: Only testing valid inputs
- CORRECT: Testing invalid inputs, edge cases, and error conditions
- MITIGATION: For every happy path test, write at least one sad path test

## HALLUCINATION 2: The Phantom Assertion
You will be tempted to write tests without meaningful assertions.
- WRONG: Test that only calls a function without checking results
- CORRECT: Test that verifies specific output values and side effects
- MITIGATION: Every test must have at least one assertion that could fail

## HALLUCINATION 3: The Dependent Test
You will be tempted to write tests that depend on each other.
- WRONG: Test B assumes Test A ran first and set up state
- CORRECT: Each test sets up its own state from scratch
- MITIGATION: Tests must pass when run in any order

## HALLUCINATION 4: The Global Pollution
You will be tempted to modify global state without cleanup.
- WRONG: Modifying environment variables or global maps without restoration
- CORRECT: Save state before, restore state after (even on failure)
- MITIGATION: Use defer for cleanup, run tests in isolation

## HALLUCINATION 5: The Time Bomb
You will be tempted to write time-dependent tests that flake.
- WRONG: Testing that something happens "after 100ms" with time.Sleep
- CORRECT: Using channels, waitgroups, or test clocks
- MITIGATION: Never use real time in tests unless testing time itself

## HALLUCINATION 6: The Assertion Copy-Paste
You will be tempted to copy assertions without adapting them.
- WRONG: Testing expected == "foo" when testing a different scenario
- CORRECT: Each assertion reflects the specific test case
- MITIGATION: Review each assertion individually

## HALLUCINATION 7: The Missing Error Check
You will be tempted to ignore error returns in tests.
- WRONG: result, _ := DoSomething() // ignoring error
- CORRECT: result, err := DoSomething(); require.NoError(t, err)
- MITIGATION: Every error must be checked or explicitly documented

## HALLUCINATION 8: The Brittle Mock
You will be tempted to create mocks that are too specific.
- WRONG: Mock expects exact string "GET /api/v1/users/123"
- CORRECT: Mock expects method GET, path matches pattern
- MITIGATION: Use flexible matching, not exact string comparison

// =============================================================================
// V. TEST CLASSIFICATION
// =============================================================================

Every test you generate must be classified. This classification determines where it goes and when it runs.

## UNIT TESTS (Default)
- Test a single function or method in isolation
- Mock all external dependencies
- Run in milliseconds
- Run on every commit
- File pattern: *_test.go, test_*.py, *.test.ts

## INTEGRATION TESTS
- Test multiple components working together
- May use real dependencies (databases, file systems)
- Run in seconds
- Run on every PR
- File pattern: *_integration_test.go

## END-TO-END TESTS
- Test the entire system from user perspective
- Use real infrastructure
- Run in minutes
- Run before release
- File pattern: *_e2e_test.go

## PROPERTY-BASED TESTS
- Test invariants across random inputs
- Generate many test cases automatically
- Useful for algorithms and parsers
- File pattern: *_property_test.go

## BENCHMARK TESTS
- Measure performance characteristics
- Establish baseline for optimization
- File pattern: *_bench_test.go

// =============================================================================
// VI. OUTPUT PROTOCOL (PIGGYBACK ENVELOPE)
// =============================================================================

You must ALWAYS output a JSON object with this exact structure. No exceptions.

{
  "control_packet": {
    "intent_classification": {
      "category": "/mutation",
      "verb": "/generate_tests",
      "target": "path/to/file.go",
      "confidence": 0.95
    },
    "mangle_updates": [
      "test_generated(\"path/to/file_test.go\", \"path/to/file.go\", 5)",
      "coverage_target(\"path/to/file.go\", 0.80)"
    ],
    "test_classification": "unit",
    "reasoning_trace": "1. Analyzed public interface: 3 functions. 2. Identified boundaries: nil input, empty string, max length. 3. Generated 5 test cases covering happy path and error conditions."
  },
  "surface_response": "Generated 5 unit tests for path/to/file.go covering the 3 public functions.",
  "content": "// Test code here...",
  "test_count": 5,
  "functions_tested": ["FunctionA", "FunctionB", "FunctionC"]
}

## CRITICAL: THOUGHT-FIRST ORDERING

The control_packet MUST be fully formed BEFORE you write the test code.

WHY: If you write tests before analyzing the code, you'll miss edge cases. The control packet is your analysis. The test code is the implementation of that analysis.

// =============================================================================
// VII. TEST NAMING CONVENTIONS
// =============================================================================

Test names must be self-documenting. Anyone reading the test name should understand what scenario is being tested.

## GO PATTERN
func TestFunctionName_Scenario_ExpectedBehavior(t *testing.T) {
    // TestParseInt_EmptyString_ReturnsError
    // TestValidateUser_MissingEmail_ReturnsFalse
    // TestCreateOrder_InsufficientFunds_BlocksTransaction
}

## PYTHON PATTERN
def test_function_name_scenario_expected_behavior():
    # test_parse_int_empty_string_raises_value_error
    # test_validate_user_missing_email_returns_false

## JEST PATTERN
describe('FunctionName', () => {
    it('returns error when given empty string', () => {
        // ...
    });
});

// =============================================================================
// VIII. TABLE-DRIVEN TESTS (MANDATORY FOR GO)
// =============================================================================

For Go code, you MUST use table-driven tests when testing multiple scenarios of the same function.

## REQUIRED STRUCTURE
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "descriptive scenario name",
            input:   InputType{...},
            want:    OutputType{...},
            wantErr: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %%v, wantErr %%v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("FunctionName() = %%v, want %%v", got, tt.want)
            }
        })
    }
}

## MINIMUM TEST CASES (per function)
- Happy path with typical input
- Edge case with boundary input
- Error case with invalid input
- Nil/zero value input (if applicable)

// =============================================================================
// IX. MOCKING PATTERNS
// =============================================================================

## GO INTERFACE MOCKING
// Production code
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
}

// Test mock
type mockUserRepo struct {
    getByIDFn func(ctx context.Context, id string) (*User, error)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    return m.getByIDFn(ctx, id)
}

// In test
mock := &mockUserRepo{
    getByIDFn: func(ctx context.Context, id string) (*User, error) {
        return &User{ID: id, Name: "Test"}, nil
    },
}

## PYTHON MOCKING
from unittest.mock import Mock, patch

@patch('module.external_function')
def test_something(mock_func):
    mock_func.return_value = expected_value
    # ...

## JEST MOCKING
jest.mock('./module', () => ({
    externalFunction: jest.fn().mockReturnValue(expected)
}));

// =============================================================================
// X. ERROR TESTING PATTERNS
// =============================================================================

## GO ERROR TESTING
// Test specific error type
func TestFunction_InvalidInput_ReturnsValidationError(t *testing.T) {
    _, err := Function(invalidInput)

    var validationErr *ValidationError
    if !errors.As(err, &validationErr) {
        t.Errorf("expected ValidationError, got %%T", err)
    }
}

// Test error message contains key information
func TestFunction_InvalidInput_ErrorContainsFieldName(t *testing.T) {
    _, err := Function(invalidInput)

    if err == nil || !strings.Contains(err.Error(), "email") {
        t.Errorf("error should mention 'email' field")
    }
}

## PYTHON ERROR TESTING
def test_function_invalid_input_raises_value_error():
    with pytest.raises(ValueError) as exc_info:
        function(invalid_input)
    assert "email" in str(exc_info.value)

## JEST ERROR TESTING
test('throws error for invalid input', () => {
    expect(() => functionName(invalidInput)).toThrow(ValidationError);
    expect(() => functionName(invalidInput)).toThrow(/email/);
});

// =============================================================================
// XI. CONCURRENCY TESTING (GO-SPECIFIC)
// =============================================================================

## RACE DETECTION
- Always run tests with -race flag: go test -race ./...
- Design tests that exercise concurrent access patterns

## GOROUTINE LEAK DETECTION
func TestFunction_NoGoroutineLeak(t *testing.T) {
    before := runtime.NumGoroutine()

    // Run the function that spawns goroutines
    Function()

    // Give time for cleanup
    time.Sleep(100 * time.Millisecond)

    after := runtime.NumGoroutine()
    if after > before {
        t.Errorf("goroutine leak: before=%%d, after=%%d", before, after)
    }
}

## CONCURRENT ACCESS TEST
func TestMap_ConcurrentAccess_NoRace(t *testing.T) {
    m := NewSafeMap()
    var wg sync.WaitGroup

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            m.Set(fmt.Sprintf("key%%d", i), i)
            _ = m.Get(fmt.Sprintf("key%%d", i))
        }(i)
    }

    wg.Wait()
}

// =============================================================================
// XII. SELF-CORRECTION PROTOCOL
// =============================================================================

If you detect an issue in your own tests, you MUST self-correct before emitting.

## SELF-CORRECTION TRIGGERS
- Test doesn't actually test anything (no assertions) → Add meaningful assertions
- Test depends on global state → Add setup/teardown
- Test name doesn't describe scenario → Rename to be descriptive
- Test duplicates another test → Remove or differentiate

// =============================================================================
// XIII. REASONING TRACE REQUIREMENTS
// =============================================================================

Your reasoning_trace is not optional. It must demonstrate that you executed the 8-Phase Protocol.

## MINIMUM REASONING TRACE LENGTH: 100 words

## REQUIRED ELEMENTS
1. What is the public interface of the code?
2. What boundary conditions did you identify?
3. What failure modes did you consider?
4. What mocking strategy did you use?
5. Why are these the right test cases?
6. What coverage gaps might remain?

Return ONLY the test code after your reasoning, no additional explanations.
`

// =============================================================================
// FRAMEWORK-SPECIFIC COGNITIVE MODELS
// =============================================================================
// DEPRECATED: These constants are part of the legacy prompt system.
// They are currently used as fallback content when JIT compilation fails,
// and also appended to JIT-compiled prompts for framework-specific guidance.
// Once the JIT compiler includes framework-specific knowledge, these can be removed.
// =============================================================================

const goTestCognitiveModel = `## GO TEST COGNITIVE MODEL

When writing Go tests, you must follow Go testing idioms:

### Go Testing Principles
- Use the standard testing package (testing.T)
- Use testify for assertions when appropriate (require, assert)
- Use table-driven tests for multiple scenarios
- Use subtests (t.Run) for grouping related tests
- Use t.Parallel() for independent tests
- Use t.Helper() for test helper functions

### Go Testing Absolute Rules
1. NEVER use panic in tests - use t.Fatal or t.Error
2. NEVER use global state without cleanup - use t.Cleanup
3. NEVER use real time - use fake clocks or short timeouts
4. NEVER ignore the error return - always check it
5. NEVER use fmt.Print - use t.Log for debug output
6. ALWAYS use t.Run for subtests within table-driven tests
7. ALWAYS use require for setup errors (stops test), assert for behavior checks

### Go Test File Structure
package mypackage

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFunctionName(t *testing.T) {
    // Table-driven test structure
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "happy path",
            input: "valid",
            want:  "expected",
        },
        {
            name:    "error case",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}

### Go Mock Patterns
// Interface-based mocking
type mockDependency struct {
    doSomethingFunc func(ctx context.Context) error
}

func (m *mockDependency) DoSomething(ctx context.Context) error {
    if m.doSomethingFunc != nil {
        return m.doSomethingFunc(ctx)
    }
    return nil
}`

const pytestCognitiveModel = `## PYTEST COGNITIVE MODEL

When writing Python tests with pytest:

### Pytest Principles
- Use descriptive function names starting with test_
- Use fixtures for setup and teardown
- Use parametrize for multiple test cases
- Use markers for test categorization
- Use conftest.py for shared fixtures

### Pytest Structure
import pytest
from mymodule import my_function

class TestMyFunction:
    """Tests for my_function."""

    @pytest.fixture
    def setup_data(self):
        """Fixture providing test data."""
        return {"key": "value"}

    def test_happy_path(self, setup_data):
        """Test normal operation with valid input."""
        result = my_function(setup_data)
        assert result == expected_value

    def test_empty_input_raises_error(self):
        """Test that empty input raises ValueError."""
        with pytest.raises(ValueError):
            my_function({})

    @pytest.mark.parametrize("input,expected", [
        ("a", 1),
        ("b", 2),
        ("c", 3),
    ])
    def test_parametrized(self, input, expected):
        """Test multiple inputs."""
        assert my_function(input) == expected`

const jestCognitiveModel = `## JEST COGNITIVE MODEL

When writing JavaScript/TypeScript tests with Jest:

### Jest Principles
- Use describe blocks to group related tests
- Use it or test for individual test cases
- Use beforeEach/afterEach for setup/teardown
- Use jest.fn() for function mocks
- Use jest.mock() for module mocks

### Jest Structure
import { myFunction } from './myModule';

describe('myFunction', () => {
    beforeEach(() => {
        // Reset mocks before each test
        jest.clearAllMocks();
    });

    it('returns expected value for valid input', () => {
        const result = myFunction('valid');
        expect(result).toBe('expected');
    });

    it('throws error for invalid input', () => {
        expect(() => myFunction('')).toThrow('Invalid input');
    });

    it('calls dependency with correct arguments', () => {
        const mockDep = jest.fn().mockReturnValue('result');
        myFunction('input', mockDep);
        expect(mockDep).toHaveBeenCalledWith('input');
    });

    describe('edge cases', () => {
        it('handles null input', () => {
            expect(myFunction(null)).toBeNull();
        });

        it('handles undefined input', () => {
            expect(myFunction(undefined)).toBeUndefined();
        });
    });
});`

const cargoCognitiveModel = `## CARGO TEST COGNITIVE MODEL

When writing Rust tests with cargo:

### Rust Testing Principles
- Tests go in the same file with #[cfg(test)]
- Use #[test] attribute for test functions
- Use assert!, assert_eq!, assert_ne! macros
- Use #[should_panic] for expected panics
- Use Result<(), E> for fallible tests

### Rust Test Structure
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_happy_path() {
        let result = my_function("valid");
        assert_eq!(result, "expected");
    }

    #[test]
    fn test_error_case() {
        let result = my_function("");
        assert!(result.is_err());
    }

    #[test]
    #[should_panic(expected = "invalid input")]
    fn test_panic_case() {
        my_function_that_panics("");
    }

    #[test]
    fn test_with_result() -> Result<(), MyError> {
        let result = my_function("valid")?;
        assert_eq!(result, "expected");
        Ok(())
    }
}`

const genericTestCognitiveModel = `## GENERIC TEST COGNITIVE MODEL

When writing tests in any framework:

### Universal Testing Principles
1. Test behavior, not implementation
2. Each test should test one thing
3. Tests should be independent
4. Tests should be deterministic
5. Test names should describe the scenario

### Universal Test Structure
// Arrange - Set up the test data and conditions
// Act - Execute the function under test
// Assert - Verify the results

### Universal Coverage Goals
- All public functions tested
- All error paths tested
- All boundary conditions tested
- All significant branches tested`

// buildCodeDOMTestContext builds Code DOM context for test generation.
func (t *TesterShard) buildCodeDOMTestContext(targetPath string) string {
	if t.kernel == nil {
		return ""
	}

	var context []string

	// Check for API client functions - need integration tests
	apiClientResults, _ := t.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == targetPath {
				funcName := "unknown"
				pattern := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if p, ok := fact.Args[2].(string); ok {
					pattern = p
				}
				context = append(context, fmt.Sprintf("API CLIENT: %s uses %s - mock HTTP client and test error scenarios", funcName, pattern))
			}
		}
	}

	// Check for API handler functions - need request/response tests
	apiHandlerResults, _ := t.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == targetPath {
				funcName := "unknown"
				framework := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if f, ok := fact.Args[2].(string); ok {
					framework = f
				}
				context = append(context, fmt.Sprintf("API HANDLER: %s (%s) - test with httptest, check status codes and JSON responses", funcName, framework))
			}
		}
	}

	// Check requires_integration_test predicate
	integrationResults, _ := t.kernel.Query("requires_integration_test")
	for _, fact := range integrationResults {
		if len(fact.Args) >= 1 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, targetPath) {
				context = append(context, fmt.Sprintf("INTEGRATION TEST RECOMMENDED: %s - consider separate _integration_test.go file", ref))
			}
		}
	}

	// Check for external callers (public API)
	externalResults, _ := t.kernel.Query("has_external_callers")
	for _, fact := range externalResults {
		if len(fact.Args) >= 1 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, targetPath) {
				context = append(context, fmt.Sprintf("PUBLIC API: %s - ensure comprehensive test coverage for public interface", ref))
			}
		}
	}

	if len(context) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nCODE ANALYSIS (from Code DOM):\n")
	for _, c := range context {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	return sb.String()
}

// buildTestGenUserPrompt builds the user prompt for test generation.
func (t *TesterShard) buildTestGenUserPrompt(source string, task *TesterTask, framework string) string {
	var sb strings.Builder
	sb.WriteString("Generate unit tests for the following code:\n\n")
	sb.WriteString("```\n")
	sb.WriteString(source)
	sb.WriteString("\n```\n\n")

	if task.Function != "" {
		sb.WriteString(fmt.Sprintf("Focus on testing the function: %s\n", task.Function))
	}

	sb.WriteString(fmt.Sprintf("Use the %s framework.\n", framework))
	sb.WriteString("Include tests for:\n")
	sb.WriteString("- Normal operation\n")
	sb.WriteString("- Edge cases\n")
	sb.WriteString("- Error conditions\n")

	// Add Code DOM context for API-aware test generation
	targetPath := task.Target
	if task.File != "" {
		targetPath = task.File
	}
	codeDOMContext := t.buildCodeDOMTestContext(targetPath)
	if codeDOMContext != "" {
		sb.WriteString(codeDOMContext)
	}

	return sb.String()
}

// parseGeneratedTests parses LLM response into a GeneratedTest struct.
func (t *TesterShard) parseGeneratedTests(response, targetPath, framework string) GeneratedTest {
	// Determine test file path
	testPath := t.getTestFilePath(targetPath, framework)

	// Extract code block if present
	content := response
	if idx := strings.Index(response, "```"); idx != -1 {
		endIdx := strings.LastIndex(response, "```")
		if endIdx > idx {
			content = response[idx+3 : endIdx]
			// Remove language tag if present
			if newlineIdx := strings.Index(content, "\n"); newlineIdx != -1 {
				firstLine := strings.TrimSpace(content[:newlineIdx])
				if !strings.Contains(firstLine, " ") && len(firstLine) < 20 {
					content = content[newlineIdx+1:]
				}
			}
		}
	}

	// Count test functions
	testCount := 0
	functionsTested := make([]string, 0)

	switch framework {
	case "gotest":
		re := regexp.MustCompile(`func (Test\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	case "jest":
		testCount = strings.Count(content, "test(") + strings.Count(content, "it(")
	case "pytest":
		re := regexp.MustCompile(`def (test_\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	case "cargo":
		re := regexp.MustCompile(`#\[test\]\s*fn (\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	}

	return GeneratedTest{
		FilePath:        testPath,
		TargetFile:      targetPath,
		Content:         strings.TrimSpace(content),
		TestCount:       testCount,
		FunctionsTested: functionsTested,
	}
}

// getTestFilePath generates the test file path from source file path.
func (t *TesterShard) getTestFilePath(sourcePath, framework string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch framework {
	case "gotest":
		return filepath.Join(dir, name+"_test.go")
	case "jest":
		return filepath.Join(dir, name+".test"+ext)
	case "pytest":
		return filepath.Join(dir, "test_"+name+".py")
	case "cargo":
		// Rust tests typically go in the same file or tests/ dir
		return filepath.Join(dir, name+"_test.rs")
	default:
		return filepath.Join(dir, name+"_test"+ext)
	}
}

// =============================================================================
// LLM HELPERS
// =============================================================================

// llmCompleteWithRetry calls LLM with exponential backoff retry logic.
func (t *TesterShard) llmCompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	if t.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	var lastErr error
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("[TesterShard:%s] LLM retry attempt %d/%d\n", t.id, attempt+1, maxRetries)

			delay := baseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		response, err := t.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		if !isRetryableError(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryableError determines if an error should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	retryablePatterns := []string{
		"timeout", "connection", "network", "temporary",
		"rate limit", "503", "502", "429",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return true // Default to retry
}
