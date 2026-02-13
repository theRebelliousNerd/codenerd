# internal/autopoiesis/

Self-Modification Capabilities - Enabling codeNERD to evolve itself.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The autopoiesis package implements self-creation capabilities - the ability for codeNERD to detect needs and generate new capabilities at runtime. Named after the biological concept of self-maintaining systems.

## Architecture

```text
Detection → Specification → Safety Check → Compile → Register → Execute
    ↑                                                              |
    └────────── Evaluate → Detect Patterns → Refine ───────────────┘
```

## Structure

```text
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
| `StageSpecification` | Generate tool code via LLM |
| `StageSafetyCheck` | Verify no forbidden imports/calls |
| `StageCompilation` | Compile to standalone binary |
| `StageRegistration` | Register in runtime registry |
| `StageExecution` | Execute with JSON I/O |

## Feedback & Learning

```text
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

### Forbidden Imports

| Import | Reason |
|--------|--------|
| `unsafe` | Memory safety |
| `syscall` | System calls |
| `os/exec` | Command execution |
| `net/http` | Networking |

### Forbidden Calls

- `os.RemoveAll` - Recursive deletion
- `os.Remove` - File deletion
- `unsafe.Pointer` - Unsafe pointers

## Directory Structure

```text
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
```

---

**Last Updated:** December 2024


> *[Archived & Reviewed by The Librarian on 2026-01-25]*