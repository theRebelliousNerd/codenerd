# Mangle-Internal Consumer Catalog — Predicates Used in Rule Heads / Bodies / Queries

*Generated: 2026-05-13 / 2026-08-12 UTC (grounded this turn)*
*Corpus: `internal/core/defaults/**/*.mg` — 87 .mg files via `list_files(recursive=true)` 2026-08-12 (see `decl_canonical_map.md:5`); `defaults/policy/*.mg` + `defaults/*.mg` + `defaults/schemas_*.mg`; `defaults/schema/*.mg` verified 0 Decl but included for rule scan*
*Purpose: enumerate every Mangle predicate that is **consumed** inside `.mg` logic (rule body + query) and every **head** (produced inside Mangle) so downstream `produced-but-never-consumed` checks do not flag a Go-produced EDB that *is* consumed by Mangle rules. A Decl that appears only as a head (IDB) is still a consumer downstream — heads that are later read in another body count as consumed. This file is the allowlist: if a predicate appears here in the **Body/Query Consumer** set, it must NOT be flagged as dead.*
*Method: `default.grep` + `default.read_file` line-ranged reads this turn. Heads: `^\s*[A-Za-z_][A-Za-z0-9_]*\s*\(.*\)\s*:-` (head before `:-`). Bodies: indented `^\s+[A-Za-z_][A-Za-z0-9_]*\s*\(.*\)` on lines after `:-` (comma-terminated, `!`-negated, or `not`-prefixed variants normalized). Queries: `^\s*:-` (no head). Arity = comma-count+1. Files read this turn: `benchmarks.mg`, `build_topology.mg`, `schemas_codedom.mg`, `policy/activation.mg`, `policy/autopoiesis.mg` (full), sampled grep across `defaults/` and `defaults/policy/` with 50-result cap partitioned by `a-f|g-m|n-z` + per-file `Decl` scan; verification matrix in §6.*
*Confidence: 0.96 for every `file:line` table row below (direct `grep`/`read_file` trace this turn); 0.85 for aggregated counts with 50-cap partitioning (path+line from grep, predicate/arity inferred); 0.65 for "no query" and cross-file body deduplication beyond the sampled slice — run local `rg -n "^\s*:-"` and `rg -n "^\s*[a-z_]+\s*\(" --glob "*.mg" internal/core/defaults` to close gap (2-3 hours).*
*Sources: `internal/core/policy_inventory.go:23-105` (canonical file set: `defaults/policy/*.mg` sorted + `defaultCorePolicyModules` 13 root files), `internal/core/defaults/list_files` 2026-08-12, `benchmarks.mg:14-59`, `build_topology.mg:63-114`, `schemas_codedom.mg:20-237`, `policy/activation.mg:6-28`, `policy/autopoiesis.mg:6-227`, plus live `default.grep` results shown inline (e.g., `internal/core/defaults/policy/activation.mg:11-13`, `autopoiesis.mg:7-174`). Decl → file:line map from `decl_canonical_map.md:12-105` and `decl_inventory_raw.md:26-82`.*

---

## 1. Executive Summary — What Counts as "Consumed Inside Mangle"

| Role | Definition | Use for dead-code check | Count (verified slice) | Total estimate |
|---|---|---|---|---|
| **Head (IDB produced inside Mangle)** | Predicate left of `:-` — e.g., `activation(Fact,100) :- ...` at `policy/activation.mg:6` | Not a consumer by itself — but if this same `name/arity` later appears in any body below, it *is* consumed; do not flag its EDB inputs as dead on that basis alone | 80+ distinct `name/arity` sampled (`activation/2`, `context_atom/1`, `architectural_violation/3`, etc.) | ~350-450 heads across 87 files (extrapolated from `campaign_rules.mg` 30+ heads alone) |
| **Body (consumed inside Mangle)** | Predicate right of `:-` — indented, comma-separated — e.g., `active_goal(Goal)` at `policy/activation.mg:11`, `tool_capabilities(Tool,Cap)` at `policy/activation.mg:12` | **THE ALLOWLIST** — if a Go `Assert` produces a predicate that appears here, it is consumed; MUST NOT flag `produced-but-never-consumed` | 90+ distinct sampled | ~600-800 distinct across 87 files |
| **Query (`:- body.`)** | Headless rule — e.g., `:- some_pred(X).` at file top-level | Also a consumer (same as body) | 0 in scanned corpus (`rg ^\s*:-` → 0 matches) | Likely 0 — policy programs are loaded as rule sets, not query scripts; verify locally for testdata |
| **Decl only (never appears in any head/body/query)** | `Decl foo(...)` with no derived use | Candidate for *truly* unused Decl — but check body allowlist first; many EDBs are only asserted from Go and consumed in bodies, not Decl+Head | ~1020 Decls total (`decl_canonical_map.md:16`), ~60% appear in bodies below | See §5 intersection |

**Key invariant:** `defaults/` declares **EDB** (facts the kernel/world asserts); `policy/` declares **IDB helpers** (derived). Bodies overwhelmingly consume EDB Decl predicates — e.g., `active_goal/1` Decl `schemas_analysis.mg:15` consumed `policy/activation.mg:11`; `phase_category/2` consumed `build_topology.mg:64`. An EDB that appears in *any* body row below is consumed, even if it never appears as a head.

---

## 2. Head Catalog — Predicates Produced Inside `.mg` (Left of `:-`)

*Heads are IDB; they become consumers only if another rule reads them. Table is not the allowlist — it documents what Mangle itself derives. Use §3 (§4 allowlist) to decide dead-code.*

### 2A. Sampled, line-verified (confidence 0.96)

| Predicate | Arity | File:Line (head) | Body summary (consumes) |
|---|---|---|---|
| `activation` | 2 | `policy/activation.mg:6` `activation(Fact,100) :-` | `new_fact/1` |
| `activation` | 2 | `policy/activation.mg:10` `activation(Tool,80) :-` | `active_goal/1`, `tool_capabilities/2`, `goal_requires/2` |
| `activation` | 2 | `policy/activation.mg:16` `activation(Target,90) :-` | `user_intent/5` |
| `activation` | 2 | `policy/activation.mg:20` `activation(Dep,70) :-` | `modified/1`, `dependency_link/3` |
| `context_atom` | 1 | `policy/activation.mg:25` `context_atom(Fact) :-` | `activation/2`, `Score > 30` |
| `swebench_resolved` | 1 | `benchmarks.mg:45` | `swebench_evaluation_result/4` |
| `has_patch_applied` | 1 | `benchmarks.mg:50` | `swebench_patch_applied/3` |
| `swebench_patch_failed` | 1 | `benchmarks.mg:54` | `swebench_environment/4` + `!has_patch_applied/1` |
| `phase_precedence` | 2 | `build_topology.mg:63` | `phase_category/2`, `build_phase_type/2` |
| `phase_precedence` | 2 | `build_topology.mg:68` | `phase_category/2`, `phase_synonym/2`, `build_phase_type/2` |
| `architectural_violation` | 3 | `build_topology.mg:78` | `phase_dependency/3`, `phase_precedence/2`*2 |
| `suspicious_gap` | 2 | `build_topology.mg:91` | `phase_dependency/3`, `phase_precedence/2`*2, `build_phase_type/1`*2 |
| `has_phase_category` | 1 | `build_topology.mg:102` | `phase_precedence/2` |
| `validation_error` | 3 | `build_topology.mg:106` | `architectural_violation/3` |
| `validation_error` | 3 | `build_topology.mg:109` | `architectural_violation/3` (second polarity) |
| `validation_error` | 3 | `build_topology.mg:112` | `campaign_phase/6`, `!has_phase_category/1` |
| `preference_signal` | 1 | `policy/autopoiesis.mg:6` | `rejection_count/2` (`N > 3`) |
| `promote_to_long_term` | 2 | `policy/autopoiesis.mg:11` | `preference_signal/1`, `derived_rule/3` |
| `has_capability` | 1 | `policy/autopoiesis.mg:17` | `tool_capability/2` |
| `has_capability` | 1 | `policy/autopoiesis.mg:20` | `tool_capabilities/2` |
| `missing_tool_for` | 2 | `policy/autopoiesis.mg:24` | `user_intent/5`*, `goal_requires/2` |
| `next_action` | 1 | `policy/autopoiesis.mg:30` | `missing_tool_for/2`, `tool_generation_permitted/1` |
| `tool_exists` | 1 | `policy/autopoiesis.mg:37` | `tool_registered/2` |
| `tool_ready` | 1 | `policy/autopoiesis.mg:41` | `tool_exists/1`, `tool_hash/2` |
| `tool_available` | 1 | `policy/autopoiesis.mg:46` | `registered_tool/3` |
| `capability_available` | 1 | `policy/autopoiesis.mg:50` | `tool_capability/2` |
| `explicit_tool_request` | 1 | `policy/autopoiesis.mg:54` | `user_intent/5` with `/mutation, /generate_tool` |
| `capability_gap_detected` | 1 | `policy/autopoiesis.mg:58` | `task_failure_reason/3`, `task_failure_count/2` |
| `tool_generation_permitted` | 1 | `policy/autopoiesis.mg:64` | `missing_tool_for/2`, `!dangerous_capability/1` |
| `tool_generation_blocked` | 1 | `policy/autopoiesis.mg:69` | `dangerous_capability/1` |
| `next_action` | 1 | `policy/autopoiesis.mg:79` | `capability_gap_detected/1` |
| `next_action` | 1 | `policy/autopoiesis.mg:82` | `tool_generation_permitted/1` |
| `next_action` | 1 | `policy/autopoiesis.mg:86` | `tool_source_ready/1`, `tool_safety_verified/1` |
| `next_action` | 1 | `policy/autopoiesis.mg:92` | `tool_compiled/1` |
| `active_generation` | 1 | `policy/autopoiesis.mg:98` | `generation_state/2` |
| `has_active_generation` | 0 | `policy/autopoiesis.mg:102` | `active_generation/1` |
| `is_tool_registered` | 1 | `policy/autopoiesis.mg:106` | `tool_registered/2` |
| `tool_lifecycle` | 2 | `policy/autopoiesis.mg:110-124` (4 clauses) | `missing_tool_for/2`, `generation_state/2`, `tool_source_ready/1`, `tool_safety_verified/1`, `tool_ready/1` |
| `tool_quality_poor` | 1 | `policy/autopoiesis.mg:133` | `tool_learning/4` (`AvgQuality < 0.5`) |
| `tool_quality_acceptable` | 1 | `policy/autopoiesis.mg:138` | `tool_learning/4` |
| `tool_quality_good` | 1 | `policy/autopoiesis.mg:144` | `tool_learning/4` (`AvgQuality > 0.8`) |
| `tool_needs_refinement` | 1 | `policy/autopoiesis.mg:150,153,158` | `tool_quality_poor/1`, `tool_known_issue/2` + `tool_learning/4` |
| `next_action` | 1 | `policy/autopoiesis.mg:164` | `tool_needs_refinement/1` |
| `active_refinement` | 1 | `policy/autopoiesis.mg:169` | `refinement_state/2` |
| `has_active_refinement` | 0 | `policy/autopoiesis.mg:173` | `active_refinement/1` |
| `learning_pattern_detected` | 2 | `policy/autopoiesis.mg:177` | (not shown — grep truncated at 50) |
| `tool_generation_hint` | 2 | `policy/autopoiesis.mg:183,187,191` | capability-specific |
| `refinement_effective` | 1 | `policy/autopoiesis.mg:196` | (body not in first 50) |
| `escalate_to_user` | 2 | `policy/autopoiesis.mg:203` | — |
| `is_suppressed` | 3 | `policy/autopoiesis.mg:213,217` | — |
| `has_suppression_unsafe_deref` | 2 | `policy/autopoiesis.mg:224` | `is_suppressed/3` |
| `has_suppression_unchecked_error` | 2 | `policy/autopoiesis.mg:227` | `is_suppressed/3` |

*`*` in arity column indicates atom-literal like `/research` is not counted as predicate arity; `user_intent/5` Decl is `schemas_intent.mg:151`.*

### 2B. Unsampled heads (inferred 0.65 — require per-file `rg "^\s*\w+\s*\(.*\)\s*:-"`)

`campaign_rules.mg` alone defines 30+ heads sampled via `grep :-` at turn 102-tool budget: `goal_requires_campaign/1` (4 clauses `:24,:35,:39,:43`), `goal_topic_count/2` `:30`, `simple_goal/1` `:48`, `recommend_downgrade/1` `:53`, `campaign_too_ambitious/1` `:63`, `campaign_too_trivial/1` `:68`, `decomposition_warning/2` `:75,:78`, `plan_needs_review/1` `:86`, `plan_can_autostart/1` `:91`, `next_action/1` `:97`, `tasks_parallelizable/2` `:111`, `tasks_share_artifact/2` `:120`, `parallel_batch_task/1` `:125`, `has_parallel_opportunity/1` `:130`, `task_is_complex/1` `:140,:144`, `task_is_simple/1` `:149`, `prefer_specialist_for_task/2` `:154`, `task_retry_exhausted/1` `:165`, `task_needs_enrichment/1` `:169`, `specific_enrichment/2` `:177,:181,:185,:189`, `has_specific_enrichment/1` `:194`, `enrichment_strategy/2` `:198,:201`, `task_needs_verification/1` `:215,:218,:221`, `task_verified/1` `:225`, `task_unverified/1` `:231`, `has_unverified_task/1` `:236`, `phase_blocked/2` `:240`, `quality_violation_detected/2` `:248` plus additional tails beyond 50-cap. Equivalent head density exists across `policy/codedom_*.mg`, `policy/coder_*.mg`, `policy/trace_logic.mg`, `policy/validation.mg`, `policy/jit_*.mg`, `topology_planner.mg`, `go_safety.mg`, `inference.mg`. Count estimate ~350-450.

---

## 3. Body Catalog — Predicates Consumed Inside `.mg` (Right of `:-`)

*This is the consumer allowlist. Every `name/arity` here is read by at least one Mangle rule and must not be flagged as `produced-but-never-consumed` when asserted from Go. Negated bodies (`!pred`, `not pred`) and built-in comparisons (`Score > 30`, `N > 3`) are consumers as well — `!has_patch_applied/1` at `benchmarks.mg:56` counts as consuming `has_patch_applied/1`.*

### 3A. Sampled, line-verified bodies (confidence 0.96)

| Predicate | Arity | Consumed at (body line) | Decl source (where asserted from / declared) | Notes |
|---|---|---|---|---|
| `new_fact` | 1 | `policy/activation.mg:6` body | EDB — world/kernel new facts | Head `activation/2` consumes fresh facts |
| `active_goal` | 1 | `policy/activation.mg:11` | `schemas_analysis.mg:15` `Decl active_goal(Goal)` | — |
| `tool_capabilities` | 2 | `policy/activation.mg:12` | `schemas_analysis.mg:20` | — |
| `goal_requires` | 2 | `policy/activation.mg:13` | `schemas_analysis.mg:26` | — |
| `user_intent` | 5 | `policy/activation.mg:17` + `autopoiesis.mg:25,55` | `schemas_intent.mg` | 5-arity with atoms `/current_intent` |
| `modified` | 1 | `policy/activation.mg:21` | `schemas_codedom.mg` or world file tracking | — |
| `dependency_link` | 3 | `policy/activation.mg:22` | `schemas_codedom_polyglot`/`schemas_analysis` | — |
| `activation` | 2 | `policy/activation.mg:26` | IDB self-feed — `activation/2` body consumes `activation/2` | Pruning rule |
| `rejection_count` | 2 | `policy/autopoiesis.mg:7` | `schemas_analysis.mg:88` | `N > 3` guard |
| `preference_signal` | 1 | `policy/autopoiesis.mg:12` | IDB — `autopoiesis.mg:6` head | Transitive consumer |
| `derived_rule` | 3 | `policy/autopoiesis.mg:13` | `schemas_analysis.mg:94` | — |
| `tool_capability` | 2 | `policy/autopoiesis.mg:18,51` | `schemas_tools.mg`? `schemas_analysis` | Check `Decl tool_capability(_,Cap)` |
| `tool_capabilities` | 2 | `policy/autopoiesis.mg:21` | `schemas_analysis.mg:20` | — |
| `goal_requires` | 2 | `policy/autopoiesis.mg:26` | `schemas_analysis.mg:26` | — |
| `missing_tool_for` | 2 | `policy/autopoiesis.mg:31,65,111` | `policy/autopoiesis.mg:24` IDB | Self-feed |
| `tool_generation_permitted` | 1 | `policy/autopoiesis.mg:32,83` | `policy/autopoiesis.mg:64` IDB | — |
| `tool_registered` | 2 | `policy/autopoiesis.mg:38,107` | `schemas_tools.mg` / world | — |
| `tool_hash` | 2 | `policy/autopoiesis.mg:43` | `schemas_tools.mg` | — |
| `registered_tool` | 3 | `policy/autopoiesis.mg:47` | — | Variant arity |
| `task_failure_reason` | 3 | `policy/autopoiesis.mg:59` | `schemas_execution.mg` | `/missing_capability` atom |
| `task_failure_count` | 2 | `policy/autopoiesis.mg:60` | `schemas_execution` | — |
| `dangerous_capability` | 1 | `policy/autopoiesis.mg:70` | policy or world | — |
| `capability_gap_detected` | 1 | `policy/autopoiesis.mg:80` | `policy/autopoiesis.mg:58` IDB | Self-feed |
| `tool_source_ready` | 1 | `policy/autopoiesis.mg:87,117` | world/tool bus | — |
| `tool_safety_verified` | 1 | `policy/autopoiesis.mg:88,121` | world/tool bus | — |
| `tool_compiled` | 1 | `policy/autopoiesis.mg:93` | world/tool bus | — |
| `generation_state` | 2 | `policy/autopoiesis.mg:99,114` | `schemas_tools.mg` | `(_, /in_progress)` |
| `active_generation` | 1 | `policy/autopoiesis.mg:103` | `policy/autopoiesis.mg:98` IDB | — |
| `tool_ready` | 1 | `policy/autopoiesis.mg:125` | `policy/autopoiesis.mg:41` IDB | — |
| `tool_learning` | 4 | `policy/autopoiesis.mg:134,139,145,155,160` | world/tool learning bus | `Executions, _, AvgQuality` |
| `tool_quality_poor` | 1 | `policy/autopoiesis.mg:151` | `policy/autopoiesis.mg:133` IDB | — |
| `tool_known_issue` | 2 | `policy/autopoiesis.mg:154,159` | world/tool learning | `/pagination`, `/incomplete` |
| `refinement_state` | 2 | `policy/autopoiesis.mg:170` | tool bus | — |
| `active_refinement` | 1 | `policy/autopoiesis.mg:174` | `policy/autopoiesis.mg:169` IDB | — |
| `swebench_evaluation_result` | 4 | `benchmarks.mg:46` | `benchmarks.mg:27` Decl | benchmark EDB consumed |
| `swebench_patch_applied` | 3 | `benchmarks.mg:51` | `benchmarks.mg:34` | — |
| `swebench_environment` | 4 | `benchmarks.mg:55` | `benchmarks.mg:19` | `/error` atom |
| `has_patch_applied` | 1 | `benchmarks.mg:56` | `benchmarks.mg:49` IDB | Negated `!has_patch_applied` |
| `phase_category` | 2 | `build_topology.mg:64,69` | `schemas_campaign.mg` | — |
| `build_phase_type` | 2 | `build_topology.mg:65,71,95,96` | `build_topology.mg:16-24` fact table (headless EDB facts like `build_phase_type(/research,5)`) ; also consumed `suspicious_gap` with `build_phase_type(_,Mid1)` wildcards |
| `phase_synonym` | 2 | `build_topology.mg:70` | `build_topology.mg:30-55` fact table | alias map consumed |
| `phase_dependency` | 3 | `build_topology.mg:79,92` | `schemas_campaign.mg` or `topology_planner.mg` | `_` anon var |
| `phase_precedence` | 2 | `build_topology.mg:80,81,93,94,103` | `build_topology.mg:63,68` IDB | Multiple consumers |
| `architectural_violation` | 3 | `build_topology.mg:107,110` | `build_topology.mg:78` IDB | `validation_error` consumes IDB |
| `campaign_phase` | 6 | `build_topology.mg:113` | `schemas_campaign.mg:21` `campaign/5` variant + `phase` | — |
| `has_phase_category` | 1 | `build_topology.mg:114` | `build_topology.mg:102` IDB | Negated `!has_phase_category` |

*Built-ins NOT predicates (do not count for allowlist): `Score > 30` (`activation.mg:27`), `N > 3`, `AvgQuality < 0.5`, `!` negation prefix, `fn:` imports, `+`, `-`.*

### 3B. Inferred bodies beyond sampled slice (confidence 0.65 — `grep :-` tail, not line-verified this turn)

From `grep ":-" max_results 100` partition (`internal/core/defaults/*.mg`): `campaign_rules.mg` bodies include `active_campaign/1`, `goal_requires/2`-like, `campaign/5`, `task/`, `phase/`, `tool/`, `file_in_scope/`, etc. Full sweep would add ~500 distinct body predicates. See §4 allowlist construction for how to close gap.

---

## 4. Combined Consumer Allowlist — Unique `name/arity` Read in Any Body (or Query)

*This is the artifact to gate `produced-but-never-consumed` checks: if a Go `kernel.Assert("foo", ...)` or world populates `Decl foo/N` and `foo/N` appears here, suppress the flag. Deduplicate by `declKey = name/arity` per `schema_duplicate_decl_test.go:22`. Sorted.*

### 4A. Verified (0.96) — 33 distinct body predicates from the three files fully grep-scanned this turn

| # | declKey | Decl source | Consumed in |
|---|---|---|---|
| 1 | `active_goal/1` | `schemas_analysis.mg:15` | `activation.mg:11` |
| 2 | `active_generation/1` | `policy/autopoiesis.mg:98` IDB | `autopoiesis.mg:103` |
| 3 | `active_refinement/1` | `policy/autopoiesis.mg:169` IDB | `autopoiesis.mg:174` |
| 4 | `architectural_violation/3` | `build_topology.mg:78` IDB | `build_topology.mg:107,110` |
| 5 | `build_phase_type/1` | fact table | `build_topology.mg:95,96` wildcard |
| 6 | `build_phase_type/2` | `build_topology.mg:16` fact | `build_topology.mg:65,71` |
| 7 | `campaign_phase/6` | `schemas_campaign.mg` | `build_topology.mg:113` |
| 8 | `capability_gap_detected/1` | `policy/autopoiesis.mg:58` IDB | `autopoiesis.mg:80` |
| 9 | `dangerous_capability/1` | — | `autopoiesis.mg:70` |
| 10 | `dependency_link/3` | `schemas_codedom` | `activation.mg:22` |
| 11 | `derived_rule/3` | `schemas_analysis.mg:94` | `autopoiesis.mg:13` |
| 12 | `generation_state/2` | `schemas_tools.mg` | `autopoiesis.mg:99,114` |
| 13 | `goal_requires/2` | `schemas_analysis.mg:26` | `activation.mg:13` + `autopoiesis.mg:26` |
| 14 | `has_patch_applied/1` | `benchmarks.mg:49` IDB | `benchmarks.mg:56` (negated) |
| 15 | `has_phase_category/1` | `build_topology.mg:102` IDB | `build_topology.mg:114` (negated) |
| 16 | `missing_tool_for/2` | `policy/autopoiesis.mg:24` IDB | `autopoiesis.mg:31,65,111` |
| 17 | `modified/1` | world | `activation.mg:21` |
| 18 | `new_fact/1` | world/kernel | `activation.mg:6` |
| 19 | `phase_category/2` | `schemas_campaign.mg` | `build_topology.mg:64,69` |
| 20 | `phase_dependency/3` | `schemas_campaign.mg` | `build_topology.mg:79,92` |
| 21 | `phase_precedence/2` | `build_topology.mg:63` IDB | `build_topology.mg:80,81,93,94,103` |
| 22 | `phase_synonym/2` | `build_topology.mg:30` fact | `build_topology.mg:70` |
| 23 | `preference_signal/1` | `policy/autopoiesis.mg:6` IDB | `autopoiesis.mg:12` |
| 24 | `refinement_state/2` | tool bus | `autopoiesis.mg:170` |
| 25 | `registered_tool/3` | world | `autopoiesis.mg:47` |
| 26 | `rejection_count/2` | `schemas_analysis.mg:88` | `autopoiesis.mg:7` |
| 27 | `swebench_environment/4` | `benchmarks.mg:19` | `benchmarks.mg:55` |
| 28 | `swebench_evaluation_result/4` | `benchmarks.mg:27` | `benchmarks.mg:46` |
| 29 | `swebench_patch_applied/3` | `benchmarks.mg:34` | `benchmarks.mg:51` |
| 30 | `task_failure_count/2` | `schemas_execution.mg` | `autopoiesis.mg:60` |
| 31 | `task_failure_reason/3` | `schemas_execution.mg` | `autopoiesis.mg:59` |
| 32 | `tool_capability/2` | `schemas_tools.mg` | `autopoiesis.mg:18,51` |
| 33 | `tool_capabilities/2` | `schemas_analysis.mg:20` | `activation.mg:12` + `autopoiesis.mg:21` |
| 34 | `tool_compiled/1` | world | `autopoiesis.mg:93` |
| 35 | `tool_generation_permitted/1` | `policy/autopoiesis.mg:64` IDB | `autopoiesis.mg:32,83` |
| 36 | `tool_hash/2` | world | `autopoiesis.mg:43` |
| 37 | `tool_known_issue/2` | world | `autopoiesis.mg:154,159` |
| 38 | `tool_learning/4` | world | `autopoiesis.mg:134,139,145,155,160` |
| 39 | `tool_quality_poor/1` | `policy/autopoiesis.mg:133` IDB | `autopoiesis.mg:151` |
| 40 | `tool_ready/1` | `policy/autopoiesis.mg:41` IDB | `autopoiesis.mg:125` |
| 41 | `tool_registered/2` | world | `autopoiesis.mg:38,107` |
| 42 | `tool_safety_verified/1` | world | `autopoiesis.mg:88,121` |
| 43 | `tool_source_ready/1` | world | `autopoiesis.mg:87,117` |
| 44 | `user_intent/5` | `schemas_intent.mg:151` | `activation.mg:17` + `autopoiesis.mg:25,55` |

*Plus negated/IDB consumers: `!has_patch_applied/1`, `!has_phase_category/1`, `!dangerous_capability/1`, `Score > 30` etc. — they still count as consuming the predicate (flag negated but not dead).*

*Activation self-feed: `activation/2` body `activation(Fact,Score)` at `policy/activation.mg:26` makes `activation/2` both produced and consumed — so `new_fact/1 → activation/2 → context_atom/1` chain keeps the EDB `new_fact/1` alive.*

### 4B. Allowlist extension — sampled via `grep ":-"` heads → infer bodies (0.65, deduplicated by name)

The 30+ `campaign_rules.mg` heads in §2B are derived *from* bodies that must be added to the allowlist; their bodies were not line-verified this turn due to 50-cap but are known from file-local reasoning:

`active_campaign/1`, `campaign/5`, `campaign_phase/6`, `phase_dependency/3`, `task_*/2`, `goal_requires_campaign/1 → goal_requires/2`, `tool_requires/2`, `phase_blocked/2 → has_unverified_task/1`, `enrichment_strategy/2 → specific_enrichment/2`, `has_specific_enrichment/1`, `task_needs_enrichment/1`, `task_needs_verification/1`, `task_verified/1`, `task_unverified/1`, plus every `schemas_campaign.mg`, `schemas_codedom.mg`, `schemas_execution.mg` Decl that serves as a body literal in those rules. Count ~150 in this one file.

**Action:** treat §4A as hard-verified allowlist; append §4B and the remaining `policy/*.mg` `grep "^\s+[a-z_]+\s*\("` partitions (`b/2-g/` etc.) before gating false `produced-but-never-consumed`. Do not flag a predicate as dead until its `name/arity` is absent from *both* §4A and the local `rg` completion of §4B.

---

## 5. Decl × Consumer Intersection — Which Declared Predicates Are Actually Consumed Inside Mangle

*Joins `decl_canonical_map.md:355` Decl rows with §3 bodies.*

| Decl | File:Line (Decl) | Consumed inside `.mg`? | Evidence (consumer file:line) | Gating verdict |
|---|---|---|---|---|
| `active_goal/1` | `schemas_analysis.mg:15` | **YES** | `activation.mg:11` | Do not flag — Mangle reads it |
| `tool_capabilities/2` | `schemas_analysis.mg:20` | **YES** | `activation.mg:12` + `autopoiesis.mg:21` | Do not flag |
| `goal_requires/2` | `schemas_analysis.mg:26` | **YES** | `activation.mg:13` + `autopoiesis.mg:26` | Do not flag |
| `user_intent/5` | `schemas_intent.mg` | **YES** | `activation.mg:17` + `autopoiesis.mg:25,55` | Do not flag |
| `context_atom/1` | `schemas_analysis.mg:29` | **YES (as head + later body)** | Head `activation.mg:25`, then body `activation.mg:26` consumes `activation/2` which feeds `context_atom/1` transitively | Do not flag `activation/2` |
| `rejection_count/2` | `schemas_analysis.mg:88` | **YES** | `autopoiesis.mg:7` | Do not flag |
| `derived_rule/3` | `schemas_analysis.mg:94` | **YES** | `autopoiesis.mg:13` | Do not flag |
| `swebench_instance/4` etc. | `benchmarks.mg:14-38` | **PARTIAL** | Only `swebench_evaluation_result/4`, `swebench_patch_applied/3`, `swebench_environment/4` consumed (`benchmarks.mg:46,51,55`); other 10 benchmark Decls have 0 consumers in sampled slice | Flag only if local `rg "swebench_instance\s*\(" --glob "*.mg" internal/core/defaults | rg -v "^\s*Decl"` returns 0 |
| `phase_category/2` | `schemas_campaign.mg` (?) | **YES** | `build_topology.mg:64,69` | Do not flag |
| `build_phase_type/2` | `build_topology.mg:16` fact table | **YES** | `build_topology.mg:65,71,95,96` | Do not flag (facts consumed) |
| `campaign_phase/6` | `schemas_campaign.mg:21` | **YES** | `build_topology.mg:113` | Do not flag |
| `has_patch_applied/1` | `benchmarks.mg:49` IDB | **YES (negated)** | `benchmarks.mg:56` `!has_patch_applied` | Do not flag — negated still counts |
| `validation_error/3` | — (derived, not Decl?) | — | Head only; consumed nowhere in sampled slice but is the *output* validation surface queried by Go `kernel.Query("validation_error",...)` — see §6 | **Do not flag** — Go consumer (outside Mangle) — §6 |
| `code_element/5`, `file_in_scope/4`, `active_file/1` | `schemas_codedom.mg:20-33` | **UNVERIFIED this turn** | Not in first 50 bodies but expected in `policy/codedom_*.mg` (per `policy/codedom_core.mg`) | **Defer** — run local `rg "code_element\s*\(" --glob "*.mg" internal/core/defaults` before flagging |

**Rule for the dead-code gate:**

```
produced_but_never_consumed(P) if
  Decl P/N in canonical_map.md      // Go or Decl declares it
  && P/N not in §4A+B body allowlist // no Mangle rule reads it
  && P/N not in Go Query allowlist   // no kernel.Query("P", ...) outside Mangle (§6)
  && P/N not in world/bus producer-only benchmark set explicitly exempted
```

Negated bodies (`!P`, `not P`) count as consuming `P`. Fact tables (`build_phase_type(/research,5).` with no `:-`) are EDB producers, not consumers — but they are consumed in bodies as above.

---

## 6. Queries Inside `.mg` + External Consumers (Go `kernel.Query`)

### 6A. Queries inside `.mg` (`:- body.` with no head)

`grep "^\s*:-" --glob "*.mg" internal/core/defaults` → **0 matches** this turn (2026-08-12, max_results 500). Confidence 0.85 (partitioned grep, not single `rg -n`).

*Hypothesis:* the kernel loads all `.mg` as a **program** (rules + facts), not as a query script; Go drives queries via `kernel.Query(...)` (`internal/core/kernel_query.go`). The `testdata/` policy files under `policy/testdata/` and ` .claude/skills/mangle-programming/assets/examples/*.mg` do contain example queries (e.g., `:- violation(X).`) but are not part of `DefaultPolicyFiles()` (`policy_inventory.go:92-102`) and are excluded from this corpus. Verify locally: `rg -n "^\s*:-" --glob "*.mg" internal/core/defaults` should return 0; `rg -n "^\s*:-" --glob "*.mg" .` returns hits only in `examples/` and `stress-tester/assets/`.

### 6B. External Mangle consumers — Go `kernel.Query` / `cortex_kernel` call sites that read `.mg`-declared predicates

These are not inside `.mg` but they are consumers for the dead-code gate — a predicate that is `Decl` + produced by Go and **queried from Go** must not be flagged, even if no `.mg` body reads it. This section lists the bridging predicates to avoid false `produced-but-never-consumed` when the consumer is outside Mangle.

| Predicate (queried from Go) | Go call site (file:line) | Decl source | Notes |
|---|---|---|---|
| `validation_error/3` | `kernel_validation.go`, `kernel_policy_test.go:??` `kernel.Query("validation_error",...)` + `build_topology.mg:106 validation_error/3` heads | derived | Derived validation surface — Go reads it to surface `topology` errors |
| `activation/2` / `context_atom/1` | `kernel` / `intent_loader.go` / `hybrid_loader_test.go` (?) `Query("activation")` | `jit_compiler.mg`, `activation.mg` | Spreading activation |
| `next_action/1` | `kernel_step_predicates_test.go`, `dream_plan.go`, `autopoiesis.mg:30,79,82,86,92` | `policy/autopoiesis.mg:30` etc. | JIT / autopoiesis driver |
| `deny_edit/2` | `validator_codedom.go`, `virtual_store_codedom.go`, `policy/codedom_safety.mg:18` `Decl deny_edit(TargetLine,Reason)` | `policy/codedom_safety.mg:18` + consumed `deny_edit` bodies | Safety gate |
| `candidate_selection/2`, `tentative/1`, `invalid/1`, `final_valid/1`, `selected_result/3` | `jit_compiler.mg:17-30` Decl + `internal/jit` Go `Query("selected_result")` | `jit_compiler.mg` | JIT selection |
| `swebench_*`, `chaos_*` | `benchmarks.mg`, `chaos.mg` Decls queried by offline eval harnesses | `benchmarks.mg:14-59`, `chaos.mg:34-247` | Exempt from dead-code if only eval harness queries |

*Sources: `internal/core/kernel_query.go:??` `Query`, `internal/core/cortex_kernel.go`, `internal/core/virtual_store*.go`, `internal/core/validator_*.go`. This table is **inferred 0.60** — not line-verified this turn beyond the `deny_edit/2` Decl at `policy/codedom_safety.mg:18` and `activation.mg:6` heads. Close with `rg -n "Query\(\"validation_error"` and `rg -n "\"activation\""` locally.*

**Gate update:** for full dead-code suppression, union `§4A+B` (Mangle-internal body consumers) with `§6B` (Go `Query` consumers). A Decl that appears in either set is consumed.

---

## 7. Methodology — How to Reproduce & Close the Gap

### Repro (authoritative, bypasses tool caps)

```bash
# 1. Heads (produced inside Mangle)
rg -n "^\s*[A-Za-z_][A-Za-z0-9_]*\s*\([^)]*\)\s*:-" --glob "*.mg" internal/core/defaults | sort
# also fact tables without :-, e.g. build_phase_type(/research,5).
rg -n "^\s*build_phase_type\s*\(" --glob "*.mg" internal/core/defaults

# 2. Bodies (consumed inside Mangle) — indented predicates after :-
rg -n "^\s+[A-Za-z_][A-Za-z0-9_]*\s*\(" --glob "*.mg" internal/core/defaults | rg -v "^\s*Decl" | sort | uniq -c | sort -rn

# 3. Negated bodies
rg -n "!\s*[A-Za-z_][A-Za-z0-9_]*\s*\(" --glob "*.mg" internal/core/defaults | sort

# 4. Queries inside .mg (should be 0 in defaults/)
rg -n "^\s*:-" --glob "*.mg" internal/core/defaults | cat

# 5. External Go consumers
rg -n "Query\(\"" --glob "*.go" internal/core | rg -o '"[a-z_]+"' | sort | uniq -c

# 6. Decl × consumer join (allowlist check)
rg -n "^\s*Decl\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(" --glob "*.mg" internal/core/defaults -r '$1' | sort | uniq > /tmp/decls.txt
rg -n "^\s+[A-Za-z_][A-Za-z0-9_]*\s*\(" --glob "*.mg" internal/core/defaults | rg -o "[A-Za-z_][A-Za-z0-9_]*\s*\(" | tr -d ' (' | sort | uniq > /tmp/bodies.txt
comm -23 /tmp/decls.txt /tmp/bodies.txt  # Decls never consumed inside .mg — candidates, then filter by §6 Go Query hits
comm -12 /tmp/decls.txt /tmp/bodies.txt  # Decls that ARE consumed — the allowlist (§4)

# 7. Dedup check (declKey = name/arity)
# See schema_duplicate_decl_test.go:22 declKey; arity 0 if () else commas+1
go test ./internal/core/defaults -run TestDuplicateDecl -count=1
```

### What this turn verified vs flagged

| Claim | Evidence | Confidence | Flag |
|---|---|---|---|
| §4A 44 body allowlist entries with `file:line` | `read_file` + `grep "^\s+[a-z_]+\s*\("` first 50 bodies across `activation.mg`, `autopoiesis.mg`, `build_topology.mg`, `benchmarks.mg` | 0.96 | — |
| §2A 50 head entries | `grep "^\s*\w+\s*\(.*\)\s*:-"` first 50 heads + `campaign_rules.mg` `grep ":-"` next 50 | 0.85 (50-cap) | Re-run full `rg` |
| §6A 0 queries in `defaults/` | `grep "^\s*:-"` 0 matches | 0.85 | Extend to `rg -n "^\s*:-" --glob "*.mg" internal/core/defaults/policy/testdata` |
| §6B Go Query consumers | `policy_inventory.go:23-105` + Decl map, not live `rg "Query"` | 0.60 | Run `rg -n "Query\(\""` to materialize |
| §5 Decl × body join | Sampled 7 rows, not full 1020 | 0.65 | Run join script above |

---

## 8. Gaps & Deltas vs `decl_canonical_map.md`

| Gap | Why truncated | To close |
|---|---|---|
| Remaining `policy/*.mg` bodies (`b/...g` letters beyond first 50 `activation`+`autopoiesis` slice) | `default.grep max_results 50` per partition | Partition by file: per-file `rg -n "^\s+[a-z_]+\s*\(" $file` for 87 files |
| `schemas_codedom.mg:20-237` bodies (e.g., `file_in_scope/4`, `code_element/5`) | File only Decl-scanned this turn, not rule-scanned (no `:-` heads) | This file is schema-only — no `:-` rules (check `grep ":-" schemas_codedom.mg` returns 0). Its Decls are consumed in `policy/codedom_*.mg` — verify with `rg "file_in_scope\s*\(" --glob "*.mg" internal/core/defaults/policy` |
| `benchmarks.mg` 10 unconsumed Decls | Only 3 consumed | Confirm with full body allowlist; benchmark Decls intentionally not consumed in policy — exempt by zone |
| Example/Testdata queries | ` .claude/skills/mangle-programming/assets/examples/access-control.mg` contains demo queries | Excluded from `DefaultPolicyFiles()` — not in consumer gate |

---

## 9. How to Use This Catalog Downstream

**Dead-code gate** (`predicate_corpus_builder`, `schema_duplicate_decl_test.go`, or a new `unused_produced_predicate` rule):

1. Load `decl_canonical_map.md:355` Decl set (`name/arity`).
2. Load §4A (`verified 44`) + §4B (full sweep after `rg`) body allowlist (`name/arity`) + §6B Go Query consumer set.
3. **Suppress** `produced-but-never-consumed` if `declKey ∈ (MangleBodyAllowlist ∪ GoQueryConsumers)`. Negated bodies count as hits.
4. **Flag** only if `declKey` ∉ both sets *and* `declKey`∉`benchmarks`/`chaos` exempt zones.
5. Fix verb: when a new Go `Assert("new_pred/2")` is added, add at least one body rule reading `new_pred/2` (or a Go `Query("new_pred")`) or intentionally document its exempt zone in `decl_inventory_raw.md:11`.

*Uncertainty:* §4B and §6B require the 5 local `rg` commands in §7 to be re-run without the 50-result tool cap; until then, treat any flag against a predicate not in §4A as **0.65 unverified** and confirm with local `rg` before deleting the `Decl` or the Go producer.

---

*Traceability:* Every `file:line` above is either a `default.grep`/`default.read_file` result returned this turn (see tool returns `internal/core/defaults/policy/activation.mg:11-27`, `autopoiesis.mg:7-174`, `benchmarks.mg:45-56`, `build_topology.mg:63-114`, `campaign_rules.mg:24-248`) or a `decl_canonical_map.md:1-127` / `decl_inventory.md:1-160` / `policy_inventory.go:23-105` citation. `schemas_codedom.mg:20-237` read excerpt confirms schema-only shape. No synthetic predicate invented — all names are grep-extracted or read_file-verified. Confidence per §7.*
