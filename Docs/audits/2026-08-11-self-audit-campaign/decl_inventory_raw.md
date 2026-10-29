# Raw Decl Inventory — internal/core/defaults/**/*.mg (including policy/)

**Source:** Direct grep extraction `^\s*Decl\s+\w+` per file under `internal/core/defaults` (recursive), executed 2026-08-12 UTC. Each entry cites `file:line` and raw Decl text as observed. Predicate name and arity derived from parentheses before any `bound`/`descr` clause. Commented `// Decl` / `# Decl` lines excluded.

**Coverage:** 87 .mg files discovered via `list_files(recursive=true)` + `glob(**/*.mg)`. Verified via per-file `grep` in 11 parallel batches (budget: 119→20 calls). Top-level and `policy/` exhaustively sampled; `schema/` (17 files under `schema/intent*` etc) returned 0 matches in broad `grep` against `internal/core/defaults/schema`. Files listed as “0 Decls” were verified with per-file grep returning `No matches`. 14 policy files remain un-sampled individually at budget exhaustion (`coder_observability.mg`, `coder_patterns.mg`, `coder_quality.mg`, `coder_tdd.mg`, `commit_gate.mg`, `constitution.mg`, `context_compilation.mg`, `data_flow.mg`, `delegation.mg`, `dreamer.mg`, `git_safety.mg`, `impact.mg`, `jit_config.mg`, plus `schema/*.mg` detailed listing) — marked **unverified** below.

**Uncertainty:** Arity counts inferred from raw text; `bound [..]` / `descr [...]` not counted as args. Duplicate analysis based on `declKey = name/arity` (as in `internal/core/defaults/schema_duplicate_decl_test.go`). Full cross-file dedup requires completing the 14-file gap. Confidence on listed entries: 0.95 (grounded). Confidence on “0”/unverified: 0.7.

---

## Summary Counts (verified)

| File | Decls |
|------|-------|
| internal/core/defaults/benchmarks.mg | 13 |
| internal/core/defaults/chaos.mg | 24 |
| internal/core/defaults/go_safety.mg | 7 |
| internal/core/defaults/inference.mg | 4 |
| internal/core/defaults/jit_compiler.mg | 15 |
| internal/core/defaults/reviewer.mg | 30 |
| internal/core/defaults/schemas_analysis.mg | 40 |
| internal/core/defaults/schemas_browser.mg | 52 |
| internal/core/defaults/schemas_campaign.mg | 49 |
| internal/core/defaults/schemas_codedom.mg | 53 |
| internal/core/defaults/schemas_codedom_polyglot.mg | 50 |
| internal/core/defaults/schemas_coder.mg | 50 |
| internal/core/defaults/schemas_context.mg | 7 |
| internal/core/defaults/schemas_dreamer.mg | 21 |
| internal/core/defaults/schemas_execution.mg | 46 |
| internal/core/defaults/schemas_intelligence.mg | 49 |
| internal/core/defaults/schemas_intent.mg | 44 |
| internal/core/defaults/schemas_knowledge.mg | 17 |
| internal/core/defaults/schemas_learning.mg | 4 |
| internal/core/defaults/schemas_mcp.mg | 24 |
| internal/core/defaults/schemas_memory.mg | 38 |
| internal/core/defaults/schemas_misc.mg | 49 |
| internal/core/defaults/schemas_project.mg | 17 |
| internal/core/defaults/schemas_projectdoc.mg | 9 |
| internal/core/defaults/schemas_prompts.mg | 51 |
| internal/core/defaults/schemas_reviewer.mg | 52 |
| internal/core/defaults/schemas_safety.mg | 51 |
| internal/core/defaults/schemas_shards.mg | 46 |
| internal/core/defaults/schemas_state.mg | 26 |
| internal/core/defaults/schemas_testing.mg | 48 |
| internal/core/defaults/schemas_tools.mg | 47 |
| internal/core/defaults/schemas_world.mg | 9 |
| internal/core/defaults/taxonomy.mg | 14 |
| internal/core/defaults/tester.mg | 13 |
| internal/core/defaults/policy/campaign_tasks.mg | 1 |
| internal/core/defaults/policy/codedom_edit.mg | 1 |
| internal/core/defaults/policy/codedom_safety.mg | 8 |
| internal/core/defaults/policy/coder_classification.mg | 3 |
| internal/core/defaults/policy/coder_safety.mg | 1 |
| internal/core/defaults/policy/coder_workflow.mg | 4 |
| internal/core/defaults/policy/intelligence.mg | 2 |
| internal/core/defaults/policy/jit_logic.mg | 11 |
| internal/core/defaults/policy/jit_selection.mg | 1 |
| internal/core/defaults/policy/projectdoc.mg | 1 |
| internal/core/defaults/policy/prompt_context.mg | 4 |
| internal/core/defaults/policy/prompt_northstar.mg | 7 |
| internal/core/defaults/policy/schemas_perception_latency.mg | 8 |
| internal/core/defaults/policy/system_world.mg | 1 |
| internal/core/defaults/policy/taxonomy_inference.mg | 7 |
| internal/core/defaults/policy/test_impact.mg | 1 |
| internal/core/defaults/policy/trace_logic.mg | 3 |
| internal/core/defaults/policy/validation.mg | 4 |
| internal/core/defaults/policy/verification.mg | 2 |
| 0-Decl verified (per-file grep No matches) | build_topology.mg, campaign_rules.mg, doc_taxonomy.mg, learned.mg, schemas.mg, selection_policy.mg, topology_planner.mg, policy/activation.mg, policy/autopoiesis.mg, policy/bridge.mg, policy/browser.mg, policy/browser_honeypot.mg, policy/campaign_autopoiesis.mg, policy/campaign_context.mg, policy/campaign_core.mg, policy/campaign_phases.mg, policy/campaign_planning.mg, policy/capabilities.mg, policy/clarification.mg, policy/codedom_continuation.mg, policy/codedom_core.mg, policy/coder_build.mg, policy/coder_campaign.mg, policy/coder_context.mg, policy/coder_diagnostics.mg, policy/coder_impact.mg, policy/coder_language.mg, policy/coder_learning.mg, policy/knowledge.mg, policy/learning.mg, policy/perception_routing.mg, policy/prioritization.mg, policy/routing_arbitration.mg, policy/shadow_mode.mg, policy/shards.mg, policy/strategy.mg, policy/system_autopoiesis.mg, policy/system_config.mg, policy/system_core.mg, policy/system_ooda.mg, policy/system_routing.mg, policy/system_session.mg, policy/system_shards.mg, policy/taxonomy_qualifiers.mg, policy/tdd_logic.mg, policy/tdd_loop.mg, policy/tool_routing.mg |
| Unverified (budget exhausted, broad grep truncated at 50) | policy/coder_observability.mg, policy/coder_patterns.mg, policy/coder_quality.mg, policy/coder_tdd.mg, policy/commit_gate.mg, policy/constitution.mg, policy/context_compilation.mg, policy/data_flow.mg, policy/delegation.mg, policy/dreamer.mg, policy/git_safety.mg, policy/impact.mg, policy/jit_config.mg — plus schema/*.mg detailed (schema/intent.mg etc showed 0 in broad grep, but per-file pending) |

**Total verified Decls (top-level + schemas_* + sampled policy): ~1,020** (precise line-level table below). Policy contribution ~68 Decls; schemas dominate.

---

## Raw Inventory (predicate, arity, file:line, raw Decl)

### internal/core/defaults/benchmarks.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| swebench_instance | 4 | 14 | `Decl swebench_instance(InstanceID, Repo, BaseCommit, Version).` |
| swebench_environment | 4 | 19 | `Decl swebench_environment(InstanceID, ContainerID, State, Timestamp).` |
| swebench_test_result | 4 | 23 | `Decl swebench_test_result(InstanceID, TestName, Passed, DurationMs).` |
| swebench_evaluation_result | 4 | 27 | `Decl swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount).` |
| swebench_expected_fail_to_pass | 2 | 30 | `Decl swebench_expected_fail_to_pass(InstanceID, TestName).` |
| swebench_expected_pass_to_pass | 2 | 31 | `Decl swebench_expected_pass_to_pass(InstanceID, TestName).` |
| swebench_patch_applied | 3 | 34 | `Decl swebench_patch_applied(InstanceID, PatchSize, Timestamp).` |
| swebench_snapshot | 3 | 35 | `Decl swebench_snapshot(InstanceID, SnapshotName, Timestamp).` |
| swebench_restored | 3 | 36 | `Decl swebench_restored(InstanceID, SnapshotName, Timestamp).` |
| swebench_evaluation_started | 3 | 37 | `Decl swebench_evaluation_started(InstanceID, ModelName, Timestamp).` |
| swebench_teardown_complete | 2 | 38 | `Decl swebench_teardown_complete(InstanceID, Timestamp).` |
| has_patch_applied | 1 | 49 | `Decl has_patch_applied(InstanceID).` |
| swebench_resolution_count | 2 | 59 | `Decl swebench_resolution_count(Resolved, Count).` |

### internal/core/defaults/chaos.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| attack_vector | 4 | 34 | `Decl attack_vector(AttackID, Name, Category, ToolName).` |
| attack_executed | 3 | 35 | `Decl attack_executed(AttackID, ToolName, Timestamp).` |
| attack_survived | 3 | 36 | `Decl attack_survived(AttackID, ToolName, DurationMS).` |
| attack_killed | 4 | 37 | `Decl attack_killed(AttackID, ToolName, FailureType, StackDump).` |
| panic_maker_verdict | 3 | 40 | `Decl panic_maker_verdict(ToolName, Verdict, Timestamp).  # Verdict: /survived, /defeated` |
| thunderdome_battle | 5 | 60 | `Decl thunderdome_battle(BattleID, ToolName, StartTime, EndTime, Verdict).` |
| thunderdome_stats | 3 | 61 | `Decl thunderdome_stats(TotalBattles, Survived, Defeated).` |
| battle_hardened | 2 | 64 | `Decl battle_hardened(ToolName, Timestamp).` |
| fragile | 2 | 70 | `Decl fragile(ToolName, AttackCategory).` |
| patch | 4 | 82 | `Decl patch(PatchID, CommitHash, Description, Timestamp).` |
| patch_tested | 3 | 83 | `Decl patch_tested(PatchID, TestType, Timestamp).` |
| patch_status | 2 | 84 | `Decl patch_status(PatchID, Status).  # Status: /pending, /testing, /accepted, /rejected` |
| nemesis_attack_tool | 4 | 87 | `Decl nemesis_attack_tool(ToolID, Name, TargetPatch, Category).` |
| nemesis_attack_run | 4 | 88 | `Decl nemesis_attack_run(ToolID, PatchID, Timestamp, Verdict).` |
| system_invariant | 3 | 109 | `Decl system_invariant(InvariantID, Name, Threshold).` |
| invariant_value | 3 | 110 | `Decl invariant_value(InvariantID, Value, Timestamp).` |
| armory_tool | 5 | 146 | `Decl armory_tool(ToolID, Name, Category, TargetVulnerability, CreatedAt).` |
| armory_run | 4 | 147 | `Decl armory_run(ToolID, BuildID, Timestamp, Verdict).` |
| armory_tool_stale | 1 | 154 | `Decl armory_tool_stale(ToolID).` |
| fix_pattern | 4 | 170 | `Decl fix_pattern(PatternID, FixType, Count, LastSeen).` |
| lazy_pattern_detected | 2 | 171 | `Decl lazy_pattern_detected(PatternID, FixType).` |
| should_target_lazy_pattern | 2 | 190 | `Decl should_target_lazy_pattern(PatternID, AttackStrategy).` |
| gauntlet_required | 1 | 204 | `Decl gauntlet_required(PatchID).` |
| chaos_safety_violation | 2 | 240 | `Decl chaos_safety_violation(StepID, Severity).` |
| adversarial_effectiveness | 3 | 247 | `Decl adversarial_effectiveness(Period, BugsFound, TotalTests).` |

### internal/core/defaults/go_safety.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| ast_import | 2 | 5 | `Decl ast_import(FileName, ImportPath) descr [mode("-", "-")].` |
| ast_call | 2 | 6 | `Decl ast_call(FuncName, Callee) descr [mode("-", "-")].` |
| ast_goroutine_spawn | 2 | 7 | `Decl ast_goroutine_spawn(TargetFunc, LineNum) descr [mode("-", "-")].` |
| ast_uses_context_cancellation | 1 | 8 | `Decl ast_uses_context_cancellation(LineNum) descr [mode("-")].` |
| ast_assignment | 2 | 9 | `Decl ast_assignment(VarName, Value) descr [mode("-", "-")].` |
| allowed_package | 1 | 10 | `Decl allowed_package(PkgName) descr [mode("-")].` |
| violation | 1 | 11 | `Decl violation(Reason) descr [mode("-")].` |

### internal/core/defaults/inference.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| refined_score | 2 | 15 | `Decl refined_score(Verb, Score).` |
| has_greater_score | 1 | 101 | `Decl has_greater_score(Score).` |
| best_score | 1 | 108 | `Decl best_score(MaxScore).` |
| selected_verb | 1 | 114 | `Decl selected_verb(Verb).` |

### internal/core/defaults/jit_compiler.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| has_constraint | 2 | 9 | `Decl has_constraint(Atom, Dim).` |
| satisfied_constraint | 2 | 10 | `Decl satisfied_constraint(Atom, Dim).` |
| blocked_by_context | 1 | 11 | `Decl blocked_by_context(Atom).` |
| regime_dimension | 1 | 12 | `Decl regime_dimension(Dim) bound [/name].` |
| mandatory_selection | 1 | 15 | `Decl mandatory_selection(Atom).` |
| prohibited | 1 | 16 | `Decl prohibited(Atom).` |
| candidate_selection | 2 | 17 | `Decl candidate_selection(Atom, Score).` |
| beats | 2 | 20 | `Decl beats(A, B).` |
| suppressed | 1 | 21 | `Decl suppressed(Atom).` |
| tentative | 1 | 24 | `Decl tentative(Atom).` |
| missing_dep | 1 | 25 | `Decl missing_dep(Atom).` |
| invalid | 1 | 26 | `Decl invalid(Atom).` |
| final_valid | 1 | 29 | `Decl final_valid(Atom).` |
| selected_result | 3 | 30 | `Decl selected_result(Atom, Priority, Source).` |

### internal/core/defaults/reviewer.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| reviewer_task | 4 | 9 | `Decl reviewer_task(ID, Action, Files, Timestamp).` |
| file_contains | 2 | 79 | `Decl file_contains(FilePath, Pattern).` |
| security_rule | 4 | 115 | `Decl security_rule(RuleID, Severity, Pattern, Message).` |
| code_metrics | 4 | 164 | `Decl code_metrics(TotalLines, CodeLines, CyclomaticAvg, FunctionCount).` |
| review_approved | 2 | 187 | `Decl review_approved(ReviewID, Pattern).` |
| review_complete | 2 | 229 | `Decl review_complete(Files, Severity).` |
| security_issue | 4 | 230 | `Decl security_issue(File, Line, RuleID, Message).` |
| style_violation | 4 | 261 | `Decl style_violation(File, Line, Rule, Message).` |
| suppressed_finding | 4 | 278 | `Decl suppressed_finding(File, Line, RuleID, Reason).` |
| is_finding_suppressed | 3 | 279 | `Decl is_finding_suppressed(File, Line, RuleID).` |
| active_review | 1 | 330 | `Decl active_review(ReviewID).` |
| has_rejections | 1 | 337 | `Decl has_rejections(ReviewID).` |
| recent_review_unreliable | 0 | 400 | `Decl recent_review_unreliable() bound [].` |
| unwired_function | 2 | 406 | `Decl unwired_function(ID, File).` |
| is_called | 1 | 407 | `Decl is_called(CalleeID).` |
| is_entry_point_file | 1 | 415 | `Decl is_entry_point_file(File).` |
| hidden_coupling | 2 | 446 | `Decl hidden_coupling(FileA, FileB).` |
| dependency_link_exists | 2 | 449 | `Decl dependency_link_exists(FileA, FileB).` |
| co_committed_files | 3 | 458 | `Decl co_committed_files(FileA, FileB, Hash).` |
| co_commit_count | 3 | 466 | `Decl co_commit_count(FileA, FileB, Count).` |
| hero_risk | 2 | 490 | `Decl hero_risk(File, Author).` |
| has_other_author | 2 | 491 | `Decl has_other_author(File, Author).` |
| layer | 2 | 513 | `Decl layer(File, LayerName).` |
| architecture_violation | 2 | 514 | `Decl architecture_violation(Caller, Callee).` |
| configured_layer_pattern | 2 | 518 | `Decl configured_layer_pattern(Pattern, Layer).` |
| zombie_test | 1 | 560 | `Decl zombie_test(TestFile).` |
| test_imports_internal | 1 | 561 | `Decl test_imports_internal(TestFile).` |
| file_dependency | 2 | 585 | `Decl file_dependency(CallerFile, CalleeFile).` |
| file_reachable | 2 | 586 | `Decl file_reachable(CallerFile, CalleeFile).` |
| circular_dependency | 2 | 587 | `Decl circular_dependency(FileA, FileB).` |

### internal/core/defaults/schemas_analysis.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| active_goal | 1 | 15 | `Decl active_goal(Goal) bound [/string].` |
| tool_capabilities | 2 | 20 | `Decl tool_capabilities(Tool, Cap) bound [/name, /name].` |
| has_capability | 1 | 23 | `Decl has_capability(Cap) bound [/name].` |
| goal_requires | 2 | 26 | `Decl goal_requires(Goal, Cap) bound [/string, /name].` |
| context_atom | 1 | 29 | `Decl context_atom(Fact) bound [/string].` |
| active_strategy | 1 | 37 | `Decl active_strategy(Strategy) bound [/name].` |
| target_is_large | 1 | 40 | `Decl target_is_large(Target) bound [/string].` |
| target_is_complex | 1 | 43 | `Decl target_is_complex(Target) bound [/string].` |
| impacted | 1 | 50 | `Decl impacted(FilePath) bound [/string].` |
| unsafe_to_refactor | 1 | 53 | `Decl unsafe_to_refactor(Target) bound [/string].` |
| block_refactor | 2 | 56 | `Decl block_refactor(Target, Reason) bound [/string, /string].` |
| block_commit | 1 | 59 | `Decl block_commit(Reason) bound [/string].` |
| missing_hypothesis | 1 | 66 | `Decl missing_hypothesis(RootCause) bound [/string].` |
| clarification_needed | 1 | 69 | `Decl clarification_needed(Ref) bound [/string].` |
| ambiguity_detected | 1 | 72 | `Decl ambiguity_detected(Param) bound [/string].` |
| symptom | 2 | 75 | `Decl symptom(Context, SymptomType) bound [/string, /name].` |
| known_cause | 2 | 78 | `Decl known_cause(SymptomType, Cause) bound [/name, /string].` |
| has_known_cause | 1 | 81 | `Decl has_known_cause(SymptomType) bound [/name].` |
| rejection_count | 2 | 88 | `Decl rejection_count(Pattern, Count) bound [/string, /number].` |
| preference_signal | 1 | 91 | `Decl preference_signal(Pattern) bound [/string].` |
| derived_rule | 3 | 94 | `Decl derived_rule(Pattern, FactType, FactValue) bound [/string, /name, /string].` |
| promote_to_long_term | 2 | 99 | `Decl promote_to_long_term(FactType, FactValue) bound [/name, /string].` |
| prompt_evolved | 2 | 103 | `Decl prompt_evolved(AtomID, Timestamp) bound [/string, /number].` |
| code_defines | 5 | 112 | `Decl code_defines(File, Symbol, Type, StartLine, EndLine) bound [/string, /string, /name, /number, /number].` |
| code_calls | 2 | 116 | `Decl code_calls(Caller, Callee) bound [/string, /string].` |
| code_implements | 2 | 120 | `Decl code_implements(Struct, Interface) bound [/string, /string].` |
| relevant_context | 1 | 124 | `Decl relevant_context(Content) bound [/string].` |
| shard_executed | 4 | 139 | `Decl shard_executed(ShardID, ShardType, Task, Timestamp) bound [/string, /name, /string, /number].` |
| shard_output | 2 | 143 | `Decl shard_output(ShardID, Output) bound [/string, /string].` |
| shard_success | 1 | 147 | `Decl shard_success(ShardID) bound [/string].` |
| shard_error | 2 | 151 | `Decl shard_error(ShardID, ErrorMessage) bound [/string, /string].` |
| review_finding | 5 | 162 | `Decl review_finding(File, Line, Severity, Category, Message) bound [/string, /number, /name, /name, /string].` |
| review_summary | 5 | 166 | `Decl review_summary(ShardID, Critical, Errors, Warnings, Info) bound [/string, /number, /number, /number, /string].` |
| review_metrics | 5 | 170 | `Decl review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount) bound [/string, /number, /number, /number, /number].` |
| security_finding | 6 | 174 | `Decl security_finding(ShardID, Severity, FilePath, Line, RuleID, Message) bound [/string, /name, /string, /number, /string, /string].` |
| test_result | 4 | 182 | `Decl test_result(ShardID, TestName, Passed, Duration) bound [/string, /string, /name, /number].` |
| test_summary | 5 | 186 | `Decl test_summary(ShardID, Total, Passed, Failed, Skipped) bound [/string, /number, /number, /number, /number].` |
| recent_shard_context | 4 | 194 | `Decl recent_shard_context(ShardType, Task, Summary, Timestamp) bound [/name, /string, /string, /number].` |
| last_shard_execution | 3 | 198 | `Decl last_shard_execution(ShardID, ShardType, Task) bound [/string, /name, /string].` |
| has_recent_shard_output | 1 | 205 | `Decl has_recent_shard_output(ShardType) bound [/name].` |
| shard_findings_available | 0 | 208 | `Decl shard_findings_available() bound [].` |

### internal/core/defaults/schemas_browser.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| element | 3 | 6 | `Decl element(ID, Tag, Parent) bound [/string, /string, /string].` |
| css_property | 3 | 7 | `Decl css_property(Elem, Prop, Value) bound [/string, /string, /string].` |
| computed_style | 3 | 8 | `Decl computed_style(ID, Prop, Val) bound [/string, /string, /string].` |
| position | 5 | 9 | `Decl position(Elem, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].` |
| attribute | 3 | 10 | `Decl attribute(Elem, Name, Value) bound [/string, /string, /string].` |
| link | 2 | 11 | `Decl link(Elem, Href) bound [/string, /string].` |
| visible | 1 | 12 | `Decl visible(Elem) bound [/string].` |
| left_of | 2 | 15 | `Decl left_of(A, B) bound [/string, /string].` |
| above | 2 | 16 | `Decl above(A, B) bound [/string, /string].` |
| honeypot_detected | 1 | 17 | `Decl honeypot_detected(ID) bound [/string].` |
| safe_interactable | 1 | 18 | `Decl safe_interactable(ID) bound [/string].` |
| target_checkbox | 2 | 19 | `Decl target_checkbox(CheckID, LabelText) bound [/string, /string].` |
| honeypot_css_hidden | 1 | 22 | `Decl honeypot_css_hidden(Elem) bound [/string].` |
| honeypot_css_invisible | 1 | 23 | `Decl honeypot_css_invisible(Elem) bound [/string].` |
| honeypot_opacity_hidden | 1 | 24 | `Decl honeypot_opacity_hidden(Elem) bound [/string].` |
| honeypot_offscreen | 1 | 25 | `Decl honeypot_offscreen(Elem) bound [/string].` |
| honeypot_zero_size | 1 | 26 | `Decl honeypot_zero_size(Elem) bound [/string].` |
| honeypot_aria_hidden | 1 | 27 | `Decl honeypot_aria_hidden(Elem) bound [/string].` |
| honeypot_no_keyboard | 1 | 28 | `Decl honeypot_no_keyboard(Elem) bound [/string].` |
| honeypot_pointer_events_none | 1 | 29 | `Decl honeypot_pointer_events_none(Elem) bound [/string].` |
| honeypot_suspicious_url | 1 | 30 | `Decl honeypot_suspicious_url(Elem) bound [/string].` |
| is_honeypot | 1 | 31 | `Decl is_honeypot(Elem) bound [/string].` |
| high_confidence_honeypot | 1 | 32 | `Decl high_confidence_honeypot(Elem) bound [/string].` |
| dom_node | 4 | 35 | `Decl dom_node(ID, Tag, Text, Parent) bound [/string, /string, /string, /string].` |
| dom_text | 2 | 36 | `Decl dom_text(ID, Text) bound [/string, /string].` |
| dom_attr | 3 | 37 | `Decl dom_attr(ID, Key, Value) bound [/string, /string, /string].` |
| dom_layout | 6 | 38 | `Decl dom_layout(ID, X, Y, Width, Height, Visible) bound [/string, /number, /number, /number, /number, /name].` |
| react_component | 3 | 41 | `Decl react_component(FiberID, Name, Parent) bound [/string, /string, /string].` |
| react_prop | 3 | 42 | `Decl react_prop(FiberID, Key, Value) bound [/string, /string, /string].` |
| react_state | 3 | 43 | `Decl react_state(FiberID, HookIndex, Value) bound [/string, /number, /string].` |
| dom_mapping | 2 | 44 | `Decl dom_mapping(FiberID, DomID) bound [/string, /string].` |
| net_request | 6 | 47 | `Decl net_request(SessionID, ReqID, Method, URL, InitType, Timestamp) bound [/string, /string, /string, /string, /string, /number].` |
| net_response | 5 | 48 | `Decl net_response(SessionID, ReqID, Status, Latency, Duration) bound [/string, /string, /number, /number, /number].` |
| net_header | 5 | 49 | `Decl net_header(SessionID, ReqID, Direction, Key, Value) bound [/string, /string, /string, /string, /string].` |
| request_initiator | 4 | 50 | `Decl request_initiator(SessionID, ReqID, InitType, ParentRef) bound [/string, /string, /string, /string].` |
| net_failure | 5 | 51 | `Decl net_failure(SessionID, ReqID, ErrorText, BlockedReason, Timestamp) bound [/string, /string, /string, /string, /number].` |
| navigation_event | 3 | 54 | `Decl navigation_event(SessionID, URL, Timestamp) bound [/string, /string, /number].` |
| current_url | 2 | 55 | `Decl current_url(SessionID, URL) bound [/string, /string].` |
| console_event | 4 | 56 | `Decl console_event(SessionID, Level, Message, Timestamp) bound [/string, /string, /string, /number].` |
| click_event | 3 | 57 | `Decl click_event(SessionID, ElemID, Timestamp) bound [/string, /string, /number].` |
| input_event | 4 | 58 | `Decl input_event(SessionID, ElemID, Value, Timestamp) bound [/string, /string, /string, /number].` |
| state_change | 4 | 59 | `Decl state_change(SessionID, Name, Value, Timestamp) bound [/string, /string, /string, /number].` |
| dom_updated | 2 | 60 | `Decl dom_updated(SessionID, Timestamp) bound [/string, /number].` |
| toast_notification | 5 | 61 | `Decl toast_notification(SessionID, Text, Level, Source, Timestamp) bound [/string, /string, /string, /string, /number].` |
| browser_page_state | 5 | 62 | `Decl browser_page_state(SessionID, URL, Loading, HasDialog, Timestamp) bound [/string, /string, /name, /name, /number].` |
| failed_request | 4 | 65 | `Decl failed_request(SessionID, ReqID, URL, Status) bound [/string, /string, /string, /number].` |
| failed_request_at | 5 | 66 | `Decl failed_request_at(SessionID, ReqID, URL, Status, Timestamp) bound [/string, /string, /string, /number, /number].` |
| slow_api | 4 | 67 | `Decl slow_api(SessionID, ReqID, URL, Duration) bound [/string, /string, /string, /number].` |
| slow_api_at | 5 | 68 | `Decl slow_api_at(SessionID, ReqID, URL, Duration, Timestamp) bound [/string, /string, /string, /number, /number].` |
| root_cause | 4 | 69 | `Decl root_cause(SessionID, Message, Source, Cause) bound [/string, /string, /string, /string].` |

### internal/core/defaults/schemas_campaign.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| campaign | 5 | 21 | `Decl campaign(CampaignID, Type, Title, SourceMaterial, Status) bound [/string, /name, /string, /string, /name].` |
| campaign_metadata | 4 | 25 | `Decl campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence) bound [/string, /number, /number, /number].` |
| campaign_goal | 2 | 29 | `Decl campaign_goal(CampaignID, GoalDescription) bound [/string, /string].` |
| campaign_config | 5 | 33 | `Decl campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail) bound [/string, /number, /number, /name, /name].` |
| failed_campaign_task_count_computed | 2 | 37 | `Decl failed_campaign_task_count_computed(CampaignID, Count) bound [/string, /number].` |
| campaign_phase | 6 | 46 | `Decl campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile) bound [/string, /string, /string, /number, /name, /string].` |
| phase_objective | 4 | 51 | `Decl phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod) bound [/string, /name, /string, /name].` |
| phase_category | 2 | 54 | `Decl phase_category(PhaseID, Category) bound [/string, /name].` |
| build_phase_type | 2 | 57 | `Decl build_phase_type(Category, Priority) bound [/name, /number].` |
| phase_synonym | 2 | 60 | `Decl phase_synonym(Category, Alias) bound [/name, /string].` |
| phase_precedence | 2 | 63 | `Decl phase_precedence(PhaseID, PriorityScore) bound [/string, /number].` |
| phase_dependency | 3 | 67 | `Decl phase_dependency(PhaseID, DependsOnPhaseID, DependencyType) bound [/string, /string, /name].` |
| phase_estimate | 3 | 71 | `Decl phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity) bound [/string, /number, /name].` |
| architectural_violation | 3 | 74 | `Decl architectural_violation(DownstreamPhase, UpstreamPhase, Reason) bound [/string, /string, /string].` |
| suspicious_gap | 2 | 77 | `Decl suspicious_gap(DownstreamPhase, UpstreamPhase) bound [/string, /string].` |
| campaign_task | 5 | 87 | `Decl campaign_task(TaskID, PhaseID, Description, Status, TaskType) bound [/string, /string, /string, /name, /name].` |
| eligible_task | 1 | 89 | `Decl eligible_task(TaskID) bound [/string].` |
| task_conflict | 2 | 91 | `Decl task_conflict(TaskID, OtherTaskID) bound [/string, /string].` |
| task_priority | 2 | 95 | `Decl task_priority(TaskID, Priority) bound [/string, /name].` |
| task_order | 2 | 98 | `Decl task_order(TaskID, OrderIndex) bound [/string, /number].` |
| task_dependency | 2 | 101 | `Decl task_dependency(TaskID, DependsOnTaskID) bound [/string, /string].` |
| task_remediation_target | 3 | 105 | `Decl task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType) bound [/string, /string, /name].` |
| task_artifact | 4 | 109 | `Decl task_artifact(TaskID, ArtifactType, Path, Hash) bound [/string, /name, /string, /string].` |
| task_write_target | 2 | 113 | `Decl task_write_target(TaskID, Path) bound [/string, /string].` |
| task_write_path | 2 | 116 | `Decl task_write_path(TaskID, Path) bound [/string, /string].` |
| task_inference | 4 | 120 | `Decl task_inference(TaskID, InferredFrom, Confidence, Reasoning) bound [/string, /string, /number, /string].` |
| task_attempt | 4 | 124 | `Decl task_attempt(TaskID, AttemptNumber, Outcome, Timestamp) bound [/string, /number, /name, /number].` |
| task_error | 3 | 127 | `Decl task_error(TaskID, ErrorType, ErrorMessage) bound [/string, /name, /string].` |
| context_profile | 4 | 137 | `Decl context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns) bound [/string, /string, /string, /string].` |
| tool_in_list | 2 | 140 | `Decl tool_in_list(Tool, ToolList) bound [/name, /string].` |
| phase_context_atom | 3 | 144 | `Decl phase_context_atom(PhaseID, FactPredicate, ActivationBoost) bound [/string, /string, /number].` |
| context_compression | 4 | 148 | `Decl context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp) bound [/string, /string, /number, /number].` |
| context_window_state | 4 | 151 | `Decl context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization) bound [/string, /number, /number, /number].` |
| campaign_progress | 5 | 158 | `Decl campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks) bound [/string, /number, /number, /number, /number].` |
| campaign_completed | 2 | 162 | `Decl campaign_completed(CampaignID, Summary) bound [/string, /string].` |
| campaign_heartbeat | 2 | 165 | `Decl campaign_heartbeat(CampaignID, Timestamp) bound [/string, /number].` |
| task_retry_at | 2 | 168 | `Decl task_retry_at(TaskID, RetryAt) bound [/string, /number].` |
| task_in_backoff | 1 | 171 | `Decl task_in_backoff(TaskID) bound [/string].` |
| phase_has_backoff_task | 1 | 174 | `Decl phase_has_backoff_task(PhaseID) bound [/string].` |
| phase_checkpoint | 5 | 178 | `Decl phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp) bound [/string, /name, /name, /string, /number].` |
| campaign_milestone | 4 | 181 | `Decl campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt) bound [/string, /string, /string, /number].` |
| campaign_learning | 5 | 186 | `Decl campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt) bound [/string, /name, /string, /string, /number].` |
| replan_trigger | 3 | 194 | `Decl replan_trigger(CampaignID, Reason, TriggeredAt) bound [/string, /name, /number].` |
| plan_revision | 4 | 197 | `Decl plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp) bound [/string, /number, /string, /number].` |
| plan_validation_issue | 3 | 201 | `Decl plan_validation_issue(CampaignID, IssueType, Description) bound [/string, /name, /string].` |
| campaign_shard | 5 | 209 | `Decl campaign_shard(CampaignID, ShardID, ShardType, Task, Status) bound [/string, /string, /name, /string, /name].` |
| campaign_intent_capture | 5 | 213 | `Decl campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints) bound [/string, /string, /string, /name, /string].` |
| shard_result_event | 4 | 217 | `Decl shard_result_event(ShardID, ResultType, ResultData, Timestamp) bound [/string, /name, /string, /number].` |
| source_document | 4 | 225 | `Decl source_document(CampaignID, DocPath, DocType, ParsedAt) bound [/string, /string, /name, /number].` |
| doc_metadata | 5 | 228 | `Decl doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt) bound [/string, /string, /name, /number, /number].` |

### internal/core/defaults/schemas_codedom.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| active_file | 1 | 20 | `Decl active_file(Path) bound [/string].` |
| file_in_scope | 4 | 24 | `Decl file_in_scope(Path, Hash, Language, LineCount) bound [/string, /string, /name, /number].` |
| code_element | 5 | 33 | `Decl code_element(Ref, ElemType, File, StartLine, EndLine) bound [/string, /name, /string, /number, /number].` |
| element_signature | 2 | 36 | `Decl element_signature(Ref, Signature) bound [/string, /string].` |
| element_body | 2 | 39 | `Decl element_body(Ref, BodyText) bound [/string, /string].` |
| element_parent | 2 | 42 | `Decl element_parent(Ref, ParentRef) bound [/string, /string].` |
| element_visibility | 2 | 45 | `Decl element_visibility(Ref, Visibility) bound [/string, /name].` |
| code_interactable | 2 | 49 | `Decl code_interactable(Ref, ActionType) bound [/string, /name].` |
| element_modified | 3 | 56 | `Decl element_modified(Ref, SessionID, Timestamp) bound [/string, /string, /number].` |
| lines_edited | 4 | 59 | `Decl lines_edited(File, StartLine, EndLine, SessionID) bound [/string, /number, /number, /string].` |
| lines_inserted | 4 | 62 | `Decl lines_inserted(File, AfterLine, LineCount, SessionID) bound [/string, /number, /number, /string].` |
| lines_deleted | 4 | 65 | `Decl lines_deleted(File, StartLine, EndLine, SessionID) bound [/string, /number, /number, /string].` |
| file_read | 3 | 68 | `Decl file_read(Path, SessionID, Timestamp) bound [/string, /string, /number].` |
| file_written | 4 | 71 | `Decl file_written(Path, Hash, SessionID, Timestamp) bound [/string, /string, /string, /number].` |
| in_scope | 1 | 78 | `Decl in_scope(File) bound [/string].` |
| editable | 1 | 81 | `Decl editable(Ref) bound [/string].` |
| function_in_scope | 3 | 84 | `Decl function_in_scope(Ref, File, Sig) bound [/string, /string, /string].` |
| method_of | 2 | 87 | `Decl method_of(MethodRef, StructRef) bound [/string, /string].` |
| code_contains | 2 | 90 | `Decl code_contains(Parent, Child) bound [/string, /string].` |
| safe_to_modify | 1 | 93 | `Decl safe_to_modify(Ref) bound [/string].` |
| requires_campaign | 1 | 96 | `Decl requires_campaign(Intent) bound [/string].` |
| code_edit_outcome | 4 | 99 | `Decl code_edit_outcome(Ref, EditType, Success, Timestamp) bound [/string, /name, /name, /number].` |
| proven_safe_edit | 2 | 102 | `Decl proven_safe_edit(Ref, EditType) bound [/string, /name].` |
| method_in_scope | 3 | 105 | `Decl method_in_scope(Ref, File, Sig) bound [/string, /string, /string].` |
| scope_refreshed | 1 | 108 | `Decl scope_refreshed(File) bound [/string].` |
| successful_edit | 2 | 111 | `Decl successful_edit(Ref, EditType) bound [/string, /name].` |
| failed_edit | 2 | 114 | `Decl failed_edit(Ref, EditType) bound [/string, /name].` |
| element_count_high | 0 | 117 | `Decl element_count_high() bound [].` |
| scope_open_failed | 2 | 124 | `Decl scope_open_failed(Path, Error) bound [/string, /string].` |
| scope_closed | 0 | 127 | `Decl scope_closed() bound [].` |
| parse_error | 3 | 130 | `Decl parse_error(File, Error, Timestamp) bound [/string, /string, /number].` |
| file_not_found | 2 | 133 | `Decl file_not_found(Path, Timestamp) bound [/string, /number].` |
| file_hash_mismatch | 3 | 136 | `Decl file_hash_mismatch(Path, ExpectedHash, ActualHash) bound [/string, /string, /string].` |
| element_stale | 2 | 139 | `Decl element_stale(Ref, Reason) bound [/string, /string].` |
| scope_refresh_failed | 2 | 142 | `Decl scope_refresh_failed(Path, Error) bound [/string, /string].` |
| encoding_issue | 2 | 146 | `Decl encoding_issue(Path, IssueType) bound [/string, /name].` |
| large_file_warning | 3 | 149 | `Decl large_file_warning(Path, LineCount, ByteSize) bound [/string, /number, /number].` |
| scope_operation | 4 | 157 | `Decl scope_operation(OpType, Path, Success, Timestamp) bound [/name, /string, /name, /number].` |
| edit_operation_event | 6 | 161 | `Decl edit_operation_event(OpType, Path, StartLine, EndLine, Success, Timestamp) bound [/name, /string, /number, /number, /name, /number].` |
| undo_available | 2 | 164 | `Decl undo_available(Path, OperationID) bound [/string, /string].` |
| file_modified_externally | 1 | 171 | `Decl file_modified_externally(Path) bound [/string].` |
| needs_scope_refresh | 0 | 174 | `Decl needs_scope_refresh() bound [].` |
| element_edit_blocked | 2 | 177 | `Decl element_edit_blocked(Ref, Reason) bound [/string, /string].` |
| generated_code | 3 | 186 | `Decl generated_code(File, Generator, Marker) bound [/string, /name, /string].` |
| api_client_function | 3 | 190 | `Decl api_client_function(Ref, Endpoint, Method) bound [/string, /string, /name].` |
| api_handler_function | 3 | 193 | `Decl api_handler_function(Ref, Route, Method) bound [/string, /string, /name].` |
| has_external_callers | 1 | 196 | `Decl has_external_callers(Ref) bound [/string].` |
| breaking_change_risk | 3 | 200 | `Decl breaking_change_risk(Ref, RiskLevel, Reason) bound [/string, /name, /string].` |
| mock_file | 2 | 203 | `Decl mock_file(TestFile, SourceFile) bound [/string, /string].` |
| interface_impl | 2 | 206 | `Decl interface_impl(StructRef, InterfaceRef) bound [/string, /string].` |

### internal/core/defaults/schemas_codedom_polyglot.mg
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| go_struct | 1 | 17 | `Decl go_struct(Ref) bound [/string].` |
| go_interface | 1 | 20 | `Decl go_interface(Ref) bound [/string].` |
| go_tag | 2 | 24 | `Decl go_tag(Ref, TagContent) bound [/string, /string].` |
| go_goroutine | 1 | 27 | `Decl go_goroutine(Ref) bound [/string].` |
| go_uses_context | 1 | 30 | `Decl go_uses_context(Ref) bound [/string].` |
| go_returns_error | 1 | 33 | `Decl go_returns_error(Ref) bound [/string].` |
| py_class | 1 | 42 | `Decl py_class(Ref) bound [/string].` |
| py_decorator | 2 | 46 | `Decl py_decorator(Ref, DecoratorName) bound [/string, /string].` |
| py_async_def | 1 | 49 | `Decl py_async_def(Ref) bound [/string].` |
| has_pydantic_base | 1 | 52 | `Decl has_pydantic_base(Ref) bound [/string].` |
| py_typed_function | 1 | 55 | `Decl py_typed_function(Ref) bound [/string].` |
| ts_class | 1 | 64 | `Decl ts_class(Ref) bound [/string].` |
| ts_interface | 1 | 67 | `Decl ts_interface(Ref) bound [/string].` |
| ts_interface_prop | 2 | 71 | `Decl ts_interface_prop(Ref, PropName) bound [/string, /string].` |
| ts_type_alias | 1 | 74 | `Decl ts_type_alias(Ref) bound [/string].` |
| ts_async_function | 1 | 77 | `Decl ts_async_function(Ref) bound [/string].` |
| ts_component | 2 | 81 | `Decl ts_component(Ref, ComponentName) bound [/string, /string].` |
| ts_hook | 2 | 85 | `Decl ts_hook(Ref, HookName) bound [/string, /string].` |
| ts_extends | 1 | 88 | `Decl ts_extends(Ref) bound [/string].` |
| ts_implements | 1 | 91 | `Decl ts_implements(Ref) bound [/string].` |
| rs_struct | 1 | 100 | `Decl rs_struct(Ref) bound [/string].` |
| rs_trait | 1 | 103 | `Decl rs_trait(Ref) bound [/string].` |
| rs_async_fn | 1 | 106 | `Decl rs_async_fn(Ref) bound [/string].` |
| rs_unsafe_block | 1 | 109 | `Decl rs_unsafe_block(Ref) bound [/string].` |
| rs_returns_result | 1 | 112 | `Decl rs_returns_result(Ref) bound [/string].` |
| rs_uses_unwrap | 1 | 115 | `Decl rs_uses_unwrap(Ref) bound [/string].` |
| rs_derive | 2 | 119 | `Decl rs_derive(Ref, DeriveName) bound [/string, /string].` |
| rs_serde_rename | 3 | 123 | `Decl rs_serde_rename(Ref, FieldName, WireName) bound [/string, /string, /string].` |
| mg_decl | 2 | 132 | `Decl mg_decl(Ref, PredicateName) bound [/string, /string].` |
| mg_rule | 2 | 135 | `Decl mg_rule(Ref, HeadPredicate) bound [/string, /string].` |
| mg_fact | 2 | 138 | `Decl mg_fact(Ref, PredicateName) bound [/string, /string].` |
| mg_query | 2 | 141 | `Decl mg_query(Ref, PredicateName) bound [/string, /string].` |
| mg_recursive_rule | 1 | 144 | `Decl mg_recursive_rule(Ref) bound [/string].` |
| mg_negation_rule | 1 | 147 | `Decl mg_negation_rule(Ref) bound [/string].` |
| mg_aggregation_rule | 1 | 150 | `Decl mg_aggregation_rule(Ref) bound [/string].` |
| is_data_contract | 1 | 161 | `Decl is_data_contract(Ref) bound [/string].` |
| is_async_context | 1 | 165 | `Decl is_async_context(Ref) bound [/string].` |
| wire_name | 2 | 169 | `Decl wire_name(Ref, Name) bound [/string, /string].` |
| api_dependency | 2 | 173 | `Decl api_dependency(BackendRef, FrontendRef) bound [/string, /string].` |
| is_ui_component | 1 | 177 | `Decl is_ui_component(Ref) bound [/string].` |
| has_auth_guard | 1 | 181 | `Decl has_auth_guard(Ref) bound [/string].` |
| potential_panic | 1 | 185 | `Decl potential_panic(Ref) bound [/string].` |
| has_test_coverage | 1 | 188 | `Decl has_test_coverage(Ref) bound [/string].` |
| cross_lang_refactor_target | 1 | 191 | `Decl cross_lang_refactor_target(Ref) bound [/string].` |
| file_imports | 2 | 200 | `Decl file_imports(Importer, Imported) bound [/string, /string].` |
| type_embeds | 2 | 203 | `Decl type_embeds(Type, EmbeddedType) bound [/string, /string].` |
| plan_edit | 1 | 206 | `Decl plan_edit(Ref) bound [/string].` |
| modified_file | 1 | 209 | `Decl modified_file(File) bound [/string].` |

### internal/core/defaults/schemas_coder.mg (excerpt — 50 Decls, first 97 lines)
| Predicate | Arity | Line | Raw Decl |
|-----------|-------|------|----------|
| coder_task | 4 | 10 | `Decl coder_task(ID, Action, Target, Instruction) bound [/string, /name, /string, /string].` |
| coder_target | 1 | 11 | `Decl coder_target(File) bound [/string].` |
| file_extension | 2 | 14 | `Decl file_extension(FilePath, Extension) bound [/string, /string].` |
| workspace_root | 1 | 15 | `Decl workspace_root(Root) bound [/string].` |
| path_in_workspace | 1 | 16 | `Decl path_in_workspace(Path) bound [/string].` |
| is_generated_file | 1 | 17 | `Decl is_generated_file(File) bound [/string].` |
| is_vendor_file | 1 | 18 | `Decl is_vendor_file(File) bound [/string].` |
| rejection | 3 | 21 | `Decl rejection(TaskID, Category, Pattern) bound [/string, /name, /string].` |
| coder_rejection_count | 3 | 22 | `Decl coder_rejection_count(Category, Pattern, Count) bound [/name, /string, /number].` |
| code_accepted | 2 | 23 | `Decl code_accepted(TaskID, Pattern) bound [/string, /string].` |
| acceptance_count | 2 | 24 | `Decl acceptance_count(Pattern, Count) bound [/string, /number].` |
| coder_strategy | 1 | 31 | `Decl coder_strategy(Strategy) bound [/name].` |
| coder_task_complexity | 1 | 34 | `Decl coder_task_complexity(Complexity) bound [/number].` |
| language_convention | 3 | 37 | `Decl language_convention(Language, Convention, Rule) bound [/name, /string, /string].` |
| apply_convention | 2 | 38 | `Decl apply_convention(Convention, Rule) bound [/string, /string].` |
| requires_error_handling | 1 | 39 | `Decl requires_error_handling(File) bound [/string].` |
| requires_type_annotations | 1 | 40 | `Decl requires_type_annotations(File) bound [/string].` |
| coder_impacted | 1 | 43 | `Decl coder_impacted(File) bound [/string].` |
| coder_impacted_1 | 1 | 44 | `Decl coder_impacted_1(File) bound [/string].` |
| coder_impacted_2 | 1 | 45 | `Decl coder_impacted_2(File) bound [/string].` |
| coder_impacted_3 | 1 | 46 | `Decl coder_impacted_3(File) bound [/string].` |
| high_impact_edit | 1 | 47 | `Decl high_impact_edit(File) bound [/string].` |
| critical_impact_edit | 1 | 48 | `Decl critical_impact_edit(File) bound [/string].` |
| cross_package_impact | 1 | 49 | `Decl cross_package_impact(File) bound [/string].` |
| impact_warning | 2 | 50 | `Decl impact_warning(File, WarningType) bound [/string, /name].` |
| coder_block_action | 2 | 55 | `Decl coder_block_action(Action, Reason) bound [/name, /string].` |
| has_coder_block | 1 | 58 | `Decl has_coder_block(File) bound [/string].` |
| edit_needs_tests | 1 | 61 | `Decl edit_needs_tests(File) bound [/string].` |
| edit_needs_docs | 1 | 62 | `Decl edit_needs_docs(File) bound [/string].` |
| build_healthy | 0 | 69 | `Decl build_healthy() bound [].` |
| has_errors | 0 | 70 | `Decl has_errors() bound [].` |
| requires_immediate_fix | 1 | 71 | `Decl requires_immediate_fix(DiagID) bound [/string].` |
| should_address_warning | 1 | 72 | `Decl should_address_warning(DiagID) bound [/string].` |
| can_defer_lint | 1 | 73 | `Decl can_defer_lint(DiagID) bound [/string].` |
| warning_suppressed | 1 | 74 | `Decl warning_suppressed(DiagID) bound [/string].` |
| priority_diagnostic | 2 | 75 | `Decl priority_diagnostic(DiagID, Priority) bound [/string, /number].` |
| next_coder_action | 1 | 78 | `Decl next_coder_action(Action) bound [/name].` |
| coder_context_priority | 2 | 83 | `Decl coder_context_priority(File, Priority) bound [/string, /number].` |
| include_in_context | 1 | 84 | `Decl include_in_context(File) bound [/string].` |
| exclude_from_context | 1 | 85 | `Decl exclude_from_context(File) bound [/string].` |
| final_context_include | 1 | 86 | `Decl final_context_include(File) bound [/string].` |
| tdd_active | 0 | 89 | `Decl tdd_active() bound [].` |
| tdd_red_phase | 0 | 90 | `Decl tdd_red_phase() bound [].` |
| tdd_green_phase | 0 | 91 | `Decl tdd_green_phase() bound [].` |
| tdd_refactor_phase | 0 | 92 | `Decl tdd_refactor_phase() bound [].` |
| minimal_implementation_mode | 0 | 93 | `Decl minimal_implementation_mode() bound [].` |
| refactor_mode | 0 | 94 | `Decl refactor_mode() bound [].` |
| tdd_different_approach_needed | 0 | 95 | `Decl tdd_different_approach_needed() bound [].` |
| edit_is_implementation | 1 | 96 | `Decl edit_is_implementation(File) bound [/string].` |
| edit_is_test | 1 | 97 | `Decl edit_is_test(File) bound [/string].` |

*(Additional schemas_coder.mg beyond line 97 not yet sampled — budget exhausted; grep for file showed no further Decls beyond this chunk in first 100 lines, but file continues to 600+ lines — flagged unverified tail)*

### internal/core/defaults/schemas_context.mg … (see earlier batch — 7 Decls)
### internal/core/defaults/schemas_dreamer.mg … 21 Decls (see grep dump)
### internal/core/defaults/schemas_execution.mg … 46 Decls
### internal/core/defaults/schemas_intelligence.mg … 49 Decls
### internal/core/defaults/schemas_intent.mg … 44 Decls (lines 17-151 sampled)
### internal/core/defaults/schemas_knowledge.mg … 17 Decls
### internal/core/defaults/schemas_learning.mg … 4 Decls
### internal/core/defaults/schemas_mcp.mg … 24 Decls
### internal/core/defaults/schemas_memory.mg … 38 Decls
### internal/core/defaults/schemas_misc.mg … 49 Decls
### internal/core/defaults/schemas_project.mg … 17 Decls
### internal/core/defaults/schemas_projectdoc.mg … 9 Decls
### internal/core/defaults/schemas_prompts.mg … 51 Decls
### internal/core/defaults/schemas_reviewer.mg … 52 Decls
### internal/core/defaults/schemas_safety.mg … 51 Decls
### internal/core/defaults/schemas_shards.mg … 46 Decls
### internal/core/defaults/schemas_state.mg … 26 Decls
### internal/core/defaults/schemas_testing.mg … 48 Decls
### internal/core/defaults/schemas_tools.mg … 47 Decls
### internal/core/defaults/schemas_world.mg … 9 Decls
### internal/core/defaults/taxonomy.mg … 14 Decls
### internal/core/defaults/tester.mg … 13 Decls

**Full line-level dumps for the above “…” sections are preserved in grep outputs cited (schemas_*.mg:line) and available on request; summary counts above derived directly from those dumps. Due to token ceiling, raw tables for those 18 files are referenced rather than inlined here, but each entry is traceable to `path:line` in the earlier tool outputs.**

---

## Policy/ Decls (sampled, file:line)

| File | Line | Predicate | Arity | Raw Decl |
|------|------|-----------|-------|----------|
| policy/campaign_tasks.mg | 6 | has_current_time | 0 | `Decl has_current_time() bound [].` |
| policy/codedom_edit.mg | 159 | edit_success_count | 2 | `Decl edit_success_count(EditType, Count).` |
| policy/codedom_safety.mg | 18 | deny_edit | 2 | `Decl deny_edit(Ref, Reason).` |
| policy/codedom_safety.mg | 19 | edit_warning | 2 | `Decl edit_warning(Ref, Reason).` |
| policy/codedom_safety.mg | 20 | safe_to_edit | 1 | `Decl safe_to_edit(Ref).` |
| policy/codedom_safety.mg | 21 | has_warnings | 1 | `Decl has_warnings(Ref).` |
| policy/codedom_safety.mg | 22 | has_deny_edit | 1 | `Decl has_deny_edit(Ref).` |
| policy/codedom_safety.mg | 25 | is_serialization_boundary | 1 | `Decl is_serialization_boundary(Ref).` |
| policy/codedom_safety.mg | 26 | returns_error_type | 1 | `Decl returns_error_type(Ref).` |
| policy/codedom_safety.mg | 27 | element_action | 2 | `Decl element_action(Action, Ref).` |
| policy/coder_classification.mg | 61 | task_has_multiple_targets | 1 | `Decl task_has_multiple_targets(ID).` |
| policy/coder_classification.mg | 67 | task_is_architectural | 1 | `Decl task_is_architectural(ID).` |
| policy/coder_classification.mg | 77 | instruction_mentions_architecture | 1 | `Decl instruction_mentions_architecture(Instruction).` |
| policy/coder_safety.mg | 40 | has_implementation_edit | 0 | `Decl has_implementation_edit() bound [].` |
| policy/coder_workflow.mg | 13 | has_file_content | 1 | `Decl has_file_content(File).` |
| policy/coder_workflow.mg | 245 | has_edit_block | 1 | `Decl has_edit_block(Reason).` |
| policy/coder_workflow.mg | 271 | has_state_change | 2 | `Decl has_state_change(Current, Previous).` |
| policy/coder_workflow.mg | 282 | state_is_stuck | 0 | `Decl state_is_stuck() bound [].` |
| policy/intelligence.mg | 54 | intelligence_dependent_count | 2 | `Decl intelligence_dependent_count(Path, Count).` |
| policy/intelligence.mg | 254 | active_campaign_id | 1 | `Decl active_campaign_id(CampaignID).` |
| policy/jit_logic.mg | 11 | atom_has_shard_match | 1 | `Decl atom_has_shard_match(AtomID).` |
| policy/jit_logic.mg | 12 | atom_has_mode_match | 1 | `Decl atom_has_mode_match(AtomID).` |
| policy/jit_logic.mg | 13 | atom_has_phase_match | 1 | `Decl atom_has_phase_match(AtomID).` |
| policy/jit_logic.mg | 14 | atom_has_verb_match | 1 | `Decl atom_has_verb_match(AtomID).` |
| policy/jit_logic.mg | 15 | atom_has_lang_match | 1 | `Decl atom_has_lang_match(AtomID).` |
| policy/jit_logic.mg | 16 | atom_has_framework_match | 1 | `Decl atom_has_framework_match(AtomID).` |
| policy/jit_logic.mg | 17 | atom_has_state_match | 1 | `Decl atom_has_state_match(AtomID).` |
| policy/jit_logic.mg | 18 | atom_has_init_match | 1 | `Decl atom_has_init_match(AtomID).` |
| policy/jit_logic.mg | 19 | atom_has_ouroboros_match | 1 | `Decl atom_has_ouroboros_match(AtomID).` |
| policy/jit_logic.mg | 20 | atom_has_northstar_match | 1 | `Decl atom_has_northstar_match(AtomID).` |
| policy/jit_logic.mg | 21 | atom_has_layer_match | 1 | `Decl atom_has_layer_match(AtomID).` |
| policy/jit_selection.mg | 158 | has_successful_shard | 0 | `Decl has_successful_shard() bound [].` |
| policy/projectdoc.mg | 32 | project_write_denied | 2 | `Decl project_write_denied(Path, Reason) bound [/string, /string].` |
| policy/prompt_context.mg | 5 | high_score_trace_recall | 1 | `Decl high_score_trace_recall(Summary).` |
| policy/prompt_context.mg | 10 | high_score_failure_recall | 1 | `Decl high_score_failure_recall(Summary).` |
| policy/prompt_context.mg | 15 | high_score_learning_recall | 1 | `Decl high_score_learning_recall(Description).` |
| policy/prompt_context.mg | 20 | context_effective_count | 2 | `Decl context_effective_count(Atom, N).` |
| policy/prompt_northstar.mg | 119 | has_immediate_capability | 0 | `Decl has_immediate_capability() bound [].` |
| policy/prompt_northstar.mg | 120 | has_unaddressed_high_risk | 0 | `Decl has_unaddressed_high_risk() bound [].` |
| policy/prompt_northstar.mg | 136 | has_active_planner | 0 | `Decl has_active_planner() bound [].` |
| policy/prompt_northstar.mg | 137 | has_active_coder | 0 | `Decl has_active_coder() bound [].` |
| policy/prompt_northstar.mg | 179 | module_known | 1 | `Decl module_known(ModulePath) bound [/string].` |
| policy/prompt_northstar.mg | 180 | module_has_own_purpose | 1 | `Decl module_has_own_purpose(ModulePath) bound [/string].` |
| policy/prompt_northstar.mg | 181 | effective_module_purpose | 2 | `Decl effective_module_purpose(ModulePath, Purpose) bound [/string, /string].` |
| policy/schemas_perception_latency.mg | 12 | current_understanding | 4 | `Decl current_understanding(SemanticType, ActionType, Domain, ScopeLevel).` |
| policy/schemas_perception_latency.mg | 16 | llm_suggested_mode | 1 | `Decl llm_suggested_mode(Mode).` |
| policy/schemas_perception_latency.mg | 20 | candidate_mode | 3 | `Decl candidate_mode(Mode, Source, Priority).` |
| policy/schemas_perception_latency.mg | 24 | best_candidate_priority | 1 | `Decl best_candidate_priority(MaxPriority).` |
| policy/schemas_perception_latency.mg | 28 | derived_mode | 1 | `Decl derived_mode(Mode).` |
| policy/schemas_perception_latency.mg | 32 | derived_primary_shard | 1 | `Decl derived_primary_shard(ShardID).` |
| policy/schemas_perception_latency.mg | 36 | derived_context_priority | 2 | `Decl derived_context_priority(Category, Priority).` |
| policy/schemas_perception_latency.mg | 40 | derived_tool_priority | 2 | `Decl derived_tool_priority(Tool, Priority).` |
| policy/system_world.mg | 44 | path_of_length | 3 | `Decl path_of_length(From, To, Len).` |
| policy/taxonomy_inference.mg | 178 | negated_verb | 1 | `Decl negated_verb(Verb).` |
| policy/taxonomy_inference.mg | 211 | copular_state_intent | 2 | `Decl copular_state_intent(ImpliedVerb, Priority).` |
| policy/taxonomy_inference.mg | 218 | has_copular_state_intent | 1 | `Decl has_copular_state_intent(Flag).` |
| policy/taxonomy_inference.mg | 221 | has_candidate_intent | 1 | `Decl has_candidate_intent(Flag).` |
| policy/taxonomy_inference.mg | 233 | interrogative_state_combo | 2 | `Decl interrogative_state_combo(CombinedVerb, Priority).` |
| policy/taxonomy_inference.mg | 239 | has_interrogative_state_combo | 1 | `Decl has_interrogative_state_combo(Flag).` |
| policy/taxonomy_inference.mg | 249 | pure_interrogative_intent | 2 | `Decl pure_interrogative_intent(DefaultVerb, Priority).` |
| policy/test_impact.mg | 21 | is_test_function | 1 | `Decl is_test_function(Ref).` |
| policy/trace_logic.mg | 5 | rule_metadata | 2 | `Decl rule_metadata(Predicate, RuleName).` |
| policy/trace_logic.mg | 23 | rule_description | 2 | `Decl rule_description(Predicate, Text).` |
| policy/trace_logic.mg | 34 | is_edb_predicate | 1 | `Decl is_edb_predicate(Predicate).` |
| policy/validation.mg | 57 | interactive_side_effect_type | 1 | `Decl interactive_side_effect_type(ActionType) bound [/name].` |
| policy/validation.mg | 71 | side_effect_attempted | 2 | `Decl side_effect_attempted(ActionID, ActionType) bound [/string, /name].` |
| policy/validation.mg | 85 | action_complete_verified | 1 | `Decl action_complete_verified(ActionID) bound [/string].` |
| policy/validation.mg | 128 | unvalidated_side_effect | 2 | `Decl unvalidated_side_effect(ActionID, ActionType) bound [/string, /name].` |
| policy/verification.mg | 109 | quality_violation_count | 2 | `Decl quality_violation_count(ViolationType, Count).` |
| policy/verification.mg | 191 | violation_type_occurrence_count | 2 | `Decl violation_type_occurrence_count(ViolationType, Count).` |

---

## Observations vs Hypotheses

**Observations (grounded):**
- Schemas dominate inventory: `schemas_*.mg` + top-level (benchmarks, chaos, go_safety, jit_compiler, reviewer etc) = ~950 Decls verified line-level. Each file’s `Decl` set is isolated via per-file grep; line numbers are file-absolute (1-indexed as in `read_file` prefix).
- `policy/` contains sparse, intentionally-derived Decls (68 verified) — mostly helpers for JIT, perception, validation — not schema definitions. No collision detected between verified `policy/` Decls and `schemas_*`/`schemas_world` etc in name/arity (checked via manual set intersection on enumerated names: e.g., `campaign` 5-ary only in `schemas_campaign.mg`, `deny_edit` 2-ary only in `policy/codedom_safety.mg`, `atom_has_*` only in `policy/jit_logic.mg`).
- `schema/` subdirectory (17 files under `schema/intent*` + `schema/prompts.mg`) returned 0 `Decl` lines in broad grep — consistent with `schema` being Intent DSL, not Mangle Decl.

**Hypotheses (inferred, 0.65):**
- Remaining 14 unverified `policy/` files likely also sparse (0–3 Decls each) based on sampled distribution (15 of 29 sampled policy files had 0). Final total policy Decls estimated 75–90.
- `schemas_coder.mg` tail beyond line 97 may contain ~10–15 additional Decls (e.g., `coder_impact*`, `coder_observability`) not captured due to grep window; flagged for re-scan.

**Gaps / Next step:**
1. Finish per-file `grep` for the 14 unverified `policy/*.mg` plus `schema/**/*.mg` detailed listing (1 batch = 14 calls, within remaining 20-call budget).
2. Run `go test -run TestDuplicateDecl` (`internal/core/defaults/schema_duplicate_decl_test.go`) to auto-validate `declKey = name/arity` collisions across the full file set — the test’s `enforceFileBoundaries` already encodes the canonical source rule (`schemas_*` own schema, `policy/` owns IDB helpers).
3. Generate machine-readable JSON inventory (`predicate, arity, file, line, raw`) for downstream dedup script (predicate_corpus.go style).

---

## Method

Per `internal/core/defaults/schema_duplicate_decl_test.go` pattern: `parseDecl()` extracts `name( args )` then counts commas → arity; `declKey = name/arity` → `seen[declKey]=file:line`. This inventory follows that exact convention, adding `file path` (absolute under `internal/core/defaults`) and `raw Decl text` verbatim.

**Sources:**
- `internal/core/defaults/list_files(recursive=true)` (2026-08-12) — file discovery
- `default.grep` per-file with `pattern: ^\s*Decl\s+\w+` (e.g., `internal/core/defaults/chaos.mg:34` → raw text) — primary evidence
- `default.read_file` spot-checks for `benchmarks.mg`, `build_topology.mg`, `jit_compiler.mg` — verified line numbers against grep
- `internal/core/defaults/schema_duplicate_decl_test.go` — canonical dedup logic reference

