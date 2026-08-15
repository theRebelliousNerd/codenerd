# 02 — Current State: `internal/types`

> Last verified: **2026-08-15**

## 1. Inventory summary

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | 9 + 1 in `typestest/` | types, interfaces, extract, shard, ctxkeys, atom, transaction, mangle_scale, transparency (+ `typestest/mockkernel.go`) |
| Test `.go` | 12 + 1 in `typestest/` | unit tables, executed examples, one external `types_test` package, two repo-wide ratchets |
| `.mg` | 0 | N/A — but the package is now *checked against* `internal/core/defaults/*.mg` |
| Package docs | package comment only | no `agents.md` / README in package |
| Approx source LOC | ~2,032 | |
| Approx test LOC | ~2,556 | strong pure-function coverage + two repo-wide invariant ratchets |

## 2. File roles

### `types.go` (455 lines) — data model + narrow kernel bridge

Hotspots:

- Context attach: `WithSessionContext` / `GetSessionContext`
- `MangleAtom`, `Fact`, `String()`, `ToAtom()` — **primary safety surface**
- `isValidMangleNameConstant` / `hasFileExtension` heuristics
- `KernelFact` (now `= Fact`), `KernelInterface` (deprecated, with a written 3-step removal path)
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

### `shard.go` (213 lines) — shard taxonomy

Enums + `ShardConfig` + `SpawnPriority` + the deprecated string context keys.

### `ctxkeys.go` (101 lines) — typed context keys

`WithSpawnPriority` / `WithModelCapability` / `WithModelName` and their readers. Setters dual-write the
legacy string keys so unmigrated readers keep working.

### `atom.go` (48 lines) — runtime `/name` construction

`Atom(s)` normalizes an arbitrary identifier into a valid Mangle name constant, so a `/name`-declared
slot fed from an enum or error category is a one-line fix rather than a remembered naming rule.

### `transparency.go` (81 lines) — operator visibility contract

`ShardPhase`, `OperationRecord`, `TransparencyManager`; lets ShardManager report into
`internal/transparency` without importing it.

### `transaction.go` (112 lines) — atomic EDB ops

`KernelTransaction`, `KernelTransactor`, `KernelTx`, `TransactorOf`, `NewKernelTx` (panic policy).

### `typestest/mockkernel.go` (343 lines) — shared test double

`MockKernel` implements `Kernel` **and** `KernelTransactor`, with compile-time assertions for both.

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
| Dual Kernel APIs | Adapter sprawl; reduced — `KernelFact` collapsed into `Fact`, removal path written |
| `SessionContext` growth | Becomes god-struct; token budget discipline external |
| `NewKernelTx` panic | Hard crash if the kernel (or an adapter in front of it) lacks `Transaction()`; now ratcheted and the message names the type |
| Name-constant heuristic | False positive/negative on edge path strings |
| String context keys | Resolved: typed keys in `ctxkeys.go`, legacy strings dual-written until readers migrate |

## 7. What “done” looks like for current state

The package is **production-complete for its role**. Remaining work is evolutionary API hygiene, not missing foundational types.
