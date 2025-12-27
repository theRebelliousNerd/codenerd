# Intent Routing Rules
# These rules replace hardcoded shard logic with declarative Mangle derivations.
# The JIT system queries these rules to determine agent behavior.

# =============================================================================
# SECTION 1: Action Type Derivation
# =============================================================================
# What used to be hardcoded in CoderShard.parseTask()

# Create actions - wholly new functionality
action_type(/create) :- user_intent(_, /command, /create, _, _).
action_type(/create) :- user_intent(_, /command, /implement, _, _).
action_type(/create) :- user_intent(_, /command, /add, _, _).
action_type(/create) :- user_intent(_, /command, /new, _, _).
action_type(/create) :- user_intent(_, /command, /generate, _, _).

# Modify actions - changes to existing code
action_type(/modify) :- user_intent(_, /command, /fix, _, _).
action_type(/modify) :- user_intent(_, /command, /refactor, _, _).
action_type(/modify) :- user_intent(_, /command, /update, _, _).
action_type(/modify) :- user_intent(_, /command, /change, _, _).
action_type(/modify) :- user_intent(_, /command, /edit, _, _).
action_type(/modify) :- user_intent(_, /command, /patch, _, _).

# Delete actions
action_type(/delete) :- user_intent(_, /command, /remove, _, _).
action_type(/delete) :- user_intent(_, /command, /delete, _, _).

# Query actions - read-only
action_type(/query) :- user_intent(_, /question, _, _, _).
action_type(/query) :- user_intent(_, /command, /find, _, _).
action_type(/query) :- user_intent(_, /command, /search, _, _).
action_type(/query) :- user_intent(_, /command, /explain, _, _).

# =============================================================================
# SECTION 2: Persona Selection
# =============================================================================
# Maps intent verbs to persona atoms for JIT compilation

# Coder persona
persona(/coder) :- user_intent(_, _, /fix, _, _).
persona(/coder) :- user_intent(_, _, /implement, _, _).
persona(/coder) :- user_intent(_, _, /refactor, _, _).
persona(/coder) :- user_intent(_, _, /create, _, _).
persona(/coder) :- user_intent(_, _, /modify, _, _).
persona(/coder) :- user_intent(_, _, /add, _, _).
persona(/coder) :- user_intent(_, _, /update, _, _).
persona(/coder) :- action_type(/create).
persona(/coder) :- action_type(/modify).

# Tester persona
persona(/tester) :- user_intent(_, _, /test, _, _).
persona(/tester) :- user_intent(_, _, /cover, _, _).
persona(/tester) :- user_intent(_, _, /verify, _, _).
persona(/tester) :- user_intent(_, _, /validate, _, _).

# Reviewer persona
persona(/reviewer) :- user_intent(_, _, /review, _, _).
persona(/reviewer) :- user_intent(_, _, /audit, _, _).
persona(/reviewer) :- user_intent(_, _, /check, _, _).
persona(/reviewer) :- user_intent(_, _, /analyze, _, _).
persona(/reviewer) :- user_intent(_, _, /inspect, _, _).

# Researcher persona
persona(/researcher) :- user_intent(_, _, /research, _, _).
persona(/researcher) :- user_intent(_, _, /learn, _, _).
persona(/researcher) :- user_intent(_, _, /document, _, _).
persona(/researcher) :- user_intent(_, _, /understand, _, _).
persona(/researcher) :- user_intent(_, _, /explore, _, _).
persona(/researcher) :- user_intent(_, _, /find, _, _).

# Default to coder for unmatched intents
persona(/coder) :- user_intent(_, _, V, _, _), not persona_matched(V).
persona_matched(V) :- persona(P), P != /coder, user_intent(_, _, V, _, _).

# =============================================================================
# SECTION 3: Test Framework Detection
# =============================================================================
# What used to be hardcoded in TesterShard.detectFramework()

# Go testing
test_framework(/go_test) :- file_exists("go.mod").

# JavaScript/TypeScript
test_framework(/jest) :- file_exists("jest.config.js").
test_framework(/jest) :- file_exists("jest.config.ts").
test_framework(/vitest) :- file_exists("vitest.config.js").
test_framework(/vitest) :- file_exists("vitest.config.ts").
test_framework(/mocha) :- file_exists("mocharc.json").
test_framework(/mocha) :- file_exists(".mocharc.js").

# Python
test_framework(/pytest) :- file_exists("pytest.ini").
test_framework(/pytest) :- file_exists("pyproject.toml"), file_contains("pyproject.toml", "pytest").
test_framework(/pytest) :- file_exists("conftest.py").
test_framework(/unittest) :- file_exists("test_*.py"), not test_framework(/pytest).

# Rust
test_framework(/cargo_test) :- file_exists("Cargo.toml").

# Ruby
test_framework(/rspec) :- file_exists(".rspec").
test_framework(/minitest) :- file_exists("Gemfile"), file_contains("Gemfile", "minitest").

# =============================================================================
# SECTION 4: Tool Selection
# =============================================================================
# Maps personas and action types to allowed tools

# Core tools available to all personas
tool_allowed(P, /read_file) :- persona(P).
tool_allowed(P, /search_code) :- persona(P).
tool_allowed(P, /list_files) :- persona(P).

# Coder-specific tools
tool_allowed(/coder, /write_file).
tool_allowed(/coder, /edit_file).
tool_allowed(/coder, /create_file).
tool_allowed(/coder, /run_build).
tool_allowed(/coder, /git_operation).

# Tester-specific tools
tool_allowed(/tester, /run_tests).
tool_allowed(/tester, /coverage_report).
tool_allowed(/tester, /write_file).  # Can write test files

# Reviewer-specific tools (read-heavy)
tool_allowed(/reviewer, /git_diff).
tool_allowed(/reviewer, /git_log).
tool_allowed(/reviewer, /security_scan).

# Researcher-specific tools
tool_allowed(/researcher, /web_search).
tool_allowed(/researcher, /web_fetch).
tool_allowed(/researcher, /write_file).  # Can write documentation

# =============================================================================
# SECTION 5: Safety Constraints
# =============================================================================
# Constitutional gate integration

# Dangerous operations require explicit permission
requires_permission(/delete_file).
requires_permission(/git_push).
requires_permission(/git_force).
requires_permission(/run_arbitrary_command).
requires_permission(/system_modify).

# Block dangerous patterns
blocked_pattern("rm -rf").
blocked_pattern("sudo").
blocked_pattern("> /dev/").
blocked_pattern("mkfs").
blocked_pattern("dd if=").

# =============================================================================
# SECTION 6: Subagent Spawning
# =============================================================================
# Rules for when to spawn subagents vs inline execution

# Spawn subagent for complex research tasks
spawn_subagent(/researcher) :-
    persona(/researcher),
    user_intent(_, _, _, Target, _),
    complex_target(Target).

# Spawn subagent for parallel test execution
spawn_subagent(/tester) :-
    persona(/tester),
    test_scope(/full_suite).

# Spawn nemesis for adversarial review
spawn_subagent(/nemesis) :-
    persona(/reviewer),
    review_type(/security).

# Complex target detection
complex_target(T) :- target_word_count(T, N), N > 50.
complex_target(T) :- target_contains_multiple_files(T).

# =============================================================================
# SECTION 7: Context Selection
# =============================================================================
# Rules for spreading activation context selection

# High priority: directly referenced files
context_priority(Path, 100) :- user_intent(_, _, _, Path, _), file_exists(Path).

# Medium priority: files in same package
context_priority(Path, 70) :-
    user_intent(_, _, _, Target, _),
    same_package(Target, Path),
    file_exists(Path).

# Lower priority: imported files
context_priority(Path, 50) :-
    user_intent(_, _, _, Target, _),
    imports(Target, Path),
    file_exists(Path).

# Lowest priority: test files for non-test intents
context_priority(Path, 20) :-
    file_topology(Path, _, _),
    is_test_file(Path),
    not persona(/tester).

# Boost priority for failing tests
context_priority(Path, 90) :-
    test_failed(Path, _, _),
    persona(/coder).

# =============================================================================
# SECTION 8: Workflow State Machine
# =============================================================================
# TDD repair loop and other workflow patterns

# TDD states
tdd_state(/red) :- test_failed(_, _, _), not test_passed_after_fix.
tdd_state(/green) :- not test_failed(_, _, _), code_modified_recently.
tdd_state(/refactor) :- tdd_state(/green), code_quality_issue(_, _).

# Next action derivation for TDD
next_action(/run_tests) :- tdd_state(/green), not tests_run_recently.
next_action(/fix_code) :- tdd_state(/red).
next_action(/refactor_code) :- tdd_state(/refactor).

# General next action
next_action(/execute_intent) :- user_intent(_, _, _, _, _), not tdd_state(_).
