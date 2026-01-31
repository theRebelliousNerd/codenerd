# Tester Shard Policy - TDD & Test Execution Logic
# Loaded by TesterShard kernel alongside base policy.gl
# Part of Cortex 1.5.0 Architecture

# =============================================================================
# SECTION 1: TEST FRAMEWORK DETECTION
# =============================================================================

Decl file_exists(FilePath).
Decl package_has_dep(DepName).

# Test file detection - declared in schemas.mg (shared predicate)
# is_test_file(FilePath) populated by Go runtime based on file naming patterns

# Test package - populated by Go runtime with packages containing tests
Decl test_package(PackagePath).

# File has public API - populated by Go runtime based on AST analysis
Decl file_has_public_api(FilePath).

test_framework(/gotest) :-
    file_exists("go.mod").

test_framework(/jest) :-
    file_exists("package.json"),
    package_has_dep("jest").

test_framework(/pytest) :-
    file_exists("pyproject.toml").

test_framework(/pytest) :-
    file_exists("setup.py").

test_framework(/pytest) :-
    file_exists("requirements.txt").

test_framework(/cargo) :-
    file_exists("Cargo.toml").

test_framework(/junit) :-
    file_exists("pom.xml").

test_framework(/xunit) :-
    file_exists("*.csproj").

# =============================================================================
# SECTION 2: TESTER TASK CLASSIFICATION
# =============================================================================

Decl tester_task(ID, Action, Target, Timestamp).

tester_action(/run_tests) :-
    tester_task(_, /run_tests, _, _).

tester_action(/generate_tests) :-
    tester_task(_, /generate_tests, _, _).

tester_action(/coverage) :-
    tester_task(_, /coverage, _, _).

tester_action(/tdd_loop) :-
    tester_task(_, /tdd, _, _).

# =============================================================================
# SECTION 3: TDD STATE MACHINE EXTENSIONS
# =============================================================================
# Uses predicates from schemas.gl: test_state, retry_count

# Decl max_retries(Max) - Declared in schemas_state.mg

# Test generation needed when file modified without coverage
needs_test_generation(File) :-
    modified(File),
    !is_test_file(File),
    !test_coverage(File).

# Generate test action
next_tester_action(/generate_test, File) :-
    needs_test_generation(File).

# Run tests action
next_tester_action(/run_tests, Pkg) :-
    test_state(/unknown),
    test_package(Pkg).

next_tester_action(/run_tests, Pkg) :-
    test_state(/idle),
    test_package(Pkg).

# Analyze failures
next_tester_action(/analyze_failure, "") :-
    test_state(/failing),
    retry_count(N),
    max_retries(Max),
    N < Max.

# Escalate after max retries
next_tester_action(/escalate, "") :-
    test_state(/failing),
    retry_count(N),
    max_retries(Max),
    N >= Max.

# =============================================================================
# SECTION 4: COVERAGE ANALYSIS
# =============================================================================

Decl coverage_metric(FilePath, Percentage).
Decl coverage_goal(Goal).

coverage_below_goal(File) :-
    coverage_metric(File, Pct),
    coverage_goal(Goal),
    Pct < Goal.

needs_more_tests(File) :-
    coverage_below_goal(File).

# Coverage warning
coverage_warning(File, Pct, Goal) :-
    coverage_metric(File, Pct),
    coverage_goal(Goal),
    Pct < Goal.

# =============================================================================
# SECTION 5: TEST FILE IDENTIFICATION
# =============================================================================

# is_test_file(File) :-
#    fn:string_ends_with(File, "_test.go").
# is_test_file(File) :-
#    fn:string_ends_with(File, ".test.ts").
# is_test_file(File) :-
#    fn:string_ends_with(File, ".test.js").
# is_test_file(File) :-
#    fn:string_starts_with(fn:basename(File), "test_").
# is_test_file(File) :-
#    fn:string_ends_with(File, "_test.rs").
# is_test_file(File) :-
#    fn:string_contains(File, "/tests/").
# is_test_file(File) :-
#    fn:string_contains(File, "/__tests__/").

# =============================================================================
# SECTION 6: FAILED TEST TRACKING
# =============================================================================

Decl failed_test(TestName, FilePath, Message).
Decl test_output(Output).

# Count failures - requires fn:count builtin (not yet implemented)
# failure_count(N) :-
#    fn:count(failed_test(_, _, _), N).

# Critical failure threshold - requires aggregate counting
# critical_failure_state() :-
#     failure_count(N),
#     N > 10.

# =============================================================================
# SECTION 7: AUTOPOIESIS - LEARNING FROM TEST FAILURES
# =============================================================================

Decl test_failure(TestID, Pattern, Message).
Decl failure_count(Pattern, Count).
Decl test_passed(TestID, Pattern).
Decl pass_count(Pattern, Count).

# Track recurring test failures
recurring_failure_pattern(Pattern) :-
    test_failure(_, Pattern, _),
    failure_count(Pattern, N),
    N >= 3.

# Learn to avoid patterns that cause failures
promote_to_long_term(/avoid_pattern, Pattern) :-
    recurring_failure_pattern(Pattern).

# Track successful test patterns
successful_test_pattern(Pattern) :-
    test_passed(_, Pattern),
    pass_count(Pattern, N),
    N >= 5.

# Promote test templates that work well
promote_to_long_term(/test_template, Pattern) :-
    successful_test_pattern(Pattern).

# =============================================================================
# SECTION 8: TDD LOOP CONSTRAINTS
# =============================================================================

# Block commit while tests failing
block_commit("tests_failing") :-
    test_state(/failing).

# Block commit with low coverage
# Note: Pct is integer 0-100, not float
block_commit("low_coverage") :-
    coverage_metric(_, Pct),
    coverage_goal(Goal),
    Pct < 50.  # Hard minimum

# Require tests for critical files
require_tests(File) :-
    file_topology(File, _, _, _, /false),
    file_has_public_api(File),
    !test_coverage(File).

# =============================================================================
# SECTION 9: PYTEST DIAGNOSTIC RULES (SWE-bench)
# =============================================================================
# Uses predicates from schemas.mg Section 51:
#   pytest_failure(TestName, ErrorCategory, RootFile, RootLine, Message)
#   assertion_mismatch(TestName, Expected, Actual)
#   traceback_frame(TestName, Depth, File, Line, Function, IsTestFile)
#   pytest_root_cause(TestName, FilePath, Line, Function)

# Identify source file failures (not test file failures)
# These are the actual bugs we need to fix
source_file_failure(TestName, File, Line) :-
    pytest_root_cause(TestName, File, Line, _),
    !is_test_file(File).

# Repair priority by error category
# Import errors are highest priority - they block everything
pytest_repair_priority(TestName, 100) :-
    pytest_failure(TestName, /import, _, _, _).

# Type errors indicate fundamental issues
pytest_repair_priority(TestName, 90) :-
    pytest_failure(TestName, /type, _, _, _).

# Attribute errors often from API changes
pytest_repair_priority(TestName, 85) :-
    pytest_failure(TestName, /attribute, _, _, _).

# Key errors from dict/mapping issues
pytest_repair_priority(TestName, 80) :-
    pytest_failure(TestName, /key, _, _, _).

# Assertion errors are the most common test failures
pytest_repair_priority(TestName, 70) :-
    pytest_failure(TestName, /assertion, _, _, _).

# Value errors from input validation
pytest_repair_priority(TestName, 60) :-
    pytest_failure(TestName, /value, _, _, _).

# Runtime errors are catch-all
pytest_repair_priority(TestName, 50) :-
    pytest_failure(TestName, /runtime, _, _, _).

# Fixture errors from test setup
pytest_repair_priority(TestName, 40) :-
    pytest_failure(TestName, /fixture, _, _, _).

# Unknown errors get lowest priority
pytest_repair_priority(TestName, 10) :-
    pytest_failure(TestName, /unknown, _, _, _).

# Find the next pytest test to repair (highest priority, source file failure)
next_pytest_repair(TestName, File, Line, Msg) :-
    source_file_failure(TestName, File, Line),
    pytest_failure(TestName, _, _, _, Msg),
    pytest_repair_priority(TestName, Priority),
    !higher_priority_repair(TestName, Priority).

# Helper: check if there's a higher priority repair waiting
higher_priority_repair(TestName, Priority) :-
    pytest_repair_priority(TestName, Priority),
    source_file_failure(OtherTest, _, _),
    pytest_repair_priority(OtherTest, OtherPriority),
    OtherPriority > Priority,
    TestName != OtherTest.

# Assertion mismatch analysis for targeted fixes
assertion_fix_needed(TestName, Expected, Actual) :-
    assertion_mismatch(TestName, Expected, Actual),
    source_file_failure(TestName, _, _).

# Count traceback depth to source (useful for complexity estimation)
traceback_to_source_depth(TestName, Depth) :-
    pytest_root_cause(TestName, File, _, _),
    traceback_frame(TestName, Depth, File, _, _, /false).

# Identify tests with deep call stacks (may need more context)
complex_failure(TestName) :-
    traceback_to_source_depth(TestName, Depth),
    Depth > 5.

# Identify tests failing in same source file (likely related bugs)
related_failures(TestA, TestB, File) :-
    source_file_failure(TestA, File, _),
    source_file_failure(TestB, File, _),
    TestA != TestB.

# Prioritize file with most failures for batch repair
# Note: Requires aggregation in Go runtime to populate failure_count_by_file
Decl failure_count_by_file(File, Count).

hotspot_file(File) :-
    failure_count_by_file(File, Count),
    Count >= 3.
