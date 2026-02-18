# Intent Routing Rules
# These rules replace hardcoded shard logic with declarative Mangle derivations.
# The JIT system queries these rules to determine agent behavior.

# =============================================================================
# Note: This file depends on predicates declared in:
#   - schemas.mg (user_intent, file_topology, test_failed, etc.)
#   - tester.mg (file_exists, file_contains)
#   - Various schema files for virtual predicates
# =============================================================================

# =============================================================================
# LOCAL SCHEMA DECLARATIONS (for standalone validation)
# These predicates are from other .mg files - not loaded by default in check-mangle
# =============================================================================
# From tester.mg (not in default schemas)
# Decl file_exists(FilePath) - Moved to schemas_world.mg (global)
Decl file_contains(FilePath, Pattern).

# Internal predicates defined only in this file (or missing from defaults)
Decl test_scope(Scope).
Decl review_type(Type).
Decl code_modified_recently().
Decl code_quality_issue(Issue, Details).
Decl complex_target(Target).
Decl target_contains_multiple_files(Target).
Decl target_word_count(Target, Cnt).
Decl tests_run_recently().
Decl test_passed_after_fix().
Decl verb_has_specialist(Verb).
Decl imports(Target, Path).
Decl test_failed(Path, TestName, Reason).
Decl diagnostic_active(Path, Line, Severity, Message).
Decl verb_category(Verb, Category).

# =============================================================================
# SECTION 1: Action Type Derivation
# =============================================================================
# What used to be hardcoded in CoderShard.parseTask()
# Note: Using intent_action_type to avoid schema conflict with action_type/2

# Create actions - wholly new functionality
intent_action_type(/create) :- user_intent(_, /command, /create, _, _).
intent_action_type(/create) :- user_intent(_, /command, /implement, _, _).
intent_action_type(/create) :- user_intent(_, /command, /add, _, _).
intent_action_type(/create) :- user_intent(_, /command, /new, _, _).
intent_action_type(/create) :- user_intent(_, /command, /generate, _, _).

# Modify actions - changes to existing code
intent_action_type(/modify) :- user_intent(_, /command, /fix, _, _).
intent_action_type(/modify) :- user_intent(_, /command, /refactor, _, _).
intent_action_type(/modify) :- user_intent(_, /command, /update, _, _).
intent_action_type(/modify) :- user_intent(_, /command, /change, _, _).
intent_action_type(/modify) :- user_intent(_, /command, /edit, _, _).
intent_action_type(/modify) :- user_intent(_, /command, /patch, _, _).

# Delete actions
intent_action_type(/delete) :- user_intent(_, /command, /remove, _, _).
intent_action_type(/delete) :- user_intent(_, /command, /delete, _, _).

# Query actions - read-only
intent_action_type(/query) :- user_intent(_, /question, _, _, _).
intent_action_type(/query) :- user_intent(_, /command, /find, _, _).
intent_action_type(/query) :- user_intent(_, /command, /search, _, _).
intent_action_type(/query) :- user_intent(_, /command, /explain, _, _).

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
persona(/coder) :- intent_action_type(/create).
persona(/coder) :- intent_action_type(/modify).

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
# Note: We check if specific verbs are NOT matched by tester/reviewer/researcher
# This avoids stratification issues by not referencing persona/1 in the check
persona(/coder) :- user_intent(_, _, V, _, _), !verb_has_specialist(V).

# Verbs that have specialist personas (not coder)
verb_has_specialist(/test).
verb_has_specialist(/cover).
verb_has_specialist(/verify).
verb_has_specialist(/validate).
verb_has_specialist(/review).
verb_has_specialist(/audit).
verb_has_specialist(/check).
verb_has_specialist(/analyze).
verb_has_specialist(/inspect).
verb_has_specialist(/research).
verb_has_specialist(/learn).
verb_has_specialist(/document).
verb_has_specialist(/understand).
verb_has_specialist(/explore).
verb_has_specialist(/find).

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
# Use intermediate predicate to avoid stratification cycle
pytest_detected() :- file_exists("pytest.ini").
pytest_detected() :- file_exists("pyproject.toml"), file_contains("pyproject.toml", "pytest").
pytest_detected() :- file_exists("conftest.py").

test_framework(/pytest) :- pytest_detected().
# Use file_topology directly to check for python test files (IsTestFile=/true)
test_framework(/unittest) :- file_topology(_, _, /python, _, /true), !pytest_detected().

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
tool_allowed(/tester, /get_impacted_tests).
tool_allowed(/tester, /run_impacted_tests).

# Coder can also use test impact tools
tool_allowed(/coder, /get_impacted_tests).
tool_allowed(/coder, /run_impacted_tests).

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
modular_tool_allowed(/write_file, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/edit_file, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/delete_file, Intent) :- verb_category(Intent, /code).

# Shell tools - available for code and test intents
modular_tool_allowed(/run_command, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/run_command, Intent) :- verb_category(Intent, /test).
modular_tool_allowed(/bash, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/bash, Intent) :- verb_category(Intent, /test).
modular_tool_allowed(/run_build, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/run_tests, Intent) :- verb_category(Intent, /test).

# Code DOM tools - available for code intents
modular_tool_allowed(/get_elements, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/get_element, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/edit_lines, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/insert_lines, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/delete_lines, Intent) :- verb_category(Intent, /code).

# Test impact analysis tools - available for code and test intents
modular_tool_allowed(/get_impacted_tests, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/get_impacted_tests, Intent) :- verb_category(Intent, /test).
modular_tool_allowed(/run_impacted_tests, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/run_impacted_tests, Intent) :- verb_category(Intent, /test).

# Intent category mappings for code
verb_category(/fix, /code) :- user_intent(_, _, /fix, _, _).
verb_category(/implement, /code) :- user_intent(_, _, /implement, _, _).
verb_category(/refactor, /code) :- user_intent(_, _, /refactor, _, _).
verb_category(/create, /code) :- user_intent(_, _, /create, _, _).
verb_category(/modify, /code) :- user_intent(_, _, /modify, _, _).
verb_category(/add, /code) :- user_intent(_, _, /add, _, _).
verb_category(/update, /code) :- user_intent(_, _, /update, _, _).

# Intent category mappings for test
verb_category(/test, /test) :- user_intent(_, _, /test, _, _).
verb_category(/cover, /test) :- user_intent(_, _, /cover, _, _).

# Research tools - available for /research intent
modular_tool_allowed(/context7_fetch, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/web_search, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/web_fetch, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_navigate, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_extract, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_screenshot, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_click, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_type, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_close, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/research_cache_get, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/research_cache_set, Intent) :- verb_category(Intent, /research).

# Context7 also available for /learn and /document intents
modular_tool_allowed(/context7_fetch, Intent) :- verb_category(Intent, /learn).
modular_tool_allowed(/context7_fetch, Intent) :- verb_category(Intent, /document).

# Browser tools also available for verification intents
modular_tool_allowed(/browser_navigate, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_extract, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_screenshot, Intent) :- verb_category(Intent, /verify).

# Intent category mappings for research/learn/document/verify
verb_category(/research, /research) :- user_intent(_, _, /research, _, _).
verb_category(/explore, /research) :- user_intent(_, _, /explore, _, _).
verb_category(/learn, /learn) :- user_intent(_, _, /learn, _, _).
verb_category(/understand, /learn) :- user_intent(_, _, /understand, _, _).
verb_category(/document, /document) :- user_intent(_, _, /document, _, _).
verb_category(/verify, /verify) :- user_intent(_, _, /verify, _, _).
verb_category(/validate, /verify) :- user_intent(_, _, /validate, _, _).

# Tool priority (prefer cached results)
modular_tool_priority(/research_cache_get, 90).
modular_tool_priority(/context7_fetch, 80).
modular_tool_priority(/web_search, 75).
modular_tool_priority(/web_fetch, 70).
modular_tool_priority(/browser_navigate, 60).

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
    file_topology(Path, _, _, _, /true),  # IsTestFile = /true
    !persona(/tester).

# Boost priority for failing tests
context_priority(Path, 90) :-
    test_failed(Path, _, _),
    persona(/coder).

# =============================================================================
# SECTION 8: Workflow State Machine
# =============================================================================
# TDD repair loop and other workflow patterns

# TDD states
tdd_state(/red) :- test_failed(_, _, _), !test_passed_after_fix().
tdd_state(/green) :- !test_failed(_, _, _), code_modified_recently().
tdd_state(/refactor) :- tdd_state(/green), code_quality_issue(_, _).

# Next action derivation for TDD
next_action(/run_tests) :- tdd_state(/green), !tests_run_recently().
next_action(/fix_code) :- tdd_state(/red).
next_action(/refactor_code) :- tdd_state(/refactor).

# General next action (only when not in TDD loop)
next_action(/execute_intent) :-
    user_intent(_, _, _, _, _),
    !tdd_state(/red),
    !tdd_state(/green),
    !tdd_state(/refactor).

# =============================================================================
# SECTION 9: Wired Predicates (Improvement)
# =============================================================================
# Wiring for predicates that were previously declared but unconnected

# Derive code modification from execution history
code_modified_recently() :- file_edited(_).

# Derive recent test execution
tests_run_recently() :- action_verified(_, /run_tests, _, _, _).

# Derive test success (heuristic: 100% confidence verification on test run)
test_passed_after_fix() :- action_verified(_, /run_tests, _, 100, _).

# Map diagnostics to intent routing predicates
diagnostic_active(Path, Line, Severity, Message) :-
    diagnostic(Severity, Path, Line, _, Message).

code_quality_issue(/diagnostic, Message) :-
    diagnostic(_, _, _, _, Message).
