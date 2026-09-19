# 020 — Engine truths on the pinned commit

Engine: `codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3`. Every line below
was established by running a program against that engine (`Docs/journeys/M3-mangle-verification.md`,
probe files named there; items 17-31 with `nerd check-mangle --standalone --eval` on 2026-09-19,
the probes inline) or by a production failure in codeNERD on the date given. "The docs say" is not
evidence here: the 0.4.0 documentation of the downstream snapshot is wrong about several of these,
and the engine's own `docs/spec_decls.md` is wrong about one (item 22).

**How to check a claim yourself:** write the smallest program that shows it and run
`nerd check-mangle --standalone --eval <predicates> probe.mg`. It loads the file through the same
parser, analysis and stratification the kernel uses, evaluates it, and prints what each predicate
holds in the engine's own spelling.

## Silent deletions and rewrites

1. **A negated literal with a wildcard, or with a named variable no positive atom binds, is
   deleted from the clause with no error.** `q(X) :- p(X), !r(X, Y).` and `q(X) :- p(X), !r(X, _).`
   both evaluate as `q(X) :- p(X).` (with `r(/a, /z)` present, `q(/a)` is still derived). When the
   positive atoms in the same body also hold a `_`, the same mistake surfaces as an analysis error
   instead: `q(X) :- edge(X, _), !r(X, _).` reports `variable X1 is not bound`. A lint must
   compare the negated-atom count per clause before and after analysis; looking for `_` alone
   misses the named-variable form. Idiom: project first (`has_r(X) :- r(X, _).`), negate the
   projection; use a zero-arity projection for "any".
2. **A rule's analysed form is not its text**: comparisons become `:ge(Score, Min)`, negations are
   reordered (item 18). Provenance and rule hashes are over the analysed clause.
3. **Unknown `Decl` descriptors are accepted end to end.** Useful as host annotations; a typo in
   one (`upsrt()`) is never reported.
4. **A name constant ending a clause swallows the period** after `=` or `!=`, mid-file as well as
   at end of file (`X != /done.`). The cause is lexical: a name's characters include `.`
   (`parse/gen/Mangle.g4`, `CONSTANT_CHAR`), so `/done.` is one token. Mid-file the error appears
   one statement late (`missing '.' at 'Decl'`). A number in that position is fine. Escapes: a
   space or a line break before the period (`X != /done .`), or the name first (`/done != X.`).

## External (virtual) predicates

5. **Inputs must be literal constants in the rule text.** The premise reaches
   `EvalExternalQuery` without the current substitution (`engine/seminaivebottomup.go:831`), which
   then asserts every input-mode argument is a constant (`engine/topdown.go:99`): a variable bound
   by an earlier atom panics with `ast.BaseTerm is ast.Variable, not ast.Constant`. Analysis does
   not check the written order of moded externals either. Found again in production 2026-09-18
   (commit `6ec80601`). The upstream fix is one line (apply the substitution first). Until then, a
   check that needs the host (a signature, a model's score) runs in Go where the data arrives and
   is asserted as a witness fact the rule joins on.
6. **An external's answer is cached for the whole evaluation only when it returned at least one
   row.** An empty answer is re-requested each time; return a sentinel row from a mount that can
   be empty.

## Aggregation

7. **A single-atom body keeps wildcard columns; a multi-atom body projects onto named variables
   first.** `c(N) :- p(X, _) |> do fn:group_by(), let N = fn:count().` counts stored rows. Rule:
   no `_` in the body of a `do`-transform rule.
8. **Reducers that work, with their arity** (`symbols/symbols.go`): `fn:count()` (no argument:
   `fn:count(X)` fails analysis), `fn:sum(X)`, `fn:max(X)`, `fn:min(X)`, `fn:avg(X)` (there is no
   `fn:mean`; it yields a float), `fn:collect(...)`, `fn:collect_distinct(...)`, and the
   `fn:float:` forms of sum/max/min. **That do not:** `fn:count_distinct` (declared in symbols,
   never wired: "unknown function" in every spelling), `fn:map:get`, `fn:pick_any`. A distinct
   count is a projection rule followed by `fn:count()`: facts are a set. `fn:collect` yields one
   entry per row with duplicates, in unstable order. All four rejected forms were run through
   `nerd check-mangle` on 2026-09-18, the day codeNERD's own `/mangle` prompt atoms were found
   teaching three of them on 23 lines (fixed, and pinned by
   `TestEmbeddedCorpus_TeachesOnlyReducersThePinnedEngineAccepts`).
9. **An aggregation is its own stratum**, and the zero case is a separate negation rule (an
   aggregate over no rows derives nothing, not zero). Aggregating or negating inside a recursion
   cannot be stratified: derive the recursive set positively, aggregate in a later stratum.

## Stores, limits and re-evaluation

10. **`SimpleInMemoryStore` can collapse `0`, `0.0`, `[]` and `{}` into one fact**, keeping
    whichever came first in source order: a reordered `.mg` file changes the derived set. Never mix
    numbers with lists or structs in one column. `String()` cannot tell `1` from `1.0`; only the
    constant's type can.
11. **The created-fact limit is checked against the whole store and aborts the evaluation**, with
    the predicate being evaluated at that moment in the message. `fact size limit reached
    "layer(File,Layer)" 500016 > 500000` (2026-09-18) did not mean `layer/2` was at fault: the
    repository's world had outgrown a constant. codeNERD's ceilings are now
    `core_limits.max_facts_in_kernel` and `core_limits.max_derived_facts_limit`, installed
    process-wide at boot (`core.ConfigureFactLimits`).
12. **There is no retraction inside the engine.** A store retained across evaluations keeps its
    derived facts, so a fact derived under `!p` survives `p` arriving and an aggregate is never
    replaced. codeNERD's differential path did exactly this and was deleted (2026-09-18); every
    evaluation now rebuilds from the EDB. Consequence: EDB residency is an evaluation cost, every
    time.
13. **Strata add across files loaded as one unit** (seven toy patterns: 52 strata, the exact sum).

## Host boundary (codeNERD)

14. **`types.Fact.ToAtom` turns a Go string into a name when it starts with `/`, has at most two
    slashes and no file extension.** A rule's quoted literal `"/related_to"` stays a string. The two
    never unify, and the rule never fires. Assert `types.MangleAtom(...)` or a plain string
    deliberately, and declare the column.
15. **A float where a number column is declared aborts the fixpoint** (`kernel_fact_decl.go`,
    codeNERD's own check on facts Go asserts; the engine does not check source facts, item 22).
    Scores are int64 in 0..100.
16. **Negating a multi-argument literal with wildcards does not exclude** (item 1): this is the
    single most repeated bug in the corpus's history. On 2026-09-19 the legislator prompt atom,
    which teaches codeNERD's agents to write permission policy, presented
    `blocked(X) :- candidate_action(X), !permitted(X, _, _).` as CORRECT; run, it blocks every
    candidate action. Pinned since by `TestEmbeddedCorpus_TeachesNoWildcardNegation`.
    `working_set.mg` documents the idiom in place.

## Syntax the grammar decides (2026-09-19)

17. **Negation is `!atom`.** There is no `not` keyword: `final(X) :- base(X), not derived(X).` is a
    parse error (`extraneous input 'derived' expecting '('`). Booleans are the names `/true` and
    `/false`; a bare `true` is a parse error.
18. **Negation may be written anywhere in the body; comparisons may not.** Analysis moves a negated
    atom after the atoms that bind it, so `q(X) :- !foo(X), source(X).` is accepted and correct.
    Comparisons are not moved: `even(N) :- N > 0, ...` with `N` unbound fails analysis (`for goal
    ":gt(N,0)" expected N (arg 0) to be constant or bound variable`). Bottom-up, a number must be
    generated before it is tested: count up from a base case and bound the domain.
19. **Variables are an uppercase letter then letters and digits** (`VARIABLE` in the grammar); `_`
    alone is the wildcard. `P_Loser` does not lex. Name constants take letters, digits, `.`, `-`,
    `_`, `~`, `%` and further `/` segments: `/us-east-1`, `/v1.2`, `/codenerd/policy/jit`.
20. **Comparisons are integer-only.** `<`, `<=`, `>`, `>=` on a float abort evaluation
    (`Score > 0.85` over a `/float64` column: `value 0.9 (4) is not a number`), and the engine has no
    float comparison predicate.
21. **A zero-arity predicate keeps its parentheses:** `Decl has_stop().`, `has_stop() :- ...`,
    `!has_stop()`. `test :- ...` is a parse error, and so is a query (`?pred(X)`) in a `.mg` file:
    that is the interactive interpreter's syntax (`docs/using_the_interpreter.md`). `bound` is a
    keyword and cannot name a predicate (`bound(X) :- ...` is a parse error).
22. **Declarations.** The keyword is `bound` (the engine's own `docs/spec_decls.md` says `bounds`,
    which does not parse). The number of bounds must equal the arity (`expected 2 bounds, got 1`);
    `/any` leaves a column unconstrained. Type constants: `/number`, `/float64`, `/string`, `/name`,
    `/bytes`, `/any`. Type functions are capitalised: `fn:List(T)`, `fn:Map(K, V)`,
    `fn:Pair(A, B)`, `fn:Struct(/f, T, ...)`, `fn:Union(T1, T2)`, `fn:Singleton(/n)`, and the
    `.TaggedUnion</tag, /variant : .Struct<...>, ...>` expression; the lowercase `fn:list(/string)`
    in a bound is an analysis error. Several `bound` blocks are alternatives. **Source facts are not
    checked against bounds:** `t(1.5).` loads under `Decl t(X) bound [/number].`
23. **Transforms.** One stage is one `do` followed by `let`s, and a `let` may use the lets before it
    in the same stage (`let Count = fn:count(), let Total = fn:sum(V), let Avg = fn:div(Total,
    Count)` works). `do A, do B` in one stage is a parse error; a second `|>` stage of only `let`s
    does not bind (`variable Avg is not bound`); a reducer in a let-only first stage fails at
    evaluation (`|> let S = fn:sum(V)`: `not a list constant 2`); `let Price = P` is a parse error
    (`expected fn application got P`). Filtering is a comparison in the body before `|>`. There is
    no `fn:filter`, `fn:transform`, `fn:divide` or `fn:multiply` (`fn:div`, `fn:mult`).

## Structured data (2026-09-19)

24. **A struct literal with variables in a premise does not destructure.** `pair_of(X, Y) :-
    data({/a: X, /b: Y}).` loads, then evaluation fails (`evaluation produced something that is not
    a value: X ast.Variable`). Use `:match_field(D, /a, X)`: it needs the struct bound by an earlier
    premise and the output free; a constant in the third position is an analysis error (`expected
    /CallExpr (arg 2) to be a free variable`), so bind it and compare (`T = /call .`).
25. **Lists.** There is no `[Head|Tail]` syntax (a lexer error). `fn:list:cons(H, T)` prepends;
    `fn:list:append(L, E)` appends one element, so appending `[3]` to `[1, 2]` gives `[1, 2, [3]]`;
    `fn:list:get(L, I)` is 0-based; `fn:list:len(L)`; `fn:list:contains(L, X)` returns `/true` or
    `/false`; `:list:member(X, L)` enumerates; `:match_cons(L, H, T)` and `:match_nil(L)` need `L`
    bound. **Aggregating over `:list:member` collapses duplicates** (facts are a set): the sum of
    `[2, 2, 5]` that way is 7. Key elements by position (walk the suffixes with `:match_cons` and
    `fn:list:len`) and the sum is 9.
26. **Maps are `["key": value]`; structs are `{/field: value}`.** `:match_entry(M, "key", V)` reads a
    map entry.

## Declared but not usable in codeNERD (2026-09-19)

27. **Temporal reasoning parses and never evaluates in codeNERD.** `Decl p(X) temporal bound [...]`,
    `<-[0d, 30d] p(X)`, `[-[0d, 30d] p(X)`, `<+`, `[+` and `p(X)@[T1, T2]` all analyse (durations
    `d h m s ms`, timestamps, variables, `now`). A temporal operator cannot follow `!` (project,
    then negate). codeNERD configures no temporal store, so evaluation aborts: `temporal literal
    encountered but no temporal store configured`. No policy file uses temporal syntax.
28. **Custom lattices hang.** The spec's `fundep([K], [V])` + `merge([...], <pred>)` descriptors are
    implemented (`engine/seminaivebottomup.go`, `mergeDelta`, which keys on the leading columns), and
    a program using them loads; its evaluation never finishes, with the merge predicate spelled as a
    string or a name. Keep a minimum or maximum with `fn:min` / `fn:max` instead.

## Tools that disagreed with the engine (removed 2026-09-19)

The skill used to bundle a JavaScript CLI and Python analysers with their own Mangle grammars.
Each disagreed with the engine on a basic case (an error on `Decl p(A, B) bound [/name, /name].`,
VALID for a duplicate `Decl`, no result for a rule the engine evaluates, a negation read as a
positive premise) and was deleted. `scripts/README.md` lists them; the verdict is always
`nerd check-mangle`.

## What the canonical line has that the corpus does not use yet

`DerivationRecorder` / `MemoryRecorder` / `BuildFromRecording` (provenance), the `mgwhy` tool,
zstd simplecolumn fact stores, tagged-union types. See `960-FORK_FEATURES_v0.5.1.md`.
