# Coder Shard Schemas (EDB & IDB Declarations)
# Version: 2.0.0
# Description: Declarations for the Coder Shard logic.

# =============================================================================
# SECTION 1: EDB DECLARATIONS (Facts)
# =============================================================================

# File System & Environment
Decl file_extension(FilePath, Extension) bound [/string, /string].
Decl workspace_root(Root) bound [/string].
Decl path_in_workspace(Path) bound [/string].
# Autopoiesis / Learning
Decl rejection(TaskID, Category, Pattern) bound [/string, /name, /string].
Decl coder_rejection_count(Category, Pattern, Count) bound [/name, /string, /number].
Decl code_accepted(TaskID, Pattern) bound [/string, /string].
Decl acceptance_count(Pattern, Count) bound [/string, /number].

# =============================================================================
# SECTION 2: IDB DECLARATIONS (Rules)
# =============================================================================

# Convention: /naming, /typing, /interfaces, /errors, ... (closed vocabulary)
# Rule: free-form guidance text
Decl language_convention(Language, Convention, Rule) bound [/name, /name, /string].
# Impact Analysis
Decl coder_impacted(File) bound [/string].
Decl coder_impacted_1(File) bound [/string].
Decl coder_impacted_2(File) bound [/string].
Decl coder_impacted_3(File) bound [/string].
Decl cross_package_impact(File) bound [/string].
Decl impact_warning(File, WarningType) bound [/string, /name].

# Safety & Blocking
# coder_block_write(File, Reason) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_block_write(File, Reason).
Decl coder_block_action(Action, Reason) bound [/name, /string].
# coder_safe_to_write(File) is ALREADY DECLARED in schemas_shards.mg
# Decl coder_safe_to_write(File).
Decl has_coder_block(File) bound [/string].

# Bound-negation helper. A negated literal containing an anonymous wildcard
# excludes nothing in this Mangle build (see internal/core/bound_negation_test.go);
# projecting the wildcard away makes the negation actually filter.
Decl any_edit_needs_tests(Flag) bound [/name].
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

# Workflow / Next Action
Decl next_coder_action(Action) bound [/name].

Decl include_in_context(File) bound [/string].
Decl exclude_from_context(File) bound [/string].
Decl final_context_include(File) bound [/string].

# TDD Integration
Decl tdd_active() bound [].
Decl tdd_red_phase() bound [].
Decl tdd_green_phase() bound [].
Decl tdd_refactor_phase() bound [].
Decl minimal_implementation_mode() bound [].
Decl refactor_mode() bound [].
Decl edit_is_implementation(File) bound [/string].
Decl edit_is_test(File) bound [/string].
# Violation: /red_phase_impl, /green_phase_test (closed vocabulary, see coder_tdd.mg)
Decl tdd_violation(Violation) bound [/name].

Decl function_too_long(File, FuncName) bound [/string, /string].
Decl complexity_too_high(File, FuncName) bound [/string, /string].
Decl too_many_params(File, FuncName) bound [/string, /string].
Decl deep_nesting(File, FuncName) bound [/string, /string].
Decl recommend_extraction(File, FuncName) bound [/string, /string].
Decl recommend_simplify(File, FuncName) bound [/string, /string].
Decl recommend_param_object(File, FuncName) bound [/string, /string].

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
Decl in_campaign_context() bound [].
Decl campaign_coder_focus(Objective) bound [/string].
Decl campaign_requires_tests() bound [].
Decl campaign_requires_build() bound [].
Decl coder_quality_mode(Mode) bound [/name].
Decl coder_blocked_reason(File, Reason) bound [/string, /string].
