# Mangle QA Journal: Boundary Value Analysis and Edge Case Deep Dive
## 1. Executive Summary
This journal entry analyzes the missing boundary value analysis and negative testing edge cases in the test suite `internal/mangle/torture_test.go`.
The evaluation covers Null/Undefined/Empty vectors, Type Coercion anomalies, User Request Extremes, and State Conflicts.
We correlate these testing gaps with the architectural design principles of the Mangle logic engine, analyzing the neuro-symbolic implementation and its constraints.
## 2. Null/Undefined/Empty Edge Cases
The current test suite lacks explicit tests for empty strings, null bytes, and empty arrays.
Mangle evaluates facts by matching against atoms and strings. If an empty string or a null byte is provided, the engine might crash or match unexpectedly during string comparisons.
Passing an empty array to `testFact.Args` could cause panic when rules expect specific arity.
These boundary conditions directly interact with the engine's perception and binding mechanisms.
## 3. Type Coercion Edge Cases
Mangle operates with strict typing for numbers (int64 only).
Passing a string containing digits to an integer field, or a float value to an integer field in `testFact` should be explicitly rejected.
The engine should raise a clear error rather than panicking or silently failing to evaluate.
We must ensure the test harness explicitly constructs `ast.String` versus `ast.Name` to catch these discrepancies.
## 4. User Request Extremes
Extreme boundary tests are necessary to evaluate the engine's termination safety and memory footprint.
Large strings exceeding 1MB could lead to Out of Memory (OOM) errors or cause severe latency in string manipulation methods.
Deep recursion limits (e.g., transitive closures over a graph of 100,000 edges) need testing to ensure the Stratification engine prevents infinite loops.
Evaluation at these extremes validates the fact size limit and the engine's halt conditions.
## 5. State Conflicts & Concurrency
Concurrent calls to `evaluateAndQuery()` should be thread-safe.
Tests should verify that multiple parallel evaluations do not corrupt the engine's internal state or cause race conditions, especially under heavy load.
If facts are retracted immediately before evaluation, the engine must handle the missing dependency gracefully without deadlocking.
## 6. Architectural Context and Deep Dive Analysis
The following analysis contextualizes the boundary cases within the Mangle programming model:
Architectural Assessment: # M2 — Mangle programming model for codeNERD
Architectural Assessment: ## Status
Architectural Assessment: - last updated: 2026-09-18 10:02 (local) — pass 2 (revision against M3's refutation list) complete; M0 corrections appended
Architectural Assessment: - done: 1 principles (pass 2: one Decl per predicate; `NegAtom`-count lint; no `_` in `do` bodies; `mgv.exe` both-stores/5-runs rule); 2 Pattern A (holds per M3; pass 2: empty-external re-call + `ShouldQuery`-once/sentinel fixes verified, variable revision join, E1 order caveat); 3 Pattern B (pass 2: tier prefix sum, linear in atoms, `hit_boost`, `fn:collect_distinct`; 11 strata); 4 Pattern C (pass 2: per task, `test_run/4` shared with G, failure guard on `test_green_now`, `/write` → `/rerun` chain; 9 strata); 5 Pattern D (pass 2: task-joined obligations, windowed `failure_count`, `current_rev` invariant; 13 strata); 6 Pattern E (pass 2: `plan_site` carries its change, `blast_radius` over distinct refs, `edit_applied` per op; 9 strata); 7 Pattern F (pass 2: depth column, `why_direct`, analysed-clause hash, R2 partial-from-builtin; 5 strata); 8 Pattern G (pass 2: build-failure guard, `/reran` stage, shard-typed claims, T1 widened; 10 strata); 9 migration map (row 16 verified; seams extended); 10 token economy (strata add linearly: 61 composed); 11 open risks (measured quadratic + tier fix; retained-store negation trap); composition of the seven pass-2 programs as one unit (`compose_v2.mg`, 116 Decls, `strata=61`, D reads C and G across the seam)
Architectural Assessment: - open: (a) composed cost of Patterns A–G loaded *beside the shipped policy* through `rebuildProgram`, unmeasured (the raw-engine composition is done); (b) whether E1 is fixed upstream or carried as an engine bump, and whether the analyzer gate gains the external-order check with it (architect decision); (c) `nerd why` needs a `rule_origin(RuleID, File, Line)` producer in the loader because the recorded rule text is the rewritten clause (§7 rule 5) — design only; (d) codeNERD's differential engine against the stale-negation program `m0_AN1/2.mg` (M3 open item (d)) — **unverified**; (e) the `Fact.ToAtom` string→name heuristic fix (Decl-directed coercion or `ast.String` at path-carrying assert sites) is named in §9 but is a corpus change outside this study; (f) M0's `"etc/passwd"`/`"passwd"` 1-row halves remain unexplained (M3 open item (c))
Architectural Assessment: - evidence rule: every claim cites `path:line`, `path#symbol`, or a predicate in a named `.mg`; anything not checked says **unverified**.
Architectural Assessment: - runnable examples: each pattern carries a `.mg` block executed against the pinned engine (`codeberg.org/TauCeti/mangle-go@v0.5.1-0.20260413190942-4dcaa582c6d3`, see M0) with the verifier's harness `C:/Temp/mangle-study/verify/mgv.exe` (M3 §1; PowerShell: `& C:\Temp\mangle-study\verify\mgv.exe [-store simple|multi] [-det=false -runs N] [-prov] [-ext] <file.mg> <pred> ...`). Pass-2 programs and outputs: `C:/Temp/mangle-study/m2v2/` — `patternB_v2.mg`, `patternC_v2.mg`, `patternD_v3.mg` (+`_spread`, `_2rev`), `patternE_v3.mg` (+`_blocked`), `patternF_v2.mg`, `patternF_v2_B.mg` (why-program over the fresh `inject` proof, `provB_v2.txt`), `patternG_v2.mg`, `compose_v2.mg` (+`_onerev`), `perfB_tier_{200,400,800,3200}.mg`, `prov_builtin.mg`, `prov_fn.mg`, `ext_sentinel.mg` (run with the forked harness `m2v2/harness/mgv2.exe`, which adds `sentinel_all/2` and `once_all/2`). Pattern A's program is `verify/patternA.mg`, unchanged. The first draft's files under `C:/Temp/mangle-study/m2/` are what M3 verified and refuted.
Architectural Assessment: ## Revision log
Architectural Assessment: - 2026-09-18 08:28 — second pass started (programming-model designer, pass 2). Reading M3 sections 2-5 (refutation list) and revising each refuted / holds-with-changes pattern in place. Entries below are appended as each revision lands; each names the M3 item, what changed in M2, and why.
Architectural Assessment: - 2026-09-18 08:42 — **§4 Pattern C rewritten** (M3 items 1, 7, 8, 9, 17). `changed_symbol` gains a `Task` column and `behavior_touched`/`test_obligation`/`obligation_open` are per task (item 1: the old shape cross-joined every active task with every behavior obligation in Pattern D). `test_run` is now the one shape `test_run(Task, TestRef, Rev, Outcome)` shared with Pattern G (item 17). `test_green_now` is guarded by `has_test_failure_now` so a `/fail` at the current revision defeats a `/pass` at the same revision (item 7). Rule 3 now says the `Rev` column is provenance and is not joined — obligations persist until a test exists (item 8, decision: persist). Rule 5 says adding a test converts `/write` into `/rerun` for the new test (item 9). Program `C:/Temp/mangle-study/m2v2/patternC_v2.mg`, run on `mgv.exe` (both stores, 5 nondeterministic runs identical), `strata=9`.
Architectural Assessment: - 2026-09-18 08:47 — **§5 Pattern D rewritten** (M3 items 1, 2, 16). `obligation` joins `test_obligation(T, B, Target, Kind)` on the task (item 1; the cross product is gone — a second active task with nothing open is now `/complete`, `patternD_v3.mg`). `failure_count` counts `recent_failure`, the failures inside the last `repeated_failure_rounds(K)` rounds (item 2; the 1/5/9 spread case is `/continue`, `patternD_v3_spread.mg`). `/hollow` reads Pattern G's `unverified_claim` instead of a `hollow_success(T, Reason)` witness. New rule 6: `current_rev_count` + `kernel_invariant_violated(/current_rev, N)` (item 16; `patternD_v3_2rev.mg` prints the violation). `strata=13`.
Architectural Assessment: - 2026-09-18 08:55 — **§6 Pattern E rewritten** (M3 items 3, 4). `plan_site` is now `plan_site(Task, Change, Op, Ref, Tier)` and `edit_plan` joins a site to its own `chosen_change(T, Change, Op, Spec)` (item 3; two changes on one task give 7 rows, not 12, `patternE_v3.mg`). `blast_radius` counts `site_ref(T, Ref)`, a projection, so it is distinct refs (item 4; 6 for 8 site rows). `edit_applied` and `edit_done_now` carry the `Op`, because applying a rename at the definition does not apply the signature change there (found while rewriting; step 1 stays pending for the second change). Every aggregation body names its columns (item 5's corpus rule). `strata=9`.
Architectural Assessment: - 2026-09-18 09:02 — **§8 Pattern G rewritten** (M3 items 7, 10, 17, 22). `has_build_failure` guards `/build_green` the way `has_test_failure` guards `/tests_green` (item 7; t7's fail-then-pass at one revision is `/edited`). New stage `/reran` for a coder task with no write and green tests, and `verdict(T, /verified, /reran)` (item 10; t6 is no longer `/nothing_done`, which contradicted Pattern C's `/rerun`). Claim rules carry `shard_run(T, /coder|/reviewer, Rev)`; a reviewer's `/done` is unverified only with no findings and no review tool (item 10; t4 is no longer hollow). Rule 1 names `test_run/4` as the one witness shared with Pattern C (item 17). T1 paragraph widened per item 22. Every aggregation body and positive premise names its columns. `strata=10`.
Architectural Assessment: - 2026-09-18 09:12 — **§3 Pattern B rewritten** (M3 items 6, 20). The budget prefix sum is over priority tiers (`tie_sum` → `above_tier` → `above_sum(A, P, S)`), quadratic in distinct priorities and linear in atoms, replacing the atom-pair `above_pair` (item 20). Measured with the verifier's perf inputs: same `fits`/`dropped` sets at 200 candidates (0 differences), 0.14–0.34 s at 800 and 0.33–0.87 s at 3 200 candidates, where the pair form overflowed the 100 000 created-fact limit at 800 (`perfB_tier_*.mg`). `hit_boost` (`fn:max` over cleared tiers) makes a knowledge hit one candidate row (the per-tier design note). `inject_reasons` uses `fn:collect_distinct` and the last paragraph no longer says `fn:collect` gives a clean list (item 6). `inject`/`dropped` output unchanged; `strata=11`.
Architectural Assessment: - 2026-09-18 09:28 — **§7 Pattern F rewritten** (M3 items 13, 14). The why-program carries a depth column (`supports/3`, `why/4`, `why_direct/3`) so the rule's own premises are separable from transitive support (item 13; over the fresh Pattern B v2 `inject` proof, depth 1 is exactly `prompt_atom` + `window_slot`, `patternF_v2_B.mg`). The leaf list for the `inject` proof is replaced by what the recorder actually emits, before and after R1. Rule 5 says `/rule/<hash>` hashes the analysed, rewritten clause (including `__tmp` rules) and `nerd why --diff` is over rule IDs; a `rule_origin(RuleID, File, Line)` fact is proposed for the file position (item 14). New **R2**: a comparison builtin marks a node `[partial]` without losing leaves (`recorder.go:233-241`, `prov_builtin.mg`) — every threshold rule's proof is `[partial]`, so the flag must not be rendered as "incomplete". Also: `EmitFacts` rows need a terminating `.` before they parse as a program. `strata=5`.
Architectural Assessment: - 2026-09-18 09:40 — **§2 Pattern A revised** (M3 items 11, 12; engine surprises 1–3). The program holds (M3 re-ran it on both stores) and is unchanged. Cost note rewritten: an all-output external that returns no rows is called on every premise evaluation (`v_ext_empty.mg`, 6 calls); two host-side fixes verified on a forked harness (`m2v2/harness/mgv2.exe`, `ext_sentinel.mg`): a per-evaluation `ShouldQuery` that declines after the first call (1 call) — the design's choice — and a sentinel row (1 call). Rule 2 now says the revision join must be a variable, because a constant in an external's output position is a cache key and a filter (surprise 3). E1 paragraph adds the order caveat: analysis accepts a `+` external written before its binder, which panics at runtime (surprise 2), and notes the M0 correction (item 12).
Architectural Assessment: - 2026-09-18 09:52 — **§1, §9, §10, §11 revised** (M3 items 5, 15, 16, 17, 18, 19, 20, 21, 22, 23). §1 convention 1: exactly one Decl per predicate, in its owning file (item 16; the seven pass-2 programs compose with one Decl each, `compose_v2.mg`, `strata=61`). Convention 5: the dropped-negation lint compares `NegAtom` counts, unbound named variables are deleted too (item 21). Convention 6: no `_` in any `do`-transform body, `fn:collect_distinct` (items 5, 6). §1 testing paragraph now names `mgv.exe` and the both-stores/5-runs rule. §9 row 16 marked verified (item 18); seams gain the `current_rev` invariant (16), the `Fact.ToAtom` string→name heuristic with the assert-site fix (19), one witness shape per name (17), the widened T1 and `NegAtom`-count lints (21, 22), `ShouldQuery`-once and the external order check. §10: strata add linearly — 52 for the first draft, 61 for pass 2 (item 15). §11 risk 2 carries the measured quadratic curve and the tier fix (20); risk 4 adds R2, the empty-external re-call, the order caveat and the retained-store stale negation (23).
Architectural Assessment: - 2026-09-18 10:02 — **M0 corrections appended** (`M0-mangle-language-surface.md` "## Corrections from verification", 14 entries: items 5, 12, 19, 21, 22, 23 and the engine surprises that hit M0 claims; M0 open item (a) closed to its residue). Pattern A's run line repointed to `mgv.exe`. No refutation in M3 §5 is disputed: every item was either adopted with a program that shows the corrected behaviour or, for items 8 and 10, resolved by a stated design decision (obligations persist regardless of revision; `/reran` stage and shard-typed claims). Pass 2 complete.
Architectural Assessment: ## 1. Design principles
Architectural Assessment: **What is asserted (EDB) vs what is derived (IDB).** Go asserts only *witnesses*: things it measured or was told — a tool ran, a file's hash is H at revision R, the user typed T, the vector index returned (Atom, Score) for query Q at revision R, the test runner exited non-zero with this output. Go never asserts a *judgement*: not "done", not "relevant", not "needs a test", not "delegate to coder", not "mandatory". Every judgement is the head of a rule. The corpus already states this contract for one predicate family — `turn_evidence/6` is asserted, `hollow_success/1`, `turn_executed/1`, `turn_done/1` are derived (`internal/core/defaults/policy/coder_safety.mg:68-115`) and the model is hard-denied from asserting the conclusions (`internal/core/mangle_updates.go:145-150`, M1 §6). The design generalises that one contract to every executive decision in M1 §7.
Architectural Assessment: **Where Go stops and Mangle starts.** Go is the FFI and the effect side (root `CLAUDE.md` "Clean fixpoint, not clean loop"): it parses, embeds, runs tools, calls models, reads SQLite/vector/CodeDOM stores, and asserts what came back. Mangle owns *selection* (which atom, which file, which shard, which test), *ordering* (where a fact sits in the window), *gating* (`permitted`, `turn_done`, `task_incomplete`), *budgeting* (counts and sums compared with facts, never Go constants) and *explanation* (provenance). The one sanctioned Go opinion is fuzzy signal extraction from natural language (`delegation.mg:340-360` + `delegation_routing.go:104`, M1 §7 row 15, root `CLAUDE.md` Mangle guardrails) — Go emits a `*_signal` fact; the combination is a rule. Concretely, Go stops at the four seams the engine gives a host (M0 §9): `extraPredicates`/`Assert` (facts in), `external()` callbacks (facts pulled on demand), `Query`/`GetFacts` (facts out), and `WithDerivationRecorder` (why). A Go function that reads two kernel facts and decides something is the drift to hunt; it becomes a rule and Go reads one row.
Architectural Assessment: **Naming and Decl conventions** (every predicate in this document follows them):
Architectural Assessment: 1. Every predicate has **exactly one** `Decl`, in the file that owns it, with `bound [...]` on every column, before use — the bounds are a codeNERD convention, not an engine rule (synthetic decls, M0 §3), kept because they are the only contract a reader can check (`internal/mangle/agents.md:9-42`); the *one* is an engine rule: a second `Decl` of the same predicate anywhere in the loaded unit is a hard analysis error (`analysis/validation.go:173-175`; M3 composition: the first draft's seven files declared six predicates twice and refused to load as one unit). A predicate shared by two patterns (`code_element/5`, `current_rev/1`, `test_run/4`, `test_obligation/4`, `unverified_claim/2`, …) is declared once in the schema file and *used* by both; the pass-2 examples are composed that way in `C:/Temp/mangle-study/m2v2/compose_v2.mg` (116 Decls, loads, `strata=61`). Same name with a different arity is a different predicate and loads silently — the worse failure (M3 engine surprise 8).
Architectural Assessment: 2. Identity columns are `/string` refs in the exact shape their producer emits (`<pkg>.<Name>` for CodeDOM, `internal/mangle/agents.md:24-31`); enumerations are `/name` atoms; counts, percentages and scores are `/number` int64 in 0–100 (a float aborts the fixpoint, M0 §4, `internal/core/kernel_fact_decl.go:27-45`). Never mix a number column with a list/struct column in one position (hash collisions on `SimpleInMemoryStore`, M0 §2).
Architectural Assessment: 3. Witness predicates are named by what happened, past tense or noun (`tool_ran`, `test_run`, `workspace_snapshot`); judgement predicates by the decision (`inject`, `test_obligation`, `task_incomplete`, `edit_plan`); thresholds and policy constants are `*_threshold/2` or `*_rounds/1` facts (`working_set.mg:53-57` is the existing precedent).
Architectural Assessment: 4. Every mounted external is revision-keyed: an input or output column carries the revision the answer was computed at, because the engine caches external answers for the whole evaluation (M0 §3, `engine/topdown.go:84-93`) and stale evidence must not survive a source change (root `CLAUDE.md` North Star).
Architectural Assessment: 5. Negation only over a projection whose every argument is bound; never a wildcard *or an otherwise-unbound named variable* in a negated literal (M0 §4 probes A/B/D; M3 §3 row 4: `q(X) :- p(X), !r(X,Y).` is silently rewritten to `q(X) :- p(X).` exactly like the wildcard form; `working_set.mg:88-91`). Zero-arity projections (`has_hollow_success()`) are the idiom. The lint that catches the silent deletion compares the `NegAtom` count of each clause before and after `AnalyzeOneUnit` — it cannot look for `_` alone (M3 item 21; the verifier's `mgv.exe` warns on the same comparison the first draft's harness refused on, and both are host-side because the engine never reports the drop).
Architectural Assessment: 6. Aggregation is its own stratum: one `|> do fn:group_by` per rule, the zero case as a negation rule (M0 §6), reducers `fn:count/fn:sum/fn:max/fn:min/fn:collect/fn:collect_distinct` only — `fn:count_distinct`, `fn:map:get`, `fn:pick_any` do not work (M0 §5). **No `_` in the body of a `do`-transform rule, ever**: a single-atom body is not rewritten and the reducer sees stored rows *including* the wildcard column, while a multi-atom body is projected onto the named variables first — so `c1(N) :- p(X, _) |> do fn:group_by(), let N = fn:count().` counts facts and `c2(N) :- p(X, _), q(X) |> …` counts distinct `X` (M3 engine surprise 4, `v_agg_wildcard.mg`: `c1(3)`, `c2(2)`; `rewrite/rewrite.go:40,52`). Name every column, and when a count over distinct keys is wanted, project first (`site_ref` in Pattern E). `fn:collect` keeps duplicates and its order is map-iteration order; `fn:collect_distinct` is the set (M3 engine surprise 5).
Architectural Assessment: 7. Selective, bound atoms first in every body — the engine is nested loops in written order (M0 §7); an external or aggregate is never the first premise.
Architectural Assessment: 8. A rule that must not fire twice on the same key uses a merge predicate (`fundep`+`merge`, M0 §3 item 4) or a `_superseded` projection (`working_set.mg:103-123`), never Go retract-before-assert discipline (M1 §5 item 11).
Architectural Assessment: **How a pattern is tested.** Each pattern below ships a `.mg` that the verifier's harness `C:/Temp/mangle-study/verify/mgv.exe` (M3 §1; source `main.go`, built with `GOPROXY=off GOFLAGS=-mod=mod go build -C /c/Temp/mangle-study/verify -o mgv.exe .` from bash, **run from PowerShell** — the exe returns no output under the Git-Bash tool until its timeout) parses, analyses, warns if the rewrite dropped a negation (M0 §12 item 1; the first draft's `m2/mgrun.exe` refused instead), stratifies, evaluates and prints the named predicates; `-store simple|multi` picks the store (codeNERD runs `SimpleInMemoryStore` below 1024 facts, `kernel_eval.go:842-846`), `-det=false -runs N` drops `WithDeterministicOrder` and reports any run whose output differs, `-prov` prints the proof and its `EmitFacts` encoding, `-ext` registers the stub externals. A pattern *holds* only when the derived set is identical on both stores and across 5 nondeterministic runs (M3's rule, adopted here). The pass-2 programs live in `C:/Temp/mangle-study/m2v2/` (`patternB_v2.mg`, `patternC_v2.mg`, `patternD_v3.mg`, `patternE_v3.mg`, `patternF_v2.mg`, `patternG_v2.mg`; Pattern A's `verify/patternA.mg` is unchanged) with the composed unit `compose_v2.mg`; the first draft's `C:/Temp/mangle-study/m2/` files are what M3 §2 verified and refuted. Inside codeNERD the same test is a kernel test in the shape of `TestImpactChain_EndToEndThroughVirtualStore` (`internal/mangle/agents.md:36-38`): assert the witness facts through the real `Assert` path, query the *derived* predicate, compare rows — never test producer and consumer separately, because a shape mismatch passes both (`agents.md:11-13`).
Architectural Assessment: ## 2. Pattern A — mounting knowledge as virtual predicates
Architectural Assessment: **Intent.** The deductive database is the delivery mechanism: SQLite domain knowledge, vector search, CodeDOM, the world model and session history are *relations the kernel joins*, not Go lookups whose results a Go function ranks. Today the kernel pulls only from SQLite and the vector index, through 10 `external()` Decls (`schemas_memory.mg:54-84`, `schemas_tools.mg:320-325`; M1 §4.1); **no live rule body calls any of them** (grep of `internal/core/defaults` and `internal/context` for `recall_similar(|query_session(|query_knowledge_graph(|query_graph(|query_traces(` in rule bodies: 0 hits, 2026-09-18); CodeDOM, world and history reach the kernel only as bulk `AssertBatch` from Go (M1 §4.1, last paragraph). Relevance thresholds are literals inside rules (`activation.mg:25-27`, `Score > 30`) or Go constants (`internal/context/compressor.go:568,595`, `AtomReserve`).
Architectural Assessment: **Engine features relied on.** `external()` + `mode` Decls and `WithExternalPredicates` (M0 §3 "external()", §9 item 7); the once-per-evaluation answer cache (`engine/topdown.go:84-93`, M0 §3); typed `bound` columns (M0 §3 — documentation-only unless `AnalyzeAndCheckBounds` is turned on, M0 §12 item 8); int64 comparison builtins (M0 §5); `!=` on bound constants (M0 §4).
Architectural Assessment: **Existing predicates it builds on (M1).** The external Decl shape at `schemas_memory.mg:54-84`; the adapter `virtualExternalPredicate` (`internal/core/external_predicates.go:29-85`), which already emits output positions only; `file_topology/5` (`schemas_world.mg:17`), whose `Hash` column is the natural revision key for file-scoped knowledge; `code_element/5`, `code_calls/2`, `code_defines/5` (`schemas_codedom.mg:33`, `schemas_analysis.mg:112-116`); `activation/2` and `context_atom/1` (`activation.mg:20-27`); `should_include_context/2` (`schemas_context.mg:26`).
Architectural Assessment: **Engine defect E1, found while building this pattern (2026-09-18).** An `external()` predicate whose `+` (input) position holds a *variable bound earlier in the rule* **panics the pinned engine**: `oneStepEvalPremise` hands the premise atom to `EvalExternalQuery` **without applying the current substitution** (`engine/seminaivebottomup.go:831`, versus `premiseAtom(p, lookupFn, subst)` at `:851` for the store lookup), and `EvalExternalQuery` does `arg.(ast.Constant)` on every input position (`engine/topdown.go:99`) → `panic: interface conversion: ast.BaseTerm is ast.Variable, not ast.Constant` (probe `C:/Temp/mangle-study/m2/extA_min.mg`: `out(S,D) :- focus(S), sqlite_symbol_doc(S, "rev-17", D).` under `mode("+","+","-")`). The same rule with constants in both input positions works (`extA_const.mg`). M0 §3's sentence "called … once per binding of the input positions" is therefore wrong for variables — it is called with the unsubstituted atom (M3 item 12; corrected in M0's "Corrections from verification"). The verifier added two facts (M3 §2A, engine surprise 2): E1 is not specific to strings or mixed input lists (`v_ext_plus_var.mg`, `q(X,Y) :- p(X), dbl_ext(X,Y).` panics the same way), and **analysis does not check the written order of moded externals** — `q(X,Y) :- dbl_ext(X,Y), p(X).` with the external first passes `AnalyzeOneUnit` (positive atoms are order-independent for safety, M0 §4) and panics at runtime (`v_ext_plus_first.mg`). So even after the one-line upstream fix, a `+` external written before its binder will pass the analyzer and see an unbound variable at runtime; the analyzer gate (§9 cross-cutting seams) needs an order check for moded externals. Constant inputs are cached as claimed: `q(Y) :- dbl_ext(4, Y).` and a second rule with three premise evaluations of the same constant make **one** call (`v_ext_plus_const.mg`). This is why codeNERD's `+`-moded externals are unusable from rules and why the corpus never calls them. The fix is one line upstream (`p = p.ApplySubst(subst).(ast.Atom)` before `:831`; `ast.Atom.ApplySubst` exists at `ast/ast.go:1023`); whether to carry it as a pinned-engine bump is an architect question. **Until then Pattern A uses all-output externals** (`mode("-",…)`): the engine calls them once per evaluation with no inputs (`inputs=[]` in the harness output), Go enumerates rows keyed on EDB facts it reads itself (the VirtualStore already holds the kernel, `internal/core/kernel_virtual.go:8`), and the join happens in Mangle. This is how `query_learned`, `query_strategic`, `query_activations`, `has_learned` are declared today (`schemas_memory.mg:54,76,80,84`).
Architectural Assessment: **The pattern.** Five rules, no Go decisions:
Architectural Assessment: 1. *Mount* — `Decl <source>_all(K1…, Rev, V1…) descr [external(), mode("-",…)] bound […]`. Every row carries the revision (`file_topology.Hash` for file-scoped knowledge, an index generation string for the vector index, a schema/version stamp for SQLite tables) at which Go computed it.
Architectural Assessment: 2. *The current revision is a fact, joined through a variable* — `knowledge_rev(Source, Rev)`, asserted by Go when a source changes (world scan, reembed, SQLite migration). The join `knowledge_rev(/vectors, Rev), recall_similar_all(Q, Rev, Atom, S)` is what keeps stale rows out; there is no Go cache invalidation. The revision must be a *variable* bound by the join, never a constant written into the external's argument: a constant in an output position of an all-output external becomes part of the cache key and a filter — rows that do not match it are discarded, not stored (`engine/topdown.go:119-133`), so two rules with two constants each call the external and neither can reuse the other's table (M3 engine surprise 3, `v_ext_filter.mg`: `old(A) :- recall_similar_all(_, "rev-16", A, _)` and `cur(A) :- … "rev-17" …` made two calls with filters `[_ "rev-17" A _]` and `[_ "rev-16" A _]`).
Architectural Assessment: 3. *Thresholds are facts* — `relevance_threshold(Kind, Pct)`; a rule compares with `>=`. Changing policy is asserting a fact, and `nerd why` can show which threshold admitted an atom (Pattern F). `activation.mg:27` (`Score > 30`) and `AtomReserve` (`compressor.go:568`) become rows.
Architectural Assessment: 4. *Exclusions are derived, not filtered* — a row that fails a gate becomes a fact naming the gate (`knowledge_stale`, `knowledge_hit_offlang`), so "why was this not injected" has a derivation. The engine has no negative provenance (M0 §8), so the design manufactures positive facts for the negatives that matter.
Architectural Assessment: 5. *Consumers join on typed columns* — `task_query/2`, `task_focus_symbol/2`, `task_language/2` are the task frame Go asserts; `knowledge_hit/3` and `symbol_doc/3` are what Pattern B consumes.
Architectural Assessment: **Runnable example** (`C:/Temp/mangle-study/verify/patternA.mg`, byte-identical to the first draft's `m2/patternA.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -ext -store simple -det=false -runs 5 patternA.mg knowledge_hit knowledge_hit_offlang symbol_doc knowledge_stale`):
Architectural Assessment: ```mangle
Architectural Assessment: # Pattern A -- mounting knowledge as virtual predicates (runs on the pinned engine).
Architectural Assessment: Decl recall_similar_all(Query, Rev, Atom, ScorePct) descr [external(), mode("-", "-", "-", "-")] bound [/string, /string, /string, /number].
Architectural Assessment: Decl sqlite_symbol_doc_all(Sym, Rev, Doc) descr [external(), mode("-", "-", "-")] bound [/string, /string, /string].
Architectural Assessment: Decl knowledge_rev(Source, Rev) bound [/name, /string].
Architectural Assessment: Decl relevance_threshold(Kind, Pct) bound [/name, /number].
Architectural Assessment: Decl task_query(TaskID, Text) bound [/string, /string].
Architectural Assessment: Decl task_focus_symbol(TaskID, Sym) bound [/string, /string].
Architectural Assessment: Decl task_language(TaskID, Lang) bound [/string, /name].
Architectural Assessment: Decl atom_language(Atom, Lang) bound [/string, /name].
Architectural Assessment: Decl knowledge_hit(TaskID, Atom, ScorePct) bound [/string, /string, /number].
Architectural Assessment: Decl knowledge_hit_offlang(TaskID, Atom) bound [/string, /string].
Architectural Assessment: Decl knowledge_stale(Atom, Rev) bound [/string, /string].
Architectural Assessment: Decl symbol_doc(TaskID, Sym, Doc) bound [/string, /string, /string].
Architectural Assessment: # Facts Go asserts: current revisions, thresholds, task frame.
Architectural Assessment: knowledge_rev(/vectors, "rev-17").
Architectural Assessment: knowledge_rev(/sqlite, "rev-17").
Architectural Assessment: relevance_threshold(/vector, 30).
Architectural Assessment: task_query("t1", "wrap errors with context").
Architectural Assessment: task_focus_symbol("t1", "core.evaluate").
Architectural Assessment: task_language("t1", /go).
Architectural Assessment: atom_language("knowledge/go/error_wrapping", /go).
Architectural Assessment: atom_language("knowledge/go/table_tests", /go).
Architectural Assessment: atom_language("knowledge/python/asyncio", /python).
Architectural Assessment: atom_language("knowledge/go/stale_atom", /go).
Architectural Assessment: # The mount. Bound EDB atoms first; the revision join drops rows computed at
Architectural Assessment: # an older revision; the threshold is a fact.
Architectural Assessment: knowledge_hit(T, Atom, S) :-
Architectural Assessment: task_query(T, Q),
Architectural Assessment: knowledge_rev(/vectors, Rev),
Architectural Assessment: recall_similar_all(Q, Rev, Atom, S),
Architectural Assessment: relevance_threshold(/vector, Min),
Architectural Assessment: S >= Min.
Architectural Assessment: # Rows at a revision that is no longer current: derived, so "why not" has an answer.
Architectural Assessment: knowledge_stale(Atom, Rev) :-
Architectural Assessment: recall_similar_all(_, Rev, Atom, _),
Architectural Assessment: knowledge_rev(/vectors, Current),
Architectural Assessment: Rev != Current.
Architectural Assessment: # A hit in the wrong language is a derived fact too, not a Go filter.
Architectural Assessment: knowledge_hit_offlang(T, Atom) :-
Architectural Assessment: knowledge_hit(T, Atom, _),
Architectural Assessment: task_language(T, Lang),
Architectural Assessment: atom_language(Atom, Other),
Architectural Assessment: Lang != Other.
Architectural Assessment: symbol_doc(T, Sym, Doc) :-
Architectural Assessment: task_focus_symbol(T, Sym),
Architectural Assessment: knowledge_rev(/sqlite, Rev),
Architectural Assessment: sqlite_symbol_doc_all(Sym, Rev, Doc).
Architectural Assessment: ```
Architectural Assessment: Stub externals (`main.go`): `recall_similar_all` returns `("wrap errors with context","rev-17","knowledge/go/error_wrapping",91)`, `(…,"rev-17","knowledge/go/table_tests",74)`, `(…,"rev-17","knowledge/python/asyncio",31)`, `(…,"rev-16","knowledge/go/stale_atom",99)`; `sqlite_symbol_doc_all` returns `("core.evaluate","rev-17","evaluate runs the stratified fixpoint")` and `("core.assert","rev-17","assert adds an EDB fact")`.
Architectural Assessment: **Expected derived facts** (verified 2026-09-18; re-verified by M3 §2A on both stores and 5 nondeterministic runs; `strata=4`; each external called exactly once with `inputs=[]` — *because each returns at least one row*, see the cost note):
Architectural Assessment: ```
Architectural Assessment: knowledge_hit("t1","knowledge/go/error_wrapping",91)
Architectural Assessment: knowledge_hit("t1","knowledge/go/table_tests",74)
Architectural Assessment: knowledge_hit("t1","knowledge/python/asyncio",31)
Architectural Assessment: knowledge_hit_offlang("t1","knowledge/python/asyncio")
Architectural Assessment: knowledge_stale("knowledge/go/stale_atom","rev-16")
Architectural Assessment: symbol_doc("t1","core.evaluate","evaluate runs the stratified fixpoint")
Architectural Assessment: ```
Architectural Assessment: With `relevance_threshold(/vector, 60)` instead, `knowledge_hit` has 2 rows and `knowledge_hit_offlang` none (also run). The `rev-16` row scored 99 and was still excluded: revision beats score, which is "stale evidence must not survive a source change" expressed as a join.
Architectural Assessment: **Mounting the other sources with the same shape** (Decls proposed; none exist yet; producer cost **unverified**):
Architectural Assessment: | Source | Mount Decl | Revision column | Go enumerates from | Replaces |
Architectural Assessment: |---|---|---|---|---|
Architectural Assessment: | CodeDOM | `codedom_callers_all(Ref, Rev, Caller)` | `file_topology.Hash` of the callee's file | the `code_calls` index the Cartographer keeps (`internal/mangle/agents.md:27-29`) | bulk `AssertBatch` of the `code_*` rows every scan (M1 §9: EDB 50–70k at evaluation) |
Architectural Assessment: | World model | `world_file_all(Path, Hash, Lang, IsTest)` | `Hash` | the `file_topology` producer | nothing yet; the point is that consumers join on `Hash` |
Architectural Assessment: | History | `history_built_all(Entity, Rev, Verb, Turn)` | session turn number | SQLite `session_history` (`QuerySession`, `virtual_store_predicates.go:294`) | Go-side "have we already done this" checks |
Architectural Assessment: | SQLite knowledge | `sqlite_symbol_doc_all(Sym, Rev, Doc)`, `learned_fact_all(Pred, Rev, Args)` | schema version | `QueryLearned` (`virtual_store_predicates.go:27`) | `HydrateLearnings` bulk assert (`schemas_memory.mg:88-92`) |
Architectural Assessment: | Vector index | `recall_similar_all(Query, Rev, Atom, Pct)` | index generation | sqlite-vec (`RecallSimilar`, `virtual_store_predicates.go:268`), with `Query` taken from the kernel's own `task_query` rows | `recall_similar/3` (`schemas_memory.mg:62`), whose `+` mode cannot be called from a rule (E1) |
Architectural Assessment: **Cost note (revised in pass 2 — M3 item 11, engine surprise 1).** An all-output external returns its whole table once per evaluation **only when it returns at least one row**. The "once" is the store cache in `EvalExternalQuery` (`engine/topdown.go:84-93`: `Store.GetFacts(query, …)` short-circuits when any stored fact matches the premise pattern); with no row stored there is nothing to hit, and the callback runs again on every premise evaluation — one Go call per solution row of everything to its left, per delta round. The verifier measured it: `empty_all(K, V)` after `p(X)` with three `p` rows, two rules, **6 calls** (`v_ext_empty.mg`). An empty vector search, a symbol with no doc, an empty history table are the common case, not the edge case, so the mount has to engage the cache itself. Two host-side mitigations, both verified 2026-09-18 on a fork of the verifier's harness (`C:/Temp/mangle-study/m2v2/harness/mgv2.exe`, `ext_sentinel.mg`, `p(1). p(2). p(3).` and one rule each):
Architectural Assessment: - *Decline the second query.* `ExternalPredicateCallback.ShouldQuery` is consulted before every call (`topdown.go:106`); a callback object created per evaluation (codeNERD registers callbacks per evaluation, `kernel_eval.go:333-344`, on a fresh store, `:256-259`) returns `false` once it has answered. `once_all/2`: **1 call**, two `ShouldQuery=false` refusals. This is the design's choice: it lives in the FFI adapter (`internal/core/external_predicates.go#virtualExternalPredicate`), decides nothing, and leaves the data clean.
Architectural Assessment: - *Return a sentinel row.* `sentinel_all/2` returning `("", "")`: **1 call**, the cache engages; the consuming rule filters it (`q(X, K, V) :- p(X), sentinel_all(K, V), K != "".`, `q` empty). Works in pure Mangle but every consumer of the mount must know the sentinel — a rule with a variable over the key column (`knowledge_stale`) would otherwise see it.
Architectural Assessment: With the cache engaged, the fresh store per evaluation (`kernel_eval.go:256`, M1 §1.3) makes the cost once per dirty query. For CodeDOM that is the volume the bulk assert already moves, but it no longer counts against the 250k EDB ceiling (M1 §1.4) and the revision join is done by the engine. Once E1 is fixed *and the analyzer checks external order*, the `+` form makes it one call per bound key, which is the design target; the Decls above are written so that only the `mode` string changes.
Architectural Assessment: ## 3. Pattern B — derived context injection
Architectural Assessment: **Intent.** What enters an agent's window, why, where, and at what priority is one derived relation, `inject(Agent, Atom, Reason, Slot, Prio)`, computed from the task frame, the mounted knowledge (Pattern A), the history of what has been built, and the atom catalogue — including the **budget cut**, which today is Go (`internal/prompt/budget.go#TokenBudgetManager`, "Mandatory atom %s rejected", M1 §7 row 12; `internal/context/compressor.go:683-696` sorts `should_include_context` rows by parsed priority and "selects the top N within the AtomReserve token budget", `schemas_context.mg:26-28`). Today the kernel derives selection (`selected_result(Atom, Priority, Source)`, `jit_compiler.mg:310-316`, read by `internal/prompt/selector.go:997,1232`) but not fit, not position, and the mandatory flag is YAML data forced in Go (`compiler.go:1099,1148`). Section order is a static table (`category_order/2`, `jit_config.mg:9-22`) that ignores lost-in-the-middle.
Architectural Assessment: **Engine features relied on.** Aggregation with `fn:max` and `fn:sum` after `fn:group_by`, each in its own stratum, zero case via negation (M0 §6, probes J, AJ); int64 arithmetic `fn:plus` and comparisons (M0 §5); `:string:contains` (M0 §5, the one builtin the corpus already leans on, M1 §3); stratified negation over bound projections (`has_above/2`, `fits/2`; M0 §4); `fn:collect_distinct` for the reason set (M0 §5 probe AJ). Eleven strata for the revised example — aggregation chains are the cost centre (see §11), and every aggregation body names all its columns (M3 engine surprise 4; §1 convention 6).
Architectural Assessment: **Existing predicates it builds on (M1).** `prompt_atom/5` (`schemas_prompts.mg:198`), `atom_selector/3` (`:213`), `compile_shard/2` (`:261`), `compile_budget/1` (`:257` — declared, never asserted or read: grep `compile_budget` in `internal/prompt/*.go` → 0 hits; this pattern is its first consumer, widened to `/2` with the agent), `skeleton_category/1` (`jit_selection.mg:187-190`), `category_order/2`, `category_budget/2` (`jit_config.mg`), `mandatory_atom/1`, `selected_atom/1` (`schemas_prompts.mg:426,446`), `should_include_context/2` and `context_relevant/2` (`schemas_context.mg:20-26`, `context_compilation.mg:21-66`), `knowledge_hit/3` and `symbol_doc/3` from Pattern A.
Architectural Assessment: **The pattern** (revised in pass 2 — M3 §5 items 6, 20; the program below is `C:/Temp/mangle-study/m2v2/patternB_v2.mg`, run on the verifier's harness 2026-09-18).
Architectural Assessment: 1. *Every reason is a rule and the reason is a column.* `candidate(Agent, Atom, Reason, Prio)` has one rule per admission reason: `/skeleton` (category), `/task_verb` (selector matches the intent verb), `/knowledge_hit` (Pattern A row above a boost threshold — `knowledge_boost(MinPct, Boost)` facts replace `category_budget` percentages), `/focal_symbol` (CodeDOM ref in the task frame), `/built_before` (history: the model changed this symbol at turn N, so the diff atom outranks the doc atom). **One row per (agent, atom, reason)**: the knowledge boost is `hit_boost(A, Atom, Boost)`, the `fn:max` over the tiers a score clears, so a 91-score hit is one candidate row at +40, not three rows at +40/+20/+5 (the first draft's per-tier rule left three rows behind `fn:max`, which any consumer counting candidates — and every proof of `inject_prio` — saw, M3 §2B design note). No rule says "mandatory": skeleton categories *are* the mandatory set, and `prompt_atom(..., /true)` is a legacy input the rule ignores.
Architectural Assessment: 2. *One priority per atom, and the reasons as a set.* `inject_prio(A, Atom, P)` is `fn:max` over the reasons — the kernel resolves multi-reason conflicts, not `selector.go`. `inject_reasons(A, Atom, Rs)` is `fn:collect_distinct(R)` over the same body: `fn:collect` would give one entry per candidate row, duplicates included (M3 `v_patternB_collect.mg`: `[/knowledge_hit, /knowledge_hit, /knowledge_hit]` under the old per-tier rule), and the list order is map-iteration order either way (3 of 8 nondeterministic runs flipped it, M3 §2B) — so the list is a set to a consumer, never compared or persisted as a sequence.
Architectural Assessment: 3. *The window shape is a prefix sum over priority tiers, not over atom pairs.* `tie_sum(A, P, S)` sums the tokens of each priority tier; `above_tier(A, P, Q, S)` pairs each tier with every strictly higher tier; `above_sum(A, P, S)` sums those. An atom `fits` when its tier's `above + tier <= budget`; a tier is all-or-nothing because Mangle has no ordering inside a tier (M0 §6 "no limit/top-k") and the design chooses not to invent one in Go: equal priority means equally justified, and a tie that does not fit is a budget fact to raise, not a coin to flip. The first draft paired *atoms* (`above_pair(A, Atom, Other, T)`), which is quadratic in candidates per agent: the verifier measured 0.46 / 1.5 / 5.8 s at 200 / 400 / 800 candidates and a 100 000 created-fact overflow at 400 (M3 §4 item 11, `v_perfB_*.mg`); the atom library is 351 atoms and codeNERD's ceiling is 500 000 (`kernel_init.go:188`). The tier form is quadratic in *distinct priorities* (tens) and linear in atoms: the same derived `fits`/`dropped` sets at 200 candidates (`perfB_tier_200.mg` vs `v_perfB_200.mg`, 200 rows, 0 differences), and 0.12–0.22 s at 400, 0.14–0.34 s at 800, 0.33–0.87 s at 3 200 candidates (`perfB_tier_400/800/3200.mg`, three runs each, process start included) where the pair form could not finish 800 under the fact limit. Per-agent scoping (only the compiling agent's rows in the store) is still the right operating mode; it is no longer what keeps the rule affordable.
Architectural Assessment: 4. *Position is a fact.* `window_slot(Category, Slot)` with `/head`, `/middle`, `/tail` replaces `category_order`: skeleton at the head, knowledge in the middle, the focal context and history at the tail next to the request. That is the lost-in-the-middle decision made in policy; Go concatenates slots in fixed order and priority-descending inside a slot, nothing else.
Architectural Assessment: 5. *Negatives are facts.* `dropped(A, Atom, /budget)` is derived for a candidate that does not fit; an atom that was never a candidate is absent from `inject_prio`, so "not relevant" and "relevant but cut" are different answers to `nerd why` (Pattern F).
Architectural Assessment: **How the JIT consumes it.** `selector.go` replaces its `selected_result/3` query, its Go re-ranking and `TokenBudgetManager` with one query, `inject(Agent, Atom, Reason, Slot, Prio)` for the compile's agent, and one deterministic assembly: for each slot in `/head, /middle, /tail`, atoms by `Prio` descending, ties by atom ID (a string sort in Go is presentation, not a decision). `compressor.go:688` does the same for observation entities (`working_selected/2`, `working_set.mg:125-132`, is already this shape). The `kernel/context/*` and `kernel/knowledge/*` dynamic atoms that Go forces mandatory (`compiler.go:1099,1148`) become `prompt_atom` rows with a `/context` category and a `window_slot(/context, /tail)` fact. The manifest (`compiler.go:1581`) records the `inject` rows verbatim — that is the receipt `nerd meter atoms` (`cmd/nerd/cmd_meter.go:115`) already prints from `.nerd/meter/atom-selections.jsonl`.
Architectural Assessment: **Runnable example** (`C:/Temp/mangle-study/m2v2/patternB_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternB_v2.mg inject dropped inject_prio inject_reasons candidate above_sum tie_sum hit_boost`):
Architectural Assessment: ```mangle
Architectural Assessment: # Pattern B v2 -- derived context injection: inject(Agent, Atom, Reason, Slot, Prio).
Architectural Assessment: # Pass-2 changes (M3 items 6, 20): the budget prefix sum is over priority tiers, not atom pairs;
Architectural Assessment: # one candidate row per knowledge hit; reasons collected with fn:collect_distinct.
Architectural Assessment: Decl compile_shard(Agent, ShardType) bound [/string, /name].
Architectural Assessment: Decl compile_budget(Agent, Tokens) bound [/string, /number].
Architectural Assessment: Decl task_verb(Agent, Verb) bound [/string, /name].
Architectural Assessment: Decl task_focus_symbol(Agent, Sym) bound [/string, /string].
Architectural Assessment: Decl prompt_atom(Atom, Category, Prio, Tokens, IsMandatory) bound [/string, /name, /number, /number, /name].
Architectural Assessment: Decl atom_selector(Atom, Dim, Value) bound [/string, /name, /name].
Architectural Assessment: Decl skeleton_category(Category) bound [/name].
Architectural Assessment: Decl window_slot(Category, Slot) bound [/name, /name].
Architectural Assessment: Decl knowledge_hit(Agent, Atom, ScorePct) bound [/string, /string, /number].
Architectural Assessment: Decl knowledge_boost(Pct, Prio) bound [/number, /number].
Architectural Assessment: Decl built_before(Agent, Sym, Turn) bound [/string, /string, /number].
Architectural Assessment: Decl symbol_atom(Sym, Atom) bound [/string, /string].
Architectural Assessment: Decl hit_boost(Agent, Atom, Boost) bound [/string, /string, /number].
Architectural Assessment: Decl candidate(Agent, Atom, Reason, Prio) bound [/string, /string, /name, /number].
Architectural Assessment: Decl inject_prio(Agent, Atom, Prio) bound [/string, /string, /number].
Architectural Assessment: Decl inject_reasons(Agent, Atom, Reasons) bound [/string, /string, /any].
Architectural Assessment: Decl tie_sum(Agent, Prio, Sum) bound [/string, /number, /number].
Architectural Assessment: Decl above_tier(Agent, Prio, Higher, Sum) bound [/string, /number, /number, /number].
Architectural Assessment: Decl above_sum(Agent, Prio, Sum) bound [/string, /number, /number].
Architectural Assessment: Decl has_above(Agent, Prio) bound [/string, /number].
Architectural Assessment: Decl fits(Agent, Atom) bound [/string, /string].
Architectural Assessment: Decl inject(Agent, Atom, Reason, Slot, Prio) bound [/string, /string, /name, /name, /number].
Architectural Assessment: Decl dropped(Agent, Atom, Why) bound [/string, /string, /name].
Architectural Assessment: # policy facts (jit_config.mg)
Architectural Assessment: skeleton_category(/identity).
Architectural Assessment: skeleton_category(/safety).
Architectural Assessment: window_slot(/identity, /head).
Architectural Assessment: window_slot(/safety, /head).
Architectural Assessment: window_slot(/knowledge, /middle).
Architectural Assessment: window_slot(/language, /middle).
Architectural Assessment: window_slot(/context, /tail).
Architectural Assessment: knowledge_boost(90, 40).
Architectural Assessment: knowledge_boost(60, 20).
Architectural Assessment: knowledge_boost(30, 5).
Architectural Assessment: # witnesses for one compile
Architectural Assessment: compile_shard("coder-7", /coder).
Architectural Assessment: compile_budget("coder-7", 260).
Architectural Assessment: task_verb("coder-7", /fix).
Architectural Assessment: task_focus_symbol("coder-7", "core.evaluate").
Architectural Assessment: built_before("coder-7", "core.evaluate", 12).
Architectural Assessment: prompt_atom("identity/coder", /identity, 100, 40, /true).
Architectural Assessment: prompt_atom("safety/no_secrets", /safety, 100, 30, /true).
Architectural Assessment: prompt_atom("language/go/errors", /language, 50, 60, /false).
Architectural Assessment: prompt_atom("knowledge/go/error_wrapping", /knowledge, 40, 80, /false).
Architectural Assessment: prompt_atom("knowledge/go/table_tests", /knowledge, 40, 70, /false).
Architectural Assessment: prompt_atom("knowledge/python/asyncio", /knowledge, 40, 50, /false).
Architectural Assessment: prompt_atom("context/core.evaluate", /context, 30, 90, /false).
Architectural Assessment: prompt_atom("context/history/core.evaluate", /context, 30, 20, /false).
Architectural Assessment: atom_selector("language/go/errors", /intent_verb, /fix).
Architectural Assessment: atom_selector("knowledge/python/asyncio", /language, /python).
Architectural Assessment: knowledge_hit("coder-7", "knowledge/go/error_wrapping", 91).
Architectural Assessment: knowledge_hit("coder-7", "knowledge/go/table_tests", 74).
Architectural Assessment: symbol_atom("core.evaluate", "context/core.evaluate").
Architectural Assessment: symbol_atom("core.evaluate", "context/history/core.evaluate").
Architectural Assessment: # candidates: every reason is a rule, the reason is a column; one row per (agent, atom, reason)
Architectural Assessment: candidate(A, Atom, /skeleton, P) :-
Architectural Assessment: compile_shard(A, Shard), prompt_atom(Atom, Cat, P, T, M), skeleton_category(Cat).
Architectural Assessment: candidate(A, Atom, /task_verb, P) :-
Architectural Assessment: task_verb(A, V), atom_selector(Atom, /intent_verb, V), prompt_atom(Atom, Cat, P, T, M).
Architectural Assessment: # the boost is the highest tier the score clears, so a hit is one candidate row, not one per tier
Architectural Assessment: hit_boost(A, Atom, Boost) :-
Architectural Assessment: knowledge_hit(A, Atom, Score), knowledge_boost(Min, B0), Score >= Min
Architectural Assessment: |> do fn:group_by(A, Atom), let Boost = fn:max(B0).
Architectural Assessment: candidate(A, Atom, /knowledge_hit, Boosted) :-
Architectural Assessment: hit_boost(A, Atom, Boost), prompt_atom(Atom, Cat, P, T, M), Boosted = fn:plus(P, Boost).
Architectural Assessment: candidate(A, Atom, /focal_symbol, P) :-
Architectural Assessment: task_focus_symbol(A, Sym), symbol_atom(Sym, Atom), prompt_atom(Atom, Cat, P, T, M).
Architectural Assessment: candidate(A, Atom, /built_before, 95) :-
Architectural Assessment: built_before(A, Sym, Turn), symbol_atom(Sym, Atom), prompt_atom(Atom, /context, P, T, M),
Architectural Assessment: :string:contains(Atom, "history").
Architectural Assessment: # one priority per atom: the max over its reasons; the reasons as a set (order is not stable)
Architectural Assessment: inject_prio(A, Atom, P) :-
Architectural Assessment: candidate(A, Atom, R, P0) |> do fn:group_by(A, Atom), let P = fn:max(P0).
Architectural Assessment: inject_reasons(A, Atom, Rs) :-
Architectural Assessment: candidate(A, Atom, R, P0) |> do fn:group_by(A, Atom), let Rs = fn:collect_distinct(R).
Architectural Assessment: # the window-shape decision: prefix sums by priority tier, no Go sort.
Architectural Assessment: # tie_sum is per tier; above_tier pairs tiers (quadratic in tiers, linear in atoms).
Architectural Assessment: tie_sum(A, P, S) :-
Architectural Assessment: inject_prio(A, Atom, P), prompt_atom(Atom, Cat, P0, T, M) |> do fn:group_by(A, P), let S = fn:sum(T).
Architectural Assessment: above_tier(A, P, Q, S) :- tie_sum(A, P, S0), tie_sum(A, Q, S), Q > P.
Architectural Assessment: above_sum(A, P, S) :-
Architectural Assessment: above_tier(A, P, Q, S0) |> do fn:group_by(A, P), let S = fn:sum(S0).
Architectural Assessment: has_above(A, P) :- above_sum(A, P, S).
Architectural Assessment: fits(A, Atom) :-
Architectural Assessment: inject_prio(A, Atom, P), above_sum(A, P, Above), tie_sum(A, P, Tie),
Architectural Assessment: compile_budget(A, B), Total = fn:plus(Above, Tie), Total <= B.
Architectural Assessment: fits(A, Atom) :-
Architectural Assessment: inject_prio(A, Atom, P), !has_above(A, P), tie_sum(A, P, Tie),
Architectural Assessment: compile_budget(A, B), Tie <= B.
Architectural Assessment: inject(A, Atom, Reason, Slot, P) :-
Architectural Assessment: fits(A, Atom), inject_prio(A, Atom, P), candidate(A, Atom, Reason, P0),
Architectural Assessment: prompt_atom(Atom, Cat, P1, T, M), window_slot(Cat, Slot).
Architectural Assessment: dropped(A, Atom, /budget) :-
Architectural Assessment: inject_prio(A, Atom, P), !fits(A, Atom).
Architectural Assessment: ```
Architectural Assessment: **Expected derived facts** (verified 2026-09-18 09:10, `mgv.exe -store simple -det=false -runs 5`, identical; `strata=11` — two more than the first draft's nine: `hit_boost` and `inject_reasons` are each their own aggregation stratum):
Architectural Assessment: ```
Architectural Assessment: inject("coder-7","identity/coder",/skeleton,/head,100)
Architectural Assessment: inject("coder-7","safety/no_secrets",/skeleton,/head,100)
Architectural Assessment: inject("coder-7","context/history/core.evaluate",/built_before,/tail,95)
Architectural Assessment: inject("coder-7","context/history/core.evaluate",/focal_symbol,/tail,95)
Architectural Assessment: inject("coder-7","knowledge/go/error_wrapping",/knowledge_hit,/middle,80)
Architectural Assessment: inject("coder-7","knowledge/go/table_tests",/knowledge_hit,/middle,60)
Architectural Assessment: dropped("coder-7","language/go/errors",/budget)
Architectural Assessment: dropped("coder-7","context/core.evaluate",/budget)
Architectural Assessment: hit_boost("coder-7","knowledge/go/error_wrapping",40)
Architectural Assessment: hit_boost("coder-7","knowledge/go/table_tests",20)
Architectural Assessment: candidate: 8 rows, one per (atom, reason)
Architectural Assessment: inject_reasons("coder-7","context/history/core.evaluate",[/focal_symbol, /built_before])   (a set; order varies between runs)
Architectural Assessment: tie_sum: (100,70) (95,20) (80,80) (60,70) (50,60) (30,90)
Architectural Assessment: above_sum: (95,70) (80,90) (60,170) (50,240) (30,300)
Architectural Assessment: ```
Architectural Assessment: The `inject` and `dropped` sets are byte-identical to the first draft's (the verifier's `v_patternB_300.mg` and `v_patternB_2agents.mg` follow-ons — budget 300 admits `language/go/errors`; a second agent with its own budget is scoped by `A` — hold unchanged, because `fits` compares the same numbers, now indexed by tier instead of by atom). Arithmetic the kernel did: tiers 100 (70 tokens) + 95 (20) + 80 (80) + 60 (70) = 240 ≤ 260; the tier-50 atom would make 300 and is cut; the tier-30 doc atom (90 tokens) is cut behind it. `knowledge/python/asyncio` is not in `inject_prio` at all — no rule admitted it — which is a different answer from `dropped`. Raising `compile_budget` to 300 admits `language/go/errors` (one fact); the same policy change in Go today is a constant in `budget.go`.
Architectural Assessment: **Where the two reasons for one atom go.** `context/history/core.evaluate` carries two `inject` rows (`/built_before`, `/focal_symbol`); Go groups by atom and keeps both reasons in the manifest, or reads `inject_reasons` — a `fn:collect_distinct` set whose order is map-iteration order (M0 §7, M3 §2B), so never compare or persist it as a sequence.
Architectural Assessment: ## 4. Pattern C — derived test obligations
Architectural Assessment: **Intent.** The tests an agent must write or run are derived from the north-star behaviors and the change set, keyed on *behavior*, not on line coverage: `test_obligation(Behavior, Target, Kind)` with `Kind ∈ {/write, /rerun}` and `coverage_missing(Behavior, Sym)`. Today the coder shard writes tests only if told (root `CLAUDE.md`, "Use codeNERD Where It Can Do the Job"), the only test-related verdict is `hollow_success("new source was created without a test file")` via `missing_test_for/1` (`coder_safety.mg:28,104-107`, file-level, created files only), and the test witness is one global atom with no test name and no revision — `test_state(/passing|/failing)` asserted by `handleRunTests` (`internal/core/virtual_store_actions.go:442-455`) and `tdd_loop.go:251-254,958`. `test_result/4` is declared (`schemas_analysis.mg:182`) but grep of `internal/core`, `internal/session`, `internal/shards` (non-test) finds no producer (**unverified** elsewhere). `test_impact.mg` already derives `impacted_test/1` from `plan_edit` + `test_depends_on_transitive` (`policy/test_impact.mg:72-140`) — but its producers were never wired with the right identifier shape (`internal/mangle/agents.md:16-20`), so it derives nothing.
Architectural Assessment: **Engine features relied on.** Linear recursion for `test_depends_on/2` closure (allowed, M0 §4 stratification; the corpus already has 18 self-recursive predicates, M1 §3); stratified negation over bound projections (`has_test_for/1`, `has_test_failure_now/1`, `test_green_now/1`); a revision join to age evidence (Pattern A rule 2). No aggregation is needed — obligations are a set.
Architectural Assessment: **Existing predicates it builds on (M1).** `code_element/5`, `code_calls/2`, `is_test_function/1`, `test_depends_on/2`, `test_depends_on_transitive/2`, `impacted_test/1` (`schemas_codedom.mg:33`, `schemas_analysis.mg:116`, `test_impact.mg:60-140`); `modified_file/1` (produced by `transaction_manager.go:610`), `turn_created_source/1` (`executor.go:2416`, Decl `schemas_execution.mg:245`), `plan_edit/1` (`schemas_codedom_polyglot.mg:206`, currently fed a file path instead of a ref, `agents.md:20`); `northstar_requirement/4`, `northstar_capability/4`, `northstar_supports/2` (`schemas_misc.mg:47-80`) and the `.nerd/northstar.mg` extension point (M1 §1.1); `test_state/1`, `failing_test/2` (`schemas_execution.mg:16`, `schemas_reviewer.mg:121`); `has_test_coverage/1`, `test_coverage/1` (`schemas_codedom_polyglot.mg:188`, `schemas_world.mg:38`).
Architectural Assessment: **The pattern** (revised in pass 2 — M3 §5 items 1, 7, 8, 9, 17; every rule below is the program the verifier's harness ran on 2026-09-18, `C:/Temp/mangle-study/m2v2/patternC_v2.mg`).
Architectural Assessment: 1. *Behaviors are facts with witnesses in the code.* `behavior(B, Text)` rows come from the north star (`northstar_requirement(ReqID, /behavior, Text, Prio)` is the natural source); `behavior_witness(B, Sym)` maps a behavior to the CodeDOM refs that realise it. The mapping is seeded by hand in `.nerd/northstar.mg` and can be learned (a `learned_behavior_witness` row promoted through the existing learned-rule path, `kernel_policy.go:360`), but it is never inferred by the model at task time.
Architectural Assessment: 2. *Tests witness behaviors through the dependency closure.* `behavior_test(B, T) :- behavior_witness(B, Sym), test_depends_on(T, Sym)` — this is `test_impact.mg`'s closure with the behavior as the key.
Architectural Assessment: 3. *The change set is per task.* `changed_symbol(Task, Sym, Rev)` is asserted by the transaction manager where the ref is known (`agents.md:32-35` says exactly where that is and is not), stamped with the task that made the change. **The `Rev` column is provenance, not a filter**: no rule in this pattern joins it, so an obligation created by a change at `rev-15` stays open at `rev-17` until a test exists — the obligation is "this task touched this behavior", and touching does not expire with a revision (M3 §2C `v_patternC_oldrev.mg` showed the column was never read; the first draft called it a "revision-stamped witness" as if it aged, which it does not). The task column is what makes obligations per task: `behavior_touched(Task, B)` joins the task's own change set, so a session with two active tasks (campaigns, parallel shards) does not cross-join every task with every behavior (M3 §2D `v_patternD_2tasks.mg`).
Architectural Assessment: 4. *Evidence expires with the revision, and a failure at the revision defeats a pass.* `test_green_now(T) :- test_run(Task, T, Rev, /pass), current_rev(Rev), !has_test_failure_now(T)` with `has_test_failure_now(T) :- test_run(Task, T, Rev, /fail), current_rev(Rev)`. A green run at `rev-16` is not evidence at `rev-17`; a `/pass` and a `/fail` both at `rev-17` (a flaky test, or a re-run after a fix that the harness recorded twice) is *not* green — the first draft's rule meant "some run at this revision passed", and the verifier's `v_patternC_cycle.mg` showed that made a failing test green. The witness is **`test_run(Task, TestRef, Rev, Outcome)`** — one shape, task-scoped, shared with Pattern G (the first draft declared a `/3` here and a `/4` there; two witnesses with one name and different arities are two predicates, and a producer that emits one leaves the other's rules silently empty, M3 §2 composition). If the harness can give runs an ordinal, `fn:max` over it and "the latest run passed" is the stronger rule; the negation form needs no new column.
Architectural Assessment: 5. *Obligations are the negations.* `test_obligation(Task, B, Target, Kind)`: `/write` when a touched behavior's changed symbol has no test at all; `/rerun` when a touched behavior's test is not green at the current revision; `coverage_missing(B, Sym)` for the untouched gap the north star still names (behavior-level, not task-level — it is a property of the codebase). `obligation_open(Task, B)` is the projection Pattern D negates. When the model adds a test for the `/write` target, the obligation does not vanish: the new test has no run at the current revision, so it becomes `/rerun` for that test (M3 `v_patternC_assert.mg`), and `obligation_open` stays true until the run is green. That is the intended chain — write, then run, then done.
Architectural Assessment: **What the harness asserts, and when.** After each edit: `changed_symbol(Task, Sym, Rev)` per touched ref and `current_rev(Rev)` (retract-and-assert of a one-row predicate through `Transaction()`, `kernel_transactions.go:42`; Pattern D carries the invariant rule that reports a second row as `kernel_invariant_violated(/current_rev, N)`); after each CodeDOM refresh: the `code_*` rows for touched files (or the Pattern A mount at the new hash). After each test run: one `test_run(Task, TestRef, Rev, Outcome)` per test the runner reported, parsed from `go test -json` (`internal/core/virtual_store_actions.go#handleRunTests` has the output, `:452-455`) — never a single global atom. After the model writes a test: nothing special; the next CodeDOM refresh adds the `is_test_function` and `code_calls` rows, the `/write` obligation stops deriving and the `/rerun` obligation for the new test starts.
Architectural Assessment: **Runnable example** (`C:/Temp/mangle-study/m2v2/patternC_v2.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternC_v2.mg test_obligation coverage_missing behavior_touched test_green_now has_test_failure_now obligation_open`):
Architectural Assessment: ```mangle
Architectural Assessment: # Pattern C v2 -- derived test obligations, per task (M3 items 1, 7, 8, 9, 17).
Architectural Assessment: Decl behavior(B, Description) bound [/string, /string].
Architectural Assessment: Decl behavior_witness(B, Sym) bound [/string, /string].
Architectural Assessment: Decl code_element(Ref, Kind, File, StartLine, EndLine) bound [/string, /name, /string, /number, /number].
Architectural Assessment: Decl code_calls(Caller, Callee) bound [/string, /string].
Architectural Assessment: Decl is_test_function(Ref) bound [/string].
Architectural Assessment: Decl current_rev(Rev) bound [/string].
Architectural Assessment: Decl task_active(Task) bound [/string].
Architectural Assessment: Decl changed_symbol(Task, Sym, Rev) bound [/string, /string, /string].
Architectural Assessment: Decl test_run(Task, TestRef, Rev, Outcome) bound [/string, /string, /string, /name].
Architectural Assessment: Decl test_depends_on(TestRef, Sym) bound [/string, /string].
Architectural Assessment: Decl behavior_test(B, TestRef) bound [/string, /string].
Architectural Assessment: Decl has_test_for(Sym) bound [/string].
Architectural Assessment: Decl behavior_touched(Task, B) bound [/string, /string].
Architectural Assessment: Decl has_test_failure_now(TestRef) bound [/string].
Architectural Assessment: Decl test_green_now(TestRef) bound [/string].
Architectural Assessment: Decl test_obligation(Task, B, Target, Kind) bound [/string, /string, /string, /name].
Architectural Assessment: Decl coverage_missing(B, Sym) bound [/string, /string].
Architectural Assessment: Decl obligation_open(Task, B) bound [/string, /string].
Architectural Assessment: # north star (.nerd/northstar.mg / northstar_requirement rows)
Architectural Assessment: behavior("B1", "stale evidence never survives a source change").
Architectural Assessment: behavior("B2", "a turn is done only with executed acceptance").
Architectural Assessment: behavior_witness("B1", "core.evaluate").
Architectural Assessment: behavior_witness("B1", "core.assert").
Architectural Assessment: behavior_witness("B2", "session.turnDone").
Architectural Assessment: # CodeDOM at the current revision
Architectural Assessment: code_element("core.evaluate", /function, "internal/core/kernel_eval.go", 181, 260).
Architectural Assessment: code_element("core.assert", /function, "internal/core/kernel_facts.go", 555, 600).
Architectural Assessment: code_element("session.turnDone", /function, "internal/session/executor.go", 2330, 2360).
Architectural Assessment: code_element("core.TestEvaluateFresh", /function, "internal/core/kernel_eval_test.go", 10, 40).
Architectural Assessment: code_element("session.TestTurnDone", /function, "internal/session/executor_test.go", 5, 30).
Architectural Assessment: is_test_function("core.TestEvaluateFresh").
Architectural Assessment: is_test_function("session.TestTurnDone").
Architectural Assessment: code_calls("core.TestEvaluateFresh", "core.evaluate").
Architectural Assessment: code_calls("session.TestTurnDone", "session.turnDone").
Architectural Assessment: # change set and test runs: per task, asserted by the harness after each edit / run
Architectural Assessment: current_rev("rev-17").
Architectural Assessment: task_active("t1").
Architectural Assessment: task_active("t2").
Architectural Assessment: changed_symbol("t1", "core.evaluate", "rev-17").
Architectural Assessment: changed_symbol("t1", "core.assert", "rev-17").
Architectural Assessment: changed_symbol("t2", "session.turnDone", "rev-17").
Architectural Assessment: test_run("t1", "core.TestEvaluateFresh", "rev-16", /pass).
Architectural Assessment: test_run("t2", "session.TestTurnDone", "rev-17", /pass).
Architectural Assessment: test_run("t2", "session.TestTurnDone", "rev-17", /fail).
Architectural Assessment: test_depends_on(T, Sym) :- is_test_function(T), code_calls(T, Sym).
Architectural Assessment: test_depends_on(T, Sym) :- test_depends_on(T, Mid), code_calls(Mid, Sym).
Architectural Assessment: behavior_test(B, T) :- behavior_witness(B, Sym), test_depends_on(T, Sym).
Architectural Assessment: has_test_for(Sym) :- test_depends_on(T, Sym).
Architectural Assessment: # touched is per task: the task's own change set, not every task's
Architectural Assessment: behavior_touched(Task, B) :- changed_symbol(Task, Sym, Rev), behavior_witness(B, Sym).
Architectural Assessment: # Evidence is current only at the current revision, and a failure at that
Architectural Assessment: # revision defeats a pass at the same revision (flaky or re-run tests).
Architectural Assessment: has_test_failure_now(T) :- test_run(Task, T, Rev, /fail), current_rev(Rev).
Architectural Assessment: test_green_now(T) :- test_run(Task, T, Rev, /pass), current_rev(Rev), !has_test_failure_now(T).
Architectural Assessment: test_obligation(Task, B, Sym, /write) :-
Architectural Assessment: behavior_touched(Task, B), behavior_witness(B, Sym), changed_symbol(Task, Sym, Rev), !has_test_for(Sym).
Architectural Assessment: test_obligation(Task, B, T, /rerun) :-
Architectural Assessment: behavior_touched(Task, B), behavior_test(B, T), !test_green_now(T).
Architectural Assessment: coverage_missing(B, Sym) :- behavior_witness(B, Sym), !has_test_for(Sym).
Architectural Assessment: obligation_open(Task, B) :- test_obligation(Task, B, Target, Kind).
Architectural Assessment: ```
Architectural Assessment: **Expected derived facts** (verified 2026-09-18 08:40 with the verifier's harness, `-store simple -det=false -runs 5`, identical across runs; `strata=9`):
Architectural Assessment: ```
Architectural Assessment: behavior_touched("t1","B1")
Architectural Assessment: behavior_touched("t2","B2")
Architectural Assessment: has_test_failure_now("session.TestTurnDone")
Architectural Assessment: test_obligation("t1","B1","core.TestEvaluateFresh",/rerun)
Architectural Assessment: test_obligation("t1","B1","core.assert",/write)
Architectural Assessment: test_obligation("t2","B2","session.TestTurnDone",/rerun)
Architectural Assessment: coverage_missing("B1","core.assert")
Architectural Assessment: obligation_open("t1","B1")
Architectural Assessment: obligation_open("t2","B2")
Architectural Assessment: ```
Architectural Assessment: `test_green_now` is empty: t1's only run is one revision old, and t2's test has a `/pass` *and* a `/fail` at `rev-17`, so it is not green and B2 — touched by t2 — carries a `/rerun`. t1 is touched twice: `core.evaluate` has a test whose last green run is one revision old (`/rerun`), `core.assert` has no test (`/write`). Nothing of t1's leaks into t2 or back. Asserting `test_run("t1","core.TestEvaluateFresh","rev-17",/pass)` removes t1's `/rerun` row; adding a test that calls `core.assert` (two CodeDOM rows + `is_test_function`) removes `/write` and `coverage_missing` and adds `test_obligation("t1","B1","core.TestAssert",/rerun)` until that test runs green — no Go code decides any of it.
Architectural Assessment: **Identifier shape.** Every `Sym`/`Ref` column is the Cartographer's `<pkg>.<Name>` string (`agents.md:27-31`). The example uses `core.evaluate`-style refs deliberately; a producer that emits bare names or file paths makes every rule above derive nothing, silently, and only the end-to-end kernel test of §1 catches it. No rule above has a wildcard in a positive premise (R1, §7) or in an aggregation body (§1 convention 6) — the unused variables (`Task` in `test_green_now`, `Rev` in `behavior_touched`) are named so the proof keeps the leaf.
Architectural Assessment: ## 5. Pattern D — completion as obligation fixpoint
Architectural Assessment: **Intent.** A task is not done when the model says so; it is done when nothing obliges more work. `task_incomplete(Task, Kind, Target)` is the union of every open obligation from the other patterns, `task_complete(Task)` is its negation, and the loop reads one row, `loop_verdict(Task, /complete | /continue | /stop_stalled)`. Progress, stalls and repeated failures are derived from round witnesses; the iteration cap and the adaptive extension logic in `internal/session/tool_budget_controller.go:192-238` (`maybeExtend`: hard limit, extension count, repeated tail cycle, "no novel successful tool result", "write-oriented intent stalled") become policy facts and rules — the same move `working_set.mg:53-110` already made for `working_stop`, `working_nudge` and `working_finalize`, but in the executive kernel and gating completion, not just continuation. `working_continue()`'s own doc says it: "Continuation requires no observed stall; it never means task completion" (`working_set.mg:51`).
Architectural Assessment: **Engine features relied on.** Stratified negation over zero/one-arity projections (`has_obligation/1`, `has_acceptance_now/1`, `stalled/1`; M0 §4, §12 item 1); `fn:max` and `fn:count` aggregation, each its own stratum (M0 §6); `fn:minus` and `fn:string:concat` (M0 §5); fact-valued thresholds compared with `>=`. Thirteen strata in the revised example (ten in the first draft; the failure window and the `current_rev` invariant add three); the union predicate `obligation/3` is the seam every other pattern writes into, and every source predicate carries a task column.
Architectural Assessment: **Existing predicates it builds on (M1).** `turn_evidence/6`, `hollow_success/1`, `turn_executed/1`, `turn_done/1`, `turn_acceptance/3` (`coder_safety.mg:79-115`) — the only completion verdict that is derived today, and it is per *turn*, not per task; `working_progress/5`, `working_control/2`, `working_stop/1`, `working_stall_rounds/1`, `working_nudge_rounds/1` (`working_set.mg:33-57`); `task_status`/`task_completed` in the model-observation allowlist (`mangle_updates.go:30-53`) — model-assertable, and by this pattern never an obligation source; `test_obligation/4` (Pattern C, task-scoped), `edit_plan`/`edit_step_pending` (Pattern E), `verdict` and `unverified_claim` facts (Pattern G), `acceptance_state`/`workspace_snapshot` (M1 §7 row 7).
Architectural Assessment: **The pattern** (revised in pass 2 — M3 §5 items 1, 2, 16; the program below is `C:/Temp/mangle-study/m2v2/patternD_v3.mg`, run on the verifier's harness 2026-09-18).
Architectural Assessment: 1. *Obligations are a union predicate with a kind column, joined on the task.* One rule per source: `/test_write`, `/test_rerun` (Pattern C — `test_obligation(Task, B, Target, Kind)` carries the task, so the join is `test_obligation(T, B, Target, /write)`, never `task_active(T), test_obligation(_, Target, /write)`: the first draft's cross product gave every active task every other task's obligations, M3 `v_patternD_2tasks.mg`), `/edit_step` (Pattern E), `/acceptance` (Pattern G — no `acceptance(T, Rev)` at the *current* revision), `/hollow` (Pattern G's `unverified_claim(T, Claim)`, task-scoped by construction). New obligation kinds are new rules; the loop never learns about them. Every source predicate in the union has a task column — that is the contract this pattern imposes on the others.
Architectural Assessment: 2. *Complete is the negation.* `task_complete(T) :- task_active(T), !has_obligation(T).` Nothing the model asserts can make this true: `task_status`/`task_completed` are not in any obligation rule, and `turn_done` is host-witnessed (`mangle_updates.go:145-150`).
Architectural Assessment: 3. *Progress is a derived comparison over round witnesses; repeated failure is a window, not a lifetime total.* At each round boundary Go asserts `round_done(T, R)`, `open_count_at(T, R, N)` (the kernel's own `obligation` count from the previous fixpoint, stamped — derived facts are not persisted across evaluations, M1 §1.3, so history of a derivation is a witness) and `tool_failure(T, R, Digest)` for every failed tool call, `Digest` being a hash of (tool, args, error class). `stall(T, /no_progress)` holds when the open count has not fallen over `stall_rounds(K)`; `stall(T, /repeated_failure)` when one digest recurs `repeated_failure_rounds(K)` times **within the last K rounds** — `recent_failure(T, R, D)` keeps only rows with `R > latest_round − K`, and `failure_count` counts those. The first draft counted the whole history, which stopped a task that hit the same build error at rounds 1, 5 and 9 with progress in between (M3 `v_patternD_spread.mg`); the controller's "period-1..3 repeated trace" detection (`tool_budget_controller.go:276-302`) looks at the tail of the trace, and the window is what makes the rule the same decision. `stall(T, /spend_ceiling)` when the meter's `spent(T, Tokens)` (from `.nerd/meter/receipts.jsonl`, §10) reaches `spend_ceiling(T, Tokens)`.
Architectural Assessment: 4. *The verdict is one row per active task.* `/complete` when complete; `/stop_stalled` when obligations remain and a stall holds; `/continue` otherwise. The loop cannot end on `/continue` and cannot continue on `/complete` — and there is no round number in any rule.
Architectural Assessment: 5. *What replaces tool budgets and iteration caps.* Nothing counts tool calls. The only ceilings are facts: `spend_ceiling/2` (tokens, from the meter, the user's real constraint), `stall_rounds/1`, `repeated_failure_rounds/1`. `ExecutorConfig`'s iteration limit, `maxExtensions`, `extensionSize`, `hardLimit` (`tool_budget_controller.go:21,79-96`) are deleted, not moved; "arbitrary tool-call counts are not task-completion criteria" (root `CLAUDE.md`, North Star). The nudges (`working_nudge/1`) stay as steering facts the loop appends to a tool result; they are not verdicts.
Architectural Assessment: 6. *Kernel invariants are facts too.* `current_rev/1` must be one row — every revision join in Patterns C, E, G widens silently if it is not (M3 composition: two `current_rev` rows flipped a Pattern G verdict). The invariant is a rule, `kernel_invariant_violated(/current_rev, N)` from a `fn:count` over `current_rev`, and the loop treats any `kernel_invariant_violated` row as `/stop_stalled` with that reason (a rule to add when the loop lands; the example prints the fact). An alternative is a `fundep([], [Rev])` + `merge` Decl (M0 §3 item 4), which makes the second assert *replace* the first instead of reporting it — replacing hides the producer bug, reporting names it, so the design reports.
Architectural Assessment: **Runnable example** (`C:/Temp/mangle-study/m2v2/patternD_v3.mg`; run `& C:\Temp\mangle-study\verify\mgv.exe -store simple -det=false -runs 5 patternD_v3.mg loop_verdict task_incomplete stall failure_count recent_failure latest_round kernel_invariant_violated`):
Architectural Assessment: ```mangle
Architectural Assessment: # Pattern D v3 -- completion as an obligation fixpoint; per-task obligations,
Architectural Assessment: # repeated failures windowed over the last K rounds (M3 items 1, 2, 16).
Architectural Assessment: Decl task_active(Task) bound [/string].
Architectural Assessment: Decl round_done(Task, Round) bound [/string, /number].
Architectural Assessment: Decl open_count_at(Task, Round, N) bound [/string, /number, /number].
Architectural Assessment: Decl tool_failure(Task, Round, Digest) bound [/string, /number, /string].
Architectural Assessment: Decl spent(Task, Tokens) bound [/string, /number].
Architectural Assessment: Decl test_obligation(Task, B, Target, Kind) bound [/string, /string, /string, /name].
Architectural Assessment: Decl edit_step_pending(Task, Step) bound [/string, /number].
Architectural Assessment: Decl acceptance(Task, Rev) bound [/string, /string].
Architectural Assessment: Decl current_rev(Rev) bound [/string].
Architectural Assessment: Decl unverified_claim(Task, Claim) bound [/string, /name].
Architectural Assessment: Decl stall_rounds(N) bound [/number].
Architectural Assessment: Decl repeated_failure_rounds(N) bound [/number].
Architectural Assessment: Decl spend_ceiling(Task, Tokens) bound [/string, /number].
Architectural Assessment: Decl obligation(Task, Kind, Target) bound [/string, /name, /string].
Architectural Assessment: Decl has_obligation(Task) bound [/string].
Architectural Assessment: Decl has_acceptance_now(Task) bound [/string].
Architectural Assessment: Decl task_incomplete(Task, Kind, Target) bound [/string, /name, /string].
Architectural Assessment: Decl task_complete(Task) bound [/string].
Architectural Assessment: Decl latest_round(Task, R) bound [/string, /number].
Architectural Assessment: Decl recent_failure(Task, Round, Digest) bound [/string, /number, /string].
Architectural Assessment: Decl failure_count(Task, Digest, N) bound [/string, /string, /number].
Architectural Assessment: Decl stall(Task, Reason) bound [/string, /name].
Architectural Assessment: Decl stalled(Task) bound [/string].
Architectural Assessment: Decl loop_verdict(Task, Verdict) bound [/string, /name].
Architectural Assessment: Decl current_rev_count(N) bound [/number].
Architectural Assessment: Decl kernel_invariant_violated(Pred, N) bound [/name, /number].
Architectural Assessment: stall_rounds(3).
Architectural Assessment: repeated_failure_rounds(3).
Architectural Assessment: spend_ceiling("t1", 200000).
Architectural Assessment: task_active("t1").
Architectural Assessment: task_active("t2").
Architectural Assessment: current_rev("rev-17").
Architectural Assessment: round_done("t1", 1). round_done("t1", 2). round_done("t1", 3). round_done("t1", 4). round_done("t1", 5).
Architectural Assessment: open_count_at("t1", 1, 3). open_count_at("t1", 2, 3). open_count_at("t1", 3, 2).
Architectural Assessment: open_count_at("t1", 4, 2). open_count_at("t1", 5, 2).
Architectural Assessment: tool_failure("t1", 3, "sha:go-build-undefined-foo").
Architectural Assessment: tool_failure("t1", 4, "sha:go-build-undefined-foo").
Architectural Assessment: tool_failure("t1", 5, "sha:go-build-undefined-foo").
Architectural Assessment: spent("t1", 84000).
Architectural Assessment: test_obligation("t1", "B1", "core.assert", /write).
Architectural Assessment: edit_step_pending("t1", 2).
Architectural Assessment: acceptance("t1", "rev-16").
Architectural Assessment: # t2: a second active task with nothing open
Architectural Assessment: round_done("t2", 1).
Architectural Assessment: open_count_at("t2", 1, 0).
Architectural Assessment: acceptance("t2", "rev-17").
Architectural Assessment: # obligations: a union over every pattern's open items, joined on the task
Architectural Assessment: obligation(T, /test_write, Target) :- test_obligation(T, B, Target, /write).
Architectural Assessment: obligation(T, /test_rerun, Target) :- test_obligation(T, B, Target, /rerun).
Architectural Assessment: obligation(T, /edit_step, Step) :- edit_step_pending(T, N), Step = fn:string:concat("step-", N).
Architectural Assessment: obligation(T, /acceptance, "acceptance at current revision") :- task_active(T), !has_acceptance_now(T).
