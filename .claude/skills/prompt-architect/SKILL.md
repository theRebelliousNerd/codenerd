---
name: prompt-architect
description: Master prompt engineering for codeNERD's neuro-symbolic architecture. Use when writing new shard prompts, auditing existing prompts, debugging LLM behavior, or optimizing context injection. Covers static prompts, dynamic injection, Piggybacking protocol, tool steering, and specialist knowledge hydration.
---

# Prompt Architect Skill

## Purpose & Philosophy

The `prompt-architect` skill is the definitive guide for designing, writing, and auditing prompts within the codeNERD neuro-symbolic architecture.

In codeNERD, a "prompt" is not a static string; it is a **cybernetic control system** that bridges the gap between the Stochastic (LLM) and the Deterministic (Mangle Kernel).

Unlike standard LLM prompting, codeNERD requires mastering a unique set of constraints and capabilities:

- **Dual-Layer Architecture**: Static Go constants (Immutable) mixed with dynamic Mangle-driven injections (Mutable).
- **Piggybacking Protocol**: A mandatory dual-channel output format (Control Packet + Surface Response) to prevent "Premature Articulation" (acting before thinking).
- **Context Compression**: High-ratio semantic compression (>100:1) allowing rich context injection (Git history, dependency graphs, diagnostics) within strict token budgets.
- **Maximalist Prompting**: Because of our high compression ratio, our system prompts can and should be **2-3x longer** than standard frameworks. We have the "legroom" to define God Tier personas, extensive edge cases, and deep methodology without exhausting the window.
- **Deterministic Tool Steering**: Kernel-driven tool selection via Mangle predicates rather than LLM improvisation. The Agent does *not* choose the tool; the Agent *describes* the need, and the Kernel *grants* the tool.

## When to Use

| Activity | Trigger | Goal |
|----------|---------|------|
| **Writing New Shards** | When creating a new shard type (e.g., `SecurityShard`) or modifying core shard logic. | Define the shard's persona, capabilities, and interface with the kernel. |
| **Auditing Prompts** | When reviewing PRs involving prompt changes or running the `audit_prompts.py` script. | Ensure strict adherence to the Piggyback Protocol and Constitutional boundaries. |
| **Debugging LLM Behavior** | When the model ignores tools, hallucinates capabilities, or breaks the thought-first protocol. | Identify "Context Starvation" or "Ambiguous Steering" in the prompt chain. |
| **Optimizing Context** | When tuning the `SessionContext` or `Spreading Activation` rules. | Maximize relevance per token; strictly curate what enters the "Working Memory". |
| **Creating Specialists** | When defining Type B/U persistent specialists. | Inject domain-specific "Knowledge Atoms" to hydrate the specialist's expertise. |

## Core Concepts

| Concept | Description | Criticality | Location |
|---------|-------------|-------------|----------|
| **Static Prompts** | Base instructions defined as Go constants. Immutable bedrock. | **High** | `internal/shards/*/generation.go` |
| **Dynamic Injection** | Real-time context validation/insertion via `SessionContext`. The "Now". | **High** | `internal/core/shard_manager.go` |
| **Piggybacking** | `{"control":..., "surface":...}` JSON protocol. The brain-mouth filter. | **CRITICAL** | `internal/articulation/emitter.go` |
| **Thought-First** | Control packets MUST precede surface text. Prevents hallucinated actions. | **CRITICAL** | Bug #14 Fix |
| **Tool Steering** | Description-based affinity and capability mapping. | **Medium** | `internal/core/tools/definitions.go` |
| **Artifact Types** | `project_code` vs `self_tool` vs `diagnostic`. Defines lifecycle. | **High** | `internal/articulation/types.go` |
| **Spreading Activation** | Logic-driven recall of relevant facts. The "Long Term Memory". | **Medium** | `internal/core/defaults/policy.mg` |

## Quick-Start Patterns

### 1. The Thought-First Protocol (Defense Against Hallucination)

**Context**: The model loves to say "I have fixed the bug" while it is *still writing information* about the bug.
**Pattern**: Force the "Control Packet" (JSON) to be emitted *before* the "Surface Response" (Text).

```go
const SystemPrompt = `...
CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object containing
"control_packet" and "surface_response".

THOUGHT-FIRST ORDERING:
You MUST output control_packet BEFORE surface_response.
Do NOT output text until the control packet is complete.
...`
```

### 2. Artifact Classification (The Autopoiesis Gate)

**Context**: The system needs to know if code is for the *User* (User's Project) or for the *System* (Self-Correction Tool).
**Pattern**: Mandate an `artifact_type` field.

```text
ARTIFACT CLASSIFICATION (MANDATORY):
- "project_code": Code that belongs in the user's codebase (default).
- "self_tool": A temporary tool/utility for codeNERD to use internally (Autopoiesis).
- "diagnostic": A one-time inspection/debugging script (Ephemeral).

If you are creating something for YOUR OWN USE, sets artifact_type to "self_tool".
```

### 3. Context Contextualization (The "Why")

**Context**: Dumping raw data (file lists, errors) confuses the model's priority mechanism.
**Pattern**: Explain *why* context is present to guide attention.

```text
CONTEXT PROVIDED:
- GIT HISTORY: Use this to understand the "why" behind current code (Chesterton's Fence).
- DIAGNOSTICS: These are the active errors you must fix FIRST.
- IMPACT ANALYSIS: These files will break if you change the target.
```

### 4. Deterministic Tooling (Kernel Authority)

**Context**: LLMs love to hallucinate new tools ("I will use the `fix_bug` tool").
**Pattern**: Explicitly forbid improvisation and defer to the Kernel's selection.

```text
AVAILABLE TOOLS (Selected by Kernel):
- [tool_name]: [description]

You MUST use one of these tools. Do not invent tools.
If no tool matches your intent, emit missing_tool_for(intent) in the control packet.
```

### 5. Specialist Knowledge Injection

**Context**: Type B/U specialists require domain knowledge beyond general programming.
**Pattern**: Inject pre-hydrated knowledge atoms and specialist hints from the knowledge base.

```go
// Pattern for Type B/U specialists
const SpecialistPrompt = `
DOMAIN KNOWLEDGE:
{{range .KnowledgeAtoms}}
- [{{.Concept}}]: {{.Content}}
{{end}}

{{range .SpecialistHints}}
- HINT: {{.}}
{{end}}

CONSULTATION PROTOCOL:
When encountering edge cases outside your knowledge atoms, emit
consult_specialist(domain, question) in your control packet.
`
```

## Creating God Tier Prompts

The codeNERD architecture's high compression ratio enables a fundamentally different approach to prompt engineering: **maximalism over minimalism**.

### The God Tier Philosophy

Traditional prompt engineering emphasizes brevity due to token constraints. codeNERD inverts this:

- **Minimum 8,000 characters** for functional prompts (transducers, validators, utilities)
- **Minimum 15,000-20,000 characters** for shard agents (Coder, Reviewer, Tester, Specialists)
- **Production standard: 20,000+ characters** for specialist agents with full domain knowledge

### Why Maximalism Works

1. **Semantic Compression**: Our compression pipeline achieves >100:1 ratios, converting verbose documentation into dense knowledge atoms.

2. **Edge Case Documentation**: Every edge case documented in the prompt saves hours of future debugging. A 2,000-character section on "When NOT to use this tool" prevents dozens of hallucinated actions.

3. **Persona Depth**: A rich persona (background, expertise, constraints, failure modes) creates consistent behavior across thousands of invocations.

4. **Self-Contained Context**: God Tier prompts embed their own reference material, reducing dependence on external documentation that may drift.

### What to Include in a God Tier Prompt

- **Persona** (500+ chars): Role, expertise level, domain boundaries, interaction style
- **Core Responsibilities** (1,000+ chars): Primary tasks, secondary tasks, out-of-scope activities
- **Protocol Definitions** (2,000+ chars): Piggybacking schema, artifact types, control packet structure
- **Methodology** (3,000+ chars): Step-by-step decision trees, edge case handling, failure recovery
- **Context Schema** (1,500+ chars): What each injected field means and how to use it
- **Tool Catalog** (1,000+ chars): Available tools, selection criteria, anti-patterns
- **Edge Cases** (2,000+ chars): "What NOT to do" encyclopedia with examples
- **Output Examples** (2,000+ chars): Side-by-side weak vs strong outputs
- **Constitutional Boundaries** (1,000+ chars): Safety constraints, permission model, escalation paths
- **Quality Standards** (1,500+ chars): What "done" looks like, acceptance criteria, self-verification

**Result**: A 15,000-20,000 character prompt that operates as a self-contained expert system.

## Specialist Creation Quick Reference

### Type B: Project-Specific Specialists

**Creation Trigger**: Automatic during `/init` based on project detection

**Detection Heuristics**:

- `go.mod` with `github.com/go-rod/rod` → Create **RodExpert**
- `go.mod` with `testing` + high test coverage → Create **TestArchitect**
- Project structure with `/cmd`, `/internal` → Create **GoExpert**
- Security scan findings → Create **SecurityAuditor**

**Hydration Sources** (Context7 Protocol):

1. **llms.txt** from project repository (primary)
2. **GitHub README** and `/docs` directory
3. **pkg.go.dev** API documentation for Go packages
4. **Official framework docs** (Rod CDP docs, etc.)
5. **Project commit history** (architectural decisions)

**Knowledge Persistence**: SQLite-backed (Vector + Graph + Cold tiers)

### Type U: User-Defined Specialists

**Creation Trigger**: Manual via `/define-agent` wizard

**Wizard Flow**:

1. **Domain Definition**: User specifies expertise area (e.g., "Stripe API Integration")
2. **Knowledge Source**: User provides URLs, local docs, or example code
3. **Viva Voce Examination**: System tests specialist with domain questions
4. **Quality Gate**: Minimum 80% accuracy on domain Q&A before activation
5. **Registration**: Specialist added to ShardManager registry

**Example Use Cases**:

- "The Stripe API Expert" (internal payment system knowledge)
- "Internal Microservices Guru" (company-specific architecture)
- "Legacy PHP Refactoring Specialist" (codebase-specific patterns)

### Knowledge Hydration Strategies

| Strategy | Source | Compression | Quality | Speed |
|----------|--------|-------------|---------|-------|
| **llms.txt Parsing** | Standardized docs | High (>100:1) | Excellent | Fast |
| **GitHub Docs Crawl** | README + /docs | Medium (50:1) | Good | Medium |
| **API Scraping** | pkg.go.dev, official sites | Low (20:1) | Variable | Slow |
| **Commit History** | Git log with context | Medium (40:1) | Excellent (intentions) | Medium |
| **User Examples** | Uploaded code samples | None (1:1) | Variable | Instant |

### Viva Voce Examination Pattern

The system validates specialist quality through automated examination:

```go
// Example questions for RodExpert specialist
questions := []ExamQuestion{
    {Q: "What is the difference between Page.Element() and Page.Elements()?",
     ExpectedConcepts: ["selector", "single vs multiple", "error handling"]},
    {Q: "How do you wait for an element to be visible before clicking?",
     ExpectedConcepts: ["WaitVisible", "race conditions", "timeout"]},
    {Q: "When should you use Page.MustElement() vs Page.Element()?",
     ExpectedConcepts: ["panic vs error", "test code", "production safety"]},
}

pass_rate := specialist.Examine(questions)
if pass_rate < 0.80 {
    return errors.New("insufficient domain knowledge - re-hydrate KB")
}
```

**Pass Criteria**: 80%+ accuracy with concept coverage verification

## Reference Navigation

The architecture is too deep for a single file. Consult these specialists:

| Document | Scope & Contents |
|----------|------------------|
| [**God Tier Templates**](references/god-tier-templates.md) | **THE STANDARD**: Full 20,000+ character production-ready prompts for Coder, Reviewer, and Transducer. Copy and customize. |
| [Prompt Anatomy](references/prompt-anatomy.md) | **Structure**: Static vs Dynamic layers, The Piggyback Envelope Schema, JSON Robustness, Reasoning Trace Directives. |
| [Context Injection](references/context-injection.md) | **Memory**: Complete `SessionContext` schema (15+ fields), Spreading Activation rules, Token Budgeting, Priority Ordering. |
| [Tool Steering](references/tool-steering.md) | **Action**: Writing effective tool descriptions, Shard Affinity, Mangle Predicates (`tool_capability`), Deterministic Selection. |
| [Shard Prompts](references/shard-prompts.md) | **Persona**: Templates for Coder, Tester, Reviewer, and System Shards. Side-by-side comparison of "Weak" vs "Strong" prompts. |
| [Specialist Prompts](references/specialist-prompts.md) | **Expertise**: Type B/U Specialist architecture (2,600+ lines), Knowledge Atom injection, Viva Voce examination pattern, Domain Hydration strategies. |
| [Anti-Patterns](references/anti-patterns.md) | **Risk**: 15+ failure modes with root causes, symptoms, and fixes. The "What NOT to do" encyclopedia. |
| [Audit Checklist](references/audit-checklist.md) | **Quality**: Structural verification (JSON schema), Semantic verification (Context usage), Safety verification (Constitutional checks). |

## Tools & Scripts

### audit_prompts.py

Automated Python script for prompt quality enforcement. Scans Go files for prompt constants and validates them against the God Tier standard.

**Location**: `.claude/skills/prompt-architect/scripts/audit_prompts.py`

**Command Line Options**:

```bash
# Scan current directory
python audit_prompts.py

# Scan specific root directory
python audit_prompts.py --root /path/to/codebase

# Example: Scan entire codeNERD project
python audit_prompts.py --root c:/CodeProjects/codeNERD
```

**Validation Criteria**:

| Check | Severity | Description |
|-------|----------|-------------|
| Length (Functional) | ERROR | Minimum 8,000 characters for functional prompts |
| Length (Shard) | ERROR | Minimum 15,000 characters for shard/agent prompts |
| Control Packet Schema | ERROR | Must define `control_packet` and `surface_response` |
| Thought-First Ordering | ERROR | Must explicitly state control_packet BEFORE surface_response |
| Artifact Type | WARNING | Required when prompt handles code mutations |
| Reasoning Trace | WARNING | Shard prompts should include reasoning trace directive |
| Context Injection | WARNING | System prompts should have dynamic injection markers (`%s`, `{{.}}`) |
| Intent Reference | WARNING | Control packets should reference Mangle atoms/intents |

**Output Format**:

```text
[INFO] Scanning codebase at c:/CodeProjects/codeNERD...

Issues in internal/shards/coder/generation.go :: CoderSystemPrompt
  - Shard Prompt Insufficient Context Depth: 12483 chars (Min: 15000)
  - Missing explicit 'Thought-First' ordering directive

[FAIL] Issues Found: 2
```

**Exit Codes**:

- `0`: No violations found (safe for CI/CD)
- `1`: Violations detected (blocks merge)

**CI/CD Integration**:

```yaml
# .github/workflows/prompt-quality.yml
- name: Audit Prompts
  run: |
    python .claude/skills/prompt-architect/scripts/audit_prompts.py --root .
```

## Common Issues & Quick Fixes

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| **Model ignores tools** | Hardcoded tool list in prompt; model doesn't see Kernel-selected tools | Replace static tool list with `AVAILABLE TOOLS (Kernel-Selected): %s` and inject dynamically via `SessionContext.AvailableTools` |
| **Premature articulation** | Missing thought-first directive; model outputs surface text before control packet | Add explicit ordering: `CRITICAL: Output control_packet BEFORE surface_response. Do NOT emit text until packet is complete.` |
| **Generic specialist responses** | Empty or stale knowledge atoms; specialist KB not hydrated | Run `/init --force` to re-hydrate knowledge base. Verify `knowledge_atom/3` facts exist in FactStore. Check Vector tier for embeddings. |
| **Context starvation** | No injection markers in static prompt; SessionContext not reaching model | Add injection points: `{{.SessionContext}}` for Go templates or `%s` for fmt.Sprintf. Verify ShardManager passes context. |
| **Tool hallucination** | Weak prohibition language; model invents tools | Use strong language: `You MUST NOT invent tools. If no tool matches, emit missing_tool_for(intent). Kernel will reject hallucinated tools.` |
| **Incorrect artifact type** | Missing artifact classification directive | Add mandatory field: `ARTIFACT CLASSIFICATION (MANDATORY): Set artifact_type to "project_code", "self_tool", or "diagnostic"` |
| **Inconsistent behavior** | Persona drift; insufficient persona definition | Expand persona section to 500+ chars with concrete constraints, expertise level, and interaction style boundaries |
| **Missing context fields** | Template injection errors; SessionContext schema mismatch | Verify field names match exactly: `{{.GitHistory}}` not `{{.git_history}}`. Check ShardManager context population. |
| **Autopoiesis loop** | Specialist creates tools for user codebase instead of internal use | Clarify: `If creating a tool for YOUR OWN debugging, set artifact_type="self_tool". User code gets "project_code".` |
| **Constitutional violations** | Weak safety boundaries; no escalation path | Add explicit constraints: `You MUST NOT [action]. If user requests [prohibited], emit constitutional_violation(reason) and suggest alternative.` |

## JIT Prompt Compiler Architecture

### Status: Phase 8 (Testing) - IN PROGRESS

**Prompts as Compiled Binaries**: Instead of monolithic 20,000-character prompts, break them into atomic composable units. The system compiles the optimal prompt for each context dynamically using a 10-stage compilation pipeline.

### Implementation Status

| Phase | Status | Description |
|-------|--------|-------------|
| **Phase 1: Core Data Model** | ✅ COMPLETE | `PromptAtom` struct with 10 contextual tiers |
| **Phase 2: Storage** | ✅ COMPLETE | Unified SQLite schema in agent knowledge DBs |
| **Phase 3: YAML Loader** | ✅ COMPLETE | Runtime YAML → SQLite ingestion |
| **Phase 4: Compilation Context** | ✅ COMPLETE | `CompilationContext` with 10-tier dimensions |
| **Phase 5: Atom Selector** | ✅ COMPLETE | Rule-based + vector-augmented selection |
| **Phase 6: Dependency Resolver** | ✅ COMPLETE | Graph resolution with conflict detection |
| **Phase 7: Budget Manager** | ✅ COMPLETE | Token budget fitting with priority |
| **Phase 8: Testing** | 🔄 IN PROGRESS | Blocked by import cycle in integration tests |
| **Phase 9: Integration** | ⏳ PENDING | Wire into PromptAssembler |
| **Phase 10: Production** | ⏳ PENDING | Enable via `USE_JIT_PROMPTS=true` |

### Critical Files

| Component | Location | Purpose |
|-----------|----------|---------|
| **JIT Compiler** | `internal/prompt/compiler.go` | Main compilation orchestrator |
| **Atoms** | `internal/prompt/atoms.go` | Core data model (14 categories) |
| **Context** | `internal/prompt/context.go` | 10-tier contextual dimensions |
| **Selector** | `internal/prompt/selector.go` | Mangle + vector selection |
| **Resolver** | `internal/prompt/resolver.go` | Dependency graph resolution |
| **Budget** | `internal/prompt/budget.go` | Token budget management |
| **Assembler** | `internal/prompt/assembler.go` | Final prompt assembly |
| **Loader** | `internal/prompt/loader.go` | YAML → SQLite ingestion |
| **Integration** | `internal/articulation/prompt_assembler.go` | JIT integration point |

### Storage Architecture (Unified Design)

**Key Decision**: No separate `atoms.db`. Prompts are stored in each agent's existing `_knowledge.db` alongside knowledge atoms.

```text
.nerd/agents/{name}/
├── prompts.yaml              # YAML source (human-editable)
└── {name}_knowledge.db       # Unified SQLite database
    ├── knowledge_atoms       # Domain knowledge (existing)
    ├── knowledge_graph       # Entity relationships (existing)
    └── prompt_atoms          # JIT prompt atoms (NEW TABLE)
```

**Benefits**:

- Single source of truth per agent
- Atomic backups (one DB file)
- Simpler lifecycle management
- Consistent schema evolution

### 10-Tier Contextual Dimensions

The JIT compiler selects atoms based on 10 independent context dimensions:

| Tier | Dimension | Example Values | Purpose |
|------|-----------|----------------|---------|
| **1** | **Operational Mode** | `/active`, `/dream`, `/debugging`, `/tdd_repair`, `/creative`, `/scaffolding`, `/shadow` | High-level agent mode |
| **2** | **Campaign Phase** | `/planning`, `/decomposing`, `/validating`, `/active`, `/completed`, `/paused`, `/failed` | Multi-phase goal orchestration |
| **3** | **Build Taxonomy** | `/scaffold`, `/domain_core`, `/data_layer`, `/service`, `/transport`, `/integration` | Architectural layer |
| **4** | **Init Phase** | `/migration`, `/setup`, `/scanning`, `/analysis`, `/profile`, `/facts`, `/agents`, `/kb_agent`, `/kb_complete` | Project initialization stage |
| **5** | **Northstar Phase** | `/doc_ingestion`, `/problem`, `/vision`, `/requirements`, `/architecture`, `/roadmap`, `/validation` | Vision definition stage |
| **6** | **Ouroboros Stage** | `/detection`, `/specification`, `/safety_check`, `/simulation`, `/codegen`, `/testing`, `/deployment` | Self-improvement stage |
| **7** | **Intent Verb** | `/fix`, `/debug`, `/refactor`, `/test`, `/review`, `/create`, `/research`, `/explain` | Action type |
| **8** | **Shard Type** | `/coder`, `/tester`, `/reviewer`, `/researcher`, `/librarian`, `/planner`, `/custom` | Agent archetype |
| **9** | **World Model State** | `failing_tests`, `diagnostics`, `large_refactor`, `security_issues`, `new_files`, `high_churn` | Codebase state |
| **10** | **Language & Framework** | `/go`, `/python`, `/typescript` + `/bubbletea`, `/gin`, `/react`, `/rod` | Technology stack |

**Matching Logic**: Each `PromptAtom` has selector arrays for each tier. An atom matches if it satisfies ALL non-empty dimensions. Empty selectors mean "match any".

### 14 Atom Categories

Prompts are decomposed into 14 semantic categories:

| Category | Code | Purpose | Token Budget % |
|----------|------|---------|----------------|
| **identity** | `CategoryIdentity` | Who the agent is, core capabilities | 10% |
| **protocol** | `CategoryProtocol` | Piggyback, OODA, TDD protocols | 8% |
| **safety** | `CategorySafety` | Constitutional constraints | 12% |
| **methodology** | `CategoryMethodology` | How to approach problems | 15% |
| **hallucination** | `CategoryHallucination` | Anti-hallucination guardrails | 5% |
| **language** | `CategoryLanguage` | Language-specific guidance | 8% |
| **framework** | `CategoryFramework` | Framework-specific patterns | 8% |
| **domain** | `CategoryDomain` | Project-specific knowledge | 12% |
| **campaign** | `CategoryCampaign` | Campaign/goal context | 5% |
| **init** | `CategoryInit` | Initialization guidance | 3% |
| **northstar** | `CategoryNorthstar` | Planning/vision guidance | 3% |
| **ouroboros** | `CategoryOuroboros` | Self-improvement guidance | 3% |
| **context** | `CategoryContext` | Dynamic runtime context | 6% |
| **exemplar** | `CategoryExemplar` | Few-shot examples | 2% |

**Budget Allocation**: Percentages are soft targets. Mandatory atoms always fit first, then optional atoms fill remaining budget by priority.

### 10-Step Compilation Flow

```text
1. LLM Call Requested
   │   Trigger: ShardManager.Execute() or PromptAssembler.AssembleSystemPrompt()
   │
   ↓
2. Build CompilationContext
   │   Extract: operational_mode, campaign_phase, shard_type, language, intent_verb, etc.
   │   Sources: SessionContext, UserIntent, kernel state
   │
   ↓
3. Assert compile_context facts to kernel
   │   Format: current_mode(/active), current_shard_type(/coder), ...
   │   Purpose: Enable Mangle rule-based selection
   │
   ↓
4. Load candidate atoms from 3 tiers
   │   - Tier 1: Embedded corpus (go:embed, baked-in)
   │   - Tier 2: Project DB (.nerd/prompts/project_atoms.db) [FUTURE]
   │   - Tier 3: Agent DB (.nerd/agents/{name}/{name}_knowledge.db)
   │
   ↓
5. Vector search for semantically similar atoms (optional)
   │   Query: SemanticQuery (e.g., intent target: "authentication bug")
   │   Engine: Gemini embeddings + cosine similarity
   │   TopK: 20 candidates
   │
   ↓
6. Kernel derives atom_selected via Mangle rules
   │   Rules: Context matching, phase gating, tool affinity
   │   Output: ScoredAtom[] with logic scores + vector scores
   │   Combine: 70% logic, 30% vector (configurable)
   │
   ↓
7. Resolve dependencies, check conflicts
   │   DependsOn: Auto-include required atoms (e.g., SQL safety with SQL tools)
   │   ConflictsWith: Remove mutually exclusive atoms
   │   IsExclusive: Pick highest-priority atom per group
   │
   ↓
8. Fit within token budget
   │   Budget: TokenBudget - ReservedTokens (default: 100k - 8k = 92k)
   │   Priority: Mandatory first, then by priority score
   │   Strategy: Greedy knapsack with category quotas
   │
   ↓
9. Assemble final prompt
   │   Order: Categories in fixed order (identity → protocol → ... → exemplar)
   │   Format: Markdown sections with clear delimiters
   │   Inject: Dynamic context (session state, files, diagnostics)
   │
   ↓
10. Return to caller
    │   Result: CompilationResult{Prompt, IncludedAtoms, TokenCount, BudgetUsed}
    │   Fallback: If JIT fails, legacy assembler provides baseline prompt
```

### YAML Atom Format

```yaml
# .nerd/agents/go-expert/prompts.yaml

- id: go-error-handling-v2
  version: 2
  category: language
  subcategory: error-patterns
  content: |
    ## Go Error Handling Best Practices

    You MUST follow these patterns:
    - Always handle errors explicitly, never ignore with `_`
    - Return errors, don't log and continue
    - Wrap errors with context: `fmt.Errorf("context: %w", err)`

  # Selectors (empty = match all)
  languages:
    - /go

  shard_types:
    - /coder
    - /reviewer

  world_states:
    - diagnostics  # Include when there are active errors

  # Composition
  priority: 80
  is_mandatory: false
  depends_on: []
  conflicts_with: []

- id: piggyback-protocol-v3
  version: 3
  category: protocol
  content: |
    ## CRITICAL: Piggyback Protocol

    Output JSON with structure:
    {
      "control_packet": { ... },
      "surface_response": "..."
    }

    THOUGHT-FIRST: control_packet BEFORE surface_response.

  # Match ALL shards, ALL modes (universal protocol)
  priority: 100
  is_mandatory: true
```

### Avoiding the "DSL Trap"

**Critical Lesson**: Don't store prompts as Mangle facts expecting fuzzy retrieval. Mangle is a strict compiler, not a search engine.

```mangle
# ❌ WRONG - Exact match only, no semantic search
prompt_template("security", "For security audits...").
prompt_template("auth", "For authentication...").

# User says "vulnerability scan" → NO MATCH!
```

**Correct Architecture**:

1. Store atoms in **SQLite** (with optional vector embeddings)
2. Use **vector search** for semantic retrieval
3. Let **Mangle** reason over retrieved candidates (dependencies, conflicts, priorities)

### Mangle Compilation Rules (Excerpt)

```mangle
# internal/mangle/prompt_compiler.gl

# Atom matches context if all selectors satisfied
atom_matches_context(AtomID) :-
    prompt_atom(AtomID, Category, _, _, /true),  # Mandatory
    current_shard_type(ShardType),
    atom_selector(AtomID, /shard_type, ShardType).

# Auto-inject safety atoms when SQL tools present
atom_selected(SafetyID) :-
    atom_selected(ToolID),
    prompt_atom(ToolID, /tool, Content, _, _),
    fn:contains(Content, "sql"),
    prompt_atom(SafetyID, /safety, SafeContent, _, _),
    fn:contains(SafeContent, "DROP").

# Conflict resolution
atom_conflict(ID1, ID2) :-
    atom_selected(ID1),
    atom_selected(ID2),
    atom_conflicts(ID1, ID2).
```

### Benefits Over Monolithic Prompts

| Aspect | Monolithic (20K chars) | JIT Compiled |
|--------|------------------------|--------------|
| **Maintenance** | Edit entire 20K file | Edit single 200-char atom |
| **Reuse** | Copy-paste between shards | Automatic composition |
| **Context-Awareness** | Manual `if` branches | Phase-gated selection |
| **Safety** | Hope you remembered | Auto-injected with tools |
| **Token Efficiency** | Always full 20K | Only relevant atoms (8K-18K) |
| **Learning** | Requires code change | Add YAML atom, reload |
| **Versioning** | Git diff nightmare | Atom-level version tracking |
| **Testing** | Test 20K monolith | Test individual atoms |

### Integration with God Tier Prompts

The JIT compiler **implements** God Tier prompts dynamically:

**Before JIT**:

- One 20,000-character static Go constant
- Same prompt for all contexts
- Hard to maintain, test, or evolve

**After JIT**:

- 40-50 atomic prompts (200-500 chars each)
- Composition rules in Mangle
- Dynamic assembly based on 10 context dimensions
- **Final prompt still 15,000-20,000 chars** (God Tier standard)

The assembled prompt is:

- ✅ Contextually relevant (only matching atoms)
- ✅ Phase-appropriate (different atoms for /planning vs /coding)
- ✅ Safety-guaranteed (auto-injected safety atoms)
- ✅ Dynamically updatable (edit YAML, no code changes)
- ✅ Token-optimized (fits budget constraints)

### Feature Flag

JIT compilation is opt-in via environment variable:

```bash
# Enable JIT prompt compilation
export USE_JIT_PROMPTS=true

# Fallback behavior if disabled (default)
# Uses legacy PromptAssembler with static templates
```

**Rollout Strategy**:

1. Phase 8-9: Test JIT with synthetic scenarios
2. Phase 10: Shadow mode (run JIT + legacy, compare outputs)
3. Phase 11: Enable by default for new agents
4. Phase 12: Migrate existing agents incrementally

## Integration Points

This skill interacts with:

- **mangle-programming**: For defining the logic predicates used in dynamic injection.
- **codenerd-builder**: For the overall system architecture concepts (including semantic classification).
- **go-architect**: For structural Go constraints within prompt templates.
- **research-builder**: For knowledge hydration strategies and Context7 protocol.
- **rod-builder**: For RodExpert specialist creation and CDP-specific prompting.

## Version History

### v3.0 (December 2024)

- **JIT Prompt Compiler**: Complete architecture documentation for the JIT prompt compilation system
- **10-Tier Context Model**: Documented the 10 contextual dimensions for atom selection
- **14 Atom Categories**: Full taxonomy with token budget allocation percentages
- **10-Step Compilation Flow**: Detailed pipeline from LLM call to assembled prompt
- **Storage Architecture**: Unified design with prompts in agent knowledge databases
- **YAML Atom Format**: Complete specification for human-editable prompt atoms
- **Implementation Status**: Phase 1-7 complete, Phase 8 in progress
- **Critical Files Map**: Complete reference to all JIT compiler components

### v2.0 (December 2024)

- **God Tier Standard**: Formalized 15,000-20,000 character minimum for shard prompts
- **Enhanced References**: specialist-prompts.md expanded to 2,600+ lines with complete hydration strategies
- **Specialist Creation**: Added Type B/U creation workflows and Viva Voce examination patterns
- **Audit Tooling**: Comprehensive audit_prompts.py documentation with CI/CD integration
- **Anti-Patterns**: Expanded common issues table with 10+ symptoms and fixes
- **Quick-Start Patterns**: Added Pattern #5 for specialist knowledge injection

### v1.0 (November 2024)

- Initial release
- Basic Piggyback Protocol documentation
- Core concepts and structure
- Four fundamental quick-start patterns
- Reference document framework
