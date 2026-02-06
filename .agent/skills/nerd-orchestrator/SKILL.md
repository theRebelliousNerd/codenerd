---
name: nerd-orchestrator
description: Master orchestration skill for codeNERD. Use to navigate the architecture, run CLI commands, execute stress tests, and analyze logs in a unified loop.
---

# codeNERD Orchestrator: Operator's Manual

## Overview

You are the pilot of the `codeNERD` system. This skill is your flight manual. Do not guess commands; use the references below to drive the system precisely.

**Core Workflow:**
1.  **Navigate** (Know where you are)
2.  **Operate** (Run CLI commands)
3.  **Stress** (Execute test scenarios)
4.  **Analyze** (Query logs for truth)

## 1. Architecture & CLI

*   **[Architecture Map](references/architecture_map.md)**: Breakdown of `cmd/`, `internal/`, and `.nerd/` directories.
*   **[CLI Commands](references/cli_commands.md)**: Reference for `nerd spawn`, `nerd campaign`, `nerd scan`, etc.

## 2. Stress Testing (The "How-To")

Don't write scripts. Execute these patterns directly in the terminal to validate the system.

*   **[Stress Playbook](references/stress_playbook.md)**: Step-by-step commands for:
    *   **Queue Saturation**: Testing backpressure and concurrency.
    *   **API Scheduler**: Forcing slot contention (5 slots vs 12 shards).
    *   **Mangle Self-Healing**: Injecting invalid logic to test auto-repair.
    *   **Campaign Marathon**: Testing long-running state persistence.

## 3. Log Forensics (The "Why")

When something breaks, don't just read the last line of the log. Use the Mangle Analyzer.

*   **[Log Forensics Guide](references/log_analysis.md)**:
    *   How to parse logs into facts (`parse_log.py`).
    *   How to query facts (`logquery.exe`).
    *   **Copy-Paste Queries** for: Panics, Deadlocks, Slot Leaks, and Schema Drift.

## Quick Start: The Validation Loop

**1. Build (CGO Required)**
```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build -o nerd.exe ./cmd/nerd
```

**2. Clean**
```powershell
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

**3. Run a Test (Example: Conservative Queue Test)**
```powershell
./nerd.exe spawn coder "write hello world"
```

**4. Analyze**
```powershell
python .agent/skills/log-analyzer/scripts/parse_log.py .nerd/logs/* --no-schema > debug.mg
.agent/skills/log-analyzer/scripts/logquery/logquery.exe debug.mg -i
# Then type: ?panic_detected(T, C, M)
```