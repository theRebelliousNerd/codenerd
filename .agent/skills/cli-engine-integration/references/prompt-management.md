# JIT Prompt Compiler: Kernel-Driven Dynamic Prompt Assembly

## Overview

codeNERD uses a **Just-in-Time (JIT) Prompt Compiler** where system prompts are not static text blocks but **compiled binaries** generated at runtime. This achieves:

- **100:1 Semantic Compression** - Only relevant atoms assembled
- **Dependency Resolution** - Safety rules auto-included with dangerous tools
- **Conflict Resolution** - No "schizophrenic LLM" with contradictory instructions
- **Phase Gating** - Behavior changes as development progresses

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                         JIT PROMPT COMPILER                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌────────────────────┐    ┌────────────────────┐    ┌────────────────┐ │
│  │   VECTOR DB        │    │   MANGLE KERNEL    │    │   GO RUNTIME   │ │
│  │   (Search Engine)  │───▶│   (Linker)         │───▶│   (Assembler)  │ │
│  │                    │    │                    │    │                │ │
│  │  - Atomic Prompts  │    │  - Dependencies    │    │  - Formatting  │ │
│  │  - Semantic Search │    │  - Conflicts       │    │  - Ordering    │ │
│  │  - Similarity Rank │    │  - Phase Gates     │    │  - Token Mgmt  │ │
│  └────────────────────┘    └────────────────────┘    └────────────────┘ │
│           │                         │                        │          │
│           └─────────────────────────┴────────────────────────┘          │
│                                     │                                    │
│                                     ▼                                    │
│                    ┌────────────────────────────────┐                   │
│                    │      COMPILED PROMPT           │                   │
│                    │  (Hot-swappable, Phase-aware)  │                   │
│                    └────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────────────┘
```

## The Three-Layer System

| Layer | Tool | Role | Analogy |
|-------|------|------|---------|
| **Discovery** | Vector DB | Find relevant prompt atoms | Search Engine |
| **Resolution** | Mangle Kernel | Resolve dependencies/conflicts | Linker |
| **Assembly** | Go Runtime | Format and order final prompt | Compiler Output |

## 1. Atomic Prompts: The Data Model

Break large prompts into composable "atoms" stored in the vector DB. Mangle holds only metadata (IDs, categories, dependencies) while text lives in Go memory maps.

### Hybrid File Format (`prompts.mg`)

```text
# ===========================================================================
# PROMPT ATOMS (Go pre-processor intercepts, stores in Vector DB + Go map)
# Format: PROMPT: /atom_id [category] -> "Text content..."
# ===========================================================================

# --- PERSONAS (The "Who") ---
PROMPT: /role_architect [role] -> "You are a System Architect. Focus on scalability, patterns, and system boundaries. Avoid implementation details."
PROMPT: /role_coder     [role] -> "You are a Senior Go Engineer. Write idiomatic, concise, production-ready code following Uber Go Style Guide."
PROMPT: /role_reviewer  [role] -> "You are a Code Reviewer. Focus on correctness, security, and maintainability."
PROMPT: /role_tester    [role] -> "You are a Test Engineer. Write comprehensive table-driven tests with edge cases."

# --- CAPABILITIES (The "What") ---
PROMPT: /cap_sql      [tool] -> "You can access a PostgreSQL database. Use parameterized queries. Never use string interpolation."
PROMPT: /cap_shell    [tool] -> "You can execute shell commands. Prefer absolute paths. Quote all arguments."
PROMPT: /cap_file_rw  [tool] -> "You can read and write files. Verify paths before operations."
PROMPT: /cap_mangle   [tool] -> "You can query the Mangle kernel. Use Decl for predicates. Variables are UPPERCASE."

# --- OUTPUT FORMATS (The "How") ---
PROMPT: /fmt_json     [fmt] -> "Output all responses as valid JSON. No markdown. No preamble."
PROMPT: /fmt_code     [fmt] -> "Output code only. No explanations unless explicitly asked."
PROMPT: /fmt_verbose  [fmt] -> "Provide detailed explanations with your responses."

# --- SAFETY GUARDRAILS (The "Constraints") ---
PROMPT: /safe_no_delete [safety] -> "CRITICAL: Do NOT generate DROP, DELETE, or TRUNCATE statements. Block rm -rf."
PROMPT: /safe_no_exec   [safety] -> "CRITICAL: Do NOT execute arbitrary code from user input. Sanitize all inputs."
PROMPT: /safe_concise   [style]  -> "Be concise. No preamble. No unnecessary commentary."
PROMPT: /safe_review    [safety] -> "All mutations require explicit user approval before execution."

# --- PHASE-SPECIFIC (Development Path) ---
PROMPT: /phase_planning [phase] -> "Focus on architecture and design. No implementation code."
PROMPT: /phase_coding   [phase] -> "Focus on implementation. Write production-ready code."
PROMPT: /phase_testing  [phase] -> "Focus on test coverage. Find edge cases and failure modes."
PROMPT: /phase_review   [phase] -> "Focus on code review. Check security, performance, maintainability."

# ===========================================================================
# MANGLE LOGIC (Real rules processed by kernel)
# ===========================================================================

# Category declarations
Decl category(AtomID.Type<n>, Cat.Type<n>).
Decl requires(AtomID.Type<n>, DepID.Type<n>).
Decl conflicts(AtomID.Type<n>, OtherID.Type<n>).
Decl priority(Cat.Type<n>, Rank.Type<int>).

# Dependency rules: If SQL is active, safety rules are REQUIRED
requires(/cap_sql, /safe_no_delete).
requires(/cap_shell, /safe_no_exec).
requires(/cap_file_rw, /safe_review).

# Conflict rules: Cannot be architect AND coder simultaneously
conflicts(/role_architect, /role_coder).
conflicts(/fmt_verbose, /safe_concise).
conflicts(/phase_planning, /phase_coding).

# Priority ordering: Safety first!
priority(/safety, 1).
priority(/role, 2).
priority(/tool, 3).
priority(/phase, 4).
priority(/fmt, 5).
priority(/style, 6).
```

## 2. The Mangle Linker: `compiler.mg`

Mangle determines which atoms make it into the final prompt. It filters Vector DB results through business logic.

```mangle
# ===========================================================================
# PROMPT COMPILER LOGIC (compiler.mg)
# ===========================================================================

# --- INPUTS (Injected by Go at Runtime) ---
Decl vector_hit(AtomID.Type<n>, Score.Type<float>).
Decl current_phase(Phase.Type<n>).
Decl current_shard(ShardType.Type<n>).
Decl env_mode(Mode.Type<n>).  # /production, /development, /testing

# --- SELECTION LOGIC ---

# A. Semantic Relevance (Vector DB results with high confidence)
selected(P) :-
    vector_hit(P, Score),
    Score > 0.75.

# B. Dependency Resolution (The Linker - recursive!)
# If P is selected, also select everything it requires.
# This ensures safety rules are auto-included when tools are picked.
selected(Dep) :-
    selected(P),
    requires(P, Dep).

# C. Phase Gating (Development Path)
# Force role based on current phase
selected(/role_architect) :- current_phase(/planning).
selected(/role_coder)     :- current_phase(/coding).
selected(/role_tester)    :- current_phase(/testing).
selected(/role_reviewer)  :- current_phase(/review).

# Force phase prompt
selected(/phase_planning) :- current_phase(/planning).
selected(/phase_coding)   :- current_phase(/coding).
selected(/phase_testing)  :- current_phase(/testing).
selected(/phase_review)   :- current_phase(/review).

# D. Shard-Specific Defaults
selected(/fmt_code)    :- current_shard(/coder).
selected(/fmt_verbose) :- current_shard(/reviewer).
selected(/safe_review) :- current_shard(/coder).

# E. Environment-Based Safety
selected(/safe_no_delete) :- env_mode(/production).
selected(/safe_no_exec)   :- env_mode(/production).

# --- EXCLUSION LOGIC ---

# Exclude formatting details in planning phase
excluded(P) :-
    current_phase(/planning),
    category(P, /fmt).

# Exclude verbose in coder shard (contradicts concise)
excluded(/fmt_verbose) :-
    current_shard(/coder).

# Production blocks dangerous tools entirely
excluded(/cap_shell) :-
    env_mode(/production).

# --- CONFLICT RESOLUTION ---

# If conflict exists, suppress based on phase precedence
suppressed(P_Loser) :-
    selected(P_Winner),
    selected(P_Loser),
    conflicts(P_Winner, P_Loser),
    current_phase(Phase),
    phase_prefers(Phase, P_Winner, P_Loser).

# Phase preferences for conflict resolution
phase_prefers(/planning, /role_architect, /role_coder).
phase_prefers(/coding, /role_coder, /role_architect).
phase_prefers(/testing, /role_tester, /role_coder).
phase_prefers(/review, /role_reviewer, /role_coder).

# --- FINAL ASSEMBLY ---

# Final atom passes all filters
final_atom(P) :-
    selected(P),
    !excluded(P),
    !suppressed(P).

# Ordered by category priority (safety first!)
ordered_result(P, Rank) :-
    final_atom(P),
    category(P, Cat),
    priority(Cat, Rank).

# --- DEBUGGING RULES ---

# Find why an atom was excluded
debug_excluded(P, "phase_filter") :-
    selected(P),
    excluded(P),
    current_phase(Phase),
    category(P, Cat),
    phase_excludes_category(Phase, Cat).

debug_excluded(P, "conflict") :-
    selected(P),
    suppressed(P).

# Count final atoms (for token budgeting)
atom_count(N) :-
    final_atom(_) |> let N = fn:Count().
```

## 3. Go Runtime: The Assembler

```go
package prompt

import (
    "bufio"
    "fmt"
    "os"
    "sort"
    "strings"

    "github.com/google/mangle/ast"
    "github.com/google/mangle/factstore"
)

// AtomicPrompt represents a decomposed prompt fragment.
type AtomicPrompt struct {
    ID       string  // e.g., "/role_coder"
    Category string  // e.g., "role", "tool", "safety"
    Text     string  // The actual prompt content
}

// PromptTextMap holds atom ID -> text mappings (loaded at startup).
var PromptTextMap = make(map[string]string)

// CompiledResult represents a Mangle-selected atom with priority.
type CompiledResult struct {
    AtomID string
    Rank   int
}

// PromptCompiler implements JIT prompt compilation.
type PromptCompiler struct {
    vectorDB VectorStore
    kernel   MagleKernel
}

// NewPromptCompiler creates a compiler with vector and logic backends.
func NewPromptCompiler(vectorDB VectorStore, kernel MagleKernel) *PromptCompiler {
    return &PromptCompiler{
        vectorDB: vectorDB,
        kernel:   kernel,
    }
}

// CompilePrompt generates a context-aware prompt from atomic fragments.
func (pc *PromptCompiler) CompilePrompt(ctx context.Context, query string, phase string, shardType string, env string) (string, error) {
    // 1. DISCOVERY: Vector search for relevant atoms
    hits, err := pc.vectorDB.Search(ctx, query, 10)
    if err != nil {
        return "", fmt.Errorf("vector search failed: %w", err)
    }

    // 2. STATE INJECTION: Set up Mangle context
    store := factstore.NewSimpleInMemoryStore()

    // Inject current phase
    if phase != "" {
        atom := ast.NewAtom("current_phase", ast.Name("/"+phase))
        store.Add(atom)
    }

    // Inject current shard type
    if shardType != "" {
        atom := ast.NewAtom("current_shard", ast.Name("/"+shardType))
        store.Add(atom)
    }

    // Inject environment mode
    if env != "" {
        atom := ast.NewAtom("env_mode", ast.Name("/"+env))
        store.Add(atom)
    }

    // Inject vector hits as facts
    for _, hit := range hits {
        atom := ast.NewAtom("vector_hit", ast.Name(hit.ID), ast.Float64(hit.Score))
        store.Add(atom)
    }

    // 3. COMPILATION: Run Mangle linker
    results, err := pc.kernel.Query(ctx, "ordered_result", store)
    if err != nil {
        return "", fmt.Errorf("mangle compilation failed: %w", err)
    }

    // 4. ASSEMBLY: Sort by rank and stitch text
    compiled := make([]CompiledResult, 0, len(results))
    for _, r := range results {
        atomID, _ := r["P"].(string)
        rank, _ := r["Rank"].(int64)
        compiled = append(compiled, CompiledResult{AtomID: atomID, Rank: int(rank)})
    }

    sort.Slice(compiled, func(i, j int) bool {
        return compiled[i].Rank < compiled[j].Rank
    })

    var promptBuilder strings.Builder
    for _, c := range compiled {
        if text, ok := PromptTextMap[c.AtomID]; ok {
            promptBuilder.WriteString(text)
            promptBuilder.WriteString("\n\n")
        }
    }

    return promptBuilder.String(), nil
}

// LoadHybridFile parses a prompts.mg file, routing content to correct backends.
func LoadHybridFile(path string, vectorDB VectorStore, store factstore.FactStore) (string, error) {
    file, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var mangleCode strings.Builder

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // Skip comments and empty lines
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        // Route PROMPT atoms to Vector DB + Text Map
        if strings.HasPrefix(line, "PROMPT:") {
            atomID, category, text := parsePromptLine(line)
            if atomID != "" {
                // Store text in Go map
                PromptTextMap[atomID] = text

                // Embed and store in Vector DB for semantic search
                if vectorDB != nil {
                    vectorDB.Add(ctx, atomID, text, category)
                }

                // Create category fact for Mangle
                mangleCode.WriteString(fmt.Sprintf("category(%s, /%s).\n", atomID, category))
            }
            continue
        }

        // Keep real Mangle logic
        mangleCode.WriteString(line + "\n")
    }

    return mangleCode.String(), nil
}

// parsePromptLine extracts atom ID, category, and text from PROMPT: lines.
// Format: PROMPT: /atom_id [category] -> "Text content..."
func parsePromptLine(line string) (atomID, category, text string) {
    // Remove "PROMPT:" prefix
    line = strings.TrimPrefix(line, "PROMPT:")
    line = strings.TrimSpace(line)

    // Extract atom ID (starts with /)
    parts := strings.SplitN(line, " ", 2)
    if len(parts) < 2 {
        return "", "", ""
    }
    atomID = parts[0]

    // Extract category [in brackets]
    remaining := parts[1]
    if strings.HasPrefix(remaining, "[") {
        endBracket := strings.Index(remaining, "]")
        if endBracket > 0 {
            category = remaining[1:endBracket]
            remaining = strings.TrimSpace(remaining[endBracket+1:])
        }
    }

    // Extract text after ->
    if strings.HasPrefix(remaining, "->") {
        remaining = strings.TrimPrefix(remaining, "->")
        remaining = strings.TrimSpace(remaining)
        // Remove surrounding quotes
        if strings.HasPrefix(remaining, "\"") && strings.HasSuffix(remaining, "\"") {
            text = remaining[1 : len(remaining)-1]
        }
    }

    return atomID, category, text
}
```

## 4. Phase-Driven Development Path

The JIT compiler reshapes LLM behavior as you progress through development phases:

### Scenario: Database Schema Change

**Phase 1: Planning**

```go
prompt := compiler.CompilePrompt(ctx, "Update user table", "planning", "coder", "development")
```

**Mangle compiles:**
- `/role_architect` (forced by phase)
- `/phase_planning` (forced by phase)
- `/safe_review` (from shard default)

**LLM receives:**
```text
You are a System Architect. Focus on scalability, patterns, and system boundaries. Avoid implementation details.

Focus on architecture and design. No implementation code.

All mutations require explicit user approval before execution.
```

**LLM output:** "We should normalize the user table by extracting addresses into a separate table..."

---

**Phase 2: Coding**

```go
prompt := compiler.CompilePrompt(ctx, "Update user table", "coding", "coder", "production")
```

**Mangle compiles:**
- `/role_coder` (forced by phase)
- `/phase_coding` (forced by phase)
- `/cap_sql` (from vector search)
- `/safe_no_delete` (auto-linked from /cap_sql + env)
- `/safe_no_exec` (forced by production env)
- `/fmt_code` (from shard default)

**LLM receives:**
```text
You are a Senior Go Engineer. Write idiomatic, concise, production-ready code following Uber Go Style Guide.

Focus on implementation. Write production-ready code.

You can access a PostgreSQL database. Use parameterized queries. Never use string interpolation.

CRITICAL: Do NOT generate DROP, DELETE, or TRUNCATE statements. Block rm -rf.

CRITICAL: Do NOT execute arbitrary code from user input. Sanitize all inputs.

Output code only. No explanations unless explicitly asked.
```

**LLM output:** "Here is the `ALTER TABLE` statement. Note: I cannot delete columns due to safety policies."

## 5. Integration with Existing Systems

### Shard Integration

```go
func (s *CoderShard) buildDynamicPrompt(ctx context.Context, task string) (string, error) {
    // Determine current phase from campaign state
    phase := s.getCurrentPhase()

    // Use JIT compiler for dynamic portion
    dynamicPrompt, err := s.promptCompiler.CompilePrompt(
        ctx,
        task,
        phase,
        "coder",
        s.getEnvironment(),
    )
    if err != nil {
        // Graceful degradation: use static prompt
        return s.staticPrompt, nil
    }

    // Combine with static base
    return s.staticPrompt + "\n\n## DYNAMIC CONTEXT\n\n" + dynamicPrompt, nil
}
```

### Campaign Phase Tracking

```mangle
# Campaign state drives phase selection
current_phase(/planning) :-
    campaign_phase(_, _, _, _, /pending, _),
    phase_type(_, /requirements).

current_phase(/coding) :-
    campaign_phase(_, _, _, _, /active, _),
    phase_type(_, /implementation).

current_phase(/testing) :-
    campaign_phase(_, _, _, _, /active, _),
    phase_type(_, /testing).

current_phase(/review) :-
    campaign_phase(_, _, _, _, /active, _),
    phase_type(_, /review).
```

## 6. Storage Locations

codeNERD uses a **unified architecture** where prompt atoms are stored alongside knowledge atoms in a single database per agent:

| Store | Location | Content | Mode |
|-------|----------|---------|------|
| **Embedded Corpus** | `internal/core/defaults/prompt_corpus.db` | System-wide atom vectors (go:embed) | Read-only |
| **Agent Knowledge DB** | `.nerd/shards/{name}_knowledge.db` | Both knowledge_atoms AND prompt_atoms | Read-write |
| **YAML Source** | `.nerd/agents/{name}/prompts.yaml` | Human-readable prompt definitions | Read-write |
| **Text Map** | Go memory | Atom ID → text mapping | Runtime |

### Unified Schema

Both knowledge and prompt atoms coexist in the same database, with prompt atoms in a dedicated table:

```sql
CREATE TABLE prompt_atoms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    atom_id TEXT NOT NULL UNIQUE,
    version INTEGER DEFAULT 1,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    category TEXT NOT NULL,
    subcategory TEXT,

    -- Context selectors (JSON arrays)
    operational_modes TEXT,     -- ["/development", "/production"]
    campaign_phases TEXT,        -- ["/planning", "/coding", "/testing"]
    build_layers TEXT,           -- ["/foundation", "/features", "/polish"]
    init_phases TEXT,            -- ["/scaffold", "/config", "/deps"]
    northstar_phases TEXT,       -- ["/vision", "/roadmap", "/milestones"]
    ouroboros_stages TEXT,       -- ["/detect", "/generate", "/integrate"]
    intent_verbs TEXT,           -- ["/code", "/test", "/review"]
    shard_types TEXT,            -- ["/coder", "/tester", "/reviewer"]
    languages TEXT,              -- ["go", "python", "rust"]
    frameworks TEXT,             -- ["gin", "cobra", "bubbletea"]
    world_states TEXT,           -- ["/git_dirty", "/tests_failing"]

    -- Control fields
    priority INTEGER DEFAULT 50,
    is_mandatory BOOLEAN DEFAULT FALSE,
    is_exclusive TEXT,           -- JSON: null or category name
    depends_on TEXT,             -- JSON: ["atom_id1", "atom_id2"]
    conflicts_with TEXT,         -- JSON: ["atom_id1", "atom_id2"]

    -- Vector search
    embedding BLOB,
    embedding_task TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Shard Type → Storage Mapping

| Shard Type | Embedded Corpus | Agent Knowledge DB | prompts.yaml |
|------------|-----------------|-------------------|--------------|
| **Type A (Ephemeral)** | ✅ Yes | ❌ No | ❌ No |
| **Type B (Persistent)** | ✅ Yes | ✅ Yes | ✅ Yes (generated by /init) |
| **Type U (User-defined)** | ✅ Yes | ✅ Yes | ✅ Yes (generated by /define-agent) |
| **Type S (System)** | ✅ Yes | ❌ No | ❌ No |

**Rationale:**

- **Type A/S** (Ephemeral/System) use only the embedded corpus - no persistent storage needed
- **Type B/U** (Persistent/User) have dedicated knowledge DBs that also store customized prompt atoms
- YAML files provide human-readable editing for persistent agents

## 7. Benefits

1. **Token Efficiency** - Send only ~5 relevant atoms instead of 8000-char monolithic prompts
2. **Hallucination Firewall** - Mangle blocks dangerous atoms: `excluded(P) :- dangerous(P), env(/production)`
3. **Coherence** - Conflict resolution prevents contradictory instructions
4. **Hot-Swappable** - Change phase, instantly change behavior without prompt editing
5. **Audit Trail** - Mangle derivation shows exactly why each atom was included
6. **Learning** - User corrections become new atoms in learned.db

## 8. Debugging

```mangle
# Query why an atom was excluded
?debug_excluded(/cap_shell, Reason).
# Result: Reason = "env_filter" (blocked in production)

# Query final compilation
?ordered_result(Atom, Rank).
# Result: /safe_no_delete:1, /role_coder:2, /cap_sql:3, /phase_coding:4, /fmt_code:5

# Count atoms (for token budgeting)
?atom_count(N).
# Result: N = 5
```

## 9. Key Files

| File | Purpose |
|------|---------|
| `internal/prompt/compiler.go` | JITPromptCompiler - orchestrates the pipeline |
| `internal/prompt/atoms.go` | PromptAtom type definition |
| `internal/prompt/corpus.go` | EmbeddedPromptCorpus (go:embed) |
| `internal/prompt/selector.go` | AtomSelector (Mangle + Vector hybrid) |
| `internal/prompt/resolver.go` | DependencyResolver (graph traversal) |
| `internal/prompt/budget.go` | TokenBudgetManager (dynamic sizing) |
| `internal/prompt/assembler.go` | FinalAssembler (ordering + formatting) |
| `internal/prompt/loader.go` | YAML→SQLite loader + JIT wiring |
| `internal/prompt/context.go` | CompilationContext (state injection) |
| `internal/core/defaults/prompt_corpus.go` | go:embed for corpus.db |
| `cmd/tools/prompt_builder/main.go` | Build embedded corpus from YAML sources |
| `build/prompt_atoms/**/*.yaml` | 50+ source atoms (14 categories) |

## 10. JIT Wiring Flow

### Bootstrap Sequence (System Startup)

```text
┌─────────────────────────────────────────────────────────────────────┐
│ 1. EMBEDDED CORPUS INITIALIZATION                                   │
│    internal/core/defaults/prompt_corpus.db (go:embed)               │
│    → Loaded into memory at startup                                  │
│    → 50+ system-wide atoms with embeddings                          │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 2. KERNEL BOOT                                                       │
│    → Mangle kernel starts                                           │
│    → VirtualStore initializes                                       │
│    → ShardManager creates system shards (Type S)                    │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 3. SYSTEM SHARD PROMPT COMPILATION                                  │
│    For each Type S shard:                                           │
│      → JITPromptCompiler.Compile()                                  │
│      → AtomSelector queries embedded corpus only                    │
│      → No agent-specific atoms (no knowledge DB)                    │
│      → DependencyResolver pulls safety atoms                        │
│      → FinalAssembler builds system prompt                          │
└─────────────────────────────────────────────────────────────────────┘
```

### Shard Spawn Sequence (Runtime)

```text
User Command: /code "Add user authentication"
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 1. PERCEPTION LAYER                                                  │
│    → Transducer extracts intent: verb=/code, target="authentication"│
│    → Kernel derives: spawn_shard(/coder, Task)                      │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 2. SHARD INSTANTIATION                                              │
│    ShardManager.SpawnShard("coder", Task)                           │
│    → Check shard type: Type A (ephemeral) or Type B (persistent)?  │
│    → If Type B: load .nerd/shards/coder_knowledge.db                │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 3. COMPILATION CONTEXT ASSEMBLY                                     │
│    CompilationContext.Build():                                      │
│      → task_description: "Add user authentication"                  │
│      → shard_type: "/coder"                                         │
│      → campaign_phase: "/coding" (from kernel)                      │
│      → operational_mode: "/development"                             │
│      → world_state: ["/git_clean", "/tests_passing"]                │
│      → language_hints: ["go"] (from project detection)              │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 4. ATOM SELECTION (Hybrid Search)                                   │
│    AtomSelector.Select(context):                                    │
│      → Vector search embedded corpus for semantic matches           │
│      → Vector search agent knowledge DB for custom atoms            │
│      → Filter by context (shard_type, phase, mode, etc.)            │
│      → Mangle rules enforce mandatory/exclusive atoms               │
│      → Result: ~20-30 candidate atoms                               │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 5. DEPENDENCY RESOLUTION                                            │
│    DependencyResolver.Resolve(candidates):                          │
│      → For each atom, check depends_on field                        │
│      → Recursive pull: /cap_sql → /safe_no_delete                   │
│      → Conflict detection: /fmt_verbose ⊥ /safe_concise             │
│      → Result: ~25-40 atoms (with dependencies)                     │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 6. TOKEN BUDGET ENFORCEMENT                                         │
│    TokenBudgetManager.Trim(atoms, maxTokens=4000):                  │
│      → Sort by priority (safety=1, role=2, tools=3, etc.)           │
│      → Mandatory atoms always included                              │
│      → Trim from lowest priority until budget met                   │
│      → Result: ~15-25 atoms (within budget)                         │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 7. FINAL ASSEMBLY                                                   │
│    FinalAssembler.Build(atoms):                                     │
│      → Order by priority (safety first!)                            │
│      → Format with section headers                                  │
│      → Inject campaign context (current phase, goals)               │
│      → Result: Final compiled prompt string                         │
└─────────────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────────────┐
│ 8. SHARD EXECUTION                                                  │
│    CoderShard.Execute(ctx, task, compiledPrompt)                    │
│    → LLM receives JIT-compiled prompt                               │
│    → Executes with phase-aware behavior                             │
│    → Returns result via Piggyback Protocol                          │
└─────────────────────────────────────────────────────────────────────┘
```

## 11. Atom Categories

The JIT compiler organizes atoms into 14 semantic categories, each with specific storage and priority rules:

| Category | Description | Priority | Mandatory | Storage |
|----------|-------------|----------|-----------|---------|
| **constitution** | Hard safety constraints | 1 | ✅ Yes | Embedded only |
| **role** | Persona definitions | 2 | Phase-driven | Embedded + Agent |
| **safety** | Guardrails and policies | 3 | Context-driven | Embedded + Agent |
| **capability** | Tool/API availability | 4 | ❌ No | Embedded + Agent |
| **protocol** | Interaction patterns | 5 | Shard-driven | Embedded + Agent |
| **format** | Output structure | 6 | ❌ No | Embedded + Agent |
| **phase** | Development stage | 7 | Phase-driven | Embedded only |
| **context** | Domain knowledge | 8 | ❌ No | Agent only |
| **style** | Preferences (tone, verbosity) | 9 | ❌ No | Agent only |
| **example** | Few-shot patterns | 10 | ❌ No | Embedded + Agent |
| **constraint** | Task-specific limits | 11 | ❌ No | Agent only |
| **meta** | Self-reflection prompts | 12 | ❌ No | Embedded only |
| **debug** | Troubleshooting aids | 13 | ❌ No | Embedded only |
| **learned** | User-specific patterns | 14 | ❌ No | Agent only |

**Storage Rules:**

- **Embedded only**: System-wide atoms that never change per-agent
- **Agent only**: Customizations specific to a persistent shard
- **Embedded + Agent**: Base atoms in corpus, overrides in agent DB (agent wins)

## 12. Migration from Static Prompts

1. **Decompose** existing 8000+ char prompts into atoms by category
2. **Extract** reusable patterns (safety, format, role)
3. **Define** dependencies and conflicts in YAML metadata
4. **Test** with different phases to verify behavior changes
5. **Build** embedded corpus: `go run cmd/tools/prompt_builder`
6. **Deploy** with graceful fallback to static prompts
