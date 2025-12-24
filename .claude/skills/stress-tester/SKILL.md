---
name: stress-tester
description: Live stress testing of codeNERD via CLI. Use when testing system stability, finding panics, edge cases, and failure modes across all 31+ subsystems. Includes 35 comprehensive workflows with conservative, aggressive, chaos, and hybrid severity levels. Features extensive Mangle self-healing validation and new system stress tests (MCP, Prompt Evolution, LLM Providers).
---

# Stress Tester

Live stress testing for codeNERD that systematically pushes all subsystems to their limits via CLI commands. Unlike unit tests, these are extensive end-to-end scenarios designed to find panics, race conditions, resource exhaustion, and edge cases.

**When to use:**

- Pre-release stability verification
- After major architectural changes
- Debugging intermittent failures
- Validating resource limits
- Finding panic vectors
- Testing Mangle self-healing systems

## Quick Start

### 1. Build codeNERD

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd
```

### 2. Clear Logs & Pick a Workflow

```bash
rm .nerd/logs/*
```

| Quick Test | Category | Duration |
|------------|----------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | Kernel | 10-15 min |
| [api-scheduler-stress.md](references/workflows/01-kernel-core/api-scheduler-stress.md) | API | 25-45 min |
| [mangle-self-healing.md](references/workflows/01-kernel-core/mangle-self-healing.md) | Mangle | 15-30 min |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | Shards | 15-25 min |

### 3. Analyze Results

```bash
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py
```

**Note:** If system bugs out, combined Mangle schema dumps to `C:\CodeProjects\codeNERD\debug_program_ERROR.mg`. Set long timeouts (5000-10000 facts in kernel).

## Workflow Catalog

**Total: 35 workflows across 9 categories**

### 01-kernel-core (8 workflows)

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | SpawnQueue backpressure | 10-25 min |
| [api-scheduler-stress.md](references/workflows/01-kernel-core/api-scheduler-stress.md) | APIScheduler slot contention | 25-45 min |
| [mangle-explosion.md](references/workflows/01-kernel-core/mangle-explosion.md) | Derivation explosion | 15-30 min |
| [memory-pressure.md](references/workflows/01-kernel-core/memory-pressure.md) | 250k facts, compression | 20-40 min |
| [concurrent-derivations.md](references/workflows/01-kernel-core/concurrent-derivations.md) | 4 shards querying kernel | 10-20 min |
| [mangle-self-healing.md](references/workflows/01-kernel-core/mangle-self-healing.md) | PredicateCorpus, MangleRepairShard | 15-30 min |
| [mangle-startup-validation.md](references/workflows/01-kernel-core/mangle-startup-validation.md) | Boot-time validation | 10-30 min |
| [mangle-failure-modes.md](references/workflows/01-kernel-core/mangle-failure-modes.md) | 69 AI failure modes | 30-60 min |

### 02-perception-articulation (3 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [intent-fuzzing.md](references/workflows/02-perception-articulation/intent-fuzzing.md) | Malformed NL, adversarial strings |
| [piggyback-corruption.md](references/workflows/02-perception-articulation/piggyback-corruption.md) | Truncated JSON, invalid ControlPackets |
| [taxonomy-exhaustion.md](references/workflows/02-perception-articulation/taxonomy-exhaustion.md) | Every verb + unknown verbs |

### 03-shards-campaigns (4 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [campaign-marathon.md](references/workflows/03-shards-campaigns/campaign-marathon.md) | 50-phase campaign, 500 tasks |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | All shard types rapidly |
| [tdd-infinite-loop.md](references/workflows/03-shards-campaigns/tdd-infinite-loop.md) | Always-failing test, repair loop |
| [reviewer-finding-explosion.md](references/workflows/03-shards-campaigns/reviewer-finding-explosion.md) | 1000+ issues |

### 04-autopoiesis-ouroboros (3 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [tool-generation-nesting.md](references/workflows/04-autopoiesis-ouroboros/tool-generation-nesting.md) | Nested tool generation |
| [thunderdome-battle.md](references/workflows/04-autopoiesis-ouroboros/thunderdome-battle.md) | 100 attack vectors |
| [safety-checker-bypass.md](references/workflows/04-autopoiesis-ouroboros/safety-checker-bypass.md) | Forbidden imports |

### 05-world-context (3 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [large-codebase-scan.md](references/workflows/05-world-context/large-codebase-scan.md) | 10k+ files, symlinks |
| [context-compression.md](references/workflows/05-world-context/context-compression.md) | 100+ turn conversation |
| [holographic-impact.md](references/workflows/05-world-context/holographic-impact.md) | Massive change set impact |

### 06-advanced-features (3 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [dream-state-load.md](references/workflows/06-advanced-features/dream-state-load.md) | 4 consultants x 100 perspectives |
| [shadow-mode-stress.md](references/workflows/06-advanced-features/shadow-mode-stress.md) | Complex simulation + rollback |
| [browser-automation.md](references/workflows/06-advanced-features/browser-automation.md) | 50 concurrent page fetches |

### 07-full-system-chaos (3 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [everything-at-once.md](references/workflows/07-full-system-chaos/everything-at-once.md) | All subsystems simultaneously |
| [long-running-session.md](references/workflows/07-full-system-chaos/long-running-session.md) | 2+ hour stability |
| [recovery-after-panic.md](references/workflows/07-full-system-chaos/recovery-after-panic.md) | Force panic, verify recovery |

### 08-hybrid-integration (4 workflows)

| Workflow | What It Stresses |
|----------|------------------|
| [perception-to-campaign.md](references/workflows/08-hybrid-integration/perception-to-campaign.md) | NL → full campaign |
| [research-to-coder-to-tester.md](references/workflows/08-hybrid-integration/research-to-coder-to-tester.md) | Full shard handoff |
| [ouroboros-thunderdome-nemesis.md](references/workflows/08-hybrid-integration/ouroboros-thunderdome-nemesis.md) | Adversarial evolution loop |
| [full-ooda-loop-stress.md](references/workflows/08-hybrid-integration/full-ooda-loop-stress.md) | Complete OODA cycle |

### 09-new-systems (6 workflows)

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [mcp-jit-compiler.md](references/workflows/09-new-systems/mcp-jit-compiler.md) | MCP tool discovery, JIT selection, skeleton/flesh bifurcation | 20-35 min |
| [prompt-evolution.md](references/workflows/09-new-systems/prompt-evolution.md) | LLM-as-Judge, strategy database, atom generation | 30-50 min |
| [llm-provider-system.md](references/workflows/09-new-systems/llm-provider-system.md) | Multi-provider client, rate limits, failover | 25-40 min |
| [timeout-consolidation.md](references/workflows/09-new-systems/timeout-consolidation.md) | 3-tier timeout hierarchy, deadline propagation | 15-25 min |
| [glass-box-visibility.md](references/workflows/09-new-systems/glass-box-visibility.md) | TUI glass-box rendering, concurrent tool updates | 15-25 min |
| [knowledge-discovery.md](references/workflows/09-new-systems/knowledge-discovery.md) | Semantic Knowledge Bridge, document ingestion, vector search | 20-35 min |

## Severity Levels

| Level | Description | Use When |
|-------|-------------|----------|
| **Conservative** | Within limits, test edge cases | CI/CD, smoke testing |
| **Aggressive** | Approach/exceed limits | Pre-release validation |
| **Chaos** | Random inputs, race conditions | Finding unknown failures |
| **Hybrid** | Multiple subsystems simultaneously | Integration validation |

## Log Analysis

```bash
# Quick analysis
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py

# Manual Mangle query
cd .claude/skills/log-analyzer/scripts
python parse_log.py .nerd/logs/* --no-schema > /tmp/stress.mg
cd logquery && ./logquery.exe /tmp/stress.mg --builtin errors
```

### Key Stress Queries

| Query | Purpose |
|-------|---------|
| `panic_detected(T, C, M)` | Panic events with stack traces |
| `oom_event(T, M)` | Out of memory events |
| `gas_limit_hit(T, M)` | Mangle gas limit exceeded |
| `repair_attempt(T, M)` | MangleRepairShard activity |
| `healing_critical(T, Type, M)` | Critical self-healing issues |

## Success Criteria

Every stress test should verify:

- [ ] No panics in logs (`grep -i "panic" .nerd/logs/*.log`)
- [ ] Memory stayed within limits
- [ ] All commands completed
- [ ] No orphaned goroutines
- [ ] Data integrity maintained
- [ ] Recovery after any failures

## Root-Cause Mandate

> **NO BAND-AIDS ALLOWED.** When stress testing reveals broken artifacts that codeNERD created, the artifact is NOT the bug - it is a SYMPTOM of a deeper systemic failure. Deleting, commenting out, or patching the artifact is strictly forbidden.

See [root-cause-mandate.md](references/root-cause-mandate.md) for complete investigation protocol and anti-pattern catalog.

### Quick Reference

| DO NOT | DO |
|--------|-----|
| Comment out broken rule | Find why autopoiesis generated invalid rule |
| Delete corrupted fact | Find why fact was written incorrectly |
| Manually fix config | Trace config generation code path |

## Reference Documentation

| Reference | Contents |
|-----------|----------|
| [subsystem-stress-points.md](references/subsystem-stress-points.md) | All 31+ subsystems with failure modes |
| [panic-catalog.md](references/panic-catalog.md) | Known panic vectors with triggers |
| [resource-limits.md](references/resource-limits.md) | Config limits and safe/dangerous values |
| [root-cause-mandate.md](references/root-cause-mandate.md) | Investigation protocol, anti-patterns |

## Test Assets

- `assets/cyclic_rules.mg` - Rules causing derivation explosion
- `assets/stress_queries.mg` - Extended log analysis queries
- `assets/mangle-adversarial/` - 264+ invalid Mangle patterns (30 error types)
- `scripts/fixtures/` - Input generators for synthetic projects

## Testing Strategy

1. **Conservative smoke test** - 1 workflow from each category
2. **Aggressive on failures** - Re-run failed workflows at aggressive level
3. **Chaos for unknowns** - Find novel failure patterns
4. **Hybrid for integration** - Test cross-subsystem interactions

### Mangle Self-Healing Priority

- Run `mangle-self-healing.md` before any major release
- Run `mangle-startup-validation.md` after schema changes
- Monitor `healing_critical` events in production logs
