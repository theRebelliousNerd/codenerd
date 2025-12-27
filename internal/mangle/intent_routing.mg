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
tool_allowed(P, /glob) :- persona(P).
tool_allowed(P, /grep) :- persona(P).

# Code DOM tools - available to all personas for semantic code navigation
tool_allowed(P, /get_elements) :- persona(P).
tool_allowed(P, /get_element) :- persona(P).

# Coder-specific tools
tool_allowed(/coder, /write_file).
tool_allowed(/coder, /edit_file).
tool_allowed(/coder, /delete_file).
tool_allowed(/coder, /run_build).
tool_allowed(/coder, /run_command).
tool_allowed(/coder, /bash).
tool_allowed(/coder, /git_operation).
tool_allowed(/coder, /edit_lines).
tool_allowed(/coder, /insert_lines).
tool_allowed(/coder, /delete_lines).

# Tester-specific tools
tool_allowed(/tester, /run_tests).
tool_allowed(/tester, /run_command).
tool_allowed(/tester, /bash).
tool_allowed(/tester, /write_file).  # Can write test files
tool_allowed(/tester, /edit_file).
tool_allowed(/tester, /edit_lines).
tool_allowed(/tester, /insert_lines).
tool_allowed(/tester, /delete_lines).

# Reviewer-specific tools (read-heavy)
tool_allowed(/reviewer, /git_diff).
tool_allowed(/reviewer, /git_log).
tool_allowed(/reviewer, /run_command).  # For static analysis tools

# Researcher-specific tools
tool_allowed(/researcher, /web_search).
tool_allowed(/researcher, /web_fetch).
tool_allowed(/researcher, /context7_fetch).
tool_allowed(/researcher, /write_file).  # Can write documentation

# =============================================================================
# SECTION 4.5: Modular Tool Routing
# =============================================================================
# Maps intents to modular tools (internal/tools/*)
# These tools are available to any agent via the JIT system.

# Core filesystem tools - available to all intents
modular_tool_allowed(/read_file, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/list_files, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/glob, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/grep, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/search_code, Intent) :- user_intent(_, _, Intent, _, _).

# Write tools - available for code modification intents
modular_tool_allowed(/write_file, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/edit_file, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/delete_file, Intent) :- intent_category(Intent, /code).

# Shell tools - available for code and test intents
modular_tool_allowed(/run_command, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/run_command, Intent) :- intent_category(Intent, /test).
modular_tool_allowed(/bash, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/bash, Intent) :- intent_category(Intent, /test).
modular_tool_allowed(/run_build, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/run_tests, Intent) :- intent_category(Intent, /test).

# Code DOM tools - available for code intents
modular_tool_allowed(/get_elements, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/get_element, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/edit_lines, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/insert_lines, Intent) :- intent_category(Intent, /code).
modular_tool_allowed(/delete_lines, Intent) :- intent_category(Intent, /code).

# Intent category mappings for code
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /fix.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /implement.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /refactor.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /create.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /modify.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /add.
intent_category(Intent, /code) :-
    user_intent(_, _, Intent, _, _),
    Intent = /update.

# Intent category mappings for test
intent_category(Intent, /test) :-
    user_intent(_, _, Intent, _, _),
    Intent = /test.
intent_category(Intent, /test) :-
    user_intent(_, _, Intent, _, _),
    Intent = /cover.

# Research tools - available for /research intent
modular_tool_allowed(/context7_fetch, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/web_search, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/web_fetch, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_navigate, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_extract, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_screenshot, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_click, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_type, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/browser_close, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/research_cache_get, Intent) :- intent_category(Intent, /research).
modular_tool_allowed(/research_cache_set, Intent) :- intent_category(Intent, /research).

# Context7 also available for /learn and /document intents
modular_tool_allowed(/context7_fetch, Intent) :- intent_category(Intent, /learn).
modular_tool_allowed(/context7_fetch, Intent) :- intent_category(Intent, /document).

# Browser tools also available for verification intents
modular_tool_allowed(/browser_navigate, Intent) :- intent_category(Intent, /verify).
modular_tool_allowed(/browser_extract, Intent) :- intent_category(Intent, /verify).
modular_tool_allowed(/browser_screenshot, Intent) :- intent_category(Intent, /verify).

# Intent category mapping
intent_category(Intent, /research) :-
    user_intent(_, _, Intent, _, _),
    Intent = /research.
intent_category(Intent, /research) :-
    user_intent(_, _, Intent, _, _),
    Intent = /explore.
intent_category(Intent, /learn) :-
    user_intent(_, _, Intent, _, _),
    Intent = /learn.
intent_category(Intent, /learn) :-
    user_intent(_, _, Intent, _, _),
    Intent = /understand.
intent_category(Intent, /document) :-
    user_intent(_, _, Intent, _, _),
    Intent = /document.
intent_category(Intent, /verify) :-
    user_intent(_, _, Intent, _, _),
    Intent = /verify.
intent_category(Intent, /verify) :-
    user_intent(_, _, Intent, _, _),
    Intent = /validate.

# Tool priority (prefer cached results)
modular_tool_priority(/research_cache_get, 90).
modular_tool_priority(/context7_fetch, 80).
modular_tool_priority(/web_search, 75).
modular_tool_priority(/web_fetch, 70).
modular_tool_priority(/browser_navigate, 60).

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
