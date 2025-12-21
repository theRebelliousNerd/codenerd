# internal/shards/coder - CoderShard Code Generation Engine

This package implements the CoderShard for code writing, modification, and refactoring. It is language-agnostic with automatic language detection, transaction-based file edits, and integrates with Ouroboros for self-tool generation.

**Related Packages:**
- [internal/core](../../core/CLAUDE.md) - Kernel for fact assertion and policy evaluation
- [internal/articulation](../../articulation/CLAUDE.md) - PromptAssembler for JIT prompt compilation
- [internal/store](../../store/CLAUDE.md) - LearningStore for autopoiesis pattern persistence

## Architecture

The coder shard implements a multi-phase code generation pipeline:
1. **Task Parsing** - Extracts action, target, and instruction from task string
2. **Impact Check** - Queries kernel for safety blocks and breaking changes
3. **Context Reading** - Loads file/directory content with intelligent summarization
4. **Code Generation** - JIT-compiled prompts with language-specific cognitive models
5. **Syntax Validation** - Go stdlib parser and Tree-sitter for multi-language support
6. **Transactional Apply** - Atomic file edits with rollback on failure
7. **Build Verification** - Post-edit build checking with diagnostic extraction

## File Index

| File | Description |
|------|-------------|
| `coder.go` | Main CoderShard implementation with Execute() orchestrating the full pipeline. Exports CoderShard, CoderConfig, CoderResult, CodeEdit, CodeTask types and dependency injection methods (SetLLMClient, SetParentKernel, SetVirtualStore, SetPromptAssembler). |
| `generation.go` | Code generation via LLM with JIT or legacy template prompts. Exports buildSystemPrompt() using PromptAssembler when available, language-specific cognitive models (Go, Python, TypeScript), and buildSessionContextPrompt() for Blackboard Pattern context injection. |
| `context.go` | File and directory context reading with intelligent summarization. Exports readFileContext() for single files, readDirectoryContext() for directory listings with Go file extraction, and extractGoFileSummary() for package docs and function signatures. |
| `apply.go` | Edit application with syntax validation and transactional rollback. Exports applyEdits() routing through VirtualStore or kernel pipeline, syntax gates for Go/Python/Rust/TypeScript via Tree-sitter, and estimateLineChangeRatio() to block accidental rewrites. |
| `transaction.go` | FileTransaction providing atomic multi-file edit batches with rollback. Exports FileTransaction struct with Stage() for backup creation, Commit() to finalize changes, and Rollback() for failure recovery. |
| `build.go` | Post-edit build verification and diagnostic extraction. Exports runBuildCheck() executing detected build commands (go/npm/cargo/python), detectBuildCommand() for project type detection, and parseBuildOutput() for Go-style error parsing. |
| `safety.go` | Impact checking via kernel queries for safety blocks. Exports checkImpact() querying coder_block_write, edit_unsafe, and breaking_change_risk predicates to prevent dangerous edits. |
| `facts.go` | Mangle fact generation from coder results for kernel propagation. Exports generateFacts() creating modified, file_topology, build_state, and promote_to_long_term facts, plus assertTaskFacts() for coder_task predicates. |
| `autopoiesis.go` | Self-improvement pattern tracking for rejection/acceptance learnings. Exports trackRejection(), trackAcceptance() with LearningStore persistence, and loadLearnedPatterns() for initialization from stored patterns. |
| `llm.go` | LLM interaction with exponential backoff retry logic. Exports llmCompleteWithRetry() handling timeouts and rate limits, and isRetryableError() classifying network vs auth errors. |
| `response.go` | LLM response parsing and artifact type classification. Exports ArtifactType constants (project_code, self_tool, diagnostic), CodeResponse, ParsedCodeResult, and parseCodeResponse() extracting edits from JSON or markdown code blocks. |
| `helpers.go` | Utility functions for path resolution and language detection. Exports resolvePath(), hashContent(), detectLanguage() supporting 20+ languages, languageDisplayName(), and isTestFile() for multi-language test detection. |
| `format.go` | Human-readable response formatting from coder results. Exports buildResponse() formatting duration, edits applied, and build status with diagnostic summaries. |
| `ouroboros.go` | Self-tool routing through Ouroboros pipeline for safety checking. Exports routeToOuroboros() submitting self-tool artifacts to ToolGenerator, and fallbackDirectWrite() for degraded mode without safety checks. |
| `context_test.go` | Unit tests for directory context reading and Go file summarization. Tests readDirectoryContext() output formatting and extractGoFileSummary() with various file sizes. |

## Key Types

### CoderShard
```go
type CoderShard struct {
    id              string
    kernel          *core.RealKernel
    llmClient       types.LLMClient
    virtualStore    *core.VirtualStore
    promptAssembler *articulation.PromptAssembler
    learningStore   core.LearningStore
    coderConfig     CoderConfig
    rejectionCount  map[string]int
    acceptanceCount map[string]int
}
```

### CodeEdit
```go
type CodeEdit struct {
    File       string // Target file path
    OldContent string // For modification verification
    NewContent string // Generated content
    Type       string // create, modify, refactor, fix, delete
    Language   string // Detected language
    Rationale  string // Explanation of change
}
```

### ArtifactType
```go
const (
    ArtifactTypeProjectCode ArtifactType = "project_code"  // Default: user's codebase
    ArtifactTypeSelfTool    ArtifactType = "self_tool"     // Route to Ouroboros
    ArtifactTypeDiagnostic  ArtifactType = "diagnostic"    // One-time script
)
```

## Execution Pipeline

```
Task String
    |
    v
parseTask() → Action, Target, Instruction
    |
    v
checkImpact() → Query kernel for blocks
    |
    v
readFileContext() → Load existing content
    |
    v
generateCode() → JIT prompt + LLM
    |
    v
Artifact Type?
    |
    +--[self_tool/diagnostic]--> routeToOuroboros()
    |
    +--[project_code]----------> applyEdits()
                                     |
                                     v
                                 Syntax gates (Go/Python/TS/Rust)
                                     |
                                     v
                                 FileTransaction.Stage()
                                     |
                                     v
                                 VirtualStore.RouteAction()
                                     |
                                     v
                                 runBuildCheck() → diagnostics
                                     |
                                     v
                                 generateFacts() → kernel
```

## Language Cognitive Models

The coder uses language-specific "cognitive models" - detailed style guides injected into prompts:

| Language | Focus Areas |
|----------|-------------|
| Go | Error handling, goroutine lifecycle, table-driven tests, receiver naming |
| Python | PEP 8, mutable defaults, exception handling, type hints |
| TypeScript | any avoidance, discriminated unions, type guards over assertions |
| Generic | Universal error handling, input validation, single responsibility |

## Testing

```bash
go test ./internal/shards/coder/...
```
