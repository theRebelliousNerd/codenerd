# M2 — Mangle programming model for codeNERD

## Status

- last updated: 2026-09-18 10:02 (local) — pass 2 (revision against M3's refutation list) complete; M0 corrections appended
- done: 1 principles (pass 2: one Decl per predicate; `NegAtom`-count lint; no `_` in `do` bodies; `mgv.exe` both-stores/5-runs rule); 2 Pattern A (holds per M3; pass 2: empty-external re-call + `ShouldQuery`-once/sentinel fixes verified, variable revision join, E1 order caveat); 3 Pattern B (pass 2: tier prefix sum, linear in atoms, `hit_boost`, `fn:collect_distinct`; 11 strata); 4 Pattern C (pass 2: per task, `test_run/4` shared with G, failure guard on `test_green_now`, `/write` → `/rerun` chain; 9 strata); 5 Pattern D (pass 2: task-joined obligations, windowed `failure_count`, `current_rev` invariant; 13 strata); 6 Pattern E (pass 2: `plan_site` carries its change, `blast_radius` over distinct refs, `edit_applied` per op; 9 strata); 7 Pattern F (pass 2: depth column, `why_direct`, analysed-clause hash, R2 partial-from-builtin; 5 strata); 8 Pattern G (pass 2: build-failure guard, `/reran` stage, shard-typed claims, T1 widened; 10 strata); 9 migration map (row 16 verified; seams extended); 10 token economy (strata add linearly: 61 composed); 11 open risks (measured quadratic + tier fix; retained-store negation trap); composition of the seven pass-2 programs as one unit (`compose_v2.mg`, 116 Decls, `strata=61`, D reads C and G across the seam)
- open: (a) composed cost of Patterns A–G loaded *beside the shipped policy* through `rebuildProgram`, unmeasured (the raw-engine composition is done); (b) whether E1 is fixed upstream or carried as an engine bump, and whether the analyzer gate gains the external-order check with it (architect decision); (c) `nerd why` needs a `rule_origin(RuleID, File, Line)` producer in the loader because the recorded rule text is the rewritten clause (§7 rule 5) — design only; (d) codeNERD's differential engine against the stale-negation program `m0_AN1/2.mg` (M3 open item (d)) — **unverified**; (e) the `Fact.ToAtom` string→name heuristic fix (Decl-directed coercion or `ast.String` at path-carrying assert sites) is named in §9 but is a corpus change outside this study; (f) M0's `"etc/passwd"`/`"passwd"` 1-row halves remain unexplained (M3 open item (c))
- evidence rule: every claim cites `path:line`, `path#symbol`, or a predicate in a named `.mg`; anything not checked says **unverified**.
- runnable examples: each pattern carries a `.mg` block executed against the pinned engine (`codeberg.org/TauCeti/mangle-go@v0.5.1-0.20260413190942-4dcaa582c6d3`, see M0) with the verifier's harness `C:/Temp/mangle-study/verify/mgv.exe` (M3 §1; PowerShell: `& C:\Temp\mangle-study\verify\mgv.exe [-store simple|multi] [-det=false -runs N] [-prov] [-ext] <file.mg> <pred> ...`). Pass-2 programs and outputs: `C:/Temp/mangle-study/m2v2/` — `patternB_v2.mg`, `patternC_v2.mg`, `patternD_v3.mg` (+`_spread`, `_2rev`), `patternE_v3.mg` (+`_blocked`), `patternF_v2.mg`, `patternF_v2_B.mg` (why-program over the fresh `inject` proof, `provB_v2.txt`), `patternG_v2.mg`, `compose_v2.mg` (+`_onerev`), `perfB_tier_{200,400,800,3200}.mg`, `prov_builtin.mg`, `prov_fn.mg`, `ext_sentinel.mg` (run with the forked harness `m2v2/harness/mgv2.exe`, which adds `sentinel_all/2` and `once_all/2`). Pattern A's program is `verify/patternA.mg`, unchanged. The first draft's files under `C:/Temp/mangle-study/m2/` are what M3 verified and refuted.

## Revision log

- 2026-09-18 08:28 — second pass started (programming-model designer, pass 2). Reading M3 sections 2-5 (refutation list) and revising each refuted / holds-with-changes pattern in place. Entries below are appended as each revision lands; each names the M3 item, what changed in M2, and why.
- 2026-09-18 08:42 — **§4 Pattern C rewritten** (M3 items 1, 7, 8, 9, 17). `changed_symbol` gains a `Task` column and `behavior_touched`/`test_obligation`/`obligation_open` are per task (item 1: the old shape cross-joined every active task with every behavior obligation in Pattern D). `test_run` is now the one shape `test_run(Task, TestRef, Rev, Outcome)` shared with Pattern G (item 17). `test_green_now` is guarded by `has_test_failure_now` so a `/fail` at the current revision defeats a `/pass` at the same revision (item 7). Rule 3 now says the `Rev` column is provenance and is not joined — obligations persist until a test exists (item 8, decision: persist). Rule 5 says adding a test converts `/write` into `/rerun` for the new test (item 9). Program `C:/Temp/mangle-study/m2v2/patternC_v2.mg`, run on `mgv.exe` (both stores, 5 nondeterministic runs identical), `strata=9`.
- 2026-09-18 08:47 — **§5 Pattern D rewritten** (M3 items 1, 2, 16). `obligation` joins `test_obligation(T, B, Target, Kind)` on the task (item 1; the cross product is gone — a second active task with nothing open is now `/complete`, `patternD_v3.mg`). `failure_count` counts `recent_failure`, the failures inside the last `repeated_failure_rounds(K)` rounds (item 2; the 1/5/9 spread case is `/continue`, `patternD_v3_spread.mg`). `/hollow` reads Pattern G's `unverified_claim` instead of a `hollow_success(T, Reason)` witness. New rule 6: `current_rev_count` + `kernel_invariant_violated(/current_rev, N)` (item 16; `patternD_v3_2rev.mg` prints the violation). `strata=13`.
- 2026-09-18 08:55 — **§6 Pattern E rewritten** (M3 items 3, 4). `plan_site` is now `plan_site(Task, Change, Op, Ref, Tier)` and `edit_plan` joins a site to its own `chosen_change(T, Change, Op, Spec)` (item 3; two changes on one task give 7 rows, not 12, `patternE_v3.mg`). `blast_radius` counts `site_ref(T, Ref)`, a projection, so it is distinct refs (item 4; 6 for 8 site rows). `edit_applied` and `edit_done_now` carry the `Op`, because applying a rename at the definition does not apply the signature change there (found while rewriting; step 1 stays pending for the second change). Every aggregation body names its columns (item 5's corpus rule). `strata=9`.
- 2026-09-18 09:02 — **§8 Pattern G rewritten** (M3 items 7, 10, 17, 22). `has_build_failure` guards `/build_green` the way `has_test_failure` guards `/tests_green` (item 7; t7's fail-then-pass at one revision is `/edited`). New stage `/reran` for a coder task with no write and green tests, and `verdict(T, /verified, /reran)` (item 10; t6 is no longer `/nothing_done`, which contradicted Pattern C's `/rerun`). Claim rules carry `shard_run(T, /coder|/reviewer, Rev)`; a reviewer's `/done` is unverified only with no findings and no review tool (item 10; t4 is no longer hollow). Rule 1 names `test_run/4` as the one witness shared with Pattern C (item 17). T1 paragraph widened per item 22. Every aggregation body and positive premise names its columns. `strata=10`.
- 2026-09-18 09:12 — **§3 Pattern B rewritten** (M3 items 6, 20). The budget prefix sum is over priority tiers (`tie_sum` → `above_tier` → `above_sum(A, P, S)`), quadratic in distinct priorities and linear in atoms, replacing the atom-pair `above_pair` (item 20). Measured with the verifier's perf inputs: same `fits`/`dropped` sets at 200 candidates (0 differences), 0.14–0.34 s at 800 and 0.33–0.87 s at 3 200 candidates, where the pair form overflowed the 100 000 created-fact limit at 800 (`perfB_tier_*.mg`). `hit_boost` (`fn:max` over cleared tiers) makes a knowledge hit one candidate row (the per-tier design note). `inject_reasons` uses `fn:collect_distinct` and the last paragraph no longer says `fn:collect` gives a clean list (item 6). `inject`/`dropped` output unchanged; `strata=11`.
- 2026-09-18 09:28 — **§7 Pattern F rewritten** (M3 items 13, 14). The why-program carries a depth column (`supports/3`, `why/4`, `why_direct/3`) so the rule's own premises are separable from transitive support (item 13; over the fresh Pattern B v2 `inject` proof, depth 1 is exactly `prompt_atom` + `window_slot`, `patternF_v2_B.mg`). The leaf list for the `inject` proof is replaced by what the recorder actually emits, before and after R1. Rule 5 says `/rule/<hash>` hashes the analysed, rewritten clause (including `__tmp` rules) and `nerd why --diff` is over rule IDs; a `rule_origin(RuleID, File, Line)` fact is proposed for the file position (item 14). New **R2**: a comparison builtin marks a node `[partial]` without losing leaves (`recorder.go:233-241`, `prov_builtin.mg`) — every threshold rule's proof is `[partial]`, so the flag must not be rendered as "incomplete". Also: `EmitFacts` rows need a terminating `.` before they parse as a program. `strata=5`.
- 2026-09-18 09:40 — **§2 Pattern A revised** (M3 items 11, 12; engine surprises 1–3). The program holds (M3 re-ran it on both stores) and is unchanged. Cost note rewritten: an all-output external that returns no rows is called on every premise evaluation (`v_ext_empty.mg`, 6 calls); two host-side fixes verified on a forked harness (`m2v2/harness/mgv2.exe`, `ext_sentinel.mg`): a per-evaluation `ShouldQuery` that declines after the first call (1 call) — the design's choice — and a sentinel row (1 call). Rule 2 now says the revision join must be a variable, because a constant in an external's output position is a cache key and a filter (surprise 3). E1 paragraph adds the order caveat: analysis accepts a `+` external written before its binder, which panics at runtime (surprise 2), and notes the M0 correction (item 12).
- 2026-09-18 09:52 — **§1, §9, §10, §11 revised** (M3 items 5, 15, 16, 17, 18, 19, 20, 21, 22, 23). §1 convention 1: exactly one Decl per predicate, in its owning file (item 16; the seven pass-2 programs compose with one Decl each, `compose_v2.mg`, `strata=61`). Convention 5: the dropped-negation lint compares `NegAtom` counts, unbound named variables are deleted too (item 21). Convention 6: no `_` in any `do`-transform body, `fn:collect_distinct` (items 5, 6). §1 testing paragraph now names `mgv.exe` and the both-stores/5-runs rule. §9 row 16 marked verified (item 18); seams gain the `current_rev` invariant (16), the `Fact.ToAtom` string→name heuristic with the assert-site fix (19), one witness shape per name (17), the widened T1 and `NegAtom`-count lints (21, 22), `ShouldQuery`-once and the external order check. §10: strata add linearly — 52 for the first draft, 61 for pass 2 (item 15). §11 risk 2 carries the measured quadratic curve and the tier fix (20); risk 4 adds R2, the empty-external re-call, the order caveat and the retained-store stale negation (23).
- 2026-09-18 10:02 — **M0 corrections appended** (`M0-mangle-language-surface.md` "## Corrections from verification", 14 entries: items 5, 12, 19, 21, 22, 23 and the engine surprises that hit M0 claims; M0 open item (a) closed to its residue). Pattern A's run line repointed to `mgv.exe`. No refutation in M3 §5 is disputed: every item was either adopted with a program that shows the corrected behaviour or, for items 8 and 10, resolved by a stated design decision (obligations persist regardless of revision; `/reran` stage and shard-typed claims). Pass 2 complete.

## 1. Design principles

**What is asserted (EDB) vs what is derived (IDB).** Go asserts only *witnesses*: things it measured or was told — a tool ran, a file's hash is H at revision R, the user typed T, the vector index returned (Atom, Score) for query Q at revision R, the test runner exited non-zero with this output. Go never asserts a *judgement*: not "done", not "relevant", not "needs a test", not "delegate to coder", not "mandatory". Every judgement is the head of a rule. The corpus already states this contract for one predicate family — `turn_evidence/6` is asserted, `hollow_success/1`, `turn_executed/1`, `turn_done/1` are derived (`internal/core/defaults/policy/coder_safety.mg:68-115`) and the model is hard-denied from asserting the conclusions (`internal/core/mangle_updates.go:145-150`, M1 §6). The design generalises that one contract to every executive decision in M1 §7.

**Where Go stops and Mangle starts.** Go is the FFI and the effect side (root `CLAUDE.md` "Clean fixpoint, not clean loop"): it parses, embeds, runs tools, calls models, reads SQLite/vector/CodeDOM stores, and asserts what came back. Mangle owns *selection* (which atom, which file, which shard, which test), *ordering* (where a fact sits in the window), *gating* (`permitted`, `turn_done`, `task_incomplete`), *budgeting* (counts and sums compared with facts, never Go constants) and *explanation* (provenance). The one sanctioned Go opinion is fuzzy signal extraction from natural language (`delegation.mg:340-360` + `delegation_routing.go:104`, M1 §7 row 15, root `CLAUDE.md` Mangle guardrails) — Go emits a `*_signal` fact; the combination is a rule. Concretely, Go stops at the four seams the engine gives a host (M0 §9): `extraPredicates`/`Assert` (facts in), `external()` callbacks (facts pulled on demand), `Query`/`GetFacts` (facts out), and `WithDerivationRecorder` (why). A Go function that reads two kernel facts and decides something is the drift to hunt; it becomes a rule and Go reads one row.

**Naming and Decl conventions** (every predicate in this document follows them):

1. Every predicate has **exactly one** `Decl`, in the file that owns it, with `bound [...]` on every column, before use — the bounds are a codeNERD convention, not an engine rule (synthetic decls, M0 §3), kept because they are the only contract a reader can check (`internal/mangle/agents.md:9-42`); the *one* is an engine rule: a second `Decl` of the same predicate anywhere in the loaded unit is a hard analysis error (`analysis/validation.go:173-175`; M3 composition: the first draft's seven files declared six predicates twice and refused to load as one unit). A predicate shared by two patterns (`code_element/5`, `current_rev/1`, `test_run/4`, `test_obligation/4`, `unverified_claim/2`, …) is declared once in the schema file and *used* by both; the pass-2 examples are composed that way in `C:/Temp/mangle-study/m2v2/compose_v2.mg` (116 Decls, loads, `strata=61`). Same name with a different arity is a different predicate and loads silently — the worse failure (M3 engine surprise 8).
2. Identity columns are `/string` refs in the exact shape their producer emits (`<pkg>.<Name>` for CodeDOM, `internal/mangle/agents.md:24-31`); enumerations are `/name` atoms; counts, percentages and scores are `/number` int64 in 0–100 (a float aborts the fixpoint, M0 §4, `internal/core/kernel_fact_decl.go:27-45`). Never mix a number column with a list/struct column in one position (hash collisions on `SimpleInMemoryStore`, M0 §2).
3. Witness predicates are named by what happened, past tense or noun (`tool_ran`, `test_run`, `workspace_snapshot`); judgement predicates by the decision (`inject`, `test_obligation`, `task_incomplete`, `edit_plan`); thresholds and policy constants are `*_threshold/2` or `*_rounds/1` facts (`working_set.mg:53-57` is the existing precedent).
4. Every mounted external is revision-keyed: an input or output column carries the revision the answer was computed at, because the engine caches external answers for the whole evaluation (M0 §3, `engine/topdown.go:84-93`) and stale evidence must not survive a source change (root `CLAUDE.md` North Star).
5. Negation only over a projection whose every argument is bound; never a wildcard *or an otherwise-unbound named variable* in a negated literal (M0 §4 probes A/B/D; M3 §3 row 4: `q(X) :- p(X), !r(X,Y).` is silently rewritten to `q(X) :- p(X).` exactly like the wildcard form; `working_set.mg:88-91`). Zero-arity projections (`has_hollow_success()`) are the idiom. The lint that catches the silent deletion compares the `NegAtom` count of each clause before and after `AnalyzeOneUnit` — it cannot look for `_` alone (M3 item 21; the verifier's `mgv.exe` warns on the same comparison the first draft's harness refused on, and both are host-side because the engine never reports the drop).
6. Aggregation is its own stratum: one `|> do fn:group_by` per rule, the zero case as a negation rule (M0 §6), reducers `fn:count/fn:sum/fn:max/fn:min/fn:collect/fn:collect_distinct` only — `fn:count_distinct`, `fn:map:get`, `fn:pick_any` do not work (M0 §5). **No `_` in the body of a `do`-transform rule, ever**: a single-atom body is not rewritten and the reducer sees stored rows *including* the wildcard column, while a multi-atom body is projected onto the named variables first — so `c1(N) :- p(X, _) |> do fn:group_by(), let N = fn:count().` counts facts and `c2(N) :- p(X, _), q(X) |> …` counts distinct `X` (M3 engine surprise 4, `v_agg_wildcard.mg`: `c1(3)`, `c2(2)`; `rewrite/rewrite.go:40,52`). Name every column, and when a count over distinct keys is wanted, project first (`site_ref` in Pattern E). `fn:collect` keeps duplicates and its order is map-iteration order; `fn:collect_distinct` is the set (M3 engine surprise 5).
7. Selective, bound atoms first in every body — the engine is nested loops in written order (M0 §7); an external or aggregate is never the first premise.
8. A rule that must not fire twice on the same key uses a merge predicate (`fundep`+`merge`, M0 §3 item 4) or a `_superseded` projection (`working_set.mg:103-123`), never Go retract-before-assert discipline (M1 §5 item 11).

**How a pattern is tested.** Each pattern below ships a `.mg` that the verifier's harness `C:/Temp/mangle-study/verify/mgv.exe` (M3 §1; source `main.go`, built with `GOPROXY=off GOFLAGS=-mod=mod go build -C /c/Temp/mangle-study/verify -o mgv.exe .` from bash, **run from PowerShell** — the exe returns no output under the Git-Bash tool until its timeout) parses, analyses, warns if the rewrite dropped a negation (M0 §12 item 1; the first draft's `m2/mgrun.exe` refused instead), stratifies, evaluates and prints the named predicates; `-store simple|multi` picks the store (codeNERD runs `SimpleInMemoryStore` below 1024 facts, `kernel_eval.go:842-846`), `-det=false -runs N` drops `WithDeterministicOrder` and reports any run whose output differs, `-prov` prints the proof and its `EmitFacts` encoding, `-ext` registers the stub externals. A pattern *holds* only when the derived set is identical on both stores and across 5 nondeterministic runs (M3's rule, adopted here). The pass-2 programs live in `C:/Temp/mangle-study/m2v2/` (`patternB_v2.mg`, `patternC_v2.mg`, `patternD_v3.mg`, `patternE_v3.mg`, `patternF_v2.mg`, `patternG_v2.mg`; Pattern A's `verify/patternA.mg` is unchanged) with the composed unit `compose_v2.mg`; the first draft's `C:/Temp/mangle-study/m2/` files are what M3 §2 verified and refuted. Inside codeNERD the same test is a kernel test in the shape of `TestImpactChain_EndToEndThroughVirtualStore` (`internal/mangle/agents.md:36-38`): assert the witness facts through the real `Assert` path, query the *derived* predicate, compare rows — never test producer and consumer separately, because a shape mismatch passes both (`agents.md:11-13`).

## 2. Pattern A — mounting knowledge as virtual predicates

**Intent.** The deductive database is the delivery mechanism: SQLite domain knowledge, vector search, CodeDOM, the world model and session history are *relations the kernel joins*, not Go lookups whose results a Go function ranks. Today the kernel pulls only from SQLite and the vector index, through 10 `external()` Decls (`schemas_memory.mg:54-84`, `schemas_tools.mg:320-325`; M1 §4.1); **no live rule body calls any of them** (grep of `internal/core/defaults` and `internal/context` for `recall_similar(|query_session(|query_knowledge_graph(|query_graph(|query_traces(` in rule bodies: 0 hits, 2026-09-18); CodeDOM, world and history reach the kernel only as bulk `AssertBatch` from Go (M1 §4.1, last paragraph). Relevance thresholds are literals inside rules (`activation.mg:25-27`, `Score > 30`) or Go constants (`internal/context/compressor.go:568,595`, `AtomReserve`).

**Engine features relied on.** `external()` + `mode` Decls and `WithExternalPredicates` (M0 §3 "external()", §9 item 7); the once-per-evaluation answer cache (`engine/topdown.go:84-93`, M0 §3); typed `bound` columns (M0 §3 — documentation-only unless `AnalyzeAndCheckBounds` is turned on, M0 §12 item 8); int64 comparison builtins (M0 §5); `!=` on bound constants (M0 §4).

**Existing predicates it builds on (M1).** The external Decl shape at `schemas_memory.mg:54-84`; the adapter `virtualExternalPredicate` (`internal/core/external_predicates.go:29-85`), which already emits output positions only; `file_topology/5` (`schemas_world.mg:17`), whose `Hash` column is the natural revision key for file-scoped knowledge; `code_element/5`, `code_calls/2`, `code_defines/5` (`schemas_codedom.mg:33`, `schemas_analysis.mg:112-116`); `activation/2` and `context_atom/1` (`activation.mg:20-27`); `should_include_context/2` (`schemas_context.mg:26`).

**Engine defect E1, found while building this pattern (2026-09-18).** An `external()` predicate whose `+` (input) position holds a *variable bound earlier in the rule* **panics the pinned engine**: `oneStepEvalPremise` hands the premise atom to `EvalExternalQuery` **without applying the current substitution** (`engine/seminaivebottomup.go:831`, versus `premiseAtom(p, lookupFn, subst)` at `:851` for the store lookup), and `EvalExternalQuery` does `arg.(ast.Constant)` on every input position (`engine/topdown.go:99`) → `panic: interface conversion: ast.BaseTerm is ast.Variable, not ast.Constant` (probe `C:/Temp/mangle-study/m2/extA_min.mg`: `out(S,D) :- focus(S), sqlite_symbol_doc(S, "rev-17", D).` under `mode("+","+","-")`). The same rule with constants in both input positions works (`extA_const.mg`). M0 §3's sentence "called … once per binding of the input positions" is therefore wrong for variables — it is called with the unsubstituted atom (M3 item 12; corrected in M0's "Corrections from verification"). The verifier added two facts (M3 §2A, engine surprise 2): E1 is not specific to strings or mixed input lists (`v_ext_plus_var.mg`, `q(X,Y) :- p(X), dbl_ext(X,Y).` panics the same way), and **analysis does not check the written order of moded externals** — `q(X,Y) :- dbl_ext(X,Y), p(X).` with the external first passes `AnalyzeOneUnit` (positive atoms are order-independent for safety, M0 §4) and panics at runtime (`v_ext_plus_first.mg`). So even after the one-line upstream fix, a `+` external written before its binder will pass the analyzer and see an unbound variable at runtime; the analyzer gate (§9 cross-cutting seams) needs an order check for moded externals. Constant inputs are cached as claimed: `q(Y) :- dbl_ext(4, Y).` and a second rule with three premise evaluations of the same constant make **one** call (`v_ext_plus_const.mg`). This is why codeNERD's `+`-moded externals are unusable from rules and why the corpus never calls them. The fix is one line upstream (`p = p.ApplySubst(subst).(ast.Atom)` before `:831`; `ast.Atom.ApplySubst` exists at `ast/ast.go:1023`); whether to carry it as a pinned-engine bump is an architect question. **Until then Pattern A uses all-output externals** (`mode("-",…)`): the engine calls them once per evaluation with no inputs (`inputs=[]` in the harness output), Go enumerates rows keyed on EDB facts it reads itself (the VirtualStore already holds the kernel, `internal/core/kernel_virtual.go:8`), and the join happens in Mangle. This is how `query_learned`, `query_strategic`, `query_activations`, `has_learned` are declared today (`schemas_memory.mg:54,76,80,84`).

**The pattern.** Five rules, no Go decisions:

1. *Mount* — `Decl <source>_all(K1…, Rev, V1…) descr [external(), mode("-",…)] bound […]`. Every row carries the revision (`file_topology.Hash` for file-scoped knowledge, an index generation string for the vector index, a schema/version stamp for SQLite tables) at which Go computed it.
2. *The current revision is a fact, joined through a variable* — `knowledge_rev(Source, Rev)`, asserted by Go when a source changes (world scan, reembed, SQLite migration). The join `knowledge_rev(/vectors, Rev), recall_similar_all(Q, Rev, Atom, S)` is what keeps stale rows out; there is no Go cache invalidation. The revision must be a *variable* bound by the join, never a constant written into the external's argument: a constant in an output position of an all-output external becomes part of the cache key and a filter — rows that do not match it are discarded, not stored (`engine/topdown.go:119-133`), so two rules with two constants each call the external and neither can reuse the other's table (M3 engine surprise 3, `v_ext_filter.mg`: `old(A) :- recall_similar_all(_, "rev-16", A, _)` and `cur(A) :- … "rev-17" …` made two calls with filters `[_ "rev-17" A _]` and `[_ "rev-16" A _]`).
3. *Thresholds are facts* — `relevance_threshold(Kind, Pct)`; a rule compares with `>=`. Changing policy is asserting a fact, and `nerd why` can show which threshold admitted an atom (Pattern F). `activation.mg:27` (`Score > 30`) and `AtomReserve` (`compressor.go:568`) become rows.
4. *Exclusions are derived, not filtered* — a row that fails a gate becomes a fact naming the gate (`knowledge_stale`, `knowledge_hit_offlang`), so "why was this not injected" has a derivation. The engine has no negative provenance (M0 §8), so the design manufactures positive facts for the negatives that matter.
5. *Consumers join on typed columns* — `task_query/2`, `task_focus_symbol/2`, `task_language/2` are the task frame Go asserts; `knowledge_hit/3` and `symbol_doc/3` are what Pattern B consumes.

**Runnable example** (`C:/Temp/mangle-study/verify/patternA.mg`, byte-identical to the first draft's `m2/patternA.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -ext -store simple -det=false -runs 5 patternA.mg knowledge_hit knowledge_hit_offlang symbol_doc knowledge_stale`):

```mangle
# Pattern A -- mounting knowledge as virtual predicates (runs on the pinned engine).
Decl recall_similar_all(Query, Rev, Atom, ScorePct) descr [external(), mode("-", "-", "-", "-")] bound [/string, /string, /string, /number].
Decl sqlite_symbol_doc_all(Sym, Rev, Doc) descr [external(), mode("-", "-", "-")] bound [/string, /string, /string].
Decl knowledge_rev(Source, Rev) bound [/name, /string].
Decl relevance_threshold(Kind, Pct) bound [/name, /number].
Decl task_query(TaskID, Text) bound [/string, /string].
Decl task_focus_symbol(TaskID, Sym) bound [/string, /string].
Decl task_language(TaskID, Lang) bound [/string, /name].
Decl atom_language(Atom, Lang) bound [/string, /name].
Decl knowledge_hit(TaskID, Atom, ScorePct) bound [/string, /string, /number].
Decl knowledge_hit_offlang(TaskID, Atom) bound [/string, /string].
Decl knowledge_stale(Atom, Rev) bound [/string, /string].
Decl symbol_doc(TaskID, Sym, Doc) bound [/string, /string, /string].

# Facts Go asserts: current revisions, thresholds, task frame.
knowledge_rev(/vectors, "rev-17").
knowledge_rev(/sqlite, "rev-17").
relevance_threshold(/vector, 30).
task_query("t1", "wrap errors with context").
task_focus_symbol("t1", "core.evaluate").
task_language("t1", /go).
atom_language("knowledge/go/error_wrapping", /go).
atom_language("knowledge/go/table_tests", /go).
atom_language("knowledge/python/asyncio", /python).
atom_language("knowledge/go/stale_atom", /go).

# The mount. Bound EDB atoms first; the revision join drops rows computed at
# an older revision; the threshold is a fact.
knowledge_hit(T, Atom, S) :-
    task_query(T, Q),
    knowledge_rev(/vectors, Rev),
    recall_similar_all(Q, Rev, Atom, S),
    relevance_threshold(/vector, Min),
    S >= Min.

# Rows at a revision that is no longer current: derived, so "why not" has an answer.
knowledge_stale(Atom, Rev) :-
    recall_similar_all(_, Rev, Atom, _),
    knowledge_rev(/vectors, Current),
    Rev != Current.

# A hit in the wrong language is a derived fact too, not a Go filter.
knowledge_hit_offlang(T, Atom) :-
    knowledge_hit(T, Atom, _),
    task_language(T, Lang),
    atom_language(Atom, Other),
    Lang != Other.

symbol_doc(T, Sym, Doc) :-
    task_focus_symbol(T, Sym),
    knowledge_rev(/sqlite, Rev),
    sqlite_symbol_doc_all(Sym, Rev, Doc).
```

Stub externals (`main.go`): `recall_similar_all` returns `("wrap errors with context","rev-17","knowledge/go/error_wrapping",91)`, `(…,"rev-17","knowledge/go/table_tests",74)`, `(…,"rev-17","knowledge/python/asyncio",31)`, `(…,"rev-16","knowledge/go/stale_atom",99)`; `sqlite_symbol_doc_all` returns `("core.evaluate","rev-17","evaluate runs the stratified fixpoint")` and `("core.assert","rev-17","assert adds an EDB fact")`.

**Expected derived facts** (verified 2026-09-18; re-verified by M3 §2A on both stores and 5 nondeterministic runs; `strata=4`; each external called exactly once with `inputs=[]` — *because each returns at least one row*, see the cost note):

```
knowledge_hit("t1","knowledge/go/error_wrapping",91)
knowledge_hit("t1","knowledge/go/table_tests",74)
knowledge_hit("t1","knowledge/python/asyncio",31)
knowledge_hit_offlang("t1","knowledge/python/asyncio")
knowledge_stale("knowledge/go/stale_atom","rev-16")
symbol_doc("t1","core.evaluate","evaluate runs the stratified fixpoint")
```

With `relevance_threshold(/vector, 60)` instead, `knowledge_hit` has 2 rows and `knowledge_hit_offlang` none (also run). The `rev-16` row scored 99 and was still excluded: revision beats score, which is "stale evidence must not survive a source change" expressed as a join.

**Mounting the other sources with the same shape** (Decls proposed; none exist yet; producer cost **unverified**):

| Source | Mount Decl | Revision column | Go enumerates from | Replaces |
|---|---|---|---|---|
| CodeDOM | `codedom_callers_all(Ref, Rev, Caller)` | `file_topology.Hash` of the callee's file | the `code_calls` index the Cartographer keeps (`internal/mangle/agents.md:27-29`) | bulk `AssertBatch` of the `code_*` rows every scan (M1 §9: EDB 50–70k at evaluation) |
| World model | `world_file_all(Path, Hash, Lang, IsTest)` | `Hash` | the `file_topology` producer | nothing yet; the point is that consumers join on `Hash` |
| History | `history_built_all(Entity, Rev, Verb, Turn)` | session turn number | SQLite `session_history` (`QuerySession`, `virtual_store_predicates.go:294`) | Go-side "have we already done this" checks |
| SQLite knowledge | `sqlite_symbol_doc_all(Sym, Rev, Doc)`, `learned_fact_all(Pred, Rev, Args)` | schema version | `QueryLearned` (`virtual_store_predicates.go:27`) | `HydrateLearnings` bulk assert (`schemas_memory.mg:88-92`) |
| Vector index | `recall_similar_all(Query, Rev, Atom, Pct)` | index generation | sqlite-vec (`RecallSimilar`, `virtual_store_predicates.go:268`), with `Query` taken from the kernel's own `task_query` rows | `recall_similar/3` (`schemas_memory.mg:62`), whose `+` mode cannot be called from a rule (E1) |

**Cost note (revised in pass 2 — M3 item 11, engine surprise 1).** An all-output external returns its whole table once per evaluation **only when it returns at least one row**. The "once" is the store cache in `EvalExternalQuery` (`engine/topdown.go:84-93`: `Store.GetFacts(query, …)` short-circuits when any stored fact matches the premise pattern); with no row stored there is nothing to hit, and the callback runs again on every premise evaluation — one Go call per solution row of everything to its left, per delta round. The verifier measured it: `empty_all(K, V)` after `p(X)` with three `p` rows, two rules, **6 calls** (`v_ext_empty.mg`). An empty vector search, a symbol with no doc, an empty history table are the common case, not the edge case, so the mount has to engage the cache itself. Two host-side mitigations, both verified 2026-09-18 on a fork of the verifier's harness (`C:/Temp/mangle-study/m2v2/harness/mgv2.exe`, `ext_sentinel.mg`, `p(1). p(2). p(3).` and one rule each):

- *Decline the second query.* `ExternalPredicateCallback.ShouldQuery` is consulted before every call (`topdown.go:106`); a callback object created per evaluation (codeNERD registers callbacks per evaluation, `kernel_eval.go:333-344`, on a fresh store, `:256-259`) returns `false` once it has answered. `once_all/2`: **1 call**, two `ShouldQuery=false` refusals. This is the design's choice: it lives in the FFI adapter (`internal/core/external_predicates.go#virtualExternalPredicate`), decides nothing, and leaves the data clean.
- *Return a sentinel row.* `sentinel_all/2` returning `("", "")`: **1 call**, the cache engages; the consuming rule filters it (`q(X, K, V) :- p(X), sentinel_all(K, V), K != "".`, `q` empty). Works in pure Mangle but every consumer of the mount must know the sentinel — a rule with a variable over the key column (`knowledge_stale`) would otherwise see it.

With the cache engaged, the fresh store per evaluation (`kernel_eval.go:256`, M1 §1.3) makes the cost once per dirty query. For CodeDOM that is the volume the bulk assert already moves, but it no longer counts against the 250k EDB ceiling (M1 §1.4) and the revision join is done by the engine. Once E1 is fixed *and the analyzer checks external order*, the `+` form makes it one call per bound key, which is the design target; the Decls above are written so that only the `mode` string changes.

## 3. Pattern B — derived context injection

**Intent.** What enters an agent's window, why, where, and at what priority is one derived relation, `inject(Agent, Atom, Reason, Slot, Prio)`, computed from the task frame, the mounted knowledge (Pattern A), the history of what has been built, and the atom catalogue — including the **budget cut**, which today is Go (`internal/prompt/budget.go#TokenBudgetManager`, "Mandatory atom %s rejected", M1 §7 row 12; `internal/context/compressor.go:683-696` sorts `should_include_context` rows by parsed priority and "selects the top N within the AtomReserve token budget", `schemas_context.mg:26-28`). Today the kernel derives selection (`selected_result(Atom, Priority, Source)`, `jit_compiler.mg:310-316`, read by `internal/prompt/selector.go:997,1232`) but not fit, not position, and the mandatory flag is YAML data forced in Go (`compiler.go:1099,1148`). Section order is a static table (`category_order/2`, `jit_config.mg:9-22`) that ignores lost-in-the-middle.

**Engine features relied on.** Aggregation with `fn:max` and `fn:sum` after `fn:group_by`, each in its own stratum, zero case via negation (M0 §6, probes J, AJ); int64 arithmetic `fn:plus` and comparisons (M0 §5); `:string:contains` (M0 §5, the one builtin the corpus already leans on, M1 §3); stratified negation over bound projections (`has_above/2`, `fits/2`; M0 §4); `fn:collect_distinct` for the reason set (M0 §5 probe AJ). Eleven strata for the revised example — aggregation chains are the cost centre (see §11), and every aggregation body names all its columns (M3 engine surprise 4; §1 convention 6).

**Existing predicates it builds on (M1).** `prompt_atom/5` (`schemas_prompts.mg:198`), `atom_selector/3` (`:213`), `compile_shard/2` (`:261`), `compile_budget/1` (`:257` — declared, never asserted or read: grep `compile_budget` in `internal/prompt/*.go` → 0 hits; this pattern is its first consumer, widened to `/2` with the agent), `skeleton_category/1` (`jit_selection.mg:187-190`), `category_order/2`, `category_budget/2` (`jit_config.mg`), `mandatory_atom/1`, `selected_atom/1` (`schemas_prompts.mg:426,446`), `should_include_context/2` and `context_relevant/2` (`schemas_context.mg:20-26`, `context_compilation.mg:21-66`), `knowledge_hit/3` and `symbol_doc/3` from Pattern A.

**The pattern** (revised in pass 2 — M3 §5 items 6, 20; the program below is `C:/Temp/mangle-study/m2v2/patternB_v2.mg`, run on the verifier's harness 2026-09-18).

1. *Every reason is a rule and the reason is a column.* `candidate(Agent, Atom, Reason, Prio)` has one rule per admission reason: `/skeleton` (category), `/task_verb` (selector matches the intent verb), `/knowledge_hit` (Pattern A row above a boost threshold — `knowledge_boost(MinPct, Boost)` facts replace `category_budget` percentages), `/focal_symbol` (CodeDOM ref in the task frame), `/built_before` (history: the model changed this symbol at turn N, so the diff atom outranks the doc atom). **One row per (agent, atom, reason)**: the knowledge boost is `hit_boost(A, Atom, Boost)`, the `fn:max` over the tiers a score clears, so a 91-score hit is one candidate row at +40, not three rows at +40/+20/+5 (the first draft's per-tier rule left three rows behind `fn:max`, which any consumer counting candidates — and every proof of `inject_prio` — saw, M3 §2B design note). No rule says "mandatory": skeleton categories *are* the mandatory set, and `prompt_atom(..., /true)` is a legacy input the rule ignores.
2. *One priority per atom, and the reasons as a set.* `inject_prio(A, Atom, P)` is `fn:max` over the reasons — the kernel resolves multi-reason conflicts, not `selector.go`. `inject_reasons(A, Atom, Rs)` is `fn:collect_distinct(R)` over the same body: `fn:collect` would give one entry per candidate row, duplicates included (M3 `v_patternB_collect.mg`: `[/knowledge_hit, /knowledge_hit, /knowledge_hit]` under the old per-tier rule), and the list order is map-iteration order either way (3 of 8 nondeterministic runs flipped it, M3 §2B) — so the list is a set to a consumer, never compared or persisted as a sequence.
3. *The window shape is a prefix sum over priority tiers, not over atom pairs.* `tie_sum(A, P, S)` sums the tokens of each priority tier; `above_tier(A, P, Q, S)` pairs each tier with every strictly higher tier; `above_sum(A, P, S)` sums those. An atom `fits` when its tier's `above + tier <= budget`; a tier is all-or-nothing because Mangle has no ordering inside a tier (M0 §6 "no limit/top-k") and the design chooses not to invent one in Go: equal priority means equally justified, and a tie that does not fit is a budget fact to raise, not a coin to flip. The first draft paired *atoms* (`above_pair(A, Atom, Other, T)`), which is quadratic in candidates per agent: the verifier measured 0.46 / 1.5 / 5.8 s at 200 / 400 / 800 candidates and a 100 000 created-fact overflow at 400 (M3 §4 item 11, `v_perfB_*.mg`); the atom library is 351 atoms and codeNERD's ceiling is 500 000 (`kernel_init.go:188`). The tier form is quadratic in *distinct priorities* (tens) and linear in atoms: the same derived `fits`/`dropped` sets at 200 candidates (`perfB_tier_200.mg` vs `v_perfB_200.mg`, 200 rows, 0 differences), and 0.12–0.22 s at 400, 0.14–0.34 s at 800, 0.33–0.87 s at 3 200 candidates (`perfB_tier_400/800/3200.mg`, three runs each, process start included) where the pair form could not finish 800 under the fact limit. Per-agent scoping (only the compiling agent's rows in the store) is still the right operating mode; it is no longer what keeps the rule affordable.
4. *Position is a fact.* `window_slot(Category, Slot)` with `/head`, `/middle`, `/tail` replaces `category_order`: skeleton at the head, knowledge in the middle, the focal context and history at the tail next to the request. That is the lost-in-the-middle decision made in policy; Go concatenates slots in fixed order and priority-descending inside a slot, nothing else.
5. *Negatives are facts.* `dropped(A, Atom, /budget)` is derived for a candidate that does not fit; an atom that was never a candidate is absent from `inject_prio`, so "not relevant" and "relevant but cut" are different answers to `nerd why` (Pattern F).

**How the JIT consumes it.** `selector.go` replaces its `selected_result/3` query, its Go re-ranking and `TokenBudgetManager` with one query, `inject(Agent, Atom, Reason, Slot, Prio)` for the compile's agent, and one deterministic assembly: for each slot in `/head, /middle, /tail`, atoms by `Prio` descending, ties by atom ID (a string sort in Go is presentation, not a decision). `compressor.go:688` does the same for observation entities (`working_selected/2`, `working_set.mg:125-132`, is already this shape). The `kernel/context/*` and `kernel/knowledge/*` dynamic atoms that Go forces mandatory (`compiler.go:1099,1148`) become `prompt_atom` rows with a `/context` category and a `window_slot(/context, /tail)` fact. The manifest (`compiler.go:1581`) records the `inject` rows verbatim — that is the receipt `nerd meter atoms` (`cmd/nerd/cmd_meter.go:115`) already prints from `.nerd/meter/atom-selections.jsonl`.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternB_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternB_v2.mg inject dropped inject_prio inject_reasons candidate above_sum tie_sum hit_boost`):

```mangle
# Pattern B v2 -- derived context injection: inject(Agent, Atom, Reason, Slot, Prio).
# Pass-2 changes (M3 items 6, 20): the budget prefix sum is over priority tiers, not atom pairs;
# one candidate row per knowledge hit; reasons collected with fn:collect_distinct.
Decl compile_shard(Agent, ShardType) bound [/string, /name].
Decl compile_budget(Agent, Tokens) bound [/string, /number].
Decl task_verb(Agent, Verb) bound [/string, /name].
Decl task_focus_symbol(Agent, Sym) bound [/string, /string].
Decl prompt_atom(Atom, Category, Prio, Tokens, IsMandatory) bound [/string, /name, /number, /number, /name].
Decl atom_selector(Atom, Dim, Value) bound [/string, /name, /name].
Decl skeleton_category(Category) bound [/name].
Decl window_slot(Category, Slot) bound [/name, /name].
Decl knowledge_hit(Agent, Atom, ScorePct) bound [/string, /string, /number].
Decl knowledge_boost(Pct, Prio) bound [/number, /number].
Decl built_before(Agent, Sym, Turn) bound [/string, /string, /number].
Decl symbol_atom(Sym, Atom) bound [/string, /string].
Decl hit_boost(Agent, Atom, Boost) bound [/string, /string, /number].
Decl candidate(Agent, Atom, Reason, Prio) bound [/string, /string, /name, /number].
Decl inject_prio(Agent, Atom, Prio) bound [/string, /string, /number].
Decl inject_reasons(Agent, Atom, Reasons) bound [/string, /string, /any].
Decl tie_sum(Agent, Prio, Sum) bound [/string, /number, /number].
Decl above_tier(Agent, Prio, Higher, Sum) bound [/string, /number, /number, /number].
Decl above_sum(Agent, Prio, Sum) bound [/string, /number, /number].
Decl has_above(Agent, Prio) bound [/string, /number].
Decl fits(Agent, Atom) bound [/string, /string].
Decl inject(Agent, Atom, Reason, Slot, Prio) bound [/string, /string, /name, /name, /number].
Decl dropped(Agent, Atom, Why) bound [/string, /string, /name].

# policy facts (jit_config.mg)
skeleton_category(/identity).
skeleton_category(/safety).
window_slot(/identity, /head).
window_slot(/safety, /head).
window_slot(/knowledge, /middle).
window_slot(/language, /middle).
window_slot(/context, /tail).
knowledge_boost(90, 40).
knowledge_boost(60, 20).
knowledge_boost(30, 5).

# witnesses for one compile
compile_shard("coder-7", /coder).
compile_budget("coder-7", 260).
task_verb("coder-7", /fix).
task_focus_symbol("coder-7", "core.evaluate").
built_before("coder-7", "core.evaluate", 12).
prompt_atom("identity/coder", /identity, 100, 40, /true).
prompt_atom("safety/no_secrets", /safety, 100, 30, /true).
prompt_atom("language/go/errors", /language, 50, 60, /false).
prompt_atom("knowledge/go/error_wrapping", /knowledge, 40, 80, /false).
prompt_atom("knowledge/go/table_tests", /knowledge, 40, 70, /false).
prompt_atom("knowledge/python/asyncio", /knowledge, 40, 50, /false).
prompt_atom("context/core.evaluate", /context, 30, 90, /false).
prompt_atom("context/history/core.evaluate", /context, 30, 20, /false).
atom_selector("language/go/errors", /intent_verb, /fix).
atom_selector("knowledge/python/asyncio", /language, /python).
knowledge_hit("coder-7", "knowledge/go/error_wrapping", 91).
knowledge_hit("coder-7", "knowledge/go/table_tests", 74).
symbol_atom("core.evaluate", "context/core.evaluate").
symbol_atom("core.evaluate", "context/history/core.evaluate").

# candidates: every reason is a rule, the reason is a column; one row per (agent, atom, reason)
candidate(A, Atom, /skeleton, P) :-
    compile_shard(A, Shard), prompt_atom(Atom, Cat, P, T, M), skeleton_category(Cat).
candidate(A, Atom, /task_verb, P) :-
    task_verb(A, V), atom_selector(Atom, /intent_verb, V), prompt_atom(Atom, Cat, P, T, M).
# the boost is the highest tier the score clears, so a hit is one candidate row, not one per tier
hit_boost(A, Atom, Boost) :-
    knowledge_hit(A, Atom, Score), knowledge_boost(Min, B0), Score >= Min
    |> do fn:group_by(A, Atom), let Boost = fn:max(B0).
candidate(A, Atom, /knowledge_hit, Boosted) :-
    hit_boost(A, Atom, Boost), prompt_atom(Atom, Cat, P, T, M), Boosted = fn:plus(P, Boost).
candidate(A, Atom, /focal_symbol, P) :-
    task_focus_symbol(A, Sym), symbol_atom(Sym, Atom), prompt_atom(Atom, Cat, P, T, M).
candidate(A, Atom, /built_before, 95) :-
    built_before(A, Sym, Turn), symbol_atom(Sym, Atom), prompt_atom(Atom, /context, P, T, M),
    :string:contains(Atom, "history").

# one priority per atom: the max over its reasons; the reasons as a set (order is not stable)
inject_prio(A, Atom, P) :-
    candidate(A, Atom, R, P0) |> do fn:group_by(A, Atom), let P = fn:max(P0).
inject_reasons(A, Atom, Rs) :-
    candidate(A, Atom, R, P0) |> do fn:group_by(A, Atom), let Rs = fn:collect_distinct(R).

# the window-shape decision: prefix sums by priority tier, no Go sort.
# tie_sum is per tier; above_tier pairs tiers (quadratic in tiers, linear in atoms).
tie_sum(A, P, S) :-
    inject_prio(A, Atom, P), prompt_atom(Atom, Cat, P0, T, M) |> do fn:group_by(A, P), let S = fn:sum(T).
above_tier(A, P, Q, S) :- tie_sum(A, P, S0), tie_sum(A, Q, S), Q > P.
above_sum(A, P, S) :-
    above_tier(A, P, Q, S0) |> do fn:group_by(A, P), let S = fn:sum(S0).
has_above(A, P) :- above_sum(A, P, S).
fits(A, Atom) :-
    inject_prio(A, Atom, P), above_sum(A, P, Above), tie_sum(A, P, Tie),
    compile_budget(A, B), Total = fn:plus(Above, Tie), Total <= B.
fits(A, Atom) :-
    inject_prio(A, Atom, P), !has_above(A, P), tie_sum(A, P, Tie),
    compile_budget(A, B), Tie <= B.

inject(A, Atom, Reason, Slot, P) :-
    fits(A, Atom), inject_prio(A, Atom, P), candidate(A, Atom, Reason, P0),
    prompt_atom(Atom, Cat, P1, T, M), window_slot(Cat, Slot).
dropped(A, Atom, /budget) :-
    inject_prio(A, Atom, P), !fits(A, Atom).
```

**Expected derived facts** (verified 2026-09-18 09:10, `mgv.exe -store simple -det=false -runs 5`, identical; `strata=11` — two more than the first draft's nine: `hit_boost` and `inject_reasons` are each their own aggregation stratum):

```
inject("coder-7","identity/coder",/skeleton,/head,100)
inject("coder-7","safety/no_secrets",/skeleton,/head,100)
inject("coder-7","context/history/core.evaluate",/built_before,/tail,95)
inject("coder-7","context/history/core.evaluate",/focal_symbol,/tail,95)
inject("coder-7","knowledge/go/error_wrapping",/knowledge_hit,/middle,80)
inject("coder-7","knowledge/go/table_tests",/knowledge_hit,/middle,60)
dropped("coder-7","language/go/errors",/budget)
dropped("coder-7","context/core.evaluate",/budget)
hit_boost("coder-7","knowledge/go/error_wrapping",40)
hit_boost("coder-7","knowledge/go/table_tests",20)
candidate: 8 rows, one per (atom, reason)
inject_reasons("coder-7","context/history/core.evaluate",[/focal_symbol, /built_before])   (a set; order varies between runs)
tie_sum: (100,70) (95,20) (80,80) (60,70) (50,60) (30,90)
above_sum: (95,70) (80,90) (60,170) (50,240) (30,300)
```

The `inject` and `dropped` sets are byte-identical to the first draft's (the verifier's `v_patternB_300.mg` and `v_patternB_2agents.mg` follow-ons — budget 300 admits `language/go/errors`; a second agent with its own budget is scoped by `A` — hold unchanged, because `fits` compares the same numbers, now indexed by tier instead of by atom). Arithmetic the kernel did: tiers 100 (70 tokens) + 95 (20) + 80 (80) + 60 (70) = 240 ≤ 260; the tier-50 atom would make 300 and is cut; the tier-30 doc atom (90 tokens) is cut behind it. `knowledge/python/asyncio` is not in `inject_prio` at all — no rule admitted it — which is a different answer from `dropped`. Raising `compile_budget` to 300 admits `language/go/errors` (one fact); the same policy change in Go today is a constant in `budget.go`.

**Where the two reasons for one atom go.** `context/history/core.evaluate` carries two `inject` rows (`/built_before`, `/focal_symbol`); Go groups by atom and keeps both reasons in the manifest, or reads `inject_reasons` — a `fn:collect_distinct` set whose order is map-iteration order (M0 §7, M3 §2B), so never compare or persist it as a sequence.

## 4. Pattern C — derived test obligations

**Intent.** The tests an agent must write or run are derived from the north-star behaviors and the change set, keyed on *behavior*, not on line coverage: `test_obligation(Behavior, Target, Kind)` with `Kind ∈ {/write, /rerun}` and `coverage_missing(Behavior, Sym)`. Today the coder shard writes tests only if told (root `CLAUDE.md`, "Use codeNERD Where It Can Do the Job"), the only test-related verdict is `hollow_success("new source was created without a test file")` via `missing_test_for/1` (`coder_safety.mg:28,104-107`, file-level, created files only), and the test witness is one global atom with no test name and no revision — `test_state(/passing|/failing)` asserted by `handleRunTests` (`internal/core/virtual_store_actions.go:442-455`) and `tdd_loop.go:251-254,958`. `test_result/4` is declared (`schemas_analysis.mg:182`) but grep of `internal/core`, `internal/session`, `internal/shards` (non-test) finds no producer (**unverified** elsewhere). `test_impact.mg` already derives `impacted_test/1` from `plan_edit` + `test_depends_on_transitive` (`policy/test_impact.mg:72-140`) — but its producers were never wired with the right identifier shape (`internal/mangle/agents.md:16-20`), so it derives nothing.

**Engine features relied on.** Linear recursion for `test_depends_on/2` closure (allowed, M0 §4 stratification; the corpus already has 18 self-recursive predicates, M1 §3); stratified negation over bound projections (`has_test_for/1`, `has_test_failure_now/1`, `test_green_now/1`); a revision join to age evidence (Pattern A rule 2). No aggregation is needed — obligations are a set.

**Existing predicates it builds on (M1).** `code_element/5`, `code_calls/2`, `is_test_function/1`, `test_depends_on/2`, `test_depends_on_transitive/2`, `impacted_test/1` (`schemas_codedom.mg:33`, `schemas_analysis.mg:116`, `test_impact.mg:60-140`); `modified_file/1` (produced by `transaction_manager.go:610`), `turn_created_source/1` (`executor.go:2416`, Decl `schemas_execution.mg:245`), `plan_edit/1` (`schemas_codedom_polyglot.mg:206`, currently fed a file path instead of a ref, `agents.md:20`); `northstar_requirement/4`, `northstar_capability/4`, `northstar_supports/2` (`schemas_misc.mg:47-80`) and the `.nerd/northstar.mg` extension point (M1 §1.1); `test_state/1`, `failing_test/2` (`schemas_execution.mg:16`, `schemas_reviewer.mg:121`); `has_test_coverage/1`, `test_coverage/1` (`schemas_codedom_polyglot.mg:188`, `schemas_world.mg:38`).

**The pattern** (revised in pass 2 — M3 §5 items 1, 7, 8, 9, 17; every rule below is the program the verifier's harness ran on 2026-09-18, `C:/Temp/mangle-study/m2v2/patternC_v2.mg`).

1. *Behaviors are facts with witnesses in the code.* `behavior(B, Text)` rows come from the north star (`northstar_requirement(ReqID, /behavior, Text, Prio)` is the natural source); `behavior_witness(B, Sym)` maps a behavior to the CodeDOM refs that realise it. The mapping is seeded by hand in `.nerd/northstar.mg` and can be learned (a `learned_behavior_witness` row promoted through the existing learned-rule path, `kernel_policy.go:360`), but it is never inferred by the model at task time.
2. *Tests witness behaviors through the dependency closure.* `behavior_test(B, T) :- behavior_witness(B, Sym), test_depends_on(T, Sym)` — this is `test_impact.mg`'s closure with the behavior as the key.
3. *The change set is per task.* `changed_symbol(Task, Sym, Rev)` is asserted by the transaction manager where the ref is known (`agents.md:32-35` says exactly where that is and is not), stamped with the task that made the change. **The `Rev` column is provenance, not a filter**: no rule in this pattern joins it, so an obligation created by a change at `rev-15` stays open at `rev-17` until a test exists — the obligation is "this task touched this behavior", and touching does not expire with a revision (M3 §2C `v_patternC_oldrev.mg` showed the column was never read; the first draft called it a "revision-stamped witness" as if it aged, which it does not). The task column is what makes obligations per task: `behavior_touched(Task, B)` joins the task's own change set, so a session with two active tasks (campaigns, parallel shards) does not cross-join every task with every behavior (M3 §2D `v_patternD_2tasks.mg`).
4. *Evidence expires with the revision, and a failure at the revision defeats a pass.* `test_green_now(T) :- test_run(Task, T, Rev, /pass), current_rev(Rev), !has_test_failure_now(T)` with `has_test_failure_now(T) :- test_run(Task, T, Rev, /fail), current_rev(Rev)`. A green run at `rev-16` is not evidence at `rev-17`; a `/pass` and a `/fail` both at `rev-17` (a flaky test, or a re-run after a fix that the harness recorded twice) is *not* green — the first draft's rule meant "some run at this revision passed", and the verifier's `v_patternC_cycle.mg` showed that made a failing test green. The witness is **`test_run(Task, TestRef, Rev, Outcome)`** — one shape, task-scoped, shared with Pattern G (the first draft declared a `/3` here and a `/4` there; two witnesses with one name and different arities are two predicates, and a producer that emits one leaves the other's rules silently empty, M3 §2 composition). If the harness can give runs an ordinal, `fn:max` over it and "the latest run passed" is the stronger rule; the negation form needs no new column.
5. *Obligations are the negations.* `test_obligation(Task, B, Target, Kind)`: `/write` when a touched behavior's changed symbol has no test at all; `/rerun` when a touched behavior's test is not green at the current revision; `coverage_missing(B, Sym)` for the untouched gap the north star still names (behavior-level, not task-level — it is a property of the codebase). `obligation_open(Task, B)` is the projection Pattern D negates. When the model adds a test for the `/write` target, the obligation does not vanish: the new test has no run at the current revision, so it becomes `/rerun` for that test (M3 `v_patternC_assert.mg`), and `obligation_open` stays true until the run is green. That is the intended chain — write, then run, then done.

**What the harness asserts, and when.** After each edit: `changed_symbol(Task, Sym, Rev)` per touched ref and `current_rev(Rev)` (retract-and-assert of a one-row predicate through `Transaction()`, `kernel_transactions.go:42`; Pattern D carries the invariant rule that reports a second row as `kernel_invariant_violated(/current_rev, N)`); after each CodeDOM refresh: the `code_*` rows for touched files (or the Pattern A mount at the new hash). After each test run: one `test_run(Task, TestRef, Rev, Outcome)` per test the runner reported, parsed from `go test -json` (`internal/core/virtual_store_actions.go#handleRunTests` has the output, `:452-455`) — never a single global atom. After the model writes a test: nothing special; the next CodeDOM refresh adds the `is_test_function` and `code_calls` rows, the `/write` obligation stops deriving and the `/rerun` obligation for the new test starts.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternC_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternC_v2.mg test_obligation coverage_missing behavior_touched test_green_now has_test_failure_now obligation_open`):

```mangle
# Pattern C v2 -- derived test obligations, per task (M3 items 1, 7, 8, 9, 17).
Decl behavior(B, Description) bound [/string, /string].
Decl behavior_witness(B, Sym) bound [/string, /string].
Decl code_element(Ref, Kind, File, StartLine, EndLine) bound [/string, /name, /string, /number, /number].
Decl code_calls(Caller, Callee) bound [/string, /string].
Decl is_test_function(Ref) bound [/string].
Decl current_rev(Rev) bound [/string].
Decl task_active(Task) bound [/string].
Decl changed_symbol(Task, Sym, Rev) bound [/string, /string, /string].
Decl test_run(Task, TestRef, Rev, Outcome) bound [/string, /string, /string, /name].
Decl test_depends_on(TestRef, Sym) bound [/string, /string].
Decl behavior_test(B, TestRef) bound [/string, /string].
Decl has_test_for(Sym) bound [/string].
Decl behavior_touched(Task, B) bound [/string, /string].
Decl has_test_failure_now(TestRef) bound [/string].
Decl test_green_now(TestRef) bound [/string].
Decl test_obligation(Task, B, Target, Kind) bound [/string, /string, /string, /name].
Decl coverage_missing(B, Sym) bound [/string, /string].
Decl obligation_open(Task, B) bound [/string, /string].

# north star (.nerd/northstar.mg / northstar_requirement rows)
behavior("B1", "stale evidence never survives a source change").
behavior("B2", "a turn is done only with executed acceptance").
behavior_witness("B1", "core.evaluate").
behavior_witness("B1", "core.assert").
behavior_witness("B2", "session.turnDone").

# CodeDOM at the current revision
code_element("core.evaluate", /function, "internal/core/kernel_eval.go", 181, 260).
code_element("core.assert", /function, "internal/core/kernel_facts.go", 555, 600).
code_element("session.turnDone", /function, "internal/session/executor.go", 2330, 2360).
code_element("core.TestEvaluateFresh", /function, "internal/core/kernel_eval_test.go", 10, 40).
code_element("session.TestTurnDone", /function, "internal/session/executor_test.go", 5, 30).
is_test_function("core.TestEvaluateFresh").
is_test_function("session.TestTurnDone").
code_calls("core.TestEvaluateFresh", "core.evaluate").
code_calls("session.TestTurnDone", "session.turnDone").

# change set and test runs: per task, asserted by the harness after each edit / run
current_rev("rev-17").
task_active("t1").
task_active("t2").
changed_symbol("t1", "core.evaluate", "rev-17").
changed_symbol("t1", "core.assert", "rev-17").
changed_symbol("t2", "session.turnDone", "rev-17").
test_run("t1", "core.TestEvaluateFresh", "rev-16", /pass).
test_run("t2", "session.TestTurnDone", "rev-17", /pass).
test_run("t2", "session.TestTurnDone", "rev-17", /fail).

test_depends_on(T, Sym) :- is_test_function(T), code_calls(T, Sym).
test_depends_on(T, Sym) :- test_depends_on(T, Mid), code_calls(Mid, Sym).

behavior_test(B, T) :- behavior_witness(B, Sym), test_depends_on(T, Sym).
has_test_for(Sym) :- test_depends_on(T, Sym).

# touched is per task: the task's own change set, not every task's
behavior_touched(Task, B) :- changed_symbol(Task, Sym, Rev), behavior_witness(B, Sym).

# Evidence is current only at the current revision, and a failure at that
# revision defeats a pass at the same revision (flaky or re-run tests).
has_test_failure_now(T) :- test_run(Task, T, Rev, /fail), current_rev(Rev).
test_green_now(T) :- test_run(Task, T, Rev, /pass), current_rev(Rev), !has_test_failure_now(T).

test_obligation(Task, B, Sym, /write) :-
    behavior_touched(Task, B), behavior_witness(B, Sym), changed_symbol(Task, Sym, Rev), !has_test_for(Sym).
test_obligation(Task, B, T, /rerun) :-
    behavior_touched(Task, B), behavior_test(B, T), !test_green_now(T).

coverage_missing(B, Sym) :- behavior_witness(B, Sym), !has_test_for(Sym).
obligation_open(Task, B) :- test_obligation(Task, B, Target, Kind).
```

**Expected derived facts** (verified 2026-09-18 08:40 with the verifier's harness, `-store simple -det=false -runs 5`, identical across runs; `strata=9`):

```
behavior_touched("t1","B1")
behavior_touched("t2","B2")
has_test_failure_now("session.TestTurnDone")
test_obligation("t1","B1","core.TestEvaluateFresh",/rerun)
test_obligation("t1","B1","core.assert",/write)
test_obligation("t2","B2","session.TestTurnDone",/rerun)
coverage_missing("B1","core.assert")
obligation_open("t1","B1")
obligation_open("t2","B2")
```

`test_green_now` is empty: t1's only run is one revision old, and t2's test has a `/pass` *and* a `/fail` at `rev-17`, so it is not green and B2 — touched by t2 — carries a `/rerun`. t1 is touched twice: `core.evaluate` has a test whose last green run is one revision old (`/rerun`), `core.assert` has no test (`/write`). Nothing of t1's leaks into t2 or back. Asserting `test_run("t1","core.TestEvaluateFresh","rev-17",/pass)` removes t1's `/rerun` row; adding a test that calls `core.assert` (two CodeDOM rows + `is_test_function`) removes `/write` and `coverage_missing` and adds `test_obligation("t1","B1","core.TestAssert",/rerun)` until that test runs green — no Go code decides any of it.

**Identifier shape.** Every `Sym`/`Ref` column is the Cartographer's `<pkg>.<Name>` string (`agents.md:27-31`). The example uses `core.evaluate`-style refs deliberately; a producer that emits bare names or file paths makes every rule above derive nothing, silently, and only the end-to-end kernel test of §1 catches it. No rule above has a wildcard in a positive premise (R1, §7) or in an aggregation body (§1 convention 6) — the unused variables (`Task` in `test_green_now`, `Rev` in `behavior_touched`) are named so the proof keeps the leaf.

## 5. Pattern D — completion as obligation fixpoint

**Intent.** A task is not done when the model says so; it is done when nothing obliges more work. `task_incomplete(Task, Kind, Target)` is the union of every open obligation from the other patterns, `task_complete(Task)` is its negation, and the loop reads one row, `loop_verdict(Task, /complete | /continue | /stop_stalled)`. Progress, stalls and repeated failures are derived from round witnesses; the iteration cap and the adaptive extension logic in `internal/session/tool_budget_controller.go:192-238` (`maybeExtend`: hard limit, extension count, repeated tail cycle, "no novel successful tool result", "write-oriented intent stalled") become policy facts and rules — the same move `working_set.mg:53-110` already made for `working_stop`, `working_nudge` and `working_finalize`, but in the executive kernel and gating completion, not just continuation. `working_continue()`'s own doc says it: "Continuation requires no observed stall; it never means task completion" (`working_set.mg:51`).

**Engine features relied on.** Stratified negation over zero/one-arity projections (`has_obligation/1`, `has_acceptance_now/1`, `stalled/1`; M0 §4, §12 item 1); `fn:max` and `fn:count` aggregation, each its own stratum (M0 §6); `fn:minus` and `fn:string:concat` (M0 §5); fact-valued thresholds compared with `>=`. Thirteen strata in the revised example (ten in the first draft; the failure window and the `current_rev` invariant add three); the union predicate `obligation/3` is the seam every other pattern writes into, and every source predicate carries a task column.

**Existing predicates it builds on (M1).** `turn_evidence/6`, `hollow_success/1`, `turn_executed/1`, `turn_done/1`, `turn_acceptance/3` (`coder_safety.mg:79-115`) — the only completion verdict that is derived today, and it is per *turn*, not per task; `working_progress/5`, `working_control/2`, `working_stop/1`, `working_stall_rounds/1`, `working_nudge_rounds/1` (`working_set.mg:33-57`); `task_status`/`task_completed` in the model-observation allowlist (`mangle_updates.go:30-53`) — model-assertable, and by this pattern never an obligation source; `test_obligation/4` (Pattern C, task-scoped), `edit_plan`/`edit_step_pending` (Pattern E), `verdict` and `unverified_claim` facts (Pattern G), `acceptance_state`/`workspace_snapshot` (M1 §7 row 7).

**The pattern** (revised in pass 2 — M3 §5 items 1, 2, 16; the program below is `C:/Temp/mangle-study/m2v2/patternD_v3.mg`, run on the verifier's harness 2026-09-18).

1. *Obligations are a union predicate with a kind column, joined on the task.* One rule per source: `/test_write`, `/test_rerun` (Pattern C — `test_obligation(Task, B, Target, Kind)` carries the task, so the join is `test_obligation(T, B, Target, /write)`, never `task_active(T), test_obligation(_, Target, /write)`: the first draft's cross product gave every active task every other task's obligations, M3 `v_patternD_2tasks.mg`), `/edit_step` (Pattern E), `/acceptance` (Pattern G — no `acceptance(T, Rev)` at the *current* revision), `/hollow` (Pattern G's `unverified_claim(T, Claim)`, task-scoped by construction). New obligation kinds are new rules; the loop never learns about them. Every source predicate in the union has a task column — that is the contract this pattern imposes on the others.
2. *Complete is the negation.* `task_complete(T) :- task_active(T), !has_obligation(T).` Nothing the model asserts can make this true: `task_status`/`task_completed` are not in any obligation rule, and `turn_done` is host-witnessed (`mangle_updates.go:145-150`).
3. *Progress is a derived comparison over round witnesses; repeated failure is a window, not a lifetime total.* At each round boundary Go asserts `round_done(T, R)`, `open_count_at(T, R, N)` (the kernel's own `obligation` count from the previous fixpoint, stamped — derived facts are not persisted across evaluations, M1 §1.3, so history of a derivation is a witness) and `tool_failure(T, R, Digest)` for every failed tool call, `Digest` being a hash of (tool, args, error class). `stall(T, /no_progress)` holds when the open count has not fallen over `stall_rounds(K)`; `stall(T, /repeated_failure)` when one digest recurs `repeated_failure_rounds(K)` times **within the last K rounds** — `recent_failure(T, R, D)` keeps only rows with `R > latest_round − K`, and `failure_count` counts those. The first draft counted the whole history, which stopped a task that hit the same build error at rounds 1, 5 and 9 with progress in between (M3 `v_patternD_spread.mg`); the controller's "period-1..3 repeated trace" detection (`tool_budget_controller.go:276-302`) looks at the tail of the trace, and the window is what makes the rule the same decision. `stall(T, /spend_ceiling)` when the meter's `spent(T, Tokens)` (from `.nerd/meter/receipts.jsonl`, §10) reaches `spend_ceiling(T, Tokens)`.
4. *The verdict is one row per active task.* `/complete` when complete; `/stop_stalled` when obligations remain and a stall holds; `/continue` otherwise. The loop cannot end on `/continue` and cannot continue on `/complete` — and there is no round number in any rule.
5. *What replaces tool budgets and iteration caps.* Nothing counts tool calls. The only ceilings are facts: `spend_ceiling/2` (tokens, from the meter, the user's real constraint), `stall_rounds/1`, `repeated_failure_rounds/1`. `ExecutorConfig`'s iteration limit, `maxExtensions`, `extensionSize`, `hardLimit` (`tool_budget_controller.go:21,79-96`) are deleted, not moved; "arbitrary tool-call counts are not task-completion criteria" (root `CLAUDE.md`, North Star). The nudges (`working_nudge/1`) stay as steering facts the loop appends to a tool result; they are not verdicts.
6. *Kernel invariants are facts too.* `current_rev/1` must be one row — every revision join in Patterns C, E, G widens silently if it is not (M3 composition: two `current_rev` rows flipped a Pattern G verdict). The invariant is a rule, `kernel_invariant_violated(/current_rev, N)` from a `fn:count` over `current_rev`, and the loop treats any `kernel_invariant_violated` row as `/stop_stalled` with that reason (a rule to add when the loop lands; the example prints the fact). An alternative is a `fundep([], [Rev])` + `merge` Decl (M0 §3 item 4), which makes the second assert *replace* the first instead of reporting it — replacing hides the producer bug, reporting names it, so the design reports.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternD_v3.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternD_v3.mg loop_verdict task_incomplete stall failure_count recent_failure latest_round kernel_invariant_violated`):

```mangle
# Pattern D v3 -- completion as an obligation fixpoint; per-task obligations,
# repeated failures windowed over the last K rounds (M3 items 1, 2, 16).
Decl task_active(Task) bound [/string].
Decl round_done(Task, Round) bound [/string, /number].
Decl open_count_at(Task, Round, N) bound [/string, /number, /number].
Decl tool_failure(Task, Round, Digest) bound [/string, /number, /string].
Decl spent(Task, Tokens) bound [/string, /number].
Decl test_obligation(Task, B, Target, Kind) bound [/string, /string, /string, /name].
Decl edit_step_pending(Task, Step) bound [/string, /number].
Decl acceptance(Task, Rev) bound [/string, /string].
Decl current_rev(Rev) bound [/string].
Decl unverified_claim(Task, Claim) bound [/string, /name].
Decl stall_rounds(N) bound [/number].
Decl repeated_failure_rounds(N) bound [/number].
Decl spend_ceiling(Task, Tokens) bound [/string, /number].
Decl obligation(Task, Kind, Target) bound [/string, /name, /string].
Decl has_obligation(Task) bound [/string].
Decl has_acceptance_now(Task) bound [/string].
Decl task_incomplete(Task, Kind, Target) bound [/string, /name, /string].
Decl task_complete(Task) bound [/string].
Decl latest_round(Task, R) bound [/string, /number].
Decl recent_failure(Task, Round, Digest) bound [/string, /number, /string].
Decl failure_count(Task, Digest, N) bound [/string, /string, /number].
Decl stall(Task, Reason) bound [/string, /name].
Decl stalled(Task) bound [/string].
Decl loop_verdict(Task, Verdict) bound [/string, /name].
Decl current_rev_count(N) bound [/number].
Decl kernel_invariant_violated(Pred, N) bound [/name, /number].

stall_rounds(3).
repeated_failure_rounds(3).
spend_ceiling("t1", 200000).

task_active("t1").
task_active("t2").
current_rev("rev-17").
round_done("t1", 1). round_done("t1", 2). round_done("t1", 3). round_done("t1", 4). round_done("t1", 5).
open_count_at("t1", 1, 3). open_count_at("t1", 2, 3). open_count_at("t1", 3, 2).
open_count_at("t1", 4, 2). open_count_at("t1", 5, 2).
tool_failure("t1", 3, "sha:go-build-undefined-foo").
tool_failure("t1", 4, "sha:go-build-undefined-foo").
tool_failure("t1", 5, "sha:go-build-undefined-foo").
spent("t1", 84000).
test_obligation("t1", "B1", "core.assert", /write).
edit_step_pending("t1", 2).
acceptance("t1", "rev-16").
# t2: a second active task with nothing open
round_done("t2", 1).
open_count_at("t2", 1, 0).
acceptance("t2", "rev-17").

# obligations: a union over every pattern's open items, joined on the task
obligation(T, /test_write, Target) :- test_obligation(T, B, Target, /write).
obligation(T, /test_rerun, Target) :- test_obligation(T, B, Target, /rerun).
obligation(T, /edit_step, Step) :- edit_step_pending(T, N), Step = fn:string:concat("step-", N).
obligation(T, /acceptance, "acceptance at current revision") :- task_active(T), !has_acceptance_now(T).
obligation(T, /hollow, Reason) :- unverified_claim(T, Claim), Reason = fn:string:concat("claim:", Claim).
has_acceptance_now(T) :- acceptance(T, Rev), current_rev(Rev).
has_obligation(T) :- obligation(T, K, Target).

task_incomplete(T, K, Target) :- obligation(T, K, Target).
task_complete(T) :- task_active(T), !has_obligation(T).

# progress, stalls, repeated failures: derived from round witnesses
latest_round(T, R) :- round_done(T, R0) |> do fn:group_by(T), let R = fn:max(R0).
stall(T, /no_progress) :-
    latest_round(T, R), stall_rounds(K), R0 = fn:minus(R, K),
    open_count_at(T, R, N), open_count_at(T, R0, N0), N >= N0.
# a failure counts only inside the last repeated_failure_rounds(K) rounds
recent_failure(T, R, D) :-
    tool_failure(T, R, D), latest_round(T, L), repeated_failure_rounds(K),
    Lo = fn:minus(L, K), R > Lo.
failure_count(T, D, N) :-
    recent_failure(T, R, D) |> do fn:group_by(T, D), let N = fn:count().
stall(T, /repeated_failure) :-
    failure_count(T, D, N), repeated_failure_rounds(K), N >= K.
stall(T, /spend_ceiling) :-
    spent(T, S), spend_ceiling(T, C), S >= C.
stalled(T) :- stall(T, Reason).

# the verdict the loop reads: exactly one row per active task
loop_verdict(T, /complete) :- task_complete(T).
loop_verdict(T, /stop_stalled) :- task_active(T), has_obligation(T), stalled(T).
loop_verdict(T, /continue) :- task_active(T), has_obligation(T), !stalled(T).

# kernel invariant: current_rev is one row; a violation is a fact, not a silent widening
current_rev_count(N) :- current_rev(R) |> do fn:group_by(), let N = fn:count().
kernel_invariant_violated(/current_rev, N) :- current_rev_count(N), N > 1.
```

**Expected derived facts** (verified 2026-09-18 08:45, `mgv.exe -store simple -det=false -runs 5`, identical; `strata=13` — three more than the first draft: `recent_failure` sits between the `latest_round` aggregate and the `failure_count` aggregate, and the invariant adds an aggregate plus its consumer):

```
task_incomplete("t1",/acceptance,"acceptance at current revision")
task_incomplete("t1",/edit_step,"step-2")
task_incomplete("t1",/test_write,"core.assert")
latest_round("t1",5)
latest_round("t2",1)
recent_failure("t1",3,"sha:go-build-undefined-foo")
recent_failure("t1",4,"sha:go-build-undefined-foo")
recent_failure("t1",5,"sha:go-build-undefined-foo")
failure_count("t1","sha:go-build-undefined-foo",3)
stall("t1",/repeated_failure)
loop_verdict("t1",/stop_stalled)
loop_verdict("t2",/complete)
```

t2 has nothing open and its acceptance is at the current revision: `/complete`, and none of t1's obligations reach it (the first draft gave t2 a `/test_write` it never earned). `stall(/no_progress)` does not derive for t1: at round 5 the open count is 2, at round 2 it was 3. `kernel_invariant_violated` is empty. Two variants were also run:

- `patternD_v3_spread.mg` — nine rounds, the same digest failing at rounds 1, 5 and 9 with the open count falling 3→0 in between: `recent_failure` has only the round-9 row, `failure_count(...,1)`, no `stall`, `loop_verdict("t1",/continue)`. The first draft's lifetime count said `/stop_stalled` here (M3 `v_patternD_spread.mg`).
- `patternD_v3_2rev.mg` — a second `current_rev("rev-18")` row: `current_rev_count(2)`, `kernel_invariant_violated(/current_rev,2)`; the verdicts are unchanged in this toy because no acceptance sits at `rev-18`, which is exactly why the violation must be a fact and not left to be noticed through a widened join.

Removing the round-5 `tool_failure` row yields `/continue` for t1 (the window then holds two rows). Asserting `acceptance("t1","rev-17")`, a green run of a test for `core.assert` (Pattern C) and the step-2 edit witness (Pattern E) empties `obligation` and the verdict becomes `/complete` — three witnesses, zero Go.

**Where the round witnesses come from.** `round_done` and `tool_failure` from the executor's existing tool-trace bookkeeping (`tool_budget_controller.go:333` "It only counts"); `open_count_at` from the previous fixpoint's `obligation` rows (Go counts rows of a derived predicate and stamps the round — a witness of kernel state, not a decision); `spent` from the metered boundary (`cmd/nerd/cmd_meter.go:98-103`). The chat loop's `working_control(/yes, _)` cycle flag (`working_set.mg:79`, Go-detected) is subsumed by the windowed `failure_count` once the digest includes the tool and arguments; whether an identical *successful* call repeated N times is a stall is a policy choice — add `tool_call(T, R, Digest)` and a `repeated_call_rounds/1` fact if so, with the same `recent_*` window.

## 6. Pattern E — blast-radius edits as derived plans

**Intent.** A change with a blast radius is carried out by a tool, not by the LLM hand-editing (root `CLAUDE.md`, "Tools exist for exactly three things"). The model chooses *what* — one typed fact, `chosen_change(Task, Ref, Op, Spec)`, produced by a typed tool call, never parsed from prose — and the kernel derives *where*: `edit_plan(Task, Step, Ref, File, Op, Spec)` over the CodeDOM closure, plus `blast_radius/2`, `plan_gate/2` (`/apply`, `/confirm`, `/blocked`) and `edit_step_pending/2` for Pattern D. A deterministic CodeDOM operation (`handleEditElement`, `internal/core/virtual_store_codedom.go:155`, today a single-element edit) applies the rows; every applied row comes back as a witness `edit_applied(Task, Ref, Op, Rev)`. Today `codedom_edit.mg` derives risk labels (`breaking_change_risk/3`, `edit_unsafe/2`, `element_edit_blocked/2`, `:39-115`) but no plan, and `has_external_callers/1` (`:75-82`) is defined by *visibility*, not by `code_calls` rows — the name promises a join that is not there. `plan_edit/1` is fed a file path where the rules want an element ref (`agents.md:20`).

**Engine features relied on.** Joins over the CodeDOM relations with bound refs first (M0 §7); a two-step closure through interface → implementers → callers written as separate rules (no general recursion needed: rename radius is bounded by one interface hop; `code_calls` closure for `/signature` changes would use the `test_depends_on` recursion of Pattern C); `fn:count` aggregation for the radius over a projection with every column named (M0 §6; M3 engine surprise 4); negation over `has_block/1` and `edit_done_now/3` projections (M0 §4); a revision join so "applied" means applied *at this revision*.

**Existing predicates it builds on (M1).** `code_element/5`, `code_calls/2`, `code_implements/2`, `method_of/2` (`schemas_codedom.mg:33,87`, `schemas_analysis.mg:116,120`), `element_visibility/2`, `element_parent/2`, `api_handler_function/3` and the `edit_unsafe`/`element_edit_blocked` rules (`codedom_edit.mg:39-70`), `impact_graph/3` and `impacted/1` (`schemas_reviewer.mg:358`, `schemas_analysis.mg:50`), `next_action(/edit_element)` (`codedom_edit.mg:25`), `successful_edit/2`, `failed_edit/2`, `edit_success_count/2` (`:151-165` — already an aggregation), `modified_file/1` from the transaction manager (`transaction_manager.go:610`).

**The pattern** (revised in pass 2 — M3 §5 items 3, 4; the program below is `C:/Temp/mangle-study/m2v2/patternE_v3.mg`, run on the verifier's harness 2026-09-18).

1. *The choice is a fact with an operation vocabulary.* `chosen_change(Task, Ref, Op, Spec)` with `Op ∈ {/rename, /signature, /extract, /inline, /delete}` and `Spec` the operation's argument (new name, new signature string). A task may hold several rows at once — two operations on one ref, or two refs — and the plan keeps them apart (rule 2). The tool that accepts it is the only write path for a multi-site change; free-text `edit_lines` on a caller of a renamed symbol is a constitutional denial once `edit_plan` exists (`permitted` gating in `constitution.mg`, M1 §4.2).
2. *Sites are derived per tier and belong to a change.* `plan_site(Task, Change, Op, Ref, Tier)` names the change (`Change` = the chosen ref, `Op`) every site was derived from. `/definition` (the ref, for every `Op`), `/implementer` (every method of every struct implementing the interface the ref belongs to), `/caller` (non-test callers of the definition and of each implementer), `/test` (test callers) — the last three are `/rename` rules today; a `/signature` change gets only its definition site until a `code_calls`-closure rule for it is written. Each tier is a rule; a new language, relation or operation is a new rule. The first draft's `plan_site(Task, Ref, Tier)` had no change column, and `edit_plan` joined sites to changes on the task alone: with a `/rename` and a `/signature` on one task it emitted 12 rows and put the signature spec on every rename site (M3 `v_patternE_2changes.mg`). Now `edit_plan` joins `plan_site(T, Change, Op, Ref, Tier)` to `chosen_change(T, Change, Op, Spec)` — a site can only carry its own change's spec.
3. *Blocks are derived and block the whole plan.* A site in a generated file (`generated_file/1`, cf. `edit_unsafe(Ref, /generated_code)`, `codedom_edit.mg:39`) derives `plan_blocked`, and `edit_plan` has `!has_block(T)` — a partial rename is worse than none. Other block reasons (`/cgo_code`, `/parse_error`, `/concurrent_modification` from `codedom_edit.mg:44-70`) are more rules into `plan_blocked`.
4. *Steps come from tiers, not from Go.* `tier_step(Tier, Step)` facts give the order the tool applies rows in (definition, then implementers and callers, then tests); rows with the same step are independent and the tool may apply them in any order. There is no per-row ordering because Mangle has none (M0 §6) and none is needed. A ref that is a site in two tiers (an implementer that also calls the interface method — `mock.Evaluate` below) is two `plan_site` rows and, when the tiers map to different steps, two `edit_plan` rows; the tool applies the edit once at the earliest step and the later row is already `edit_done_now`.
5. *The gate is a threshold fact over distinct refs.* `blast_radius(T, N)` is `fn:count` over `site_ref(T, Ref) :- plan_site(T, Change, Op, Ref, Tier)` — a projection, so a ref reached by two tiers or two changes counts once. The first draft counted `plan_site` rows directly, which is `(Ref, Tier)` pairs: 7 for 6 refs in `v_patternE_2tiers.mg`, and the `confirm_threshold` compared against tier-inflated numbers (M3 engine surprise 4: a single-atom aggregation body is not rewritten and counts stored rows, wildcard columns included). `plan_gate(T, /confirm)` when `N >= confirm_threshold(K)`; `/apply` below it; `/blocked` if any block. Go reads one row and either applies, asks, or refuses — it never counts.
6. *Pending is the negation of applied, per operation, at this revision.* `edit_applied(Task, Ref, Op, Rev)` is the witness the tool asserts per applied row; `edit_done_now(T, Ref, Op)` requires `current_rev(Rev)`; `edit_step_pending(T, Step) :- edit_plan(T, Step, Ref, File, Op, Spec), !edit_done_now(T, Ref, Op)`. The operation is in the witness because applying the rename at the definition does not apply the signature change there. A rollback or a file change bumps the revision and re-opens the steps — this is the obligation Pattern D consumes.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternE_v3.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternE_v3.mg edit_plan blast_radius plan_gate edit_step_pending plan_site site_ref`):

```mangle
# Pattern E v3 -- blast-radius edits as derived plans; a site belongs to a change,
# the radius counts distinct refs (M3 items 3, 4).
Decl code_element(Ref, Kind, File, StartLine, EndLine) bound [/string, /name, /string, /number, /number].
Decl code_calls(Caller, Callee) bound [/string, /string].
Decl code_implements(Struct, Interface) bound [/string, /string].
Decl method_of(MethodRef, StructRef) bound [/string, /string].
Decl is_test_function(Ref) bound [/string].
Decl generated_file(File) bound [/string].
Decl current_rev(Rev) bound [/string].
Decl chosen_change(Task, Ref, Op, Spec) bound [/string, /string, /name, /string].
Decl edit_applied(Task, Ref, Op, Rev) bound [/string, /string, /name, /string].
Decl confirm_threshold(N) bound [/number].
Decl tier_step(Tier, Step) bound [/name, /number].
Decl plan_site(Task, Change, Op, Ref, Tier) bound [/string, /string, /name, /string, /name].
Decl site_ref(Task, Ref) bound [/string, /string].
Decl edit_plan(Task, Step, Ref, File, Op, Spec) bound [/string, /number, /string, /string, /name, /string].
Decl plan_blocked(Task, Ref, Why) bound [/string, /string, /name].
Decl has_block(Task) bound [/string].
Decl blast_radius(Task, N) bound [/string, /number].
Decl plan_gate(Task, Gate) bound [/string, /name].
Decl edit_done_now(Task, Ref, Op) bound [/string, /string, /name].
Decl edit_step_pending(Task, Step) bound [/string, /number].

confirm_threshold(4).
tier_step(/definition, 1).
tier_step(/implementer, 2).
tier_step(/caller, 2).
tier_step(/test, 3).

code_element("core.Evaluate", /method, "internal/core/kernel_eval.go", 181, 260).
code_element("core.Kernel", /interface, "internal/core/kernel.go", 10, 40).
code_element("core.Kernel.Evaluate", /method, "internal/core/kernel.go", 22, 22).
code_element("mock.Evaluate", /method, "internal/core/mock_kernel.go", 50, 60).
code_element("session.runTurn", /function, "internal/session/executor.go", 900, 990).
code_element("chat.process", /function, "cmd/nerd/chat/process.go", 300, 420).
code_element("core.TestEvaluate", /function, "internal/core/kernel_eval_test.go", 10, 40).
code_element("gen.Evaluate", /function, "internal/gen/zz_generated.go", 1, 9).
method_of("core.Kernel.Evaluate", "core.Kernel").
code_implements("core.RealKernel", "core.Kernel").
code_implements("mock.MockKernel", "core.Kernel").
method_of("core.Evaluate", "core.RealKernel").
method_of("mock.Evaluate", "mock.MockKernel").
code_calls("session.runTurn", "core.Kernel.Evaluate").
code_calls("chat.process", "core.Kernel.Evaluate").
code_calls("core.TestEvaluate", "core.Evaluate").
# mock.Evaluate is both an implementer and a caller (M3 v_patternE_2tiers)
code_calls("mock.Evaluate", "core.Kernel.Evaluate").
is_test_function("core.TestEvaluate").
generated_file("internal/gen/zz_generated.go").

# The model's choices arrive as typed tool calls, never as prose. Two changes on one task.
current_rev("rev-17").
chosen_change("t1", "core.Kernel.Evaluate", /rename, "Fixpoint").
chosen_change("t1", "core.Kernel.Evaluate", /signature, "Evaluate(ctx) error").
edit_applied("t1", "core.Kernel.Evaluate", /rename, "rev-17").

# every site names the change (Change, Op) it belongs to
plan_site(T, Ref, Op, Ref, /definition) :- chosen_change(T, Ref, Op, Spec).
plan_site(T, Ref, /rename, Impl, /implementer) :-
    chosen_change(T, Ref, /rename, Spec), method_of(Ref, Iface),
    code_implements(Struct, Iface), method_of(Impl, Struct).
plan_site(T, Ref, /rename, Caller, /caller) :-
    chosen_change(T, Ref, /rename, Spec), code_calls(Caller, Ref), !is_test_function(Caller).
plan_site(T, Ref, /rename, Caller, /caller) :-
    plan_site(T, Ref, /rename, Impl, /implementer), code_calls(Caller, Impl), !is_test_function(Caller).
plan_site(T, Ref, /rename, Test, /test) :-
    chosen_change(T, Ref, /rename, Spec), code_calls(Test, Ref), is_test_function(Test).
plan_site(T, Ref, /rename, Test, /test) :-
    plan_site(T, Ref, /rename, Impl, /implementer), code_calls(Test, Impl), is_test_function(Test).

plan_blocked(T, Ref, /generated_code) :-
    plan_site(T, Change, Op, Ref, Tier), code_element(Ref, Kind, File, S, E), generated_file(File).
has_block(T) :- plan_blocked(T, Ref, Why).

# the plan joins a site to its own change, so a /signature spec never lands on a rename site
edit_plan(T, Step, Ref, File, Op, Spec) :-
    plan_site(T, Change, Op, Ref, Tier), tier_step(Tier, Step),
    code_element(Ref, Kind, File, S, E), chosen_change(T, Change, Op, Spec), !has_block(T).

# the radius is the number of distinct refs the plan touches, not (ref, tier) pairs
site_ref(T, Ref) :- plan_site(T, Change, Op, Ref, Tier).
blast_radius(T, N) :- site_ref(T, Ref) |> do fn:group_by(T), let N = fn:count().
plan_gate(T, /blocked) :- has_block(T).
plan_gate(T, /confirm) :- blast_radius(T, N), confirm_threshold(K), N >= K, !has_block(T).
plan_gate(T, /apply) :- blast_radius(T, N), confirm_threshold(K), N < K, !has_block(T).

edit_done_now(T, Ref, Op) :- edit_applied(T, Ref, Op, Rev), current_rev(Rev).
edit_step_pending(T, Step) :- edit_plan(T, Step, Ref, File, Op, Spec), !edit_done_now(T, Ref, Op).
```

**Expected derived facts** (verified 2026-09-18 08:52, `mgv.exe -store simple -det=false -runs 5`, identical, and on `-store multi`; `strata=9`):

```
plan_site("t1","core.Kernel.Evaluate",/rename,"core.Kernel.Evaluate",/definition)
plan_site("t1","core.Kernel.Evaluate",/signature,"core.Kernel.Evaluate",/definition)
plan_site("t1","core.Kernel.Evaluate",/rename,"core.Evaluate",/implementer)
plan_site("t1","core.Kernel.Evaluate",/rename,"mock.Evaluate",/implementer)
plan_site("t1","core.Kernel.Evaluate",/rename,"mock.Evaluate",/caller)
plan_site("t1","core.Kernel.Evaluate",/rename,"session.runTurn",/caller)
plan_site("t1","core.Kernel.Evaluate",/rename,"chat.process",/caller)
plan_site("t1","core.Kernel.Evaluate",/rename,"core.TestEvaluate",/test)
site_ref: core.Kernel.Evaluate, core.Evaluate, mock.Evaluate, session.runTurn, chat.process, core.TestEvaluate   (6 rows)
blast_radius("t1",6)
plan_gate("t1",/confirm)
edit_plan("t1",1,"core.Kernel.Evaluate","internal/core/kernel.go",/rename,"Fixpoint")
edit_plan("t1",1,"core.Kernel.Evaluate","internal/core/kernel.go",/signature,"Evaluate(ctx) error")
edit_plan("t1",2,"core.Evaluate","internal/core/kernel_eval.go",/rename,"Fixpoint")
edit_plan("t1",2,"mock.Evaluate","internal/core/mock_kernel.go",/rename,"Fixpoint")
edit_plan("t1",2,"session.runTurn","internal/session/executor.go",/rename,"Fixpoint")
edit_plan("t1",2,"chat.process","cmd/nerd/chat/process.go",/rename,"Fixpoint")
edit_plan("t1",3,"core.TestEvaluate","internal/core/kernel_eval_test.go",/rename,"Fixpoint")
edit_step_pending("t1",1)
edit_step_pending("t1",2)
edit_step_pending("t1",3)
```

Seven `edit_plan` rows for two changes — six rename sites and one signature site — where the first draft produced twelve. `plan_site` has eight rows (`mock.Evaluate` in two tiers) and `blast_radius` is 6, the distinct refs; the first draft's count would have said 8. Step 1 is still pending because only the `/rename` was applied at the definition; the `/signature` row at the same ref is not `edit_done_now`. Adding `code_calls("gen.Evaluate", "core.Kernel.Evaluate")` (`patternE_v3_blocked.mg`, also run): `plan_blocked("t1","gen.Evaluate",/generated_code)`, `plan_gate("t1",/blocked)`, `blast_radius("t1",7)`, and `edit_plan` / `edit_step_pending` are empty — the generated caller blocks the whole plan.

The tool applies step 1's remaining row and step 2's four rows, asserts five `edit_applied(T, Ref, Op, Rev)` witnesses, the CodeDOM refresh bumps nothing (same revision until the harness commits), and step 3 becomes the only pending obligation.

**Where the model stops.** The model sees `edit_plan` rows and `plan_gate` in its window (Pattern B injects them at the tail with reason `/edit_plan`); its legal moves are to confirm, choose a different change, or conclude. It never writes `edit_lines` against a file that has a pending `edit_plan` row — that denial is a `permitted` rule, not a prompt instruction.

## 7. Pattern F — provenance for `nerd why`

**Intent.** Every executive answer — why this atom was injected, why this shard was chosen, why the task is not done — is a derivation the engine already recorded, rendered by a second Mangle program over the proof facts, not a Go tracer that knows four rule names. Today `nerd why` is that tracer: `RealKernel.TraceQuery` names rules from 12 `rule_metadata/2` facts and reconstructs premises for exactly four hard-coded rule names (`internal/core/trace.go:16-49,131,144`; M1 §8 item 2); the real recorder exists (`kernel_provenance.go`, `EnableProvenance`, `Explain`) and is wired only to the TUI `/explain` command (`commands_handlers_misc.go:98-125`, M1 §8 item 1). Answers to "why not" do not exist at all (M1 §8, last paragraph).

**Engine features relied on.** `WithDerivationRecorder` + `provenance.MemoryRecorder` (M0 §8: fires per rule solution, per let/do emission, with premise facts, group keys and input facts); `BuildFromRecording` (memoised, cycle-cut, negation re-checked as closed-world absence → `Absence` leaves); `EmitFacts` schema `proves/2 uses_rule/2 premise/3 edb_leaf/2 absence_leaf/2 binding/3 rule_source/2 uses_transform/2 group_key/3` with facts encoded as lists `[/pred, arg…]` (`provenance/facts.go:47-109`); content-hashed `/proof/…` and `/rule/…` IDs stable across runs (M0 §8, §12 item 6); list-valued columns bound `/any` and positive recursion in the second program.

**Existing predicates it builds on (M1).** `rule_metadata/2` (12 facts; to be deleted, not extended), `EnableProvenance`/`Explain` (`kernel_provenance.go:72`), the `inject/5`, `dropped/3`, `knowledge_stale/2`, `task_incomplete/3`, `stall/2`, `plan_blocked/3` predicates of Patterns A–E, `route_decision/2` and `final_route/2` (`routing_arbitration.mg`, M1 §7 row 14), `hollow_success/1` (`coder_safety.mg:92-107`).

**Verified behaviour of the recorder on the patterns (2026-09-18; re-recorded in pass 2 against Pattern B v2, `C:/Temp/mangle-study/m2v2/provB_v2.txt`).** `mgv.exe -prov patternB_v2.mg inject` produces a proof of `inject("coder-7","safety/no_secrets",/skeleton,/head,100)` (80 recorder events for 11 strata): the `fits` rule with bindings `B=260, P=100, Tie=70`; under it the `inject_prio` **do-aggregate node** with its group key `("coder-7","safety/no_secrets")` and its one contributing `candidate` row (one, because `hit_boost` collapsed the tiers — the first draft's proof showed two of three per-tier rows); the `tie_sum` node whose input facts are the two tier-100 atoms, i.e. what it shares the tier with; and `absence_leaf([/has_above, "coder-7", 100])` — the load-bearing negation "nothing outranks this tier". With the first draft's `above_pair`, "what it beat" was every higher-priority atom; with `above_tier` it is every higher *tier* and its sum, which is the same answer with fewer nodes. `-prov patternB.mg dropped` (first draft, M3 §2F) proves `dropped(...,/budget)` with `absence_leaf([/fits, "coder-7", "context/core.evaluate"])`. So "why was this injected", "what did it beat", and "why was this cut" are one recorder pass each.

**Recorder rule R1 — no wildcards in the premises of rules whose proofs matter.** A `_` in a positive premise drops that premise from the proof: `q(X) :- p(X,_), r(X).` proves `q(1)` with only `r(1)` as a premise (and re-indexes it to 0), while `q(X) :- p(X,Y), r(X).` proves it with `p(1,/a)` and `r(1)` (`prov_wild.mg` vs `prov_named.mg`, both run; re-run by M3 §2F). The loss is transitive: over the first draft's Pattern B the leaves `built_before`, `compile_shard` and `prompt_atom` were all absent from the `inject` proof because every rule reaching them had a wildcard (M3 `v_patternF_B.mg`; the first draft listed `built_before` as present and blamed only `prompt_atom`). Every pattern in this document now names every column (pass 2), so the corpus rule is the same but the examples obey it. (Wildcards in *negated* literals are already forbidden by rule 5 of §1.)

**Recorder rule R2 — `[partial]` is not "leaves were lost" (found in pass 2).** A comparison builtin in the body — `X > 1`, `Score >= Min`, `Total <= B`, `:string:contains(...)` — marks the node `[partial]` while keeping every EDB leaf: `q(X) :- p(X), X > 1.` proves `q(2)` `[partial]` with `p(2)` as its `edb_leaf` (`prov_builtin.mg`, `prov_fn.mg`, both run). The cause is `provenance/recorder.go:233-241` — a builtin atom is an `ast.Atom` premise with no recorded premise fact, and the builder marks any such premise partial and continues (an `Eq` premise such as `Y = fn:plus(X, 1)` is "satisfied by construction", `:264-265`, and does not). Since every threshold rule in Patterns A–E compares, every one of their proofs is `[partial]`; `nerd why` must not render the flag as "incomplete proof" — it should count `edb_leaf` rows under the node against the rule's positive premises and report only a real gap. The first draft attributed the Pattern B `[partial]` markers to `prompt_atom(Atom,_,P,_,_)` alone; with the wildcards gone the `fits` node is still partial, from `:le(Tie,B)`.

**The pattern.**

1. *Record on demand, not always.* The recorder sees every derivation including re-derivations (M0 §8; `engine/seminaivebottomup.go:779` fires before dedup) and the executive kernel re-derives 913 strata over 50–70k facts per evaluation (M1 §9), so a permanently attached recorder is the wrong default. `nerd why <goal>` and the loop's own `/stop_stalled` verdict trigger **one** recorded re-evaluation on a `Clone()` of the live kernel (`kernel_eval.go:694`; the JIT already clones per compile, `factory_adapters.go:79-101`), build the proof of the goal, `EmitFacts` it into a fresh store, and persist that store as `.nerd/why/<session>/<turn>-<goal-hash>.mg` (every emitted row needs its terminating `.` added — `EmitFacts` prints atoms, not clauses; the list-valued facts otherwise parse as Mangle source unchanged, `patternF_v2_B.mg`). Cost: one fixpoint (1.6–2.3 s at 52k facts, M1 §9) per question.
2. *Explain with rules, not Go, and say how deep.* The why-program below derives `why(Fact, RuleText, WitnessFact, Depth)`, `why_direct/3` (depth 1: the rule's own EDB premises — "because of"), `load_bearing_absence(Fact, Missing)` and `why_var(Fact, Var, Value)` from the emitted schema. The depth column is what the first draft lacked: `why/3` mixed the rule's direct premises with every leaf under every competing atom, so `why(inject(...), rule, [/skeleton_category, /identity])` read as "injected because of the identity skeleton", which is false at depth 0 (M3 item 13). `nerd why` prints `why_direct` first, then the transitive rows grouped by depth. Aggregation nodes carry `group_key/3` and `uses_transform/2`, so "which candidates competed" is `premise` rows under a node that `uses_transform`.
3. *"Why not" is a positive fact.* The engine cannot witness a failed derivation (M0 §8), so the design never asks it to: every interesting negative is a derived positive — `dropped(A, Atom, /budget)`, `knowledge_stale(Atom, Rev)`, `knowledge_hit_offlang`, `plan_blocked(T, Ref, Why)`, `task_incomplete(T, Kind, Target)`, `stall(T, Reason)`, `coverage_missing(B, Sym)`, `kernel_invariant_violated(Pred, N)`. "Why is this not done" is `nerd why task_incomplete` and each row has a proof whose leaves are the missing witnesses (`absence_leaf([/acceptance, …])`, `absence_leaf([/test_green_now, …])`). `load_bearing_absence` is per node, so for a decision whose negation sits one rule down (`inject` → `fits` → `!has_above`) the reader walks `premise` rows; a transitive `absence_under(Fact, Missing, Depth)` is the same recursion as `supports` over `absence_leaf` and is left as the obvious extension.
4. *Why delegated.* `final_route(/delegate, Shard)` (M1 §7 row 14, once derived rather than applied in Go) proves through `route_decision/2` to `delegation_candidate/3` to `verb_shard/2` and `perception_confidence/1` leaves (row 10); `nerd why final_route` replaces the glass-box narration `"Route: delegate (verb=…)"` (`process.go:357-365`, M1 §8 item 3) with the derivation.
5. *Proofs are stable and diffable — over rule IDs, not policy text.* `/rule/<hash>` is the hash of the **analysed, rewritten clause** (`Clause.String()` after `RewriteClause` and the `__tmp` rewrite), not of the text in the `.mg` file: negations are moved after their binders, comparisons print as `:ge(Score,Min)` / `:le(Tie,B)`, whitespace is normalised, and an aggregation over a multi-atom body records two rules, `tie_sum(A,P,S) :- tie_sum1__tmp(...) |> do ...` and `tie_sum1__tmp(...) :- inject_prio(...), prompt_atom(...)` (`provB_v2.txt`; M3 `v_prov_reorder.mg`, engine surprise 6). Consequences: a cosmetic edit of the policy file does not change the hash (good); `nerd why` prints a rule the user cannot grep verbatim in the file (so it must print the file and line from a `rule_origin(RuleID, File, Line)` fact the loader asserts when it parses the corpus — the `Clause` carries no position after analysis); and a reorder of two *positive* atoms changes the hash while a reorder that the rewrite would have done anyway does not. `/proof/<hash>` is rule + goal + sub-proofs, so two sessions that injected the same atom for the same reasons produce the same proof ID. `nerd why --diff <a> <b>` is a set difference over `uses_rule`/`edb_leaf` rows — a third small Mangle program over rule IDs, not a diff of policy text.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternF_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternF_v2.mg why why_direct load_bearing_absence supports why_var`). The facts are the verbatim `EmitFacts` output for `q(1)` from `smoke.mg` (`q(X) :- p(X), !n(X).` with `p(1). p(2). n(2).`), produced by `mgv.exe -prov smoke.mg q` (byte-identical to the first draft's, which M3 re-derived on a different driver — the IDs are content hashes):

```mangle
# Pattern F v2 -- a second program over the engine's own proof facts, with depth
# (M3 item 13: why/3 mixed direct premises with transitive support).
Decl proves(P, Fact) bound [/name, /any].
Decl uses_rule(P, R) bound [/name, /name].
Decl rule_source(R, Text) bound [/name, /string].
Decl binding(P, Var, Value) bound [/name, /string, /any].
Decl premise(P, Index, Child) bound [/name, /number, /name].
Decl edb_leaf(P, Fact) bound [/name, /any].
Decl absence_leaf(P, Fact) bound [/name, /any].
Decl supports(P, Leaf, Depth) bound [/name, /name, /number].
Decl why(Fact, RuleText, LeafFact, Depth) bound [/any, /string, /any, /number].
Decl why_direct(Fact, RuleText, LeafFact) bound [/any, /string, /any].
Decl load_bearing_absence(Fact, Missing) bound [/any, /any].
Decl why_var(Fact, Var, Value) bound [/any, /string, /any].

absence_leaf(/proof/eabbf24771a3a8556d98ee61ce03b684, [/n, 1]).
proves(/proof/267d476fa072cc672c7ffc98ef6c9f22, [/q, 1]).
proves(/proof/93d47d7a178da28b7460ba64d2bd87e5, [/p, 1]).
uses_rule(/proof/267d476fa072cc672c7ffc98ef6c9f22, /rule/59f3ea79441f5202ec1c533e902b7ccd).
rule_source(/rule/59f3ea79441f5202ec1c533e902b7ccd, "q(X) :- p(X), !n(X).").
binding(/proof/267d476fa072cc672c7ffc98ef6c9f22, "X", 1).
premise(/proof/267d476fa072cc672c7ffc98ef6c9f22, 0, /proof/93d47d7a178da28b7460ba64d2bd87e5).
premise(/proof/267d476fa072cc672c7ffc98ef6c9f22, 1, /proof/eabbf24771a3a8556d98ee61ce03b684).
edb_leaf(/proof/93d47d7a178da28b7460ba64d2bd87e5, [/p, 1]).

# EDB leaves reachable under a proof node, with the number of premise edges walked.
supports(P, Leaf, 1) :- premise(P, I, Leaf), edb_leaf(Leaf, F).
supports(P, Leaf, D) :- premise(P, I, Q), supports(Q, Leaf, D0), D = fn:plus(D0, 1).

# why(Fact, Rule, Witness, Depth): the rule that fired and every witness fact under it, by depth.
why(Fact, Text, LeafFact, D) :-
    proves(P, Fact), uses_rule(P, R), rule_source(R, Text),
    supports(P, L, D), edb_leaf(L, LeafFact).
# depth 1 is "because of": the rule's own premises. Deeper rows are "what it beat / what fed it".
why_direct(Fact, Text, LeafFact) :- why(Fact, Text, LeafFact, 1).
# The negations that were load-bearing: "this held because that was absent".
load_bearing_absence(Fact, Missing) :-
    proves(P, Fact), premise(P, I, A), absence_leaf(A, Missing).
why_var(Fact, Var, Value) :- proves(P, Fact), binding(P, Var, Value).
```

**Expected derived facts** (verified 2026-09-18 09:25, `mgv.exe -store simple -det=false -runs 5`, identical; `strata=5`):

```
supports(/proof/267d476fa072cc672c7ffc98ef6c9f22,/proof/93d47d7a178da28b7460ba64d2bd87e5,1)
why([/q, 1],"q(X) :- p(X), !n(X).",[/p, 1],1)
why_direct([/q, 1],"q(X) :- p(X), !n(X).",[/p, 1])
load_bearing_absence([/q, 1],[/n, 1])
why_var([/q, 1],"X",1)
```

The recursion through `fn:plus` terminates because a proof is a DAG (the builder is memoised and cycle-cut, M0 §8); a leaf reachable by two paths of different length is two rows, which is the intended reading ("this fact fed the decision at two removes and at four").

**The same program over the Pattern B v2 `inject` proof** (`patternF_v2_B.mg`: the 114 distinct `EmitFacts` rows of `provB_v2.txt` + the rules above; run for `why_direct why load_bearing_absence`): 55 `why` rows and 11 `why_direct` rows in total; for the `inject` fact itself, **depth 1 is exactly `prompt_atom("safety/no_secrets",…)` and `window_slot(/safety,/head)`** — the two EDB premises of the `inject` rule; depth 2 adds `compile_budget`, `compile_shard`, `skeleton_category` and the atom's `prompt_atom` again (through `fits` → `inject_prio` → `candidate`); depths 3, 4 and 6 are the *other* tier-100 atom's leaves reached through `tie_sum` — "what shares the tier". The first draft's list ("`knowledge_hit`, `knowledge_boost`, `compile_budget`, `built_before`, `symbol_atom`, `task_focus_symbol`, `prompt_atom` (once R1 is applied)") was for a different `inject` row and, as M3 item 13 found, wrong on `built_before` and `compile_shard` (lost to wildcards) and silent on `skeleton_category`/`window_slot`; with the wildcards gone every leaf the rules touch is present and the depth column says which are direct. `load_bearing_absence` has one row here and it is on `fits`, not `inject` (`[/has_above, "coder-7", 100]`), per rule 3.

**Engine limits carried into the design.** No proofs for facts produced by `deferred` or external callbacks (M0 §8) — the mounted `*_all` rows of Pattern A are EDB leaves, which is the right granularity ("the vector index said so at rev-17"). No proofs across evaluations — hence persist per question. `premise` indexes shift when a wildcard premise is dropped (R1) and follow the *rewritten* premise order, not the file's (M3 `v_prov_reorder.mg`: `!r` is index 2 after the rewrite moved it) — a consumer must read `premise(P, I, Child)` by `Child`, never by position in the source rule. A string literal containing a quote round-trips escaped in `rule_source` (`v_prov_quote.mg`).

## 8. Pattern G — verdicts as facts

**Intent.** A shard's result is a set of typed witnesses — what tools ran and how they exited, what was written at which revision, what the build and the test runner reported, what findings were filed — plus, separately, what the model *claimed*. Outcome and stage are derived: `verdict(Task, Outcome, Stage)`, `unverified_claim(Task, Claim)`. Today the status is guessed from prose: `injectShardResultFacts` (`cmd/nerd/chat/process_continuation.go:167-222`, M1 §7 row 4) maps `TODO`/`FIXME` substrings to `/incomplete`, "no word 'test' in coder output" to `/code_generated`, "'issue' in reviewer output" to `pending_review`, and writes `shard_result(TaskID, Status, ShardType, Desc, Summary)` (`schemas_misc.mg:188`); `codedom_continuation.mg:8-26` derives next actions from that guessed atom, two of its rules waiting for `/tests_needed`/`/review_needed` values no producer emits (M1 §5 item 10). At the turn level the design already exists — `turn_evidence/6` measured, `hollow_success/1` derived (`coder_safety.mg:79-107`) — but Go re-implements the four rules as a fallback and adds a fifth ("tools errored and the response is empty") before the kernel is consulted (`executor.go:1093-1095, 2509-2555`; M1 §7 rows 5–6), and acceptance is downgraded to `unverified` on snapshot drift in Go (`change_evidence.go:62-107`, row 7).

**Engine features relied on.** Negation ladders over bound projections for the stage (M0 §4); `fn:count` for findings (M0 §6); `!=` on name constants (M0 §4); revision joins. **Grammar trap T1 (found here, widened by M3 item 22):** a clause that *ends* in a bare name constant swallows the terminating period — `q(X) :- p(X), X != /b.` is `PARSE ERROR: missing '.' at '<EOF>'` because `.` is a legal name character (`Mangle.g4:186-187`, M0 §2), while `X != /b .` parses (`trailing_name.mg`, `trailing_name_space.mg`, both run). The verifier showed it is not an end-of-file artefact: the same clause followed by another clause fails with `missing '.' at 'r'` (`v_trailing_multi.mg`), `X = /b.` fails the same way (`v_trailing_eq.mg`), a number in that position is fine (`X != 1.`, `v_trailing_num.mg`), and a line break before the period is a legal escape (`v_trailing_fact.mg`). The lint pattern is therefore "a name constant immediately followed by `.` at the end of a clause, after `=` or `!=`"; the shipped corpus is clean today (grep of `internal/core/defaults/**/*.mg` for `(=|!=) */name.` at end of line: 0 hits, M3 §2G). End such clauses with a positive atom, as every rule below does, or put a space before the period.

**Existing predicates it builds on (M1).** `turn_evidence/6`, `turn_acceptance/3`, `turn_created_source/1`, `hollow_success/1`, `turn_executed/1`, `turn_done/1` (`coder_safety.mg:79-115`, `schemas_execution.mg:245`); `shard_result/5`, `pending_test/2`, `pending_review/2` (`schemas_misc.mg:188-194`); `build_state/1` (`schemas_shards.mg:238`), `test_state/1`, `test_output/1` (`virtual_store_actions.go:453-455`), `failing_test/2`, `test_result/4` (declared, unproduced); `review_finding` in the model-observation allowlist (`mangle_updates.go:30-53`); `successful_edit/2`, `failed_edit/2` (`codedom_edit.mg:151-154`); the executor's `SuccessfulWriteTools` and tool-error bookkeeping (M1 §7 row 4, "Facts needed").

**The pattern** (revised in pass 2 — M3 §5 items 7, 10, 17, 22; the program below is `C:/Temp/mangle-study/m2v2/patternG_v2.mg`, run on the verifier's harness 2026-09-18).

1. *Witnesses are measured, revision-stamped, per task.* `shard_tool(T, Round, Tool, Outcome)`, `shard_write(T, File, Rev)`, `build_run(T, Rev, Outcome)`, `test_run(T, TestRef, Rev, Outcome)` — **the same `test_run/4` Pattern C reads** (the first draft had a `/3` in C and a `/4` here and called them one witness; they were two predicates), `review_finding(T, Severity, Ref)`. Each is produced by the harness from the tool trace, never from the response text. `test_state(/passing)` and `build_state(/failing)` are replaced, not kept beside.
2. *Claims are witnesses of claims, and claim rules are shard-typed.* `shard_claim(T, Claim)` records what the control packet asserted (`/tests_pass`, `/done`, `/wrote`). It is admissible evidence of nothing; `unverified_claim(T, C)` derives when a claim has no matching evidence *for that shard type*, and Pattern D treats an `unverified_claim` as an obligation kind (`/hollow`). A coder's `/done` is verified by `stage(/verified)` or `stage(/reran)`; a reviewer's `/done` is verified by findings or by a review tool having run — the first draft's `!stage(T, /verified)` was defined for coders only, so every reviewer `/done` was hollow (M3 `v_patternG_more.mg` t4). This is the existing rule "response presents test-runner output but no test-execution tool ran" (`coder_safety.mg:100-102`) generalised to every claim the packet can make. Model-asserted `review_finding` rows (allowed today by `ModelObservationPolicy`) become `review_claim` rows; a finding is a fact only when its `Ref` joins `code_element/5` — the same shape discipline as `agents.md:24-31`.
3. *Evidence kinds are rules at the current revision, and a failure at that revision defeats a pass.* `evidence(T, /wrote)`, `/build_green` (green and no `has_build_failure` at the revision), `/tests_green` (green and no `has_test_failure`), `/findings`. The first draft guarded tests but not builds, so a `/fail` then a `/pass` at one revision was `/build_green` (M3 t7). A revision is a workspace snapshot (`change_evidence.go:62-107`), so a fix between two builds changes it; two outcomes at *one* revision is nondeterminism, and nondeterminism is not green. A green build at the previous revision is not `/build_green` (task t5 below).
4. *Stage is a ladder of negations with a verification-only rung.* `/verified` ⊃ `/built` ⊃ `/edited` for tasks that wrote; `/reran` for a coder task that wrote nothing and has green tests at the revision — the shape of a Pattern C `/rerun` obligation being discharged; `/unstarted` for no write and no green tests. Exactly one stage per coder task, by construction. The first draft keyed the whole ladder on `/wrote`, so a task that did exactly what Pattern C asked was `/nothing_done` (M3 t6) — the two patterns contradicted each other.
5. *The verdict is one derived row and the old status is a view.* `verdict/3` for coder and reviewer shards (`/verified` carries `/verified` or `/reran` as its stage); `shard_status/2` derives the legacy `Status` atoms (`/complete`, `/code_generated`, `/failed`, `/review_needed`) so `codedom_continuation.mg` keeps working while its rules are repointed — and its two dead rules start firing, because `/review_needed` now has a producer.

**Runnable example** (`C:/Temp/mangle-study/m2v2/patternG_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternG_v2.mg verdict stage evidence unverified_claim shard_status has_build_failure`):

```mangle
# Pattern G v2 -- verdicts as facts: the shard result is typed evidence, success is derived
# (M3 items 7, 10, 17: build failures guard build_green, a tests-only coder task is a verdict,
# claim rules are shard-typed, test_run/4 is the one witness shared with Pattern C).
Decl shard_run(Task, Shard, Rev) bound [/string, /name, /string].
Decl current_rev(Rev) bound [/string].
Decl shard_tool(Task, Round, Tool, Outcome) bound [/string, /number, /name, /name].
Decl shard_write(Task, File, Rev) bound [/string, /string, /string].
Decl build_run(Task, Rev, Outcome) bound [/string, /string, /name].
Decl test_run(Task, TestRef, Rev, Outcome) bound [/string, /string, /string, /name].
Decl review_finding(Task, Severity, Ref) bound [/string, /name, /string].
Decl shard_claim(Task, Claim) bound [/string, /name].
Decl evidence(Task, Kind) bound [/string, /name].
Decl has_tool_error(Task) bound [/string].
Decl has_test_failure(Task) bound [/string].
Decl has_build_failure(Task) bound [/string].
Decl reviewed_with_tool(Task) bound [/string].
Decl finding_count(Task, N) bound [/string, /number].
Decl stage(Task, Stage) bound [/string, /name].
Decl unverified_claim(Task, Claim) bound [/string, /name].
Decl verdict(Task, Outcome, Stage) bound [/string, /name, /name].
Decl shard_status(Task, Status) bound [/string, /name].

current_rev("rev-18").
# t1: coder that wrote, built, tested at the current revision
shard_run("t1", /coder, "rev-18").
shard_tool("t1", 1, /read_file, /ok). shard_tool("t1", 2, /edit_element, /ok).
shard_tool("t1", 3, /build, /ok). shard_tool("t1", 4, /run_tests, /ok).
shard_write("t1", "internal/core/kernel_eval.go", "rev-18").
shard_write("t1", "internal/core/kernel_eval_test.go", "rev-18").
build_run("t1", "rev-18", /pass).
test_run("t1", "core.TestEvaluateFresh", "rev-18", /pass).
shard_claim("t1", /tests_pass).
# t2: coder that wrote and claims tests pass, but never ran them
shard_run("t2", /coder, "rev-18").
shard_tool("t2", 1, /edit_element, /ok).
shard_write("t2", "internal/session/executor.go", "rev-18").
shard_claim("t2", /tests_pass).
shard_claim("t2", /done).
# t3: coder whose only tool calls failed and who wrote nothing
shard_run("t3", /coder, "rev-18").
shard_tool("t3", 1, /edit_element, /error). shard_tool("t3", 2, /edit_element, /error).
shard_claim("t3", /done).
# t4: reviewer with findings, claiming done
shard_run("t4", /reviewer, "rev-18").
review_finding("t4", /high, "core.assert").
review_finding("t4", /low, "core.evaluate").
shard_claim("t4", /done).
# t5: coder whose green build is one revision old
shard_run("t5", /coder, "rev-18").
shard_write("t5", "internal/core/kernel_facts.go", "rev-17").
build_run("t5", "rev-17", /pass).
# t6: coder that only re-ran tests (a Pattern C /rerun obligation) and wrote nothing
shard_run("t6", /coder, "rev-18").
shard_tool("t6", 1, /run_tests, /ok).
test_run("t6", "core.TestEvaluateFresh", "rev-18", /pass).
shard_claim("t6", /done).
# t7: two builds at the current revision, one failed then one passed
shard_run("t7", /coder, "rev-18").
shard_write("t7", "internal/core/kernel_eval.go", "rev-18").
build_run("t7", "rev-18", /fail).
build_run("t7", "rev-18", /pass).

# evidence: each kind is a rule over measured witnesses at the current revision;
# a failure at the revision defeats a pass at the same revision, for builds and tests alike
evidence(T, /wrote) :- shard_write(T, File, Rev), current_rev(Rev).
evidence(T, /build_green) :- build_run(T, Rev, /pass), current_rev(Rev), !has_build_failure(T).
evidence(T, /tests_green) :- test_run(T, Test, Rev, /pass), current_rev(Rev), !has_test_failure(T).
has_build_failure(T) :- build_run(T, Rev, /fail), current_rev(Rev).
has_test_failure(T) :- test_run(T, Test, Rev, /fail), current_rev(Rev).
has_tool_error(T) :- shard_tool(T, Round, Tool, /error).
reviewed_with_tool(T) :- shard_tool(T, Round, /review, /ok).
finding_count(T, N) :- review_finding(T, Sev, Ref) |> do fn:group_by(T), let N = fn:count().
evidence(T, /findings) :- finding_count(T, N), N > 0.

# stage: the furthest evidence reached, as a ladder of negations; one stage per coder task
stage(T, /verified) :- evidence(T, /wrote), evidence(T, /build_green), evidence(T, /tests_green).
stage(T, /built) :- evidence(T, /wrote), evidence(T, /build_green), !evidence(T, /tests_green).
stage(T, /edited) :- evidence(T, /wrote), !evidence(T, /build_green).
stage(T, /reran) :- shard_run(T, /coder, Rev), !evidence(T, /wrote), evidence(T, /tests_green).
stage(T, /unstarted) :- shard_run(T, /coder, Rev), !evidence(T, /wrote), !evidence(T, /tests_green).

# a claim without its evidence is a fact, never a guess about prose; claim rules are shard-typed
unverified_claim(T, /tests_pass) :- shard_claim(T, /tests_pass), shard_run(T, /coder, Rev), !evidence(T, /tests_green).
unverified_claim(T, /done) :- shard_claim(T, /done), shard_run(T, /coder, Rev), !stage(T, /verified), !stage(T, /reran).
unverified_claim(T, /done) :- shard_claim(T, /done), shard_run(T, /reviewer, Rev), !evidence(T, /findings), !reviewed_with_tool(T).

# the verdict: one row per task, derived, stage-carrying
verdict(T, /verified, /verified) :- stage(T, /verified).
verdict(T, /verified, /reran) :- stage(T, /reran).
verdict(T, /incomplete, S) :- stage(T, S), S != /verified, S != /reran, S != /unstarted, shard_run(T, /coder, Rev).
verdict(T, /failed, /unstarted) :- stage(T, /unstarted), has_tool_error(T).
verdict(T, /nothing_done, /unstarted) :- stage(T, /unstarted), !has_tool_error(T).
verdict(T, /review_findings, /reviewed) :- shard_run(T, /reviewer, Rev), evidence(T, /findings).
verdict(T, /review_clean, /reviewed) :- shard_run(T, /reviewer, Rev), !evidence(T, /findings).

# what codedom_continuation.mg consumes today as shard_result's guessed Status
shard_status(T, /complete) :- verdict(T, /verified, S).
shard_status(T, /code_generated) :- verdict(T, /incomplete, S).
shard_status(T, /failed) :- verdict(T, /failed, S).
shard_status(T, /review_needed) :- verdict(T, /review_findings, S).
```

**Expected derived facts** (verified 2026-09-18 09:00, `mgv.exe -store simple -det=false -runs 5`, identical; `strata=10`):

```
evidence("t1",/wrote)  evidence("t1",/build_green)  evidence("t1",/tests_green)
evidence("t2",/wrote)  evidence("t4",/findings)  evidence("t6",/tests_green)  evidence("t7",/wrote)
has_build_failure("t7")
stage("t1",/verified)  stage("t2",/edited)  stage("t3",/unstarted)  stage("t5",/unstarted)  stage("t6",/reran)  stage("t7",/edited)
unverified_claim("t2",/tests_pass)  unverified_claim("t2",/done)  unverified_claim("t3",/done)
verdict("t1",/verified,/verified)
verdict("t2",/incomplete,/edited)
verdict("t3",/failed,/unstarted)
verdict("t4",/review_findings,/reviewed)
verdict("t5",/nothing_done,/unstarted)
verdict("t6",/verified,/reran)
verdict("t7",/incomplete,/edited)
shard_status("t1",/complete)  shard_status("t2",/code_generated)  shard_status("t3",/failed)
shard_status("t4",/review_needed)  shard_status("t6",/complete)  shard_status("t7",/code_generated)
```

t2 claimed `tests_pass` and `done` with one edit and no test run: two `unverified_claim` rows and an `/incomplete` verdict at stage `/edited`, without any substring test. t4 claimed `/done` with findings on file: not hollow (the first draft said it was). t5's write and green build were at `rev-17`; at `rev-18` it has done nothing — the revision join, not a Go snapshot comparison (`change_evidence.go:62-107`), produced the downgrade. t6 re-ran the tests it was asked to re-run and is `/verified` at stage `/reran`, which is what Pattern C's `/rerun` obligation being cleared looks like from the verdict side (the first draft said `/nothing_done`). t7's failed-then-passed build at one revision is `/edited`, not `/built` (the first draft said `/built`). t3's `/done` claim over two failed tools is `/failed`, which is M1 §7 row 5's "conservative heuristic" as a rule. Exactly one `verdict` row per task in all seven cases.

**Consumers.** Pattern D's `obligation(T, /hollow, …)` reads `unverified_claim`; `codedom_continuation.mg` reads `shard_status`; the chat loop reads `verdict` for the user-facing trailer that `appendEvidenceReport` composes today (row 7) — the stage column is the "Evidence: <stage>" it prints. `checkHollowSuccess`'s Go fallback and the read-only exemption (`executor_tools.go:1923-1977`, row 6) become `read_only_verb/1` facts and one more `verdict` rule; the `MockKernel` argument for keeping the fallback (`executor.go:2505-2508`) is answered by testing against a real kernel (§1).

## 9. Migration map (M1 section 7 → patterns)

Row numbers are M1 §7's. "Seam" names what must change for the decision to move: the Go type that stops deciding, the Decls that must exist, and the callers that read one row instead. Every row follows the no-shim rule (root `CLAUDE.md`): the Go decision is deleted in the same change that lands the rule and its end-to-end kernel test (§1).

| M1 row | Go decision today | Pattern | Seam: types | Seam: Decls | Seam: callers |
|---|---|---|---|---|---|
| 1 | `shouldAutoClarify` substrings + intent-shape test (`process_dream_delegation.go:23-44`) | B (reason `/clarify` is an inject reason) + D (a missing target is an obligation `/clarify`) | delete `shouldAutoClarify`; Go emits `campaign_signal(/keyword)` only (row 15 boundary) | `clarify_reason(Reason)`; `route_decision(/clarify, /none)` rule in `routing_arbitration.mg` | `process.go:369` reads `final_route/2` (row 14) |
| 2 | `shouldClarifyFromKernel` + Go fallback `shouldClarifyIntent` (`process.go:392-408`) | same as row 1 | delete the fallback | none new (`delegation_candidate/3` carries confidence) | same |
| 3 | `formatShardTask` rewrites the brief (`delegation.go:311-375`) | B: the shard's window is `inject` rows for `Agent = <shard task>`; the brief is `task_text` + `task_scope_file` rows | `formatShardTask` becomes a renderer of `inject` rows | `delegated_task(TaskID, Shard, Verb, Target, Constraint)`, `task_scope_file(TaskID, Path)`, `task_query/2`, `task_focus_symbol/2` (Pattern A/B) | `formatShardTaskWithContext` callers pass the task ID |
| 4 | `injectShardResultFacts` guesses status from prose (`process_continuation.go:167-222`) | G | delete the substring tests; `injectShardResultFacts` asserts `shard_tool/4`, `shard_write/3`, `build_run/3`, `test_run/4`, `review_finding/3`, `shard_claim/2` | Pattern G Decls; `shard_status/2` as the view over `verdict/3` | `codedom_continuation.mg:8-26` joins `shard_status`; delete `shard_result/5` and `pending_test/2`/`pending_review/2` writers |
| 5 | "tools errored and response empty → failed" before the kernel (`executor.go:1093-1095`) | G (`verdict(T, /failed, /unstarted)`) + D | delete the branch | `shard_tool/4` outcome column; `turn_response_empty(Verb)` if the empty-response case is wanted | executor reads `verdict/3` |
| 6 | `consumeHollowSuccessVerdict` switch on reason strings + imperative fallback + read-only exemption (`executor.go:2432-2557`, `executor_tools.go:1923-1977`) | G | delete the fallback and the `requiresTools` switch | `read_only_verb(Verb)` facts; `turn_failed(Verb)` rule | executor reads `turn_failed/1`, `verdict/3` |
| 7 | acceptance downgraded on snapshot drift; trailer composed in Go (`change_evidence.go:62-107`) | G (`current_rev` join) + D (`/acceptance` obligation) | `appendEvidenceReport` prints `verdict` rows | `acceptance(Task, Rev)`, `current_rev(Rev)` (one row, replaced per turn via `Transaction()`) | executor asserts acceptance at the rev it verified; never downgrades |
| 8 | adaptive tool-budget extension, loop detection, verification-tool list (`tool_budget_controller.go:192-302`) | D | delete `toolBudgetController`'s decision fields (`hardLimit`, `maxExtensions`, `extensionSize`, `iterationLimit`, `:21,79-96`); keep the counters as witness producers | `round_done/2`, `open_count_at/3`, `tool_failure/3`, `spent/2`; policy facts `stall_rounds/1`, `repeated_failure_rounds/1` (a window over the last K rounds, not a lifetime count — §5 rule 3), `spend_ceiling/2`; `verification_tool(Name)` facts for `working_nudge(/verify)` | the loop reads `loop_verdict/2`; `working_set.mg`'s `working_stop`/`working_nudge`/`working_finalize` move from the private scope to the executive kernel or stay private and feed `stall/2` |
| 9 | `planTurnSteps` decides planned vs unplanned, drops < 2 steps, retries once (`work_steps.go:157-400`) | E | the LLM's step plan is asserted as `chosen_change/4` rows (typed), not text; `runPlannedSteps` applies `edit_plan` rows | Pattern E Decls; `turn_shape(/planned)` rule over `edit_plan` count | `work_steps.go` reads `edit_plan/6`, `plan_gate/2`, `edit_step_pending/2` |
| 10 | verb→shard table in Go, LLM `shard=` hint, substring fallback (`delegation_routing.go:205-227`, `transducer.go:481-489`) | B (delegation is `inject` of a task to an agent) + F (`nerd why final_route`) | delete `resolveShardTypeForIntent`; Go emits `target_scope(/whole_repo)` signal only | add `verb_def` to `defaultIntentFactPredicates()` (`intent_defaults.go:27`) so `verb_def/4` has rows in the executive kernel; `shard_mutates(Shard)` facts; `delegation_candidate/3` rules | `decideRoute` reads `final_route/2` |
| 11 | clarifier fired from Go heuristics; kernel's `next_action(/interrogative_mode)` ignored on the chat path (`process_seed.go:147-162`) | D (an open `/clarify` obligation) | delete the Go triggers; `runClarifierShard` is the effect of `next_action(/interrogative_mode)` | none new | `process.go` executes `next_action` rows like the system-shard path does (`executive_intent.go:252-261`) |
| 12 | JIT mandatory flags forced in Go, budget fitting in Go (`compiler.go:1000-1016,1099,1148`, `budget.go`) | B | delete `TokenBudgetManager` fitting and the forced `IsMandatory`; `selector.go` reads `inject/5` | `compile_budget/2`, `window_slot/2`, `knowledge_boost/2`, `inject/5`, `dropped/3`; `prompt_atom` rows for dynamic `kernel/*` atoms | `selector.go:977-1010,1232`; `compressor.go:688` reads `working_selected`/`inject` |
| 13 | `intentRequiresReasoningModel` + `isConsultIntentVerb` short-circuit (`executor.go:532-566`) | already derived; add `consult_verb(V)` facts | delete the short-circuit | `consult_verb/1` | unchanged |
| 14 | lane precedence in `decideRoute` (`delegation_routing.go:152-164`) | B/F | delete the precedence code | `final_route/2` rules in `routing_arbitration.mg` (precedence as negations) | Go reads exactly one `final_route` row |
| 15 | signal extraction (`multiStepSignals`) | stays in Go by design (§1) | — | — | — |
| 16 | predicate-name special cases in the kernel (`system_heartbeat` upsert, numeric sanitiser, `jitPredicates` log map) | A (Decl-level annotation) | `IsEphemeral` and the upsert list read a Decl descriptor | `Decl … descr [doc(…), upsert()]` — **verified** (M3 §3 row 15, `m0_upsert.mg`): a Decl with `upsert()` and `ephemeral()` descriptors parses, analyses and evaluates end to end; `analysis/declcheck.go:87-88` ignores unknown descriptors. The flip side is that a typo (`upsrt()`) is never reported, so the host must list the descriptors it reads and reject the rest at load | `kernel_facts.go:566,1204-1305` |
| 17 | three model-assertion allowlists in Go (`mangle_updates.go:30-53`, `planner.go:1071-1091`, `perception.go:283-289`) | G (claims are witnesses) + F | `FilterMangleUpdates` queries the kernel | `model_may_assert(Surface, Pred)` facts; `host_witness(Pred)` facts; rule `mangle_update_allowed(S, P)` | the three callers pass their surface name |

**Cross-cutting seams that every row shares.**

- *Revision.* `current_rev/1` (one row — enforced by the `current_rev_count` / `kernel_invariant_violated(/current_rev, N)` rule of §5, because nothing else enforces it and the first composition of the seven patterns had two rows and silently widened every revision join, M3 item 16) and a `Rev` column on every witness. Producer: the executor's workspace snapshot (`change_evidence.go`) — today computed and compared in Go, tomorrow asserted and joined.
- *Identifier shape.* Every `Ref`/`Sym` column is the Cartographer ref (`agents.md:27-31`). `plan_edit` and `modified_function` are fixed or deleted with their producers, not worked around. The string-vs-name half of this class has a concrete cause in codeNERD: `types.Fact.ToAtom` (`internal/types/types.go:246-253`) turns a Go string with a leading `/`, at most two slashes and no file extension into a **name** constant, so a path witness can land as `/etc/passwd` (name) while the rule literal is `"/etc/passwd"` (string) — M3 §3, resolving M0 open item (a); `coerceAtomToDeclLocked` never coerces string↔name. Path-carrying witnesses in these patterns (`shard_write`, `code_element` File columns, `generated_file`) must be asserted as `ast.String` explicitly or the Decl's `/string` bound must direct the coercion; the `agents.md:53-89` recipe (join on a variable seeded from Go) works only because both sides pass through the same heuristic.
- *One witness shape per name.* `test_run(Task, TestRef, Rev, Outcome)` is the only `test_run`; `test_obligation(Task, B, Target, Kind)` the only `test_obligation`; `unverified_claim(Task, Claim)` the only claim verdict. Two arities under one name load without error and leave one consumer silently empty (M3 item 17; the first draft had `test_run/3` in C and `/4` in G).
- *No wildcards in decision rules* (R1, §7), *no wildcards in any `do`-transform body* (§1 convention 6), and *no clause-final bare name constants after `=` or `!=`* (T1, §8, widened by M3 item 22: mid-file too, `=` too, numbers are fine, a newline before the period is a legal escape) — lints in the analyzer gate alongside the dropped-negation check, which must compare `NegAtom` counts per clause before and after analysis rather than look for `_` (M0 §12 item 1; M3 item 21). All four are host-side: the engine reports none of them.
- *Externals.* Until E1 (§2) is fixed upstream, every mount is all-output; `+`-moded Decls stay declared but no rule calls them (as today). Every all-output mount's adapter (`external_predicates.go#virtualExternalPredicate`) answers `ShouldQuery` once per evaluation and `false` after, so an empty source costs one call, not one per solution row (§2 cost note, M3 item 11). When E1 is fixed the analyzer gate also checks that a `+`-moded external is written after every binder of its input positions, because analysis does not (M3 engine surprise 2).
- *Tests.* One end-to-end kernel test per row, asserting through `Assert` and querying the derived predicate (§1); `TestStarvedPredicateBudget` (`starved_predicate_test.go:185`) is updated in the same change so the new producers are counted.

## 10. Token economy

Tokens are the constraint, not the goal (root `CLAUDE.md`, Vision). For each pattern: what leaves the model's window or the turn count because the kernel does it, and how the change is measured with what exists — `nerd meter` over `.nerd/meter/receipts.jsonl` (one receipt per metered inference call: `purpose`, `estimated.segments{system,history,user,tools}`, `actual.input_tokens/output_tokens`, `decision`; writer `internal/broker/filesink.go:17`, reader `cmd/nerd/cmd_meter.go:95-130`), `nerd meter epochs` (calls per epoch, `:331`), `nerd meter atoms` (atom co-use from `.nerd/meter/atom-selections.jsonl`, `:444`; writer `internal/prompt/couse_persist.go:18`), and the kernel/performance logs (M1 §9). The session measured in M1 §9 is the baseline: 3,084 receipts, 155 JIT selections, 127 fixpoints, 106 s of evaluation in ~15 minutes.

| Pattern | Leaves the window / the turn count | Measure |
|---|---|---|
| A — mounted knowledge | The model no longer asks for knowledge: no `recall`/`search`/`read` round to find the doc for the focal symbol, no re-reading a file the kernel knows is unchanged (the `working_superseded` observation, `working_set.mg:103-123`, generalised by the revision join). Retrieval turns become zero-turn EDB rows. | `nerd meter epochs`: calls per epoch for `purpose=shard`/`coder` before vs after; receipts with `segments.tools` dominated by read/recall tools; kernel log `populating store with N EDB facts` should *fall* once `code_*` rows are mounted instead of asserted (M1 §9: 50–70k today). |
| B — derived injection | Every atom in the window has a derived reason; `dropped` rows show what the budget cut. The mandatory set shrinks from 36.7% flagged (M1 §7 row 12) to the skeleton categories; knowledge enters by hit score, not by category percentage. Lost-in-the-middle: the request-adjacent tail holds the focal context. | `nerd meter atoms --top 30`: co-use and per-atom frequency; `estimated.segments.system` per receipt (should drop and stabilise); `inject` row count per compile in the manifest (`compiler.go:1581`); the `dropped` rows are the direct measure of budget pressure. |
| C — test obligations | The model is told *which* test to write or run (`test_obligation` rows injected at the tail) instead of being told "write tests" in a methodology atom and deciding; the tester shard's discovery turns ("what tests exist for this?") vanish. | receipts with `purpose=tester`; turns per task that end in `/rerun` obligations cleared; `coverage_missing` count over time in the kernel log. |
| D — obligation fixpoint | No iteration cap means no wasted "budget exhausted" restarts and no premature stop with obligations open; `stall(/repeated_failure)` stops the loop after `K` identical failures rather than after `N` rounds of anything. The nudge text is the only thing added to the window. | `loop_verdict` distribution per task (`/complete` vs `/stop_stalled`); `spent` vs `spend_ceiling`; `nerd meter` spend per purpose per task; rounds per task from `round_done`. |
| E — derived edit plans | A rename over N sites is one typed tool call plus N deterministic edits, not N model-authored `edit_lines` calls with N file reads before them (M1 §7 row 8 observed 12-site changes at one edit per 8-round cycle, `working_set.mg:66-70`). Output tokens fall by the size of the code the model no longer retypes. | receipts `actual.output_tokens` per task for `/rename`-class changes; `blast_radius` vs `edit_applied` rows; `nerd meter epochs` calls per task. |
| F — provenance | Nothing in the window; `nerd why` costs one recorded fixpoint on a clone (1.6–2.3 s at 52k facts, M1 §9) and zero inference. The glass-box narration strings leave the transcript. | `performance.log` `kernel.evaluate.fixpoint` entries tagged with the why-goal; receipts should show no `purpose=explain` inference. |
| G — verdicts as facts | No "summarise what the shard did" inference; the trailer is `verdict` rows. No re-run of a shard whose verdict was mis-guessed from prose (`/complete` on a `TODO`-free but untested change). | receipts per task after a shard returns (should be 0 for verdict formation); `unverified_claim` counts — a rising count is the model learning it cannot claim its way out. |

**Kernel cost is part of the economy.** Every pattern adds strata, and **strata add linearly** unless one pattern's aggregation consumes another's derived predicate — the verifier composed the first draft's seven files as one unit and got `strata=52` = 4+9+9+10+8+4+8 exactly (M3 item 15, `v_compose_dedup.mg`); the pass-2 programs compose to `strata=61` = 4+11+9+13+9+5+10 (`C:/Temp/mangle-study/m2v2/compose_v2.mg`, 0.34 s including process start on the toy data, with D reading G's `unverified_claim` and C's `test_obligation` across the seam as intended). The first draft's "they share predicates and will not add linearly" was wrong: sharing an EDB predicate does not merge strata; only a shared *aggregation* would. Sixty-one strata on top of a 913-stratum program that is re-derived from scratch on every dirty query (M1 §1.3, §9). The measure is the kernel log's `fixpoint reached` durations (`evalstats.py` in `C:/Temp/mangle-study/`) and the number of evaluations per turn; the JIT's per-compile clone-and-fixpoint (19 × ~2 s in the baseline session, M1 §9) is the first thing Pattern B's single-query consumer should remove, by compiling against the live kernel's already-derived `inject` rows instead of a clone.

## 11. Open risks

1. **Stratification.** Each `|> do fn:group_by` is its own stratum and recursion through aggregation is rejected at `Stratify`, *after* `AnalyzeOneUnit` succeeds (M0 §4, §6 probe K) — so a bad rule passes the boot-time analyzer and fails the first evaluation. Pattern D's `open_count_at` deliberately makes progress a *witness* rather than an aggregate over the previous fixpoint's `obligation` rows precisely to avoid `obligation → count → stall → obligation` cycles; Pattern B's `inject_prio → above_sum → fits → inject` is acyclic only because `candidate` never reads `inject`. Rule for the corpus: a decision predicate never feeds a rule that feeds its own aggregation; the `shard_join_audit.py` (M1 §5 item 9) gains a check that the aggregation dependency graph of the decision predicates is a DAG. Also: `WithDeterministicOrder` sorts predicates, not solutions (M0 §7) — any `fn:collect` in a decision predicate is nondeterministic and must not be compared or persisted.
2. **Evaluation cost at fact ceilings.** The executive kernel evaluates 913 strata over 50–70k facts in 75 ms median / 1.9 s p90 (M1 §9) with no incremental path in production (M1 §1.3; the engine has none, M0 §7). The patterns add aggregations, and aggregation is evaluated after the stratum's fixpoint over the *full* `__tmp` relation (M0 §6). The first draft's `above_pair` in Pattern B was quadratic in candidates per agent, and the verifier measured it (M3 item 20, engine surprise 11): 0.46 / 1.5 / 5.8 s at 200 / 400 / 800 candidates on `MultiIndexedArrayInMemoryStore` (0.67 / 1.7 / 10.2 s on `SimpleInMemoryStore`), and 100 800 created facts at 400 — over a 100 000 limit, and codeNERD's `defaultDerivedFactLimit` is 500 000 (`kernel_init.go:188`) with a 351-atom library. Pass 2 replaced it with the tier prefix sum (§3 rule 3): quadratic in distinct priorities, linear in atoms, same derived sets, 0.14–0.34 s at 800 and 0.33–0.87 s at 3 200 candidates process start included (`m2v2/perfB_tier_*.mg`). The general rule: an aggregation body that pairs a relation with itself is a quadratic to be bucketed — sum per tier, then pair the tiers. Mitigations that stay in policy: (a) mounted knowledge (Pattern A) keeps CodeDOM out of the EDB count and out of the strata that do not need it; (b) per-agent scoping — `inject` for one compile only needs that agent's rows, so the JIT compiles against the live kernel's `inject` rows rather than a clone with all of them; (c) the fact ceilings that exist are display-only (M1 §1.4) — wire `core_limits.max_facts_in_kernel` and `max_derived_facts_limit` before adding rows, or an oversized self-join hits the 500k gas limit and aborts the whole evaluation (`EVAL ERROR: fact size limit reached`, M3 `-limit` probe) with a partially updated store (M0 §7; the partial-update half is **unverified**, M3 open item (b)). **Still unverified:** the composed cost of Patterns A–G beside the shipped policy through `rebuildProgram` — the seven pass-2 files compose as one unit through the raw engine (`compose_v2.mg`), but not yet beside the 913-stratum corpus.
3. **Forcing grows in Go again.** The last time forcing was added it grew as `toolBudgetController` (M1 §7 row 8) — a Go type with `hardLimit`, `maxExtensions`, `extensionSize` — even though `working_set.mg` had the rules. This design keeps forcing in policy by three mechanisms: (a) *the loop reads one predicate*, `loop_verdict/2`, and has no other exit — a test asserts that `internal/session` has no field or constant named `*Limit`, `*Iterations`, `*Extension*` on the executor config, in the style of `TestStarvedPredicateBudget` (fails on growth); (b) *every threshold is a fact* (`stall_rounds`, `repeated_failure_rounds`, `spend_ceiling`, `confirm_threshold`, `relevance_threshold`, `knowledge_boost`) and `nerd why loop_verdict` shows which one fired — a Go constant cannot appear in a proof, which is how a reviewer notices it; (c) *obligations are the only completion criterion*: adding a new "the model must…" is adding a rule into `obligation/3`, and the dogfood ledger records each one (root `CLAUDE.md`, ledger path). The remaining honest risk: a stall the rules do not recognise (a model looping on *successful* identical calls) tempts a Go patch; the answer is a `tool_call(T, R, Digest)` witness and a `repeated_call_rounds/1` fact, and the design should ship that pair before it is needed.
4. **Engine defects the design depends on being fixed or avoided.** E1 (`+`-moded externals panic on bound variables, §2) forces all-output mounts and whole-table enumeration per evaluation until an upstream fix; `deferred` predicates are unusable (M0 §3, four defects) so on-demand backward chaining is off the table; `SimpleInMemoryStore` drops hash-colliding facts (M0 §2) and is still selected below 1,024 facts (`kernel_eval.go:841-846`) — every test kernel in the tree runs on it; unsafe negation is silently deleted (M0 §4) — the dropped-negation check from `main.go:79-98` belongs in `rebuildProgram`; a wildcard in a positive premise drops the leaf from the proof (R1, §7) and a comparison builtin marks the node `[partial]` without dropping anything (R2, §7); a clause-final bare name constant after `=`/`!=` swallows the period (T1, §8); an all-output external that returns nothing is re-called per solution row (§2 cost note); a `+`-moded external before its binder passes analysis (M3 engine surprise 2); and **a retained store defeats negation on re-evaluation** — a derived fact from a previous program stays in the store, and in the next program `s(X) :- p(X), !q(X).` fails for the row whose stale `q` is still there (M3 §3 row 19, `m0_AN1/2.mg`: `s(1)` missing). The last one is the trap for any incremental path: codeNERD's differential engine (`internal/mangle/differential.go`) must clear IDB facts before re-running or be tested against exactly that program (M3 open item (d), **unverified**). None of these is a reason to keep a decision in Go; each is a reason for a lint, a host-side guard, or an engine bump.
5. **Witness quality.** Every pattern is only as good as the witnesses Go asserts: a `test_run` row parsed wrongly from `go test -json`, a `changed_symbol` with a bare name instead of a ref (`agents.md:16-20`), a `current_rev` not bumped after a write. The Decl-contract failure class is silent by construction (`agents.md:11-13`), and the bounds checker that would catch the type half of it is off (`AnalyzeOneUnit` everywhere, M0 §3). Turning on `AnalyzeAndCheckBounds(…, ErrorForBoundsMismatch)` for the decision predicates' Decls is cheap insurance; the shape half needs the end-to-end tests of §1, one per migration row.
6. **Two kernels, two truths.** `working_set.mg` runs in a private `mangle.Engine` (M1 §2.2) with its own coercion regime (M1 §5 item 14); Pattern D wants `stall/2` in the executive kernel. Either the working set's witnesses are asserted into both (two truths — the thing the no-shim rule forbids) or the working-set rules move into the executive kernel with `working_*` predicates scoped by task ID. The second is the design's choice; the cost is the extra rows in the 913-stratum program (risk 2).
7. **Model-assertable predicates.** `task_status`, `task_completed` and the planner's `campaign_*`/`task_*` prefixes remain model-assertable (M1 §6). Pattern D reads none of them, but until row 17 lands a model can still assert a `campaign_active` that some other rule trusts. The allowlist migration (row 17) should land before Pattern D is the loop's only exit.

