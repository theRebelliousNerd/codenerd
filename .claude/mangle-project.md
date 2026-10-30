# Mangle in codeNERD
Engine: codeberg.org/TauCeti/mangle-go v0.7.0 (go.mod) | Checked: 2026-10-28

codeNERD is the premier Mangle showcase (Mangle's author has a fork).
Shipped Mangle-only features here become the public examples.

## Programs
- **Action kernel**: `internal/core/defaults/schemas*.mg`, then `policy/*.mg`,
  then learned rules. All three are concatenated and parsed on every
  `rebuildProgram` (`internal/core/kernel_eval.go`). Order is part of the
  trust model, and a parse or analysis error anywhere is a boot failure
  everywhere. The embed set `coreLogic` is 172 files.
- **Orientation**: `internal/orient/*.mg` (`//go:embed *.mg`, `internal/orient/engine.go`).
  A separate engine and program.
- **Context**: `internal/context/{working_set,holographic_render,working_retention}.mg`
  are embedded per package. They read `config_param`, which the kernel
  program declares, so they analyze only together with it.
- **Prompt evolution**: `schemas_evolution.mg` and
  `policy/prompt_evolution.mg` are also embedded by
  `internal/core/defaults/evolution_policy.go` for a group-local adaptation
  kernel. Go measures evidence ranks and used logical screening calls; rules
  derive cohort membership, screen allowance and generation admission. The
  shared corpus loads the same schema/rules; all predicates are homed in the
  prompt shard. Lab and production-path test execution are pending under the
  author-only task restriction; no new engine behavior has been verified.
- **General adapter**: `internal/mangle/engine.go`. **Synth/transpiler**:
  `internal/mangle/{synth,transpiler}`.

## Facts in and decisions out
- Go measures, asserts and executes; Mangle judges permission, routing,
  freshness, budgets and completion. A policy `if` in Go is a bug.
- EDB enters as `core.Fact{Predicate, Args}`. Names come from
  `types.MangleAtom`, and `types.Fact` treats name-looking strings as names.
  `mangle.Fact` coerces values to the Decl's type. Build the declared shape
  at the producer; `sanitizeFactForNumericPredicates` is no excuse.
- Config values enter as `config_param` facts. A Go constant for a policy
  threshold is a bug.
- Wrong arity is a different predicate. `validateAgainstDeclLocked` warns
  instead of rejecting because legacy producers exist. Don't add to that
  count (the budget tests in `internal/core`).

## Externals
- About a dozen VirtualStore predicates are registered in
  `internal/core/external_predicates.go` (`BuildExternalPredicates`/`mkPred`):
  `query_learned/2`, `query_session/3`, `recall_similar/3`,
  `query_knowledge_graph/3`, `query_graph/3`, `query_strategic/3`,
  `query_activations/2` and others. `mangle-cli boundary`: all 10 are wired
  with agreeing modes, and none is negated (2026-10-28).

## Analysis and evaluation
- Analysis uses `AnalyzeOneUnit`, so **bounds are unchecked** for the corpus.
  Generated rules get `ErrorForBoundsMismatch` via `checkCandidateBounds`
  (`kernel_rule_bounds.go`).
- Store: `SimpleInMemoryStore`, which has `Remove`, so lattices work. No
  temporal store. Fact limit `WithCreatedFactLimit`. Cone evaluation
  (`kernel_eval_cone.go`) must stay identical to the full fixpoint.
- Strata come from `mangle.Stratify` (`internal/mangle/strata_layers.go`):
  1,315 components layered into 16 (the engine's per-call cost is
  strata x predicates). `mangle.Engine` evaluates over
  its store directly under `e.mu` (lattices work there since 2026-10-29).
- Cost: measure on the real world EDB (`LoadAllWorldFacts("fast")` from a
  copy of `.nerd/knowledge.db`, 345k facts; dependency_link is file level,
  162k rows) per component with fine strata; boot-EDB numbers hide joins.
- Proofs: `kernel_explanation.go` (`provenance.Explain`). Recorder mode is
  needed for `do`/`let` evidence; don't put it on a hot path unmeasured.
- Retraction rebuilds from a fresh EDB. Never reuse derived facts.

## Layers and conventions
- One Decl per predicate, in one `schemas_*.mg`. Rules live in `policy/` or
  topic files.
- LLM-proposed rules go through `internal/mangle/feedback` (`HotLoadRule`)
  plus the bounds gate.
- Prompt atoms that teach a predicate change in the same commit as it,
  mirrors included.
- No prose or taxonomies in `.mg`. Similarity belongs to embeddings or Jev
  behind an external.

## Commands
```bash
CGO_CFLAGS="-I$PWD/sqlite_headers" go test -tags sqlite_vec -count=1 ./internal/mangle/... ./internal/core/
CGO_CFLAGS="-I$PWD/sqlite_headers" go build -tags sqlite_vec -o nerd ./cmd/nerd   # after any .mg change: embedded
./nerd check-mangle internal/core/defaults/policy/<file>.mg
mangle-cli review --root .        # whole repo, engine-backed, about 6 s
mangle-cli boundary --root .      # where .mg meets Go (profile codenerd)
```
Before calling something done, run the whole owning package and one
real-kernel boot test.

## Known state (mangle-cli 1.3.0, 2026-10-30)
- Error review: 26 classified tool false positives remain: 22 action-routing
  envelopes, one visualization fixture (B004), valid task_complexity/1,2
  overloading (E046), and two context embeds checked without their host's
  schemas (ENGINE). Cases and exact sites: `_mangle_lab/REVIEW_FALSE_POSITIVES.md`.
- E071/E072 and B007 findings are repaired at the contract or producer.
  Existing strict per-clause bounds analysis still tracks disagreements;
  whole-corpus strict analysis is not yet enabled.
- Prompt conflict ties compare measured atom_ordinal/2 integers, supplied
  across both selection phases, homed in the prompt shard and cleared per
  compile. Mangle still decides the winner. No prompt atom content changed.
- Activation uses `bound [/string, /number] bound [/name, /number]` alternatives;
  the parsed `fn:Union(/string, /name)` passes strict analysis but its runtime
  type handle rejects both primitives at Go assertion (`kernel_fact_decl.go`).
  Identity stays typed; context_atom and context_priority render it as text.
- New behavioral tests are authored; their owning-package and sharded-boot
  execution is pending. Interpreter probes exercise the pinned v0.7.0 engine.

## Wired chains (read before deleting a "dead" family)
- Static findings (`reviewer.mg`): the world scan measures per-file Go
  `detected_security_flow`, `cyclomatic_complexity`, `nesting_depth` and
  `file_line_count` (`internal/world/review_metrics.go`); rules derive
  `raw_finding` -> `active_finding` (suppressed by rules the user keeps
  rejecting) -> `file_finding_risk` (summed rule weights, `group_by`) ->
  `block_commit` for modified files. Test:
  `internal/system/static_findings_boot_test.go`.
- Repository facts (`world/repo_facts.go`): `git_history`, `churn_rate`,
  `current_user`; age is judged in policy against `current_time`.
- Plan intelligence (`policy/intelligence.mg`): joins the world model with
  the plan's `task_artifact` rows in the world shard (copies in the manifest);
  `validation_error(/needs_tests_first)` feeds plan refinement,
  `intelligence_plan_{risk,complexity,advisory}` is read by
  `campaign.ReadPlanIntelligence`, and `context_relevant` ranks dangerous plan
  files. Chain rules must start from a world-owned premise
  (`intelligence_campaign_path`) or they fire partially in the campaign shard.
  Test: `internal/system/plan_intelligence_boot_test.go`.
- Campaign management (`campaign_rules.mg`, `campaign/orchestrator_policy_signals.go`): the pager's use of its budget -> `campaign_prefetch_limit` (integer cross-multiplication, thresholds are facts); measured placeholders, credentials, dropped errors and missing tests -> `remediation_task_needed` -> fix tasks; progress -> `milestone_pending` -> events. Tests: `internal/system/campaign_policy_boot_test.go`, `internal/campaign/orchestrator_policy_signals_test.go`.
- Coder policy (`coder_*.mg`): the coder's inputs are derived (`coder_world.mg`: coder_task/coder_target from `user_intent` and `pending_edit`, dependent_count from the import edges) and read by three consumers: `context_relevant` (coder_guidance.mg), `injectable_context("coder", ...)` rows the JIT compiler renders, and `coder_edit_barred` (coder_edit_bar.mg) which `permitted/3` negates. Test: `internal/system/coder_policy_boot_test.go`. The constitution declares `coder_edit_barred` itself because per-agent kernels load it without the coder files.
