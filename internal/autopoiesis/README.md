# internal/autopoiesis/

Self-Modification Capabilities - Enabling codeNERD to evolve itself.

**Architecture Version:** 2.1.0 (August 2026 — JIT-driven, single audited creation path)

Corpus: [`Docs/architecture/autopoiesis/`](../../Docs/architecture/autopoiesis/) is the
living reference; this file is the package-level orientation.

## Overview

The autopoiesis package implements self-creation capabilities - the ability for codeNERD to detect needs and generate new capabilities at runtime. Named after the biological concept of self-maintaining systems.

## Architecture

```
Detection → Specification → Safety Check → Compile → Register → Execute
    ↑                                                              |
    └────────── Evaluate → Detect Patterns → Refine ───────────────┘
```

## Structure

```
autopoiesis/
├── autopoiesis.go          # Package marker (modularized)
├── autopoiesis_*.go        # Modular orchestrator files (10 files)
├── complexity.go           # Campaign need detection
├── persistence.go          # Persistent agent detection
├── tool_detection.go       # Missing capability detection
├── tool_generation.go      # LLM-based tool creation
├── ouroboros.go            # Self-generation state machine
├── feedback.go             # Learning from executions
├── thunderdome.go          # Adversarial testing arena
├── panic_maker.go          # Attack vector generation
├── checker.go              # Safety policy validation
├── profiles.go             # Tool quality profiles
├── traces.go               # Reasoning trace capture
└── prompt_evolution/       # System Prompt Learning (SPL)
```

## Core Capabilities

| Capability | Description |
|------------|-------------|
| **Complexity Analysis** | Detect when campaigns are needed |
| **Tool Generation** | Create new tools for missing capabilities |
| **Ouroboros Loop** | Full tool self-generation cycle |
| **Feedback & Learning** | Evaluate tool quality, improve over time |
| **Thunderdome** | Adversarial testing arena |
| **Prompt Evolution** | Automatic prompt improvement (SPL) |

## The Ouroboros Loop

Named after the serpent eating its own tail - enables runtime tool generation.

| Stage | Description |
|-------|-------------|
| `StageDetection` | Detect missing capability via Mangle |
| `StageSpecification` | Generate tool code via LLM (regenerates with safety feedback on retry) |
| `StageSafetyCheck` | `go_safety.mg` audit over AST facts; **fails closed** if the policy cannot load |
| `StageThunderdome` | PanicMaker attacks run against a compiled arena binary |
| `StageSimulation` | Mangle transition/stagnation gate (`schemas_state.mg`) |
| `StageCompilation` | Compile to standalone binary (runs the generated tests first) |
| `StageRegistration` | Register in runtime registry + assert kernel facts |
| `StageExecution` | Execute with JSON I/O |

### One creation path

`Orchestrator.ExecuteOuroborosLoop` (and the thin wrappers `GenerateTool`,
`ExecuteAction`, `GenerateToolWithTracing`) is the **only** production route to
a registered tool. `WriteAndRegisterTool` is an unaudited test/diagnostic seam.
`tool_creation_routing_test.go` fails the build if a new caller reaches
`ToolGenerator` directly, and campaign pre-generation is pinned to the same
`Execute` entry point so it runs at the same safety depth as chat.

## Feedback & Learning

```
Execute Tool → Evaluate Quality → Detect Patterns → Refine Tool
      ↑                                                  |
      └────────────────────────────────────────────────→┘
```

### Quality Dimensions

| Dimension | Description |
|-----------|-------------|
| **Completeness** | Did we get ALL available data? |
| **Accuracy** | Was output correct and well-formed? |
| **Efficiency** | Resource usage and execution time |
| **Relevance** | Was output relevant to user's intent? |

## Thunderdome

Adversarial testing arena where tools fight attack vectors:

| Attack Type | Description |
|-------------|-------------|
| `memory_exhaustion` | Unbounded memory allocation |
| `nil_deref` | Trigger nil pointer dereference |
| `race_condition` | Concurrent access without sync |
| `malformed_input` | Invalid/malicious input data |

## Safety Features

Generated code is gated by an **allowlist**, not a blocklist: anything not on
`SafetyChecker.buildAllowedPackages()` is a violation.

| Import | Status |
|--------|--------|
| `unsafe`, `reflect`, `plugin`, `syscall`, `C` | Never allowed |
| `net`, `net/http` | Off unless `AllowNetworking` |
| `os`, `path/filepath` | Off unless `AllowFileSystem` (on by default) |
| `os/exec` | Off unless `Config.AllowToolExec` — **default deny** |

`AllowToolExec` defaults to false: `go_safety.mg` has no call-level exec rule,
so an allowlisted `os/exec` is an unrestricted shell running with the user's
workspace as its working directory.

If `go_safety.mg` cannot be loaded (or loads empty), the checker enters
**fail-closed** mode and rejects every tool. An unaudited tool is not a safe
tool.

### Not yet enforced

`os.RemoveAll` / `os.Remove` / `unsafe.Pointer` aliases are *tracked* by
`astFactEmitter.handleAssignment`, but `go_safety.mg` has no dangerous-call
rule, so they produce no violation on their own under `AllowFileSystem`. See
`ViolationDangerousCall` in `checker_failclosed_test.go`.

## Execution policy

Compiled binaries are the default and the only mode with a process boundary, a
scrubbed environment and a hard context kill. `OuroborosConfig.ExecutionMode =
ExecuteInterpreted` switches to the in-process Yaegi sandbox (no Go toolchain
required); it derives its import allowlist from the same `SafetyChecker` and
additionally strips ambient-authority packages.

## Metrics

`Orchestrator.ExportMetrics()` returns generation latency (mean/max), reject
rate, safety-violation rate, panic rate and Thunderdome kill/entry rates.
Latency is recorded for rejected runs too.

## Directory Structure

```
.nerd/tools/
├── context7_docs.go        # Generated source
├── context7_docs_test.go   # Generated tests
├── .compiled/              # Compiled binaries
├── .learnings/             # Persisted learnings
├── .profiles/              # Quality profiles
└── .traces/                # Reasoning traces
```

## Testing

```bash
go test ./internal/autopoiesis/...
go test ./internal/autopoiesis/prompt_evolution/...
```

Invariants that are enforced as tests rather than prose:

| Test | Invariant |
|------|-----------|
| `tool_creation_routing_test.go` | no production caller bypasses the Ouroboros pipeline |
| `checker_failclosed_test.go` | policy load failure denies; every `ViolationType` is classified |
| `kernel_parity_test.go` | registry tool count matches `tool_registered` facts after boot |
| `kernel_listener_wiring_test.go` | every interactive boot path starts the delegation listener |
| `ouroboros_multistage_e2e_test.go` | safety fail → regenerate → survive arena → compile → run |

---

**Last Updated:** August 2026
