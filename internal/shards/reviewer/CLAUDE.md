# internal/shards/reviewer - ReviewerShard Code Review System

This package implements the ReviewerShard for code review with neuro-symbolic hypothesis verification. It combines deterministic Mangle rules for issue detection with LLM verification for semantic understanding.

**Related Packages:**
- [internal/core](../../core/CLAUDE.md) - Kernel for hypothesis querying
- [internal/articulation](../../articulation/CLAUDE.md) - Piggyback Protocol for LLM responses
- [internal/store](../../store/CLAUDE.md) - LearningStore for pattern persistence

## Architecture

The reviewer implements a neuro-symbolic pipeline:
1. **Static Analysis** - Regex-based security/style checks in `checks.go`
2. **Mangle Hypotheses** - Kernel queries generate candidate issues in `hypotheses.go`
3. **LLM Verification** - Hypotheses verified semantically in `verification.go`
4. **Specialist Orchestration** - Domain experts consulted via `specialists.go`
5. **Creative Enhancement** - Two-pass improvement suggestions in `creative.go`

## File Index

| File | Description |
|------|-------------|
| `reviewer.go` | Main ReviewerShard implementation with ReviewerConfig and Execute() method. Exports ReviewFinding, ReviewResult, ReviewSeverity types and DefaultReviewerConfig() for shard initialization. |
| `hypotheses.go` | Hypothesis generation from Mangle kernel queries for LLM verification. Exports HypothesisType constants (sql_injection, race_condition, etc.) and Hypothesis struct with confidence scoring. |
| `verification.go` | LLM verification flow for Mangle-generated hypotheses implementing neuro-symbolic pattern. Exports Verdict, VerifiedFinding types and VerifyHypotheses() for semantic issue confirmation. |
| `impact.go` | Impact-aware context builder using Mangle queries to find affected callers. Exports ImpactContext, ModifiedFunction, ImpactedCaller types and BuildImpactContext() for targeted review scope. |
| `preflight.go` | Pre-flight compilation checks running go build/vet before LLM review. Exports PreFlightCheck() that validates code compiles cleanly before analysis. |
| `specialists.go` | Specialist agent matching using technology detection patterns. Exports AgentRegistry, RegisteredAgent, SpecialistMatch types and MatchSpecialistsForReview() for domain expert selection. |
| `checks.go` | Static security and style checks using regex patterns for SQL injection, XSS, command injection. Implements checkSecurity() with severity scoring and language-specific rule filtering. |
| `custom_rules.go` | User-defined custom review rules loaded from JSON configuration files. Exports CustomRule, CustomRulesFile types and LoadCustomRules() for extending built-in checks. |
| `creative.go` | Two-pass creative enhancement pipeline for improvement suggestions (Steps 8-12). Exports ExecuteCreativeEnhancement() with first-pass analysis, vector search, self-interrogation, and second-pass synthesis. |
| `creative_prompts.go` | LLM prompt builders for creative enhancement analysis. Implements buildFirstPassPrompt() and buildSecondPassPrompt() with holographic context and existing findings. |
| `enhancement.go` | Type definitions for creative enhancement pipeline results. Exports EnhancementResult, FileSuggestion, ModuleSuggestion, SystemInsight, FeatureIdea types for multi-level improvement tracking. |
| `dependencies.go` | One-hop dependency fetching for review context using kernel dependency_link facts. Exports DependencyContext and getOneHopDependencies() for upstream/downstream file loading. |
| `format.go` | Output formatting functions for human-readable review results. Implements formatResult() generating markdown tables with severity icons and JSON metadata blocks. |
| `knowledge.go` | Knowledge base query helpers for specialist integration using vector and graph stores. Exports RetrievedKnowledge and LoadAndQueryKnowledgeBase() for domain knowledge retrieval. |
| `metrics.go` | Code metrics calculation including cyclomatic complexity and language detection. Exports Language constants, detectLanguage(), and regex patterns for decision keyword counting. |
| `persistence.go` | Review persistence and export to knowledge database and markdown files. Exports PersistedReview, PersistReview() for storing reviews in cold storage with vector embeddings. |
| `autopoiesis.go` | Self-improvement pattern tracking for autopoiesis learning loops. Exports trackReviewPatterns(), LearnAntiPattern() for persisting flagged/approved patterns to LearningStore. |
| `facts.go` | Fact generation for kernel propagation from review results. Implements assertInitialFacts(), generateFacts() creating review_finding, security_issue, code_metrics predicates. |
| `feedback.go` | Reviewer feedback loop for self-correction and accuracy tracking. Exports ReviewFeedback, RejectedFinding, FalsePositivePattern types and RecordFinding() for learning from user corrections. |
| `llm.go` | Piggyback Protocol processing for all LLM responses in reviewer. Exports processLLMResponse() extracting control_packet for kernel routing and surface_response for display. |
| `specialist_review.go` | Task formatting for specialist domain reviews with knowledge injection. Exports SpecialistReviewTask, FormatSpecialistReviewTask(), BuildSpecialistTask() for domain expert task construction. |
| `reviewer_test.go` | Unit tests for ReviewerShard core functionality. Tests Execute(), finding generation, and result formatting. |
| `hypotheses_test.go` | Unit tests for hypothesis generation from Mangle queries. Tests HypothesisType classification and confidence scoring logic. |
| `verification_test.go` | Unit tests for LLM verification flow and verdict parsing. Tests VerifyHypotheses() and confirmed/dismissed decision handling. |
| `impact_test.go` | Unit tests for impact context building from modified functions. Tests BuildImpactContext() with caller resolution and priority scoring. |
| `preflight_test.go` | Unit tests for pre-flight compilation check execution. Tests go build/vet integration and error detection. |
| `specialists_test.go` | Unit tests for specialist matching technology patterns. Tests MatchSpecialistsForReview() with various file types and tech stacks. |
| `specialist_review_test.go` | Unit tests for specialist task formatting and knowledge injection. Tests FormatSpecialistReviewTask() output structure. |
| `creative_test.go` | Unit tests for creative enhancement pipeline execution. Tests first-pass, second-pass, and vector search integration. |
| `knowledge_test.go` | Unit tests for knowledge base loading and query operations. Tests LoadAndQueryKnowledgeBase() with vector and graph retrieval. |
| `metrics_test.go` | Unit tests for cyclomatic complexity calculation across languages. Tests detectLanguage() and decision keyword counting. |
| `persistence_test.go` | Unit tests for review persistence to knowledge database. Tests PersistReview() storage and fact generation. |
| `advanced_logic_test.go` | Advanced integration tests for neuro-symbolic pipeline. Tests full hypothesis→verification→finding flow with kernel integration. |

## Key Types

### ReviewerShard
```go
type ReviewerShard struct {
    id             string
    kernel         *core.RealKernel
    llmClient      perception.LLMClient
    learningStore  *store.LearningStore
    reviewerConfig ReviewerConfig
    customRules    []CustomRule
    flaggedPatterns map[string]int
    approvedPatterns map[string]int
}
```

### Hypothesis
```go
type Hypothesis struct {
    Type       HypothesisType // "unsafe_deref", "sql_injection", etc.
    File       string
    Line       int
    Variable   string
    LogicTrace string  // Mangle derivation for explainability
    Confidence float64
    Priority   int
}
```

### Verdict
```go
type Verdict struct {
    Decision   string  // "CONFIRMED" or "DISMISSED"
    Reasoning  string
    Fix        string
    Confidence float64
}
```

## Neuro-Symbolic Pipeline

```
File Changes
    |
    v
PreFlightCheck() → Compilation OK?
    |
    v
checkSecurity() → Static regex findings
    |
    v
GenerateHypotheses() → Mangle kernel queries
    |
    v
VerifyHypotheses() → LLM semantic verification
    |
    v
MatchSpecialistsForReview() → Domain experts
    |
    v
AggregatedReview → Findings + Insights
```

## Testing

```bash
go test ./internal/shards/reviewer/...
```
