# CLI Surface — how to exercise each component

How to put a given codeNERD component on the live path via the `nerd` CLI. Grows
as new invocations are learned. Always `CGO_CFLAGS=-IC:/CodeProjects/codeNERD/sqlite_headers`
in the environment; rebuild after any Go/`.mg` change.

## One-shot invocations

| Command | Exercises | Notes |
|---------|-----------|-------|
| `nerd run "<instruction>"` | perception → kernel → JIT prompt → coder/reviewer subagent → tools → validators | One-shot OODA loop; exits after. Routes edit/commit intent to the coder path. Best for `internal/features`, `internal/tools/core` self-improvement. |
| `nerd campaign start "<goal>" --type audit --timeout 30m` | full orchestrator: decompose → phases (DAG) → tasks → checkpoints (`/shard_validation`, `/manual_review`, `/nemesis_gauntlet`) | Longest-horizon exercise; drives world scan, advisory board, risk gating, journal, state persistence. |
| `nerd commit "<msg>"` | raw `git add -A` + commit | No LLM. Tests the git action path only. |
| `nerd campaign start ... --type greenfield\|feature\|migration\|remediation` | decomposer variants; build/verify (`/verify` = `go build`) vs analytical paths | Type changes objective inference and checkpoint mix. |
| `nerd -w <dir> ...` | workspace isolation / `CODENERD_WORKSPACE_ROOT` containment | Write-set containment, `-w` vs CWD. |

## Campaign flags that matter

- `--type` — greenfield · feature · audit · migration · remediation. Audit/remediation
  decomposers emit analytical work; greenfield/feature emit file-producing work.
- `--timeout` — default 25m; use 30m+ for real audits. Kernel holds 5k–10k facts;
  set long timeouts.
- `--disable-system-shard world` — makes world-scan failures non-fatal (diagnostic
  only; the goal is to *fix* world, not disable it).
- `-v` — verbose logging.

## Watching a run live

```
# Clear logs first (archive-move if the delete glob is guard-blocked), then:
Monitor on .nerd/logs/<date>_campaign.log filtered to:
  Phase completed | Checkpoint PASSED | Checkpoint FAILED | Persisted durable output |
  generation_degraded | research_empty | Campaign completed | campaign failed |
  phase blocked | all_tasks_blocked | panic | out of memory | fatal error | traceRegion
```

Cross-reference the per-subsystem logs (all under `.nerd/logs/<date>_*.log`):
`campaign`, `session`, `shards`, `tools`, `virtual_store`, `articulation`,
`perception`, `kernel`, `jit`, `world`, `store`, `context`, `boot`, `performance`,
`audit` (JSON events). If the system crashes it dumps a combined
`debug_program_ERROR.mg` — that means a Mangle failure.

## Reading verdicts

- **Infra failure** (`executable file not found`, `command timed out`, `tool execution
  failed`) → the shard couldn't run its probe. Fix tooling/steering, not content.
- **Substantive verdict** (`## Phase Review: ...`, `## Verdict: FAIL/PASS`) → the shard
  completed and judged the work. Treat as a real code review of the executor: it names
  the failing task and why. Fix the executor gap it points at.
