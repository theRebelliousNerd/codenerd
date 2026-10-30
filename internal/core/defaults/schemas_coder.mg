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
# Convention: /naming, /typing, /interfaces, /errors, ... (closed vocabulary)
# Rule: free-form guidance text
Decl language_convention(Language, Convention, Rule) bound [/name, /name, /string].
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

# Quality Gates
Decl edit_needs_tests(File) bound [/string].

# Bound-negation helper. A negated literal containing an anonymous wildcard
# excludes nothing in this Mangle build (see internal/core/bound_negation_test.go);
# projecting the wildcard away makes the negation actually filter.
Decl any_edit_needs_tests(Flag) bound [/name].
Decl edit_needs_docs(File) bound [/string].
# testable_language(Language) is ALREADY DECLARED in schemas_shards.mg
# Decl testable_language(Language).

# Build & Diagnostics
# block_commit(Reason) is ALREADY DECLARED in schemas_analysis.mg
# Decl block_commit(Reason).
Decl build_healthy() bound [].
Decl has_errors() bound [].
Decl requires_immediate_fix(DiagID) bound [/string].
Decl should_address_warning(DiagID) bound [/string].
Decl can_defer_lint(DiagID) bound [/string].
Decl warning_suppressed(DiagID) bound [/string].
Decl priority_diagnostic(DiagID, Priority) bound [/string, /number].

# Context Gathering
# context_priority(File, Priority) CONFLICTS with context_priority(FactID, Priority) in schemas_memory.mg
# We will rename it to coder_context_priority(File, Priority)
Decl coder_context_priority(File, Priority) bound [/string, /number].
# include_in_context(Path) is the document-selection policy's (selection_policy.mg);
# the coder's own file inclusion is coder_include_in_context.
Decl include_in_context(File) bound [/string].
Decl coder_include_in_context(File) bound [/string].
Decl coder_exclude_from_context(File) bound [/string].
Decl coder_final_context_include(File) bound [/string].

Decl edit_is_implementation(File) bound [/string].
Decl edit_is_test(File) bound [/string].
Decl function_too_long(File, FuncName) bound [/string, /string].
Decl complexity_too_high(File, FuncName) bound [/string, /string].
Decl too_many_params(File, FuncName) bound [/string, /string].
Decl deep_nesting(File, FuncName) bound [/string, /string].
Decl recommend_extraction(File, FuncName) bound [/string, /string].
Decl recommend_simplify(File, FuncName) bound [/string, /string].
Decl recommend_param_object(File, FuncName) bound [/string, /string].

# Learning Patterns
Decl coder_rejection_pattern(Style) bound [/string].
Decl coder_error_pattern(ErrorType) bound [/string].
# promote_to_long_term(Type, Value) is ALREADY DECLARED in schemas_analysis.mg
# Decl promote_to_long_term(Type, Value).
Decl coder_success_pattern(Pattern) bound [/string].
# learning_signal(Type, Pattern) is ALREADY DECLARED in schemas_shards.mg
# Decl learning_signal(Type, Pattern).
Decl has_rejection(Pattern) bound [/string].

# Campaign Integration
Decl campaign_coder_focus(Objective) bound [/string].
Decl campaign_requires_tests() bound [].
Decl campaign_requires_build() bound [].
Decl target_error_count(Count) bound [/number].
Decl target_warning_count(Count) bound [/number].
# Specialized Patterns
Decl api_endpoint_pattern(File) bound [/string].
Decl api_needs_validation(File) bound [/string].
Decl api_needs_error_handling(File) bound [/string].
Decl database_operation_pattern(File) bound [/string].
Decl db_needs_pooling(File) bound [/string].
Decl concurrency_pattern(File) bound [/string].
Decl needs_synchronization(File) bound [/string].
Decl needs_context_propagation(File) bound [/string].
