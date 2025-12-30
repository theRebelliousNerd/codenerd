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

# Check if instance is resolved (all FAIL_TO_PASS now pass, no PASS_TO_PASS regressions)
swebench_resolved(InstanceID) :-
    swebench_evaluation_result(InstanceID, /true, _, _).

# Check if instance had patch failure
swebench_patch_failed(InstanceID) :-
    swebench_environment(InstanceID, _, /error, _),
    !swebench_patch_applied(InstanceID, _, _).

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
