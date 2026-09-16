# Uplift Ledger

File-by-file: uplift, wiring, .mg correctness, modularity, brutal behavioral integration tests.
Checked = audited and uplifted (or verified sound) with behavioral coverage.

## Go files (2354)
### .agent (5)
- [ ] `.agent/skills/log-analyzer/scripts/logquery/main.go`
- [ ] `.agent/skills/mangle-programming/assets/go-integration/main.go`
- [ ] `.agent/skills/rod-builder/scripts/chrome_launcher.go`
- [ ] `.agent/skills/rod-builder/scripts/scraper_template.go`
- [ ] `.agent/skills/rod-builder/scripts/session_manager.go`

### .agents (7)
- [ ] `.agents/skills/antigravity-auth/assets/templates/auth_handler.go`
- [ ] `.agents/skills/antigravity-auth/assets/templates/client.go`
- [ ] `.agents/skills/log-analyzer/scripts/logquery/main.go`
- [ ] `.agents/skills/mangle-programming/assets/go-integration/main.go`
- [ ] `.agents/skills/rod-builder/scripts/chrome_launcher.go`
- [ ] `.agents/skills/rod-builder/scripts/scraper_template.go`
- [ ] `.agents/skills/rod-builder/scripts/session_manager.go`

### .claude (7)
- [ ] `.claude/skills/antigravity-auth/assets/templates/auth_handler.go`
- [ ] `.claude/skills/antigravity-auth/assets/templates/client.go`
- [ ] `.claude/skills/log-analyzer/scripts/logquery/main.go`
- [ ] `.claude/skills/mangle-programming/assets/go-integration/main.go`
- [ ] `.claude/skills/rod-builder/scripts/chrome_launcher.go`
- [ ] `.claude/skills/rod-builder/scripts/scraper_template.go`
- [ ] `.claude/skills/rod-builder/scripts/session_manager.go`

### .codex (5)
- [ ] `.codex/skills/log-analyzer/scripts/logquery/main.go`
- [ ] `.codex/skills/mangle-programming/assets/go-integration/main.go`
- [ ] `.codex/skills/rod-builder/scripts/chrome_launcher.go`
- [ ] `.codex/skills/rod-builder/scripts/scraper_template.go`
- [ ] `.codex/skills/rod-builder/scripts/session_manager.go`

### .gemini (1)
- [ ] `.gemini/skills/log-analyzer/scripts/logquery/main.go`

### .nerd (18)
- [ ] `.nerd/marathon-supervision/negative-kernel_eval.go`
- [ ] `.nerd/marathon-supervision/negative-risk_scoring.go`
- [ ] `.nerd/marathon-supervision/negative-rule_court.go`
- [ ] `.nerd/marathon-supervision/negative-test_output_detector.go`
- [ ] `.nerd/marathon-supervision/persist_compact_learning.go`
- [ ] `.nerd/marathon-supervision/phase3-learning-negative/budget.go`
- [ ] `.nerd/marathon-supervision/profile_runtime.go`
- [ ] `.nerd/marathon-supervision/rejected-proposal/decomposer_planning.go`
- [ ] `.nerd/marathon-supervision/rejected-proposal/orchestrator_task_handlers.go`
- [ ] `.nerd/marathon-supervision/rejected-proposal/pause_durable.go`
- [ ] `.nerd/marathon-supervision/rejected-proposal/task_effect_contract.go`
- [ ] `.nerd/marathon-supervision/snapshot_helper.go`
- [ ] `.nerd/tools/count_mangle_predicates_given_a_directory_path_s.go`
- [ ] `.nerd/tools/count_mangle_predicates_given_a_directory_path_s_test.go`
- [ ] `.nerd/tools/count_the_number_of_mangle_decl_statements_in_a.go`
- [ ] `.nerd/tools/count_the_number_of_mangle_decl_statements_in_a_test.go`
- [ ] `.nerd/tools/given_a_file_path_as_input_return_the_total_numb.go`
- [ ] `.nerd/tools/given_a_file_path_as_input_return_the_total_numb_test.go`

### cmd/nerd (247)
- [ ] `cmd/nerd/acceptance_args_test.go`
- [ ] `cmd/nerd/apikey.go`
- [ ] `cmd/nerd/apikey_test.go`
- [ ] `cmd/nerd/campaign_outcome_test.go`
- [ ] `cmd/nerd/chat/activity_pulse_test.go`
- [ ] `cmd/nerd/chat/admission_error.go`
- [ ] `cmd/nerd/chat/admission_error_test.go`
- [ ] `cmd/nerd/chat/agent_wizard.go`
- [ ] `cmd/nerd/chat/agent_wizard_test.go`
- [ ] `cmd/nerd/chat/async_test.go`
- [ ] `cmd/nerd/chat/boot_benchmark_test.go`
- [ ] `cmd/nerd/chat/browser_test.go`
- [ ] `cmd/nerd/chat/campaign.go`
- [ ] `cmd/nerd/chat/campaign_assault.go`
- [ ] `cmd/nerd/chat/campaign_assault_test.go`
- [ ] `cmd/nerd/chat/campaign_consultation_adapter.go`
- [ ] `cmd/nerd/chat/campaign_jit_provider.go`
- [ ] `cmd/nerd/chat/campaign_recurse.go`
- [ ] `cmd/nerd/chat/campaign_recurse_test.go`
- [ ] `cmd/nerd/chat/campaign_risk_test.go`
- [ ] `cmd/nerd/chat/chat_loop_contract_e2e_test.go`
- [ ] `cmd/nerd/chat/cmd_explain.go`
- [ ] `cmd/nerd/chat/cmd_explain_test.go`
- [ ] `cmd/nerd/chat/command_categories.go`
- [ ] `cmd/nerd/chat/command_categories_test.go`
- [ ] `cmd/nerd/chat/commands.go`
- [ ] `cmd/nerd/chat/commands_evolution.go`
- [ ] `cmd/nerd/chat/commands_handlers.go`
- [ ] `cmd/nerd/chat/commands_handlers_analysis.go`
- [ ] `cmd/nerd/chat/commands_handlers_evolution.go`
- [ ] `cmd/nerd/chat/commands_handlers_features.go`
- [ ] `cmd/nerd/chat/commands_handlers_features_test.go`
- [ ] `cmd/nerd/chat/commands_handlers_files.go`
- [ ] `cmd/nerd/chat/commands_handlers_misc.go`
- [ ] `cmd/nerd/chat/commands_test.go`
- [ ] `cmd/nerd/chat/commands_tools.go`
- [ ] `cmd/nerd/chat/config_path.go`
- [ ] `cmd/nerd/chat/config_wizard.go`
- [ ] `cmd/nerd/chat/config_wizard_save_test.go`
- [ ] `cmd/nerd/chat/config_wizard_steps.go`
- [ ] `cmd/nerd/chat/continuation_done_test.go`
- [ ] `cmd/nerd/chat/delegation.go`
- [ ] `cmd/nerd/chat/delegation_modes.go`
- [ ] `cmd/nerd/chat/delegation_multistep.go`
- [ ] `cmd/nerd/chat/delegation_roundtrip_test.go`
- [ ] `cmd/nerd/chat/delegation_routing.go`
- [ ] `cmd/nerd/chat/delegation_routing_resolve_test.go`
- [ ] `cmd/nerd/chat/findings_test.go`
- [ ] `cmd/nerd/chat/glass_box.go`
- [ ] `cmd/nerd/chat/glass_box_stream_test.go`
- [ ] `cmd/nerd/chat/harness_test.go`
- [ ] `cmd/nerd/chat/help_renderer.go`
- [ ] `cmd/nerd/chat/helpers.go`
- [ ] `cmd/nerd/chat/helpers_articulation.go`
- [ ] `cmd/nerd/chat/helpers_files.go`
- [ ] `cmd/nerd/chat/helpers_scan.go`
- [ ] `cmd/nerd/chat/helpers_test.go`
- [ ] `cmd/nerd/chat/helpers_tools.go`
- [ ] `cmd/nerd/chat/ingest.go`
- [ ] `cmd/nerd/chat/integration_test.go`
- [ ] `cmd/nerd/chat/knowledge_format.go`
- [ ] `cmd/nerd/chat/knowledge_synthesis_test.go`
- [ ] `cmd/nerd/chat/live_integration_test.go`
- [ ] `cmd/nerd/chat/live_kernel_test.go`
- [ ] `cmd/nerd/chat/model.go`
- [ ] `cmd/nerd/chat/model_handlers.go`
- [ ] `cmd/nerd/chat/model_helpers.go`
- [ ] `cmd/nerd/chat/model_helpers_test.go`
- [ ] `cmd/nerd/chat/model_key_handler.go`
- [ ] `cmd/nerd/chat/model_lifecycle.go`
- [ ] `cmd/nerd/chat/model_session_context.go`
- [ ] `cmd/nerd/chat/model_types.go`
- [ ] `cmd/nerd/chat/model_update.go`
- [ ] `cmd/nerd/chat/model_update_persist_test.go`
- [ ] `cmd/nerd/chat/multistep_corpus.go`
- [ ] `cmd/nerd/chat/multistep_decomposer.go`
- [ ] `cmd/nerd/chat/northstar_adapter_test.go`
- [ ] `cmd/nerd/chat/northstar_llm.go`
- [ ] `cmd/nerd/chat/northstar_llm_live_test.go`
- [ ] `cmd/nerd/chat/northstar_navigation.go`
- [ ] `cmd/nerd/chat/northstar_persistence.go`
- [ ] `cmd/nerd/chat/northstar_persistence_test.go`
- [ ] `cmd/nerd/chat/northstar_types.go`
- [ ] `cmd/nerd/chat/northstar_types_test.go`
- [ ] `cmd/nerd/chat/northstar_utils.go`
- [ ] `cmd/nerd/chat/northstar_utils_test.go`
- [ ] `cmd/nerd/chat/northstar_wizard.go`
- [ ] `cmd/nerd/chat/onboarding_wizard.go`
- [ ] `cmd/nerd/chat/perception_regression_test.go`
- [ ] `cmd/nerd/chat/performance_test.go`
- [ ] `cmd/nerd/chat/persistence.go`
- [ ] `cmd/nerd/chat/process.go`
- [ ] `cmd/nerd/chat/process_continuation.go`
- [ ] `cmd/nerd/chat/process_dream.go`
- [ ] `cmd/nerd/chat/process_dream_delegation.go`
- [ ] `cmd/nerd/chat/process_dream_parsing.go`
- [ ] `cmd/nerd/chat/process_follow_up.go`
- [ ] `cmd/nerd/chat/process_helpers.go`
- [ ] `cmd/nerd/chat/process_knowledge.go`
- [ ] `cmd/nerd/chat/process_seed.go`
- [ ] `cmd/nerd/chat/process_sync.go`
- [ ] `cmd/nerd/chat/process_test.go`
- [ ] `cmd/nerd/chat/reembed.go`
- [ ] `cmd/nerd/chat/reflection.go`
- [ ] `cmd/nerd/chat/review_aggregator.go`
- [ ] `cmd/nerd/chat/review_aggregator_specialists_test.go`
- [ ] `cmd/nerd/chat/review_aggregator_test.go`
- [ ] `cmd/nerd/chat/routing_arbitration_roundtrip_test.go`
- [ ] `cmd/nerd/chat/routing_performance_contract_e2e_test.go`
- [ ] `cmd/nerd/chat/scan_identity_test.go`
- [ ] `cmd/nerd/chat/session.go`
- [ ] `cmd/nerd/chat/session_adapters.go`
- [ ] `cmd/nerd/chat/session_adapters_test.go`
- [ ] `cmd/nerd/chat/session_boot_helpers.go`
- [ ] `cmd/nerd/chat/session_functions_test.go`
- [ ] `cmd/nerd/chat/session_language_test.go`
- [ ] `cmd/nerd/chat/session_persistence.go`
- [ ] `cmd/nerd/chat/session_persistence_test.go`
- [ ] `cmd/nerd/chat/session_shared_boot.go`
- [ ] `cmd/nerd/chat/shadow.go`
- [ ] `cmd/nerd/chat/shadow_test.go`
- [ ] `cmd/nerd/chat/specialist_policy_test.go`
- [ ] `cmd/nerd/chat/task_routing_arbitration_e2e_test.go`
- [ ] `cmd/nerd/chat/test_state_test.go`
- [ ] `cmd/nerd/chat/testutil_test.go`
- [ ] `cmd/nerd/chat/tips.go`
- [ ] `cmd/nerd/chat/tui_frame_contract_e2e_test.go`
- [ ] `cmd/nerd/chat/update_test.go`
- [ ] `cmd/nerd/chat/view.go`
- [ ] `cmd/nerd/chat/warnings_render_order_test.go`
- [ ] `cmd/nerd/chat/welcome.go`
- [ ] `cmd/nerd/chat/wizard_test.go`
- [ ] `cmd/nerd/chat/yolo.go`
- [ ] `cmd/nerd/chat/yolo_kernel_test.go`
- [ ] `cmd/nerd/chat/yolo_test.go`
- [ ] `cmd/nerd/cli_test.go`
- [ ] `cmd/nerd/cmd_advanced.go`
- [ ] `cmd/nerd/cmd_advanced_dream_test.go`
- [ ] `cmd/nerd/cmd_audit.go`
- [ ] `cmd/nerd/cmd_audit_test.go`
- [ ] `cmd/nerd/cmd_auth.go`
- [ ] `cmd/nerd/cmd_browser.go`
- [ ] `cmd/nerd/cmd_browser_config_test.go`
- [ ] `cmd/nerd/cmd_browser_snapshot_test.go`
- [ ] `cmd/nerd/cmd_campaign.go`
- [ ] `cmd/nerd/cmd_campaign_assault.go`
- [ ] `cmd/nerd/cmd_campaign_assault_test.go`
- [ ] `cmd/nerd/cmd_campaign_cortex_test.go`
- [ ] `cmd/nerd/cmd_campaign_journal.go`
- [ ] `cmd/nerd/cmd_campaign_recurse.go`
- [ ] `cmd/nerd/cmd_campaign_recurse_test.go`
- [ ] `cmd/nerd/cmd_campaign_resume_boot_test.go`
- [ ] `cmd/nerd/cmd_campaign_resume_select_test.go`
- [ ] `cmd/nerd/cmd_campaign_tool_budget_test.go`
- [ ] `cmd/nerd/cmd_chat.go`
- [ ] `cmd/nerd/cmd_chat_test.go`
- [ ] `cmd/nerd/cmd_context_stats.go`
- [ ] `cmd/nerd/cmd_context_stats_test.go`
- [ ] `cmd/nerd/cmd_debug.go`
- [ ] `cmd/nerd/cmd_direct_actions.go`
- [ ] `cmd/nerd/cmd_direct_actions_heartbeat_test.go`
- [ ] `cmd/nerd/cmd_direct_actions_root_test.go`
- [ ] `cmd/nerd/cmd_dream_learning_test.go`
- [ ] `cmd/nerd/cmd_features.go`
- [ ] `cmd/nerd/cmd_features_test.go`
- [ ] `cmd/nerd/cmd_flags_test.go`
- [ ] `cmd/nerd/cmd_init_scan.go`
- [ ] `cmd/nerd/cmd_init_scan_test.go`
- [ ] `cmd/nerd/cmd_instruction.go`
- [ ] `cmd/nerd/cmd_instruction_context_test.go`
- [ ] `cmd/nerd/cmd_instruction_fact_test.go`
- [ ] `cmd/nerd/cmd_instruction_guard_fix_test.go`
- [ ] `cmd/nerd/cmd_instruction_prohibition_test.go`
- [ ] `cmd/nerd/cmd_instruction_stopwords_test.go`
- [ ] `cmd/nerd/cmd_instruction_subtask_test.go`
- [ ] `cmd/nerd/cmd_interactive.go`
- [ ] `cmd/nerd/cmd_interactive_test.go`
- [ ] `cmd/nerd/cmd_knowledge.go`
- [ ] `cmd/nerd/cmd_logs.go`
- [ ] `cmd/nerd/cmd_mangle_check.go`
- [ ] `cmd/nerd/cmd_mangle_lsp.go`
- [ ] `cmd/nerd/cmd_mcp_select.go`
- [ ] `cmd/nerd/cmd_mcp_select_test.go`
- [ ] `cmd/nerd/cmd_meter.go`
- [ ] `cmd/nerd/cmd_meter_test.go`
- [ ] `cmd/nerd/cmd_northstar.go`
- [ ] `cmd/nerd/cmd_query.go`
- [ ] `cmd/nerd/cmd_regression.go`
- [ ] `cmd/nerd/cmd_regression_test.go`
- [ ] `cmd/nerd/cmd_retrieve.go`
- [ ] `cmd/nerd/cmd_retrieve_test.go`
- [ ] `cmd/nerd/cmd_sessions.go`
- [ ] `cmd/nerd/cmd_snapshot.go`
- [ ] `cmd/nerd/cmd_snapshot_test.go`
- [ ] `cmd/nerd/cmd_spawn.go`
- [ ] `cmd/nerd/cmd_swebench.go`
- [ ] `cmd/nerd/cmd_swebench_test.go`
- [ ] `cmd/nerd/cmd_systems.go`
- [ ] `cmd/nerd/cmd_systems_mcp_test.go`
- [ ] `cmd/nerd/cmd_test_context.go`
- [ ] `cmd/nerd/cmd_transparency.go`
- [ ] `cmd/nerd/cmd_usage.go`
- [ ] `cmd/nerd/cmd_world.go`
- [ ] `cmd/nerd/cmd_world_test.go`
- [ ] `cmd/nerd/dom_apply_cmd.go`
- [ ] `cmd/nerd/dom_cmd.go`
- [ ] `cmd/nerd/dom_replace_cmd.go`
- [ ] `cmd/nerd/dom_replace_cmd_test.go`
- [ ] `cmd/nerd/dom_utils.go`
- [ ] `cmd/nerd/dom_utils_test.go`
- [ ] `cmd/nerd/embedding_cmd.go`
- [ ] `cmd/nerd/main.go`
- [ ] `cmd/nerd/main_test.go`
- [ ] `cmd/nerd/parent_group_test.go`
- [ ] `cmd/nerd/pending_action.go`
- [ ] `cmd/nerd/pending_action_failure_test.go`
- [ ] `cmd/nerd/stats.go`
- [ ] `cmd/nerd/system_results.go`
- [ ] `cmd/nerd/ui/autopoiesis_page.go`
- [ ] `cmd/nerd/ui/campaign_page.go`
- [ ] `cmd/nerd/ui/debounce.go`
- [ ] `cmd/nerd/ui/debounce_test.go`
- [ ] `cmd/nerd/ui/diffview.go`
- [ ] `cmd/nerd/ui/diffview_scrolling_test.go`
- [ ] `cmd/nerd/ui/diffview_test.go`
- [ ] `cmd/nerd/ui/jit_page.go`
- [ ] `cmd/nerd/ui/jit_page_test.go`
- [ ] `cmd/nerd/ui/keyboard_navigation_test.go`
- [ ] `cmd/nerd/ui/layout.go`
- [ ] `cmd/nerd/ui/pages_test.go`
- [ ] `cmd/nerd/ui/render_cache.go`
- [ ] `cmd/nerd/ui/render_cache_benchmark_test.go`
- [ ] `cmd/nerd/ui/render_cache_test.go`
- [ ] `cmd/nerd/ui/resources.go`
- [ ] `cmd/nerd/ui/shard_page.go`
- [ ] `cmd/nerd/ui/simple_table.go`
- [ ] `cmd/nerd/ui/simple_table_test.go`
- [ ] `cmd/nerd/ui/splitpane.go`
- [ ] `cmd/nerd/ui/splitpane_filter_test.go`
- [ ] `cmd/nerd/ui/splitpane_test.go`
- [ ] `cmd/nerd/ui/styles.go`
- [ ] `cmd/nerd/ui/styles_json_test.go`
- [ ] `cmd/nerd/ui/styles_options_test.go`
- [ ] `cmd/nerd/ui/styles_test.go`
- [ ] `cmd/nerd/ui/usage_page.go`
- [ ] `cmd/nerd/ui/word_diff_test.go`
- [ ] `cmd/nerd/ui/word_highlight_test.go`

### cmd/query-kb (3)
- [ ] `cmd/query-kb/deep_query.go`
- [ ] `cmd/query-kb/main.go`
- [ ] `cmd/query-kb/main_test.go`

### cmd/tools/action_linter (2)
- [ ] `cmd/tools/action_linter/main.go`
- [ ] `cmd/tools/action_linter/main_test.go`

### cmd/tools/audit_committed_binaries (1)
- [ ] `cmd/tools/audit_committed_binaries/main.go`

### cmd/tools/audit_json_errors (2)
- [ ] `cmd/tools/audit_json_errors/main.go`
- [ ] `cmd/tools/audit_json_errors/main_test.go`

### cmd/tools/audit_test_bodies (2)
- [ ] `cmd/tools/audit_test_bodies/main.go`
- [ ] `cmd/tools/audit_test_bodies/main_test.go`

### cmd/tools/audit_tiebreak (2)
- [ ] `cmd/tools/audit_tiebreak/main.go`
- [ ] `cmd/tools/audit_tiebreak/main_test.go`

### cmd/tools/change_benchmark (1)
- [ ] `cmd/tools/change_benchmark/main.go`

### cmd/tools/corpus_builder (5)
- [ ] `cmd/tools/corpus_builder/extract_test.go`
- [ ] `cmd/tools/corpus_builder/helpers_test.go`
- [ ] `cmd/tools/corpus_builder/main.go`
- [ ] `cmd/tools/corpus_builder/main_test.go`
- [ ] `cmd/tools/corpus_builder/sqlite_vec.go`

### cmd/tools/mangle_check (2)
- [ ] `cmd/tools/mangle_check/inspect_clause.go`
- [ ] `cmd/tools/mangle_check/inspect_clause_test.go`

### cmd/tools/predicate_corpus_builder (5)
- [ ] `cmd/tools/predicate_corpus_builder/check.go`
- [ ] `cmd/tools/predicate_corpus_builder/main.go`
- [ ] `cmd/tools/predicate_corpus_builder/main_test.go`
- [ ] `cmd/tools/predicate_corpus_builder/parse_test.go`
- [ ] `cmd/tools/predicate_corpus_builder/schema_parsing.go`

### cmd/tools/prompt_builder (5)
- [ ] `cmd/tools/prompt_builder/createdb_test.go`
- [ ] `cmd/tools/prompt_builder/helpers_test.go`
- [ ] `cmd/tools/prompt_builder/main.go`
- [ ] `cmd/tools/prompt_builder/main_test.go`
- [ ] `cmd/tools/prompt_builder/sqlite_vec.go`

### cmd/tools/validate_prompt_atoms (4)
- [ ] `cmd/tools/validate_prompt_atoms/corpus_parity_test.go`
- [ ] `cmd/tools/validate_prompt_atoms/main.go`
- [ ] `cmd/tools/validate_prompt_atoms/main_test.go`
- [ ] `cmd/tools/validate_prompt_atoms/validate_test.go`

### cmd/tools/verify_taxonomy (2)
- [ ] `cmd/tools/verify_taxonomy/main.go`
- [ ] `cmd/tools/verify_taxonomy/main_test.go`

### internal/articulation (20)
- [ ] `internal/articulation/emitter.go`
- [ ] `internal/articulation/emitter_boundary_test.go`
- [ ] `internal/articulation/emitter_extra_test.go`
- [ ] `internal/articulation/emitter_helpers_test.go`
- [ ] `internal/articulation/emitter_placeholder_test.go`
- [ ] `internal/articulation/emitter_test.go`
- [ ] `internal/articulation/emitter_warnings_test.go`
- [ ] `internal/articulation/json_scanner.go`
- [ ] `internal/articulation/json_scanner_test.go`
- [ ] `internal/articulation/kernel_context.go`
- [ ] `internal/articulation/language_precedence_test.go`
- [ ] `internal/articulation/prompt_assembler.go`
- [ ] `internal/articulation/prompt_assembler_adapter.go`
- [ ] `internal/articulation/prompt_assembler_ouroboros_test.go`
- [ ] `internal/articulation/prompt_assembler_test.go`
- [ ] `internal/articulation/protocol_types.go`
- [ ] `internal/articulation/schema.go`
- [ ] `internal/articulation/session_context_bounds_test.go`
- [ ] `internal/articulation/stream_parser.go`
- [ ] `internal/articulation/stream_parser_test.go`

### internal/atomicfile (6)
- [ ] `internal/atomicfile/atomicfile.go`
- [ ] `internal/atomicfile/atomicfile_test.go`
- [ ] `internal/atomicfile/open_other.go`
- [ ] `internal/atomicfile/open_windows.go`
- [ ] `internal/atomicfile/replace_other.go`
- [ ] `internal/atomicfile/replace_windows.go`

### internal/autopoiesis (96)
- [ ] `internal/autopoiesis/agent_handoff_test.go`
- [ ] `internal/autopoiesis/analysis_heuristics_test.go`
- [ ] `internal/autopoiesis/autopoiesis.go`
- [ ] `internal/autopoiesis/autopoiesis_agents.go`
- [ ] `internal/autopoiesis/autopoiesis_agents_test.go`
- [ ] `internal/autopoiesis/autopoiesis_analysis.go`
- [ ] `internal/autopoiesis/autopoiesis_analysis_kernel_test.go`
- [ ] `internal/autopoiesis/autopoiesis_delegation.go`
- [ ] `internal/autopoiesis/autopoiesis_feedback.go`
- [ ] `internal/autopoiesis/autopoiesis_helpers.go`
- [ ] `internal/autopoiesis/autopoiesis_kernel.go`
- [ ] `internal/autopoiesis/autopoiesis_orchestrator.go`
- [ ] `internal/autopoiesis/autopoiesis_profiles.go`
- [ ] `internal/autopoiesis/autopoiesis_profiles_test.go`
- [ ] `internal/autopoiesis/autopoiesis_tools.go`
- [ ] `internal/autopoiesis/autopoiesis_types.go`
- [ ] `internal/autopoiesis/build_env_threading_test.go`
- [ ] `internal/autopoiesis/checker.go`
- [ ] `internal/autopoiesis/checker_failclosed_test.go`
- [ ] `internal/autopoiesis/checker_test.go`
- [ ] `internal/autopoiesis/complexity.go`
- [ ] `internal/autopoiesis/complexity_test.go`
- [ ] `internal/autopoiesis/delegation_test.go`
- [ ] `internal/autopoiesis/execution_policy_test.go`
- [ ] `internal/autopoiesis/feedback.go`
- [ ] `internal/autopoiesis/feedback_load_test.go`
- [ ] `internal/autopoiesis/feedback_test.go`
- [ ] `internal/autopoiesis/helpers_coverage_test.go`
- [ ] `internal/autopoiesis/kernel_listener_lifecycle_test.go`
- [ ] `internal/autopoiesis/kernel_listener_wiring_test.go`
- [ ] `internal/autopoiesis/kernel_parity_test.go`
- [ ] `internal/autopoiesis/metrics.go`
- [ ] `internal/autopoiesis/metrics_test.go`
- [ ] `internal/autopoiesis/mocks_test.go`
- [ ] `internal/autopoiesis/orchestrator_test.go`
- [ ] `internal/autopoiesis/ouroboros.go`
- [ ] `internal/autopoiesis/ouroboros_multistage_e2e_test.go`
- [ ] `internal/autopoiesis/ouroboros_panic_test.go`
- [ ] `internal/autopoiesis/ouroboros_test.go`
- [ ] `internal/autopoiesis/ouroboros_tool_test.go`
- [ ] `internal/autopoiesis/ouroboros_version_test.go`
- [ ] `internal/autopoiesis/ouroboros_wrapper_test.go`
- [ ] `internal/autopoiesis/panic_maker.go`
- [ ] `internal/autopoiesis/patterns.go`
- [ ] `internal/autopoiesis/patterns_coverage_test.go`
- [ ] `internal/autopoiesis/persistence.go`
- [ ] `internal/autopoiesis/persistence_test.go`
- [ ] `internal/autopoiesis/profiles.go`
- [ ] `internal/autopoiesis/prompt_evolution/atom_generator.go`
- [ ] `internal/autopoiesis/prompt_evolution/atom_promoted_callback_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/classifier.go`
- [ ] `internal/autopoiesis/prompt_evolution/classifier_bench_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/evolver.go`
- [ ] `internal/autopoiesis/prompt_evolution/feedback_collector.go`
- [ ] `internal/autopoiesis/prompt_evolution/feedback_collector_migration_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/feedback_collector_stats_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/judge.go`
- [ ] `internal/autopoiesis/prompt_evolution/pinning_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/promotion_gate_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/prompt_evolution_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/strategy_atoms.go`
- [ ] `internal/autopoiesis/prompt_evolution/strategy_atoms_test.go`
- [ ] `internal/autopoiesis/prompt_evolution/strategy_store.go`
- [ ] `internal/autopoiesis/prompt_evolution/types.go`
- [ ] `internal/autopoiesis/quality.go`
- [ ] `internal/autopoiesis/quality_test.go`
- [ ] `internal/autopoiesis/runtime_output_test.go`
- [ ] `internal/autopoiesis/runtime_registry.go`
- [ ] `internal/autopoiesis/safety_adversarial_test.go`
- [ ] `internal/autopoiesis/should_generate_tool_test.go`
- [ ] `internal/autopoiesis/stability_score_test.go`
- [ ] `internal/autopoiesis/stage_budget_test.go`
- [ ] `internal/autopoiesis/templates_coverage_test.go`
- [ ] `internal/autopoiesis/thunderdome.go`
- [ ] `internal/autopoiesis/thunderdome_harness_test.go`
- [ ] `internal/autopoiesis/thunderdome_normalize_test.go`
- [ ] `internal/autopoiesis/thunderdome_result_test.go`
- [ ] `internal/autopoiesis/tool_compiler.go`
- [ ] `internal/autopoiesis/tool_compiler_gating_test.go`
- [ ] `internal/autopoiesis/tool_compiler_test.go`
- [ ] `internal/autopoiesis/tool_creation_routing_test.go`
- [ ] `internal/autopoiesis/tool_detection.go`
- [ ] `internal/autopoiesis/tool_detection_coercion_test.go`
- [ ] `internal/autopoiesis/tool_generation.go`
- [ ] `internal/autopoiesis/tool_generation_contract_test.go`
- [ ] `internal/autopoiesis/tool_templates.go`
- [ ] `internal/autopoiesis/tool_validation.go`
- [ ] `internal/autopoiesis/toolgen.go`
- [ ] `internal/autopoiesis/toolgen_gaps_test.go`
- [ ] `internal/autopoiesis/toolgen_test.go`
- [ ] `internal/autopoiesis/traces.go`
- [ ] `internal/autopoiesis/traces_bench_test.go`
- [ ] `internal/autopoiesis/traces_benchmark_test.go`
- [ ] `internal/autopoiesis/types_coverage_test.go`
- [ ] `internal/autopoiesis/utils_coverage_test.go`
- [ ] `internal/autopoiesis/yaegi_executor.go`

### internal/broker (43)
- [ ] `internal/broker/broker.go`
- [ ] `internal/broker/broker_test.go`
- [ ] `internal/broker/calibration.go`
- [ ] `internal/broker/calibration_test.go`
- [ ] `internal/broker/compression.go`
- [ ] `internal/broker/compression_test.go`
- [ ] `internal/broker/context.go`
- [ ] `internal/broker/counter.go`
- [ ] `internal/broker/counter_anthropic.go`
- [ ] `internal/broker/counter_anthropic_test.go`
- [ ] `internal/broker/decorator_audit_test.go`
- [ ] `internal/broker/default.go`
- [ ] `internal/broker/doc.go`
- [ ] `internal/broker/epoch.go`
- [ ] `internal/broker/epoch_method_test.go`
- [ ] `internal/broker/epoch_test.go`
- [ ] `internal/broker/errors.go`
- [ ] `internal/broker/extrasink_handle_test.go`
- [ ] `internal/broker/fakes_test.go`
- [ ] `internal/broker/filesink.go`
- [ ] `internal/broker/filesink_test.go`
- [ ] `internal/broker/helpers_test.go`
- [ ] `internal/broker/integrity_test.go`
- [ ] `internal/broker/ledger.go`
- [ ] `internal/broker/ledger_test.go`
- [ ] `internal/broker/measure.go`
- [ ] `internal/broker/measure_test.go`
- [ ] `internal/broker/optional.go`
- [ ] `internal/broker/passthrough.go`
- [ ] `internal/broker/passthrough_test.go`
- [ ] `internal/broker/purpose_wiring_test.go`
- [ ] `internal/broker/receipt.go`
- [ ] `internal/broker/reconcile.go`
- [ ] `internal/broker/reconcile_test.go`
- [ ] `internal/broker/shapes_test.go`
- [ ] `internal/broker/stream.go`
- [ ] `internal/broker/stream_test.go`
- [ ] `internal/broker/text.go`
- [ ] `internal/broker/text_default_test.go`
- [ ] `internal/broker/types.go`
- [ ] `internal/broker/wiring_test.go`
- [ ] `internal/broker/wrap.go`
- [ ] `internal/broker/wrap_test.go`

### internal/browser (60)
- [ ] `internal/browser/browser_integration_test.go`
- [ ] `internal/browser/contract_audit.go`
- [ ] `internal/browser/contract_audit_facts.go`
- [ ] `internal/browser/contract_audit_facts_test.go`
- [ ] `internal/browser/contract_audit_report.go`
- [ ] `internal/browser/contract_audit_report_test.go`
- [ ] `internal/browser/contract_audit_test.go`
- [ ] `internal/browser/declarative_matcher.go`
- [ ] `internal/browser/declarative_matcher_test.go`
- [ ] `internal/browser/docker_correlation.go`
- [ ] `internal/browser/docker_correlation_test.go`
- [ ] `internal/browser/docker_correlation_wiring_test.go`
- [ ] `internal/browser/docker_fetcher.go`
- [ ] `internal/browser/docker_fetcher_test.go`
- [ ] `internal/browser/dom_policy_derivation_test.go`
- [ ] `internal/browser/element_registry.go`
- [ ] `internal/browser/element_registry_test.go`
- [ ] `internal/browser/fact_epoch.go`
- [ ] `internal/browser/fact_epoch_test.go`
- [ ] `internal/browser/fact_redaction.go`
- [ ] `internal/browser/fact_redaction_test.go`
- [ ] `internal/browser/flight_recorder.go`
- [ ] `internal/browser/flight_recorder_test.go`
- [ ] `internal/browser/honeypot.go`
- [ ] `internal/browser/honeypot_coverage_test.go`
- [ ] `internal/browser/honeypot_gate.go`
- [ ] `internal/browser/honeypot_gate_live_test.go`
- [ ] `internal/browser/honeypot_policy_test.go`
- [ ] `internal/browser/honeypot_test.go`
- [ ] `internal/browser/kernel_bridge.go`
- [ ] `internal/browser/kernel_bridge_test.go`
- [ ] `internal/browser/lifecycle_coverage_test.go`
- [ ] `internal/browser/progressive_action.go`
- [ ] `internal/browser/progressive_action_test.go`
- [ ] `internal/browser/progressive_observe.go`
- [ ] `internal/browser/repo_trace.go`
- [ ] `internal/browser/repo_trace_test.go`
- [ ] `internal/browser/schema_contract_test.go`
- [ ] `internal/browser/security/path_policy.go`
- [ ] `internal/browser/security/path_policy_test.go`
- [ ] `internal/browser/security/private_permissions_unix.go`
- [ ] `internal/browser/security/private_permissions_windows.go`
- [ ] `internal/browser/security/redactor.go`
- [ ] `internal/browser/security/redactor_test.go`
- [ ] `internal/browser/session_lifecycle.go`
- [ ] `internal/browser/session_lifecycle_test.go`
- [ ] `internal/browser/session_list_test.go`
- [ ] `internal/browser/session_manager.go`
- [ ] `internal/browser/session_manager_coverage_test.go`
- [ ] `internal/browser/session_manager_dom.go`
- [ ] `internal/browser/specs/catalog.go`
- [ ] `internal/browser/specs/catalog_alias_test.go`
- [ ] `internal/browser/specs/catalog_test.go`
- [ ] `internal/browser/specs/parser.go`
- [ ] `internal/browser/specs/parser_test.go`
- [ ] `internal/browser/specs/types.go`
- [ ] `internal/browser/start_coverage_test.go`
- [ ] `internal/browser/testspec/parser.go`
- [ ] `internal/browser/testspec/parser_test.go`
- [ ] `internal/browser/testspec/types.go`

### internal/build (7)
- [ ] `internal/build/env.go`
- [ ] `internal/build/env_features_test.go`
- [ ] `internal/build/env_gaps_test.go`
- [ ] `internal/build/env_test.go`
- [ ] `internal/build/go_invocation_inventory_test.go`
- [ ] `internal/build/tags.go`
- [ ] `internal/build/tags_test.go`

### internal/campaign/assault_campaign.go (1)
- [ ] `internal/campaign/assault_campaign.go`

### internal/campaign/assault_helpers_test.go (1)
- [ ] `internal/campaign/assault_helpers_test.go`

### internal/campaign/assault_prompts.go (1)
- [ ] `internal/campaign/assault_prompts.go`

### internal/campaign/assault_report.go (1)
- [ ] `internal/campaign/assault_report.go`

### internal/campaign/assault_report_test.go (1)
- [ ] `internal/campaign/assault_report_test.go`

### internal/campaign/assault_tasks.go (1)
- [ ] `internal/campaign/assault_tasks.go`

### internal/campaign/assault_tasks_test.go (1)
- [ ] `internal/campaign/assault_tasks_test.go`

### internal/campaign/assault_types.go (1)
- [ ] `internal/campaign/assault_types.go`

### internal/campaign/campaign_fact_sync.go (1)
- [ ] `internal/campaign/campaign_fact_sync.go`

### internal/campaign/campaign_prompts.go (1)
- [ ] `internal/campaign/campaign_prompts.go`

### internal/campaign/campaign_prompts_atoms_test.go (1)
- [ ] `internal/campaign/campaign_prompts_atoms_test.go`

### internal/campaign/campaign_ref_e2e_test.go (1)
- [ ] `internal/campaign/campaign_ref_e2e_test.go`

### internal/campaign/checkpoint.go (1)
- [ ] `internal/campaign/checkpoint.go`

### internal/campaign/checkpoint_failclosed_test.go (1)
- [ ] `internal/campaign/checkpoint_failclosed_test.go`

### internal/campaign/checkpoint_gotest_json_test.go (1)
- [ ] `internal/campaign/checkpoint_gotest_json_test.go`

### internal/campaign/checkpoint_integration_test.go (1)
- [ ] `internal/campaign/checkpoint_integration_test.go`

### internal/campaign/checkpoint_manual_review_test.go (1)
- [ ] `internal/campaign/checkpoint_manual_review_test.go`

### internal/campaign/checkpoint_parsers_test.go (1)
- [ ] `internal/campaign/checkpoint_parsers_test.go`

### internal/campaign/checkpoint_phase_completion_test.go (1)
- [ ] `internal/campaign/checkpoint_phase_completion_test.go`

### internal/campaign/checkpoint_test.go (1)
- [ ] `internal/campaign/checkpoint_test.go`

### internal/campaign/checkpoint_verdict_contract_test.go (1)
- [ ] `internal/campaign/checkpoint_verdict_contract_test.go`

### internal/campaign/checkpoint_verdict_test.go (1)
- [ ] `internal/campaign/checkpoint_verdict_test.go`

### internal/campaign/config_wiring_test.go (1)
- [ ] `internal/campaign/config_wiring_test.go`

### internal/campaign/context_pager.go (1)
- [ ] `internal/campaign/context_pager.go`

### internal/campaign/context_pager_bench_test.go (1)
- [ ] `internal/campaign/context_pager_bench_test.go`

### internal/campaign/context_pager_test.go (1)
- [ ] `internal/campaign/context_pager_test.go`

### internal/campaign/decomposer.go (1)
- [ ] `internal/campaign/decomposer.go`

### internal/campaign/decomposer_documents.go (1)
- [ ] `internal/campaign/decomposer_documents.go`

### internal/campaign/decomposer_helpers_test.go (1)
- [ ] `internal/campaign/decomposer_helpers_test.go`

### internal/campaign/decomposer_planning.go (1)
- [ ] `internal/campaign/decomposer_planning.go`

### internal/campaign/decomposer_requirements.go (1)
- [ ] `internal/campaign/decomposer_requirements.go`

### internal/campaign/decomposer_requirements_test.go (1)
- [ ] `internal/campaign/decomposer_requirements_test.go`

### internal/campaign/decomposer_retype_test.go (1)
- [ ] `internal/campaign/decomposer_retype_test.go`

### internal/campaign/decomposer_test.go (1)
- [ ] `internal/campaign/decomposer_test.go`

### internal/campaign/document_ingestor.go (1)
- [ ] `internal/campaign/document_ingestor.go`

### internal/campaign/edge_case_detector.go (1)
- [ ] `internal/campaign/edge_case_detector.go`

### internal/campaign/edge_case_detector_gaps_test.go (1)
- [ ] `internal/campaign/edge_case_detector_gaps_test.go`

### internal/campaign/edge_case_detector_test.go (1)
- [ ] `internal/campaign/edge_case_detector_test.go`

### internal/campaign/edge_case_helpers_test.go (1)
- [ ] `internal/campaign/edge_case_helpers_test.go`

### internal/campaign/errors.go (1)
- [ ] `internal/campaign/errors.go`

### internal/campaign/failure_classification_test.go (1)
- [ ] `internal/campaign/failure_classification_test.go`

### internal/campaign/file_task_target_test.go (1)
- [ ] `internal/campaign/file_task_target_test.go`

### internal/campaign/holographic_gathering_test.go (1)
- [ ] `internal/campaign/holographic_gathering_test.go`

### internal/campaign/intelligence_formatting.go (1)
- [ ] `internal/campaign/intelligence_formatting.go`

### internal/campaign/intelligence_gatherer.go (1)
- [ ] `internal/campaign/intelligence_gatherer.go`

### internal/campaign/intelligence_gatherer_gaps_test.go (1)
- [ ] `internal/campaign/intelligence_gatherer_gaps_test.go`

### internal/campaign/intelligence_gatherer_test.go (1)
- [ ] `internal/campaign/intelligence_gatherer_test.go`

### internal/campaign/intelligence_gathering_methods.go (1)
- [ ] `internal/campaign/intelligence_gathering_methods.go`

### internal/campaign/journal_ops.go (1)
- [ ] `internal/campaign/journal_ops.go`

### internal/campaign/journal_ops_test.go (1)
- [ ] `internal/campaign/journal_ops_test.go`

### internal/campaign/kernel_assert.go (1)
- [ ] `internal/campaign/kernel_assert.go`

### internal/campaign/kernel_assert_test.go (1)
- [ ] `internal/campaign/kernel_assert_test.go`

### internal/campaign/main_test.go (1)
- [ ] `internal/campaign/main_test.go`

### internal/campaign/metrics.go (1)
- [ ] `internal/campaign/metrics.go`

### internal/campaign/micro_checkpoint.go (1)
- [ ] `internal/campaign/micro_checkpoint.go`

### internal/campaign/micro_checkpoint_test.go (1)
- [ ] `internal/campaign/micro_checkpoint_test.go`

### internal/campaign/mocks_test.go (1)
- [ ] `internal/campaign/mocks_test.go`

### internal/campaign/normalization.go (1)
- [ ] `internal/campaign/normalization.go`

### internal/campaign/northstar_wiring_test.go (1)
- [ ] `internal/campaign/northstar_wiring_test.go`

### internal/campaign/orchestrator.go (1)
- [ ] `internal/campaign/orchestrator.go`

### internal/campaign/orchestrator_behavior_test.go (1)
- [ ] `internal/campaign/orchestrator_behavior_test.go`

### internal/campaign/orchestrator_blockreason_cap_test.go (1)
- [ ] `internal/campaign/orchestrator_blockreason_cap_test.go`

### internal/campaign/orchestrator_callsite_test.go (1)
- [ ] `internal/campaign/orchestrator_callsite_test.go`

### internal/campaign/orchestrator_control.go (1)
- [ ] `internal/campaign/orchestrator_control.go`

### internal/campaign/orchestrator_di_test.go (1)
- [ ] `internal/campaign/orchestrator_di_test.go`

### internal/campaign/orchestrator_doc_degeneracy_test.go (1)
- [ ] `internal/campaign/orchestrator_doc_degeneracy_test.go`

### internal/campaign/orchestrator_durable_artifact_test.go (1)
- [ ] `internal/campaign/orchestrator_durable_artifact_test.go`

### internal/campaign/orchestrator_events.go (1)
- [ ] `internal/campaign/orchestrator_events.go`

### internal/campaign/orchestrator_events_test.go (1)
- [ ] `internal/campaign/orchestrator_events_test.go`

### internal/campaign/orchestrator_execution.go (1)
- [ ] `internal/campaign/orchestrator_execution.go`

### internal/campaign/orchestrator_execution_test.go (1)
- [ ] `internal/campaign/orchestrator_execution_test.go`

### internal/campaign/orchestrator_failure.go (1)
- [ ] `internal/campaign/orchestrator_failure.go`

### internal/campaign/orchestrator_failure_test.go (1)
- [ ] `internal/campaign/orchestrator_failure_test.go`

### internal/campaign/orchestrator_fallback_test.go (1)
- [ ] `internal/campaign/orchestrator_fallback_test.go`

### internal/campaign/orchestrator_init.go (1)
- [ ] `internal/campaign/orchestrator_init.go`

### internal/campaign/orchestrator_init_validation_test.go (1)
- [ ] `internal/campaign/orchestrator_init_validation_test.go`

### internal/campaign/orchestrator_journal.go (1)
- [ ] `internal/campaign/orchestrator_journal.go`

### internal/campaign/orchestrator_journal_test.go (1)
- [ ] `internal/campaign/orchestrator_journal_test.go`

### internal/campaign/orchestrator_lifecycle.go (1)
- [ ] `internal/campaign/orchestrator_lifecycle.go`

### internal/campaign/orchestrator_phases.go (1)
- [ ] `internal/campaign/orchestrator_phases.go`

### internal/campaign/orchestrator_phases_test.go (1)
- [ ] `internal/campaign/orchestrator_phases_test.go`

### internal/campaign/orchestrator_resume.go (1)
- [ ] `internal/campaign/orchestrator_resume.go`

### internal/campaign/orchestrator_resume_test.go (1)
- [ ] `internal/campaign/orchestrator_resume_test.go`

### internal/campaign/orchestrator_retry_replan_regression_test.go (1)
- [ ] `internal/campaign/orchestrator_retry_replan_regression_test.go`

### internal/campaign/orchestrator_sched_regression_test.go (1)
- [ ] `internal/campaign/orchestrator_sched_regression_test.go`

### internal/campaign/orchestrator_task_context_test.go (1)
- [ ] `internal/campaign/orchestrator_task_context_test.go`

### internal/campaign/orchestrator_task_handlers.go (1)
- [ ] `internal/campaign/orchestrator_task_handlers.go`

### internal/campaign/orchestrator_task_handlers_bench_test.go (1)
- [ ] `internal/campaign/orchestrator_task_handlers_bench_test.go`

### internal/campaign/orchestrator_task_handlers_test.go (1)
- [ ] `internal/campaign/orchestrator_task_handlers_test.go`

### internal/campaign/orchestrator_task_results.go (1)
- [ ] `internal/campaign/orchestrator_task_results.go`

### internal/campaign/orchestrator_task_transaction.go (1)
- [ ] `internal/campaign/orchestrator_task_transaction.go`

### internal/campaign/orchestrator_task_transaction_test.go (1)
- [ ] `internal/campaign/orchestrator_task_transaction_test.go`

### internal/campaign/orchestrator_taskexecutor_callsite_test.go (1)
- [ ] `internal/campaign/orchestrator_taskexecutor_callsite_test.go`

### internal/campaign/orchestrator_tasks.go (1)
- [ ] `internal/campaign/orchestrator_tasks.go`

### internal/campaign/orchestrator_types.go (1)
- [ ] `internal/campaign/orchestrator_types.go`

### internal/campaign/orchestrator_utils.go (1)
- [ ] `internal/campaign/orchestrator_utils.go`

### internal/campaign/orchestrator_write_set_gating_test.go (1)
- [ ] `internal/campaign/orchestrator_write_set_gating_test.go`

### internal/campaign/pause_durable.go (1)
- [ ] `internal/campaign/pause_durable.go`

### internal/campaign/pause_durable_test.go (1)
- [ ] `internal/campaign/pause_durable_test.go`

### internal/campaign/persist_campaign_test.go (1)
- [ ] `internal/campaign/persist_campaign_test.go`

### internal/campaign/prompts.go (1)
- [ ] `internal/campaign/prompts.go`

### internal/campaign/recurse_dag.go (1)
- [ ] `internal/campaign/recurse_dag.go`

### internal/campaign/recurse_dag_test.go (1)
- [ ] `internal/campaign/recurse_dag_test.go`

### internal/campaign/recurse_plan.go (1)
- [ ] `internal/campaign/recurse_plan.go`

### internal/campaign/recurse_plan_test.go (1)
- [ ] `internal/campaign/recurse_plan_test.go`

### internal/campaign/recurse_runner.go (1)
- [ ] `internal/campaign/recurse_runner.go`

### internal/campaign/recurse_runner_test.go (1)
- [ ] `internal/campaign/recurse_runner_test.go`

### internal/campaign/replan.go (1)
- [ ] `internal/campaign/replan.go`

### internal/campaign/replan_retype_test.go (1)
- [ ] `internal/campaign/replan_retype_test.go`

### internal/campaign/replan_test.go (1)
- [ ] `internal/campaign/replan_test.go`

### internal/campaign/replanner_fix_test.go (1)
- [ ] `internal/campaign/replanner_fix_test.go`

### internal/campaign/risk_gate_contract.go (1)
- [ ] `internal/campaign/risk_gate_contract.go`

### internal/campaign/risk_gate_contract_test.go (1)
- [ ] `internal/campaign/risk_gate_contract_test.go`

### internal/campaign/risk_scoring.go (1)
- [ ] `internal/campaign/risk_scoring.go`

### internal/campaign/risk_scoring_test.go (1)
- [ ] `internal/campaign/risk_scoring_test.go`

### internal/campaign/root_write_report_test.go (1)
- [ ] `internal/campaign/root_write_report_test.go`

### internal/campaign/shard_advisory_board.go (1)
- [ ] `internal/campaign/shard_advisory_board.go`

### internal/campaign/shard_advisory_board_test.go (1)
- [ ] `internal/campaign/shard_advisory_board_test.go`

### internal/campaign/shard_manifest_contract_test.go (1)
- [ ] `internal/campaign/shard_manifest_contract_test.go`

### internal/campaign/specialist_knowledge.go (1)
- [ ] `internal/campaign/specialist_knowledge.go`

### internal/campaign/task_dedup_test.go (1)
- [ ] `internal/campaign/task_dedup_test.go`

### internal/campaign/task_effect_contract.go (1)
- [ ] `internal/campaign/task_effect_contract.go`

### internal/campaign/task_effect_contract_test.go (1)
- [ ] `internal/campaign/task_effect_contract_test.go`

### internal/campaign/task_mutation_types.go (1)
- [ ] `internal/campaign/task_mutation_types.go`

### internal/campaign/test_failure_summary_test.go (1)
- [ ] `internal/campaign/test_failure_summary_test.go`

### internal/campaign/thunderdome_verdict_test.go (1)
- [ ] `internal/campaign/thunderdome_verdict_test.go`

### internal/campaign/tool_pregenerator.go (1)
- [ ] `internal/campaign/tool_pregenerator.go`

### internal/campaign/tool_pregenerator_test.go (1)
- [ ] `internal/campaign/tool_pregenerator_test.go`

### internal/campaign/topology_contract_test.go (1)
- [ ] `internal/campaign/topology_contract_test.go`

### internal/campaign/types.go (1)
- [ ] `internal/campaign/types.go`

### internal/campaign/types_test.go (1)
- [ ] `internal/campaign/types_test.go`

### internal/campaign/types_tofacts_golden_test.go (1)
- [ ] `internal/campaign/types_tofacts_golden_test.go`

### internal/campaign/upstream_context.go (1)
- [ ] `internal/campaign/upstream_context.go`

### internal/campaign/upstream_context_test.go (1)
- [ ] `internal/campaign/upstream_context_test.go`

### internal/campaign/utils.go (1)
- [ ] `internal/campaign/utils.go`

### internal/campaign/utils_test.go (1)
- [ ] `internal/campaign/utils_test.go`

### internal/campaign/write_set_lock_manager.go (1)
- [ ] `internal/campaign/write_set_lock_manager.go`

### internal/campaign/write_set_lock_manager_test.go (1)
- [ ] `internal/campaign/write_set_lock_manager_test.go`

### internal/config (34)
- [ ] `internal/config/browser_config_test.go`
- [ ] `internal/config/browser_correlation_test.go`
- [ ] `internal/config/build.go`
- [ ] `internal/config/config_comprehensive_test.go`
- [ ] `internal/config/config_defaults_test.go`
- [ ] `internal/config/config_security_test.go`
- [ ] `internal/config/config_test.go`
- [ ] `internal/config/core_limits_load_test.go`
- [ ] `internal/config/execution.go`
- [ ] `internal/config/integrations.go`
- [ ] `internal/config/jit.go`
- [ ] `internal/config/limits.go`
- [ ] `internal/config/limits_tool_budget_test.go`
- [ ] `internal/config/llm.go`
- [ ] `internal/config/llm_timeouts.go`
- [ ] `internal/config/llm_timeouts_config.go`
- [ ] `internal/config/llm_timeouts_config_test.go`
- [ ] `internal/config/logging.go`
- [ ] `internal/config/mangle.go`
- [ ] `internal/config/memory.go`
- [ ] `internal/config/ollama_worker_config_test.go`
- [ ] `internal/config/persistence.go`
- [ ] `internal/config/persistence_replace_other.go`
- [ ] `internal/config/persistence_replace_windows.go`
- [ ] `internal/config/reflection.go`
- [ ] `internal/config/reflection_test.go`
- [ ] `internal/config/shard.go`
- [ ] `internal/config/tool_generation.go`
- [ ] `internal/config/tool_generation_test.go`
- [ ] `internal/config/user_config.go`
- [ ] `internal/config/user_config_api_key_test.go`
- [ ] `internal/config/ux.go`
- [ ] `internal/config/world.go`
- [ ] `internal/config/yolo_test.go`

### internal/context (30)
- [ ] `internal/context/activation.go`
- [ ] `internal/context/activation_caps_test.go`
- [ ] `internal/context/activation_race_test.go`
- [ ] `internal/context/activation_scoring.go`
- [ ] `internal/context/activation_setters_test.go`
- [ ] `internal/context/activation_test.go`
- [ ] `internal/context/budget_helpers_test.go`
- [ ] `internal/context/chat_history_parity_test.go`
- [ ] `internal/context/compressor.go`
- [ ] `internal/context/compressor_accessors_test.go`
- [ ] `internal/context/compressor_metrics.go`
- [ ] `internal/context/compressor_race_test.go`
- [ ] `internal/context/compressor_test.go`
- [ ] `internal/context/compressor_turns.go`
- [ ] `internal/context/feedback_store.go`
- [ ] `internal/context/feedback_store_scoring_test.go`
- [ ] `internal/context/feedback_store_test.go`
- [ ] `internal/context/kernel_context_test.go`
- [ ] `internal/context/long_session_test.go`
- [ ] `internal/context/memory_op_coverage_test.go`
- [ ] `internal/context/mocks_test.go`
- [ ] `internal/context/serializer.go`
- [ ] `internal/context/serializer_bounds_test.go`
- [ ] `internal/context/serializer_test.go`
- [ ] `internal/context/token_counter_extra_test.go`
- [ ] `internal/context/tokens.go`
- [ ] `internal/context/types.go`
- [ ] `internal/context/working_set.go`
- [ ] `internal/context/working_set_test.go`
- [ ] `internal/context/working_store.go`

### internal/core/action_validator.go (1)
- [x] `internal/core/action_validator.go`

### internal/core/action_validator_gaps_test.go (1)
- [ ] `internal/core/action_validator_gaps_test.go`

### internal/core/action_validator_test.go (1)
- [ ] `internal/core/action_validator_test.go`

### internal/core/api_scheduler.go (1)
- [x] `internal/core/api_scheduler.go`

### internal/core/api_scheduler_gaps_test.go (1)
- [ ] `internal/core/api_scheduler_gaps_test.go`

### internal/core/api_scheduler_more_coverage_test.go (1)
- [ ] `internal/core/api_scheduler_more_coverage_test.go`

### internal/core/api_scheduler_slot_leak_test.go (1)
- [ ] `internal/core/api_scheduler_slot_leak_test.go`

### internal/core/api_scheduler_test.go (1)
- [ ] `internal/core/api_scheduler_test.go`

### internal/core/bound_negation_test.go (1)
- [ ] `internal/core/bound_negation_test.go`

### internal/core/build_tool_env_test.go (1)
- [ ] `internal/core/build_tool_env_test.go`

### internal/core/campaign_phase_category_test.go (1)
- [ ] `internal/core/campaign_phase_category_test.go`

### internal/core/canary_helpers_test.go (1)
- [ ] `internal/core/canary_helpers_test.go`

### internal/core/codedom_modified_symbols.go (1)
- [x] `internal/core/codedom_modified_symbols.go`

### internal/core/codedom_modified_symbols_test.go (1)
- [ ] `internal/core/codedom_modified_symbols_test.go`

### internal/core/codedom_reason_atom_unification_test.go (1)
- [ ] `internal/core/codedom_reason_atom_unification_test.go`

### internal/core/consistency_test.go (1)
- [ ] `internal/core/consistency_test.go`

### internal/core/cortex_derivation_wiring_test.go (1)
- [ ] `internal/core/cortex_derivation_wiring_test.go`

### internal/core/cortex_kernel.go (1)
- [x] `internal/core/cortex_kernel.go`

### internal/core/cortex_kernel_extra_test.go (1)
- [ ] `internal/core/cortex_kernel_extra_test.go`

### internal/core/cortex_kernel_query_fanout_test.go (1)
- [ ] `internal/core/cortex_kernel_query_fanout_test.go`

### internal/core/cortex_kernel_test.go (1)
- [ ] `internal/core/cortex_kernel_test.go`

### internal/core/cortex_uplift_test.go (1)
- [x] `internal/core/cortex_uplift_test.go`

### internal/core/cortex_kernel_transaction_test.go (1)
- [ ] `internal/core/cortex_kernel_transaction_test.go`

### internal/core/cortex_mutation_failure_test.go (1)
- [ ] `internal/core/cortex_mutation_failure_test.go`

### internal/core/cortex_split_join_test.go (1)
- [ ] `internal/core/cortex_split_join_test.go`

### internal/core/coverage_boost_test.go (1)
- [ ] `internal/core/coverage_boost_test.go`

### internal/core/defaults (18)
- [ ] `internal/core/defaults/build_topology_test.go`
- [ ] `internal/core/defaults/corpus_float_comparison_test.go`
- [ ] `internal/core/defaults/corpus_test.go`
- [ ] `internal/core/defaults/intent_corpus.go`
- [ ] `internal/core/defaults/policy/browser_honeypot_test.go`
- [ ] `internal/core/defaults/policy/browser_reasoning_test.go`
- [ ] `internal/core/defaults/policy/logic_test.go`
- [ ] `internal/core/defaults/policy/main_test.go`
- [ ] `internal/core/defaults/policy/safety_exec_test.go`
- [ ] `internal/core/defaults/policy/safety_test.go`
- [ ] `internal/core/defaults/policy/tdd_logic_test.go`
- [ ] `internal/core/defaults/predicate_corpus.go`
- [ ] `internal/core/defaults/prompt_corpus.go`
- [ ] `internal/core/defaults/query_only_predicate_test.go`
- [ ] `internal/core/defaults/schema_duplicate_decl_test.go`
- [ ] `internal/core/defaults/starved_atom_value_test.go`
- [ ] `internal/core/defaults/starved_predicate_test.go`
- [ ] `internal/core/defaults/undeclared_assert_test.go`

### internal/core/dependency_reachability_test.go (1)
- [ ] `internal/core/dependency_reachability_test.go`

### internal/core/derivation_map.go (1)
- [x] `internal/core/derivation_map.go`

### internal/core/derivation_map_test.go (1)
- [ ] `internal/core/derivation_map_test.go`

### internal/core/dream_learning.go (1)
- [x] `internal/core/dream_learning.go`

### internal/core/dream_learning_test.go (1)
- [ ] `internal/core/dream_learning_test.go`

### internal/core/dream_plan.go (1)
- [x] `internal/core/dream_plan.go`

### internal/core/dream_plan_extractor.go (1)
- [x] `internal/core/dream_plan_extractor.go`

### internal/core/dream_plan_extractor_test.go (1)
- [ ] `internal/core/dream_plan_extractor_test.go`

### internal/core/dream_plan_gaps_test.go (1)
- [ ] `internal/core/dream_plan_gaps_test.go`

### internal/core/dream_plan_manager.go (1)
- [x] `internal/core/dream_plan_manager.go`

### internal/core/dream_plan_manager_test.go (1)
- [ ] `internal/core/dream_plan_manager_test.go`

### internal/core/dream_plan_test.go (1)
- [ ] `internal/core/dream_plan_test.go`

### internal/core/dream_router.go (1)
- [x] `internal/core/dream_router.go`

### internal/core/dream_router_persist_test.go (1)
- [ ] `internal/core/dream_router_persist_test.go`

### internal/core/dream_router_test.go (1)
- [ ] `internal/core/dream_router_test.go`

### internal/core/dream_singleton_test.go (1)
- [ ] `internal/core/dream_singleton_test.go`

### internal/core/dreamer.go (1)
- [x] `internal/core/dreamer.go`

### internal/core/dreamer_authorization_test.go (1)
- [ ] `internal/core/dreamer_authorization_test.go`

### internal/core/dreamer_benchmark_test.go (1)
- [ ] `internal/core/dreamer_benchmark_test.go`

### internal/core/dreamer_gaps_test.go (1)
- [ ] `internal/core/dreamer_gaps_test.go`

### internal/core/dreamer_perf_test.go (1)
- [ ] `internal/core/dreamer_perf_test.go`

### internal/core/dreamer_test.go (1)
- [ ] `internal/core/dreamer_test.go`

### internal/core/external_predicates.go (1)
- [x] `internal/core/external_predicates.go`

### internal/core/external_predicates_coverage_test.go (1)
- [ ] `internal/core/external_predicates_coverage_test.go`

### internal/core/fact_categories.go (1)
- [x] `internal/core/fact_categories.go`

### internal/core/fact_categories_coverage_test.go (1)
- [ ] `internal/core/fact_categories_coverage_test.go`

### internal/core/fact_event_bus.go (1)
- [x] `internal/core/fact_event_bus.go`

### internal/core/fact_event_bus_test.go (1)
- [ ] `internal/core/fact_event_bus_test.go`

### internal/core/hybrid_loader.go (1)
- [x] `internal/core/hybrid_loader.go`

### internal/core/hybrid_loader_test.go (1)
- [ ] `internal/core/hybrid_loader_test.go`

### internal/core/integration_path.go (1)
- [x] `internal/core/integration_path.go`

### internal/core/intent_coverage_test.go (1)
- [ ] `internal/core/intent_coverage_test.go`

### internal/core/intent_decl_interning_test.go (1)
- [ ] `internal/core/intent_decl_interning_test.go`

### internal/core/intent_defaults.go (1)
- [x] `internal/core/intent_defaults.go`

### internal/core/intent_identity_atom_unification_test.go (1)
- [ ] `internal/core/intent_identity_atom_unification_test.go`

### internal/core/intent_inference.go (1)
- [x] `internal/core/intent_inference.go`

### internal/core/intent_inference_test.go (1)
- [ ] `internal/core/intent_inference_test.go`

### internal/core/intent_loader.go (1)
- [x] `internal/core/intent_loader.go`

### internal/core/intent_uplift_test.go (1)
- [x] `internal/core/intent_uplift_test.go`

### internal/core/intent_schema_files_test.go (1)
- [ ] `internal/core/intent_schema_files_test.go`

### internal/core/jit_pinning_test.go (1)
- [ ] `internal/core/jit_pinning_test.go`

### internal/core/jit_retrieved_context_test.go (1)
- [ ] `internal/core/jit_retrieved_context_test.go`

### internal/core/jit_supersession_test.go (1)
- [ ] `internal/core/jit_supersession_test.go`

### internal/core/kernel.go (1)
- [x] `internal/core/kernel.go`

### internal/core/kernel_accessors.go (1)
- [x] `internal/core/kernel_accessors.go`

### internal/core/kernel_assert_rejection_test.go (1)
- [ ] `internal/core/kernel_assert_rejection_test.go`

### internal/core/kernel_cache_sync_test.go (1)
- [ ] `internal/core/kernel_cache_sync_test.go`

### internal/core/kernel_eval.go (1)
- [x] `internal/core/kernel_eval.go`

### internal/core/kernel_eval_demote_test.go (1)
- [ ] `internal/core/kernel_eval_demote_test.go`

### internal/core/kernel_eval_large_test.go (1)
- [ ] `internal/core/kernel_eval_large_test.go`

### internal/core/kernel_eval_test.go (1)
- [ ] `internal/core/kernel_eval_test.go`

### internal/core/kernel_fact_decl.go (1)
- [x] `internal/core/kernel_fact_decl.go`

### internal/core/kernel_fact_decl_test.go (1)
- [ ] `internal/core/kernel_fact_decl_test.go`

### internal/core/kernel_facts.go (1)
- [x] `internal/core/kernel_facts.go`

### internal/core/kernel_facts_gaps_test.go (1)
- [ ] `internal/core/kernel_facts_gaps_test.go`

### internal/core/kernel_facts_intern.go (1)
- [x] `internal/core/kernel_facts_intern.go`

### internal/core/kernel_facts_intern_test.go (1)
- [ ] `internal/core/kernel_facts_intern_test.go`

### internal/core/kernel_facts_scrub_test.go (1)
- [ ] `internal/core/kernel_facts_scrub_test.go`

### internal/core/kernel_facts_test.go (1)
- [ ] `internal/core/kernel_facts_test.go`

### internal/core/kernel_features_test.go (1)
- [ ] `internal/core/kernel_features_test.go`

### internal/core/kernel_heartbeat_test.go (1)
- [ ] `internal/core/kernel_heartbeat_test.go`

### internal/core/kernel_indexed_store_test.go (1)
- [ ] `internal/core/kernel_indexed_store_test.go`

### internal/core/kernel_init.go (1)
- [x] `internal/core/kernel_init.go`

### internal/core/kernel_init_bench_test.go (1)
- [ ] `internal/core/kernel_init_bench_test.go`

### internal/core/kernel_intelligence_test.go (1)
- [ ] `internal/core/kernel_intelligence_test.go`

### internal/core/kernel_loadfacts_lazy_test.go (1)
- [ ] `internal/core/kernel_loadfacts_lazy_test.go`

### internal/core/kernel_policy.go (1)
- [x] `internal/core/kernel_policy.go`

### internal/core/kernel_policy_test.go (1)
- [ ] `internal/core/kernel_policy_test.go`

### internal/core/kernel_policy_uplift_test.go (1)
- [x] `internal/core/kernel_policy_uplift_test.go`

### internal/core/kernel_provenance.go (1)
- [x] `internal/core/kernel_provenance.go`

### internal/core/kernel_provenance_test.go (1)
- [ ] `internal/core/kernel_provenance_test.go`

### internal/core/kernel_query.go (1)
- [x] `internal/core/kernel_query.go`

### internal/core/kernel_query_gaps_test.go (1)
- [ ] `internal/core/kernel_query_gaps_test.go`

### internal/core/kernel_query_multi_arity_test.go (1)
- [ ] `internal/core/kernel_query_multi_arity_test.go`

### internal/core/kernel_query_security_test.go (1)
- [ ] `internal/core/kernel_query_security_test.go`

### internal/core/kernel_query_test.go (1)
- [ ] `internal/core/kernel_query_test.go`

### internal/core/kernel_safe_action.go (1)
- [x] `internal/core/kernel_safe_action.go`

### internal/core/kernel_safe_action_test.go (1)
- [ ] `internal/core/kernel_safe_action_test.go`

### internal/core/kernel_sandbox_logging_test.go (1)
- [ ] `internal/core/kernel_sandbox_logging_test.go`

### internal/core/kernel_shard.go (1)
- [x] `internal/core/kernel_shard.go`

### internal/core/kernel_step_predicates_test.go (1)
- [ ] `internal/core/kernel_step_predicates_test.go`

### internal/core/kernel_test.go (1)
- [ ] `internal/core/kernel_test.go`

### internal/core/kernel_transactions.go (1)
- [x] `internal/core/kernel_transactions.go`

### internal/core/kernel_transactions_uplift_test.go (1)
- [x] `internal/core/kernel_transactions_uplift_test.go`

### internal/core/kernel_types.go (1)
- [x] `internal/core/kernel_types.go`

### internal/core/kernel_undeclared.go (1)
- [x] `internal/core/kernel_undeclared.go`

### internal/core/kernel_undeclared_test.go (1)
- [ ] `internal/core/kernel_undeclared_test.go`

### internal/core/kernel_utils.go (1)
- [x] `internal/core/kernel_utils.go`

### internal/core/kernel_validation.go (1)
- [x] `internal/core/kernel_validation.go`

### internal/core/kernel_validation_absurd_test.go (1)
- [ ] `internal/core/kernel_validation_absurd_test.go`

### internal/core/kernel_validation_gaps_test.go (1)
- [ ] `internal/core/kernel_validation_gaps_test.go`

### internal/core/kernel_validation_test.go (1)
- [ ] `internal/core/kernel_validation_test.go`

### internal/core/kernel_virtual.go (1)
- [x] `internal/core/kernel_virtual.go`

### internal/core/kernel_workspace_constructor_test.go (1)
- [ ] `internal/core/kernel_workspace_constructor_test.go`

### internal/core/knowledge_graph_test.go (1)
- [ ] `internal/core/knowledge_graph_test.go`

### internal/core/learning.go (1)
- [x] `internal/core/learning.go`

### internal/core/limits.go (1)
- [x] `internal/core/limits.go`

### internal/core/limits_coverage_test.go (1)
- [ ] `internal/core/limits_coverage_test.go`

### internal/core/limits_test.go (1)
- [ ] `internal/core/limits_test.go`

### internal/core/line_ending.go (1)
- [x] `internal/core/line_ending.go`

### internal/core/line_ending_test.go (1)
- [ ] `internal/core/line_ending_test.go`

### internal/core/llm_client.go (1)
- [x] `internal/core/llm_client.go`

### internal/core/llm_client_test.go (1)
- [ ] `internal/core/llm_client_test.go`

### internal/core/manager_routing_test.go (1)
- [ ] `internal/core/manager_routing_test.go`

### internal/core/mangle_evidence_authority_test.go (1)
- [ ] `internal/core/mangle_evidence_authority_test.go`

### internal/core/mangle_updates.go (1)
- [x] `internal/core/mangle_updates.go`

### internal/core/mangle_updates_test.go (1)
- [ ] `internal/core/mangle_updates_test.go`

### internal/core/mangle_watcher.go (1)
- [x] `internal/core/mangle_watcher.go`

### internal/core/mangle_watcher_test.go (1)
- [ ] `internal/core/mangle_watcher_test.go`

### internal/core/mg_decl_body_literal_wiring_test.go (1)
- [ ] `internal/core/mg_decl_body_literal_wiring_test.go`

### internal/core/mg_decl_literal_conformance_test.go (1)
- [ ] `internal/core/mg_decl_literal_conformance_test.go`

### internal/core/ondemand_watcher.go (1)
- [x] `internal/core/ondemand_watcher.go`

### internal/core/ondemand_watcher_test.go (1)
- [ ] `internal/core/ondemand_watcher_test.go`

### internal/core/parse_serial.go (1)
- [x] `internal/core/parse_serial.go`

### internal/core/pending_edit.go (1)
- [x] `internal/core/pending_edit.go`

### internal/core/performance_bench_test.go (1)
- [ ] `internal/core/performance_bench_test.go`

### internal/core/policy_inventory.go (1)
- [x] `internal/core/policy_inventory.go`

### internal/core/policy_inventory_test.go (1)
- [ ] `internal/core/policy_inventory_test.go`

### internal/core/predicate_corpus.go (1)
- [x] `internal/core/predicate_corpus.go`

### internal/core/predicate_corpus_extra_test.go (1)
- [ ] `internal/core/predicate_corpus_extra_test.go`

### internal/core/predicate_corpus_test.go (1)
- [ ] `internal/core/predicate_corpus_test.go`

### internal/core/query_pattern_test.go (1)
- [ ] `internal/core/query_pattern_test.go`

### internal/core/routing_performance_test.go (1)
- [ ] `internal/core/routing_performance_test.go`

### internal/core/rule_court.go (1)
- [x] `internal/core/rule_court.go`

### internal/core/rule_court_coverage_test.go (1)
- [ ] `internal/core/rule_court_coverage_test.go`

### internal/core/rule_court_gaps_test.go (1)
- [ ] `internal/core/rule_court_gaps_test.go`

### internal/core/rule_court_test.go (1)
- [ ] `internal/core/rule_court_test.go`

### internal/core/scheduled_grounded_test.go (1)
- [ ] `internal/core/scheduled_grounded_test.go`

### internal/core/scheduled_llm_client.go (1)
- [x] `internal/core/scheduled_llm_client.go`

### internal/core/scheduled_llm_client_coverage_test.go (1)
- [ ] `internal/core/scheduled_llm_client_coverage_test.go`

### internal/core/scheduled_llm_client_trace_test.go (1)
- [ ] `internal/core/scheduled_llm_client_trace_test.go`

### internal/core/scheduled_llm_tool_results_test.go (1)
- [ ] `internal/core/scheduled_llm_tool_results_test.go`

### internal/core/self_healing.go (1)
- [x] `internal/core/self_healing.go`

### internal/core/self_healing_test.go (1)
- [ ] `internal/core/self_healing_test.go`

### internal/core/shadow_failclosed_test.go (1)
- [ ] `internal/core/shadow_failclosed_test.go`

### internal/core/shadow_mode.go (1)
- [x] `internal/core/shadow_mode.go`

### internal/core/shadow_mode_assert_test.go (1)
- [ ] `internal/core/shadow_mode_assert_test.go`

### internal/core/shadow_mode_gaps_test.go (1)
- [ ] `internal/core/shadow_mode_gaps_test.go`

### internal/core/shadow_mode_test.go (1)
- [ ] `internal/core/shadow_mode_test.go`

### internal/core/shard_fact_router.go (1)
- [x] `internal/core/shard_fact_router.go`

### internal/core/shard_fact_router_test.go (1)
- [ ] `internal/core/shard_fact_router_test.go`

### internal/core/shards (23)
- [ ] `internal/core/shards/agents.go`
- [ ] `internal/core/shards/agents_test.go`
- [ ] `internal/core/shards/base_agent_test.go`
- [ ] `internal/core/shards/config.go`
- [ ] `internal/core/shards/config_test.go`
- [ ] `internal/core/shards/hollow_spawn_test.go`
- [ ] `internal/core/shards/image_generator.go`
- [ ] `internal/core/shards/image_generator_test.go`
- [ ] `internal/core/shards/manager.go`
- [ ] `internal/core/shards/manager_accessors_test.go`
- [ ] `internal/core/shards/manager_facts_test.go`
- [ ] `internal/core/shards/manager_ondemand.go`
- [ ] `internal/core/shards/manager_ondemand_test.go`
- [ ] `internal/core/shards/manager_spawn.go`
- [ ] `internal/core/shards/manager_spawn_context_test.go`
- [ ] `internal/core/shards/manager_test.go`
- [ ] `internal/core/shards/manager_tools.go`
- [ ] `internal/core/shards/queue_stress_test.go`
- [ ] `internal/core/shards/review_feedback_test.go`
- [ ] `internal/core/shards/shards_coverage_test.go`
- [ ] `internal/core/shards/spawn_queue.go`
- [ ] `internal/core/shards/spawn_queue_test.go`
- [ ] `internal/core/shards/transparency_feed_test.go`

### internal/core/shards_decl_atom_unification_test.go (1)
- [ ] `internal/core/shards_decl_atom_unification_test.go`

### internal/core/stage_context_test.go (1)
- [ ] `internal/core/stage_context_test.go`

### internal/core/task_stage_test.go (1)
- [ ] `internal/core/task_stage_test.go`

### internal/core/tdd_loop.go (1)
- [x] `internal/core/tdd_loop.go`

### internal/core/tdd_loop_test.go (1)
- [ ] `internal/core/tdd_loop_test.go`

### internal/core/test_helpers_test.go (1)
- [ ] `internal/core/test_helpers_test.go`

### internal/core/tool_registry.go (1)
- [x] `internal/core/tool_registry.go`

### internal/core/tool_registry_test.go (1)
- [ ] `internal/core/tool_registry_test.go`

### internal/core/trace.go (1)
- [x] `internal/core/trace.go`

### internal/core/trace_classification_test.go (1)
- [ ] `internal/core/trace_classification_test.go`

### internal/core/trace_coverage_test.go (1)
- [ ] `internal/core/trace_coverage_test.go`

### internal/core/trace_test.go (1)
- [ ] `internal/core/trace_test.go`

### internal/core/transaction_atomic_test.go (1)
- [ ] `internal/core/transaction_atomic_test.go`

### internal/core/transaction_manager.go (1)
- [x] `internal/core/transaction_manager.go`

### internal/core/transaction_manager_gaps_test.go (1)
- [ ] `internal/core/transaction_manager_gaps_test.go`

### internal/core/transaction_manager_test.go (1)
- [ ] `internal/core/transaction_manager_test.go`

### internal/core/transparency_deny_test.go (1)
- [ ] `internal/core/transparency_deny_test.go`

### internal/core/validation_reason_vocabulary_test.go (1)
- [ ] `internal/core/validation_reason_vocabulary_test.go`

### internal/core/validator_codedom.go (1)
- [x] `internal/core/validator_codedom.go`

### internal/core/validator_codedom_crlf_test.go (1)
- [ ] `internal/core/validator_codedom_crlf_test.go`

### internal/core/validator_codedom_delete_lines_test.go (1)
- [ ] `internal/core/validator_codedom_delete_lines_test.go`

### internal/core/validator_dir.go (1)
- [x] `internal/core/validator_dir.go`

### internal/core/validator_edit_enhanced.go (1)
- [x] `internal/core/validator_edit_enhanced.go`

### internal/core/validator_exec.go (1)
- [x] `internal/core/validator_exec.go`

### internal/core/validator_exec_gaps_test.go (1)
- [ ] `internal/core/validator_exec_gaps_test.go`

### internal/core/validator_exec_test.go (1)
- [ ] `internal/core/validator_exec_test.go`

### internal/core/validator_file.go (1)
- [x] `internal/core/validator_file.go`

### internal/core/validator_file_integration_test.go (1)
- [ ] `internal/core/validator_file_integration_test.go`

### internal/core/validator_paranoid.go (1)
- [x] `internal/core/validator_paranoid.go`

### internal/core/validator_paranoid_test.go (1)
- [ ] `internal/core/validator_paranoid_test.go`

### internal/core/validator_registry.go (1)
- [x] `internal/core/validator_registry.go`

### internal/core/validator_registry_test.go (1)
- [ ] `internal/core/validator_registry_test.go`

### internal/core/validator_syntax.go (1)
- [x] `internal/core/validator_syntax.go`

### internal/core/validator_syntax_gaps_test.go (1)
- [ ] `internal/core/validator_syntax_gaps_test.go`

### internal/core/validator_syntax_test.go (1)
- [ ] `internal/core/validator_syntax_test.go`

### internal/core/validator_tool_contract_test.go (1)
- [ ] `internal/core/validator_tool_contract_test.go`

### internal/core/verification_host_test.go (1)
- [ ] `internal/core/verification_host_test.go`

### internal/core/virtual_store.go (1)
- [x] `internal/core/virtual_store.go`

### internal/core/virtual_store_actions.go (1)
- [x] `internal/core/virtual_store_actions.go`

### internal/core/virtual_store_actions_coverage_test.go (1)
- [ ] `internal/core/virtual_store_actions_coverage_test.go`

### internal/core/virtual_store_cli_fact_test.go (1)
- [ ] `internal/core/virtual_store_cli_fact_test.go`

### internal/core/virtual_store_codedom.go (1)
- [x] `internal/core/virtual_store_codedom.go`

### internal/core/virtual_store_codedom_coverage_test.go (1)
- [ ] `internal/core/virtual_store_codedom_coverage_test.go`

### internal/core/virtual_store_codedom_test.go (1)
- [ ] `internal/core/virtual_store_codedom_test.go`

### internal/core/virtual_store_command_guard_test.go (1)
- [ ] `internal/core/virtual_store_command_guard_test.go`

### internal/core/virtual_store_constitution.go (1)
- [x] `internal/core/virtual_store_constitution.go`

### internal/core/virtual_store_delegate_codec_test.go (1)
- [ ] `internal/core/virtual_store_delegate_codec_test.go`

### internal/core/virtual_store_denial_feedback_test.go (1)
- [ ] `internal/core/virtual_store_denial_feedback_test.go`

### internal/core/virtual_store_file_actions.go (1)
- [x] `internal/core/virtual_store_file_actions.go`

### internal/core/virtual_store_gaps_test.go (1)
- [ ] `internal/core/virtual_store_gaps_test.go`

### internal/core/virtual_store_graph.go (1)
- [x] `internal/core/virtual_store_graph.go`

### internal/core/virtual_store_grounded_test.go (1)
- [ ] `internal/core/virtual_store_grounded_test.go`

### internal/core/virtual_store_integration_test.go (1)
- [ ] `internal/core/virtual_store_integration_test.go`

### internal/core/virtual_store_interactive_gate.go (1)
- [x] `internal/core/virtual_store_interactive_gate.go`

### internal/core/virtual_store_interactive_gate_test.go (1)
- [ ] `internal/core/virtual_store_interactive_gate_test.go`

### internal/core/virtual_store_interface_test.go (1)
- [ ] `internal/core/virtual_store_interface_test.go`

### internal/core/virtual_store_link_facts_test.go (1)
- [ ] `internal/core/virtual_store_link_facts_test.go`

### internal/core/virtual_store_mcp_proxy.go (1)
- [x] `internal/core/virtual_store_mcp_proxy.go`

### internal/core/virtual_store_mcp_wiring_test.go (1)
- [ ] `internal/core/virtual_store_mcp_wiring_test.go`

### internal/core/virtual_store_predicates.go (1)
- [x] `internal/core/virtual_store_predicates.go`

### internal/core/virtual_store_predicates_coverage_test.go (1)
- [ ] `internal/core/virtual_store_predicates_coverage_test.go`

### internal/core/virtual_store_projectdoc.go (1)
- [x] `internal/core/virtual_store_projectdoc.go`

### internal/core/virtual_store_projectdoc_test.go (1)
- [ ] `internal/core/virtual_store_projectdoc_test.go`

### internal/core/virtual_store_python.go (1)
- [x] `internal/core/virtual_store_python.go`

### internal/core/virtual_store_python_test.go (1)
- [ ] `internal/core/virtual_store_python_test.go`

### internal/core/virtual_store_routing.go (1)
- [x] `internal/core/virtual_store_routing.go`

### internal/core/virtual_store_safety_test.go (1)
- [ ] `internal/core/virtual_store_safety_test.go`

### internal/core/virtual_store_tactile_audit_test.go (1)
- [ ] `internal/core/virtual_store_tactile_audit_test.go`

### internal/core/virtual_store_test.go (1)
- [ ] `internal/core/virtual_store_test.go`

### internal/core/virtual_store_tool_facts.go (1)
- [x] `internal/core/virtual_store_tool_facts.go`

### internal/core/virtual_store_tools.go (1)
- [x] `internal/core/virtual_store_tools.go`

### internal/core/virtual_store_types.go (1)
- [x] `internal/core/virtual_store_types.go`

### internal/core/virtual_store_uplift_test.go (1)
- [x] `internal/core/virtual_store_uplift_test.go`

### internal/core/virtual_store_workflows.go (1)
- [x] `internal/core/virtual_store_workflows.go`

### internal/core/virtual_store_workflows_coverage_test.go (1)
- [ ] `internal/core/virtual_store_workflows_coverage_test.go`

### internal/core/virtual_store_workflows_test.go (1)
- [ ] `internal/core/virtual_store_workflows_test.go`

### internal/core/virtual_store_write_guard.go (1)
- [x] `internal/core/virtual_store_write_guard.go`

### internal/core/yolo_policy_test.go (1)
- [ ] `internal/core/yolo_policy_test.go`

### internal/diff (7)
- [ ] `internal/diff/benchmark_test.go`
- [ ] `internal/diff/cache.go`
- [ ] `internal/diff/cache_test.go`
- [ ] `internal/diff/diff.go`
- [ ] `internal/diff/diff_comprehensive_test.go`
- [ ] `internal/diff/diff_test.go`
- [ ] `internal/diff/word_span_test.go`

### internal/embedding (15)
- [ ] `internal/embedding/engine.go`
- [ ] `internal/embedding/engine_coverage_test.go`
- [ ] `internal/embedding/genai.go`
- [ ] `internal/embedding/genai_bench_test.go`
- [ ] `internal/embedding/genai_coverage_test.go`
- [ ] `internal/embedding/genai_test.go`
- [ ] `internal/embedding/math_amd64.go`
- [ ] `internal/embedding/math_generic.go`
- [ ] `internal/embedding/ollama.go`
- [ ] `internal/embedding/ollama_coverage_test.go`
- [ ] `internal/embedding/ollama_ensure_test.go`
- [ ] `internal/embedding/ollama_test.go`
- [ ] `internal/embedding/task_selector.go`
- [ ] `internal/embedding/task_selector_coverage_test.go`
- [ ] `internal/embedding/task_selector_test.go`

### internal/evidence (3)
- [ ] `internal/evidence/change.go`
- [ ] `internal/evidence/change_test.go`
- [ ] `internal/evidence/context.go`

### internal/features (8)
- [ ] `internal/features/config_roundtrip_test.go`
- [ ] `internal/features/features.go`
- [ ] `internal/features/features_defaults_test.go`
- [ ] `internal/features/features_test.go`
- [ ] `internal/features/migration_test.go`
- [ ] `internal/features/resolved_test.go`
- [ ] `internal/features/schema.go`
- [ ] `internal/features/schema_test.go`

### internal/init (37)
- [ ] `internal/init/agent_generation_contract_test.go`
- [ ] `internal/init/agents.go`
- [ ] `internal/init/agents_curation.go`
- [ ] `internal/init/agents_curation_test.go`
- [ ] `internal/init/agents_knowledge.go`
- [ ] `internal/init/agents_knowledge_helpers_test.go`
- [ ] `internal/init/agents_prompts_test.go`
- [ ] `internal/init/agents_registration.go`
- [ ] `internal/init/discovered_agent_store_test.go`
- [ ] `internal/init/eta_tracker.go`
- [ ] `internal/init/init_coverage_test.go`
- [ ] `internal/init/init_test.go`
- [ ] `internal/init/initializer.go`
- [ ] `internal/init/initializer_truth_test.go`
- [ ] `internal/init/interactive.go`
- [ ] `internal/init/interactive_display_test.go`
- [ ] `internal/init/jit_integration.go`
- [ ] `internal/init/jit_kernel_verify_test.go`
- [ ] `internal/init/modules_test.go`
- [ ] `internal/init/preferences_preservation_test.go`
- [ ] `internal/init/profile.go`
- [ ] `internal/init/profile_detection_test.go`
- [ ] `internal/init/prompt_deadline_test.go`
- [ ] `internal/init/scanner.go`
- [ ] `internal/init/scanner_dependencies.go`
- [ ] `internal/init/scanner_dependencies_test.go`
- [ ] `internal/init/scanner_determinism_test.go`
- [ ] `internal/init/scanner_test.go`
- [ ] `internal/init/shared_kb.go`
- [ ] `internal/init/strategic_documents.go`
- [ ] `internal/init/strategic_knowledge.go`
- [ ] `internal/init/strategic_knowledge_parsing_test.go`
- [ ] `internal/init/tool_target_test.go`
- [ ] `internal/init/tools.go`
- [ ] `internal/init/typeu_agents.go`
- [ ] `internal/init/typeu_coverage_test.go`
- [ ] `internal/init/validation.go`

### internal/jit (2)
- [ ] `internal/jit/config/types.go`
- [ ] `internal/jit/config/types_test.go`

### internal/jsonl (2)
- [ ] `internal/jsonl/jsonl.go`
- [ ] `internal/jsonl/jsonl_test.go`

### internal/logging (32)
- [ ] `internal/logging/audit.go`
- [ ] `internal/logging/audit_benchmark_test.go`
- [ ] `internal/logging/audit_concurrency_test.go`
- [ ] `internal/logging/audit_coverage_test.go`
- [ ] `internal/logging/audit_facts.go`
- [ ] `internal/logging/audit_facts_test.go`
- [ ] `internal/logging/audit_reader.go`
- [ ] `internal/logging/audit_reader_test.go`
- [ ] `internal/logging/category_inventory_test.go`
- [ ] `internal/logging/config_schema_test.go`
- [ ] `internal/logging/coverage_boost_test.go`
- [ ] `internal/logging/fresh_run.go`
- [ ] `internal/logging/fresh_run_substantive_test.go`
- [ ] `internal/logging/fresh_run_test.go`
- [ ] `internal/logging/init_rebind_test.go`
- [ ] `internal/logging/late_config_test.go`
- [ ] `internal/logging/llm_io_logger.go`
- [ ] `internal/logging/llm_io_trace_test.go`
- [ ] `internal/logging/logger.go`
- [ ] `internal/logging/logger_convenience.go`
- [ ] `internal/logging/logger_test.go`
- [ ] `internal/logging/logging_comprehensive_test.go`
- [ ] `internal/logging/problems_log_test.go`
- [ ] `internal/logging/redact.go`
- [ ] `internal/logging/redact_test.go`
- [ ] `internal/logging/request_logger_fields_test.go`
- [ ] `internal/logging/rotate.go`
- [ ] `internal/logging/rotate_test.go`
- [ ] `internal/logging/safety_callsite_audit_test.go`
- [ ] `internal/logging/sink_lifecycle_test.go`
- [ ] `internal/logging/structured_decorators_test.go`
- [ ] `internal/logging/testsupport_test.go`

### internal/mangle/differential.go (1)
- [x] `internal/mangle/differential.go`

### internal/mangle/differential_test.go (1)
- [ ] `internal/mangle/differential_test.go`

### internal/mangle/engine.go (1)
- [x] `internal/mangle/engine.go`

### internal/mangle/engine_behavior_regression_test.go (1)
- [ ] `internal/mangle/engine_behavior_regression_test.go`

### internal/mangle/engine_clear_query_test.go (1)
- [ ] `internal/mangle/engine_clear_query_test.go`

### internal/mangle/engine_control_facts_test.go (1)
- [ ] `internal/mangle/engine_control_facts_test.go`

### internal/mangle/engine_coverage_regression_test.go (1)
- [ ] `internal/mangle/engine_coverage_regression_test.go`

### internal/mangle/engine_evaluate_test.go (1)
- [ ] `internal/mangle/engine_evaluate_test.go`

### internal/mangle/engine_fact_arg_encode_test.go (1)
- [ ] `internal/mangle/engine_fact_arg_encode_test.go`

### internal/mangle/engine_fact_string_test.go (1)
- [ ] `internal/mangle/engine_fact_string_test.go`

### internal/mangle/engine_failclosed_test.go (1)
- [ ] `internal/mangle/engine_failclosed_test.go`

### internal/mangle/engine_gap_survey_test.go (1)
- [ ] `internal/mangle/engine_gap_survey_test.go`

### internal/mangle/engine_harden_gap_regression_test.go (1)
- [ ] `internal/mangle/engine_harden_gap_regression_test.go`

### internal/mangle/engine_harden_step2_final_regression_test.go (1)
- [ ] `internal/mangle/engine_harden_step2_final_regression_test.go`

### internal/mangle/engine_harden_step2b_regression_test.go (1)
- [ ] `internal/mangle/engine_harden_step2b_regression_test.go`

### internal/mangle/engine_harden_step2c_regression_test.go (1)
- [ ] `internal/mangle/engine_harden_step2c_regression_test.go`

### internal/mangle/engine_harden_step2e_regression_test.go (1)
- [ ] `internal/mangle/engine_harden_step2e_regression_test.go`

### internal/mangle/engine_mode_synth_test.go (1)
- [ ] `internal/mangle/engine_mode_synth_test.go`

### internal/mangle/engine_step1_gap_probe_test.go (1)
- [ ] `internal/mangle/engine_step1_gap_probe_test.go`

### internal/mangle/engine_step2_gap_regression_test.go (1)
- [ ] `internal/mangle/engine_step2_gap_regression_test.go`

### internal/mangle/engine_step2_regression_test.go (1)
- [ ] `internal/mangle/engine_step2_regression_test.go`

### internal/mangle/engine_test.go (1)
- [ ] `internal/mangle/engine_test.go`

### internal/mangle/engine_uplift_test.go (1)
- [x] `internal/mangle/engine_uplift_test.go`
- [x] `internal/mangle/doc600_pin_test.go`
- [x] `internal/mangle/corpus_gate_test.go`
- [x] `internal/core/kernel_boot_uplift_test.go`
- [x] `internal/core/kernel_facts_uplift_test.go`
- [x] `internal/core/kernel_sysfacts.go`
- [x] `internal/core/kernel_query_uplift_test.go`
- [x] `internal/core/kernel_eval_uplift_test.go`

### internal/mangle/fact_store_proxy_test.go (1)
- [ ] `internal/mangle/fact_store_proxy_test.go`

### internal/mangle/feedback (18)
- [x] `internal/mangle/feedback/error_classifier.go`
- [ ] `internal/mangle/feedback/error_classifier_benchmark_test.go`
- [ ] `internal/mangle/feedback/error_classifier_test.go`
- [ ] `internal/mangle/feedback/feedback_test.go`
- [ ] `internal/mangle/feedback/jit_test.go`
- [x] `internal/mangle/feedback/loop.go`
- [ ] `internal/mangle/feedback/loop_test.go`
- [x] `internal/mangle/feedback/normalize.go`
- [x] `internal/mangle/feedback/normalize_test.go`
- [x] `internal/mangle/feedback/pre_validator.go`
- [ ] `internal/mangle/feedback/pre_validator_bench_test.go`
- [ ] `internal/mangle/feedback/pre_validator_benchmark_test.go`
- [ ] `internal/mangle/feedback/pre_validator_test.go`
- [x] `internal/mangle/feedback/prompt_builder.go`
- [ ] `internal/mangle/feedback/prompt_builder_test.go`
- [x] `internal/mangle/feedback/types.go`
- [ ] `internal/mangle/feedback/types_test.go`
- [x] `internal/mangle/feedback/uplift_test.go`

### internal/mangle/grammar.go (1)
- [x] `internal/mangle/grammar.go`

### internal/mangle/grammar_argtype_test.go (1)
- [ ] `internal/mangle/grammar_argtype_test.go`

### internal/mangle/grammar_fuzz_test.go (1)
- [ ] `internal/mangle/grammar_fuzz_test.go`

### internal/mangle/grammar_helpers_test.go (1)
- [ ] `internal/mangle/grammar_helpers_test.go`

### internal/mangle/grammar_internal_test.go (1)
- [ ] `internal/mangle/grammar_internal_test.go`

### internal/mangle/grammar_test.go (1)
- [ ] `internal/mangle/grammar_test.go`

### internal/mangle/inspect_test.go (1)
- [ ] `internal/mangle/inspect_test.go`

### internal/mangle/intent_imports_test.go (1)
- [ ] `internal/mangle/intent_imports_test.go`

### internal/mangle/intent_wiring_test.go (1)
- [ ] `internal/mangle/intent_wiring_test.go`

### internal/mangle/lsp.go (1)
- [x] `internal/mangle/lsp.go`

### internal/mangle/lsp_helpers_test.go (1)
- [ ] `internal/mangle/lsp_helpers_test.go`

### internal/mangle/lsp_test.go (1)
- [ ] `internal/mangle/lsp_test.go`

### internal/mangle/mangle_validation_test.go (1)
- [ ] `internal/mangle/mangle_validation_test.go`

### internal/mangle/parse_callers_integration_test.go (1)
- [ ] `internal/mangle/parse_callers_integration_test.go`

### internal/mangle/parse_lock.go (1)
- [x] `internal/mangle/parse_lock.go`

### internal/mangle/parse_lock_test.go (1)
- [ ] `internal/mangle/parse_lock_test.go`

### internal/mangle/proof_tree.go (1)
- [x] `internal/mangle/proof_tree.go`

### internal/mangle/proof_tree_test.go (1)
- [ ] `internal/mangle/proof_tree_test.go`

### internal/mangle/proof_tree_uplift_test.go (1)
- [ ] `internal/mangle/proof_tree_uplift_test.go`

### internal/mangle/schema_validator.go (1)
- [x] `internal/mangle/schema_validator.go`

### internal/mangle/schema_validator_program_test.go (1)
- [ ] `internal/mangle/schema_validator_program_test.go`

### internal/mangle/schema_validator_test.go (1)
- [ ] `internal/mangle/schema_validator_test.go`

### internal/mangle/schema_validator_uplift_test.go (1)
- [ ] `internal/mangle/schema_validator_uplift_test.go`

### internal/mangle/simd_intersect_generic.go (1)
- [x] `internal/mangle/simd_intersect_generic.go`

### internal/mangle/simd_intersect_generic_test.go (1)
- [ ] `internal/mangle/simd_intersect_generic_test.go`

### internal/mangle/simd_intersect_test.go (1)
- [ ] `internal/mangle/simd_intersect_test.go`

### internal/mangle/synth (14)
- [x] `internal/mangle/synth/compile.go`
- [x] `internal/mangle/synth/compile_test.go`
- [x] `internal/mangle/synth/decoder.go`
- [ ] `internal/mangle/synth/decoder_fromresponse_test.go`
- [ ] `internal/mangle/synth/decoder_more_test.go`
- [ ] `internal/mangle/synth/decoder_test.go`
- [x] `internal/mangle/synth/schema.go`
- [ ] `internal/mangle/synth/schema_test.go`
- [x] `internal/mangle/synth/spec.go`
- [ ] `internal/mangle/synth/spec_test.go`
- [ ] `internal/mangle/synth/synth_test.go`
- [x] `internal/mangle/synth/validate.go`
- [ ] `internal/mangle/synth/validate_test.go`
- [x] `internal/mangle/synth/uplift_test.go`

### internal/mangle/torture_test.go (1)
- [ ] `internal/mangle/torture_test.go`

### internal/mangle/transpiler (4)
- [x] `internal/mangle/transpiler/sanitizer.go`
- [ ] `internal/mangle/transpiler/sanitizer_atoms_test.go`
- [x] `internal/mangle/transpiler/sanitizer_test.go`
- [x] `internal/mangle/transpiler/uplift_test.go`

### internal/mangle/verification_logic_test.go (1)
- [ ] `internal/mangle/verification_logic_test.go`

### internal/mcp (52)
- [ ] `internal/mcp/analyzer.go`
- [ ] `internal/mcp/analyzer_coverage_test.go`
- [ ] `internal/mcp/analyzer_test.go`
- [ ] `internal/mcp/client.go`
- [ ] `internal/mcp/client_boundary_test.go`
- [ ] `internal/mcp/client_coverage_test.go`
- [ ] `internal/mcp/client_test.go`
- [ ] `internal/mcp/compiler.go`
- [ ] `internal/mcp/compiler_select_test.go`
- [ ] `internal/mcp/compiler_test.go`
- [ ] `internal/mcp/concurrency_test.go`
- [ ] `internal/mcp/controlplane.go`
- [ ] `internal/mcp/controlplane_invoke.go`
- [ ] `internal/mcp/controlplane_test.go`
- [ ] `internal/mcp/digest.go`
- [ ] `internal/mcp/digest_test.go`
- [ ] `internal/mcp/expand_brutal_test.go`
- [ ] `internal/mcp/export_test.go`
- [ ] `internal/mcp/facets.go`
- [ ] `internal/mcp/facets_test.go`
- [ ] `internal/mcp/facts.go`
- [ ] `internal/mcp/facts_lifecycle_test.go`
- [ ] `internal/mcp/fake_server_test.go`
- [ ] `internal/mcp/handles.go`
- [ ] `internal/mcp/handles_test.go`
- [ ] `internal/mcp/headers.go`
- [ ] `internal/mcp/integration.go`
- [ ] `internal/mcp/integration_bridge_test.go`
- [ ] `internal/mcp/integration_coverage_test.go`
- [ ] `internal/mcp/integration_test.go`
- [ ] `internal/mcp/kernel_integration_test.go`
- [ ] `internal/mcp/mcp_client_integration_test.go`
- [ ] `internal/mcp/metrics.go`
- [ ] `internal/mcp/metrics_test.go`
- [ ] `internal/mcp/policy_golden_test.go`
- [ ] `internal/mcp/redact.go`
- [ ] `internal/mcp/redact_test.go`
- [ ] `internal/mcp/renderer.go`
- [ ] `internal/mcp/renderer_coverage_test.go`
- [ ] `internal/mcp/resources.go`
- [ ] `internal/mcp/signature.go`
- [ ] `internal/mcp/signature_test.go`
- [ ] `internal/mcp/store.go`
- [ ] `internal/mcp/store_coverage_test.go`
- [ ] `internal/mcp/store_query_test.go`
- [ ] `internal/mcp/store_test.go`
- [ ] `internal/mcp/transport_http.go`
- [ ] `internal/mcp/transport_http_test.go`
- [ ] `internal/mcp/transport_sse.go`
- [ ] `internal/mcp/transport_stdio.go`
- [ ] `internal/mcp/types.go`
- [ ] `internal/mcp/types_coverage_test.go`

### internal/northstar (25)
- [ ] `internal/northstar/alignment_prompt.go`
- [ ] `internal/northstar/alignment_prompt_test.go`
- [ ] `internal/northstar/bridge.go`
- [ ] `internal/northstar/bridge_test.go`
- [ ] `internal/northstar/campaign_observer.go`
- [ ] `internal/northstar/docs.go`
- [ ] `internal/northstar/docs_test.go`
- [ ] `internal/northstar/facts_links_test.go`
- [ ] `internal/northstar/guardian.go`
- [ ] `internal/northstar/guardian_querier_wiring_test.go`
- [ ] `internal/northstar/guardian_test.go`
- [ ] `internal/northstar/guardian_warn_test.go`
- [ ] `internal/northstar/guardian_wiring_test.go`
- [ ] `internal/northstar/kernel_integration_test.go`
- [ ] `internal/northstar/module.go`
- [ ] `internal/northstar/module_test.go`
- [ ] `internal/northstar/ns6_diag_test.go`
- [ ] `internal/northstar/observer.go`
- [ ] `internal/northstar/observer_test.go`
- [ ] `internal/northstar/registry.go`
- [ ] `internal/northstar/store.go`
- [ ] `internal/northstar/store_test.go`
- [ ] `internal/northstar/types.go`
- [ ] `internal/northstar/types_facts_test.go`
- [ ] `internal/northstar/types_test.go`

### internal/observability (6)
- [ ] `internal/observability/flight_recorder.go`
- [ ] `internal/observability/flight_recorder_lifecycle_test.go`
- [ ] `internal/observability/flight_recorder_test.go`
- [ ] `internal/observability/flight_recorder_watchdog_test.go`
- [ ] `internal/observability/runtime_metrics.go`
- [ ] `internal/observability/runtime_metrics_test.go`

### internal/observation (8)
- [ ] `internal/observation/codesearch.go`
- [ ] `internal/observation/codesearch_test.go`
- [ ] `internal/observation/doc.go`
- [ ] `internal/observation/fileread.go`
- [ ] `internal/observation/fileread_test.go`
- [ ] `internal/observation/precondition/precondition.go`
- [ ] `internal/observation/precondition/precondition_test.go`
- [ ] `internal/observation/subagent.go`

### internal/perception (131)
- [ ] `internal/perception/assault_verb_test.go`
- [ ] `internal/perception/benchmark_test.go`
- [ ] `internal/perception/break_test.go`
- [ ] `internal/perception/broker_install.go`
- [ ] `internal/perception/broker_sentinel_test.go`
- [ ] `internal/perception/claude_cli_client.go`
- [ ] `internal/perception/claude_cli_client_test.go`
- [ ] `internal/perception/cli_client_identity_test.go`
- [ ] `internal/perception/client.go`
- [ ] `internal/perception/client_anthropic.go`
- [ ] `internal/perception/client_factory.go`
- [ ] `internal/perception/client_factory_extra_test.go`
- [ ] `internal/perception/client_factory_test.go`
- [ ] `internal/perception/client_gemini.go`
- [ ] `internal/perception/client_gemini_files.go`
- [ ] `internal/perception/client_gemini_files_test.go`
- [ ] `internal/perception/client_gemini_getters_test.go`
- [ ] `internal/perception/client_gemini_http_test.go`
- [ ] `internal/perception/client_gemini_schema_test.go`
- [ ] `internal/perception/client_gemini_streaming.go`
- [ ] `internal/perception/client_gemini_test.go`
- [ ] `internal/perception/client_gemini_toolcalls_test.go`
- [ ] `internal/perception/client_gemini_tools.go`
- [ ] `internal/perception/client_meta_model_test.go`
- [ ] `internal/perception/client_meta_piggyback_test.go`
- [ ] `internal/perception/client_meta_responses.go`
- [ ] `internal/perception/client_meta_responses_concurrency_test.go`
- [ ] `internal/perception/client_meta_responses_dedup_test.go`
- [ ] `internal/perception/client_meta_responses_retry_test.go`
- [ ] `internal/perception/client_ollama.go`
- [ ] `internal/perception/client_ollama_test.go`
- [ ] `internal/perception/client_openai.go`
- [ ] `internal/perception/client_openai_compat.go`
- [ ] `internal/perception/client_openai_compat_allow_empty_test.go`
- [ ] `internal/perception/client_openai_compat_empty_retry_test.go`
- [ ] `internal/perception/client_openai_compat_grounded_supports_test.go`
- [ ] `internal/perception/client_openai_compat_grounding.go`
- [ ] `internal/perception/client_openai_compat_grounding_test.go`
- [ ] `internal/perception/client_openai_compat_live_test.go`
- [ ] `internal/perception/client_openai_compat_meta_tools_test.go`
- [ ] `internal/perception/client_openai_compat_test.go`
- [ ] `internal/perception/client_openai_compat_truncation_test.go`
- [ ] `internal/perception/client_openai_http_test.go`
- [ ] `internal/perception/client_openrouter.go`
- [ ] `internal/perception/client_planner_slot_test.go`
- [ ] `internal/perception/client_schema.go`
- [ ] `internal/perception/client_schema_strict_test.go`
- [ ] `internal/perception/client_tool_helpers.go`
- [ ] `internal/perception/client_tool_helpers_test.go`
- [ ] `internal/perception/client_types.go`
- [ ] `internal/perception/client_worker_providers_test.go`
- [ ] `internal/perception/client_xai.go`
- [ ] `internal/perception/client_zai.go`
- [ ] `internal/perception/client_zai_retry.go`
- [ ] `internal/perception/client_zai_retry_test.go`
- [ ] `internal/perception/client_zai_streaming.go`
- [ ] `internal/perception/client_zai_test.go`
- [ ] `internal/perception/codex_cli_client.go`
- [ ] `internal/perception/codex_cli_client_test.go`
- [ ] `internal/perception/codex_cli_probe.go`
- [ ] `internal/perception/codex_cli_probe_test.go`
- [ ] `internal/perception/codex_exec_client.go`
- [x] `internal/perception/consolidation.go`
- [ ] `internal/perception/context_token_phrase_unification_test.go`
- [ ] `internal/perception/debug.go`
- [ ] `internal/perception/gemini_live_test.go`
- [ ] `internal/perception/gemini_structured_test.go`
- [ ] `internal/perception/gemini_thinking_test.go`
- [x] `internal/perception/learning.go`
- [ ] `internal/perception/learning_test.go`
- [ ] `internal/perception/metrics.go`
- [ ] `internal/perception/piggyback_contract_test.go`
- [ ] `internal/perception/scanner_pool.go`
- [x] `internal/perception/semantic_classifier.go`
- [ ] `internal/perception/semantic_classifier_test.go`
- [x] `internal/perception/taxonomy.go`
- [ ] `internal/perception/taxonomy_benchmark_test.go`
- [ ] `internal/perception/taxonomy_extra_test.go`
- [x] `internal/perception/taxonomy_persistence.go`
- [ ] `internal/perception/taxonomy_persistence_test.go`
- [ ] `internal/perception/taxonomy_test.go`
- [ ] `internal/perception/tracing_client.go`
- [ ] `internal/perception/tracing_client_extra_test.go`
- [ ] `internal/perception/tracing_client_test.go`
- [ ] `internal/perception/tracing_grounded_test.go`
- [ ] `internal/perception/tracing_tool_results_test.go`
- [x] `internal/perception/transducer.go`
- [ ] `internal/perception/transducer_coverage_test.go`
- [ ] `internal/perception/transducer_extra_test.go`
- [x] `internal/perception/transducer_gemini.go`
- [ ] `internal/perception/transducer_gemini_extra_test.go`
- [ ] `internal/perception/transducer_live_test.go`
- [x] `internal/perception/transducer_llm.go`
- [ ] `internal/perception/transducer_llm_extra2_test.go`
- [ ] `internal/perception/transducer_llm_test.go`
- [ ] `internal/perception/transducer_prompt_bounds_test.go`
- [ ] `internal/perception/transducer_unit_test.go`
- [ ] `internal/perception/transport.go`
- [ ] `internal/perception/transport_pool_test.go`
- [ ] `internal/perception/truncation.go`
- [x] `internal/perception/understanding.go`
- [x] `internal/perception/understanding_adapter.go`
- [ ] `internal/perception/understanding_adapter_extra_test.go`
- [ ] `internal/perception/understanding_adapter_test.go`
- [ ] `internal/perception/understanding_adapter_transient_test.go`
- [ ] `internal/perception/understanding_verbs_test.go`
- [ ] `internal/perception/usage_track.go`
- [ ] `internal/perception/usage_track_test.go`
- [ ] `internal/perception/utils.go`
- [ ] `internal/perception/utils_coverage_test.go`
- [ ] `internal/perception/xai_torture_test.go`
- [ ] `internal/perception/xaioauth/auth_device.go`
- [ ] `internal/perception/xaioauth/chat.go`
- [ ] `internal/perception/xaioauth/chat_test.go`
- [ ] `internal/perception/xaioauth/classification_test.go`
- [ ] `internal/perception/xaioauth/client.go`
- [ ] `internal/perception/xaioauth/config.go`
- [ ] `internal/perception/xaioauth/doc.go`
- [ ] `internal/perception/xaioauth/errors.go`
- [ ] `internal/perception/xaioauth/errors_test.go`
- [ ] `internal/perception/xaioauth/grok_auth_import.go`
- [ ] `internal/perception/xaioauth/probe.go`
- [ ] `internal/perception/xaioauth/store.go`
- [ ] `internal/perception/xaioauth/store_test.go`
- [ ] `internal/perception/xaioauth/streaming.go`
- [ ] `internal/perception/xaioauth/token.go`
- [ ] `internal/perception/xaioauth/token_quarantine_test.go`
- [ ] `internal/perception/xaioauth/token_test.go`
- [ ] `internal/perception/xaioauth/tools.go`
- [ ] `internal/perception/xaioauth/transport.go`
- [ ] `internal/perception/zai_live_test.go`

### internal/persist (10)
- [ ] `internal/persist/doc.go`
- [ ] `internal/persist/factsnap/codec_parity_test.go`
- [ ] `internal/persist/factsnap/factsnap.go`
- [ ] `internal/persist/factsnap/factsnap_codec_test.go`
- [ ] `internal/persist/factsnap/factsnap_robustness_test.go`
- [ ] `internal/persist/factsnap/factsnap_test.go`
- [ ] `internal/persist/factsnap/legacy_test.go`
- [ ] `internal/persist/snapshot/kernel_roundtrip_test.go`
- [ ] `internal/persist/snapshot/snapshot.go`
- [ ] `internal/persist/snapshot/snapshot_test.go`

### internal/processutil (5)
- [ ] `internal/processutil/cancellation_test.go`
- [ ] `internal/processutil/command.go`
- [ ] `internal/processutil/command_test.go`
- [ ] `internal/processutil/proc_tree_other.go`
- [ ] `internal/processutil/proc_tree_windows.go`

### internal/projectdoc (12)
- [ ] `internal/projectdoc/facts.go`
- [ ] `internal/projectdoc/gate.go`
- [ ] `internal/projectdoc/gofmt_gate_test.go`
- [ ] `internal/projectdoc/loadall_test.go`
- [ ] `internal/projectdoc/nerdmd.go`
- [ ] `internal/projectdoc/nerdmd_test.go`
- [ ] `internal/projectdoc/northstar_facts_test.go`
- [ ] `internal/projectdoc/northstar_schema_test.go`
- [ ] `internal/projectdoc/prompt_section_bounds_test.go`
- [ ] `internal/projectdoc/readfile_tool_test.go`
- [ ] `internal/projectdoc/tool_gate.go`
- [ ] `internal/projectdoc/tool_gate_test.go`

### internal/prompt (103)
- [ ] `internal/prompt/assembler.go`
- [ ] `internal/prompt/assembler_gaps_test.go`
- [ ] `internal/prompt/assembler_test.go`
- [ ] `internal/prompt/atom_pinning_test.go`
- [ ] `internal/prompt/atom_schema.go`
- [ ] `internal/prompt/atom_schema_test.go`
- [ ] `internal/prompt/atoms.go`
- [ ] `internal/prompt/atoms_placeholder_test.go`
- [ ] `internal/prompt/atoms_test.go`
- [ ] `internal/prompt/atoms_verification_test.go`
- [ ] `internal/prompt/baseline.go`
- [ ] `internal/prompt/budget.go`
- [ ] `internal/prompt/budget_enforcement_test.go`
- [ ] `internal/prompt/budget_test.go`
- [ ] `internal/prompt/capability_gating_test.go`
- [ ] `internal/prompt/codedom_policy_test.go`
- [ ] `internal/prompt/compile_shard_test.go`
- [ ] `internal/prompt/compiler.go`
- [ ] `internal/prompt/compiler_boundary_test.go`
- [ ] `internal/prompt/compiler_db.go`
- [ ] `internal/prompt/compiler_expert_knowledge_test.go`
- [ ] `internal/prompt/compiler_gaps_test.go`
- [ ] `internal/prompt/compiler_kernel_atoms_test.go`
- [ ] `internal/prompt/compiler_options.go`
- [ ] `internal/prompt/compiler_scope_test.go`
- [ ] `internal/prompt/compiler_shutdown_test.go`
- [ ] `internal/prompt/compiler_specialists.go`
- [ ] `internal/prompt/compiler_test.go`
- [ ] `internal/prompt/config_defaults.go`
- [ ] `internal/prompt/config_factory.go`
- [ ] `internal/prompt/config_factory_all_verbs_test.go`
- [ ] `internal/prompt/config_factory_mcp_test.go`
- [ ] `internal/prompt/config_factory_taxonomy_test.go`
- [ ] `internal/prompt/config_factory_test.go`
- [ ] `internal/prompt/config_generation_test.go`
- [ ] `internal/prompt/config_policy_registry_test.go`
- [ ] `internal/prompt/config_registry.go`
- [ ] `internal/prompt/config_registry_test.go`
- [ ] `internal/prompt/context.go`
- [ ] `internal/prompt/context_hash_test.go`
- [ ] `internal/prompt/context_test.go`
- [ ] `internal/prompt/couse.go`
- [ ] `internal/prompt/couse_default.go`
- [ ] `internal/prompt/couse_log_handle_test.go`
- [ ] `internal/prompt/couse_persist.go`
- [ ] `internal/prompt/couse_report.go`
- [ ] `internal/prompt/couse_test.go`
- [ ] `internal/prompt/couse_wiring_test.go`
- [ ] `internal/prompt/debugging_atoms_test.go`
- [ ] `internal/prompt/default_corpus.go`
- [ ] `internal/prompt/default_corpus_benchmark_test.go`
- [ ] `internal/prompt/embedded.go`
- [ ] `internal/prompt/embedded_test.go`
- [ ] `internal/prompt/evolved_atoms.go`
- [ ] `internal/prompt/grounded_web_search_test.go`
- [ ] `internal/prompt/kernel_injection_bounds_test.go`
- [ ] `internal/prompt/learning_evidence_test.go`
- [ ] `internal/prompt/limits.go`
- [ ] `internal/prompt/limits_test.go`
- [ ] `internal/prompt/loader.go`
- [ ] `internal/prompt/loader_bench_test.go`
- [ ] `internal/prompt/loader_embedding.go`
- [ ] `internal/prompt/loader_example_test.go`
- [ ] `internal/prompt/loader_test.go`
- [ ] `internal/prompt/loader_yaml_fields_test.go`
- [ ] `internal/prompt/manifest.go`
- [ ] `internal/prompt/marathon/marathon.go`
- [ ] `internal/prompt/marathon/marathon_test.go`
- [ ] `internal/prompt/marathon/optimize.go`
- [ ] `internal/prompt/marathon/research.go`
- [ ] `internal/prompt/marathon/types.go`
- [ ] `internal/prompt/minify_whitespace_perf_test.go`
- [ ] `internal/prompt/output_mode.go`
- [ ] `internal/prompt/output_mode_campaign_test.go`
- [ ] `internal/prompt/pinning.go`
- [ ] `internal/prompt/pinning_test.go`
- [ ] `internal/prompt/predicate_selector.go`
- [ ] `internal/prompt/predicate_selector_test.go`
- [ ] `internal/prompt/prompt_gaps_test.go`
- [ ] `internal/prompt/query_expansion.go`
- [ ] `internal/prompt/query_expansion_semantic_test.go`
- [ ] `internal/prompt/query_expansion_test.go`
- [ ] `internal/prompt/reconciler.go`
- [ ] `internal/prompt/reconciler_test.go`
- [ ] `internal/prompt/refactoring_atoms_test.go`
- [ ] `internal/prompt/reload_failure_contract_test.go`
- [ ] `internal/prompt/render_mode_test.go`
- [ ] `internal/prompt/resolver.go`
- [ ] `internal/prompt/resolver_gaps_test.go`
- [ ] `internal/prompt/resolver_test.go`
- [ ] `internal/prompt/selector.go`
- [ ] `internal/prompt/selector_gaps_test.go`
- [ ] `internal/prompt/selector_test.go`
- [ ] `internal/prompt/selector_vector_score_test.go`
- [ ] `internal/prompt/shard_gating_test.go`
- [ ] `internal/prompt/specialist_benchmark_test.go`
- [ ] `internal/prompt/strategy_atoms.go`
- [ ] `internal/prompt/strategy_atoms_test.go`
- [ ] `internal/prompt/structured_output_corpus_test.go`
- [ ] `internal/prompt/sync/synchronizer.go`
- [ ] `internal/prompt/sync/synchronizer_test.go`
- [ ] `internal/prompt/vector_searcher.go`
- [ ] `internal/prompt/verify_pe1_test.go`

### internal/regression (7)
- [ ] `internal/regression/battery.go`
- [ ] `internal/regression/battery_features_test.go`
- [ ] `internal/regression/battery_test.go`
- [ ] `internal/regression/policy.go`
- [ ] `internal/regression/policy_test.go`
- [ ] `internal/regression/seed.go`
- [ ] `internal/regression/seed_test.go`

### internal/retain (3)
- [ ] `internal/retain/retain.go`
- [ ] `internal/retain/retain_brutal_test.go`
- [ ] `internal/retain/retain_test.go`

### internal/retrieval (19)
- [ ] `internal/retrieval/backend.go`
- [ ] `internal/retrieval/bounds_test.go`
- [ ] `internal/retrieval/facts.go`
- [ ] `internal/retrieval/facts_test.go`
- [ ] `internal/retrieval/go_imports.go`
- [ ] `internal/retrieval/invalidation.go`
- [ ] `internal/retrieval/metrics.go`
- [ ] `internal/retrieval/scanner_generic.go`
- [ ] `internal/retrieval/semantic.go`
- [ ] `internal/retrieval/semantic_tier_test.go`
- [ ] `internal/retrieval/sparse.go`
- [ ] `internal/retrieval/sparse_bench_test.go`
- [ ] `internal/retrieval/sparse_integration_test.go`
- [ ] `internal/retrieval/sparse_search_test.go`
- [ ] `internal/retrieval/sparse_test.go`
- [ ] `internal/retrieval/tiered_context.go`
- [ ] `internal/retrieval/tiered_context_coverage_test.go`
- [ ] `internal/retrieval/tiered_context_test.go`
- [ ] `internal/retrieval/wiring_test.go`

### internal/session (90)
- [ ] `internal/session/build_repair_regime_test.go`
- [ ] `internal/session/build_verify.go`
- [ ] `internal/session/build_verify_test.go`
- [ ] `internal/session/change_evidence.go`
- [ ] `internal/session/check_safety_real_kernel_test.go`
- [ ] `internal/session/check_safety_write_large_test.go`
- [ ] `internal/session/coverage_profile.go`
- [ ] `internal/session/coverage_profile_test.go`
- [ ] `internal/session/critic.go`
- [ ] `internal/session/critic_test.go`
- [ ] `internal/session/executor.go`
- [ ] `internal/session/executor_boundary_test.go`
- [ ] `internal/session/executor_budget_exhaustion_test.go`
- [ ] `internal/session/executor_capability_test.go`
- [ ] `internal/session/executor_gate_test.go`
- [ ] `internal/session/executor_history_test.go`
- [ ] `internal/session/executor_learning.go`
- [ ] `internal/session/executor_learning_e2e_test.go`
- [ ] `internal/session/executor_learning_test.go`
- [ ] `internal/session/executor_mangle_test.go`
- [ ] `internal/session/executor_mangle_updates_test.go`
- [ ] `internal/session/executor_memory.go`
- [ ] `internal/session/executor_memory_test.go`
- [ ] `internal/session/executor_no_tool_retry_test.go`
- [ ] `internal/session/executor_planner_routing_test.go`
- [ ] `internal/session/executor_process_test.go`
- [ ] `internal/session/executor_projectdoc_test.go`
- [ ] `internal/session/executor_retrieval_context_test.go`
- [ ] `internal/session/executor_test.go`
- [ ] `internal/session/executor_tool_context_test.go`
- [ ] `internal/session/executor_tool_loop_timeout_test.go`
- [ ] `internal/session/executor_tools.go`
- [ ] `internal/session/executor_usage_tracker_test.go`
- [ ] `internal/session/file_context_injection_test.go`
- [ ] `internal/session/gate_names.go`
- [ ] `internal/session/gate_names_test.go`
- [ ] `internal/session/history_bounds_test.go`
- [ ] `internal/session/hollow_success_test.go`
- [ ] `internal/session/journey_adversarial_test.go`
- [ ] `internal/session/journey_planned_steps_test.go`
- [ ] `internal/session/lsp_diagnostics.go`
- [ ] `internal/session/lsp_diagnostics_test.go`
- [ ] `internal/session/mocks_test.go`
- [ ] `internal/session/modularity.go`
- [ ] `internal/session/modularity_guard_verify_test.go`
- [ ] `internal/session/modularity_test.go`
- [ ] `internal/session/observed_return.go`
- [ ] `internal/session/pending_edit_path_validation_test.go`
- [ ] `internal/session/pending_edit_test.go`
- [ ] `internal/session/persistence.go`
- [ ] `internal/session/piggyback_promotion.go`
- [ ] `internal/session/piggyback_promotion_test.go`
- [ ] `internal/session/placeholder_test_guard_test.go`
- [ ] `internal/session/semantic_compressor.go`
- [ ] `internal/session/semantic_compressor_test.go`
- [ ] `internal/session/session_wiring_test.go`
- [ ] `internal/session/severity.go`
- [ ] `internal/session/severity_test.go`
- [ ] `internal/session/shell_gate_test.go`
- [ ] `internal/session/spawner.go`
- [ ] `internal/session/spawner_executor_config_test.go`
- [ ] `internal/session/spawner_gaps_test.go`
- [ ] `internal/session/spawner_improvements_test.go`
- [ ] `internal/session/spawner_test.go`
- [ ] `internal/session/subagent.go`
- [ ] `internal/session/subagent_test.go`
- [ ] `internal/session/subagent_tool_error_test.go`
- [ ] `internal/session/task_executor.go`
- [ ] `internal/session/task_executor_test.go`
- [ ] `internal/session/test_output_detector.go`
- [ ] `internal/session/test_verify.go`
- [ ] `internal/session/test_verify_test.go`
- [ ] `internal/session/testtool_test.go`
- [ ] `internal/session/tool_budget_controller.go`
- [ ] `internal/session/tool_budget_controller_test.go`
- [ ] `internal/session/turn_done_test.go`
- [ ] `internal/session/turn_summary.go`
- [ ] `internal/session/turn_summary_test.go`
- [ ] `internal/session/unverified_test_claim_test.go`
- [ ] `internal/session/user_agent_wiring_test.go`
- [ ] `internal/session/verify_created2_test.go`
- [ ] `internal/session/work_steps.go`
- [ ] `internal/session/work_steps_test.go`
- [ ] `internal/session/working_context.go`
- [ ] `internal/session/working_context_test.go`
- [ ] `internal/session/working_loop_gating_test.go`
- [ ] `internal/session/wrap_tool_loop_error_test.go`
- [ ] `internal/session/write_guards.go`
- [ ] `internal/session/write_mutation_tool_test.go`
- [ ] `internal/session/written_path_test.go`

### internal/shards/consultation.go (1)
- [ ] `internal/shards/consultation.go`

### internal/shards/consultation_test.go (1)
- [ ] `internal/shards/consultation_test.go`

### internal/shards/integration_path_test.go (1)
- [ ] `internal/shards/integration_path_test.go`

### internal/shards/matching.go (1)
- [ ] `internal/shards/matching.go`

### internal/shards/matching_classification_test.go (1)
- [ ] `internal/shards/matching_classification_test.go`

### internal/shards/matching_test.go (1)
- [ ] `internal/shards/matching_test.go`

### internal/shards/observer_integration_test.go (1)
- [ ] `internal/shards/observer_integration_test.go`

### internal/shards/observer_manager.go (1)
- [ ] `internal/shards/observer_manager.go`

### internal/shards/observer_manager_accessors_test.go (1)
- [ ] `internal/shards/observer_manager_accessors_test.go`

### internal/shards/observer_manager_test.go (1)
- [ ] `internal/shards/observer_manager_test.go`

### internal/shards/registration.go (1)
- [ ] `internal/shards/registration.go`

### internal/shards/registration_manifest_test.go (1)
- [ ] `internal/shards/registration_manifest_test.go`

### internal/shards/registration_test.go (1)
- [ ] `internal/shards/registration_test.go`

### internal/shards/requirements_interrogator.go (1)
- [ ] `internal/shards/requirements_interrogator.go`

### internal/shards/requirements_interrogator_test.go (1)
- [ ] `internal/shards/requirements_interrogator_test.go`

### internal/shards/shard_join_audit_test.go (1)
- [ ] `internal/shards/shard_join_audit_test.go`

### internal/shards/specialist_facts.go (1)
- [ ] `internal/shards/specialist_facts.go`

### internal/shards/system (42)
- [ ] `internal/shards/system/action_pipeline_test.go`
- [ ] `internal/shards/system/base.go`
- [ ] `internal/shards/system/base_coverage_test.go`
- [ ] `internal/shards/system/base_shard_pattern_test.go`
- [ ] `internal/shards/system/campaign_runner.go`
- [ ] `internal/shards/system/campaign_runner_test.go`
- [ ] `internal/shards/system/constitution.go`
- [ ] `internal/shards/system/constitution_coverage_test.go`
- [ ] `internal/shards/system/constitution_ownership_test.go`
- [ ] `internal/shards/system/escalation_target_atom_test.go`
- [ ] `internal/shards/system/executive.go`
- [ ] `internal/shards/system/executive_autopoiesis.go`
- [ ] `internal/shards/system/executive_bench_test.go`
- [ ] `internal/shards/system/executive_coverage_test.go`
- [ ] `internal/shards/system/executive_helpers_test.go`
- [ ] `internal/shards/system/executive_intent.go`
- [ ] `internal/shards/system/executive_learning_test.go`
- [ ] `internal/shards/system/executive_ooda_test.go`
- [ ] `internal/shards/system/learning_test.go`
- [ ] `internal/shards/system/legislator.go`
- [ ] `internal/shards/system/mangle_repair.go`
- [ ] `internal/shards/system/mangle_repair_bench_test.go`
- [ ] `internal/shards/system/mangle_repair_test.go`
- [ ] `internal/shards/system/payloads.go`
- [ ] `internal/shards/system/perception.go`
- [ ] `internal/shards/system/perception_transient_test.go`
- [ ] `internal/shards/system/perception_validation_test.go`
- [ ] `internal/shards/system/planner.go`
- [ ] `internal/shards/system/planner_activation_test.go`
- [ ] `internal/shards/system/planner_test.go`
- [ ] `internal/shards/system/policy_action_routes_test.go`
- [ ] `internal/shards/system/policy_audit_route_test.go`
- [ ] `internal/shards/system/policy_mock_file_scale_test.go`
- [ ] `internal/shards/system/policy_optimize_route_test.go`
- [ ] `internal/shards/system/policy_query_verb_route_test.go`
- [ ] `internal/shards/system/policy_reasoning_model_test.go`
- [ ] `internal/shards/system/router.go`
- [ ] `internal/shards/system/router_escalation_test.go`
- [ ] `internal/shards/system/router_route_selection_test.go`
- [ ] `internal/shards/system/system_helpers_test.go`
- [ ] `internal/shards/system/world_model.go`
- [ ] `internal/shards/system/world_model_rootpath_test.go`

### internal/sqlpragmas (12)
- [ ] `internal/sqlpragmas/connector.go`
- [ ] `internal/sqlpragmas/corpus_coverage_test.go`
- [ ] `internal/sqlpragmas/features_test.go`
- [ ] `internal/sqlpragmas/golden_test.go`
- [ ] `internal/sqlpragmas/hostclass.go`
- [ ] `internal/sqlpragmas/imports_test.go`
- [ ] `internal/sqlpragmas/metrics.go`
- [ ] `internal/sqlpragmas/modernc_integration_test.go`
- [ ] `internal/sqlpragmas/open_site_audit_test.go`
- [ ] `internal/sqlpragmas/pragma_integration_test.go`
- [ ] `internal/sqlpragmas/pragmas.go`
- [ ] `internal/sqlpragmas/pragmas_test.go`

### internal/store (96)
- [ ] `internal/store/archival_test.go`
- [ ] `internal/store/cold_storage_integration_test.go`
- [ ] `internal/store/corpus_search_boundaries_test.go`
- [ ] `internal/store/embedded_store.go`
- [ ] `internal/store/fact_codec.go`
- [ ] `internal/store/fact_codec_int64_test.go`
- [ ] `internal/store/fact_codec_test.go`
- [ ] `internal/store/indexes.go`
- [ ] `internal/store/init_sqlite.go`
- [ ] `internal/store/init_vec.go`
- [ ] `internal/store/knowledge_lexical_db.go`
- [ ] `internal/store/knowledge_lexical_db_test.go`
- [ ] `internal/store/learned_store.go`
- [ ] `internal/store/learning.go`
- [ ] `internal/store/learning_bench_test.go`
- [ ] `internal/store/learning_candidates.go`
- [ ] `internal/store/learning_candidates_test.go`
- [ ] `internal/store/learning_content_test.go`
- [ ] `internal/store/learning_recall_regression_test.go`
- [ ] `internal/store/learning_reflection.go`
- [ ] `internal/store/learning_stats_test.go`
- [ ] `internal/store/local.go`
- [ ] `internal/store/local_cold.go`
- [ ] `internal/store/local_cold_extra_test.go`
- [ ] `internal/store/local_core.go`
- [ ] `internal/store/local_core_test.go`
- [ ] `internal/store/local_graph.go`
- [ ] `internal/store/local_graph_benchmark_test.go`
- [ ] `internal/store/local_graph_extra_test.go`
- [ ] `internal/store/local_graph_integration_test.go`
- [ ] `internal/store/local_graph_migration_test.go`
- [ ] `internal/store/local_graph_query.go`
- [ ] `internal/store/local_graph_test.go`
- [ ] `internal/store/local_knowledge.go`
- [ ] `internal/store/local_knowledge_benchmark_test.go`
- [ ] `internal/store/local_knowledge_extra_test.go`
- [ ] `internal/store/local_knowledge_recent_test.go`
- [ ] `internal/store/local_prompt.go`
- [ ] `internal/store/local_prompt_extra_test.go`
- [ ] `internal/store/local_prompt_selector_test.go`
- [ ] `internal/store/local_review.go`
- [ ] `internal/store/local_review_extra_test.go`
- [ ] `internal/store/local_session.go`
- [ ] `internal/store/local_session_extra_test.go`
- [ ] `internal/store/local_session_integration_test.go`
- [ ] `internal/store/local_session_test.go`
- [ ] `internal/store/local_vector.go`
- [ ] `internal/store/local_vector_test.go`
- [ ] `internal/store/local_verification.go`
- [ ] `internal/store/local_verification_extra_test.go`
- [ ] `internal/store/local_world.go`
- [ ] `internal/store/local_world_extra_test.go`
- [ ] `internal/store/migrations.go`
- [ ] `internal/store/migrations_benchmark_test.go`
- [ ] `internal/store/migrations_test.go`
- [ ] `internal/store/mocks_test.go`
- [ ] `internal/store/pragmas.go`
- [ ] `internal/store/prompt_reembed.go`
- [ ] `internal/store/prompt_reembed_benchmark_test.go`
- [ ] `internal/store/reembed_all.go`
- [ ] `internal/store/reembed_all_test.go`
- [ ] `internal/store/reflection_reembed.go`
- [ ] `internal/store/reflection_reembed_test.go`
- [ ] `internal/store/reflection_search.go`
- [ ] `internal/store/reflection_search_extra_test.go`
- [ ] `internal/store/reflection_utils.go`
- [ ] `internal/store/reflection_utils_test.go`
- [ ] `internal/store/reflection_worker.go`
- [ ] `internal/store/reflection_worker_test.go`
- [ ] `internal/store/serialization_test.go`
- [ ] `internal/store/tool_cleanup.go`
- [ ] `internal/store/tool_cleanup_extra_test.go`
- [ ] `internal/store/tool_cleanup_test.go`
- [ ] `internal/store/tool_store.go`
- [ ] `internal/store/tool_store_test.go`
- [ ] `internal/store/trace_reflection.go`
- [ ] `internal/store/trace_reflection_extra_test.go`
- [ ] `internal/store/trace_store.go`
- [ ] `internal/store/trace_store_integration_test.go`
- [ ] `internal/store/trace_store_test.go`
- [ ] `internal/store/vec_probe_test.go`
- [ ] `internal/store/vec_support_disabled.go`
- [ ] `internal/store/vec_support_enabled.go`
- [ ] `internal/store/vector_boundary_test.go`
- [ ] `internal/store/vector_e2e_test.go`
- [ ] `internal/store/vector_store.go`
- [ ] `internal/store/vector_store_batch_test.go`
- [ ] `internal/store/vector_store_benchmark_test.go`
- [ ] `internal/store/vector_store_brute_test.go`
- [ ] `internal/store/vector_store_bruteforce.go`
- [ ] `internal/store/vector_store_extra_test.go`
- [ ] `internal/store/vector_store_reembed.go`
- [ ] `internal/store/vector_store_search_test.go`
- [ ] `internal/store/vector_store_test.go`
- [ ] `internal/store/vector_utils.go`
- [ ] `internal/store/vector_utils_test.go`

### internal/system (59)
- [ ] `internal/system/agent_definition.go`
- [ ] `internal/system/agent_registry.go`
- [ ] `internal/system/agent_registry_coverage_test.go`
- [ ] `internal/system/boot_test.go`
- [ ] `internal/system/broker_meter.go`
- [ ] `internal/system/browser_wiring_test.go`
- [ ] `internal/system/close_releases_handles_test.go`
- [ ] `internal/system/containment_binding_test.go`
- [ ] `internal/system/cortex_close.go`
- [ ] `internal/system/cortex_close_test.go`
- [ ] `internal/system/cortex_permission_routing_test.go`
- [ ] `internal/system/decorator_chain_test.go`
- [ ] `internal/system/dom_demo_test.go`
- [ ] `internal/system/dom_mangle_test.go`
- [ ] `internal/system/expert_knowledge_integration_test.go`
- [ ] `internal/system/factory.go`
- [ ] `internal/system/factory_adapters.go`
- [ ] `internal/system/factory_adapters_float_test.go`
- [ ] `internal/system/factory_adapters_test.go`
- [ ] `internal/system/factory_adapters_usage_test.go`
- [ ] `internal/system/factory_boot_test.go`
- [ ] `internal/system/factory_cache_test.go`
- [ ] `internal/system/factory_execution.go`
- [ ] `internal/system/factory_execution_base_env_test.go`
- [ ] `internal/system/factory_execution_test.go`
- [ ] `internal/system/factory_gate_test.go`
- [ ] `internal/system/factory_helpers_test.go`
- [ ] `internal/system/factory_kernel_shards_test.go`
- [ ] `internal/system/factory_learning.go`
- [ ] `internal/system/factory_learning_test.go`
- [ ] `internal/system/factory_ouroboros_toolstore_test.go`
- [ ] `internal/system/factory_projectdoc_test.go`
- [ ] `internal/system/factory_rollback_test.go`
- [ ] `internal/system/factory_test.go`
- [ ] `internal/system/factory_tool_executor.go`
- [ ] `internal/system/factory_world_refresh_test.go`
- [ ] `internal/system/holographic_code_scope.go`
- [ ] `internal/system/impact_chain_e2e_test.go`
- [ ] `internal/system/learning_report.go`
- [ ] `internal/system/learning_report_test.go`
- [ ] `internal/system/maintenance_schedule_test.go`
- [ ] `internal/system/mocks_test.go`
- [ ] `internal/system/nontest_callers_test.go`
- [ ] `internal/system/ondemand_e2e_test.go`
- [ ] `internal/system/planner_e2e_test.go`
- [ ] `internal/system/planning_factory_ownership_test.go`
- [ ] `internal/system/prompt_capability_production_test.go`
- [ ] `internal/system/prompt_kernel_scope_test.go`
- [ ] `internal/system/router_e2e_test.go`
- [ ] `internal/system/session_kernel_adapter_test.go`
- [ ] `internal/system/session_wiring_test.go`
- [ ] `internal/system/system_shards_switch_test.go`
- [ ] `internal/system/test_impact_provider.go`
- [ ] `internal/system/test_impact_provider_test.go`
- [ ] `internal/system/tool_compilation_test.go`
- [ ] `internal/system/user_agent_prompt_test.go`
- [ ] `internal/system/virtual_store_test_helpers_test.go`
- [ ] `internal/system/workspace_root_env_test.go`
- [ ] `internal/system/world_shard_eval_bench_test.go`

### internal/tactile (35)
- [ ] `internal/tactile/audit.go`
- [ ] `internal/tactile/audit_test.go`
- [ ] `internal/tactile/base_env_test.go`
- [ ] `internal/tactile/coverage_boost_test.go`
- [ ] `internal/tactile/direct.go`
- [ ] `internal/tactile/direct_test.go`
- [ ] `internal/tactile/docker.go`
- [ ] `internal/tactile/docker_detection_test.go`
- [ ] `internal/tactile/docker_platform_test.go`
- [ ] `internal/tactile/docker_platform_windows_test.go`
- [ ] `internal/tactile/docker_test.go`
- [ ] `internal/tactile/execution_boundaries_test.go`
- [ ] `internal/tactile/executor_interface.go`
- [ ] `internal/tactile/factory.go`
- [ ] `internal/tactile/files.go`
- [ ] `internal/tactile/files_test.go`
- [ ] `internal/tactile/line_ending.go`
- [ ] `internal/tactile/line_ending_test.go`
- [ ] `internal/tactile/persistent_docker.go`
- [ ] `internal/tactile/platform_darwin.go`
- [ ] `internal/tactile/platform_linux.go`
- [ ] `internal/tactile/platform_linux_executor_test.go`
- [ ] `internal/tactile/platform_linux_firejail.go`
- [ ] `internal/tactile/platform_unix.go`
- [ ] `internal/tactile/platform_windows.go`
- [ ] `internal/tactile/python/environment.go`
- [ ] `internal/tactile/python/environment_test.go`
- [ ] `internal/tactile/swebench/coverage_boost_test.go`
- [ ] `internal/tactile/swebench/harness.go`
- [ ] `internal/tactile/swebench/harness_extra_test.go`
- [ ] `internal/tactile/swebench/instance.go`
- [ ] `internal/tactile/swebench/instance_test.go`
- [ ] `internal/tactile/tactile_test.go`
- [ ] `internal/tactile/types.go`
- [ ] `internal/tactile/types_coverage_test.go`

### internal/testing (29)
- [ ] `internal/testing/context_harness/activation_tracer.go`
- [ ] `internal/testing/context_harness/compression_viz.go`
- [ ] `internal/testing/context_harness/engine_interface.go`
- [ ] `internal/testing/context_harness/fact_seeder.go`
- [ ] `internal/testing/context_harness/feedback_test.go`
- [ ] `internal/testing/context_harness/feedback_tracer.go`
- [ ] `internal/testing/context_harness/file_logger.go`
- [ ] `internal/testing/context_harness/file_logger_test.go`
- [ ] `internal/testing/context_harness/harness.go`
- [ ] `internal/testing/context_harness/helpers_extra_test.go`
- [ ] `internal/testing/context_harness/inspector.go`
- [ ] `internal/testing/context_harness/jit_tracer.go`
- [ ] `internal/testing/context_harness/metrics.go`
- [ ] `internal/testing/context_harness/metrics_test.go`
- [ ] `internal/testing/context_harness/mock_engine.go`
- [ ] `internal/testing/context_harness/piggyback_tracer.go`
- [ ] `internal/testing/context_harness/real_engine.go`
- [ ] `internal/testing/context_harness/reporter.go`
- [ ] `internal/testing/context_harness/reporter_test.go`
- [ ] `internal/testing/context_harness/scenarios.go`
- [ ] `internal/testing/context_harness/scenarios_integration.go`
- [ ] `internal/testing/context_harness/seeder_logger_test.go`
- [ ] `internal/testing/context_harness/simulator.go`
- [ ] `internal/testing/context_harness/simulator_test.go`
- [ ] `internal/testing/context_harness/test_kernel_factory.go`
- [ ] `internal/testing/context_harness/tracer_helpers_test.go`
- [ ] `internal/testing/context_harness/types.go`
- [ ] `internal/testing/doc.go`
- [ ] `internal/testing/harness_subsystem.go`

### internal/testoutput (2)
- [ ] `internal/testoutput/testoutput.go`
- [ ] `internal/testoutput/testoutput_test.go`

### internal/tools/allowlist_test.go (1)
- [ ] `internal/tools/allowlist_test.go`

### internal/tools/args.go (1)
- [ ] `internal/tools/args.go`

### internal/tools/catalog_golden_test.go (1)
- [ ] `internal/tools/catalog_golden_test.go`

### internal/tools/catalog_policy_parity_test.go (1)
- [ ] `internal/tools/catalog_policy_parity_test.go`

### internal/tools/codedom (22)
- [ ] `internal/tools/codedom/apply_edits.go`
- [ ] `internal/tools/codedom/apply_edits_test.go`
- [ ] `internal/tools/codedom/benchmark_test.go`
- [ ] `internal/tools/codedom/directory_error_test.go`
- [ ] `internal/tools/codedom/doc.go`
- [ ] `internal/tools/codedom/edited_refs_test.go`
- [ ] `internal/tools/codedom/elements.go`
- [ ] `internal/tools/codedom/elements_containment_test.go`
- [ ] `internal/tools/codedom/elements_test.go`
- [ ] `internal/tools/codedom/extent_realfile_test.go`
- [ ] `internal/tools/codedom/impact_test.go`
- [ ] `internal/tools/codedom/line_ending_test.go`
- [ ] `internal/tools/codedom/lines.go`
- [ ] `internal/tools/codedom/lines_balance_test.go`
- [ ] `internal/tools/codedom/lines_precondition_test.go`
- [ ] `internal/tools/codedom/lines_shift_test.go`
- [ ] `internal/tools/codedom/lines_test.go`
- [ ] `internal/tools/codedom/register.go`
- [ ] `internal/tools/codedom/register_test.go`
- [ ] `internal/tools/codedom/run_impacted_tests.go`
- [ ] `internal/tools/codedom/run_impacted_tests_extra_test.go`
- [ ] `internal/tools/codedom/workspace_ctx_test.go`

### internal/tools/context_recall.go (1)
- [ ] `internal/tools/context_recall.go`

### internal/tools/core (23)
- [ ] `internal/tools/core/context_recall.go`
- [ ] `internal/tools/core/context_recall_test.go`
- [ ] `internal/tools/core/doc.go`
- [ ] `internal/tools/core/file_ops.go`
- [ ] `internal/tools/core/file_ops_delete_confirm_test.go`
- [ ] `internal/tools/core/file_ops_lines_test.go`
- [ ] `internal/tools/core/file_ops_notfound_test.go`
- [ ] `internal/tools/core/file_ops_test.go`
- [ ] `internal/tools/core/line_ending_test.go`
- [ ] `internal/tools/core/read_codec_test.go`
- [ ] `internal/tools/core/read_file_directory_test.go`
- [ ] `internal/tools/core/register.go`
- [ ] `internal/tools/core/register_test.go`
- [ ] `internal/tools/core/search.go`
- [ ] `internal/tools/core/search_codec_test.go`
- [ ] `internal/tools/core/search_containment_test.go`
- [ ] `internal/tools/core/search_default_path_test.go`
- [ ] `internal/tools/core/search_numeric_coercion_test.go`
- [ ] `internal/tools/core/search_test.go`
- [ ] `internal/tools/core/subagent.go`
- [ ] `internal/tools/core/workspace_guard.go`
- [ ] `internal/tools/core/workspace_guard_test.go`
- [ ] `internal/tools/core/workspace_relative_test.go`

### internal/tools/effects.go (1)
- [ ] `internal/tools/effects.go`

### internal/tools/effects_catalog_test.go (1)
- [ ] `internal/tools/effects_catalog_test.go`

### internal/tools/effects_conformance_test.go (1)
- [ ] `internal/tools/effects_conformance_test.go`

### internal/tools/effects_test.go (1)
- [ ] `internal/tools/effects_test.go`

### internal/tools/errors.go (1)
- [ ] `internal/tools/errors.go`

### internal/tools/framework.go (1)
- [ ] `internal/tools/framework.go`

### internal/tools/framework_test.go (1)
- [ ] `internal/tools/framework_test.go`

### internal/tools/mcpctl (4)
- [ ] `internal/tools/mcpctl/mcpctl.go`
- [ ] `internal/tools/mcpctl/register.go`
- [ ] `internal/tools/mcpctl/tools.go`
- [ ] `internal/tools/mcpctl/tools_test.go`

### internal/tools/policy_exceptions.go (1)
- [ ] `internal/tools/policy_exceptions.go`

### internal/tools/registry.go (1)
- [ ] `internal/tools/registry.go`

### internal/tools/registry_allowlist_test.go (1)
- [ ] `internal/tools/registry_allowlist_test.go`

### internal/tools/registry_boundary_test.go (1)
- [ ] `internal/tools/registry_boundary_test.go`

### internal/tools/registry_extra_test.go (1)
- [ ] `internal/tools/registry_extra_test.go`

### internal/tools/registry_test.go (1)
- [ ] `internal/tools/registry_test.go`

### internal/tools/registry_unregister_test.go (1)
- [ ] `internal/tools/registry_unregister_test.go`

### internal/tools/registry_write_guard_test.go (1)
- [ ] `internal/tools/registry_write_guard_test.go`

### internal/tools/research (36)
- [ ] `internal/tools/research/browser.go`
- [ ] `internal/tools/research/browser_audit.go`
- [ ] `internal/tools/research/browser_audit_test.go`
- [ ] `internal/tools/research/browser_binding_test.go`
- [ ] `internal/tools/research/browser_declarative.go`
- [ ] `internal/tools/research/browser_evidence.go`
- [ ] `internal/tools/research/browser_evidence_test.go`
- [ ] `internal/tools/research/browser_extract_test.go`
- [ ] `internal/tools/research/browser_progressive.go`
- [ ] `internal/tools/research/browser_progressive_live_test.go`
- [ ] `internal/tools/research/browser_reasoning.go`
- [ ] `internal/tools/research/browser_reasoning_container_test.go`
- [ ] `internal/tools/research/browser_reasoning_live_test.go`
- [ ] `internal/tools/research/browser_reasoning_test.go`
- [ ] `internal/tools/research/browser_specs.go`
- [ ] `internal/tools/research/browser_specs_test.go`
- [ ] `internal/tools/research/browser_test_test.go`
- [ ] `internal/tools/research/cache.go`
- [ ] `internal/tools/research/cache_disk.go`
- [ ] `internal/tools/research/cache_disk_test.go`
- [ ] `internal/tools/research/context7.go`
- [ ] `internal/tools/research/context7_no_docs_test.go`
- [ ] `internal/tools/research/context7_tool_test.go`
- [ ] `internal/tools/research/doc.go`
- [ ] `internal/tools/research/fetch_tool_test.go`
- [ ] `internal/tools/research/grounded_web_search.go`
- [ ] `internal/tools/research/grounded_web_search_test.go`
- [ ] `internal/tools/research/grounding.go`
- [ ] `internal/tools/research/numeric_args.go`
- [ ] `internal/tools/research/numeric_args_test.go`
- [ ] `internal/tools/research/register.go`
- [ ] `internal/tools/research/research_coverage_test.go`
- [ ] `internal/tools/research/research_test.go`
- [ ] `internal/tools/research/thinking.go`
- [ ] `internal/tools/research/web_fetch.go`
- [ ] `internal/tools/research/web_search.go`

### internal/tools/shell (20)
- [ ] `internal/tools/shell/builtins.go`
- [ ] `internal/tools/shell/builtins_containment_test.go`
- [ ] `internal/tools/shell/builtins_test.go`
- [ ] `internal/tools/shell/command_semantics.go`
- [ ] `internal/tools/shell/command_semantics_test.go`
- [ ] `internal/tools/shell/command_timeout.go`
- [ ] `internal/tools/shell/command_timeout_test.go`
- [ ] `internal/tools/shell/doc.go`
- [ ] `internal/tools/shell/execute.go`
- [ ] `internal/tools/shell/execute_extra_test.go`
- [ ] `internal/tools/shell/execute_test.go`
- [ ] `internal/tools/shell/proc_tree_other.go`
- [ ] `internal/tools/shell/proc_tree_windows.go`
- [ ] `internal/tools/shell/register.go`
- [ ] `internal/tools/shell/register_test.go`
- [ ] `internal/tools/shell/shell_integration_test.go`
- [ ] `internal/tools/shell/verification.go`
- [ ] `internal/tools/shell/verification_test.go`
- [ ] `internal/tools/shell/waitdelay_test.go`
- [ ] `internal/tools/shell/workspace_containment_test.go`

### internal/tools/types.go (1)
- [ ] `internal/tools/types.go`

### internal/tools/workspace_guard.go (1)
- [ ] `internal/tools/workspace_guard.go`

### internal/tools/workspace_guard_hardening_test.go (1)
- [ ] `internal/tools/workspace_guard_hardening_test.go`

### internal/tools/workspace_identity_test.go (1)
- [ ] `internal/tools/workspace_identity_test.go`

### internal/transparency (23)
- [ ] `internal/transparency/doc.go`
- [ ] `internal/transparency/error_classifier.go`
- [ ] `internal/transparency/error_classifier_test.go`
- [ ] `internal/transparency/event_bus.go`
- [ ] `internal/transparency/event_bus_category_test.go`
- [ ] `internal/transparency/event_bus_test.go`
- [ ] `internal/transparency/explainer.go`
- [ ] `internal/transparency/explainer_test.go`
- [ ] `internal/transparency/glass_box_events.go`
- [ ] `internal/transparency/glass_box_events_test.go`
- [ ] `internal/transparency/glass_box_helpers_test.go`
- [ ] `internal/transparency/ndjson_sink.go`
- [ ] `internal/transparency/observability_test.go`
- [ ] `internal/transparency/process.go`
- [ ] `internal/transparency/process_test.go`
- [ ] `internal/transparency/safety_reporter.go`
- [ ] `internal/transparency/safety_reporter_test.go`
- [ ] `internal/transparency/shard_observer.go`
- [ ] `internal/transparency/shard_observer_test.go`
- [ ] `internal/transparency/transparency.go`
- [ ] `internal/transparency/transparency_comprehensive_test.go`
- [ ] `internal/transparency/transparency_test.go`
- [ ] `internal/transparency/wiring_test.go`

### internal/types (28)
- [ ] `internal/types/atom.go`
- [ ] `internal/types/atom_test.go`
- [ ] `internal/types/container_toatom_test.go`
- [ ] `internal/types/ctxkeys.go`
- [ ] `internal/types/ctxkeys_test.go`
- [ ] `internal/types/example_test.go`
- [ ] `internal/types/extract.go`
- [ ] `internal/types/extract_test.go`
- [ ] `internal/types/fact_conventions_guard_test.go`
- [ ] `internal/types/fact_text.go`
- [ ] `internal/types/fact_text_test.go`
- [ ] `internal/types/interfaces.go`
- [ ] `internal/types/kernel_transactor_guard_test.go`
- [ ] `internal/types/mangle_roundtrip_external_test.go`
- [ ] `internal/types/mangle_scale.go`
- [ ] `internal/types/mangle_scale_test.go`
- [ ] `internal/types/mangle_string_test.go`
- [ ] `internal/types/path_identity.go`
- [ ] `internal/types/shard.go`
- [ ] `internal/types/shard_test.go`
- [ ] `internal/types/transaction.go`
- [ ] `internal/types/transparency.go`
- [ ] `internal/types/truncation.go`
- [ ] `internal/types/types.go`
- [ ] `internal/types/types_comprehensive_test.go`
- [ ] `internal/types/types_test.go`
- [ ] `internal/types/typestest/mockkernel.go`
- [ ] `internal/types/typestest/mockkernel_test.go`

### internal/usage (17)
- [ ] `internal/usage/crossprocess_test.go`
- [ ] `internal/usage/durability_test.go`
- [ ] `internal/usage/filelock_other.go`
- [ ] `internal/usage/filelock_test.go`
- [ ] `internal/usage/filelock_windows.go`
- [ ] `internal/usage/observer.go`
- [ ] `internal/usage/pricing.go`
- [ ] `internal/usage/pricing_test.go`
- [ ] `internal/usage/shared_ownership_test.go`
- [ ] `internal/usage/shared_tracker_test.go`
- [ ] `internal/usage/turn_tokens_test.go`
- [ ] `internal/usage/usage_comprehensive_test.go`
- [ ] `internal/usage/usage_tracker.go`
- [ ] `internal/usage/usage_tracker_context_test.go`
- [ ] `internal/usage/usage_tracker_test.go`
- [ ] `internal/usage/usage_types.go`
- [ ] `internal/usage/usage_types_test.go`

### internal/ux (8)
- [ ] `internal/ux/doc.go`
- [ ] `internal/ux/migration.go`
- [ ] `internal/ux/migration_extra_test.go`
- [ ] `internal/ux/migration_test.go`
- [ ] `internal/ux/preferences.go`
- [ ] `internal/ux/preferences_test.go`
- [ ] `internal/ux/user_state.go`
- [ ] `internal/ux/user_state_test.go`

### internal/verification (5)
- [ ] `internal/verification/verifier.go`
- [ ] `internal/verification/verifier_failclosed_test.go`
- [ ] `internal/verification/verifier_gaps_test.go`
- [ ] `internal/verification/verifier_normalize_test.go`
- [ ] `internal/verification/verifier_test.go`

### internal/world (105)
- [ ] `internal/world/apply_incremental.go`
- [ ] `internal/world/apply_incremental_test.go`
- [ ] `internal/world/ast.go`
- [ ] `internal/world/ast_test.go`
- [ ] `internal/world/ast_treesitter.go`
- [ ] `internal/world/ast_treesitter_bench_test.go`
- [ ] `internal/world/ast_treesitter_package_test.go`
- [ ] `internal/world/cache.go`
- [ ] `internal/world/cache_metrics_test.go`
- [ ] `internal/world/cache_test.go`
- [ ] `internal/world/canonical_path.go`
- [ ] `internal/world/canonical_path_test.go`
- [ ] `internal/world/cartographer.go`
- [ ] `internal/world/cartographer_multilang.go`
- [ ] `internal/world/cartographer_multilang_test.go`
- [ ] `internal/world/cartographer_test.go`
- [ ] `internal/world/code_elements.go`
- [ ] `internal/world/code_elements_extra_test.go`
- [ ] `internal/world/code_elements_mangle.go`
- [ ] `internal/world/code_elements_mangle_test.go`
- [ ] `internal/world/code_elements_patterns_test.go`
- [ ] `internal/world/code_elements_test.go`
- [ ] `internal/world/dataflow.go`
- [ ] `internal/world/dataflow_cache.go`
- [ ] `internal/world/dataflow_cache_test.go`
- [ ] `internal/world/dataflow_javascript.go`
- [ ] `internal/world/dataflow_multilang.go`
- [ ] `internal/world/dataflow_multilang_test.go`
- [ ] `internal/world/dataflow_python.go`
- [ ] `internal/world/dataflow_python_test.go`
- [ ] `internal/world/dataflow_rust.go`
- [ ] `internal/world/dataflow_test.go`
- [ ] `internal/world/decl_conformance_test.go`
- [ ] `internal/world/deep_scan.go`
- [ ] `internal/world/deep_scan_test.go`
- [ ] `internal/world/deleted_file_retraction_test.go`
- [ ] `internal/world/dependency_links.go`
- [ ] `internal/world/dependency_links_test.go`
- [ ] `internal/world/fs.go`
- [ ] `internal/world/fs_cache_test.go`
- [ ] `internal/world/fs_canonical_join_test.go`
- [ ] `internal/world/fs_test.go`
- [ ] `internal/world/git_scanner.go`
- [ ] `internal/world/git_scanner_test.go`
- [ ] `internal/world/go_parser.go`
- [ ] `internal/world/go_parser_test.go`
- [ ] `internal/world/graph_interface.go`
- [ ] `internal/world/graph_interface_test.go`
- [ ] `internal/world/holographic.go`
- [ ] `internal/world/holographic_cache.go`
- [ ] `internal/world/holographic_cache_test.go`
- [ ] `internal/world/holographic_dependencies.go`
- [ ] `internal/world/holographic_dependencies_test.go`
- [ ] `internal/world/holographic_formatting.go`
- [ ] `internal/world/holographic_formatting_test.go`
- [ ] `internal/world/holographic_impact.go`
- [ ] `internal/world/holographic_impact_chain_test.go`
- [ ] `internal/world/holographic_perf_test.go`
- [ ] `internal/world/holographic_querier_test.go`
- [ ] `internal/world/holographic_ranking_test.go`
- [ ] `internal/world/holographic_test.go`
- [ ] `internal/world/incremental_canonical_test.go`
- [ ] `internal/world/incremental_retire_test.go`
- [ ] `internal/world/incremental_scan.go`
- [ ] `internal/world/incremental_scan_test.go`
- [ ] `internal/world/lsp/client.go`
- [ ] `internal/world/lsp/client_test.go`
- [ ] `internal/world/lsp/manager.go`
- [ ] `internal/world/lsp/manager_extra_test.go`
- [ ] `internal/world/lsp/manager_initialize_test.go`
- [ ] `internal/world/lsp/manager_test.go`
- [ ] `internal/world/mangle_fastparse.go`
- [ ] `internal/world/mangle_fastparse_test.go`
- [ ] `internal/world/mangle_parser.go`
- [ ] `internal/world/parser_factory.go`
- [ ] `internal/world/parser_factory_test.go`
- [ ] `internal/world/parser_interface.go`
- [ ] `internal/world/parser_test.go`
- [ ] `internal/world/persist.go`
- [ ] `internal/world/persist_test.go`
- [ ] `internal/world/python_parser.go`
- [ ] `internal/world/reviewer_capabilities_test.go`
- [ ] `internal/world/runbook.go`
- [ ] `internal/world/rust_parser.go`
- [ ] `internal/world/scan_edge_test.go`
- [ ] `internal/world/scan_nerd_artifacts_test.go`
- [ ] `internal/world/scanner_config.go`
- [ ] `internal/world/scope.go`
- [ ] `internal/world/scope_identity_test.go`
- [ ] `internal/world/scope_mangle_test.go`
- [ ] `internal/world/scope_package_test.go`
- [ ] `internal/world/scope_predicates_conformance_test.go`
- [ ] `internal/world/scope_test.go`
- [ ] `internal/world/symbol_graph_atom_test.go`
- [ ] `internal/world/test_dependency.go`
- [ ] `internal/world/test_dependency_elements.go`
- [ ] `internal/world/test_dependency_test.go`
- [ ] `internal/world/test_file_for_test.go`
- [ ] `internal/world/testdata/large_file.go`
- [ ] `internal/world/types.go`
- [ ] `internal/world/typescript_parser.go`
- [ ] `internal/world/world_fact_path_test.go`
- [ ] `internal/world/world_ownership_test.go`
- [ ] `internal/world/world_predicates.go`
- [ ] `internal/world/world_predicates_test.go`

### scratch_omitzero (1)
- [ ] `scratch_omitzero/main.go`

### scripts (3)
- [ ] `scripts/probe_safety/main.go`
- [ ] `scripts/torture_live/main.go`
- [ ] `scripts/torture_live/main_test.go`

### tests (38)
- [ ] `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go`
- [ ] `tests/e2e/Session_Kernel_Boundary_integration_test.go`
- [ ] `tests/e2e/autopoiesis_kernel_ouroboros_integration_test.go`
- [ ] `tests/e2e/campaign_decomposer_orchestrator_integration_test.go`
- [ ] `tests/e2e/campaign_session_integration_test.go`
- [ ] `tests/e2e/cross_boundary_integration_test.go`
- [ ] `tests/e2e/dreamer_kernelclone_integration_test.go`
- [ ] `tests/e2e/dreamer_verdict_freshness_integration_test.go`
- [ ] `tests/e2e/dreamer_virtualstore_integration_test.go`
- [ ] `tests/e2e/intent_mangle_routing_integration_test.go`
- [ ] `tests/e2e/jit_kernel_context_cleanup_test.go`
- [ ] `tests/e2e/kernelquery_virtualstore_integration_test.go`
- [ ] `tests/e2e/mcp_virtualstore_integration_test.go`
- [ ] `tests/e2e/orchestrator_executor_integration_test.go`
- [ ] `tests/e2e/orchestrator_executor_race_integration_test.go`
- [ ] `tests/e2e/perception_adversarial_e2e_test.go`
- [ ] `tests/e2e/perception_contract_e2e_test.go`
- [ ] `tests/e2e/perception_stateful_e2e_test.go`
- [ ] `tests/e2e/piggyback_control_packet_boundary_test.go`
- [ ] `tests/e2e/piggyback_executor_full_boundary_test.go`
- [ ] `tests/e2e/prompt_compiler_llm_integration_test.go`
- [ ] `tests/e2e/promptcompiler_llmclient_integration_test.go`
- [ ] `tests/e2e/rulecourt_feedback_integration_test.go`
- [ ] `tests/e2e/scheduler_session_llm_integration_test.go`
- [ ] `tests/e2e/session_clean_loop_integration_test.go`
- [ ] `tests/e2e/session_context_isolation_test.go`
- [ ] `tests/e2e/session_executor_kernel_integration_test.go`
- [ ] `tests/e2e/session_kernel_vstore_integration_test.go`
- [ ] `tests/e2e/session_spawner_config_integration_test.go`
- [ ] `tests/e2e/shadowmode_commit_safety_boundary_test.go`
- [ ] `tests/e2e/spawner_apischeduler_integration_test.go`
- [ ] `tests/e2e/task_executor_async_lifecycle_test.go`
- [ ] `tests/e2e/tddloop_executor_integration_test.go`
- [ ] `tests/e2e/tool_safety_fallback_config_test.go`
- [ ] `tests/e2e/virtualstore_dreamer_integration_test.go`
- [ ] `tests/e2e/virtualstore_graphquery_integration_test.go`
- [ ] `tests/e2e/virtualstore_interactive_gate_integration_test.go`
- [ ] `tests/e2e/write_turn_fixture_test.go`

### tools (3)
- [ ] `tools/count_lines.go`
- [ ] `tools/count_lines_test.go`
- [ ] `tools/registry.go`

## Mangle files (278)
### .agent (34)
- [x] `.agent/skills/log-analyzer/assets/log-schema.mg`
- [x] `.agent/skills/log-analyzer/scripts/logquery/schema.mg`
- [x] `.agent/skills/mangle-programming/assets/codenerd-schemas.mg`
- [x] `.agent/skills/mangle-programming/assets/examples/access-control.mg`
- [x] `.agent/skills/mangle-programming/assets/examples/aggregation-patterns.mg`
- [x] `.agent/skills/mangle-programming/assets/examples/vulnerability-scanner.mg`
- [x] `.agent/skills/mangle-programming/assets/starter-policy.mg`
- [x] `.agent/skills/mangle-programming/assets/starter-schema.mg`
- [x] `.agent/skills/mangle-programming/scripts/examples/performance_antipatterns.mg`
- [x] `.agent/skills/stress-tester/assets/cyclic_rules.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/loops/cartesian_explosion.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/loops/direct_self_reference.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/loops/mutual_recursion.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/loops/unbounded_counter.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/safety/anonymous_misuse.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/safety/negation_order.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/safety/stratification_cycles.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/safety/unbound_head_vars.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/safety/unsafe_negation.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/structures/bracket_notation.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/structures/dot_notation.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/structures/json_syntax.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/assignment_operators.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/atom_string_confusion.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/inline_aggregation.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/lowercase_vars.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/missing_periods.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/souffle_syntax.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/syntactic/wrong_comments.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/types/atom_vs_string.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/types/hallucinated_functions.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/types/int_vs_float.mg`
- [ ] `.agent/skills/stress-tester/assets/mangle-adversarial/types/list_in_scalar.mg`
- [x] `.agent/skills/stress-tester/assets/stress_queries.mg`

### .agents (9)
- [x] `.agents/skills/log-analyzer/assets/log-schema.mg`
- [x] `.agents/skills/log-analyzer/scripts/logquery/schema.mg`
- [x] `.agents/skills/mangle-programming/assets/codenerd-schemas.mg`
- [x] `.agents/skills/mangle-programming/assets/examples/access-control.mg`
- [x] `.agents/skills/mangle-programming/assets/examples/aggregation-patterns.mg`
- [x] `.agents/skills/mangle-programming/assets/examples/vulnerability-scanner.mg`
- [x] `.agents/skills/mangle-programming/assets/starter-policy.mg`
- [x] `.agents/skills/mangle-programming/assets/starter-schema.mg`
- [x] `.agents/skills/mangle-programming/scripts/examples/performance_antipatterns.mg`

### .claude (9)
- [x] `.claude/skills/log-analyzer/assets/log-schema.mg`
- [x] `.claude/skills/log-analyzer/scripts/logquery/schema.mg`
- [x] `.claude/skills/mangle-programming/assets/codenerd-schemas.mg`
- [x] `.claude/skills/mangle-programming/assets/examples/access-control.mg`
- [x] `.claude/skills/mangle-programming/assets/examples/aggregation-patterns.mg`
- [x] `.claude/skills/mangle-programming/assets/examples/vulnerability-scanner.mg`
- [x] `.claude/skills/mangle-programming/assets/starter-policy.mg`
- [x] `.claude/skills/mangle-programming/assets/starter-schema.mg`
- [x] `.claude/skills/mangle-programming/scripts/examples/performance_antipatterns.mg`

### .codex (34)
- [x] `.codex/skills/log-analyzer/assets/log-schema.mg`
- [x] `.codex/skills/log-analyzer/scripts/logquery/schema.mg`
- [x] `.codex/skills/mangle-programming/assets/codenerd-schemas.mg`
- [x] `.codex/skills/mangle-programming/assets/examples/access-control.mg`
- [x] `.codex/skills/mangle-programming/assets/examples/aggregation-patterns.mg`
- [x] `.codex/skills/mangle-programming/assets/examples/vulnerability-scanner.mg`
- [x] `.codex/skills/mangle-programming/assets/starter-policy.mg`
- [x] `.codex/skills/mangle-programming/assets/starter-schema.mg`
- [x] `.codex/skills/mangle-programming/scripts/examples/performance_antipatterns.mg`
- [x] `.codex/skills/stress-tester/assets/cyclic_rules.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/loops/cartesian_explosion.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/loops/direct_self_reference.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/loops/mutual_recursion.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/loops/unbounded_counter.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/safety/anonymous_misuse.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/safety/negation_order.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/safety/stratification_cycles.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/safety/unbound_head_vars.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/safety/unsafe_negation.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/structures/bracket_notation.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/structures/dot_notation.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/structures/json_syntax.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/assignment_operators.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/atom_string_confusion.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/inline_aggregation.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/lowercase_vars.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/missing_periods.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/souffle_syntax.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/syntactic/wrong_comments.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/types/atom_vs_string.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/types/hallucinated_functions.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/types/int_vs_float.mg`
- [ ] `.codex/skills/stress-tester/assets/mangle-adversarial/types/list_in_scalar.mg`
- [x] `.codex/skills/stress-tester/assets/stress_queries.mg`

### .gemini (27)
- [x] `.gemini/skills/log-analyzer/assets/log-schema.mg`
- [x] `.gemini/skills/log-analyzer/scripts/logquery/schema.mg`
- [x] `.gemini/skills/stress-tester/assets/cyclic_rules.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/loops/cartesian_explosion.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/loops/direct_self_reference.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/loops/mutual_recursion.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/loops/unbounded_counter.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/safety/anonymous_misuse.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/safety/negation_order.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/safety/stratification_cycles.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/safety/unbound_head_vars.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/safety/unsafe_negation.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/structures/bracket_notation.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/structures/dot_notation.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/structures/json_syntax.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/assignment_operators.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/atom_string_confusion.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/inline_aggregation.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/lowercase_vars.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/missing_periods.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/souffle_syntax.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/syntactic/wrong_comments.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/types/atom_vs_string.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/types/hallucinated_functions.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/types/int_vs_float.mg`
- [ ] `.gemini/skills/stress-tester/assets/mangle-adversarial/types/list_in_scalar.mg`
- [x] `.gemini/skills/stress-tester/assets/stress_queries.mg`

### .nerd (12)
- [ ] `.nerd/agents/GofmtExpert/gofmt_windows_line_endings.mg`
- [ ] `.nerd/browser/snapshots/083e619b-8447-476f-b218-40932e0141fc_1786230728.mg`
- [ ] `.nerd/browser/snapshots/6cf237f9-9394-4f2f-801c-898e8cfc6a7c_1786230231.mg`
- [ ] `.nerd/browser/snapshots/95f21c42-3133-4d0c-ac21-a589ea64cffc_1786506804.mg`
- [ ] `.nerd/mangle/extensions.mg`
- [ ] `.nerd/mangle/learned.mg`
- [ ] `.nerd/mangle/learned_taxonomy.mg`
- [ ] `.nerd/mangle/policy_overrides.mg`
- [ ] `.nerd/mangle/scan.mg`
- [ ] `.nerd/marathon-supervision/phase3-control-jit_compiler.mg`
- [ ] `.nerd/northstar.mg`
- [ ] `.nerd/profile.mg`

### cmd/nerd (2)
- [ ] `cmd/nerd/.nerd/debug/debug_program_ERROR.mg`
- [ ] `cmd/nerd/chat/.nerd/debug/debug_program_ERROR.mg`

### internal/campaign/.nerd (1)
- [ ] `internal/campaign/.nerd/debug/debug_program_ERROR.mg`

### internal/context (2)
- [ ] `internal/context/.nerd/debug/debug_program_ERROR.mg`
- [x] `internal/context/working_set.mg`

### internal/core/.nerd (1)
- [ ] `internal/core/.nerd/debug/debug_program_ERROR.mg`

### internal/core/defaults (134)
- [x] `internal/core/defaults/benchmarks.mg`
- [x] `internal/core/defaults/build_topology.mg`
- [x] `internal/core/defaults/campaign_rules.mg`
- [x] `internal/core/defaults/chaos.mg`
- [x] `internal/core/defaults/doc_taxonomy.mg`
- [x] `internal/core/defaults/go_safety.mg`
- [x] `internal/core/defaults/inference.mg`
- [x] `internal/core/defaults/jit_compiler.mg`
- [x] `internal/core/defaults/learned.mg`
- [x] `internal/core/defaults/policy/activation.mg`
- [x] `internal/core/defaults/policy/autopoiesis.mg`
- [x] `internal/core/defaults/policy/bridge.mg`
- [x] `internal/core/defaults/policy/browser.mg`
- [x] `internal/core/defaults/policy/browser_honeypot.mg`
- [x] `internal/core/defaults/policy/campaign_autopoiesis.mg`
- [x] `internal/core/defaults/policy/campaign_context.mg`
- [x] `internal/core/defaults/policy/campaign_core.mg`
- [x] `internal/core/defaults/policy/campaign_phases.mg`
- [x] `internal/core/defaults/policy/campaign_planning.mg`
- [x] `internal/core/defaults/policy/campaign_tasks.mg`
- [x] `internal/core/defaults/policy/capabilities.mg`
- [x] `internal/core/defaults/policy/clarification.mg`
- [x] `internal/core/defaults/policy/codedom_continuation.mg`
- [x] `internal/core/defaults/policy/codedom_core.mg`
- [x] `internal/core/defaults/policy/codedom_edit.mg`
- [x] `internal/core/defaults/policy/codedom_safety.mg`
- [x] `internal/core/defaults/policy/coder_build.mg`
- [x] `internal/core/defaults/policy/coder_campaign.mg`
- [x] `internal/core/defaults/policy/coder_classification.mg`
- [x] `internal/core/defaults/policy/coder_context.mg`
- [x] `internal/core/defaults/policy/coder_diagnostics.mg`
- [x] `internal/core/defaults/policy/coder_impact.mg`
- [x] `internal/core/defaults/policy/coder_language.mg`
- [x] `internal/core/defaults/policy/coder_learning.mg`
- [x] `internal/core/defaults/policy/coder_observability.mg`
- [x] `internal/core/defaults/policy/coder_patterns.mg`
- [x] `internal/core/defaults/policy/coder_quality.mg`
- [x] `internal/core/defaults/policy/coder_safety.mg`
- [x] `internal/core/defaults/policy/coder_tdd.mg`
- [x] `internal/core/defaults/policy/coder_workflow.mg`
- [x] `internal/core/defaults/policy/commit_gate.mg`
- [x] `internal/core/defaults/policy/constitution.mg`
- [x] `internal/core/defaults/policy/context_compilation.mg`
- [x] `internal/core/defaults/policy/data_flow.mg`
- [x] `internal/core/defaults/policy/delegation.mg`
- [x] `internal/core/defaults/policy/dreamer.mg`
- [x] `internal/core/defaults/policy/git_safety.mg`
- [x] `internal/core/defaults/policy/impact.mg`
- [x] `internal/core/defaults/policy/intelligence.mg`
- [x] `internal/core/defaults/policy/intent_routing_rules.mg`
- [x] `internal/core/defaults/policy/jit_config.mg`
- [x] `internal/core/defaults/policy/jit_logic.mg`
- [x] `internal/core/defaults/policy/jit_selection.mg`
- [x] `internal/core/defaults/policy/knowledge.mg`
- [x] `internal/core/defaults/policy/learning.mg`
- [x] `internal/core/defaults/policy/perception_routing.mg`
- [x] `internal/core/defaults/policy/policy_mcp.mg`
- [x] `internal/core/defaults/policy/prioritization.mg`
- [x] `internal/core/defaults/policy/projectdoc.mg`
- [x] `internal/core/defaults/policy/prompt_context.mg`
- [x] `internal/core/defaults/policy/prompt_northstar.mg`
- [x] `internal/core/defaults/policy/regression_battery.mg`
- [x] `internal/core/defaults/policy/routing_arbitration.mg`
- [x] `internal/core/defaults/policy/schemas_perception_latency.mg`
- [x] `internal/core/defaults/policy/shadow_mode.mg`
- [x] `internal/core/defaults/policy/shards.mg`
- [x] `internal/core/defaults/policy/stage_context.mg`
- [x] `internal/core/defaults/policy/strategy.mg`
- [x] `internal/core/defaults/policy/system_autopoiesis.mg`
- [x] `internal/core/defaults/policy/system_config.mg`
- [x] `internal/core/defaults/policy/system_core.mg`
- [x] `internal/core/defaults/policy/system_ooda.mg`
- [x] `internal/core/defaults/policy/system_routing.mg`
- [x] `internal/core/defaults/policy/system_session.mg`
- [x] `internal/core/defaults/policy/system_shards.mg`
- [x] `internal/core/defaults/policy/system_world.mg`
- [x] `internal/core/defaults/policy/task_stage.mg`
- [x] `internal/core/defaults/policy/taxonomy_inference.mg`
- [x] `internal/core/defaults/policy/taxonomy_qualifiers.mg`
- [x] `internal/core/defaults/policy/tdd_logic.mg`
- [x] `internal/core/defaults/policy/tdd_loop.mg`
- [x] `internal/core/defaults/policy/test_impact.mg`
- [x] `internal/core/defaults/policy/tool_routing.mg`
- [x] `internal/core/defaults/policy/trace_logic.mg`
- [x] `internal/core/defaults/policy/validation.mg`
- [x] `internal/core/defaults/policy/verification.mg`
- [x] `internal/core/defaults/reviewer.mg`
- [x] `internal/core/defaults/schema/intent_campaign.mg`
- [x] `internal/core/defaults/schema/intent_code_mutations.mg`
- [x] `internal/core/defaults/schema/intent_code_review.mg`
- [x] `internal/core/defaults/schema/intent_conversational.mg`
- [x] `internal/core/defaults/schema/intent_index.mg`
- [x] `internal/core/defaults/schema/intent_instructions.mg`
- [x] `internal/core/defaults/schema/intent_multi_step.mg`
- [x] `internal/core/defaults/schema/intent_mutations.mg`
- [x] `internal/core/defaults/schema/intent_operations.mg`
- [x] `internal/core/defaults/schema/intent_qualifiers.mg`
- [x] `internal/core/defaults/schema/intent_queries.mg`
- [x] `internal/core/defaults/schema/intent_routing.mg`
- [x] `internal/core/defaults/schema/intent_stats.mg`
- [x] `internal/core/defaults/schema/intent_system.mg`
- [x] `internal/core/defaults/schema/intent_testing.mg`
- [x] `internal/core/defaults/schema/prompts.mg`
- [x] `internal/core/defaults/schemas.mg`
- [x] `internal/core/defaults/schemas_analysis.mg`
- [x] `internal/core/defaults/schemas_browser.mg`
- [x] `internal/core/defaults/schemas_campaign.mg`
- [x] `internal/core/defaults/schemas_codedom.mg`
- [x] `internal/core/defaults/schemas_codedom_polyglot.mg`
- [x] `internal/core/defaults/schemas_coder.mg`
- [x] `internal/core/defaults/schemas_context.mg`
- [x] `internal/core/defaults/schemas_dreamer.mg`
- [x] `internal/core/defaults/schemas_execution.mg`
- [x] `internal/core/defaults/schemas_intelligence.mg`
- [x] `internal/core/defaults/schemas_intent.mg`
- [x] `internal/core/defaults/schemas_knowledge.mg`
- [x] `internal/core/defaults/schemas_learning.mg`
- [x] `internal/core/defaults/schemas_mcp.mg`
- [x] `internal/core/defaults/schemas_memory.mg`
- [x] `internal/core/defaults/schemas_misc.mg`
- [x] `internal/core/defaults/schemas_project.mg`
- [x] `internal/core/defaults/schemas_projectdoc.mg`
- [x] `internal/core/defaults/schemas_prompts.mg`
- [x] `internal/core/defaults/schemas_reviewer.mg`
- [x] `internal/core/defaults/schemas_safety.mg`
- [x] `internal/core/defaults/schemas_shards.mg`
- [x] `internal/core/defaults/schemas_state.mg`
- [x] `internal/core/defaults/schemas_testing.mg`
- [x] `internal/core/defaults/schemas_tools.mg`
- [x] `internal/core/defaults/schemas_world.mg`
- [x] `internal/core/defaults/selection_policy.mg`
- [x] `internal/core/defaults/taxonomy.mg`
- [x] `internal/core/defaults/tester.mg`
- [x] `internal/core/defaults/topology_planner.mg`

### internal/init (1)
- [ ] `internal/init/.nerd/debug/debug_program_ERROR.mg`

### internal/mcp (1)
- [ ] `internal/mcp/.nerd/debug/debug_program_ERROR.mg`

### internal/northstar (1)
- [ ] `internal/northstar/.nerd/debug/debug_program_ERROR.mg`

### internal/persist (1)
- [ ] `internal/persist/snapshot/.nerd/debug/debug_program_ERROR.mg`

### internal/retrieval (1)
- [ ] `internal/retrieval/.nerd/debug/debug_program_ERROR.mg`

### internal/session (1)
- [ ] `internal/session/.nerd/debug/debug_program_ERROR.mg`

### internal/shards/system (1)
- [ ] `internal/shards/system/.nerd/debug/debug_program_ERROR.mg`

### internal/system (1)
- [ ] `internal/system/.nerd/debug/debug_program_ERROR.mg`

### internal/testing (1)
- [ ] `internal/testing/context_harness/.nerd/debug/debug_program_ERROR.mg`

### internal/world (1)
- [ ] `internal/world/.nerd/debug/debug_program_ERROR.mg`

### test_init (3)
- [ ] `test_init/.nerd/mangle/extensions.mg`
- [ ] `test_init/.nerd/mangle/policy_overrides.mg`
- [ ] `test_init/.nerd/profile.mg`

**Progress: 18/2632**