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
