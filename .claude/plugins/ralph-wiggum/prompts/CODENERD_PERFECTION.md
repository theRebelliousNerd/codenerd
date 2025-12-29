# codeNERD Perfection Loop (Condensed)

> **Ralph Wiggum Prompt for Achieving 100% Tested Reliability**
>
> This prompt is fed iteratively. Each cycle, you see your previous work in files and git.
> Progress is tracked in `.nerd/ralph/perfection_state.json`.
> All fixes go through root-cause analysis - NO BAND-AIDS.
> Documentation updates (README.md, CLAUDE.md) are REQUIRED for every fix.

---

## Test Bed Configuration

| Item | Path |
|------|------|
| Test Bed | `C:\CodeProjects\tribalFitness` |
| Binary Location | `C:\CodeProjects\tribalFitness\nerd.exe` |
| codeNERD Source | `C:\CodeProjects\codeNERD` |

**ALL testing runs in tribalFitness, NOT codeNERD.**

---

## Completion Promise

Output this ONLY when ALL conditions are TRUE:

```
<promise>CODENERD PERFECTION ACHIEVED</promise>
```

### Promise Conditions (ALL must be true)

- [ ] All 50+ subsystems pass stress tests with zero panics
- [ ] All 22 log categories show clean output (no ERROR, PANIC, FATAL, undefined, nil pointer)
- [ ] tribalFitness app builds and passes tests using codeNERD autonomously
- [ ] Domain experts (coder, tester, reviewer, researcher) complete tasks correctly
- [ ] Northstar vision alignment checks pass
- [ ] Code DOM multi-file edits execute correctly
- [ ] Context compression maintains memory bounds
- [ ] Spreading activation selects relevant facts
- [ ] World systems (holographic, dataflow) analyze correctly
- [ ] MCP integrations discover and install tools
- [ ] JIT prompt compilation works for all personas
- [ ] Ouroboros successfully generates and executes a custom tool
- [ ] Nemesis adversarial review completes without triggering panics
- [ ] Thunderdome attack vectors execute in sandbox without escape
- [ ] Dream State consultation produces valid hypothetical exploration
- [ ] Prompt Evolution records feedback and generates improvement atoms
- [ ] Glass Box visibility shows tool execution
- [ ] Shadow system tracks proposal states
- [ ] All CLI commands verified functional
- [ ] Root-cause fixes committed for every failure encountered
- [ ] All CLAUDE.md files updated for affected packages
- [ ] `.nerd/ralph/perfection_state.json` shows all phases complete

---

## Available Skills

Use these skills proactively during execution:

| Skill | When to Use |
|-------|-------------|
| **go-architect** | ALWAYS for Go code changes - ensures idiomatic patterns, error handling, concurrency safety |
| **mangle-programming** | For Mangle rules, stratification, safety violations, aggregation syntax |
| **codenerd-builder** | For codeNERD components: kernel, transducers, JIT, SubAgents |
| **integration-auditor** | For "code exists but doesn't run" issues, system wiring, SubAgent creation |
| **stress-tester** | For live stress testing, finding panics, edge cases, failure modes |
| **log-analyzer** | For debugging via log analysis, converting logs to Mangle facts |
| **prompt-architect** | For persona atoms, prompt debugging, context injection optimization |
| **charm-tui** | For TUI components with Bubbletea and Lipgloss |
| **rod-builder** | For browser automation with Rod |
| **research-builder** | For ResearcherShard, knowledge extraction, documentation gathering |

---

## Self-Enhancement Protocol

You have FULL PERMISSION to add logging, tests, and instrumentation as needed.

### When to Add Logging

Add logging when you encounter:
- [ ] Silent failures - code paths that fail without output
- [ ] Missing visibility - operations where you can't tell what happened
- [ ] Cross-system boundaries - entry/exit points between packages
- [ ] Error paths - all error returns should log context
- [ ] State transitions - campaign phases, shard lifecycle, session events

**Logging Categories (22 total):**
CategoryKernel, CategorySession, CategoryShards, CategoryPerception, CategoryArticulation, CategoryCampaign, CategoryAutopoiesis, CategoryNorthstar, CategoryJIT, CategoryWorld, CategoryMCP, CategoryContext, CategoryTactile, CategoryEmbedding, CategoryBrowser, CategoryVerification, CategoryRetrieval, CategoryTransparency, CategoryUX, CategoryConfig, CategoryInit, CategoryTypes

### When to Add Test Files

Add tests when you discover:
- [ ] Untested code paths - functions without `_test.go`
- [ ] Edge cases discovered during stress testing
- [ ] Regression prevention after fixing a bug
- [ ] Integration gaps - cross-package interactions
- [ ] Missing coverage for critical functions

### Self-Enhancement Workflow

1. RUN TEST → Failure or missing visibility
2. DIAGNOSE → What's missing? Logging? Tests? Both?
3. ADD INSTRUMENTATION → Add appropriate logging/tests
4. REBUILD → Build binary, copy to test bed
5. RE-RUN TEST → Verify enhancement worked
6. UPDATE DOCS → CLAUDE.md for affected packages
7. COMMIT → Descriptive commit message

### Packages Likely Needing Enhancement

| Package | What to Add |
|---------|-------------|
| `internal/world/` | Dataflow cache logging, cross-lang tracing tests |
| `internal/context/` | Compression ratio logging, pruning decision logs |
| `internal/northstar/` | Alignment check result logging, observer tests |
| `internal/mcp/` | Tool discovery logging, JIT compilation tests |
| `internal/tactile/` | Docker execution logging, output analyzer tests |
| `internal/session/` | Spawner lifecycle logging, SubAgent tests |
| `internal/autopoiesis/` | Ouroboros step logging, Thunderdome attack logs |

---

## Phase 0: Environment Setup & Binary Deployment

**First iteration only. Skip if `.nerd/ralph/perfection_state.json` exists.**

### Step 1: Recompile codeNERD binary

- [ ] Navigate to codeNERD source directory
- [ ] Clean old binary (remove nerd.exe)
- [ ] Build with sqlite-vec support using CGO_CFLAGS
- [ ] Verify build succeeded (nerd.exe exists)

**How to know it's done:** `nerd.exe` file exists in codeNERD directory

### Step 2: Deploy to tribalFitness test bed

- [ ] Copy nerd.exe to tribalFitness directory
- [ ] Verify binary exists at destination

**How to know it's done:** `ls` shows nerd.exe in tribalFitness

### Step 3: Initialize test bed workspace

- [ ] Navigate to tribalFitness
- [ ] Run `nerd.exe init` if .nerd directory doesn't exist
- [ ] Create Ralph perfection state directories: `.nerd/ralph`, `.nerd/ralph/bugs`
- [ ] Create initial `perfection_state.json` with all 41 phases set to "pending"

**How to know it's done:** `.nerd/ralph/perfection_state.json` exists with proper structure

### Step 4: Verify test bed is ready

- [ ] Run `nerd.exe --version` successfully
- [ ] Run `nerd.exe scan` and see output

**How to know it's done:** Version displays, scan produces output without panic

---

## Phase 1: Kernel Core Stability

**Location:** `internal/core/`
**Subsystems:** RealKernel, VirtualStore, SpawnQueue, LimitsEnforcer, Mangle Self-Healing

### Tests to Run

- [ ] Kernel boot test: Run `nerd.exe scan` with 30s timeout
- [ ] Check logs for panic/fatal/nil pointer
- [ ] Mangle validation: Run a query command
- [ ] Queue stress: Run 5 concurrent simple commands
- [ ] VirtualStore fact operations: List files command

### Verification Criteria

- [ ] All commands complete without panic
- [ ] No error patterns in logs (grep for panic, fatal, nil pointer)
- [ ] Derivation completes within gas limit

**How to know it's done:** All tests pass cleanly, update state file to "complete", move to Phase 2

---

## Phase 2: Perception & Articulation

**Location:** `internal/perception/`, `internal/articulation/`
**Subsystems:** Transducer, LLMClient (7 providers), Emitter, Taxonomy

### Tests to Run

- [ ] Intent parsing with multiple verbs: fix, review, explain, test, research
- [ ] Adversarial input: special characters, Unicode, emojis, null bytes
- [ ] Provider detection: check which LLM provider is configured
- [ ] Articulation output: summarize command produces clean output

### Verification Criteria

- [ ] All intent types parse correctly
- [ ] Adversarial input doesn't panic (graceful error OK)
- [ ] Provider status shows configured provider
- [ ] Emitter produces clean natural language output

**How to know it's done:** All parsing works, no crashes on edge cases

---

## Phase 3: JIT Prompt Compiler

**Location:** `internal/prompt/compiler.go`, `internal/jit/`
**Subsystems:** PromptCompiler, AtomSelector, TokenBudgeter, ContextInjector

### Tests to Run

- [ ] JIT compilation inspection: `nerd.exe jit show`
- [ ] Persona-specific compilation: test coder, tester, reviewer, researcher personas
- [ ] Context-aware atom selection: atoms for specific intent verbs
- [ ] Token budget enforcement: check stats show tokens within budget

### Verification Criteria

- [ ] JIT show displays compiled prompt without crash
- [ ] Each persona compiles different atom sets
- [ ] Intent atoms match expected tools/capabilities
- [ ] Token counts within configured budget

**How to know it's done:** All 4 personas compile correctly, token budget enforced

---

## Phase 4: ConfigFactory (Intent → Tools Mapping)

**Location:** `internal/prompt/config_factory.go`
**Subsystems:** ConfigFactory, ConfigAtom, ToolSelection

### Tests to Run

- [ ] Intent-to-tool mapping: show config for /fix, /test, /research intents
- [ ] Tool registry hydration: list all available tools

### Verification Criteria

- [ ] /fix intent shows: read_file, write_file, edit_file, run_build, git_operation
- [ ] /test intent shows: run_tests, read_file, write_file
- [ ] /research intent shows: web_search, web_fetch, context7_fetch, read_file
- [ ] Tools list shows all categories

**How to know it's done:** Each intent maps to correct tool set

---

## Phase 5: Session & SubAgent Architecture

**Location:** `internal/session/`
**Subsystems:** Executor, Spawner, SubAgent, TaskExecutor

### Tests to Run

- [ ] Clean loop execution: simple read and summarize command
- [ ] SubAgent spawn (ephemeral): spawn coder for simple task
- [ ] Session lifecycle: check logs for session/spawn/subagent events
- [ ] TaskExecutor interface: execute a task command

### Verification Criteria

- [ ] Full OODA cycle completes
- [ ] Ephemeral subagent spawns, executes, terminates
- [ ] Session logs show proper lifecycle events
- [ ] TaskExecutor completes task

**How to know it's done:** SubAgents spawn and terminate correctly

---

## Phase 6: Domain Expert - Coder

**Location:** `internal/prompt/atoms/identity/coder.yaml`
**Subsystems:** CoderPersona, CodeGeneration, BugFixing, Refactoring

### Tests to Run

- [ ] Simple code generation: create a new file with a function
- [ ] Bug fix workflow: find and address TODOs
- [ ] Refactoring: add documentation to exported functions
- [ ] Verify coder identity atom loads in JIT logs

### Verification Criteria

- [ ] New code file created correctly
- [ ] TODOs found and addressed
- [ ] Documentation added appropriately
- [ ] Coder persona atoms visible in logs

**How to know it's done:** Coder generates valid, compilable code

---

## Phase 7: Domain Expert - Tester

**Location:** `internal/prompt/atoms/identity/tester.yaml`
**Subsystems:** TesterPersona, TestGeneration, TDDLoop, Coverage

### Tests to Run

- [ ] Test generation: generate unit tests for a package
- [ ] Test execution: run tests and report results
- [ ] TDD workflow: write tests, implement if needed
- [ ] Coverage report: check test coverage

### Verification Criteria

- [ ] Test files created with valid Go test syntax
- [ ] Tests execute and report pass/fail
- [ ] TDD loop produces working code
- [ ] Coverage percentage reported

**How to know it's done:** Tests generated are valid and executable

---

## Phase 8: Domain Expert - Reviewer

**Location:** `internal/prompt/atoms/identity/reviewer.yaml`
**Subsystems:** ReviewerPersona, CodeReview, SecurityScan, QualityAnalysis

### Tests to Run

- [ ] Code review: review a specific file for quality issues
- [ ] Security scan: scan for potential vulnerabilities
- [ ] Full codebase review: comprehensive review
- [ ] Verify findings are structured in logs

### Verification Criteria

- [ ] Review produces actionable findings
- [ ] Security issues identified (if any exist)
- [ ] Findings structured with file/line references
- [ ] No false positives on clean code

**How to know it's done:** Reviewer produces structured, actionable feedback

---

## Phase 9: Domain Expert - Researcher

**Location:** `internal/prompt/atoms/identity/researcher.yaml`
**Subsystems:** ResearcherPersona, WebFetch, Context7, KnowledgeExtraction

### Tests to Run

- [ ] Documentation research: research best practices
- [ ] Context7 fetch: find library documentation
- [ ] Codebase exploration: explain project structure
- [ ] Knowledge extraction: summarize architectural decisions

### Verification Criteria

- [ ] Research produces relevant information
- [ ] Context7 fetches documentation (if available)
- [ ] Structure explanation is accurate
- [ ] Knowledge atoms extracted

**How to know it's done:** Researcher gathers and synthesizes information correctly

---

## Agent Architecture Overview

codeNERD has **two tiers** of agents with **271+ prompt atom files**:

### Tier 1: Embedded Permanent Agents (in binary)

Located in `internal/prompt/atoms/` - compiled into binary, always available.

| Category | Agents |
|----------|--------|
| **Domain Experts** (`identity/`) | coder, tester, reviewer, researcher, nemesis, panic_maker, tool_generator, librarian |
| **Campaign** (`campaign/`) | analysis, assault, extractor, librarian, planner, replanning, taxonomist |
| **System** (`system/`) | autopoiesis, executive, legislator, perception, router, world_model |
| **Ouroboros** (`ouroboros/`) | specification, detection, refinement, safety_check, simulation_deployment |
| **Framework** (`framework/`) | bubbletea, bubbles, lipgloss, glamour, cobra, rod, react, django, gin, genai |

### Tier 2: Project-Specific Agents (runtime)

Located in `.nerd/agents/` → synced to `.nerd/shards/{name}_knowledge.db`

These are whatever the project defines - test dynamically, do NOT hardcode names.

---

## Phase 9A: Embedded Agent Verification

**Location:** `internal/prompt/atoms/`
**Subsystems:** Embedded Corpus, JIT Compiler, Atom Loader

### Tests to Run

**Corpus Integrity:**
- [ ] All 271+ YAML files parse without error
- [ ] Required fields present: id, category, priority, content
- [ ] No duplicate atom IDs across files
- [ ] Mandatory atoms (`is_mandatory: true`) exist for each identity

**Domain Expert Loading:**
- [ ] Spawn coder → coder.yaml atoms loaded
- [ ] Spawn tester → tester.yaml atoms loaded
- [ ] Spawn reviewer → reviewer.yaml atoms loaded
- [ ] Spawn researcher → researcher.yaml atoms loaded
- [ ] JIT logs show correct identity atoms

**Adversarial Agent Loading:**
- [ ] Spawn nemesis → nemesis.yaml atoms loaded
- [ ] Spawn panic_maker → panic_maker.yaml atoms loaded
- [ ] Adversarial constraints active

**System Shard Loading:**
- [ ] Legislator atoms load during rule generation
- [ ] Perception atoms load during input filtering
- [ ] Executive atoms load during decision making
- [ ] World model atoms load during state queries

### Verification Criteria

- [ ] All YAML files valid
- [ ] Each agent type loads its specific atoms
- [ ] No cross-contamination between agent contexts
- [ ] Mandatory atoms always present in compiled prompts

**How to know it's done:** All embedded agents load their atoms correctly

---

## Phase 9B: Campaign Sub-Agent Verification

**Location:** `internal/prompt/atoms/campaign/`, `internal/campaign/`
**Subsystems:** Campaign Orchestrator, Sub-Agent Spawner

### Tests to Run

**Campaign Agent Spawning:**
- [ ] Start campaign → planner atoms loaded for decomposition
- [ ] Decomposition phase → extractor atoms loaded
- [ ] Execution phase → appropriate executor atoms loaded
- [ ] Failure → replanning atoms loaded

**Sub-Agent Coordination:**
- [ ] Librarian manages campaign context across phases
- [ ] Taxonomist categorizes tasks correctly
- [ ] Analysis agent assesses campaign progress
- [ ] Each sub-agent has isolated context

**Campaign Phases:**
- [ ] Planning phase uses planner + taxonomist
- [ ] Research phase uses researcher + librarian
- [ ] Implementation phase uses coder + tester
- [ ] Review phase uses reviewer + nemesis

### Verification Criteria

- [ ] Each campaign phase loads correct sub-agents
- [ ] Sub-agents receive phase-appropriate context
- [ ] Handoff between phases preserves relevant state
- [ ] No atom loading errors during phase transitions

**How to know it's done:** Campaigns orchestrate correct sub-agents per phase

---

## Phase 9C: Ouroboros Sub-Agent Verification

**Location:** `internal/prompt/atoms/ouroboros/`, `internal/autopoiesis/`
**Subsystems:** Ouroboros Loop, Tool Generator, Safety Checker

### Tests to Run

**Tool Generation Pipeline:**
- [ ] Gap detection → detection atoms loaded
- [ ] Specification → specification atoms loaded
- [ ] Generation → tool_generator atoms loaded
- [ ] Safety check → safety_check atoms loaded
- [ ] Refinement → refinement atoms loaded
- [ ] Deployment → simulation_deployment atoms loaded

**Safety Constraints:**
- [ ] Adversarial warning atoms active
- [ ] Safety check blocks dangerous tools
- [ ] Refinement improves rejected tools
- [ ] Simulation runs before live deployment

**End-to-End:**
- [ ] Generate tool from scratch
- [ ] Tool passes safety check
- [ ] Tool executes correctly
- [ ] Learning captured for future

### Verification Criteria

- [ ] All Ouroboros phases load correct atoms
- [ ] Safety check never bypassed
- [ ] Generated tools are functional
- [ ] Pipeline completes without panic

**How to know it's done:** Ouroboros generates safe, working tools

---

## Phase 9D: Framework Atom Verification

**Location:** `internal/prompt/atoms/framework/`
**Subsystems:** JIT Compiler, Context Selector

### Tests to Run

**Context-Aware Loading:**
- [ ] Working on .go file → Go framework atoms available
- [ ] Using bubbletea imports → bubbletea atoms loaded
- [ ] Using cobra imports → cobra atoms loaded
- [ ] Using rod imports → rod atoms loaded
- [ ] React/TS file → react atoms loaded

**Framework Expertise:**
- [ ] Bubbletea task → MVU pattern guidance present
- [ ] Cobra task → command structure guidance present
- [ ] Rod task → browser automation patterns present
- [ ] React task → component patterns present

**No Irrelevant Loading:**
- [ ] Python task → no Go framework atoms
- [ ] Backend task → no React atoms (unless cross-platform)
- [ ] CLI task → no Django atoms

### Verification Criteria

- [ ] Framework atoms load based on detected context
- [ ] Correct patterns available for each framework
- [ ] No framework atom overload (token budget respected)
- [ ] Context detection accurate

**How to know it's done:** Framework-specific guidance loads contextually

---

## Phase 9E: Project-Specific Agent Infrastructure

**Location:** `.nerd/agents/`, `.nerd/shards/`
**Subsystems:** AgentRegistry, PromptSync, KnowledgeDB

### Architecture

```
.nerd/agents/{name}/prompts.yaml  -->  .nerd/shards/{name}_knowledge.db
```

### Tests to Run (Dynamic - Do NOT Hardcode Agent Names)

**Agent Discovery:**
- [ ] List all agent directories in `.nerd/agents/`
- [ ] Each agent directory contains `prompts.yaml`
- [ ] Agent registry (`agents.json`) lists discovered agents

**Prompt Sync:**
- [ ] Run `/init` or TUI boot to trigger sync
- [ ] For each agent, `{name}_knowledge.db` exists in `.nerd/shards/`
- [ ] Knowledge DB contains `prompt_atoms` table
- [ ] Atoms from prompts.yaml appear in DB

**Knowledge DB Integrity:**
- [ ] Each knowledge DB is valid SQLite
- [ ] `prompt_atoms` table has required columns
- [ ] No orphaned DBs without agent directory

### Verification Criteria

- [ ] Agent count matches: agents/ dirs == registry count
- [ ] All prompts.yaml are valid YAML
- [ ] All knowledge DBs valid SQLite with correct schema
- [ ] Sync completes without errors

**How to know it's done:** All project agents discovered, synced, DBs valid

---

## Phase 9F: Project Agent Spawning & Learning

**Location:** `internal/session/spawner.go`, `internal/store/learning.go`
**Subsystems:** Spawner, JIT Compiler, Learning Store

### Tests to Run (Dynamic - Whatever Agents Exist)

**Dynamic Agent Spawning:**
- [ ] List available agents from registry
- [ ] Spawn each with domain-appropriate task
- [ ] Verify agent receives knowledge atoms
- [ ] Agent completes task using domain knowledge

**Knowledge Injection:**
- [ ] JIT logs show atoms from agent's knowledge DB
- [ ] Mandatory atoms always included
- [ ] Domain constraints active
- [ ] Correct tool access for domain

**Learning Capture:**
- [ ] Agent executes task with feedback
- [ ] Learning written to `{agent}_learnings.db`
- [ ] Learning includes context

**Learning Retrieval:**
- [ ] On next spawn, learnings loaded
- [ ] JIT includes relevant learnings
- [ ] Agent behavior reflects learnings

**Persistence:**
- [ ] Exit and restart
- [ ] Agents retain knowledge DBs
- [ ] Learnings persist and reload

### Verification Criteria

- [ ] All registered agents spawn successfully
- [ ] Each agent's JIT includes its knowledge
- [ ] Agents perform domain tasks
- [ ] Learnings persist across sessions

**How to know it's done:** Project agents spawn, learn, and remember

---

## Phase 10: Northstar Guardian

**Location:** `internal/northstar/`
**Subsystems:** Guardian, Store, CampaignObserver, TaskObserver, AlignmentCheck

### Tests to Run

- [ ] Northstar initialization: check status
- [ ] Vision definition: set or verify vision exists
- [ ] On-demand alignment check: check feature alignment
- [ ] Verify alignment result format in logs
- [ ] Verify Northstar DB exists

### Verification Criteria

- [ ] Northstar status returns without error
- [ ] Vision is defined and persisted
- [ ] Alignment check returns score and explanation
- [ ] `.nerd/northstar_knowledge.db` exists

**How to know it's done:** Alignment checks pass, DB persists vision

---

## Phase 11: Campaign Orchestrator

**Location:** `internal/campaign/orchestrator*.go`
**Subsystems:** Orchestrator, PhaseManager, TaskDispatcher, Checkpoint

### Tests to Run

- [ ] Simple campaign: single-step task
- [ ] Campaign status: check current campaign state
- [ ] Multi-phase campaign: task requiring multiple steps
- [ ] Check campaign logs for phase/task/checkpoint events

### Verification Criteria

- [ ] Simple campaign completes
- [ ] Status shows phase progression
- [ ] Multi-phase decomposes correctly
- [ ] Checkpoints logged

**How to know it's done:** Campaigns decompose and execute through phases

---

## Phase 12: Campaign Decomposer

**Location:** `internal/campaign/decomposer.go`
**Subsystems:** GoalDecomposer, TaskPlanner, DependencyAnalyzer

### Tests to Run

- [ ] Goal decomposition: decompose complex feature
- [ ] Task dependency analysis: plan with dependencies
- [ ] Check decomposition structure in logs

### Verification Criteria

- [ ] Complex goal breaks into smaller tasks
- [ ] Dependencies identified between tasks
- [ ] Task ordering respects dependencies

**How to know it's done:** Decomposer creates valid task DAG

---

## Phase 13: Requirements Interrogator

**Location:** `internal/shards/requirements_interrogator.go`
**Subsystems:** RequirementsInterrogatorShard, ClarifyingQuestions, AmbiguityDetection

### Tests to Run

- [ ] Ambiguous task clarification: vague input like "make it better"
- [ ] Complex requirement analysis: input with multiple interpretations
- [ ] Clear requirement: specific input requiring no questions
- [ ] Check interrogation logs for clarify/question patterns

### Verification Criteria

- [ ] Ambiguous input triggers clarifying questions
- [ ] Complex requirements get broken down
- [ ] Clear requirements proceed without excessive questions
- [ ] Questions are relevant and helpful

**How to know it's done:** Interrogator asks appropriate questions only when needed

**If Not Wired:** Add registration in `internal/shards/registration.go`, update CLAUDE.md

---

## Phase 14: Context Compression & Pruning

**Location:** `internal/context/compressor.go`, `internal/context/serializer.go`
**Subsystems:** SemanticCompressor, TokenReducer, HistoryCondenser, ContextPruner

**CRITICAL TEST:** Verify correct pruning of UNRELATED context while keeping RELEVANT context.

### Tests to Run

- [ ] Compression test: long conversation simulation
- [ ] Memory check: monitor memory during long operation (should stay bounded)
- [ ] Verify compression events in logs
- [ ] No OOM errors in logs
- [ ] Backend focus test: ask about Go, verify low Android context loading
- [ ] Android focus test: ask about Kotlin, verify low backend context loading
- [ ] Cross-reference test: verify selective loading
- [ ] Token budget enforcement: check budget stats

### Verification Criteria

- [ ] Memory stays bounded (no OOM)
- [ ] Compression log events present
- [ ] Backend queries don't load heavy Android context
- [ ] Android queries don't load heavy backend context
- [ ] Token budget enforced
- [ ] Pruning decisions logged

**How to know it's done:** Context properly isolates by relevance, memory bounded

---

## Phase 15: Spreading Activation

**Location:** `internal/context/activation.go`
**Subsystems:** ActivationEngine, FactSelector, RelevanceScoring

### Tests to Run

- [ ] Activation trace: trace for specific query
- [ ] Fact relevance scoring: score query relevance
- [ ] Context-directed spreading: find related files
- [ ] Verify activation logs for spread/relevance events

### Verification Criteria

- [ ] Trace shows activation path through facts
- [ ] Scoring produces numeric relevance
- [ ] Related files found via spreading
- [ ] Activation events logged

**How to know it's done:** Facts selected by relevance, not just recency

---

## Phase 16: Sparse Retrieval

**Location:** `internal/retrieval/`
**Subsystems:** SparseRetriever, BM25, InvertedIndex

### Tests to Run

- [ ] Keyword search: search for specific terms
- [ ] File retrieval: find files by content
- [ ] Symbol retrieval: find symbols by name
- [ ] Hybrid retrieval: sparse + vector combined

### Verification Criteria

- [ ] Keyword search returns relevant results
- [ ] Files with matching content found
- [ ] Symbols located across codebase
- [ ] Hybrid produces better results than either alone

**How to know it's done:** All retrieval modes return relevant results

---

## Phase 17: Code DOM - Single File Edits

**Location:** `internal/tools/codedom/`
**Subsystems:** ElementGetter, LineEditor, ASTParser

### Tests to Run

- [ ] Get elements from file: extract functions, types
- [ ] Edit specific lines: add comment to file
- [ ] AST-aware modification: add new function
- [ ] Verify edits are atomic: check git diff

### Verification Criteria

- [ ] Elements extracted with correct types
- [ ] Line edits apply cleanly
- [ ] AST modifications preserve syntax
- [ ] Single file changes appear in diff

**How to know it's done:** Single-file edits work correctly

---

## Phase 18: Code DOM - Multi-File & Cross-Platform Edits

**Location:** `internal/tools/codedom/`, `internal/core/virtual_store_codedom.go`
**Subsystems:** BatchEditor, TransactionManager, RollbackHandler, FlowEditor

**CRITICAL TEST:** Edit data flow from Android → Backend → Frontend simultaneously.

### Tests to Run

- [ ] Multi-file same platform: create package with two files
- [ ] Coordinated refactoring: rename across all backend files
- [ ] Transaction rollback: edit that fails should rollback
- [ ] Cross-platform flow edit: create matching data model in Go, TypeScript, Kotlin
- [ ] Verify all three platform files created
- [ ] API layer flow edit: handler + client in all platforms
- [ ] Verify API flow files created

### Verification Criteria

- [ ] Single-platform multi-file edits work
- [ ] Cross-platform model creation works atomically
- [ ] API layer flow (handler + clients) created atomically
- [ ] Transaction rollback works on failure
- [ ] Git diff shows coordinated changes

**How to know it's done:** Multi-file and cross-platform edits execute atomically

---

## Phase 19: World System - Holographic View & Cartographer

**Location:** `internal/world/holographic.go`, `internal/world/cartographer.go`, `internal/world/code_elements.go`
**Subsystems:** HolographicView, ImpactAnalysis, ChangeGraph, Cartographer, CodeElementExtractor

### Tests to Run

- [ ] Impact analysis (single file): what would be affected if file changes
- [ ] Change graph: generate graph for file
- [ ] Cross-platform impact: Go model change → Android/Frontend impact
- [ ] Cartographer code element mapping: map directories
- [ ] Code elements extraction: extract functions, classes by type
- [ ] Holographic context building: natural language impact query
- [ ] Cross-reference impact: find all callers of endpoint
- [ ] Check holographic logs

### Verification Criteria

- [ ] Impact analysis produces dependency graph
- [ ] Cross-platform impact detected (Go→TS, Go→Kotlin)
- [ ] Cartographer maps code elements
- [ ] CodeElements extracts functions, classes, types
- [ ] Natural language queries return impact info

**How to know it's done:** Impact analysis traces across platforms

---

## Phase 20: World System - Dataflow & Taint Analysis

**Location:** `internal/world/dataflow.go`, `internal/world/dataflow_multilang.go`, `internal/world/dataflow_cache.go`
**Subsystems:** TaintAnalysis, DataflowTracker, SinkDetector, MultiLangDataflow, DataflowCache

**CRITICAL TEST:** Trace data flow across platform boundaries (Android → Backend → Database).

### Tests to Run

- [ ] Single-language dataflow: Go backend analysis
- [ ] Multi-language dataflow: cross-language trace
- [ ] Taint tracking: track user input through languages
- [ ] Cross-platform data flow: Android → Retrofit → Go → PostgreSQL
- [ ] Security sink detection: SQL, command, file sinks
- [ ] Dataflow cache: second run should be faster
- [ ] API boundary dataflow: frontend/backend boundary
- [ ] Check dataflow logs

### Verification Criteria

- [ ] Single-language dataflow works
- [ ] Multi-language dataflow tracks across Go/Kotlin/TS
- [ ] Cross-platform flow (Android→Backend→DB) traced
- [ ] Security sinks detected
- [ ] Dataflow cache improves repeat query performance

**How to know it's done:** Data flow traced across all platform boundaries

---

## Phase 21: World System - AST, Scope & Scanning

**Location:** `internal/world/ast.go`, `internal/world/ast_treesitter.go`, `internal/world/scope.go`, `internal/world/deep_scan.go`, `internal/world/incremental_scan.go`, `internal/world/git_scanner.go`
**Subsystems:** ASTProjector, TreeSitter, SymbolGraph, ScopeAnalyzer, DeepScanner, IncrementalScanner, GitScanner

### Tests to Run

- [ ] AST projection (Go): parse Go file
- [ ] Tree-sitter multi-language: parse Kotlin and TSX
- [ ] Symbol graph: generate for file and project
- [ ] Scope analysis: analyze package and function scopes
- [ ] Dependency extraction: internal and external deps
- [ ] Deep scan: full codebase analysis
- [ ] Incremental scan: detect changes efficiently
- [ ] Git scanner: status, changes, commit impact
- [ ] World model query: query file_topology and symbol_graph facts
- [ ] Check AST/world logs

### Verification Criteria

- [ ] Go AST projection works
- [ ] Tree-sitter parses Kotlin and TSX
- [ ] Symbol graph generated
- [ ] Scope analysis completes
- [ ] Deep scan analyzes full codebase
- [ ] Incremental scan detects changes efficiently
- [ ] Git scanner provides commit/change info
- [ ] World model queries return valid Mangle facts

**How to know it's done:** All language ASTs parse, world model populated

---

## Phase 22: LSP Integration (Mangle + World)

**Location:** `internal/mangle/lsp.go`, `internal/world/lsp/manager.go`
**Subsystems:** MangleLSP, DiagnosticsProvider, CompletionProvider, WorldLSPManager

### Tests to Run

- [ ] LSP server availability check
- [ ] Mangle file diagnostics (if .mg files exist)
- [ ] Schema validation: copy and validate codeNERD schemas
- [ ] World LSP Manager: hover, definition, references
- [ ] LSP diagnostics on Go code
- [ ] Check LSP logs

### Verification Criteria

- [ ] Mangle LSP checks schemas
- [ ] World LSP Manager provides code intelligence
- [ ] Hover, definition, references work
- [ ] Diagnostics produced for Go code

**How to know it's done:** LSP provides code intelligence for all languages

---

## Phase 23: MCP Discovery

**Location:** `internal/mcp/`
**Subsystems:** MCPClientManager, ServerDiscovery, ToolAnalyzer

### Tests to Run

- [ ] MCP server discovery
- [ ] List available MCP servers
- [ ] MCP tool analysis
- [ ] Search for MCP servers (if network available)

### Verification Criteria

- [ ] Discovery finds available servers
- [ ] Server list shows capabilities
- [ ] Tool analysis extracts metadata
- [ ] Search returns relevant servers

**How to know it's done:** MCP servers discovered and analyzed

---

## Phase 24: MCP Execution

**Location:** `internal/mcp/client.go`, `internal/mcp/compiler.go`
**Subsystems:** MCPClient, JITToolCompiler, ToolRenderer

### Tests to Run

- [ ] Install an MCP server (filesystem or standard)
- [ ] Execute MCP tool
- [ ] JIT tool compilation for MCP
- [ ] Three-tier rendering: full, condensed, minimal

### Verification Criteria

- [ ] Server installs successfully
- [ ] Tool execution returns results
- [ ] JIT compiles appropriate tools for task
- [ ] All three render modes work

**How to know it's done:** MCP tools execute correctly

---

## Phase 25: Glass Box Visibility

**Location:** `cmd/nerd/chat/glass_box.go`
**Subsystems:** GlassBox, ToolTracer, ExecutionVisualizer

### Tests to Run

- [ ] Enable glass box mode for command
- [ ] Tool execution visibility with verbose flag
- [ ] Check glass box output in logs

### Verification Criteria

- [ ] Glass box shows which tools invoked
- [ ] Parameters displayed in real-time
- [ ] Results visible as they arrive

**How to know it's done:** Tool execution visible in real-time

---

## Phase 26: Shadow Mode

**Location:** `internal/core/shadow_mode.go`
**Subsystems:** ShadowExecutor, ProposalTracker, DiffGenerator

### Tests to Run

- [ ] Shadow mode execution: proposed changes without applying
- [ ] Proposal review: list pending proposals
- [ ] Apply shadow proposal
- [ ] Check shadow logs

### Verification Criteria

- [ ] Shadow execution proposes without changing files
- [ ] Proposals listed with diffs
- [ ] Apply works when approved
- [ ] Proposal state tracked

**How to know it's done:** Shadow mode previews changes before applying

---

## Phase 27: Usage Tracking

**Location:** `internal/usage/`
**Subsystems:** UsageTracker, TokenCounter, CostEstimator

### Tests to Run

- [ ] Check usage stats
- [ ] Token usage report
- [ ] Session history
- [ ] Cost estimate

### Verification Criteria

- [ ] Stats show token consumption
- [ ] Sessions tracked
- [ ] Costs estimated (if enabled)

**How to know it's done:** Usage tracked across sessions

---

## Phase 28: Verification System

**Location:** `internal/verification/`
**Subsystems:** CodeVerifier, TestRunner, BuildChecker

### Tests to Run

- [ ] Build verification
- [ ] Test verification
- [ ] Lint verification
- [ ] Full verification suite

### Verification Criteria

- [ ] Build check reports status
- [ ] Tests run and report
- [ ] Lint issues identified
- [ ] Full suite completes

**How to know it's done:** Verification catches issues before commit

---

## Phase 29: Context Harness (Infinite Context Validation)

**Location:** `internal/testing/context_harness/`
**Subsystems:** ContextHarness, SessionSimulator, MetricsCollector, ActivationTracer, CompressionViz, JitTracer, Inspector

### Tests to Run

- [ ] Quick scenario: baseline validation
- [ ] Debugging marathon: long-term context retention (50 turns)
- [ ] Feature implementation: multi-phase context paging (75 turns)
- [ ] Refactoring campaign: cross-file tracking (100 turns)
- [ ] Metrics collection: compression/retrieval stats
- [ ] Monorepo platform isolation: verify context separation
- [ ] Activation tracer: fact graph spreading
- [ ] JIT tracer: prompt compilation stats
- [ ] Compression visualization: semantic compression stats
- [ ] Inspector deep checks: context analysis and dump
- [ ] Full suite: all scenarios with JSON output

### Expected Results

- [ ] Quick scenario: PASS
- [ ] Compression ratio: >50% reduction
- [ ] Retrieval accuracy: >90%
- [ ] Activation spread: Facts selected by relevance
- [ ] JIT compilation: Persona atoms loaded correctly
- [ ] Monorepo isolation: Each platform gets focused context
- [ ] No OOM or memory explosions

**How to know it's done:** All scenarios pass, metrics within targets

---

## Phase 30: Full Integration Sweep & Log Analysis

**Location:** `internal/logging/`, `internal/tactile/`, `.claude/skills/log-analyzer/`
**Subsystems:** Logger, LogAnalyzer, Tactile OutputAnalyzer, Mangle Log Query

**SKILL INVOCATION:** Use `/log-analyzer` for advanced debugging.

### Tests to Run

- [ ] Generate activity for logging
- [ ] Basic error scan (grep for error patterns)
- [ ] Clean log check by all 22 categories
- [ ] Log analyzer Mangle query
- [ ] Tactile output analysis
- [ ] Docker execution (if available)
- [ ] Test/build output analysis
- [ ] Cross-system log correlation
- [ ] Performance log analysis
- [ ] Session log inspection
- [ ] Final log summary

### Verification Criteria

- [ ] ALL 22 log categories show 0 errors
- [ ] Log analyzer skill queries execute
- [ ] Tactile output analyzer parses test/build output
- [ ] Docker execution captures logs (if available)
- [ ] No panics, deadlocks, or nil pointers in any log
- [ ] Cross-system log correlation works

**How to know it's done:** Zero errors across all log categories

---

## Phase 31: Test Bed Validation (tribalFitness)

**The Real Proof:** Use codeNERD to enhance tribalFitness.

### Tests to Run

- [ ] Build the tribalFitness project
- [ ] Use codeNERD to add a /api/health endpoint with tests
- [ ] Verify the addition compiles
- [ ] Run tests for new endpoint
- [ ] Briefly run the app and hit the health endpoint

### Verification Criteria

- [ ] Project builds before changes
- [ ] codeNERD adds endpoint correctly
- [ ] Tests pass for new endpoint
- [ ] Health endpoint returns expected JSON

**How to know it's done:** codeNERD autonomously adds working feature

---

## Phase 32: Ouroboros - Self-Generating Tools

**Location:** `internal/autopoiesis/ouroboros.go`
**Subsystems:** OuroborosLoop, ToolGenerator, PanicMaker, SafetyChecker

### Tests to Run

- [ ] Generate a custom analysis tool (e.g., TODO counter)
- [ ] Execute the generated tool
- [ ] Safety validation: attempt to generate dangerous tool (should be blocked)
- [ ] Verify generated tools in .nerd/tools/

### Verification Criteria

- [ ] At least 1 tool generated
- [ ] Generated tool compiles
- [ ] Generated tool executes correctly
- [ ] Dangerous tools blocked by SafetyChecker

**How to know it's done:** Ouroboros generates usable, safe tools

---

## Phase 33: Nemesis - Adversarial Code Review

**Location:** `internal/shards/nemesis/`
**Subsystems:** NemesisShard, AttackVectorGenerator, VulnerabilityDB

### Tests to Run

- [ ] Run Nemesis review on tribalFitness
- [ ] Nemesis self-review on codeNERD component
- [ ] Verify findings in logs

### Verification Criteria

- [ ] Nemesis identifies vulnerabilities (if any)
- [ ] Attack programs generated to prove issues
- [ ] Findings documented with evidence
- [ ] No panics during adversarial review

**How to know it's done:** Nemesis completes review with structured findings

---

## Phase 34: Thunderdome - Adversarial Battle Arena

**Location:** `internal/autopoiesis/thunderdome.go`
**Subsystems:** Thunderdome, AttackExecutor, SandboxManager

### Tests to Run

- [ ] Run attack vectors against target
- [ ] Verify sandbox isolation (git status shows no unexpected changes)
- [ ] Attack catalog listing
- [ ] Verify results in logs (SURVIVED/DEFEATED)

### Verification Criteria

- [ ] Attacks execute
- [ ] Sandbox contains all changes
- [ ] No sandbox escapes
- [ ] Results logged

**How to know it's done:** Attacks run safely in sandbox

---

## Phase 35: Dream State - Hypothetical Exploration

**Location:** `internal/core/dream_router.go`, `internal/core/dream_learning.go`
**Subsystems:** DreamRouter, ConsultantPool, HypothesisGenerator

### Tests to Run

- [ ] Enter dream state with hypothetical scenario
- [ ] Verify isolation (git status shows no changes)
- [ ] Check dream logs for hypothesis generation

### Verification Criteria

- [ ] Dream generates hypotheses
- [ ] No file modifications from dream
- [ ] Insights captured in logs

**How to know it's done:** Dream explores without modifying reality

---

## Phase 36: Prompt Evolution - System Prompt Learning

**Location:** `internal/autopoiesis/prompt_evolution/`
**Subsystems:** Evolver, Judge, FeedbackCollector, AtomGenerator

### Tests to Run

- [ ] Record execution feedback via command
- [ ] Trigger evolution cycle
- [ ] Verify evolved atoms in .nerd/prompts/evolved/
- [ ] Verify YAML validity of evolved atoms

### Verification Criteria

- [ ] Execution feedback recorded
- [ ] Evolution cycle completes
- [ ] Evolved atoms generated (if issues occurred)
- [ ] All atoms valid YAML

**How to know it's done:** Prompt evolution records and learns from feedback

---

## Phase 37: Autopoiesis Integration - The Full Loop

### Tests to Run

Full loop: Generate → Review → Attack → Learn

- [ ] Generate tool
- [ ] Nemesis reviews generated tool
- [ ] Thunderdome attacks tool
- [ ] Prompt evolution records learnings

### Verification Criteria

- [ ] Tool generated
- [ ] Nemesis findings documented
- [ ] Thunderdome battles completed
- [ ] Evolution cycle triggered

**How to know it's done:** Full autopoiesis loop executes end-to-end

---

## Phase 38: CLI Command Audit

### Commands to Verify

**Core commands:**
- [ ] `--help`
- [ ] `version`
- [ ] `init --help`
- [ ] `scan --help`
- [ ] `run --help`

**Session commands:**
- [ ] `sessions --help`
- [ ] `session new --help`

**Campaign commands:**
- [ ] `campaign --help`
- [ ] `campaign start --help`
- [ ] `campaign status --help`

**Spawn commands:**
- [ ] `spawn --help`

**JIT commands:**
- [ ] `jit --help`

**Mangle commands:**
- [ ] `check-mangle --help`
- [ ] `query --help`

**Tool commands:**
- [ ] `tool --help`
- [ ] `tools --help`

**Northstar commands:**
- [ ] `northstar --help`
- [ ] `alignment --help`

**MCP commands:**
- [ ] `mcp --help`

**Advanced commands:**
- [ ] `dream --help`
- [ ] `shadow --help`
- [ ] `thunderdome --help`
- [ ] `prompt --help`

### If Commands Missing

- [ ] Add implementation to `cmd/nerd/cmd_*.go`
- [ ] Register in `cmd/nerd/main.go`
- [ ] Update `cmd/nerd/README.md`
- [ ] Update `cmd/nerd/CLAUDE.md`

**How to know it's done:** All CLI commands respond to --help

---

## Phase 39: Documentation Audit

### CLAUDE.md Audit

- [ ] Find all CLAUDE.md files
- [ ] Check if Go files modified after CLAUDE.md
- [ ] Update outdated CLAUDE.md files

### README.md Audit

- [ ] README.md exists
- [ ] Documents Kernel
- [ ] Documents JIT
- [ ] Documents Campaign
- [ ] Documents Northstar
- [ ] Documents MCP
- [ ] Documents Autopoiesis

### Documentation Checklist

- [ ] All 51+ CLAUDE.md files reflect current code
- [ ] README.md describes all major features
- [ ] File Index tables are accurate
- [ ] Key types documented
- [ ] Usage examples provided

**How to know it's done:** All docs up-to-date with code

---

## Phase 40: Final Verification

### Fresh Build & Deploy

- [ ] Rebuild codeNERD with clean build
- [ ] Build succeeds
- [ ] Deploy to tribalFitness

### Comprehensive Execution

- [ ] Clear logs
- [ ] Run comprehensive verification command
- [ ] Execution completes without panic

### Log Cleanliness

- [ ] Check all log files for errors
- [ ] Total errors = 0

### Domain Experts

- [ ] Coder: operational (events in logs)
- [ ] Tester: operational
- [ ] Reviewer: operational
- [ ] Researcher: operational

### Subsystems

- [ ] Northstar DB exists
- [ ] Code DOM: operational
- [ ] Holographic: operational
- [ ] Dataflow: operational
- [ ] AST: operational
- [ ] MCP: operational
- [ ] JIT: operational

### Autopoiesis

- [ ] Tools generated: >= 1
- [ ] Nemesis findings recorded
- [ ] Thunderdome battles executed
- [ ] Dream events recorded
- [ ] Evolution cycles triggered

### Test Bed

- [ ] tribalFitness builds
- [ ] Tests pass

### Phase Status

- [ ] All phases 1-40 marked complete in state file

### Documentation

- [ ] Documentation updates recorded

**How to know it's done:** All checks pass, ready for Phase 41

---

## Phase 41: THE ULTIMATE TEST - Autonomous Monorepo Campaign

> **This is the final proof.** codeNERD must autonomously research, plan, and implement a cross-platform feature across the tribalFitness monorepo using ALL systems.

### tribalFitness Monorepo Structure

```
tribalFitness/
├── android/        # Kotlin Android app (Jetpack Compose)
├── backend/        # Go API server (Chi router, PostgreSQL)
├── frontend/       # React/TypeScript web app (Vite)
├── Docs/
│   ├── references/         # 7 research categories
│   └── blueprints/features/ # 26 feature categories
```

### Campaign Goal: Guild Challenge System

**Target Feature Block:** `02-guild-social` + `03-competition` + `17-gamification`

**Cross-Platform Implementation Required:**
- Backend (Go): API endpoints for challenges, leaderboards, rewards
- Android (Kotlin): Native challenge UI, real-time updates
- Frontend (React): Web dashboard for guild management

---

### Phase 41.1: Research Ingestion

- [ ] Use Researcher shard to ingest Docs/references/gamification/
- [ ] Use Researcher shard to ingest Docs/references/community-social/
- [ ] Extract knowledge atoms from research
- [ ] Verify research ingestion in logs

**Success Criteria:**
- [ ] Researcher reads reference documents
- [ ] Knowledge atoms extracted and stored
- [ ] Research summary produced

---

### Phase 41.2: Blueprint Analysis

- [ ] Analyze Docs/blueprints/features/02-guild-social/
- [ ] Analyze Docs/blueprints/features/03-competition/
- [ ] Analyze Docs/blueprints/features/17-gamification/
- [ ] Extract requirements with Requirements Interrogator

**Success Criteria:**
- [ ] Blueprints analyzed across categories
- [ ] Requirements extracted and clarified
- [ ] Cross-platform scope identified

---

### Phase 41.3: Northstar Alignment

- [ ] Ensure vision is set for tribalFitness
- [ ] Check alignment of Guild Challenge System feature
- [ ] Verify alignment score >= 0.7

**Success Criteria:**
- [ ] Vision defined
- [ ] Alignment check passes
- [ ] Feature approved for implementation

---

### Phase 41.4: Tool Generation (Ouroboros)

- [ ] Generate monorepo analyzer tool
- [ ] Generate cross-platform code correlator tool
- [ ] Verify tools compile
- [ ] No safety violations

**Success Criteria:**
- [ ] At least 1 tool generated
- [ ] Tools compile without errors
- [ ] No safety violations

---

### Phase 41.5: Campaign Launch - Full Autonomous Implementation

Launch multi-phase campaign implementing across all three platforms:

**Backend (Go) Requirements:**
- [ ] Create internal/challenges/ package
- [ ] Challenge model with required fields
- [ ] ChallengeService with CRUD
- [ ] LeaderboardService for rankings
- [ ] API endpoints for challenges
- [ ] Database migrations

**Frontend (React) Requirements:**
- [ ] Create components in src/components/challenges/
- [ ] ChallengeCard, ChallengeList, LeaderboardView, ChallengeCreator
- [ ] GuildChallengesPage
- [ ] API client in src/api/challenges.ts
- [ ] Routing added

**Android (Kotlin) Requirements:**
- [ ] Create challenges package
- [ ] ChallengeModel, ChallengeRepository, ChallengeViewModel
- [ ] Composable screens: ChallengeListScreen, ChallengeDetailScreen, LeaderboardScreen
- [ ] Navigation added

**Testing Requirements:**
- [ ] Unit tests for backend services
- [ ] Integration tests for API endpoints
- [ ] Component tests for React

**Success Criteria:**
- [ ] Campaign decomposes into multiple phases
- [ ] Backend Go code generated
- [ ] Frontend React components generated
- [ ] Android Kotlin code generated
- [ ] Tests generated for each platform
- [ ] No panics during campaign

---

### Phase 41.6: Code Review (Reviewer Shard)

- [ ] Review Go code for error handling, SQL injection, race conditions
- [ ] Review React code for architecture, type safety, accessibility
- [ ] Review Kotlin code for Compose practices, state management

**Success Criteria:**
- [ ] All three platforms reviewed
- [ ] Findings documented
- [ ] No critical security issues

---

### Phase 41.7: Testing (Tester Shard)

- [ ] Run Go backend tests
- [ ] Run React frontend tests
- [ ] Run Android tests
- [ ] Analyze test results with codeNERD

**Success Criteria:**
- [ ] Backend tests pass
- [ ] Frontend tests run
- [ ] Android tests run
- [ ] Coverage reported

---

### Phase 41.8: Adversarial Testing (Nemesis + Thunderdome)

- [ ] Nemesis analyzes for SQL injection, XSS, data leakage
- [ ] Attack programs generated
- [ ] Thunderdome attacks backend challenges
- [ ] Thunderdome attacks frontend challenges

**Success Criteria:**
- [ ] Nemesis completes analysis
- [ ] Vulnerabilities documented
- [ ] Attacks contained in sandbox
- [ ] No sandbox escapes

---

### Phase 41.9: Dream State Exploration

- [ ] Dream about future enhancements (multiplayer, AR, AI, tournaments)
- [ ] Verify no file modifications from dream

**Success Criteria:**
- [ ] Dream generates hypotheses
- [ ] No file modifications
- [ ] Insights captured

---

### Phase 41.10: Prompt Evolution

- [ ] Record feedback from entire campaign
- [ ] Check evolved atoms created
- [ ] Verify learnings captured

**Success Criteria:**
- [ ] Execution feedback recorded
- [ ] Strategies updated
- [ ] Evolved atoms generated if issues occurred

---

### Phase 41.11: Integration Verification

**Backend Verification:**
- [ ] challenges package created
- [ ] Go files present
- [ ] Backend builds

**Frontend Verification:**
- [ ] challenges components created
- [ ] TSX files present
- [ ] Frontend builds

**Android Verification:**
- [ ] Kotlin files in challenges
- [ ] Android builds

**Cross-Platform Summary:**
- [ ] Backend files count > 0
- [ ] Frontend files count > 0
- [ ] Android files count > 0

**Campaign Execution:**
- [ ] Campaign phases completed
- [ ] Tasks executed
- [ ] Errors = 0

**Systems Exercised:**
- [ ] Researcher events > 0
- [ ] Coder events > 0
- [ ] Tester events > 0
- [ ] Reviewer events > 0
- [ ] Nemesis events > 0
- [ ] Ouroboros events > 0
- [ ] Northstar events > 0
- [ ] Dream events > 0
- [ ] Evolution events > 0

---

### Phase 41 Completion Checklist

- [ ] Research from Docs/references/ ingested
- [ ] Blueprints from Docs/blueprints/features/ analyzed
- [ ] Northstar alignment verified
- [ ] Custom tools generated by Ouroboros
- [ ] Backend Go code implemented in backend/internal/challenges/
- [ ] Frontend React code implemented in frontend/src/components/challenges/
- [ ] Android Kotlin code implemented in android/.../challenges/
- [ ] All platforms build successfully
- [ ] Tests pass on all platforms
- [ ] Reviewer analyzed all generated code
- [ ] Nemesis adversarial review completed
- [ ] Thunderdome attacks executed safely
- [ ] Dream state exploration completed
- [ ] Prompt evolution recorded learnings
- [ ] Zero panics throughout campaign

**If ALL boxes checked, Phase 41 is COMPLETE.**

---

## Root-Cause Investigation Template

When you find a bug, document in `.nerd/ralph/bugs/BUG-XXX.md`:

### Template Structure

1. **Symptom:** What happened
2. **Proximate Cause:** Immediate trigger
3. **Root Cause (Five Whys):**
   - Why 1? → Answer
   - Why 2? → Answer
   - Why 3? → Answer
   - Why 4? → Answer
   - Why 5? → ROOT CAUSE
4. **Systemic Fix:** Code change to prevent forever
5. **Files Changed:** List with descriptions
6. **Documentation Updates Required:** CLAUDE.md and README.md
7. **Verification:** How to verify fix works

**IMPORTANT:** Every fix MUST include:
- [ ] CLAUDE.md update in affected package(s)
- [ ] README.md update if user-facing behavior changed
- [ ] Entry in `docs_updated` array in state file

---

## Anti-Patterns (FORBIDDEN)

You MUST NOT:
- [ ] Comment out broken code
- [ ] Delete corrupted artifacts
- [ ] Add nil checks without tracing nil source
- [ ] Wrap in recover() to hide panics
- [ ] Increase timeouts to hide slowness
- [ ] Add special cases for specific failures
- [ ] Disable features that stress test broke
- [ ] Use `// TODO: fix later` comments
- [ ] Mark bugs as fixed without verification
- [ ] Skip documentation updates after fixes
- [ ] Leave CLAUDE.md files outdated after refactors

---

## Iteration Strategy

Each Ralph iteration:

1. [ ] Read state file - know current phase
2. [ ] Check binaries - recompile if source changed
3. [ ] Deploy to test bed - copy if rebuilt
4. [ ] Run next test - based on current phase
5. [ ] If pass - mark complete, move to next
6. [ ] If fail - apply root-cause protocol, fix, verify
7. [ ] Update docs - CLAUDE.md and README.md
8. [ ] Update state - increment iteration, update timestamps
9. [ ] Check completion - all phases done + logs clean?
10. [ ] Output promise - ONLY if truly complete

---

## Binary Recompilation Checklist

After ANY code change in codeNERD:

1. [ ] Navigate to codeNERD source
2. [ ] Run go vet with CGO_CFLAGS
3. [ ] Remove old nerd.exe
4. [ ] Build with sqlite_vec tag
5. [ ] Copy to tribalFitness
6. [ ] Verify with --version

---

## Files to Monitor

| File | Purpose |
|------|---------|
| `.nerd/ralph/perfection_state.json` | Progress tracking |
| `.nerd/ralph/bugs/` | Bug documentation |
| `.nerd/logs/*` | All 22 log categories |
| `.nerd/tools/` | Ouroboros-generated tools |
| `.nerd/nemesis/attacks/` | Nemesis attack programs |
| `.nerd/prompts/evolution.db` | Prompt evolution feedback |
| `.nerd/prompts/strategies.db` | Learning strategy database |
| `.nerd/prompts/evolved/` | Evolved prompt atoms |
| `.nerd/northstar_knowledge.db` | Northstar vision DB |
| `internal/prompt/atoms/` | Core persona atoms |
| `internal/mangle/*.mg` | Mangle schemas and policies |
| `**/CLAUDE.md` | Package documentation |
| `README.md` | Project documentation |

---

## Expected Duration

| Phases | Duration | Focus |
|--------|----------|-------|
| 0 | 15 min | Setup, build, deploy |
| 1-5 | 2-3 hours | Core stability |
| 6-9 | 2-3 hours | Domain experts |
| 10-13 | 1-2 hours | Northstar, campaigns |
| 14-16 | 1-2 hours | Context systems |
| 17-18 | 1-2 hours | Code DOM |
| 19-22 | 2-3 hours | World systems |
| 23-24 | 1-2 hours | MCP integration |
| 25-29 | 2-3 hours | Visibility and testing |
| 30-31 | 1-2 hours | Integration and test bed |
| 32-37 | 3-4 hours | Autopoiesis systems |
| 38-40 | 2-3 hours | CLI, docs, final verification |
| **41** | **4-8 hours** | **Autonomous monorepo campaign** |

**Total: 22-38 hours** of Ralph iterations.

---

## Remember

> "The artifact is NOT the bug - it is a SYMPTOM of a deeper systemic failure."
>
> "Deleting, commenting out, or patching the artifact is strictly forbidden."
>
> "Always trace back to the EARLIEST point where the bug could have been prevented."
>
> "Every fix MUST update documentation - CLAUDE.md files are living specifications."
>
> "Test in tribalFitness, fix in codeNERD, redeploy, verify."

You are building a coding agent that can create complex applications autonomously.
Every root-cause fix makes codeNERD stronger.
Every band-aid makes it weaker.
Every undocumented change creates technical debt.

**Choose strength. Choose clarity.**
