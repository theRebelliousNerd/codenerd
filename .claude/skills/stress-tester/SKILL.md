---
name: stress-tester
description: Live stress testing of codeNERD via CLI. Use when testing system stability, finding panics, edge cases, and failure modes across all 25+ subsystems. Includes comprehensive multi-minute workflows with conservative, aggressive, chaos, and hybrid severity levels.
---

# Stress Tester

## Overview

Live stress testing skill for codeNERD that systematically pushes all subsystems to their limits via CLI commands. Unlike unit tests, these are extensive end-to-end scenarios designed to find panics, race conditions, resource exhaustion, and edge cases across the entire system.

**When to use:**

- Pre-release stability verification
- After major architectural changes
- Debugging intermittent failures
- Validating resource limits
- Finding panic vectors

## Quick Start

### 1. Build codeNERD

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd
```

### 2. Clear Logs

```bash
rm .nerd/logs/*
```

### 3. Pick a Workflow

Start with a conservative test from any category:

| Quick Test | Category | Duration |
|------------|----------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | Kernel | 10-15 min |
| [intent-fuzzing.md](references/workflows/02-perception-articulation/intent-fuzzing.md) | Perception | 15-20 min |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | Shards | 15-25 min |

### 4. Analyze Results

```bash
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py
```

## Workflow Catalog

### 01-kernel-core (4 workflows)

Tests the Mangle kernel, SpawnQueue, and core runtime.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | SpawnQueue backpressure with 100+ spawn requests | 10-25 min |
| [mangle-explosion.md](references/workflows/01-kernel-core/mangle-explosion.md) | Cyclic rules + large EDB causing derivation explosion | 15-30 min |
| [memory-pressure.md](references/workflows/01-kernel-core/memory-pressure.md) | Load 250k facts, trigger emergency compression | 20-40 min |
| [concurrent-derivations.md](references/workflows/01-kernel-core/concurrent-derivations.md) | 4 shards querying kernel simultaneously | 10-20 min |

### 02-perception-articulation (3 workflows)

Tests NL parsing, intent classification, and response formatting.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [intent-fuzzing.md](references/workflows/02-perception-articulation/intent-fuzzing.md) | Malformed NL inputs, adversarial strings, edge case verbs | 15-25 min |
| [piggyback-corruption.md](references/workflows/02-perception-articulation/piggyback-corruption.md) | Truncated JSON, invalid ControlPackets | 10-20 min |
| [taxonomy-exhaustion.md](references/workflows/02-perception-articulation/taxonomy-exhaustion.md) | Every verb in corpus + unknown verbs | 15-25 min |

### 03-shards-campaigns (4 workflows)

Tests shard lifecycle, campaigns, and TDD loops.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [campaign-marathon.md](references/workflows/03-shards-campaigns/campaign-marathon.md) | 50-phase campaign with 500 tasks | 45-90 min |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | Spawn all shard types rapidly | 15-25 min |
| [tdd-infinite-loop.md](references/workflows/03-shards-campaigns/tdd-infinite-loop.md) | Test that always fails, repair loop stress | 20-30 min |
| [reviewer-finding-explosion.md](references/workflows/03-shards-campaigns/reviewer-finding-explosion.md) | Large codebase with 1000+ issues | 20-30 min |

### 04-autopoiesis-ouroboros (3 workflows)

Tests self-modification, tool generation, and adversarial testing.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [tool-generation-nesting.md](references/workflows/04-autopoiesis-ouroboros/tool-generation-nesting.md) | Tool that generates tool that generates tool | 20-35 min |
| [thunderdome-battle.md](references/workflows/04-autopoiesis-ouroboros/thunderdome-battle.md) | 100 attack vectors against generated tools | 25-40 min |
| [safety-checker-bypass.md](references/workflows/04-autopoiesis-ouroboros/safety-checker-bypass.md) | Forbidden imports, dangerous operations | 15-25 min |

### 05-world-context (3 workflows)

Tests filesystem scanning, context building, and impact analysis.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [large-codebase-scan.md](references/workflows/05-world-context/large-codebase-scan.md) | 10k+ files, symlink loops, deep nesting | 25-40 min |
| [context-compression.md](references/workflows/05-world-context/context-compression.md) | 100+ turn conversation, emergency compression | 20-30 min |
| [holographic-impact.md](references/workflows/05-world-context/holographic-impact.md) | Impact analysis on massive change set | 20-30 min |

### 06-advanced-features (3 workflows)

Tests dream state, shadow mode, and browser automation.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [dream-state-load.md](references/workflows/06-advanced-features/dream-state-load.md) | 4 consultants x 100 perspectives | 25-40 min |
| [shadow-mode-stress.md](references/workflows/06-advanced-features/shadow-mode-stress.md) | Complex action simulation with rollback | 15-25 min |
| [browser-automation.md](references/workflows/06-advanced-features/browser-automation.md) | 50 concurrent page fetches via rod | 25-40 min |

### 07-full-system-chaos (3 workflows)

Tests system-wide stability under extreme conditions.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [everything-at-once.md](references/workflows/07-full-system-chaos/everything-at-once.md) | All subsystems stressed simultaneously | 60-120 min |
| [long-running-session.md](references/workflows/07-full-system-chaos/long-running-session.md) | 2+ hour session stability | 120+ min |
| [recovery-after-panic.md](references/workflows/07-full-system-chaos/recovery-after-panic.md) | Force panic, verify recovery | 20-30 min |

### 08-hybrid-integration (4 workflows)

Tests cross-subsystem integration under load.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [perception-to-campaign.md](references/workflows/08-hybrid-integration/perception-to-campaign.md) | NL input through full campaign execution | 25-40 min |
| [research-to-coder-to-tester.md](references/workflows/08-hybrid-integration/research-to-coder-to-tester.md) | Full shard handoff pipeline | 30-45 min |
| [ouroboros-thunderdome-nemesis.md](references/workflows/08-hybrid-integration/ouroboros-thunderdome-nemesis.md) | Adversarial tool evolution loop | 35-50 min |
| [full-ooda-loop-stress.md](references/workflows/08-hybrid-integration/full-ooda-loop-stress.md) | Complete OODA cycle under pressure | 40-60 min |

## Severity Levels

Each workflow supports 4 severity levels:

| Level | Description | Use When |
|-------|-------------|----------|
| **Conservative** | Stay within configured limits, test edge cases | Regular CI/CD, smoke testing |
| **Aggressive** | Approach/exceed limits, stress resources | Pre-release validation |
| **Chaos** | Random inputs, race conditions, resource exhaustion | Finding unknown failure modes |
| **Hybrid** | Multiple subsystems stressed simultaneously | Integration validation |

## Log Analysis Integration

After any stress test, analyze logs using the integrated log-analyzer:

```bash
# Quick analysis
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py

# Verbose with custom output
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py -v -o report.md

# Manual query with logquery
cd .claude/skills/log-analyzer/scripts
python parse_log.py .nerd/logs/* --no-schema | grep "^log_entry" > /tmp/stress.mg
cd logquery
./logquery.exe /tmp/stress.mg --builtin errors
./logquery.exe /tmp/stress.mg --builtin kernel-errors
```

### Custom Stress Queries

The skill includes [stress_queries.mg](assets/stress_queries.mg) with predicates for:

- `panic_detected/3` - Panic events with stack traces
- `nil_pointer_error/3` - Nil pointer dereferences
- `oom_event/3` - Out of memory events
- `timeout_event/3` - Operation timeouts
- `queue_full/3` - Queue saturation events
- `gas_limit_hit/3` - Mangle gas limit exceeded
- `critical_issue/3` - Any critical failure

## Test Fixtures

### Mangle Stress Files

- [cyclic_rules.mg](assets/cyclic_rules.mg) - Rules causing derivation explosion
- [stress_queries.mg](assets/stress_queries.mg) - Log analysis queries

### Input Generators

- [generate_large_project.py](scripts/fixtures/generate_large_project.py) - Creates synthetic Go projects
- [malformed_inputs.py](scripts/fixtures/malformed_inputs.py) - Generates fuzzing payloads

### Malformed Data

- [malformed_piggyback.json](assets/malformed_piggyback.json) - Invalid JSON for articulation testing

## Reference Documentation

- [subsystem-stress-points.md](references/subsystem-stress-points.md) - All 25+ subsystems with failure modes
- [panic-catalog.md](references/panic-catalog.md) - Known panic vectors with triggers
- [resource-limits.md](references/resource-limits.md) - Config limits and safe/dangerous values

## Success Criteria

Every stress test should verify:

- [ ] No panics in logs (`grep -i "panic" .nerd/logs/*.log`)
- [ ] Memory stayed within limits
- [ ] All commands completed
- [ ] No orphaned goroutines
- [ ] Data integrity maintained
- [ ] Recovery after any failures

## Common Failure Patterns

| Pattern | Symptom | Check |
|---------|---------|-------|
| Queue saturation | "queue full" errors | `spawn_queue_depth` predicate |
| Gas exhaustion | "gas limit" errors | `gas_limit_hit` query |
| Memory pressure | OOM or slowdown | `memory_usage` predicate |
| Derivation explosion | Long delays, high CPU | `derived_fact_count` query |
| Panic | Process crash | Log files for stack trace |
