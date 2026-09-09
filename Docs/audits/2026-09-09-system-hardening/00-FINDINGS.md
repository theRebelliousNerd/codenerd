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

---

## F10 — The reviewer self-correction loop was starved

`internal/core/defaults/reviewer.mg` Section 11 is a complete learning loop:
`user_rejected_finding/5` → `review_rejection_count/2` and
`review_suspect(ReviewID, "multiple_rejections")` → `reviewer_needs_validation/1`,
which `cmd/nerd/chat/process.go:596` reads back through
`ShardManager.CheckReviewNeedsValidation`.

Nothing produced `user_rejected_finding` or `user_accepted_finding`. Both are
declared (`schemas_tools.mg:339,343`) and joined by rules; the Go side had
`AcceptReviewFinding` / `RejectReviewFinding` guarded on a
`ReviewerFeedbackProvider` that **no type in the repo implements** and that
nothing calls `SetReviewerFeedbackProvider` for.

So the chat commands where a person says "this finding was a false positive"
(`cmd/nerd/chat/commands_handlers_misc.go:205`, `:255`) dropped that judgement
into a nil check. `GetReviewAccuracyReport` returned the fixed string
`"Review feedback provider not available"` in every production process.

Same shape as F1: a complete Mangle loop with no Go producer.

Note the asymmetry that hid it — `CheckReviewNeedsValidation` and
`GetReviewSuspectReasons` already *read* from the kernel when the provider is
nil. Only the write half was missing.

## F11 — A feature flag an operator could set with no effect

`features.IsSystemShardsEnabled()` is documented as "the master switch for
booting the autopoiesis/observer background shards" and is reported by
`nerd features`. It had **zero non-test callers**: `CODENERD_SYSTEM_SHARDS=0`
did nothing.

Its doc comment also described a legacy `NERD_DISABLE_SYSTEM_SHARDS` env var
"parsed at the call site". That string appears in **no `.go` file in the repo**.
Nine documents under `Docs/architecture/` repeat the claim, sourced from this
comment. The real per-shard mechanism is the `--disable-system-shard` CLI flag
(`cmd/nerd/main.go:189`).

A flag an operator can see, read a description of, and set with no result is
worse than an absent one: it makes them believe they have turned something off.

## F12 — Thirteen Mangle rules that could never fire

`jit_logic.mg` held thirteen `atom_has_*_match(AtomID)` rules, one per selector
dimension, all joining `atom_selector/3`. Inert at both ends:

- **No production producer.** The only Go emitter of `atom_selector` is
  `PromptAtom.ToSelectorFacts` (`internal/prompt/atoms.go:540`), called from
  `atoms_test.go` and `atom_pinning_test.go` only. The live selector emits a
  different vocabulary entirely, matching `promptEphemeralPredicates`
  (`internal/prompt/compiler.go:60-75`). `atom_selector` is not in it.
- **No consumer.** Nothing in any `.mg` or `.go` file referenced
  `atom_has_*_match`. `atom_matches_context` joins `prompt_atom` and
  `atom_context_boost`; it never mentions them.

They derived nothing from nothing at N×M join cost per compile. What kept them
looking alive was a golden test (`testdata/jit_logic.edb`) that seeded
`atom_selector` facts by hand.

**Dimensional matching is not missing.** It happens on the live path:
`jit_compiler.mg` joins `atom_tag` against `current_context` to derive
`blocked_by_context`, and `selected_result/3` is what `selector.go:877` and
`:1044` query.

### Related, recorded and deliberately not fixed

`atom_context_boost` is declared as a virtual predicate ("Go-computed boost")
and has no producer either — `RegisterVirtualPredicate`
(`internal/mangle/differential.go:847`) has **no call site anywhere in the
repo**, so the virtual-predicate mechanism is entirely unused. The
non-mandatory branch of `atom_matches_context` therefore does not fire in
production. Whether to implement the boost or drop the branch is a design
decision, not a wiring fix, so it is recorded here rather than acted on.

## F13 — A content-free holographic section for undescribable targets

`PromptSection`'s architecture facets are inferred from path patterns, so they
are produced even for a file that does not exist. A missing or non-Go target
rendered a header, an inferred `**Role**`, and `**Tests**: no` — prompt tokens
spent to say nothing, on the turn where the model is already looking at a path
that is not there. The facet separator also had a dangling-` · ` bug when the
package clause was absent, which reads like content was dropped.

Found by a test written for the campaign wiring in F14.

## F14 — The campaign's holographic provider was stored and never read

`IntelligenceGatherer` is handed a `*world.HolographicProvider` at every one of
its six construction sites. The only three occurrences of the field were its
declaration (`intelligence_gatherer.go:71`), the constructor parameter (`:325`)
and the assignment (`:335`). The campaign's most decision-relevant context —
what the target file offers, what its package holds, who calls into it — was
gathered nowhere.

---

## F15 — 65 starved predicates, and a gate so the number cannot grow silently

Every defect in this audit that took real work to find was the same shape: a
**starved predicate** — declared, joined by a rule body, and produced by
nothing. No rule head, no ground fact, no Go code naming it. Rules that read one
derive nothing, forever, and no test fails, because both halves are correct in
isolation.

Four were found and fixed the hard way:

| Predicate | What it starved |
|---|---|
| `modified_function` | The entire caller-impact chain, and with it the flagship impact-prioritized holographic context (F1). |
| `user_rejected_finding` / `user_accepted_finding` | reviewer.mg's self-correction loop (F10). |
| `atom_selector` | Thirteen dimension rules — dead at both ends, removed (F12). |

A systematic sweep of the corpus found **65** more. The full inventory is
`internal/core/defaults/testdata/starved_predicates.txt`.

### The gate

`TestStarvedPredicateBudget` (`internal/core/defaults/starved_predicate_test.go`)
parses every `.mg` file for Decls, rule heads and body uses, cross-references
every lowercase quoted identifier in non-test Go, and diffs the result against
the checked-in baseline. It fails when:

- a rule starts reading a predicate nothing produces, **and**
- a predicate on the list gains a producer without leaving the list.

The second direction matters as much as the first: without it the baseline rots
into a file nobody trusts, and the count stops being a measurement.

The Go-producer check is deliberately over-approximate — any predicate name
appearing as a quoted string in non-test Go counts as possibly produced. A false
"produced" costs one missed finding; a false "starved" costs someone hunting a
bug that is not there.

The target is not zero. Some entries are genuinely optional inputs an operator
or an integration supplies. But the list is now visible, diffed on every test
run, and can only shrink by accident of someone doing the work.

Regenerate with:

```
CODENERD_UPDATE_STARVED=1 go test ./internal/core/defaults/ -run TestStarvedPredicateBudget
```

Opt-in through the environment rather than a test flag, because a gate that
rewrites its own baseline on failure is not a gate.

## F16 — 869 unreachable functions, and a budget so that number cannot grow either

F15 gates the Mangle half of the recurring defect. This is the Go half.

Rapid type-aware reachability (RTA) from every `main` package
(`golang.org/x/tools/cmd/deadcode`) reports **869 functions unreachable from any
binary**. Unlike a grep, it understands interfaces and method sets, so an
implementation reached only through an interface counts as live.

Package concentration (top 10):

| Package | Unreachable |
|---|---|
| `internal/tactile` | 93 |
| `internal/core` | 77 |
| `internal/logging` | 68 |
| `cmd/nerd/ui` | 68 |
| `cmd/nerd/chat` | 41 |
| `internal/perception` | 39 |
| `internal/world/lsp` | 37 |
| `internal/world` | 36 |
| `internal/tactile/python` | 34 |
| `cmd/nerd` | 33 |

Not all of it is rot — platform-specific paths, accessors kept for symmetry, and
code reachable only from a path RTA cannot see all appear here. The number is
not meant to reach zero.

`scripts/deadcode-budget.sh` baselines it and fails on drift in both directions,
the same contract as the starved-predicate gate. It is a **script, not a Go
test**, because the analysis needs a tool fetched from the module proxy: a test
that silently skips when the cache is cold gives false assurance, and one that
requires the network makes `go test ./...` fragile. Run it in CI.

```
scripts/deadcode-budget.sh            # check
scripts/deadcode-budget.sh --update   # rebaseline
```

Line and column are stripped from the baseline so it survives edits that only
move code.

---

## The two gates, together

The defects in this audit divide cleanly:

| | Mangle side | Go side |
|---|---|---|
| Defect | predicate declared and read, produced by nothing | function defined and exported, called by nothing |
| Why invisible | both halves correct in isolation | same |
| Instances found | `modified_function`, `user_{accepted,rejected}_finding`, `atom_selector` | `BuildWithImpactPriorities`, `RegisterTestImpactProvider`, `IsSystemShardsEnabled`, `RegisterFileValidators` … |
| Gate | `TestStarvedPredicateBudget` | `scripts/deadcode-budget.sh` |
| Baseline | 65 | 869 |

Neither number is a target. Both are now measurements that cannot move without
someone noticing.
