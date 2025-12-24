# JIT Prompt Compiler Architecture

## System 2 Architecture for Prompt Engineering

The JIT Prompt Compiler is a **System 2 Architecture** applied to Prompt Engineering. It moves from "Prompt String Concatenation" to a **JIT Linking Loader** with a 50ms latency budget and high reliability guarantees.

**Core Philosophy**: Prompts are not static text but **logic-derived assemblies** where the kernel selects the most relevant instruction atoms from a knowledge base based on current context dimensions.

## The Skeleton vs. Flesh Split

The biggest risk in dynamic prompt assembly is the **"Frankenstein Prompt"** anti-pattern—where 20 individually valid atoms assemble into an incoherent mess. The architecture prevents this by bifurcating selection into two distinct layers:

| Layer | Source | Selection | Failure Mode |
|-------|--------|-----------|--------------|
| **Skeleton** (Deterministic) | Identity, Protocol, Safety, Methodology | Pure Mangle Logic | **Panic** - Cannot run without identity |
| **Flesh** (Probabilistic) | Exemplars, Domain Knowledge, Previous Fixes | Vector Search + Mangle Filtering | Graceful degradation - prompt less helpful but safe |

**Key Insight**: Safety constraints and Identity must be deterministic; Context can be fuzzy. If Skeleton atoms fail to load, the compiler **panics** (fail safe). If Flesh atoms fail, the prompt is simply less helpful but still safe and functional.

## Architectural Hardening Features

| Feature | Purpose | Implementation |
|---------|---------|----------------|
| **Normalized Context Tags** | Zero JSON parsing overhead | `atom_context_tags` link table instead of JSON columns |
| **Description-Based Embedding** | Semantic retrieval via intent | Embed `description` field, not `content` |
| **Atom Polymorphism** | Graceful token pressure handling | `content`, `content_concise`, `content_min` levels |
| **Prompt Manifest** | Ouroboros debugging | Flight recorder logs atom selection rationale |
| **Context Hashing** | Latency optimization | Cache Skeleton for repeated context hashes |

## Implementation Status

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

## Critical Files

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

## Storage Architecture (Unified Design)

**Key Decision**: No separate `atoms.db`. Prompts are stored in each agent's existing `_knowledge.db` alongside knowledge atoms.

```text
.nerd/agents/{name}/
├── prompts.yaml              # YAML source (human-editable)
└── {name}_knowledge.db       # Unified SQLite database
    ├── knowledge_atoms       # Domain knowledge (existing)
    ├── knowledge_graph       # Entity relationships (existing)
    └── prompt_atoms          # JIT prompt atoms (NEW TABLE)
```

## 10-Tier Contextual Dimensions

The JIT compiler selects atoms based on 10 independent context dimensions:

| Tier | Dimension | Example Values |
|------|-----------|----------------|
| **1** | **Operational Mode** | `/active`, `/dream`, `/debugging`, `/tdd_repair`, `/creative`, `/scaffolding`, `/shadow` |
| **2** | **Campaign Phase** | `/planning`, `/decomposing`, `/validating`, `/active`, `/completed`, `/paused`, `/failed` |
| **3** | **Build Taxonomy** | `/scaffold`, `/domain_core`, `/data_layer`, `/service`, `/transport`, `/integration` |
| **4** | **Init Phase** | `/migration`, `/setup`, `/scanning`, `/analysis`, `/profile`, `/facts`, `/agents`, `/kb_agent`, `/kb_complete` |
| **5** | **Northstar Phase** | `/doc_ingestion`, `/problem`, `/vision`, `/requirements`, `/architecture`, `/roadmap`, `/validation` |
| **6** | **Ouroboros Stage** | `/detection`, `/specification`, `/safety_check`, `/simulation`, `/codegen`, `/testing`, `/deployment` |
| **7** | **Intent Verb** | `/fix`, `/debug`, `/refactor`, `/test`, `/review`, `/create`, `/research`, `/explain` |
| **8** | **Shard Type** | `/coder`, `/tester`, `/reviewer`, `/researcher`, `/librarian`, `/planner`, `/custom` |
| **9** | **World Model State** | `failing_tests`, `diagnostics`, `large_refactor`, `security_issues`, `new_files`, `high_churn` |
| **10** | **Language & Framework** | `/go`, `/python`, `/typescript` + `/bubbletea`, `/gin`, `/react`, `/rod` |

**Matching Logic**: Each `PromptAtom` has selector arrays for each tier. An atom matches if it satisfies ALL non-empty dimensions. Empty selectors mean "match any".

## 14 Atom Categories

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

## 10-Step Compilation Flow

```text
1. LLM Call Requested
   │   Trigger: ShardManager.Execute() or PromptAssembler.AssembleSystemPrompt()
   ↓
2. Build CompilationContext
   │   Extract: operational_mode, campaign_phase, shard_type, language, intent_verb, etc.
   ↓
3. Assert compile_context facts to kernel
   │   Format: current_mode(/active), current_shard_type(/coder), ...
   ↓
4. Load candidate atoms from 3 tiers
   │   - Tier 1: Embedded corpus (go:embed, baked-in)
   │   - Tier 2: Project DB (.nerd/prompts/project_atoms.db)
   │   - Tier 3: Agent DB (.nerd/agents/{name}/{name}_knowledge.db)
   ↓
5. Vector search for semantically similar atoms (optional)
   │   Query: SemanticQuery (e.g., intent target: "authentication bug")
   ↓
6. Kernel derives atom_selected via Mangle rules
   │   Combine: 70% logic, 30% vector (configurable)
   ↓
7. Resolve dependencies, check conflicts
   │   DependsOn: Auto-include required atoms
   │   ConflictsWith: Remove mutually exclusive atoms
   ↓
8. Fit within token budget
   │   Priority: Mandatory first, then by priority score
   ↓
9. Assemble final prompt
   │   Order: Categories in fixed order (identity → protocol → ... → exemplar)
   ↓
10. Return to caller
    │   Result: CompilationResult{Prompt, IncludedAtoms, TokenCount, BudgetUsed}
```

## YAML Atom Format

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

  languages:
    - /go
  shard_types:
    - /coder
    - /reviewer
  world_states:
    - diagnostics

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

  priority: 100
  is_mandatory: true
```

## Mangle Compilation Rules (Excerpt)

```mangle
# Atom matches context if all selectors satisfied
atom_matches_context(AtomID) :-
    prompt_atom(AtomID, Category, _, _, /true),
    current_shard_type(ShardType),
    atom_selector(AtomID, /shard_type, ShardType).

# Auto-inject safety atoms when SQL tools present
atom_selected(SafetyID) :-
    atom_selected(ToolID),
    prompt_atom(ToolID, /tool, Content, _, _),
    fn:contains(Content, "sql"),
    prompt_atom(SafetyID, /safety, SafeContent, _, _),
    fn:contains(SafeContent, "DROP").
```

## Benefits Over Monolithic Prompts

| Aspect | Monolithic (20K chars) | JIT Compiled |
|--------|------------------------|--------------|
| **Maintenance** | Edit entire 20K file | Edit single 200-char atom |
| **Reuse** | Copy-paste between shards | Automatic composition |
| **Context-Awareness** | Manual `if` branches | Phase-gated selection |
| **Safety** | Hope you remembered | Auto-injected with tools |
| **Token Efficiency** | Always full 20K | Only relevant atoms (8K-18K) |
| **Learning** | Requires code change | Add YAML atom, reload |
| **Versioning** | Git diff nightmare | Atom-level version tracking |

## Feature Flag

```bash
# Enable JIT prompt compilation
export USE_JIT_PROMPTS=true
```

**Rollout Strategy**:
1. Phase 8-9: Test JIT with synthetic scenarios
2. Phase 10: Shadow mode (run JIT + legacy, compare outputs)
3. Phase 11: Enable by default for new agents
4. Phase 12: Migrate existing agents incrementally
