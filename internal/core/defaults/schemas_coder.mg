# Coder Shard Schemas (EDB & IDB Declarations)
# Version: 2.0.0
# Description: Declarations for the Coder Shard logic.

# =============================================================================
# SECTION 1: EDB DECLARATIONS (Facts)
# =============================================================================

# Coder Task
Decl coder_task(ID, Action, Target, Instruction) bound [/string, /name, /string, /string].
Decl coder_target(File) bound [/string].

# File System & Environment
Decl file_extension(FilePath, Extension) bound [/string, /string].
Decl workspace_root(Root) bound [/string].
Decl path_in_workspace(Path) bound [/string].
Decl is_generated_file(File) bound [/string].
Decl is_vendor_file(File) bound [/string].

# Autopoiesis / Learning
Decl rejection(TaskID, Category, Pattern) bound [/string, /name, /string].
Decl coder_rejection_count(Category, Pattern, Count) bound [/name, /string, /number].
Decl code_accepted(TaskID, Pattern) bound [/string, /string].
Decl acceptance_count(Pattern, Count) bound [/string, /number].

# =============================================================================
# SECTION 2: IDB DECLARATIONS (Rules)
# =============================================================================

# Classification
Decl coder_strategy(Strategy) bound [/name].
# task_complexity(Complexity) CONFLICTS with schemas_shards.mg
# We will rename it to coder_task_complexity(Complexity)
Decl coder_task_complexity(Complexity) bound [/number].
# detected_language(File, Language) is ALREADY DECLARED in schemas_shards.mg
# Decl detected_language(File, Language).
Decl language_convention(Language, Convention, Rule) bound [/name, /string, /string].
Decl apply_convention(Convention, Rule) bound [/string, /string].
Decl requires_error_handling(File) bound [/string].
Decl requires_type_annotations(File) bound [/string].

# Impact Analysis
Decl coder_impacted(File) bound [/string].
Decl coder_impacted_1(File) bound [/string].
Decl coder_impacted_2(File) bound [/string].
Decl coder_impacted_3(File) bound [/string].
Decl high_impact_edit(File) bound [/string].
Decl critical_impact_edit(File) bound [/string].
Decl cross_package_impact(File) bound [/string].
Decl impact_warning(File, WarningType) bound [/string, /name].

# Safety & Blocking
# coder_block_write(File, Reason) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_block_write(File, Reason).
Decl coder_block_action(Action, Reason) bound [/name, /string].
# coder_safe_to_write(File) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_safe_to_write(File).
Decl has_coder_block(File) bound [/string].

# Quality Gates
Decl edit_needs_tests(File) bound [/string].
Decl edit_needs_docs(File) bound [/string].
# testable_language(Language) is ALREADY DECLARED in schemas_shards.mg
# Decl testable_language(Language).

# Build & Diagnostics
# block_commit(Reason) is ALREADY DECLARED in schemas_analysis.mg
# Decl block_commit(Reason).
Decl build_healthy().
Decl has_errors().
Decl requires_immediate_fix(DiagID) bound [/string].
Decl should_address_warning(DiagID) bound [/string].
Decl can_defer_lint(DiagID) bound [/string].
Decl warning_suppressed(DiagID) bound [/string].
Decl priority_diagnostic(DiagID, Priority) bound [/string, /number].

# Workflow / Next Action
Decl next_coder_action(Action) bound [/name].

# Context Gathering
# context_priority(File, Priority) CONFLICTS with context_priority(FactID, Priority) in schemas_memory.mg
# We will rename it to coder_context_priority(File, Priority)
Decl coder_context_priority(File, Priority) bound [/string, /number].
Decl include_in_context(File) bound [/string].
Decl exclude_from_context(File) bound [/string].
Decl final_context_include(File) bound [/string].

# TDD Integration
Decl tdd_active().
Decl tdd_red_phase().
Decl tdd_green_phase().
Decl tdd_refactor_phase().
Decl minimal_implementation_mode().
Decl refactor_mode().
Decl tdd_different_approach_needed().
Decl edit_is_implementation(File) bound [/string].
Decl edit_is_test(File) bound [/string].
Decl tdd_violation(Violation) bound [/string].

# Code Quality Rules
Decl go_needs_error_check(File) bound [/string].
Decl go_needs_context(File) bound [/string].
Decl go_goroutine_leak_risk(File) bound [/string].
Decl go_interface_too_large(File) bound [/string].
Decl function_too_long(File, FuncName) bound [/string, /string].
Decl complexity_too_high(File, FuncName) bound [/string, /string].
Decl too_many_params(File, FuncName) bound [/string, /string].
Decl deep_nesting(File, FuncName) bound [/string, /string].
Decl recommend_extraction(File, FuncName) bound [/string, /string].
Decl recommend_simplify(File, FuncName) bound [/string, /string].
Decl recommend_param_object(File, FuncName) bound [/string, /string].

# Edit Helpers (populated by Go runtime or derived)
Decl edit_handles_errors(File) bound [/string].
Decl edit_has_context(File) bound [/string].
Decl edit_has_waitgroup(File) bound [/string].
Decl edit_has_context_cancel(File) bound [/string].
Decl edit_spawns_goroutine(File) bound [/string].
Decl edit_is_public_function(File) bound [/string].
Decl edit_does_io(File) bound [/string].
Decl edit_defines_interface(File, Name, Count) bound [/string, /string, /number].
Decl edit_contains_operation(File, Op) bound [/string, /name].

# Learning Patterns
Decl coder_rejection_pattern(Style) bound [/string].
Decl coder_error_pattern(ErrorType) bound [/name].
# promote_to_long_term(Type, Value) is ALREADY DECLARED in schemas_analysis.mg
# Decl promote_to_long_term(Type, Value).
Decl coder_success_pattern(Pattern) bound [/string].
# learning_signal(Type, Pattern) is ALREADY DECLARED in schemas_shards.mg
# Decl learning_signal(Type, Pattern).
Decl has_rejection(Pattern) bound [/string].

# Campaign Integration
Decl in_campaign_context().
Decl campaign_coder_focus(Objective) bound [/string].
Decl campaign_requires_tests().
Decl campaign_requires_build().
Decl coder_quality_mode(Mode) bound [/name].
Decl coder_task_completed(TaskID) bound [/string].
Decl coder_task_failed(TaskID, Reason) bound [/string, /string].

# Observability
Decl coder_status(State, Target, Strategy) bound [/name, /string, /name].
Decl coder_blocked_reason(File, Reason) bound [/string, /string].
Decl target_error_count(Count) bound [/number].
Decl target_warning_count(Count) bound [/number].
Decl coder_progressing().
Decl coder_stuck().

# Specialized Patterns
Decl api_endpoint_pattern(File) bound [/string].
Decl api_needs_validation(File) bound [/string].
Decl api_needs_error_handling(File) bound [/string].
Decl database_operation_pattern(File) bound [/string].
Decl db_needs_transaction(File) bound [/string].
Decl db_needs_pooling(File) bound [/string].
Decl concurrency_pattern(File) bound [/string].
Decl needs_synchronization(File) bound [/string].
Decl needs_context_propagation(File) bound [/string].
