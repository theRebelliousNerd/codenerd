# codeNERD Systems Integration Map

**Document Version:** 1.0
**Last Updated:** 2025-12-08
**Architecture Version:** Cortex 1.5.0

---

## Executive Summary

This document provides a comprehensive map of ALL 42+ systems in codeNERD that require integration wiring. It serves as the authoritative reference for understanding:

- **Boot Sequence** - The exact order systems must be initialized
- **Dependency Injection** - What each system needs wired TO it
- **Component Relationships** - What each system injects INTO others
- **Integration Points** - Where systems connect and communicate
- **Logging Categories** - Debug tracing for each system

This map is critical for:
- Onboarding new developers
- Debugging initialization failures
- Refactoring system boundaries
- Auditing integration completeness

---

## Table of Contents

1. [Boot Sequence Overview](#boot-sequence-overview)
2. [Request Processing Flow](#request-processing-flow)
3. [System Categories](#system-categories)
4. [System Inventory (40+ Systems)](#system-inventory)
5. [Integration Diagrams](#integration-diagrams)
6. [Common Integration Patterns](#common-integration-patterns)
7. [Troubleshooting Guide](#troubleshooting-guide)

---

## Boot Sequence Overview

codeNERD follows a strict 13-step boot sequence to ensure all dependencies are satisfied before systems become active:

```
1. Logging System           ← Must be FIRST (all systems depend on it)
2. Configuration Loader      ← Reads .nerd/config.json
3. LLM Client Provider       ← Auto-detects provider (zai/anthropic/openai/gemini/xai)
4. JIT Prompt Compiler       ← Embedded corpus + prompt assembly
5. Usage Tracker            ← Lightweight initialization
6. LocalStore/SQLite        ← Persistent storage layer
7. Embedding Engine         ← Vector embeddings (Ollama/GenAI)
8. Mangle Kernel            ← Logic engine initialization
9. Perception Transducer    ← NL → Intent transduction
10. World Scanner            ← Filesystem/AST projection
11. VirtualStore            ← FFI to external systems
12. Shard Manager           ← Shard lifecycle + JIT callbacks
13. Autopoiesis Orchestrator ← Self-modification
14. System Shards           ← Always-on services

↓ System Ready ↓
```

**Critical Boot Rule:** No system may be initialized until ALL its dependencies (systems above it) are initialized.

---

## Request Processing Flow

Every user request flows through this pipeline:

```
┌─────────────┐
│ User Input  │
└─────┬───────┘
      │
      ↓
┌────────────────────────┐  Step 1: PERCEPTION
│ Perception Transducer  │  - Parse NL → structured Intent
│ (Piggyback Protocol)   │  - Extract category/verb/target
└──────────┬─────────────┘
           │
           ↓
┌────────────────────────┐  Step 2: ORIENT (OODA Loop)
│ Spreading Activation   │  - Load relevant facts from LocalStore
│ + Context Compression  │  - Activate related knowledge atoms
└──────────┬─────────────┘
           │
           ↓
┌────────────────────────┐  Step 3: DECIDE
│ Mangle Kernel          │  - Assert user_intent fact
│ Policy Derivation      │  - Derive next_action via rules
└──────────┬─────────────┘
           │
           ├──────────────────────┐
           ↓                      ↓
┌──────────────────┐    ┌──────────────────┐
│ Direct Action    │    │ Shard Delegation │
│ (VirtualStore)   │    │ (ShardManager)   │
└────────┬─────────┘    └────────┬─────────┘
         │                       │
         └───────────┬───────────┘
                     ↓
         ┌────────────────────────┐  Step 4: ACT
         │ Tactical Execution     │  - Execute via Tactile Layer
         │ (Safe Executor)        │  - Constitutional safety check
         └──────────┬─────────────┘
                    │
                    ↓
         ┌────────────────────────┐  Step 5: ARTICULATE
         │ Articulation Emitter   │  - Format response for user
         │ (Piggyback Protocol)   │  - Inject control packet
         └──────────┬─────────────┘
                    │
                    ↓
         ┌────────────────────────┐  Step 6: COMPRESS
         │ Context Compressor     │  - Compress history
         │ + Memory Tiering       │  - Move to appropriate tier
         └────────────────────────┘
```

---

## System Categories

Systems are organized into 13 functional categories:

| # | Category | System Count | Purpose |
|---|----------|--------------|---------|
| 1 | **Boot Sequence** | 3 | Foundation systems (logging, config, LLM) |
| 2 | **Kernel & Logic** | 7 | Mangle engine, virtual store, shard lifecycle |
| 3 | **Perception & Transduction** | 3 | NL parsing, taxonomy, tracing |
| 4 | **Articulation & Output** | 4 | Response generation, Piggyback Protocol, JIT prompts |
| 5 | **Storage & Persistence** | 5 | SQLite, vector search, embeddings |
| 6 | **Context & Compression** | 4 | Token mgmt, activation, serialization |
| 7 | **Campaign Orchestration** | 5 | Multi-phase goals, checkpointing, replanning |
| 8 | **Autopoiesis & Self-Mod** | 7 | Tool generation, Ouroboros, feedback |
| 9 | **Shard Implementations** | 5 | Coder, Tester, Reviewer, Researcher, System |
| 10 | **World Model** | 3 | File scanning, AST projection, symbol graph |
| 11 | **Tactical Execution** | 2 | Safe executor, tool registry |
| 12 | **TUI & Chat Interface** | 4 | Bubble Tea model, session, commands, view |
| 13 | **Initialization** | 3 | Project setup, profile generation |

**Total Systems:** 55 documented integration points

---

## System Inventory

### 1. BOOT SEQUENCE SYSTEMS (3 systems)

#### 1.1 Logging System
- **File:** `internal/logging/logger.go`
- **Boot Step:** #1 (MUST BE FIRST)
- **Logging Category:** `boot`
- **Dependencies:** None (foundation system)
- **Dependents:** ALL systems (every system uses logging)
- **Key Wiring:**
  - `Initialize(workspace)` - Sets up log directory
  - `Get(category)` - Returns logger for category
  - Config-driven enable/disable via `.nerd/config.json`
- **Categories Defined:**
  - `boot`, `session`, `kernel`, `api`
  - `perception`, `articulation`, `routing`, `tools`, `virtual_store`
  - `shards`, `coder`, `tester`, `reviewer`, `researcher`, `system_shards`
  - `dream`, `autopoiesis`, `campaign`, `context`, `world`, `embedding`, `store`

#### 1.2 Configuration System
- **File:** `internal/config/config.go`
- **Boot Step:** #2
- **Logging Category:** `boot`
- **Dependencies:**
  - `logging` (for error reporting)
- **Dependents:**
  - `perception.Client` (API keys, provider selection)
  - `embedding.Engine` (provider config)
  - `mangle.Kernel` (fact limits, query timeout)
  - `store.LocalStore` (database path)
  - `autopoiesis.Orchestrator` (tool generation settings)
  - All shards (per-shard profiles)
- **Key Wiring:**
  - `Load(path)` - Loads from `.nerd/config.json`
  - `GlobalConfig()` - Returns singleton config
  - `GetShardProfile(type)` - Per-shard settings
  - `GetActiveProvider()` - LLM provider detection
- **Configuration Hierarchy:**
  - Defaults → YAML → Environment Variables → Runtime Overrides

#### 1.3 LLM Client Provider
- **File:** `internal/perception/client.go`
- **Boot Step:** #3
- **Logging Category:** `api`
- **Dependencies:**
  - `config` (API keys, model selection)
  - `logging` (API call tracing)
- **Dependents:**
  - `perception.Transducer` (intent parsing)
  - `coder.Shard` (code generation)
  - `reviewer.Shard` (code review)
  - `tester.Shard` (test generation)
  - `researcher.Shard` (knowledge gathering)
  - `autopoiesis.Orchestrator` (tool generation)
  - `campaign.Decomposer` (goal decomposition)
- **Key Wiring:**
  - `NewClientFromEnv()` - Auto-detect provider
  - `DetectProvider()` - Read config/env for provider
  - Supported: Z.AI, Anthropic, OpenAI, Gemini, xAI, OpenRouter
- **Provider Detection Order:**
  1. Explicit `provider` in config
  2. First available API key (anthropic → openai → gemini → xai → zai → openrouter)
  3. Legacy `api_key` field (defaults to zai)

---

### 2. KERNEL & LOGIC SYSTEMS (7 systems)

#### 2.1 Mangle Kernel
- **File:** `internal/mangle/engine.go`
- **Boot Step:** #7
- **Logging Category:** `kernel`
- **Dependencies:**
  - `config` (fact limits, query timeout)
  - `logging`
  - `store.LocalStore` (optional persistence)
- **Dependents:**
  - `core.VirtualStore` (action routing)
  - `core.ShardManager` (shard delegation queries)
  - `core.ShadowMode` (simulation)
  - `autopoiesis.Orchestrator` (tool generation triggers)
  - `campaign.Orchestrator` (campaign planning)
  - All shards (fact assertion/query)
- **Key Wiring:**
  - `NewEngine(cfg, persistence)` - Create kernel
  - `LoadFacts(facts)` - Load EDB facts
  - `Query(predicate)` - Query IDB derivations
  - `Evaluate()` - Run inference to fixpoint
  - `Assert(fact)` - Add runtime fact
- **Schema Locations:**
  - Embedded defaults: `internal/mangle/schemas.gl`
  - Project overrides: `.nerd/mangle/schemas.mg`
- **Policy Locations:**
  - Embedded defaults: `internal/mangle/policy.gl`
  - Project overrides: `.nerd/mangle/policy.mg`

#### 2.2 DifferentialEngine
- **File:** `internal/mangle/differential.go`
- **Boot Step:** Integrated with Kernel (#7)
- **Logging Category:** `kernel`
- **Dependencies:**
  - `mangle.Kernel`
- **Dependents:**
  - `core.VirtualStore` (incremental updates)
- **Key Wiring:**
  - Tracks fact deltas between kernel evaluations
  - Enables incremental reasoning without full re-evaluation

#### 2.3 Virtual Store
- **File:** `internal/core/virtual_store.go`
- **Boot Step:** #10
- **Logging Category:** `virtual_store`
- **Dependencies:**
  - `mangle.Kernel` (action routing)
  - `tactile.Executor` (command execution)
  - `store.LocalStore` (optional persistence)
  - `store.LearningStore` (optional learning)
  - `autopoiesis.Orchestrator` (optional tool generation)
- **Dependents:**
  - `campaign.Orchestrator` (action execution)
  - TUI chat model (user actions)
- **Key Wiring:**
  - `RouteAction(ctx, fact)` - Route kernel-derived actions
  - `SetLocalDB(db)` - Inject persistence
  - `SetKernel(kernel)` - Inject kernel
  - `SetLearningStore(store)` - Inject learning
  - `SetToolGenerator(gen)` - Inject Ouroboros
- **Action Routing Table:**
  - `/query` → Read-only operations
  - `/mutate` → File modifications
  - `/execute` → Command execution
  - `/research` → Knowledge gathering
  - `/delegate` → Shard spawning
  - `/generate_tool` → Ouroboros tool creation

#### 2.4 Shard Manager
- **File:** `internal/core/shard_manager.go`
- **Boot Step:** #11
- **Logging Category:** `shards`
- **Dependencies:**
  - `mangle.Kernel` (parent kernel for delegation queries)
  - `perception.Client` (LLM access for shards)
  - `config` (shard profiles, resource limits)
- **Dependents:**
  - `campaign.Orchestrator` (shard execution)
  - TUI chat model (manual spawning)
  - `system.Factory` (system shard registration)
- **Key Wiring:**
  - `SetParentKernel(kernel)` - Inject kernel
  - `SetLLMClient(client)` - Inject LLM
  - `RegisterShard(name, factory)` - Register factory function
  - `Spawn(ctx, type, task)` - Spawn ephemeral shard
  - `StartSystemShards(ctx)` - Boot Type S shards
  - `DisableSystemShard(name)` - Disable specific system shard
- **Shard Types:**
  - **Type A (Ephemeral):** `coder`, `tester`, `reviewer`, `researcher`
  - **Type B (Persistent):** User-defined specialists
  - **Type U (User):** Alias for Type B
  - **Type S (System):** `legislator`, `tactile_router`, `world_model_ingestor`

#### 2.5 Dreamer / Precog Safety
- **File:** `internal/core/dreamer.go`
- **Boot Step:** #12 (part of Autopoiesis)
- **Logging Category:** `dream`
- **Dependencies:**
  - `mangle.Kernel` (simulation kernel)
  - `core.ShadowMode`
- **Dependents:**
  - `core.VirtualStore` (safety checks before execution)
- **Key Wiring:**
  - `Simulate(action)` - Run what-if simulation
  - `CheckSafety(action)` - Constitutional safety validation
- **Safety Checks:**
  - File impact analysis (`impacted/1`)
  - Test coverage validation
  - Chesterton's Fence (git history checks)
  - Constitutional rule compliance

#### 2.6 Shadow Mode
- **File:** `internal/core/shadow_mode.go`
- **Boot Step:** Lazy (on-demand)
- **Logging Category:** `dream`
- **Dependencies:**
  - `mangle.Kernel` (forked kernel for simulation)
- **Dependents:**
  - `core.Dreamer`
  - TUI `/shadow` command
- **Key Wiring:**
  - `NewShadowMode(kernel)` - Create simulation
  - `Simulate(action, target)` - Run counterfactual
- **Use Cases:**
  - What-if analysis
  - Risk assessment
  - Safety validation

#### 2.7 TDD Loop
- **File:** `internal/core/tdd_loop.go`
- **Boot Step:** Lazy (triggered by test failures)
- **Logging Category:** `tester`
- **Dependencies:**
  - `coder.Shard` (code generation)
  - `tester.Shard` (test execution)
  - `tactile.Executor` (test running)
- **Dependents:**
  - `coder.Shard` (auto-fix on test failure)
- **Key Wiring:**
  - **Red:** Detect test failure
  - **Green:** Generate fix
  - **Refactor:** Optimize code
  - **Retest:** Validate fix

---

### 3. PERCEPTION & TRANSDUCTION (3 systems)

#### 3.1 Perception Transducer
- **File:** `internal/perception/transducer.go`
- **Boot Step:** #8
- **Logging Category:** `perception`
- **Dependencies:**
  - `perception.Client` (LLM for intent parsing)
  - `perception.Taxonomy` (verb corpus)
- **Dependents:**
  - TUI chat model (user input parsing)
  - `campaign.Decomposer` (goal parsing)
- **Key Wiring:**
  - `NewRealTransducer(client)` - Create transducer
  - `ParseIntent(ctx, input)` - NL → structured Intent
  - `ExtractPiggyback(response)` - Extract control packet
- **Piggyback Protocol:**
  ```
  Surface Response (for user)
  ───────────────────────────────
  CONTROL_PACKET_START
  {
    "intent_classification": {...},
    "mangle_updates": [...],
    "memory_operations": [...]
  }
  CONTROL_PACKET_END
  ```
- **Intent Structure:**
  - `Category` - `/query`, `/mutation`, `/instruction`
  - `Verb` - `/review`, `/fix`, `/test`, etc.
  - `Target` - File path, symbol, or "codebase"
  - `Constraint` - Additional filters/requirements
  - `Confidence` - 0.0-1.0

#### 3.2 Taxonomy Engine
- **File:** `internal/perception/taxonomy.go`
- **Boot Step:** #8 (during Transducer init)
- **Logging Category:** `perception`
- **Dependencies:**
  - `mangle.Kernel` (stores verb mappings as facts)
  - `perception.TaxonomyPersistence` (save/load)
- **Dependents:**
  - `perception.Transducer` (VerbCorpus loading)
- **Key Wiring:**
  - `GetVerbs()` - Load verb taxonomy
  - `SaveTaxonomy()` - Persist to disk
- **Verb Corpus:**
  - 40+ verbs with synonyms, patterns, priorities
  - Mappings: `/review` ← "review", "check", "audit", "inspect"
  - Category inference: `/mutation` vs `/query`
  - Shard routing: `reviewer`, `coder`, `tester`, `researcher`

#### 3.3 Tracing Client
- **File:** `internal/perception/tracing_client.go`
- **Boot Step:** #3 (wraps LLM Client)
- **Logging Category:** `api`
- **Dependencies:**
  - `perception.Client` (wraps)
  - `store.TraceStore` (persistence)
- **Dependents:**
  - All shards (when tracing enabled)
- **Key Wiring:**
  - `NewTracingLLMClient(client, store)` - Wrap client
  - Auto-logs all LLM calls with:
    - System prompt
    - User prompt
    - Raw response
    - Duration
    - Token counts

---

### 4. ARTICULATION & OUTPUT (2 systems)

#### 4.1 Articulation Emitter
- **File:** `internal/articulation/emitter.go`
- **Boot Step:** On-demand
- **Logging Category:** `articulation`
- **Dependencies:**
  - None (stateless utility)
- **Dependents:**
  - All shards (response formatting)
  - TUI chat model (message display)
- **Key Wiring:**
  - `NewEmitter()` - Create emitter
  - `Emit(envelope)` - Format Piggyback envelope
  - `FormatMarkdown(text)` - Render markdown for terminal
- **Piggyback Envelope:**
  - `Surface` - Human-readable response
  - `Control.IntentClassification` - Parsed intent
  - `Control.MangleUpdates` - Facts to assert
  - `Control.MemoryOperations` - Storage directives
  - `Control.SelfCorrection` - Learning feedback

#### 4.2 Piggyback Protocol Handler
- **File:** Integrated across `perception` and `articulation`
- **Boot Step:** N/A (protocol, not system)
- **Logging Category:** `perception`, `articulation`
- **Dependencies:**
  - `perception.Transducer` (extraction)
  - `articulation.Emitter` (formatting)
- **Dependents:**
  - All LLM-using systems
- **Key Wiring:**
  - Dual-channel communication:
    - **Surface:** User-facing NL response
    - **Control:** Machine-readable instructions
  - JSON extraction via delimiters
  - Fallback to pipe-delimited parsing

#### 4.3 JIT Prompt Compiler
- **File:** `internal/prompt/jit_compiler.go`
- **Boot Step:** #3.5 (after LLM Client creation)
- **Logging Category:** `articulation`
- **Dependencies:**
  - `go:embed` corpus (prompt fragments YAML)
  - SQLite (for Type B/U shard prompt tables)
- **Dependents:**
  - `articulation.PromptAssembler` (JIT compilation)
  - `core.ShardManager` (shard DB registration)
- **Key Wiring:**
  - `NewJITPromptCompiler(corpus)` - Create compiler with embedded corpus
  - `SetJITCompiler(compiler)` - Inject into PromptAssembler
  - `RegisterAgentDB(name, db)` - Register Type B/U shard DB
  - `UnregisterAgentDB(name)` - Cleanup on shard completion
  - `CompilePrompt(ctx, template, atoms)` - Runtime compilation
- **Embedded Corpus:**
  - Location: `internal/prompt/corpus/*.yaml`
  - Structure: Static fragments + dynamic atom injection
  - Compilation: YAML → SQLite → Runtime assembly
- **Boot Integration Points:**

  1. Create JITPromptCompiler with embedded corpus
  2. Wire ShardManager callbacks (promptLoader, jitRegistrar, jitUnregistrar)
  3. Attach compiler to PromptAssembler via SetJITCompiler()
  4. Enable JIT compilation (optional via USE_JIT_PROMPTS env var)

#### 4.4 Prompt Loader
- **File:** `internal/prompt/loader.go`
- **Boot Step:** Lazy (during Type B/U shard spawn)
- **Logging Category:** `articulation`
- **Dependencies:**
  - SQLite (shard knowledge base)
  - YAML parser (prompts.yaml)
- **Dependents:**
  - `core.ShardManager` (promptLoader callback)
  - Type B/U shards (persistent agents)
- **Key Wiring:**
  - `CreateJITDBRegistrar(compiler)` - Creates registration callback for ShardManager
  - `CreateJITDBUnregistrar(compiler)` - Creates cleanup callback
  - `RegisterAgentDBWithJIT(compiler, name, dbPath)` - Opens DB & registers with compiler
  - `LoadAgentPrompts(dbPath, yamlPath)` - Loads YAML → SQLite prompt_atoms table
  - `ensurePromptAtomsTable(db)` - Creates table in existing KB if missing
- **Integration Flow:**

  1. ShardManager spawns Type B/U shard
  2. Check for `.nerd/agents/{name}/prompts.yaml`
  3. Call promptLoader → opens KB, ensures table, loads YAML
  4. Call jitRegistrar → registers DB with compiler
  5. Shard executes with JIT-compiled prompts from corpus + KB atoms
  6. On completion, call jitUnregistrar → cleanup

---

### 5. STORAGE & PERSISTENCE (5 systems)

#### 5.1 LocalStore / SQLite
- **File:** `internal/store/local.go`
- **Boot Step:** #5
- **Logging Category:** `store`
- **Dependencies:**
  - `embedding.Engine` (optional, for vector search)
  - `config` (database path)
- **Dependents:**
  - `mangle.Kernel` (optional persistence)
  - `core.VirtualStore` (fact storage)
  - `store.LearningStore` (underlying DB)
  - `store.TraceStore` (underlying DB)
- **Key Wiring:**
  - `NewLocalStore(path)` - Initialize DB
  - `StoreFact(predicate, args, domain, weight)` - Save fact
  - `LoadFacts(predicate)` - Retrieve facts
  - `VectorSearch(embedding, topK)` - Semantic search (requires sqlite-vec)
- **Database Schema:**
  - `cold_storage` - Persistent facts (user preferences, learned patterns)
  - `knowledge_graph` - Entity relationships
  - `vector_store` - Embeddings for semantic search
  - `archival_storage` - Cold facts (access < threshold)
  - `activation_log` - Spreading activation history
- **Maintenance:**
  - Archive facts not accessed in 90 days
  - Purge archived facts older than 365 days
  - Vacuum database to reclaim space

#### 5.2 LearningStore
- **File:** `internal/store/learning.go`
- **Boot Step:** #5 (alongside LocalStore)
- **Logging Category:** `store`
- **Dependencies:**
  - SQLite (underlying)
  - `logging`
- **Dependents:**
  - `core.VirtualStore` (learning injection)
  - `autopoiesis.Orchestrator` (tool feedback)
- **Key Wiring:**
  - `NewLearningStore(path)` - Initialize store
  - `RecordLearning(event)` - Save learning event
  - `GetLearnings(domain)` - Retrieve patterns
- **Learning Events:**
  - Tool generation outcomes
  - Shard execution feedback
  - Constitutional rejections
  - TDD loop iterations

#### 5.3 TraceStore
- **File:** `internal/store/trace_store.go`
- **Boot Step:** #5 (alongside LocalStore)
- **Logging Category:** `store`
- **Dependencies:**
  - SQLite (underlying)
- **Dependents:**
  - `perception.TracingClient` (LLM call logging)
  - `autopoiesis.TraceCollector` (reasoning traces)
- **Key Wiring:**
  - `StoreReasoningTrace(trace)` - Save LLM reasoning
  - `LoadReasoningTrace(id)` - Retrieve trace
- **Trace Data:**
  - System prompt
  - User prompt
  - Raw response
  - Chain of thought
  - Key decisions
  - Quality score (post-execution)

#### 5.4 Embedding Engine
- **File:** `internal/embedding/`
- **Boot Step:** #6
- **Logging Category:** `embedding`
- **Dependencies:**
  - `config` (provider selection: Ollama/GenAI)
- **Dependents:**
  - `store.LocalStore` (vector search)
  - `context.Activation` (semantic activation)
- **Key Wiring:**
  - `NewEmbeddingEngine(cfg)` - Create engine
  - `Embed(text)` - Generate embedding vector
  - Supported: Ollama (local), Google GenAI (cloud)
- **Providers:**
  - **Ollama:** Local, fast, embeddinggemma model
  - **GenAI:** Cloud, high-quality, gemini-embedding-001

#### 5.5 Vector Search (sqlite-vec)
- **File:** `internal/store/init_vec.go`
- **Boot Step:** #5 (during LocalStore init)
- **Logging Category:** `store`
- **Dependencies:**
  - SQLite with vec0 extension
- **Dependents:**
  - `store.LocalStore` (semantic search)
- **Key Wiring:**
  - `detectVecExtension()` - Check for sqlite-vec
  - Requires: `CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build`
- **Build Requirement:**
  - MUST use custom SQLite build with vec0 extension
  - Fails fast if not available (no graceful degradation)

---

### 6. CONTEXT & COMPRESSION (4 systems)

#### 6.1 Context Compressor
- **File:** `internal/context/compressor.go`
- **Boot Step:** On-demand (after each LLM interaction)
- **Logging Category:** `context`
- **Dependencies:**
  - `perception.Client` (LLM for compression)
  - `context.TokenCounter` (budget tracking)
- **Dependents:**
  - TUI chat model (history compression)
  - Shards (context injection)
- **Key Wiring:**
  - `CompressHistory(messages, budget)` - Semantic compression
  - `TargetRatio` - 100:1 compression (configurable)
  - Triggered at 80% of context window
- **Compression Strategy:**
  - Core reserve: 5% (constitutional facts)
  - Atom reserve: 30% (high-activation atoms)
  - History reserve: 15% (compressed history)
  - Working reserve: 50% (recent turns)

#### 6.2 Activation Engine
- **File:** `internal/context/activation.go`
- **Boot Step:** On-demand
- **Logging Category:** `context`
- **Dependencies:**
  - `mangle.Kernel` (fact graph)
  - `store.LocalStore` (activation logs)
  - `embedding.Engine` (semantic similarity)
- **Dependents:**
  - `context.Compressor` (fact selection)
  - Shards (context injection)
- **Key Wiring:**
  - `Activate(seedFacts, threshold)` - Spreading activation
  - Traverses Mangle fact graph
  - Scores facts by relevance (0-100)
  - Selects top-K facts above threshold (default: 30.0)
- **Activation Sources:**
  - User intent facts (seed)
  - Impacted files (`impacted/1`)
  - Recent modifications (`modified/1`)
  - Symbol graph (`symbol_graph/4`)
  - Diagnostic facts (`diagnostic/5`)

#### 6.3 Token Management
- **File:** `internal/context/tokens.go`
- **Boot Step:** On-demand
- **Logging Category:** `context`
- **Dependencies:**
  - None (stateless utility)
- **Dependents:**
  - `context.Compressor` (budget allocation)
  - Shards (context validation)
- **Key Wiring:**
  - `CountTokens(text)` - Estimate token count
  - `ReserveTokens(budget, percentages)` - Allocate budget
  - Heuristic: ~4 chars/token (English text)

#### 6.4 Fact Serializer
- **File:** `internal/context/serializer.go`
- **Boot Step:** On-demand
- **Logging Category:** `context`
- **Dependencies:**
  - None (stateless utility)
- **Dependents:**
  - `context.Compressor` (fact formatting)
  - Shards (context injection)
- **Key Wiring:**
  - `SerializeFacts(facts)` - Convert to Datalog string
  - `DeserializeFacts(text)` - Parse Datalog string
  - Format: `predicate(arg1, arg2, ...).`

---

### 7. CAMPAIGN ORCHESTRATION (5 systems)

#### 7.1 Campaign Orchestrator
- **File:** `internal/campaign/orchestrator.go`
- **Boot Step:** On-demand (via `/campaign start`)
- **Logging Category:** `campaign`
- **Dependencies:**
  - `perception.Client` (LLM for planning)
  - `mangle.Kernel` (fact storage)
  - `core.ShardManager` (shard execution)
  - `core.VirtualStore` (action execution)
  - `tactile.Executor` (command execution)
- **Dependents:**
  - TUI chat model (campaign commands)
  - `campaign.Decomposer` (planning)
  - `campaign.Replanner` (adaptive replanning)
- **Key Wiring:**
  - `NewOrchestrator(cfg)` - Create orchestrator
  - `SetCampaign(campaign)` - Load campaign plan
  - `Run(ctx)` - Execute campaign
  - `Pause()`, `Resume()` - Lifecycle control
- **Campaign Lifecycle:**
  1. Planning → Decomposition
  2. Execution → Phase-by-phase task execution
  3. Checkpointing → Save state after each task
  4. Replanning → Adapt to failures/changes
  5. Completion → Final validation

#### 7.2 Decomposer
- **File:** `internal/campaign/decomposer.go`
- **Boot Step:** On-demand
- **Logging Category:** `campaign`
- **Dependencies:**
  - `perception.Client` (LLM for decomposition)
  - `mangle.Kernel` (world model facts)
- **Dependents:**
  - `campaign.Orchestrator`
- **Key Wiring:**
  - `Decompose(req)` - Break goal into phases/tasks
  - Analyzes source documents
  - Generates executable task DAG
- **Decomposition Strategy:**
  - Extract requirements from docs
  - Identify dependencies
  - Estimate effort
  - Generate validation criteria

#### 7.3 Context Pager
- **File:** `internal/campaign/context_pager.go`
- **Boot Step:** On-demand
- **Logging Category:** `campaign`
- **Dependencies:**
  - `context.Compressor` (compression)
  - `context.Activation` (fact selection)
- **Dependents:**
  - `campaign.Orchestrator` (context injection to shards)
- **Key Wiring:**
  - `PageContext(taskID, budget)` - Select relevant facts
  - Injects: campaign goal, phase context, task dependencies
  - Respects token budget per shard

#### 7.4 Checkpoint Runner
- **File:** `internal/campaign/checkpoint.go`
- **Boot Step:** On-demand
- **Logging Category:** `campaign`
- **Dependencies:**
  - File I/O (save campaign state)
- **Dependents:**
  - `campaign.Orchestrator`
- **Key Wiring:**
  - `SaveCheckpoint(campaign)` - Persist state
  - `LoadCheckpoint(id)` - Resume campaign
  - Checkpoint after each task completion
- **Checkpoint Data:**
  - Campaign ID
  - Current phase
  - Completed tasks
  - Failed tasks
  - Learnings applied
  - Revision number

#### 7.5 Replanner
- **File:** `internal/campaign/replan.go`
- **Boot Step:** On-demand
- **Logging Category:** `campaign`
- **Dependencies:**
  - `perception.Client` (LLM for replanning)
  - `mangle.Kernel` (current state)
- **Dependents:**
  - `campaign.Orchestrator`
- **Key Wiring:**
  - `Replan(campaign, failures)` - Adaptive replanning
  - Triggered on:
    - Task failure (3+ consecutive)
    - Blocked tasks (dependencies unmet)
    - User intervention
  - Generates revised plan with new tasks

---

### 8. AUTOPOIESIS & SELF-MODIFICATION (7 systems)

#### 8.1 Autopoiesis Orchestrator
- **File:** `internal/autopoiesis/autopoiesis.go`
- **Boot Step:** #12
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `perception.Client` (LLM for generation)
  - `mangle.Kernel` (fact assertions)
  - `config` (tool generation settings)
- **Dependents:**
  - `core.VirtualStore` (tool generation routing)
  - `autopoiesis.OuroborosLoop`
  - `autopoiesis.QualityEvaluator`
  - `autopoiesis.ToolRefiner`
- **Key Wiring:**
  - `NewOrchestrator(client, cfg)` - Create orchestrator
  - `SetKernel(kernel)` - Inject kernel
  - `ExecuteOuroborosLoop(ctx, need)` - Full tool generation
  - `RecordExecution(feedback)` - Learning
  - `EvaluateToolQuality(feedback)` - Quality assessment
  - `ShouldRefineTool(name)` - Refinement trigger
  - `RefineTool(ctx, name, code)` - Improvement

#### 8.2 Ouroboros Loop
- **File:** `internal/autopoiesis/ouroboros.go`
- **Boot Step:** Lazy (on tool generation)
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `perception.Client` (code generation)
  - `autopoiesis.SafetyChecker`
  - Go compiler (`go build`)
- **Dependents:**
  - `autopoiesis.Orchestrator`
  - `core.VirtualStore` (tool registration)
- **Key Wiring:**
  - `GenerateTool(ctx, need)` - LLM code generation
  - `CheckSafety(code)` - Safety validation
  - `CompileTool(code, name)` - Compile to binary
  - `RegisterTool(name, path)` - Add to registry
  - `ExecuteTool(name, input)` - Run tool
- **Loop Stages:**
  1. **Detection** - Identify missing capability
  2. **Specification** - LLM generates code
  3. **Safety Check** - No forbidden imports/calls
  4. **Compilation** - `go build` to binary
  5. **Registration** - Add to tool registry
  6. **Execution** - JSON input/output

#### 8.3 Safety Checker
- **File:** `internal/autopoiesis/checker.go`
- **Boot Step:** Lazy
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - None (static analysis)
- **Dependents:**
  - `autopoiesis.OuroborosLoop`
- **Key Wiring:**
  - `CheckSafety(code)` - Validate code
  - Forbidden imports: `unsafe`, `syscall`, `plugin`, `net`
  - Forbidden calls: `os.RemoveAll`, `os.Chmod`, `unsafe.Pointer`

#### 8.4 Quality Evaluator
- **File:** `internal/autopoiesis/quality.go`
- **Boot Step:** Lazy
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `autopoiesis.ToolQualityProfile` (expectations)
- **Dependents:**
  - `autopoiesis.Orchestrator`
- **Key Wiring:**
  - `EvaluateQuality(feedback, profile)` - Assess quality
  - Dimensions: Completeness, Accuracy, Efficiency, Relevance
  - Returns score (0-1) + issues + suggestions

#### 8.5 Feedback System
- **File:** `internal/autopoiesis/feedback.go`
- **Boot Step:** Lazy
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `store.LearningStore` (persistence)
- **Dependents:**
  - `autopoiesis.Orchestrator`
- **Key Wiring:**
  - `RecordFeedback(exec)` - Save execution result
  - `GetFeedback(toolName)` - Retrieve history
  - Tracks: success rate, avg quality, known issues

#### 8.6 Tool Profiles
- **File:** `internal/autopoiesis/profiles.go`
- **Boot Step:** Lazy
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `perception.Client` (LLM for profile generation)
- **Dependents:**
  - `autopoiesis.QualityEvaluator`
- **Key Wiring:**
  - `GenerateProfile(ctx, name, desc, code)` - LLM creates profile
  - `GetProfile(name)` - Retrieve profile
  - Defines: expected duration, output size, caching, custom dimensions

#### 8.7 Trace Collector
- **File:** `internal/autopoiesis/traces.go`
- **Boot Step:** Lazy
- **Logging Category:** `autopoiesis`
- **Dependencies:**
  - `store.TraceStore` (persistence)
- **Dependents:**
  - `autopoiesis.Orchestrator`
- **Key Wiring:**
  - `CaptureTrace(gen)` - Save reasoning trace
  - `AnalyzeGenerations(ctx)` - Audit all traces
  - Captures: system prompt, user prompt, raw response, chain of thought

---

### 9. SHARD IMPLEMENTATIONS (5 systems)

#### 9.1 CoderShard
- **File:** `internal/shards/coder/coder.go`
- **Boot Step:** Registered during ShardManager init (#11)
- **Logging Category:** `coder`
- **Dependencies:**
  - `perception.Client` (code generation)
  - `mangle.Kernel` (parent kernel)
  - `core.VirtualStore` (file operations)
  - `tactile.Executor` (build/test execution)
- **Dependents:**
  - `core.ShardManager` (spawning)
  - `campaign.Orchestrator` (task execution)
  - `core.TDDLoop` (auto-fix)
- **Key Wiring:**
  - Factory registered as `"coder"`
  - `Execute(ctx, task, sessionCtx)` - Main execution
  - `SetParentKernel(kernel)` - Inject kernel
  - `SetLLMClient(client)` - Inject LLM
  - `SetVirtualStore(store)` - Inject file ops
- **Capabilities:**
  - Code generation
  - Refactoring
  - Bug fixing
  - TDD loop (Red → Green → Refactor)
  - Tool delegation (Ouroboros routing)

#### 9.2 TesterShard
- **File:** `internal/shards/tester/tester.go`
- **Boot Step:** Registered during ShardManager init (#11)
- **Logging Category:** `tester`
- **Dependencies:**
  - `perception.Client` (test generation)
  - `mangle.Kernel` (parent kernel)
  - `tactile.Executor` (test execution)
- **Dependents:**
  - `core.ShardManager` (spawning)
  - `core.TDDLoop` (test execution)
- **Key Wiring:**
  - Factory registered as `"tester"`
  - `Execute(ctx, task, sessionCtx)` - Main execution
  - Capabilities:
    - Test generation
    - Coverage analysis
    - Assertion generation
    - Mocking

#### 9.3 ReviewerShard
- **File:** `internal/shards/reviewer/reviewer.go`
- **Boot Step:** Registered during ShardManager init (#11)
- **Logging Category:** `reviewer`
- **Dependencies:**
  - `perception.Client` (review generation)
  - `mangle.Kernel` (parent kernel)
  - `verification.Verifier` (static analysis)
- **Dependents:**
  - `core.ShardManager` (spawning)
  - `campaign.Orchestrator` (quality gates)
- **Key Wiring:**
  - Factory registered as `"reviewer"`
  - `Execute(ctx, task, sessionCtx)` - Main execution
  - Capabilities:
    - Code review
    - Security analysis
    - Dependency checks
    - Custom rule validation

#### 9.4 ResearcherShard
- **File:** `internal/shards/researcher/researcher.go`
- **Boot Step:** Registered during ShardManager init (#11)
- **Logging Category:** `researcher`
- **Dependencies:**
  - `perception.Client` (research synthesis)
  - `mangle.Kernel` (parent kernel)
  - Context7 API (optional, for doc fetching)
- **Dependents:**
  - `core.ShardManager` (spawning)
  - `campaign.Orchestrator` (knowledge gathering)
  - `init.Initializer` (project profiling)
- **Key Wiring:**
  - Factory registered as `"researcher"`
  - `Execute(ctx, task, sessionCtx)` - Main execution
  - Capabilities:
    - Documentation ingestion
    - llms.txt parsing
    - Context7-style research
    - Knowledge atom extraction

#### 9.5 System Shards
- **Files:** `internal/shards/system/`
- **Boot Step:** #13 (after ShardManager)
- **Logging Category:** `system_shards`
- **Dependencies:**
  - Varies per shard
- **Dependents:**
  - `core.ShardManager` (lifecycle)
- **System Shard Inventory:**

  | Shard | Purpose | Key Dependencies |
  |-------|---------|------------------|
  | `legislator` | Constitutional enforcement | `mangle.Kernel` |
  | `tactile_router` | Action routing | `core.VirtualStore`, `browser.SessionManager` |
  | `world_model_ingestor` | File topology updates | `world.Scanner`, `mangle.Kernel` |
  | `perception_firewall` | NL validation | `perception.Transducer` |
  | `executive_policy` | next_action derivation | `mangle.Kernel` |
  | `session_planner` | Campaign triggers | `campaign.Orchestrator` |

---

### 10. WORLD MODEL (3 systems)

#### 10.1 World Scanner
- **File:** `internal/world/scanner.go`
- **Boot Step:** #9 (or lazy)
- **Logging Category:** `world`
- **Dependencies:**
  - File I/O
  - AST parsers (Go, JS, Python, etc.)
- **Dependents:**
  - `mangle.Kernel` (fact loading)
  - `system.WorldModelIngestor` (continuous updates)
  - `init.Initializer` (cold start)
- **Key Wiring:**
  - `ScanWorkspace(path)` - Scan directory
  - Generates facts:
    - `file_topology/4` - File metadata
    - `directory/1` - Directory structure
    - `symbol_graph/4` - Function/class definitions
    - `depends_on/2` - Import graph

#### 10.2 AST Projection
- **File:** `internal/world/ast.go`
- **Boot Step:** During scanning
- **Logging Category:** `world`
- **Dependencies:**
  - Language-specific parsers
- **Dependents:**
  - `world.Scanner`
- **Key Wiring:**
  - `ParseAST(file, lang)` - Extract symbols
  - Supports: Go, JavaScript, Python, Rust (basic)

#### 10.3 Symbol Graph
- **File:** Integrated in `world.Scanner`
- **Boot Step:** During scanning
- **Logging Category:** `world`
- **Dependencies:**
  - AST projection
- **Dependents:**
  - `mangle.Kernel` (symbol_graph facts)
  - `context.Activation` (semantic links)
- **Key Wiring:**
  - `symbol_graph(File, SymbolName, SymbolType, LineNum)`
  - Types: `/function`, `/class`, `/method`, `/variable`, `/import`

---

### 11. TACTICAL EXECUTION (2 systems)

#### 11.1 Safe Executor
- **File:** `internal/tactile/executor.go`
- **Boot Step:** #10 (before VirtualStore)
- **Logging Category:** `tools`
- **Dependencies:**
  - `config` (allowed binaries, env vars)
- **Dependents:**
  - `core.VirtualStore` (command execution)
  - `coder.Shard` (build/test)
  - `tester.Shard` (test execution)
  - `campaign.Orchestrator` (action execution)
- **Key Wiring:**
  - `NewSafeExecutor()` - Create executor
  - `Execute(cmd, args, workdir)` - Run command
  - Validates: binary in allowlist, safe environment
- **Safety Features:**
  - Binary allowlist (go, git, npm, python, etc.)
  - Environment variable filtering
  - Working directory validation
  - Timeout enforcement

#### 11.2 Tool Registry
- **File:** `internal/core/tool_registry.go`
- **Boot Step:** Lazy (on tool registration)
- **Logging Category:** `tools`
- **Dependencies:**
  - File I/O (tool discovery)
- **Dependents:**
  - `autopoiesis.OuroborosLoop` (tool registration)
  - `core.VirtualStore` (tool invocation)
- **Key Wiring:**
  - `RegisterTool(name, path)` - Add tool
  - `GetTool(name)` - Retrieve tool path
  - `ListTools()` - Enumerate all tools
  - Tool path: `.nerd/tools/.compiled/<name>`

---

### 12. TUI & CHAT INTERFACE (4 systems)

#### 12.1 Chat Model (Bubble Tea)
- **File:** `cmd/nerd/chat/model.go`
- **Boot Step:** #13 (after all backend systems)
- **Logging Category:** `session`
- **Dependencies:**
  - ALL backend systems (full integration)
  - Bubble Tea framework
- **Dependents:**
  - None (top-level UI)
- **Key Wiring:**
  - `InitChat(cfg)` - Create model
  - `Init()` - Bubble Tea init
  - `Update(msg)` - Bubble Tea update loop
  - `View()` - Bubble Tea view render
  - Backend injection:
    - `kernel` - Mangle kernel
    - `client` - LLM client
    - `shardMgr` - Shard manager
    - `virtualStore` - Virtual store
    - `transducer` - Perception transducer
    - `orchestrator` - Autopoiesis orchestrator
    - `campaignOrch` - Campaign orchestrator
    - `shadowMode` - Shadow mode
    - `compressor` - Context compressor
    - `usageTracker` - Usage tracking

#### 12.2 Session Management
- **File:** `cmd/nerd/chat/session.go`
- **Boot Step:** During `InitChat`
- **Logging Category:** `session`
- **Dependencies:**
  - `system.Factory` (BootCortex)
  - `config` (session config)
  - `store.LocalStore` (persistence)
- **Dependents:**
  - Chat Model
- **Key Wiring:**
  - `loadOrCreateSession(cfg)` - Hydrate session
  - `saveSession(session)` - Persist state
  - Calls `system.BootCortex()` for full system initialization

#### 12.3 Command Handling
- **File:** `cmd/nerd/chat/commands.go`
- **Boot Step:** N/A (event handlers)
- **Logging Category:** `session`
- **Dependencies:**
  - Chat Model (all backend systems)
- **Dependents:**
  - Chat Model (command routing)
- **Key Wiring:**
  - `handleCommand(input)` - Route `/commands`
  - Commands:
    - `/help` - Show help
    - `/clear` - Clear history
    - `/agent <name> <task>` - Spawn shard
    - `/review [target]` - Code review
    - `/campaign start <goal>` - Start campaign
    - `/shadow <action>` - Shadow mode
    - `/config wizard` - Config wizard
    - `/init` - Initialize workspace
    - `/scan` - Re-scan workspace

#### 12.4 View Rendering
- **File:** `cmd/nerd/chat/view.go`
- **Boot Step:** N/A (rendering)
- **Logging Category:** N/A
- **Dependencies:**
  - Chat Model
  - Glamour (markdown rendering)
- **Dependents:**
  - Chat Model
- **Key Wiring:**
  - `View()` - Render full UI
  - `renderHeader()` - Status bar
  - `renderHistory()` - Message list
  - `renderFooter()` - Input box
  - `renderCampaignStatus()` - Campaign progress

---

### 13. INITIALIZATION (3 systems)

#### 13.1 Project Initializer
- **File:** `internal/init/initializer.go`
- **Boot Step:** On-demand (`/init` command)
- **Logging Category:** `boot`
- **Dependencies:**
  - `perception.Client` (LLM for profiling)
  - `researcher.Shard` (documentation ingestion)
  - `world.Scanner` (codebase scanning)
  - `mangle.Kernel` (fact storage)
- **Dependents:**
  - TUI `/init` command
  - CLI `nerd init`
- **Key Wiring:**
  - `Initialize(ctx)` - Full cold start
  - Steps:
    1. Create `.nerd/` directory structure
    2. Scan codebase (file topology)
    3. Detect language/framework
    4. Generate project profile
    5. Initialize knowledge DB
    6. Set up user preferences

#### 13.2 Profile Generation
- **File:** `internal/init/profile.go`
- **Boot Step:** During initialization
- **Logging Category:** `boot`
- **Dependencies:**
  - `perception.Client` (LLM analysis)
  - `world.Scanner` (codebase facts)
- **Dependents:**
  - `init.Initializer`
- **Key Wiring:**
  - `GenerateProfile(ctx, facts)` - LLM creates profile
  - Generates `.nerd/profile.mg` with:
    - Language facts
    - Framework facts
    - Architecture facts
    - Preference facts

#### 13.3 Agent Recommendations
- **File:** `internal/init/agents.go`
- **Boot Step:** During initialization
- **Logging Category:** `boot`
- **Dependencies:**
  - `perception.Client` (LLM analysis)
  - Project profile
- **Dependents:**
  - `init.Initializer`
- **Key Wiring:**
  - `RecommendAgents(ctx, profile)` - Suggest specialist shards
  - Examples:
    - Go project → GoExpert shard
    - React project → ReactExpert shard
    - REST API → APIDesigner shard

---

## Integration Diagrams

### Boot Sequence Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│ STEP 1: LOGGING SYSTEM                                          │
│ - Initialize .nerd/logs/ directory                              │
│ - Load config from .nerd/config.json                            │
│ - Create loggers for 22 categories                              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 2: CONFIGURATION LOADER                                    │
│ - Load .nerd/config.json                                        │
│ - Apply environment variable overrides                          │
│ - Validate provider API keys                                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 3: LLM CLIENT PROVIDER                                     │
│ - Detect provider (zai/anthropic/openai/gemini/xai/openrouter) │
│ - Initialize client with API key                                │
│ - Wrap with TracingClient if tracing enabled                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 4: JIT PROMPT COMPILER                                     │
│ - Load embedded prompt corpus (go:embed)                        │
│ - Initialize JITPromptCompiler                                  │
│ - Create registrar/unregistrar callbacks for ShardManager       │
│ - Wire compiler to PromptAssembler                              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 5: USAGE TRACKER                                           │
│ - Initialize lightweight tracking DB                            │
│ - Create usage metrics tables                                   │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 6: LOCALSTORE / SQLITE                                     │
│ - Open/create .nerd/knowledge.db                                │
│ - Initialize schema (cold_storage, knowledge_graph, vectors)    │
│ - Detect sqlite-vec extension (FAIL FAST if missing)            │
│ - Initialize TraceStore, LearningStore                          │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 7: EMBEDDING ENGINE                                        │
│ - Detect provider (Ollama/GenAI)                                │
│ - Initialize connection                                         │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 8: MANGLE KERNEL                                           │
│ - Load embedded schemas.gl + policy.gl                          │
│ - Load user overrides from .nerd/mangle/                        │
│ - Force initial evaluation (CRITICAL: prevents "not init" bugs) │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 9: PERCEPTION TRANSDUCER                                   │
│ - Load VerbCorpus from Mangle taxonomy                          │
│ - Initialize Piggyback Protocol parser                          │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 10: WORLD SCANNER                                          │
│ - Initialize AST parsers                                        │
│ - Ready to scan workspace on demand                             │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 11: VIRTUAL STORE                                          │
│ - Create Safe Executor                                          │
│ - Inject LocalDB, Kernel, LearningStore                         │
│ - Wire Ouroboros as ToolGenerator                               │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 12: SHARD MANAGER                                          │
│ - Inject Parent Kernel                                          │
│ - Inject LLM Client                                             │
│ - Inject JIT callbacks (promptLoader, jitRegistrar, jitUnreg.)  │
│ - Register all shard factories (coder, tester, reviewer, etc.)  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 13: AUTOPOIESIS ORCHESTRATOR                               │
│ - Initialize Ouroboros Loop                                     │
│ - Initialize Quality Evaluator, ToolRefiner, etc.               │
│ - Inject Kernel bridge                                          │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ STEP 14: SYSTEM SHARDS                                          │
│ - Start legislator, tactile_router, world_model_ingestor        │
│ - Respect disable flags                                         │
└─────────────────────────────────────────────────────────────────┘

                           ↓
                 ┌──────────────────┐
                 │ SYSTEM READY     │
                 └──────────────────┘
```

### Dependency Injection Graph

```
                    ┌──────────────────┐
                    │   Config.json    │
                    └────────┬─────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
          ↓                  ↓                  ↓
   ┌──────────┐      ┌──────────┐      ┌──────────┐
   │ Logging  │      │ LLM      │      │ Mangle   │
   │ System   │      │ Client   │      │ Kernel   │
   └──────────┘      └────┬─────┘      └────┬─────┘
          │               │                  │
          │               ├──────────────────┤
          │               │                  │
          │               ↓                  ↓
          │        ┌─────────────┐    ┌─────────────┐
          │        │ Perception  │    │ Virtual     │
          │        │ Transducer  │    │ Store       │
          │        └──────┬──────┘    └──────┬──────┘
          │               │                  │
          │               └────────┬─────────┘
          │                        │
          │                        ↓
          │                ┌───────────────┐
          │                │ Shard         │
          │                │ Manager       │
          │                └───────┬───────┘
          │                        │
          │     ┌──────────────────┼──────────────────┐
          │     │                  │                  │
          │     ↓                  ↓                  ↓
          │ ┌────────┐      ┌──────────┐      ┌──────────┐
          │ │ Coder  │      │ Tester   │      │ Reviewer │
          │ │ Shard  │      │ Shard    │      │ Shard    │
          │ └────────┘      └──────────┘      └──────────┘
          │
          │                        ↓
          │                ┌───────────────┐
          │                │ Autopoiesis   │
          │                │ Orchestrator  │
          │                └───────┬───────┘
          │                        │
          │                        ↓
          │                ┌───────────────┐
          │                │ Campaign      │
          │                │ Orchestrator  │
          │                └───────────────┘
          │
          └────────────────→ ALL SYSTEMS
```

### Shard Lifecycle Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│ SHARD REGISTRATION (Boot Step #11)                              │
│                                                                  │
│  ShardManager.RegisterShard("coder", CoderFactory)              │
│  ShardManager.RegisterShard("tester", TesterFactory)            │
│  ShardManager.RegisterShard("reviewer", ReviewerFactory)        │
│  ShardManager.RegisterShard("researcher", ResearcherFactory)    │
│  ShardManager.RegisterShard("tactile_router", TactileFactory)   │
│  ...                                                             │
└──────────────────────────────────────────────────────────────────┘
                           │
                           ↓
┌──────────────────────────────────────────────────────────────────┐
│ SHARD SPAWNING (User Request)                                    │
│                                                                   │
│  user_intent(_, /mutation, /fix, "auth.go", _) →                │
│  Kernel derives: delegate_task(/coder, "fix auth.go", /pending) │
│                                                                   │
│  ShardManager.Spawn(ctx, "coder", "fix auth.go")                │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│ SHARD INITIALIZATION                                              │
│                                                                   │
│  1. Factory creates shard instance                               │
│  2. Inject ParentKernel                                          │
│  3. Inject LLMClient                                             │
│  4. Inject VirtualStore (for coder/tester)                       │
│  5. Inject BrowserManager (for tactile_router)                   │
│  6. Load SessionContext (blackboard pattern)                     │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│ SHARD EXECUTION                                                   │
│                                                                   │
│  shard.Execute(ctx, task, sessionCtx) →                          │
│    - Parse task                                                  │
│    - Query parent kernel for context                             │
│    - Call LLM with injected context                              │
│    - Extract Piggyback control packet                            │
│    - Execute actions via VirtualStore                            │
│    - Assert result facts to parent kernel                        │
│    - Return surface response                                     │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│ SHARD TEARDOWN                                                    │
│                                                                   │
│  Type A (Ephemeral): Immediately destroyed after execution       │
│  Type B (Persistent): Stays loaded in memory until eviction      │
│  Type S (System): Runs until shutdown                            │
└──────────────────────────────────────────────────────────────────┘
```

---

## Common Integration Patterns

### Pattern 1: Kernel Bridge Pattern

Many components need to interact with the Mangle kernel but can't import `internal/core` directly due to import cycles. Solution: Define a minimal interface.

**Example: Autopoiesis ↔ Kernel**

```go
// internal/autopoiesis/autopoiesis.go
type KernelInterface interface {
    AssertFact(fact KernelFact) error
    QueryPredicate(predicate string) ([]KernelFact, error)
    QueryBool(predicate string) bool
}

// internal/core/autopoiesis_bridge.go
type AutopoiesisBridge struct {
    kernel *RealKernel
}

func (b *AutopoiesisBridge) AssertFact(fact autopoiesis.KernelFact) error {
    return b.kernel.Assert(core.Fact{
        Predicate: fact.Predicate,
        Args:      fact.Args,
    })
}
```

### Pattern 2: Optional Dependency Injection

Components should gracefully degrade when optional dependencies are unavailable.

**Example: VirtualStore with Optional LearningStore**

```go
// internal/core/virtual_store.go
type VirtualStore struct {
    kernel        Kernel
    executor      tactile.Executor
    localDB       *store.LocalStore      // Optional
    learningStore *store.LearningStore   // Optional
    toolGen       autopoiesis.ToolGenerator // Optional
}

func (vs *VirtualStore) SetLearningStore(store *store.LearningStore) {
    vs.learningStore = store
}

func (vs *VirtualStore) recordLearning(event LearningEvent) {
    if vs.learningStore != nil {
        vs.learningStore.RecordLearning(event)
    }
    // Graceful no-op if not available
}
```

### Pattern 3: Factory Registration

Shard implementations are registered via factory functions to avoid direct instantiation.

**Example: CoderShard Registration**

```go
// internal/shards/registration.go
func RegisterAllShardFactories(mgr *core.ShardManager, ctx RegistryContext) {
    // Coder Shard
    mgr.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
        shard := coder.NewCoderShard(id, config)
        shard.SetParentKernel(ctx.Kernel)
        shard.SetLLMClient(ctx.LLMClient)
        shard.SetVirtualStore(ctx.VirtualStore)
        return shard
    })

    // Tester Shard
    mgr.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
        shard := tester.NewTesterShard(id, config)
        shard.SetParentKernel(ctx.Kernel)
        shard.SetLLMClient(ctx.LLMClient)
        return shard
    })

    // ... etc for all shards
}
```

### Pattern 4: Blackboard Session Context

Shards receive compressed session context via the blackboard pattern to avoid re-querying kernel.

**Example: SessionContext Injection**

```go
// internal/core/shard_manager.go
func (sm *ShardManager) Spawn(ctx context.Context, shardType, task string) (string, error) {
    // Build SessionContext from kernel + store
    sessionCtx := buildSessionContext(sm.kernel, sm.localDB)

    // Inject into shard
    shard := sm.factory[shardType](id, config)
    result, err := shard.Execute(ctx, task, sessionCtx)

    return result, err
}

func buildSessionContext(kernel Kernel, db *store.LocalStore) SessionContext {
    return SessionContext{
        CompressedHistory: compressor.Compress(db.GetHistory()),
        ImpactedFiles:     kernel.Query("impacted"),
        CurrentDiagnostics: kernel.Query("diagnostic"),
        UserIntent:        kernel.Query("user_intent"),
        // ... etc
    }
}
```

### Pattern 5: Piggyback Protocol

All LLM responses use dual-channel communication for human + machine consumption.

**Example: Piggyback Envelope**

```go
// LLM response format:
```
Here's how to fix the authentication bug:

1. Update the password hashing to use bcrypt
2. Add rate limiting to the login endpoint
3. Enable 2FA for all accounts

CONTROL_PACKET_START
{
  "intent_classification": {
    "category": "/mutation",
    "verb": "/fix",
    "target": "auth.go",
    "confidence": 0.95
  },
  "mangle_updates": [
    "modified(\"/src/auth.go\")",
    "diagnostic_resolved(/auth_bug, /fixed)"
  ],
  "memory_operations": [
    {
      "operation": "store",
      "domain": "security_fixes",
      "content": "bcrypt hashing prevents rainbow table attacks"
    }
  ]
}
CONTROL_PACKET_END
```

// Parsing:
envelope, err := transducer.ExtractPiggyback(response)
// envelope.Surface → "Here's how to fix..."
// envelope.Control.IntentClassification → {...}
// envelope.Control.MangleUpdates → ["modified(...)", ...]
```

---

## Troubleshooting Guide

### Problem: "Kernel not initialized" error

**Symptom:** Shards fail with `kernel.Query()` panic

**Root Cause:** Mangle kernel not evaluated before first query

**Solution:**
```go
// internal/system/factory.go (BootCortex)
kernel := core.NewRealKernel()
if err := kernel.Evaluate(); err != nil {  // ← CRITICAL
    return nil, fmt.Errorf("failed to boot kernel: %w", err)
}
```

---

### Problem: Shard factory not registered

**Symptom:** `ShardManager.Spawn()` returns "unknown shard type"

**Root Cause:** Factory not registered during initialization

**Solution:**
```go
// internal/shards/registration.go
func RegisterAllShardFactories(mgr *core.ShardManager, ctx RegistryContext) {
    mgr.RegisterShard("your_shard", YourShardFactory)
}

// internal/system/factory.go
shards.RegisterAllShardFactories(shardManager, regCtx)
```

---

### Problem: sqlite-vec extension not found

**Symptom:** "sqlite-vec extension not available" error on boot

**Root Cause:** SQLite not built with vec0 extension

**Solution:**
```powershell
# Windows (from CLAUDE.md)
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd
```

---

### Problem: LLM provider detection fails

**Symptom:** "No LLM provider configured" error

**Root Cause:** No API key found in config or environment

**Solution:**
```json
// .nerd/config.json
{
  "provider": "zai",
  "zai_api_key": "your-key-here"
}
```

Or set environment variable:
```bash
export ZAI_API_KEY=your-key-here
# Or: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, XAI_API_KEY
```

---

### Problem: Circular import dependency

**Symptom:** `import cycle not allowed`

**Root Cause:** Component A imports B, B imports A

**Solution:** Use interface-based dependency injection (see Pattern 1)

---

### Problem: Shard has no parent kernel

**Symptom:** Shard tries to query facts but kernel is nil

**Root Cause:** `SetParentKernel()` not called during factory registration

**Solution:**
```go
mgr.RegisterShard("shard_name", func(id string, cfg core.ShardConfig) core.ShardAgent {
    shard := NewYourShard(id, cfg)
    shard.SetParentKernel(ctx.Kernel)  // ← REQUIRED
    return shard
})
```

---

### Problem: Context compression not triggered

**Symptom:** Token usage exceeds limit, no compression happening

**Root Cause:** Compressor not wired to chat session

**Solution:**
```go
// cmd/nerd/chat/session.go
model.compressor = ctxcompress.NewCompressor(client, kernel, config)
```

---

### Problem: Tool generation fails silently

**Symptom:** Ouroboros loop returns success but no tool binary

**Root Cause:** Compilation failed but error not surfaced

**Solution:** Check logging:
```
.nerd/logs/2025-12-08_autopoiesis.log
```

Look for `[TOOL_ERROR]` tags.

---

### Problem: Campaign orchestrator stuck

**Symptom:** Campaign shows "in_progress" but no tasks executing

**Root Cause:** Checkpoint not loaded correctly on resume

**Solution:** Delete corrupted checkpoint:
```bash
rm .nerd/campaigns/<campaign-id>.json
```

Then restart campaign from scratch.

---

## Appendix: Logging Category Reference

| Category | Systems | Log File Pattern |
|----------|---------|------------------|
| `boot` | Logging, Config, Init | `YYYY-MM-DD_boot.log` |
| `session` | Chat, Session Mgmt | `YYYY-MM-DD_session.log` |
| `kernel` | Mangle Kernel, Differential | `YYYY-MM-DD_kernel.log` |
| `api` | LLM Clients, Tracing | `YYYY-MM-DD_api.log` |
| `perception` | Transducer, Taxonomy | `YYYY-MM-DD_perception.log` |
| `articulation` | Emitter, Piggyback | `YYYY-MM-DD_articulation.log` |
| `routing` | VirtualStore routing | `YYYY-MM-DD_routing.log` |
| `tools` | Safe Executor, Tool Registry | `YYYY-MM-DD_tools.log` |
| `virtual_store` | VirtualStore | `YYYY-MM-DD_virtual_store.log` |
| `shards` | ShardManager | `YYYY-MM-DD_shards.log` |
| `coder` | CoderShard | `YYYY-MM-DD_coder.log` |
| `tester` | TesterShard | `YYYY-MM-DD_tester.log` |
| `reviewer` | ReviewerShard | `YYYY-MM-DD_reviewer.log` |
| `researcher` | ResearcherShard | `YYYY-MM-DD_researcher.log` |
| `system_shards` | System Shards | `YYYY-MM-DD_system_shards.log` |
| `dream` | Dreamer, ShadowMode | `YYYY-MM-DD_dream.log` |
| `autopoiesis` | Autopoiesis, Ouroboros | `YYYY-MM-DD_autopoiesis.log` |
| `campaign` | Campaign Orchestration | `YYYY-MM-DD_campaign.log` |
| `context` | Compression, Activation | `YYYY-MM-DD_context.log` |
| `world` | World Scanner, AST | `YYYY-MM-DD_world.log` |
| `embedding` | Embedding Engine | `YYYY-MM-DD_embedding.log` |
| `store` | LocalStore, LearningStore | `YYYY-MM-DD_store.log` |

---

## Appendix: Key Configuration Files

| File | Purpose | Format |
|------|---------|--------|
| `.nerd/config.json` | User configuration | JSON |
| `.nerd/profile.mg` | Project profile facts | Mangle |
| `.nerd/mangle/schemas.mg` | Custom schema extensions | Mangle |
| `.nerd/mangle/policy.mg` | Custom policy rules | Mangle |
| `.nerd/knowledge.db` | Persistent fact storage | SQLite |
| `.nerd/campaigns/<id>.json` | Campaign checkpoint | JSON |
| `.nerd/tools/.compiled/<tool>` | Generated tool binaries | Binary |
| `.nerd/browser/sessions.json` | Browser session state | JSON |
| `.nerd/logs/<date>_<category>.log` | Category logs | Text |

---

## Conclusion

This systems integration map documents all 40+ systems in codeNERD and their wiring requirements. It serves as:

1. **Onboarding Guide** - New developers can understand the full architecture
2. **Integration Reference** - Clear dependency chains and injection points
3. **Debugging Aid** - Troubleshooting common integration failures
4. **Audit Tool** - Verify all systems are correctly wired

**Critical Takeaway:** The boot sequence MUST be followed exactly. Systems cannot be initialized out of order without violating dependency contracts.

---

**Document Maintainers:**
- Update this document when adding new systems
- Update when changing boot sequence
- Update when modifying integration interfaces

**Version History:**
- v1.0 (2025-12-08): Initial comprehensive map
