---
name: mangle-programming
description: >
  Program in Mangle, the deductive database programming language codeNERD's kernel runs on
  (TauCeti's canonical mangle-go, pinned v0.5.1-0.20260413190942-4dcaa582c6d3). Use this skill to
  design or change anything in a .mg file: move a decision out of Go into policy, derive what enters
  the context window, derive test obligations and completion, mount a store as a virtual predicate,
  write recursion, negation or an aggregation, or debug a rule that never fires or derives too
  much. It is a programming model first (what is a witness, what is a judgement, which pattern
  fits) and a syntax reference second, with the engine behaviours that were verified by running
  them and the ones that were refuted. Also use it to check or lint .mg files and to find out what
  a program derives: `nerd check-mangle` (the kernel's own parser and analysis) and
  `nerd check-mangle --standalone --eval <predicates>` (run it and print the facts).
license: Apache-2.0
version: 1.1.0
mangle_engine: codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3
last_updated: 2026-09-19
---

# Mangle Programming: the programming model, then the syntax

Mangle is a deductive database programming language: typed declarations with modes, stratified
negation, aggregation through `|>` transforms, `fn:` builtins, structured data, and external
predicates over a host FFI. TauCeti's `mangle-go` is the canonical implementation and the one
codeNERD pins; Google's `github.com/google/mangle` is a frozen downstream snapshot, and anything
written against its 0.4.0 documentation is suspect until it has been run on the pinned engine.
The engine source and its spec are readable offline in the module cache
(`$GOMODCACHE/codeberg.org/!tau!ceti/mangle-go@v0.5.1-*/docs/`); where the spec and the engine
disagree, the engine wins (the spec spells the declaration keyword `bounds`, which does not parse).

In codeNERD the kernel is the executive. Go is the FFI, the drivers and the tools; every decision
that matters (what a turn is, whether it is done, what is delegated, what enters the window, what
the verdict is) is meant to be the fixpoint of rules over facts. The drift to hunt is a decision
computed in Go instead of derived. This skill exists to make the derived version the easy one to
write.

**Run it; do not reason about it.** Every example in this skill loads on the pinned engine
(checked 2026-09-19: 206 of 220 fenced programs load, the other 14 are marked wrong-way examples
that fail with the error their text states). This skill and codeNERD's prompt atoms have both been
wrong before, in ways that looked right, so any claim that matters gets a probe:

```bash
nerd check-mangle --standalone --eval sibling,has_stop probe.mg
```

## 1. The model in five sentences

1. **Go asserts witnesses; rules derive judgements.** A witness is something measured or told: a
   tool ran, a file's hash at a revision, a test exited non-zero, the user typed this. A judgement
   is a decision: done, relevant, needs a test, delegate. Go never asserts a judgement; every
   judgement is the head of a rule. (`turn_gate/3` is asserted; `turn_verified/1` is derived.)
2. **Thresholds and budgets are facts, never Go constants.** `working_nudge_rounds(8).` lives in
   the policy file, where a rule can read it and a reviewer can see it.
3. **The model cannot reach a judgement.** Predicates that carry a verdict are hard-blocked from
   the model's own assertions (`internal/core/mangle_updates.go`); a new verdict predicate is added
   there in the same change that declares it.
4. **What is resident is paid for on every evaluation.** The kernel re-derives the whole fixpoint
   from all EDB facts each time (there is no incremental path: it was unsound and was deleted), so
   bulk knowledge is mounted or loaded on demand, and per-turn facts are retracted at turn close.
5. **A rule that cannot fire is a bug nobody sees.** Every new predicate gets a test that asserts
   the witnesses and queries the judgement on a real kernel.

## 2. Pick the pattern

The patterns are specified, with runnable `.mg` and their verification results, in
`Docs/journeys/M2-mangle-programming-model.md` (design) and `Docs/journeys/M3-mangle-verification.md`
(what held when it was run). The distilled version, with what M3 changed, is
[references/010-PROGRAMMING_MODEL.md](references/010-PROGRAMMING_MODEL.md).

| You are trying to | Pattern | Live example in the corpus |
|---|---|---|
| read SQLite, vector search, CodeDOM or history from a rule | A: mount knowledge as a virtual predicate | `external_predicates.go`, `query_knowledge_graph` (constant inputs only, see section 3) |
| decide what enters a prompt, when, for whom | B: derived context injection | `policy/jit_needs.mg` (`target_need/2` -> atoms gated `world_states:`), `jit_compiler.mg`, `policy/stage_context.mg` |
| force the tests that harden a change | C: derived test obligations | `turn_untested/2` -> `turn_missing_evidence(_, /tests_not_written)` in `policy/coder_safety.mg` |
| keep working until the behaviour holds | D: completion as an obligation fixpoint | `policy/codedom_continuation.mg` (`pending_test`, `pending_fix`, discharge by the acting shard's result) |
| carry out a change with a blast radius | E: edit plans derived, executed by a tool | CodeDOM `apply_edits`; impact rules in `policy/` |
| answer "why was this decided" | F: provenance | `nerd why <pred>`; the engine's `DerivationRecorder` |
| turn evidence into a verdict | G: verdicts as facts | `turn_gate/3` -> `turn_verified/1` -> `turn_done/1` in `policy/coder_safety.mg` |

## 3. Engine truths on the pinned commit (verified by running them)

Full list with the probes: [references/020-ENGINE_TRUTHS_v0.5.1.md](references/020-ENGINE_TRUTHS_v0.5.1.md).
The ones that cost hours when forgotten:

- **A wildcard or an unbound variable inside a negated atom is silently deleted.**
  `q(X) :- p(X), !r(X, _).` becomes `q(X) :- p(X).` with no error (when another `_` sits in the
  same body it surfaces instead as `variable X1 is not bound`). Project first:
  `has_r(X) :- r(X, _).` then `!has_r(X)`. codeNERD's legislator atom taught the broken form as
  CORRECT until 2026-09-19; run, it blocked every action.
- **Negation is `!`; there is no `not`.** Analysis moves a negation after the atoms that bind it,
  so its position does not matter. Comparisons are not moved: `N > 0` before `N` is bound fails.
- **Comparisons are integer-only.** `<`, `>`, `<=`, `>=` on a float abort evaluation; scores are
  integers 0..100.
- **An `external()` premise whose input is a variable bound by an earlier atom panics**
  (`engine/topdown.go:99`). Externals join on literal constants in the rule text, or Go does the
  work where the data arrives and asserts a witness the rule joins on.
- **Transforms:** one stage is one `do`, then `let`s (a `let` may use the lets before it). No `_`
  in a transform rule's body. A second `|>` stage of only `let`s does not bind; a reducer with no
  `do` fails at evaluation; `let X = P` is a parse error. Filter with a comparison in the body.
- **Declarations:** the keyword is `bound`; one bound per argument, exactly; `/any` for an
  unconstrained column; type functions are capitalised (`fn:List(/string)`, `fn:Map`,
  `fn:Struct`, `fn:Union`). Source facts are not checked against bounds.
- **Structs do not destructure in a premise** (`data({/a: X})` fails at evaluation): use
  `:match_field(D, /a, X)` with `D` bound and `X` free. There is no `[H|T]`: `fn:list:cons`.
  Aggregating over `:list:member` collapses duplicate elements (facts are a set).
- **A name constant at the end of a clause swallows the period** after `=` or `!=`
  (`X != /done.`): a name may contain `.`. Put a space or a line break before the period.
- **Variables are letters and digits** (`P_Loser` does not lex); zero-arity predicates keep their
  parentheses (`has_stop()`); `bound` cannot name a predicate; `?pred(X)` is interpreter syntax,
  not program text.
- **One `Decl` per predicate per loaded unit**; a second anywhere is a hard analysis error.
  Unknown Decl descriptors are accepted silently, so a typo in one is never reported.
- **The created-fact limit counts the whole store and aborts the evaluation**, naming whichever
  predicate crossed the line: the named predicate is usually innocent. codeNERD's limits come from
  `core_limits` in config.
- **Strata add**, an aggregation is its own stratum, and the zero case of a count is a separate
  negation rule. Aggregating or negating inside a recursion cannot be stratified.
- **Body order is execution order** for atoms (nested loops as written): selective, bound atoms
  first; an external or an aggregate is never the first premise.
- **A Go string that starts with `/` may arrive as a name** (`types.Fact.ToAtom`), while the rule's
  quoted literal stays a string, and the two never unify. Declare the column's type and assert
  `types.MangleAtom` or a plain string on purpose.
- **Declared but unusable in codeNERD:** temporal operators (no temporal store: evaluation
  aborts), custom `fundep`/`merge` lattices (evaluation never finishes), `fn:count_distinct`,
  `fn:map:get`, `fn:pick_any`.

## 4. Syntax in one screen

```mangle
Decl turn_gate(Verb, Gate, Verdict) bound [/name, /name, /name].   # every predicate, before use
Decl turn_tests_green(Verb) bound [/name].                          # one bound per argument
Decl working_stop(Reason) bound [/name].
Decl working_control(Kind, Round) bound [/name, /number].
Decl has_stop().
Decl working_continue().
Decl layer(File, Layer) bound [/string, /name].
Decl files_per_layer(Layer, N) bound [/name, /number].
turn_tests_green(Verb) :- turn_gate(Verb, /test, /passing).          # variables UPPERCASE, atoms /lowercase
has_stop() :- working_stop(_).                                       # project before negating
working_continue() :- working_control(_, _), !has_stop().
files_per_layer(Layer, N) :- layer(File, Layer)
    |> do fn:group_by(Layer), let N = fn:count().                    # aggregation is a transform
# comments are '#'; every clause ends with '.'; strings are "quoted"; numbers are int64
```

Before writing any rule, read [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md): the
errors models make are Prolog, SQL and Souffle habits, and that file lists them with corrections.

## 5. Adding a decision to codeNERD's policy (the working checklist)

1. Name the witness Go already measures (or must start asserting) and the judgement you want.
2. `Decl` both in the file that owns them, under `internal/core/defaults/policy/` (or
   `schemas_*.mg` for shared EDB), with `bound [...]` on every column.
3. Write the rule; project before every negation; keep thresholds as facts.
4. Run it: put the rule and a few witness facts in a scratch file and
   `nerd check-mangle --standalone --eval <judgement> scratch.mg`, including the case that must
   derive nothing. Then check the real file in context: `nerd check-mangle <policy file>`.
5. Assert the witness from Go next to the measurement, and retract it where the turn's other
   per-turn facts are retracted (`internal/session/executor.go`, `recordBuildState` is the model).
6. If the predicate carries a verdict, add it to the hard block in
   `internal/core/mangle_updates.go` and to the world manifest in `internal/shards/registration.go`.
7. Regenerate the predicate corpus: build and run `cmd/tools/predicate_corpus_builder`.
8. Test on a real kernel: assert witnesses, query the judgement, and include the negative case.
9. Run `go test ./internal/core/ ./internal/session/ ./internal/mangle/ ./cmd/nerd/chat/`: the
   wiring tests there fail on a body literal with no Decl and on a Go query of a predicate no rule
   or assertion produces.

## 6. Just-in-time knowledge: predict the need, serve it then, never stuff the window

The window is not a bucket. Every atom in a prompt is there because the kernel derived that
this compile needs it, at this moment, for this agent; everything else stays out and remains
retrievable. The mechanism, live since 2026-09-18:

1. **A need is a derived world state.** `policy/jit_needs.mg` declares what a compile aimed at
   a kind of target will need, e.g. `target_need(/mangle, /authoring_mangle).` More needs are
   rules over witnesses (a red gate, a failing test, a tool misused), never Go booleans.
2. **The executor asks at every compile boundary** (the turn, and each planned step when it
   starts) with the target bound as a constant: `target_need(/mangle, Need)`. It copies the
   answer into `CompilationContext.DerivedNeeds`; it does not decide anything.
3. **An atom declares the need it serves** with `world_states: [<need>]`. World-state gates are
   fail-closed: the atom is in the prompt when the need holds and absent otherwise. Pair it
   with `is_mandatory: true` for knowledge that must be present whenever the need holds.
4. **Keep each need narrow.** A need admits its atoms into the skeleton on every compile it
   holds for, so it costs tokens every time. Serve the core (a few thousand tokens), and let
   relevance (vector search, which the language unblocks) admit the rest of a corpus.
5. **A second opinion may veto, never admit wholesale.** Wiring a broad policy admission into
   the selector once took a compile from 67 atoms to 254 and 99% of the budget
   (`policy/jit_selection.mg` header). Admission is the narrow, declared need; everything else
   in policy is a veto.
6. **Test it end to end on a real kernel**: derive the need, compile, assert the atoms are in;
   derive no need, compile, assert they are out (`internal/system/jit_need_served_test.go`).

Measured when it landed: a coder compile aimed at a `.mg` file went from 41 atoms with none of
the 119 `/mangle` atoms to 43 atoms carrying the Mangle core and the engine truths, and at
13k tokens it is still smaller than the same compile for a Go file (16k).

## 6a. How codeNERD's own agents get this knowledge

The prompt corpus carries a Mangle expert library: 119 atoms keyed `languages: ["/mangle"]` under
`internal/prompt/atoms/mangle/` and `internal/prompt/atoms/language/mangle*.yaml` (syntax, safety,
stratification, aggregation edge cases, external predicates, performance, debugging). The JIT
selects them when the compilation context's language is `/mangle`, which the session executor sets
from the file the turn or the planned step is aimed at (`languageOfFile`, `stepSystemPrompt` in
`internal/session/work_steps.go`). An engine truth learned the hard way belongs in that corpus as an
atom as well as here: this skill teaches the development agent, the atoms teach the agent inside
the harness. The corpus is guarded by tests in `internal/prompt/embedded_corpus_mangle_reducers_test.go`
(reducer forms, the typed-variable Decl, wildcard negation) and ratcheted by
`cmd/tools/validate_prompt_atoms/mangle_examples_load_test.go` (fenced examples that fail to load
may only decrease).

## 7. The reference library: what each file is for, and when to load it

| File | What it holds | Load it when |
|---|---|---|
| [000-ORIENTATION](references/000-ORIENTATION.md) | reading paths through the library by goal | you do not know which file answers your question |
| [010-PROGRAMMING_MODEL](references/010-PROGRAMMING_MODEL.md) | the seven patterns (A-G), distilled, with what verification changed | designing a new decision, mount, obligation or verdict in policy |
| [020-ENGINE_TRUTHS_v0.5.1](references/020-ENGINE_TRUTHS_v0.5.1.md) | every verified and refuted engine behaviour, with the error each one prints | a rule derives too much or nothing, an error message makes no sense, or before trusting any claim |
| [100-FUNDAMENTALS](references/100-FUNDAMENTALS.md) | facts, rules, bottom-up evaluation, the closed world, stratification, how Mangle differs from SQL and Prolog | you are new to Mangle or your mental model is SQL/Prolog-shaped |
| [150-AI_FAILURE_MODES](references/150-AI_FAILURE_MODES.md) | the mistakes models make, each wrong form next to the right one; the engine's complete builtin list | before writing any rule, and when reviewing model-written Mangle |
| [200-SYNTAX_REFERENCE](references/200-SYNTAX_REFERENCE.md) | every construct: literals, operators, negation, transforms, declarations, safety | you need the exact form of a construct |
| [250-BUILTINS_COMPLETE](references/250-BUILTINS_COMPLETE.md) | builtin functions and predicates with modes and examples | calling an `fn:` or `:` builtin, or checking whether one exists |
| [300-PATTERN_LIBRARY](references/300-PATTERN_LIBRARY.md) | worked programs (SBOM analysis, infrastructure policy, matching, routes) and SQL equivalents | you want a complete program to start from |
| [400-RECURSION_MASTERY](references/400-RECURSION_MASTERY.md) | closure, paths with lists, cycle-free paths, shortest paths, mutual recursion, trees, levels | writing any recursive rule |
| [450-PROMPT_ATOM_PREDICATES](references/450-PROMPT_ATOM_PREDICATES.md) | the JIT compiler's predicates (selection, dependencies, exclusion, ordering) | changing atom selection or reading `jit_compiler.mg` |
| [500-AGGREGATION_TRANSFORMS](references/500-AGGREGATION_TRANSFORMS.md) | transform pipelines, grouping, conditional and nested aggregation | writing `|>` / `do` / `let` |
| [600-TYPE_SYSTEM](references/600-TYPE_SYSTEM.md) | `Decl` and `bound` forms, every type constant and function, what is checked where | writing or fixing a declaration |
| [700-OPTIMIZATION](references/700-OPTIMIZATION.md) | rule ordering, semi-naive internals, memory, scaling limits | a rule is slow or the fact limit trips |
| [800-THEORY](references/800-THEORY.md) | the logic underneath: fixpoint and stratified semantics, complexity | you need to argue why a program terminates or is well-defined |
| [900-ECOSYSTEM](references/900-ECOSYSTEM.md) | embedding in Go, production architectures, monitoring, testing, deployment | building a service around the engine outside codeNERD |
| [950-ADVANCED_ARCHITECTURE](references/950-ADVANCED_ARCHITECTURE.md) | Mangle as a reasoning kernel: ReBAC, graph topology, neuro-symbolic mediation, AST analysis, taint | designing a large policy system or static analysis in Mangle |
| [960-FORK_FEATURES_v0.5.1](references/960-FORK_FEATURES_v0.5.1.md) | what the canonical line has that 0.4.0 lacks: provenance recorder, `mgwhy`, temporal syntax, tagged unions, simplecolumn stores | using a feature newer than Google's 0.4.0 docs (read its codeNERD caveats) |
| [ADVANCED_PATTERNS](references/ADVANCED_PATTERNS.md) | stratified negation, path tracking, complex aggregation, structured types, what lattices do on this engine | a pattern the numbered files do not cover |
| [GO_API_REFERENCE](references/GO_API_REFERENCE.md) | the engine's Go packages: `ast`, `parse`, `analysis`, `engine`, `factstore`, conversions | writing Go against the engine (in codeNERD, start from `internal/mangle`) |
| [PRODUCTION](references/PRODUCTION.md) | architecture, monitoring, debugging by stages, tests as failure predicates, error handling | debugging a program in place, or writing a Mangle test |
| [SHARDING_STRATEGIES](references/SHARDING_STRATEGIES.md) | when and how to split a large world across kernels, and what codeNERD already does | a single kernel's fact count or evaluation time is the problem |

Design-side sources in the repository (the skill points at them, it does not copy them):
`Docs/journeys/M0-mangle-language-surface.md` (the engine's real feature surface with
`path#symbol` evidence), `M1-mangle-corpus-usage.md` (what codeNERD's `.mg` files use and which Go
decisions are expressible as rules), `M2` and `M3` as above, `M4-skill-uplift-log.md` (how this
skill was checked), and `internal/mangle/agents.md` (the repository's own Mangle rules of the road).

## 8. Checking and running Mangle

The verdict comes from the pinned engine, through the `nerd` binary:

| Command | Does |
|---|---|
| `nerd check-mangle <files>` | parse, declarations, arity, safety, stratification, after loading codeNERD's shared schemas: a policy file checked in the context the kernel loads it in |
| `nerd check-mangle --standalone <files>` | the same, for a program on its own (an example, a probe, a template) |
| `nerd check-mangle --standalone --eval p,q <file>` | run it and print every fact `p` and `q` hold, in the engine's own spelling |
| `nerd query <predicate>` | the facts a predicate holds in the live kernel |
| `nerd why <predicate>` | how the live kernel derived them |

`--eval` is the tracer: when a rule derives too much or nothing, evaluate each predicate in its
chain and find the first one whose facts are wrong. Two advisory Python helpers remain in
[scripts/](scripts/README.md): `diagnose_stratification.py` names the cycle when a program cannot
be stratified, and `profile_rules.py` flags join orders that risk a large intermediate result. The
JavaScript CLI and the other analysers that used to ship here disagreed with the engine and were
removed (the list and the reasons are in `scripts/README.md`).

## 9. Templates and worked programs

[assets/](assets/README.md) holds programs that load and evaluate on the pinned engine:
`starter-schema.mg` + `starter-policy.mg` (a pair that loads as one unit: closure, paths,
set difference by projection, aggregation, classification, structured data, time, validation),
`examples/vulnerability-scanner.mg`, `examples/access-control.mg`,
`examples/aggregation-patterns.mg`, and `go-integration/` (embedding the engine directly). For
codeNERD's own declarations read the live `internal/core/defaults/schemas*.mg`; the skill keeps no
copy of them.
