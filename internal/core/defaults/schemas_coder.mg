# Coder Shard Schemas (EDB & IDB Declarations)
# Version: 2.0.0
# Description: Declarations for the Coder Shard logic.

# =============================================================================
# SECTION 1: EDB DECLARATIONS (Facts)
# =============================================================================

# Coder Task
Decl coder_task(ID, Action, Target, Instruction).
Decl coder_target(File).

# File System & Environment
Decl file_extension(FilePath, Extension).
Decl workspace_root(Root).
Decl path_in_workspace(Path).
Decl is_generated_file(File).
Decl is_vendor_file(File).

# Autopoiesis / Learning
Decl rejection(TaskID, Category, Pattern).
Decl coder_rejection_count(Category, Pattern, Count).
Decl code_accepted(TaskID, Pattern).
Decl acceptance_count(Pattern, Count).

# =============================================================================
# SECTION 2: IDB DECLARATIONS (Rules)
# =============================================================================

# Classification
Decl coder_strategy(Strategy).
# task_complexity(Complexity) CONFLICTS with schemas_shards.mg
# We will rename it to coder_task_complexity(Complexity)
Decl coder_task_complexity(Complexity).
# detected_language(File, Language) is ALREADY DECLARED in schemas_shards.mg
# Decl detected_language(File, Language).
Decl language_convention(Language, Convention, Rule).
Decl apply_convention(Convention, Rule).
Decl requires_error_handling(File).
Decl requires_type_annotations(File).

# Impact Analysis
Decl coder_impacted(File).
Decl coder_impacted_1(File).
Decl coder_impacted_2(File).
Decl coder_impacted_3(File).
Decl high_impact_edit(File).
Decl critical_impact_edit(File).
Decl cross_package_impact(File).
Decl impact_warning(File, WarningType).

# Safety & Blocking
# coder_block_write(File, Reason) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_block_write(File, Reason).
Decl coder_block_action(Action, Reason).
# coder_safe_to_write(File) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_safe_to_write(File).
Decl has_coder_block(File).

# Quality Gates
Decl edit_needs_tests(File).
Decl edit_needs_docs(File).
# testable_language(Language) is ALREADY DECLARED in schemas_shards.mg
# Decl testable_language(Language).

# Build & Diagnostics
# block_commit(Reason) is ALREADY DECLARED in schemas_analysis.mg
# Decl block_commit(Reason).
Decl build_healthy().
Decl has_errors().
Decl requires_immediate_fix(DiagID).
Decl should_address_warning(DiagID).
Decl can_defer_lint(DiagID).
Decl warning_suppressed(DiagID).
Decl priority_diagnostic(DiagID, Priority).

# Workflow / Next Action
Decl next_coder_action(Action).

# Context Gathering
# context_priority(File, Priority) CONFLICTS with context_priority(FactID, Priority) in schemas_memory.mg
# We will rename it to coder_context_priority(File, Priority)
Decl coder_context_priority(File, Priority).
Decl include_in_context(File).
Decl exclude_from_context(File).
Decl final_context_include(File).

# TDD Integration
Decl tdd_active().
Decl tdd_red_phase().
Decl tdd_green_phase().
Decl tdd_refactor_phase().
Decl minimal_implementation_mode().
Decl refactor_mode().
Decl tdd_different_approach_needed().
Decl edit_is_implementation(File).
Decl edit_is_test(File).
Decl tdd_violation(Violation).

# Code Quality Rules
Decl go_needs_error_check(File).
Decl go_needs_context(File).
Decl go_goroutine_leak_risk(File).
Decl go_interface_too_large(File).
Decl function_too_long(File, FuncName).
Decl complexity_too_high(File, FuncName).
Decl too_many_params(File, FuncName).
Decl deep_nesting(File, FuncName).
Decl recommend_extraction(File, FuncName).
Decl recommend_simplify(File, FuncName).
Decl recommend_param_object(File, FuncName).

# Edit Helpers (populated by Go runtime or derived)
Decl edit_handles_errors(File).
Decl edit_has_context(File).
Decl edit_has_waitgroup(File).
Decl edit_has_context_cancel(File).
Decl edit_spawns_goroutine(File).
Decl edit_is_public_function(File).
Decl edit_does_io(File).
Decl edit_defines_interface(File, Name, Count).
Decl edit_contains_operation(File, Op).

# Learning Patterns
Decl coder_rejection_pattern(Style).
Decl coder_error_pattern(ErrorType).
# promote_to_long_term(Type, Value) is ALREADY DECLARED in schemas_analysis.mg
# Decl promote_to_long_term(Type, Value).
Decl coder_success_pattern(Pattern).
# learning_signal(Type, Pattern) is ALREADY DECLARED in schemas_shards.mg
# Decl learning_signal(Type, Pattern).
Decl has_rejection(Pattern).

# Campaign Integration
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

# Specialized Patterns
Decl api_endpoint_pattern(File).
Decl api_needs_validation(File).
Decl api_needs_error_handling(File).
Decl database_operation_pattern(File).
Decl db_needs_transaction(File).
Decl db_needs_pooling(File).
Decl concurrency_pattern(File).
Decl needs_synchronization(File).
Decl needs_context_propagation(File).
