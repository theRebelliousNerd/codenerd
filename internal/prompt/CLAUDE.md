# internal/prompt - JIT Prompt Compiler + ConfigFactory

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

This package implements codeNERD's JIT (Just-In-Time) Prompt Compiler and ConfigFactory. Together, they compile optimal system prompts and agent configurations from atomic fragments based on compilation context (operational mode, agent type, language, intent, etc.).

**Key Components:**
1. **JIT Prompt Compiler** - Runtime prompt assembly from prompt atoms
2. **ConfigFactory** - AgentConfig generation (tools + policies + mode)

These components replaced ~35,000 lines of hardcoded shard implementations with declarative, runtime-configurable agent behavior.

**Related Packages:**
- [internal/session](../session/CLAUDE.md) - Session Executor consuming JIT + ConfigFactory output
- [internal/jit/config](../jit/CLAUDE.md) - AgentConfig schema
- [internal/articulation](../articulation/CLAUDE.md) - PromptAssembler consuming JIT output
- [internal/store](../store/CLAUDE.md) - Prompt atom persistence
- [internal/embedding](../embedding/CLAUDE.md) - Vector search for atom selection

## Architecture

The JIT system achieves "infinite" effective prompt length and zero-boilerplate agent creation through:

### JIT Prompt Compiler
1. **Atomic decomposition** - Prompts stored as small, reusable YAML atoms
2. **Context-aware selection** - Only relevant atoms selected via Mangle + vector
3. **Skeleton/Flesh bifurcation** - Mandatory vs probabilistic atoms
4. **Budget-constrained assembly** - Polymorphic content fitting within token limits

### ConfigFactory
1. **ConfigAtom retrieval** - Fetch tools/policies for intent verb
2. **AgentConfig generation** - Merge ConfigAtoms into complete configuration
3. **Tool enforcement** - Session Executor validates tool calls against AllowedTools
4. **Policy loading** - Mangle files loaded into kernel for safety checks

**Integration Flow:**
```
User Input → Intent Routing → persona(/coder)
                                  ↓
                    ┌─────────────┴─────────────┐
                    ↓                           ↓
            JIT Compiler                  ConfigFactory
    (CompilationContext → Prompt)  (Intent → AgentConfig)
                    ↓                           ↓
            "You are a code fixer..."   {Tools: [...], Policies: [...]}
                    ↓                           ↓
                    └─────────────┬─────────────┘
                                  ↓
                          Session Executor
                                  ↓
                      LLM + VirtualStore + Safety
```

## File Index

| File | Description |
|------|-------------|
| `atoms.go` | Core atom types and AtomCategory constants (identity, protocol, safety, etc.). Exports PromptAtom with 14 category types and MatchesContext() for selector dimension matching. |
| `compiler.go` | JITPromptCompiler orchestrating the full compilation pipeline. Exports Compile(), CompilationStats with phase timing metrics, and SearchResult for vector integration. |
| `config_factory.go` | **NEW:** ConfigFactory generating AgentConfig objects from intent. Exports Generate(), ConfigAtom (tools + policies), and GetAtom() for intent-based config retrieval. |
| `assembler.go` | FinalAssembler concatenating atoms into final prompt string with category ordering. Exports NewFinalAssembler() with configurable section headers and separators. |
| `selector.go` | AtomSelector implementing Skeleton/Flesh bifurcation for hybrid selection. Exports ScoredAtom, NewAtomSelector() with Mangle rules + vector search scoring. |
| `context.go` | CompilationContext holding 10 contextual tiers for atom selection. Exports 10-tier structure: OperationalMode, CampaignPhase, BuildLayer, AgentType (formerly ShardType), Language, etc. |
| `budget.go` | BudgetManager allocating tokens across categories with priority levels. Exports BudgetPriority constants, CategoryBudget, and FitToBudget() for polymorphic content. |
| `resolver.go` | DependencyResolver ordering atoms by dependencies with cycle detection. Exports OrderedAtom, NewDependencyResolver() with topological sorting. |
| `loader.go` | AtomLoader for runtime YAML→SQLite ingestion of prompt atoms. Exports LoadFromYAML(), LoadFromDirectory(), StoreAtom() with embedding generation. |
| `embedded.go` | Embedded corpus loader using go:embed for baked-in atoms. Exports LoadEmbeddedCorpus() extracting atoms from internal/prompt/atoms/ at startup. |
| `baseline.go` | Baseline prompt assembly when JIT unavailable or disabled. Exports AssembleEmbeddedBaselinePrompt() using mandatory atoms without Mangle/vector search. |
| `manifest.go` | PromptManifest for detailed observability into compilation decisions. Exports AtomManifestEntry, DroppedAtomEntry for "Flight Recorder" diagnostics. |
| `vector_searcher.go` | CompilerVectorSearcher for semantic search over prompt_atoms embeddings. Exports NewCompilerVectorSearcher() with task-type-aware embedding support. |
| `predicate_selector.go` | PredicateSelector for context-aware predicate injection (~50-100 from 799). Exports SelectionContext, SelectedPredicate, Select() for domain-filtered predicates. |
| `default_corpus.go` | MaterializeDefaultPromptCorpus extracting embedded DB to filesystem. Exports HydrateAtomContextTags() for legacy corpus migration. |
| `assembler_test.go` | Unit tests for FinalAssembler category ordering and separator handling. Tests section header generation and atom concatenation. |
| `atoms_test.go` | Unit tests for PromptAtom context matching and category classification. Tests MatchesContext() with various selector combinations. |
| `budget_test.go` | Unit tests for budget allocation and polymorphic content fitting. Tests BudgetManager with various priority and token configurations. |
| `compiler_test.go` | Unit tests for JITPromptCompiler end-to-end compilation. Tests Compile() with various context configurations. |
| `compiler_kernel_atoms_test.go` | Integration tests for kernel-based atom selection. Tests Mangle rule-driven atom filtering. |
| `context_test.go` | Unit tests for CompilationContext dimension matching. Tests tier priority and context hashing. |
| `selector_test.go` | Unit tests for AtomSelector skeleton/flesh bifurcation. Tests combined scoring with logic and vector weights. |
| `resolver_test.go` | Unit tests for DependencyResolver topological sorting. Tests cycle detection and dependency ordering. |
| `embedded_test.go` | Unit tests for embedded corpus loading from go:embed. Tests YAML parsing and atom extraction. |
| `loader_test.go` | Unit tests for AtomLoader YAML parsing and SQLite storage. Tests LoadFromYAML() with various atom configurations. |
| `loader_example_test.go` | Example tests demonstrating loader usage patterns. Tests directory loading and embedding generation. |
| `loader_yaml_fields_test.go` | Unit tests for YAML field parsing edge cases. Tests all 11 selector dimension fields. |
| `sync/synchronizer.go` | AgentSynchronizer syncing agent YAML to shard-specific SQLite. Exports SyncAll() for .nerd/agents/ → .nerd/shards/ sync. |

## Key Types

### PromptAtom
```go
type PromptAtom struct {
    ID       string
    Category AtomCategory
    Content  string

    // Contextual Selectors (11 dimensions)
    OperationalModes []string  // ["/active", "/debugging", "/dream"]
    AgentTypes       []string  // ["/coder", "/tester", "/reviewer"] (formerly ShardTypes)
    IntentVerbs      []string  // ["/fix", "/debug", "/refactor"]
    Languages        []string  // ["/go", "/python", "/typescript"]

    // Composition
    Priority      int
    IsMandatory   bool
    DependsOn     []string
    ConflictsWith []string
}
```

### CompilationContext
```go
type CompilationContext struct {
    // 10 Tiers
    OperationalMode string  // Tier 1
    CampaignPhase   string  // Tier 2
    BuildLayer      string  // Tier 3
    InitPhase       string  // Tier 4
    NorthstarPhase  string  // Tier 5
    OuroborosStage  string  // Tier 6
    IntentVerb      string  // Tier 7
    AgentType       string  // Tier 8 (formerly ShardType)
    Language        string  // Tier 9
    Framework       string  // Tier 10

    TokenBudget int
    WorldStates []string
}
```

### CompilationStats
```go
type CompilationStats struct {
    Duration        time.Duration
    AtomsSelected   int
    SkeletonAtoms   int
    FleshAtoms      int
    TokensUsed      int
    TokenBudget     int
    BudgetUtilization float64
}
```

## Atom Categories

| Category | Purpose | Skeleton? |
|----------|---------|-----------|
| identity | Agent identity and capabilities | Yes |
| protocol | Operational protocols (Piggyback, OODA) | Yes |
| safety | Constitutional constraints | Yes |
| methodology | Problem-solving approach (TDD) | Yes |
| language | Language-specific guidance | No |
| framework | Framework-specific guidance | No |
| domain | Project/domain context | No |
| campaign | Active campaign context | No |
| context | Dynamic context (files, symbols) | No |
| exemplar | Few-shot examples | No |

## Compilation Pipeline

```
CompilationContext
    |
    v
Collect Atoms (embedded + DB)
    |
    v
Select Skeleton (mandatory categories)
    |
    v
Select Flesh (vector search + Mangle filter)
    |
    v
Resolve Dependencies
    |
    v
Fit Budget (polymorphism: standard→concise→min)
    |
    v
Assemble (category ordering)
    |
    v
Final Prompt + CompilationStats
```

## Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/mattn/go-sqlite3` - SQLite storage
- `internal/embedding` - Vector embeddings

## Testing

```bash
go test ./internal/prompt/...
```

---

**Remember: Push to GitHub regularly!**
