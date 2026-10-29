# M1 — How codeNERD uses its Mangle kernel today

## Status

- last updated: 2026-09-18 03:55
- done: 1 engine embedding; 2 corpus inventory; 3 feature census; 4 virtual predicates; 5 misuse catalogue; 6 piggyback allowlist; 7 Go-side executive decisions; 8 provenance; 9 performance; open items (a)–(e) from the first pass closed (see §5 item 10, §7 rows 10–11, §2.2, §9)
- open: none blocking. Residual **unverified** points are marked inline: `query_graph` handler line; `cortex_kernel.go` per-shard evaluation line; `shouldClarifyIntent` body; `model_lifecycle.go` TraceQuery purpose; the exact breakdown of 1,845 declared vs 1,693 `Decl` lines; which `clarification.mg` rule reads perception's `clarification_needed`

Conventions: every claim cites `path:line`, `path#symbol`, or a predicate in a named `.mg` file. Anything not checked against source is marked **unverified**. Counting commands are recorded so the numbers can be rerun. Scratch under `C:/Temp/mangle-study/` (`census.py`, `evalstats.py`, `per_file.tsv`, `decl_arity.json`, `mg_inventory.tsv`). Branch `dogfood/c2-closure`. Paths are relative to `C:/CodeProjects/codeNERD`.

## 1. How the engine is embedded

There are **two** embeddings of the TauCeti `mangle-go` engine, and they are not the same object:

| Layer | Type | File | Role |
|---|---|---|---|
| Kernel | `core.RealKernel` | `internal/core/kernel_types.go:47` | The executive. Holds the constitution (schemas + policy + learned) as three strings, an EDB slice `k.facts`, a cached atom slice, and rebuilds a fresh `factstore` on every evaluation. Wraps `mangle-go` directly (`analysis`, `engine`, `factstore`, `provenance` imports at `kernel_types.go:16-19`). |
| Wrapper | `mangle.Engine` | `internal/mangle/engine.go:62` | A "Hollow Kernel" wrapper with its own `ConcurrentFactStore`, persistence hooks, `ReplaceFactsForFile`, and per-file reverse index. Used for task-private engines, the `nerd why` hollow tracer (`cmd/nerd/cmd_query.go:289-311`), and as the base under `DifferentialEngine` (`kernel_eval.go:506`). Comment at `engine.go:1` still says "Google Mangle"; the import is `codeberg.org/TauCeti/mangle-go` (`engine.go:23-30`). |

The pinned engine is `codeberg.org/TauCeti/mangle-go` (TauCeti is the canonical Mangle; pin `v0.5.1-0.20260413190942-4dcaa582c6d3` per project memory). Production parsing goes through `internal/mangle/parse_lock.go` (`ParseUnit`/`ParseAtom`) because the ANTLR prediction state is not concurrency-safe (`internal/mangle/agents.md:5`).

### 1.1 Load (boot)

`NewRealKernel` / `NewRealKernelWithWorkspace` / `NewRealKernelWithPath` (`kernel_init.go:70,101,139`) all do: `newRealKernelBase()` → `loadMangleFiles()` → `injectBootFacts()` → `evaluate()`. A boot that fails to compile the embedded constitution is fatal (`kernel_init.go:88-92`). `injectBootFacts` filters ephemeral predicates (`IsEphemeral`, `internal/core/fact_categories.go:131`) so a stale `user_intent`/`pending_action` cannot fire at boot ("quiescent boot", `kernel_init.go:16-37`).

`loadMangleFiles` (`kernel_init.go:265-505`) builds three strings in this order:

1. **Schemas**: `defaults/schemas.mg` (index), then every file in `defaultSchemaFiles` in list order (`kernel_types.go:210-240`, 28 files). Each is embedded via `//go:embed defaults/*.mg defaults/schema/*.mg defaults/policy/*.mg` (`kernel_types.go:193`). A missing embedded file is fatal (`readRequiredEmbeddedFile`, `internal/core/policy_inventory.go:118`).
2. **Policy**: every `defaults/policy/*.mg` in `embed.FS.ReadDir` order (lexicographic, `kernel_init.go:311-329`), then the 12 root "core modules" from `defaultCorePolicyModules` (`policy_inventory.go`: `doc_taxonomy, topology_planner, build_topology, campaign_rules, selection_policy, taxonomy, inference, jit_compiler, reviewer, tester, go_safety, benchmarks`). Then `loadEmbeddedIntentFacts()` (`kernel_init.go:354`; body `internal/core/intent_loader.go:5-33`) parses `defaults/schema/intent_*.mg` as **ground facts** and keeps only predicates in `defaultIntentFactPredicates()`, appending them to `bootFacts`.
3. **Learned**: `defaults/learned.mg` (2 lines, effectively empty) then the workspace `.nerd/mangle/learned.mg`, self-healed through `SchemaValidator` before it is appended (`kernel_init.go:439-483`, `kernel_validation.go:79`).

User extension points, all under `.nerd/mangle/` unless noted: `extensions.mg` (schemas), `policy_overrides.mg` (policy), `.nerd/northstar.mg` (schemas), `learned.mg`. They pass through `LoadHybridMangleFile`, which splits `TAXONOMY:/INTENT:/PROMPT:` directives from logic (`kernel_init.go:379-437`).

`rebuildProgram` (`kernel_eval.go:75-170`) concatenates schemas + policy + learned (`writeProgramLocked`, `kernel_eval.go:52`), parses **one** unit, runs `analysis.AnalyzeOneUnit`, then `analysis.Stratify`, caching `strata` and `predToStratum`. It runs only when `policyDirty` is true or `programInfo` is nil (`kernel_eval.go:191`). Live measurement (kernel.log 2026-09-17 23:59:54): program size 898,609 bytes, 3,453 clauses, **1,845 predicates declared** (the corpus has 1,693 `Decl` lines; the remainder are learned/extension/multi-arity declarations, **unverified** breakdown).

### 1.2 Assert / retract

- `Assert(fact)` (`kernel_facts.go:555`): `sanitizeFactForNumericPredicates` → `validateAgainstDeclLocked` → `addFactIfNewLockedErr` (dedupe by canonical string, EDB cap, `factToAtomLocked` conversion) → `factsDirty.Store(true)` → publish on `FactEventBus`. **A rejected fact returns an error; a duplicate returns nil** (the conflation was a real defect, comment at `kernel_facts.go:424-441`).
- `system_heartbeat` is special-cased **by predicate name in Go** to upsert in place without dirtying (`kernel_facts.go:566,608`), because every heartbeat used to force a 10–17 s re-eval on a 28k-fact kernel.
- `AssertBatch` (`kernel_facts.go:677`): same checks per fact, one dirty mark, collects rejections with `errors.Join`.
- `AssertWithoutEval` + `Evaluate` (`kernel_facts.go:762,792`) for bulk loads.
- `Retract(predicate)`, `RetractFact`, `RetractExactFact`, `RetractExactFactsBatch`, `RemoveFactsByPredicateSet` (`kernel_facts.go:847-1005`) — all call `rebuild()` which nils the atom cache, dirties, and **invalidates the differential engine** (`kernel_eval.go:597-603`): there is no DRed; a retract forces a full re-derivation.
- `Transaction()` (`kernel_transactions.go:42`) stages retracts+asserts and commits once.
- Type coercion at the Go boundary: `types.Fact.ToAtom()` then `coerceAtomToDeclLocked` (`kernel_fact_decl.go:46`) which narrows an integral float in a `/number` slot to `ast.Number` and **rejects** a fractional float, because the fork's `<`/`<=`/`>`/`>=` are int64-only and one float aborts the entire fixpoint (`kernel_fact_decl.go:31-40`). The wrapper's `convertValueToTypedTerm` (`engine.go:735`) is a *different* coercion (auto-promotes identifier-like strings to names when the bound is unknown); `evaluateDiffLocked` deliberately bypasses it (`kernel_eval.go:415-421`).

### 1.3 When the fixpoint is recomputed

Evaluation is **lazy and whole-program**: nothing re-derives on assert. `Query`, `QueryCallback`, `QueryAll`, `Explain` call `ensureEvaluated()` (`kernel_eval.go:615`), which under a singleflight mutex runs `evaluate()` if `factsDirty`. `evaluate()` (`kernel_eval.go:181`) rebuilds the program if dirty, then chooses:

- **Differential path** (`evaluateDiffLocked`, `kernel_eval.go:406`) only when `CODENERD_DIFF_EVAL=1` (`features.IsDiffEvalEnabled`, `internal/features/features.go:479`), the EDB is ≤ 10,000 facts (`differentialFactCeiling`, `kernel_types.go:161`), no provenance recorder, no external predicates registered, and the path has not been demoted after a > 2 s delta (`diffDemoteThreshold`, `kernel_types.go:173`). Measured 2026-09-05: a one-fact delta on the 48K-fact world shard took 91 s (`kernel_types.go:139-144`). **Default is OFF**, so in production every evaluation is the full path.
- **Full path** (`evaluateFullLocked`, `kernel_eval.go:256`): a fresh store (`MultiIndexedArrayInMemoryStore` above 1,024 facts, `kernel_eval.go:841`) is filled from `cachedAtoms`, external predicates are registered from the VirtualStore filtered by `Decl … external()` (`kernel_eval.go:331-345`), and `engine.EvalStratifiedProgramWithStats` runs with `WithCreatedFactLimit`. The result replaces `k.store` wholesale (`kernel_eval.go:361`). Derived facts are therefore **never persisted** and are rebuilt from scratch on every dirty query.

Live numbers (`.nerd/logs/20260918_035947…_kernel.log`, session 2026-09-17 23:59 → 00:14): 127 "fixpoint reached" lines, 179 "lazy_evaluate triggered" lines, **913 strata** per program. Distribution in §9.

### 1.4 Fact ceilings

| Ceiling | Where enforced | Default | Config knob | Is the knob wired? |
|---|---|---|---|---|
| EDB facts | `addFactIfNewLockedErr` (`kernel_facts.go:443-452`) | `defaultMaxFacts = 250_000` (`kernel_init.go:215`) | `core_limits.max_facts_in_kernel` (`internal/config/limits.go:14`) | **No.** `SetMaxFacts` (`kernel_init.go:219`) has zero production callers. The value reaches `core.LimitsEnforcer` (`internal/system/factory.go:1747`) whose `GetMaxFactsInKernel` (`internal/core/limits.go:300`) is never called. |
| Derived facts (gas) | `evaluateFullLocked` via `engine.WithCreatedFactLimit` (`kernel_eval.go:313`) | `defaultDerivedFactLimit = 500_000` (`kernel_init.go:188`) | `core_limits.max_derived_facts_limit` (default 100,000 in config, `limits.go:146`) | **No.** `SetDerivedFactLimit` has one caller, the rule-court sandbox (`internal/core/rule_court.go:99`). Production runs at 500k, not the configured 100k. |
| Wrapper engine | `Engine.insertFactLocked` (`engine.go:631`), `evalWithGasLimit` (`engine.go:230`) | `FactLimit 100_000`, `DerivedFactsLimit 100_000` (`engine.go:44-52`) | `mangle.Config` | Per constructor. |

Finding: the user-facing `core_limits` fact ceilings are display-only for the RealKernel. Config says 250k/100k; the kernel enforces 250k/500k from constants.

### 1.5 How Decl type checks run

Three layers, none complete:

1. **Analyzer** (`analysis.AnalyzeOneUnit`, `kernel_eval.go:121`): declaration-first, arity, stratification, safety of negation; runs only at `rebuildProgram`. It cannot see runtime facts.
2. **Go-side bound check on assert** (`validateAgainstDeclLocked`, `kernel_undeclared.go:80`): checks only `/number` and `/string` bounds (`checkBoundAgainstValue`, `kernel_undeclared.go:109`); `/name` is deliberately unchecked because a Go string cannot distinguish an atom from a string. An undeclared predicate/arity is **stored anyway** with one warning per process (`warnIfUndeclaredLocked`, `kernel_undeclared.go:52-72`): "the fact is stored but no rule can read it, and Query will not return it".
3. **Decl-directed coercion** (`coerceAtomToDeclLocked`, above). Only the **first** `Bounds[0]` of a Decl is consulted (`kernel_fact_decl.go:24`; same limitation in `engine.go:692-695`).

Not checked anywhere at runtime: `/name` vs `/string` mismatches, list vs scalar (see §5 item 2), and Decls with union bounds.

### 1.6 How errors surface

- Boot compile failure → constructor error, process fails (`kernel_init.go:88`).
- Fixpoint failure → `evaluate` returns error, `k.store` is **not** replaced (`kernel_eval.go:352-359`), every `Query` returns the same error until the offending fact is gone. The error message names the value, not the predicate (`kernel_fact_decl.go:38-40`).
- Failed writes → `noteMutationFailure` logs WARN "`the kernel does not hold it`" (`kernel_facts.go:545`).
- Learned-rule failures → `HotLoadRule` (validate only), `HotLoadLearnedRule` (validate + persist), `validateRuleSandbox` clones a sandbox kernel with `sandbox=true` so rejections log at DEBUG (`kernel_policy.go:180,360,443`; `kernel_types.go:65-78`).
- Query for an undeclared predicate → WARN "not found in declarations" and empty result (`kernel_query.go:121-125`).
- Slow fixpoint → `performance.log` line `kernel.evaluate.fixpoint` at `warn` above `threshold_ms:500` (observed).

### 1.7 `debug_program_ERROR.mg`

Written by `writeFailedProgramDump` (`kernel_eval.go:826`) to `<workspace>/.nerd/debug/debug_program_ERROR.mg` when `analysis.AnalyzeOneUnit` fails on a **non-sandbox** kernel. It contains the full concatenated program (schemas + policy + learned), which is why a dump is ~850 KB. The tree currently holds **15 stale dumps** under `internal/*/.nerd/debug/` and `cmd/nerd/**/.nerd/debug/` (all 2026-09-04 16:15–22:02 except `internal/core/.nerd/debug/` at 2026-09-08 00:07, 552,006 bytes). These are test-run artefacts: tests that boot a kernel with `workspaceRoot` = the package dir and hit an analysis failure. They are untracked (`.nerd` is gitignored) but they inflate any naive `.mg` census (see §2.1).

## 2. The corpus

### 2.1 Counting commands

```powershell
# inventory of every .mg file (pruned), path/lines/Decl/rule counts -> C:\Temp\mangle-study\mg_inventory.tsv
$files = Get-ChildItem -Recurse -Filter *.mg -File | Where-Object { $_.FullName -notmatch '\\(\.git|\.nerd|node_modules|Docs|\.gocache[^\\]*)\\' }
foreach ($f in $files) { $c = Get-Content $f.FullName; "$($f.FullName)`t$($c.Count)`t$(($c | Select-String '^\s*Decl\s').Count)`t$(($c | Select-String ':-').Count)" }
```

Whole tree (pruned as above): **251 files / 45,715 lines**. Of those, 103 are skill assets duplicated across `.agent/`, `.agents/`, `.claude/`, `.codex/`, `.gemini/` (log-analyzer schema, mangle-programming examples, stress-tester adversarial corpora) and are never loaded by the binary. The **live corpus is `internal/**`**: `python C:/Temp/mangle-study/census.py` (comment-stripped, `.nerd` excluded) → **135 files / 27,159 lines / 1,693 Decl / 2,173 rules / 4,980 ground facts**. The earlier measurement (134 / 26,881 / 1,670 / 2,179) is consistent within corpus churn.

**Census hazard**: counting `internal/**/*.mg` without excluding `.nerd/` yields 148 files / 283,904 lines / 21,219 Decl because of the 13 stale `debug_program_ERROR.mg` dumps (§1.7).

### 2.2 Directory groups (live corpus)

| Group | Files | Lines | Decl | Rules | Loaded how |
|---|---|---|---|---|---|
| `internal/core/defaults/schemas*.mg` + `schemas.mg` | 28 | 6,478 | 1,506 | 24 | schema string, `defaultSchemaFiles` order |
| `internal/core/defaults/chaos.mg` | 1 | 272 | 25 | 26 | in `defaultSchemaFiles` (position 11) |
| `internal/core/defaults/policy/*.mg` | 77 | 11,059 | 111 | 2,075 | policy string, lexicographic |
| `internal/core/defaults/*.mg` root core modules | 12 | 3,420 | 96 | 435 | policy string, after `policy/`, `defaultCorePolicyModules` order |
| `internal/core/defaults/learned.mg` | 1 | 2 | 0 | 0 | learned string |
| `internal/core/defaults/schema/intent_*.mg`, `prompts.mg` | 16 | 6,144 | 0 | 6 | `loadEmbeddedIntentFacts` (`intent_loader.go:5`) — parsed as ground facts, filtered by `defaultIntentFactPredicates()` |
| `internal/context/working_set.mg` | 1 | 143 | 23 | 18 | `//go:embed working_set.mg` (`internal/context/working_set.go:24`); `NewWorkingSet` (:42-78) builds a **private `mangle.Engine`** (`AutoEval=true`) from `core.DefaultCorpusText()` schemas + `policy/context_compilation.mg` + this file, then `Evaluate()`s once. "It never writes facts to the action kernel" (:31-32) |

Per-file rows (path, lines, Decl, rules, ground facts) are in `C:/Temp/mangle-study/per_file.tsv`. Purpose lines below are the file's own header comment, lightly trimmed.

#### Schemas (28 + chaos), load order

| # | File | Lines | Decl | Purpose (header) |
|---|---|---|---|---|
| 0 | `schemas.mg` | 146 | 0 | index / core predicate docs |
| 1 | `schemas_intent.mg` | 235 | 84 | Intent & Focus Resolution |
| 2 | `schemas_world.mg` | 98 | 13 | File topology, symbol graph, diagnostics |
| 3 | `schemas_execution.mg` | 288 | 54 | TDD loop & action execution |
| 4 | `schemas_browser.mg` | 109 | 67 | Browser DOM semantic layer |
| 5 | `schemas_project.mg` | 76 | 17 | Project profile, user prefs, session state |
| 6 | `schemas_dreamer.mg` | 140 | 23 | Speculative Dreamer |
| 7 | `schemas_memory.mg` | 197 | 43 | Memory tiers; **all 8 `external()` virtual-predicate Decls live here** (lines 54–84) |
| 8 | `schemas_knowledge.mg` | 139 | 20 | Knowledge atoms, LSP, semantic matching |
| 9 | `schemas_learning.mg` | 29 | 4 | Learned exemplars + intent overrides |
| 10 | `schemas_state.mg` | 226 | 26 | Ouroboros state machine (22 rules inside a schema file) |
| 11 | `chaos.mg` | 272 | 25 | Adversarial testing (PanicMaker, Nemesis) |
| 12 | `schemas_safety.mg` | 277 | 60 | Constitution, git safety, shadow mode |
| 13 | `schemas_analysis.mg` | 209 | 41 | Spreading activation, strategy, impact |
| 14 | `schemas_misc.mg` | 236 | 49 | Northstar, continuation, benchmarks; `shard_result/5`, `pending_test/2`, `pending_review/2` at 188–194 |
| 15 | `schemas_codedom.mg` | 263 | 61 | Code DOM & interactive elements |
| 16 | `schemas_codedom_polyglot.mg` | 209 | 48 | Polyglot language facts |
| 17 | `schemas_testing.mg` | 258 | 48 | Verification, reasoning traces, pytest |
| 18 | `schemas_campaign.mg` | 446 | 105 | Campaign orchestration |
| 19 | `schemas_intelligence.mg` | 142 | 63 | Campaign intelligence & context |
| 20 | `schemas_tools.mg` | 422 | 91 | Ouroboros, tool learning, routing; `query_traces`/`query_trace_stats` externals at 320/325 |
| 21 | `schemas_mcp.mg` | 275 | 48 | MCP integration |
| 22 | `schemas_prompts.mg` | 474 | 82 | Dynamic prompt composition & JIT |
| 23 | `schemas_reviewer.mg` | 438 | 71 | Static analysis & data flow |
| 24 | `schemas_shards.mg` | 702 | 206 | Shard delegation & coordination (largest Decl file); `delegation_candidate/3` at 24 |
| 25 | `schemas_coder.mg` | 169 | 98 | Coder shard declarations |
| 26 | `schemas_projectdoc.mg` | 70 | 10 | nerd.md machine-readable half |
| 27 | `schemas_context.mg` | 70 | 7 | Context compilation pipeline (C1+C4) |

#### Policy (`defaults/policy/`, 77 files, lexicographic load)

Largest by rules: `intent_routing_rules.mg` 588 lines / 193 rules ("replace hardcoded shard logic with declarative Mangle derivations"); `constitution.mg` 586 / 133 (constitutional safety, `permitted/…`); `intelligence.mg` 413 / 80; `autopoiesis.mg` 242 / 47; `coder_classification.mg` 224 / 47; `coder_workflow.mg` 296 / 45; `jit_selection.mg` 333 / 43; `coder_quality.mg` 227 / 42; `bridge.mg` 207 / 41 (stratum-1 normalisation); `delegation.mg` 410 / 40 (shard delegation, `reasoning_intensive_action/1`, `should_delegate/1` at 335, `is_multi_step/0` at 355); `shards.mg` 306 / 38; `browser_honeypot.mg` 115 / 37; `policy_mcp.mg` 362 / 37; `validation.mg` 325 / 37; `prompt_northstar.mg` 213 / 36; `campaign_tasks.mg` 206 / 35; `prompt_context.mg` 199 / 35; `taxonomy_inference.mg` 294 / 35; `verification.mg` 203 / 35. Notable self-descriptions: `routing_arbitration.mg:1` "the single DECIDE point for an interactive turn"; `stage_context.mg`: "Mangle tables, not Go switches"; `context_compilation.mg`: "Derives context relevance from kernel state instead of Go-side heuristics"; `regression_battery.mg` records an explicit DECISION that `/run_regression_battery` is not a `safe_action`; `coder_safety.mg:71-115` holds the turn verdict (`turn_evidence/6`, `hollow_success/1`, `turn_executed/1`, `turn_done/1`). Two files are pure EDB (`jit_config.mg`, `system_config.mg`: 0 rules), one is schema-in-policy (`schemas_perception_latency.mg`: 9 Decl, 0 rules).

#### Root core modules (12, loaded after `policy/`)

`doc_taxonomy.mg` 42/3, `topology_planner.mg` 39/5, `build_topology.mg` 114/8, `campaign_rules.mg` 1050/160 (largest single rule file), `selection_policy.mg` 27/5, `taxonomy.mg` 568/7 (14 Decl, mostly taxonomy facts), `inference.mg` 118/14, `jit_compiler.mg` 319/29, `reviewer.mg` 618/80 (header: "Loaded by ReviewerShard kernel alongside base policy" — but it is in the base load list), `tester.mg` 305/54 (same note), `go_safety.mg` 39/8, `benchmarks.mg` 81/3.

Three headers still say `internal/mangle/<file>.mg` (`build_topology.mg`, `doc_taxonomy.mg`, `topology_planner.mg`) — the files moved; the comment did not.

## 3. Feature-usage census

Command: `python C:/Temp/mangle-study/census.py` (source in scratch; scans `internal/**/*.mg`, excludes `.nerd`, strips `#` comments before counting, so commented-out examples are not counted). Raw grep without comment stripping gives slightly higher numbers (e.g. 42 `|>` vs 34).

| Feature | Count | Note |
|---|---|---|
| Files / lines | 135 / 27,159 | |
| `Decl` | 1,693 | 1,512 carry `bound [...]`; 21 carry `descr [...]`; 17 carry `mode(...)`; 10 carry `external()` |
| Rules (`:-`) | 2,173 | |
| Ground facts in `.mg` | 4,980 | mostly intent corpus (`schema/intent_*.mg`) and `taxonomy.mg` |
| Negated literals | 391 | `!pred(...)` after comment stripping (399 raw) |
| `|>` transforms | 34 | 29 `|> do`, 5 `|> let` |
| `fn:group_by` | 29 | |
| `fn:count` | 24 | |
| `fn:plus` / `fn:minus` / `fn:max` / `fn:div` / `fn:mult` | 20 / 6 / 5 / 3 / 3 | arithmetic is int64-only on this fork (`kernel_fact_decl.go:33`) |
| `fn:pair`, `fn:string:concat` | 1 each | |
| `:string:contains` | 122 | the one built-in predicate in heavy use (mostly `intent_routing_rules.mg`, `taxonomy_*`) |
| `:string:ends_with` | 1 | |
| `fn:list:*`, `fn:map:*`, `fn:struct:*`, `fn:sum`, `fn:min`, `fn:collect` | 0 | **never used** in the live corpus (they do appear in skill example files, which are not loaded) |
| Structured types in Decls | 0 | the ~1,453 Decl lines containing `[` are the `bound [/string, /name]` syntax. True `fn:List(...)`/`fn:Map`/`fn:Struct`/`fn:Union` bounds: 0 |
| Multi-arity predicates | 1 | `task_complexity/1` and `/2` — the only symbol declared at two arities (from `decl_arity.json`) |
| Recursive predicates | 18, all directly self-recursive: `activation, base_prohibited, code_contains, context_reachable, file_reachable, has_test_coverage, impact_graph, impacted, include_in_context, intelligence_depends_transitive, invalid, is_relevant, path_of_length, prohibited, symbol_reachable, tdd_state, tentative, test_depends_on_transitive` | detected by head→body reachability over rule bodies; mutual recursion beyond self-loops: none found |

Interpretation: the corpus is overwhelmingly **flat Horn clauses over atoms and strings** with negation. Aggregation is rare (29 `do` blocks in 2,173 rules) and structured data (lists, maps, structs) is unused in Decls; where a "list" is needed the Go side serialises to a JSON string (`engine.go:834-856`) or the schema joins on multiple rows. Modes are declared on only 17 predicates — the 10 externals plus 7 others — so `Engine.Query` synthesises all-output modes (`engine.go:889-901`). `Package`/`Use` declarations: none found by the census.

## 4. Virtual predicates and FFI

Two mechanisms exist and are both live:

### 4.1 Engine-native external predicates (`Decl … descr [external(), mode(...)]`)

Registered per evaluation from `VirtualStore.BuildExternalPredicates` (`internal/core/external_predicates.go:103-157`) and filtered by whether the loaded program declares the predicate as external (`kernel_eval.go:331-345`). The adapter `virtualExternalPredicate` (`external_predicates.go:29-85`) reconstructs the query atom from input constants, calls the legacy handler, and emits output positions. `ShouldPushdown` is false and `ShouldQuery` is always true (`external_predicates.go:37-43`).

| Predicate | Decl (file:line) | Mode | Go handler | Reads |
|---|---|---|---|---|
| `query_learned(Predicate, Args)` | `schemas_memory.mg:54` | `-,-` | `getQueryLearnedAtoms` (`virtual_store_predicates.go:752`) → `QueryLearned`/`QueryAllLearned` (:27,:59) | SQLite knowledge store (`v.localDB`) |
| `query_session(SessionID, Turn, Input)` | `schemas_memory.mg:58` | `+,-,-` | `getQuerySessionAtoms` (:808) → `QuerySession` (:294) | SQLite session history |
| `recall_similar(Query, TopK, Results)` | `schemas_memory.mg:62` | `+,-,-` | `getRecallSimilarAtoms` (:829) → `RecallSimilar` (:268) | vector search (sqlite-vec / embeddings) |
| `query_knowledge_graph(A, Rel, B)` | `schemas_memory.mg:66` | `+,-,-` | `getQueryKnowledgeGraphAtoms` (:855) → `QueryKnowledgeGraph` (:191) | SQLite knowledge-graph links |
| `query_graph(QueryType, Params, Result)` | `schemas_memory.mg:72` | `+,-,-` | `getQueryGraphAtoms` (`virtual_store_graph.go`, line **unverified**) | graph store |
| `query_strategic(Category, Content, Confidence)` | `schemas_memory.mg:76` | `-,-,-` | `getQueryStrategicAtoms` (:1109) → `QueryStrategicKnowledge` (:1062) | SQLite strategic knowledge |
| `query_activations(FactID, Score)` | `schemas_memory.mg:80` | `-,-` | `getQueryActivationsAtoms` (:880) → `QueryActivations` (:227) | activation store |
| `has_learned(Predicate)` | `schemas_memory.mg:84` | `-` | `getHasLearnedAtoms` (:777) → `HasLearned` (:326) | SQLite |
| `query_traces(ShardType, Limit, TraceID, Success, DurationMs)` | `schemas_tools.mg:320` | `+,-,-,-,-` | `getQueryTracesAtoms` (:899) → `QueryTraces` (:336) | SQLite shard traces |
| `query_trace_stats(ShardType, Succ, Fail, AvgDur)` | `schemas_tools.mg:325` | `+,-,-,-` | `getQueryTraceStatsAtoms` (:935) → `QueryTraceStats` (:385) | SQLite shard traces |
| `string_contains` | `schemas_safety.mg:30` (commented out) | — | `getStringContainsAtoms` (:962); registration commented out (`external_predicates.go:151-153`) | replaced by native `:string:contains` |

So the "12 virtual-predicate annotations" measured earlier = 10 live `external()` Decls + the commented `string_contains` + `query_graph` (registered in Go and declared in schema). Every external reads **SQLite or the vector index**; none reads CodeDOM, the world model, the filesystem, or an LLM — those are all pushed in as EDB facts by Go writers, not pulled by the kernel. The world model, CodeDOM and per-turn state enter through `AssertBatch` from Go producers (e.g. `internal/system/factory.go:1738`).

Consequence noted in `kernel_eval.go:201-216`: because externals are registered, `hasExternalPredicatesLocked()` returns true whenever a VirtualStore is attached, which **disables the differential path** in production even if the flag were on.

Direct query of an external (`kernel.Query("recall_similar(\"x\", 5, R)")`) is routed live through `queryExternalVirtualStore` (`kernel_query.go:378`), superseding any cached rows.

### 4.2 Action routing through the VirtualStore (the other "virtual" surface)

The kernel derives `next_action(...)`/`permitted(...)`; the VirtualStore executes (`internal/core/virtual_store_routing.go`, `virtual_store_actions.go`, `virtual_store_file_actions.go`, `virtual_store_codedom.go`, `virtual_store_tools.go`, `virtual_store_mcp_proxy.go`, `virtual_store_python.go`, `virtual_store_workflows.go`, `virtual_store_projectdoc.go`). These are not Mangle-visible predicates; they are the effect side. `kernel_virtual.go:8` attaches the store. The constitution's `permitted/…` derivation is the gate (`policy/constitution.mg`; `virtual_store_constitution.go`, `virtual_store_write_guard.go`, `virtual_store_interactive_gate.go`). Detailed per-action mapping is out of scope for this map; §7 covers the decisions that bypass it.

## 5. Misuse and hazard catalogue

Each item: the pattern, why it is wrong on this engine, and where the evidence is.

1. **Wildcard in a negated multi-arg literal does not exclude.** Canonical statement at `schemas_safety.mg:142` ("`!p(X, _)`, `!p(X, _, _)`, `!p(_)` and `!p(_, _)` all derive every…") and `working_set.mg:88-91` (project to arity 0 first). The live corpus now has **zero** non-comment instances (grep `^\s*[^#]*!\s*[a-z_]+\([^)]*\b_\b[^)]*\)` over `internal/core/defaults`, `internal/context` → 0 hits) and **17 comment-documented past instances**, each recording what the bad rule silently did: `campaign_rules.mg:729,872,1023`, `schemas_prompts.mg:104`, `browser_honeypot.mg:110`, `codedom_edit.mg:99`, `codedom_safety.mg:175`, `coder_workflow.mg:53`, `constitution.mg:37,508,531` (admin override never excluded; `!permitted(Action,_,_)` fired for every candidate; `!action_denied(_,_)` reported zero blocked actions), `intent_routing_rules.mg:528` (`bound_negation_test.go`), `prompt_northstar.mg:198`, `shards.mg:26`, `validation.mg:117,207,315`. Regression test named at `intent_routing_rules.mg:528`: `internal/core/bound_negation_test.go`.

2. **Go writer shape ≠ Decl shape (silent empty join).** `learned_preference/2`, `learned_fact/2`, `learned_constraint/2` are `bound [/string, /string]` (`schemas_memory.mg:93`) but the store returns `Args []any`; asserting the slice was rejected on every boot ("hydrate learnings incomplete after 249 facts … arg 1 declared /string, got []interface {}") until `learnedArgsValue` flattened it (`virtual_store_predicates.go:562-575`). Same class: `modified_function` (bare `Name` vs `<pkg>.<Name>`), `plan_edit` (file path vs element ref) — `internal/mangle/agents.md:12-45`. The only runtime guard is `checkBoundAgainstValue` for `/number` and `/string` (`kernel_undeclared.go:109`).

3. **`/name` vs `/string` on the same value in adjacent lines.** `turn_evidence` carries the verb as `MangleAtom` (`executor.go:2358`) because the rules join it to `/name`-typed `write_oriented_intent/1` (`coder_safety.mg:74-78`); two lines later `claimed_test_output` carries the same verb as `MangleString` (`executor.go:2373`). Both are deliberate today, but nothing enforces it: `/name` is unchecked at the boundary (`kernel_undeclared.go:103-108`), so a future writer that flips one will produce a rule that "silently never fire[s]" (`coder_safety.mg:78`).

4. **A quoted literal beginning with `/` is not the stored string.** `"/etc/passwd"` in a rule matched 0 rows against a stored `"/etc/passwd"`; write the join on a variable and seed the path as a fact (`internal/mangle/agents.md:61-96`). Absolute paths are everywhere in this system.

5. **A fractional float in a `/number` slot aborts the whole fixpoint.** The fork's comparison builtins are int64-only; `coerceAtomToDeclLocked` rejects such facts at assert (`kernel_fact_decl.go:27-45`). Observed: `dream_preference(Content, 0.85)` dropped silently while reporting success; "~4 aborts every 2 seconds for an entire session". Go now scales ratios to integer percent (`delegation_routing.go:86-89,247-249`; `types.PercentScale`).

6. **Undeclared predicate/arity is stored, not rejected.** `warnIfUndeclaredLocked` warns once per process and keeps the fact (`kernel_undeclared.go:52-72`). Arity is identity, so `state/1` and `state/3` are different predicates (`kernel_undeclared.go:74-79`). Only one symbol is legitimately multi-arity today (`task_complexity/1,/2`).

7. **Only `Bounds[0]` of a Decl is honoured** by both coercers (`kernel_fact_decl.go:24`, `engine.go:692-695`); overloaded Decls are unsupported.

8. **Starved predicates (declared, joined, never produced).** Baseline of **62** in `internal/core/defaults/testdata/starved_predicates.txt`, gated by `TestStarvedPredicateBudget` (`starved_predicate_test.go:185`) which fails on growth or on silent wiring. Examples on the list: `campaign_active`, `review_approved`, `review_complete`, `test_passed`, `system_shard`, `reviewer_task`, `tester_task`, `checkpoint_needed`, `coverage_metric`, `known_cause`, `symptom`, `target_is_complex`. Every rule that reads one of these derives nothing.

9. **Rules whose body joins facts that live in different shard kernels.** `.claude/skills/codenerd-dogfood/scripts/shard_join_audit.py` (ran in 0.85 s): "policy corpus: 2080 rules, 876 derived predicates, 102 program-EDB predicates, 282 owned predicates, 47 shared predicates; accepted residue: 4 split joins, 9 blind negations; OK beyond the residue". The residue is pinned in `internal/shards/testdata/accepted_seams.json`: three `permitted`/`final_action` override paths in `constitution.mg` ("architect's call"), two `quality_violation_detected` rules in `campaign_rules.mg` ("needs restructure"), and blind negations `coder_quality_mode(/normal) :- !in_campaign_context()` and `final_context_include(File) :- include_in_context(File), !exclude_from_context(File)` — each **duplicated verbatim in two files** (`coder_campaign.mg`/`coder_workflow.mg` and `coder_context.mg`/`coder_workflow.mg`), plus `permission_denied`/`action_denied` rules that "over-deny only".

10. **Rules that cannot fire on the shipped corpus.** `mandatory_superseded` is inert: "no atom under internal/prompt/atoms declares conflicts_with" (`jit_compiler.mg:188-193`). `string_contains` external is declared only in a comment (`schemas_safety.mg:30`) and unregistered (`external_predicates.go:151-153`). The 62 starved predicates above. `codedom_continuation.mg:22,26` read `shard_result(_, /tests_needed, …)` and `/review_needed`, while the only Go producer writes `/complete`, `/failed`, `/incomplete`, `/code_generated` (`process_continuation.go:173-184`); grep of `cmd/` and `internal/` (non-test) for `/tests_needed|/review_needed` → **0 hits**, so those two rules can never fire. `verb_def/4` is declared in the executive kernel (`schemas_intent.mg:160`) but its rows are only ever added to the perception taxonomy's own `mangle.Engine` (`internal/perception/taxonomy.go:114`) and `verb_def` is not in `defaultIntentFactPredicates()` (`internal/core/intent_defaults.go:27`), so in the executive kernel it is a Decl with no rows — invisible to the starved-predicate gate because Go names the string.

11. **Monotone evaluation + accumulating control facts.** `Engine.ReplaceControlFacts` exists because `working_control(/no, 0)` was never removed by the file-keyed replace and "a stop derived from a fact that no longer exists stayed derived" (`engine.go:560-571`). On the RealKernel the same hazard is handled by retract-before-assert discipline in Go (`delegation_routing.go:91-95`, `executor.go:2326-2328`) — a convention, not a mechanism.

12. **Persisted poison.** `sanitizeFactForNumericPredicates` rewrites priority atoms (`/high` → 80) and pointer-hex strings (`0x…` → 0) in `/number` slots of `agenda_item`, `prompt_atom`, `atom_priority` (`kernel_facts.go:1204-1305`): evidence that Go writers once emitted atoms and `%v`-formatted pointers into numeric slots and the rows are still in `.nerd/shards/*.db`.

13. **Predicate-name special cases inside the kernel.** `system_heartbeat` upsert (`kernel_facts.go:566`), the numeric sanitiser above, and a `jitPredicates` logging map in `Query` (`kernel_query.go:128-134`). Each is executive knowledge about a predicate encoded in Go rather than in the Decl.

14. **Two coercion regimes.** `mangle.Engine.convertValueToTypedTerm` auto-promotes identifier-like strings to names when the bound is unknown (`engine.go:790-800`); `types.Fact.ToAtom` does not. The differential path bypasses the first to stay bit-identical with the second (`kernel_eval.go:415-421`). A task-private `mangle.Engine` and the RealKernel can therefore store the same Go value as different constants.

15. **`Query` pattern parsing.** Free-variable patterns like `next_action(Var0)` "may be misparsed as constants" by `RealKernel.Query` (`cmd_query.go:224-226`), which is why `nerd why` strips to the bare predicate.

16. **Config ceilings that bind nothing** (§1.4).

17. **Every kernel carries every shard's policy.** `reviewer.mg` and `tester.mg` say they are loaded "by ReviewerShard/TesterShard kernel alongside base policy" but sit in `defaultCorePolicyModules`; the CortexKernel evaluates the whole 913-stratum program per shard store (shard_join_audit.py docstring, `internal/core/cortex_kernel.go` **unverified** line). Cost is in §9.

18. **Stale debug dumps** in 15 package directories (§1.7) and three "moved file" headers (§2.2).

## 6. The Piggyback protocol and the mangle_updates allowlist

The model may volunteer facts in a control packet field `mangle_updates`. Every consumer runs them through `core.FilterMangleUpdates(kernel, updates, policy)` (`internal/core/mangle_updates.go:58`):

1. cap at `MaxUpdates`;
2. reject anything containing `:-`, or starting with `decl`, `import`, `include` (`mangle_updates.go:86-108`);
3. `ParseFactString` (`kernel_query.go:536`);
4. `predicateAllowed`: hard-deny `turn_acceptance, turn_evidence, turn_executed, turn_done, turn_cost` even with a permissive policy ("host witnesses and conclusions, never model observations", `mangle_updates.go:145-150`), then the allowlist / prefixes;
5. `validatePredicateDeclaration`: the predicate must be declared in the live `programInfo` with matching arity (`mangle_updates.go:165-194`).

**Policies in force** (the allowlist is Go data, not a kernel fact):

| Caller | Policy | Max | How an accepted fact enters |
|---|---|---|---|
| `internal/session/executor.go:1702` (`processMangleUpdatesFromEnvelope`) | `ModelObservationPolicy()`: `missing_tool_for, observation, task_status, task_completed, diagnostic, failing_test, test_state, review_finding, modified, modified_function, checkpoint_verdict` (`mangle_updates.go:30-53`) | 100 | `AssertBatch` (`executor.go:1716-1719`); blocked atoms are fed back to the model via `ApplyConstitutionalOverride` (:1709) |
| `cmd/nerd/chat/process.go:1038` | `ModelObservationPolicy()` | 100 | per-fact `Assert`; blocks and rejections become user-visible warnings (:1039-1047) |
| `internal/shards/system/perception.go:283-291` | `ambiguity_flag, clarification_needed` only | 50 | `AssertBatch` (:296) |
| `internal/shards/system/planner.go:1071-1093` | `missing_tool_for, observation, task_status, task_completed, campaign_completed` **plus prefixes** `campaign_, phase_, task_, context_, plan_, replan_, build_, architectural_, suspicious_, eligible_` | 200 | `AssertBatch` (:1098) |

The comment at `mangle_updates.go:28-29` says the prompt atom `protocol/piggyback/mangle_updates` (`internal/prompt/atoms/protocol/piggyback.yaml`, exemplars in `exemplar/piggyback_exemplars.yaml`) teaches exactly the observation set, and the two must be edited together — by hand.

Observations for the designer: (i) `task_status`/`task_completed` are model-assertable in the observation policy, so a model can still stamp its own task as complete even though it cannot assert `turn_done`; whether any rule treats that as completion evidence is a §7 question. (ii) The planner prefix list makes any `campaign_*`/`phase_*`/`task_*`/`build_*` predicate that is declared writable by the planner model, including ones the kernel is supposed to derive (e.g. `campaign_active` is on the starved list, so a planner-asserted `campaign_active(...)` would be the only producer). (iii) Perception may assert `clarification_needed`, which feeds `next_action(/interrogative_mode)` rules in `clarification.mg` (**unverified** which rule reads it).

## 7. Executive decisions made in Go that the vision says should be derived

Rows are ordered by how directly they override a kernel decision. "Facts needed" names Decls that exist or would have to exist; "Rule sketch" is what the derivation would look like.

| # | Where | Decision made in Go | Inputs it reads | Kernel state it ignores or pre-empts | Facts / Decls needed | Rule sketch |
|---|---|---|---|---|---|---|
| 1 | `cmd/nerd/chat/process.go:369` → `process_dream_delegation.go:23-44` `shouldAutoClarify` | Run the clarifier shard when the input "looks like a campaign" (substrings `campaign, plan, roadmap, project, initiative, blueprint, feature`) or the intent lacks Target/Constraint or verb is `/generate`/`/scaffold`, and category is `/mutation` or `/instruction` | raw input, `intent.Target/Constraint/Verb/Category`, `lastClarifyInput` | Runs for **every** non-direct route, including a kernel-derived `route_decision(/delegate, Shard)` or `/multi_step` (`process.go:353-369`): Go's substring test can pre-empt the kernel's lane. The kernel already has a clarify lane (`routing_arbitration.mg:107-112`) but only for low-confidence mutations. | `user_intent/5` (exists); a Go-extracted `campaign_signal(/keyword)` like `multi_step_signal` (extraction stays in Go per guardrail); `clarify_reason(Reason)` Decl | `route_decision(/clarify, /none) :- user_intent(/current_intent, Cat, Verb, Target, _), mutation_like(Cat), (campaign_signal(_) ; Target == "" ; generate_verb(Verb)), !wants_direct_answer().` with lane precedence expressed as `final_route/2` (row 14) |
| 2 | `process.go:392-408` `shouldClarifyFromKernel` then `shouldClarifyIntent` | Ask the kernel (`clarification_question`, `awaiting_clarification`, `clarification_option`) and, if it has no question, fall back to a Go heuristic ("actionable intent with low confidence or missing target") | kernel facts; `intent.Confidence/Target` | The Go fallback is a second opinion the routing file forbids ("Go … adds NO additional routing opinions", `routing_arbitration.mg:4-6`) | `delegation_candidate/3` already carries confidence | fold into the `/clarify` lane above; delete `shouldClarifyIntent` (body **unverified**) |
| 3 | `cmd/nerd/chat/delegation.go:311-375` `formatShardTask` (+ `formatShardTaskWithContext` :65) | Rewrite the user's request into a template: `/fix` → `"fix issue in <target>"` (constraint dropped), `/review` → `"review file:<t>"`/`"review all"`, `/test` → `run_tests` if target contains "run"; discover files in Go when target is broad | verb, target, constraint, workspace, `discoverFiles` | The brief the shard receives is not the user's words nor kernel facts; `user_intent/5` and focus facts already exist and could be carried to the shard as facts | `delegated_task(TaskID, Shard, Verb, Target, Constraint)` Decl; `task_scope_file(TaskID, Path)` derived from `file_topology` | `task_scope_file(T, P) :- delegated_task(T, _, _, "codebase", C), file_topology(P, _, Lang, _, _), language_for_constraint(C, Lang).` and pass the original input as `task_text` |
| 4 | `cmd/nerd/chat/process_continuation.go:167-222` `injectShardResultFacts` | Decide a shard's outcome from substrings: `TODO`/`FIXME` → `/incomplete`; coder output without the word "test" → `/code_generated` (+ assert `pending_test`); reviewer output containing "issue" → assert `pending_review`; else `/complete` or `/failed` | `result` text, `err`, `shardType` | Consumers `codedom_continuation.mg:8-26` derive next actions from these atoms; the status is a guess about text, not evidence. Rules at :22/:26 expect `/tests_needed`/`/review_needed`, which no Go writer emits (grep: 0 hits) — dead rules | `shard_output(TaskID, Shard, WriteCount, TestFilesWritten, FindingCount, ErrorFlag)` from measured tool traces (the executor already has `SuccessfulWriteTools`, `review_finding` facts) | `shard_status(T, /code_generated) :- shard_output(T, /coder, W, 0, _, /false), W > 0.` `pending_review(T, D) :- shard_output(T, /reviewer, _, _, N, _), N > 0, task_desc(T, D).` |
| 5 | `internal/session/executor.go:1093-1095` | If any tool errored **and** the final response is empty, mark the turn failed ("conservative heuristic") | `toolErrs`, `result.Response` | Runs **before** the kernel verdict; `checkHollowSuccess` is only consulted when `result.Error == nil` (:1106-1110), so this Go rule pre-empts `hollow_success`/`turn_done` | `turn_evidence/6` (exists, `coder_safety.mg:79`) lacks an error count and an empty-response flag: add `turn_tool_errors(Verb, N)` and `turn_response_empty(Verb)` | `hollow_success("tools errored and the model said nothing") :- turn_evidence(Verb, _, _, _, _, /false), turn_tool_errors(Verb, N), N > 0, turn_response_empty(Verb).` |
| 6 | `executor.go:2432-2557` `consumeHollowSuccessVerdict`; `executor_tools.go:1923-1977` `checkHollowSuccess` | The kernel derives `hollow_success(Reason)` (`coder_safety.mg:92-107`) — this is the **target pattern** — but Go then (a) switches on the exact reason string to pick the error, (b) re-implements all four rules imperatively as a fallback (:2509-2555), and (c) decides read-only intents are never failed (`requiresTools`, `executor_tools.go:1936,1959-1968`) | `hollow_success`, `missing_test_for`, `unverified_test_claim`, `intentRequiresToolCall`, `writeOrientedIntent` | The read-only exemption is a verdict rule living in Go | `turn_failed(Verb)` Decl; `read_only_verb(Verb)` (from `intent_requires_tool_call`) | `turn_failed(Verb) :- hollow_success(_), turn_evidence(Verb,…), !read_only_verb(Verb).` and drop the imperative fallback (MockKernel tests are the stated reason it exists, :2505-2508) |
| 7 | `internal/session/change_evidence.go:62-107` `appendEvidenceReport` (the verdict printer) | Downgrade acceptance to `unverified` if the workspace snapshot changed after verification; convert an unverified acceptance into `result.Error`; compose the "Wrote N file(s) … Evidence: <stage>" trailer | `Acceptance`, `evidence.Snapshot`, `WrittenPaths`, `ChangeStage`, `ChecksSnapshot` | `turn_acceptance/3` is asserted only when already verified (`executor.go:2334`), so the kernel never sees an *unverified* acceptance or the snapshot drift | `acceptance_state(Verb, Status, Snapshot)`, `workspace_snapshot(Current)`, `change_stage(Verb, Stage)` | `turn_outcome(Verb, /unverified) :- acceptance_state(Verb, /verified, S), workspace_snapshot(C), S != C.` `turn_done(Verb) :- turn_executed(Verb), turn_outcome(Verb, /verified).` |
| 8 | `internal/session/tool_budget_controller.go:192-238` `maybeExtend`; `:243-270` `isFocusedVerificationCall`; `:276-302` `repeatedTailCycle` | Grant extra LLM→tool rounds only if adaptive, under hard limit, no period-1..3 repeated trace, novel progress since last boundary, and (for write intents) a write or a verification since last boundary; which tool names count as verification is a Go list | `ExecutorConfig` limits, trace signatures, `progress/writes/verifiesSinceExtension` | `internal/context/working_set.mg:53-110` already derives `working_stop(/read_only_stall)`, `working_finalize(/verify_after_write)`, `working_nudge/1`, `working_regime(/commit)` from `working_progress/5` — but in a **private scope**, and the extension grant, loop detection and verification-tool list stay in Go (`tool_budget_controller.go:333`: "It only counts; working_set.mg decides what the counts mean" — true for stop/nudge, not for extension) | `working_cycle(Period)` (Go-detected), `working_extension(Granted, Cap)`, `verification_tool(Name)` Decl seeded from the Go list, `working_novel(N)` | `working_extend(Rounds) :- working_progress(I, R, W, SW, SV), working_extension(G, Cap), G < Cap, !working_cycle(_), working_novel(N), N > 0, (I = /read ; W > 0 ; SV = 0), working_extension_size(Rounds).` |
| 9 | `internal/session/work_steps.go:157-211` `planTurnSteps`; `:217` `isEditStep`; `:343-400` `runPlannedSteps` | Whether a turn is "planned" (world present, `ProgressDrivenTools`, write-oriented verb, native tool client, not piggyback tools, a write tool allowed); ask the LLM for steps; drop plans < 2 steps; a step with no write is retried once under `commitRegime` | executor config, client capabilities, `cfg.AllowedTools`, LLM plan text | The step plan never enters the kernel; the retry rule is Go | `plan_step(TaskID, N, File, Change)` (planner may already assert `plan_*` via the planner allowlist, `planner.go:1084`); `step_edited(TaskID, N)`; `turn_shape(/planned)` | `turn_shape(/planned) :- write_oriented_intent(V), user_intent(_,_,V,_,_), tool_allowed(/write_file), plan_step_count(N), N >= 2.` `step_retry(T, N) :- plan_step(T, N, _, _), step_done(T, N), !step_edited(T, N).` |
| 10 | `cmd/nerd/chat/delegation_routing.go:205-227` `resolveShardTypeForIntent`; `internal/perception/transducer.go:481-489` `GetShardTypeForVerb`; `process_continuation.go:226-241` `isMutationOperation` | Which shard: verb→shard table from `GetVerbCorpus()` (a Go slice, "siloed in the perception taxonomy engine, not this kernel", `delegation.mg:331-332`), else the LLM's `shard=` hint, else substrings in the target (`codebase, project, architecture, repository, entire, whole`) → `researcher` at confidence ≥ 0.7. Separately, which shards mutate is a Go map | verb, `intent.Ambiguity`, `intent.Target`, `Confidence` | `should_delegate/1` (`delegation.mg:335-338`) gates the decision but is handed the shard by Go. The executive kernel **declares** `verb_def(Verb, Category, Shard, Priority)` (`schemas_intent.mg:160`) but its rows live only in the perception taxonomy engine (`internal/perception/taxonomy.go:114`); `defaultIntentFactPredicates()` (`intent_defaults.go:27`) admits `shard_affinity_action/3`, `shard_affinity_domain/3`, `best_shard/2` (Decls at `schemas_intent.mg:222-229`) but not `verb_def` | `verb_def/4` rows loaded into the executive kernel (add to `defaultIntentFactPredicates`; the corpus already exists), `shard_mutates(Shard)`, `target_scope(/whole_repo)` Go-extracted signal | `delegation_candidate(/current_intent, S, C) :- user_intent(/current_intent,_,V,_,_), verb_shard(V, S), perception_confidence(C).` `delegation_candidate(…, /researcher, C) :- user_intent(…, V, …), explore_verb(V), target_scope(/whole_repo), C >= 70.` |
| 11 | `cmd/nerd/chat/process_seed.go:147-162` `runClarifierShard`; `commands_handlers_analysis.go:59` | The requirements interrogator fires only from `shouldAutoClarify`/fallback clarification (rows 1–2) or an explicit command | as rows 1–2 | The kernel derives `next_action(/interrogative_mode)` in nine rules (`clarification.mg:14,23,32,37,93,102,130`, `learning.mg:17`, `system_ooda.mg:75`) and lists it as `safe_action` (`constitution.mg:233`). That derivation **is** consumed — but only on the system-shard path: `ExecutivePolicyShard.hydrateActionFromIntent` (`internal/shards/system/executive_intent.go:252-261`) loads the clarification payload, the tactile router maps it to `session_control` (`router.go:947`), and the VirtualStore runs `handleInterrogative` (`virtual_store_workflows.go:175`). The chat loop (`process.go:369-408`) never consults it and fires the clarifier from Go heuristics instead; it only retracts `interrogative_mode` afterwards (`cmd/nerd/chat/model_handlers.go:286`). Two control paths, one decision. | already exists | make `runClarifierShard` the effect of `next_action(/interrogative_mode)` in the chat loop too, and delete the Go triggers |
| 12 | `internal/prompt/compiler.go:1099,1148` (kernel-injected atoms forced `IsMandatory = true`), `:1000-1016` (size caps as Go constants), YAML `is_mandatory` | What the JIT treats as mandatory: Mangle derives `mandatory_atom` from skeleton categories `identity/protocol/safety/methodology` + shard tag, from `prompt_atom(…, /true)`, and from `is_mandatory/1` (`jit_selection.mg:186-211`), then `mandatory_selection` minus `blocked_by_context`/`mandatory_superseded` (`jit_compiler.mg:196-199`). The **flag itself** is data: 336 of 916 flagged atoms in `internal/prompt/atoms/**/*.yaml` are `is_mandatory: true` (36.7%), plus Go forces it for `kernel/context/*` and `kernel/knowledge/*` dynamic atoms; budget fitting (`budget.go`, "Mandatory atom %s rejected") is Go | YAML flags, `injectable_context`, `specialist_knowledge` rows, `TokenBudget` | Selection is derived (good); mandatory-ness and the budget cliff are not | `atom_budget(Tokens)`, `atom_tokens(ID, N)` (exists as `prompt_atom/5` arg), `atom_must_fit(ID)` | `dropped_atom(ID) :- selected_atom(ID), !mandatory_atom(ID), budget_overflow(...)` via `fn:group_by` sum of `atom_tokens` — one of the few places aggregation would pay |
| 13 | `internal/session/executor.go:532-566` `intentRequiresReasoningModel` | Which LLM slot serves the turn — **derived** from `intent_requires_reasoning_model/1` (`delegation.mg`); Go only caches, short-circuits `isConsultIntentVerb` (:540) and validates the atom | verb | reference implementation of the target pattern; the consult short-circuit is a small Go opinion | `consult_verb(V)` Decl | `intent_requires_reasoning_model(V) :- reasoning_intensive_action(V), !consult_verb(V).` |
| 14 | `delegation_routing.go:152-164` `decideRoute` | Lane precedence `respond_directly > multi_step > delegate > clarify` when several `route_decision` rows derive | `route_decision/2` rows | The precedence is documented in `routing_arbitration.mg:20-24` but applied in Go | none new | `final_route(/multi_step, /none) :- route_decision(/multi_step, _), !route_decision(/respond_directly, _).` etc.; Go reads exactly one row |
| 15 | `delegation_routing.go:104` `multiStepSignals`; `delegation.mg:340-360` | Signal **extraction** (keyword/regex/verb-count) stays in Go by design; the combination decision is derived (`is_multi_step/0`) | input text | accepted boundary per the repo guardrail ("Do not use Mangle for fuzzy matching") | — | — |
| 16 | `internal/core/kernel_facts.go:566` (`system_heartbeat`), `:1204-1229` (numeric sanitiser), `kernel_query.go:128-134` | Kernel-level exceptions keyed on predicate names | predicate name | Executive knowledge about predicates in Go | `ephemeral_upsert_predicate(P)`, `priority_atom_value(Atom, N)` | `IsEphemeral` already exists as a Go table (`fact_categories.go:131`); the same list could be a Decl-level annotation |
| 17 | `internal/core/mangle_updates.go:30-53,145-150`; `planner.go:1071-1091`; `perception.go:283-289` | Which predicates a model may assert (three hand-maintained Go allowlists + one prompt atom) | predicate name, caller | Not queryable; not explainable by `nerd why` | `model_may_assert(Surface, Pred)` facts | `mangle_update_allowed(S, P) :- model_may_assert(S, P), !host_witness(P).` and have `FilterMangleUpdates` query it |

Cross-cutting: rows 1, 2, 5 and 6 are the ones where Go overrides a decision the kernel has *already derived*; rows 3, 4, 7, 8, 9 are decisions the kernel never sees because the facts that would let it decide are never asserted; rows 10–17 are lookup tables and exceptions that live in Go for want of a Decl.

## 8. Provenance and explanation today

Three distinct mechanisms; the CLI and the TUI use different ones:

1. **Engine provenance** (`internal/core/kernel_provenance.go`): `EnableProvenance()` installs a `provenance.MemoryRecorder` from the fork's `provenance` package; the next `evaluate()` records every rule firing / transform emission (`kernel_eval.go:321-325`), resetting the buffer per pass. `Explain(goal, opts)` (`kernel_provenance.go:72`) parses a ground atom and calls `provenance.BuildFromRecording` → proof trees. **Off by default.** Wired only to the TUI `/explain <goal>` command (`cmd/nerd/chat/commands_handlers_misc.go:98-125`), which enables it on first use and forces `Evaluate()`. When provenance is on, the differential path is disabled (`kernel_eval.go:216`). This is the only mechanism that reports *which rule* fired with *which bindings*.

2. **`nerd why [predicate]`** (`cmd/nerd/cmd_query.go:168-244`) does **not** use the provenance package. It boots or reuses the cortex, builds a free-variable query, strips it to the bare predicate (`whyTracePredicate`, :247), and calls `RealKernel.TraceQuery` (`internal/core/trace.go:16-49`). That tracer is heuristic: `classifyFact` decides EDB vs IDB from `programInfo` (`trace.go:84-112`), names the rule from `rule_metadata/2` facts (**12** such facts exist in the corpus; `ruleNameFor`, :131), and `findPremises` (:144) reconstructs premises only for **four** hard-coded rule names (`transitive_impact`, `permission_gate`, `focus_threshold`, `strategy_selector`). For every other derived predicate the tree is one node deep. On failure it falls back to a *hollow* `mangle.Engine` loaded with schemas+policy+learned and the kernel's base facts, traced by `mangle.NewProofTreeTracer` (`cmd_query.go:289-311`). Aliases `blocked/block → action_blocked`, `forbidden/denied → forbidden` (:158-163); default predicate `next_action`.

3. **Glass-box events**: the chat loop emits `transparency.GlassBoxEvent` summaries such as "Route: delegate (verb=…)" naming `route_decision/2` (`process.go:357-365`) — a narration of the decision, not a derivation.

What a user can ask today: `/explain <ground atom>` in the TUI for a real proof tree of anything derived in the last fixpoint; `nerd why <predicate>` for a fact listing with rule names for 12 predicates and premises for 4. What they cannot ask: why any §7 Go decision was made (no trace), why a fact was *not* derived (no negative provenance; starved predicates and blind negations are found by static scripts, §5 items 8–9), or why a fact from a previous fixpoint held (the recorder is reset each pass, `kernel_provenance.go:10-13`). The chat model also calls `TraceQuery` at `cmd/nerd/chat/model_lifecycle.go:233,244` (purpose **unverified**).

## 9. Performance today

Sources (read-only): `.nerd/logs/20260918_035947.892298800_054828_000001_9eac1a_kernel.log` (13.0 MB) and `…_performance.log` (651 KB), one TUI session 2026-09-17 23:59:47 → 00:14; `.nerd/meter/receipts.jsonl` (2.5 MB, 3,084 LLM-call receipts with estimated vs actual tokens and admission decisions) and `atom-selections.jsonl` (599 KB, 155 JIT selections). Commands: `python C:/Temp/mangle-study/evalstats.py [kernel.log]`; `Select-String -Path <kernel.log> -Pattern 'rebuildProgram: (total program size|parsed|analysis complete)|Mangle files loaded'`.

| Metric | Value | Evidence |
|---|---|---|
| Program text | schemas 327,131 B + policy 569,969 B + learned 1,452 B = 898,609 B | `Mangle files loaded` 23:59:49; `rebuildProgram: total program size` 23:59:54 |
| Clauses parsed / predicates declared | 3,453 / 1,845 | `rebuildProgram: parsed … / analysis complete` |
| Strata | **913** | every `fixpoint reached` line |
| Parse + analyse + stratify at boot | ~105 ms (23:59:54.649 → .754) | timestamps of the three `rebuildProgram` lines |
| Fixpoint evaluations in session | 127 (179 lazy triggers; singleflight coalesced the rest) | counts |
| evalTime distribution | min 5.5 ms, median 75.5 ms, p90 1,868 ms, max 2,337 ms; **sum 106.4 s eval / 112.7 s wall** in a ~15-minute session | `evalstats.py` |
| EDB size at evaluation | 2 evaluations at ~3.5k facts (boot, before world ingestion), 1 at 41,630, 66 in 50k–60k, 58 in 60k–70k; 117 distinct sizes (the EDB grows on nearly every evaluation) | `populating store with N EDB facts` |
| Evaluations caused by JIT compile scopes | **19** `Kernel cloned (facts=52767, policy=569969 bytes)` lines, each followed by `Evaluate: forcing re-evaluation of all rules` and a 52,771-fact fixpoint (1.6–2.3 s). `NewCompilationScope` snapshots the live kernel with `live.Clone()` for every prompt compilation (`internal/system/factory_adapters.go:79-101`; `Clone` at `kernel_eval.go:694`) | log context at 00:07:56 / 00:07:58 |
| Slow-eval warnings | `kernel.evaluate.fixpoint` logged at `warn` above `threshold_ms:500`; 396 `evaluate` entries in performance.log | performance.log |
| Differential path | never taken (flag off; externals registered; EDB > 10k) | §1.3 |

Reading: roughly one eighth of the session's wall clock is spent re-deriving 913 strata from 50–70k atoms into a fresh store. The distribution is bimodal (median 75 ms vs p90 1.9 s): the expensive tail is dominated by **JIT compilation scopes** — every prompt compile clones the ~52k-fact live kernel and forces a full fixpoint on the clone (19 times in this session, ~2 s each ≈ 40 s of the 106 s), while the live kernel's own lazy evaluations at 60–70k facts make up the rest. The clone exists so selector facts never touch the executive kernel (`factory_adapters.go:79-81`); the cost is a full re-derivation of every stratum, including the 900+ that have nothing to do with atom selection. Benchmarks in tree, not run here: `internal/core/kernel_init_bench_test.go`, `kernel_eval_large_test.go`. The 2026-09-05 measurement in `kernel_types.go:139-144` (91 s for a one-fact delta on 48k facts) is why the differential path is demoted on large stores.

## Verification (adversarial, 2026-09-18)

### Status

- last updated: 2026-09-18 (in progress)
- checked: 0
- refuted: 0

| # | section | claim (short) | cited location | verdict | evidence | note |
|---|---|---|---|---|---|---|
