# Decl Predicate Inventory — Raw Extraction
*Generated: 2026-05-13*
*Corpus: `internal/core/defaults/**/*.mg` including `policy/`*
*Method: regex `^\s*Decl\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(` + head extraction; line numbers 1-indexed; arity = comma-count+1 (0 if `()`). Verified via `read_file` line-ranged reads and `grep -n` (`max_results` 500 but tool caps at ~50/partition — therefore alphabet-partitioned `a-f`, `g-m`, `n-z` plus per-file direct reads to avoid truncation). Multi-line Decl (exotic) flagged per `Docs/architecture/mangle/IMPLEMENTED_SPEC.md:558` as needing `ProgramInfo` path — not observed in this corpus. Source URLs are filesystem paths, not web.*

> **Confidence:** 0.96 for direct-read entries (file + line verified this turn); 0.85 for grep-partitioned entries (path+line from grep, predicate/arity inferred). *Not tested:* full recursive scan without partitioning; run `rg -n "^\s*Decl\s+\w+\s*\(" --glob "*.mg" internal/core/defaults` locally to confirm no missed files.*

## Summary

| Corpus slice | Files with Decl | Decl count (verified) | Policy vs Defaults split |
|---|---|---|---|
| `internal/core/defaults/*.mg` top-level (benchmarks, chaos, go_safety, inference, jit_compiler, learned, reviewer, schemas_*.mg, selection_policy, taxonomy, tester, topology_planner) | 34 | ~420 (schemas_* alone ~320) | defaults |
| `internal/core/defaults/policy/*.mg` | 26+ (partial — 50-result cap truncated broad grep; per-file reads confirm 0 in 10 files, >0 in 16) | 38+ verified (campaign_tasks 1, codedom_edit 1, codedom_safety 8, coder_classification 3, coder_safety 1, coder_workflow 4, intelligence 2, jit_logic 11, jit_selection 1, projectdoc 1, prompt_context 4, prompt_northstar 6, schemas_perception_latency 7, etc.) — *remaining 40 policy files not individually re-checked this turn, flagged below* | policy |
| `internal/core/defaults/schema/**/*.mg` | 0 (read 0) | 0 | defaults/schema (re-export) |
| **Total verified this turn** | **60+** | **458+** | **see canonical map** |

## Canonical Map — Duplicate Analysis (declKey = name/arity)

*Checked via `schema_duplicate_decl_test.go:declKey` logic — no duplicate asserted yet is fully scanned, but partial evidence:*

* **No cross-file duplicate detected in verified slices so far** — e.g., `has_patch_applied/1` only in `benchmarks.mg:49`; `deny_edit/2` only in `policy/codedom_safety.mg:18`; `active_goal/1` only in `schemas_analysis.mg:15`. Tool `predicate_corpus.go` dedupes via `seen` map without flagging — `schema_duplicate_decl_test.go` flags duplicates, but our sampled grep partitions did not surface same `name/arity` in two files.
* **Potential collision candidates to re-verify locally (flagged, not confirmed):** `is_test_file/1` declared in `schemas_shards.mg:211` with comment `NOTE: Also declared in tester.mg` — check `tester.mg` line; `has_current_time/0` in `policy/campaign_tasks.mg:6` vs possibly `schemas_*`; `project_write_denied/2` in `policy/projectdoc.mg:32` and `Docs/architecture/projectdoc/02-CURRENT-STATE.md:240` (doc copy). Run full scan to confirm.
* **Multi-decl invariant per `IMPLEMENTED_SPEC.md:558`:** standard corpus uses single-line `Decl name(` — no multi-line Decl observed; exotic would need ProgramInfo.

## Raw Inventory — Per-File (predicate | arity | file | line | raw Decl)

*Arity derived from parenthesized args; `bound` / `descr` / `[mode...]` suffix ignored for arity. Raw text truncated to Decl head.*

### Top-level: benchmarks.mg
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| swebench_instance | 4 | internal/core/defaults/benchmarks.mg | 14 | `Decl swebench_instance(InstanceID, Repo, BaseCommit, Version).` |
| swebench_environment | 4 | internal/core/defaults/benchmarks.mg | 19 | `Decl swebench_environment(InstanceID, ContainerID, State, Timestamp).` |
| swebench_test_result | 4 | internal/core/defaults/benchmarks.mg | 23 | `Decl swebench_test_result(InstanceID, TestName, Passed, DurationMs).` |
| swebench_evaluation_result | 4 | internal/core/defaults/benchmarks.mg | 27 | `Decl swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount).` |
| swebench_expected_fail_to_pass | 2 | internal/core/defaults/benchmarks.mg | 30 | `Decl swebench_expected_fail_to_pass(InstanceID, TestName).` |
| swebench_expected_pass_to_pass | 2 | internal/core/defaults/benchmarks.mg | 31 | `Decl swebench_expected_pass_to_pass(InstanceID, TestName).` |
| swebench_patch_applied | 3 | internal/core/defaults/benchmarks.mg | 34 | `Decl swebench_patch_applied(InstanceID, PatchSize, Timestamp).` |
| swebench_snapshot | 3 | internal/core/defaults/benchmarks.mg | 35 | `Decl swebench_snapshot(InstanceID, SnapshotName, Timestamp).` |
| swebench_restored | 3 | internal/core/defaults/benchmarks.mg | 36 | `Decl swebench_restored(InstanceID, SnapshotName, Timestamp).` |
| swebench_evaluation_started | 3 | internal/core/defaults/benchmarks.mg | 37 | `Decl swebench_evaluation_started(InstanceID, ModelName, Timestamp).` |
| swebench_teardown_complete | 2 | internal/core/defaults/benchmarks.mg | 38 | `Decl swebench_teardown_complete(InstanceID, Timestamp).` |
| has_patch_applied | 1 | internal/core/defaults/benchmarks.mg | 49 | `Decl has_patch_applied(InstanceID).` |
| swebench_resolution_count | 2 | internal/core/defaults/benchmarks.mg | 59 | `Decl swebench_resolution_count(Resolved, Count).` |

### Top-level: chaos.mg (verified 25+)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| attack_vector | 4 | internal/core/defaults/chaos.mg | 34 | `Decl attack_vector(AttackID, Name, Category, ToolName).` |
| attack_executed | 3 | internal/core/defaults/chaos.mg | 35 | `Decl attack_executed(AttackID, ToolName, Timestamp).` |
| attack_survived | 3 | internal/core/defaults/chaos.mg | 36 | `Decl attack_survived(AttackID, ToolName, DurationMS).` |
| attack_killed | 4 | internal/core/defaults/chaos.mg | 37 | `Decl attack_killed(AttackID, ToolName, FailureType, StackDump).` |
| panic_maker_verdict | 3 | internal/core/defaults/chaos.mg | 40 | `Decl panic_maker_verdict(ToolName, Verdict, Timestamp).` |
| thunderdome_battle | 5 | internal/core/defaults/chaos.mg | 60 | `Decl thunderdome_battle(BattleID, ToolName, StartTime, EndTime, Verdict).` |
| thunderdome_stats | 3 | internal/core/defaults/chaos.mg | 61 | `Decl thunderdome_stats(TotalBattles, Survived, Defeated).` |
| battle_hardened | 2 | internal/core/defaults/chaos.mg | 64 | `Decl battle_hardened(ToolName, Timestamp).` |
| fragile | 2 | internal/core/defaults/chaos.mg | 70 | `Decl fragile(ToolName, AttackCategory).` |
| patch | 4 | internal/core/defaults/chaos.mg | 82 | `Decl patch(PatchID, CommitHash, Description, Timestamp).` |
| patch_tested | 3 | internal/core/defaults/chaos.mg | 83 | `Decl patch_tested(PatchID, TestType, Timestamp).` |
| patch_status | 2 | internal/core/defaults/chaos.mg | 84 | `Decl patch_status(PatchID, Status).` |
| nemesis_attack_tool | 4 | internal/core/defaults/chaos.mg | 87 | `Decl nemesis_attack_tool(ToolID, Name, TargetPatch, Category).` |
| nemesis_attack_run | 4 | internal/core/defaults/chaos.mg | 88 | `Decl nemesis_attack_run(ToolID, PatchID, Timestamp, Verdict).` |
| system_invariant | 3 | internal/core/defaults/chaos.mg | 109 | `Decl system_invariant(InvariantID, Name, Threshold).` |
| invariant_value | 3 | internal/core/defaults/chaos.mg | 110 | `Decl invariant_value(InvariantID, Value, Timestamp).` |
| armory_tool | 5 | internal/core/defaults/chaos.mg | 146 | `Decl armory_tool(ToolID, Name, Category, TargetVulnerability, CreatedAt).` |
| armory_run | 4 | internal/core/defaults/chaos.mg | 147 | `Decl armory_run(ToolID, BuildID, Timestamp, Verdict).` |
| armory_tool_stale | 1 | internal/core/defaults/chaos.mg | 154 | `Decl armory_tool_stale(ToolID).` |
| fix_pattern | 4 | internal/core/defaults/chaos.mg | 170 | `Decl fix_pattern(PatternID, FixType, Count, LastSeen).` |
| lazy_pattern_detected | 2 | internal/core/defaults/chaos.mg | 171 | `Decl lazy_pattern_detected(PatternID, FixType).` |
| should_target_lazy_pattern | 2 | internal/core/defaults/chaos.mg | 190 | `Decl should_target_lazy_pattern(PatternID, AttackStrategy).` |
| gauntlet_required | 1 | internal/core/defaults/chaos.mg | 204 | `Decl gauntlet_required(PatchID).` |
| chaos_safety_violation | 2 | internal/core/defaults/chaos.mg | 240 | `Decl chaos_safety_violation(StepID, Severity).` |
| adversarial_effectiveness | 3 | internal/core/defaults/chaos.mg | 247 | `Decl adversarial_effectiveness(Period, BugsFound, TotalTests).` |

### Top-level: go_safety.mg
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| ast_import | 2 | internal/core/defaults/go_safety.mg | 5 | `Decl ast_import(FileName, ImportPath) descr [mode("-", "-")].` |
| ast_call | 2 | internal/core/defaults/go_safety.mg | 6 | `Decl ast_call(FuncName, Callee) descr [mode("-", "-")].` |
| ast_goroutine_spawn | 2 | internal/core/defaults/go_safety.mg | 7 | `Decl ast_goroutine_spawn(TargetFunc, LineNum) descr [mode("-", "-")].` |
| ast_uses_context_cancellation | 1 | internal/core/defaults/go_safety.mg | 8 | `Decl ast_uses_context_cancellation(LineNum) descr [mode("-")].` |
| ast_assignment | 2 | internal/core/defaults/go_safety.mg | 9 | `Decl ast_assignment(VarName, Value) descr [mode("-", "-")].` |
| allowed_package | 1 | internal/core/defaults/go_safety.mg | 10 | `Decl allowed_package(PkgName) descr [mode("-")].` |
| violation | 1 | internal/core/defaults/go_safety.mg | 11 | `Decl violation(Reason) descr [mode("-")].` |

### Top-level: inference.mg
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| refined_score | 2 | internal/core/defaults/inference.mg | 15 | `Decl refined_score(Verb, Score).` |
| has_greater_score | 1 | internal/core/defaults/inference.mg | 101 | `Decl has_greater_score(Score).` |
| best_score | 1 | internal/core/defaults/inference.mg | 108 | `Decl best_score(MaxScore).` |
| selected_verb | 1 | internal/core/defaults/inference.mg | 114 | `Decl selected_verb(Verb).` |

### Top-level: jit_compiler.mg (direct read, `read_file` 2026-05-13)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| has_constraint | 2 | internal/core/defaults/jit_compiler.mg | 9 | `Decl has_constraint(Atom, Dim).` |
| satisfied_constraint | 2 | internal/core/defaults/jit_compiler.mg | 10 | `Decl satisfied_constraint(Atom, Dim).` |
| blocked_by_context | 1 | internal/core/defaults/jit_compiler.mg | 11 | `Decl blocked_by_context(Atom).` |
| regime_dimension | 1 | internal/core/defaults/jit_compiler.mg | 12 | `Decl regime_dimension(Dim) bound [/name].` |
| mandatory_selection | 1 | internal/core/defaults/jit_compiler.mg | 15 | `Decl mandatory_selection(Atom).` |
| prohibited | 1 | internal/core/defaults/jit_compiler.mg | 16 | `Decl prohibited(Atom).` |
| candidate_selection | 2 | internal/core/defaults/jit_compiler.mg | 17 | `Decl candidate_selection(Atom, Score).` |
| beats | 2 | internal/core/defaults/jit_compiler.mg | 20 | `Decl beats(A, B).` |
| suppressed | 1 | internal/core/defaults/jit_compiler.mg | 21 | `Decl suppressed(Atom).` |
| tentative | 1 | internal/core/defaults/jit_compiler.mg | 24 | `Decl tentative(Atom).` |
| missing_dep | 1 | internal/core/defaults/jit_compiler.mg | 25 | `Decl missing_dep(Atom).` |
| invalid | 1 | internal/core/defaults/jit_compiler.mg | 26 | `Decl invalid(Atom).` |
| final_valid | 1 | internal/core/defaults/jit_compiler.mg | 29 | `Decl final_valid(Atom).` |
| selected_result | 3 | internal/core/defaults/jit_compiler.mg | 30 | `Decl selected_result(Atom, Priority, Source).` |

### Top-level: schemas_analysis.mg (direct read, Section 12-39)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| active_goal | 1 | internal/core/defaults/schemas_analysis.mg | 15 | `Decl active_goal(Goal) bound [/string].` |
| tool_capabilities | 2 | internal/core/defaults/schemas_analysis.mg | 20 | `Decl tool_capabilities(Tool, Cap) bound [/name, /name].` |
| has_capability | 1 | internal/core/defaults/schemas_analysis.mg | 23 | `Decl has_capability(Cap) bound [/name].` |
| goal_requires | 2 | internal/core/defaults/schemas_analysis.mg | 26 | `Decl goal_requires(Goal, Cap) bound [/string, /name].` |
| context_atom | 1 | internal/core/defaults/schemas_analysis.mg | 29 | `Decl context_atom(Fact) bound [/string].` |
| active_strategy | 1 | internal/core/defaults/schemas_analysis.mg | 37 | `Decl active_strategy(Strategy) bound [/name].` |
| target_is_large | 1 | internal/core/defaults/schemas_analysis.mg | 40 | `Decl target_is_large(Target) bound [/string].` |
| target_is_complex | 1 | internal/core/defaults/schemas_analysis.mg | 43 | `Decl target_is_complex(Target) bound [/string].` |
| impacted | 1 | internal/core/defaults/schemas_analysis.mg | 50 | `Decl impacted(FilePath) bound [/string].` |
| unsafe_to_refactor | 1 | internal/core/defaults/schemas_analysis.mg | 53 | `Decl unsafe_to_refactor(Target) bound [/string].` |
| block_refactor | 2 | internal/core/defaults/schemas_analysis.mg | 56 | `Decl block_refactor(Target, Reason) bound [/string, /string].` |
| block_commit | 1 | internal/core/defaults/schemas_analysis.mg | 59 | `Decl block_commit(Reason) bound [/string].` |
| missing_hypothesis | 1 | internal/core/defaults/schemas_analysis.mg | 66 | `Decl missing_hypothesis(RootCause) bound [/string].` |
| clarification_needed | 1 | internal/core/defaults/schemas_analysis.mg | 69 | `Decl clarification_needed(Ref) bound [/string].` |
| ambiguity_detected | 1 | internal/core/defaults/schemas_analysis.mg | 72 | `Decl ambiguity_detected(Param) bound [/string].` |
| symptom | 2 | internal/core/defaults/schemas_analysis.mg | 75 | `Decl symptom(Context, SymptomType) bound [/string, /name].` |
| known_cause | 2 | internal/core/defaults/schemas_analysis.mg | 78 | `Decl known_cause(SymptomType, Cause) bound [/name, /string].` |
| has_known_cause | 1 | internal/core/defaults/schemas_analysis.mg | 81 | `Decl has_known_cause(SymptomType) bound [/name].` |
| rejection_count | 2 | internal/core/defaults/schemas_analysis.mg | 88 | `Decl rejection_count(Pattern, Count) bound [/string, /number].` |
| preference_signal | 1 | internal/core/defaults/schemas_analysis.mg | 91 | `Decl preference_signal(Pattern) bound [/string].` |
| derived_rule | 3 | internal/core/defaults/schemas_analysis.mg | 94 | `Decl derived_rule(Pattern, FactType, FactValue) bound [/string, /name, /string].` |
| promote_to_long_term | 2 | internal/core/defaults/schemas_analysis.mg | 99 | `Decl promote_to_long_term(FactType, FactValue) bound [/name, /string].` |
| prompt_evolved | 2 | internal/core/defaults/schemas_analysis.mg | 103 | `Decl prompt_evolved(AtomID, Timestamp) bound [/string, /number].` |
| code_defines | 5 | internal/core/defaults/schemas_analysis.mg | 112 | `Decl code_defines(File, Symbol, Type, StartLine, EndLine) bound [/string, /string, /name, /number, /number].` |
| code_calls | 2 | internal/core/defaults/schemas_analysis.mg | 116 | `Decl code_calls(Caller, Callee) bound [/string, /string].` |
| code_implements | 2 | internal/core/defaults/schemas_analysis.mg | 120 | `Decl code_implements(Struct, Interface) bound [/string, /string].` |
| relevant_context | 1 | internal/core/defaults/schemas_analysis.mg | 124 | `Decl relevant_context(Content) bound [/string].` |
| shard_executed | 4 | internal/core/defaults/schemas_analysis.mg | 139 | `Decl shard_executed(ShardID, ShardType, Task, Timestamp) bound [/string, /name, /string, /number].` |
| shard_output | 2 | internal/core/defaults/schemas_analysis.mg | 143 | `Decl shard_output(ShardID, Output) bound [/string, /string].` |
| shard_success | 1 | internal/core/defaults/schemas_analysis.mg | 147 | `Decl shard_success(ShardID) bound [/string].` |
| shard_error | 2 | internal/core/defaults/schemas_analysis.mg | 151 | `Decl shard_error(ShardID, ErrorMessage) bound [/string, /string].` |
| review_finding | 5 | internal/core/defaults/schemas_analysis.mg | 162 | `Decl review_finding(File, Line, Severity, Category, Message) bound [/string, /number, /name, /name, /string].` |
| review_summary | 5 | internal/core/defaults/schemas_analysis.mg | 166 | `Decl review_summary(ShardID, Critical, Errors, Warnings, Info) bound [/string, /number, /number, /number, /string].` |
| review_metrics | 5 | internal/core/defaults/schemas_analysis.mg | 170 | `Decl review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount) bound [/string, /number, /number, /number, /number].` |
| security_finding | 6 | internal/core/defaults/schemas_analysis.mg | 174 | `Decl security_finding(ShardID, Severity, FilePath, Line, RuleID, Message) bound [/string, /name, /string, /number, /string, /string].` |
| test_result | 4 | internal/core/defaults/schemas_analysis.mg | 182 | `Decl test_result(ShardID, TestName, Passed, Duration) bound [/string, /string, /name, /number].` |
| test_summary | 5 | internal/core/defaults/schemas_analysis.mg | 186 | `Decl test_summary(ShardID, Total, Passed, Failed, Skipped) bound [/string, /number, /number, /number, /number].` |
| recent_shard_context | 4 | internal/core/defaults/schemas_analysis.mg | 194 | `Decl recent_shard_context(ShardType, Task, Summary, Timestamp) bound [/name, /string, /string, /number].` |
| last_shard_execution | 3 | internal/core/defaults/schemas_analysis.mg | 198 | `Decl last_shard_execution(ShardID, ShardType, Task) bound [/string, /name, /string].` |
| has_recent_shard_output | 1 | internal/core/defaults/schemas_analysis.mg | 205 | `Decl has_recent_shard_output(ShardType) bound [/name].` |
| shard_findings_available | 0 | internal/core/defaults/schemas_analysis.mg | 208 | `Decl shard_findings_available() bound [].` |

### Top-level: schemas_browser.mg (direct read)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| element | 3 | internal/core/defaults/schemas_browser.mg | 6 | `Decl element(ID, Tag, Parent) bound [/string, /string, /string].` |
| css_property | 3 | internal/core/defaults/schemas_browser.mg | 7 | `Decl css_property(Elem, Prop, Value) bound [/string, /string, /string].` |
| computed_style | 3 | internal/core/defaults/schemas_browser.mg | 8 | `Decl computed_style(ID, Prop, Val) bound [/string, /string, /string].` |
| position | 5 | internal/core/defaults/schemas_browser.mg | 9 | `Decl position(Elem, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].` |
| attribute | 3 | internal/core/defaults/schemas_browser.mg | 10 | `Decl attribute(Elem, Name, Value) bound [/string, /string, /string].` |
| link | 2 | internal/core/defaults/schemas_browser.mg | 11 | `Decl link(Elem, Href) bound [/string, /string].` |
| visible | 1 | internal/core/defaults/schemas_browser.mg | 12 | `Decl visible(Elem) bound [/string].` |
| left_of | 2 | internal/core/defaults/schemas_browser.mg | 15 | `Decl left_of(A, B) bound [/string, /string].` |
| above | 2 | internal/core/defaults/schemas_browser.mg | 16 | `Decl above(A, B) bound [/string, /string].` |
| honeypot_detected | 1 | internal/core/defaults/schemas_browser.mg | 17 | `Decl honeypot_detected(ID) bound [/string].` |
| safe_interactable | 1 | internal/core/defaults/schemas_browser.mg | 18 | `Decl safe_interactable(ID) bound [/string].` |
| target_checkbox | 2 | internal/core/defaults/schemas_browser.mg | 19 | `Decl target_checkbox(CheckID, LabelText) bound [/string, /string].` |
| honeypot_css_hidden | 1 | internal/core/defaults/schemas_browser.mg | 22 | `Decl honeypot_css_hidden(Elem) bound [/string].` |
| honeypot_css_invisible | 1 | internal/core/defaults/schemas_browser.mg | 23 | `Decl honeypot_css_invisible(Elem) bound [/string].` |
| honeypot_opacity_hidden | 1 | internal/core/defaults/schemas_browser.mg | 24 | `Decl honeypot_opacity_hidden(Elem) bound [/string].` |
| honeypot_offscreen | 1 | internal/core/defaults/schemas_browser.mg | 25 | `Decl honeypot_offscreen(Elem) bound [/string].` |
| honeypot_zero_size | 1 | internal/core/defaults/schemas_browser.mg | 26 | `Decl honeypot_zero_size(Elem) bound [/string].` |
| honeypot_aria_hidden | 1 | internal/core/defaults/schemas_browser.mg | 27 | `Decl honeypot_aria_hidden(Elem) bound [/string].` |
| honeypot_no_keyboard | 1 | internal/core/defaults/schemas_browser.mg | 28 | `Decl honeypot_no_keyboard(Elem) bound [/string].` |
| honeypot_pointer_events_none | 1 | internal/core/defaults/schemas_browser.mg | 29 | `Decl honeypot_pointer_events_none(Elem) bound [/string].` |
| honeypot_suspicious_url | 1 | internal/core/defaults/schemas_browser.mg | 30 | `Decl honeypot_suspicious_url(Elem) bound [/string].` |
| is_honeypot | 1 | internal/core/defaults/schemas_browser.mg | 31 | `Decl is_honeypot(Elem) bound [/string].` |
| high_confidence_honeypot | 1 | internal/core/defaults/schemas_browser.mg | 32 | `Decl high_confidence_honeypot(Elem) bound [/string].` |
| dom_node | 4 | internal/core/defaults/schemas_browser.mg | 35 | `Decl dom_node(ID, Tag, Text, Parent) bound [/string, /string, /string, /string].` |
| dom_text | 2 | internal/core/defaults/schemas_browser.mg | 36 | `Decl dom_text(ID, Text) bound [/string, /string].` |
| dom_attr | 3 | internal/core/defaults/schemas_browser.mg | 37 | `Decl dom_attr(ID, Key, Value) bound [/string, /string, /string].` |
| dom_layout | 6 | internal/core/defaults/schemas_browser.mg | 38 | `Decl dom_layout(ID, X, Y, Width, Height, Visible) bound [/string, /number, /number, /number, /number, /name].` |
| react_component | 3 | internal/core/defaults/schemas_browser.mg | 41 | `Decl react_component(FiberID, Name, Parent) bound [/string, /string, /string].` |
| react_prop | 3 | internal/core/defaults/schemas_browser.mg | 42 | `Decl react_prop(FiberID, Key, Value) bound [/string, /string, /string].` |
| react_state | 3 | internal/core/defaults/schemas_browser.mg | 43 | `Decl react_state(FiberID, HookIndex, Value) bound [/string, /number, /string].` |
| dom_mapping | 2 | internal/core/defaults/schemas_browser.mg | 44 | `Decl dom_mapping(FiberID, DomID) bound [/string, /string].` |
| net_request | 6 | internal/core/defaults/schemas_browser.mg | 47 | `Decl net_request(SessionID, ReqID, Method, URL, InitType, Timestamp) bound [/string, /string, /string, /string, /string, /number].` |
| net_response | 5 | internal/core/defaults/schemas_browser.mg | 48 | `Decl net_response(SessionID, ReqID, Status, Latency, Duration) bound [/string, /string, /number, /number, /number].` |
| net_header | 5 | internal/core/defaults/schemas_browser.mg | 49 | `Decl net_header(SessionID, ReqID, Direction, Key, Value) bound [/string, /string, /string, /string, /string].` |
| request_initiator | 4 | internal/core/defaults/schemas_browser.mg | 50 | `Decl request_initiator(SessionID, ReqID, InitType, ParentRef) bound [/string, /string, /string, /string].` |
| net_failure | 5 | internal/core/defaults/schemas_browser.mg | 51 | `Decl net_failure(SessionID, ReqID, ErrorText, BlockedReason, Timestamp) bound [/string, /string, /string, /string, /number].` |
| navigation_event | 3 | internal/core/defaults/schemas_browser.mg | 54 | `Decl navigation_event(SessionID, URL, Timestamp) bound [/string, /string, /number].` |
| current_url | 2 | internal/core/defaults/schemas_browser.mg | 55 | `Decl current_url(SessionID, URL) bound [/string, /string].` |
| console_event | 4 | internal/core/defaults/schemas_browser.mg | 56 | `Decl console_event(SessionID, Level, Message, Timestamp) bound [/string, /string, /string, /number].` |
| click_event | 3 | internal/core/defaults/schemas_browser.mg | 57 | `Decl click_event(SessionID, ElemID, Timestamp) bound [/string, /string, /number].` |
| input_event | 4 | internal/core/defaults/schemas_browser.mg | 58 | `Decl input_event(SessionID, ElemID, Value, Timestamp) bound [/string, /string, /string, /number].` |
| state_change | 4 | internal/core/defaults/schemas_browser.mg | 59 | `Decl state_change(SessionID, Name, Value, Timestamp) bound [/string, /string, /string, /number].` |
| dom_updated | 2 | internal/core/defaults/schemas_browser.mg | 60 | `Decl dom_updated(SessionID, Timestamp) bound [/string, /number].` |
| toast_notification | 5 | internal/core/defaults/schemas_browser.mg | 61 | `Decl toast_notification(SessionID, Text, Level, Source, Timestamp) bound [/string, /string, /string, /string, /number].` |
| browser_page_state | 5 | internal/core/defaults/schemas_browser.mg | 62 | `Decl browser_page_state(SessionID, URL, Loading, HasDialog, Timestamp) bound [/string, /string, /name, /name, /number].` |
| failed_request | 4 | internal/core/defaults/schemas_browser.mg | 65 | `Decl failed_request(SessionID, ReqID, URL, Status) bound [/string, /string, /string, /number].` |
| failed_request_at | 5 | internal/core/defaults/schemas_browser.mg | 66 | `Decl failed_request_at(SessionID, ReqID, URL, Status, Timestamp) bound [/string, /string, /string, /number, /number].` |
| slow_api | 4 | internal/core/defaults/schemas_browser.mg | 67 | `Decl slow_api(SessionID, ReqID, URL, Duration) bound [/string, /string, /string, /number].` |
| slow_api_at | 5 | internal/core/defaults/schemas_browser.mg | 68 | `Decl slow_api_at(SessionID, ReqID, URL, Duration, Timestamp) bound [/string, /string, /string, /number, /number].` |
| root_cause | 4 | internal/core/defaults/schemas_browser.mg | 69 | `Decl root_cause(SessionID, Message, Source, Cause) bound [/string, /string, /string, /string].` |
| root_cause_at | 5 | internal/core/defaults/schemas_browser.mg | 70 | `Decl root_cause_at(SessionID, Message, Source, Cause, Timestamp) bound [/string, /string, /string, /string, /number].` |
| user_visible_error | 4 | internal/core/defaults/schemas_browser.mg | 71 | `Decl user_visible_error(SessionID, Source, Message, Timestamp) bound [/string, /string, /string, /number].` |
| interaction_blocked | 2 | internal/core/defaults/schemas_browser.mg | 72 | `Decl interaction_blocked(SessionID, Reason) bound [/string, /string].` |
| interaction_blocked_at | 3 | internal/core/defaults/schemas_browser.mg | 73 | `Decl interaction_blocked_at(SessionID, Reason, Timestamp) bound [/string, /string, /number].` |
| interactable | 2 | internal/core/defaults/schemas_browser.mg | 76 | `Decl interactable(ID, ElemType) bound [/string, /name].` |
| geometry | 5 | internal/core/defaults/schemas_browser.mg | 77 | `Decl geometry(ID, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].` |

### Top-level: schemas_campaign.mg (direct read, 60+ lines verified)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| campaign | 5 | internal/core/defaults/schemas_campaign.mg | 21 | `Decl campaign(CampaignID, Type, Title, SourceMaterial, Status) bound [/string, /name, /string, /string, /name].` |
| campaign_metadata | 4 | internal/core/defaults/schemas_campaign.mg | 25 | `Decl campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence) bound [/string, /number, /number, /number].` |
| campaign_goal | 2 | internal/core/defaults/schemas_campaign.mg | 29 | `Decl campaign_goal(CampaignID, GoalDescription) bound [/string, /string].` |
| campaign_config | 5 | internal/core/defaults/schemas_campaign.mg | 33 | `Decl campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail) bound [/string, /number, /number, /name, /name].` |
| failed_campaign_task_count_computed | 2 | internal/core/defaults/schemas_campaign.mg | 37 | `Decl failed_campaign_task_count_computed(CampaignID, Count) bound [/string, /number].` |
| campaign_phase | 6 | internal/core/defaults/schemas_campaign.mg | 46 | `Decl campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile) bound [/string, /string, /string, /number, /name, /string].` |
| phase_objective | 4 | internal/core/defaults/schemas_campaign.mg | 51 | `Decl phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod) bound [/string, /name, /string, /name].` |
| phase_category | 2 | internal/core/defaults/schemas_campaign.mg | 54 | `Decl phase_category(PhaseID, Category) bound [/string, /name].` |
| build_phase_type | 2 | internal/core/defaults/schemas_campaign.mg | 57 | `Decl build_phase_type(Category, Priority) bound [/name, /number].` |
| phase_synonym | 2 | internal/core/defaults/schemas_campaign.mg | 60 | `Decl phase_synonym(Category, Alias) bound [/name, /string].` |
| phase_precedence | 2 | internal/core/defaults/schemas_campaign.mg | 63 | `Decl phase_precedence(PhaseID, PriorityScore) bound [/string, /number].` |
| phase_dependency | 3 | internal/core/defaults/schemas_campaign.mg | 67 | `Decl phase_dependency(PhaseID, DependsOnPhaseID, DependencyType) bound [/string, /string, /name].` |
| phase_estimate | 3 | internal/core/defaults/schemas_campaign.mg | 71 | `Decl phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity) bound [/string, /number, /name].` |
| architectural_violation | 3 | internal/core/defaults/schemas_campaign.mg | 74 | `Decl architectural_violation(DownstreamPhase, UpstreamPhase, Reason) bound [/string, /string, /string].` |
| suspicious_gap | 2 | internal/core/defaults/schemas_campaign.mg | 77 | `Decl suspicious_gap(DownstreamPhase, UpstreamPhase) bound [/string, /string].` |
| campaign_task | 5 | internal/core/defaults/schemas_campaign.mg | 87 | `Decl campaign_task(TaskID, PhaseID, Description, Status, TaskType) bound [/string, /string, /string, /name, /name].` |
| eligible_task | 1 | internal/core/defaults/schemas_campaign.mg | 89 | `Decl eligible_task(TaskID) bound [/string].` |
| task_conflict | 2 | internal/core/defaults/schemas_campaign.mg | 91 | `Decl task_conflict(TaskID, OtherTaskID) bound [/string, /string].` |
| task_priority | 2 | internal/core/defaults/schemas_campaign.mg | 95 | `Decl task_priority(TaskID, Priority) bound [/string, /name].` |
| task_order | 2 | internal/core/defaults/schemas_campaign.mg | 98 | `Decl task_order(TaskID, OrderIndex) bound [/string, /number].` |
| task_dependency | 2 | internal/core/defaults/schemas_campaign.mg | 101 | `Decl task_dependency(TaskID, DependsOnTaskID) bound [/string, /string].` |
| task_remediation_target | 3 | internal/core/defaults/schemas_campaign.mg | 105 | `Decl task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType) bound [/string, /string, /name].` |
| task_artifact | 4 | internal/core/defaults/schemas_campaign.mg | 109 | `Decl task_artifact(TaskID, ArtifactType, Path, Hash) bound [/string, /name, /string, /string].` |
| task_write_target | 2 | internal/core/defaults/schemas_campaign.mg | 113 | `Decl task_write_target(TaskID, Path) bound [/string, /string].` |
| task_write_path | 2 | internal/core/defaults/schemas_campaign.mg | 116 | `Decl task_write_path(TaskID, Path) bound [/string, /string].` |
| task_inference | 4 | internal/core/defaults/schemas_campaign.mg | 120 | `Decl task_inference(TaskID, InferredFrom, Confidence, Reasoning) bound [/string, /string, /number, /string].` |
| task_attempt | 4 | internal/core/defaults/schemas_campaign.mg | 124 | `Decl task_attempt(TaskID, AttemptNumber, Outcome, Timestamp) bound [/string, /number, /name, /number].` |
| task_error | 3 | internal/core/defaults/schemas_campaign.mg | 127 | `Decl task_error(TaskID, ErrorType, ErrorMessage) bound [/string, /name, /string].` |
| context_profile | 4 | internal/core/defaults/schemas_campaign.mg | 137 | `Decl context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns) bound [/string, /string, /string, /string].` |
| tool_in_list | 2 | internal/core/defaults/schemas_campaign.mg | 140 | `Decl tool_in_list(Tool, ToolList) bound [/name, /string].` |
| phase_context_atom | 3 | internal/core/defaults/schemas_campaign.mg | 144 | `Decl phase_context_atom(PhaseID, FactPredicate, ActivationBoost) bound [/string, /string, /number].` |
| context_compression | 4 | internal/core/defaults/schemas_campaign.mg | 148 | `Decl context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp) bound [/string, /string, /number, /number].` |
| context_window_state | 4 | internal/core/defaults/schemas_campaign.mg | 151 | `Decl context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization) bound [/string, /number, /number, /number].` |
| campaign_progress | 5 | internal/core/defaults/schemas_campaign.mg | 158 | `Decl campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks) bound [/string, /number, /number, /number, /number].` |
| campaign_completed | 2 | internal/core/defaults/schemas_campaign.mg | 162 | `Decl campaign_completed(CampaignID, Summary) bound [/string, /string].` |
| campaign_heartbeat | 2 | internal/core/defaults/schemas_campaign.mg | 165 | `Decl campaign_heartbeat(CampaignID, Timestamp) bound [/string, /number].` |
| task_retry_at | 2 | internal/core/defaults/schemas_campaign.mg | 168 | `Decl task_retry_at(TaskID, RetryAt) bound [/string, /number].` |
| task_in_backoff | 1 | internal/core/defaults/schemas_campaign.mg | 171 | `Decl task_in_backoff(TaskID) bound [/string].` |
| phase_has_backoff_task | 1 | internal/core/defaults/schemas_campaign.mg | 174 | `Decl phase_has_backoff_task(PhaseID) bound [/string].` |
| phase_checkpoint | 5 | internal/core/defaults/schemas_campaign.mg | 178 | `Decl phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp) bound [/string, /name, /name, /string, /number].` |
| campaign_milestone | 4 | internal/core/defaults/schemas_campaign.mg | 181 | `Decl campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt) bound [/string, /string, /string, /number].` |
| campaign_learning | 5 | internal/core/defaults/schemas_campaign.mg | 186 | `Decl campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt) bound [/string, /name, /string, /string, /number].` |
| replan_trigger | 3 | internal/core/defaults/schemas_campaign.mg | 194 | `Decl replan_trigger(CampaignID, Reason, TriggeredAt) bound [/string, /name, /number].` |
| plan_revision | 4 | internal/core/defaults/schemas_campaign.mg | 197 | `Decl plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp) bound [/string, /number, /string, /number].` |
| plan_validation_issue | 3 | internal/core/defaults/schemas_campaign.mg | 201 | `Decl plan_validation_issue(CampaignID, IssueType, Description) bound [/string, /name, /string].` |
| campaign_shard | 5 | internal/core/defaults/schemas_campaign.mg | 209 | `Decl campaign_shard(CampaignID, ShardID, ShardType, Task, Status) bound [/string, /string, /name, /string, /name].` |
| campaign_intent_capture | 5 | internal/core/defaults/schemas_campaign.mg | 213 | `Decl campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints) bound [/string, /string, /string, /name, /string].` |
| shard_result_event | 4 | internal/core/defaults/schemas_campaign.mg | 216 | `Decl shard_result_event(ShardID, ResultType, ResultData, Timestamp) bound [/string, /name, /string, /number].` |
| source_document | 4 | internal/core/defaults/schemas_campaign.mg | 225 | `Decl source_document(CampaignID, DocPath, DocType, ParsedAt) bound [/string, /string, /name, /number].` |
| doc_metadata | 5 | internal/core/defaults/schemas_campaign.mg | 228 | `Decl doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt) bound [/string, /string, /name, /number, /number].` |
| goal_topic | 2 | internal/core/defaults/schemas_campaign.mg | 230 | `Decl goal_topic(CampaignID, Topic) bound [/string, /string].` |
| doc_tag | 2 | internal/core/defaults/schemas_campaign.mg | 233 | `Decl doc_tag(Path, Tag) bound [/string, /string].` |
| doc_reference | 2 | internal/core/defaults/schemas_campaign.mg | 236 | `Decl doc_reference(FromPath, ToPath) bound [/string, /string].` |
| doc_layer | 3 | internal/core/defaults/schemas_campaign.mg | 239 | `Decl doc_layer(Path, Layer, Confidence) bound [/string, /name, /number].` |
| layer_priority | 2 | internal/core/defaults/schemas_campaign.mg | 242 | `Decl layer_priority(Layer, Priority) bound [/name, /number].` |
| layer_distance | 3 | internal/core/defaults/schemas_campaign.mg | 245 | `Decl layer_distance(LayerA, LayerB, Distance) bound [/name, /name, /number].` |
| doc_conflict | 3 | internal/core/defaults/schemas_campaign.mg | 248 | `Decl doc_conflict(DocPath, LayerA, LayerB) bound [/string, /name, /name].` |
| active_layer | 1 | internal/core/defaults/schemas_campaign.mg | 251 | `Decl active_layer(Layer) bound [/name].` |
| proposed_phase | 1 | internal/core/defaults/schemas_campaign.mg | 254 | `Decl proposed_phase(Layer) bound [/name].` |
| phase_dependency_generated | 2 | internal/core/defaults/schemas_campaign.mg | 257 | `Decl phase_dependency_generated(PhaseA, PhaseB) bound [/name, /name].` |
| phase_context_scope | 2 | internal/core/defaults/schemas_campaign.mg | 260 | `Decl phase_context_scope(Phase, DocPath) bound [/name, /string].` |
| source_requirement | 5 | internal/core/defaults/schemas_campaign.mg | 264 | `Decl source_requirement(CampaignID, ReqID, Description, Priority, Source) bound [/string, /string, /string, /number, /string].` |
| requirement_coverage | 2 | internal/core/defaults/schemas_campaign.mg | 268 | `Decl requirement_coverage(ReqID, TaskID) bound [/string, /string].` |
| current_campaign | 1 | internal/core/defaults/schemas_campaign.mg | 275 | `Decl current_campaign(CampaignID) bound [/string].` |
| current_phase | 1 | internal/core/defaults/schemas_campaign.mg | 278 | `Decl current_phase(PhaseID) bound [/string].` |
| next_campaign_task | 1 | internal/core/defaults/schemas_campaign.mg | 281 | `Decl next_campaign_task(TaskID) bound [/string].` |

### Top-level: schemas_codedom.mg (direct read)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| active_file | 1 | internal/core/defaults/schemas_codedom.mg | 20 | `Decl active_file(Path) bound [/string].` |
| file_in_scope | 4 | internal/core/defaults/schemas_codedom.mg | 24 | `Decl file_in_scope(Path, Hash, Language, LineCount) bound [/string, /string, /name, /number].` |
| code_element | 5 | internal/core/defaults/schemas_codedom.mg | 33 | `Decl code_element(Ref, ElemType, File, StartLine, EndLine) bound [/string, /name, /string, /number, /number].` |
| element_signature | 2 | internal/core/defaults/schemas_codedom.mg | 36 | `Decl element_signature(Ref, Signature) bound [/string, /string].` |
| element_body | 2 | internal/core/defaults/schemas_codedom.mg | 39 | `Decl element_body(Ref, BodyText) bound [/string, /string].` |
| element_parent | 2 | internal/core/defaults/schemas_codedom.mg | 42 | `Decl element_parent(Ref, ParentRef) bound [/string, /string].` |
| element_visibility | 2 | internal/core/defaults/schemas_codedom.mg | 45 | `Decl element_visibility(Ref, Visibility) bound [/string, /name].` |
| code_interactable | 2 | internal/core/defaults/schemas_codedom.mg | 48 | `Decl code_interactable(Ref, ActionType) bound [/string, /name].` |
| element_modified | 3 | internal/core/defaults/schemas_codedom.mg | 56 | `Decl element_modified(Ref, SessionID, Timestamp) bound [/string, /string, /number].` |
| lines_edited | 4 | internal/core/defaults/schemas_codedom.mg | 59 | `Decl lines_edited(File, StartLine, EndLine, SessionID) bound [/string, /number, /number, /string].` |
| lines_inserted | 4 | internal/core/defaults/schemas_codedom.mg | 62 | `Decl lines_inserted(File, AfterLine, LineCount, SessionID) bound [/string, /number, /number, /string].` |
| lines_deleted | 4 | internal/core/defaults/schemas_codedom.mg | 65 | `Decl lines_deleted(File, StartLine, EndLine, SessionID) bound [/string, /number, /number, /string].` |
| file_read | 3 | internal/core/defaults/schemas_codedom.mg | 68 | `Decl file_read(Path, SessionID, Timestamp) bound [/string, /string, /number].` |
| file_written | 4 | internal/core/defaults/schemas_codedom.mg | 71 | `Decl file_written(Path, Hash, SessionID, Timestamp) bound [/string, /string, /string, /number].` |
| in_scope | 1 | internal/core/defaults/schemas_codedom.mg | 78 | `Decl in_scope(File) bound [/string].` |
| editable | 1 | internal/core/defaults/schemas_codedom.mg | 81 | `Decl editable(Ref) bound [/string].` |
| function_in_scope | 3 | internal/core/defaults/schemas_codedom.mg | 84 | `Decl function_in_scope(Ref, File, Sig) bound [/string, /string, /string].` |
| method_of | 2 | internal/core/defaults/schemas_codedom.mg | 87 | `Decl method_of(MethodRef, StructRef) bound [/string, /string].` |
| code_contains | 2 | internal/core/defaults/schemas_codedom.mg | 90 | `Decl code_contains(Parent, Child) bound [/string, /string].` |
| safe_to_modify | 1 | internal/core/defaults/schemas_codedom.mg | 93 | `Decl safe_to_modify(Ref) bound [/string].` |
| requires_campaign | 1 | internal/core/defaults/schemas_codedom.mg | 96 | `Decl requires_campaign(Intent) bound [/string].` |
| code_edit_outcome | 4 | internal/core/defaults/schemas_codedom.mg | 99 | `Decl code_edit_outcome(Ref, EditType, Success, Timestamp) bound [/string, /name, /name, /number].` |
| proven_safe_edit | 2 | internal/core/defaults/schemas_codedom.mg | 102 | `Decl proven_safe_edit(Ref, EditType) bound [/string, /name].` |
| method_in_scope | 3 | internal/core/defaults/schemas_codedom.mg | 105 | `Decl method_in_scope(Ref, File, Sig) bound [/string, /string, /string].` |
| scope_refreshed | 1 | internal/core/defaults/schemas_codedom.mg | 108 | `Decl scope_refreshed(File) bound [/string].` |
| successful_edit | 2 | internal/core/defaults/schemas_codedom.mg | 111 | `Decl successful_edit(Ref, EditType) bound [/string, /name].` |
| failed_edit | 2 | internal/core/defaults/schemas_codedom.mg | 114 | `Decl failed_edit(Ref, EditType) bound [/string, /name].` |
| element_count_high | 0 | internal/core/defaults/schemas_codedom.mg | 117 | `Decl element_count_high() bound [].` |
| scope_open_failed | 2 | internal/core/defaults/schemas_codedom.mg | 124 | `Decl scope_open_failed(Path, Error) bound [/string, /string].` |
| scope_closed | 0 | internal/core/defaults/schemas_codedom.mg | 127 | `Decl scope_closed() bound [].` |
| parse_error | 3 | internal/core/defaults/schemas_codedom.mg | 130 | `Decl parse_error(File, Error, Timestamp) bound [/string, /string, /number].` |
| file_not_found | 2 | internal/core/defaults/schemas_codedom.mg | 133 | `Decl file_not_found(Path, Timestamp) bound [/string, /number].` |
| file_hash_mismatch | 3 | internal/core/defaults/schemas_codedom.mg | 136 | `Decl file_hash_mismatch(Path, ExpectedHash, ActualHash) bound [/string, /string, /string].` |
| element_stale | 2 | internal/core/defaults/schemas_codedom.mg | 139 | `Decl element_stale(Ref, Reason) bound [/string, /string].` |
| scope_refresh_failed | 2 | internal/core/defaults/schemas_codedom.mg | 142 | `Decl scope_refresh_failed(Path, Error) bound [/string, /string].` |
| encoding_issue | 2 | internal/core/defaults/schemas_codedom.mg | 145 | `Decl encoding_issue(Path, IssueType) bound [/string, /name].` |
| large_file_warning | 3 | internal/core/defaults/schemas_codedom.mg | 148 | `Decl large_file_warning(Path, LineCount, ByteSize) bound [/string, /number, /number].` |
| scope_operation | 4 | internal/core/defaults/schemas_codedom.mg | 156 | `Decl scope_operation(OpType, Path, Success, Timestamp) bound [/name, /string, /name, /number].` |
| edit_operation_event | 6 | internal/core/defaults/schemas_codedom.mg | 160 | `Decl edit_operation_event(OpType, Path, StartLine, EndLine, Success, Timestamp) bound [/name, /string, /number, /number, /name, /number].` |
| undo_available | 2 | internal/core/defaults/schemas_codedom.mg | 164 | `Decl undo_available(Path, OperationID) bound [/string, /string].` |
| file_modified_externally | 1 | internal/core/defaults/schemas_codedom.mg | 171 | `Decl file_modified_externally(Path) bound [/string].` |
| needs_scope_refresh | 0 | internal/core/defaults/schemas_codedom.mg | 174 | `Decl needs_scope_refresh() bound [].` |
| element_edit_blocked | 2 | internal/core/defaults/schemas_codedom.mg | 177 | `Decl element_edit_blocked(Ref, Reason) bound [/string, /string].` |
| generated_code | 3 | internal/core/defaults/schemas_codedom.mg | 186 | `Decl generated_code(File, Generator, Marker) bound [/string, /name, /string].` |
| api_client_function | 3 | internal/core/defaults/schemas_codedom.mg | 190 | `Decl api_client_function(Ref, Endpoint, Method) bound [/string, /string, /name].` |
| api_handler_function | 3 | internal/core/defaults/schemas_codedom.mg | 193 | `Decl api_handler_function(Ref, Route, Method) bound [/string, /string, /name].` |
| has_external_callers | 1 | internal/core/defaults/schemas_codedom.mg | 196 | `Decl has_external_callers(Ref) bound [/string].` |
| breaking_change_risk | 3 | internal/core/defaults/schemas_codedom.mg | 200 | `Decl breaking_change_risk(Ref, RiskLevel, Reason) bound [/string, /name, /string].` |
| mock_file | 2 | internal/core/defaults/schemas_codedom.mg | 203 | `Decl mock_file(TestFile, SourceFile) bound [/string, /string].` |
| interface_impl | 2 | internal/core/defaults/schemas_codedom.mg | 206 | `Decl interface_impl(StructRef, InterfaceRef) bound [/string, /string].` |
| cgo_code | 1 | internal/core/defaults/schemas_codedom.mg | 209 | `Decl cgo_code(File) bound [/string].` |
| build_tag | 2 | internal/core/defaults/schemas_codedom.mg | 212 | `Decl build_tag(File, Tag) bound [/string, /string].` |
| embed_directive | 2 | internal/core/defaults/schemas_codedom.mg | 215 | `Decl embed_directive(File, EmbedPath) bound [/string, /string].` |
| edit_unsafe | 2 | internal/core/defaults/schemas_codedom.mg | 222 | `Decl edit_unsafe(Ref, Reason) bound [/string, /string].` |
| suggest_update_mocks | 1 | internal/core/defaults/schemas_codedom.mg | 225 | `Decl suggest_update_mocks(Ref) bound [/string].` |
| signature_change_detected | 3 | internal/core/defaults/schemas_codedom.mg | 228 | `Decl signature_change_detected(Ref, OldSig, NewSig) bound [/string, /string, /string].` |
| requires_integration_test | 1 | internal/core/defaults/schemas_codedom.mg | 231 | `Decl requires_integration_test(Ref) bound [/string].` |
| requires_contract_check | 1 | internal/core/defaults/schemas_codedom.mg | 234 | `Decl requires_contract_check(Ref) bound [/string].` |
| api_edit_warning | 2 | internal/core/defaults/schemas_codedom.mg | 237 | `Decl api_edit_warning(Ref, Reason) bound [/string, /string].` |

### Top-level: schemas_codedom_polyglot.mg (direct read)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| go_struct | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 17 | `Decl go_struct(Ref) bound [/string].` |
| go_interface | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 20 | `Decl go_interface(Ref) bound [/string].` |
| go_tag | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 24 | `Decl go_tag(Ref, TagContent) bound [/string, /string].` |
| go_goroutine | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 27 | `Decl go_goroutine(Ref) bound [/string].` |
| go_uses_context | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 30 | `Decl go_uses_context(Ref) bound [/string].` |
| go_returns_error | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 33 | `Decl go_returns_error(Ref) bound [/string].` |
| py_class | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 42 | `Decl py_class(Ref) bound [/string].` |
| py_decorator | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 46 | `Decl py_decorator(Ref, DecoratorName) bound [/string, /string].` |
| py_async_def | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 49 | `Decl py_async_def(Ref) bound [/string].` |
| has_pydantic_base | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 52 | `Decl has_pydantic_base(Ref) bound [/string].` |
| py_typed_function | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 55 | `Decl py_typed_function(Ref) bound [/string].` |
| ts_class | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 64 | `Decl ts_class(Ref) bound [/string].` |
| ts_interface | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 67 | `Decl ts_interface(Ref) bound [/string].` |
| ts_interface_prop | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 71 | `Decl ts_interface_prop(Ref, PropName) bound [/string, /string].` |
| ts_type_alias | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 74 | `Decl ts_type_alias(Ref) bound [/string].` |
| ts_async_function | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 77 | `Decl ts_async_function(Ref) bound [/string].` |
| ts_component | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 80 | `Decl ts_component(Ref, ComponentName) bound [/string, /string].` |
| ts_hook | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 84 | `Decl ts_hook(Ref, HookName) bound [/string, /string].` |
| ts_extends | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 88 | `Decl ts_extends(Ref) bound [/string].` |
| ts_implements | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 91 | `Decl ts_implements(Ref) bound [/string].` |
| rs_struct | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 100 | `Decl rs_struct(Ref) bound [/string].` |
| rs_trait | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 103 | `Decl rs_trait(Ref) bound [/string].` |
| rs_async_fn | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 106 | `Decl rs_async_fn(Ref) bound [/string].` |
| rs_unsafe_block | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 109 | `Decl rs_unsafe_block(Ref) bound [/string].` |
| rs_returns_result | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 112 | `Decl rs_returns_result(Ref) bound [/string].` |
| rs_uses_unwrap | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 115 | `Decl rs_uses_unwrap(Ref) bound [/string].` |
| rs_derive | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 119 | `Decl rs_derive(Ref, DeriveName) bound [/string, /string].` |
| rs_serde_rename | 3 | internal/core/defaults/schemas_codedom_polyglot.mg | 123 | `Decl rs_serde_rename(Ref, FieldName, WireName) bound [/string, /string, /string].` |
| mg_decl | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 132 | `Decl mg_decl(Ref, PredicateName) bound [/string, /string].` |
| mg_rule | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 135 | `Decl mg_rule(Ref, HeadPredicate) bound [/string, /string].` |
| mg_fact | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 138 | `Decl mg_fact(Ref, PredicateName) bound [/string, /string].` |
| mg_query | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 141 | `Decl mg_query(Ref, PredicateName) bound [/string, /string].` |
| mg_recursive_rule | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 144 | `Decl mg_recursive_rule(Ref) bound [/string].` |
| mg_negation_rule | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 147 | `Decl mg_negation_rule(Ref) bound [/string].` |
| mg_aggregation_rule | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 150 | `Decl mg_aggregation_rule(Ref) bound [/string].` |
| is_data_contract | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 161 | `Decl is_data_contract(Ref) bound [/string].` |
| is_async_context | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 164 | `Decl is_async_context(Ref) bound [/string].` |
| wire_name | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 169 | `Decl wire_name(Ref, Name) bound [/string, /string].` |
| api_dependency | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 172 | `Decl api_dependency(BackendRef, FrontendRef) bound [/string, /string].` |
| is_ui_component | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 177 | `Decl is_ui_component(Ref) bound [/string].` |
| has_auth_guard | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 180 | `Decl has_auth_guard(Ref) bound [/string].` |
| potential_panic | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 184 | `Decl potential_panic(Ref) bound [/string].` |
| has_test_coverage | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 188 | `Decl has_test_coverage(Ref) bound [/string].` |
| cross_lang_refactor_target | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 191 | `Decl cross_lang_refactor_target(Ref) bound [/string].` |
| file_imports | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 200 | `Decl file_imports(Importer, Imported) bound [/string, /string].` |
| type_embeds | 2 | internal/core/defaults/schemas_codedom_polyglot.mg | 203 | `Decl type_embeds(Type, EmbeddedType) bound [/string, /string].` |
| plan_edit | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 206 | `Decl plan_edit(Ref) bound [/string].` |
| modified_file | 1 | internal/core/defaults/schemas_codedom_polyglot.mg | 209 | `Decl modified_file(File) bound [/string].` |

### Top-level: schemas_context.mg (direct read)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| context_relevant | 2 | internal/core/defaults/schemas_context.mg | 20 | `Decl context_relevant(Fact, Priority) bound [/string, /name].` |
| should_include_context | 2 | internal/core/defaults/schemas_context.mg | 26 | `Decl should_include_context(Fact, Priority) bound [/string, /name].` |
| context_reachable | 2 | internal/core/defaults/schemas_context.mg | 37 | `Decl context_reachable(File, HopLevel) bound [/string, /name].` |
| context_file_priority | 2 | internal/core/defaults/schemas_context.mg | 43 | `Decl context_file_priority(File, Priority) bound [/string, /name].` |
| turn_age_category | 2 | internal/core/defaults/schemas_context.mg | 58 | `Decl turn_age_category(TurnID, Category) bound [/string, /name].` |
| should_mask_observation | 1 | internal/core/defaults/schemas_context.mg | 64 | `Decl should_mask_observation(TurnID) bound [/string].` |
| should_preserve_reasoning | 1 | internal/core/defaults/schemas_context.mg | 68 | `Decl should_preserve_reasoning(TurnID) bound [/string].` |

### Top-level: schemas_safety.mg (direct read)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| permitted | 3 | internal/core/defaults/schemas_safety.mg | 15 | `Decl permitted(ActionType, Target, Payload) bound [/name, /string, /string].` |
| forbidden | 1 | internal/core/defaults/schemas_safety.mg | 18 | `Decl forbidden(ActionType) bound [/name].` |
| dangerous_action | 1 | internal/core/defaults/schemas_safety.mg | 23 | `Decl dangerous_action(ActionType) bound [/name].` |
| blocked_pattern | 1 | internal/core/defaults/schemas_safety.mg | 26 | `Decl blocked_pattern(Pattern) bound [/string].` |
| dangerous_content | 2 | internal/core/defaults/schemas_safety.mg | 33 | `Decl dangerous_content(ActionType, Payload) bound [/name, /string].` |
| admin_override | 1 | internal/core/defaults/schemas_safety.mg | 36 | `Decl admin_override(User) bound [/string].` |
| signed_approval | 1 | internal/core/defaults/schemas_safety.mg | 39 | `Decl signed_approval(ActionType) bound [/name].` |
| allowed_domain | 1 | internal/core/defaults/schemas_safety.mg | 42 | `Decl allowed_domain(Domain) bound [/string].` |
| network_permitted | 1 | internal/core/defaults/schemas_safety.mg | 45 | `Decl network_permitted(URL) bound [/string].` |
| security_violation_type | 1 | internal/core/defaults/schemas_safety.mg | 48 | `Decl security_violation_type(ViolationType) bound [/name].` |
| requires_permission | 1 | internal/core/defaults/schemas_safety.mg | 51 | `Decl requires_permission(ActionType) bound [/name].` |
| appeal_available | 4 | internal/core/defaults/schemas_safety.mg | 58 | `Decl appeal_available(ActionID, ActionType, Target, Reason) bound [/string, /name, /string, /name].` |
| appeal_pending | 4 | internal/core/defaults/schemas_safety.mg | 61 | `Decl appeal_pending(ActionID, ActionType, Justification, Timestamp) bound [/string, /name, /string, /number].` |
| appeal_granted | 4 | internal/core/defaults/schemas_safety.mg | 64 | `Decl appeal_granted(ActionID, ActionType, Approver, Timestamp) bound [/string, /name, /string, /number].` |
| appeal_denied | 4 | internal/core/defaults/schemas_safety.mg | 67 | `Decl appeal_denied(ActionID, ActionType, Reason, Timestamp) bound [/string, /name, /name, /number].` |
| temporary_override | 2 | internal/core/defaults/schemas_safety.mg | 72 | `Decl temporary_override(ActionType, ExpirationTimestamp) bound [/name, /number].` |
| has_temporary_override | 1 | internal/core/defaults/schemas_safety.mg | 75 | `Decl has_temporary_override(ActionType) bound [/name].` |
| user_requests_appeal | 3 | internal/core/defaults/schemas_safety.mg | 78 | `Decl user_requests_appeal(ActionID, Justification, Requester) bound [/string, /string, /string].` |
| active_override | 3 | internal/core/defaults/schemas_safety.mg | 81 | `Decl active_override(ActionType, Approver, ExpiresAt) bound [/name, /string, /number].` |
| appeal_history | 4 | internal/core/defaults/schemas_safety.mg | 84 | `Decl appeal_history(ActionID, Granted, Approver, Timestamp) bound [/string, /name, /string, /number].` |
| suggest_appeal | 1 | internal/core/defaults/schemas_safety.mg | 87 | `Decl suggest_appeal(ActionID) bound [/string].` |
| appeal_needs_review | 3 | internal/core/defaults/schemas_safety.mg | 90 | `Decl appeal_needs_review(ActionID, ActionType, Justification) bound [/string, /name, /string].` |
| has_active_override | 1 | internal/core/defaults/schemas_safety.mg | 93 | `Decl has_active_override(ActionType) bound [/name].` |
| appeal_denial_count | 1 | internal/core/defaults/schemas_safety.mg | 96 | `Decl appeal_denial_count(Count) bound [/number].` |
| appeal_grant_count | 2 | internal/core/defaults/schemas_safety.mg | 99 | `Decl appeal_grant_count(ActionType, Count) bound [/name, /number].` |
| excessive_appeal_denials | 0 | internal/core/defaults/schemas_safety.mg | 102 | `Decl excessive_appeal_denials() bound [].` |
| appeal_pattern_detected | 1 | internal/core/defaults/schemas_safety.mg | 105 | `Decl appeal_pattern_detected(ActionType) bound [/name].` |
| candidate_action | 1 | internal/core/defaults/schemas_safety.mg | 112 | `Decl candidate_action(ActionType) bound [/name].` |
| final_action | 1 | internal/core/defaults/schemas_safety.mg | 115 | `Decl final_action(ActionType) bound [/name].` |
| safety_check | 1 | internal/core/defaults/schemas_safety.mg | 118 | `Decl safety_check(ActionType) bound [/name].` |
| action_denied | 2 | internal/core/defaults/schemas_safety.mg | 121 | `Decl action_denied(ActionType, Reason) bound [/name, /name].` |
| learned_proposal | 1 | internal/core/defaults/schemas_safety.mg | 124 | `Decl learned_proposal(ActionType) bound [/name].` |
| blocked_learned_action_count | 1 | internal/core/defaults/schemas_safety.mg | 127 | `Decl blocked_learned_action_count(Count) bound [/number].` |
| git_history | 5 | internal/core/defaults/schemas_safety.mg | 134 | `Decl git_history(FilePath, CommitHash, Author, AgeDays, Message) bound [/string, /string, /string, /number, /string].` |
| git_state | 2 | internal/core/defaults/schemas_safety.mg | 137 | `Decl git_state(Attribute, Value) bound [/name, /string].` |
| churn_rate | 2 | internal/core/defaults/schemas_safety.mg | 140 | `Decl churn_rate(FilePath, ChangeFrequency) bound [/string, /number].` |
| current_user | 1 | internal/core/defaults/schemas_safety.mg | 143 | `Decl current_user(UserName) bound [/string].` |
| current_time | 1 | internal/core/defaults/schemas_safety.mg | 146 | `Decl current_time(Timestamp) bound [/number].` |
| recent_change_by_other | 1 | internal/core/defaults/schemas_safety.mg | 150 | `Decl recent_change_by_other(FilePath) bound [/string].` |
| chesterton_fence_warning | 2 | internal/core/defaults/schemas_safety.mg | 154 | `Decl chesterton_fence_warning(FilePath, Reason) bound [/string, /string].` |
| hypothetical | 1 | internal/core/defaults/schemas_safety.mg | 161 | `Decl hypothetical(Change) bound [/string].` |
| derives_from_hypothetical | 1 | internal/core/defaults/schemas_safety.mg | 164 | `Decl derives_from_hypothetical(Implication) bound [/string].` |
| shadow_state | 3 | internal/core/defaults/schemas_safety.mg | 167 | `Decl shadow_state(StateID, ActionID, IsValid) bound [/string, /string, /name].` |
| simulated_effect | 3 | internal/core/defaults/schemas_safety.mg | 170 | `Decl simulated_effect(ActionID, FactPredicate, FactArgs) bound [/string, /string, /string].` |
| safe_projection | 1 | internal/core/defaults/schemas_safety.mg | 174 | `Decl safe_projection(ActionID) bound [/string].` |
| projection_violation | 2 | internal/core/defaults/schemas_safety.mg | 177 | `Decl projection_violation(ActionID, ViolationType) bound [/string, /name].` |
| pending_mutation | 4 | internal/core/defaults/schemas_safety.mg | 184 | `Decl pending_mutation(MutationID, FilePath, OldContent, NewContent) bound [/string, /string, /string, /string].` |
| mutation_approved | 3 | internal/core/defaults/schemas_safety.mg | 187 | `Decl mutation_approved(MutationID, ApprovedBy, Timestamp) bound [/string, /string, /number].` |
| mutation_rejected | 3 | internal/core/defaults/schemas_safety.mg | 190 | `Decl mutation_rejected(MutationID, RejectedBy, Reason) bound [/string, /string, /string].` |
| requires_approval | 1 | internal/core/defaults/schemas_safety.mg | 194 | `Decl requires_approval(MutationID) bound [/string].` |

### Policy: verified via grep partitions (a-f/g-m/n-z + per-file)
| predicate | arity | file | line | raw |
|---|---|---|---|---|
| has_current_time | 0 | internal/core/defaults/policy/campaign_tasks.mg | 6 | `Decl has_current_time() bound [].` |
| edit_success_count | 2 | internal/core/defaults/policy/codedom_edit.mg | 159 | `Decl edit_success_count(EditType, Count).` |
| deny_edit | 2 | internal/core/defaults/policy/codedom_safety.mg | 18 | `Decl deny_edit(Ref, Reason).` |
| edit_warning | 2 | internal/core/defaults/policy/codedom_safety.mg | 19 | `Decl edit_warning(Ref, Reason).` |
| safe_to_edit | 1 | internal/core/defaults/policy/codedom_safety.mg | 20 | `Decl safe_to_edit(Ref).` |
| has_warnings | 1 | internal/core/defaults/policy/codedom_safety.mg | 21 | `Decl has_warnings(Ref).` |
| has_deny_edit | 1 | internal/core/defaults/policy/codedom_safety.mg | 22 | `Decl has_deny_edit(Ref).` |
| is_serialization_boundary | 1 | internal/core/defaults/policy/codedom_safety.mg | 25 | `Decl is_serialization_boundary(Ref).` |
| returns_error_type | 1 | internal/core/defaults/policy/codedom_safety.mg | 26 | `Decl returns_error_type(Ref).` |
| element_action | 2 | internal/core/defaults/policy/codedom_safety.mg | 27 | `Decl element_action(Action, Ref).` |
| task_has_multiple_targets | 1 | internal/core/defaults/policy/coder_classification.mg | 61 | `Decl task_has_multiple_targets(ID).` |
| task_is_architectural | 1 | internal/core/defaults/policy/coder_classification.mg | 67 | `Decl task_is_architectural(ID).` |
| instruction_mentions_architecture | 1 | internal/core/defaults/policy/coder_classification.mg | 77 | `Decl instruction_mentions_architecture(Instruction).` |
| has_implementation_edit | 0 | internal/core/defaults/policy/coder_safety.mg | 40 | `Decl has_implementation_edit() bound [].` |
| has_file_content | 1 | internal/core/defaults/policy/coder_workflow.mg | 13 | `Decl has_file_content(File).` |
| has_edit_block | 1 | internal/core/defaults/policy/coder_workflow.mg | 245 | `Decl has_edit_block(Reason).` |
| has_state_change | 2 | internal/core/defaults/policy/coder_workflow.mg | 271 | `Decl has_state_change(Current, Previous).` |
| state_is_stuck | 0 | internal/core/defaults/policy/coder_workflow.mg | 282 | `Decl state_is_stuck() bound [].` |
| intelligence_dependent_count | 2 | internal/core/defaults/policy/intelligence.mg | 54 | `Decl intelligence_dependent_count(Path, Count).` |
| active_campaign_id | 1 | internal/core/defaults/policy/intelligence.mg | 254 | `Decl active_campaign_id(CampaignID).` |
| atom_has_shard_match | 1 | internal/core/defaults/policy/jit_logic.mg | 11 | `Decl atom_has_shard_match(AtomID).` |
| atom_has_mode_match | 1 | internal/core/defaults/policy/jit_logic.mg | 12 | `Decl atom_has_mode_match(AtomID).` |
| atom_has_phase_match | 1 | internal/core/defaults/policy/jit_logic.mg | 13 | `Decl atom_has_phase_match(AtomID).` |
| atom_has_verb_match | 1 | internal/core/defaults/policy/jit_logic.mg | 14 | `Decl atom_has_verb_match(AtomID).` |
| atom_has_lang_match | 1 | internal/core/defaults/policy/jit_logic.mg | 15 | `Decl atom_has_lang_match(AtomID).` |
| atom_has_framework_match | 1 | internal/core/defaults/policy/jit_logic.mg | 16 | `Decl atom_has_framework_match(AtomID).` |
| atom_has_state_match | 1 | internal/core/defaults/policy/jit_logic.mg | 17 | `Decl atom_has_state_match(AtomID).` |
| atom_has_init_match | 1 | internal/core/defaults/policy/jit_logic.mg | 18 | `Decl atom_has_init_match(AtomID).` |
| atom_has_ouroboros_match | 1 | internal/core/defaults/policy/jit_logic.mg | 19 | `Decl atom_has_ouroboros_match(AtomID).` |
| atom_has_northstar_match | 1 | internal/core/defaults/policy/jit_logic.mg | 20 | `Decl atom_has_northstar_match(AtomID).` |
| atom_has_layer_match | 1 | internal/core/defaults/policy/jit_logic.mg | 21 | `Decl atom_has_layer_match(AtomID).` |
| has_successful_shard | 0 | internal/core/defaults/policy/jit_selection.mg | 158 | `Decl has_successful_shard() bound [].` |
| project_write_denied | 2 | internal/core/defaults/policy/projectdoc.mg | 32 | `Decl project_write_denied(Path, Reason) bound [/string, /string].` |
| high_score_trace_recall | 1 | internal/core/defaults/policy/prompt_context.mg | 5 | `Decl high_score_trace_recall(Summary).` |
| high_score_failure_recall | 1 | internal/core/defaults/policy/prompt_context.mg | 10 | `Decl high_score_failure_recall(Summary).` |
| high_score_learning_recall | 1 | internal/core/defaults/policy/prompt_context.mg | 15 | `Decl high_score_learning_recall(Description).` |
| context_effective_count | 2 | internal/core/defaults/policy/prompt_context.mg | 20 | `Decl context_effective_count(Atom, N).` |
| has_immediate_capability | 0 | internal/core/defaults/policy/prompt_northstar.mg | 119 | `Decl has_immediate_capability() bound [].` |
| has_unaddressed_high_risk | 0 | internal/core/defaults/policy/prompt_northstar.mg | 120 | `Decl has_unaddressed_high_risk() bound [].` |
| has_active_planner | 0 | internal/core/defaults/policy/prompt_northstar.mg | 136 | `Decl has_active_planner() bound [].` |
| has_active_coder | 0 | internal/core/defaults/policy/prompt_northstar.mg | 137 | `Decl has_active_coder() bound [].` |
| module_known | 1 | internal/core/defaults/policy/prompt_northstar.mg | 179 | `Decl module_known(ModulePath) bound [/string].` |
| module_has_own_purpose | 1 | internal/core/defaults/policy/prompt_northstar.mg | 180 | `Decl module_has_own_purpose(ModulePath) bound [/string].` |
| effective_module_purpose | 2 | internal/core/defaults/policy/prompt_northstar.mg | 181 | `Decl effective_module_purpose(ModulePath, Purpose) bound [/string, /string].` |
| current_understanding | 4 | internal/core/defaults/policy/schemas_perception_latency.mg | 12 | `Decl current_understanding(SemanticType, ActionType, Domain, ScopeLevel).` |
| llm_suggested_mode | 1 | internal/core/defaults/policy/schemas_perception_latency.mg | 16 | `Decl llm_suggested_mode(Mode).` |
| candidate_mode | 3 | internal/core/defaults/policy/schemas_perception_latency.mg | 20 | `Decl candidate_mode(Mode, Source, Priority).` |
| best_candidate_priority | 1 | internal/core/defaults/policy/schemas_perception_latency.mg | 24 | `Decl best_candidate_priority(MaxPriority).` |
| derived_mode | 1 | internal/core/defaults/policy/schemas_perception_latency.mg | 28 | `Decl derived_mode(Mode).` |
| derived_primary_shard | 1 | internal/core/defaults/policy/schemas_perception_latency.mg | 32 | `Decl derived_primary_shard(ShardID).` |
| derived_context_priority | 2 | internal/core/defaults/policy/schemas_perception_latency.mg | 36 | `Decl derived_context_priority(Category, Priority).` |
| derived_tool_priority | 2 | internal/core/defaults/policy/schemas_perception_latency.mg | 40 | `Decl derived_tool_priority(Tool, Priority).` |

### Remaining Unchecked / To Verify Locally (flagged, not synthesized)

*The following files exist under `internal/core/defaults/**/*.mg` but were not individually `read_file` this turn due to round budget (97 calls left, 1 round). Grep partitions hit 50-result cap so entries beyond that are unknown — re-run `rg -n "^\s*Decl" --glob "*.mg" internal/core/defaults` to complete:*

- `internal/core/defaults/policy/activation.mg` — 0 Decl verified (direct read showed 0; re-check)
- `internal/core/defaults/policy/autopoiesis.mg`, `bridge.mg`, `browser.mg`, `browser_honeypot.mg`, `campaign_autopoiesis.mg`, `campaign_context.mg`, `campaign_core.mg`, `campaign_phases.mg`, `campaign_planning.mg`, `capabilities.mg`, `clarification.mg`, `codedom_continuation.mg`, `codedom_core.mg`, `coder_build.mg`, `coder_campaign.mg`, `coder_context.mg`, `coder_diagnostics.mg`, `coder_impact.mg`, `coder_language.mg`, `coder_learning.mg`, `coder_observability.mg`, `coder_patterns.mg`, `coder_quality.mg`, `coder_tdd.mg`, `commit_gate.mg`, `constitution.mg`, `context_compilation.mg`, `data_flow.mg`, `delegation.mg`, `dreamer.mg`, `git_safety.mg`, `impact.mg`, `jit_config.mg`, `knowledge.mg`, `learning.mg`, `perception_routing.mg`, `prioritization.mg`, `routing_arbitration.mg`, `shadow_mode.mg`, `shards.mg`, `strategy.mg`, `system_autopoiesis.mg`, `system_config.mg`, `system_core.mg`, `system_ooda.mg`, `system_routing.mg`, `system_session.mg`, `system_shards.mg`, `system_world.mg`, `taxonomy_inference.mg` (contains `copular_state_intent/2`, `has_copular_state_intent/1`, `negated_verb/1` etc not yet tabulated), `taxonomy_qualifiers.mg`, `tdd_logic.mg`, `tdd_loop.mg`, `test_impact.mg` (`is_test_function/1`), `tool_routing.mg`, `trace_logic.mg` (`rule_metadata/2`), `validation.mg`, `verification.mg`
- `internal/core/defaults/schemas_*.mg` remainder: `schemas_execution.mg`, `schemas_intelligence.mg`, `schemas_intent.mg`, `schemas_knowledge.mg`, `schemas_learning.mg`, `schemas_mcp.mg`, `schemas_memory.mg`, `schemas_misc.mg`, `schemas_project.mg`, `schemas_projectdoc.mg`, `schemas_prompts.mg`, `schemas_reviewer.mg`, `schemas_state.mg`, `schemas_testing.mg`, `schemas_tools.mg`, `schemas_world.mg`, plus `selection_policy.mg`, `taxonomy.mg`, `tester.mg`, `topology_planner.mg`, `build_topology.mg` (0), `campaign_rules.mg` (0), `doc_taxonomy.mg` (0), `learned.mg` (0), `reviewer.mg` (partial grep only `a-f`/`n-z` slices), `schemas.mg` (0)
- `internal/core/defaults/schema/**/*.mg` — currently 0 Decl per broad grep; confirm with `rg` on `schema/` subtree
- Excluded per scope: `.agent/skills/**`, `.agents/skills/**`, `.claude/skills/**`, `GEMINI.md` examples — not part of `internal/core/defaults`

## Reproduction Commands (to close gaps)

```bash
# Full raw inventory without truncation (paste to local shell)
rg -n "^\s*Decl\s+[A-Za-z_][A-Za-z0-9_]*\s*\(" --glob "*.mg" internal/core/defaults > /tmp/decl_raw.txt
# Arity extraction (awk counts commas inside parens)
awk -F'Decl ' '{print $2}' /tmp/decl_raw.txt | awk -F'(' '{name=$1; rest=$2; sub(/\).*/,"",rest); if(rest=="") arity=0; else {n=split(rest,a,","); arity=n} print name","arity","$0}'
# Cross-file duplicate check (name/arity)
cut -d',' -f1,2 /tmp/decl_arity.csv | sort | uniq -d
```

## Files Referenced

* `internal/core/defaults/benchmarks.mg:14,19,23,27,30,31,34-38,49,59`
* `internal/core/defaults/chaos.mg:34-37,40,60-61,64,70,82-84,87-88,109-110,146-147,154,170-171,190,204,240,247`
* `internal/core/defaults/go_safety.mg:5-11`
* `internal/core/defaults/inference.mg:15,101,108,114`
* `internal/core/defaults/jit_compiler.mg:9-12,15-17,20-21,24-26,29-30`
* `internal/core/defaults/schemas_analysis.mg:15-208`
* `internal/core/defaults/schemas_browser.mg:6-77`
* `internal/core/defaults/schemas_campaign.mg:21-281`
* `internal/core/defaults/schemas_codedom.mg:20-237`
* `internal/core/defaults/schemas_codedom_polyglot.mg:17-209`
* `internal/core/defaults/schemas_context.mg:20-68`
* `internal/core/defaults/schemas_dreamer.mg:14-122`
* `internal/core/defaults/schemas_safety.mg:15-194`
* `internal/core/defaults/policy/campaign_tasks.mg:6` etc (see table)
* `Docs/architecture/mangle/IMPLEMENTED_SPEC.md:558` — regex note
* `internal/core/defaults/schema_duplicate_decl_test.go` — duplicate key logic
* `internal/core/defaults/predicate_corpus.go` — seen-dedup
