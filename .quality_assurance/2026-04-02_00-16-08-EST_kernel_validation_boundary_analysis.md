# Boundary Value Analysis & Negative Testing: Kernel Validation Subsystem

**Date & Time:** 2026-04-02_00-16-08-EST
**Subsystem:** `internal/core/kernel_validation.go`
**Author:** QA Automation Engineer

## 1. Overview and Architecture Context

The Kernel Validation subsystem acts as the "Constitutional Gatekeeper" for codeNERD's Mangle-based neuro-symbolic engine. Its primary responsibility is to prevent "Schema Drift"—a phenomenon where autonomous agents invent hallucinated predicates (e.g., `magical_fix(Bug)`) that have no grounding in the underlying knowledge base, leading to silent evaluation failures or infinite derivation loops.

The subsystem parses, analyzes, and self-heals learned rules loaded from `learned.mg` files. Mangle evaluates declarative rules iteratively. A single bad rule (e.g., `next_action(X) :- !next_action(X).`) can cause the engine to loop infinitely (stratification error) or crash. Therefore, the `RealKernel` utilizes `checkInfiniteLoopRisk`, `ValidateLearnedRule`, and `validateLearnedRulesContent` to sanitize inputs before they touch the engine.

## 2. Boundary Value Analysis Vectors

We evaluated the `internal/core/kernel_validation.go` subsystem against four primary vectors: Null/Undefined/Empty inputs, Type Coercion, User Request Extremes, and State Conflicts.

### A. Null/Undefined/Empty Inputs

**Edge Case 1: Empty or Nil Rules Arrays**
- **Trigger:** Calling `ValidateLearnedRules([]string{})` or `ValidateLearnedRules(nil)`.
- **System Behavior:** The code currently checks `if k.schemaValidator == nil` but does not explicitly handle empty slices before passing them to the underlying validator. If `schemaValidator.ValidateRules` isn't robust against `nil` arrays, it might panic.
- **Risk Level:** Low, but unhandled nil slices are a common source of Go panics.

**Edge Case 2: Empty Schema Initialization**
- **Trigger:** Initializing a kernel where the embedded schema files are missing or empty, leading to `k.schemas == ""`.
- **System Behavior:** `refreshSchemaValidatorLocked` sets `k.schemaValidator = nil`. Subsequent calls to `ValidateLearnedRule` quietly return `nil` (no error).
- **Risk Level:** High. This "fail-open" design means if the schema loader breaks, all hallucinated rules are suddenly accepted by the engine, defeating the entire purpose of the validation layer.

**Edge Case 3: Completely Empty or Comment-Only `learnedText`**
- **Trigger:** Feeding `validateLearnedRulesContent` a file that consists entirely of `"\n\n\n"` or `# comments`.
- **System Behavior:** It splits the string, processes empty lines, and successfully writes an identical file. No error is thrown.
- **Risk Level:** Low. The system properly handles empty strings.

### B. Type Coercion and Parsing Extremes

**Edge Case 1: Extremely Long Single Lines (Buffer Exhaustion)**
- **Trigger:** `checkSyntax(ruleText)` takes a `string` and wraps it in a `strings.Reader` for `parse.Unit()`. What if a learned rule is 500MB long? (e.g., an LLM dumped an entire base64 encoded image as a string atom inside a rule).
- **System Behavior:** The Mangle parser will attempt to allocate enough tokens for the entire 500MB line. This can lead to Out-Of-Memory (OOM) kills on an 8GB RAM machine.
- **Performance:** `checkSyntax` has no length limits. It assumes LLM output is reasonably sized.
- **Risk Level:** Medium. A runaway LLM output can take down the agent.

**Edge Case 2: Invalid UTF-8 / Binary Injection**
- **Trigger:** A file path or string atom inside a learned rule containing malformed UTF-8 byte sequences or null bytes (`\x00`).
- **System Behavior:** `strings.Split(learnedText, "\n")` and `strings.TrimSpace(line)` operate on runes. Invalid UTF-8 can cause the Mangle parser to either panic or produce unexpected AST nodes.
- **Risk Level:** Medium. The `parse.Unit` relies on Google Mangle's internal lexer, which may or may not handle binary bytes gracefully.

**Edge Case 3: The `checkInfiniteLoopRisk` Regex-like Bypasses**
- **Trigger:** The function `checkInfiniteLoopRisk` does naive string matching, e.g., `strings.Contains(body, "current_time(")` or `strings.Contains(bodyLower, "coder_state(/idle)")`.
- **System Behavior:** Since Mangle is insensitive to whitespace within facts, an LLM might generate `next_action(X) :- current_time ( T )` (with a space before the parenthesis).
- **Risk Level:** High. String matching (`"current_time("`) fails against `current_time (`, allowing the infinite loop risk to bypass the validator entirely.

### C. User Request Extremes

**Edge Case 1: Extreme File Paths in Self-Healing**
- **Trigger:** `validateLearnedRulesContent` attempts to persist healed rules back to disk using `os.WriteFile(filePath, ...)`. What if the file path is `/dev/full` or a path exceeding the OS path length limits (e.g., Windows MAX_PATH)?
- **System Behavior:** `os.WriteFile` will fail, and the system logs an error: `Self-healing: failed to persist healed rules`. However, it returns the `result.healedText` anyway, which continues execution with the healed state in memory but unhealed state on disk.
- **Risk Level:** Low, but causes silent state divergence between memory and disk across restarts.

**Edge Case 2: Massive Mangle Programs (50 Million line monorepos)**
- **Trigger:** An agent mapping a 50M line monorepo generates 100,000 learned rules.
- **System Behavior:** `validateLearnedRulesContent` splits the string by `\n` and iterates through 100,000 lines. Inside the loop, it calls `checkSyntax`, which calls `parse.Unit` (the full parser) *for every single line*.
- **Performance:** Calling `parse.Unit` 100,000 times will instantiate 100,000 lexers, parsers, and AST trees, resulting in catastrophic garbage collection overhead. This operation could take several minutes and exhaust available memory.
- **Risk Level:** High. The system's performance does not scale linearly for large sets of rules. Validation must use batch parsing instead of per-line parsing for high performance.

**Edge Case 3: 100-Depth Negation Chains**
- **Trigger:** An LLM generates deeply nested negated logic: `a :- !b. b :- !c. c :- !d...`
- **System Behavior:** Stratification checks in Mangle operate on a dependency graph. Extreme depths could trigger worst-case O(N^2) or stack overflow scenarios during the `analysis.Stratify` phase, though the rule-level syntax check might pass.
- **Risk Level:** Medium.

### D. State Conflicts & Concurrency

**Edge Case 1: Concurrent Schema Updates vs Validation**
- **Trigger:** Goroutine A calls `SetSchemas(...)` which acquires `k.mu.Lock()` and mutates `k.schemaValidator`. Simultaneously, Goroutine B calls `ValidateLearnedRule(...)` which holds `k.mu.RLock()`.
- **System Behavior:** The RWMutex protects the pointer swap. However, if `refreshSchemaValidatorLocked` takes a long time (parsing massive schemas), the write lock will starve read locks, blocking all validation attempts.
- **Risk Level:** Low, since schema updates are typically rare, but under heavy concurrent LLM processing, this could become a bottleneck.

**Edge Case 2: Self-Healing Overwrites Concurrent Edits**
- **Trigger:** `validateLearnedRulesContent` reads the learned rules, decides to "heal" them, and calls `os.WriteFile(filePath, ...)`. Meanwhile, the User or another Agent Shard appended a new rule to `learned.mg`.
- **System Behavior:** `os.WriteFile` overwrites the file unconditionally, wiping out any concurrent appends made between the read and the write.
- **Risk Level:** High. This is a classic Time-Of-Check to Time-Of-Use (TOCTOU) race condition. The kernel should use atomic file replacements or append-only logs for learned rules.

## 3. Recommended Improvements

1. **Robust Infinite Loop Detection:** Replace the brittle string-matching in `checkInfiniteLoopRisk` with an AST-based analysis. Parse the rule using Mangle's parser, traverse the AST, and check for `current_time` or `idle` predicates structurally. This prevents whitespace bypasses.
2. **Fail-Closed Validation:** In `ValidateLearnedRule`, if `k.schemaValidator == nil`, it currently returns `nil` (allowing the rule). It should return an error: `ErrValidatorNotInitialized`. A missing schema should mean "nothing is allowed", not "everything is allowed".
3. **Batch Parsing:** Instead of parsing learned rules line-by-line in a loop (which invokes the parser overhead N times), parse the entire block, map the errors back to line numbers, and comment out the offending lines. This is critical for scaling to large codebases.
4. **Line Length Limits:** Implement a `bufio.Scanner` with a strict `MaxScanTokenSize` when processing learned rules to prevent OOM attacks from malformed LLM outputs.
5. **Atomic File Healing:** Implement a file-locking mechanism or a `.tmp` file swap when persisting self-healed rules to disk to prevent data loss from concurrent writes.
6. **Fuzz Testing:** Introduce Go fuzz tests (`func FuzzKernelValidation(f *testing.F)`) feeding random byte arrays into `ValidateLearnedProgram` and `checkSyntax` to identify parser panics.

## 4. Deep Dive into Specific Scenarios and Edge Cases

### Scenario 1: The "Fail-Open" Schema Trap
Currently, if the underlying schema cannot load, `k.schemaValidator` is set to `nil`.
When the JIT loop attempts to assert a fact or rule, the code calls `ValidateLearnedRule`. If `schemaValidator == nil`, the code assumes all rules are valid.
This is extremely dangerous in a production environment.
If a disk error or bad update corrupts `schemas.mg`, the AI enters a "god mode" where it can hallucinate predicates arbitrarily because the validation logic is short-circuited.

*Recommendation:*
Change the implementation in `ValidateLearnedRule`:
```go
if k.schemaValidator == nil {
    return fmt.Errorf("FATAL: schema validator is nil, cannot guarantee safe execution")
}
```

### Scenario 2: The Silent Healer (TOCTOU Vulnerability)
In `validateLearnedRulesContent`:
1. It reads the file contents into memory.
2. It parses line-by-line, determining validity.
3. If errors are found and `heal == true`, it prepends `# SELF-HEALED: <error>` to the bad lines.
4. It calls `os.WriteFile(filePath, ...)`.

What if an external process (like an IDE plugin or a concurrent background job) modifies the file while this memory-heavy parsing is happening? The background changes will be irrevocably destroyed by `os.WriteFile`.

*Recommendation:*
Implement file locking or an append-only journal for learned rules.
Alternatively, use file stats (inode and modification time) before writing to ensure the file hasn't been modified since it was read.

### Scenario 3: Memory Exhaustion via Recursive Rule Expansion
Consider an LLM generating the following valid syntax rule:
`expand(X) :- expand(Y), expand(Z).`
While syntax is valid, during fixpoint evaluation, this causes combinatorial explosion.
Mangle has limits on evaluation depth, but the validation layer does not catch this ahead of time.

*Recommendation:*
Add structural checks in `schemaValidator` to analyze cyclical rule graphs and compute bounded derivations before passing to the engine.

### Scenario 4: The String Regex Bypass
In `checkInfiniteLoopRisk`:
```go
idleStatePatterns := []string{"coder_state(/idle)", "current_task(/idle)", "_state(/idle)", "_status(/idle)", "/idle)"}
```
If an LLM hallucinates `current_task ( /idle )` (spaces added), or `current_task(/IDLE)`, the simplistic `strings.Contains` checks fail to catch the infinite loop trigger. Since the engine treats spacing and case flexibly in certain contexts, this will pass validation but bring down the instance.

*Recommendation:*
AST validation is non-negotiable for robust AI safety. Do not use string manipulation on formal language code. Parse to an AST, then assert structural invariants.

### Scenario 5: Large Context Payload Truncation
If an LLM inserts a 1MB payload inside a predicate, e.g., `extracted_data("very long base64 string...")`.
The current error truncation:
```go
if len(errStr) > 100 {
    errStr = errStr[:100] + "..."
}
```
This truncates the error, which is fine, but it does not truncate the line itself when saving it to the healed file as `# <original line>`. This means the `learned.mg` file bloats indefinitely with 1MB commented-out garbage strings every time the LLM hallucinates a large payload.

*Recommendation:*
Limit the length of the commented-out lines to 1000 characters to prevent the file from swelling.

### Scenario 6: The Ubiquitous Predicate Trap
The code defines `ubiquitousPredicates`:
```go
"current_time(", "current_time(_)", "entry_point(", "current_phase(", "build_system(", "system_startup(", "northstar_defined"
```
But what if the AI hallucinates `current_time(T, X)`? Mangle allows arity overloads unless strict mode is fully enforced. The `strings.Contains(body, pred)` check will catch `current_time(T)` because it looks for `current_time(`. But if the LLM invents a completely new ubiquitous-sounding predicate like `system_time(T)`, it will bypass the hardcoded list.

*Recommendation:*
Instead of a hardcoded blacklist, use a dynamic check: query the schema registry for predicates marked with a special descriptor `@Ubiquitous`, or analyze the engine's fact count statically to identify predicates that are always active.

### Scenario 7: Malformed Comments
If a rule is preceded by a comment `/* comment */ next_action(X) :- !a.`, the naive line-by-line parser will fail to strip the comment because it only trims `strings.HasPrefix(trimmed, "#")`. The regex checks will therefore include the comment text in their evaluation.

*Recommendation:*
Use Mangle's built-in comment stripper before evaluating regexes, or better yet, rely purely on the AST.

### Scenario 8: Exhaustive CPU Bound Validation
If a user submits 50,000 rules, `checkSyntax` invokes `parse.Unit` 50,000 times. `parse.Unit` instantiates memory-heavy structures. A malicious or rogue LLM can DoS the agent.

*Recommendation:*
Combine rules into batches of 1,000 and parse them simultaneously, mapping errors back to specific rules using line annotations from the AST nodes.

## 5. Proposed Test Implementations

To effectively test these edge cases, we must implement a series of negative and boundary tests. Below are the draft implementations for the tests that should be added to the test suite to bridge the gaps identified in this analysis.

### A. Null/Undefined/Empty Inputs Tests

```go
func TestKernelValidation_NilSchemaValidator(t *testing.T) {
	// Tests the "Fail-Open" scenario.
	k := setupMockKernel(t)
	// Deliberately do not load schemas to leave k.schemaValidator == nil

	// This should ideally return an error, but currently returns nil.
	// We test current behavior to document it, then fail if it's the unsafe behavior.
	err := k.ValidateLearnedRule("some_hallucinated_rule(X) :- magic(X).")

	// QA Assertion: It SHOULD fail. If it passes, the system is fail-open.
	if err == nil {
		t.Errorf("CRITICAL SECURITY FLAW: ValidateLearnedRule returned nil when schema validator is missing.")
	}
}

func TestKernelValidation_EmptyRulesArray(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl valid_pred(Name).")
	k.Evaluate() // Initializes validator

	// Pass nil
	errors := k.ValidateLearnedRules(nil)
	if errors != nil {
		t.Errorf("Expected nil response for nil rules array, got %v", errors)
	}

	// Pass empty slice
	emptySlice := make([]string, 0)
	errors = k.ValidateLearnedRules(emptySlice)
	if len(errors) != 0 {
		t.Errorf("Expected 0 errors for empty rules array, got %d", len(errors))
	}
}

func TestKernelValidation_EmptyLearnedText(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl valid_pred(Name).")
	k.Evaluate() // Initializes validator

	// Test with completely empty string
	result := k.validateLearnedRulesContent("", "test_learned.mg", true)
	if result.stats.TotalRules != 0 {
		t.Errorf("Expected 0 rules, got %d", result.stats.TotalRules)
	}
	if result.healedText != "" {
		t.Errorf("Expected empty healed text, got %q", result.healedText)
	}

	// Test with string full of newlines
	newlines := "\n\n\n\n\n"
	result = k.validateLearnedRulesContent(newlines, "test_learned.mg", true)
	if result.stats.TotalRules != 0 {
		t.Errorf("Expected 0 rules for newline string, got %d", result.stats.TotalRules)
	}
}
```

### B. Type Coercion and Parsing Extremes Tests

```go
func TestKernelValidation_HugeLineBufferExhaustion(t *testing.T) {
	// Create a rule with a 10MB string payload
	payloadSize := 10 * 1024 * 1024
	var builder strings.Builder
	builder.WriteString("long_payload(\"")
	for i := 0; i < payloadSize; i++ {
		builder.WriteString("A")
	}
	builder.WriteString("\").")
	hugeRule := builder.String()

	// Measure memory and time to ensure it doesn't OOM or hang forever
	start := time.Now()

	// This invokes checkSyntax indirectly
	k := setupMockKernel(t)
	k.AppendPolicy("Decl long_payload(String).")
	k.Evaluate()

	err := k.ValidateLearnedRule(hugeRule)

	duration := time.Since(start)
	t.Logf("Parsing 10MB rule took %v", duration)

	if err != nil {
		t.Logf("Parser successfully rejected or handled the huge rule: %v", err)
	}

	if duration > 5*time.Second {
		t.Errorf("Performance failure: Parsing huge rule took too long (%v)", duration)
	}
}

func TestKernelValidation_InvalidUTF8(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl broken_utf(String).")
	k.Evaluate()

	// Rule containing invalid UTF-8 byte sequence (\xff\xfe\xfd)
	invalidRule := "broken_utf(\"bad\xff\xfe\xfdbytes\")."

	err := k.ValidateLearnedRule(invalidRule)
	if err == nil {
		t.Logf("Warning: Parser accepted invalid UTF-8 bytes")
	} else {
		t.Logf("Parser correctly rejected invalid UTF-8: %v", err)
	}
}
```

### C. User Request Extremes Tests

```go
func TestKernelValidation_InfiniteLoopWhitespaceBypass(t *testing.T) {
	k := setupMockKernel(t)

	// Test the exact regex bypass
	// checkInfiniteLoopRisk looks for "current_time("
	bypassedRule := "next_action(/do_nothing) :- current_time ( T )."

	loopErr := k.checkInfiniteLoopRisk(bypassedRule)
	if loopErr == "" {
		t.Errorf("CRITICAL: Infinite loop regex bypass successful using whitespace: %q", bypassedRule)
	} else {
		t.Logf("Whitespace bypass prevented: %s", loopErr)
	}

	// Test case bypass
	caseBypass := "next_action(/do_nothing) :- CURRENT_TIME(T)."
	loopErr = k.checkInfiniteLoopRisk(caseBypass)
	if loopErr == "" {
		t.Errorf("CRITICAL: Infinite loop regex bypass successful using uppercase: %q", caseBypass)
	}
}

func TestKernelValidation_MassiveRuleSetPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	k := setupMockKernel(t)
	k.AppendPolicy("Decl test_rule(Number).")
	k.Evaluate()

	// Generate 100,000 rules
	ruleCount := 100000
	var builder strings.Builder
	for i := 0; i < ruleCount; i++ {
		builder.WriteString(fmt.Sprintf("test_rule(%d).\n", i))
	}
	massiveProgram := builder.String()

	start := time.Now()
	result := k.validateLearnedRulesContent(massiveProgram, "", false)
	duration := time.Since(start)

	t.Logf("Validated %d rules in %v", ruleCount, duration)

	if result.stats.ValidRules != ruleCount {
		t.Errorf("Expected %d valid rules, got %d", ruleCount, result.stats.ValidRules)
	}

	// 100k rules should ideally take less than 2 seconds with batch parsing
	if duration > 5*time.Second {
		t.Errorf("Performance failure: Line-by-line validation is too slow (%v)", duration)
	}
}
```

### D. State Conflicts & Concurrency Tests

```go
func TestKernelValidation_ConcurrentSchemaUpdates(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl initial_pred(Name).")
	k.Evaluate()

	// Run concurrent validations while updating schemas
	var wg sync.WaitGroup
	errs := make(chan error, 1000)

	// 10 goroutines constantly validating
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				err := k.ValidateLearnedRule("initial_pred(/test).")
				if err != nil {
					errs <- fmt.Errorf("Validation failed during concurrent update: %v", err)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// 2 goroutines updating schemas
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				newSchema := fmt.Sprintf("Decl initial_pred(Name).\nDecl new_pred_%d_%d(Name).", id, j)
				k.SetSchemas(newSchema)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestKernelValidation_TOCTOU_FileOverwrite(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "learned_*.mg")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	initialContent := "valid_pred(/test).\ninvalid_syntax( :- broken.\n"
	os.WriteFile(tmpFile.Name(), []byte(initialContent), 0644)

	k := setupMockKernel(t)
	k.AppendPolicy("Decl valid_pred(Name).")
	k.Evaluate()

	// Simulate a concurrent process modifying the file JUST BEFORE healLearnedRules finishes

	// We need to instrument the validation to pause, but since we can't easily do that
	// without changing the source code, we will simulate the vulnerability logically.

	// 1. Kernel reads the file content into memory
	content, _ := os.ReadFile(tmpFile.Name())

	// 2. CONCURRENT EDIT: Another process appends a highly critical rule
	criticalAppend := "critical_safety_rule(/do_not_delete_root).\n"
	os.WriteFile(tmpFile.Name(), []byte(initialContent+criticalAppend), 0644)

	// 3. Kernel finishes validation and writes back the healed content
	healedContent := k.healLearnedRules(string(content), tmpFile.Name())

	// 4. Verify data loss
	finalDiskContent, _ := os.ReadFile(tmpFile.Name())

	if !strings.Contains(string(finalDiskContent), "critical_safety_rule") {
		t.Errorf("CRITICAL TOCTOU FLAW: Concurrent file append was silently overwritten and lost by healLearnedRules.")
		t.Logf("Final disk content:\n%s", string(finalDiskContent))
	}
}
```

## 6. Fuzzing Implementation Strategy

To properly fuzz the parser boundaries, we need to utilize the native Go fuzzing tools. The following implementation should be added to `internal/core/kernel_validation_fuzz_test.go`.

```go
package core

import (
	"testing"
)

// FuzzKernelValidation Syntax Parsing
func FuzzCheckSyntax(f *testing.F) {
	// Seed corpus with valid and invalid examples
	f.Add("valid_pred(/test).")
	f.Add("next_action(/do) :- current_time(T).")
	f.Add("broken_rule :- (.")
	f.Add("")
	f.Add("/* comment */ rule(X).")

	f.Fuzz(func(t *testing.T, ruleText string) {
		// We expect this to return errors for garbage data,
		// but it should NEVER panic.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("checkSyntax PANICKED on input %q: %v", ruleText, r)
			}
		}()

		checkSyntax(ruleText)
	})
}

// Fuzz Infinite Loop Detection
func FuzzCheckInfiniteLoopRisk(f *testing.F) {
	f.Add("next_action(/test) :- current_time(T).")
	f.Add("next_action(/test) :- coder_state(/idle).")
	f.Add("safe_rule(X) :- other(X).")

	k, _ := NewRealKernel()

	f.Fuzz(func(t *testing.T, ruleText string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("checkInfiniteLoopRisk PANICKED on input %q: %v", ruleText, r)
			}
		}()

		k.checkInfiniteLoopRisk(ruleText)
	})
}
```

By integrating these fuzz tests into the CI/CD pipeline, we can automatically discover new ways the LLM might hallucinate malformed strings that crash the parser.

## 7. Conclusion

The current validation layer serves as an adequate first line of defense, but it relies too heavily on string manipulation for semantic analysis. Given that LLMs are notorious for injecting unexpected whitespace, comments, unicode characters, and massive arbitrary payloads, a purely AST-driven validation pipeline is required to achieve the highest level of stability. Furthermore, the file-handling logic must be upgraded to be atomic and concurrency-safe to prevent silent data loss during system self-healing.
