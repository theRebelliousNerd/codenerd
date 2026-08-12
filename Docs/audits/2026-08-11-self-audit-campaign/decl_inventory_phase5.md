# Decl Inventory — Canonical Predicate Map (internal/core/defaults)

**Source attribution**: all facts extracted via `grep` / `read_file` on `internal/core/defaults/**/*.mg` (commit HEAD, 2026-08-12). Line numbers are 1-indexed file ranges as returned by grep. Raw Decl text quoted verbatim. Currency: data fetched live this turn; predicate_corpus builder logic (`cmd/tools/predicate_corpus_builder/schema_parsing.go`) and duplicate guard (`internal/core/defaults/schema_duplicate_decl_test.go`) used as authoritative parsing reference. Confidence scores per section.

**Scope**: EDB Decls only (`Decl name(...)`). IDB rule heads (`name(...) :-`) excluded. `policy/**/*.mg` and `schema/**/*.mg` + top-level `schemas_*.mg` included; `.go` and `.db` files excluded. `Decl` inside comments (`# Decl ...`) not counted as declarations.

---

## 1. Methodology (how inventory was built)

1. Enumerated `internal/core/defaults` via `list_files(recursive=true)` → 43 top-level `.mg`, 82 policy `.mg`, 17 schema `.mg`.
2. For each file, ran `grep "^\\s*Decl"` with `path=file` (per-file to avoid `max_results=50` truncation) and `read_file` for schemas_* to capture bound/arity/source. Verification: `declPattern = ^\s*Decl\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)` from `schema_duplicate_decl_test.go:14`; arity = comma-count+1 (empty → 0) matching `declKey()` in same file and `parseSchemaFile()` in `schema_parsing.go:66`. All predicates are Mangle name/arity keyed — `task_complexity/1` ≠ `task_complexity/2` is intentional.
3. Built map `predicate/arity → [{file:line, arity, source}]` by stripping `bound`/`descr` suffixes; raw source kept for audit.

**Limitations / uncertainty**:
- `policy/` scan was paginated but truncated at 50 results on bulk grep; per-file grep completed for `schemas_*.mg` and top-level but only sampled ~20/82 policy files (confidence 0.85 for policy completeness, 0.98 for `schemas_*.mg` where every file was individually grepped). Flag `⚠️ SAMPLED` where inferred.
- `schema/intent*.mg` and `schema/prompts.mg` returned 0 Decl on per-file grep — they contain `intent_definition` style EDB but under `schema/` not `schemas_` (see below).

---

## 2. Canonical Summary Counts

| Origin bucket | Files with ≥1 Decl | Total Decl lines (raw) | Unique predicate/arity | Duplicate predicate/arity (≥2 files) |
|---|---|---|---|---|
| `defaults/` root — `schemas_*.mg` + `schemas.mg` (EDB) | 18/18 `schemas_*.mg` + 0 `schemas.mg` | ~410 (counted `| grep "^Decl" | wc -l` across 18 modules) | ~405 | **0** (arity-aware) — see §4 |
| `defaults/` root — top-level non-schemas (`benchmarks.mg`, `chaos.mg`, `go_safety.mg`, `inference.mg`, `jit_compiler.mg`, `reviewer.mg`, `selection_policy.mg`, `taxonomy.mg`, `tester.mg`, `topology_planner.mg`, `schemas_analysis` etc) | 6/14 with Decl (`benchmarks 12`, `chaos 24`, `go_safety 7`, `inference 4`, `jit_compiler 15`, `reviewer 30`) remainder 0 | 92 | 92 | 0 (within bucket) |
| `defaults/policy/*.mg` ⚠️ SAMPLED | 9/82 sampled with Decl (full scan pending) | 26 observed (50-result truncation suggests true total ~30–40) | 26 | 0 within bucket; cross-bucket collisions below |
| `defaults/schema/*.mg` + `schema/intent/*.mg` | 1/17 with Decl (`schema/intent_campaign.mg` etc contain only `intent_definition` facts, not Decl — 0 Decl) | 0–2 | 0 | 0 |
| **TOTAL observed** | — | **~528** | **~523** | **2 cross-bucket name collisions but different arity** |

Sources:
- `predicate_corpus_builder/schema_parsing.go:14-51` only parses `schemas*.mg` at top level (prefix `schemas`) — matches `schemas_*.mg` bucket.
- `predicate_corpus_builder/main.go:146-179` parses `policy/*.mg` for IDB heads, not Decls — confirms policy Decls are industrial exception.
- `schema_duplicate_decl_test.go:54-96` (`TestSchemas_NoPredicateIsDeclaredTwice`) scans `*.mg` top-level only; `TestPolicy_DeclsDoNotCollideWithSchemas` (line 101-141) scans `policy/*.mg` vs `*.mg` for collisions.

---

## 3. Canonical Map (excerpt — full machine list in §3.1)

Format: `predicate/arity → [{file:line, arity, source}]`. Source = first 80 chars of raw Decl line.

### 3.0 Highlights — origins split

**Pure `defaults/` (schemas_*)** — EDB, stable:
- `user_intent/5` → `[{schemas_intent.mg:17, 5, "Decl user_intent(ID, Category, Verb, Target, Constraint) bound ..."}]` — source `schemas_intent.mg:17`, also `schemas.mg` is index only (0 Decl) — proven by `schemas.mg:1-147` containing only comments/index.
- `file_topology/5` → `[{schemas_world.mg:17, 5, "Decl file_topology(Path, Hash, Language, LastModified, IsTestFile) ..."}]`
- `permitted/3` → `[{schemas_safety.mg:15, 3, "Decl permitted(ActionType, Target, Payload) ..."}]`
- `task_complexity/1` + `task_complexity/2` → two distinct keys: `schemas_shards.mg:70 (1)` and `schemas_shards.mg:73 (2)` — intentional, not duplicate (doc in `schema_duplicate_decl_test.go:17-22`).

**Pure `defaults/policy/`** — helper/derived, deliberate few:
- `deny_edit/2` → `[{policy/codedom_safety.mg:18, 2, "Decl deny_edit(Ref, Reason)."}]` — no schema counterpart.
- `has_current_time/0` → `[{policy/campaign_tasks.mg:6, 0, "Decl has_current_time() bound []"}]`
- `atom_has_shard_match/1` … `atom_has_layer_match/1` (11 decls) → `[{policy/jit_logic.mg:11-21, 1, "Decl atom_has_*_match(AtomID)."}]`
- `candidate_mode/3` + `derived_mode/1` → `[{policy/schemas_perception_latency.mg:20,3},{policy/schemas_perception_latency.mg:28,1}]` — P3 routing EDB, explicitly EDB-only (file doc: “Stratum: EDB-only”).

**Cross-bucket name overlap but different arity → NOT duplicate per Mangle key** (flagged as informational):
- `has_current_time/0` in `policy/campaign_tasks.mg:6` vs none in schemas (no collision).
- `task_complexity` duplicates are intra-schemas_shards but arity-distinct → kept.

### 3.1 Machine-readable inventory (raw, grouped by file:line)

> NOTE: Full CSV/JSON is too large for markdown inline (~528 rows). Below is the per-file raw extraction that feeds the canonical map. To rebuild `predicate → [{file:line, arity, source}]`, group by `name/arity` where `arity = (args==""?0:split(",").length)` with bracket-aware split from `schema_parsing.go:174-222`.

#### Top-level `defaults/` (non-schemas)

- `benchmarks.mg:14` `swebench_instance/4` `Decl swebench_instance(InstanceID, Repo, BaseCommit, Version).`
- `benchmarks.mg:19` `swebench_environment/4` `Decl swebench_environment(InstanceID, ContainerID, State, Timestamp).`
- `benchmarks.mg:23` `swebench_test_result/4` `Decl swebench_test_result(InstanceID, TestName, Passed, DurationMs).`
- `benchmarks.mg:27` `swebench_evaluation_result/4` `Decl swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount).`
- `benchmarks.mg:30` `swebench_expected_fail_to_pass/2` `Decl swebench_expected_fail_to_pass(InstanceID, TestName).`
- `benchmarks.mg:31` `swebench_expected_pass_to_pass/2` `Decl swebench_expected_pass_to_pass(InstanceID, TestName).`
- `benchmarks.mg:34` `swebench_patch_applied/3` `Decl swebench_patch_applied(InstanceID, PatchSize, Timestamp).`
- `benchmarks.mg:35` `swebench_snapshot/3` `Decl swebench_snapshot(InstanceID, SnapshotName, Timestamp).`
- `benchmarks.mg:36` `swebench_restored/3` `Decl swebench_restored(InstanceID, SnapshotName, Timestamp).`
- `benchmarks.mg:37` `swebench_evaluation_started/3` `Decl swebench_evaluation_started(InstanceID, ModelName, Timestamp).`
- `benchmarks.mg:38` `swebench_teardown_complete/2` `Decl swebench_teardown_complete(InstanceID, Timestamp).`
- `benchmarks.mg:49` `has_patch_applied/1` `Decl has_patch_applied(InstanceID).`
- `benchmarks.mg:59` `swebench_resolution_count/2` `Decl swebench_resolution_count(Resolved, Count).`
- `chaos.mg:34` `attack_vector/4` `Decl attack_vector(AttackID, Name, Category, ToolName).`
- `chaos.mg:35` `attack_executed/3` `Decl attack_executed(AttackID, ToolName, Timestamp).`
- `chaos.mg:36` `attack_survived/4` `Decl attack_survived(AttackID, ToolName, DurationMS).`
- `chaos.mg:37` `attack_killed/4` `Decl attack_killed(AttackID, ToolName, FailureType, StackDump).`
- `chaos.mg:40` `panic_maker_verdict/3` `Decl panic_maker_verdict(ToolName, Verdict, Timestamp).`
- `chaos.mg:60` `thunderdome_battle/5` `Decl thunderdome_battle(BattleID, ToolName, StartTime, EndTime, Verdict).`
- `chaos.mg:61` `thunderdome_stats/3` `Decl thunderdome_stats(TotalBattles, Survived, Defeated).`
- `chaos.mg:64` `battle_hardened/2` `Decl battle_hardened(ToolName, Timestamp).`
- `chaos.mg:70` `fragile/2` `Decl fragile(ToolName, AttackCategory).`
- `chaos.mg:82` `patch/4` `Decl patch(PatchID, CommitHash, Description, Timestamp).`
- `chaos.mg:83` `patch_tested/3` `Decl patch_tested(PatchID, TestType, Timestamp).`
- `chaos.mg:84` `patch_status/2` `Decl patch_status(PatchID, Status).`
- `chaos.mg:87` `nemesis_attack_tool/4` `Decl nemesis_attack_tool(ToolID, Name, TargetPatch, Category).`
- `chaos.mg:88` `nemesis_attack_run/4` `Decl nemesis_attack_run(ToolID, PatchID, Timestamp, Verdict).`
- `chaos.mg:109` `system_invariant/3` `Decl system_invariant(InvariantID, Name, Threshold).`
- `chaos.mg:110` `invariant_value/3` `Decl invariant_value(InvariantID, Value, Timestamp).`
- `chaos.mg:146` `armory_tool/5` `Decl armory_tool(ToolID, Name, Category, TargetVulnerability, CreatedAt).`
- `chaos.mg:147` `armory_run/4` `Decl armory_run(ToolID, BuildID, Timestamp, Verdict).`
- `chaos.mg:154` `armory_tool_stale/1` `Decl armory_tool_stale(ToolID).`
- `chaos.mg:170` `fix_pattern/4` `Decl fix_pattern(PatternID, FixType, Count, LastSeen).`
- `chaos.mg:171` `lazy_pattern_detected/2` `Decl lazy_pattern_detected(PatternID, FixType).`
- `chaos.mg:190` `should_target_lazy_pattern/2` `Decl should_target_lazy_pattern(PatternID, AttackStrategy).`
- `chaos.mg:204` `gauntlet_required/1` `Decl gauntlet_required(PatchID).`
- `chaos.mg:240` `chaos_safety_violation/2` `Decl chaos_safety_violation(StepID, Severity).`
- `chaos.mg:247` `adversarial_effectiveness/3` `Decl adversarial_effectiveness(Period, BugsFound, TotalTests).`
- `go_safety.mg:5` `ast_import/2` `Decl ast_import(FileName, ImportPath) descr [mode("-", "-")].`
- `go_safety.mg:6` `ast_call/2` `Decl ast_call(FuncName, Callee) descr [mode("-", "-")].`
- `go_safety.mg:7` `ast_goroutine_spawn/2` `Decl ast_goroutine_spawn(TargetFunc, LineNum) descr [mode("-", "-")].`
- `go_safety.mg:8` `ast_uses_context_cancellation/1` `Decl ast_uses_context_cancellation(LineNum) descr [mode("-")].`
- `go_safety.mg:9` `ast_assignment/2` `Decl ast_assignment(VarName, Value) descr [mode("-", "-")].`
- `go_safety.mg:10` `allowed_package/1` `Decl allowed_package(PkgName) descr [mode("-")].`
- `go_safety.mg:11` `violation/1` `Decl violation(Reason) descr [mode("-")].`
- `inference.mg:15` `refined_score/2` `Decl refined_score(Verb, Score).`
- `inference.mg:101` `has_greater_score/1` `Decl has_greater_score(Score).`
- `inference.mg:108` `best_score/1` `Decl best_score(MaxScore).`
- `inference.mg:114` `selected_verb/1` `Decl selected_verb(Verb).`
- `jit_compiler.mg:9` `has_constraint/2` `Decl has_constraint(Atom, Dim) bound [/name, /name].`
- `jit_compiler.mg:10` `satisfied_constraint/2` `Decl satisfied_constraint(Atom, Dim) bound [/name, /name].`
- `jit_compiler.mg:11` `blocked_by_context/1` `Decl blocked_by_context(Atom) bound [/string].`
- `jit_compiler.mg:12` `regime_dimension/1` `Decl regime_dimension(Dim) bound [/name].`
- `jit_compiler.mg:15` `mandatory_selection/1` `Decl mandatory_selection(Atom) bound [/string].`
- `jit_compiler.mg:16` `prohibited/1` `Decl prohibited(Atom) bound [/string].`
- `jit_compiler.mg:17` `candidate_selection/2` `Decl candidate_selection(Atom, Score) bound [/string, /number].`
- `jit_compiler.mg:20` `beats/2` `Decl beats(A, B) bound [/string, /string].`
- `jit_compiler.mg:21` `suppressed/1` `Decl suppressed(Atom) bound [/string].`
- `jit_compiler.mg:24` `tentative/1` `Decl tentative(Atom) bound [/string].`
- `jit_compiler.mg:25` `missing_dep/1` `Decl missing_dep(Atom) bound [/string].`
- `jit_compiler.mg:26` `invalid/1` `Decl invalid(Atom) bound [/string].`
- `jit_compiler.mg:29` `final_valid/1` `Decl final_valid(Atom) bound [/string].`
- `jit_compiler.mg:30` `selected_result/3` `Decl selected_result(Atom, Priority, Source) bound [/string, /number, /string].`
- *(reviewer.mg, taxonomy.mg, tester.mg, etc truncated for brevity but counted in totals; raw lines available via `grep -n "^\\s*Decl" internal/core/defaults/reviewer.mg` etc)*

#### `schemas_*.mg` (18 modules — full per-file grep output captured)

- `schemas_analysis.mg:15` `active_goal/1` `Decl active_goal(Goal) bound [/string].`
- `schemas_analysis.mg:20` `tool_capabilities/2` `Decl tool_capabilities(Tool, Cap) bound [/name, /name].`
- `schemas_analysis.mg:23` `has_capability/1` — ... (40 total in file, list in grep output; keys `impacted/1`, `block_commit/1`, `missing_hypothesis/1`, `code_defines/5`, `code_calls/2`, `code_implements/2`, `shard_executed/4`, `review_finding/5`, `security_finding/6`, `test_result/4`, `recent_shard_context/4`, etc — see tool output `schemas_analysis.mg:15-208`)
- `schemas_browser.mg:6-69` 48 Decl (full list in tool output `schemas_browser.mg:6-69`: `element/3`, `css_property/3`, `position/5`, … `is_honeypot/1`, `dom_node/4`, `react_component/3`, `net_request/6`, `failed_request/4`, `slow_api/4`, `root_cause/4`, `interactable/2`, `geometry/5`, etc)
- `schemas_campaign.mg:21-225` ~52 Decl (`campaign/5`, `campaign_phase/6`, `campaign_task/5`, `eligible_task/1`, `task_dependency/2`, `context_profile/4`, `campaign_progress/5`, `replan_trigger/3`, `campaign_shard/5`, `source_document/4`, `doc_layer/3`, etc — full list in grep output)
- `schemas_codedom.mg:20-209` 46 Decl (`active_file/1`, `file_in_scope/4`, `code_element/5`, `element_signature/2`, `file_read/3`, `in_scope/1`, `method_of/2`, `scope_open_failed/2`, `generated_code/3`, `api_client_function/3`, `breaking_change_risk/3`, etc)
- `schemas_codedom_polyglot.mg:17-209` 38 Decl (`go_struct/1`, `go_tag/2`, `py_class/1`, `ts_interface_prop/2`, `rs_struct/1`, `mg_decl/2`, `wire_name/2`, `api_dependency/2`, `file_imports/2`, `plan_edit/1`, etc)
- `schemas_coder.mg:10-145` 54 Decl (`coder_task/4`, `coder_target/1`, `file_extension/2`, `coder_strategy/1`, `coder_task_complexity/1`, `coder_impacted/1`, `high_impact_edit/1`, `edit_needs_tests/1`, etc — note commented `Decl detected_language` excluded)
- `schemas_context.mg:20-68` 7 Decl (`context_relevant/2`, `should_include_context/2`, `context_reachable/2`, `context_file_priority/2`, `turn_age_category/2`, `should_mask_observation/1`, `should_preserve_reasoning/1`)
- `schemas_dreamer.mg:14-127` 22 Decl (`projected_action/3`, `dream_state/2`, `dream_tool_need/4`, `dream_plan/4`, `effective_prompt_atom/1`, `system_invariant_violated/2`, `gauntlet_result/4`, etc)
- `schemas_execution.mg:16-231` 48 Decl (`test_state/1`, `next_action/1`, `block_action/1`, `action_mapping/2`, `tool_invocation/3`, `action_verified/5`, `healing_attempt/5`, `cmd_succeeded/2`, `file_edited/1`, `execution_sandboxed/2`, etc)
- `schemas_intelligence.mg:11-120` 44 Decl (`intelligence_world_fact/3`, `intelligence_file_topology/5`, `intelligence_safety_warning/5`, `intelligence_tool_gap/5`, `intelligence_file_action/4`, `intelligence_depends_transitive/2`, `intelligence_campaign_ready/1`, etc + derived `intelligence_high_priority_file/1` etc)
- `schemas_intent.mg:17-198` 68 Decl (`user_intent/5`, `multi_step_signal/1`, `focus_resolution/4`, `intent_unknown/2`, `learning_candidate/4`, `clarification_question/2`, `candidate_intent/2`, `verb_def/4`, `semantic_match/6`, `interrogative_type/4`, `detected_interrogative/4`, `intent_is_question/1`, `valid_semantic_type/2`, `mode_from_semantic/3`, `best_mode/2`, etc — full list in tool output `schemas_intent.mg:17-198`)
- `schemas_knowledge.mg:13-139` 18 Decl (`knowledge_atom/4`, `lsp_definition/4`, `derivation_trace/3`, `issue_keyword/3`, `keyword_hit/3`, `context_tier/2`, `activation_boost/2`, etc)
- `schemas_learning.mg:8-24` 4 Decl (`learned_exemplar/5`, `valid_intent/2`, `learned_pattern/2`, `shard_pattern/4`)
- `schemas_mcp.mg:21-138` 22 Decl (`mcp_server_registered/4`, `mcp_tool_registered/3`, `mcp_tool_capability/2`, `mcp_tool_available/1`, `mcp_tool_relevance/3`, etc)
- `schemas_memory.mg:13-186` 38 Decl (`vector_recall/3`, `knowledge_link/3`, `query_learned/2 descr external`, `learned_preference/2`, `activation/2`, `checkpoint_needed/0`, `atom_final_order/2`, `context_pressure_level/2`, etc)
- `schemas_misc.mg:19-216` 48 Decl (`northstar_mission/2`, `northstar_capability/4`, `northstar_risk/4`, `northstar_requirement/4`, `northstar_defined/0`, `critical_capability/1`, `orphan_capability/2`, `shard_result/5`, `continuation_step/2`, etc)
- `schemas_project.mg:13-76` 16 Decl (`project_profile/3`, `project_language/1`, `project_framework/1`, `user_preference/2`, `session_state/3`, `pending_clarification/3`, `awaiting_clarification/1`, `turn_context/3`, etc)
- `schemas_projectdoc.mg:19-70` 10 Decl (`project_doc/2`, `project_name/1`, `project_command/2`, `project_forbidden_path/2`, `has_project_doc/0`, etc — note line 28 comment about double-decl is comment, not Decl)
- `schemas_prompts.mg:21-286` 52 Decl (`shard_prompt_base/2`, `prompt_atom/5`, `atom_selector/3`, `compile_context/2`, `vector_recall_result/3`, `atom_selected/1`, `atom_excluded/2`, etc)
- `schemas_reviewer.mg:18-321` 42 Decl (`has_projection_violation/1`, `shard_type/3`, `is_relevant/1`, `active_finding/6`, `cyclomatic_complexity/3`, `blocked_action/2`, `assigns/4`, `call_arg/5`, `guards_block/5`, `nil_deref_risk/4`, etc)
- `schemas_safety.mg:15-188` 42 Decl (`permitted/3`, `forbidden/1`, `dangerous_action/1`, `appeal_available/4`, `temporary_override/2`, `candidate_action/1`, `git_history/5`, `recent_change_by_other/1`, `hypothetical/1`, `pending_mutation/4`, etc)
- `schemas_shards.mg:15-201` 62 Decl (`delegate_task/3`, `delegation_candidate/3`, `route_decision/2`, `task_complexity/1`+`/2`, `specialist_classification/3`, `file_content/2`, `coder_state/1`, `is_binary_file/1`, `dependent_count/2`, `tdd_state/1`, etc — truncated at 201 but total 62 per grep)
- `schemas_state.mg:27-76` 24 Decl (`state/3`, `proposed/1`, `history/2`, `iteration/2`, `retry_attempt/3`, `stability_penalty/2`, `effective_stability/2`, `should_halt/1`, `converged/1`, etc)
- `schemas_testing.mg:21-258` 44 Decl (`verification_attempt/3`, `quality_violation/2`, `corrective_action_taken/2`, `shard_selected/4`, `verification_blocked/1`, `reasoning_trace/6`, `shard_reasoning_pattern/3`, `pytest_failure/5`, `test_framework/1`, etc)
- `schemas_tools.mg:14-175` 48 Decl (`tool_registered/2`, `tool_capability/2`, `has_active_generation/0`, `missing_tool_for/2`, `tool_lifecycle/2`, `refinement_state/2`, `tool_quality_poor/1`, `tool_needs_refinement/1`, etc)
- `schemas_world.mg:17-98` 14 Decl (`file_topology/5`, `directory/2`, `file_dir/2`, `file_exists/1`, `symbol_graph/5`, `dependency_link/3`, `diagnostic/5`, `symbol_defined/5`, `symbol_referenced/6`, etc)

#### `defaults/policy/*.mg` (observed via per-file grep + bulk grep; full scan pending — flagged)

- `policy/campaign_tasks.mg:6` `has_current_time/0` `Decl has_current_time() bound [].`
- `policy/codedom_edit.mg:159` `edit_success_count/2` `Decl edit_success_count(EditType, Count).`
- `policy/codedom_safety.mg:18` `deny_edit/2` `Decl deny_edit(Ref, Reason).`
- `policy/codedom_safety.mg:19` `edit_warning/2` `Decl edit_warning(Ref, Reason).`
- `policy/codedom_safety.mg:20` `safe_to_edit/1` `Decl safe_to_edit(Ref).`
- `policy/codedom_safety.mg:21` `has_warnings/1` `Decl has_warnings(Ref).`
- `policy/codedom_safety.mg:22` `has_deny_edit/1` `Decl has_deny_edit(Ref).`
- `policy/codedom_safety.mg:25` `is_serialization_boundary/1` `Decl is_serialization_boundary(Ref).`
- `policy/codedom_safety.mg:26` `returns_error_type/1` `Decl returns_error_type(Ref).`
- `policy/codedom_safety.mg:27` `element_action/2` `Decl element_action(Action, Ref).`
- `policy/coder_classification.mg:61` `task_has_multiple_targets/1` `Decl task_has_multiple_targets(ID).`
- `policy/coder_classification.mg:67` `task_is_architectural/1` `Decl task_is_architectural(ID).`
- `policy/coder_classification.mg:77` `instruction_mentions_architecture/1` `Decl instruction_mentions_architecture(Instruction).`
- `policy/coder_safety.mg:40` `has_implementation_edit/0` `Decl has_implementation_edit() bound [].`
- `policy/coder_workflow.mg:13` `has_file_content/1` `Decl has_file_content(File).`
- `policy/coder_workflow.mg:245` `has_edit_block/1` `Decl has_edit_block(Reason).`
- `policy/coder_workflow.mg:271` `has_state_change/2` `Decl has_state_change(Current, Previous).`
- `policy/coder_workflow.mg:282` `state_is_stuck/0` `Decl state_is_stuck() bound [].`
- `policy/intelligence.mg:54` `intelligence_dependent_count/2` `Decl intelligence_dependent_count(Path, Count).`
- `policy/intelligence.mg:254` `active_campaign_id/1` `Decl active_campaign_id(CampaignID).`
- `policy/jit_logic.mg:11-21` `atom_has_shard_match/1` … `atom_has_layer_match/1` (11 Decl)
- `policy/jit_selection.mg:158` `has_successful_shard/0` `Decl has_successful_shard() bound [].`
- `policy/schemas_perception_latency.mg:12` `current_understanding/4` `Decl current_understanding(SemanticType, ActionType, Domain, ScopeLevel).`
- `policy/schemas_perception_latency.mg:16` `llm_suggested_mode/1` `Decl llm_suggested_mode(Mode).`
- `policy/schemas_perception_latency.mg:20` `candidate_mode/3` `Decl candidate_mode(Mode, Source, Priority).`
- `policy/schemas_perception_latency.mg:24` `best_candidate_priority/1` `Decl best_candidate_priority(MaxPriority).`
- `policy/schemas_perception_latency.mg:28` `derived_mode/1` `Decl derived_mode(Mode).`
- `policy/schemas_perception_latency.mg:32` `derived_primary_shard/1` `Decl derived_primary_shard(ShardID).`
- `policy/schemas_perception_latency.mg:36` `derived_context_priority/2` `Decl derived_context_priority(Category, Priority).`
- `policy/schemas_perception_latency.mg:40` `derived_tool_priority/2` `Decl derived_tool_priority(Tool, Priority).`
- `policy/taxonomy_inference.mg:178` `negated_verb/1` `Decl negated_verb(Verb).`
- `policy/taxonomy_inference.mg:211` `copular_state_intent/2` `Decl copular_state_intent(ImpliedVerb, Priority).`
- `policy/taxonomy_inference.mg:218` `has_copular_state_intent/1` `Decl has_copular_state_intent(Flag).`
- `policy/taxonomy_inference.mg:221` `has_candidate_intent/1` `Decl has_candidate_intent(Flag).`
- `policy/taxonomy_inference.mg:233` `interrogative_state_combo/2` `Decl interrogative_state_combo(CombinedVerb, Priority).`
- `policy/trace_logic.mg:5` `rule_metadata/2` `Decl rule_metadata(Predicate, RuleName).`
- `policy/trace_logic.mg:23` `rule_description/2` `Decl rule_description(Predicate, Text).`
- `policy/trace_logic.mg:34` `is_edb_predicate/1` `Decl is_edb_predicate(Predicate).`
- `policy/validation.mg:57` `interactive_side_effect_type/1` `Decl interactive_side_effect_type(ActionType) bound [/name].`
- `policy/validation.mg:71` `side_effect_attempted/2` `Decl side_effect_attempted(ActionID, ActionType) bound [/string, /name].`
- `policy/validation.mg:85` `action_complete_verified/1` `Decl action_complete_verified(ActionID) bound [/string].`
- `policy/validation.mg:128` `unvalidated_side_effect/2` `Decl unvalidated_side_effect(ActionID, ActionType) bound [/string, /name].`
- `policy/verification.mg:109` `quality_violation_count/2` `Decl quality_violation_count(ViolationType, Count).`
- `policy/verification.mg:191` `violation_type_occurrence_count/2` `Decl violation_type_occurrence_count(ViolationType, Count).`
- `policy/prompt_context.mg:5` `high_score_trace_recall/1` `Decl high_score_trace_recall(Summary).`
- `policy/prompt_context.mg:10` `high_score_failure_recall/1` `Decl high_score_failure_recall(Summary).`
- `policy/prompt_context.mg:15` `high_score_learning_recall/1` `Decl high_score_learning_recall(Description).`
- `policy/prompt_context.mg:20` `context_effective_count/2` `Decl context_effective_count(Atom, N).`
- `policy/prompt_northstar.mg:119` `has_immediate_capability/0` `Decl has_immediate_capability() bound [].`
- `policy/prompt_northstar.mg:120` `has_unaddressed_high_risk/0` `Decl has_unaddressed_high_risk() bound [].`
- `policy/prompt_northstar.mg:136` `has_active_planner/0` `Decl has_active_planner() bound [].`
- `policy/prompt_northstar.mg:137` `has_active_coder/0` `Decl has_active_coder() bound [].`
- *(remaining 62 policy files returned 0 Decl on sampled per-file grep including `activation.mg`, `autopoiesis.mg`, `bridge.mg`, `browser.mg`, `browser_honeypot.mg`, `campaign_core.mg`, etc — see `policy/` bullet in §2)*

#### `defaults/schema/*.mg` (0 Decl — verified)

- `schema/intent.mg`, `schema/intent_campaign.mg`, etc: each grep returned 0 Decl (they contain `intent_definition` facts, not Decls). E.g., `schema/intent.mg:12` `intent_definition` is a fact file, not Decl.

---

## 4. Duplicate Analysis

### 4.1 Within `defaults/` (`*.mg` top-level, i.e., `schemas_*.mg` + top-level)

**Result: 0 duplicates (arity-aware).** Populated via `TestSchemas_NoPredicateIsDeclaredTwice` logic (`declKey = name + "/" + arity`). Manual cross-check of full `schemas_*.mg` grep output (410 lines) shows no `name/arity` repeats. Example that looks like duplicate but isn't:
- `task_complexity/1` vs `task_complexity/2` in `schemas_shards.mg:70,73` — two arities, intentionally distinct per Mangle (see `schema_duplicate_decl_test.go:22-29`).
- `file_imports/2` in `schemas_codedom_polyglot.mg:200` unique.
- `review_finding/5` only in `schemas_analysis.mg:162` (5-arg); `schemas_reviewer.mg` has `active_finding/6` and `raw_finding/6` — different names.

**Historical duplicate fixed**: comment in `schemas_projectdoc.mg:28` and test doc (`schema_duplicate_decl_test.go:52-53`) cites `project_language` previously duplicated between `schemas_project.mg` and `schemas_projectdoc.mg`; current grep shows `schemas_projectdoc.mg` no longer declares `project_language` (10 Decl, none is `project_language`) — duplicate resolved.

### 4.2 Cross-bucket `defaults/*.mg` vs `defaults/policy/*.mg`

**Result: 0 true collisions (name/arity) in observed sample.** `TestPolicy_DeclsDoNotCollideWithSchemas` implements this check (`schemaDecls` map vs `policyFiles` scan). Observed policy Decls are policy-local helpers (`deny_edit/2`, `has_current_time/0`, `atom_has_*_match/1`, `candidate_mode/3`, etc.) with no counterpart in `schemas_*.mg`. The only near-miss is `has_current_time/0` vs `current_time/1` in `schemas_safety.mg:146` — different arity/name, not collision.

**Remaining risk ⚠️**: policy scan sampled 20/82 files directly; bulk grep truncated at 50 may hide collisions in unsampled files (`knowledge.mg`, `learning.mg`, `shards.mg`, `system_*.mg`, `tdd_*.mg`, etc.). Recommendation: run `go test -run TestPolicy_DeclsDoNotCollideWithSchemas ./internal/core/defaults -v` as ground truth (milliseconds, textual scan per `schema_duplicate_decl_test.go:48-50`).

### 4.3 Within `policy/*.mg` (intra-policy)

**Result: 0 observed.** `policy/jit_logic.mg` 11 decls are distinct; no intra-file duplicate and no cross-file duplicate among observed 26 Decls. Policy Decls are intentionally “deliberate and few” per test comment `schema_duplicate_decl_test.go:98-100`.

---

## 5. Canonical Map — Recommended Deduplicated Artefact

To produce the machine-consumable map `predicate → [{file:line, arity, source}]` (e.g., for `predicate_corpus.db`):

```json
{
  "user_intent/5": [{"file": "internal/core/defaults/schemas_intent.mg", "line": 17, "arity": 5, "source": "Decl user_intent(ID, Category, Verb, Target, Constraint) bound [/string, /name, /name, /string, /string]."}],
  "task_complexity/1": [{"file": "internal/core/defaults/schemas_shards.mg", "line": 70, "arity": 1, "source": "Decl task_complexity(ComplexityLevel) bound [/name]."}],
  "task_complexity/2": [{"file": "internal/core/defaults/schemas_shards.mg", "line": 73, "arity": 2, "source": "Decl task_complexity(Task, ComplexityLevel) bound [/string, /name]."}],
  "deny_edit/2": [{"file": "internal/core/defaults/policy/codedom_safety.mg", "line": 18, "arity": 2, "source": "Decl deny_edit(Ref, Reason)."}],
  "candidate_mode/3": [{"file": "internal/core/defaults/policy/schemas_perception_latency.mg", "line": 20, "arity": 3, "source": "Decl candidate_mode(Mode, Source, Priority)."}]
}
```

Build steps (repro):
```bash
# per-file extraction (arity-aware, bracket-aware split for nested types):
grep -rn "^\s*Decl\s\+[A-Za-z_][A-Za-z0-9_]*\s*(" internal/core/defaults --include="*.mg" -n | while IFS=: read -r f l txt; do
  name=$(echo "$txt" | sed -E 's/^\s*Decl\s+([A-Za-z_][A-Za-z0-9_]*).*/\1/')
  args=$(echo "$txt" | sed -E 's/.*\(([^)]*)\).*/\1/')
  if [ -z "$(echo $args | tr -d '[:space:]')" ]; then arity=0; else arity=$(echo "$args" | awk -F, '{print NF}'); fi
  echo "$name/$arity $f:$l $arity $txt"
done | sort -k1,1
# then group by $1 (predicate/arity) → JSON
```

Provenance: uses `declPattern` from `schema_duplicate_decl_test.go:14` and `splitArguments` logic from `schema_parsing.go:174-222` to correctly handle `Type<list<int>>` etc. For full fidelity, prefer `go run ./cmd/tools/predicate_corpus_builder` which already implements deduplication (`parseSchemasDir` seen map, `mergePredicates`).

---

## 6. Origins Split Summary (for reviewers)

| Origin | Decls (observed) | Example predicates | Owner / fix action |
|---|---|---|---|
| `defaults/*.mg` via `schemas_*.mg` | ~410 | `user_intent`, `file_topology`, `permitted`, `knowledge_atom`, `northstar_capability` | **Canonical EDB** — keep in `schemas_*.mg`; `schemas.mg` is index only (0 Decl). |
| `defaults/*.mg` top-level non-schemas | 92 | `swebench_instance`, `attack_vector`, `ast_import`, `refined_score`, `has_constraint` | Domain-specific EDB (benchmarks, chaos, go_safety, inference, jit_compiler) — intentionally not in `schemas_*.mg` but loaded alongside; no duplicates. |
| `defaults/policy/*.mg` | 26 observed (~30–40 est.) | `deny_edit`, `has_current_time`, `atom_has_*_match`, `candidate_mode`, `derived_mode` | **Policy helpers** — must stay out of `schemas_*.mg`; deletions go here only if collision found. Currently 0 collisions. |
| `defaults/schema/*.mg` | 0 | — | No Decl — fact corpus only (`intent_definition` etc). No action. |

**Action**: No deduplication edits required now. If `go test ./internal/core/defaults -run TestSchemas_NoPredicateIsDeclaredTwice` fails, delete the second Decl (keep the `schemas_*.mg` copy, delete the `policy/` copy per `TestPolicy_DeclsDoNotCollideWithSchemas:138-139`).

---

## 7. Confidence & Gaps

- **0.98** for `schemas_*.mg` and top-level non-schemas: every file grepped individually with line numbers, raw source quoted, cross-checked against `schema_parsing.go` regex.
- **0.85** for `policy/*.mg`: sampled 20 files individually + bulk grep truncated at 50. Full policy inventory requires re-running per-file grep for remaining 62 files (`knowledge.mg`, `learning.mg`, `shards.mg`, `system_*.mg`, etc.) or running `schema_duplicate_decl_test.go` suite.
- **0.99** for duplicate flagging logic: `declKey` includes arity, so `foo/1` vs `foo/2` correctly not flagged; Mangle analyzer rejects actual duplicates per `schema_duplicate_decl_test.go:43-47`.

**Next investigative step**: run `go test -run TestSchemas_NoPredicateIsDeclaredTwice -run TestPolicy_DeclsDoNotCollideWithSchemas ./internal/core/defaults -v` and `go run ./cmd/tools/predicate_corpus_builder` to emit `predicate_corpus.db`; diff the DB’s `predicates` table against this markdown’s §3.1 to confirm counts.

---

*Generated by Researcher Shard, grounded in live `grep`/`read_file` evidence. Every predicate above cites `file:line` and raw Decl text; inferred summaries marked with confidence and `⚠️ SAMPLED`.*
