# Coder Shard Schema - Declarations
# Extracted from coder.mg
# =============================================================================

# Coder-specific predicates
Decl coder_task(ID, Action, Target, Instruction).
Decl coder_target(File).
Decl file_extension(FilePath, Extension).
Decl workspace_root(Root).
Decl path_in_workspace(Path).

# Rejection/acceptance tracking for autopoiesis
Decl rejection(TaskID, Category, Pattern).
# Note: coder_rejection_count has 3 args vs schemas.mg's rejection_count/2 for autopoiesis
Decl coder_rejection_count(Category, Pattern, Count).
Decl code_accepted(TaskID, Pattern).
Decl acceptance_count(Pattern, Count).

# Strategy & Complexity
Decl coder_strategy(Strategy).
Decl task_complexity(Complexity).
Decl task_has_multiple_targets(ID).
Decl task_is_architectural(ID).
Decl instruction_mentions_architecture(Instruction).
Decl instruction_contains(Instruction, Term).

# Language Detection
Decl detected_language(File, Language).
Decl language_convention(Language, Convention, Rule).
Decl apply_convention(Convention, Rule).
Decl requires_error_handling(Target).
Decl requires_type_annotations(Target).

# Impact Analysis
Decl coder_impacted_1(File).
Decl coder_impacted_2(File).
Decl coder_impacted_3(File).
Decl coder_impacted(File).
Decl high_impact_edit(File).
Decl critical_impact_edit(File).
Decl cross_package_impact(File).
Decl impact_warning(File, WarningType).

# Safety & Blocking
Decl coder_block_write(File, Reason).
Decl coder_block_action(Action, Reason).
Decl has_implementation_edit().
Decl is_generated_file(Path).
Decl is_vendor_file(Path).
Decl has_coder_block(File).
Decl coder_safe_to_write(File).
Decl edit_needs_tests(File).
Decl edit_needs_docs(File).
Decl testable_language(Language).
Decl is_test_file(File).
Decl is_public_api(File).
Decl doc_exists_for(File).
Decl path_contains(Path, Substring).

# Build State & Diagnostics
Decl block_commit(Reason).
Decl build_healthy().
Decl has_errors().
Decl requires_immediate_fix(DiagID).
Decl should_address_warning(DiagID).
Decl can_defer_lint(DiagID).
Decl warning_suppressed(DiagID).
Decl priority_diagnostic(DiagID, Priority).

# Next Action
Decl has_file_content(File).
Decl next_coder_action(Action).

# Context
Decl context_priority(File, Priority).
Decl include_in_context(File).
Decl exclude_from_context(File).
Decl final_context_include(File).
Decl test_file_for(TestFile, TargetFile).
Decl same_package(File1, File2).
Decl type_definition_file(File).
Decl file_in_project(File).
Decl is_interface_file(File).
Decl is_binary_file(File).
Decl file_package(File, Package).

# TDD
Decl tdd_active().
Decl tdd_red_phase().
Decl tdd_green_phase().
Decl tdd_refactor_phase().
Decl minimal_implementation_mode().
Decl refactor_mode().
Decl tdd_different_approach_needed().
Decl edit_is_implementation(File).
Decl edit_is_test(File).
Decl tdd_violation(ViolationType).

# Quality
Decl go_needs_error_check(File).
Decl go_needs_context(File).
Decl go_goroutine_leak_risk(File).
Decl go_interface_too_large(File).
Decl edit_handles_errors(File).
Decl edit_has_context(File).
Decl edit_has_waitgroup(File).
Decl edit_has_context_cancel(File).
Decl edit_spawns_goroutine(File).
Decl edit_is_public_function(File).
Decl edit_does_io(File).
Decl edit_defines_interface(File, Name, Count).
Decl edit_contains_operation(File, Op).
Decl interface_definition(File, Name, Count).
Decl edit_operation(File, Op).
Decl edit_analysis(File, Aspect).
Decl function_too_long(File, FuncName).
Decl complexity_too_high(File, FuncName).
Decl too_many_params(File, FuncName).
Decl deep_nesting(File, FuncName).
Decl recommend_extraction(File, FuncName).
Decl recommend_simplify(File, FuncName).
Decl recommend_param_object(File, FuncName).
Decl function_metrics(File, FuncName, Lines, Complexity).
Decl function_params(File, FuncName, Count).
Decl function_nesting(File, FuncName, Depth).

# Autopoiesis
Decl coder_rejection_pattern(Pattern).
Decl coder_error_pattern(ErrorType).
Decl promote_to_long_term(Type, Pattern).
Decl coder_success_pattern(Pattern).
Decl learning_signal(Type, Pattern).
Decl has_rejection(Pattern).

# Campaign
Decl in_campaign_context().
Decl campaign_coder_focus(Objective).
Decl campaign_requires_tests().
Decl campaign_requires_build().
Decl coder_quality_mode(Mode).
Decl coder_task_completed(TaskID).
Decl coder_task_failed(TaskID, Reason).

# Observability
Decl coder_status(State, Target, Strategy).
Decl coder_blocked_reason(File, Reason).
Decl target_error_count(Count).
Decl target_warning_count(Count).
Decl coder_progressing().
Decl coder_stuck().
Decl diagnostic_count(File, Type, Count).
Decl previous_coder_state(State).
Decl state_unchanged_count(Count).

# Patterns
Decl api_endpoint_pattern(File).
Decl api_needs_validation(File).
Decl api_needs_error_handling(File).
Decl database_operation_pattern(File).
Decl db_needs_transaction(File).
Decl db_needs_pooling(File).
Decl instruction_contains_write(File).
Decl concurrency_pattern(File).
Decl needs_synchronization(File).
Decl needs_context_propagation(File).
