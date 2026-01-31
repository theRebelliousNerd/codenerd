# Intent Definitions - Multi-Step Task Patterns
# SECTIONS 23-24: Multi-step task patterns and encyclopedic sentence corpus
# Pattern detection for decomposing complex multi-step requests.

# SECTION 23: MULTI-STEP TASK PATTERNS (/multi_step)
# Encyclopedic corpus for detecting and decomposing multi-step requests.
# These patterns trigger task decomposition into multiple sequential steps.
# =============================================================================

# --- Multi-Step Pattern Declarations ---
# Decl multistep_pattern(Pattern, Category, Relation, Priority).
# Decl multistep_keyword(Pattern, Keyword).
# Decl multistep_verb_pair(Pattern, Verb1, Verb2).
# Decl multistep_example(Pattern, Example).

# =============================================================================
# SEQUENTIAL EXPLICIT PATTERNS
# "first X, then Y, finally Z" style requests
# =============================================================================

multistep_pattern("explicit_first_then", "sequential_explicit", "sequential", 100).
multistep_keyword("explicit_first_then", "first").
multistep_keyword("explicit_first_then", "then").
multistep_keyword("explicit_first_then", "finally").
multistep_keyword("explicit_first_then", "start by").
multistep_keyword("explicit_first_then", "begin with").
multistep_example("explicit_first_then", "first review the code, then fix any issues").
multistep_example("explicit_first_then", "first create the file, then add tests, finally commit").
multistep_example("explicit_first_then", "start by analyzing the codebase, then refactor the hot spots").

multistep_pattern("explicit_step_numbers", "sequential_explicit", "sequential", 95).
multistep_keyword("explicit_step_numbers", "step 1").
multistep_keyword("explicit_step_numbers", "step 2").
multistep_keyword("explicit_step_numbers", "step 3").
multistep_keyword("explicit_step_numbers", "1.").
multistep_keyword("explicit_step_numbers", "2.").
multistep_keyword("explicit_step_numbers", "3.").
multistep_example("explicit_step_numbers", "1. create the handler 2. add tests 3. update the router").
multistep_example("explicit_step_numbers", "step 1: review, step 2: fix, step 3: test").

multistep_pattern("explicit_after_that", "sequential_explicit", "sequential", 90).
multistep_keyword("explicit_after_that", "after that").
multistep_keyword("explicit_after_that", "afterward").
multistep_keyword("explicit_after_that", "afterwards").
multistep_keyword("explicit_after_that", "following that").
multistep_example("explicit_after_that", "fix the bug, after that run the tests").
multistep_example("explicit_after_that", "refactor the function and afterward update the docs").

multistep_pattern("explicit_next", "sequential_explicit", "sequential", 85).
multistep_keyword("explicit_next", "next").
multistep_keyword("explicit_next", "subsequently").
multistep_example("explicit_next", "create the interface, next implement it").
multistep_example("explicit_next", "review the PR and next merge it").

multistep_pattern("explicit_once_done", "sequential_explicit", "sequential", 88).
multistep_keyword("explicit_once_done", "once done").
multistep_keyword("explicit_once_done", "when done").
multistep_keyword("explicit_once_done", "when finished").
multistep_keyword("explicit_once_done", "after done").
multistep_keyword("explicit_once_done", "after finished").
multistep_example("explicit_once_done", "fix the tests, once done commit the changes").
multistep_example("explicit_once_done", "refactor that function and when you're done run the benchmarks").

# =============================================================================
# REVIEW-THEN-FIX PATTERNS
# "review X and fix issues" - implicit sequential dependency
# =============================================================================

multistep_pattern("implicit_review_fix", "review_then_fix", "sequential", 92).
multistep_keyword("implicit_review_fix", "review and fix").
multistep_keyword("implicit_review_fix", "check and fix").
multistep_keyword("implicit_review_fix", "find and fix").
multistep_keyword("implicit_review_fix", "audit and fix").
multistep_verb_pair("implicit_review_fix", /review, /fix).
multistep_verb_pair("implicit_review_fix", /analyze, /fix).
multistep_verb_pair("implicit_review_fix", /security, /fix).
multistep_example("implicit_review_fix", "review auth.go and fix any issues").
multistep_example("implicit_review_fix", "check the handlers and fix any bugs").
multistep_example("implicit_review_fix", "find and fix all security issues").

# =============================================================================
# CREATE-THEN-VALIDATE PATTERNS
# "create X and test it" - mutation followed by verification
# =============================================================================

multistep_pattern("implicit_create_test", "create_then_validate", "sequential", 90).
multistep_keyword("implicit_create_test", "create and test").
multistep_keyword("implicit_create_test", "implement and test").
multistep_keyword("implicit_create_test", "add and test").
multistep_keyword("implicit_create_test", "build and test").
multistep_keyword("implicit_create_test", "make sure it works").
multistep_verb_pair("implicit_create_test", /create, /test).
multistep_verb_pair("implicit_create_test", /fix, /test).
multistep_verb_pair("implicit_create_test", /refactor, /test).
multistep_example("implicit_create_test", "create a new handler and test it").
multistep_example("implicit_create_test", "implement the feature and write tests").
multistep_example("implicit_create_test", "add the endpoint and make sure it works").

# =============================================================================
# VERIFY-AFTER-MUTATION PATTERNS
# "fix X and verify/run tests" - mutation followed by verification
# =============================================================================

multistep_pattern("implicit_fix_verify", "verify_after_mutation", "sequential", 88).
multistep_keyword("implicit_fix_verify", "fix and verify").
multistep_keyword("implicit_fix_verify", "change and test").
multistep_keyword("implicit_fix_verify", "update and check").
multistep_keyword("implicit_fix_verify", "fix and run tests").
multistep_verb_pair("implicit_fix_verify", /fix, /test).
multistep_verb_pair("implicit_fix_verify", /refactor, /test).
multistep_verb_pair("implicit_fix_verify", /create, /test).
multistep_example("implicit_fix_verify", "fix the authentication and verify it works").
multistep_example("implicit_fix_verify", "change the handler and run the tests").
multistep_example("implicit_fix_verify", "update the config and make sure nothing breaks").

# =============================================================================
# RESEARCH-THEN-ACT PATTERNS
# "figure out X then implement" - learning followed by action
# =============================================================================

multistep_pattern("implicit_research_implement", "research_then_act", "sequential", 85).
multistep_keyword("implicit_research_implement", "research and implement").
multistep_keyword("implicit_research_implement", "figure out and").
multistep_keyword("implicit_research_implement", "learn how to and").
multistep_keyword("implicit_research_implement", "understand and then").
multistep_verb_pair("implicit_research_implement", /research, /create).
multistep_verb_pair("implicit_research_implement", /research, /fix).
multistep_verb_pair("implicit_research_implement", /explore, /create).
multistep_example("implicit_research_implement", "research how to implement OAuth and then add it").
multistep_example("implicit_research_implement", "figure out the API and implement the client").
multistep_example("implicit_research_implement", "understand the codebase structure and then refactor").

# =============================================================================
# ANALYZE-THEN-OPTIMIZE PATTERNS
# "analyze X and improve" - analysis followed by improvement
# =============================================================================

multistep_pattern("implicit_analyze_optimize", "analyze_then_optimize", "sequential", 85).
multistep_keyword("implicit_analyze_optimize", "analyze and optimize").
multistep_keyword("implicit_analyze_optimize", "profile and improve").
multistep_keyword("implicit_analyze_optimize", "find bottlenecks and fix").
multistep_verb_pair("implicit_analyze_optimize", /analyze, /refactor).
multistep_verb_pair("implicit_analyze_optimize", /analyze, /fix).
multistep_example("implicit_analyze_optimize", "analyze the performance and optimize the hot paths").
multistep_example("implicit_analyze_optimize", "profile the API and improve response times").
multistep_example("implicit_analyze_optimize", "find bottlenecks in the database layer and fix them").

# =============================================================================
# SECURITY-AUDIT-FIX PATTERNS
# "security scan and fix" - security analysis followed by remediation
# =============================================================================

multistep_pattern("implicit_security_fix", "security_audit_fix", "sequential", 93).
multistep_keyword("implicit_security_fix", "security scan and fix").
multistep_keyword("implicit_security_fix", "audit and fix").
multistep_keyword("implicit_security_fix", "find vulnerabilities and fix").
multistep_verb_pair("implicit_security_fix", /security, /fix).
multistep_example("implicit_security_fix", "security scan the API handlers and fix any vulnerabilities").
multistep_example("implicit_security_fix", "audit the auth module and patch any issues").
multistep_example("implicit_security_fix", "find security issues in the input validation and fix them").

# =============================================================================
# DOCUMENT-AFTER-CHANGE PATTERNS
# "change X and update docs" - mutation followed by documentation
# =============================================================================

multistep_pattern("implicit_change_document", "document_after_change", "sequential", 80).
multistep_keyword("implicit_change_document", "and update docs").
multistep_keyword("implicit_change_document", "and document").
multistep_keyword("implicit_change_document", "and add comments").
multistep_verb_pair("implicit_change_document", /refactor, /document).
multistep_verb_pair("implicit_change_document", /create, /document).
multistep_verb_pair("implicit_change_document", /fix, /document).
multistep_example("implicit_change_document", "refactor the handler and update the documentation").
multistep_example("implicit_change_document", "add the new endpoint and document it").
multistep_example("implicit_change_document", "change the algorithm and add comments explaining it").

# =============================================================================
# TEST-DRIVEN FLOW PATTERNS
# "write tests first then implement" - TDD style
# =============================================================================

multistep_pattern("tdd_test_first", "test_driven_flow", "sequential", 88).
multistep_keyword("tdd_test_first", "write tests first").
multistep_keyword("tdd_test_first", "tdd").
multistep_keyword("tdd_test_first", "test-driven").
multistep_keyword("tdd_test_first", "tests then implement").
multistep_verb_pair("tdd_test_first", /test, /create).
multistep_example("tdd_test_first", "write tests for the parser first, then implement it").
multistep_example("tdd_test_first", "TDD the new authentication flow").
multistep_example("tdd_test_first", "create tests and then the implementation for the cache").

# =============================================================================
# CONDITIONAL SUCCESS PATTERNS
# "X, if successful, Y" - conditional execution on success
# =============================================================================

multistep_pattern("conditional_if_success", "conditional_success", "conditional", 85).
multistep_keyword("conditional_if_success", "if it works").
multistep_keyword("conditional_if_success", "if successful").
multistep_keyword("conditional_if_success", "if it passes").
multistep_keyword("conditional_if_success", "on success").
multistep_keyword("conditional_if_success", "assuming it works").
multistep_example("conditional_if_success", "run the tests, if they pass, commit").
multistep_example("conditional_if_success", "fix the bug and if it works deploy to staging").
multistep_example("conditional_if_success", "refactor and on success merge the PR").

multistep_pattern("conditional_tests_pass", "conditional_success", "conditional", 87).
multistep_keyword("conditional_tests_pass", "if tests pass").
multistep_keyword("conditional_tests_pass", "when tests pass").
multistep_keyword("conditional_tests_pass", "once tests are green").
multistep_example("conditional_tests_pass", "fix the handler, if the tests pass, push to main").
multistep_example("conditional_tests_pass", "refactor and when tests are green merge").

# =============================================================================
# CONDITIONAL FAILURE / FALLBACK PATTERNS
# "try X, if fails, Y" - fallback on failure
# =============================================================================

multistep_pattern("fallback_if_fails", "conditional_failure", "fallback", 83).
multistep_keyword("fallback_if_fails", "if it fails").
multistep_keyword("fallback_if_fails", "if it doesn't work").
multistep_keyword("fallback_if_fails", "otherwise").
multistep_keyword("fallback_if_fails", "or else").
multistep_keyword("fallback_if_fails", "on failure").
multistep_example("fallback_if_fails", "try the migration, if it fails, rollback").
multistep_example("fallback_if_fails", "apply the patch, otherwise revert to the backup").
multistep_example("fallback_if_fails", "run the deployment and on failure alert the team").

multistep_pattern("fallback_try_revert", "undo_recovery", "fallback", 82).
multistep_keyword("fallback_try_revert", "revert if fails").
multistep_keyword("fallback_try_revert", "rollback if").
multistep_keyword("fallback_try_revert", "undo if fails").
multistep_keyword("fallback_try_revert", "revert if needed").
multistep_example("fallback_try_revert", "try the database migration, revert if it fails").
multistep_example("fallback_try_revert", "apply the changes but be ready to rollback if needed").
multistep_example("fallback_try_revert", "deploy to production and rollback if something goes wrong").

# =============================================================================
# PARALLEL / INDEPENDENT PATTERNS
# "X and Y" where both can run concurrently
# =============================================================================

multistep_pattern("parallel_independent_and", "parallel_independent", "parallel", 75).
multistep_keyword("parallel_independent_and", "review X and review Y").
multistep_keyword("parallel_independent_and", "scan X and scan Y").
multistep_example("parallel_independent_and", "review auth.go and review handler.go").
multistep_example("parallel_independent_and", "analyze the frontend and the backend").
multistep_example("parallel_independent_and", "scan the API and scan the database layer").

multistep_pattern("parallel_also_additionally", "parallel_independent", "parallel", 70).
multistep_keyword("parallel_also_additionally", "also").
multistep_keyword("parallel_also_additionally", "additionally").
multistep_keyword("parallel_also_additionally", "at the same time").
multistep_keyword("parallel_also_additionally", "simultaneously").
multistep_keyword("parallel_also_additionally", "in parallel").
multistep_example("parallel_also_additionally", "review the API, also check the tests").
multistep_example("parallel_also_additionally", "fix the bug, additionally update the changelog").
multistep_example("parallel_also_additionally", "run lint and at the same time run tests").

# =============================================================================
# COMPOUND WITH REFERENCE PATTERNS
# "X and Y it" - pronoun reference to target
# =============================================================================

multistep_pattern("compound_pronoun_ref", "compound_with_ref", "sequential", 88).
multistep_keyword("compound_pronoun_ref", "and test it").
multistep_keyword("compound_pronoun_ref", "and commit it").
multistep_keyword("compound_pronoun_ref", "and deploy it").
multistep_keyword("compound_pronoun_ref", "and document it").
multistep_keyword("compound_pronoun_ref", "and run them").
multistep_example("compound_pronoun_ref", "create the handler and test it").
multistep_example("compound_pronoun_ref", "fix the bug and commit it").
multistep_example("compound_pronoun_ref", "implement the feature and deploy it").
multistep_example("compound_pronoun_ref", "write the tests and run them").

# =============================================================================
# ITERATIVE / BATCH PATTERNS
# "do X to each/every/all Y"
# =============================================================================

multistep_pattern("iterative_each_every", "iterative_collection", "iterative", 80).
multistep_keyword("iterative_each_every", "each").
multistep_keyword("iterative_each_every", "every").
multistep_keyword("iterative_each_every", "all the").
multistep_keyword("iterative_each_every", "for each").
multistep_keyword("iterative_each_every", "for every").
multistep_example("iterative_each_every", "review each handler in cmd/api/").
multistep_example("iterative_each_every", "fix every failing test").
multistep_example("iterative_each_every", "refactor all the deprecated functions").
multistep_example("iterative_each_every", "for each model, add validation").

multistep_pattern("batch_all_files", "batch_operation", "iterative", 78).
multistep_keyword("batch_all_files", "all files").
multistep_keyword("batch_all_files", "entire codebase").
multistep_keyword("batch_all_files", "whole project").
multistep_keyword("batch_all_files", "all go files").
multistep_keyword("batch_all_files", "all typescript files").
multistep_example("batch_all_files", "format all go files").
multistep_example("batch_all_files", "lint the entire codebase").
multistep_example("batch_all_files", "review all typescript files").

# =============================================================================
# PIPELINE / CHAIN PATTERNS
# "X then pass results to Y"
# =============================================================================

multistep_pattern("pipeline_pass_output", "pipeline_chain", "sequential", 85).
multistep_keyword("pipeline_pass_output", "pass the results to").
multistep_keyword("pipeline_pass_output", "feed output to").
multistep_keyword("pipeline_pass_output", "use the results to").
multistep_keyword("pipeline_pass_output", "pipe to").
multistep_example("pipeline_pass_output", "analyze the code and pass the results to the optimizer").
multistep_example("pipeline_pass_output", "review for security issues and use the findings to fix").
multistep_example("pipeline_pass_output", "run static analysis and feed output to the report generator").

multistep_pattern("pipeline_based_on", "pipeline_chain", "sequential", 86).
multistep_keyword("pipeline_based_on", "based on the results").
multistep_keyword("pipeline_based_on", "according to findings").
multistep_keyword("pipeline_based_on", "based on issues").
multistep_example("pipeline_based_on", "review the handlers and then fix based on the findings").
multistep_example("pipeline_based_on", "analyze complexity and refactor according to the results").

# =============================================================================
# COMPARE AND CHOOSE PATTERNS
# "compare X and Y, pick best"
# =============================================================================

multistep_pattern("compare_and_choose", "compare_and_choose", "sequential", 75).
multistep_keyword("compare_and_choose", "compare and pick").
multistep_keyword("compare_and_choose", "compare and choose").
multistep_keyword("compare_and_choose", "evaluate and recommend").
multistep_example("compare_and_choose", "compare the two implementations and pick the best").
multistep_example("compare_and_choose", "evaluate approach A vs B and recommend one").

# =============================================================================
# CONSTRAINT / EXCLUSION PATTERNS
# "do X but not Y" or "do X while keeping Y"
# =============================================================================

multistep_pattern("constraint_but_not", "refactor_preserve", "sequential", 82).
multistep_keyword("constraint_but_not", "but not").
multistep_keyword("constraint_but_not", "but skip").
multistep_keyword("constraint_but_not", "except").
multistep_keyword("constraint_but_not", "excluding").
multistep_keyword("constraint_but_not", "while keeping").
multistep_keyword("constraint_but_not", "preserving").
multistep_example("constraint_but_not", "refactor the handlers but not the middleware").
multistep_example("constraint_but_not", "review all files except tests").
multistep_example("constraint_but_not", "update the API while keeping backwards compatibility").
multistep_example("constraint_but_not", "fix the auth but don't touch the session logic").

# =============================================================================
# GIT WORKFLOW PATTERNS
# "commit and push", "add, commit, and push"
# =============================================================================

multistep_pattern("git_commit_push", "sequential_implicit", "sequential", 85).
multistep_keyword("git_commit_push", "commit and push").
multistep_keyword("git_commit_push", "add and commit").
multistep_keyword("git_commit_push", "stage and commit and push").
multistep_verb_pair("git_commit_push", /git, /git).
multistep_example("git_commit_push", "commit and push").
multistep_example("git_commit_push", "add the changes, commit, and push").
multistep_example("git_commit_push", "stage everything and commit and push to origin").

multistep_pattern("git_branch_workflow", "sequential_implicit", "sequential", 80).
multistep_keyword("git_branch_workflow", "create branch and").
multistep_keyword("git_branch_workflow", "checkout and").
multistep_keyword("git_branch_workflow", "switch to and").
multistep_example("git_branch_workflow", "create a new branch for the feature and start working").
multistep_example("git_branch_workflow", "checkout main and pull the latest").
