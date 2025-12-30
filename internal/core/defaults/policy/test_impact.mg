# Test Impact Analysis Rules: Smart Test Selection
# Version: 1.0.0
# Philosophy: Run only tests affected by code changes
#
# This file implements test impact analysis using the dependency graph
# to determine which tests need to run when code is modified.

# =============================================================================
# SECTION 1: TEST IDENTIFICATION
# =============================================================================
# Identify test files and functions by naming convention.

# Go test files end in _test.go
is_test_file(File) :- file_topology(File, _, _), fn:match("_test\\.go$", File).

# Python test files start with test_ or end with _test.py
is_test_file(File) :- file_topology(File, _, _), fn:match("^test_.*\\.py$", fn:basename(File)).
is_test_file(File) :- file_topology(File, _, _), fn:match("_test\\.py$", File).

# TypeScript/JavaScript test files
is_test_file(File) :- file_topology(File, _, _), fn:match("\\.test\\.(ts|tsx|js|jsx)$", File).
is_test_file(File) :- file_topology(File, _, _), fn:match("\\.spec\\.(ts|tsx|js|jsx)$", File).

# Rust test files in tests/ directory
is_test_file(File) :- file_topology(File, _, _), fn:match("/tests/.*\\.rs$", File).

# Test functions are code elements in test files
is_test_function(Ref) :-
    code_element(Ref, /function, File, _, _),
    is_test_file(File).

# Go: Functions starting with Test
is_test_function(Ref) :-
    code_element(Ref, /function, File, _, _),
    fn:match("\\.go$", File),
    fn:match(":Test[A-Z]", Ref).

# Python: Functions starting with test_
is_test_function(Ref) :-
    code_element(Ref, /function, File, _, _),
    fn:match("\\.py$", File),
    fn:match(":test_", Ref).


# =============================================================================
# SECTION 2: DIRECT TEST DEPENDENCIES
# =============================================================================
# Build the direct dependency graph between tests and source code.

# Test depends on source if test imports the source file
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    code_element(SourceRef, _, SourceFile, _, _),
    file_imports(TestFile, SourceFile).

# Test depends on source if test calls source function
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_calls(TestRef, SourceRef).

# Test depends on source if they share the same package and test references source symbol
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    code_element(SourceRef, _, SourceFile, _, _),
    same_package(TestFile, SourceFile),
    test_references_symbol(TestRef, SourceRef).


# =============================================================================
# SECTION 3: TRANSITIVE DEPENDENCIES
# =============================================================================
# Compute transitive closure of test dependencies.

# Direct dependency is transitive
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on(TestRef, SourceRef).

# Transitive through call chain
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on(TestRef, MidRef),
    code_calls(MidRef, SourceRef).

# Transitive through method_of relationship
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on_transitive(TestRef, MethodRef),
    method_of(MethodRef, SourceRef).

# Transitive through struct embedding
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on_transitive(TestRef, TypeRef),
    type_embeds(TypeRef, SourceRef).


# =============================================================================
# SECTION 4: IMPACTED TEST DETECTION
# =============================================================================
# Determine which tests are affected by planned edits.

# A test is impacted if we're editing something it depends on
impacted_test(TestRef) :-
    plan_edit(TargetRef),
    test_depends_on_transitive(TestRef, TargetRef).

# A test is impacted if it's in the same file as something we're editing
impacted_test(TestRef) :-
    plan_edit(TargetRef),
    code_element(TargetRef, _, File, _, _),
    code_element(TestRef, _, File, _, _),
    is_test_function(TestRef).

# A test is impacted if it depends on a modified file (file-level granularity fallback)
impacted_test(TestRef) :-
    modified_file(File),
    code_element(TestRef, _, TestFile, _, _),
    is_test_function(TestRef),
    file_imports(TestFile, File).


# =============================================================================
# SECTION 5: PACKAGE-LEVEL TEST SELECTION
# =============================================================================
# For languages that run tests at package level (Go).

# Get the package containing a test function
test_package(TestRef, Pkg) :-
    is_test_function(TestRef),
    code_element(TestRef, _, File, _, _),
    file_package(File, Pkg).

# A package has impacted tests if any test in it is impacted
impacted_test_package(Pkg) :-
    impacted_test(TestRef),
    test_package(TestRef, Pkg).


# =============================================================================
# SECTION 6: TEST COVERAGE GAPS
# =============================================================================
# Identify code that lacks test coverage.

# A public function has test coverage if any test depends on it
has_test_coverage(Ref) :-
    code_element(Ref, /function, _, _, _),
    test_depends_on_transitive(_, Ref).

# A method has coverage through its receiver
has_test_coverage(Ref) :-
    method_of(Ref, TypeRef),
    has_test_coverage(TypeRef).

# Coverage gap: Public function without tests
coverage_gap(Ref, /no_direct_tests) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    !is_test_file(File),
    !has_test_coverage(Ref).


# =============================================================================
# SECTION 7: TEST PRIORITY SCORING
# =============================================================================
# Score tests for execution priority.

# High priority detection (helper predicate to avoid stratification)
is_high_priority_test(TestRef) :-
    impacted_test(TestRef),
    plan_edit(TargetRef),
    test_depends_on(TestRef, TargetRef).

# High priority: Test directly tests the edited function
test_priority(TestRef, /high) :-
    is_high_priority_test(TestRef).

# Medium priority: Test indirectly depends on edited function (impacted but not high)
test_priority(TestRef, /medium) :-
    impacted_test(TestRef),
    !is_high_priority_test(TestRef).

# Low priority detection (helper predicate)
is_low_priority_test(TestRef) :-
    is_test_function(TestRef),
    plan_edit(TargetRef),
    code_element(TestRef, _, TestFile, _, _),
    code_element(TargetRef, _, TargetFile, _, _),
    same_package(TestFile, TargetFile),
    !impacted_test(TestRef).

# Low priority: Test in same package but no dependency
test_priority(TestRef, /low) :-
    is_low_priority_test(TestRef).


# =============================================================================
# SECTION 8: AGGREGATION HELPERS
# =============================================================================
# Aggregate test results for reporting.

# Count impacted tests (use in transform pipeline)
# impacted_test_count :- impacted_test(Ref) |> let Count = fn:count().

# Get all impacted test files
impacted_test_file(File) :-
    impacted_test(TestRef),
    code_element(TestRef, _, File, _, _).


# =============================================================================
# SECTION 9: HELPER PREDICATES
# =============================================================================
# Supporting predicates for test analysis.

# Two files are in the same package if they share the same directory
same_package(File1, File2) :-
    file_topology(File1, _, _),
    file_topology(File2, _, _),
    fn:dirname(File1) = fn:dirname(File2).

# Test references a symbol (simplified - could be enhanced with AST analysis)
test_references_symbol(TestRef, SourceRef) :-
    code_calls(TestRef, SourceRef).


