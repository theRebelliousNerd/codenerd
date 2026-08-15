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
# Decl for file_contains intentionally omitted: internal/core/defaults declares it
# identically, and a second Decl makes the whole program fail analysis with
# "declared more than once" — which is why this file could never be loaded
# into the kernel alongside the constitution.
# Decl file_imports(Importer, Imported) - From schemas_codedom_polyglot.mg
# Decl for file_imports intentionally omitted: internal/core/defaults declares it
# identically, and a second Decl makes the whole program fail analysis with
# "declared more than once" — which is why this file could never be loaded
# into the kernel alongside the constitution.

# Internal predicates defined only in this file (or missing from defaults)
# These declarations ensure standalone validation works correctly
# Decl for same_package intentionally omitted: internal/core/defaults declares it
# identically, and a second Decl makes the whole program fail analysis with
# "declared more than once" — which is why this file could never be loaded
# into the kernel alongside the constitution.
# Decl for diagnostic intentionally omitted: internal/core/defaults declares it
# identically, and a second Decl makes the whole program fail analysis with
# "declared more than once" — which is why this file could never be loaded
# into the kernel alongside the constitution.
# Decl for pytest_failure intentionally omitted: internal/core/defaults declares it
# identically, and a second Decl makes the whole program fail analysis with
# "declared more than once" — which is why this file could never be loaded
# into the kernel alongside the constitution.

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
persona_tool_allowed(P, /read_file) :- persona(P).
persona_tool_allowed(P, /search_code) :- persona(P).
persona_tool_allowed(P, /list_files) :- persona(P).
persona_tool_allowed(P, /glob) :- persona(P).
persona_tool_allowed(P, /grep) :- persona(P).

# Code DOM tools - available to all personas for semantic code navigation
persona_tool_allowed(P, /get_elements) :- persona(P).
persona_tool_allowed(P, /get_element) :- persona(P).

# Coder-specific tools
persona_tool_allowed(/coder, /write_file).
persona_tool_allowed(/coder, /edit_file).
persona_tool_allowed(/coder, /delete_file).
persona_tool_allowed(/coder, /run_build).
persona_tool_allowed(/coder, /run_command).
persona_tool_allowed(/coder, /bash).
persona_tool_allowed(/coder, /git_operation).
persona_tool_allowed(/coder, /edit_lines).
persona_tool_allowed(/coder, /insert_lines).
persona_tool_allowed(/coder, /delete_lines).

# Tester-specific tools
persona_tool_allowed(/tester, /run_tests).
persona_tool_allowed(/tester, /run_command).
persona_tool_allowed(/tester, /bash).
persona_tool_allowed(/tester, /write_file).  # Can write test files
persona_tool_allowed(/tester, /edit_file).
persona_tool_allowed(/tester, /edit_lines).
persona_tool_allowed(/tester, /insert_lines).
persona_tool_allowed(/tester, /delete_lines).
persona_tool_allowed(/tester, /get_impacted_tests).
persona_tool_allowed(/tester, /run_impacted_tests).

# Coder can also use test impact tools
persona_tool_allowed(/coder, /get_impacted_tests).
persona_tool_allowed(/coder, /run_impacted_tests).

# Reviewer-specific tools (read-heavy)
persona_tool_allowed(/reviewer, /git_diff).
persona_tool_allowed(/reviewer, /git_log).
persona_tool_allowed(/reviewer, /run_command).  # For static analysis tools

# Researcher-specific tools
persona_tool_allowed(/researcher, /web_search).
persona_tool_allowed(/researcher, /web_fetch).
persona_tool_allowed(/researcher, /context7_fetch).
persona_tool_allowed(/researcher, /write_file).  # Can write documentation
persona_tool_allowed(/researcher, /grounded_web_search).

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

# Transactional multi-file edit - a code mutation, same envelope as edit_lines
modular_tool_allowed(/apply_edits, Intent) :- verb_category(Intent, /code).

# Git tools. shell.RegisterAll has registered git_diff, git_log and
# git_operation since the package was split out, and none of them appeared
# here, so the Mangle catalog and the Go registry disagreed about what exists.
#
# Read-only history is available wherever reading a file is: reviewing a diff
# is how an agent orients, and both refuse to leave the workspace.
modular_tool_allowed(/git_diff, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/git_log, Intent) :- user_intent(_, _, Intent, _, _).

# git_operation mutates the repository (add/commit/checkout/push/reset), so it
# is scoped to the intents that are allowed to change the working tree. The
# constitution still gates the individual operation; this only decides which
# intents may reach the tool at all.
modular_tool_allowed(/git_operation, Intent) :- verb_category(Intent, /code).
modular_tool_allowed(/git_operation, Intent) :- verb_category(Intent, /git).

verb_category(/git, /git) :- user_intent(_, _, /git, _, _).
verb_category(/commit, /git) :- user_intent(_, _, /commit, _, _).

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
modular_tool_allowed(/browser_observe, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_act, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_mangle, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_wait, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_reason, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_evidence, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_specs, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/browser_test, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/research_cache_get, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/research_cache_set, Intent) :- verb_category(Intent, /research).
# research_cache_stats is read-only bookkeeping and belongs everywhere the
# cache itself is reachable: an agent that can Get/Set but cannot see the hit
# rate re-fetches pages it already has.
modular_tool_allowed(/research_cache_stats, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/research_cache_stats, Intent) :- verb_category(Intent, /learn).
modular_tool_allowed(/research_cache_stats, Intent) :- verb_category(Intent, /document).
modular_tool_allowed(/research_cache_stats, Intent) :- verb_category(Intent, /verify).
# research_cache_clear discards work every other agent in the process shares —
# the cache is a package-level singleton — so it stays confined to /research,
# where the agent that filled it is the agent that empties it.
modular_tool_allowed(/research_cache_clear, Intent) :- verb_category(Intent, /research).
# Provider-native grounded search is restricted to research and verification.
modular_tool_allowed(/grounded_web_search, Intent) :- verb_category(Intent, /research).
modular_tool_allowed(/grounded_web_search, Intent) :- verb_category(Intent, /verify).

# Context7 also available for /learn and /document intents
modular_tool_allowed(/context7_fetch, Intent) :- verb_category(Intent, /learn).
modular_tool_allowed(/context7_fetch, Intent) :- verb_category(Intent, /document).

# Browser tools also available for verification intents
modular_tool_allowed(/browser_navigate, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_extract, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_screenshot, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_observe, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_act, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_mangle, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_wait, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_reason, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_evidence, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_specs, Intent) :- verb_category(Intent, /verify).
modular_tool_allowed(/browser_test, Intent) :- verb_category(Intent, /verify).

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
modular_tool_priority(/browser_observe, 70).
modular_tool_priority(/browser_act, 65).
modular_tool_priority(/browser_mangle, 72).
modular_tool_priority(/browser_wait, 71).
modular_tool_priority(/browser_reason, 73).
modular_tool_priority(/browser_evidence, 74).
modular_tool_priority(/browser_specs, 75).
modular_tool_priority(/browser_test, 76).

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
#
# any_test_failed projects the wildcard away: a negated literal containing an
# anonymous wildcard excludes nothing in this Mangle build (proved in
# internal/core/bound_negation_test.go), so `!test_failed(_, _, _)` derived
# /green even with failing tests — and /red and /green held simultaneously.
Decl any_test_failed(Flag).
any_test_failed(/yes) :- test_failed(Path, TestName, Reason).

tdd_state(/red) :- test_failed(_, _, _), !test_passed_after_fix().
tdd_state(/green) :- !any_test_failed(/yes), code_modified_recently().
tdd_state(/refactor) :- tdd_state(/green), code_quality_issue(_, _).

# Next action derivation for TDD
next_action(/run_tests) :- tdd_state(/green), !tests_run_recently().

# Three further TDD next_action rules are deliberately absent here.
#
# This file was unreachable by the kernel until it moved into defaults/policy/
# (no embed pattern covered internal/mangle/), so nothing in it had ever
# derived. Making it live turns each derived next_action into a plan the
# executor is asked to carry out. The rule above survives because /run_tests
# maps to ActionRunTests; the three that were dropped named actions with no
# VirtualStore route at all, so the kernel would have handed the agent a next
# action nothing could execute — worse than the silence they produced while the
# file was dead. cmd/tools/action_linter reports exactly this as "policy emits
# action but router has no matching route".
#
# The TDD loop they belong to is real, so they should return once their
# executors exist. They are described rather than left commented out because
# the linter's .mg scanner does not strip # comments, and a commented rule is
# still counted as emitted.
#
# Dropped, pending executors: the red state's fix action, the refactor state's
# refactor action, and the generic execute-intent fallback for the case where
# no TDD state holds. Restore them next to tdd_state above; the linter will
# confirm the routes exist.

# =============================================================================
# SECTION 9: Wired Predicates (Improvement)
# =============================================================================
# Wiring for predicates that were previously declared but unconnected

# Derive code modification from execution history
code_modified_recently() :- file_edited(_).

# Derive recent test execution
tests_run_recently() :- action_verified(_, /run_tests, _, _, _).

# Derive test success (heuristic: >=80% confidence verification on test run)
test_passed_after_fix() :- action_verified(_, /run_tests, _, Confidence, _), Confidence >= 80.

# Map diagnostics to intent routing predicates
diagnostic_active(Path, Line, Severity, Message) :-
    diagnostic(Severity, Path, Line, _, Message).

code_quality_issue(/diagnostic, Message) :-
    diagnostic(_, _, _, _, Message).

# Map test failures to generic test_failed predicate
# Used for TDD loop state transitions (/red state)
test_failed(Path, Name, Msg) :- pytest_failure(Name, _, Path, _, Msg).

# Map file imports to context scope
imports(Target, Path) :- file_imports(Target, Path).
