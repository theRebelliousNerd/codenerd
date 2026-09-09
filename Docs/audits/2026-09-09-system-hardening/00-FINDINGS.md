# codeNERD System Hardening — Findings

Date: 2026-09-09
Branch: `claude/codenerd-system-hardening-1xorkw`
Scope: every subsystem — is it built, wired, called at the right time, and does
it put the right context in front of the model and nothing more?

## Baseline (measured, not assumed)

| Gate | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `go test ./...` | see [04-TEST-BASELINE.md](04-TEST-BASELINE.md) |
| Non-test Go LOC | ~250k across 41 `internal/` packages |
| Test Go LOC | ~230k |

The codebase compiles and vets clean. The defects here are **wiring and
selection** defects, not compile defects: capability that exists, is tested in
isolation, and never runs in production; and context that reaches the model
without earning its tokens.

---

## F1 — The impact-aware holographic path is dead in three places at once

**Severity: high. This is the flagship "holographic awareness" feature and it
has never executed in production.**

The chain is supposed to be:

```
a function is edited
  -> modified_function(Func, File)              [EDB]
  -> impact_caller(Target, Caller)              [impact.mg:29]
  -> impact_graph(Target, Caller, Depth<=3)     [impact.mg:41-53]
  -> context_priority_file(File, Func, Prio)    [impact.mg:68-78]
  -> HolographicProvider.BuildWithImpactPriorities
  -> HolographicContext.PrioritizedCallers
  -> PromptSection "### Callers (impact-prioritized)"
```

Three links are cut:

1. **No producer for `modified_function`.** The predicate is declared
   (`schemas_reviewer.mg:243`) and a rule consumes it (`impact.mg:30`), but the
   only two Go references are an *allow-list* entry letting the LLM volunteer
   the fact (`internal/session/executor.go:1500`) and a shard's owned-predicate
   list (`internal/shards/registration.go:82`). Nothing derives it from what the
   system actually did. The kernel is supposed to be the executive; here it is
   waiting for the model to tell it what changed.

2. **`BuildWithImpactPriorities` has zero non-test callers.** All 730 lines of
   `internal/world/holographic_impact.go` — the priority query, the caller-body
   fetcher, the depth sort, the bounded 10-caller cap — run only under test.

3. **The prompt branch is therefore unreachable.** `getContextInternal`
   (`holographic.go:408`) never populates `PrioritizedCallers`, so
   `PromptSection`'s `if len(hc.PrioritizedCallers) > 0` (`holographic.go:341`)
   is always false in production. Every real turn falls through to the plain
   `CallGraph` branch: an unordered list of caller names with no ranking.

**What the model gets today** where it should get impact-ranked callers: a
deduplicated, arbitrarily-ordered list of up to 8 names.

## F2 — Signature selection spends its whole budget on noise for large packages

`PromptSection` shows up to 8 exported signatures. It prefers symbols defined in
the target file, and falls back to package-wide symbols in **directory read
order** when the target file defines none.

Measured on `internal/core/kernel.go` (a package-marker file whose implementation
lives in `kernel_*.go`, so it exports nothing itself):

```
### Exported signatures
- `func NewValidatorRegistry() (*ValidatorRegistry)` — `action_validator.go`
- ... 7 more from action_validator.go ...
- … and 592 more
```

Eight slots, all spent on `action_validator.go` because it sorts first
alphabetically, and a "592 more" that tells the model its view is 1.3% complete.
This is the exact inverse of "the right context and nothing more": the tokens are
spent, and the signal is zero.

## F3 — Six declared dimensions of `HolographicContext` are never populated

`DirectImports`, `DirectImporters`, `ExternalDeps`, `RelatedEntities`,
`ComplexityHints`, `TODOCount` are struct fields with JSON tags on the type that
represents the system's "X-ray vision" (`holographic.go:26-62`). Verified by
grep: **no assignment and no read anywhere in the repo, test or not.**

`PackageImports` *is* populated (`holographic.go:546`) and never rendered.
`CountTODOs` exists (`holographic_formatting.go:233`) with zero callers.

Reverse dependency ("who depends on this package") is the single most useful
dimension for judging blast radius, and it is declared and absent.

## F4 — Holographic context is recomputed from scratch on every turn

`PromptSection` -> `getContextInternal` -> `buildGoContextWithContext` re-reads
the target's directory and re-parses up to 100 sibling files with
`parser.ParseComments` on **every call**. It is called from
`Executor.withFileContext` (`internal/session/executor_tools.go:894`) at
`internal/session/executor.go:933` — i.e. once per LLM turn with a file target.

Measured (`internal/core`, 150+ files):

```
BenchmarkHolographicPromptSectionLargePkg-4   5   60960352 ns/op   12282140 B/op   293557 allocs/op
```

**61 ms and 12 MB of garbage per turn** to produce ~1.6 KB of prompt text that is
byte-identical until a file in the package changes. There is cache
infrastructure in the same package (`cache.go`, `dataflow_cache.go`) that this
path does not use.

## F5 — Dead exported surface in the holographic package

Zero non-test callers, verified by grep across all non-test `.go`:

| Symbol | File |
|---|---|
| `BuildWithImpactPriorities` | `holographic_impact.go:42` |
| `FormatWithPriorities` | `holographic_impact.go:600` |
| `FormatPrioritizedCallersCompact` | `holographic_impact.go:662` |
| `HasPrioritizedCallers` | `holographic_impact.go:710` |
| `GetHighPriorityCallers` | `holographic_impact.go:715` |
| `FormatForPrompt` | `holographic_formatting.go:99` |
| `FormatSignaturesCompact` | `holographic_formatting.go:209` |
| `CountTODOs` | `holographic_formatting.go:233` |

Per `nerd.md`'s `audit-before-delete` convention these are wiring gaps, not
garbage: each one is the rendering or accessor half of a capability whose
producer half is also unwired. The fix is to connect them, not delete them.

---

## Remediation program

See [01-PLAN.md](01-PLAN.md).

---

## F6 — Two registered tools failed on every call

`run_impacted_tests` and `get_impacted_tests` (`internal/tools/codedom/run_impacted_tests.go:72`,
`:114`) are registered into both the VirtualStore registry and the global
registry (`internal/core/virtual_store_tools.go:91`, `:95`) and advertised to the
model in `internal/prompt/atoms/capability/codedom_tools.yaml`. Both open with a
nil check on a package-level provider set only by `RegisterTestImpactProvider`,
which had **no non-test caller anywhere in the repo**.

Every invocation returned `test impact provider not initialized`. The model was
told it could scope its test runs, spent a turn and a tool-budget slot asking,
and learned the capability does not exist.

Underneath that, a second defect that would have kept the tools silent even once
wired: `goTestFuncPattern` was `:Test[A-Z]` matched against the raw ref, while
the Go parser builds refs as `fn:<pkg>.<Name>` (`go_parser.go:174-178`). A real
test function is `fn:world.TestFoo`; the character after the colon is `w`. The
pattern only ever matched the unqualified `fn:<name>` form the tree-sitter path
emits (`ast_treesitter.go:487`). `testFuncs` stayed empty, so
`buildDependencyEdges` added no edges, so `GetImpactedTests` returned nothing for
every input. The Python and Rust checks had the same defect.

## F7 — The adversarial tool gate could not fail

`internal/campaign/tool_pregenerator.go:544` `runThunderdomeForTool` was a stub
returning `(true, nil)` unconditionally. It was called under
`RequireThunderdome`, which **defaults to true**
(`DefaultPregeneratorConfig`, `:65`). Every pregenerated tool was recorded
`PassedThunderdome` without a single attack vector being fired.

The Ouroboros loop does run a real Thunderdome (`internal/autopoiesis/ouroboros.go:507`,
`EnableThunderdome` default true), but `LoopResult` carried no verdict, so the
campaign had nothing to read and invented a pass.

## F8 — Safety queries failed open

`Kernel.Query` returns `([]Fact, error)`. Dropping the error makes "the kernel
could not evaluate" indistinguishable from "the kernel found nothing wrong".

- `internal/core/shadow_mode.go:325,341,357,377` — `block_commit`,
  `unsafe_to_refactor`, `chesterton_fence_warning`, `projection_violation`.
  A stratification error, parse failure or cancelled fixpoint returned zero
  violations and the caller read `IsSafe`.
- `internal/core/transaction_manager.go:298` — `deny_edit`. A failed query
  produced zero `SafetyBlock`s and left `IsValid` true.

In a system whose constitution is default-deny, these inverted the default for
exactly the cases where the kernel is least healthy.

Found while fixing it: `RealKernel.Query` **panicked on a nil receiver**. Query
is reached through several interfaces (`core.Kernel`, `world.FactQuerier`, the
shadow querier), and a nil `*RealKernel` in an interface is a non-nil interface,
so a caller's `if k == nil` does not fire. `ShadowMode.shadowKernel` is nil until
`StartSimulation` clones the parent.

## F9 — CodeDOM facts leaked for the lifetime of a session

`clearCodeDOMFacts` (`internal/core/virtual_store.go`) listed 20 element and
diagnostic predicates and **none of the 32 per-language Stratum-0 predicates**
the parsers emit through `world.FileScope.safeParseFile` (`scope.go:525`) and
return in `ScopeFacts`.

Those facts were added on every `open_file`, `edit_element` and `refresh_scope`
and never removed. Worse than the EDB growth: the kernel kept deriving from
files that had left scope entirely — a `go_struct` for a type the user stopped
looking at half an hour ago still satisfied every rule that joins on it.

Verified empirically by parsing fixtures through the real parser factory: 23
language predicates emitted by the fixture set, 32 across all parsers, zero
retracted.

---

## Measured results

| Change | Before | After |
|---|---|---|
| `PromptSection` on `internal/core` (per LLM turn) | 49.4 ms, 12.6 MB, 296,445 allocs | **1.2 ms, 532 KB, 1,165 allocs** |
| Impact-prioritized callers in the prompt | never (branch unreachable) | derived from `modified_function` on every CodeDOM edit |
| Direct-caller priority as rendered | `MINIMAL` (raw 3 vs 80/50/25 buckets) | `CRITICAL` (100) |
| `run_impacted_tests` / `get_impacted_tests` | error on every call | return real impacted tests |
| Thunderdome gate | always passed | passes only on a real surviving battle |
| Shadow safety queries | fail open | fail closed with a named reason |
| CodeDOM predicates retracted on scope change | 20 of 52 | 52 of 52, pinned by a conformance test |
