# Mangle Fact Assert / Retract Go APIs — Inventory

> **Date:** 2026-05-13 | **Scope:** `internal/core`, `internal/types`, `internal/mangle`  
> **Method:** grep + direct file reads (see Sources).  
> **Confidence:** High for listed APIs (verified via file reads); Low/negative for `pending_edit` (no evidence found).

---

## 1. Executive Summary

- **Primary assertion/retraction surface is `RealKernel` (`internal/core/kernel_facts.go`) + `Kernel` interface (`internal/types/interfaces.go`).**
- **`pending_edit` does NOT exist** in Go code (grep for `pending_edit` returned 0 hits; `pending_` also 0 hits via grep engine). The closest existing predicate is **`pending_mutation`** asserted by `TransactionManager.AddEdit` (`internal/core/transaction_manager.go:186`).
- **Transactional batching** exists in two layers: `KernelTransaction` (`internal/core/kernel_transactions.go`) for kernel EDB atomicity, and `TransactionManager` (file 2PC) for filesystem edits. Both are relevant if introducing a new `pending_edit` lifecycle.
- **Control-packet path** for LLM-produced facts is `FilterMangleUpdates` (`internal/core/mangle_updates.go`) → `Assert`/`AssertBatch`.

---

## 2. Core Types

### 2.1 Fact (`internal/types/types.go:47-51` + `internal/core/kernel_types.go:24`)

```go
type Fact = types.Fact // alias in core
type Fact struct {
    Predicate string
    Args      []any
}
func (f Fact) String() string          // Datalog: `pred(arg, ...).`
func (f Fact) ToAtom() (ast.Atom, error)
type MangleAtom string                 // name constant `/foo`
```

- Also `KernelFact` wrapper in `types.go:249-262` for `KernelInterface`.

### 2.2 Kernel Interface (`internal/types/interfaces.go:10-33`)

```go
type Kernel interface {
    LoadFacts(facts []Fact) error
    Query(predicate string) ([]Fact, error)
    QueryAll() (map[string][]Fact, error)
    Assert(fact Fact) error
    AssertBatch(facts []Fact) error
    Retract(predicate string) error          // removes ALL facts of predicate
    RetractFact(fact Fact) error             // predicate + first arg match
    UpdateSystemFacts() error
    GetProgramInfo() *analysis.ProgramInfo
    Reset()
    AppendPolicy(policy string)
    RetractExactFactsBatch(facts []Fact) error
    RemoveFactsByPredicateSet(predicates map[string]struct{}) error
}
```

*Source: `internal/types/interfaces.go:10-33` (read_file verified).*

Secondary interface (`internal/types/types.go:267-278`) for lighter coupling:

```go
type KernelInterface interface {
    AssertFact(fact KernelFact) error
    AssertFactBatch(facts []KernelFact) error
    QueryPredicate(predicate string) ([]KernelFact, error)
    QueryBool(predicate string) bool
    RetractFact(fact KernelFact) error
}
```

---

## 3. Assert APIs (`internal/core/kernel_facts.go`)

All methods on `*RealKernel`. File read covered lines 1-~560 and 560-~930 (truncated at EOF).

| Method | Signature | Semantics | Source Anchor |
|---|---|---|---|
| `LoadFactsSeq` | `func (k *RealKernel) LoadFactsSeq(seq iter.Seq[Fact]) error` | Deduped insert via `addFactIfNewLocked`, rebuild if any new, full `evaluate()` | `kernel_facts.go:22` |
| `LoadFacts` | `func (k *RealKernel) LoadFacts(facts []Fact) error` | Batch boot path; sanitizes numeric predicates, dedupes, invalidates `cachedAtoms`, calls `evaluate()` eagerly | `kernel_facts.go:61` |
| `Assert` | `func (k *RealKernel) Assert(fact Fact) error` | Single-fact; special-cases `system_heartbeat` via `assertHeartbeat`; otherwise `sanitizeFactForNumericPredicates` → `addFactIfNewLocked` → `factsDirty.Store(true)` → `eventBus.Publish`; dedup is no-op | `kernel_facts.go:471` |
| `assertHeartbeat` | `func (k *RealKernel) assertHeartbeat(fact Fact) error` | Upsert `system_heartbeat(Shard, Timestamp)` in-place without dirtying (avoids re-eval storm); only first heartbeat per shard dirties | `kernel_facts.go:507` |
| `AssertBatch` | `func (k *RealKernel) AssertBatch(facts []Fact) error` | Multiple facts, single dirty flag, O(N) eval vs O(M*N); publishes one event per predicate | `kernel_facts.go:569` |
| `AssertString` | `func (k *RealKernel) AssertString(factStr string) error` | `ParseFactString` then `Assert` | `kernel_facts.go:616` |
| `AssertWithoutEval` | `func (k *RealKernel) AssertWithoutEval(fact Fact)` | Adds without `Evaluate()`; deferred batch use | `kernel_facts.go:626` |
| `assertWithoutEvalChecked` | `func (k *RealKernel) assertWithoutEvalChecked(fact Fact) error` (private) | Like above but returns error for `maxFacts` or `ToAtom` failure; used for safety simulation fail-closed | `kernel_facts.go:640` |
| `Evaluate` | `func (k *RealKernel) Evaluate() error` | Force re-eval after `AssertWithoutEval` batch | `kernel_facts.go:667` |

**Helpers:**

- `addFactIfNewLocked(f Fact) bool` (`kernel_facts.go:394`) — dedup via `canonFact`/`factIndex`, interning, `factToAtomLocked` validation, `maxFacts` guard (`defaultMaxFacts` or `k.maxFacts`), appends to `cachedAtoms` + `factsSinceLastEval` + `markStratumDirtyLocked` when diff eval enabled.
- `canonFact`/`canonValue`/`canonString` (`kernel_facts.go:130-217`) — stable string key for dedup.
- `sanitizeFactForNumericPredicates` — called on every assert path (grep shows usage in all assert entry points).
- `rebuildFactIndexLocked`, `ensureFactIndexLocked` (`kernel_facts.go:373-388`).
- `markStratumDirtyLocked` (`kernel_facts.go:442`) — dirty strata tracking for differential eval.

**Parsing:** `ParseFactString` (referenced in `AssertString` and `mangle_updates.go:77` as `ParseFactString(factText)`). Actual definition not read (likely `internal/core/parse_serial.go` or `kernel_fact_decl.go`), but `mangle_updates.go` shows usage pattern: `strings.TrimSuffix(trimmed,".")` then `ParseFactString`.

---

## 4. Retract APIs (`internal/core/kernel_facts.go:685-933`)

| Method | Signature | Matching Semantics | Rebuild |
|---|---|---|---|
| `Retract` | `func (k *RealKernel) Retract(predicate string) error` | Removes **all** facts where `f.Predicate == predicate`; parallel filter of `facts` + `cachedAtoms`; rebuilds `factIndex`; calls `rebuild()` | `kernel_facts.go:687` |
| `RetractFact` | `func (k *RealKernel) RetractFact(fact Fact) error` | Keeps facts where predicate differs OR first arg differs (`!argsEqual(f.Args[0], fact.Args[0])`); requires `len(fact.Args)>0` | `kernel_facts.go:750` |
| `RetractExactFact` | `func (k *RealKernel) RetractExactFact(fact Fact) error` | Exact match on predicate **and all args** (`argsSliceEqual`) | `kernel_facts.go:816` |
| `RetractExactFactsBatch` | `func (k *RealKernel) RetractExactFactsBatch(facts []Fact) error` | Builds `map[canonFact]struct{}` then filters; single rebuild | `kernel_facts.go:858` |
| `RemoveFactsByPredicateSet` | `func (k *RealKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error` | Removes where predicate ∈ set; single rebuild | `kernel_facts.go:900` |

**Argument equality:** `argsEqual(a,b any) bool` (`kernel_facts.go:943`) — type-switch for `string`/`MangleAtom`/`int`/`int64`/`uint`/`uint64`/`int32`/`uint32`/`float64`/`bool`/`map[string]any`/`[]any`; falls back to `reflect.DeepEqual`. Note symmetry handling for `MangleAtom` ↔ `string`. `argsSliceEqual` (`kernel_facts.go:1039`) checks length then elementwise `argsEqual`.

---

## 5. Transactional / Atomic APIs

### 5.1 Kernel EDB Transaction (`internal/core/kernel_transactions.go` + `internal/types/transaction.go`)

**Type:** `KernelTransaction` (`kernel_transactions.go:25-36`)

```go
type KernelTransaction struct {
    kernel              *RealKernel
    retractPredicates   []string
    retractFacts        []Fact
    retractExactFacts   []Fact
    retractPredicateSet map[string]struct{}
    assertFacts         []Fact
    committed bool
}
```

**Constructor:** `func (k *RealKernel) Transaction() types.KernelTransaction` (`kernel_transactions.go:41`)

**Methods** (`kernel_transactions.go:49-73` + `transaction.go:9-13`):

```go
func (tx *KernelTransaction) Retract(predicate string)
func (tx *KernelTransaction) RetractFact(fact Fact)
func (tx *KernelTransaction) RetractExactFact(fact Fact)
func (tx *KernelTransaction) RetractPredicateSet(predicates map[string]struct{})
func (tx *KernelTransaction) Assert(fact Fact)
func (tx *KernelTransaction) Commit() error // single lock, single rebuild()
```

**Wrapper:** `types.KernelTx` (`internal/types/transaction.go:33-85`) — convenience over `KernelTransactor`:

```go
type KernelTransaction interface { Retract(string); RetractFact(Fact); RetractExactFact(Fact); RetractPredicateSet(map[string]struct{}); Assert(Fact); Commit() error }
type KernelTransactor interface { Transaction() KernelTransaction }
func NewKernelTx(k Kernel) *KernelTx // panics if kernel lacks KernelTransactor (fallback removed)
func (t *KernelTx) LoadFacts(facts []Fact) // buffers each as Assert
```

**Commit phases** (`kernel_transactions.go:77-169`): Phase 1 retract by predicate, Phase 2 predicate set, Phase 3 first-arg retract, Phase 4 exact, rebuild index + invalidate `cachedAtoms`, Phase 5 asserts via `sanitizeFactForNumericPredicates` + `addFactIfNewLocked`, Phase 6 single `k.rebuild()`. Respects `k.simulateCommitErr` test hook.

### 5.2 Filesystem 2PC TransactionManager (`internal/core/transaction_manager.go`)

- **Distinct from kernel EDB transaction** — orchestrates multi-file edits with shadow validation.
- `AddEdit` asserts `pending_mutation(mutationID, filePath, oldContent[0:200], newContent[0:200])` via `tm.kernel.Assert` (`transaction_manager.go:186-191`). Content truncated to 200 chars + `"..."`.
- `Commit` emits `file_written(path, newHash, txnID, now)` (`transaction_manager.go:397-400`).
- `Prepare` runs `ShadowMode.StartSimulation` + `SimulateAction` + queries `deny_edit` (`transaction_manager.go:298-314`).

---

## 6. Control-Packet / Mangle Updates Path (`internal/core/mangle_updates.go`)

Used for LLM `mangle_updates` array in Piggyback envelope.

```go
type MangleUpdatePolicy struct {
    AllowedPredicates map[string]struct{}
    AllowedPrefixes   []string
    MaxUpdates        int
}
type MangleUpdateBlock struct { Update string; Reason string }

func FilterMangleUpdates(kernel Kernel, updates []string, policy MangleUpdatePolicy) ([]Fact, []MangleUpdateBlock)
```

Validation steps (`mangle_updates.go:24-108`):
1. Trims, skips empties, enforces `MaxUpdates`.
2. Rejects `:-` (rules), `decl `, `import `/`include `.
3. `ParseFactString(strings.TrimSuffix(trimmed,"."))` — parse error → block.
4. `predicateAllowed` — checks `AllowedPredicates`/`AllowedPrefixes`.
5. `validatePredicateDeclaration` — checks `programInfo.Decls` arity/existence or `schemaValidator.IsDeclared` if no programInfo.

Returns `[]Fact` (allowed) + `[]MangleUpdateBlock` (rejected). Caller then typically does `Assert`/`AssertBatch` or `Transaction`.

---

## 7. `pending_edit` / `pending_*` Current Usage

### 7.1 Grep Evidence (verbatim)

- `pending_edit` → **0 matches** (grep max_results 100).
- `pending_mutation|pending_edit|pending_action` → **0 matches via grep engine** (note: manual read found `pending_mutation` in transaction_manager.go, suggesting grep index may be stale or scoped).
- `pending_` → 0 matches via grep engine.

### 7.2 Direct File Evidence

- **Only `pending_mutation` exists** in `internal/core/transaction_manager.go:186-191` (verified via `read_file`):
  ```go
  tm.kernel.Assert(Fact{
      Predicate: "pending_mutation",
      Args:      []any{mutationID, edit.FilePath, oldContent, newContent},
  })
  ```
  Where `mutationID = fmt.Sprintf("%s_edit_%d", txn.ID, len(txn.Edits)-1)`.

- **No `pending_edit` predicate** found in any `read_file` output. No schema declaration for it was read; to confirm schema, check `mangle/*.mg`, `internal/core/defaults/*.mg`, `internal/mangle/*.mg` (not yet read — see Uncertainty).

- **`file_written`** is the completion counterpart (`transaction_manager.go:397`).

### 7.3 Interpretation

- If task requires `pending_edit`, it is **new work** — no prior Go API or predicate to extend. Model new API on `pending_mutation` pattern (truncate args, assert via kernel, retract via transaction on commit/abort).
- Consider whether `pending_edit` should be EDB fact (like `pending_mutation`) or IDB derived; current `pending_mutation` is EDB asserted directly.

---

## 8. Query / Introspection APIs (for completeness)

- `Query(predicate string) ([]Fact, error)` and `QueryAll() (map[string][]Fact, error)` on `Kernel` (`interfaces.go:12-13`). Implementation in `kernel_query.go` (not read this run; file exists).
- `GetProgramInfo() *analysis.ProgramInfo` for decl inspection; used in `mangle_updates.go:126-134`.
- `FactEventBus` (`kernel_facts.go:496-499`, `fact_event_bus.go`) — `Publish(predicate)` after assert.

---

## 9. File Map (where to edit)

| Concern | File | Key Symbols |
|---|---|---|
| Assert/Retract impl | `internal/core/kernel_facts.go` | `Assert`, `AssertBatch`, `Retract*`, `addFactIfNewLocked`, `canonFact` |
| Kernel interfaces | `internal/types/interfaces.go` | `Kernel` |
| Light interface | `internal/types/types.go` | `KernelInterface`, `Fact` |
| Transaction (EDB) | `internal/core/kernel_transactions.go` | `KernelTransaction`, `Transaction()` |
| Transaction wrapper | `internal/types/transaction.go` | `KernelTx`, `KernelTransactor`, `NewKernelTx` |
| Filesystem 2PC | `internal/core/transaction_manager.go` | `TransactionManager`, `pending_mutation`, `file_written` |
| Mangle updates | `internal/core/mangle_updates.go` | `FilterMangleUpdates`, `MangleUpdatePolicy` |
| Eval / rebuild | `internal/core/kernel_eval.go` | `rebuildProgram`, `evaluate`, `hasExternalPredicatesLocked` |
| Parse | `internal/core/parse_serial.go` | `parseUnit` (delegates to `internal/mangle.ParseUnit`) |
| Types / Fact | `internal/core/kernel_types.go` | `RealKernel` struct, `Fact` alias |

ECW / policy schemas (not read): `internal/core/defaults/*.mg`, `mangle/*.mg`, `internal/mangle/*.mg` — check for `Decl pending_*` declarations.

---

## 10. Recommended Use Patterns (derived)

**Single fact:**
```go
err := kernel.Assert(types.Fact{Predicate: "my_pred", Args: []any{"/atom", "value", int64(42)}})
```

**Batch (preferred for N>1):**
```go
err := kernel.AssertBatch(facts) // or AssertWithoutEval + Evaluate
```

**Atomic retract+assert:**
```go
tx := types.NewKernelTx(kernel) // or kernel.Transaction()
tx.Retract("pending_edit") // or RetractFact for selective
tx.Assert(types.Fact{Predicate: "pending_edit", Args: []any{id, path, old, new}})
if err := tx.Commit(); err != nil { ... }
```

**From LLM mangle_updates:**
```go
facts, blocked := core.FilterMangleUpdates(kernel, envelope.Control.MangleUpdates, policy)
if len(facts)>0 { _ = kernel.AssertBatch(facts) }
```

**Pending-edit lifecycle (proposed, modeled on pending_mutation):**
- Assert on edit enqueue: `pending_edit(id, path, oldHash, newHash, status)`
- Retract on commit/abort via `RetractFact` (first-arg id) or `Retract("pending_edit")` in same `KernelTransaction` as `file_written` assert.

---

## 11. Uncertainty & Gaps

- **Grep vs Read divergence:** `grep pending_mutation` returned 0 hits but `read_file transaction_manager.go` shows `pending_mutation` at line 186. **Uncertainty:** grep index may be stale/partial; other `pending_*` occurrences could exist outside `internal/core` (e.g., in `mangle/*.mg` policy files) not yet searched. **Next step:** run `grep -r pending_` on filesystem incl. `*.mg` and `internal/mangle/`.
- **`ParseFactString` definition** not read — inferred signature; confirm in `internal/core/kernel_fact_decl.go` or `parse_serial.go`.
- **`defaultMaxFacts` value** not captured (referenced in `addFactIfNewLocked` but truncated). Read `kernel_types.go` fully or `limits.go`.
- **Schema declarations** for `pending_mutation` / `file_written` / potential `pending_edit` not verified — check `internal/core/defaults/schema/*.mg` and `internal/mangle/intent_routing.mg`.
- **Differential eval impact:** `markStratumDirtyLocked` and `factsSinceLastEval` imply `Assert` dirties strata; `pending_edit` facts will participate in incremental eval if `CODENERD_DIFF_EVAL=1` and policy stable.
- **No `pending_edit` API** to extend — if tasked to add it, new predicate must be declared in schemas and allowed in `MangleUpdatePolicy.AllowedPredicates` / `AllowedPrefixes`.

---

## 12. Sources (traceable)

- `internal/types/types.go:47-51, 249-278, 145-243` — `Fact`, `KernelInterface`, `ToAtom`
- `internal/types/interfaces.go:10-33` — `Kernel` interface
- `internal/types/transaction.go:1-85` — `KernelTx`, `KernelTransactor`, `KernelTransaction` interface
- `internal/core/kernel_types.go:24,42-111` — `RealKernel` alias and struct
- `internal/core/kernel_facts.go:22-124, 130-440, 471-683, 685-933, 943-1050` — all Assert/Retract impl
- `internal/core/kernel_transactions.go:1-239` — `KernelTransaction` impl
- `internal/core/transaction_manager.go:186-191, 397-400, 298-314` — `pending_mutation` + `file_written` + `deny_edit` query
- `internal/core/mangle_updates.go:8-154` — `FilterMangleUpdates`
- `internal/core/kernel_eval.go:56-200` — `rebuildProgram`, `evaluate`
- `internal/core/parse_serial.go:1-24` — `parseUnit` delegation
- Grep results: `pending_edit` 0 hits, `AssertFact|RetractFact` 0 hits (via grep engine), `mangle` hits enumerated, glob `**/*.go` truncated.

---

*Generated under exploration-budget exhausted constraint — no further reads/searches performed. Verify schema/*.mg and `ParseFactString` source for completeness before implementing new `pending_edit` flow.*
