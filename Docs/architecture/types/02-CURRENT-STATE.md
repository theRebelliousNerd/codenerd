# 02 — Current State: `internal/types`

> Last verified: **2026-07-13**

## 1. Inventory summary

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | 5 | types, interfaces, extract, shard, transaction |
| Test `.go` | 4 | comprehensive + unit suites |
| `.mg` | 0 | N/A |
| Package docs | package comment only | no `agents.md` / README in package |
| Approx source LOC | ~1,287 | |
| Approx test LOC | ~943 | strong pure-function coverage |

## 2. File roles

### `types.go` (455 lines) — data model + narrow kernel bridge

Hotspots:

- Context attach: `WithSessionContext` / `GetSessionContext`
- `MangleAtom`, `Fact`, `String()`, `ToAtom()` — **primary safety surface**
- `isValidMangleNameConstant` / `hasFileExtension` heuristics
- `KernelFact`, `KernelInterface`
- `StructuredIntent`, tool/shard/knowledge summaries
- `AmbientContext`, **`SessionContext`** (largest struct)

### `interfaces.go` (380 lines) — behavioral contracts

Hotspots:

- `Kernel` full API
- `LLMClient` + tool/message types + `LLMToolResponse`
- `ShardAgent`, factories, JIT registrar funcs
- `ReviewerFeedbackProvider`, `LimitsEnforcer`
- `LearningStore`, `VirtualStore`, `GraphQuery`
- Optional LLM: Grounding*, Piggyback, Thinking, ThoughtSignature, File, Cache, TokenCounter

### `extract.go` (210 lines) — decode helpers

All `Extract*` / `Arg*` / `StripAtomPrefix`. Pure, well-tested.

### `shard.go` (157 lines) — shard taxonomy

Enums + `ShardConfig` + `SpawnPriority` + model context string keys.

### `transaction.go` (85 lines) — atomic EDB ops

`KernelTransaction`, `KernelTransactor`, `KernelTx`, `NewKernelTx` (panic policy).

## 3. Exported surface count (approx)

| Kind | Approx count |
|------|-------------:|
| Interfaces | ~20 |
| Struct types | ~15 |
| Named string/int enums | ~6 |
| Constants (enums + ctx keys) | ~20+ |
| Functions | ~15 Extract/Arg + session helpers + NewKernelTx |
| Methods on Fact/KernelFact/KernelTx/SpawnPriority | ~10 |

Exact catalog: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).

## 4. Dependency state (outbound)

```
internal/types
  ├── context, encoding/json, fmt, strings, time
  ├── codeberg.org/TauCeti/mangle-go/ast
  ├── codeberg.org/TauCeti/mangle-go/analysis   (Kernel.GetProgramInfo)
  └── codenerd/internal/logging                (transaction only)
```

No imports of `core`, `session`, `shards`, `perception`, `store`.

## 5. Inbound consumers (representative)

Major consumers (non-exhaustive; dozens of files):

| Area | Examples |
|------|----------|
| Core | `cortex_kernel.go`, `kernel_utils.go`, `kernel_transactions.go`, `tdd_loop.go`, shards base agent |
| Articulation | `prompt_assembler.go`, `kernel_context.go` |
| Autopoiesis | orchestrator, ouroboros, mocks |
| Campaign | fact sync, context pager, task handlers |
| World | `Fact` alias, graph interface |
| Store / persist | fact_codec, factsnap |
| Perception | LLM client alias, anthropic client |
| Init | agents registration, strategic documents |
| CLI/chat | session boot, process, delegation, spawn, campaign |
| e2e | many `tests/e2e/*` mocks |

## 6. Hotspots & risk concentration

| Hotspot | Risk |
|---------|------|
| `Fact.ToAtom` | Wrong encoding poisons entire EDB |
| Dual Kernel APIs | Adapter sprawl; confusing for new code |
| `SessionContext` growth | Becomes god-struct; token budget discipline external |
| `NewKernelTx` panic | Hard crash if mock kernel incomplete |
| Name-constant heuristic | False positive/negative on edge path strings |
| String context keys | Collision / type confusion vs typed session key |

## 7. What “done” looks like for current state

The package is **production-complete for its role**. Remaining work is evolutionary API hygiene, not missing foundational types.
