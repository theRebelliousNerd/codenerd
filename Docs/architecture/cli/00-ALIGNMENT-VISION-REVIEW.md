# 00 — Alignment & Vision Review: CLI (`cmd/nerd`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `cmd/nerd/` (113 non-test Go files ≈ 38.1k lines; 55 test files ≈ 16.8k lines)

## 1. North-star statement

codeNERD’s CLI is the **human membrane** over a logic-first agent: the user speaks natural language or issues structured commands; the **Mangle kernel** decides what is permitted and what happens next; LLMs act as perception/articulation transducers, not as unconstrained executors.

The CLI must therefore:

1. Boot Cortex correctly (config, logging, stores, system shards, perception).
2. Expose **both** a rich interactive TUI and a non-interactive Cobra command surface.
3. Never become a bypass around `permitted(...)` / Dreamer / policy.
4. Prefer JIT prompt atoms for LLM-facing paths rather than monolithic chat prompts.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Default `RunE` launches chat that routes through perception → kernel → VirtualStore (`cmd/nerd/main.go`, `chat/process.go`, `chat/session_boot.go`) |
| Fact-flow fidelity | **5** | Single-shot `run` and TUI both drive OODA-style instruction handling (`cmd_instruction.go`, `chat/process.go`) |
| Dual surface completeness | **4** | Large Cobra tree + Bubble Tea chat; some system features only deep in TUI slash commands |
| Test grounding | **3** | ~16.8k test lines across 55 files, but largest chat files (process, session_boot, multistep) remain under-tested relative to size |
| Observability | **4** | Categorized logging, glass box, transparency commands, activity pulse (`cmd_transparency.go`, `chat/glass_box.go`) |
| Safety at edge | **4** | Timeouts, workspace checks, system-shard disable flags; safety still owned by core policy |
| Portability | **5** | Documented drop-in `nerd.exe` + `nerd init` → `.nerd/` workspace (`cmd/nerd/README.md`) |
| JIT / atom discipline | **4** | Campaign JIT provider, `/jit` status, chat comment that domain shards removed in favor of JIT atoms (`campaign_jit_provider.go`, `session_boot.go` comments) |

**Overall alignment: 4.3 / 5** — CLI is a mature, living surface; residual risk is TUI complexity and uneven test density on largest files.

## 3. What “good” looks like (CLI-specific)

| Good | Bad |
|------|-----|
| Kernel-backed actions | Ad-hoc shell from chat without policy |
| Slash command mirrors Cobra verb where sensible | Silent drift between TUI and CLI flags |
| Boot steps logged and recoverable | Opaque hang on sqlite/embedding init |
| Panic recovery in chat goroutines | Process death on model bug (`process.go` recover) |
| Explicit workspace / config paths | Implicit CWD-only without `--workspace` |

## 4. Related corpora

- `Docs/architecture/core/` — kernel, VirtualStore, Dreamer  
- `Docs/architecture/session/` — clean executor  
- `Docs/architecture/perception/` / `articulation/` / `prompt/` — transducers + JIT  
- `Docs/architecture/shards/` — registration and domain/system shards  
- `Docs/architecture/campaign/` — multi-phase goals  
