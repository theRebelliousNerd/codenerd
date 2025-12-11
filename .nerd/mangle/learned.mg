
# Autopoiesis-learned rule (added 2025-12-09 15:38:31)
permitted(Action) :- Action = "system_start".

# Autopoiesis-learned rule (added 2025-12-10 10:35:56)
system_shard_state(/boot,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 11:45:05)
entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 16:26:54)
active_strategy(/system_start) :- system_heartbeat(_,/cold).


# Autopoiesis-learned rule (added 2025-12-10 16:29:52)
ooda_phase(/observe) :- system_startup(_,_).


# Autopoiesis-learned rule (added 2025-12-10 16:53:11)
system_startup(/start,/init).


# Autopoiesis-learned rule (added 2025-12-10 17:08:30)
next_action(/system_start) :- session_state(_,/initializing,_).


# Autopoiesis-learned rule (added 2025-12-10 17:10:39)
strategy_activated(/initialization,/system_start).


# Autopoiesis-learned rule (added 2025-12-10 17:11:58)
next_action(/system_start) :- session_turn(_,_,/init,_).


# Autopoiesis-learned rule (added 2025-12-10 17:15:53)
active_strategy(/system_start) :- session_state(_,_,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 17:33:05)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 17:33:54)
system_startup(/initialized,/core_services) :- !system_shard_healthy(/core_services).


# Autopoiesis-learned rule (added 2025-12-10 17:43:14)
next_action(/system_start) :- dream_state(/idle,_).


# Autopoiesis-learned rule (added 2025-12-10 17:44:10)
next_action(/initialize) :- system_heartbeat(/boot,_).


# Autopoiesis-learned rule (added 2025-12-10 17:46:49)
next_action(/initialize) :- session_state(_,/booting,_).


# Autopoiesis-learned rule (added 2025-12-10 17:47:37)
next_action(/system_start) :- context_atom(/boot).


# Autopoiesis-learned rule (added 2025-12-10 17:55:45)
next_action(/initialize) :- system_shard(_,/boot).


# Autopoiesis-learned rule (added 2025-12-10 18:13:55)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:18:12)
next_action(/initialize) :- current_intent(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:19:56)
action_permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 19:47:55)
next_action(/initialize) :- session_planner_status(_,_,/idle,_,_,_).


# Autopoiesis-learned rule (added 2025-12-10 19:49:40)
next_action(/initialize) :- system_startup(_,_).


# Autopoiesis-learned rule (added 2025-12-10 19:50:15)
final_action(/system_start) :- session_state(_,/initializing,_).


# Autopoiesis-learned rule (added 2025-12-10 19:57:39)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 19:59:27)
next_action(/system_start) :- entry_point(/init).


# Autopoiesis-learned rule (added 2025-12-10 20:15:45)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 20:25:42)
next_action(/initialize) :- current_phase(/start).


# Autopoiesis-learned rule (added 2025-12-10 20:35:38)
next_action(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 20:36:01)
next_action(/system_start) :- session_state(_,_,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 21:25:16)
next_action(/system_start) :- system_shard_unhealthy(/true).


# Autopoiesis-learned rule (added 2025-12-10 22:47:53)
next_action(/initialize) :- system_event_handled(/start,_,_).


# Autopoiesis-learned rule (added 2025-12-10 22:49:29)
next_action(/initialize) :- system_event_handled(/start,_,_).


# Autopoiesis-learned rule (added 2025-12-10 22:50:04)
permitted(/system_start) :- current_task(/initialization).


# Autopoiesis-learned rule (added 2025-12-10 22:50:10)
system_startup(/ready,/initialized) :- has_current_intent().


# Autopoiesis-learned rule (added 2025-12-10 22:50:44)
next_action(/idle) :- coder_state(/idle).


# Autopoiesis-learned rule (added 2025-12-10 22:54:04)
system_startup(/initializing,/cold_start).


# Autopoiesis-learned rule (added 2025-12-10 22:57:13)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 22:57:39)
next_action(/system_start) :- entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 23:23:29)
build_system(/initialized).


# Autopoiesis-learned rule (added 2025-12-10 23:29:30)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 23:33:36)
next_action(/system_start) :- session_planner_status(_,_,_,_,_,/idle).


# Autopoiesis-learned rule (added 2025-12-10 23:34:39)
next_action(/system_start) :- system_startup(/ready,/system_start).


# Autopoiesis-learned rule (added 2025-12-10 23:36:11)
# SELF-HEALED: rule uses undefined predicates: [suppressed] (available: [is_test_file execution_sandbox tool_language context_pressure_level final_action tool_ready security_violation shard_type northstar_addresses atom_excluded ooda_timeout computed_style tool_generation_hint state_unchanged_count task_conflict_active element_visibility has_stale_context strategic_warning mutation_rejected ancestor campaign_prompt_policy atom_loses_conflict guards_block shard_profile task_artifact pending_intent breaking_change_risk api_edit_warning required_retry trace_task_type has_skeleton_category tool_description temporary_override tool_quality_acceptable style_rule executive_error panic_state active_hypothesis appeal_pattern_detected eligible_task tool_generated uses learned_constraint appeal_history doc_reference execution_resource_usage unchecked_error learned_fact retry_count action_mapping tool_registered action_routed build_tag query_trace_stats trace_pattern task_failure_reason code_implements review_summary current_task high_risk is_excluded project_framework has_active_refinement tool_advisory_block northstar_defined clarification_needed ooda_phase verification_attempt shard_switch_suggestion is_mutation_approved phase_stuck refinement_count repeated_violation_pattern generated_code dream_preference git_branch atom_context_boost route_action quality_signal last_action_time element_body mock_file task_priority shard_output safety_violation churn_rate instruction_contains permission_denied quality_violation_evidence priority_boost campaign_milestone northstar_problem atom_candidate git_history context_profile system_shard_state safety_check safe_projection is_core_file lines_inserted file_not_found unaddressed_high_risk error_checked_return candidate_action architectural_pattern current_campaign code_edit_outcome api_handler_function review_rejection_rate_high atom_final_order missing_hypothesis preference_signal preference_learned simulated_effect campaign_intent_capture edit_analysis call_arg appeal_needs_review phase_context_scope campaign_blocked pending_edit system_shard file_in_scope has_suppression_unsafe_deref high_element_count_flag attr context_atom chesterton_fence_warning campaign_phase tool_binary_path missing_tool_for tool_executed blocked_learned_action_count turn_context has_incomplete_phase last_checkpoint_time rule_outcome file_hash_mismatch shard_can_handle slow_reasoning_detected tool_compiled action_pending_permission scope_open_failed learning_from_traces tool_generation_failed review_suspect northstar_requirement conflict_loser unhandled_case_count encoding_issue shard_family atom_dependency atom_content error_ignored patch_diff phase_objective symbol_reachable unhandled_cases should_auto_continue appeal_denied tool_exec_failed file_truncated undo_available shard_error shard_model_config test_coverage network_permitted above plan_validation_issue tool_generation_permitted test_summary context_priority_file has_learned campaign_goal intent_ready_for_executive lines_edited requirement_task_link context_budget_sufficient capability_addresses_need compilation_error file_topology tool_in_list coder_safe_to_write execution_error northstar_pain_point nil_returns same_scope bug_history known_cause doc_metadata tool_safety_verified generation_state test_result shard_selected specialist_knowledge_updated mandatory_atom replan_trigger has_block_commit has_incomplete_hard_dep shard_executed trace_quality recent_commit unguarded_use function_scope diagnostic action_violates visible_text goal_requires build_system file_line_count perception_error successful_edit tool_exec_success function_params build_state checks_passed file_in_project tool_execution context_stale northstar_mitigation coder_state routing_succeeded investigation_result tool_priority_rank has_current_intent critical_capability orphan_requirement requirement_coverage file_package exec_request context_injection_effective northstar_supports final_atom prioritized_hypothesis effective_prompt_atom new_fact appeal_granted execution_output last_shard_execution multi_shard_review context_budget project_architecture context_priority registered_tool tool_execution_count ready_for_routing checkpoint_due signature_change_detected shard_performance unsafe_to_refactor phase_category goal_topic source_requirement tool_needs_refinement refinement_effective world_model_stale auto_apply_rule dom_node safe_to_commit has_earlier_phase has_passed_checkpoint next_agenda_item proposed_rule code_contains has_quality_violation has_eligible_phase execution_killed specialist_recommended final_injectable northstar_serves capability_is_linked moonshot_capability atom_matches_context phase_context_atom failing_test prompt_ready atom_category guard_dominates lsp_reference task_attempt path_contains task_completed suppression safe_access signed_approval active_refinement focus_needs_resolution has_external_callers review_metrics shard_performing_well tool_intent_relevance has_high_priority_context dependency_link active_strategy phase_dependency northstar_vision immediate_capability max_continuation_steps agenda_item_escalate action_ready_for_routing symbol_verified_exists risk_addressing_requirement delegate_task doc_conflict function_nesting element_stale impact_caller context_window_state previous_coder_state execution_blocked execution_failure dream_state must_have_requirement atom_dependency_satisfied assigns layer_distance system_shard_unhealthy learning_signal function_in_scope escalation_required knowledge_link task_conflict violation_type_count_high learned_preference security_violation_type refinement_state execution_working_dir recent_change_by_other anti_pattern tool_exists has_active_generation method_in_scope suggest_update_mocks requires_contract_check review_finding task_dependency tool_capability tdd_retry_count priority_higher world_model_error current_ooda_phase has_recent_shard_output shard_selection_confidence query_activations campaign_progress applied_rule multi_review_finding compile_query in_modified_code active_finding type_priority campaign_progress_over_50 plan_progress scope_closed query_knowledge_graph activation honeypot_detected tool_success_relevance northstar_mission code_defines specialist_outperforms phase_checkpoint agenda_item_ready rule_applied has_next_action dream_tool_need injectable_context has_identity_atom has_bug_history has_temporary_override source_document file_content has_in_progress_phase execution_completed escalation_needed action_type verification_result low_quality_trace current_intent false_positive_pattern has_mitigation phase_precedence detected_language executive_trace scope_operation verification_blocked has_compilation_error similar_content constraint_violation projection_violation tool_quality_good task_blocked context_budget_constrained has_injectable_context northstar_risk directory tool_generation_blocked item_retry_count corrective_query corrective_action_effective suppressed_rule safe_action edit_unsafe pending_review active_goal has_capability has_running_tasks tool_quality_poor execution_nonzero tool_allowlist needs_scope_refresh parent tool_lifecycle northstar_need compile_budget prohibited_atom has_error_check_at session_turn execution_io tool_base_relevance specialist_knowledge atom_selected unhandled_case_count_computed action_details research_complete architectural_violation dangerous_capability interface_definition permission_check_result propose_new_rule relevant_to_intent edit_operation system_heartbeat current_shard_type geometry target_is_large rejection_count phase_synonym intent_processed has_specialist_knowledge is_mandatory has_guard_at mutation_approved knowledge_atom campaign_task has_projection_violation focus_clarification has_incomplete_phase_task propose_safety_rule lines_deleted proven_safe_edit recent_shard_context orphan_capability atom_conflict campaign_complete block_all_actions tool_allowed relevant_context_file continuation_step excessive_appeal_denials campaign_learning issue_occurrence_count type_definition_file raw_finding compilation_valid campaign_shard learning_pattern_detected scope_refresh_failed atom_requires interface_impl tool_priority_score tool_combined_score has_campaign_constraints learned_exemplar diagnostic_count system_shard_healthy method_of shard_success northstar_capability guards_return action_blocked vector_recall lsp_definition capability_similar_to version_quality plan_task final_system_prompt reviewer_needs_validation ambiguity_flag pending_clarification lsp_hover security_anomaly file_read file_written requires_campaign atom_meets_threshold project_profile user_preference shard_result phase_tool_permitted corrective_context active_prompt_customization compile_context modified_interface has_next_campaign_task pending_permission_check tool_domain_relevance active_shard focus_resolution block_commit phase_estimate tool_source_ready is_binary_file tdd_state routing_failed query_traces project_language current_user next_campaign_task tool_available task_failure_count testable_language execution_success first_attempt_success doc_tag processed_intent routing_error active_file relevant_tool atom_exclusion_group verification_summary pending_subtask_count_computed block_refactor build_phase_type has_higher_priority_item element_count_high user_rejected_finding context_overflow semantic_suggested_verb doc_layer editable context_to_inject has_unsatisfied_hard_dep candidate_atom promote_to_long_term doc_exists_for has_tool_for_action atom has_known_cause has_pending_tasks same_package scope_refreshed shard_findings_available reasoning_trace is_supported_req dangerous_action routing_table cgo_code dream_block is_guarded has_continuation_block has_active_override replan_needed is_public_api rule_proposal_pending session_checkpoint projected_fact has_earlier_task failed_campaign_task projected_action user_input_string prompt_exemplar impact_graph appeal_available active_layer phase_blocked tool_not_found tool_last_execution has_pending_subtask derived_rule escalate_to_user tool_recency_relevance missing_skeleton_category guarded_use has_guard instruction_contains_write phase_success_pattern pending_action injectable_context_priority nil_deref_risk permitted tool_capabilities campaign phase_eligible tool_learning is_interface_file strategy_activated element_signature task_inference anomaly_investigated quality_score long_term_capability category_order atom_tag suggest_appeal user_accepted_finding prompt_customization has_suppression_unchecked_error learned_proposal function_metrics ooda_stalled security_finding atom_selector compile_shard has_protocol_atom modified_function related_context appeal_pending task_error tool_hash has_incomplete_dependency element_modified dream_risk_pattern has_pending_checkpoint test_type safe_interactable suspicious_gap session_planner_status failed_edit critical_path_prefix tool_trace diff_introduces_risk target_is_complex ambiguity_detected proposed_phase phase_dependency_generated explicit_tool_request large_file_warning requires_integration_test has_tool_domain entry_point task_order world_model_heartbeat atom_loses_exclusion is_suppressed unsafe_deref has_test_coverage system_invariant_violated session_state finding_count verification_succeeded is_served_persona compound_suggestion pending_test modified activate_shard relevant_context shard_context_atom error_checked_block admin_override tool_issue_pattern user_intent symbol_graph test_state agenda_item execution_started violation_count file_modified_externally element_edit_blocked recall_similar allowed_domain user_requests_appeal context_compression plan_revision code_calls shard_reasoning_pattern intent_requires_capability execution_command routing_result routing_blocked api_client_function dream_learning_confirmed has_tool_usage cross_shard_insight category_budget review_accuracy semantic_match vector_hit action_denied pending_mutation layer_priority all_phase_tasks_complete code_interactable execution_result base_prohibited next_action derivation_trace validation_error quality_violation corrective_action_taken current_time task_remediation_target active_generation action_permitted campaign_active impact_implementer symptom shadow_state requires_approval in_scope shard_struggling left_of proof_tree_node current_phase tool_domain agenda_dependency needs_corrective_action trace_insight has_priority_boost lsp_diagnostic system_event_handled suggest_use_specialist blocked_action interactable target_checkbox code_pattern element_parent safe_to_modify parse_error shard_capability_affinity shard_prompt_base tool_known_issue rule_needs_approval high_quality_trace northstar_constraint unmitigated_risk prompt_atom skeleton_category selected_atom checkpoint_needed permission_checked unserved_persona atom_conflicts active_override has_blocking_task_dep tool_usage_stats risk_is_addressed atom_priority suppression_confidence campaign_metadata test_file_for build_result code_element review_insight vector_recall_result is_error_checked interrupt_requested impacted is_tool_registered tool_refined dependent_count route_added shard_heartbeat_stale trace_error is_relevant query_session coder_block_write system_startup current_context debug_why_blocked capability_gap_detected embed_directive critical_file multi_review_participant specialist_match prompt_context_budget northstar_persona query_learned capability_available safety_warning near_term_capability continuation_blocked])
# next_action(/initialize) :- system_heartbeat(_,_), !suppressed(/system_start).

