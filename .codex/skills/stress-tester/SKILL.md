---
name: stress-tester
description: Evidence-driven stress, race, chaos, assault-campaign, recovery, and Mangle-adversarial testing for codeNERD. Use when validating stability, reproducing panics or resource failures, exercising campaign and kernel boundaries, analyzing stress logs, or proving a root-cause fix under load.
metadata:
  version: 3.0.0
---

# codeNERD Stress Tester

Use this skill to turn a stability claim into a reproducible command, a persisted receipt, and—when something breaks—a minimized root cause with a regression test. The active workflow is Windows-safe, repository-grounded, and intentionally separates deterministic test profiles from live LLM-backed campaigns.

## Non-negotiable contract

1. Read the current repository and scoped `agents.md` files before selecting a profile.
2. Run preflight before load. Never treat a missing tool, absent log stream, or unconfigured provider as a product pass.
3. Establish a baseline with the narrowest relevant deterministic profile.
4. Increase one pressure dimension at a time: repetition, concurrency/race, input size, duration, then cross-system composition.
5. Persist commands, exit codes, duration, and bounded output under `.nerd/campaigns/stress-tester/<run-id>/`.
6. Report `PASS`, `FAIL`, `PARTIAL`, `BLOCKED`, or `NO_SIGNAL`; do not turn “the command ran” into “the system is healthy.”
7. On failure, minimize the reproducer, identify the owning invariant, add or name the regression, and rerun the exact failing profile.
8. Never delete user data, reset Git state, force-push, kill unrelated processes, or launch unbounded load as part of a stress run.

## Active control plane

All paths are relative to the repository root.

```powershell
# Inspect the host and repository without changing them.
python .codex/skills/stress-tester/scripts/preflight.py

# Validate the skill package, agent wiring, profiles, and adversarial corpus.
python .codex/skills/stress-tester/scripts/validate_skill.py

# Preview an exact profile. Execution is opt-in and bounded.
python .codex/skills/stress-tester/scripts/run_suite.py --profile smoke

# Execute and persist the receipt.
python .codex/skills/stress-tester/scripts/run_suite.py --profile smoke --execute

# Analyze current logs and emit Markdown plus a JSON sidecar.
python .codex/skills/stress-tester/scripts/analyze_stress_logs.py --output .nerd/campaigns/stress-tester/log-analysis.md

# Assert a persisted receipt without re-running the profile.
python .codex/skills/stress-tester/scripts/assert_receipt.py .nerd/campaigns/stress-tester/<run-id>/run.json --expect-verdict passed --no-critical-output

# Inventory intentionally-invalid Mangle fixtures; add --execute to prove rejection.
python .codex/skills/stress-tester/scripts/verify_adversarial.py
```

`references/profile-registry.json` is the machine-readable source of truth for deterministic profiles. Do not invent a command from memory when a registered profile covers the target.

## Profile selection

| Profile | Use it for | Pressure | External LLM |
|---|---|---|---|
| `structural` | Skill and control-plane integrity | parser/config/files | No |
| `build` | Compile the current CLI with repository headers | bounded build | No |
| `smoke` | Fast pre-handoff confidence | build + key packages | No |
| `kernel` | Mangle, policy, VirtualStore, executive behavior | focused Go tests | No |
| `campaign` | assault orchestration and chat routing | focused Go tests | No |
| `race` | shared-state and lifecycle concurrency | Go race detector | No |
| `full` | repository-wide regression | `go test ./...` | No |

Use `--list-profiles` for the exact current registry. The runner enforces per-command timeouts, caps captured output, stops a profile after a failure by default, and never executes merely because it was invoked.

## Live assault campaigns

The repository’s supported assault surface is the interactive chat command, not a fabricated Cobra subcommand:

```text
/campaign assault subsystem internal/core --race --vet --batch 25 --cycles 3
```

Natural language such as `run an assault campaign on internal/core` is also routed by `cmd/nerd/chat/campaign_assault.go`. Results persist under `.nerd/campaigns/<campaign>/assault/`.

Before a live campaign:

- build successfully and run the `campaign` deterministic profile;
- verify provider/auth state and the intended workspace;
- choose a bounded subsystem before whole-repository scope;
- state whether `--race`, `--vet`, nemesis review, cycles, and batch size are enabled;
- monitor `targets.json`, `batches/`, `results/`, `logs/`, and `triage/latest.json`.

A live campaign can incur model usage and mutate code through remediation tasks. Run it only when the user’s request authorizes that scope. Do not launch it as an incidental validation step.

## Escalation ladder

### 1. Baseline

Run `preflight`, then `build`, `smoke`, or the narrower owning profile. Save the receipt even if it passes.

### 2. Repetition

Use `--repeat N` on a deterministic profile. Repetition proves recurrence; it does not replace race instrumentation.

### 3. Concurrency

Use the `race` profile. For a newly isolated package, add a bounded profile entry rather than manually spawning untracked background jobs.

### 4. Adversarial inputs

Use `verify_adversarial.py --execute` for intentionally-invalid Mangle fixtures. A passing check means every invalid fixture was rejected; parser crashes, hangs, or accepted-invalid files fail the profile. Counts are discovered from the corpus at runtime and are never hard-coded.

### 5. Cross-system assault

Use the interactive assault campaign only after deterministic campaign routing is green. Start with one subsystem and one cycle; expand based on evidence.

### 6. Soak and recovery

Define duration, expected throughput, stop conditions, artifact path, and cleanup before starting. Observe memory and goroutine trends rather than a single end value. If a panic produces `debug_program_ERROR.mg` or a flight trace, preserve it before reproduction.

## Failure protocol

For each failure, capture:

- profile/run ID and exact command;
- first failing iteration, exit code, duration, and timeout state;
- smallest input/package that reproduces it;
- first causal error, not only the last cascade message;
- suspected owner and violated invariant;
- regression test or a precise reason one cannot yet be added;
- rerun result after repair.

Use the following distinction:

- **Product failure:** an assertion, panic, race, invariant violation, corrupt artifact, or bounded command timeout attributable to codeNERD.
- **Harness failure:** malformed profile, broken helper, unsupported command, missing parser, or receipt-write error.
- **Environment block:** missing compiler/header/provider, resource policy, or access constraint.
- **No signal:** no relevant logs/events were produced, or the exercised path never reached the claimed subsystem.

Do not patch the symptom at a downstream formatting or retry layer when the evidence points to an upstream lifecycle, logic, ownership, or scoping invariant.

## Log analysis

`analyze_stress_logs.py` reuses the live log-analyzer parser from `.codex/skills/log-analyzer/scripts/parse_log.py`. It reads `.nerd/logs/*.log`, classifies critical/resource/concurrency signals, and emits bounded evidence with a JSON sidecar. It does not claim that a substring query proves a component was exercised.

Interpretation rules:

- zero critical events plus zero relevant events is `NO_SIGNAL`, not `PASS`;
- warnings are evidence to triage, not automatic failures;
- panic, fatal runtime error, out-of-memory, concurrent map failure, and race-detector evidence are failures;
- timeouts require attribution: an expected cancellation is not the same as a deadlock;
- compare timestamps to the test window so old logs cannot contaminate the verdict.

Use the `log-analyzer` skill for multi-category causal chains or Mangle queries beyond the bounded stress summary.

## Scenario library

`references/workflows/` contains historical scenario designs. They are an advisory threat-model library, not a copy/paste command contract. Some describe older CLI forms or Unix shells. Consult `references/workflows/README.md`, verify every referenced command against current source/help, and promote a scenario into `profile-registry.json` only after it has a bounded executable contract.

**09-cli-workspace-matrix (2026-07 live matrix — 6 workflows):** Prefer PowerShell procedures under `references/workflows/09-cli-workspace-matrix/`. Includes workspace isolation, full CLI surface, polyglot vehicle, **one-shot-cli-exit** (maintenance cancel + Close 8s bounds / e18d6818), **dual-llm-routing** (main vs worker Ollama vs Gemini Nano Banana 2), and **define-agent-flags** (`--name` / `--topic` required). Catalog counts live in `.agents` / `.claude` skill SKILL.md (35 total workflows).

Supporting references:

- `references/testing-strategy.md` — pressure dimensions, oracles, and stopping rules.
- `references/artifact-contract.md` — receipt schema and handoff expectations.
- `references/panic-catalog.md` — symptom-to-owner hints; verify against live code (P0/P0c Close timeouts).
- `references/resource-limits.md` — historical limits inventory; measure current defaults before citing values.
- `references/subsystem-stress-points.md` — broad threat model, not authoritative commands.
- `assets/mangle-adversarial/` — intentionally-invalid negative fixtures.

## Completion gate

A stress-testing task is complete only when:

- the selected profile matches the user’s named subsystem and risk;
- preflight and harness validation are recorded;
- every executed command has an exit code and duration;
- failures are classified and minimized, or explicitly remain unresolved;
- a passing rerun exists for any implemented repair;
- the report names unexercised surfaces and environmental limitations;
- repository changes, including stress artifacts, are intentional and scoped.

Keep the final report compact: verdict, measured scope, failures/root causes, artifact path, and remaining risk.
