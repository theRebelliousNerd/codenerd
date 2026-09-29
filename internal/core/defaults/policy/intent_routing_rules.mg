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

# ---------------------------------------------------------------------------
# Build/test command derivation — single projection from test_framework.
# The canonical derivation lives once in the leaf Go helper
# internal/tools/framework.go (TestFrameworkForDir/TestCommandForDir/
# BuildCommandForDir); both Go call sites (campaign checkpoints, shell
# run_tests/run_build) delegate to it instead of keeping their own tables.
# The test_framework facts above and the test_command/build_command rules
# below mirror that same mapping in policy. Do NOT add per-framework
# file_exists checks here and do NOT reintroduce Go detector tables.
# ---------------------------------------------------------------------------
Decl test_command(Command) bound [/string].
Decl build_command(Command) bound [/string].

test_command("go test ./...") :- test_framework(/go_test).
build_command("go build ./...") :- test_framework(/go_test).
test_command("cargo test") :- test_framework(/cargo_test).
build_command("cargo build") :- test_framework(/cargo_test).
test_command("pytest") :- test_framework(/pytest).
test_command("npm test") :- test_framework(/jest).
build_command("npm run build") :- test_framework(/jest).
test_command("npm test") :- test_framework(/vitest).
build_command("npm run build") :- test_framework(/vitest).
test_command("npm test") :- test_framework(/mocha).

# ---------------------------------------------------------------------------
# safe_action projection — permit test/build execution when a framework is
# detected. The constitution already lists /run_tests and /run_build
# unconditionally; re-deriving them here keeps intent_routing as the single
# routing source so a future allowlist prune cannot silently block the
# tester/coder loop. No Decl for safe_action exists (constitution.mg holds
# only facts), so deriving here cannot trigger a "declared more than once"
# analysis failure.
# ---------------------------------------------------------------------------
safe_action(/run_tests) :- test_framework(_).
safe_action(/run_build) :- test_framework(_).


# =============================================================================
# SECTION 4: Turn tool envelope
# =============================================================================
# The turn's tool catalog is this projection. Go used to keep a second copy in
# NewDefaultConfigAtomProvider; E1 measured the two against each other and
# zero of the 65 factory verbs matched, because persona/1 (section 2) defaults
# every non-specialist verb — including the read-only ones — to /coder, and
# the old tables then handed that persona /bash, /run_command and /run_check.
# Those three are registered and the old rules granted them. The factory never
# offered them, and the repo contract forbids offering a free-form shell by
# default, so they are absent here on purpose. Parity is the gate: no verb
# gains a tool the factory withheld, and every tool the factory offered is
# named below. persona/1 and verb_has_specialist stay; delegation, campaign
# and prompt_northstar still read them. They do not decide this catalog.
#
# search_expand rides with search_code. search_code elides the matching lines
# and returns a handle; without the redemption verb that handle is a promise
# the model cannot keep. subagent_expand is on every persona for a sharper
# reason: a subagent-return handle is minted by a delegation and arrives on a
# turn that never ran one, so there is no narrower catalog to pair it with.
# The MCP surface is five fixed verbs on every persona. Per-remote-tool blast
# radius is mcp_tool_gated in policy_mcp.mg, not this catalog; scoping the
# five by verb left configured servers unreachable from whole regions of the
# taxonomy, which fails silently. apply_edits was registered and taught while
# missing from every catalog until 2026-09-22, so it sits with the other
# CodeDOM edits. The researcher gets the read half of that surface because
# campaign grounding ran on this persona with grep as its only cross-file
# tool. The reviewer keeps the edit half too: that is the factory envelope
# (it does not include edit_file or write_file). Narrowing it is a separate
# decision, not a side effect of moving the catalog.
#
# /explain, /read and the other ShardType /none verbs carry the core set.
# They drifted off the factory once, an unregistered verb resolved to zero
# tools, and `nerd explain <file>` answered "reading the file now" and exited
# 0. An unknown verb gets the same floor (the last rule). An empty derivation
# is a broken projection, and the Go consumer fail-closes the turn rather
# than treating "no tools" as "all tools".

Decl envelope_tool(Group, Tool) bound [/name, /name].
Decl persona_envelope(Persona, Group) bound [/name, /name].
Decl verb_persona(Verb, Persona) bound [/name, /name].
Decl verb_has_persona(Verb) bound [/name].
Decl turn_tool_allowed(Verb, Tool) bound [/name, /name].
# One fact per tool a user agent (.nerd/agents.json) declared. The host
# asserts these at registration, and only for a tool it has already seen
# registered or already inside a persona envelope. Not a verb_persona:
# these agents are not specialists the delegation rules know.
Decl user_agent_declared_tool(Verb, Tool) bound [/name, /name].

# --- /core: every persona. Read, recall, search, list, and the MCP plane. ---
envelope_tool(/core, /recall_context).
envelope_tool(/core, /read_file).
envelope_tool(/core, /search_code).
envelope_tool(/core, /search_expand).
envelope_tool(/core, /subagent_expand).
envelope_tool(/core, /list_files).
envelope_tool(/core, /glob).
envelope_tool(/core, /grep).
envelope_tool(/core, /mcp_map).
envelope_tool(/core, /mcp_probe).
envelope_tool(/core, /mcp_call).
envelope_tool(/core, /mcp_expand).
envelope_tool(/core, /mcp_context).

# --- /codedom: structural reads and the edits that replace a line edit. ---
envelope_tool(/codedom, /find_symbol).
envelope_tool(/codedom, /package_outline).
envelope_tool(/codedom, /callers_of).
envelope_tool(/codedom, /callees_of).
envelope_tool(/codedom, /unreferenced_symbols).
envelope_tool(/codedom, /importers_of).
envelope_tool(/codedom, /find_text).
envelope_tool(/codedom, /predicate_outline).
envelope_tool(/codedom, /get_elements).
envelope_tool(/codedom, /get_element).
envelope_tool(/codedom, /edit_element).
envelope_tool(/codedom, /replace_element).
envelope_tool(/codedom, /insert_element).
envelope_tool(/codedom, /delete_element).
envelope_tool(/codedom, /create_file).
envelope_tool(/codedom, /repoint).
envelope_tool(/codedom, /edit_lines).
envelope_tool(/codedom, /insert_lines).
envelope_tool(/codedom, /delete_lines).
envelope_tool(/codedom, /apply_edits).

# --- /codedom_read: the read half, for the researcher. ---
envelope_tool(/codedom_read, /find_symbol).
envelope_tool(/codedom_read, /package_outline).
envelope_tool(/codedom_read, /callers_of).
envelope_tool(/codedom_read, /callees_of).
envelope_tool(/codedom_read, /unreferenced_symbols).
envelope_tool(/codedom_read, /importers_of).
envelope_tool(/codedom_read, /find_text).
envelope_tool(/codedom_read, /predicate_outline).
envelope_tool(/codedom_read, /get_elements).
envelope_tool(/codedom_read, /get_element).

envelope_tool(/impact, /get_impacted_tests).
envelope_tool(/impact, /run_impacted_tests).

# The eight-verb browser session. Screenshot, click, type, close and audit
# are registered and the old modular rules granted them to /research; the
# factory never did, so they stay out.
envelope_tool(/browser_session, /browser_observe).
envelope_tool(/browser_session, /browser_act).
envelope_tool(/browser_session, /browser_mangle).
envelope_tool(/browser_session, /browser_wait).
envelope_tool(/browser_session, /browser_reason).
envelope_tool(/browser_session, /browser_evidence).
envelope_tool(/browser_session, /browser_specs).
envelope_tool(/browser_session, /browser_test).

persona_envelope(/general, /core).

persona_envelope(/coder, /core).
persona_envelope(/coder, /codedom).
persona_envelope(/coder, /impact).

persona_envelope(/tester, /core).
persona_envelope(/tester, /codedom).
persona_envelope(/tester, /impact).
persona_envelope(/tester, /browser_session).

persona_envelope(/reviewer, /core).
persona_envelope(/reviewer, /codedom).

persona_envelope(/researcher, /core).
persona_envelope(/researcher, /codedom_read).
persona_envelope(/researcher, /browser_session).

persona_envelope(/nemesis, /core).
persona_envelope(/nemesis, /codedom).

persona_envelope(/tool_generator, /core).

# A persona's envelope is the groups it includes plus these extras. The
# groups are facts so the only rule head is persona_tool_allowed, which
# turn_tool_allowed reads.
persona_tool_allowed(P, T) :- persona_envelope(P, G), envelope_tool(G, T).

# Coder extras. run_tests is here because the factory offered it; the old
# persona table did not. bash, run_command and run_check are not.
persona_tool_allowed(/coder, /write_file).
persona_tool_allowed(/coder, /edit_file).
persona_tool_allowed(/coder, /delete_file).
persona_tool_allowed(/coder, /run_build).
persona_tool_allowed(/coder, /run_tests).
persona_tool_allowed(/coder, /git_operation).

# Tester extras. No delete_file, run_build, git_operation, or free-form shell.
persona_tool_allowed(/tester, /run_tests).
persona_tool_allowed(/tester, /write_file).
persona_tool_allowed(/tester, /edit_file).

persona_tool_allowed(/reviewer, /git_diff).
persona_tool_allowed(/reviewer, /git_log).

# Researcher extras. browser_navigate and browser_extract sit beside the
# session verbs; the cache is get and set only (stats and clear are
# registered, and the factory did not offer them). git_diff and git_log
# were universal in the old modular rules and are reviewer-only here.
persona_tool_allowed(/researcher, /context7_fetch).
persona_tool_allowed(/researcher, /web_search).
persona_tool_allowed(/researcher, /grounded_web_search).
persona_tool_allowed(/researcher, /web_fetch).
persona_tool_allowed(/researcher, /browser_navigate).
persona_tool_allowed(/researcher, /browser_extract).
persona_tool_allowed(/researcher, /research_cache_get).
persona_tool_allowed(/researcher, /research_cache_set).
persona_tool_allowed(/researcher, /write_file).

persona_tool_allowed(/nemesis, /run_build).
persona_tool_allowed(/nemesis, /run_tests).
persona_tool_allowed(/nemesis, /write_file).

persona_tool_allowed(/tool_generator, /write_file).
persona_tool_allowed(/tool_generator, /run_build).
persona_tool_allowed(/tool_generator, /run_tests).

# --- verb to persona. One fact per factory intent, including the aliases
# perception has historically emitted. /generate-tool is not a fact: a
# Mangle atom cannot spell a hyphen, and the consumer rewrites that one
# alias to /generate_tool before it queries. /consult/<name> is rewritten
# to /<name> the same way. ---

verb_persona(/fix, /coder).
verb_persona(/refactor, /coder).
verb_persona(/create, /coder).
verb_persona(/write, /coder).
verb_persona(/delete, /coder).
verb_persona(/debug, /coder).
verb_persona(/campaign, /coder).
verb_persona(/git, /coder).
verb_persona(/migrate, /coder).
verb_persona(/optimize, /coder).
verb_persona(/document, /coder).
verb_persona(/scaffold, /coder).
verb_persona(/format, /coder).
verb_persona(/deploy, /coder).
verb_persona(/implement, /coder).
verb_persona(/modify, /coder).
verb_persona(/add, /coder).
verb_persona(/update, /coder).

verb_persona(/test, /tester).
verb_persona(/benchmark, /tester).
verb_persona(/profile, /tester).
verb_persona(/cover, /tester).

verb_persona(/verify, /tester).
verb_persona(/validate, /tester).

verb_persona(/review, /reviewer).
verb_persona(/review_enhance, /reviewer).
verb_persona(/security, /reviewer).
verb_persona(/analyze, /reviewer).
verb_persona(/audit, /reviewer).
verb_persona(/lint, /reviewer).
verb_persona(/check, /reviewer).
verb_persona(/inspect, /reviewer).

verb_persona(/explore, /researcher).
verb_persona(/search, /researcher).
verb_persona(/research, /researcher).
verb_persona(/init, /researcher).
verb_persona(/learn, /researcher).
verb_persona(/understand, /researcher).
verb_persona(/find, /researcher).

verb_persona(/attack, /nemesis).
verb_persona(/break, /nemesis).
verb_persona(/exploit, /nemesis).
verb_persona(/fuzz, /nemesis).
verb_persona(/pentest, /nemesis).
verb_persona(/nemesis, /nemesis).

verb_persona(/generate_tool, /tool_generator).
verb_persona(/generate, /tool_generator).
verb_persona(/tool_generator, /tool_generator).
verb_persona(/create_tool, /tool_generator).

verb_persona(/general, /general).
verb_persona(/explain, /general).
verb_persona(/read, /general).
verb_persona(/stats, /general).
verb_persona(/knowledge, /general).
verb_persona(/help, /general).
verb_persona(/greet, /general).
verb_persona(/configure, /general).
verb_persona(/dream, /general).
verb_persona(/shadow, /general).
verb_persona(/assault, /general).
verb_persona(/converse, /general).
verb_persona(/forget, /general).
verb_persona(/remember, /general).
verb_persona(/requirements_interrogator, /general).

verb_has_persona(V) :- verb_persona(V, _).

# The catalog for a known verb is its persona's tools. Static: it does not
# consult persona/1, so a read-only verb cannot inherit /coder.
turn_tool_allowed(V, T) :- verb_persona(V, P), persona_tool_allowed(P, T).

# /verify and /validate are the tester envelope plus grounded search, and
# nothing else the old modular /verify rules added (no browser_navigate,
# no browser_extract). The two facts are the whole difference.
turn_tool_allowed(/verify, /grounded_web_search).
turn_tool_allowed(/validate, /grounded_web_search).

# A verb with no persona fact, seen on a real turn, gets the core floor.
# V is bound by user_intent before the negation. The Go consumer asks
# verb_has_persona first and reads /general itself when the answer is no,
# because the spawner compiles a config without asserting user_intent;
# both paths name the same floor, and a persona-bearing verb that derives
# nothing is a broken projection rather than a silent widening.
turn_tool_allowed(V, T) :-
    user_intent(_, _, V, _, _),
    persona_tool_allowed(/general, T),
    !verb_has_persona(V).

# A user agent's declared tools, plus the same /general floor an unknown
# verb gets. DeriveTurnTools returns the first non-empty derivation and
# does not union /general afterwards, so a specialist that declared
# go_build would lose read_file if the floor were left to the Go fallback.
# The fallback still covers an agent that declared nothing: it has no
# user_agent_declared_tool fact, verb_has_persona stays false, and Go
# reads /general itself.
turn_tool_allowed(V, T) :-
    user_agent_declared_tool(V, T).

turn_tool_allowed(V, T) :-
    user_agent_declared_tool(V, _),
    persona_tool_allowed(/general, T).

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

# Medium priority: files in the same directory as the target.
# Target is bound; file_dir binds the directory, then the other file.
context_priority(Path, 70) :-
    user_intent(_, _, _, Target, _),
    file_dir(Target, D),
    file_dir(Path, D),
    Target != Path,
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
