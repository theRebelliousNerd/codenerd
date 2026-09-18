# S19 — Compilation scope measurement: the cost of cloning the live kernel per prompt compile

## Status

- last updated: 2026-09-18
- done: 1 mechanism (verified against source); 2 numbers (re-measured from two verifiable
  sessions; the three sessions named in the brief do not all exist in the current
  `.nerd/logs/` — see open items); 3 what the compile actually needs (traced predicate by
  predicate); 4 design options; 5 recommendation; 6 open
- open: pid `030000`/`21:24` session and the `05:06–05:22 nerd fix` run are not present in
  the current log directory (see §2.0); whether the wired JIT ruleset alone stratifies to
  fewer than 913 strata was not directly measured (§4, Option A); a few smaller items in §6

## 1. The mechanism, verified

### 1.1 The interface and its two production implementations

`KernelScopeProvider.NewCompilationScope() (KernelCompilationScope, error)`
(`internal/prompt/compiler.go:57-59`) is the seam. `acquireCompilationKernel`
(`internal/prompt/compiler.go:924-960`) is the only caller: if the compiler's kernel
implements `KernelScopeProvider` it calls `NewCompilationScope()` and defers
`scope.Close()`; otherwise (a legacy adapter with only `KernelRetracter`) it falls back to
retracting a fixed list of ephemeral predicates after the compile
(`promptEphemeralPredicates`, `compiler.go:61-77`).

Two implementations exist, both wrapping the same underlying method:

- **Production**: `internal/system/factory_adapters.go:79-101` `KernelAdapter.NewCompilationScope`
  unwraps to a `*core.RealKernel` (directly, or via a `GetPrimaryRealKernel()` accessor) and
  returns `NewKernelAdapter(live.Clone())` wrapped in a `kernelCompilationScope`.
  `kernelCompilationScope.Close()` (`:72-77`) just nils the embedded adapter — the clone is
  discarded to the garbage collector, there is no explicit retract.
- **Init path**: `internal/init/jit_integration.go:130-140` `initJITKernelAdapter.NewCompilationScope`
  does the same thing (`a.kernel.Clone()`) against the initializer's own, much smaller,
  `*core.RealKernel` (documented at `:16-25` as a separate adapter because `internal/system`
  cannot be imported from `internal/init` — a genuine duplicate, not a divergent one: both
  call the identical `RealKernel.Clone()`).

Both call sites invoke the *same* `Clone()` (`internal/core/kernel_eval.go:694-775`), so
everything below applies to both, differing only in how many EDB facts the source kernel
holds at clone time (the init kernel is small; the production kernel carries the whole
session's world/session/history EDB).

### 1.2 What `Clone()` actually does (`kernel_eval.go:694-775`)

Contrary to the "re-stratifies" framing in the corpus-usage study, `Clone()` **shares**, by
reference, everything that stratification produced:

```go
programInfo:       k.programInfo,   // Share programInfo (immutable after analysis)
strata:            k.strata,        // Share strata (immutable after stratification)
predToStratum:     k.predToStratum, // Share predToStratum (immutable after stratification)
schemas/policy/learned: k.schemas / k.policy / k.learned,  // same source strings
virtualStore:      k.virtualStore,  // SAME VirtualStore instance — externals travel with it
```

So `analysis.Stratify` (the pass that produces the 913 strata) is **not** re-run per compile.
What *is* freshly built on every clone:

```go
store:       factstore.NewSimpleInMemoryStore(), // empty
cachedAtoms: nil,  // "Rebuild fresh to avoid shared memory pointers"
eventBus:    NewFactEventBus(),
```

and deep-copied, per-argument (`deepCopyArg`, `:777-797`):

```go
facts:     make([]Fact, len(k.facts))       // every live EDB fact, args deep-copied
bootFacts, factIndex, loadedPolicyFiles      // also deep/shallow-copied as appropriate
```

`factsDirty` (an `atomic.Bool`, can't be struct-copied) is mirrored from the source kernel's
current value (`clone.factsDirty.Store(k.factsDirty.Load())`, `:734`).

**Consequence for the next query.** Because `cachedAtoms` is explicitly nilled but `facts` is
fully populated, the very next `evaluate()` on the clone — see
`evaluateFullLocked` (`kernel_eval.go:256-380`) — hits the cache-desync branch at `:263-300`:
`len(k.cachedAtoms) != len(k.facts)` is true (`0 != N`), so **every single cloned fact is
reconverted from `Fact` to `ast.Atom` via `factToAtomLocked`, from scratch**, before the
fixpoint runs. This reconversion, not stratification, is the clone-specific tax; it does not
exist on the live kernel's own re-evaluations because the live kernel keeps `cachedAtoms` in
lockstep with `k.facts` incrementally (`addFactIfNewLockedErr` appends to both).

### 1.3 What triggers the fixpoint on the scope

`JITPromptCompiler.compile()` (`compiler.go:636-911`), inside the singleflight-guarded
cache-miss branch:

1. `acquireCompilationKernel(c.kernel)` → `NewCompilationScope()` → `Clone()` (§1.1/1.2).
2. **Step 1** (`compiler.go:670-683`): `selectionKernel.AssertBatch(cc.ToContextFacts())` —
   asserts `compile_context(Dim, Value)` facts (`context.go:516-523`, via
   `CompilationContext.GenerateFacts`). `Assert`/`AssertBatch`
   (`internal/core/kernel_facts.go:555,677`) call `factsDirty.Store(true)` on the clone for
   any non-`system_heartbeat` predicate — this is what dirties the fresh clone.
3. **Step 2** (candidate collection, `collectAtomsWithStats`) runs **concurrently** with
   **Step 1.5b** `collectKernelInjectedAtoms` (`compiler.go:1053-1156`). That function queries
   `c.kernel.Query("injectable_context")` / `"specialist_knowledge")` — **`c.kernel`, the
   compiler's own live-kernel reference, not `selectionKernel`, the scope.** This is a live,
   uncloned-kernel read; it never goes through the scope and is unaffected by anything in this
   study about the clone (see §3).
4. **Step 3** (`AtomSelector.selectAtomsWithTimingKernel` → `runSelection` →
   `loadSkeletonAtomsKernel`/flesh path): `buildContextFacts` (`selector.go:1421-1421+`) is
   called at least twice per compile — once for the skeleton-category candidates
   (`selector.go:934`) and once for the flesh candidates (`selector.go:1173`) — each call
   emitting on the order of **15 facts per candidate atom**
   (`atom`, `atom_category`, `atom_priority`, `is_mandatory`, `prompt_atom`, `atom_tag`
   (one per tag dimension), `atom_requires`, `atom_conflicts`, `atom_requires_tool`) via
   `AssertBatch`/`LoadFacts`. Observed batch sizes (§2) match this: a 2,844-fact batch for
   ~316 candidate atoms, logged with `LoadFacts JIT: is_mandatory=191 atom_tag=1171
   atom_priority=316 … atom=316`.
5. The first `kernel.Query(...)` after step 2/4 (`querySkeletonAtoms`, `selector.go:971-1030`,
   querying `blocked_by_context`, `mandatory_selection`, then `selected_result(Atom, Priority,
   Source)`) calls `ensureEvaluated()` (`kernel_query.go:43`), which — because `factsDirty` is
   now true on the clone — runs `evaluate()` → `evaluateFullLocked()`: reconvert all cloned
   facts to atoms (§1.2), rebuild the store, run `engine.EvalStratifiedProgramWithStats` over
   the shared 913-strata program, replace `k.store`.
6. `Close()` drops the reference; the clone (facts, store, atom cache) is garbage-collected.
   No explicit retract ever runs against it.

### 1.4 The diff (incremental) path is never available here

`evaluate()` (`kernel_eval.go:181-231`) only takes the differential path when
`diffEvalEnabled() && !diffPathDemoted && proofRecorder==nil && !hasExternalPredicatesLocked()`,
and demotes immediately once `len(k.facts) > differentialFactCeiling` (**10,000**,
`kernel_types.go:161`). A compilation-scope clone inherits the source kernel's
`virtualStore` (§1.2), so `hasExternalPredicatesLocked()` (`:237-251`) is true whenever the
live kernel has any registered `external()` predicate — production always does (10, per
§2's logs). Combined with the fact count (52k–70k observed, §2), **the differential path
is unreachable for a compilation scope twice over**, independent of the `CODENERD_DIFF_EVAL`
flag's setting. Every compile-scope evaluation takes the full path.

### 1.5 Other callers of the identical `Clone()` — the confound that matters for §2

`grep -rn "\.Clone()"` over `internal/` and `cmd/` (excluding tests and worktrees) turns up
exactly three call sites that invoke `*core.RealKernel.Clone()`:

| Caller | File:line | Purpose | Log signature right after `Kernel cloned` |
|---|---|---|---|
| JIT compilation scope | `internal/system/factory_adapters.go:99` | one scope per `Compile()` call | `LoadFacts: loading N facts into EDB` where the facts are `compile_context(...)` |
| Dreamer safety projection | `internal/core/dreamer.go:295` (`evaluateProjection`) | clone + `AssertWithoutEval` projected facts + `clone.Evaluate()` (force) before every simulated action, to check `panic_state` | `Evaluate: forcing re-evaluation of all rules` (no preceding context-fact `LoadFacts`) |
| Shadow Mode | `internal/core/shadow_mode.go:131` (`StartSimulation`) | counterfactual simulation kernel, one per `sim_<nanos>` | asserts `shadow_state` next (not observed in the sampled sessions, so not in §2's tables) |

All three produce the **identical** `logging.KernelDebug("Kernel cloned (facts=%d,
policy=%d bytes)", ...)` line (`kernel_eval.go:773`) with no caller tag. A log-only study
that counts `"Kernel cloned"` lines and calls the total "N compiles" is conflating three
unrelated subsystems that happen to share one expensive primitive. §2 shows this is exactly
what happened to the corpus-usage study's number.

## 2. The numbers, re-measured from logs

### 2.0 Which sessions actually exist

The brief named three windows: 2026-09-17 21:24 (pid 030000), 2026-09-18 00:00 (pid 054828),
and 2026-09-18 05:06–05:22 (a `nerd fix` run). `.nerd/logs/` timestamps its files in the
filename using a clock **4 hours ahead of the timestamps printed inside the log lines**
(confirmed by comparing, e.g., file `20260918_035947...` against its own first log line
`2026/09/17 23:59:47...`). Listing every session actually present (`ls .nerd/logs/*.log`,
deduplicated to one row per session):

```
20260917_202104 pid061360   20260917_204446 pid042892   20260917_210740 pid062100
20260917_211550 pid065580   20260917_212008 pid058620   20260917_214029 pid057064
20260918_032709 pid065440   20260918_035623 pid067120   20260918_035947 pid054828
20260918_063702 pid035220   20260918_063832 pid069108   20260918_070626 pid061112
20260918_070633 pid064408   20260918_071019 pid014392   20260918_071027 pid042352
20260918_073105 pid045008   20260918_090622 pid048364
```

**pid 054828 exists** and matches the brief and M1's own citation (internal timestamps
23:59:47 → 00:07:58, i.e. the "2026-09-18 00:00" window). **pid 030000 does not exist** in
this directory under either clock convention (the nearest candidates by pid are unrelated
sessions; the nearest by either literal time — 21:24 filename-clock or 21:24 content-clock —
is `20260917_214029 pid057064`, which is used below as the second data point instead).
**No session starts in the 05:06–05:22 filename-clock window** (content-clock that would be
01:06–01:22, also absent); the closest run before the 03:27 session is 03:56 (pid067120, only
5 log files, no `jit.log` or populated `kernel.log` — likely a very short-lived process, not
a `nerd fix` run). This gap is recorded as open in §6 rather than papered over with a
plausible-looking but unverifiable number.

The two sessions below (054828 and 057064) are the ones actually re-measured.

### 2.1 Session pid 054828 (2026-09-17 23:59:47 → 00:07:58, ~8 minutes)

`grep -c "Kernel cloned" kernel.log` → **19** — the exact count M1 §9 / contract C-30 cite.
Classifying each by the signature in §1.5 (the line immediately following):

| Clone # | Timestamp | Facts | Signature | Subsystem |
|---|---|---|---|---|
| 1 | 00:00:15.471 | 52,632 | `LoadFacts: loading 6 facts` → `compile_context(...)` | **JIT compile** |
| 2–19 (18 clones, in 9 pairs) | 00:03:26 … 00:07:58 | 52,661–52,767 | `Evaluate: forcing re-evaluation of all rules` | **Dreamer `evaluateProjection`** |

Cross-checked against `jit.log` for the same session: **exactly one** `compilation_complete`
event, at 00:00:16.895 (`shard_id:coder intent_verb:/fix atoms_candidates:925
embedded_atoms:914 skeleton_atoms:38 flesh_atoms:46 tokens_used:21985 duration_ms:1423`) —
a 1:1 match with clone #1. The other 18 clones correlate one-for-one with the pattern in
`internal/core/dreamer.go:295`'s `evaluateProjection` (called from `SimulateAction`, run
before simulated write actions for `panic_state` safety checking) — 9 pairs about 2 seconds
apart each, consistent with a before/after check around 9 tool-call actions in this session.

**Cost of the one real JIT compile-scope evaluate**, read directly off `kernel.log` around
clone #1 (line 99870 onward):

```
Kernel cloned (facts=52632, policy=569969 bytes)
LoadFacts: loading 6 facts into EDB          # compile_context (Step 1)
Query: injectable_context/specialist_knowledge  # 0s — on the LIVE kernel, not this clone (§1.3 step 3)
LoadFacts: loading 2844 facts (skeleton pass)   # is_mandatory=191 atom_tag=1171 atom_priority=316 atom=316
LoadFacts: loading 5284 facts, 4256 added (flesh pass, 1028 rejected as dup/invalid)
kernel.lazy_evaluate triggered | factsDirty=true
evaluate: populating store with 59738 EDB facts
evaluate: cache empty (facts=59738), populating cache      # the reconversion tax, §1.2
evaluate: running fixpoint evaluation (derivedFactLimit=500000)
evaluate.fixpoint completed in 253.3709ms   (evalTime=200.9ms, strata=913)
evaluate completed in 344.2353ms
```

`evaluate completed in 344.2353ms` at 00:00:16.893 lines up almost exactly with
`compilation_complete`'s own timestamp (00:00:16.895) — i.e. **the kernel-side cost (clone +
reconvert + fixpoint) was ~344 ms of the compile's 1,423 ms total wall time, ≈24%.**
`stats.SelectAtomsMs` in the same `compilation_complete` line is 615 ms (the 344 ms kernel
cost plus the skeleton/flesh fact-building overhead and a second, cheaper query round);
`stats.CollectAtomsMs` (807 ms, unrelated to the kernel) is the larger single contributor.

**Cost of the 18 Dreamer clones**, sampled (clone at 00:03:47.171):

```
Kernel cloned (facts=52671, ...)
evaluate: populating store with 52675 EDB facts
evaluate: cache empty (facts=52675), populating cache
evaluate: fixpoint reached - strata=913, evalTime=59.7ms, wallTime=101.9ms
evaluate completed in 191.5726ms
```

~180–200 ms each — the same order of magnitude as the JIT clone, for the same structural
reason (fresh `cachedAtoms`, same shared 913 strata, similar EDB size at clone time).

**Session-wide fixpoint total**, counting every `"fixpoint reached - strata=913"` line
(script: sum `wallTime` across all matches) regardless of source kernel: **127 events,
112.65 s of wall time** (M1's "127… 106s" is the same measurement to within the rounding of
where the window was cut). Distribution: median 129 ms, 68 events ≤200 ms, 58 events
>500 ms (max 2.4 s). Of these 127:

- **1** is the JIT compile-scope evaluate (344 ms end-to-end, 253 ms of which is the fixpoint
  itself) — **≈0.3% of the session's total fixpoint wall time.**
- **18** are Dreamer clones (~180–200 ms typical, a few landing in the >500 ms tail under
  contention) — roughly another ~3–4 s, **≈3%**.
- **The remaining ~108 events (≈105 s, ≈93% of the session total) are the LIVE, uncloned
  kernel's own repeated full-path re-evaluations**, triggered by ordinary `Query()` calls from
  many unrelated subsystems (`[shard:world]` queries against `safe_action`, `diagnostic`,
  `delegate_task`, `activate_shard`, session-executor turn-evidence queries, etc.) throughout
  the session. These sometimes cost 1.6–2.1 s apiece — not because of reconversion (the log
  shows no `"cache empty"`/`"reconverting"` line ahead of several of them — the live kernel's
  `cachedAtoms` stays valid across ordinary asserts) but apparently from lock contention:
  several `"evaluate: running fixpoint evaluation"` lines appear within the same millisecond
  from what must be more than one goroutine serialized on `k.mu`, and the cheap (~50–130 ms)
  and expensive (~1.6–2.1 s) samples run over comparable fact counts (~52k–69k either way).

**Correction to M1 §9 / contract C-30.** The cited figure — "19 `Kernel cloned` lines … ~2 s
each … ≈40 s of 106 s of evaluation" — attributed to prompt compilation a cost that is
mostly (a) a different subsystem (Dreamer, 18 of the 19 clones) and (b) not attributable to
cloning at all (the ~1.6–2.1 s samples are live-kernel re-evaluations that would happen with
or without any `NewCompilationScope` call). **The one JIT compile-scope evaluate this session
actually paid cost 344 ms, not ~2 s, and consumed ≈0.3% of the session's Mangle evaluation
time, not ≈38%.**

### 2.2 Session pid 057064 (2026-09-17 17:40:29 → 17:52:00, ~12 minutes) — corroboration

`grep -c "Kernel cloned"` → **3**, all three matching the JIT signature exactly (`LoadFacts:
loading N facts` → `compile_context(...)`), and `jit.log` for the same session independently
shows **3** `compilation_complete` events (`duration_ms: 1076, 620, 727`) — a clean 1:1 match
with no Dreamer/Shadow interleaving in this session.

| Clone | EDB at clone | EDB after skeleton+flesh facts | fixpoint evalTime / wallTime | `evaluate completed` | Compile() `duration_ms` | kernel share |
|---|---|---|---|---|---|---|
| 1 (17:42:57.9) | 3,619 | 10,494 | 51.3 ms / 92.1 ms | 103.7 ms | 1,076 | ~10% |
| 2 (17:49:45.9) | 4,088 | 11,157 | 69.4 ms / 110.7 ms | 123.1 ms | 620 | ~20% |
| 3 (17:51:59.7) | 4,131 | 10,995 | 85.5 ms / 126.9 ms | 139.5 ms | 727 | ~19% |

This session's live EDB (3.6k–4.1k facts before the compile's own atom facts land) is much
smaller than session 054828's (52k+), and the per-compile kernel cost scales down with it
(~104–140 ms vs ~344 ms) — consistent with §1.2's account of the dominant clone-specific cost
being the O(EDB size) atom-cache reconversion, not a fixed per-compile penalty.

### 2.3 What is and isn't logged

Logged today, sufficient for the reconstruction above: `Kernel cloned (facts=N, policy=N
bytes)`; `LoadFacts:`/`Assert:` lines showing which facts landed and in what predicate mix
(the `LoadFacts JIT: is_mandatory=… atom_tag=… …` summary line is particularly useful);
`kernel.lazy_evaluate triggered`/`Evaluate: forcing re-evaluation`; `evaluate: populating
store with N EDB facts`; `evaluate: cache empty …`/`reconverting N facts` (present only when
the atom cache was actually invalidated); `evaluate.fixpoint completed in Xms` and `fixpoint
reached - strata=913, evalTime=X, wallTime=Y`; `evaluate completed in Xms`; and, separately,
`jit.log`'s `compilation_complete` event with the full `CompilationStats` map.

**Not logged, and needed to do this reconstruction reliably instead of by hand-correlating
timestamps**: (1) a caller/purpose tag on `Clone()` and on `evaluate()` distinguishing JIT
compile scope vs. Dreamer projection vs. Shadow simulation vs. the live kernel's own
lazy-evaluate — today this requires reading the lines that happen to follow (§1.5); (2) a
shared correlation ID between a `Kernel cloned` line and its `compilation_complete` line (the
timestamp-adjacency match used above is reliable only because these sessions had at most one
compile in flight at a time — it would not hold under concurrent compiles, which the
compiler's own singleflight/goroutine design explicitly allows for); (3) the split between
"reconversion time" and "fixpoint propagation time" within `evaluate: cache empty …` →
`running fixpoint evaluation` is inferable from the gap between those two timestamps but is
not itself logged as a duration.

## 3. What the compile actually needs

Tracing every predicate `jit_compiler.mg` (`internal/core/defaults/jit_compiler.mg`) and the
**wired** subset of `policy/jit_selection.mg` (`internal/core/defaults/policy/jit_selection.mg`)
read, against where each is asserted (`internal/prompt/context.go:516-523` `ToContextFacts`;
`internal/prompt/selector.go:1421+` `buildContextFacts`; the vector-hit/retrieved-context
emission in `selector.go` around `:1199,1207`):

Go queries exactly three predicates from a compilation scope (§1.3 step 5; the two debug
queries plus the one actually consumed): `blocked_by_context(Atom)`, `mandatory_selection(Atom)`,
`selected_result(Atom, Priority, Source)`. Everything those three predicates transitively
depend on, per `jit_compiler.mg`:

`selected_result` ← `final_valid`, `atom_priority`, `mandatory_selection`
`final_valid` ← `tentative`, `!invalid`
`tentative` ← `mandatory_selection` | (`candidate_selection`, `!suppressed`) | (recursive `atom_requires`)
`mandatory_selection` ← `is_mandatory`, `!blocked_by_context`, `!mandatory_superseded`
`mandatory_superseded` ← `atom_conflicts`, `is_mandatory`, `!blocked_by_context` — **inert**: no
  atom in the shipped corpus declares `atom_conflicts` as a producer of this pair (documented
  inline at `jit_compiler.mg:188-193`; confirmed again here, no change)
`blocked_by_context` ← `has_constraint`+`current_context`+`!satisfied_constraint`, or
  `regime_dimension`+`has_constraint`+`!satisfied_constraint` (fail-closed regime dimensions:
  `/shard /mode /phase /layer /init_phase /northstar_phase /ouroboros_stage /provider /model`),
  or `base_prohibited`
`has_constraint`/`satisfied_constraint` ← `atom_tag`, `current_context`
`candidate_selection` ← `vector_hit` or `retrieved_context`+`atom_priority`, both gated by
  `!blocked_by_context`, `!prohibited`
`prohibited` ← `atom_tag`+`current_context` (dream/mode combos), `atom_requires`+`prohibited`
  (recursive), `atom_conflicts`+`mandatory_selection`, or the veto bridge
  `prohibited_atom`/`conflict_loser` from `jit_selection.mg`
`prohibited_atom` ← `base_prohibited` ← `compile_context`+`atom_tag`, or
  `atom_requires`+`base_prohibited` (recursive), or `blocked_by_missing_tool` ←
  `atom_requires_tool`+`!available_tool`
`conflict_loser` ← `candidate_atom`(`vector_hit`|`prompt_atom`+`atom_tag`+`compile_shard`) +
  `atom_conflicts`/`prompt_atom` priority comparisons, gated by `mandatory_atom`
  (← `prompt_atom`+`skeleton_category`+`atom_tag`+`compile_shard`, or `prompt_atom`
  `is_mandatory=/true`, or `is_mandatory(AtomID)`)

**Classification of every base (EDB) predicate this chain touches:**

**(a) Fully derivable from atom metadata + the compile request alone — 100% of what Go
actually reads today.** `compile_context`, `current_context`, `compile_shard` (from
`CompilationContext` via `ToContextFacts`/`GenerateFacts`, `context.go:516-523`); `atom`,
`atom_category`, `atom_priority`, `is_mandatory`, `prompt_atom`, `atom_tag`, `atom_requires`,
`atom_conflicts`, `atom_requires_tool`, `available_tool` (from the in-memory candidate
`PromptAtom` list — embedded/project/shard corpus, held in Go structs — via
`buildContextFacts`, `selector.go:1421+`); `vector_hit`, `retrieved_context` (from the vector
search result and the knowledge/learning-atom collectors, emitted inline in `selector.go`
around `:1199,1207`). **All of these are asserted fresh, into the scope, on every single
compile** — none of them are read out of the ~52k pre-existing live-kernel EDB. The compile
constructs its own working set from scratch every time; the clone's pre-existing facts are
never consulted by this rule chain.

**(b) Needing a genuine live session fact — none, in the path Go reads.** `jit_selection.mg`
does read one live/session-produced fact, `shard_success(ShardID)` (asserted by
`internal/core/shards/manager_spawn.go:680` after a shard finishes), feeding
`has_successful_shard` → `effective_prompt_atom` → `learning_signal(/effective_prompt_atom,
AtomID)`. But nothing in `internal/prompt` queries `learning_signal` or
`effective_prompt_atom` — the file's own status header (`jit_selection.mg:13-20`) says as
much: "`selected_atom`, `candidate_atom` and `mandatory_atom` are admissions, and nothing
queries them" (re-verified: `grep` of `selector.go` for those three names returns only their
production sites, never a `kernel.Query` read). So today, **zero** live session facts are
required by the output Go consumes.

**(c) Needing the world model — none, and the current code already keeps it that way.** The
one place a compile touches genuinely live/world-derived data is `collectKernelInjectedAtoms`
(`compiler.go:1053-1156`), and it queries `c.kernel` — the compiler's persistent live-kernel
reference — **not** `selectionKernel`, the per-compile scope (confirmed at `compiler.go:708-715`
and the function body). That data (`injectable_context`, `specialist_knowledge`, from
spreading activation and northstar mission constraints) never goes through
`NewCompilationScope` at all, so it is unaffected by anything proposed in §4.

**Bottom line**: the ~52k-fact deep copy and the subsequent full 913-stratum fixpoint pass
over it produce **no observable difference** in `selected_result` versus a scope that started
empty except for schemas + the two JIT policy files + the same per-compile atom/context
facts. The clone's pre-existing EDB is present in the store during evaluation, and the shared
913-strata program derives facts across the *entire* corpus (campaign rules, delegation,
constitution, etc.) inside that store, but none of those extra derivations are on the path to
`blocked_by_context`/`mandatory_selection`/`selected_result`, and Go never asks for them.

## 4. Design options with evidence

### Option A — selector-only scope (schemas + `jit_*.mg` + per-compile atom/context facts, no world clone)

**What changes.** `NewCompilationScope` stops calling `live.Clone()` and instead builds (or
reuses a pooled) minimal kernel from just the schema strings (needed so `AnalyzeOneUnit`
resolves every `Decl` the JIT rules reference — schemas are Decl-heavy and cheap to include
wholesale even though most Decls are irrelevant here) plus `jit_compiler.mg` and
`policy/jit_selection.mg`. Per §3, this produces byte-identical `selected_result` output,
because every predicate the wired chain reads is asserted fresh by the compiler/selector on
every call regardless of what else is in the kernel. The reconversion cost (§1.2) drops from
O(live EDB, 52k–70k observed) to O(atoms actually asserted this compile, ~900–3,700 facts per
§2's `LoadFacts JIT:` lines) — a 15–50× reduction directly proportional to the fact-count
ratios already measured in §2.1/2.2. The 913-strata fixpoint shrinks to however many strata
the trimmed program alone produces (not directly measured here — see §6).

**What could break.** (1) If `collectKernelInjectedAtoms` is ever changed to read through the
scope instead of `c.kernel` (§1.3 step 3, §3c) — not the case today, but a plausible future
"simplify by using one kernel handle" refactor — a selector-only scope would silently start
returning empty `injectable_context`/`specialist_knowledge` results, because that data lives
only in the world's EDB. (2) A future JIT rule that reads a genuine live/world/session
predicate not in today's wired chain (§3b's `shard_success` branch is the closest existing
example, and it is currently unconsumed) would silently derive nothing — this is exactly the
"starved predicate" failure mode already catalogued in M1 §5 item 8, and the
`TestStarvedPredicateBudget` gate would need the new predicate added to its baseline or
extended to distinguish "genuinely unwired" from "wired against an intentionally narrow
scope." (3) `mandatory_superseded`'s `atom_conflicts` producer is inert today (§3, confirmed
again) — a future atom author who starts emitting `atom_conflicts` would need it declared in
the trimmed schema subset too, an easy miss if the schema subset is hand-curated rather than
kept as "all schemas, none of the other 76 policy files."

**Expected cost**: implementation is bounded — a new minimal-kernel constructor plus a
regression test (§5) asserting `selected_result` parity between the old (full clone) and new
(selector-only) scopes on a representative compile. No engine change required.

### Option B — incremental (keep the clone, avoid the full fixpoint)

**Not available on this engine for this path**, for two independent reasons, both already
established in this repo's own study material:

1. `Docs/journeys/M0-mangle-language-surface.md` §7 (`:243`): the pinned `mangle-go` engine
   has **no incremental/differential evaluation** — "no retraction, no DRed, no delta between
   evaluations." `internal/mangle/differential.go`'s `DifferentialEngine` is explicitly a
   **host-side** scheme built on top of repeated full `EvalStratifiedProgramWithStats` calls,
   not an engine feature.
2. Even codeNERD's own host-side `DifferentialEngine` is structurally excluded from a
   compilation scope twice over (§1.4): it is disabled whenever any `external()` predicate is
   registered (a compile-scope clone always inherits the live `virtualStore` and its 10
   registered externals), and it demotes once the EDB exceeds `differentialFactCeiling =
   10,000` (`kernel_types.go:161`) — the live EDB observed in §2 is 3.6k–70k depending on
   session stage, so this ceiling is either already exceeded or one session away from it.

Building a viable version of this option would mean either (a) extending
`DifferentialEngine` to carry external-predicate callbacks through a delta application
(currently dropped entirely — a semantic change to what the ~10 externals return under diff
mode, a correctness surface this repo has already flagged as risky: "differential mode must
not drop semantics", M0 `agents.md` per the same citation) or (b) hand-writing a narrower
semi-naive evaluator scoped to just `jit_compiler.mg`/`jit_selection.mg`. Option (b) is Option
A with an additional, unneeded evaluation-engine project layered on top, since Option A
already removes the world-clone cost by construction without any engine work. **Not
recommended** — the cost (new engine code, new correctness surface, §M0's own warning) buys
nothing that Option A doesn't already buy for free.

### Option C — cache the scope across compiles in a turn, invalidate on assert

**Partially built already.** `JITPromptCompiler.cache`/`cacheList`/`compileGroup`
(`compiler.go:366-401`) cache the **final compiled prompt**, keyed on `cc.Hash()`
(`compiler.go:622-634`), and a cache hit returns before `acquireCompilationKernel` is ever
called (`compiler.go:625-633`, ahead of the singleflight block that contains the kernel
acquisition) — so a cache hit pays **zero** kernel cost, no clone, no fixpoint. This is
almost certainly why session 054828 (§2.1) shows only one clone tagged as a JIT compile
despite the session's overall activity: most turns' `CompilationContext` hashes repeat and
never reach `NewCompilationScope` at all. What Option C would add **beyond** the existing
result cache is caching the *scope itself* (a warm minimal kernel with schemas+JIT-policy
already parsed and stratified) across *distinct* `cc.Hash()`es within one turn — e.g. several
shards each compiling once with different `ShardType`/`IntentVerb` — so each miss reuses a
warm program/strata instead of re-running `rebuildProgram`+`Stratify` (which Option A's
minimal kernel would otherwise need to do once, on first use, and could then keep resident).

**Expected cost / value**: valuable primarily as a companion to Option A (pool one minimal
JIT kernel and reuse it across compiles in a turn, invalidating/clearing only the small
atom+context EDB between compiles) rather than as a fix to the current full-clone cost, since
Option A already eliminates the expensive part (the world-sized reconversion) that this
option would otherwise be working around. Applied to today's full-clone mechanism instead, it
would still pay the `Clone()` deep-copy of the live EDB once per turn instead of once per
compile — a partial win, but strictly worse than Option A once Option A is available.

## 5. Recommendation

**Adopt Option A** (selector-only scope), optionally paired with a pooled/reused minimal
kernel across compiles in a turn (the useful half of Option C). §3 shows this changes nothing
about what a compile can select today — every predicate the wired rule chain reads is
already asserted fresh per compile from atom metadata and the compile request; nothing reads
the live kernel's pre-existing EDB through this path. §2 shows the "expensive" full clone
that this replaces was **already producing near-worst-case cost within measurement noise of
what the trimmed alternative would cost for its own smaller input** (the difference is the
reconversion volume: ~52k–70k facts today vs. ~900–3,700 per compile under Option A) — i.e.
the fix is cheap to justify and cheap to build, precisely because §1–§3 show the extra facts
were never load-bearing. Option B is not viable without a separate, larger, and independently
risky engine project, and buys nothing Option A doesn't already deliver. Reject it explicitly
rather than leaving it open, so a future reader doesn't re-propose it without re-reading M0
§7.

Before landing Option A, write the invariant test contract C-30 already asks for but never
got ("a compile must not re-derive strata whose predicates the selector never queries"): run
the same `CompilationContext` through today's full-clone scope and a prototype selector-only
scope and assert `selected_result` is byte-identical. That test is the cheapest possible
guard against the one real risk in §4A (a future rule quietly starts depending on world/live
data through this path) — it will fail loudly instead of silently starving.

**What a live run should record**, to make the next measurement a query instead of a
half-hour of manual log correlation (§2.3): tag every `Kernel cloned` / `evaluate()` call
with its caller (`jit_compile`, `dreamer_projection`, `shadow_mode`, or `live`); attach the
originating compile's cache key (or a turn/compile correlation ID) to the JIT-tagged ones;
and log the reconversion step's own duration separately from the fixpoint's (today only their
sum, `evaluate completed in Xms`, and the fixpoint's own `evalTime`/`wallTime` are logged —
the gap between "cache empty, populating cache" and "running fixpoint evaluation" is the
reconversion time and is only recoverable by timestamp subtraction, as done by hand in §2.1).

## 6. Open

- **The two sessions this study could not verify.** Pid `030000` (brief: 2026-09-17 21:24)
  does not exist under either clock convention in the current `.nerd/logs/`; no session
  starts in the 05:06–05:22 window (brief: a `nerd fix` run) either. §2's numbers instead come
  from pid `054828` (matches the brief's second window exactly) and pid `057064` (nearest
  available session to the first window, used as a second, cleaner data point since it had no
  Dreamer interleaving). If the two missing sessions exist in an archived/rotated log
  location, re-running §2.1's classification procedure against them would strengthen or
  correct the "≈24%/≈10–20% of compile latency is kernel-side" figures with a `nerd fix`
  specific sample, which this study does not have.
- **Stratum count under the trimmed program was not measured.** §4 Option A asserts the
  fixpoint "shrinks to however many strata the trimmed program alone produces" but this
  requires actually running `analysis.Stratify` over {schemas + `jit_compiler.mg` +
  `policy/jit_selection.mg`} and counting — not done here (read-only, no builds permitted for
  this study). This is the concrete next step before implementing Option A, not a blocking
  unknown for the recommendation itself (§3's predicate-tracing argument does not depend on
  the exact stratum count).
- **Whether any caller beyond the three in §1.5 invokes `RealKernel.Clone()`.** The grep was
  tree-wide, excluding `.git`, `.nerd`, `node_modules`, `Docs`, `.gocache*`, and
  `.claude/worktrees`, and excluding `_test.go` files — a fourth production caller introduced
  later would silently join the "Kernel cloned" line's ambiguity problem described in §1.5/§2.3
  unless the caller-tag recommendation in §5 is adopted.
- **The `shard_success` → `learning_signal` branch in `jit_selection.mg`** (§3b) is wired at
  the Mangle level, reads a genuine live fact, and is never consumed by Go. Worth a decision
  (finish wiring `learning_signal` into atom-priority feedback, or delete the dead branch) —
  out of scope for this measurement study, flagged here so it isn't mistaken for something
  Option A needs to account for (it doesn't, since nothing reads it).
