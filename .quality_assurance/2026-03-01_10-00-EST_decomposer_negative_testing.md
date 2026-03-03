# QA Journal Entry: Campaign Decomposer Boundary Analysis
**Date:** 2026-02-28 00:18 EST
**Author:** Jules (QA Automation Engineer)
**Subsystem:** Internal Campaign Decomposer
**File:** `internal/campaign/decomposer.go`

## 1. Subsystem Overview

The `decomposer.go` file houses the `Decomposer` struct, an absolutely critical component of the Codenerd Campaign orchestration system. The Decomposer acts as the primary orchestrator for translating high-level user goals into structured, executable campaign plans. It sits at the intersection of several complex subsystems:
-   **LLM Integration (`perception.LLMClient`)**: Uses large language models (like Gemini) to propose plans, extract requirements, and classify documents.
-   **Mangle Kernel (`core.Kernel`)**: Utilizes Mangle's deterministic logic to validate generated plans, enforce taxonomies, and manage state/facts.
-   **Knowledge Base / Vector Store (`store.NewLocalStore`)**: Ingests source documents and uses vector embeddings for semantic retrieval to ground LLM prompts.
-   **Sub-components**: Integrates with `IntelligenceGatherer`, `ShardAdvisoryBoard`, `EdgeCaseDetector`, and `ToolPregenerator`.

The core `Decompose` method is a multi-step orchestration pipeline:
1.  Gather pre-planning intelligence.
2.  Ingest and classify source documents.
3.  Ingest into a local knowledge store (vectors + graph).
4.  Extract requirements using RAG-based vector recall.
5.  Propose a raw plan via LLM using context and topology hints.
6.  Consult an advisory board of Shards.
7.  Load plan facts into the Mangle kernel for deterministic validation.
8.  Refine the plan via LLM if Mangle detects issues (e.g., circular dependencies).
9.  Link requirements to specific tasks.
10. Pre-generate missing tools (Autopoiesis).

## 2. Test Evaluation (Current State)

The current test suite (`internal/campaign/decomposer_test.go`) is alarmingly sparse for a component of this complexity and criticality.

**Current Test Coverage:**
-   `TestNewDecomposer`: Basic constructor verification.
-   `TestDecomposer_Setters`: Verifies that setter methods (`SetShardLister`, `SetIntelligenceGatherer`, etc.) do not panic.
-   `TestDecomposer_InferDocType`: Tests a simple string-matching heuristic.
-   `TestDecomposer_ClassifyDocument_Trivial`: Tests an optimization short-circuit for trivial file classification.
-   `TestDecomposer_Decompose_ValidationFailure`: A stub test that admits it cannot easily test the `Decompose` method because it lacks a proper mock for the `core.Kernel` interface.

**Gaps Identified:**
-   **No Integration Testing**: The primary `Decompose` pipeline is entirely untested. There are no tests verifying the orchestration of LLM calls, kernel assertions, and vector store ingestion.
-   **Zero Error Handling Coverage**: The extensive error handling and fallback logic (e.g., LLM JSON parsing retries, refinement loops, partial failures) are untested.
-   **Boundary & Edge Cases Missing**: The system accepts arbitrary file paths, token budgets, and LLM responses without verified safety constraints.
-   **Concurrency/Timeout Risks**: The use of `context.Context` throughout the pipeline is not tested for proper cancellation or timeout handling.

## 3. Boundary Value Analysis & Negative Testing Vectors

To elevate this subsystem to a robust, production-ready state ("PhD level"), rigorous testing of edge cases and extreme inputs is required. The following vectors highlight significant vulnerabilities.

### 3.1 Null / Undefined / Empty Inputs

**Vector:** Core parameters and configurations missing or empty.
-   **Empty Goal (`req.Goal == ""`)**:
    -   *Scenario*: User requests a campaign with no stated goal.
    -   *Impact*: `generateDiscoveryQuestions` correctly returns `nil`, but `extractRequirementsSmart` might skip extraction. The LLM might hallucinate a plan or fail. The plan title fallback will extract an empty string.
    -   *Test Case*: Call `Decompose` with `req.Goal = ""`. Verify it returns a validation error early or degrades gracefully without panicking.
-   **Empty/Nil Source Paths (`req.SourcePaths`)**:
    -   *Scenario*: Campaign initiated with no reference documents.
    -   *Impact*: `ingestSourceDocuments` returns empty arrays. The planner operates purely on the goal. This is a valid use case but needs verification that the system doesn't crash expecting files.
    -   *Test Case*: Call `Decompose` with `req.SourcePaths = nil`.
-   **Missing Workspace (`d.workspace == ""`)**:
    -   *Scenario*: Decomposer initialized with an empty workspace path.
    -   *Impact*: `filepath.Join` will join with the current directory. `ingestSourceDocuments` will attempt to stat relative paths. If paths are absolute, it might work, but relative paths will resolve unpredictably.
    -   *Test Case*: Initialize Decomposer with empty workspace, pass relative `SourcePaths`.
-   **Nil External Systems**:
    -   *Scenario*: `intelligence`, `advisoryBoard`, `toolPregenerator`, or even `kernel`/`llmClient` are nil.
    -   *Impact*: The code contains many `if d.intelligence != nil` checks, but are they exhaustive? What if `d.kernel` is nil during `seedDocFacts` or validation?
    -   *Test Case*: Initialize with minimal/nil dependencies and assert no panics. (Partially covered, but needs functional verification).

### 3.2 Type Coercion & Data Integrity

**Vector:** Dealing with untyped, mis-typed, or maliciously formatted data, particularly from the LLM.
-   **LLM JSON Malformation**:
    -   *Scenario*: The LLM returns a response that looks like JSON but is structurally invalid (e.g., missing quotes, trailing commas, mixed types).
    -   *Impact*: `json.Unmarshal` fails. The Decomposer implements a retry mechanism (`"Retrying plan proposal with JSON enforcement"`), but what if the retry *also* fails?
    -   *Test Case*: Mock `LLMClient` to consistently return malformed JSON (`{title: "bad"}`). Verify the error cascades appropriately.
-   **Type Mismatches in LLM Output**:
    -   *Scenario*: The `RawPlan` struct expects `Confidence` as a `float64`, but the LLM returns a string `"0.95"`.
    -   *Impact*: `json.Unmarshal` will fail.
    -   *Test Case*: Mock LLM returning `{"confidence": "high"}`.
-   **Missing Required Fields in JSON**:
    -   *Scenario*: LLM returns valid JSON, but omits the `phases` array.
    -   *Impact*: `buildCampaign` iterates over a nil slice. The system implements a fallback scaffolding phase (`if len(plan.Phases) == 0`), which is good, but does the fallback handle all missing data gracefully?
    -   *Test Case*: Mock LLM returning `{}`.
-   **Binary Files in Source Paths**:
    -   *Scenario*: A compiled binary (e.g., `program.exe`) is passed in `SourcePaths`.
    -   *Impact*: `classifyDocuments` reads the entire binary into a string (`string(data)`) and passes it to the LLM. This wastes tokens, corrupts the prompt with non-UTF8 characters, and could cause LLM rejection.
    -   *Test Case*: Pass a binary file path. Note: `isSupportedDocExt` exists but is not shown in the file; we must ensure it strictly blocks binaries.
-   **Invalid Paths in Artifacts**:
    -   *Scenario*: LLM generates a task with an artifact path like `../../../etc/passwd`.
    -   *Impact*: Path traversal vulnerability when tasks are later executed. The `normalizePath` function is called, but does it strictly jail the path to the workspace?
    -   *Test Case*: Mock LLM proposing task with `../../../secret.txt`.

### 3.3 User Request Extremes

**Vector:** Pushing the system to its physical and logical limits.
-   **Extreme Token Budget (`req.ContextBudget`)**:
    -   *Scenario*: User specifies a budget of 10 tokens or 100,000,000 tokens.
    -   *Impact*: 10 tokens might break `ContextPager`. 100M tokens might cause OOM or API rejection if passed blindly to the LLM.
    -   *Test Case*: Call `Decompose` with `req.ContextBudget = -1` and `req.ContextBudget = 1000000000`.
-   **Massive Monorepo Ingestion**:
    -   *Scenario*: `SourcePaths` points to a directory with 50,000 source files.
    -   *Impact*: `ingestSourceDocuments` loads all files sequentially. `classifyDocuments` calls the LLM sequentially for *each* non-trivial file. This will take hours and exhaust API limits.
    -   *Test Case*: Pass a directory with 1000 files. Verify performance and whether batching/limits are enforced. (Currently, there appears to be no limit on the number of files processed).
-   **Extremely Long Goal String**:
    -   *Scenario*: The goal is a 50MB string (e.g., someone pasted an entire log dump).
    -   *Impact*: `extractRequirementsSmart` might embed the entire goal into prompts. `seedDocFacts` asserts it into Mangle. Potential OOM or massive token usage.
    -   *Test Case*: Pass a 5MB string as `req.Goal`.
-   **Infinite Refinement Loop**:
    -   *Scenario*: Mangle continuously finds validation issues (e.g., circular dependencies), and the LLM repeatedly fails to fix them, proposing the same broken plan.
    -   *Impact*: The code currently attempts refinement exactly once (`Step 7: If issues, attempt LLM refinement`). This prevents an infinite loop, but leaves the campaign in a broken state.
    -   *Test Case*: Mock Mangle to always return validation errors, and mock LLM to return the identical plan on refinement. Verify the system halts and returns the validation errors.
-   **Deep Dependency Chains**:
    -   *Scenario*: LLM proposes 100 phases, each depending on the previous one.
    -   *Impact*: `buildCampaign` creates the structure. Does the downstream execution engine support this depth?
    -   *Test Case*: Mock LLM proposing 100 sequential phases.

### 3.4 State Conflicts & Concurrency

**Vector:** Context cancellation, timeouts, and race conditions.
-   **Context Cancellation During Ingestion**:
    -   *Scenario*: User cancels the campaign planning (e.g., via UI) while `ingestSourceDocuments` is reading 1000 files.
    -   *Impact*: The code checks `ctx.Done()` between file reads, which is excellent. However, `classifyDocuments` also needs to check `ctx.Done()`, which it does. What about the knowledge store ingestion?
    -   *Test Case*: Start `Decompose` with a context that cancels after 100ms. Verify it returns early with `context.Canceled`.
-   **Context Cancellation During Mangle Validation**:
    -   *Scenario*: Validation takes too long (complex rule fixpoint calculation) and context expires.
    -   *Impact*: The Mangle engine (`d.kernel.Query`) must respect the context. If it doesn't, the goroutine leaks.
    -   *Test Case*: Cancel context right before step 6.
-   **Database File Locks (Knowledge Store)**:
    -   *Scenario*: Two campaigns try to write to the same `knowledge.db` path simultaneously.
    -   *Impact*: SQLite `database is locked` error. The `kbPath` is derived from `campaignID`, which is UUID-based, so collisions are highly unlikely. But what if `safeCampaignID` generation has a flaw?
    -   *Test Case*: Verify `campaignID` uniqueness.
-   **Atomic Rebuild Failure**:
    -   *Scenario*: During plan refinement (Step 7), facts are retracted and reloaded. If `tx.Commit()` fails, the kernel is left in an inconsistent state (partial facts).
    -   *Impact*: Subsequent queries or the final validation check will fail unpredictably.
    -   *Test Case*: Mock `tx.Commit()` to return an error. Verify the campaign creation fails gracefully instead of proceeding with a corrupt state.

## 4. Performance & Scalability Considerations

-   **Sequential LLM Calls**: `classifyDocuments` processes files one by one. For 100 files, this means 100 sequential network requests to the LLM. This is a severe bottleneck.
    -   *Improvement*: Implement bounded concurrency (e.g., an errgroup with 10 workers) for document classification.
-   **Memory Accumulation**: `readDocumentsFromDir` builds slices of `SourceDocument` and `FileMetadata` in memory. `classifyDocuments` reads the entire file content into memory via `os.ReadFile`. For large repositories, this will cause memory pressure.
    -   *Improvement*: Stream files or use memory limits.
-   **N+1 Query Pattern in Requirements Extraction**: `extractRequirementsSmart` loops over questions, and for each question, queries the vector database, then calls the LLM.
    -   *Improvement*: Can questions be batched? Can the vector recall be combined?

## 5. Improvement Plan: Required Tests

To address these gaps, I recommend implementing the following test categories in `internal/campaign/decomposer_test.go`:

### 5.1 Orchestration Error Propagation Tests
We need a test that mocks `LLMClient`, `Kernel`, and `store` to verify error propagation from dependencies.
-   **Test**: `TestDecompose_KnowledgeStoreFail`
    -   Mock knowledge store to return initialization error.
    -   Verify `Decompose` handles failure gracefully.

### 5.2 LLM Resilience Tests
-   **Test**: `TestDecompose_LLMMalformedJSON`
    -   Mock `LLMClient.Complete` to return invalid JSON on the first try, and valid JSON on the retry.
    -   Verify `Decompose` recovers and succeeds.
-   **Test**: `TestDecompose_LLMTotalFailure`
    -   Mock `LLMClient.Complete` to return an error or timeout.
    -   Verify `Decompose` returns a wrapped error and cleans up.
-   **Test**: `TestCleanJSONResponse_EdgeCases`
    -   Input: Markdown containing text before and after the ` ```json ` block.
    -   Input: Raw JSON without markdown fences but with trailing garbage.
    -   Input: Nested JSON objects.

### 5.3 Mangle Integration Tests
-   **Test**: `TestValidatePlan_CircularDependency`
    -   Mock `Kernel.Query("validation_error")` to return a circular dependency issue.
    -   Verify `validatePlan` correctly parses the issue into the `PlanValidationIssue` slice.
-   **Test**: `TestRefinePlan_Success`
    -   Provide a `RawPlan` and a list of issues. Mock LLM to return a corrected plan.
    -   Verify the refined plan is returned.

### 5.4 File System and Ingestion Tests
-   **Test**: `TestIngestSourceDocuments_Cancellation`
    -   Create a directory with 10 dummy files.
    -   Pass a context that cancels after 2 files are processed.
    -   Verify the function returns early with `context.Canceled`.
-   **Test**: `TestIngestSourceDocuments_MissingFiles`
    -   Pass paths that do not exist.
    -   Verify the function skips them without failing the entire ingestion process.

### 5.5 Advanced Feature Tests (Gemini Specific)
-   **Test**: `TestCompleteWithGrounding`
    -   Test the behavior when `d.grounding` is configured vs nil.
    -   Verify that thinking mode metadata is captured if enabled.

## 6. Journal Summary

The `Decomposer` is a sophisticated orchestrator that heavily relies on external, non-deterministic systems (LLMs) and complex deterministic logic engines (Mangle). While the code exhibits defensive programming (e.g., retries on JSON parsing, fallback scaffolding phases, context cancellation checks), the complete lack of functional and integration tests leaves significant blind spots.

The most critical vulnerabilities lie in **scalability** (sequential processing of large source trees) and **data integrity** (trusting LLM outputs without robust schema validation beyond basic JSON unmarshaling).

Implementing the outlined boundary value analysis tests will drastically improve the reliability of the Autopoiesis and campaign generation pipelines, moving the codebase toward a "PhD level" of quality assurance.

## 7. Detailed Breakdown of Missing Test Cases

### 7.1 LLM JSON Parsing Boundaries (`cleanJSONResponse`)
The `cleanJSONResponse` function attempts to extract JSON from messy LLM text.
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestCleanJSON_MarkdownFence` | ` ```json { "a": 1 } ``` ` | `{ "a": 1 }` |
| `TestCleanJSON_NoFence_Preamble` | `Here is the plan: { "a": 1 }` | `{ "a": 1 }` |
| `TestCleanJSON_NestedBraces` | `{ "a": { "b": 1 } }` | `{ "a": { "b": 1 } }` |
| `TestCleanJSON_ArrayRoot` | `[ {"a": 1} ]` | `[ {"a": 1} ]` |
| `TestCleanJSON_StringsWithBraces`| `{ "str": "hello { world }" }` | `{ "str": "hello { world }" }` |
| `TestCleanJSON_IncompleteJSON` | `{ "a": 1 ` | Fallback behavior (likely returns original) |

### 7.2 Decompose Pipeline Extreme Inputs
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestDecompose_EmptyGoal` | `req.Goal = ""` | Handled gracefully (error or fallback plan) |
| `TestDecompose_MassiveGoal` | `req.Goal = strings.Repeat("a", 10*1024*1024)` | Context budget limit triggers or early rejection |
| `TestDecompose_ZeroContext` | `req.ContextBudget = 0` | Reverts to default 200k |

### 7.3 Mangle Kernel State Conflicts
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestRefinePlan_TxCommitFail` | `tx.Commit()` returns error | Reverts state, returns error, logs warning |
| `TestValidatePlan_MalformedFact` | Kernel returns fact with missing args | Ignored safely, doesn't panic on `ExtractString` |

## 8. Deep Dive: Mangle Integration Risks

The `Decomposer` bridges the gap between probabilistic LLM outputs and deterministic Mangle logic. This creates a "Type Dissonance" boundary.
-   **Risk**: LLM outputs a phase category `"scaffold"` but Mangle expects an atom `"/scaffold"`. The `normalizeCategory` function handles some of this, but any un-normalized string passed to Mangle will result in a silent join failure (0 tuples returned).
-   **Mitigation Test**: Add tests specifically checking the serialization of `Campaign` structs into Mangle facts (`campaign.ToFacts()`). Ensure all strings that represent enums or categories are properly prefixed with `/` to become valid Mangle Atoms.

## 9. Deep Dive: Prompt Provider Integration

The `Decomposer` allows swapping out the `PromptProvider` for JIT compiled prompts.
-   **Risk**: If the JIT compilation fails, does the system fall back to the `StaticPromptProvider` gracefully? The code shows `if err != nil { basePrompt = LibrarianLogic }`, which is good.
-   **Mitigation Test**: Create a `MockFailingPromptProvider` and ensure `Decompose` still successfully generates a plan using the static fallbacks.

## 10. Final Recommendation

The `internal/campaign/decomposer.go` file contains elegant orchestration logic, particularly the integration of 12 intelligence systems into the planning prompt. However, its test suite is a façade.

Immediate action is required to implement mock-based integration tests for the `Decompose` method. Furthermore, performance testing on large repositories (1000+ files) should be conducted to identify bottlenecks in the `classifyDocuments` sequential loop.

## 11. Hypothetical Test Implementations

To further illustrate the rigorous testing required, below are hypothetical implementations of key missing edge case tests. These tests demonstrate the techniques required to properly mock the complex dependencies of the `Decomposer`.

### 11.1 Mocking the LLM for JSON Recovery

```go
func TestDecomposer_LLMMalformedJSON(t *testing.T) {
	// Setup a mock LLM that returns invalid JSON on the first call,
	// and valid JSON on the second call (simulating a successful retry).
	callCount := 0
	mockClient := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			callCount++
			if callCount == 1 {
				// Malformed JSON (missing quotes around keys)
				return `{title: "Broken Plan", confidence: 0.5}`, nil
			}
			// Valid JSON on retry
			return `{"title": "Recovered Plan", "confidence": 0.9, "phases": []}`, nil
		},
	}

	mockKernel := &core.RealKernel{} // Or a properly mocked interface
	d := NewDecomposer(mockKernel, mockClient, "/tmp/workspace")

	req := DecomposeRequest{
		Goal: "Test JSON recovery",
	}

	// Execute
	res, err := d.Decompose(context.Background(), req)

	// Verify
	if err != nil {
		t.Fatalf("Decompose failed: %v", err)
	}
	if res.Campaign.Title != "Recovered Plan" {
		t.Errorf("Expected 'Recovered Plan', got %s", res.Campaign.Title)
	}
	if callCount != 2 {
		t.Errorf("Expected LLM to be called twice, got %d", callCount)
	}
}
```

### 11.2 Testing Context Cancellation During Ingestion

```go
func TestDecomposer_IngestSourceDocuments_Cancellation(t *testing.T) {
	// Setup a workspace with numerous dummy files to ensure ingestion takes some time
	workspace, err := os.MkdirTemp("", "decomposer_test_cancel")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workspace)

	// Create 100 dummy files
	for i := 0; i < 100; i++ {
		path := filepath.Join(workspace, fmt.Sprintf("file_%d.txt", i))
		err := os.WriteFile(path, []byte("dummy content"), 0644)
		if err != nil {
			t.Fatalf("Failed to write dummy file: %v", err)
		}
	}

	d := NewDecomposer(nil, nil, workspace)

	// Create a context that is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Attempt ingestion
	paths := []string{workspace} // Pass the directory to trigger traversal
	_, _, err = d.ingestSourceDocuments(ctx, "test_campaign_id", paths)

	// Verify the early return
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}
```

### 11.3 Testing Empty Goals (Boundary Value)

```go
func TestDecomposer_Decompose_EmptyGoal(t *testing.T) {
	mockClient := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			// The LLM should still produce a valid JSON structure, even if the goal is vague
			return `{"title": "Empty Goal Fallback", "confidence": 0.1, "phases": []}`, nil
		},
	}
	mockKernel := &core.RealKernel{} // Mocked
	d := NewDecomposer(mockKernel, mockClient, "/tmp/workspace")

	req := DecomposeRequest{
		Goal: "", // The boundary value
	}

	res, err := d.Decompose(context.Background(), req)

	// Depending on requirements, an empty goal might be an immediate error,
	// or it might result in a highly uncertain, generic plan.
	// We must verify whichever behavior is intended.
	if err != nil {
		// If error is expected, ensure it's a specific validation error
		if !strings.Contains(err.Error(), "goal") {
			t.Errorf("Expected validation error regarding empty goal, got: %v", err)
		}
	} else {
		// If it succeeds, verify the fallback mechanisms kicked in
		if res.Campaign.Confidence > 0.5 {
			t.Errorf("Confidence should be very low for an empty goal, got %f", res.Campaign.Confidence)
		}
		if res.Campaign.Title == "" {
			t.Error("Campaign title should not be empty, even if goal was")
		}
	}
}
```

### 11.4 Testing Refinement on Mangle Validation Failure

```go
func TestDecomposer_RefinePlan_CircularDependency(t *testing.T) {
	// This test requires mocking the Mangle kernel's Query function
	// to return a specific validation_error fact.

	// Mock Kernel behavior
	// Mock.Query("validation_error") -> returns fact: phase_1, circular_dependency, "Phase 1 depends on itself"

	// Setup LLM to return the refined (fixed) plan
	mockClient := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			if strings.Contains(prompt, "ISSUES:") && strings.Contains(prompt, "circular_dependency") {
				return `{"title": "Fixed Plan", "confidence": 0.9, "phases": []}`, nil
			}
			return `{"title": "Broken Plan", "confidence": 0.5, "phases": []}`, nil // Initial broken plan
		},
	}

	// ... [Initialization of Decomposer with mocks] ...

	// The verification would assert that the final Campaign object
	// matches the "Fixed Plan" and that the refinement step was actually executed.
}
```

## 12. Security Audit: Artifact Paths

The Decomposer handles paths generated by the LLM (e.g., in `RawTask.Artifacts`). These paths are normalized, but we must ensure they cannot break out of the workspace sandbox during execution.

While the Decomposer itself only plans, it defines the paths that the Orchestrator will write to.

**Vulnerability Vector:**
An LLM, either hallucinating or maliciously prompted (Prompt Injection via `SourcePaths`), proposes a task artifact path like:
`"artifacts": ["../../../../etc/shadow"]`

**Analysis:**
The `buildCampaign` method iterates over `RawTask.Artifacts`:
```go
normalizedPath := normalizePath(artifactPath)
task.Artifacts = append(task.Artifacts, TaskArtifact{
	Type: artifactType,
	Path: normalizedPath,
})
```

The definition of `normalizePath` is not present in `decomposer.go`, but it is absolutely critical that it performs robust path jailing. A simple `filepath.Clean` is insufficient as it allows relative traversal if joined insecurely later.

**Required Test:**
We must add a test confirming that `normalizePath` strips leading `../` sequences or that the system rejects tasks with paths resolving outside the intended scope.

```go
func TestDecomposer_ArtifactPathSecurity(t *testing.T) {
	// ... Setup Decomposer ...

	req := DecomposeRequest{
		Goal: "Generate a plan with a malicious path",
	}

	// LLM proposes a malicious path
	mockClient := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{
				"title": "Malicious Plan",
				"confidence": 0.9,
				"phases": [
					{
						"name": "Phase 1",
						"tasks": [
							{
								"description": "Write to shadow",
								"type": "/code",
								"artifacts": ["../../../../../etc/shadow"]
							}
						]
					}
				]
			}`, nil
		},
	}

	// ... Execute ...

	// Verification: Ensure the resulting Campaign struct does NOT contain
	// the literal "../../../../../etc/shadow" path, or that it has been safely scrubbed.
}
```

## 13. Deep Dive: Memory Leaks in Sequential Processing

The method `readDocumentsFromDir` walks a directory and appends to slices:
```go
filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
    // ...
    mds, mmeta := d.readDocumentsFromPath(path, campaignID)
    docs = append(docs, mds...)
    meta = append(meta, mmeta...)
    return nil
})
```

If the user points the Decomposer at a large monorepo (e.g., 500,000 files), this synchronous `WalkDir` will build an enormous slice of structs in memory before returning.

Furthermore, `classifyDocuments` then iterates over this massive slice, reading each file's content entirely into memory via `os.ReadFile` and sending it to the LLM.

**Analysis of Failure Modes:**
1.  **OOM (Out Of Memory)**: Loading metadata for 500k files might consume gigabytes of RAM. Loading the content of those files in `classifyDocuments` will certainly cause an OOM panic.
2.  **API Rate Limiting**: Sending 500k sequential requests to the LLM API will hit rate limits immediately, blocking the campaign generation.
3.  **Timeout**: The process will take hours, likely exceeding any reasonable context deadline.

**Architectural Recommendations:**
1.  **Pagination/Streaming**: The Decomposer must not ingest the entire world at once. It should ingest a prioritized subset of files based on heuristics or an initial LLM triage step based purely on directory tree structure.
2.  **Concurrency**: If 1000 files need classification, do it with an `errgroup` and a semaphore limiting concurrency to 10-20.
3.  **Size Limits**: Introduce hard limits on file sizes read for classification. A 50MB SQL dump should not be sent to an LLM for classification.

## 14. Conclusion

The codenerd Campaign Decomposer is a powerful engine, but its current test suite leaves it exposed to numerous edge cases, type coercion vulnerabilities, and extreme scaling issues. By implementing the tests outlined in this document, we can secure the foundation of the Autopoiesis orchestration loop.

Signed,
Jules
QA Automation Engineer
2026-02-28 00:18 EST
