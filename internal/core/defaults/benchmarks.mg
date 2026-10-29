# Benchmark-Specific Mangle Predicates
# This file contains predicates specific to code evaluation benchmarks.
# These are NOT required for general-purpose code assistance.
# Load this file only when running benchmark evaluations.

# =============================================================================
# SECTION 1: SWE-BENCH PREDICATES
# =============================================================================
# Predicates for tracking SWE-bench instance state and evaluation.
# These are BENCHMARK-SPECIFIC and not needed for regular use.

# Instance metadata from HuggingFace dataset
# swebench_instance(InstanceID, Repo, BaseCommit, Version)
Decl swebench_instance(InstanceID, Repo, BaseCommit, Version).

# Environment lifecycle tracking
# swebench_environment(InstanceID, ContainerID, State, Timestamp)
# State: /initializing, /cloning, /setup, /ready, /patched, /testing, /evaluating, /terminated
Decl swebench_environment(InstanceID, ContainerID, State, Timestamp).

# Individual test results
# swebench_test_result(InstanceID, TestName, Passed, DurationMs)
Decl swebench_test_result(InstanceID, TestName, Passed, DurationMs).

# Overall evaluation result
# swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount)
Decl swebench_evaluation_result(InstanceID, Resolved, PassedCount, FailedCount).

# Expected test lists from instance
Decl swebench_expected_fail_to_pass(InstanceID, TestName).
Decl swebench_expected_pass_to_pass(InstanceID, TestName).

# Patch tracking
Decl swebench_patch_applied(InstanceID, PatchSize, Timestamp).
Decl swebench_snapshot(InstanceID, SnapshotName, Timestamp).
Decl swebench_restored(InstanceID, SnapshotName, Timestamp).
Decl swebench_evaluation_started(InstanceID, ModelName, Timestamp).
Decl swebench_teardown_complete(InstanceID, Timestamp).

# =============================================================================
# SECTION 2: SWE-BENCH DERIVED RULES
# =============================================================================

# Resolution is the kernel's verdict, not the harness's: an evaluated instance
# is resolved when every expected test (FAIL_TO_PASS and PASS_TO_PASS) has a
# passing swebench_test_result. The Go harness reports its own summary in
# swebench_evaluation_result; that is a record, not the decision. A patch that
# failed to apply records no test results, so every expectation is unmet.
Decl swebench_expected_test(InstanceID, TestName).
swebench_expected_test(InstanceID, TestName) :-
    swebench_expected_fail_to_pass(InstanceID, TestName).
swebench_expected_test(InstanceID, TestName) :-
    swebench_expected_pass_to_pass(InstanceID, TestName).

Decl swebench_test_passed(InstanceID, TestName).
swebench_test_passed(InstanceID, TestName) :-
    swebench_test_result(InstanceID, TestName, /true, _).

# unmet expectation: an expected test with no passing result.
Decl swebench_unmet_expectation(InstanceID, TestName).
swebench_unmet_expectation(InstanceID, TestName) :-
    swebench_expected_test(InstanceID, TestName),
    !swebench_test_passed(InstanceID, TestName).

# Projection for bound negation (a wildcard in a negated literal excludes
# nothing in this Mangle build).
Decl swebench_has_unmet_expectation(InstanceID).
swebench_has_unmet_expectation(InstanceID) :-
    swebench_unmet_expectation(InstanceID, _).

Decl swebench_evaluated(InstanceID).
swebench_evaluated(InstanceID) :-
    swebench_evaluation_result(InstanceID, _, _, _).

Decl swebench_resolved(InstanceID).
swebench_resolved(InstanceID) :-
    swebench_evaluated(InstanceID),
    !swebench_has_unmet_expectation(InstanceID).

# Helper for safe negation
Decl has_patch_applied(InstanceID).
has_patch_applied(InstanceID) :-
    swebench_patch_applied(InstanceID, _, _).

# Check if instance had patch failure
Decl swebench_patch_failed(InstanceID).
swebench_patch_failed(InstanceID) :-
    swebench_environment(InstanceID, _, /error, _),
    !has_patch_applied(InstanceID).

# Count instances by resolution status (requires aggregation in Go runtime)
Decl swebench_resolution_count(Resolved, Count).

# =============================================================================
# SECTION 3: HUMANEVAL PREDICATES (Future)
# =============================================================================
# Placeholder for HumanEval benchmark predicates

# humaneval_problem(ProblemID, FunctionName, DocString)
# Decl humaneval_problem(ProblemID, FunctionName, DocString).

# humaneval_result(ProblemID, Passed, Output)
# Decl humaneval_result(ProblemID, Passed, Output).

# =============================================================================
# SECTION 4: MBPP PREDICATES (Future)
# =============================================================================
# Placeholder for MBPP benchmark predicates

# mbpp_task(TaskID, Description, TestCases)
# Decl mbpp_task(TaskID, Description, TestCases).

# mbpp_result(TaskID, Passed, Output)
# Decl mbpp_result(TaskID, Passed, Output).
