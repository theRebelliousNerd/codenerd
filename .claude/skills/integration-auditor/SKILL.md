---
name: integration-auditor
description: Comprehensive audit and repair of integration wiring across all 39+ codeNERD systems. Use when implementing new features, creating shards, debugging "code exists but doesn't run" issues, or verifying system integration.
---

# Integration Auditor

Systematic verification that all codeNERD components are properly wired through the 39+ integration points.

## Core Principle: Wire, Don't Remove

**CRITICAL:** When finding unused code (channels, fields, functions, parameters), the correct response is to **wire it up**, not remove it.

codeNERD is a living codebase where integration gaps are common. Unused code usually represents:

1. **Planned functionality** that was never connected
2. **Infrastructure** waiting for consumers
3. **API contracts** that should be honored

**Decision tree for unused code:**

```text
Unused code found?
├─ Is it clearly obsolete? (old API, deprecated pattern)
│   └─ Yes → Remove with explanation
└─ No → WIRE IT UP
    ├─ Unused channel → Add goroutine to consume it
    ├─ Unused field → Find where it should be assigned/read
    ├─ Unused parameter → Pass meaningful data to it
    └─ Unused function → Find where it should be called
```

**Example - the wrong approach:**

```go
// Found: progressChan never read
// WRONG: Remove progressChan
// This silently discards real-time event streaming
```

**Example - the correct approach:**

```go
// Found: progressChan never read
// CORRECT: Add consumer goroutine
go func() {
    for progress := range progressChan {
        m.handleProgress(progress)  // Wire to UI/logging
    }
}()
```

**Before removing ANY code, ask:**

1. Why was this code written in the first place?
2. What functionality does it enable?
3. Where should it be connected?

**Only remove code when:**

- It's genuinely dead (no possible use case)
- It's been superseded by a different implementation
- Keeping it would cause confusion or maintenance burden

---

## Overview

codeNERD's power comes from the integration of 39+ distinct systems. A feature can have perfect implementation but fail silently if any integration point is missed. This skill provides:

- **Diagnostic tooling** - Automated wiring scanner (`audit_wiring.py`)
- **Audit workflows** - Step-by-step checklists for common scenarios
- **System-specific guides** - Deep dives into each integration point
- **Failure pattern catalog** - Common symptoms and fixes

**When to use this skill:**

- Implementing any new feature
- Creating or modifying shards (Type A/B/U/S)
- Debugging "works in isolation but fails in system"
- Pre-commit integration verification
- Investigating silent failures

---

## Quick Start

### Run the Diagnostic Scripts

```bash
cd .claude/skills/integration-auditor/scripts

# Full audit (all systems)
python audit_wiring.py

# Execution wiring audit (objects run, channels listened, etc.)
python audit_execution.py --verbose

# Audit specific component
python audit_wiring.py --component coder --verbose
python audit_execution.py --component campaign --verbose
```

### Three-Question Audit

Ask these before any commit:

1. **Does it have a path IN?** (CLI command OR transducer verb OR system auto-start)
2. **Does it have a path OUT?** (Facts to kernel OR logging OR articulation)
3. **Does it have dependencies?** (Kernel, LLM, VirtualStore injected)

If any answer is NO, the feature has a wiring gap.

### Severity Levels

- `ERROR` - Critical wiring gap, feature won't work
- `WARNING` - Potential issue, may work but unstable
- `INFO` - Suggestion or optional improvement
- `OK` - Properly wired

---

## The 39+ Integration Systems

Every feature touches multiple systems. Summary table:

| Layer | Systems | Key Files |
|-------|---------|-----------|
| **Kernel (9)** | Schema, Policy, Facts, Query, Virtual, FactStore, Compilation, Trace, Activation | `schemas.mg`, `policy.gl`, `kernel.go` |
| **Shard (8)** | Type A/B/U/S Registration, Injection, Spawning, Messaging, Lifecycle | `registration.go`, `shard_manager.go` |
| **Memory (4)** | RAM, Vector, Graph, Cold | `store/`, SQLite DBs |
| **Executive (6)** | Perception, World Model, Policy, Constitution, Legislator, Router | Type S shards |
| **I/O (6)** | CLI, Transducer, Articulation, Actions, TUI, Session | `commands.go`, `transducer.go` |
| **Orchestration (4)** | Campaign, TDD, Autopoiesis, Piggyback | `campaign/`, `autopoiesis/` |
| **Cross-Cutting (2)** | Logging (22 categories), Config | `logging/`, `.nerd/config.json` |
| **Execution (6)** | Object Run, Channel Listen, Message Handle, Field Assign, Goroutine Spawn, Reference Store | All `.go` files |

**Full details:** See [systems-integration-map.md](references/systems-integration-map.md)

### Boot Sequence

```text
1. Config      → 2. Logging     → 3. Kernel    → 4. Storage
5. VirtualStore → 6. Registration → 7. System Shards → 8. Session
9. World Model  → 10. Ready
```

If a component fails, check that its dependencies booted first.

---

## Audit Workflows

### Workflow 1: New Feature Audit

7 phases covering entry points, core logic, kernel integration, actions, output, testing, and config.

**Full checklist:** [audit-workflows.md#workflow-1-new-feature-audit](references/audit-workflows.md#workflow-1-new-feature-audit)

### Workflow 2: New Shard Audit

Type-specific checklists for all 4 shard types:

| Type | Lifecycle | Memory | Key Requirement |
|------|-----------|--------|-----------------|
| **A** (Ephemeral) | Spawn→Execute→Die | RAM | Basic injection |
| **B** (Persistent) | Created at /init | SQLite | + LearningStore |
| **U** (User) | Via /define-agent | SQLite | + Dynamic profile |
| **S** (System) | Auto-start | RAM | + StartSystemShards() list |

**Full matrix:** [shard-wiring-matrix.md](references/shard-wiring-matrix.md)

### Workflow 3: Debugging "Code Exists But Doesn't Run"

7-step systematic trace:

1. Verify entry point (CLI/Transducer/System)
2. Verify shard registration (Factory/Profile)
3. Verify kernel wiring (Decl/LoadFacts/Rules)
4. Verify action layer (Type/Handler/Permission)
5. Verify output path (Logging/Facts/Piggyback)
6. Verify dependencies (SQLite/LLM/Files)
7. Check tests pass

**Full guide:** [audit-workflows.md#workflow-2-debugging-code-exists-but-doesnt-run](references/audit-workflows.md#workflow-2-debugging-code-exists-but-doesnt-run)

### Workflow 4: Pre-Commit Check

```bash
# 1. Build
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd

# 2. Test
go test ./...

# 3. Audit (both types!)
python audit_wiring.py --verbose
python audit_execution.py --verbose

# 4. Smoke test
./nerd.exe
```

**Full checklist:** [audit-workflows.md#workflow-3-pre-commit-integration-check](references/audit-workflows.md#workflow-3-pre-commit-integration-check)

### Workflow 5: Execution Wiring Audit

Detects "code exists but doesn't execute" - the most insidious bugs:

```bash
python audit_execution.py --verbose
```

**What it catches:**

| Pattern | Issue | Example |
|---------|-------|---------|
| **Object Execution** | `New*()` without `Run()`/`Start()` | `orch := NewOrchestrator()` but no `orch.Run()` |
| **Channel Listeners** | Channels created but never read | `ch := make(chan X)` but no `<-ch` |
| **Message Handlers** | Bubbletea `*Msg` without case handler | `type fooMsg struct{}` but no `case fooMsg:` |
| **Field Assignment** | Struct fields checked but never assigned | `if m.orch != nil` but `m.orch =` never set |
| **Goroutine Spawn** | Blocking calls not in goroutines | `orch.Run(ctx)` without `go` prefix |
| **Reference Storage** | Local vars that should be struct fields | `orch :=` in function, lost on return |

**Real example (the bug that inspired this):**

```go
// cmd/nerd/chat/campaign.go:73-94
func (m Model) startCampaign(goal string) tea.Cmd {
    orch := campaign.NewOrchestrator(config)  // Created
    orch.SetCampaign(result.Campaign)          // Configured
    return campaignStartedMsg(result.Campaign) // But Run() never called!
}
// orch goes out of scope, gets garbage collected
// m.campaignOrch is never assigned
// Campaign NEVER EXECUTES
```

**Full reference:** [execution-wiring.md](references/execution-wiring.md)

---

## 9-Point Integration Checklist

Quick checklist for any new feature:

1. **Logging** - Uses one of 22 categories
2. **Shard Registration** - Factory + Profile + Injection
3. **Kernel/Schema** - Decl + Rules + ToAtom()
4. **Virtual Predicates** - Decl + Get() handler
5. **CLI Commands** - Case in handleCommand()
6. **Transducer** - Verb in VerbCorpus
7. **Actions** - Type constant + Execute() case
8. **Tests** - Unit + Integration + Live
9. **Config** - Struct field + Default

**Full details:** [wiring-checklist.md](references/wiring-checklist.md)

---

## Quick Decision Trees

### "Where should this logging go?"

```text
LLM API calls? → CategoryAPI
Mangle kernel? → CategoryKernel
Specific shard (coder/tester)? → Use shard category
Generic shard ops? → CategoryShards
Perception/transduction? → CategoryPerception
Otherwise → Choose from 22 categories by component
```

### "What shard type should I use?"

```text
Persists knowledge across sessions?
├─ No → Type A (Ephemeral)
└─ Yes
    └─ User-defined at runtime?
        ├─ Yes → Type U (User)
        └─ No
            └─ Core system service?
                ├─ Yes → Type S (System)
                └─ No → Type B (Persistent)
```

### "Why isn't my feature working?"

```text
Entry point logged? → Add logging, check if reached
Shard spawning? → Check registration (factory + profile)
Execute() called? → Check spawn parameters
Kernel wired? → Check Decl, LoadFacts, rules
Actions executing? → Check VirtualStore handlers
Otherwise → Check output path (logging, facts, return)
```

---

## Top 15 Failure Patterns

| Symptom | Root Cause | Fix |
|---------|-----------|-----|
| "unknown shard type" | Factory not registered | Add to registration.go |
| "undeclared predicate" | Missing Decl | Add to schemas.mg |
| Nil pointer in shard | Missing injection | Add Set* calls in factory |
| "permission denied" | Profile lacks permission | Update Permissions list |
| Silent failure | No logging | Add logging statements |
| Facts not querying | LoadFacts not called | Add kernel.LoadFacts() |
| Type B forgets | No LearningStore | Add SetLearningStore() |
| System shard won't start | Not in auto-start | Add to StartSystemShards() |
| Context overflow (400) | Budget exceeded | Lower MemoryLimit |
| Goroutine leak | No Stop() cleanup | Cancel contexts, close channels |
| **Campaign never runs** | **Run() not called** | **Add `go orch.Run(ctx)`** |
| **Orchestrator lost** | **Local var not stored** | **Add `m.orch = orch`** |
| **Progress not shown** | **Channel not listened** | **Add goroutine reading channel** |
| **Message ignored** | **No case handler** | **Add `case fooMsg:` in Update()** |
| **Feature works once** | **Blocking main thread** | **Wrap in `go func()`** |
| **Lost functionality** | **Removed "unused" code** | **Wire it up instead** |

**Full catalog:** [failure-patterns.md](references/failure-patterns.md)

---

## Diagnostic Scripts

### Master Audit (audit_wiring.py)

```bash
python audit_wiring.py           # Full audit (all 7 auditors)
python audit_wiring.py --component coder  # Specific component
python audit_wiring.py --verbose # With suggestions
python audit_wiring.py --json    # For tooling
```

**Runs 7 auditors:**
1. Shard Registration - factory, profile, injection
2. Mangle Schema/Policy - declarations, rules
3. Action Layer - CLI commands, transducer verbs
4. Logging Coverage - category usage
5. Cross-System - boot sequence, dependencies
6. Execution Wiring - Run() calls, channel listeners
7. **Unwired Code** - unused params, fields, interfaces (Wire, Don't Remove!)

### Unwired Code Audit (audit_unwired.py)

**NEW:** Dedicated script for finding code that needs wiring, not removal.

```bash
python audit_unwired.py                      # Full unwired audit
python audit_unwired.py --verbose            # Show INFO findings
python audit_unwired.py --fix-suggestions    # Include wiring suggestions
python audit_unwired.py --component campaign # Focus on component
```

**Detects 10 categories of unwired code:**

| Category | What It Finds | Suggestion |
|----------|---------------|------------|
| `unused_param` | Function params never used | Wire into function logic |
| `unused_field` | Struct fields never accessed | Assign in constructor, use in methods |
| `unimplemented_interface` | Missing interface methods | Implement the method |
| `orphan_channel` | Channels never consumed | Add consumer goroutine |
| `dead_callback` | Callbacks never registered | Register with event system |
| `missing_injection` | DI fields never set | Add to constructor or Set method |
| `unrouted_handler` | Handlers not connected | Register with router |
| `unused_return` | Return values discarded | Capture and handle errors |
| `unused_factory` | New* functions never called | Wire into initialization |
| `incomplete_builder` | Builders without Build() | Call terminal operation |

### Execution Wiring Audit (audit_execution.py)

```bash
python audit_execution.py --verbose
```

**Detects 6 execution patterns:**
- Objects created but Run() never called
- Channels created but never read
- Bubbletea messages without handlers
- Struct fields checked but never assigned
- Blocking operations not in goroutines
- Local variables that should be struct fields

### Exit Codes

- `0` - No errors
- `1` - Errors detected

---

## System-Specific Audits

Detailed audit guides for each subsystem:

| System | What to Check | Reference |
|--------|---------------|-----------|
| **Mangle** | Decls, Rules, Facts, Safety | [system-audits.md#mangle-system-audit](references/system-audits.md#mangle-system-audit) |
| **Storage** | 4 tiers: RAM, Vector, Graph, Cold | [system-audits.md#storage-system-audit](references/system-audits.md#storage-system-audit) |
| **Compression** | Token budgets, Spreading activation | [system-audits.md#compression-system-audit](references/system-audits.md#compression-system-audit) |
| **Autopoiesis** | Ouroboros, Safety, Learning | [system-audits.md#autopoiesis-audit](references/system-audits.md#autopoiesis-audit) |
| **TUI** | Commands, Views, State | [system-audits.md#tui-integration-audit](references/system-audits.md#tui-integration-audit) |
| **Campaign** | Phases, Paging, Recovery | [system-audits.md#campaign-system-audit](references/system-audits.md#campaign-system-audit) |

---

## Live Testing Patterns

Verify end-to-end wiring with proven test patterns:

| Pattern | What It Verifies |
|---------|------------------|
| Full Cortex Boot | Config, kernel, system shards start |
| Shard Lifecycle | Factory→Execute→Facts→Cleanup |
| Transducer NL→Action | Parse→Intent→Facts→next_action |
| Virtual Predicate | Decl→Get()→External data→Facts |
| Piggyback Protocol | Dual-channel user+control output |
| TDD Loop | Test fail→Coder fix→Test pass |
| Type B Persistence | Learn→Shutdown→Boot→Recall |
| Action Permissions | Permission check enforcement |

**Full code examples:** [live-testing-patterns.md](references/live-testing-patterns.md)

---

## References

### Bundled References

| File | Content |
|------|---------|
| [audit-workflows.md](references/audit-workflows.md) | Step-by-step audit workflows |
| [wiring-checklist.md](references/wiring-checklist.md) | 9-point integration checklist |
| [shard-wiring-matrix.md](references/shard-wiring-matrix.md) | Per-type shard requirements |
| [systems-integration-map.md](references/systems-integration-map.md) | All 39 systems detailed |
| [system-audits.md](references/system-audits.md) | System-specific audit guides |
| [live-testing-patterns.md](references/live-testing-patterns.md) | Test code examples |
| [failure-patterns.md](references/failure-patterns.md) | Failure catalog |
| [execution-wiring.md](references/execution-wiring.md) | Execution wiring patterns |

### Related Skills

- **[codenerd-builder](../codenerd-builder/SKILL.md)** - Architecture, transducers, kernel, shards
- **[mangle-programming](../mangle-programming/SKILL.md)** - Datalog syntax, rules, queries
- **[go-architect](../go-architect/SKILL.md)** - Go patterns, error handling
- **[charm-tui](../charm-tui/SKILL.md)** - Bubbletea TUI patterns

### Scripts

- [audit_wiring.py](scripts/audit_wiring.py) - Master orchestrator (runs all 7 auditors)
- [audit_unwired.py](scripts/audit_unwired.py) - Unwired code detection (10 categories, Wire Don't Remove!)
- [audit_execution.py](scripts/audit_execution.py) - Execution wiring detection (6 patterns)
- [audit_shards.py](scripts/audit_shards.py) - Shard registration audit
- [audit_mangle.py](scripts/audit_mangle.py) - Mangle schema/policy audit
- [audit_actions.py](scripts/audit_actions.py) - Action layer audit
- [audit_logging.py](scripts/audit_logging.py) - Logging coverage audit

---

## Summary

Integration audit is not optional. codeNERD's architecture distributes logic across 39+ systems. Missing one wiring point causes silent failures.

**Use this skill to:**

- Run `audit_wiring.py` before every commit
- Follow type-specific checklists for new shards
- Systematically debug "doesn't work" issues
- Verify all 9 integration points are complete

**Key principles:**

1. **Wire, don't remove** - Unused code usually needs connection, not deletion. Ask "why was this written?" before removing anything.
2. **Fix wiring, not code** - Code that works in isolation but fails in the system has a wiring gap.

**Next steps:**

1. Run audit script on your workspace
2. Fix any ERRORs found
3. Review WARNINGs for your component
4. Add integration tests for critical paths
5. Commit with confidence
