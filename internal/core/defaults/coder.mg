# Coder Shard Policy - Advanced Code Generation Logic
# Version: 2.0.0
# =============================================================================
# CODER INTELLIGENCE LAYER
# This file contains advanced code generation rules for the CoderShard.
# Loaded alongside base policy.mg.
#
# Architecture:
#   policy.mg  → Base action routing, permissions
#   coder.mg   → THIS FILE: Code generation intelligence, safety, quality
# =============================================================================

# =============================================================================
# SECTION 1: DECLARATIONS
# =============================================================================
# Core predicates used by the coder shard.

# NOTE: These predicates are declared in schemas.mg (canonical location):
#   - coder_state(State)           @ schemas.mg Section 25
#   - file_content(File, Content)  @ schemas.mg Section 25
#   - pending_edit(File, Content)  @ schemas.mg Section 25
#   - retry_count(Count)           @ schemas.mg Section 9

# Coder-specific predicates (not in schemas.mg)
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

# =============================================================================
# SECTION 2: TASK CLASSIFICATION
# =============================================================================
# Classify coding tasks to determine strategy.

# -----------------------------------------------------------------------------
# 2.1 Primary Strategy Selection
# -----------------------------------------------------------------------------

coder_strategy(/generate) :-
    coder_task(_, /create, _, _).

coder_strategy(/modify) :-
    coder_task(_, /modify, _, _).

coder_strategy(/refactor) :-
    coder_task(_, /refactor, _, _).

coder_strategy(/fix) :-
    coder_task(_, /fix, _, _).

coder_strategy(/integrate) :-
    coder_task(_, /integrate, _, _).

coder_strategy(/document) :-
    coder_task(_, /document, _, _).

# -----------------------------------------------------------------------------
# 2.2 Task Complexity Classification
# -----------------------------------------------------------------------------

# Simple task: single file, clear instruction
task_complexity(/simple) :-
    coder_task(ID, _, Target, _),
    !task_has_multiple_targets(ID),
    !task_is_architectural(ID).

# Complex task: multiple files or architectural change
task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_has_multiple_targets(ID).

task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_is_architectural(ID).

# Critical task: affects core interfaces or has many dependents
task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    is_core_file(Target).

task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    dependent_count(Target, N),
    N > 10.

# Helpers for safe negation
task_has_multiple_targets(ID) :-
    coder_task(ID, _, T1, _),
    coder_task(ID, _, T2, _),
    T1 != T2.

task_is_architectural(ID) :-
    coder_task(ID, _, Target, _),
    is_interface_file(Target).

task_is_architectural(ID) :-
    coder_task(ID, /refactor, _, Instruction),
    instruction_mentions_architecture(Instruction).

# Heuristics for architectural changes
instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "interface").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "abstraction").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "architecture").

# =============================================================================
# SECTION 3: LANGUAGE DETECTION & CONVENTIONS
# =============================================================================
# Detect language and apply language-specific rules.

# -----------------------------------------------------------------------------
# 3.1 Language Detection
# -----------------------------------------------------------------------------

detected_language(File, /go) :-
    file_extension(File, ".go").

detected_language(File, /python) :-
    file_extension(File, ".py").

detected_language(File, /typescript) :-
    file_extension(File, ".ts").

detected_language(File, /typescript) :-
    file_extension(File, ".tsx").

detected_language(File, /javascript) :-
    file_extension(File, ".js").

detected_language(File, /javascript) :-
    file_extension(File, ".jsx").

detected_language(File, /rust) :-
    file_extension(File, ".rs").

detected_language(File, /java) :-
    file_extension(File, ".java").

detected_language(File, /csharp) :-
    file_extension(File, ".cs").

detected_language(File, /ruby) :-
    file_extension(File, ".rb").

detected_language(File, /php) :-
    file_extension(File, ".php").

detected_language(File, /cpp) :-
    file_extension(File, ".cpp").

detected_language(File, /cpp) :-
    file_extension(File, ".cc").

detected_language(File, /c) :-
    file_extension(File, ".c").

detected_language(File, /kotlin) :-
    file_extension(File, ".kt").

detected_language(File, /swift) :-
    file_extension(File, ".swift").

detected_language(File, /mangle) :-
    file_extension(File, ".mg").

detected_language(File, /mangle) :-
    file_extension(File, ".gl").

detected_language(File, /sql) :-
    file_extension(File, ".sql").

detected_language(File, /yaml) :-
    file_extension(File, ".yaml").

detected_language(File, /yaml) :-
    file_extension(File, ".yml").

detected_language(File, /json) :-
    file_extension(File, ".json").

detected_language(File, /markdown) :-
    file_extension(File, ".md").

detected_language(File, /shell) :-
    file_extension(File, ".sh").

detected_language(File, /powershell) :-
    file_extension(File, ".ps1").

# -----------------------------------------------------------------------------
# 3.2 Language-Specific Conventions
# -----------------------------------------------------------------------------

# Go conventions
language_convention(/go, /error_handling, "return errors, don't panic").
language_convention(/go, /naming, "camelCase for private, PascalCase for exported").
language_convention(/go, /interfaces, "accept interfaces, return concrete types").
language_convention(/go, /context, "first parameter should be ctx context.Context").
language_convention(/go, /defer, "use defer for cleanup").
language_convention(/go, /channels, "close channels from sender side").

# Python conventions
language_convention(/python, /naming, "snake_case for functions and variables").
language_convention(/python, /docstrings, "use docstrings for public functions").
language_convention(/python, /typing, "use type hints for function signatures").
language_convention(/python, /exceptions, "use specific exception types").

# TypeScript conventions
language_convention(/typescript, /typing, "prefer explicit types over any").
language_convention(/typescript, /null, "use strict null checks").
language_convention(/typescript, /interfaces, "prefer interfaces over type aliases for objects").

# Rust conventions
language_convention(/rust, /errors, "use Result<T, E> for fallible operations").
language_convention(/rust, /ownership, "prefer borrowing over cloning").
language_convention(/rust, /lifetimes, "explicit lifetimes only when necessary").

# Java conventions
language_convention(/java, /naming, "PascalCase for classes, camelCase for methods").
language_convention(/java, /exceptions, "use checked exceptions for recoverable conditions").
language_convention(/java, /interfaces, "program to interfaces").

# -----------------------------------------------------------------------------
# 3.3 Convention Application
# -----------------------------------------------------------------------------

# Conventions to apply for current task
apply_convention(Convention, Rule) :-
    coder_task(_, _, Target, _),
    detected_language(Target, Lang),
    language_convention(Lang, Convention, Rule).

# Language requires specific patterns
requires_error_handling(Target) :-
    detected_language(Target, /go).

requires_error_handling(Target) :-
    detected_language(Target, /rust).

requires_type_annotations(Target) :-
    detected_language(Target, /typescript).

requires_type_annotations(Target) :-
    detected_language(Target, /python).

# =============================================================================
# SECTION 4: IMPACT ANALYSIS
# =============================================================================
# Analyze impact of changes before making them.

# -----------------------------------------------------------------------------
# 4.1 Bounded Impact Propagation
# -----------------------------------------------------------------------------
# "Recursive Logic Bomb" Fix: Bounded recursion depth (3 levels)

# Direct impact (1-hop)
coder_impacted_1(X) :- dependency_link(X, Y, _), modified(Y).

# Transitive impact (2-hop)
coder_impacted_2(X) :- dependency_link(X, Z, _), coder_impacted_1(Z).

# Deep impact (3-hop)
coder_impacted_3(X) :- dependency_link(X, Z, _), coder_impacted_2(Z).

# Union of all levels
coder_impacted(X) :- coder_impacted_1(X).
coder_impacted(X) :- coder_impacted_2(X).
coder_impacted(X) :- coder_impacted_3(X).

# -----------------------------------------------------------------------------
# 4.2 Impact Classification
# -----------------------------------------------------------------------------

# High impact: many dependents affected
high_impact_edit(File) :-
    pending_edit(File, _),
    dependent_count(File, N),
    N > 5.

# Critical impact: core files or interfaces
critical_impact_edit(File) :-
    pending_edit(File, _),
    is_core_file(File).

critical_impact_edit(File) :-
    pending_edit(File, _),
    is_interface_file(File).

# Cross-package impact
cross_package_impact(File) :-
    pending_edit(File, _),
    coder_impacted(Dependent),
    dependency_link(Dependent, File, _),
    file_package(File, Pkg1),
    file_package(Dependent, Pkg2),
    Pkg1 != Pkg2.

# -----------------------------------------------------------------------------
# 4.3 Impact Warnings
# -----------------------------------------------------------------------------

# Warn about high-impact edits
impact_warning(File, "high_dependent_count") :-
    high_impact_edit(File).

impact_warning(File, "critical_file") :-
    critical_impact_edit(File).

impact_warning(File, "cross_package") :-
    cross_package_impact(File).

# =============================================================================
# SECTION 5: EDIT SAFETY & BLOCKING
# =============================================================================
# Safety rules to prevent dangerous or low-quality edits.

# -----------------------------------------------------------------------------
# 5.1 Block Write Conditions
# -----------------------------------------------------------------------------

# Block if impacted files lack test coverage
coder_block_write(File, "uncovered_impact") :-
    pending_edit(File, _),
    coder_impacted(Dependent),
    dependency_link(Dependent, File, _),
    !test_coverage(Dependent).

# Block writes outside workspace
coder_block_action(/edit, "forbidden_path") :-
    pending_edit(Path, _),
    !path_in_workspace(Path).

# Block binary file modifications
coder_block_action(/edit, "binary_file") :-
    pending_edit(Path, _),
    is_binary_file(Path).

# Block edits to generated files
coder_block_action(/edit, "generated_file") :-
    pending_edit(Path, _),
    is_generated_file(Path).

# Block edits to vendor/third-party code
coder_block_action(/edit, "vendor_file") :-
    pending_edit(Path, _),
    is_vendor_file(Path).

# Helper: any pending edit is implementation
has_implementation_edit() :-
    edit_is_implementation(_).

# Block edits during active TDD red phase (tests should fail first)
coder_block_action(/edit, "tdd_red_phase") :-
    pending_edit(_, _),
    tdd_state(/red),
    !has_implementation_edit().

# Helpers
is_generated_file(Path) :-
    path_contains(Path, "generated").

is_generated_file(Path) :-
    path_contains(Path, "_gen.").

is_vendor_file(Path) :-
    path_contains(Path, "vendor/").

is_vendor_file(Path) :-
    path_contains(Path, "node_modules/").

# -----------------------------------------------------------------------------
# 5.2 Safety Check Aggregation
# -----------------------------------------------------------------------------

# Helper for safe negation: true if any block exists for file
has_coder_block(File) :-
    coder_block_write(File, _).

has_coder_block(File) :-
    coder_block_action(/edit, _),
    pending_edit(File, _).

# Safe to write check
coder_safe_to_write(File) :-
    pending_edit(File, _),
    !has_coder_block(File).

# -----------------------------------------------------------------------------
# 5.3 Edit Quality Gates
# -----------------------------------------------------------------------------

# Edit should include tests if creating new code
edit_needs_tests(File) :-
    coder_task(_, /create, File, _),
    detected_language(File, Lang),
    testable_language(Lang),
    !is_test_file(File).

# Edit should update docs if modifying public API
edit_needs_docs(File) :-
    coder_task(_, /modify, File, _),
    is_public_api(File),
    !doc_exists_for(File).

# Testable languages
testable_language(/go).
testable_language(/python).
testable_language(/typescript).
testable_language(/javascript).
testable_language(/rust).
testable_language(/java).

# =============================================================================
# SECTION 6: BUILD STATE & DIAGNOSTICS
# =============================================================================
# Track build state and diagnostics.

# -----------------------------------------------------------------------------
# 6.1 Build State
# -----------------------------------------------------------------------------

# Block commit on build errors
block_commit("build_errors") :-
    diagnostic(/error, _, _, _, _).

block_commit("build_errors") :-
    build_state(/failing).

# Build is healthy
build_healthy() :-
    build_state(/passing),
    !has_errors().

has_errors() :-
    diagnostic(/error, _, _, _, _).

# -----------------------------------------------------------------------------
# 6.2 Diagnostic Classification
# -----------------------------------------------------------------------------

# Error requires immediate fix
requires_immediate_fix(DiagID) :-
    diagnostic(/error, DiagID, _, _, _).

# Warning should be addressed
should_address_warning(DiagID) :-
    diagnostic(/warning, DiagID, _, _, _),
    !warning_suppressed(DiagID).

# Lint issues can be deferred
can_defer_lint(DiagID) :-
    diagnostic(/lint, DiagID, _, _, _).

# Helper for suppressed warnings
warning_suppressed(DiagID) :-
    suppression(DiagID, _).

# -----------------------------------------------------------------------------
# 6.3 Diagnostic Prioritization
# -----------------------------------------------------------------------------

# Highest priority: errors in current file
priority_diagnostic(DiagID, 100) :-
    diagnostic(/error, DiagID, File, _, _),
    coder_target(File).

# High priority: errors in impacted files
priority_diagnostic(DiagID, 80) :-
    diagnostic(/error, DiagID, File, _, _),
    coder_impacted(File).

# Medium priority: warnings in current file
priority_diagnostic(DiagID, 50) :-
    diagnostic(/warning, DiagID, File, _, _),
    coder_target(File).

# Low priority: lint issues
priority_diagnostic(DiagID, 20) :-
    diagnostic(/lint, DiagID, _, _, _).

# =============================================================================
# SECTION 7: NEXT ACTION DERIVATION
# =============================================================================
# Derive the next coder action based on current state.

# -----------------------------------------------------------------------------
# 7.1 State Machine Actions
# -----------------------------------------------------------------------------

# Helper: file has content loaded (for safe negation)
has_file_content(File) :-
    file_content(File, _).

# Read context if needed
next_coder_action(/read_context) :-
    coder_state(/idle),
    coder_task(_, _, Target, _),
    !has_file_content(Target).

# Generate code when context ready
next_coder_action(/generate_code) :-
    coder_state(/context_ready),
    coder_task(_, _, _, _).

# Apply edit when code generated
next_coder_action(/apply_edit) :-
    coder_state(/code_generated),
    pending_edit(_, _),
    coder_safe_to_write(_).

# Request review for high-impact edits
next_coder_action(/request_review) :-
    coder_state(/code_generated),
    pending_edit(File, _),
    high_impact_edit(File).

# Run build after edit applied
next_coder_action(/run_build) :-
    coder_state(/edit_applied).

# Run tests if build passes
next_coder_action(/run_tests) :-
    coder_state(/build_passed),
    edit_needs_tests(_).

# Complete if build passed
next_coder_action(/complete) :-
    coder_state(/build_passed),
    !edit_needs_tests(_).

next_coder_action(/complete) :-
    coder_state(/tests_passed).

# Escalate after too many retries
next_coder_action(/escalate) :-
    coder_state(/build_failed),
    retry_count(N),
    N >= 3.

# Retry on build failure with fewer retries
next_coder_action(/retry_with_diagnostics) :-
    coder_state(/build_failed),
    retry_count(N),
    N < 3.

# -----------------------------------------------------------------------------
# 7.2 Recovery Actions
# -----------------------------------------------------------------------------

# Roll back if critical edit fails
next_coder_action(/rollback) :-
    coder_state(/build_failed),
    pending_edit(File, _),
    critical_impact_edit(File),
    retry_count(N),
    N >= 2.

# Decompose complex task on failure
next_coder_action(/decompose_task) :-
    coder_state(/build_failed),
    task_complexity(/complex),
    retry_count(N),
    N >= 2.

# =============================================================================
# SECTION 8: CONTEXT GATHERING INTELLIGENCE
# =============================================================================
# Rules for intelligent context gathering.

# -----------------------------------------------------------------------------
# 8.1 Context Priority
# -----------------------------------------------------------------------------

# Current target has highest priority
context_priority(File, 100) :-
    coder_target(File).

# Direct dependencies have high priority
context_priority(File, 80) :-
    coder_target(Target),
    dependency_link(Target, File, _).

# Files that import target have medium priority
context_priority(File, 60) :-
    coder_target(Target),
    dependency_link(File, Target, _).

# Test files for target have high priority
context_priority(File, 75) :-
    coder_target(Target),
    test_file_for(File, Target).

# Interface definitions have high priority
context_priority(File, 70) :-
    is_interface_file(File),
    coder_target(Target),
    same_package(File, Target).

# -----------------------------------------------------------------------------
# 8.2 Context Selection
# -----------------------------------------------------------------------------

# Include file in context if priority is high enough
include_in_context(File) :-
    context_priority(File, P),
    file_in_project(File),
    P >= 50.

# Include related test files
include_in_context(File) :-
    coder_target(Target),
    test_file_for(File, Target).

# Include type definitions
include_in_context(File) :-
    coder_target(Target),
    type_definition_file(File),
    same_package(File, Target).

# -----------------------------------------------------------------------------
# 8.3 Context Exclusion
# -----------------------------------------------------------------------------

# Exclude generated files from context
exclude_from_context(File) :-
    is_generated_file(File).

# Exclude vendor files
exclude_from_context(File) :-
    is_vendor_file(File).

# Exclude binary files
exclude_from_context(File) :-
    is_binary_file(File).

# Final context decision
final_context_include(File) :-
    include_in_context(File),
    !exclude_from_context(File).

# =============================================================================
# SECTION 9: TDD INTEGRATION
# =============================================================================
# Rules for Test-Driven Development loop integration.

# -----------------------------------------------------------------------------
# 9.1 TDD State Awareness
# -----------------------------------------------------------------------------

# TDD is active
tdd_active() :-
    tdd_state(_).

# In red phase: tests should fail
tdd_red_phase() :-
    tdd_state(/red).

# In green phase: make tests pass
tdd_green_phase() :-
    tdd_state(/green).

# In refactor phase: improve code
tdd_refactor_phase() :-
    tdd_state(/refactor).

# -----------------------------------------------------------------------------
# 9.2 TDD-Aware Code Generation
# -----------------------------------------------------------------------------

# During green phase, focus on minimal implementation
minimal_implementation_mode() :-
    tdd_green_phase().

# During refactor phase, allow optimization
refactor_mode() :-
    tdd_refactor_phase().

# TDD retry awareness (don't repeat same fix)
tdd_different_approach_needed() :-
    tdd_state(/green),
    tdd_retry_count(N),
    N >= 2.

# -----------------------------------------------------------------------------
# 9.3 TDD Validation
# -----------------------------------------------------------------------------

# Edit is implementation (not test)
edit_is_implementation(File) :-
    pending_edit(File, _),
    !is_test_file(File).

# Edit is test code
edit_is_test(File) :-
    pending_edit(File, _),
    is_test_file(File).

# TDD violation: writing implementation in red phase
tdd_violation(/red_phase_impl) :-
    tdd_red_phase(),
    edit_is_implementation(_).

# TDD violation: writing tests in green phase
tdd_violation(/green_phase_test) :-
    tdd_green_phase(),
    edit_is_test(_).

# =============================================================================
# SECTION 10: CODE QUALITY RULES
# =============================================================================
# Rules for maintaining code quality.

# -----------------------------------------------------------------------------
# 10.1 Go-Specific Quality Rules
# -----------------------------------------------------------------------------

# Go file needs error handling check
go_needs_error_check(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_contains_operation(File, /return),
    !edit_handles_errors(File).

# Go file needs context parameter
go_needs_context(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_is_public_function(File),
    edit_does_io(File),
    !edit_has_context(File).

# Go file leaks goroutine
go_goroutine_leak_risk(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_spawns_goroutine(File),
    !edit_has_waitgroup(File),
    !edit_has_context_cancel(File).

# Go interface quality
go_interface_too_large(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_defines_interface(File, _, MethodCount),
    MethodCount > 5.

# Helpers (populated by Go runtime analysis)
edit_handles_errors(File) :- edit_analysis(File, /handles_errors).
edit_has_context(File) :- edit_analysis(File, /has_context).
edit_has_waitgroup(File) :- edit_analysis(File, /has_waitgroup).
edit_has_context_cancel(File) :- edit_analysis(File, /has_context_cancel).
edit_spawns_goroutine(File) :- edit_analysis(File, /spawns_goroutine).
edit_is_public_function(File) :- edit_analysis(File, /public_function).
edit_does_io(File) :- edit_analysis(File, /does_io).
edit_defines_interface(File, Name, Count) :- interface_definition(File, Name, Count).
edit_contains_operation(File, Op) :- edit_operation(File, Op).

# -----------------------------------------------------------------------------
# 10.2 General Quality Rules
# -----------------------------------------------------------------------------

# Function too long (> 100 lines)
function_too_long(File, FuncName) :-
    function_metrics(File, FuncName, Lines, _),
    Lines > 100.

# Cyclomatic complexity too high (> 15)
complexity_too_high(File, FuncName) :-
    function_metrics(File, FuncName, _, Complexity),
    Complexity > 15.

# Too many parameters (> 5)
too_many_params(File, FuncName) :-
    function_params(File, FuncName, ParamCount),
    ParamCount > 5.

# Deep nesting (> 4 levels)
deep_nesting(File, FuncName) :-
    function_nesting(File, FuncName, Depth),
    Depth > 4.

# -----------------------------------------------------------------------------
# 10.3 Quality Recommendations
# -----------------------------------------------------------------------------

# Recommend extraction for long functions
recommend_extraction(File, FuncName) :-
    function_too_long(File, FuncName).

# Recommend simplification for complex functions
recommend_simplify(File, FuncName) :-
    complexity_too_high(File, FuncName).

# Recommend parameter object for many params
recommend_param_object(File, FuncName) :-
    too_many_params(File, FuncName).

# =============================================================================
# SECTION 11: AUTOPOIESIS - LEARNING FROM PATTERNS
# =============================================================================
# Learn from rejections and acceptances.

# -----------------------------------------------------------------------------
# 11.1 Rejection Pattern Detection
# -----------------------------------------------------------------------------

# Track rejection patterns (style issues rejected 2+ times)
coder_rejection_pattern(Style) :-
    rejection(_, /style, Style),
    coder_rejection_count(/style, Style, N),
    N >= 2.

# Track error patterns (errors repeated 2+ times)
coder_error_pattern(ErrorType) :-
    rejection(_, /error, ErrorType),
    coder_rejection_count(/error, ErrorType, N),
    N >= 2.

# Promote style preference to long-term memory
promote_to_long_term(/style_preference, Style) :-
    coder_rejection_pattern(Style).

# Promote error avoidance to long-term memory
promote_to_long_term(/error_avoidance, ErrorType) :-
    coder_error_pattern(ErrorType).

# -----------------------------------------------------------------------------
# 11.2 Success Pattern Detection
# -----------------------------------------------------------------------------

# Track successful patterns (accepted 3+ times)
coder_success_pattern(Pattern) :-
    code_accepted(_, Pattern),
    acceptance_count(Pattern, N),
    N >= 3.

# Promote preferred patterns
promote_to_long_term(/preferred_pattern, Pattern) :-
    coder_success_pattern(Pattern).

# -----------------------------------------------------------------------------
# 11.3 Learning Signals
# -----------------------------------------------------------------------------

# Signal to avoid patterns that always fail
learning_signal(/avoid, Pattern) :-
    coder_rejection_count(_, Pattern, N),
    N >= 3,
    acceptance_count(Pattern, M),
    M < 1.

# Signal to prefer patterns that always succeed
learning_signal(/prefer, Pattern) :-
    acceptance_count(Pattern, N),
    N >= 5,
    coder_rejection_count(_, Pattern, M),
    M < 1.

# Helper for rejection check
has_rejection(Pattern) :-
    coder_rejection_count(_, Pattern, _).

# =============================================================================
# SECTION 12: CAMPAIGN INTEGRATION
# =============================================================================
# Rules for coder behavior during campaigns.

# -----------------------------------------------------------------------------
# 12.1 Campaign Context Awareness
# -----------------------------------------------------------------------------

# Coder is operating within campaign
in_campaign_context() :-
    current_campaign(_).

# Current phase objectives affect coder strategy
campaign_coder_focus(Objective) :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_objective(PhaseID, _, Objective, _).

# -----------------------------------------------------------------------------
# 12.2 Campaign Quality Requirements
# -----------------------------------------------------------------------------

# Campaign phase requires tests
campaign_requires_tests() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /tests, _, _, _).

# Campaign phase requires build pass
campaign_requires_build() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /build, _, _, _).

# Stricter quality during campaigns
coder_quality_mode(/strict) :-
    in_campaign_context().

coder_quality_mode(/normal) :-
    !in_campaign_context().

# -----------------------------------------------------------------------------
# 12.3 Campaign Progress Reporting
# -----------------------------------------------------------------------------

# Report coder completion to campaign
coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/build_passed).

coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/tests_passed).

# Report coder failure to campaign
coder_task_failed(TaskID, Reason) :-
    coder_task(TaskID, _, _, _),
    coder_state(/build_failed),
    retry_count(N),
    N >= 3,
    Reason = "max_retries_exceeded".

# =============================================================================
# SECTION 13: OBSERVABILITY & DEBUGGING
# =============================================================================
# Rules for monitoring coder behavior.

# -----------------------------------------------------------------------------
# 13.1 State Queries
# -----------------------------------------------------------------------------

# Current coder status
coder_status(State, Target, Strategy) :-
    coder_state(State),
    coder_target(Target),
    coder_strategy(Strategy).

# Why is coder blocked?
coder_blocked_reason(File, Reason) :-
    coder_block_write(File, Reason).

coder_blocked_reason(File, Reason) :-
    coder_block_action(/edit, Reason),
    pending_edit(File, _).

# -----------------------------------------------------------------------------
# 13.2 Diagnostic Summary
# -----------------------------------------------------------------------------

# Count errors for current target
target_error_count(Count) :-
    coder_target(Target),
    diagnostic_count(Target, /error, Count).

# Count warnings for current target
target_warning_count(Count) :-
    coder_target(Target),
    diagnostic_count(Target, /warning, Count).

# -----------------------------------------------------------------------------
# 13.3 Performance Metrics
# -----------------------------------------------------------------------------

# Coder is making progress
coder_progressing() :-
    coder_state(S1),
    previous_coder_state(S2),
    S1 != S2.

# Coder is stuck
coder_stuck() :-
    coder_state(State),
    previous_coder_state(State),
    state_unchanged_count(N),
    N >= 3.

# =============================================================================
# SECTION 14: SPECIALIZED CODE PATTERNS
# =============================================================================
# Rules for specific code generation patterns.

# -----------------------------------------------------------------------------
# 14.1 API Endpoint Generation
# -----------------------------------------------------------------------------

# API endpoint needs specific patterns
api_endpoint_pattern(File) :-
    coder_task(_, /create, File, Instruction),
    instruction_contains(Instruction, "endpoint").

api_endpoint_pattern(File) :-
    coder_task(_, /create, File, Instruction),
    instruction_contains(Instruction, "handler").

# API requires validation
api_needs_validation(File) :-
    api_endpoint_pattern(File).

# API requires error responses
api_needs_error_handling(File) :-
    api_endpoint_pattern(File).

# -----------------------------------------------------------------------------
# 14.2 Database Operation Patterns
# -----------------------------------------------------------------------------

# Database operation pattern
database_operation_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "database").

database_operation_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "query").

# Database needs transaction handling
db_needs_transaction(File) :-
    database_operation_pattern(File),
    instruction_contains_write(File).

# Database needs connection pooling awareness
db_needs_pooling(File) :-
    database_operation_pattern(File).

# -----------------------------------------------------------------------------
# 14.3 Concurrency Patterns
# -----------------------------------------------------------------------------

# Concurrency pattern detected
concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "concurrent").

concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "parallel").

concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "goroutine").

# Concurrency needs synchronization
needs_synchronization(File) :-
    concurrency_pattern(File).

# Concurrency needs context propagation
needs_context_propagation(File) :-
    concurrency_pattern(File).

# =============================================================================
# END OF CODER RULES
# =============================================================================
