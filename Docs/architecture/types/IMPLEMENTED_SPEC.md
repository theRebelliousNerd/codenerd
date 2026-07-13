# types — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/types/`  
> Scale: **5** non-test Go files ≈ **1,287** lines; **4** test files ≈ **943** lines; **0** `.mg`  
> Mode: 1:1 with `internal/types/` (complete internal coverage)

## 1. Overview

`internal/types` is the **shared contract substrate** of codeNERD. Package documentation states its purpose explicitly:

> Package types provides shared type definitions used across codeNERD packages.  
> This package exists to break import cycles between core, articulation, and autopoiesis.  
> Types in this package should be foundational data structures with no complex dependencies.

It is the package every major subsystem can import without dragging in `core`, `session`, `shards`, or LLM provider implementations. That makes it the **single source of truth** for:

| Concern | Canonical home in `types` |
|---------|---------------------------|
| Logical facts | `Fact`, `MangleAtom`, `Fact.String`, `Fact.ToAtom` |
| Kernel surface (full) | `Kernel`, `KernelTransactor`, `KernelTransaction`, `KernelTx` |
| Kernel surface (narrow bridge) | `KernelFact`, `KernelInterface` |
| LLM I/O | `LLMClient`, tool/message types, optional capability interfaces |
| Shards | `ShardAgent`, `ShardConfig`, permissions, states, priorities |
| Session blackboard | `SessionContext` + nested summary structs |
| World graph queries | `GraphQuery` (moved here to break `core` ↔ `world` cycles) |
| Virtual FS / exec | `VirtualStore` marker interface |
| Learning persistence | `LearningStore`, `ShardLearning` |
| Safe arg decoding | `Extract*`, `Arg*` helpers |

### Key characteristics

| Property | Value |
|----------|-------|
| Runtime behavior | Almost none — pure types + conversion + extractors |
| Side effects | `NewKernelTx` may **panic** if kernel lacks `KernelTransactor`; logs via `logging.CategoryKernel` |
| External deps | `mangle-go/ast`, `mangle-go/analysis`, stdlib, `internal/logging` (tx only) |
| Mangle sources | None (consumes AST types; does not ship `.mg`) |
| Import-cycle role | **Critical** — primary cycle-breaker for Cortex graph |
| Dual kernel APIs | Historical: full `Kernel` vs narrow `KernelInterface` |

### High-level position in fact-flow

```
┌─────────────┐   StructuredIntent    ┌──────────────┐
│ perception  │ ────────────────────► │ SessionContext│
└─────────────┘                       └──────┬───────┘
                                             │ inject
┌─────────────┐   Fact / KernelFact          ▼
│  callers    │ ──── ToAtom() ────► ┌─────────────────┐
│ (core, …)   │                     │ mangle-go AST   │
└─────────────┘ ◄── Extract* ────── │ EDB / engine    │
                                    └────────┬────────┘
                                             │ Query → Fact
                    types.Kernel / KernelInterface
                                             │
                    next_action → types.VirtualStore / ShardAgent
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Fact` + `String()` / `ToAtom()` | **Implemented** | Hardened against nil / pointer poisoning |
| Name-constant heuristic | **Implemented** | `isValidMangleNameConstant` + file-ext filter |
| `Kernel` full interface | **Implemented** (contract) | Impl: `core.CortexKernel` / `RealKernel` paths |
| `KernelInterface` bridge | **Implemented** (contract) | Impl: `core.AutopoiesisBridge`, `RealKernel` methods |
| `KernelTx` atomic wrapper | **Implemented** | Panic if no `KernelTransactor` |
| `LLMClient` + tool types | **Implemented** | Used by perception clients, scheduler |
| Optional LLM capabilities | **Implemented** (interfaces) | Grounding, thinking, piggyback, cache, files, tokens |
| `ShardAgent` / `ShardConfig` | **Implemented** | Used by shard manager + factories |
| `SessionContext` blackboard | **Implemented** | Large struct; populated by chat/session |
| Extract helpers | **Implemented** | Well-tested |
| GraphQuery / VirtualStore | **Implemented** (markers) | Methods expanded only as consumers need |
| LearningStore | **Implemented** (contract) | Used by autopoiesis / store |
| Single unified Kernel API | **Partial** | Dual `Kernel` + `KernelInterface` remains |
| Container JSON in `ToAtom` | **Implemented** | maps/slices → JSON string |
| Transaction unit tests in-package | **Partial** | Coverage lives more in `core` |

**Overall:** production foundational package — **~90%** for its intended role (contracts + conversion). Remaining work is API consolidation and a few untested edge contracts, not greenfield implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/types/
  types.go                    # Fact, SessionContext, KernelFact, KernelInterface, intent/tool summaries
  interfaces.go               # Kernel, LLMClient, ShardAgent, VirtualStore, optional LLM capabilities
  extract.go                  # Extract*/Arg* safe fact-arg decoding
  shard.go                    # ShardType/State/Permission/Config, spawn priority, model capability keys
  transaction.go              # KernelTransaction, KernelTransactor, KernelTx
  types_test.go               # Fact.String / ToAtom basics
  types_comprehensive_test.go # ToAtom matrix, session context helpers, edge cases
  extract_test.go             # Extract* / Arg* / StripAtomPrefix
  shard_test.go               # SpawnPriority.String
```

### 3.2 Non-test sources (line counts)

| Path | Lines | Role |
|------|------:|------|
| `internal/types/types.go` | 455 | Facts, session blackboard, intent, summaries, `KernelInterface` |
| `internal/types/interfaces.go` | 380 | Kernel, LLM, shards, VirtualStore, capability interfaces |
| `internal/types/extract.go` | 210 | Type-safe fact argument extraction |
| `internal/types/shard.go` | 157 | Shard enums/config, priority, context keys |
| `internal/types/transaction.go` | 85 | Atomic transaction wrapper |
| **Total source** | **~1,287** | |

### 3.3 Test sources

| Path | Lines | Focus |
|------|------:|-------|
| `types_comprehensive_test.go` | ~529 | ToAtom type matrix, nil/unknown rejection, session helpers |
| `extract_test.go` | ~221 | Extract/Arg helpers |
| `types_test.go` | ~174 | String/ToAtom core + name validation |
| `shard_test.go` | ~19 | Priority string names |
| **Total tests** | **~943** | High ratio vs source (good) |

---

## 4. Deep dive: Fact conversion pipeline

### 4.1 Why this is the most safety-critical code in the package

Every assert path that builds Mangle EDB from Go ultimately goes through `Fact.ToAtom()` (or an equivalent that should match its conventions). Historical bugs:

- Silent `fmt.Sprintf("%v", v)` turned pointers into hex addresses like `0x7ff…`
- Those strings entered the kernel as `StringType`
- Numeric Mangle builtins (`fn:plus`, …) then failed with “value 0x… is not a number”
- Diff / eval engines fell back to full re-eval and failed the same way

The current `ToAtom` **rejects** nil and non-JSON-encodable unknown types with errors that name **predicate + arg index**.

### 4.2 `MangleAtom` vs plain `string`

| Go type | Intent | `ToAtom` behavior |
|---------|--------|-------------------|
| `MangleAtom("/x")` | Explicit name constant | `ast.Name`; if missing `/`, fallback to string |
| `MangleAtom("/bad//x")` | Invalid name | Error |
| `string "/coder"` | Possible name | Name iff `isValidMangleNameConstant` |
| `string "/mnt/c/file.go"` | File path | String (depth + extension heuristics) |
| `string "plain"` | Text | Quoted string constant |

Heuristic gates in `isValidMangleNameConstant` (`types.go`):

1. Must start with `/`
2. No whitespace
3. Not just `/`
4. No `//`
5. Slash count ≤ 2 (deep paths treated as files)
6. No common file extensions (`.go`, `.md`, …)
7. Must parse via `ast.Name(v)`

### 4.3 Supported arg kinds in `ToAtom`

| Kind | Encoding |
|------|----------|
| `MangleAtom` | Name or string fallback |
| `string` | Name or string |
| `int`, `int64` | `ast.Number` |
| `float32`, `float64` | `ast.Float64` |
| `time.Time` | `ast.Time` (Unix nanos) |
| `time.Duration` | `ast.Duration` (nanos) |
| `bool` | `ast.TrueConstant` / `FalseConstant` |
| `map[string]any`, `[]any`, `[]string`, `[]int`, `[]int64`, `[]float64` | JSON → string |
| `nil` | **Error** |
| Other | Best-effort `json.Marshal`; reject null/`{}`/fail |

### 4.4 `Fact.String()`

Produces Datalog-ish text for logging and debugging (not the same as store insertion). Booleans render as `/true` `/false`. Invalid name-looking strings are quoted.

---

## 5. Deep dive: Dual kernel interfaces

codeNERD carries **two** kernel-facing contracts for historical and cycle-breaking reasons.

### 5.1 `types.Kernel` (`interfaces.go`)

Full operational surface used by shard agents, world scanner, power features:

```
LoadFacts, Query, QueryAll, Assert, AssertBatch,
Retract, RetractFact, UpdateSystemFacts,
GetProgramInfo, Reset, AppendPolicy,
RetractExactFactsBatch, RemoveFactsByPredicateSet
```

Aliases: `core.Kernel = types.Kernel` (`internal/core/kernel_types.go`).

### 5.2 `types.KernelInterface` + `KernelFact` (`types.go`)

Narrower bridge for packages that must not import full kernel wiring:

```
AssertFact, AssertFactBatch, QueryPredicate, QueryBool, RetractFact
```

Adapters:

- `core.AutopoiesisBridge` implements `types.KernelInterface` (`kernel_utils.go`)
- `RealKernel` also exposes matching methods for the same shape

### 5.3 Transactions

```
KernelTransactor.Transaction() → KernelTransaction
  Retract / RetractFact / RetractExactFact / RetractPredicateSet / Assert
  Commit()  // single rebuild/evaluate cycle
```

`NewKernelTx(k Kernel)` type-asserts `KernelTransactor`. If missing:

1. Logs warn on `logging.CategoryKernel`
2. **Panics** — non-atomic fallback was deliberately removed

Implementations: `CortexKernel` / `RealKernel` transaction types in `internal/core`.

---

## 6. Deep dive: LLM client contract family

### 6.1 Required: `LLMClient`

| Method | Purpose |
|--------|---------|
| `Complete` | Simple completion |
| `CompleteWithSystem` | System + user |
| `CompleteWithStreaming` | Token stream channels |
| `CompleteWithTools` | Agentic tool calling |

Aliases: `core.LLMClient`, `perception.LLMClient` → `types.LLMClient`.

### 6.2 Multi-turn tool protocol

- `Message`, `ToolCall`, `ToolResult`, `ToolDefinition`, `LLMToolResponse`, `UsageMetadata`
- Optional `ToolResultsProvider.CompleteWithToolResults` for native multi-turn function calling
- Providers without it fall back to single-turn `CompleteWithTools`

### 6.3 Optional capability interfaces (type assertion)

| Interface | Use |
|-----------|-----|
| `GroundingProvider` / `GroundingController` | Google Search / URL context |
| `PiggybackToolProvider` | Gemini: tools via Piggyback when grounding conflicts with native FC |
| `ThinkingProvider` | Thought summary / token counts / level (SPL learning) |
| `ThoughtSignatureProvider` | Gemini 3 multi-turn reasoning continuity |
| `FileProvider` | Upload/list/delete files |
| `CacheProvider` | Context caching |
| `TokenCounter` | Pre-generation token counts |

Pattern everywhere:

```go
if gp, ok := client.(types.GroundingProvider); ok { ... }
```

This keeps `LLMClient` small while allowing provider-specific power without import cycles.

---

## 7. Deep dive: SessionContext blackboard

`SessionContext` is the **compressed session payload** injected into shards (`SetSessionContext`) and prompt assembly. Sections:

| Section | Fields (summary) |
|---------|------------------|
| Core | `CompressedHistory`, findings, actions, active files, ambient IDE |
| Dream | `DreamMode` — describe-only, no exec |
| World model | impacted files, diagnostics, symbols, deps |
| Intent | `UserIntent *StructuredIntent`, focus resolutions |
| Campaign | active/phase/goal/deps/requirements |
| Git | branch, modified, commits, unstaged |
| TDD | test state, failing tests, retry count |
| Cross-shard | `PriorShardOutputs []ShardSummary` |
| Domain | knowledge atoms, specialist hints |
| Reflection | reflection hits |
| Gathered knowledge | specialist consult summaries |
| Tools | `AvailableTools`, `RecentToolExecutions` |
| Constitutional | `AllowedActions`, `BlockedActions`, `SafetyWarnings` |

Context helpers:

- `WithSessionContext(ctx, *SessionContext)` / `GetSessionContext(ctx)` — private typed context key

---

## 8. Deep dive: Shard contract surface

### 8.1 Lifecycle types

| Enum | Values |
|------|--------|
| `ShardType` | ephemeral, persistent, user, system |
| `ShardState` | idle, running, completed, failed |
| `StartupMode` | auto, on_demand |
| `SpawnPriority` | Low=0 … Critical=3 |

### 8.2 Permissions

`read_file`, `write_file`, `exec_cmd`, `network`, `browser`, `code_graph`, `ask_user`, `research`

These are **capability labels** on `ShardConfig`; enforcement is policy/kernel elsewhere (`permitted(...)`).

### 8.3 `ShardAgent`

```
Execute, GetID, GetState, GetConfig, Stop,
SetParentKernel, SetLLMClient, SetSessionContext
```

Factories: `ShardFactory func(id string, config ShardConfig) ShardAgent`.

JIT hooks: `PromptLoaderFunc`, `JITDBRegistrar`, `JITDBUnregistrar`.

### 8.4 Model hints via context keys

String keys (not typed context keys — historical):

- `CtxKeyPriority` = `"spawn_priority"`
- `CtxKeyModelCapability` = `"model_capability"`
- `CtxKeyModelName` = `"model_name"`

---

## 9. Extract helpers (consumer safety)

Replace panic-prone assertions and `%v` dumps when reading `Fact.Args`:

| Func | Returns |
|------|---------|
| `ExtractString` / `ArgString` | string (bool → `/true`/`/false`) |
| `ExtractName` / `ArgName` | name-ish string |
| `ExtractInt64` / `ArgInt64` | (int64, ok) |
| `ExtractFloat64` / `ArgFloat64` | (float64, ok) |
| `ExtractBool` | handles `/true` `/false` atoms |
| `ExtractTime` | Time or Unix nanos int64 |
| `ExtractDuration` | Duration or nanos int64 |
| `StripAtomPrefix` | `/read_file` → `read_file` |

Heavy use in articulation (`prompt_assembler.go`, `kernel_context.go`), browser honeypot, campaign fact sync, etc.

---

## 10. Integration map

| Surface | How `types` participates |
|---------|--------------------------|
| Kernel | Implements `types.Kernel` / Transactor; uses `Fact.ToAtom` |
| VirtualStore | Interface defined here; impl `*core.VirtualStore` |
| Shards | `ShardAgent` + config/permissions; base agent DI setters |
| Perception | `LLMClient` alias; Anthropic/OpenAI clients satisfy interfaces |
| Articulation | `Fact`, `SessionContext`, `ExtractString` for prompt assembly |
| Autopoiesis | `KernelInterface`, `LearningStore`, tools metadata |
| Campaign | `NewKernelTx`, facts, session context paging |
| World | `type Fact = types.Fact`; `GraphQuery` defined here |
| Store / persist | Fact codecs / factsnap round-trip `types.Fact` |
| CLI / chat | Session adapters, spawn, campaign, UI shard pages |
| e2e tests | Widespread mocks of `types.Kernel` / facts |

### Alias re-exports (compatibility)

| Location | Alias |
|----------|-------|
| `internal/core/kernel_types.go` | `Fact`, `Kernel` |
| `internal/core/llm_client.go` | `LLMClient` |
| `internal/perception/client_types.go` | `LLMClient` |
| `internal/world/types.go` | `Fact` |

---

## 11. Non-goals of this package

- Implementing the Mangle engine, policy corpus, or VirtualStore behavior
- Owning provider HTTP clients (perception / core own those)
- Growing into a dump of every domain DTO in the monorepo
- Embedding Vectryx product vocabulary or app-specific structs
- Silent coercion of bad fact args (explicitly rejected)

---

## 12. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for dual-interface debt, untested transaction constructor panic path, stringly-typed context keys, and VirtualStore method surface expansion discipline.

---

## 13. Verify commands

```powershell
go test ./internal/types/...
go test ./internal/core/ -run 'Transaction|CortexKernel|ToAtom' -count=1
```
