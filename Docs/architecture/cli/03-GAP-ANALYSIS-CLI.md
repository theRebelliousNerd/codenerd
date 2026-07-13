# 03 — Gap Analysis: CLI

> Last verified: 2026-07-13  
> Method: inventory of `cmd/nerd` vs vision principles and north-star fact-flow.

## 1. Spec vs reality matrix

| Capability | Vision | Reality | Status |
|------------|--------|---------|--------|
| Interactive default | Bare `nerd` → TUI | `rootCmd.RunE` → `chat.RunInteractiveChat` | **Yes** |
| Portable workspace | `init` + `.nerd/` | `cmd_init_scan.go` | **Yes** |
| Single-shot OODA | `run` instruction | `cmd_instruction.go` | **Yes** |
| Kernel query surface | query/why/status | `cmd_query.go` + slash mirrors | **Yes** |
| Campaign orchestration | first-class | `cmd_campaign.go` + `/campaign` | **Yes** |
| Multi-engine auth | Claude/Codex/Grok | `cmd_auth.go` | **Yes** |
| Dream / shadow / whatif | safety exploration | `cmd_advanced.go` + chat handlers | **Yes** |
| DOM surgical editing | CodeDOM CLI | `dom_*.go` | **Yes** |
| JIT visibility | inspect compiled prompts | `jit` command + `ui/jit_page.go` + `/jit` | **Partial** — deep atom selection still internal |
| Parity Cobra ↔ slash | same verbs both doors | Many mirrors; not complete 1:1 | **Partial** |
| Test density on hot paths | process/session_boot well covered | Large files, tests exist but lag complexity | **Partial** |
| Modular chat package | clear subpackages | Many large co-located files in `chat/` | **Partial** |
| Domain shard imports | JIT-only | Comments say removed; verify no regressions | **Watch** |

## 2. Built but under-documented (before this corpus)

- Multistep corpus / decomposer paths (`multistep_*.go`)
- Review aggregator pipeline
- Delegation modes
- Activity pulse / glass box timeline
- Config wizard multi-step
- Assault campaign UX

## 3. Spec’d-by-vision but thin in CLI

| Gap | Impact | Gate to close |
|-----|--------|---------------|
| Incomplete slash↔Cobra parity matrix | User confusion | Publish matrix; add missing mirrors or document intentional-only-TUI |
| Limited tests on `session_boot` failure modes | Boot regressions | Table-driven boot fakes |
| Embedding engine UX fragmentation | `embedding` cmd + `/embedding` + config wizard | Single status surface |
| Command modularization of chat | Hard reviews | Split process/session by concern (no behavior change) |

## 4. Priority assessment

| Priority | Item | Blocks |
|----------|------|--------|
| P0 | Boot reliability + panic recovery keep | All interactive use |
| P0 | Policy/safety never bypassed from CLI | Trust model |
| P1 | Hot-path tests for process + session_boot | Safe refactors |
| P1 | Documented command parity matrix | UX consistency |
| P2 | Further chat file modularization | Maintainability |
| P3 | Additional UI polish pages | Delight |

## 5. Explicit non-gaps

- CLI **does** implement a large production surface (38k+ LOC).
- Treating this package as “pre-implementation 0%” would be false.
- Kernel incompleteness is **not** a CLI gap (tracked under `core`/`mangle` corpora).
