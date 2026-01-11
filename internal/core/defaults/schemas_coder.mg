# Coder Shard Schema (EDB Declarations)
# Version: 1.0.0
# Extracted from coder.mg

# =============================================================================
# CODER SHARD SCHEMAS
# =============================================================================

# NOTE: Some predicates used by coder are declared in schemas.mg or schemas_shards.mg:
#   - coder_state(State)
#   - file_content(File, Content)
#   - pending_edit(File, Content)
#   - retry_count(Count)

# -----------------------------------------------------------------------------
# Core Coder Predicates
# -----------------------------------------------------------------------------

# coder_task(ID, Action, Target, Instruction)
# Action: /create, /modify, /refactor, /fix, /integrate, /document
# Target: FilePath
Decl coder_task(ID, Action, Target, Instruction).

# coder_target(File) - The current file being operated on
Decl coder_target(File).

# task_target_count(ID, Count) - Number of targets for a task
Decl task_target_count(ID, Count).

# file_extension(FilePath, Extension) - File extension extraction
Decl file_extension(FilePath, Extension).

# workspace_root(Root) - The root directory of the workspace
Decl workspace_root(Root).

# path_in_workspace(Path) - Validates path is within workspace
Decl path_in_workspace(Path).

# -----------------------------------------------------------------------------
# Autopoiesis & Learning
# -----------------------------------------------------------------------------

# rejection(TaskID, Category, Pattern)
# Category: /style, /error
Decl rejection(TaskID, Category, Pattern).

# coder_rejection_count(Category, Pattern, Count)
Decl coder_rejection_count(Category, Pattern, Count).

# code_accepted(TaskID, Pattern)
Decl code_accepted(TaskID, Pattern).

# acceptance_count(Pattern, Count)
Decl acceptance_count(Pattern, Count).

# -----------------------------------------------------------------------------
# Internal Logic Predicates (Previously Implicit or Local)
# -----------------------------------------------------------------------------

# coder_strategy(Strategy)
# Strategy: /generate, /modify, /refactor, /fix, /integrate, /document
Decl coder_strategy(Strategy).

# task_has_multiple_targets(ID)
Decl task_has_multiple_targets(ID).

# task_is_architectural(ID)
Decl task_is_architectural(ID).

# instruction_mentions_architecture(Instruction)
Decl instruction_mentions_architecture(Instruction).

# language_convention(Language, Convention, Rule)
Decl language_convention(Language, Convention, Rule).

# apply_convention(Convention, Rule)
Decl apply_convention(Convention, Rule).

# requires_error_handling(Target)
Decl requires_error_handling(Target).

# requires_type_annotations(Target)
Decl requires_type_annotations(Target).

# coder_impacted_1(X), coder_impacted_2(X), coder_impacted_3(X)
Decl coder_impacted_1(File).
Decl coder_impacted_2(File).
Decl coder_impacted_3(File).

# coder_impacted(X)
Decl coder_impacted(File).

# high_impact_edit(File)
Decl high_impact_edit(File).

# critical_impact_edit(File)
Decl critical_impact_edit(File).

# cross_package_impact(File)
Decl cross_package_impact(File).

# impact_warning(File, WarningType)
Decl impact_warning(File, WarningType).

# is_generated_file(Path)
Decl is_generated_file(Path).

# is_vendor_file(Path)
Decl is_vendor_file(Path).

# has_implementation_edit()
Decl has_implementation_edit().

# has_coder_block(File)
Decl has_coder_block(File).

# edit_needs_tests(File)
Decl edit_needs_tests(File).

# edit_needs_docs(File)
Decl edit_needs_docs(File).

# has_errors()
Decl has_errors().

# requires_immediate_fix(DiagID)
Decl requires_immediate_fix(DiagID).

# should_address_warning(DiagID)
Decl should_address_warning(DiagID).

# can_defer_lint(DiagID)
Decl can_defer_lint(DiagID).

# warning_suppressed(DiagID)
Decl warning_suppressed(DiagID).

# priority_diagnostic(DiagID, Priority)
Decl priority_diagnostic(DiagID, Priority).

# next_coder_action(Action)
Decl next_coder_action(Action).

# has_file_content(File)
Decl has_file_content(File).

# coder_context_priority(File, Priority)
Decl coder_context_priority(File, Priority).

# include_in_context(File)
Decl include_in_context(File).

# exclude_from_context(File)
Decl exclude_from_context(File).

# final_context_include(File)
Decl final_context_include(File).

# tdd_active()
Decl tdd_active().

# tdd_red_phase()
Decl tdd_red_phase().

# tdd_green_phase()
Decl tdd_green_phase().

# tdd_refactor_phase()
Decl tdd_refactor_phase().

# minimal_implementation_mode()
Decl minimal_implementation_mode().

# refactor_mode()
Decl refactor_mode().

# tdd_different_approach_needed()
Decl tdd_different_approach_needed().

# edit_is_implementation(File)
Decl edit_is_implementation(File).

# edit_is_test(File)
Decl edit_is_test(File).

# tdd_violation(Type)
Decl tdd_violation(Type).

# go_needs_error_check(File)
Decl go_needs_error_check(File).

# go_needs_context(File)
Decl go_needs_context(File).

# go_goroutine_leak_risk(File)
Decl go_goroutine_leak_risk(File).

# go_interface_too_large(File)
Decl go_interface_too_large(File).

# edit_handles_errors(File)
Decl edit_handles_errors(File).

# edit_has_context(File)
Decl edit_has_context(File).

# edit_has_waitgroup(File)
Decl edit_has_waitgroup(File).

# edit_has_context_cancel(File)
Decl edit_has_context_cancel(File).

# edit_spawns_goroutine(File)
Decl edit_spawns_goroutine(File).

# edit_is_public_function(File)
Decl edit_is_public_function(File).

# edit_does_io(File)
Decl edit_does_io(File).

# edit_defines_interface(File, Name, Count)
Decl edit_defines_interface(File, Name, Count).

# edit_contains_operation(File, Op)
Decl edit_contains_operation(File, Op).

# function_too_long(File, FuncName)
Decl function_too_long(File, FuncName).

# complexity_too_high(File, FuncName)
Decl complexity_too_high(File, FuncName).

# too_many_params(File, FuncName)
Decl too_many_params(File, FuncName).

# deep_nesting(File, FuncName)
Decl deep_nesting(File, FuncName).

# recommend_extraction(File, FuncName)
Decl recommend_extraction(File, FuncName).

# recommend_simplify(File, FuncName)
Decl recommend_simplify(File, FuncName).

# recommend_param_object(File, FuncName)
Decl recommend_param_object(File, FuncName).

# coder_rejection_pattern(Style)
Decl coder_rejection_pattern(Style).

# coder_error_pattern(ErrorType)
Decl coder_error_pattern(ErrorType).

# coder_promote_to_long_term(Type, Value)
Decl coder_promote_to_long_term(Type, Value).

# coder_success_pattern(Pattern)
Decl coder_success_pattern(Pattern).

# coder_learning_signal(Signal, Pattern)
Decl coder_learning_signal(Signal, Pattern).

# has_rejection(Pattern)
Decl has_rejection(Pattern).

# in_campaign_context()
Decl in_campaign_context().

# campaign_coder_focus(Objective)
Decl campaign_coder_focus(Objective).

# campaign_requires_tests()
Decl campaign_requires_tests().

# campaign_requires_build()
Decl campaign_requires_build().

# coder_quality_mode(Mode)
Decl coder_quality_mode(Mode).

# coder_task_completed(TaskID)
Decl coder_task_completed(TaskID).

# coder_task_failed(TaskID, Reason)
Decl coder_task_failed(TaskID, Reason).

# coder_status(State, Target, Strategy)
Decl coder_status(State, Target, Strategy).

# coder_blocked_reason(File, Reason)
Decl coder_blocked_reason(File, Reason).

# target_error_count(Count)
Decl target_error_count(Count).

# target_warning_count(Count)
Decl target_warning_count(Count).

# coder_progressing()
Decl coder_progressing().

# coder_stuck()
Decl coder_stuck().

# api_endpoint_pattern(File)
Decl api_endpoint_pattern(File).

# api_needs_validation(File)
Decl api_needs_validation(File).

# api_needs_error_handling(File)
Decl api_needs_error_handling(File).

# database_operation_pattern(File)
Decl database_operation_pattern(File).

# db_needs_transaction(File)
Decl db_needs_transaction(File).

# db_needs_pooling(File)
Decl db_needs_pooling(File).

# concurrency_pattern(File)
Decl concurrency_pattern(File).

# needs_synchronization(File)
Decl needs_synchronization(File).

# needs_context_propagation(File)
Decl needs_context_propagation(File).
