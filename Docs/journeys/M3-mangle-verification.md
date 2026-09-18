# M3 — Mangle verification (adversarial)

## Status

- last updated: Fri, Sep 18, 2026  8:21:00 AM
- done: 1 Harness; 2A (holds; E1 confirmed; surprises A1 empty-external re-call, A2 filter cache key); 2B (holds; collect note); 2C (holds; flaky-test and dead-Rev-column findings); 2D (holds as written; refuted per-task and repeated-failure semantics); 2E (holds for one change; refuted for two; surprise E1 count semantics); 2F (holds; rule text is rewritten form; leaf list corrected); 2G (holds; T1 wider; tests-only and flaky-build gaps); composition (duplicate Decls refuse to load; strata add linearly; current_rev doubles); 3 M0 claims (22 rows; open item (a) resolved to types.go:246); 4 engine surprises (14); 5 refutation list (23)
- open: (a) M0 probe DD (deferred cycle = fatal stack overflow) not re-run; (b) M0 §7 "store left partially updated on error" not observed; (c) the `"etc/passwd"`/`"passwd"` 1-row halves of `agents.md:57-59` unexplained; (d) codeNERD's differential engine not tested against the stale-negation program `m0_AN1/2.mg`; (e) the seven patterns were run through the raw engine and as one concatenated unit, not through `rebuildProgram` beside the shipped policy (would fail on duplicate Decls before anything else)

Role: adversarial verifier for the codeNERD Mangle workstream. Every claim below cites
`path:line`, `path#symbol`, or a predicate in a named `.mg` file; anything I could not run
says **unverified**. Default verdict when a program does not behave as claimed: **refuted**.

## 1. Harness

**Scratch root:** `C:/Temp/mangle-study/verify/` (module `verify`, `go.mod`/`go.sum` copied from `C:/Temp/mangle-study/m2/`, requiring `codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3`; resolves offline from `C:/Users/smoor/go/pkg/mod/codeberg.org/!tau!ceti/mangle-go@v0.5.1-0.20260413190942-4dcaa582c6d3`, read-only).

**Build (bash, never `cd` — a `cd` in this harness hangs on a permission prompt):**

```
GOPROXY=off GOFLAGS=-mod=mod go build -C /c/Temp/mangle-study/verify -o /c/Temp/mangle-study/verify/mgv.exe .
```

**Run (PowerShell only — running the exe from the Git-Bash tool returns no output until the 20 s timeout, then completes in the background; PowerShell returns instantly):**

```
& C:\Temp\mangle-study\verify\mgv.exe [-store simple|multi] [-det=false] [-prov] [-ext] [-rules] [-runs N] [-limit N] [-reeval second.mg] <file.mg> [pred ...]
```

- `-store` picks `SimpleInMemoryStore` (codeNERD's choice below 1024 facts) or `MultiIndexedArrayInMemoryStore` (default, what M2's harness used).
- `-det=false` drops `WithDeterministicOrder`; `-runs N` evaluates N times on fresh stores and prints any run whose output set differs.
- `-rules` prints `ProgramInfo.Rules` (the rewritten clauses that actually run). The negation-dropped check is a **warning**, not a refusal, so the engine's real behaviour is observable.
- `-ext` registers stub externals `recall_similar_all/4`, `sqlite_symbol_doc_all/3` (the M2 stubs), `sqlite_symbol_doc/3` (`+,+,-`, E1 probe), `empty_all/2` (returns no rows), `dbl_ext/2` (`+,-`); every call is logged with its inputs/filters and a total call count is printed.
- `-reeval second.mg` evaluates a second program on the **same** store after the first and prints the requested predicates again (retraction probe).
- `-prov` records with `provenance.MemoryRecorder`, builds up to 3 proofs (depth 12) of the first fact of the first listed predicate, prints them and the `EmitFacts` rows (sorted).
- Pipeline is exactly M0 §7: `parse.Unit` → `analysis.AnalyzeOneUnit(unit, nil)` → `analysis.Stratify` → `engine.EvalStratifiedProgramWithStats` with `WithCreatedFactLimit(100000)`.

M2's own `.mg` files were copied verbatim into the scratch dir (`patternA.mg` … `patternG.mg`, `patternD_v2.mg`, `patternE_v2.mg`, `smoke.mg`, `prov_*.mg`, `trailing_name*.mg`, `extA_*.mg`). New probe programs written by this study are `C:/Temp/mangle-study/verify/v_*.mg` and are quoted inline below.

**Driver source** (`C:/Temp/mangle-study/verify/main.go`, final version incl. `-reeval`):

```go
// mgv: adversarial verifier driver for the pinned mangle-go engine.
// parse -> AnalyzeOneUnit -> Stratify -> EvalStratifiedProgramWithStats, print facts.
// Usage: mgv [-store simple|multi] [-det] [-prov] [-ext] [-rules] [-runs N] [-limit N] file.mg [pred ...]
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/parse"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

var callCount = map[string]int{}

type demoExternal struct {
	name string
	rows func(inputs []ast.Constant) [][]ast.Constant
}

func (d demoExternal) ShouldQuery(inputs []ast.Constant, _ []ast.BaseTerm, _ []ast.Term) bool {
	return true
}
func (d demoExternal) ShouldPushdown() bool { return false }
func (d demoExternal) ExecuteQuery(inputs []ast.Constant, filters []ast.BaseTerm, _ []ast.Term, cb func([]ast.BaseTerm)) error {
	callCount[d.name]++
	fmt.Printf("  [external %s call #%d inputs=%v filters=%v]\n", d.name, callCount[d.name], inputs, filters)
	for _, r := range d.rows(inputs) {
		bt := make([]ast.BaseTerm, len(r))
		for i, c := range r {
			bt[i] = c
		}
		cb(bt)
	}
	return nil
}

func countNeg(ps []ast.Term) int {
	n := 0
	for _, p := range ps {
		if _, ok := p.(ast.NegAtom); ok {
			n++
		}
	}
	return n
}

func externals(pi *analysis.ProgramInfo) map[ast.PredicateSym]engine.ExternalPredicateCallback {
	exts := map[ast.PredicateSym]engine.ExternalPredicateCallback{}
	q := ast.String("wrap errors with context")
	rev := ast.String("rev-17")
	exts[ast.PredicateSym{Symbol: "recall_similar_all", Arity: 4}] = demoExternal{name: "recall_similar_all", rows: func(in []ast.Constant) [][]ast.Constant {
		return [][]ast.Constant{
			{q, rev, ast.String("knowledge/go/error_wrapping"), ast.Number(91)},
			{q, rev, ast.String("knowledge/go/table_tests"), ast.Number(74)},
			{q, rev, ast.String("knowledge/python/asyncio"), ast.Number(31)},
			{q, ast.String("rev-16"), ast.String("knowledge/go/stale_atom"), ast.Number(99)},
		}
	}}
	exts[ast.PredicateSym{Symbol: "sqlite_symbol_doc_all", Arity: 3}] = demoExternal{name: "sqlite_symbol_doc_all", rows: func(in []ast.Constant) [][]ast.Constant {
		return [][]ast.Constant{
			{ast.String("core.evaluate"), rev, ast.String("evaluate runs the stratified fixpoint")},
			{ast.String("core.assert"), rev, ast.String("assert adds an EDB fact")},
		}
	}}
	// (+,+,-) form for the E1 probe
	exts[ast.PredicateSym{Symbol: "sqlite_symbol_doc", Arity: 3}] = demoExternal{name: "sqlite_symbol_doc", rows: func(in []ast.Constant) [][]ast.Constant {
		return [][]ast.Constant{{ast.String("doc for " + in[0].Symbol)}}
	}}
	// all-output external that returns nothing: how often is it called?
	exts[ast.PredicateSym{Symbol: "empty_all", Arity: 2}] = demoExternal{name: "empty_all", rows: func(in []ast.Constant) [][]ast.Constant {
		return nil
	}}
	// (+,-) external for call-count probing
	exts[ast.PredicateSym{Symbol: "dbl_ext", Arity: 2}] = demoExternal{name: "dbl_ext", rows: func(in []ast.Constant) [][]ast.Constant {
		return [][]ast.Constant{{ast.Number(in[0].NumValue * 2)}}
	}}
	for sym := range exts {
		if _, ok := pi.Decls[sym]; !ok {
			delete(exts, sym)
		}
	}
	return exts
}

func main() {
	storeKind := flag.String("store", "multi", "simple|multi")
	det := flag.Bool("det", true, "WithDeterministicOrder")
	prov := flag.Bool("prov", false, "proof of first fact of first predicate")
	ext := flag.Bool("ext", false, "register stub externals")
	rules := flag.Bool("rules", false, "print ProgramInfo.Rules (rewritten)")
	runs := flag.Int("runs", 1, "evaluate N times on fresh stores; report if outputs differ")
	limit := flag.Int("limit", 100000, "WithCreatedFactLimit")
	reeval := flag.String("reeval", "", "second .mg evaluated on the SAME store after the first; prints requested predicates again")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("usage: mgv [flags] file.mg [pred ...]")
		os.Exit(2)
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Println("READ ERROR:", err)
		os.Exit(1)
	}
	unit, err := parse.Unit(strings.NewReader(string(src)))
	if err != nil {
		fmt.Println("PARSE ERROR:", err)
		os.Exit(1)
	}
	pi, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		fmt.Println("ANALYSIS ERROR:", err)
		os.Exit(1)
	}
	if *rules {
		for _, r := range pi.Rules {
			fmt.Println("RULE:", r.String())
		}
	}
	for _, c := range unit.Clauses {
		want := countNeg(c.Premises)
		if want == 0 {
			continue
		}
		for _, r := range pi.Rules {
			if r.Head.String() != c.Head.String() || len(r.Premises) >= len(c.Premises) {
				continue
			}
			if countNeg(r.Premises) < want {
				fmt.Printf("WARNING negation dropped by rewrite: %s  ->  %s\n", c.String(), r.String())
			}
		}
	}
	strata, predToStratum, err := analysis.Stratify(analysis.Program{EdbPredicates: pi.EdbPredicates, IdbPredicates: pi.IdbPredicates, Rules: pi.Rules})
	if err != nil {
		fmt.Println("STRATIFY ERROR:", err)
		os.Exit(1)
	}
	want := map[string]bool{}
	for _, p := range args[1:] {
		want[p] = true
	}
	var prevOut string
	for run := 0; run < *runs; run++ {
		var store factstore.FactStore
		if *storeKind == "simple" {
			store = factstore.NewSimpleInMemoryStore()
		} else {
			store = factstore.NewMultiIndexedArrayInMemoryStore()
		}
		opts := []engine.EvalOption{engine.WithCreatedFactLimit(*limit)}
		if *det {
			opts = append(opts, engine.WithDeterministicOrder())
		}
		var rec *provenance.MemoryRecorder
		if *prov {
			rec = provenance.NewMemoryRecorder()
			opts = append(opts, engine.WithDerivationRecorder(rec))
		}
		if *ext {
			opts = append(opts, engine.WithExternalPredicates(externals(pi)))
		}
		stats, err := engine.EvalStratifiedProgramWithStats(pi, strata, predToStratum, store, opts...)
		if err != nil {
			fmt.Println("EVAL ERROR:", err)
			os.Exit(1)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "strata=%d\n", len(stats.Strata))
		preds := store.ListPredicates()
		sort.Slice(preds, func(i, j int) bool { return preds[i].Symbol < preds[j].Symbol })
		var firstFact *ast.Atom
		for _, p := range preds {
			if len(want) > 0 && !want[p.Symbol] {
				continue
			}
			if len(want) == 0 {
				if _, idb := pi.IdbPredicates[p]; !idb {
					continue
				}
			}
			var out []string
			store.GetFacts(ast.NewQuery(p), func(a ast.Atom) error {
				out = append(out, a.String())
				if firstFact == nil && len(args) > 1 && p.Symbol == args[1] {
					aa := a
					firstFact = &aa
				}
				return nil
			})
			if os.Getenv("MGV_KEEP_ORDER") != "1" {
				sort.Strings(out)
			}
			fmt.Fprintf(&sb, "%s/%d (%d):\n", p.Symbol, p.Arity, len(out))
			for _, s := range out {
				fmt.Fprintf(&sb, "  %s\n", s)
			}
		}
		if *reeval != "" && run == 0 {
			src2, err := os.ReadFile(*reeval)
			if err != nil {
				fmt.Println("READ ERROR:", err)
				os.Exit(1)
			}
			unit2, err := parse.Unit(strings.NewReader(string(src2)))
			if err != nil {
				fmt.Println("PARSE ERROR (reeval):", err)
				os.Exit(1)
			}
			pi2, err := analysis.AnalyzeOneUnit(unit2, nil)
			if err != nil {
				fmt.Println("ANALYSIS ERROR (reeval):", err)
				os.Exit(1)
			}
			strata2, pts2, err := analysis.Stratify(analysis.Program{EdbPredicates: pi2.EdbPredicates, IdbPredicates: pi2.IdbPredicates, Rules: pi2.Rules})
			if err != nil {
				fmt.Println("STRATIFY ERROR (reeval):", err)
				os.Exit(1)
			}
			before := store.EstimateFactCount()
			if _, err := engine.EvalStratifiedProgramWithStats(pi2, strata2, pts2, store, engine.WithCreatedFactLimit(*limit)); err != nil {
				fmt.Println("EVAL ERROR (reeval):", err)
				os.Exit(1)
			}
			fmt.Fprintf(&sb, "--- after reeval of %s on the same store (facts before=%d after=%d) ---\n", *reeval, before, store.EstimateFactCount())
			for _, p := range preds {
				if len(want) > 0 && !want[p.Symbol] {
					continue
				}
				var out []string
				store.GetFacts(ast.NewQuery(p), func(a ast.Atom) error { out = append(out, a.String()); return nil })
				sort.Strings(out)
				fmt.Fprintf(&sb, "%s/%d (%d):\n", p.Symbol, p.Arity, len(out))
				for _, s := range out {
					fmt.Fprintf(&sb, "  %s\n", s)
				}
			}
		}
		cur := sb.String()
		if run == 0 {
			fmt.Print(cur)
		} else if cur != prevOut {
			fmt.Printf("RUN %d DIFFERS:\n%s", run, cur)
		}
		prevOut = cur
		if *prov && rec != nil && firstFact != nil && run == 0 {
			fmt.Printf("recorder events=%d\n", len(rec.Events()))
			proofs, err := provenance.BuildFromRecording(rec, store, *firstFact, provenance.Options{MaxProofs: 3, MaxDepth: 12})
			if err != nil {
				fmt.Println("PROVENANCE ERROR:", err)
				return
			}
			fmt.Printf("--- proofs of %s: %d ---\n", firstFact.String(), len(proofs))
			provenance.Print(os.Stdout, proofs)
			pstore := factstore.NewMultiIndexedArrayInMemoryStore()
			if err := provenance.EmitFacts(proofs, pstore); err != nil {
				fmt.Println("EMIT ERROR:", err)
			}
			fmt.Println("--- EmitFacts ---")
			ps := pstore.ListPredicates()
			sort.Slice(ps, func(i, j int) bool { return ps[i].Symbol < ps[j].Symbol })
			for _, p := range ps {
				var out []string
				pstore.GetFacts(ast.NewQuery(p), func(a ast.Atom) error { out = append(out, a.String()); return nil })
				sort.Strings(out)
				for _, s := range out {
					fmt.Printf("  %s\n", s)
				}
			}
		}
	}
	if *runs > 1 {
		fmt.Printf("runs=%d (differences reported above, if any)\n", *runs)
	}
	if *ext {
		fmt.Printf("external call counts: %v\n", callCount)
	}
}
```

## 2. Patterns A–G (M2)

Conventions: "expected" is the block M2 prints under *Expected derived facts*; "actual" is `mgv.exe` output. A pattern "holds" only if the derived set is identical on **both** stores (`-store simple` is what codeNERD runs below 1024 facts, `kernel_eval.go:842-846`) and identical across 5 runs without `WithDeterministicOrder` (`-det=false -runs 5`).

### Pattern A — mounting knowledge as virtual predicates

Program as run: `C:/Temp/mangle-study/verify/patternA.mg`, byte-identical to the block in M2 §2 (copied from `C:/Temp/mangle-study/m2/patternA.mg`).

Commands and actual output:

```
& mgv.exe -ext patternA.mg knowledge_hit knowledge_hit_offlang symbol_doc knowledge_stale
& mgv.exe -ext -store simple -det=false -runs 5 patternA.mg knowledge_hit knowledge_hit_offlang symbol_doc knowledge_stale
strata=4
knowledge_hit/3 (3):
  knowledge_hit("t1","knowledge/go/error_wrapping",91)
  knowledge_hit("t1","knowledge/go/table_tests",74)
  knowledge_hit("t1","knowledge/python/asyncio",31)
knowledge_hit_offlang/2 (1):
  knowledge_hit_offlang("t1","knowledge/python/asyncio")
knowledge_stale/2 (1):
  knowledge_stale("knowledge/go/stale_atom","rev-16")
symbol_doc/3 (1):
  symbol_doc("t1","core.evaluate","evaluate runs the stratified fixpoint")
external call counts: map[recall_similar_all:1 sqlite_symbol_doc_all:1]   (per run; 5 runs identical)
```

`v_patternA_60.mg` (threshold 60): `knowledge_hit` 2 rows, `knowledge_hit_offlang` 0 rows — as M2 states.

**Verdict: holds** (both stores, 5 nondeterministic runs identical, each external called once per evaluation).

**E1 re-tested independently (holds, and is worse than M2 says).**

- `extA_min.mg` (`out(S,D) :- focus(S), sqlite_symbol_doc(S, "rev-17", D).`, mode `+,+,-`): `panic: interface conversion: ast.BaseTerm is ast.Variable, not ast.Constant` at `engine/topdown.go:99` from `engine/seminaivebottomup.go:831`. Confirmed from source: `oneStepEvalPremise` passes `p` (the unsubstituted premise atom) to `EvalExternalQuery` (`seminaivebottomup.go:831`), while the store lookup two lines later goes through `premiseAtom(p, lookupFn, subst)` (`:851`); `EvalExternalQuery` does `arg.(ast.Constant)` on every input position (`topdown.go:99`). M2's diagnosis is correct.
- `extA_const.mg` (both inputs constants): `out("doc for core.evaluate")`, 1 call. Holds.
- New: `v_ext_plus_var.mg` (`q(X,Y) :- p(X), dbl_ext(X,Y).`, mode `+,-`) — same panic. So E1 is not specific to string inputs or to a mixed constant/variable input list.
- New: `v_ext_plus_first.mg` (`q(X,Y) :- dbl_ext(X,Y), p(X).`, external **first**, `X` bound only by the later `p(X)`) — **passes `AnalyzeOneUnit`** (positive atoms are order-independent for safety, M0 §4) and then panics identically. M0 §3's "an unbound variable in an input position is an error" (probe AE) holds only when *no* positive atom anywhere in the body binds the variable; written order is not checked for moded user predicates, only for builtins. Note for the analyzer gate: a `+`-moded external anywhere but after its binders is a runtime panic, not an analysis error.
- New: `v_ext_plus_const.mg` — `q(Y) :- dbl_ext(4, Y).` and `r(Y) :- p(X), dbl_ext(4, Y).` (3 `p` rows): **1 call total**; the second rule's three premise evaluations hit the store cache (`topdown.go:84-93`). The once-per-input-tuple cache is real for constant inputs.

**Engine surprise A1 — an all-output external that returns no rows is called on every premise evaluation.** `v_ext_empty.mg`:

```mangle
Decl empty_all(K, V) descr [external(), mode("-", "-")] bound [/string, /string].
Decl p(X) bound [/number].
Decl q(X, K, V) bound [/number, /string, /string].
Decl r(X) bound [/number].
p(1). p(2). p(3).
q(X, K, V) :- p(X), empty_all(K, V).
r(X) :- p(X), empty_all(_, _).
```

Output: `empty_all` called **6 times** (3 per rule, one per `p` solution row; filters `[_ _]` for the wildcard form, `[K V]` for the named form), `strata=2`, no facts. The "called once per evaluation" behaviour M2 §2 relies on ("each external called exactly once with `inputs=[]`") is an artefact of the cache check at `topdown.go:84-93`, which only short-circuits when the store already holds a matching fact. A vector search with zero hits, a symbol with no doc, an empty history table: each is one Go call per solution row of everything to its left, per delta round. In Pattern A the external sits after `task_query × knowledge_rev`, so cost is rows-of-that-join per evaluation, per empty source. M2 §2 "Cost note" must say this; M0 §3 ("if the store already has any matching fact, no call") is correct but does not draw the consequence.

**Engine surprise A2 — a constant in an output position of an all-output external changes the cache key.** `v_ext_filter.mg` (`old(A) :- recall_similar_all(_, "rev-16", A, _). cur(A) :- recall_similar_all(_, "rev-17", A, _).`): two calls, `filters=[_ "rev-17" A _]` then `[_ "rev-16" A _]`; rows not matching the constant filter are discarded, not stored (`topdown.go:119-133`), so the second rule cannot reuse the first call's table. Derived sets are right (`cur` 3 rows, `old` 1 row). Consequence for Pattern A rule 2: writing the revision join as a constant (`recall_similar_all(Q, "rev-17", …)`) instead of a variable joined to `knowledge_rev` re-runs the external per distinct constant *and* throws away every other revision's rows — the variable form in M2's example is the right one, and the reason should be stated.

**codeNERD-side check of the Decls M2 cites.** `schemas_memory.mg:54-84` and `schemas_tools.mg:320-325` are as described (single-quoted `mode('+', '-')` strings, which lex the same as double-quoted). `kernel_eval.go:256-259` builds a fresh store per full evaluation (`newEvaluationFactStore`), so the per-evaluation external cache does reset in production; `kernel_eval.go:333-344` registers only callbacks whose Decl is `IsExternal()`. Grep of `internal/core/defaults/**/*.mg` for rule-body calls of the five `+`-moded externals: 0 hits (only comments) — M2's "no live rule body calls any of them" holds.

### Pattern B — derived context injection

Program as run: `patternB.mg`, byte-identical to M2 §3.

```
& mgv.exe patternB.mg inject dropped
& mgv.exe -store simple -det=false -runs 5 patternB.mg inject dropped inject_prio above_sum tie_sum
strata=9
dropped/3 (2):
  dropped("coder-7","context/core.evaluate",/budget)
  dropped("coder-7","language/go/errors",/budget)
inject/5 (6):
  inject("coder-7","context/history/core.evaluate",/built_before,/tail,95)
  inject("coder-7","context/history/core.evaluate",/focal_symbol,/tail,95)
  inject("coder-7","identity/coder",/skeleton,/head,100)
  inject("coder-7","knowledge/go/error_wrapping",/knowledge_hit,/middle,80)
  inject("coder-7","knowledge/go/table_tests",/knowledge_hit,/middle,60)
  inject("coder-7","safety/no_secrets",/skeleton,/head,100)
above_sum: context/core.evaluate 300, context/history 70, error_wrapping 90, table_tests 170, language/go/errors 240
tie_sum: (100,70) (95,20) (80,80) (60,70) (50,60) (30,90)
```

Identical on `SimpleInMemoryStore` and across 5 nondeterministic runs. `v_patternB_300.mg` (`compile_budget 300`): `inject` gains `("coder-7","language/go/errors",/task_verb,/middle,50)`, `dropped` keeps only `context/core.evaluate` — as M2 states. `v_patternB_2agents.mg` (adds `compile_shard("rev-1", /reviewer)`, budget 100, verb `/review` selecting `language/go/errors`): `rev-1` gets the two skeleton atoms (70 tokens) and drops `language/go/errors` (60 more would be 130 > 100); the coder's rows are unchanged — per-agent scoping holds.

**Verdict: holds.**

**Holds-with-changes note on M2 §3 last paragraph** ("`fn:collect(Reason)` in the `inject_prio` transform gives a list column"). `v_patternB_collect.mg` adds `inject_reasons(A, Atom, Rs) :- candidate(A, Atom, R, _) |> do fn:group_by(A, Atom), let Rs = fn:collect(R).` Result: `inject_reasons("coder-7","knowledge/go/error_wrapping",[/knowledge_hit, /knowledge_hit, /knowledge_hit])` — one entry **per candidate row**, and the `/knowledge_hit` rule fires once per `knowledge_boost` tier the score clears (91 clears 90, 60 and 30 → three rows with priorities 80, 60, 45). `fn:collect` collects per input row, not per distinct value; the sentence must say `fn:collect_distinct`. The order claim is confirmed: 8 runs with `-det=false`, `context/history/core.evaluate` alternates between `[/focal_symbol, /built_before]` and `[/built_before, /focal_symbol]` (runs 3, 4, 6 flipped). Also `strata=10` with the extra aggregation, one more than M2's nine — every `do` rule is its own stratum even when it groups the same body (M0 §6).

**Design note (not an engine failure):** `candidate(A, Atom, /knowledge_hit, Boosted)` is a *per-tier* rule, so a 91-score atom has three candidate rows and any consumer that counts candidates (a future "how many reasons" rule) over-counts. `inject_prio` hides it via `fn:max`; `inject` re-joins `candidate(A, Atom, Reason, _)` and is not affected only because the reason column is the same for all three rows. If a proof of `inject_prio` is requested (Pattern F), all three rows appear as contributing facts — M2 §7 already reports "both contributing candidate rows (60 and 45)", i.e. it saw two of the three (the 80 row is the group max and appears as the aggregate's own value); that is the tier design showing through the proof, not a recorder defect.

### Pattern C — derived test obligations

Program as run: `patternC.mg`, byte-identical to M2 §4.

```
& mgv.exe -store simple -det=false -runs 5 patternC.mg test_obligation coverage_missing behavior_touched behavior_test test_green_now obligation_open
strata=9
behavior_test("B1","core.TestEvaluateFresh")  behavior_test("B2","session.TestTurnDone")
behavior_touched("B1")
coverage_missing("B1","core.assert")
obligation_open("B1")
test_green_now("session.TestTurnDone")
test_obligation("B1","core.TestEvaluateFresh",/rerun)  test_obligation("B1","core.assert",/write)
```

Identical to M2's expected block, on both stores, 5 runs. **Verdict: holds.**

M2's two follow-on claims:

- `v_patternC_green.mg` (adds `test_run("core.TestEvaluateFresh","rev-17",/pass)`): the `/rerun` row disappears; `/write` and `coverage_missing` remain. Holds.
- `v_patternC_assert.mg` (adds `core.TestAssert` calling `core.assert`, two CodeDOM rows + `is_test_function`): `/write` and `coverage_missing` disappear **and a new `test_obligation("B1","core.TestAssert",/rerun)` appears** because the new test has no run at `rev-17`. Correct behaviour, but M2 §4 ("adding a test … removes `/write` and `coverage_missing`") should say the obligation converts to `/rerun` rather than vanishing; `obligation_open("B1")` stays true.

Adversarial probes:

- `v_patternC_cycle.mg` adds a `code_calls` cycle (`core.evaluate ↔ core.assert`) and a *second* run of `session.TestTurnDone` at `rev-17` with `/fail`. The recursion terminates (`test_depends_on` gains `("core.TestEvaluateFresh","core.assert")` and nothing loops). **But `test_green_now("session.TestTurnDone")` still holds** with a `/pass` and a `/fail` both at `rev-17`: rule 4's witness `test_run(TestRef, Rev, Outcome)` has no run ordinal, so "green now" means "some run at this revision passed", not "the latest run passed". Pattern G guards the same situation with `!has_test_failure(T)`; Pattern C does not. **Holds with changes:** add `has_test_failure_now(T) :- test_run(T, Rev, /fail), current_rev(Rev).` and negate it in `test_green_now`, or key the witness on a run ordinal and derive the latest with `fn:max`.
- `v_patternC_oldrev.mg` moves both `changed_symbol` rows to `"rev-15"` and makes the evaluate test green at `rev-17`: `behavior_touched("B1")` and `test_obligation("B1","core.assert",/write)` still derive. The `Rev` column of `changed_symbol` is never joined by any rule in the pattern (`behavior_touched` and the `/write` rule both use `changed_symbol(Sym, _)`), so M2 §4 rule 3 ("the change set is a revision-stamped witness") is decoration: a change from any past revision keeps a behavior "touched" until a test exists. That may be the intended obligation semantics, but then the sentence and the column are misleading; if it is *not* intended, the fix is `changed_since(Sym) :- changed_symbol(Sym, R), rev_after(R, Base)` with a base-revision fact. Either way M2 must say which.

### Pattern D — completion as obligation fixpoint

Program as run: `patternD.mg` and `patternD_v2.mg`, byte-identical to M2 §5.

```
& mgv.exe -store simple -det=false -runs 5 patternD.mg loop_verdict task_incomplete stall failure_count latest_round
strata=10
failure_count("t1","sha:go-build-undefined-foo",3)
latest_round("t1",5)
loop_verdict("t1",/stop_stalled)
stall("t1",/repeated_failure)
task_incomplete("t1",/acceptance,"acceptance at current revision")
task_incomplete("t1",/edit_step,"step-2")
task_incomplete("t1",/test_write,"core.assert")
```

Identical to M2, both stores, 5 runs. `patternD_v2.mg`: `loop_verdict("t1",/continue)`, no `stall` row — as stated. `fn:string:concat("step-", N)` with a number argument gives `"step-2"` (M0 §5 auto-conversion holds). **Verdict: holds** for the program as written.

Follow-on claims and adversarial probes:

- `v_patternD_complete.mg` (acceptance at `rev-17`, the `test_obligation` and `edit_step_pending` facts removed): `loop_verdict("t1",/complete)` — and `stall("t1",/repeated_failure)` still derives beside it, harmlessly, because `/stop_stalled` requires `has_obligation`. One row. Holds.
- `v_patternD_noprog.mg` (open count 3,2,2,2,2 and no round-5 failure): `stall("t1",/no_progress)`, verdict `/stop_stalled`. The `fn:minus`/`>=` window works.
- **`v_patternD_2tasks.mg` — refutes the per-task claim.** Adding `task_active("t2")` with its own acceptance at `rev-17` yields `task_incomplete("t2",/test_write,"core.assert")` and `loop_verdict("t2",/continue)`: the rule `obligation(T, /test_write, Target) :- task_active(T), test_obligation(_, Target, /write).` is a cross product of every active task with every test obligation, because Pattern C's `test_obligation(B, Target, Kind)` carries a behavior, not a task. M2 §5 rule 1 ("one rule per source") and the migration map's "per task" language are wrong as written for any session with more than one active task (campaigns, parallel shards). **Holds with changes:** either `test_obligation` gains a task column in Pattern C (the change set is per task, so `changed_symbol(Task, Sym, Rev)` and everything downstream), or `obligation` joins through `task_behavior(T, B)`.
- **`v_patternD_spread.mg` — `failure_count` is a lifetime total, not a repeat.** Nine rounds, identical failure digest at rounds 1, 5 and 9, open count falling 3→0 in between: `failure_count(...,3)`, `stall("t1",/repeated_failure)`, `loop_verdict("t1",/stop_stalled)`. M2 §5 rule 3 says this "collapses" the controller's "period-1..3 repeated trace" detection (`tool_budget_controller.go:276-302`); it does not — the controller looks at the tail of the trace, the rule counts the whole history, so a task that hit the same build error three times over an hour of otherwise-productive rounds is stopped. **Holds with changes:** window the count: `recent_failure(T, D) :- tool_failure(T, R, D), latest_round(T, L), repeated_failure_rounds(K), Lo = fn:minus(L, K), R > Lo.` and count that (one more stratum).

### Pattern E — blast-radius edits as derived plans

Program as run: `patternE.mg` and `patternE_v2.mg`, byte-identical to M2 §6.

```
& mgv.exe -store simple -det=false -runs 5 patternE.mg edit_plan blast_radius plan_gate edit_step_pending plan_blocked plan_site
strata=8
blast_radius("t1",7)   plan_blocked("t1","gen.Evaluate",/generated_code)   plan_gate("t1",/blocked)
plan_site: 7 rows exactly as M2 lists; edit_plan and edit_step_pending empty
& mgv.exe patternE_v2.mg edit_plan blast_radius plan_gate edit_step_pending
edit_plan: the 6 rows M2 lists (steps 1/2/2/2/2/3); blast_radius("t1",6); plan_gate("t1",/confirm); edit_step_pending("t1",2), ("t1",3)
```

Identical to M2 on both stores, 5 runs. **Verdict: holds** for one `chosen_change` per task.

Adversarial probes:

- **`v_patternE_2changes.mg` — refutes the plan for more than one change per task.** A second row `chosen_change("t1", "core.Kernel.Evaluate", /signature, "Evaluate(ctx) error")` produces **12** `edit_plan` rows: every rename site is emitted twice, once with `/rename "Fixpoint"` and once with `/signature "Evaluate(ctx) error"`, although no `plan_site` rule matches `/signature` at all. The rule `edit_plan(T, Step, Ref, File, Op, Spec) :- plan_site(T, Ref, Tier), …, chosen_change(T, _, Op, Spec), !has_block(T).` joins sites to changes on the task only. M2 §6 rule 1 says the tool "accepts" `chosen_change` rows and rule 4 says "the tool applies rows in [step] order" — with two rows the tool would apply a signature change at six rename sites. **Holds with changes:** `plan_site` must carry the change it belongs to (`plan_site(T, ChangeRef, Op, Ref, Tier)`) and `edit_plan` must join on it; or the design must state "one `chosen_change` per task at a time" and enforce it with a `plan_gate(T, /blocked)` rule when `fn:count` over `chosen_change` exceeds 1.
- `v_patternE_2tiers.mg` (adds `code_calls("mock.Evaluate", "core.Kernel.Evaluate")`, so `mock.Evaluate` is both an `/implementer` and a `/caller`): `plan_site` has 7 rows for 6 refs and `blast_radius("t1",7)`. `blast_radius` counts `(Ref, Tier)` pairs, not sites, because its body is a single atom (see engine surprise E1 below) — the `confirm_threshold` compares against tier-inflated numbers. `edit_plan` is unaffected here only because both tiers map to step 2. **Holds with changes:** count over a projection `site_ref(T, Ref) :- plan_site(T, Ref, _).`

**Engine surprise E1 — `fn:count()` semantics depend on whether the body is one atom or several.** `v_agg_wildcard.mg`:

```mangle
Decl p(X, Y) bound [/number, /name].
Decl q(X) bound [/number].
Decl c1(N) bound [/number].  Decl c2(N) bound [/number].  Decl c3(N) bound [/number].  Decl c4(N) bound [/number].
p(1, /a). p(1, /b). p(2, /c).
q(1). q(2).
c1(N) :- p(X, _) |> do fn:group_by(), let N = fn:count().
c2(N) :- p(X, _), q(X) |> do fn:group_by(), let N = fn:count().
c3(N) :- p(X, Y), q(X) |> do fn:group_by(), let N = fn:count().
c4(N) :- p(X, _), q(_) |> do fn:group_by(), let N = fn:count().
```

Output: `c1(3)`, `c2(2)`, `c3(3)`, `c4(2)`. Cause: `rewrite.Rewrite` leaves a clause alone when its body is a single atom (`rewrite/rewrite.go:40`, `isSingleAtomPremise`) — the aggregate then runs over the stored facts, wildcard columns included — but for a multi-atom body it builds a `<pred>N__tmp` relation over the *named* variables only (`rewrite/rewrite.go:52`, "Wildcard variables `_` do not correspond to any column"), which dedups on those columns before the aggregate runs. So the same wildcard means "count facts" in `c1` and "count distinct `X`" in `c2`; adding a second atom to an aggregation body silently changes what is counted. M0 §6 states the `_` exclusion but not the single-atom exception or its consequence. Checked against every aggregation in M2: `blast_radius` (single atom, counts tier-duplicated rows — wrong, above); `finding_count`, `failure_count`, `inject_prio`, `above_sum` (single atom each, count/sum over facts — correct today because the wildcard column is not what is being counted, but each flips to distinct-projection semantics the moment a second atom is added); `tie_sum` (two atoms, `_` in `prompt_atom` — the `__tmp` columns are `A, Atom, P, T`, one row per atom, so the sum is right). None is wrong in the shipped examples except `blast_radius`; all are fragile. Rule for the corpus: **never put `_` in the body of a `do`-transform rule; name every column, and project first if a count over distinct keys is wanted.**

### Pattern F — provenance for `nerd why`

Program as run: `patternF.mg` (the nine `EmitFacts` rows of `smoke.mg` + the four rules), byte-identical to M2 §7.

```
& mgv.exe -store simple -det=false -runs 5 patternF.mg why load_bearing_absence supports why_var
strata=4
load_bearing_absence([/q, 1],[/n, 1])
supports(/proof/267d476fa072cc672c7ffc98ef6c9f22,/proof/93d47d7a178da28b7460ba64d2bd87e5)
why([/q, 1],"q(X) :- p(X), !n(X).",[/p, 1])
why_var([/q, 1],"X",1)
```

Identical to M2, both stores, 5 runs. **Verdict: holds.**

Provenance claims re-tested with this study's own recorder wiring (`-prov`, `MaxProofs 3`, `MaxDepth 12`):

- **ID stability (holds).** `& mgv.exe -prov smoke.mg q` on a different driver, and again with `-det=false`, emits byte-identical rows to the nine facts hard-coded in `patternF.mg` (`/proof/267d476f…`, `/proof/93d47d7a…`, `/proof/eabbf247…`, `/rule/59f3ea79…`). Content hashing is deterministic across harnesses and evaluation orders.
- **R1 (holds).** `prov_wild.mg`: `q(1)` proof is `[partial]`, only `r(1)` as premise index 0, `p(1,/a)` absent from the proof. `prov_named.mg`: both premises, `Y=/a` in `binding`. Exactly as M2 §7 says.
- **The recorded rule is the rewritten, normalised clause, not the source text.** `v_prov_reorder.mg` (`q(X, Y) :- p(X), !r(X, Y), s(Y).`): `rule_source(/rule/39ae04c1…, "q(X,Y) :- p(X), s(Y), !r(X,Y).")` — the negation moved to the end by `RewriteClause` (M0 §4), spaces after commas removed, and the proof's premise indexes follow the rewritten order (`!r` is index 2, not 1). In the Pattern B proof the text reads `:ge(Score,Min)`, `:gt(Q,P)`, `:le(Total,B)` where the policy file says `Score >= Min`, `Q > P`, `Total <= B`. So M2 §7 rule 5 ("`/rule/<hash>` is the hash of the rule text") is the hash of `Clause.String()` of the *rewritten* rule: whitespace and comparison spelling are normalised (good — cosmetic edits do not change the hash), but `nerd why` will print a rule the user cannot find verbatim in the `.mg` file, and a source-level reorder of a negation that the rewrite would have done anyway does not change the hash while a reorder of two *positive* atoms does. M2 should say "hash of the analysed clause", and `nerd why --diff` should not be described as a diff of policy text.
- **Pattern F rules over the real Pattern B proof (holds with corrections).** `v_patternF_B.mg` = the 176 distinct `EmitFacts` rows of the three proofs of `inject("coder-7","knowledge/go/error_wrapping",/knowledge_hit,/middle,80)` (the list-valued facts parse as Mangle source unchanged) + the four rules: 70 `why` rows, of which 10 have the `inject` atom as `Fact`, with leaf facts `compile_budget`, `knowledge_boost` ×3, `knowledge_hit`, `skeleton_category` ×2, `symbol_atom`, `task_focus_symbol`, `window_slot`. **Absent: `built_before`, `compile_shard`, `prompt_atom`** — every rule that reaches them has a wildcard (`built_before(A, Sym, _)`, `compile_shard(A, _)`, `prompt_atom(Atom, _, P, _, _)`), so R1 removes them. M2 §7's list "`knowledge_hit`, `knowledge_boost`, `compile_budget`, `built_before`, `symbol_atom`, `task_focus_symbol`, `prompt_atom` (once R1 is applied)" is wrong on `built_before` (needs R1 too) and omits `skeleton_category`/`window_slot` (present because the `above_pair` sub-proofs reach the competing skeleton atoms). The recorder saw **77 events** for Pattern B's 9 strata over ~40 facts; `dropped(...)` proves with `absence_leaf([/fits, "coder-7", "context/core.evaluate"])` as stated.
- **`why` is transitive over every EDB leaf under the node**, so for `inject` it also lists the leaves of the atoms it *competed with* (`skeleton_category`, `symbol_atom` for `context/history/core.evaluate`). That is what M2 wants ("what did it beat"), but the row `why(inject(...), rule, [/skeleton_category, /identity])` reads as "injected because of the identity skeleton", which is false at depth 0. The why-program needs a depth or a `via(P, Child)` column to distinguish direct premises from transitive support; as written `why/3` cannot tell them apart.

### Pattern G — verdicts as facts

Program as run: `patternG.mg`, byte-identical to M2 §8.

```
& mgv.exe -store simple -det=false -runs 5 patternG.mg verdict stage evidence unverified_claim shard_status
strata=8
evidence: t1 /build_green /tests_green /wrote; t2 /wrote; t4 /findings
stage: t1 /verified; t2 /edited; t3 /unstarted; t5 /unstarted
unverified_claim: t2 /done, t2 /tests_pass, t3 /done
verdict: t1 (/verified,/verified); t2 (/incomplete,/edited); t3 (/failed,/unstarted); t4 (/review_findings,/reviewed); t5 (/nothing_done,/unstarted)
shard_status: t1 /complete; t2 /code_generated; t3 /failed; t4 /review_needed
```

Identical to M2, both stores, 5 runs. **Verdict: holds.**

**T1 re-tested (holds, and is wider than M2 says).** `trailing_name.mg` (`X != /b.` at EOF): `PARSE ERROR: 5:0 missing '.' at '<EOF>'`; `trailing_name_space.mg` (`/b .`): parses, `q(/a)`. New: `v_trailing_multi.mg` — the same clause **followed by another clause** fails with `missing '.' at 'r'`, i.e. the trap is not an end-of-file artefact; `v_trailing_eq.mg` — `X = /b.` fails the same way (M2 only shows `!=`); `v_trailing_fact.mg` — a newline before the period (`X = /a` ⏎ `.`) parses. Grep of `internal/core/defaults/**/*.mg` for `(=|!=) */name.` at end of line: 0 hits, so the shipped corpus is clean today; the lint M2 §9 asks for is still worth having because the failure mode is a parse error at boot, not a silent one.

Adversarial probe `v_patternG_more.mg` (three more tasks):

- **t6 — a coder that only ran tests** (`shard_tool /run_tests /ok`, `test_run … rev-18 /pass`, no write): `evidence("t6",/tests_green)` derives, but `stage("t6",/unstarted)` and `verdict("t6",/nothing_done,/unstarted)`. The stage ladder is keyed on `/wrote` first (`stage(T, /unstarted) :- shard_run(T, /coder, _), !evidence(T, /wrote)`), so a verification-only task (rerun the tests, as Pattern C's `/rerun` obligation asks a shard to do) is reported as having done nothing. **Holds with changes:** the ladder needs a `/verified_only` or the `/rerun` case needs its own verdict rule; as written, Pattern C's `/rerun` obligation and Pattern G's verdict contradict each other.
- **t7 — two builds at the current revision, one `/fail` then one `/pass`**: `evidence("t7",/build_green)`, `stage("t7",/built)`. Same defect as Pattern C's flaky test (§2C): `build_run(T, Rev, Outcome)` has no run ordinal, so any pass at the revision is green. `has_test_failure` guards tests but nothing guards builds. **Holds with changes:** `has_build_failure(T)` mirror, or an ordinal column and `fn:max`.
- **t4 — a reviewer that claims `/done`**: `unverified_claim("t4",/done)` derives because `stage` is defined for coder shards only (`!stage(T, /verified)` is always true for a reviewer). Every reviewer claim of `/done` becomes a `/hollow` obligation in Pattern D. Design note; the claim rules need a shard-type guard.
- `t2`/`t3`/`t5` unchanged. One `verdict` row per task held in all seven cases.

### Composition of A–G (M2 open item (a), (d))

`v_compose_raw.mg` = the seven pattern files concatenated: `ANALYSIS ERROR: predicate task_focus_symbol(A0, A1) declared more than once`. Six predicates are declared in more than one pattern (`code_calls/2`, `code_element/5`, `current_rev/1`, `edit_step_pending/2`, `is_test_function/1`, `test_obligation/3` identically; `knowledge_hit/3` and `task_focus_symbol/2` with different argument names `TaskID` vs `Agent`). A duplicate `Decl` is a hard analysis error (M0 §3, `analysis/validation.go:173-175`), so the patterns cannot be loaded beside each other, let alone beside the shipped policy where `code_element/5`, `code_calls/2`, `is_test_function/1`, `test_run/4`, `current_rev`-like predicates already have Decls (`schemas_codedom.mg:33`, `schemas_analysis.mg:112-116`, `schemas_analysis.mg:182`). M2 §1 convention 1 ("every predicate has a Decl … before use") must add "exactly one, in the file that owns it" and the patterns must drop their copies.

`v_compose_dedup.mg` (first Decl of each predicate kept): loads, **`strata=52`** = 4+9+9+10+8+4+8 exactly. M2 §10 says "they share predicates and will not add linearly" — refuted for the shipped examples: the strata add linearly because no pattern's aggregation feeds another's. `inject`, `loop_verdict`, `plan_gate` are unchanged, but **`verdict("t5",…)` changes from `(/nothing_done,/unstarted)` to `(/incomplete,/built)`**: Patterns C, D and E assert `current_rev("rev-17")` and G asserts `current_rev("rev-18")`, so the composed kernel has two current revisions and t5's `rev-17` write and build are "current" again. `current_rev/1` "one row" (M2 §9 cross-cutting seams) is a convention no rule enforces; the first composition broke it and the revision join silently widened. Add `current_rev_count(N) :- current_rev(R) |> do fn:group_by(), let N = fn:count().` and a `kernel_invariant_violated(/current_rev, N)` rule with `N > 1`, or make it a `fundep([], [Rev])` merge predicate.

`test_run` is declared `/3` in Pattern C (`TestRef, Rev, Outcome`) and `/4` in Pattern G (`Task, TestRef, Rev, Outcome`) while M2 §8 calls the `/4` form "the Pattern C witness, task-scoped": two different witnesses with the same name, and Pattern C's rules read the `/3` one. Composition does not error (different arities are different predicates) — which is worse, because a producer that emits `/4` rows leaves every Pattern C rule empty with no diagnostic (the `agents.md` Decl-contract failure class).

## 3. M0 claims tested

Each row: the M0 claim, the program (`C:/Temp/mangle-study/verify/m0_*.mg`, quoted where short), the actual output of `mgv.exe`, the verdict. All programs were run on `MultiIndexedArrayInMemoryStore` unless stated. The deferred-cycle claim (M0 probe DD, "fatal stack overflow") was **not re-run** — it kills the process after filling a 1 GB goroutine stack, and M0 already has the dump; it stays as M0 reports it, unverified here.

| # | M0 claim (section) | Program | Actual | Verdict |
|---|---|---|---|---|
| 1 | Unsafe negation with wildcards is silently deleted (§4 probe A) | `m0_A.mg`: `p(1). p(2). r(1,/a,/b). q(X) :- p(X), !r(X,_,_).` | `RULE: q(X) :- p(X).`; `q(1) q(2)`; driver warns "negation dropped by rewrite" | **holds** |
| 2 | An earlier `_` turns the same form into an analysis error (§4 probe B) | `m0_B.mg`: `q(X) :- p(X,_), !r(X,_,_).` | `ANALYSIS ERROR: variable X1 is not bound in q(X) :- p(X,X0), !r(X,X1,X2).` (M0 quotes `X2`; the first unbound fresh variable is reported, the name differs by traversal order) | **holds** |
| 3 | Projection idiom works (§4 probe C) | `m0_C.mg`: `r1(X) :- r(X,_,_). q(X) :- p(X), !r1(X).` | `q(2)`, `strata=2` | **holds** |
| 4 | Unbound named variable in a negation is also deleted (§4 probe D) | `m0_D.mg`: `q(X) :- p(X), !r(X,Y).` | `RULE: q(X) :- p(X).`; `q(1) q(2)` | **holds** |
| 5 | `fn:count()` over an empty relation derives nothing; zero case needs a negation rule (§6 probe J) | `m0_J.mg`: `c(N) :- p(X) \|> do fn:group_by(), let N = fn:count(). hasp() :- p(_). z() :- !hasp().` with no `p` facts | no `c`; `z()` derived; `strata=3` | **holds** |
| 6 | Recursion through aggregation passes `AnalyzeOneUnit` and fails at `Stratify` (§4/§6 probe K) | `m0_K.mg` | `STRATIFY ERROR: program cannot be stratified` (analysis returned no error) | **holds** — and in codeNERD the boot-time `rebuildProgram` calls only `AnalyzeOneUnit` (`kernel_eval.go:121`), so the error surfaces at the first `evaluate`, as M2 §11 risk 1 says |
| 7 | Numeric comparison on a float aborts the whole evaluation (§4 probe L) | `m0_L.mg`: `p(1.5). p(3.5). ok(X) :- p(X). q(X) :- p(X), X < 2.` | `EVAL ERROR: value 1.5 (4) is not a number`; nothing printed for `ok` either (driver exits on error) | **holds**; the "store left partially updated" half (M0 §7) not observed here (unverified) |
| 8 | `fn:count_distinct` rejected by analysis (§5 probe F) | `m0_F.mg` | `ANALYSIS ERROR: unknown function fn:count_distinct()` | **holds** |
| 9 | `fn:pick_any` passes analysis, fails at runtime (§5 probe G) | `m0_G.mg` | `EVAL ERROR: unknown reducer fn:pick_any(V0)` | **holds** |
| 10 | `fn:map:get` rejected by analysis (§5 probe I) | `m0_H.mg`: `v(V) :- m(M), V = fn:map:get(M, /b).` | `ANALYSIS ERROR: unknown function fn:map:get(V0, V1)` | **holds** |
| 11 | `SimpleInMemoryStore` drops hash-colliding facts (§2 probes AF, 3a–3c) | `m0_AF.mg`: `p({}). p([]). p(0). p(0.0). p(65792). p([1]). q(X) :- p(X).` | `-store simple`: **`q(65792) q({})` only** (six facts → two); `-store multi`: six rows (`0` printed twice: the number and the float) | **holds**, and is worse than any single M0 row: four distinct constants collapse to whichever was inserted first |
| 12 | `1` and `1.0` are distinct; `String()` prints the float as `1` (§2 probe R/T) | `m0_R.mg`: `p(1). p(1.0). q(X) :- p(X), X = 1.0. r(X) :- p(X), X != 1.` | `q(1)` and `r(1)` — both are the float `1.0`, printed as `1` | **holds** (and the printout is unreadable: a reader cannot tell `q(1)`-the-float from `q(1)`-the-int) |
| 13 | int64 arithmetic wraps silently (§5 probe AL) | `m0_AL.mg` | `q(-9223372036854775808)` | **holds** |
| 14 | count-distinct idiom `fn:collect_distinct` + `fn:list:len` (§5 probe AJ) | `m0_AJ.mg`: `p(1,7). p(1,7). p(1,8). p(2,9).` grouped by `K` | `c(1,2) c(2,1)` | **holds** |
| 15 | Unknown Decl descriptors are ignored (§3; M2 §9 row 16 marked this **unverified at runtime**) | `m0_upsert.mg`: `Decl hb(Node, T) descr [doc("heartbeat"), upsert(), ephemeral()] bound [/string, /number].` + facts + a `fn:max` rule | parses, analyses, evaluates: `latest(2)` | **holds** — M2 row 16's open item closes: the engine ignores `upsert()`/`ephemeral()` end to end |
| 16 | `deferred` with `mode("+","-")` works through a rule; stored facts of a deferred predicate are invisible (§3 probes DA, DF) | `m0_DA.mg`: `dbl` as a rule, `tag` as two facts, both `deferred()` | `q(6)`; **no `t` rows at all** | **holds** (both halves) |
| 17 | A `"/etc/passwd"` string literal matches a stored string in the engine (§11 last row, open item (a)) | `m0_E.mg`: `f(/a, "/etc/passwd"). pp("/etc/passwd"). g(A) :- f(A, "/etc/passwd"). h(A) :- f(A, P), pp(P).` | `g(/a)`, `h(/a)` | **holds** — the engine is not the source of the codeNERD mismatch (resolved below) |
| 18 | `_` in a head is an analysis error (§4 probe AA) | `m0_AA.mg` | `ANALYSIS ERROR: variable _ is not bound in q(_) :- p(X).` | **holds** |
| 19 | No retraction; re-evaluating a changed program on the same store keeps stale derivations (§7 probe AN) | `m0_AN1.mg` (`p(1). p(2). q(X) :- p(X).`) then `-reeval m0_AN2.mg` (`p(3). q(X) :- p(X), X != 1. s(X) :- p(X), !q(X).`) on the same store | after re-eval: `p(1) p(2) p(3)`, `q(1) q(2) q(3)` — `q(1)` survives although the new program cannot derive it, **and `s(1)` is not derived** because the stale `q(1)` satisfies `!q(X)` | **holds**, with the sharper consequence that a stale positive fact silently defeats a negation in the next program |
| 20 | Provenance IDs are content hashes stable across runs; recorder fires per derivation (§8) | `smoke.mg -prov`, twice, on a different driver than M2's | identical `/proof/…`, `/rule/…` IDs to M2's `patternF.mg` literals; Pattern B: 77 events for ~40 facts | **holds** (see §2F for what the recorded rule text is) |
| 21 | External predicates: "called … once per binding of the input positions" (§3) | `extA_min.mg`, `v_ext_plus_var.mg`, `v_ext_plus_first.mg`, `v_ext_plus_const.mg`, `v_ext_empty.mg` (§2A) | panic on any bound-variable input; one call per distinct constant input tuple; **repeated calls when the callback returns nothing** | **refuted** as stated: replace with "called with the *unsubstituted* premise atom; a variable in an input position panics (E1); a constant input tuple is called once per evaluation if it returns ≥1 row, else once per premise evaluation" |
| 22 | `fn:count()` over a body with a wildcard (§6 "`_` excluded" from the `__tmp` columns) | `v_agg_wildcard.mg` (§2E) | single-atom bodies are **not** rewritten and count facts including the wildcard column; multi-atom bodies drop it | **holds with changes** — the exclusion applies only when `rewrite.Rewrite` runs, i.e. for multi-atom bodies (`rewrite/rewrite.go:40`) |

**M0 open item (a) — resolved on the codeNERD side, not the engine.** `internal/types/types.go:246-253` (`Fact.ToAtom`, the conversion every `Assert` goes through, `kernel_fact_decl.go:103-107`): a Go `string` argument is turned into a **name constant** when `isValidMangleNameConstant` (`types.go:77-113`) accepts it — leading `/`, no whitespace, no `//`, **at most two `/`**, no common file extension, and `ast.Name` parses it. Throwaway test (`internal/types/zz_m3_scratch_test.go`, run with `go test ./internal/types -run TestM3ScratchSlashStrings -v`, deleted afterwards; `git status` clean):

```
"/etc/passwd"   -> p(/etc/passwd)      NAME
"/etc/passwd/x" -> p("/etc/passwd/x")  string (three slashes)
"/tmp/a"        -> p(/tmp/a)           NAME
"/tmp/a.go"     -> p("/tmp/a.go")      string (extension)
"/yes"          -> p(/yes)             NAME
"etc/passwd"    -> p("etc/passwd")     string
"C:/x/y"        -> p("C:/x/y")         string
"/a b"          -> p("/a b")           string
```

So the stored value in the `agents.md:53-60` measurement was the *name* `/etc/passwd`, and the rule literal `"/etc/passwd"` is a *string*: type mismatch, zero rows, exactly M0's "`1` vs `1.0`, `"a"` vs `/a`" trap. The Decl's `/string` bound does not help: `coerceAtomToDeclLocked` (`kernel_fact_decl.go:46-90`) coerces only float→number, never string↔name. `agents.md`'s explanation ("the leading slash makes the literal read as something other than that string") has the direction backwards — the *fact* is what changes type, and only when the path has ≤2 segments and no extension; `/etc/passwd/x` and `/tmp/a.go` stay strings. The reported `"etc/passwd"` → 1 row and `"passwd"` → 1 row remain **unexplained and unverified** (no equality join produces them; the measurement should be re-run against a booted kernel with the rule text and the stored atom printed). The `agents.md` recipe (join on a variable, seed `protected_path("/etc/passwd")` from Go) works only because *both* sides then go through `Fact.ToAtom` and become the same name — it is a coincidence of the heuristic, not a fix, and it breaks the moment one side is a three-segment path.

## 4. Engine surprises

Behaviours nobody claimed in M0 or M2 that change the design. Each one was produced by a program in `C:/Temp/mangle-study/verify/` and is cited to the engine source at the pinned version.

1. **An all-output external that returns nothing is called on every premise evaluation** (`v_ext_empty.mg`, §2A: 6 calls for two rules over three `p` rows). The once-per-evaluation behaviour M2 §2 measured is the store cache at `engine/topdown.go:84-93` short-circuiting on an existing row; with no row there is nothing to hit. An empty vector search, an unknown symbol, an empty history table each cost one Go call per solution row to their left, per delta round, per evaluation. M2's cost note ("whole table once per evaluation") is the best case, not the case.
2. **A `+`-moded external anywhere but after all of its binders is a runtime panic that analysis accepts** (`v_ext_plus_first.mg`, §2A). Safety treats positive atoms as order-independent (`analysis/rulecheck.go`, M0 §4), but `oneStepEvalPremise` evaluates in written order and hands the unsubstituted atom to `EvalExternalQuery` (`engine/seminaivebottomup.go:831`, `engine/topdown.go:99`). With E1 present every `+` position with a variable panics regardless of order; once E1 is fixed by applying the substitution, a `+` external written *before* its binder will still pass analysis and then see an unbound variable at runtime. The analyzer gate needs an order check for moded externals.
3. **A constant in an output position of an external changes the cache key and discards non-matching rows** (`v_ext_filter.mg`, §2A). Two rules with different constants each call the external; the rows filtered out by one call are never stored (`engine/topdown.go:119-133`), so the table is fetched once per distinct constant pattern.
4. **`fn:count()`/`fn:sum()` semantics flip between single-atom and multi-atom bodies when a wildcard is present** (`v_agg_wildcard.mg`, §2E: `c1(3)` vs `c2(2)`). `rewrite.Rewrite` skips single-atom bodies (`rewrite/rewrite.go:40`) and drops `_` columns only when it builds a `__tmp` relation (`:52`). Adding one more atom to an aggregation body silently changes "count facts" into "count distinct named variables". M2's `blast_radius` counts `(Ref, Tier)` pairs because of this.
5. **`fn:collect` collects one entry per input row, including duplicates** (`v_patternB_collect.mg`, §2B: `[/knowledge_hit, /knowledge_hit, /knowledge_hit]`), and its order changes between runs even with a fixed input set (3 of 8 runs flipped). M0 §7 states the order half; the duplicate half is new.
6. **The recorded rule text is the analysed, rewritten clause** (`v_prov_reorder.mg`, §2F): negations moved after their binders, comparisons spelled as `:gt(Q,P)`, whitespace normalised. `/rule/<hash>` is therefore stable under cosmetic edits of the policy file but does not match the file, and a proof's premise indexes follow the rewritten order. A string literal containing a quote round-trips escaped: `rule_source(…, "q(X) :- p(X), X != \"a\\\"b\".")` (`v_prov_quote.mg`).
7. **Wildcards drop EDB leaves from proofs even when the leaf is the decision's only witness** (`v_patternF_B.mg`, §2F): `built_before`, `compile_shard`, `prompt_atom` are absent from the `inject` proof. R1 (M2 §7) said `prompt_atom`; it is every wildcard premise, transitively.
8. **Duplicate Decls across files are a boot-time hard error; different arities of the same name are silently two predicates** (`v_compose_raw.mg`, §2 composition). The first is loud; the second is the silent Decl-contract failure with a `test_run/3` vs `test_run/4` example already inside M2.
9. **`SimpleInMemoryStore` collapses `0`, `0.0`, `[]`, `{}` to one fact by insertion order** (`m0_AF.mg`, §3 row 11: six facts → `q(65792) q({})`). M0 lists the pairs; the point for the design is that *which* value survives is the order of `InitialFacts`, i.e. source order — a reordered `.mg` file changes the derived set.
10. **A stale derived fact from a previous evaluation defeats a negation in the next one** (`m0_AN1/2.mg`, §3 row 19: `s(1)` not derived because the old `q(1)` is still in the store). Any host that re-evaluates on a retained store (codeNERD's differential path, `internal/mangle/differential.go`) inherits this unless it clears IDB predicates first; M0 §7 says "negation-dependent facts can be wrong" — this is the concrete mechanism.
11. **The created-fact limit aborts the evaluation with the count in the message** (`-limit 10 patternB.mg`: `EVAL ERROR: fact size limit reached 40 > 10`). Pattern B's `above_pair` is quadratic in candidates per agent: `v_perfB_N.mg` (N skeleton atoms, one agent) gives 0.46 s / 1.5 s / 5.8 s for N = 200 / 400 / 800 on `MultiIndexedArrayInMemoryStore` and 0.67 / 1.7 / 10.2 s on `SimpleInMemoryStore`; N = 400 already exceeds a 100 000 created-fact limit, and codeNERD's `defaultDerivedFactLimit` is 500 000 (`internal/core/kernel_init.go:188`). The atom library has 351 YAML atoms (`internal/prompt/atoms/**/*.yaml`), so a compile whose candidate set is the whole library sits at the edge of the ceiling and at ~1.5 s per fixpoint before any other stratum runs. M2 §11 risk 2 names the quadratic; the numbers say it is not hypothetical.
12. **Clause-final name constants swallow the period after `=` as well as `!=`, mid-file as well as at EOF; a newline before the period is a legal escape** (`v_trailing_eq.mg`, `v_trailing_multi.mg`, `v_trailing_fact.mg`, §2G). A number in the same position is fine (`v_trailing_num.mg`: `X != 1.` parses).
13. **Unknown Decl descriptors are accepted end to end** (`m0_upsert.mg`, §3 row 15) — usable as host-side annotations exactly as M2 §9 row 16 hoped, and equally a place where a typo (`upsrt()`) is never reported.
14. **The engine-level float printout is ambiguous**: `q(1)` may be the number or the float (`m0_R.mg`). Any log or test that compares `String()` output cannot distinguish them; only `Constant.Type` can.

## 5. Refutation list

Numbered; each names the M0/M2 location it hits and the change the revision agent must make. "Refuted" means the text is wrong as written; "holds with changes" means the claim is true but the design around it is incomplete.

1. **M2 §5 rule 1 and §9 (Pattern D, per-task obligations) — refuted.** `obligation(T, /test_write, Target) :- task_active(T), test_obligation(_, Target, /write).` cross-joins every active task with every behavior obligation (`v_patternD_2tasks.mg`, §2D). Change: give `test_obligation` (and `changed_symbol`, `behavior_touched`) a task column in Pattern C, or join through `task_behavior(T, B)`; state that Pattern C is per task.
2. **M2 §5 rule 3 (`failure_count` "collapses" the controller's repeated-tail detection) — refuted.** The count is a lifetime total (`v_patternD_spread.mg`: failures at rounds 1, 5, 9 with progress between → `/stop_stalled`). Change: window over the last `repeated_failure_rounds(K)` rounds via `latest_round` and `fn:minus`, one extra stratum; say so in the text.
3. **M2 §6 rules 1, 4 and the `edit_plan` rule (Pattern E) — refuted for more than one `chosen_change` per task.** Sites are joined to changes on the task only (`v_patternE_2changes.mg`: 12 rows, a `/signature` spec attached to rename sites). Change: `plan_site` carries the change (ref + op) and `edit_plan` joins on it; or a `plan_gate(T, /blocked)` rule when the change count exceeds 1, with the single-change assumption stated.
4. **M2 §6 `blast_radius` — holds with changes.** It counts `(Ref, Tier)` rows, not sites (`v_patternE_2tiers.mg`: 7 for 6 refs), because a single-atom aggregation body keeps the wildcard column (engine surprise 4). Change: count over `site_ref(T, Ref) :- plan_site(T, Ref, _).`
5. **M0 §6 "`_` excluded" from the `__tmp` columns — holds with changes.** True only for multi-atom bodies (`rewrite/rewrite.go:40,52`; `v_agg_wildcard.mg`). Change: state the single-atom exception and add the corpus rule "no `_` in the body of a `do`-transform rule".
6. **M2 §3 last paragraph (`fn:collect(Reason)` "gives a list column") — holds with changes.** It gives one entry per candidate row with duplicates (`v_patternB_collect.mg`) and, as M2 says, unstable order. Change: `fn:collect_distinct`, and note the per-tier `/knowledge_hit` candidate rows.
7. **M2 §4 rule 4 / `test_green_now` (Pattern C) — holds with changes.** A `/pass` and a `/fail` at the same revision both leave the test "green now" (`v_patternC_cycle.mg`); `build_run` in Pattern G has the same hole (`v_patternG_more.mg` t7) while `test_run` in G is guarded by `has_test_failure`. Change: guard both with a failure projection at the current revision, or give the witnesses a run ordinal and derive the latest.
8. **M2 §4 rule 3 ("the change set is a revision-stamped witness") — holds with changes.** No rule in Pattern C reads `changed_symbol`'s `Rev` column (`v_patternC_oldrev.mg`: a `rev-15` change keeps `B1` touched at `rev-17`). Change: either say obligations persist regardless of revision and drop the column from the sentence, or add a base-revision join.
9. **M2 §4 follow-on ("adding a test … removes `/write` and `coverage_missing`") — holds with changes.** The obligation becomes `/rerun` for the new test (`v_patternC_assert.mg`); `obligation_open` stays true. Change: say so.
10. **M2 §8 stage ladder (Pattern G) — holds with changes.** A coder task that only re-ran tests is `/nothing_done` (`v_patternG_more.mg` t6), which contradicts Pattern C's `/rerun` obligation; a reviewer that claims `/done` is always an `unverified_claim` (t4). Change: a verification-only stage/verdict rule and a shard-type guard on the claim rules.
11. **M2 §2 "each external called exactly once" / cost note — holds with changes.** True only when the callback returns at least one row (`v_ext_empty.mg`, engine surprise 1). Change: state the empty-result re-call and its cost; recommend that an all-output mount that can be empty return a sentinel row (e.g. `(Rev, /none)`) so the cache engages.
12. **M0 §3 "called … once per binding of the input positions" (external predicates) — refuted.** The premise is passed unsubstituted (`engine/seminaivebottomup.go:831`), a bound variable panics (E1), and analysis does not check the written order of moded externals (`v_ext_plus_first.mg`). Change: rewrite the sentence per §3 row 21; add the order caveat to M0 §3/§4.
13. **M2 §7 "yields one `why` row per EDB leaf — … `built_before` …, `prompt_atom` (once R1 is applied)" — refuted in detail.** `built_before` and `compile_shard` are also lost to wildcards; `skeleton_category` and `window_slot` appear through the competing atoms (`v_patternF_B.mg`). Change: list what a proof of `inject` actually contains before and after R1; note that `why/3` mixes direct premises with transitive support and needs a depth or `via` column to answer "why" at depth 0.
14. **M2 §7 rule 5 ("`/rule/<hash>` is the hash of the rule text") — holds with changes.** It is the hash of the analysed, rewritten `Clause.String()` (`v_prov_reorder.mg`, engine surprise 6): `nerd why` prints `:ge(Score,Min)` and reordered negations, not the policy file's text. Change: say "analysed clause"; define `nerd why --diff` over rule IDs, not policy text.
15. **M2 §10 "they share predicates and will not add linearly" (composed strata) — refuted.** Seven patterns as one unit: `strata=52` = the sum (`v_compose_dedup.mg`). Change: state that strata add unless a pattern's aggregation consumes another's; the composed toy cost is 52 strata.
16. **M2 §1 convention 1 / §9 cross-cutting seams — holds with changes.** Six predicates are declared in more than one pattern and the concatenation refuses to load (`v_compose_raw.mg`); `current_rev/1` is asserted by four patterns with two values and the composed kernel widens every revision join (`verdict("t5")` flips). Change: one owning Decl per predicate; an invariant rule (or `fundep`/`merge`) that makes `current_rev` one row and reports a violation as a fact.
17. **M2 §8 vs §4: `test_run/3` (C) and `test_run/4` (G) — holds with changes.** Two witnesses with one name and different arities; M2 §8 calls the `/4` form "the Pattern C witness". Change: one shape, task-scoped, used by both patterns.
18. **M2 §9 row 16 (`upsert()` descriptor "unverified") — holds; close the item.** `m0_upsert.mg` parses, analyses and evaluates. Change: mark verified, cite `analysis/declcheck.go:87-88` and the probe.
19. **M0 §11 last row / `internal/mangle/agents.md:53-89` ("a quoted string starting with `/` does not match a stored string") — refuted as an engine claim, resolved as a codeNERD claim.** `types.Fact.ToAtom` (`internal/types/types.go:246-253`) converts a Go string with a leading `/`, ≤2 slashes and no file extension into a **name**; the rule literal stays a string; `coerceAtomToDeclLocked` never coerces string↔name. Change in M0: close open item (a) with this citation (the `"etc/passwd"`/`"passwd"` 1-row halves stay unverified). Change for the corpus (not this study's file to edit): `agents.md`'s explanation must name the heuristic, and a Decl-directed string coercion in `coerceAtomToDeclLocked` — or an `ast.String` wrapper at every path-carrying assert site — is the real fix, because the current recipe breaks on three-segment paths.
20. **M2 §11 risk 2 (quadratic `above_pair`) — holds, with numbers.** 0.46 / 1.5 / 5.8 s and a 100k-fact overflow at N = 400 candidates on one agent (`v_perfB_*.mg`); codeNERD's ceiling is 500 000 (`kernel_init.go:188`) and the atom library is 351 atoms. Change: state the measured curve and require per-agent candidate scoping (or a bucketed prefix sum by priority tier — `tier_sum` over distinct priorities is linear) before Pattern B runs against the full library.
21. **M2 §1 convention 5 / M0 §12 item 1 (the dropped-negation lint) — holds, extend.** The engine also deletes a negation with an unbound *named* variable (`m0_D.mg`), so the lint must compare `NegAtom` counts per clause, not look for wildcards. `mgv.exe` prints the same warning M2's harness refuses on; both are host-side.
22. **M2 §8 T1 (clause-final name constant) — holds, widen.** Also `=`, also mid-file, and a line break before the period is a legal workaround (`v_trailing_*.mg`). Change: the lint pattern is "a name constant immediately followed by `.` at the end of a clause after `=` or `!=`" and the corpus grep is currently clean (0 hits).
23. **M0 §7 / §12 "no retraction" — holds, sharpen.** A retained store's stale derived fact defeats a negation in the next program (`m0_AN2.mg`, `s(1)` missing). Change: add this to the trap list; any host re-evaluation path must clear IDB facts before re-running, and codeNERD's differential engine should be checked against exactly this program (not done here — **unverified**).
