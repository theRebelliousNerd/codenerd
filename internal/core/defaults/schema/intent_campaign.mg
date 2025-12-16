# Intent Campaign

# =============================================================================
# SECTION 15: CAMPAIGNS (/campaign) - CAMPAIGN SYSTEM
# Multi-phase, long-running task requests.
# =============================================================================

intent_definition("Start a campaign.", /campaign, "start").
intent_category("Start a campaign.", /mutation).

intent_definition("Start a campaign to rewrite auth.", /campaign, "rewrite auth").
intent_category("Start a campaign to rewrite auth.", /mutation).

intent_definition("I want to refactor the entire codebase.", /campaign, "refactor").
intent_category("I want to refactor the entire codebase.", /mutation).

intent_definition("Help me migrate to a new framework.", /campaign, "migration").
intent_category("Help me migrate to a new framework.", /mutation).

intent_definition("Let's do a major feature.", /campaign, "feature").
intent_category("Let's do a major feature.", /mutation).

intent_definition("This is going to be a big task.", /campaign, "big_task").
intent_category("This is going to be a big task.", /mutation).

intent_definition("Launch a campaign.", /campaign, "start").
intent_category("Launch a campaign.", /mutation).

intent_definition("Begin campaign mode.", /campaign, "start").
intent_category("Begin campaign mode.", /mutation).

intent_definition("Start a multi-phase project.", /campaign, "project").
intent_category("Start a multi-phase project.", /mutation).

intent_definition("Campaign status.", /campaign, "status").
intent_category("Campaign status.", /query).

intent_definition("Show campaign progress.", /campaign, "progress").
intent_category("Show campaign progress.", /query).

intent_definition("What phase are we on?", /campaign, "phase").
intent_category("What phase are we on?", /query).

intent_definition("Continue the campaign.", /campaign, "continue").
intent_category("Continue the campaign.", /mutation).

intent_definition("Pause the campaign.", /campaign, "pause").
intent_category("Pause the campaign.", /mutation).

intent_definition("Cancel the campaign.", /campaign, "cancel").
intent_category("Cancel the campaign.", /mutation).

intent_definition("Abort campaign.", /campaign, "abort").
intent_category("Abort campaign.", /mutation).


# =============================================================================
# SECTION 23: MULTI-STEP TASK PATTERNS (/multi_step)
# Encyclopedic corpus for detecting and decomposing multi-step requests.
# These patterns trigger task decomposition into multiple sequential steps.
# =============================================================================

# --- Multi-Step Pattern Declarations ---
Decl multistep_pattern(Pattern, Category, Relation, Priority).
Decl multistep_keyword(Pattern, Keyword).
Decl multistep_verb_pair(Pattern, Verb1, Verb2).
Decl multistep_example(Pattern, Example).

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

# =============================================================================
# MULTI-STEP INTENT DEFINITIONS (Sentence-Level)
# =============================================================================

# --- Sequential Explicit ---
intent_definition("First review the code, then fix any issues.", /multi_step, "review_fix").
intent_category("First review the code, then fix any issues.", /mutation).

intent_definition("First create the file, then add tests, finally commit.", /multi_step, "create_test_commit").
intent_category("First create the file, then add tests, finally commit.", /mutation).

intent_definition("Start by analyzing the codebase, then refactor.", /multi_step, "analyze_refactor").
intent_category("Start by analyzing the codebase, then refactor.", /mutation).

intent_definition("Fix the bug, after that run the tests.", /multi_step, "fix_test").
intent_category("Fix the bug, after that run the tests.", /mutation).

intent_definition("Create the interface, next implement it.", /multi_step, "create_implement").
intent_category("Create the interface, next implement it.", /mutation).

intent_definition("Fix the tests, once done commit the changes.", /multi_step, "fix_commit").
intent_category("Fix the tests, once done commit the changes.", /mutation).

# --- Review Then Fix ---
intent_definition("Review auth.go and fix any issues.", /multi_step, "review_fix_file").
intent_category("Review auth.go and fix any issues.", /mutation).

intent_definition("Check the handlers and fix any bugs.", /multi_step, "check_fix").
intent_category("Check the handlers and fix any bugs.", /mutation).

intent_definition("Find and fix all security issues.", /multi_step, "find_fix_security").
intent_category("Find and fix all security issues.", /mutation).

intent_definition("Review and fix issues in the codebase.", /multi_step, "review_fix_codebase").
intent_category("Review and fix issues in the codebase.", /mutation).

# --- Create Then Test ---
intent_definition("Create a new handler and test it.", /multi_step, "create_test").
intent_category("Create a new handler and test it.", /mutation).

intent_definition("Implement the feature and write tests.", /multi_step, "implement_test").
intent_category("Implement the feature and write tests.", /mutation).

intent_definition("Add the endpoint and make sure it works.", /multi_step, "add_verify").
intent_category("Add the endpoint and make sure it works.", /mutation).

intent_definition("Fix it and test it.", /multi_step, "fix_test").
intent_category("Fix it and test it.", /mutation).

intent_definition("Refactor and test.", /multi_step, "refactor_test").
intent_category("Refactor and test.", /mutation).

# --- Security Audit and Fix ---
intent_definition("Security scan and fix vulnerabilities.", /multi_step, "security_fix").
intent_category("Security scan and fix vulnerabilities.", /mutation).

intent_definition("Audit the auth module and patch any issues.", /multi_step, "audit_patch").
intent_category("Audit the auth module and patch any issues.", /mutation).

intent_definition("Scan for security issues and fix them.", /multi_step, "scan_fix").
intent_category("Scan for security issues and fix them.", /mutation).

# --- Research Then Implement ---
intent_definition("Research how to implement OAuth and then add it.", /multi_step, "research_implement").
intent_category("Research how to implement OAuth and then add it.", /mutation).

intent_definition("Figure out the API and implement the client.", /multi_step, "figure_implement").
intent_category("Figure out the API and implement the client.", /mutation).

intent_definition("Understand the codebase and then refactor.", /multi_step, "understand_refactor").
intent_category("Understand the codebase and then refactor.", /mutation).

# --- Analyze Then Optimize ---
intent_definition("Analyze performance and optimize.", /multi_step, "analyze_optimize").
intent_category("Analyze performance and optimize.", /mutation).

intent_definition("Profile the API and improve response times.", /multi_step, "profile_improve").
intent_category("Profile the API and improve response times.", /mutation).

intent_definition("Find bottlenecks and fix them.", /multi_step, "find_fix_bottlenecks").
intent_category("Find bottlenecks and fix them.", /mutation).

# --- Conditional Patterns ---
intent_definition("Run the tests, if they pass, commit.", /multi_step, "test_commit_conditional").
intent_category("Run the tests, if they pass, commit.", /mutation).

intent_definition("Fix the bug and if it works deploy to staging.", /multi_step, "fix_deploy_conditional").
intent_category("Fix the bug and if it works deploy to staging.", /mutation).

intent_definition("Try the migration, if it fails, rollback.", /multi_step, "migrate_rollback").
intent_category("Try the migration, if it fails, rollback.", /mutation).

intent_definition("Apply the patch, otherwise revert.", /multi_step, "patch_revert").
intent_category("Apply the patch, otherwise revert.", /mutation).

# --- Parallel Patterns ---
intent_definition("Review auth.go and review handler.go.", /multi_step, "review_parallel").
intent_category("Review auth.go and review handler.go.", /query).

intent_definition("Run lint and at the same time run tests.", /multi_step, "lint_test_parallel").
intent_category("Run lint and at the same time run tests.", /mutation).

intent_definition("Fix the bug, also update the changelog.", /multi_step, "fix_update_parallel").
intent_category("Fix the bug, also update the changelog.", /mutation).

# --- Pronoun Reference ---
intent_definition("Create the handler and test it.", /multi_step, "create_test_it").
intent_category("Create the handler and test it.", /mutation).

intent_definition("Fix the bug and commit it.", /multi_step, "fix_commit_it").
intent_category("Fix the bug and commit it.", /mutation).

intent_definition("Write the tests and run them.", /multi_step, "write_run_tests").
intent_category("Write the tests and run them.", /mutation).

# --- TDD Patterns ---
intent_definition("Write tests first, then implement.", /multi_step, "tdd_flow").
intent_category("Write tests first, then implement.", /mutation).

intent_definition("TDD the new feature.", /multi_step, "tdd").
intent_category("TDD the new feature.", /mutation).

intent_definition("Test-driven development for the parser.", /multi_step, "tdd_parser").
intent_category("Test-driven development for the parser.", /mutation).

# --- Iterative/Batch Patterns ---
intent_definition("Review each handler in the API.", /multi_step, "review_each").
intent_category("Review each handler in the API.", /query).

intent_definition("Fix every failing test.", /multi_step, "fix_every_test").
intent_category("Fix every failing test.", /mutation).

intent_definition("Refactor all deprecated functions.", /multi_step, "refactor_all").
intent_category("Refactor all deprecated functions.", /mutation).

intent_definition("Format all go files.", /multi_step, "format_all").
intent_category("Format all go files.", /mutation).

intent_definition("Lint the entire codebase.", /multi_step, "lint_all").
intent_category("Lint the entire codebase.", /query).

# --- Git Workflow Patterns ---
intent_definition("Commit and push.", /multi_step, "commit_push").
intent_category("Commit and push.", /mutation).

intent_definition("Add, commit, and push.", /multi_step, "add_commit_push").
intent_category("Add, commit, and push.", /mutation).

intent_definition("Stage the changes and commit.", /multi_step, "stage_commit").
intent_category("Stage the changes and commit.", /mutation).

intent_definition("Create a branch and start working.", /multi_step, "branch_work").
intent_category("Create a branch and start working.", /mutation).

intent_definition("Checkout main and pull.", /multi_step, "checkout_pull").
intent_category("Checkout main and pull.", /mutation).

# --- Document After Change ---
intent_definition("Refactor and update the documentation.", /multi_step, "refactor_document").
intent_category("Refactor and update the documentation.", /mutation).

intent_definition("Add the endpoint and document it.", /multi_step, "add_document").
intent_category("Add the endpoint and document it.", /mutation).

intent_definition("Change the algorithm and add comments.", /multi_step, "change_comment").
intent_category("Change the algorithm and add comments.", /mutation).

# --- Pipeline Patterns ---
intent_definition("Review and fix based on findings.", /multi_step, "review_fix_pipeline").
intent_category("Review and fix based on findings.", /mutation).

intent_definition("Analyze and pass results to the optimizer.", /multi_step, "analyze_optimize_pipeline").
intent_category("Analyze and pass results to the optimizer.", /mutation).

# --- Constraint Patterns ---
intent_definition("Refactor but not the middleware.", /multi_step, "refactor_constrained").
intent_category("Refactor but not the middleware.", /mutation).

intent_definition("Review all files except tests.", /multi_step, "review_except").
intent_category("Review all files except tests.", /query).

intent_definition("Update the API while keeping backwards compatibility.", /multi_step, "update_preserve").
intent_category("Update the API while keeping backwards compatibility.", /mutation).

# =============================================================================
# INFERENCE RULES FOR MULTI-STEP DETECTION
# =============================================================================

# Check if input contains a multi-step keyword
# NOTE: fn:string_contains is not a Mangle built-in. These rules need to be
# implemented as virtual predicates in Go that perform string matching.
Decl is_multistep_input(Input).
# is_multistep_input(Input) :-
#     multistep_keyword(Pattern, Keyword),
#     fn:string_contains(Input, Keyword).

# Get the best matching pattern for input
Decl best_multistep_pattern(Input, Pattern, Priority).
# best_multistep_pattern(Input, Pattern, Priority) :-
#     multistep_keyword(Pattern, Keyword),
#     fn:string_contains(Input, Keyword),
#     multistep_pattern(Pattern, _, _, Priority).

# Get verb pairs for a pattern
Decl pattern_verb_pair(Pattern, Verb1, Verb2).
pattern_verb_pair(Pattern, Verb1, Verb2) :-
    multistep_verb_pair(Pattern, Verb1, Verb2).

# Get relation type for a pattern
Decl pattern_relation(Pattern, Relation).
pattern_relation(Pattern, Relation) :-
    multistep_pattern(Pattern, _, Relation, _).


# =============================================================================
# SECTION 24: ENCYCLOPEDIC MULTI-STEP SENTENCE CORPUS
# Complete coverage of multi-step request phrasings
# =============================================================================

# ---------------------------------------------------------------------------
# SEQUENTIAL EXPLICIT - "First X, then Y" patterns
# ---------------------------------------------------------------------------

# First-Then-Finally patterns
intent_definition("First review the code, then fix any issues, finally run the tests.", /multi_step, "review_fix_test").
intent_category("First review the code, then fix any issues, finally run the tests.", /mutation).

intent_definition("First analyze the performance, then optimize the bottlenecks.", /multi_step, "analyze_optimize").
intent_category("First analyze the performance, then optimize the bottlenecks.", /mutation).

intent_definition("First understand how it works, then refactor it.", /multi_step, "understand_refactor").
intent_category("First understand how it works, then refactor it.", /mutation).

intent_definition("First check for bugs, then fix them, finally commit.", /multi_step, "check_fix_commit").
intent_category("First check for bugs, then fix them, finally commit.", /mutation).

intent_definition("First research the library, then implement the integration.", /multi_step, "research_implement").
intent_category("First research the library, then implement the integration.", /mutation).

intent_definition("First scan for security issues, then patch them.", /multi_step, "scan_patch").
intent_category("First scan for security issues, then patch them.", /mutation).

intent_definition("First explore the codebase, then identify refactoring opportunities.", /multi_step, "explore_identify").
intent_category("First explore the codebase, then identify refactoring opportunities.", /query).

intent_definition("First run the tests, then analyze failures, finally fix them.", /multi_step, "test_analyze_fix").
intent_category("First run the tests, then analyze failures, finally fix them.", /mutation).

intent_definition("First backup the database, then run the migration.", /multi_step, "backup_migrate").
intent_category("First backup the database, then run the migration.", /mutation).

intent_definition("First create a branch, then make the changes, finally open a PR.", /multi_step, "branch_change_pr").
intent_category("First create a branch, then make the changes, finally open a PR.", /mutation).

# Start-by patterns
intent_definition("Start by reviewing the authentication module.", /multi_step, "start_review").
intent_category("Start by reviewing the authentication module.", /query).

intent_definition("Start by analyzing the dependencies, then upgrade them.", /multi_step, "start_analyze_upgrade").
intent_category("Start by analyzing the dependencies, then upgrade them.", /mutation).

intent_definition("Start by creating the interface, then implement it.", /multi_step, "start_create_implement").
intent_category("Start by creating the interface, then implement it.", /mutation).

intent_definition("Start by writing failing tests, then make them pass.", /multi_step, "start_tdd").
intent_category("Start by writing failing tests, then make them pass.", /mutation).

intent_definition("Begin with security analysis, then address the findings.", /multi_step, "begin_security_fix").
intent_category("Begin with security analysis, then address the findings.", /mutation).

intent_definition("Begin by profiling the code, then optimize hot paths.", /multi_step, "begin_profile_optimize").
intent_category("Begin by profiling the code, then optimize hot paths.", /mutation).

# After-that patterns
intent_definition("Fix the null pointer, after that add proper error handling.", /multi_step, "fix_add_handling").
intent_category("Fix the null pointer, after that add proper error handling.", /mutation).

intent_definition("Refactor the function, afterward update the callers.", /multi_step, "refactor_update_callers").
intent_category("Refactor the function, afterward update the callers.", /mutation).

intent_definition("Create the model, afterwards write the repository.", /multi_step, "create_model_repo").
intent_category("Create the model, afterwards write the repository.", /mutation).

intent_definition("Implement the feature, following that write integration tests.", /multi_step, "implement_integration_test").
intent_category("Implement the feature, following that write integration tests.", /mutation).

# Once-done patterns
intent_definition("Fix all the lint errors, once done run the full test suite.", /multi_step, "fix_lint_test").
intent_category("Fix all the lint errors, once done run the full test suite.", /mutation).

intent_definition("Refactor the database layer, when finished update the documentation.", /multi_step, "refactor_document").
intent_category("Refactor the database layer, when finished update the documentation.", /mutation).

intent_definition("Complete the API endpoints, once complete deploy to staging.", /multi_step, "complete_deploy").
intent_category("Complete the API endpoints, once complete deploy to staging.", /mutation).

intent_definition("Finish the migration, when done verify data integrity.", /multi_step, "finish_verify").
intent_category("Finish the migration, when done verify data integrity.", /mutation).

# Numbered step patterns
intent_definition("1. Review the PR 2. Leave comments 3. Approve or request changes.", /multi_step, "numbered_pr_review").
intent_category("1. Review the PR 2. Leave comments 3. Approve or request changes.", /query).

intent_definition("Step 1: Create the handler. Step 2: Add routes. Step 3: Write tests.", /multi_step, "step_handler_routes_tests").
intent_category("Step 1: Create the handler. Step 2: Add routes. Step 3: Write tests.", /mutation).

intent_definition("1. Backup 2. Migrate 3. Verify 4. Deploy.", /multi_step, "numbered_backup_deploy").
intent_category("1. Backup 2. Migrate 3. Verify 4. Deploy.", /mutation).

intent_definition("1) Analyze the issue 2) Create a fix 3) Test the fix 4) Commit.", /multi_step, "numbered_analyze_commit").
intent_category("1) Analyze the issue 2) Create a fix 3) Test the fix 4) Commit.", /mutation).

# ---------------------------------------------------------------------------
# REVIEW-THEN-FIX - Analysis followed by remediation
# ---------------------------------------------------------------------------

intent_definition("Review the handler and fix any issues you find.", /multi_step, "review_fix_handler").
intent_category("Review the handler and fix any issues you find.", /mutation).

intent_definition("Check the error handling and improve where needed.", /multi_step, "check_improve_errors").
intent_category("Check the error handling and improve where needed.", /mutation).

intent_definition("Audit the authentication flow and fix vulnerabilities.", /multi_step, "audit_fix_auth").
intent_category("Audit the authentication flow and fix vulnerabilities.", /mutation).

intent_definition("Review the database queries and optimize slow ones.", /multi_step, "review_optimize_queries").
intent_category("Review the database queries and optimize slow ones.", /mutation).

intent_definition("Check the API endpoints for security issues and patch them.", /multi_step, "check_patch_api").
intent_category("Check the API endpoints for security issues and patch them.", /mutation).

intent_definition("Analyze the memory usage and fix any leaks.", /multi_step, "analyze_fix_leaks").
intent_category("Analyze the memory usage and fix any leaks.", /mutation).

intent_definition("Find all TODO comments and address them.", /multi_step, "find_address_todos").
intent_category("Find all TODO comments and address them.", /mutation).

intent_definition("Look for code duplication and refactor it out.", /multi_step, "find_refactor_duplication").
intent_category("Look for code duplication and refactor it out.", /mutation).

intent_definition("Identify dead code and remove it.", /multi_step, "identify_remove_dead_code").
intent_category("Identify dead code and remove it.", /mutation).

intent_definition("Find race conditions and fix them.", /multi_step, "find_fix_race_conditions").
intent_category("Find race conditions and fix them.", /mutation).

intent_definition("Review for OWASP top 10 and remediate.", /multi_step, "review_remediate_owasp").
intent_category("Review for OWASP top 10 and remediate.", /mutation).

intent_definition("Check for hardcoded secrets and extract to config.", /multi_step, "check_extract_secrets").
intent_category("Check for hardcoded secrets and extract to config.", /mutation).

# ---------------------------------------------------------------------------
# CREATE-THEN-VALIDATE - Creation followed by testing
# ---------------------------------------------------------------------------

intent_definition("Create a new endpoint and write tests for it.", /multi_step, "create_test_endpoint").
intent_category("Create a new endpoint and write tests for it.", /mutation).

intent_definition("Implement the service and verify it works.", /multi_step, "implement_verify_service").
intent_category("Implement the service and verify it works.", /mutation).

intent_definition("Add the feature and make sure tests pass.", /multi_step, "add_ensure_tests").
intent_category("Add the feature and make sure tests pass.", /mutation).

intent_definition("Write the migration and verify it applies correctly.", /multi_step, "write_verify_migration").
intent_category("Write the migration and verify it applies correctly.", /mutation).

intent_definition("Create the validator and test edge cases.", /multi_step, "create_test_validator").
intent_category("Create the validator and test edge cases.", /mutation).

intent_definition("Build the parser and verify it handles all formats.", /multi_step, "build_verify_parser").
intent_category("Build the parser and verify it handles all formats.", /mutation).

intent_definition("Implement caching and benchmark the improvement.", /multi_step, "implement_benchmark_cache").
intent_category("Implement caching and benchmark the improvement.", /mutation).

intent_definition("Add rate limiting and test under load.", /multi_step, "add_test_rate_limiting").
intent_category("Add rate limiting and test under load.", /mutation).

intent_definition("Create the middleware and verify it intercepts requests.", /multi_step, "create_verify_middleware").
intent_category("Create the middleware and verify it intercepts requests.", /mutation).

intent_definition("Implement retry logic and test failure scenarios.", /multi_step, "implement_test_retry").
intent_category("Implement retry logic and test failure scenarios.", /mutation).

# ---------------------------------------------------------------------------
# CONDITIONAL SUCCESS - Action contingent on success
# ---------------------------------------------------------------------------

intent_definition("Run the tests, if they pass, push to main.", /multi_step, "test_push_conditional").
intent_category("Run the tests, if they pass, push to main.", /mutation).

intent_definition("Fix the bug, and if it works, deploy to staging.", /multi_step, "fix_deploy_staging").
intent_category("Fix the bug, and if it works, deploy to staging.", /mutation).

intent_definition("Refactor the function, if successful, update the docs.", /multi_step, "refactor_docs_conditional").
intent_category("Refactor the function, if successful, update the docs.", /mutation).

intent_definition("Run the migration, if it succeeds, notify the team.", /multi_step, "migrate_notify_conditional").
intent_category("Run the migration, if it succeeds, notify the team.", /mutation).

intent_definition("Apply the patch, on success, merge the PR.", /multi_step, "patch_merge_conditional").
intent_category("Apply the patch, on success, merge the PR.", /mutation).

intent_definition("Build the project, if no errors, create a release.", /multi_step, "build_release_conditional").
intent_category("Build the project, if no errors, create a release.", /mutation).

intent_definition("Run lint, when tests pass, deploy.", /multi_step, "lint_test_deploy").
intent_category("Run lint, when tests pass, deploy.", /mutation).

intent_definition("Fix the flaky test, once it's green, enable CI.", /multi_step, "fix_enable_ci").
intent_category("Fix the flaky test, once it's green, enable CI.", /mutation).

intent_definition("Run the benchmarks, if performance improves, merge.", /multi_step, "benchmark_merge_conditional").
intent_category("Run the benchmarks, if performance improves, merge.", /mutation).

intent_definition("Test the integration, assuming it works, document it.", /multi_step, "test_document_conditional").
intent_category("Test the integration, assuming it works, document it.", /mutation).

# ---------------------------------------------------------------------------
# CONDITIONAL FAILURE / FALLBACK - Action contingent on failure
# ---------------------------------------------------------------------------

intent_definition("Try the migration, if it fails, rollback immediately.", /multi_step, "migrate_rollback").
intent_category("Try the migration, if it fails, rollback immediately.", /mutation).

intent_definition("Deploy to production, otherwise revert to the previous version.", /multi_step, "deploy_revert").
intent_category("Deploy to production, otherwise revert to the previous version.", /mutation).

intent_definition("Apply the fix, on failure, restore from backup.", /multi_step, "fix_restore_backup").
intent_category("Apply the fix, on failure, restore from backup.", /mutation).

intent_definition("Run the update, if it breaks anything, undo all changes.", /multi_step, "update_undo").
intent_category("Run the update, if it breaks anything, undo all changes.", /mutation).

intent_definition("Try the refactoring, revert if tests fail.", /multi_step, "refactor_revert_conditional").
intent_category("Try the refactoring, revert if tests fail.", /mutation).

intent_definition("Attempt the upgrade, rollback if needed.", /multi_step, "upgrade_rollback").
intent_category("Attempt the upgrade, rollback if needed.", /mutation).

intent_definition("Push the changes, if it fails, fix and retry.", /multi_step, "push_retry").
intent_category("Push the changes, if it fails, fix and retry.", /mutation).

intent_definition("Deploy to staging, if something goes wrong, alert the team.", /multi_step, "deploy_alert").
intent_category("Deploy to staging, if something goes wrong, alert the team.", /mutation).

intent_definition("Run the script, on error, log and continue.", /multi_step, "run_log_continue").
intent_category("Run the script, on error, log and continue.", /mutation).

intent_definition("Apply the patch, if errors occur, open an issue.", /multi_step, "patch_open_issue").
intent_category("Apply the patch, if errors occur, open an issue.", /mutation).

# ---------------------------------------------------------------------------
# PARALLEL - Independent concurrent operations
# ---------------------------------------------------------------------------

intent_definition("Review the frontend and review the backend.", /multi_step, "review_frontend_backend").
intent_category("Review the frontend and review the backend.", /query).

intent_definition("Run unit tests and integration tests in parallel.", /multi_step, "test_parallel").
intent_category("Run unit tests and integration tests in parallel.", /mutation).

intent_definition("Lint the code, also run type checking.", /multi_step, "lint_typecheck_parallel").
intent_category("Lint the code, also run type checking.", /mutation).

intent_definition("Analyze the API, additionally check the database schema.", /multi_step, "analyze_api_db").
intent_category("Analyze the API, additionally check the database schema.", /query).

intent_definition("Review security at the same time as reviewing performance.", /multi_step, "review_security_performance").
intent_category("Review security at the same time as reviewing performance.", /query).

intent_definition("Fix the bug, simultaneously update the changelog.", /multi_step, "fix_changelog_parallel").
intent_category("Fix the bug, simultaneously update the changelog.", /mutation).

intent_definition("Deploy to staging and production in parallel.", /multi_step, "deploy_parallel").
intent_category("Deploy to staging and production in parallel.", /mutation).

intent_definition("Run tests across multiple environments simultaneously.", /multi_step, "test_multi_env").
intent_category("Run tests across multiple environments simultaneously.", /mutation).

intent_definition("Build for Linux and Windows at the same time.", /multi_step, "build_multi_platform").
intent_category("Build for Linux and Windows at the same time.", /mutation).

intent_definition("Scan for vulnerabilities while running performance tests.", /multi_step, "scan_perf_parallel").
intent_category("Scan for vulnerabilities while running performance tests.", /mutation).

# ---------------------------------------------------------------------------
# ITERATIVE / BATCH - Operations over collections
# ---------------------------------------------------------------------------

intent_definition("Review each file in the handlers directory.", /multi_step, "review_each_handler").
intent_category("Review each file in the handlers directory.", /query).

intent_definition("Fix every failing test in the test suite.", /multi_step, "fix_every_failing_test").
intent_category("Fix every failing test in the test suite.", /mutation).

intent_definition("Refactor all deprecated functions to use the new API.", /multi_step, "refactor_all_deprecated").
intent_category("Refactor all deprecated functions to use the new API.", /mutation).

intent_definition("Update all config files to the new format.", /multi_step, "update_all_configs").
intent_category("Update all config files to the new format.", /mutation).

intent_definition("For each endpoint, add rate limiting.", /multi_step, "foreach_rate_limit").
intent_category("For each endpoint, add rate limiting.", /mutation).

intent_definition("For every model, add validation.", /multi_step, "foreach_validation").
intent_category("For every model, add validation.", /mutation).

intent_definition("Review all Go files in the project.", /multi_step, "review_all_go").
intent_category("Review all Go files in the project.", /query).

intent_definition("Fix all lint errors across the codebase.", /multi_step, "fix_all_lint").
intent_category("Fix all lint errors across the codebase.", /mutation).

intent_definition("Add logging to each handler one by one.", /multi_step, "add_logging_each").
intent_category("Add logging to each handler one by one.", /mutation).

intent_definition("Review the entire authentication module.", /multi_step, "review_entire_auth").
intent_category("Review the entire authentication module.", /query).

intent_definition("Test all edge cases for the parser.", /multi_step, "test_all_edge_cases").
intent_category("Test all edge cases for the parser.", /mutation).

intent_definition("Check throughout the codebase for SQL injection.", /multi_step, "check_throughout_sql").
intent_category("Check throughout the codebase for SQL injection.", /query).

# ---------------------------------------------------------------------------
# RESEARCH-THEN-ACT - Learning followed by implementation
# ---------------------------------------------------------------------------

intent_definition("Research how to implement WebSockets and then add them.", /multi_step, "research_websockets").
intent_category("Research how to implement WebSockets and then add them.", /mutation).

intent_definition("Figure out the OAuth flow and implement it.", /multi_step, "figure_oauth").
intent_category("Figure out the OAuth flow and implement it.", /mutation).

intent_definition("Learn how the caching works and then optimize it.", /multi_step, "learn_optimize_cache").
intent_category("Learn how the caching works and then optimize it.", /mutation).

intent_definition("Understand the event system and then add new events.", /multi_step, "understand_add_events").
intent_category("Understand the event system and then add new events.", /mutation).

intent_definition("Look up the API documentation and then integrate.", /multi_step, "lookup_integrate_api").
intent_category("Look up the API documentation and then integrate.", /mutation).

intent_definition("Investigate the bug and then fix it.", /multi_step, "investigate_fix_bug").
intent_category("Investigate the bug and then fix it.", /mutation).

intent_definition("Study the architecture and then propose improvements.", /multi_step, "study_propose_improvements").
intent_category("Study the architecture and then propose improvements.", /query).

intent_definition("Find out how the tests are structured and add new ones.", /multi_step, "findout_add_tests").
intent_category("Find out how the tests are structured and add new ones.", /mutation).

intent_definition("Explore the plugin system and then create a plugin.", /multi_step, "explore_create_plugin").
intent_category("Explore the plugin system and then create a plugin.", /mutation).

intent_definition("Read about best practices and then apply them.", /multi_step, "read_apply_practices").
intent_category("Read about best practices and then apply them.", /mutation).

# ---------------------------------------------------------------------------
# GIT WORKFLOW - Version control operations
# ---------------------------------------------------------------------------

intent_definition("Stage all changes and commit with a message.", /multi_step, "stage_commit").
intent_category("Stage all changes and commit with a message.", /mutation).

intent_definition("Commit the changes and push to origin.", /multi_step, "commit_push").
intent_category("Commit the changes and push to origin.", /mutation).

intent_definition("Create a feature branch and start implementing.", /multi_step, "branch_implement").
intent_category("Create a feature branch and start implementing.", /mutation).

intent_definition("Checkout main, pull latest, then create a branch.", /multi_step, "checkout_pull_branch").
intent_category("Checkout main, pull latest, then create a branch.", /mutation).

intent_definition("Stash changes, pull updates, then pop the stash.", /multi_step, "stash_pull_pop").
intent_category("Stash changes, pull updates, then pop the stash.", /mutation).

intent_definition("Rebase onto main and resolve any conflicts.", /multi_step, "rebase_resolve").
intent_category("Rebase onto main and resolve any conflicts.", /mutation).

intent_definition("Squash the commits and force push.", /multi_step, "squash_force_push").
intent_category("Squash the commits and force push.", /mutation).

intent_definition("Tag the release and push tags.", /multi_step, "tag_push").
intent_category("Tag the release and push tags.", /mutation).

intent_definition("Cherry-pick the commit and push to the release branch.", /multi_step, "cherrypick_push").
intent_category("Cherry-pick the commit and push to the release branch.", /mutation).

intent_definition("Merge the PR and delete the branch.", /multi_step, "merge_delete_branch").
intent_category("Merge the PR and delete the branch.", /mutation).

# ---------------------------------------------------------------------------
# PRONOUN REFERENCE - "X and Y it" patterns
# ---------------------------------------------------------------------------

intent_definition("Create the endpoint and test it thoroughly.", /multi_step, "create_test_it").
intent_category("Create the endpoint and test it thoroughly.", /mutation).

intent_definition("Fix the issue and verify it's resolved.", /multi_step, "fix_verify_it").
intent_category("Fix the issue and verify it's resolved.", /mutation).

intent_definition("Implement the feature and document it.", /multi_step, "implement_document_it").
intent_category("Implement the feature and document it.", /mutation).

intent_definition("Write the function and unit test it.", /multi_step, "write_unittest_it").
intent_category("Write the function and unit test it.", /mutation).

intent_definition("Create the migration and run it.", /multi_step, "create_run_it").
intent_category("Create the migration and run it.", /mutation).

intent_definition("Build the module and publish it.", /multi_step, "build_publish_it").
intent_category("Build the module and publish it.", /mutation).

intent_definition("Write the script and execute it.", /multi_step, "write_execute_it").
intent_category("Write the script and execute it.", /mutation).

intent_definition("Create the tests and run them.", /multi_step, "create_run_them").
intent_category("Create the tests and run them.", /mutation).

intent_definition("Find the bugs and fix them all.", /multi_step, "find_fix_them").
intent_category("Find the bugs and fix them all.", /mutation).

intent_definition("Generate the mocks and use them in tests.", /multi_step, "generate_use_them").
intent_category("Generate the mocks and use them in tests.", /mutation).

# ---------------------------------------------------------------------------
# CONSTRAINT PATTERNS - Exclusion and preservation
# ---------------------------------------------------------------------------

intent_definition("Refactor the handlers but not the middleware.", /multi_step, "refactor_not_middleware").
intent_category("Refactor the handlers but not the middleware.", /mutation).

intent_definition("Review all files except the generated ones.", /multi_step, "review_except_generated").
intent_category("Review all files except the generated ones.", /query).

intent_definition("Fix the bugs but skip the known issues.", /multi_step, "fix_skip_known").
intent_category("Fix the bugs but skip the known issues.", /mutation).

intent_definition("Update the dependencies excluding dev dependencies.", /multi_step, "update_exclude_dev").
intent_category("Update the dependencies excluding dev dependencies.", /mutation).

intent_definition("Refactor while keeping the public API stable.", /multi_step, "refactor_keep_api").
intent_category("Refactor while keeping the public API stable.", /mutation).

intent_definition("Update the code while preserving backwards compatibility.", /multi_step, "update_preserve_compat").
intent_category("Update the code while preserving backwards compatibility.", /mutation).

intent_definition("Optimize the function without changing its behavior.", /multi_step, "optimize_without_change").
intent_category("Optimize the function without changing its behavior.", /mutation).

intent_definition("Clean up the code without breaking tests.", /multi_step, "cleanup_without_break").
intent_category("Clean up the code without breaking tests.", /mutation).

intent_definition("Only update the authentication module.", /multi_step, "only_auth").
intent_category("Only update the authentication module.", /mutation).

intent_definition("Just fix the critical bugs.", /multi_step, "just_critical").
intent_category("Just fix the critical bugs.", /mutation).

# ---------------------------------------------------------------------------
# PIPELINE PATTERNS - Output passing
# ---------------------------------------------------------------------------

intent_definition("Analyze the code and use the results to prioritize fixes.", /multi_step, "analyze_prioritize").
intent_category("Analyze the code and use the results to prioritize fixes.", /mutation).

intent_definition("Run static analysis and feed the output to the fixer.", /multi_step, "static_feed_fixer").
intent_category("Run static analysis and feed the output to the fixer.", /mutation).

intent_definition("Profile the application and based on the results, optimize.", /multi_step, "profile_based_optimize").
intent_category("Profile the application and based on the results, optimize.", /mutation).

intent_definition("Review for security issues and according to findings, patch.", /multi_step, "review_according_patch").
intent_category("Review for security issues and according to findings, patch.", /mutation).

intent_definition("Collect metrics and using the output, generate a report.", /multi_step, "collect_using_report").
intent_category("Collect metrics and using the output, generate a report.", /query).

intent_definition("Run benchmarks and pass results to the optimizer.", /multi_step, "benchmark_pass_optimizer").
intent_category("Run benchmarks and pass results to the optimizer.", /mutation).

# ---------------------------------------------------------------------------
# TDD PATTERNS - Test-driven development
# ---------------------------------------------------------------------------

intent_definition("Write the tests first, then make them pass.", /multi_step, "tdd_write_pass").
intent_category("Write the tests first, then make them pass.", /mutation).

intent_definition("TDD the new authentication system.", /multi_step, "tdd_auth").
intent_category("TDD the new authentication system.", /mutation).

intent_definition("Test-driven develop the payment integration.", /multi_step, "tdd_payment").
intent_category("Test-driven develop the payment integration.", /mutation).

intent_definition("Start with failing tests, then implement until green.", /multi_step, "tdd_failing_green").
intent_category("Start with failing tests, then implement until green.", /mutation).

intent_definition("Red-green-refactor the new feature.", /multi_step, "tdd_red_green_refactor").
intent_category("Red-green-refactor the new feature.", /mutation).

intent_definition("Write acceptance tests first, then build the feature.", /multi_step, "bdd_acceptance_build").
intent_category("Write acceptance tests first, then build the feature.", /mutation).

# ---------------------------------------------------------------------------
# SECURITY PATTERNS - Security audit and fix
# ---------------------------------------------------------------------------

intent_definition("Scan for OWASP vulnerabilities and fix all critical ones.", /multi_step, "scan_fix_owasp").
intent_category("Scan for OWASP vulnerabilities and fix all critical ones.", /mutation).

intent_definition("Audit the input validation and harden it.", /multi_step, "audit_harden_input").
intent_category("Audit the input validation and harden it.", /mutation).

intent_definition("Check for injection vulnerabilities and sanitize inputs.", /multi_step, "check_sanitize_injection").
intent_category("Check for injection vulnerabilities and sanitize inputs.", /mutation).

intent_definition("Review authentication and add MFA support.", /multi_step, "review_add_mfa").
intent_category("Review authentication and add MFA support.", /mutation).

intent_definition("Find exposed secrets and rotate them.", /multi_step, "find_rotate_secrets").
intent_category("Find exposed secrets and rotate them.", /mutation).

intent_definition("Check for XSS vulnerabilities and add escaping.", /multi_step, "check_add_escaping").
intent_category("Check for XSS vulnerabilities and add escaping.", /mutation).

intent_definition("Audit the session management and fix weaknesses.", /multi_step, "audit_fix_sessions").
intent_category("Audit the session management and fix weaknesses.", /mutation).

intent_definition("Review CORS configuration and tighten it.", /multi_step, "review_tighten_cors").
intent_category("Review CORS configuration and tighten it.", /mutation).

# ---------------------------------------------------------------------------
# DOCUMENTATION PATTERNS - Change and document
# ---------------------------------------------------------------------------

intent_definition("Refactor the API and update the OpenAPI spec.", /multi_step, "refactor_update_openapi").
intent_category("Refactor the API and update the OpenAPI spec.", /mutation).

intent_definition("Add the feature and write user documentation.", /multi_step, "add_write_docs").
intent_category("Add the feature and write user documentation.", /mutation).

intent_definition("Change the configuration format and update the README.", /multi_step, "change_update_readme").
intent_category("Change the configuration format and update the README.", /mutation).

intent_definition("Rename the function and update all references in docs.", /multi_step, "rename_update_docs").
intent_category("Rename the function and update all references in docs.", /mutation).

intent_definition("Deprecate the old API and document the migration path.", /multi_step, "deprecate_document_migration").
intent_category("Deprecate the old API and document the migration path.", /mutation).

intent_definition("Add inline comments explaining the algorithm.", /multi_step, "add_comments_algorithm").
intent_category("Add inline comments explaining the algorithm.", /mutation).

# ---------------------------------------------------------------------------
# COMPARE AND CHOOSE PATTERNS
# ---------------------------------------------------------------------------

intent_definition("Compare the two approaches and pick the better one.", /multi_step, "compare_pick").
intent_category("Compare the two approaches and pick the better one.", /query).

intent_definition("Evaluate both implementations and recommend one.", /multi_step, "evaluate_recommend").
intent_category("Evaluate both implementations and recommend one.", /query).

intent_definition("Benchmark both solutions and choose the faster one.", /multi_step, "benchmark_choose").
intent_category("Benchmark both solutions and choose the faster one.", /mutation).

intent_definition("Analyze the trade-offs and suggest the best option.", /multi_step, "analyze_suggest").
intent_category("Analyze the trade-offs and suggest the best option.", /query).

intent_definition("Compare memory usage of both and select the efficient one.", /multi_step, "compare_select_memory").
intent_category("Compare memory usage of both and select the efficient one.", /query).

