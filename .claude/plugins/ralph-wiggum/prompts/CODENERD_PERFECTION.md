# codeNERD Perfection Loop

> **Ralph Wiggum Prompt for Achieving 100% Tested Reliability**
>
> This prompt is fed iteratively. Each cycle, you see your previous work in files and git.
> Progress is tracked in `.nerd/ralph/perfection_state.json`.
> All fixes go through root-cause analysis - NO BAND-AIDS.
> Documentation updates (README.md, CLAUDE.md) are REQUIRED for every fix.

## Test Bed: tribalFitness

**IMPORTANT:** All testing runs in the tribalFitness project workspace, NOT the codeNERD workspace.

```
Test Bed:     C:\CodeProjects\tribalFitness
Binary:       C:\CodeProjects\tribalFitness\nerd.exe
codeNERD Src: C:\CodeProjects\codeNERD
```

---

## Available Skills

**IMPORTANT:** Use these skills proactively during perfection loop execution to be maximally effective.

| Skill | When to Use |
|-------|-------------|
| **go-architect** | ALWAYS use when writing, reviewing, or refactoring Go code. Ensures idiomatic patterns, proper error handling, concurrency safety. Invoked automatically for any Go file changes. |
| **mangle-programming** | Use when writing/debugging Mangle rules, understanding stratification, fixing safety violations, or working with aggregation syntax. Essential for Phases 1-2, 22. |
| **codenerd-builder** | Use when implementing codeNERD components: kernel, transducers, JIT, SubAgents, Mangle integration. The master reference for architecture decisions. |
| **integration-auditor** | Use when debugging "code exists but doesn't run" issues, verifying system wiring, creating SubAgents, or auditing cross-system integration. Critical for Phases 30-31. |
| **stress-tester** | Use for live stress testing, finding panics, edge cases, and failure modes. Contains 35 workflows across severity levels. Key for Phases 32-37. |
| **log-analyzer** | Use when debugging codeNERD execution via log analysis. Converts logs to Mangle facts for querying. Essential for Phase 30 and any failure investigation. |
| **prompt-architect** | Use when writing persona atoms, auditing prompts, debugging LLM behavior, or optimizing context injection. Critical for Phases 3-9, 36. |
| **charm-tui** | Use when building/debugging TUI components with Bubbletea and Lipgloss. Covers MVU pattern, stability patterns, goroutine safety. |
| **rod-builder** | Use when implementing browser automation with Rod. Covers CDP events, session management, Chromium configuration. |
| **research-builder** | Use when implementing ResearcherShard functionality, knowledge atom extraction, or documentation gathering. Key for Phase 9. |
| **cli-engine-integration** | Use when integrating Claude CLI or Codex CLI as LLM backends. |
| **skill-creator** | Use when creating new skills or updating existing ones. |

### Skill Invocation

Invoke skills explicitly when entering relevant phases:

```
/go-architect    # Before any Go code changes
/mangle-programming  # Before Mangle rule modifications
/stress-tester   # Before running stress tests
/log-analyzer    # When investigating failures
```

---

## Completion Promise

Output this ONLY when ALL conditions are TRUE:

```
<promise>CODENERD PERFECTION ACHIEVED</promise>
```

**Conditions for promise (ALL must be true):**
1. All 50+ subsystems pass comprehensive stress tests with zero panics
2. All 22 log categories show clean output (no ERROR, PANIC, FATAL, undefined, nil pointer)
3. The tribalFitness app builds and passes all tests using codeNERD autonomously
4. Domain experts (coder, tester, reviewer, researcher) complete their tasks correctly
5. Northstar vision alignment checks pass
6. Code DOM multi-file edits execute correctly
7. Context compression maintains memory bounds
8. Spreading activation selects relevant facts
9. World systems (holographic, dataflow) analyze correctly
10. MCP integrations discover and install tools
11. JIT prompt compilation works for all personas
12. Ouroboros successfully generates and executes a custom tool
13. Nemesis adversarial review completes without triggering panics
14. Thunderdome attack vectors execute in sandbox without escape
15. Dream State consultation produces valid hypothetical exploration
16. Prompt Evolution records feedback and generates improvement atoms
17. Glass Box visibility shows tool execution
18. Shadow system tracks proposal states
19. All CLI commands verified functional
20. Root-cause fixes committed for every failure encountered
21. All CLAUDE.md files updated for affected packages
22. `.nerd/ralph/perfection_state.json` shows all phases complete

---

## Self-Enhancement Protocol: Adding Logging & Tests

> **MISSION CRITICAL:** You have FULL PERMISSION to add logging, test files, and instrumentation
> as needed to accomplish the perfection mission. Don't just test what exists - enhance the
> codebase to make it testable.

### When to Add Logging

Add logging statements when you encounter:

1. **Silent failures** - Code paths that fail without any log output
2. **Missing visibility** - Operations where you can't tell what happened
3. **Cross-system boundaries** - Entry/exit points between packages
4. **Error paths** - All error returns should log context
5. **State transitions** - Campaign phases, shard lifecycle, session events

**Logging Guidelines:**

```go
// Use the correct category from internal/logging/logger.go
import "codenerd/internal/logging"

// 22 available categories:
// CategoryKernel, CategorySession, CategoryShards, CategoryPerception,
// CategoryArticulation, CategoryCampaign, CategoryAutopoiesis, CategoryNorthstar,
// CategoryJIT, CategoryWorld, CategoryMCP, CategoryContext, CategoryTactile,
// CategoryEmbedding, CategoryBrowser, CategoryVerification, CategoryRetrieval,
// CategoryTransparency, CategoryUX, CategoryConfig, CategoryInit, CategoryTypes

// Add debug logging for visibility
logging.Get(logging.CategoryWorld).Debug("Dataflow analysis started: entry=%s, lang=%s", entry, lang)

// Add info logging for significant events
logging.Get(logging.CategoryCampaign).Info("Phase transition: %s -> %s", oldPhase, newPhase)

// Add error logging with context
logging.Get(logging.CategoryKernel).Error("Query failed: predicate=%s, err=%v", pred, err)
```

**After Adding Logging:**
1. Rebuild binary: `CGO_CFLAGS="..." go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd`
2. Copy to test bed: `cp nerd.exe /mnt/c/CodeProjects/tribalFitness/`
3. Re-run the failing test
4. Update the package's CLAUDE.md if significant logging was added

### When to Add Test Files

Add test files when you discover:

1. **Untested code paths** - Functions without corresponding `_test.go`
2. **Edge cases** - Discovered during stress testing
3. **Regression prevention** - After fixing a bug
4. **Integration gaps** - Cross-package interactions
5. **Missing coverage** - Critical functions without tests

**Test File Guidelines:**

```go
// File: internal/world/dataflow_test.go (if missing or incomplete)
package world

import (
    "testing"
    "context"
)

func TestDataflowMultiLang(t *testing.T) {
    // Test multi-language dataflow tracing
    ctx := context.Background()
    tracker := NewDataflowTracker()

    // Test Go -> TypeScript boundary
    result, err := tracker.TraceAcrossLanguages(ctx, "backend/main.go", "frontend/src/App.tsx")
    if err != nil {
        t.Fatalf("cross-language trace failed: %v", err)
    }

    if len(result.Flows) == 0 {
        t.Error("expected at least one cross-language flow")
    }
}

func TestDataflowCache(t *testing.T) {
    // Test that caching improves performance
    tracker := NewDataflowTracker()

    // First call - should populate cache
    start1 := time.Now()
    _, _ = tracker.Analyze(context.Background(), "main.go")
    duration1 := time.Since(start1)

    // Second call - should hit cache
    start2 := time.Now()
    _, _ = tracker.Analyze(context.Background(), "main.go")
    duration2 := time.Since(start2)

    if duration2 >= duration1 {
        t.Error("cached call should be faster")
    }
}
```

**Test File Naming:**
- Unit tests: `<file>_test.go` in same package
- Integration tests: `<package>_integration_test.go`
- Stress tests: `<package>_stress_test.go`
- Golden tests: Create `testdata/` directory with `.golden` files

**After Adding Tests:**
1. Run tests locally: `go test ./internal/world/... -v`
2. Ensure they pass
3. Update package CLAUDE.md with new test coverage info
4. Rebuild and redeploy binary

### Self-Enhancement Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                   SELF-ENHANCEMENT LOOP                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. RUN TEST → Failure or missing visibility                    │
│       │                                                          │
│       ▼                                                          │
│  2. DIAGNOSE → What's missing? Logging? Tests? Both?            │
│       │                                                          │
│       ▼                                                          │
│  3. ADD INSTRUMENTATION                                          │
│       ├── Missing logging → Add logging.Get().Debug/Info/Error  │
│       ├── Missing test → Create _test.go file                   │
│       └── Missing both → Add both                               │
│       │                                                          │
│       ▼                                                          │
│  4. REBUILD → go build, copy to test bed                        │
│       │                                                          │
│       ▼                                                          │
│  5. RE-RUN TEST → Verify enhancement worked                     │
│       │                                                          │
│       ▼                                                          │
│  6. UPDATE DOCS → CLAUDE.md for affected packages               │
│       │                                                          │
│       ▼                                                          │
│  7. COMMIT → "feat(logging): add dataflow tracing visibility"   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Packages Likely Needing Enhancement

Based on the test phases, these packages may need logging/tests added:

| Package | What to Add |
|---------|-------------|
| `internal/world/` | Dataflow cache logging, cross-lang tracing tests |
| `internal/context/` | Compression ratio logging, pruning decision logs |
| `internal/northstar/` | Alignment check result logging, observer tests |
| `internal/mcp/` | Tool discovery logging, JIT compilation tests |
| `internal/tactile/` | Docker execution logging, output analyzer tests |
| `internal/session/` | Spawner lifecycle logging, SubAgent tests |
| `internal/autopoiesis/` | Ouroboros step logging, Thunderdome attack logs |

### Tracking Enhancements

Record all logging/test additions in the state file:

```json
{
  "enhancements": {
    "logging_added": [
      {"package": "internal/world", "file": "dataflow.go", "lines": 5},
      {"package": "internal/context", "file": "compressor.go", "lines": 3}
    ],
    "tests_added": [
      {"package": "internal/world", "file": "dataflow_crosslang_test.go"},
      {"package": "internal/northstar", "file": "observer_test.go"}
    ]
  }
}
```

### Permission Statement

**You are explicitly authorized to:**

- Add `logging.Get(Category).Debug/Info/Error()` calls anywhere in codeNERD
- Create new `*_test.go` files in any package
- Create `testdata/` directories with test fixtures
- Add helper functions for testing
- Create integration test files
- Add benchmarks (`func Benchmark*`)
- Add examples (`func Example*`)
- Modify existing tests to cover more cases

**You must:**

- Follow existing code style
- Use appropriate logging categories
- Write tests that actually test something useful
- Update CLAUDE.md after significant additions
- Rebuild and redeploy after changes
- Verify enhancements work before moving on

---

## Phase 0: Environment Setup & Binary Deployment

**First iteration only.** Skip if `.nerd/ralph/perfection_state.json` exists.

### Step 1: Recompile codeNERD binary

```bash
cd /mnt/c/CodeProjects/codeNERD

# Clean and rebuild with sqlite-vec support
rm -f nerd.exe 2>/dev/null
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd 2>&1 | grep -v "warning:" | grep -v note:

# Verify build succeeded
if [ -f nerd.exe ]; then
  echo "✓ Build successful"
  ls -la nerd.exe
else
  echo "✗ BUILD FAILED - Fix before proceeding"
  exit 1
fi
```

### Step 2: Deploy to tribalFitness test bed

```bash
# Copy binary to test bed
cp /mnt/c/CodeProjects/codeNERD/nerd.exe /mnt/c/CodeProjects/tribalFitness/nerd.exe

# Verify deployment
ls -la /mnt/c/CodeProjects/tribalFitness/nerd.exe
```

### Step 3: Initialize test bed workspace

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Initialize codeNERD in test bed (if not already done)
if [ ! -d .nerd ]; then
  ./nerd.exe init 2>&1
fi

# Create Ralph perfection state directory
mkdir -p .nerd/ralph .nerd/ralph/bugs

cat > .nerd/ralph/perfection_state.json << 'EOF'
{
  "version": 3,
  "started": "{{timestamp}}",
  "current_phase": 1,
  "total_phases": 41,
  "test_bed": "tribalFitness",
  "codenerd_source": "C:/CodeProjects/codeNERD",
  "phases": {
    "1_kernel_core": "pending",
    "2_perception_articulation": "pending",
    "3_jit_compiler": "pending",
    "4_config_factory": "pending",
    "5_session_subagents": "pending",
    "6_domain_experts_coder": "pending",
    "7_domain_experts_tester": "pending",
    "8_domain_experts_reviewer": "pending",
    "9_domain_experts_researcher": "pending",
    "10_northstar_guardian": "pending",
    "11_campaign_orchestrator": "pending",
    "12_campaign_decomposer": "pending",
    "13_requirements_interrogator": "pending",
    "14_context_compression": "pending",
    "15_spreading_activation": "pending",
    "16_sparse_retrieval": "pending",
    "17_code_dom_single": "pending",
    "18_code_dom_multi": "pending",
    "19_world_holographic": "pending",
    "20_world_dataflow": "pending",
    "21_world_ast": "pending",
    "22_lsp_integration": "pending",
    "23_mcp_discovery": "pending",
    "24_mcp_execution": "pending",
    "25_glass_box": "pending",
    "26_shadow_mode": "pending",
    "27_usage_tracking": "pending",
    "28_verification": "pending",
    "29_context_harness": "pending",
    "30_integration_sweep": "pending",
    "31_test_bed_validation": "pending",
    "32_ouroboros": "pending",
    "33_nemesis": "pending",
    "34_thunderdome": "pending",
    "35_dream_state": "pending",
    "36_prompt_evolution": "pending",
    "37_autopoiesis_integration": "pending",
    "38_cli_audit": "pending",
    "39_documentation_audit": "pending",
    "40_final_verification": "pending",
    "41_autonomous_monorepo_campaign": "pending"
  },
  "subsystems_tested": [],
  "subsystems_passed": [],
  "subsystems_failed": [],
  "bugs_found": [],
  "bugs_fixed": [],
  "bugs_pending": [],
  "docs_updated": [],
  "tools_generated": [],
  "mcp_servers_installed": [],
  "attacks_executed": 0,
  "evolutions_triggered": 0,
  "test_bed_status": "initialized",
  "iteration": 0
}
EOF
```

### Step 4: Verify test bed is ready

```bash
cd /mnt/c/CodeProjects/tribalFitness
./nerd.exe --version 2>&1
./nerd.exe scan 2>&1 | head -20
```

Increment `iteration` on every loop.

---

## Phase 1: Kernel Core Stability

**Location:** `internal/core/`
**Subsystems:** RealKernel, VirtualStore, SpawnQueue, LimitsEnforcer, Mangle Self-Healing

**Tests to run (in tribalFitness workspace):**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Kernel boot test
timeout 30 ./nerd.exe scan 2>&1
# Must complete without panic
grep -i "panic\|fatal\|nil pointer" .nerd/logs/*.log

# 2. Mangle validation
timeout 60 ./nerd.exe run "query file_topology" 2>&1
# Derivation must complete within gas limit

# 3. Queue stress (conservative)
for i in {1..5}; do
  timeout 30 ./nerd.exe run "what files exist" &
done
wait

# 4. VirtualStore fact operations
timeout 30 ./nerd.exe run "list all Go files" 2>&1
```

**Failure Protocol:** See "Root-Cause Investigation Template" section below.

**Exit condition:** All tests pass cleanly. Update state file. Move to Phase 2.

---

## Phase 2: Perception & Articulation

**Location:** `internal/perception/`, `internal/articulation/`
**Subsystems:** Transducer, LLMClient (7 providers), Emitter, Taxonomy

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Intent parsing - multiple verbs
timeout 30 ./nerd.exe perception "fix the bug in main.go" 2>&1
timeout 30 ./nerd.exe perception "review security issues" 2>&1
timeout 30 ./nerd.exe perception "explain how the API works" 2>&1
timeout 30 ./nerd.exe perception "test the user service" 2>&1
timeout 30 ./nerd.exe perception "research best practices for Go error handling" 2>&1

# 2. Adversarial input
timeout 30 ./nerd.exe perception "!@#$%^&*() 日本語 🚀 \x00" 2>&1
# Must not panic, may return error gracefully

# 3. Provider detection
timeout 30 ./nerd.exe provider status 2>&1
# Check which LLM provider is configured

# 4. Articulation output
timeout 60 ./nerd.exe run "summarize this project in one paragraph" 2>&1
# Check Emitter produces clean output
```

---

## Phase 3: JIT Prompt Compiler

**Location:** `internal/prompt/compiler.go`, `internal/jit/`
**Subsystems:** PromptCompiler, AtomSelector, TokenBudgeter, ContextInjector

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. JIT compilation inspection
timeout 60 ./nerd.exe jit show 2>&1
# Must show compiled prompt without crash

# 2. Persona-specific compilation
timeout 60 ./nerd.exe jit compile --persona coder 2>&1
timeout 60 ./nerd.exe jit compile --persona tester 2>&1
timeout 60 ./nerd.exe jit compile --persona reviewer 2>&1
timeout 60 ./nerd.exe jit compile --persona researcher 2>&1

# 3. Context-aware atom selection
timeout 60 ./nerd.exe jit atoms --intent "/fix" 2>&1
# Should show identity/coder atoms

# 4. Token budget enforcement
timeout 60 ./nerd.exe jit stats 2>&1
# Check token counts are within budget
```

**Key Files to Check on Failure:**
- `internal/prompt/compiler.go`
- `internal/prompt/atoms.go`
- `internal/jit/config/types.go`

---

## Phase 4: ConfigFactory (Intent → Tools Mapping)

**Location:** `internal/prompt/config_factory.go`
**Subsystems:** ConfigFactory, ConfigAtom, ToolSelection

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Intent-to-tool mapping
timeout 30 ./nerd.exe config show --intent "/fix" 2>&1
# Should show: read_file, write_file, edit_file, run_build, git_operation

timeout 30 ./nerd.exe config show --intent "/test" 2>&1
# Should show: run_tests, read_file, write_file

timeout 30 ./nerd.exe config show --intent "/research" 2>&1
# Should show: web_search, web_fetch, context7_fetch, read_file

# 2. Tool registry hydration
timeout 30 ./nerd.exe tools list 2>&1
# Should list all available tools across categories
```

---

## Phase 5: Session & SubAgent Architecture

**Location:** `internal/session/`
**Subsystems:** Executor, Spawner, SubAgent, TaskExecutor

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Clean loop execution
timeout 120 ./nerd.exe run "read the README and summarize it" 2>&1
# Full OODA cycle must complete

# 2. SubAgent spawn (ephemeral)
timeout 120 ./nerd.exe spawn coder "write a hello world function in Go" 2>&1
# Ephemeral subagent must spawn, execute, and terminate

# 3. Check session lifecycle
grep -i "session\|spawn\|subagent" .nerd/logs/*session*.log | head -20

# 4. TaskExecutor interface
timeout 120 ./nerd.exe task execute "list all .go files" 2>&1
```

---

## Phase 6: Domain Expert - Coder

**Location:** `internal/prompt/atoms/identity/coder.yaml`
**Subsystems:** CoderPersona, CodeGeneration, BugFixing, Refactoring

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Simple code generation
timeout 180 ./nerd.exe spawn coder "Create a new file utils/hello.go with a Hello() function that returns 'Hello, World!'" 2>&1

# 2. Bug fix workflow
timeout 180 ./nerd.exe spawn coder "Find and fix any TODO comments in the codebase" 2>&1

# 3. Refactoring
timeout 180 ./nerd.exe spawn coder "Add comments to any exported functions that are missing documentation" 2>&1

# 4. Verify coder identity atom is loaded
grep -i "coder\|identity" .nerd/logs/*jit*.log | head -10
```

---

## Phase 7: Domain Expert - Tester

**Location:** `internal/prompt/atoms/identity/tester.yaml`
**Subsystems:** TesterPersona, TestGeneration, TDDLoop, Coverage

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Test generation
timeout 180 ./nerd.exe spawn tester "Generate unit tests for the main package" 2>&1

# 2. Test execution
timeout 120 ./nerd.exe run "run all tests and report results" 2>&1

# 3. TDD workflow
timeout 300 ./nerd.exe spawn tester "Write tests for any untested functions, then implement the functions if needed" 2>&1

# 4. Coverage report
go test -cover ./... 2>&1 | head -20
```

---

## Phase 8: Domain Expert - Reviewer

**Location:** `internal/prompt/atoms/identity/reviewer.yaml`
**Subsystems:** ReviewerPersona, CodeReview, SecurityScan, QualityAnalysis

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Code review
timeout 180 ./nerd.exe spawn reviewer "Review the main.go file for code quality issues" 2>&1

# 2. Security scan
timeout 180 ./nerd.exe spawn reviewer "Scan for potential security vulnerabilities" 2>&1

# 3. Full codebase review
timeout 300 ./nerd.exe spawn reviewer "Provide a comprehensive code review of this project" 2>&1

# 4. Verify findings are structured
grep -i "review\|finding\|issue" .nerd/logs/*reviewer*.log | head -20
```

---

## Phase 9: Domain Expert - Researcher

**Location:** `internal/prompt/atoms/identity/researcher.yaml`
**Subsystems:** ResearcherPersona, WebFetch, Context7, KnowledgeExtraction

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Documentation research
timeout 180 ./nerd.exe spawn researcher "Research Go best practices for error handling" 2>&1

# 2. Context7 fetch (if available)
timeout 120 ./nerd.exe spawn researcher "Find documentation for the chi router library" 2>&1

# 3. Codebase exploration
timeout 180 ./nerd.exe spawn researcher "Explain how this project is structured" 2>&1

# 4. Knowledge extraction
timeout 180 ./nerd.exe spawn researcher "Extract and summarize the key architectural decisions in this project" 2>&1
```

---

## Phase 10: Northstar Guardian

**Location:** `internal/northstar/`
**Subsystems:** Guardian, Store, CampaignObserver, TaskObserver, AlignmentCheck

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Northstar initialization
timeout 30 ./nerd.exe northstar status 2>&1

# 2. Vision definition (if not already set)
timeout 60 ./nerd.exe northstar wizard 2>&1 << 'EOF'
A fitness tracking application for tribal communities
Track workouts, nutrition, and community challenges
EOF

# 3. On-demand alignment check
timeout 120 ./nerd.exe alignment "Add a new feature for tracking sleep patterns" 2>&1

# 4. Check alignment result format
grep -i "alignment\|vision\|score" .nerd/logs/*northstar*.log | head -20

# 5. Verify Northstar DB exists
ls -la .nerd/northstar_knowledge.db 2>/dev/null
```

**Key Files:**
- `internal/northstar/guardian.go`
- `internal/northstar/store.go`
- `internal/northstar/observer.go`

---

## Phase 11: Campaign Orchestrator

**Location:** `internal/campaign/orchestrator*.go`
**Subsystems:** Orchestrator, PhaseManager, TaskDispatcher, Checkpoint

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Simple campaign
timeout 300 ./nerd.exe campaign start "add a comment to README explaining the project" 2>&1

# 2. Campaign status
timeout 30 ./nerd.exe campaign status 2>&1

# 3. Multi-phase campaign
timeout 600 ./nerd.exe campaign start "Create a new endpoint that returns system health status with tests" 2>&1

# 4. Check campaign logs
grep -i "phase\|task\|checkpoint" .nerd/logs/*campaign*.log | head -30
```

---

## Phase 12: Campaign Decomposer

**Location:** `internal/campaign/decomposer.go`
**Subsystems:** GoalDecomposer, TaskPlanner, DependencyAnalyzer

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Goal decomposition
timeout 180 ./nerd.exe campaign decompose "Implement user authentication with JWT" 2>&1

# 2. Task dependency analysis
timeout 120 ./nerd.exe campaign plan "Add a database migration system" 2>&1

# 3. Check decomposition structure
grep -i "decompose\|task\|dependency" .nerd/logs/*campaign*.log | head -20
```

---

## Phase 13: Requirements Interrogator

**Location:** `internal/shards/requirements_interrogator.go`
**Subsystems:** RequirementsInterrogatorShard, ClarifyingQuestions, AmbiguityDetection

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Ambiguous task clarification
timeout 180 ./nerd.exe spawn requirements_interrogator "make it better" 2>&1
# Should ask clarifying questions

# 2. Complex requirement analysis
timeout 180 ./nerd.exe spawn requirements_interrogator "Add user management" 2>&1
# Should identify ambiguities and ask for specifics

# 3. Clear requirement (no questions needed)
timeout 120 ./nerd.exe spawn requirements_interrogator "Add a function that returns the current timestamp" 2>&1
# Should proceed without excessive questioning

# 4. Check interrogation logs
grep -i "clarif\|question\|ambig" .nerd/logs/*shard*.log | head -20
```

**If RequirementsInterrogator is not wired:**
- Check `internal/shards/registration.go`
- Add registration if missing
- Update CLAUDE.md in shards package

---

## Phase 14: Context Compression & Pruning

**Location:** `internal/context/compressor.go`, `internal/context/serializer.go`
**Subsystems:** SemanticCompressor, TokenReducer, HistoryCondenser, ContextPruner

**CRITICAL TEST:** Verify codeNERD correctly prunes UNRELATED context and keeps RELEVANT context.

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# FIRST: Ensure fresh binary
cp /mnt/c/CodeProjects/codeNERD/nerd.exe ./nerd.exe

# 1. Compression test - long conversation simulation
timeout 300 ./nerd.exe run "Explain the project in detail, then summarize your explanation, then list all files" 2>&1

# 2. Memory check during long operation
timeout 300 ./nerd.exe run "Analyze all Go files in the project and report findings" 2>&1 &
PID=$!
sleep 60
ps -o pid,rss,vsz -p $PID 2>/dev/null
kill $PID 2>/dev/null

# 3. Verify compression happens
grep -i "compress\|condense\|token\|prune" .nerd/logs/*context*.log | head -30

# 4. Memory should stay bounded - no OOM
grep -i "memory\|OOM\|out of memory" .nerd/logs/*.log

# 5. CONTEXT RELEVANCE TEST: Ask about backend, should NOT load Android context heavily
echo "=== CONTEXT RELEVANCE TEST 1: Backend Focus ==="
timeout 180 ./nerd.exe run "Explain the Go backend API structure in backend/internal/" 2>&1
grep -i "android\|kotlin\|compose" .nerd/logs/*context*.log | wc -l
# Should have LOW Android mentions - we asked about backend

# 6. CONTEXT RELEVANCE TEST: Ask about Android, should NOT load backend heavily
echo "=== CONTEXT RELEVANCE TEST 2: Android Focus ==="
timeout 180 ./nerd.exe run "Explain the Android app architecture in android/app/" 2>&1
grep -i "go\|chi\|postgres" .nerd/logs/*context*.log | wc -l
# Should have LOW backend mentions - we asked about Android

# 7. Cross-reference test - verify selective loading
echo "=== CONTEXT PRUNING VERIFICATION ==="
timeout 180 ./nerd.exe run "What database migrations exist in the backend?" 2>&1
# Check that frontend/android context was pruned
grep -i "prune\|skip\|irrelevant" .nerd/logs/*context*.log | head -20

# 8. Token budget enforcement
echo "=== TOKEN BUDGET TEST ==="
timeout 120 ./nerd.exe context stats 2>&1
# Should show token budget utilization
```

**Success Criteria:**
- Memory stays bounded (no OOM)
- Compression log events present
- Backend queries don't load heavy Android context
- Android queries don't load heavy backend context
- Token budget enforced
- Pruning decisions logged

---

## Phase 15: Spreading Activation

**Location:** `internal/context/activation.go`
**Subsystems:** ActivationEngine, FactSelector, RelevanceScoring

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Activation trace
timeout 120 ./nerd.exe activation trace "fix bug in user service" 2>&1

# 2. Fact relevance scoring
timeout 60 ./nerd.exe activation score --query "authentication" 2>&1

# 3. Context-directed spreading
timeout 120 ./nerd.exe run "What files are related to user management?" 2>&1

# 4. Verify activation logs
grep -i "activation\|spread\|relevance" .nerd/logs/*context*.log | head -20
```

---

## Phase 16: Sparse Retrieval

**Location:** `internal/retrieval/`
**Subsystems:** SparseRetriever, BM25, InvertedIndex

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Keyword search
timeout 60 ./nerd.exe retrieval search "error handling" 2>&1

# 2. File retrieval
timeout 60 ./nerd.exe retrieval files "database connection" 2>&1

# 3. Symbol retrieval
timeout 60 ./nerd.exe retrieval symbols "Handler" 2>&1

# 4. Hybrid retrieval (sparse + vector)
timeout 120 ./nerd.exe retrieval hybrid "user authentication flow" 2>&1
```

---

## Phase 17: Code DOM - Single File Edits

**Location:** `internal/tools/codedom/`
**Subsystems:** ElementGetter, LineEditor, ASTParser

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Get elements from file
timeout 60 ./nerd.exe codedom elements main.go 2>&1

# 2. Edit specific lines
timeout 120 ./nerd.exe run "Add a comment at the top of main.go saying '// tribalFitness - fitness tracking for tribes'" 2>&1

# 3. AST-aware modification
timeout 120 ./nerd.exe run "Add a new function called Version() that returns '1.0.0' to main.go" 2>&1

# 4. Verify edits are atomic
git diff --stat | head -10
```

---

## Phase 18: Code DOM - Multi-File & Cross-Platform Flow Edits

**Location:** `internal/tools/codedom/`, `internal/core/virtual_store_codedom.go`
**Subsystems:** BatchEditor, TransactionManager, RollbackHandler, FlowEditor

**CRITICAL TEST:** Edit a complete data flow from Android → Backend → Frontend simultaneously.

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Multi-file simultaneous edit (same platform)
timeout 300 ./nerd.exe run "Create a new package 'backend/internal/utils' with two files: strings.go (string utilities) and numbers.go (number utilities)" 2>&1

# 2. Coordinated refactoring across files
timeout 300 ./nerd.exe run "Rename all occurrences of 'User' to 'Member' across all backend Go files" 2>&1

# 3. Transaction rollback test
timeout 180 ./nerd.exe run "Add error handling to all functions in the backend main package - if this fails, rollback" 2>&1

# 4. CROSS-PLATFORM FLOW EDIT TEST:
# Edit a data model that spans all three platforms at once
echo "=== CROSS-PLATFORM FLOW EDIT ==="
timeout 600 ./nerd.exe run "Add a new 'WorkoutStreak' data model that tracks consecutive workout days.
This should be added to:
1. backend/internal/models/streak.go - Go struct with JSON tags
2. frontend/src/types/streak.ts - TypeScript interface
3. android/app/src/main/java/com/tribalfitness/models/Streak.kt - Kotlin data class

All three must be created in ONE operation with matching field names." 2>&1

# 5. Verify all three files were created
echo "Checking cross-platform edits..."
ls -la backend/internal/models/streak.go 2>/dev/null && echo "✓ Backend model created"
ls -la frontend/src/types/streak.ts 2>/dev/null && echo "✓ Frontend type created"
find android -name "*Streak*" -o -name "*streak*" 2>/dev/null | head -1 && echo "✓ Android model created"

# 6. API LAYER FLOW EDIT:
# Add endpoint + handler + client all at once
timeout 600 ./nerd.exe run "Add a /api/v1/streaks endpoint:
1. backend/cmd/api/handlers/streak_handler.go - HTTP handler
2. frontend/src/api/streaks.ts - API client function
3. android/app/src/main/java/com/tribalfitness/api/StreakApi.kt - Retrofit interface

Implement as coordinated multi-file edit." 2>&1

# 7. Verify API flow files
echo "=== API FLOW VERIFICATION ==="
git diff --stat | head -30
```

**Success Criteria:**
- Single-platform multi-file edits work
- Cross-platform model creation works atomically
- API layer flow (handler + clients) created atomically
- Transaction rollback works on failure

---

## Phase 19: World System - Holographic View & Cartographer

**Location:** `internal/world/holographic.go`, `internal/world/cartographer.go`, `internal/world/code_elements.go`
**Subsystems:** HolographicView, ImpactAnalysis, ChangeGraph, Cartographer, CodeElementExtractor

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# BINARY CHECK - ensure fresh deployment
cp /mnt/c/CodeProjects/codeNERD/nerd.exe ./nerd.exe 2>/dev/null || true

# 1. Impact analysis - single file
echo "=== HOLOGRAPHIC: Single File Impact ==="
timeout 120 ./nerd.exe world impact backend/main.go 2>&1
# What would be affected if main.go changes?

# 2. Change graph
echo "=== HOLOGRAPHIC: Change Graph ==="
timeout 120 ./nerd.exe world graph --file backend/main.go 2>&1

# 3. CROSS-PLATFORM IMPACT ANALYSIS
echo "=== HOLOGRAPHIC: Cross-Platform Impact ==="
timeout 180 ./nerd.exe run "If I change the User model in backend/internal/models/, what Android and Frontend files would be affected?" 2>&1

# 4. Cartographer - code element mapping
echo "=== CARTOGRAPHER: Code Element Mapping ==="
timeout 120 ./nerd.exe world map backend/ 2>&1
timeout 120 ./nerd.exe world map android/app/src/ 2>&1

# 5. Code Elements extraction
echo "=== CODE ELEMENTS: Extraction ==="
timeout 120 ./nerd.exe world elements backend/main.go 2>&1
timeout 120 ./nerd.exe world elements --type function backend/ 2>&1
timeout 120 ./nerd.exe world elements --type class android/ 2>&1

# 6. Holographic context building - natural language
echo "=== HOLOGRAPHIC: Context Building ==="
timeout 180 ./nerd.exe run "What other files would be affected if I modify the database connection code in the backend?" 2>&1

# 7. Cross-reference impact
echo "=== HOLOGRAPHIC: API Impact ==="
timeout 180 ./nerd.exe run "Show me all files that call or depend on the /api/users endpoint" 2>&1

# 8. Check holographic logs
echo "=== HOLOGRAPHIC: Log Verification ==="
grep -i "holographic\|impact\|depend\|cartographer\|element" .nerd/logs/*world*.log | head -30
```

**Success Criteria:**
- Impact analysis produces dependency graph
- Cross-platform impact detected (Go→TS, Go→Kotlin)
- Cartographer maps code elements
- CodeElements extracts functions, classes, types

---

## Phase 20: World System - Dataflow & Taint Analysis

**Location:** `internal/world/dataflow.go`, `internal/world/dataflow_multilang.go`, `internal/world/dataflow_cache.go`
**Subsystems:** TaintAnalysis, DataflowTracker, SinkDetector, MultiLangDataflow, DataflowCache

**CRITICAL TEST:** Trace data flow across platform boundaries (Android → Backend API → Database).

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Single-language dataflow analysis
echo "=== DATAFLOW: Go Backend ==="
timeout 180 ./nerd.exe world dataflow --entry backend/main.go 2>&1

# 2. Multi-language dataflow
echo "=== DATAFLOW: Multi-Language ==="
timeout 240 ./nerd.exe world dataflow --multilang 2>&1

# 3. Taint tracking from user input
echo "=== TAINT: User Input Tracking ==="
timeout 180 ./nerd.exe world taint --source "user input" --lang go 2>&1
timeout 180 ./nerd.exe world taint --source "user input" --lang kotlin 2>&1

# 4. CROSS-PLATFORM DATA FLOW TEST
echo "=== DATAFLOW: Cross-Platform Trace ==="
timeout 300 ./nerd.exe run "Trace how a workout entry flows from:
1. Android app user input
2. Through Retrofit API call
3. To Go backend handler
4. Into PostgreSQL database
Show all files and functions involved." 2>&1

# 5. Security sink detection
echo "=== SINK: Security Analysis ==="
timeout 180 ./nerd.exe world sinks --type sql 2>&1
timeout 180 ./nerd.exe world sinks --type command 2>&1
timeout 180 ./nerd.exe world sinks --type file 2>&1

# 6. Dataflow cache validation
echo "=== DATAFLOW: Cache Test ==="
timeout 60 ./nerd.exe world dataflow --entry backend/main.go 2>&1  # First run
timeout 30 ./nerd.exe world dataflow --entry backend/main.go 2>&1  # Should be cached/faster
grep -i "cache\|hit\|miss" .nerd/logs/*world*.log | tail -10

# 7. API boundary dataflow
echo "=== DATAFLOW: API Boundaries ==="
timeout 180 ./nerd.exe run "Show how data flows across the frontend/backend API boundary" 2>&1

# 8. Check dataflow logs
echo "=== DATAFLOW: Log Verification ==="
grep -i "dataflow\|taint\|sink\|flow\|cache" .nerd/logs/*world*.log | head -40
```

**Success Criteria:**
- Single-language dataflow works
- Multi-language dataflow tracks across Go/Kotlin/TS
- Cross-platform flow (Android→Backend→DB) traced
- Security sinks detected
- Dataflow cache improves performance on repeat queries

---

## Phase 21: World System - AST, Scope & Scanning

**Location:** `internal/world/ast.go`, `internal/world/ast_treesitter.go`, `internal/world/scope.go`, `internal/world/deep_scan.go`, `internal/world/incremental_scan.go`, `internal/world/git_scanner.go`
**Subsystems:** ASTProjector, TreeSitter, SymbolGraph, ScopeAnalyzer, DeepScanner, IncrementalScanner, GitScanner

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. AST projection (Go)
echo "=== AST: Go Projection ==="
timeout 120 ./nerd.exe world ast backend/main.go 2>&1

# 2. Tree-sitter multi-language
echo "=== AST: Tree-Sitter Multi-Lang ==="
timeout 120 ./nerd.exe world ast android/app/src/main/java/com/tribalfitness/MainActivity.kt 2>&1 || echo "Kotlin AST"
timeout 120 ./nerd.exe world ast frontend/src/App.tsx 2>&1 || echo "TSX AST"

# 3. Symbol graph
echo "=== SYMBOLS: Graph Generation ==="
timeout 120 ./nerd.exe world symbols --file backend/main.go 2>&1
timeout 180 ./nerd.exe world symbols --project . 2>&1 | head -50

# 4. Scope analysis
echo "=== SCOPE: Analysis ==="
timeout 120 ./nerd.exe world scope backend/ 2>&1
timeout 120 ./nerd.exe world scope --function main 2>&1

# 5. Dependency extraction
echo "=== DEPS: Extraction ==="
timeout 120 ./nerd.exe world deps 2>&1
timeout 120 ./nerd.exe world deps --external 2>&1
timeout 120 ./nerd.exe world deps --internal 2>&1

# 6. Deep scan (full codebase analysis)
echo "=== DEEP SCAN: Full Analysis ==="
timeout 300 ./nerd.exe world scan --deep 2>&1

# 7. Incremental scan (detect changes)
echo "=== INCREMENTAL: Change Detection ==="
touch backend/main.go  # Simulate change
timeout 60 ./nerd.exe world scan --incremental 2>&1
git checkout backend/main.go 2>/dev/null  # Restore

# 8. Git scanner integration
echo "=== GIT: Scanner Integration ==="
timeout 60 ./nerd.exe world git status 2>&1
timeout 60 ./nerd.exe world git changes 2>&1
timeout 120 ./nerd.exe world git impact --commit HEAD 2>&1

# 9. Full world model query
echo "=== WORLD: Model Query ==="
timeout 180 ./nerd.exe run "What functions call other functions in this project?" 2>&1
timeout 180 ./nerd.exe query "file_topology(_,_,_,_)" 2>&1 | head -20
timeout 180 ./nerd.exe query "symbol_graph(_,_,_,_)" 2>&1 | head -20

# 10. Check AST/world logs
echo "=== AST/WORLD: Log Verification ==="
grep -i "ast\|tree.sitter\|scope\|scan\|symbol" .nerd/logs/*world*.log | head -40
```

**Success Criteria:**
- Go AST projection works
- Tree-sitter parses Kotlin and TSX
- Symbol graph generated
- Scope analysis completes
- Deep scan analyzes full codebase
- Incremental scan detects changes efficiently
- Git scanner provides commit/change info
- World model queries return valid Mangle facts

---

## Phase 22: LSP Integration (Mangle + World)

**Location:** `internal/mangle/lsp.go`, `internal/world/lsp/manager.go`
**Subsystems:** MangleLSP, DiagnosticsProvider, CompletionProvider, WorldLSPManager

**IMPORTANT:** All tests run from tribalFitness workspace - no switching to codeNERD.

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# BINARY CHECK
cp /mnt/c/CodeProjects/codeNERD/nerd.exe ./nerd.exe 2>/dev/null || true

# 1. LSP server availability
echo "=== LSP: Server Check ==="
timeout 30 ./nerd.exe mangle-lsp --check 2>&1

# 2. Mangle file diagnostics (tribalFitness Mangle files if any)
echo "=== LSP: Mangle Diagnostics ==="
timeout 60 ./nerd.exe check-mangle .nerd/*.mg 2>&1 || echo "No Mangle files in test bed"

# 3. Copy and validate codeNERD schemas (run validation FROM tribalFitness)
echo "=== LSP: Schema Validation ==="
mkdir -p .nerd/temp_schemas
cp /mnt/c/CodeProjects/codeNERD/internal/mangle/*.mg .nerd/temp_schemas/ 2>/dev/null || true
cp /mnt/c/CodeProjects/codeNERD/internal/core/defaults/policy/*.mg .nerd/temp_schemas/ 2>/dev/null || true
timeout 120 ./nerd.exe check-mangle .nerd/temp_schemas/*.mg 2>&1 | head -50
rm -rf .nerd/temp_schemas

# 4. World LSP Manager - code intelligence
echo "=== LSP: World Manager ==="
timeout 60 ./nerd.exe lsp status 2>&1
timeout 60 ./nerd.exe lsp hover backend/main.go:10 2>&1 || echo "Hover test"
timeout 60 ./nerd.exe lsp definition backend/main.go:15 2>&1 || echo "Definition test"
timeout 60 ./nerd.exe lsp references backend/main.go:20 2>&1 || echo "References test"

# 5. LSP Diagnostics on Go code
echo "=== LSP: Go Diagnostics ==="
timeout 120 ./nerd.exe lsp diagnostics backend/ 2>&1 | head -30

# 6. Check LSP logs
echo "=== LSP: Log Verification ==="
grep -i "lsp\|diagnostic\|hover\|definition" .nerd/logs/*.log | head -20
```

**Success Criteria:**
- Mangle LSP checks schemas
- World LSP Manager provides code intelligence
- Hover, definition, references work
- Diagnostics produced for Go code

---

## Phase 23: MCP Discovery

**Location:** `internal/mcp/`
**Subsystems:** MCPClientManager, ServerDiscovery, ToolAnalyzer

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. MCP server discovery
timeout 120 ./nerd.exe mcp discover 2>&1

# 2. List available MCP servers
timeout 60 ./nerd.exe mcp list 2>&1

# 3. MCP tool analysis
timeout 120 ./nerd.exe mcp analyze 2>&1

# 4. Search for MCP servers on GitHub (if network available)
timeout 180 ./nerd.exe mcp search "filesystem" 2>&1
```

---

## Phase 24: MCP Execution

**Location:** `internal/mcp/client.go`, `internal/mcp/compiler.go`
**Subsystems:** MCPClient, JITToolCompiler, ToolRenderer

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Install an MCP server (filesystem or another standard one)
timeout 180 ./nerd.exe mcp install filesystem 2>&1 || echo "MCP install may not be available"

# 2. Execute MCP tool
timeout 120 ./nerd.exe mcp exec "list_files" --path "." 2>&1

# 3. JIT tool compilation for MCP
timeout 120 ./nerd.exe mcp compile --task "read file contents" 2>&1

# 4. Three-tier rendering check
timeout 60 ./nerd.exe mcp render --mode full 2>&1
timeout 60 ./nerd.exe mcp render --mode condensed 2>&1
```

---

## Phase 25: Glass Box Visibility

**Location:** `cmd/nerd/chat/glass_box.go`
**Subsystems:** GlassBox, ToolTracer, ExecutionVisualizer

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Enable glass box mode
timeout 120 ./nerd.exe run --glass-box "list all Go files" 2>&1

# 2. Tool execution visibility
timeout 120 ./nerd.exe run --verbose "read main.go and explain it" 2>&1

# 3. Check glass box output
grep -i "tool\|execute\|invoke" .nerd/logs/*glass*.log 2>/dev/null | head -20 || \
grep -i "tool\|execute" .nerd/logs/*session*.log | head -20
```

---

## Phase 26: Shadow Mode

**Location:** `internal/core/shadow_mode.go`
**Subsystems:** ShadowExecutor, ProposalTracker, DiffGenerator

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Shadow mode execution
timeout 180 ./nerd.exe run --shadow "Add a comment to main.go" 2>&1

# 2. Proposal review
timeout 60 ./nerd.exe shadow list 2>&1

# 3. Apply shadow proposal
timeout 60 ./nerd.exe shadow apply 2>&1 || echo "No proposals pending"

# 4. Check shadow logs
grep -i "shadow\|proposal\|diff" .nerd/logs/*shadow*.log 2>/dev/null | head -20
```

---

## Phase 27: Usage Tracking

**Location:** `internal/usage/`
**Subsystems:** UsageTracker, TokenCounter, CostEstimator

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Check usage stats
timeout 30 ./nerd.exe usage stats 2>&1

# 2. Token usage report
timeout 30 ./nerd.exe usage tokens 2>&1

# 3. Session history
timeout 30 ./nerd.exe usage sessions 2>&1

# 4. Cost estimate (if available)
timeout 30 ./nerd.exe usage cost 2>&1
```

---

## Phase 28: Verification System

**Location:** `internal/verification/`
**Subsystems:** CodeVerifier, TestRunner, BuildChecker

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Build verification
timeout 120 ./nerd.exe verify build 2>&1

# 2. Test verification
timeout 180 ./nerd.exe verify tests 2>&1

# 3. Lint verification
timeout 120 ./nerd.exe verify lint 2>&1

# 4. Full verification suite
timeout 300 ./nerd.exe verify all 2>&1
```

---

## Phase 29: Context Harness (Infinite Context Validation)

**Location:** `internal/testing/context_harness/`
**Subsystems:** ContextHarness, SessionSimulator, MetricsCollector, ActivationTracer, CompressionViz, JitTracer, Inspector

**Purpose:** Validate codeNERD's infinite context system - compression, retrieval, activation spreading, and JIT compilation against the tribalFitness monorepo.

**IMPORTANT:** All tests run FROM tribalFitness workspace using the nerd.exe binary deployed there.

**Component Coverage:**
- `harness.go` - Main orchestrator
- `simulator.go` - Session simulation with checkpoints
- `scenarios.go` - Pre-built test scenarios (debugging marathon, feature impl, refactoring)
- `metrics.go` - Compression ratios, retrieval accuracy, latency
- `activation_tracer.go` - Spreading activation through fact graph
- `jit_tracer.go` - JIT prompt compilation tracing
- `compression_viz.go` - Semantic compression visualization
- `inspector.go` - Deep inspection tools

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Ensure we have fresh binary from codeNERD
echo "=== VERIFY BINARY ==="
cp /mnt/c/CodeProjects/codeNERD/nerd.exe ./nerd.exe 2>/dev/null || true
ls -la ./nerd.exe
./nerd.exe version 2>&1 || echo "Binary check"

# Clear logs for clean metrics
rm -rf .nerd/logs/context*.log .nerd/logs/jit*.log 2>/dev/null

# 1. Quick scenario - baseline validation against tribalFitness monorepo
echo "=== CONTEXT HARNESS: Quick Scenario ==="
timeout 600 ./nerd.exe test-context --scenario quick 2>&1

# 2. Debugging marathon - long-term context retention
echo "=== CONTEXT HARNESS: Debugging Marathon ==="
timeout 1800 ./nerd.exe test-context --scenario debugging-marathon 2>&1 || echo "Long test completed/timeout"

# 3. Feature implementation scenario - multi-phase context paging
echo "=== CONTEXT HARNESS: Feature Implementation ==="
timeout 1200 ./nerd.exe test-context --scenario feature-implementation 2>&1 || echo "Feature impl test"

# 4. Refactoring campaign - cross-file tracking
echo "=== CONTEXT HARNESS: Refactoring Campaign ==="
timeout 1200 ./nerd.exe test-context --scenario refactoring-campaign 2>&1 || echo "Refactor test"

# 5. Metrics collection - compression/retrieval stats
echo "=== CONTEXT HARNESS: Metrics Collection ==="
timeout 120 ./nerd.exe test-context --metrics 2>&1

# 6. MONOREPO-SPECIFIC: Verify context isolation across platforms
echo "=== CONTEXT HARNESS: Monorepo Platform Isolation ==="
echo "Backend (Go) context events:"
grep -c "backend/\|\.go\|chi\|postgres" .nerd/logs/*context*.log 2>/dev/null || echo "0"
echo "Android (Kotlin) context events:"
grep -c "android/\|kotlin\|compose\|gradle" .nerd/logs/*context*.log 2>/dev/null || echo "0"
echo "Frontend (React) context events:"
grep -c "frontend/\|\.tsx\|vite\|react" .nerd/logs/*context*.log 2>/dev/null || echo "0"

# 7. Activation tracer verification - spreading through fact graph
echo "=== ACTIVATION TRACER: Fact Graph Spreading ==="
grep -i "activation\|spread\|score\|select" .nerd/logs/*.log 2>/dev/null | head -20

# 8. JIT tracer verification - prompt compilation
echo "=== JIT TRACER: Prompt Compilation Stats ==="
grep -i "jit\|compiled\|atoms\|token\|persona" .nerd/logs/*jit*.log 2>/dev/null | head -20

# 9. Compression visualization check
echo "=== COMPRESSION: Semantic Compression Stats ==="
grep -i "compress\|semantic\|reduction\|ratio\|condense" .nerd/logs/*.log 2>/dev/null | head -20

# 10. Inspector deep checks
echo "=== INSPECTOR: Deep Context Analysis ==="
timeout 120 ./nerd.exe context inspect 2>&1 || echo "Context inspection"
timeout 120 ./nerd.exe context dump 2>&1 | head -50

# 11. Run ALL scenarios for comprehensive validation (full suite)
echo "=== CONTEXT HARNESS: Full Suite ==="
timeout 3600 ./nerd.exe test-context --all --format json > .nerd/ralph/context_harness_results.json 2>&1 || echo "Full suite completed/timeout"
cat .nerd/ralph/context_harness_results.json 2>/dev/null | head -100

# 12. Final context harness validation
echo "=== CONTEXT HARNESS SUMMARY ==="
echo "Total scenarios run: $(grep -c 'scenario\|completed' .nerd/ralph/context_harness_results.json 2>/dev/null || echo 0)"
echo "Compression ratio: $(grep -o 'compression.*[0-9.]*' .nerd/logs/*context*.log 2>/dev/null | tail -1 || echo 'N/A')"
echo "Retrieval accuracy: $(grep -o 'accuracy.*[0-9.]*' .nerd/logs/*context*.log 2>/dev/null | tail -1 || echo 'N/A')"
echo "Activation events: $(grep -c 'activation' .nerd/logs/*.log 2>/dev/null || echo 0)"
```

**Expected Results:**
- Quick scenario: PASS
- Compression ratio: >50% reduction
- Retrieval accuracy: >90%
- Activation spread: Facts selected by relevance, not recency alone
- JIT compilation: Persona atoms loaded correctly
- Monorepo isolation: Each platform (android/backend/frontend) gets focused context when queried specifically
- No OOM or memory explosions during long scenarios

---

## Phase 30: Full Integration Sweep & Log Analysis

**Location:** `internal/logging/`, `internal/tactile/`, `.claude/skills/log-analyzer/`
**Subsystems:** Logger, LogAnalyzer, Tactile OutputAnalyzer, Mangle Log Query

**Purpose:** Comprehensive log analysis and Tactile/Docker output integration testing.

**SKILL INVOCATION:** Use `/log-analyzer` for advanced log debugging.

**Tests:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

# BINARY CHECK
cp /mnt/c/CodeProjects/codeNERD/nerd.exe ./nerd.exe 2>/dev/null || true

# Clear logs for clean baseline
rm -rf .nerd/logs/*

# 1. Generate activity for logging
echo "=== GENERATING LOG ACTIVITY ==="
timeout 180 ./nerd.exe run "analyze the codebase structure and report any issues" 2>&1

# 2. Basic error scan (traditional grep)
echo "=== ERROR SCAN (grep) ==="
grep -i "error\|panic\|fatal\|nil pointer\|undefined\|deadlock" .nerd/logs/*.log 2>/dev/null | head -50

# 3. Clean log check by category (22 categories)
echo "=== CLEAN LOG CHECK (All 22 Categories) ==="
for category in kernel session shards perception articulation campaign autopoiesis northstar jit world mcp context tactile embedding browser verification retrieval transparency ux config init types; do
  count=$(grep -ci "error\|panic" .nerd/logs/*${category}*.log 2>/dev/null || echo 0)
  echo "$category: $count errors"
done

# 4. LOG ANALYZER SKILL - Mangle-based log query
echo "=== LOG ANALYZER: Mangle Query ==="
# Convert logs to Mangle facts and query
timeout 120 ./nerd.exe logs analyze 2>&1 || echo "Log analysis command"
timeout 60 ./nerd.exe logs query "log_error(File, Line, Msg)" 2>&1 || echo "Error query"
timeout 60 ./nerd.exe logs query "log_event(_, /panic, _, _)" 2>&1 || echo "Panic query"

# 5. TACTILE OUTPUT ANALYSIS
echo "=== TACTILE: Output Analyzer ==="
# Test command execution and output capture
timeout 60 ./nerd.exe tactile test "echo hello" 2>&1
timeout 60 ./nerd.exe tactile analyze-output 2>&1 || echo "Output analysis"

# 6. DOCKER EXECUTION (if available)
echo "=== DOCKER: Container Execution ==="
if command -v docker &> /dev/null; then
  timeout 120 ./nerd.exe tactile docker run "alpine" "echo 'docker test'" 2>&1 || echo "Docker execution test"
  timeout 60 ./nerd.exe tactile docker logs 2>&1 || echo "Docker log capture"
else
  echo "Docker not available - skipping"
fi

# 7. Test/Build output analysis
echo "=== TACTILE: Build/Test Output Analysis ==="
cd backend
go test ./... 2>&1 > /tmp/go_test_output.log || true
cd ..
timeout 60 ./nerd.exe tactile analyze-tests /tmp/go_test_output.log 2>&1 || echo "Test analysis"
timeout 60 ./nerd.exe tactile analyze-build /tmp/go_test_output.log 2>&1 || echo "Build analysis"

# 8. Cross-system log correlation
echo "=== LOG CORRELATION: Cross-System ==="
timeout 120 ./nerd.exe logs correlate 2>&1 || echo "Log correlation"

# 9. Performance log analysis
echo "=== PERFORMANCE: Log Timing ==="
grep -i "duration\|took\|elapsed\|latency" .nerd/logs/*.log | head -20

# 10. Session log inspection
echo "=== SESSION: Log Inspection ==="
timeout 60 ./nerd.exe logs session 2>&1 || echo "Session log"

# 11. Final log summary
echo "=== LOG SUMMARY ==="
echo "Total log files: $(ls .nerd/logs/*.log 2>/dev/null | wc -l)"
echo "Total log lines: $(wc -l .nerd/logs/*.log 2>/dev/null | tail -1)"
echo "Error lines: $(grep -ci 'error' .nerd/logs/*.log 2>/dev/null || echo 0)"
echo "Panic lines: $(grep -ci 'panic' .nerd/logs/*.log 2>/dev/null || echo 0)"
echo "Warning lines: $(grep -ci 'warn' .nerd/logs/*.log 2>/dev/null || echo 0)"
```

**Success Criteria:**
- ALL 22 log categories show 0 errors
- Log analyzer skill queries execute
- Tactile output analyzer parses test/build output
- Docker execution captures logs (if Docker available)
- No panics, deadlocks, or nil pointers in any log
- Cross-system log correlation works

---

## Phase 31: Test Bed Validation (tribalFitness)

**The Real Proof:** Use codeNERD to enhance the tribalFitness project.

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Build the project
go build -o tribalFitness.exe . 2>&1

# 2. Use codeNERD to add a feature
timeout 600 ./nerd.exe run "Add a new endpoint /api/health that returns JSON with:
- status: 'healthy'
- timestamp: current time
- version: '1.0.0'
Include tests for this endpoint." 2>&1

# 3. Verify the addition
go build -o tribalFitness.exe . 2>&1
go test -v ./... 2>&1

# 4. Run the app briefly
timeout 5 ./tribalFitness.exe serve 2>&1 &
sleep 2
curl http://localhost:8080/api/health 2>&1 || echo "Health endpoint test"
pkill tribalFitness 2>/dev/null
```

---

## Phase 32: Ouroboros - Self-Generating Tools

**Location:** `internal/autopoiesis/ouroboros.go`
**Subsystems:** OuroborosLoop, ToolGenerator, PanicMaker, SafetyChecker

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Generate a custom analysis tool
timeout 300 ./nerd.exe tool generate "A tool that counts TODO comments in Go files and returns them as structured JSON with file path, line number, and TODO text" 2>&1

# 2. Execute the generated tool
timeout 60 ./nerd.exe tool run "todo_counter" "." 2>&1

# 3. Safety validation (should be blocked)
timeout 60 ./nerd.exe tool generate "A tool that deletes all files matching a pattern using os.RemoveAll" 2>&1
# Should be blocked by SafetyChecker

# 4. Verify tools
echo "=== OUROBOROS STATUS ==="
ls -la .nerd/tools/*.go 2>/dev/null || echo "No tools generated"
```

---

## Phase 33: Nemesis - Adversarial Code Review

**Location:** `internal/shards/nemesis/`
**Subsystems:** NemesisShard, AttackVectorGenerator, VulnerabilityDB

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Run Nemesis review
timeout 300 ./nerd.exe spawn nemesis "Review this codebase for security vulnerabilities, race conditions, and panic vectors. Generate attack programs to prove each vulnerability." 2>&1

# 2. Nemesis self-review (on codeNERD)
cd /mnt/c/CodeProjects/codeNERD
timeout 300 ./nerd.exe spawn nemesis "Review internal/autopoiesis/ouroboros.go for vulnerabilities" 2>&1
cd /mnt/c/CodeProjects/tribalFitness

# 3. Verify findings
echo "=== NEMESIS STATUS ==="
grep -i "vulnerability\|attack\|finding\|panic" .nerd/logs/*nemesis*.log 2>/dev/null | tail -30
```

---

## Phase 34: Thunderdome - Adversarial Battle Arena

**Location:** `internal/autopoiesis/thunderdome.go`
**Subsystems:** Thunderdome, AttackExecutor, SandboxManager

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Run attack vectors
timeout 300 ./nerd.exe thunderdome run --target . --attacks 10 2>&1

# 2. Verify sandbox isolation
git status --short
# Should show no unexpected modifications

# 3. Attack catalog
timeout 120 ./nerd.exe thunderdome list 2>&1

# 4. Verify results
echo "=== THUNDERDOME STATUS ==="
grep -i "SURVIVED\|DEFEATED\|sandbox\|escape" .nerd/logs/*autopoiesis*.log 2>/dev/null | tail -30
```

---

## Phase 35: Dream State - Hypothetical Exploration

**Location:** `internal/core/dream_router.go`, `internal/core/dream_learning.go`
**Subsystems:** DreamRouter, ConsultantPool, HypothesisGenerator

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Enter dream state
timeout 300 ./nerd.exe dream "What would happen if we added social features for tribe members to compete against each other?" 2>&1

# 2. Verify isolation
git status --short
# Should show no changes from dream exploration

# 3. Check dream logs
echo "=== DREAM STATE STATUS ==="
grep -i "dream\|hypothesis\|consultant" .nerd/logs/*dream*.log 2>/dev/null | tail -20
```

---

## Phase 36: Prompt Evolution - System Prompt Learning

**Location:** `internal/autopoiesis/prompt_evolution/`
**Subsystems:** Evolver, Judge, FeedbackCollector, AtomGenerator

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Record execution feedback
timeout 120 ./nerd.exe run "Explain the purpose of main.go" 2>&1

# 2. Trigger evolution cycle
timeout 180 ./nerd.exe prompt evolve 2>&1

# 3. Verify evolved atoms
echo "=== EVOLVED ATOMS ==="
ls -la .nerd/prompts/evolved/ 2>/dev/null

# 4. Verify atom validity
for f in .nerd/prompts/evolved/**/*.yaml; do
  if [ -f "$f" ]; then
    python -c "import yaml; yaml.safe_load(open('$f'))" 2>&1 || echo "INVALID: $f"
  fi
done
```

---

## Phase 37: Autopoiesis Integration - The Full Loop

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Full loop: Generate → Review → Attack → Learn
timeout 120 ./nerd.exe tool generate "A tool that validates Go struct tags" 2>&1
timeout 180 ./nerd.exe spawn nemesis "Review the most recently generated tool" 2>&1
timeout 120 ./nerd.exe thunderdome run --target .nerd/tools/ --attacks 5 2>&1
timeout 60 ./nerd.exe prompt evolve 2>&1

echo "=== AUTOPOIESIS INTEGRATION ==="
echo "Tools generated: $(ls .nerd/tools/*.go 2>/dev/null | wc -l)"
echo "Nemesis findings: $(grep -c 'finding\|vulnerability' .nerd/logs/*nemesis*.log 2>/dev/null || echo 0)"
echo "Thunderdome battles: $(grep -c 'SURVIVED\|DEFEATED' .nerd/logs/*autopoiesis*.log 2>/dev/null || echo 0)"
```

---

## Phase 38: CLI Command Audit

**Verify ALL CLI commands work:**

```bash
cd /mnt/c/CodeProjects/tribalFitness

echo "=== CLI COMMAND AUDIT ==="

# Core commands
timeout 30 ./nerd.exe --help 2>&1 | head -50
timeout 30 ./nerd.exe version 2>&1
timeout 30 ./nerd.exe init --help 2>&1
timeout 30 ./nerd.exe scan --help 2>&1
timeout 30 ./nerd.exe run --help 2>&1

# Session commands
timeout 30 ./nerd.exe sessions --help 2>&1
timeout 30 ./nerd.exe session new --help 2>&1

# Campaign commands
timeout 30 ./nerd.exe campaign --help 2>&1
timeout 30 ./nerd.exe campaign start --help 2>&1
timeout 30 ./nerd.exe campaign status --help 2>&1

# Spawn commands
timeout 30 ./nerd.exe spawn --help 2>&1

# JIT commands
timeout 30 ./nerd.exe jit --help 2>&1

# Mangle commands
timeout 30 ./nerd.exe check-mangle --help 2>&1
timeout 30 ./nerd.exe query --help 2>&1

# Tool commands
timeout 30 ./nerd.exe tool --help 2>&1
timeout 30 ./nerd.exe tools --help 2>&1

# Northstar commands
timeout 30 ./nerd.exe northstar --help 2>&1
timeout 30 ./nerd.exe alignment --help 2>&1

# MCP commands
timeout 30 ./nerd.exe mcp --help 2>&1

# Dream/Shadow commands
timeout 30 ./nerd.exe dream --help 2>&1
timeout 30 ./nerd.exe shadow --help 2>&1

# Advanced commands
timeout 30 ./nerd.exe thunderdome --help 2>&1
timeout 30 ./nerd.exe prompt --help 2>&1

# Document any missing or broken commands
echo "Missing commands should be added to cmd/nerd/ and documented"
```

**If commands are missing:**
1. Add command implementation to `cmd/nerd/cmd_*.go`
2. Register in `cmd/nerd/main.go`
3. Update `cmd/nerd/README.md`
4. Update `cmd/nerd/CLAUDE.md`

---

## Phase 39: Documentation Audit

**Verify CLAUDE.md files are up-to-date:**

```bash
cd /mnt/c/CodeProjects/codeNERD

echo "=== CLAUDE.MD AUDIT ==="

# Find all CLAUDE.md files
find . -name "CLAUDE.md" -type f | while read f; do
  echo "Checking: $f"
  # Check if file is outdated (modified Go files newer than CLAUDE.md)
  dir=$(dirname "$f")
  go_newer=$(find "$dir" -name "*.go" -newer "$f" 2>/dev/null | head -1)
  if [ -n "$go_newer" ]; then
    echo "  ⚠ May need update - Go files modified after CLAUDE.md"
  else
    echo "  ✓ Up to date"
  fi
done

# Check README.md
echo ""
echo "=== README AUDIT ==="
if [ -f README.md ]; then
  echo "README.md exists"
  # Check if it mentions all major components
  for component in "Kernel" "JIT" "Campaign" "Northstar" "MCP" "Autopoiesis"; do
    if grep -qi "$component" README.md; then
      echo "  ✓ Documents: $component"
    else
      echo "  ⚠ Missing: $component"
    fi
  done
fi
```

**Documentation Update Checklist:**
- [ ] All 51+ CLAUDE.md files reflect current code
- [ ] README.md describes all major features
- [ ] File Index tables are accurate
- [ ] Key types documented
- [ ] Usage examples provided

---

## Phase 40: Final Verification

```bash
cd /mnt/c/CodeProjects/tribalFitness

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           CODENERD PERFECTION - FINAL VERIFICATION           ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# 1. Rebuild and redeploy
echo ""
echo "=== 1. FRESH BUILD & DEPLOY ==="
cd /mnt/c/CodeProjects/codeNERD
rm -f nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd 2>&1 | grep -v "warning:"
if [ $? -eq 0 ]; then
  echo "✓ Build successful"
  cp nerd.exe /mnt/c/CodeProjects/tribalFitness/nerd.exe
  echo "✓ Deployed to tribalFitness"
else
  echo "✗ Build FAILED"
fi
cd /mnt/c/CodeProjects/tribalFitness

# 2. Clear logs and run comprehensive test
rm -rf .nerd/logs/*
echo ""
echo "=== 2. COMPREHENSIVE EXECUTION ==="
timeout 300 ./nerd.exe run "Verify codeNERD is working correctly by:
1. Scanning the codebase
2. Querying the kernel for file_topology
3. Checking any Mangle schemas
4. Generating a simple test
Report any issues found." 2>&1

# 3. Log cleanliness
echo ""
echo "=== 3. LOG CLEANLINESS ==="
total_errors=0
for log in .nerd/logs/*.log; do
  if [ -f "$log" ]; then
    errors=$(grep -ci "error\|panic\|fatal\|nil pointer\|undefined" "$log" 2>/dev/null || echo 0)
    if [ "$errors" -gt 0 ]; then
      echo "✗ FAIL: $log has $errors issues"
      total_errors=$((total_errors + errors))
    fi
  fi
done
if [ "$total_errors" -eq 0 ]; then
  echo "✓ All log categories clean"
fi

# 4. Domain experts
echo ""
echo "=== 4. DOMAIN EXPERTS ==="
for persona in coder tester reviewer researcher; do
  if grep -qi "$persona" .nerd/logs/*.log 2>/dev/null; then
    echo "✓ $persona: operational"
  else
    echo "⚠ $persona: not tested"
  fi
done

# 5. Northstar
echo ""
echo "=== 5. NORTHSTAR ==="
if [ -f .nerd/northstar_knowledge.db ]; then
  echo "✓ Northstar DB exists"
else
  echo "⚠ Northstar DB not found"
fi

# 6. Code DOM
echo ""
echo "=== 6. CODE DOM ==="
if grep -qi "codedom\|edit" .nerd/logs/*.log 2>/dev/null; then
  echo "✓ Code DOM: operational"
else
  echo "⚠ Code DOM: not tested"
fi

# 7. World Systems
echo ""
echo "=== 7. WORLD SYSTEMS ==="
for system in holographic dataflow ast; do
  if grep -qi "$system" .nerd/logs/*.log 2>/dev/null; then
    echo "✓ $system: operational"
  fi
done

# 8. MCP
echo ""
echo "=== 8. MCP ==="
if grep -qi "mcp" .nerd/logs/*.log 2>/dev/null; then
  echo "✓ MCP: operational"
else
  echo "⚠ MCP: not tested"
fi

# 9. JIT
echo ""
echo "=== 9. JIT ==="
if grep -qi "jit\|compiled prompt" .nerd/logs/*.log 2>/dev/null; then
  echo "✓ JIT: operational"
else
  echo "⚠ JIT: not tested"
fi

# 10. Autopoiesis systems
echo ""
echo "=== 10. AUTOPOIESIS ==="
echo "Tools generated: $(ls .nerd/tools/*.go 2>/dev/null | wc -l)"
echo "Nemesis findings: $(grep -c 'finding\|vulnerability' .nerd/logs/*nemesis*.log 2>/dev/null || echo 0)"
echo "Thunderdome battles: $(grep -c 'SURVIVED\|DEFEATED' .nerd/logs/*autopoiesis*.log 2>/dev/null || echo 0)"
echo "Dream events: $(grep -c 'dream\|hypothesis' .nerd/logs/*dream*.log 2>/dev/null || echo 0)"
echo "Evolution cycles: $(grep -c 'evolve\|verdict' .nerd/logs/*autopoiesis*.log 2>/dev/null || echo 0)"

# 11. Test bed status
echo ""
echo "=== 11. TEST BED (tribalFitness) ==="
if [ -f tribalFitness.exe ] || [ -f main.go ]; then
  go build -o test_build.exe . 2>&1 && echo "✓ Test bed builds" || echo "✗ Test bed build failed"
  rm -f test_build.exe
  go test -v ./... 2>&1 | tail -5
fi

# 12. Phase status
echo ""
echo "=== 12. PHASE COMPLETION STATUS ==="
cat .nerd/ralph/perfection_state.json | grep -E '"[0-9]+_' | head -40

# 13. Documentation status
echo ""
echo "=== 13. DOCUMENTATION ==="
docs_updated=$(cat .nerd/ralph/perfection_state.json | grep -c 'docs_updated' 2>/dev/null || echo 0)
echo "Documentation updates recorded: $docs_updated"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    VERIFICATION SUMMARY                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Checklist for promise output:"
echo "  [ ] Build succeeds"
echo "  [ ] Binary deployed to tribalFitness"
echo "  [ ] All log categories clean (0 errors)"
echo "  [ ] All 4 domain experts operational"
echo "  [ ] Northstar DB exists and checks pass"
echo "  [ ] Code DOM single and multi-file edits work"
echo "  [ ] Context compression maintains memory bounds"
echo "  [ ] Spreading activation selects relevant facts"
echo "  [ ] World systems (holographic, dataflow, AST) work"
echo "  [ ] MCP discovery and execution work"
echo "  [ ] JIT compilation works for all personas"
echo "  [ ] Glass Box shows tool execution"
echo "  [ ] Shadow mode tracks proposals"
echo "  [ ] Ouroboros generated at least 1 tool"
echo "  [ ] Nemesis review completed"
echo "  [ ] Thunderdome executed attacks"
echo "  [ ] Dream State activated"
echo "  [ ] Prompt Evolution recorded feedback"
echo "  [ ] All 40 phases marked complete in state file"
echo "  [ ] All CLAUDE.md files updated for changed packages"
echo "  [ ] README.md documents all features"
echo ""
echo "IMPORTANT: Phase 40 completion enables Phase 41."
echo "Phase 41 (Autonomous Monorepo Campaign) must ALSO complete for the promise."
echo ""
echo "Proceed to Phase 41 - THE ULTIMATE TEST."
```

**Only when ALL checks pass AND Phase 41 (Autonomous Monorepo Campaign) completes, output:**

```
<promise>CODENERD PERFECTION ACHIEVED</promise>
```

---

## Phase 41: THE ULTIMATE TEST - Autonomous Monorepo Campaign

> **This is the final proof.** codeNERD must autonomously research, plan, and implement a cross-platform feature across the tribalFitness monorepo using ALL systems.

### Test Structure

```
tribalFitness Monorepo:
├── android/        # Kotlin Android app (Jetpack Compose)
├── backend/        # Go API server (Chi router, PostgreSQL)
├── frontend/       # React/TypeScript web app (Vite)
├── Docs/
│   ├── references/         # 7 research categories
│   │   ├── biofeedback-neuroscience/
│   │   ├── community-social/
│   │   ├── fitness-metrics/
│   │   ├── gamification/
│   │   ├── ml-ai-vr/
│   │   ├── psychological-assessments/
│   │   └── smart-equipment/
│   └── blueprints/features/ # 26 feature categories
│       ├── 01-core-gameplay/
│       ├── 02-guild-social/
│       ├── ...
│       └── 26-experimental-future/
```

### Campaign Goal: Guild Challenge System

**Target Feature Block:** `02-guild-social` + `03-competition` + `17-gamification`

**Cross-Platform Implementation Required:**
- **Backend (Go):** API endpoints for guild challenges, leaderboards, rewards
- **Android (Kotlin):** Native guild challenge UI, real-time updates
- **Frontend (React):** Web dashboard for guild management

### Phase 41.1: Research Ingestion

codeNERD MUST use the Researcher shard to ingest reference materials:

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Ingest all relevant research
timeout 600 ./nerd.exe spawn researcher "Read and summarize all documents in Docs/references/gamification/ and Docs/references/community-social/. Extract key patterns for guild-based competition systems." 2>&1

# 2. Extract knowledge atoms
timeout 300 ./nerd.exe run "Create knowledge atoms from the research findings for use in the campaign" 2>&1

# 3. Verify research ingestion
grep -i "knowledge\|atom\|research" .nerd/logs/*researcher*.log | tail -30
```

**Success Criteria:**
- Researcher shard successfully reads reference documents
- Knowledge atoms extracted and stored
- Research summary produced

### Phase 41.2: Blueprint Analysis

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Analyze the target feature blueprints
timeout 600 ./nerd.exe spawn researcher "Analyze these feature blueprints and create a unified implementation plan:
- Docs/blueprints/features/02-guild-social/
- Docs/blueprints/features/03-competition/
- Docs/blueprints/features/17-gamification/

Focus on: What APIs are needed? What UI components? What data models?" 2>&1

# 2. Extract requirements
timeout 300 ./nerd.exe spawn requirements_interrogator "Based on the guild social and competition blueprints, what are the key requirements for a Guild Challenge System?" 2>&1
```

**Success Criteria:**
- Blueprints analyzed across categories
- Requirements extracted and clarified
- Cross-platform scope identified

### Phase 41.3: Northstar Alignment

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Ensure vision is set (or set it)
timeout 60 ./nerd.exe northstar wizard 2>&1 << 'EOF'
tribalFitness - A gamified fitness platform that transforms individual wellness into tribal adventures
Help people achieve fitness goals through community competition, guild challenges, and gamified progression
EOF

# 2. Check alignment of proposed feature
timeout 180 ./nerd.exe alignment "Implement a Guild Challenge System where tribes compete in fitness challenges, with leaderboards, rewards, and cross-platform synchronization" 2>&1

# 3. Verify alignment score
grep -i "alignment\|score\|vision" .nerd/logs/*northstar*.log | tail -20
```

**Success Criteria:**
- Vision defined for tribalFitness
- Alignment check passes (score >= 0.7)
- Feature approved for implementation

### Phase 41.4: Tool Generation (Ouroboros)

Before implementation, codeNERD generates needed tools:

```bash
cd /mnt/c/CodeProjects/tribalFitness

# 1. Generate a monorepo analyzer tool
timeout 300 ./nerd.exe tool generate "A tool that analyzes a monorepo structure and returns a JSON map of: backend APIs (Go handlers), frontend components (React), and Android screens (Kotlin composables)" 2>&1

# 2. Generate a cross-platform code correlator
timeout 300 ./nerd.exe tool generate "A tool that given a feature name, finds all related files across Go backend, React frontend, and Kotlin Android code" 2>&1

# 3. Safety check - these should be allowed
ls -la .nerd/tools/*.go 2>/dev/null

# 4. Verify tools compile
cd .nerd/tools && go build *.go 2>&1 || echo "No tools to compile"
cd /mnt/c/CodeProjects/tribalFitness
```

**Success Criteria:**
- At least 1 tool generated
- Tools compile without errors
- No safety violations

### Phase 41.5: Campaign Launch - Full Autonomous Implementation

**THE BIG TEST:** Launch a multi-phase campaign that implements across all three platforms:

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Launch the autonomous campaign
timeout 3600 ./nerd.exe campaign start "Implement a Guild Challenge System with the following requirements:

## Backend (Go - backend/)
1. Create a new package internal/challenges/ with:
   - Challenge model (id, guild_id, type, start_date, end_date, metrics, rewards)
   - ChallengeService with CRUD operations
   - LeaderboardService for real-time rankings
2. Add API endpoints in cmd/api/:
   - POST /api/v1/guilds/{id}/challenges - Create challenge
   - GET /api/v1/guilds/{id}/challenges - List guild challenges
   - GET /api/v1/challenges/{id}/leaderboard - Get leaderboard
   - POST /api/v1/challenges/{id}/submit - Submit workout result
3. Add database migrations for challenges table

## Frontend (React - frontend/)
1. Create components in src/components/challenges/:
   - ChallengeCard.tsx - Display single challenge
   - ChallengeList.tsx - List active challenges
   - LeaderboardView.tsx - Show rankings
   - ChallengeCreator.tsx - Form to create new challenge
2. Add pages in src/pages/:
   - GuildChallengesPage.tsx - Main challenges view
3. Add API client in src/api/challenges.ts
4. Add routing in src/App.tsx

## Android (Kotlin - android/)
1. Create package app/src/main/java/com/tribalfitness/challenges/ with:
   - ChallengeModel.kt - Data class
   - ChallengeRepository.kt - API calls
   - ChallengeViewModel.kt - State management
2. Create Composable screens in ui/challenges/:
   - ChallengeListScreen.kt
   - ChallengeDetailScreen.kt
   - LeaderboardScreen.kt
3. Add navigation in ui/navigation/NavGraph.kt

## Testing
- Write unit tests for all new backend services
- Write integration tests for API endpoints
- Write component tests for React components

## Integration
- Ensure real-time updates work across platforms
- Verify data model consistency
- Test offline capability on Android

Apply research from gamification references for engagement patterns.
Use guild-social references for community features.
Follow existing code patterns in each platform." 2>&1
```

**Campaign Monitoring:**

```bash
# Monitor campaign progress
timeout 30 ./nerd.exe campaign status 2>&1

# Check phase progression
grep -i "phase\|task\|complete" .nerd/logs/*campaign*.log | tail -50

# Check for errors
grep -i "error\|fail\|panic" .nerd/logs/*.log | head -30
```

**Success Criteria:**
- Campaign decomposes into multiple phases
- Backend Go code generated in `backend/internal/challenges/`
- Frontend React components generated in `frontend/src/components/challenges/`
- Android Kotlin code generated in `android/app/src/main/java/.../challenges/`
- Tests generated for each platform
- No panics or crashes during campaign

### Phase 41.6: Code Review (Reviewer Shard)

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Review generated Go code
timeout 300 ./nerd.exe spawn reviewer "Review all new Go code in backend/internal/challenges/ for:
- Proper error handling
- SQL injection prevention
- Race condition safety
- API design best practices" 2>&1

# Review generated React code
timeout 300 ./nerd.exe spawn reviewer "Review all new React code in frontend/src/components/challenges/ for:
- Component architecture
- Type safety
- Accessibility
- Performance" 2>&1

# Review generated Kotlin code
timeout 300 ./nerd.exe spawn reviewer "Review all new Kotlin code in android/app/src/main/java/**/challenges/ for:
- Compose best practices
- State management
- Error handling
- Memory leaks" 2>&1
```

**Success Criteria:**
- All three platform codebases reviewed
- Findings documented
- No critical security issues

### Phase 41.7: Testing (Tester Shard)

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Test Go backend
cd backend
go test ./internal/challenges/... -v 2>&1 | tee /tmp/go_tests.log
cd ..

# Test React frontend
cd frontend
npm test -- --coverage 2>&1 | tee /tmp/react_tests.log || echo "React tests may need setup"
cd ..

# Test Android (if build environment available)
cd android
./gradlew test 2>&1 | tee /tmp/android_tests.log || echo "Android tests may need setup"
cd ..

# Use codeNERD to analyze test results
timeout 180 ./nerd.exe run "Analyze the test results in /tmp/*_tests.log and report:
1. Total tests passed/failed
2. Coverage percentage
3. Any critical failures" 2>&1
```

**Success Criteria:**
- Backend Go tests pass
- Frontend tests run
- Android tests run
- Coverage report generated

### Phase 41.8: Adversarial Testing (Nemesis + Thunderdome)

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Nemesis reviews the new code
timeout 600 ./nerd.exe spawn nemesis "Perform adversarial analysis on the Guild Challenge System implementation:
1. Find SQL injection vectors in backend/internal/challenges/
2. Find XSS vectors in frontend/src/components/challenges/
3. Find data leakage in android/.../challenges/
4. Generate attack programs to prove vulnerabilities" 2>&1

# Thunderdome battles
timeout 300 ./nerd.exe thunderdome run --target backend/internal/challenges --attacks 10 2>&1
timeout 300 ./nerd.exe thunderdome run --target frontend/src/components/challenges --attacks 10 2>&1
```

**Success Criteria:**
- Nemesis completes analysis
- Vulnerabilities documented
- Thunderdome attacks contained in sandbox
- No sandbox escapes

### Phase 41.9: Dream State Exploration

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Dream about future enhancements
timeout 300 ./nerd.exe dream "What would happen if we added:
1. Real-time multiplayer challenge battles
2. AR visualization of guild territories
3. AI-generated personalized challenges
4. Cross-guild tournaments

Explore how these would integrate with the current implementation." 2>&1

# Verify dream isolation
git status --short
# Should show no changes from dream exploration
```

**Success Criteria:**
- Dream state generates hypotheses
- No file modifications from dream
- Insights captured in logs

### Phase 41.10: Prompt Evolution

```bash
cd /mnt/c/CodeProjects/tribalFitness

# Record feedback from the entire campaign
timeout 180 ./nerd.exe prompt evolve 2>&1

# Check evolved atoms
ls -la .nerd/prompts/evolved/ 2>/dev/null

# Verify learnings captured
grep -i "evolve\|learn\|strategy" .nerd/logs/*autopoiesis*.log | tail -20
```

**Success Criteria:**
- Execution feedback recorded
- Strategies updated for multi-platform work
- Evolved atoms generated if issues occurred

### Phase 41.11: Integration Verification

```bash
cd /mnt/c/CodeProjects/tribalFitness

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║      AUTONOMOUS MONOREPO CAMPAIGN - FINAL VERIFICATION       ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# 1. Backend verification
echo ""
echo "=== BACKEND (Go) ==="
if [ -d backend/internal/challenges ]; then
  echo "✓ challenges package created"
  ls -la backend/internal/challenges/*.go 2>/dev/null | wc -l
  cd backend && go build ./... 2>&1 && echo "✓ Backend builds" || echo "✗ Backend build failed"
  cd ..
else
  echo "✗ challenges package NOT created"
fi

# 2. Frontend verification
echo ""
echo "=== FRONTEND (React) ==="
if [ -d frontend/src/components/challenges ]; then
  echo "✓ challenges components created"
  ls -la frontend/src/components/challenges/*.tsx 2>/dev/null | wc -l
  cd frontend && npm run build 2>&1 | tail -5 && echo "✓ Frontend builds" || echo "✗ Frontend build failed"
  cd ..
else
  echo "✗ challenges components NOT created"
fi

# 3. Android verification
echo ""
echo "=== ANDROID (Kotlin) ==="
ANDROID_CHALLENGES=$(find android -name "*.kt" -path "*challenges*" 2>/dev/null | wc -l)
if [ "$ANDROID_CHALLENGES" -gt 0 ]; then
  echo "✓ $ANDROID_CHALLENGES Kotlin files in challenges"
  cd android && ./gradlew assembleDebug 2>&1 | tail -5 && echo "✓ Android builds" || echo "✗ Android build failed"
  cd ..
else
  echo "✗ No Kotlin challenges files created"
fi

# 4. Cross-platform summary
echo ""
echo "=== CROSS-PLATFORM SUMMARY ==="
echo "Backend files: $(find backend/internal/challenges -name '*.go' 2>/dev/null | wc -l)"
echo "Frontend files: $(find frontend/src -name '*challenge*' 2>/dev/null | wc -l)"
echo "Android files: $(find android -name '*Challenge*' -o -name '*challenge*' 2>/dev/null | wc -l)"

# 5. Test summary
echo ""
echo "=== TEST RESULTS ==="
echo "Backend tests: $(grep -c 'PASS\|FAIL' /tmp/go_tests.log 2>/dev/null || echo 'not run')"
echo "Frontend tests: $(grep -c 'passed\|failed' /tmp/react_tests.log 2>/dev/null || echo 'not run')"

# 6. Campaign logs
echo ""
echo "=== CAMPAIGN EXECUTION ==="
echo "Campaign phases completed: $(grep -c 'phase.*complete' .nerd/logs/*campaign*.log 2>/dev/null || echo 0)"
echo "Tasks executed: $(grep -c 'task.*complete' .nerd/logs/*campaign*.log 2>/dev/null || echo 0)"
echo "Errors encountered: $(grep -ci 'error' .nerd/logs/*campaign*.log 2>/dev/null || echo 0)"

# 7. Systems used
echo ""
echo "=== SYSTEMS EXERCISED ==="
echo "Researcher: $(grep -c 'researcher' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Coder: $(grep -c 'coder' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Tester: $(grep -c 'tester' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Reviewer: $(grep -c 'reviewer' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Nemesis: $(grep -c 'nemesis' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Ouroboros: $(grep -c 'ouroboros\|tool generate' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Northstar: $(grep -c 'northstar\|alignment' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Dream: $(grep -c 'dream' .nerd/logs/*.log 2>/dev/null || echo 0) events"
echo "Evolution: $(grep -c 'evolve' .nerd/logs/*.log 2>/dev/null || echo 0) events"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║               PHASE 41 COMPLETION CHECKLIST                   ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  [ ] Research from Docs/references/ ingested"
echo "  [ ] Blueprints from Docs/blueprints/features/ analyzed"
echo "  [ ] Northstar alignment verified"
echo "  [ ] Custom tools generated by Ouroboros"
echo "  [ ] Backend Go code implemented in backend/internal/challenges/"
echo "  [ ] Frontend React code implemented in frontend/src/components/challenges/"
echo "  [ ] Android Kotlin code implemented in android/.../challenges/"
echo "  [ ] All platforms build successfully"
echo "  [ ] Tests pass on all platforms"
echo "  [ ] Reviewer analyzed all generated code"
echo "  [ ] Nemesis adversarial review completed"
echo "  [ ] Thunderdome attacks executed safely"
echo "  [ ] Dream state exploration completed"
echo "  [ ] Prompt evolution recorded learnings"
echo "  [ ] Zero panics throughout campaign"
echo ""
echo "If ALL boxes can be checked, Phase 41 is COMPLETE."
echo "Combined with Phases 1-40, you may output the promise."
```

### Success Criteria for Phase 41

| Requirement | Verification |
|-------------|--------------|
| Research ingested | Knowledge atoms created from reference docs |
| Blueprints analyzed | Feature requirements extracted |
| Northstar aligned | Alignment score >= 0.7 |
| Tools generated | At least 1 Ouroboros tool in .nerd/tools/ |
| Backend implemented | Go files in backend/internal/challenges/ |
| Frontend implemented | React files in frontend/src/components/challenges/ |
| Android implemented | Kotlin files in android/.../challenges/ |
| All builds pass | `go build`, `npm build`, `gradlew` succeed |
| Tests pass | Test suites execute without critical failures |
| Code reviewed | Reviewer findings documented |
| Adversarial tested | Nemesis + Thunderdome completed |
| Dream explored | Hypotheses generated, no file changes |
| Evolution triggered | Learnings recorded |
| Zero panics | No crashes during entire campaign |

### Why This Phase is Critical

Phase 41 proves that codeNERD can:

1. **Research Autonomously** - Ingest and synthesize domain knowledge
2. **Plan Multi-Platform Work** - Decompose cross-platform features
3. **Generate Multiple Languages** - Go, TypeScript/React, Kotlin
4. **Maintain Alignment** - Keep work aligned with project vision
5. **Self-Improve** - Generate tools it needs, learn from execution
6. **Quality Assure** - Review, test, and adversarially verify
7. **Explore Futures** - Dream about enhancements without breaking reality
8. **Learn** - Record feedback and evolve prompts

**This is the difference between a coding assistant and an autonomous coding agent.**

---

## Root-Cause Investigation Template

When you find a bug, document it in `.nerd/ralph/bugs/BUG-XXX.md`:

```markdown
# BUG-XXX: <Short description>

## Symptom
What happened (error message, panic, etc.)

## Proximate Cause
The immediate trigger (nil pointer, missing validation, etc.)

## Root Cause (Five Whys)
1. Why did X happen? → Because Y
2. Why did Y happen? → Because Z
3. Why did Z happen? → Because W
4. Why did W happen? → Because V
5. Why did V happen? → Because <ROOT CAUSE>

## Systemic Fix
What code change prevents this class of bug forever?

## Files Changed
- `path/to/file.go:line` - Description of change

## Documentation Updates Required
- [ ] `path/to/CLAUDE.md` - Updated because <reason>
- [ ] `README.md` - Updated because <reason>

## Verification
How to verify the fix works?
```

**IMPORTANT:** Every bug fix MUST include documentation updates:
1. Update the CLAUDE.md in the affected package(s)
2. Update README.md if user-facing behavior changed
3. Add to `docs_updated` array in state file

---

## Anti-Patterns (FORBIDDEN)

You MUST NOT:
- Comment out broken code
- Delete corrupted artifacts
- Add nil checks without tracing nil source
- Wrap in recover() to hide panics
- Increase timeouts to hide slowness
- Add special cases for specific failures
- Disable features that stress test broke
- Use `// TODO: fix later` comments
- Mark bugs as fixed without verification
- Skip documentation updates after fixes
- Leave CLAUDE.md files outdated after refactors

---

## Iteration Strategy

Each Ralph iteration:

1. **Read state file** - Know what phase you're on
2. **Check binaries** - Recompile if source changed: `go build -o nerd.exe ./cmd/nerd`
3. **Deploy to test bed** - Copy to tribalFitness if rebuilt
4. **Run next test** - Based on current phase
5. **If pass** - Mark in state, move to next
6. **If fail** - Apply root-cause protocol, fix, verify, re-run
7. **Update docs** - CLAUDE.md and README.md for any changes
8. **Update state** - Increment iteration, update timestamps
9. **Check completion** - All phases done + logs clean + test bed works?
10. **Output promise** - ONLY if truly complete

---

## Binary Recompilation Checklist

After ANY code change in codeNERD:

```bash
# 1. Navigate to source
cd /mnt/c/CodeProjects/codeNERD

# 2. Run go vet
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go vet ./... 2>&1

# 3. Rebuild
rm -f nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd 2>&1 | grep -v "warning:"

# 4. Deploy to test bed
cp nerd.exe /mnt/c/CodeProjects/tribalFitness/nerd.exe

# 5. Verify deployment
/mnt/c/CodeProjects/tribalFitness/nerd.exe --version
```

---

## Files to Monitor

| File | Purpose |
|------|---------|
| `.nerd/ralph/perfection_state.json` | Progress tracking |
| `.nerd/ralph/bugs/` | Bug documentation |
| `.nerd/logs/*` | All log categories (22 total) |
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
| `nerd.exe` | Built binary (source) |
| `tribalFitness/nerd.exe` | Deployed binary (test bed) |

---

## Expected Duration

| Phases | Duration | Focus |
|--------|----------|-------|
| 0 | 15 min | Setup, build, deploy |
| 1-5 | 2-3 hours | Core stability (kernel, perception, JIT, session) |
| 6-9 | 2-3 hours | Domain experts (coder, tester, reviewer, researcher) |
| 10-13 | 1-2 hours | Northstar, campaigns, requirements |
| 14-16 | 1-2 hours | Context systems (compression, activation, retrieval) |
| 17-18 | 1-2 hours | Code DOM (single and multi-file) |
| 19-22 | 2-3 hours | World systems (holographic, dataflow, AST, LSP) |
| 23-24 | 1-2 hours | MCP integration |
| 25-29 | 2-3 hours | Visibility and testing (glass box, shadow, harness) |
| 30-31 | 1-2 hours | Integration sweep and test bed validation |
| 32-37 | 3-4 hours | Autopoiesis (Ouroboros, Nemesis, Thunderdome, Dream, Evolution) |
| 38-40 | 2-3 hours | CLI audit, documentation, final verification |
| **41** | **4-8 hours** | **THE ULTIMATE TEST: Autonomous monorepo campaign** |

**Total: 22-38 hours** of Ralph iterations to achieve perfection.

Phase 41 is the longest because it exercises ALL systems in a real-world multi-platform implementation scenario.

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
Choose strength. Choose clarity.
