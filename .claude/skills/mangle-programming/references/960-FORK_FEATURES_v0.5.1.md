# 960: TauCeti Mangle Fork — v0.5.1 Feature Reference

**Version**: `codeberg.org/TauCeti/mangle-go@v0.5.1-0.20260413190942-4dcaa582c6d3`
**Upgrade date**: 2026-04-13 commit `4dcaa58` and later
**Audience**: anyone editing Mangle in codeNERD or any other repo pinned to the fork.

This file documents the features that exist in the codeberg.org/TauCeti fork but **NOT** in upstream `github.com/google/mangle@v0.4.0`. Everything in the older `000–950` references still applies; this is the additive layer.

If a snippet in another reference uses `github.com/google/mangle` in an import, treat that as historical attribution — for new Go code, use `codeberg.org/TauCeti/mangle-go`.

---

## 1. Provenance — `mgwhy` and `MemoryRecorder`

The fork ships a first-class **provenance** subsystem: derivations can be recorded during evaluation and replayed afterward as proof trees. This is the foundation for "why was this fact derived?" debugging and for codeNERD's planned `mgwhy`-driven diagnostics.

### 1.1 The `provenance` package

Import path: `codeberg.org/TauCeti/mangle-go/provenance`

| Type / function | Purpose |
|------|---------|
| `provenance.MemoryRecorder` | In-memory `engine.DerivationRecorder` implementation. Buffers every rule firing, let-emit, and do-emit. |
| `provenance.NewMemoryRecorder()` | Construct a fresh recorder. |
| `provenance.Event` | One recorded derivation. Fields: `Kind`, `Rule`, `Head`, `Subst`, `Row`, `PremiseFacts`, `GroupKey`, `InputFacts`, `Output`, `TransformText`. |
| `provenance.EventKind` | `EventRule`, `EventLet`, `EventDo`. |
| `provenance.BuildFromRecording(rec, store, goal, opts)` | Build proof trees from a recorded evaluation. Required for goals derived through let/do transforms. |
| `provenance.Explain(programInfo, store, goal, opts)` | Post-hoc explanation **without** a recording. Works for plain Datalog rules; cannot reconstruct aggregations. |
| `provenance.Options{MaxProofs, MaxDepth}` | Bound the search. Defaults: 1 proof, depth 64. |
| `provenance.Print(io.Writer, []*ProofNode)` | Render proofs as indented trees. |
| `provenance.EmitFacts([]*ProofNode, FactStore)` | Emit proofs as Mangle facts matching the `proves / uses_rule / premise / edb_leaf / binding / rule_source` schema. |

### 1.2 New `ProofNode` kinds

```go
const (
    KindEDB         // Leaf: fact came from the input store.
    KindDerived     // Plain Datalog rule firing.
    KindAbsence     // Closed-world negation succeeded (fact absent from store).
    KindLetRow      // NEW: one row produced by a let-transform.
    KindDoAggregate // NEW: one group produced by a do-transform.
)
```

`KindLetRow` and `KindDoAggregate` only appear when `mode = full` (recording-backed). `provenance.Explain` cannot produce them.

### 1.3 Wiring a recorder into evaluation

```go
import (
    "codeberg.org/TauCeti/mangle-go/engine"
    "codeberg.org/TauCeti/mangle-go/provenance"
)

recorder := provenance.NewMemoryRecorder()
_, err := engine.EvalStratifiedProgramWithStats(
    pi, strata, predToStratum, store,
    engine.WithDerivationRecorder(recorder),
)

proofs, err := provenance.BuildFromRecording(
    recorder, store, goalAtom,
    provenance.Options{MaxProofs: 3, MaxDepth: 32},
)
provenance.Print(os.Stdout, proofs)
```

### 1.4 The `mgwhy` CLI

Source: `cmd/mgwhy/main.go`. Verified flag set:

```
mgwhy -program PROG.mg [-facts STORE.sc[.gz|.zst]]
      [-mode simple|full] [-format tree|facts]
      [-max-proofs N] [-max-depth N] GOAL
```

| Flag | Default | Notes |
|------|---------|-------|
| `-program` | (required) | Mangle source file. |
| `-facts` | none | Optional simplecolumn factstore; extension auto-detects `.gz` / `.zst`. |
| `-mode` | `simple` | `simple` = post-hoc `Explain`; `full` = recording-backed (supports let/do). |
| `-format` | `tree` | `tree` = indented proof; `facts` = `proves(...)` / `uses_rule(...)` / `premise(...)` facts ready to feed back into Mangle. |
| `-max-proofs` | `1` | Number of alternative derivations to print. |
| `-max-depth` | `64` | Cap proof recursion. |
| `GOAL` | (positional) | Ground atom, e.g. `reachable(/a, /c)`. |

The `facts`-mode output uses the schema `proves`, `uses_rule`, `premise`, `edb_leaf`, `binding`, `rule_source` — these are the predicates codeNERD diagnostics consume.

---

## 2. Simplecolumn factstore — zstd, gzip, raw

`factstore/simplecolumn.go` is a columnar, on-disk fact format with three encodings:

| Constructor | Compression |
|-------------|-------------|
| `factstore.NewSimpleColumnStoreFromBytes(data)` | none (raw) |
| `factstore.NewSimpleColumnStoreFromGzipBytes(data)` | gzip |
| `factstore.NewSimpleColumnStoreFromZstdBytes(data)` | **zstd (new in fork)** |

Each returns `*SimpleColumnStore` which exposes `EstimateFactCount()`, `FactCount(pred)`, `ListPredicates()`, plus the standard `ReadOnlyFactStore` surface.

### 2.1 The `scinfo` CLI

Source: `cmd/scinfo/main.go`. Verified usage:

```
scinfo <file>
```

Detects compression by extension (`.gz`, `.zst`/`.zstd`, else raw) and prints:

```
file:        path
size:        bytes
compression: none|gzip|zstd
predicates:  N
facts:       M

PREDICATE  ARITY  FACTS
...
```

Use this to sanity-check a `.sc` fact bundle before feeding it into `mgwhy` or replaying it into a fresh evaluation.

---

## 3. Temporal reasoning

The fork has a substantial temporal extension that upstream Mangle does not. Facts carry validity intervals; rules can use Allen-algebra operators. See `readthedocs/temporal.md` (vendored under `C:\Users\smoor\go\pkg\mod\codeberg.org\!tau!ceti\mangle-go@v0.5.1-.../readthedocs/temporal.md`) for the full spec.

### 3.1 Syntax — interval annotation

```mangle
# Closed interval
team_member(/alice, /engineering)@[2020-01-01, 2023-06-15].

# Half-open into the future
team_member(/bob, /engineering)@[2019-06-01, _].

# Point in time
login(/alice)@[2024-03-15T10:30:00].

# From a date until evaluation 'now'
active(/alice)@[2024-01-01, now].

# Happening right now
logged_in(/bob)@[now].
```

Bounds:
- `_` — unbounded (start or end of time).
- `now` — substituted with evaluation time.

A fact without `@[...]` is eternal (the historical behavior).

### 3.2 Temporal operators

| Operator | Glyph | Meaning |
|----------|-------|---------|
| Diamond-minus | `<-[D1, D2]` | "True at some point in the past window." |
| Box-minus | `[-[D1, D2]` | "True continuously through the past window." |
| Diamond-plus | `<+[D1, D2]` | "Will be true at some point in the future window." |
| Box-plus | `[+[D1, D2]` | "Will be true continuously through the future window." |

Bounds are durations (`d`, `h`, `m`, `s`, `ms`), ISO timestamps, variables (`_` for unbounded) or
`now` (`parse/gen/Mangle.g4`, `temporalBound`).

**codeNERD cannot evaluate any of this.** Its kernel configures no temporal store: these rules
parse and analyse, and evaluation then aborts with `temporal literal encountered but no temporal
store configured` (verified 2026-09-19 with `nerd check-mangle --eval`). No codeNERD policy file
uses temporal syntax; keep it that way until a temporal store is wired.

```mangle
Decl active(X) temporal bound [/name].
Decl operated(P, E) temporal bound [/name, /name].
Decl certified(P, E) temporal bound [/name, /name].
Decl recently_active(X) bound [/name].
Decl reliably_active(X) bound [/name].
Decl certified_recently(P, E) bound [/name, /name].
Decl uncertified_operation(P, E) bound [/name, /name].

recently_active(X) :- <-[0d, 30d] active(X).
reliably_active(X) :- [-[0d, 30d] active(X).
# A temporal operator cannot follow '!': project first, then negate the projection.
certified_recently(P, E) :- <-[0d, 30d] certified(P, E).
uncertified_operation(P, E) :- <-[0d, 30d] operated(P, E), !certified_recently(P, E).
```

### 3.3 Allen interval predicates

Builtins under `:interval:` — `before`, `after`, `meets`, `overlaps`, `during`, `contains`, `starts`, `finishes`, `equals`. Each takes two interval values.

Interval inspection: `fn:interval:start(T)`, `fn:interval:end(T)`, `fn:interval:duration(T)` — return nanosecond integers.

### 3.4 Declaring a temporal predicate

```mangle
Decl employee_status(Person, Status) temporal bound [/name, /string].
```

If a predicate is declared `temporal`, **every** fact and rule for it must carry an interval. Mixing temporal and eternal facts under one predicate is a declaration error.

### 3.5 Go-side temporal wiring

Engine options:

```go
engine.WithTemporalStore(factstore.NewIntervalTreeStore())  // interval-indexed store
engine.WithEvaluationTime(time.Now())                       // pins 'now'
engine.WithNowMarker()                                      // marks rules that depend on 'now'
```

For codeNERD: temporal predicates are well-suited to session-lifetime facts (perception windows, watchdog timers, sliding context budgets). Do not retrofit existing eternal predicates without re-declaring them.

---

## 4. Tagged-union type constructor

A discriminated union built on top of struct types. From `readthedocs/typeexpressions.md`:

```mangle
Decl api_message(M)
  bound[
    .TaggedUnion</type,
      /create : .Struct</name : /string, /count : /number>,
      /delete : .Struct</id : /number>,
      /ping   : .Struct<>
    >
  ].

api_message({/type: /create, /name: "widget", /count: 5}).
api_message({/type: /ping}).
```

- First positional argument is the **tag field** (a name constant, e.g. `/type`, `/kind`).
- Each subsequent pair is `/variantTag : .Struct<...>`.
- The tag field is added to every variant automatically; do not redeclare it inside the variant struct.

Values are ordinary structs:

```mangle
event({/kind: /user_login, /user_id: 101, /ip_address: "10.0.0.1"}).
event({/kind: /bulk_import, /items: ["a", "b", "c"]}).

login_user(U) :-
    event(E),
    :match_field(E, /kind, K), K = /user_login,
    :match_field(E, /user_id, U).
```

Semantically equivalent to a union of structs that each pin the tag field to a singleton type, but more concise and the dispatcher is explicit. Prefer the tagged-union form when modeling externally-tagged JSON-like payloads.

`symbols.NewUnionType` (Go-side) constructs the equivalent union programmatically when generating types from external schemas.

---

## 5. Engine options new in the fork

Defined in `engine/seminaivebottomup.go` and `engine/recorder.go`:

| Option | Purpose |
|--------|---------|
| `engine.WithCreatedFactLimit(N)` | Gas limit. Eval aborts after `N` derived facts. Use to bound runaway recursion in untrusted policy. |
| `engine.WithExternalPredicates(...)` | Register Go callbacks that supply facts for opaque predicates on demand. Bridges Mangle to external stores without pre-loading. |
| `engine.WithDerivationRecorder(r)` | Plug a `DerivationRecorder` (e.g. `provenance.MemoryRecorder`) into evaluation. |
| `engine.WithTemporalStore(s)` | Supply an interval-tree-indexed temporal store. |
| `engine.WithEvaluationTime(t)` | Pin the `now` reference for temporal rules. |
| `engine.WithNowMarker()` | Mark rules that depend on the current time so their derived facts can be invalidated when `now` advances. |
| `engine.WithDeterministicOrder()` | Force a deterministic predicate-visit order — important for reproducible proof trees and golden tests. |

For codeNERD: every kernel evaluation should use `WithCreatedFactLimit` and `WithDeterministicOrder` in production. Don't add `WithDerivationRecorder` unconditionally — recording every event has measurable overhead.

---

## 6. Inclusion checking

`engine/inclusioncheck.go` — analysis pass that verifies that an asserted fact set is consistent with a program's intent (every fact is derivable / every required fact is present, depending on direction). Not in upstream. Useful for invariant testing — assert "ground truth" facts, run inclusion-check, fail if anything is missing or inconsistent.

This is the right hook for codeNERD's "post-action validation" pass — phrase the postconditions as facts and inclusion-check against the kernel's derived state.

---

## 7. Normalized delta predicates

In the fork's seminaive engine (`engine/seminaivebottomup.go`), the internal Δ-prefixed delta predicates are normalized out before consumers see them. You will never observe a `Δfoo` predicate via `ListPredicates()` or in derived facts — only `foo`. This matters when comparing fact counts across engine and store: the engine's working set is larger than what the store ultimately reports.

---

## 8. UnionFind.Clone

`unionfind.UnionFind` has a `Clone()` method (added 2026-03-15). Useful when branching evaluation: clone the substitution state, try a rule, backtrack on failure without mutating the parent.

---

## 9. Migration notes for existing codeNERD policy

| Area | Action |
|------|--------|
| Imports in Go integrations | Replace `github.com/google/mangle/...` with `codeberg.org/TauCeti/mangle-go/...`. The package names are identical. |
| Go module pin | `codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3`. |
| `go install` of `mg` | Use `codeberg.org/TauCeti/mangle-go/interpreter/mg@latest` (this repo, not the upstream Google path — there is no `cmd/mg` here). |
| `mgwhy` / `scinfo` install | `go install codeberg.org/TauCeti/mangle-go/cmd/mgwhy@latest` and `.../cmd/scinfo@latest`. |
| Existing rules | No syntax break. Tagged unions and `@[...]` annotations are additions, not replacements. |
| Existing fact bundles | `.sc` (raw) and `.sc.gz` continue to work. `.sc.zst` is new. |

---

## 10. What is **not** in the fork (still upstream-only or absent)

- No `github.com/google/mangle/cmd/*` tools — only the two new fork tools (`mgwhy`, `scinfo`) plus the existing `interpreter/mg`.
- The fork is library-first. Most CLI utilities you see in upstream snippets do not exist here.
- The fork does **not** add string-matching, regex, or fuzzy-matching builtins. The Mangle position on "not for fuzzy matching" still holds.

---

## See also

- [GO_API_REFERENCE](GO_API_REFERENCE.md) — broader Go surface, updated to fork imports.
- [900-ECOSYSTEM](900-ECOSYSTEM.md) — production deployment patterns including the new CLIs.
- [020-ENGINE_TRUTHS_v0.5.1](020-ENGINE_TRUTHS_v0.5.1.md) — behaviour verified by running it on this commit. (The 57 KB dump of the downstream 0.4.0 documentation, context7-mangle.md, was deleted 2026-09-18: it described an engine this repository does not run.)
- [SHARDING_STRATEGIES](SHARDING_STRATEGIES.md) — partitioning policy and facts across kernels.
- User-memory: `reference_mangle_fork_features.md` (snapshot of the v0.5.0 → v0.5.1 upgrade).
