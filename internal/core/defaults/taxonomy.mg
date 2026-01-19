


# =========================================================================
# CODE REVIEW & ANALYSIS (Reviewer)
# =========================================================================

# /review
verb_def(/review, /query, /reviewer, 100).
verb_synonym(/review, "review").
verb_synonym(/review, "code review").
verb_synonym(/review, "pr review").
verb_synonym(/review, "check code").
verb_synonym(/review, "audit").
verb_synonym(/review, "evaluate").
verb_synonym(/review, "critique").
# Regexes simplified to avoid backslash hell - using dot for wildcard
verb_pattern(/review, "(?i)review.*code").
verb_pattern(/review, "(?i)can.*you.*review").
verb_pattern(/review, "(?i)check.*(this|the).*file").

# /security
verb_def(/security, /query, /reviewer, 105).
verb_synonym(/security, "security").
verb_synonym(/security, "security scan").
verb_synonym(/security, "vulnerability").
verb_synonym(/security, "injection").
verb_synonym(/security, "xss").
verb_pattern(/security, "(?i)security.*scan").
verb_pattern(/security, "(?i)check.*for.*vuln").
verb_pattern(/security, "(?i)find.*vulnerabilities").

# /analyze
verb_def(/analyze, /query, /reviewer, 95).
verb_synonym(/analyze, "analyze").
verb_synonym(/analyze, "complexity").
verb_synonym(/analyze, "metrics").
verb_synonym(/analyze, "lint").
verb_synonym(/analyze, "code smell").
verb_pattern(/analyze, "(?i)analyze.*code").
verb_pattern(/analyze, "(?i)static.*analysis").

# =========================================================================
# UNDERSTANDING (Researcher/None)
# =========================================================================

# /explain
verb_def(/explain, /query, /none, 80).
verb_synonym(/explain, "explain").
verb_synonym(/explain, "describe").
verb_synonym(/explain, "what is").
verb_synonym(/explain, "how does").
verb_synonym(/explain, "help me understand").
verb_pattern(/explain, "(?i)explain.*this").
verb_pattern(/explain, "(?i)tell.*me.*about").
verb_pattern(/explain, "(?i)help.*understand").

# /explore
verb_def(/explore, /query, /researcher, 75).
verb_synonym(/explore, "explore").
verb_synonym(/explore, "browse").
verb_synonym(/explore, "show structure").
verb_synonym(/explore, "list files").
verb_pattern(/explore, "(?i)show.*structure").
verb_pattern(/explore, "(?i)explore.*codebase").

# /search
verb_def(/search, /query, /researcher, 85).
verb_synonym(/search, "search").
verb_synonym(/search, "find").
verb_synonym(/search, "grep").
verb_synonym(/search, "occurrences").
verb_pattern(/search, "(?i)search.*for").
verb_pattern(/search, "(?i)find.*all").
verb_pattern(/search, "(?i)grep").

# =========================================================================
# MUTATION (Coder)
# =========================================================================

# /fix
verb_def(/fix, /mutation, /coder, 90).
verb_synonym(/fix, "fix").
verb_synonym(/fix, "repair").
verb_synonym(/fix, "patch").
verb_synonym(/fix, "resolve").
verb_synonym(/fix, "bug fix").
verb_pattern(/fix, "(?i)fix.*bug").
verb_pattern(/fix, "(?i)repair.*this").
verb_pattern(/fix, "(?i)resolve.*issue").

# /refactor
verb_def(/refactor, /mutation, /coder, 88).
verb_synonym(/refactor, "refactor").
verb_synonym(/refactor, "clean up").
verb_synonym(/refactor, "improve").
verb_synonym(/refactor, "optimize").
verb_synonym(/refactor, "simplify").
verb_pattern(/refactor, "(?i)refactor").
verb_pattern(/refactor, "(?i)clean.*up").
verb_pattern(/refactor, "(?i)improve.*code").

# /create
verb_def(/create, /mutation, /coder, 85).
verb_synonym(/create, "create").
verb_synonym(/create, "new").
verb_synonym(/create, "add").
verb_synonym(/create, "implement").
verb_synonym(/create, "generate").
verb_pattern(/create, "(?i)create.*new").
verb_pattern(/create, "(?i)add.*new").
verb_pattern(/create, "(?i)implement").

# /write
verb_def(/write, /mutation, /coder, 70).
verb_synonym(/write, "write").
verb_synonym(/write, "save").
verb_synonym(/write, "export").
verb_pattern(/write, "(?i)write.*to").
verb_pattern(/write, "(?i)save.*to").

# /delete
verb_def(/delete, /mutation, /coder, 85).
verb_synonym(/delete, "delete").
verb_synonym(/delete, "remove").
verb_synonym(/delete, "drop").
verb_pattern(/delete, "(?i)delete").
verb_pattern(/delete, "(?i)remove").

# =========================================================================
# DEBUGGING (Coder)
# =========================================================================

# /debug
verb_def(/debug, /query, /coder, 92).
verb_synonym(/debug, "debug").
verb_synonym(/debug, "troubleshoot").
verb_synonym(/debug, "diagnose").
verb_synonym(/debug, "root cause").
verb_pattern(/debug, "(?i)debug").
verb_pattern(/debug, "(?i)troubleshoot").
verb_pattern(/debug, "(?i)why.*fail").

# =========================================================================
# TESTING (Tester)
# =========================================================================

# /test
verb_def(/test, /mutation, /tester, 88).
verb_synonym(/test, "test").
verb_synonym(/test, "unit test").
verb_synonym(/test, "run tests").
verb_synonym(/test, "coverage").
verb_pattern(/test, "(?i)write.*test").
verb_pattern(/test, "(?i)run.*test").
verb_pattern(/test, "(?i)test.*coverage").

# =========================================================================
# RESEARCH (Researcher)
# =========================================================================

# /research
verb_def(/research, /query, /researcher, 75).
verb_synonym(/research, "research").
verb_synonym(/research, "learn").
verb_synonym(/research, "docs").
verb_synonym(/research, "documentation").
verb_pattern(/research, "(?i)research").
verb_pattern(/research, "(?i)learn.*about").
verb_pattern(/research, "(?i)find.*docs").

# =========================================================================
# SETUP & CONFIG
# =========================================================================

# /init
verb_def(/init, /mutation, /researcher, 70).
verb_synonym(/init, "init").
verb_synonym(/init, "setup").
verb_synonym(/init, "bootstrap").
verb_pattern(/init, "(?i)^init").
verb_pattern(/init, "(?i)set.*up").

# /configure
verb_def(/configure, /instruction, /none, 65).
verb_synonym(/configure, "configure").
verb_synonym(/configure, "config").
verb_synonym(/configure, "settings").
verb_pattern(/configure, "(?i)configure").
verb_pattern(/configure, "(?i)change.*setting").

# =========================================================================
# CAMPAIGN
# =========================================================================

# /campaign
verb_def(/campaign, /mutation, /coder, 95).
verb_synonym(/campaign, "campaign").
verb_synonym(/campaign, "epic").
verb_synonym(/campaign, "feature").
verb_pattern(/campaign, "(?i)start.*campaign").
verb_pattern(/campaign, "(?i)implement.*feature").

# =========================================================================
# AUTOPOIESIS (Tool Generation)
# =========================================================================

# /generate_tool
verb_def(/generate_tool, /mutation, /tool_generator, 95).
verb_synonym(/generate_tool, "generate tool").
verb_synonym(/generate_tool, "create tool").
verb_synonym(/generate_tool, "need a tool").
verb_pattern(/generate_tool, "(?i)create.*tool").
verb_pattern(/generate_tool, "(?i)need.*tool").

# =========================================================================
# MULTI-STEP VERB TAXONOMY
# Encyclopedic definitions for multi-step task detection and decomposition
# =========================================================================

# --- Multi-Step Declarations ---
# Decl verb_composition imported from schemas_intent.mg
Decl step_connector(Connector, ConnectorType, StepBoundary).
Decl completion_marker(Marker, MarkerType).
Decl pronoun_ref(Pronoun, Resolution).
Decl constraint_marker(Marker, ConstraintType).

# --- Step Relations ---
# sequential: Verb2 depends on Verb1 completing
# parallel: Verb1 and Verb2 can run concurrently
# conditional: Verb2 runs only if Verb1 succeeds
# fallback: Verb2 runs only if Verb1 fails
# iterative: Verb1 repeats over a collection

# =========================================================================
# VERB COMPOSITIONS - Which verbs naturally follow each other
# =========================================================================

# --- Review-Then-Fix Compositions (highest priority) ---
verb_composition(/review, /fix, "sequential", 95).
verb_composition(/analyze, /fix, "sequential", 93).
verb_composition(/security, /fix, "sequential", 97).
verb_composition(/debug, /fix, "sequential", 94).
verb_composition(/review, /refactor, "sequential", 90).
verb_composition(/analyze, /refactor, "sequential", 88).

# --- Create-Then-Validate Compositions ---
verb_composition(/create, /test, "sequential", 92).
verb_composition(/fix, /test, "sequential", 94).
verb_composition(/refactor, /test, "sequential", 91).
verb_composition(/create, /review, "sequential", 85).
verb_composition(/fix, /review, "sequential", 86).

# --- Research-Then-Act Compositions ---
verb_composition(/research, /create, "sequential", 88).
verb_composition(/research, /fix, "sequential", 87).
verb_composition(/research, /refactor, "sequential", 86).
verb_composition(/explore, /create, "sequential", 85).
verb_composition(/explore, /refactor, "sequential", 84).

# --- Analysis-Then-Optimize Compositions ---
verb_composition(/analyze, /refactor, "sequential", 89).
verb_composition(/analyze, /fix, "sequential", 88).
verb_composition(/review, /optimize, "sequential", 87).

# --- Documentation Compositions ---
verb_composition(/create, /document, "sequential", 80).
verb_composition(/refactor, /document, "sequential", 79).
verb_composition(/fix, /document, "sequential", 78).

# --- Git Workflow Compositions ---
verb_composition(/fix, /commit, "sequential", 85).
verb_composition(/create, /commit, "sequential", 84).
verb_composition(/refactor, /commit, "sequential", 83).
verb_composition(/commit, /push, "sequential", 90).
verb_composition(/fix, /push, "sequential", 82).

# --- Parallel Analysis Compositions ---
verb_composition(/review, /security, "parallel", 75).
verb_composition(/review, /analyze, "parallel", 74).
verb_composition(/test, /lint, "parallel", 76).

# --- Conditional Compositions ---
verb_composition(/test, /commit, "conditional", 88).
verb_composition(/test, /push, "conditional", 87).
verb_composition(/test, /deploy, "conditional", 90).
verb_composition(/fix, /deploy, "conditional", 85).

# --- Fallback Compositions ---
verb_composition(/migrate, /rollback, "fallback", 85).
verb_composition(/deploy, /rollback, "fallback", 88).
verb_composition(/refactor, /revert, "fallback", 80).

# =========================================================================
# STEP CONNECTORS - Words that signal step boundaries
# =========================================================================

# --- Sequential Connectors (explicit ordering) ---
step_connector("first", "sequential_start", /true).
step_connector("then", "sequential_continue", /true).
step_connector("next", "sequential_continue", /true).
step_connector("after that", "sequential_continue", /true).
step_connector("afterwards", "sequential_continue", /true).
step_connector("afterward", "sequential_continue", /true).
step_connector("following that", "sequential_continue", /true).
step_connector("subsequently", "sequential_continue", /true).
step_connector("finally", "sequential_end", /true).
step_connector("lastly", "sequential_end", /true).
step_connector("start by", "sequential_start", /true).
step_connector("begin with", "sequential_start", /true).
step_connector("once done", "sequential_continue", /true).
step_connector("when done", "sequential_continue", /true).
step_connector("when finished", "sequential_continue", /true).
step_connector("after done", "sequential_continue", /true).
step_connector("after you're done", "sequential_continue", /true).
step_connector("when complete", "sequential_continue", /true).
step_connector("once complete", "sequential_continue", /true).

# --- Numbered Step Connectors ---
step_connector("step 1", "numbered", /true).
step_connector("step 2", "numbered", /true).
step_connector("step 3", "numbered", /true).
step_connector("step 4", "numbered", /true).
step_connector("step 5", "numbered", /true).
step_connector("1.", "numbered", /true).
step_connector("2.", "numbered", /true).
step_connector("3.", "numbered", /true).
step_connector("4.", "numbered", /true).
step_connector("5.", "numbered", /true).
step_connector("1)", "numbered", /true).
step_connector("2)", "numbered", /true).
step_connector("3)", "numbered", /true).
step_connector("first,", "numbered", /true).
step_connector("second,", "numbered", /true).
step_connector("third,", "numbered", /true).

# --- Implicit Sequential Connectors ---
step_connector("and", "implicit_sequential", /false).
step_connector("and then", "sequential_continue", /true).
step_connector("then also", "sequential_continue", /true).

# --- Parallel Connectors ---
step_connector("also", "parallel", /true).
step_connector("additionally", "parallel", /true).
step_connector("at the same time", "parallel", /true).
step_connector("simultaneously", "parallel", /true).
step_connector("in parallel", "parallel", /true).
step_connector("as well as", "parallel", /false).
step_connector("along with", "parallel", /false).
step_connector("together with", "parallel", /false).
step_connector("plus", "parallel", /false).

# --- Conditional Success Connectors ---
step_connector("if it works", "conditional_success", /true).
step_connector("if successful", "conditional_success", /true).
step_connector("if it passes", "conditional_success", /true).
step_connector("if tests pass", "conditional_success", /true).
step_connector("when tests pass", "conditional_success", /true).
step_connector("once tests pass", "conditional_success", /true).
step_connector("on success", "conditional_success", /true).
step_connector("assuming it works", "conditional_success", /true).
step_connector("provided it works", "conditional_success", /true).
step_connector("if no errors", "conditional_success", /true).
step_connector("if it compiles", "conditional_success", /true).
step_connector("if build succeeds", "conditional_success", /true).
step_connector("once tests are green", "conditional_success", /true).
step_connector("when green", "conditional_success", /true).

# --- Conditional Failure / Fallback Connectors ---
step_connector("if it fails", "conditional_failure", /true).
step_connector("if it doesn't work", "conditional_failure", /true).
step_connector("if it breaks", "conditional_failure", /true).
step_connector("otherwise", "conditional_failure", /true).
step_connector("or else", "conditional_failure", /true).
step_connector("on failure", "conditional_failure", /true).
step_connector("on error", "conditional_failure", /true).
step_connector("if errors occur", "conditional_failure", /true).
step_connector("if tests fail", "conditional_failure", /true).
step_connector("revert if fails", "conditional_failure", /true).
step_connector("rollback if fails", "conditional_failure", /true).
step_connector("undo if fails", "conditional_failure", /true).
step_connector("revert if needed", "conditional_failure", /true).
step_connector("if something goes wrong", "conditional_failure", /true).

# --- Pipeline Connectors ---
step_connector("pass the results to", "pipeline", /true).
step_connector("feed output to", "pipeline", /true).
step_connector("use the results to", "pipeline", /true).
step_connector("pipe to", "pipeline", /true).
step_connector("based on the results", "pipeline", /true).
step_connector("according to findings", "pipeline", /true).
step_connector("based on issues", "pipeline", /true).
step_connector("using the output", "pipeline", /true).

# =========================================================================
# COMPLETION MARKERS - Words that indicate task boundaries
# =========================================================================

completion_marker("done", "completion").
completion_marker("finished", "completion").
completion_marker("complete", "completion").
completion_marker("completed", "completion").
completion_marker("all done", "completion").
completion_marker("that's it", "completion").
completion_marker("verified", "verification").
completion_marker("confirmed", "verification").
completion_marker("tested", "verification").
completion_marker("working", "verification").
completion_marker("passes", "verification").
completion_marker("green", "verification").
completion_marker("ready", "readiness").
completion_marker("ready to", "readiness").
completion_marker("ready for", "readiness").

# =========================================================================
# PRONOUN REFERENCES - How pronouns resolve to targets
# =========================================================================

pronoun_ref("it", "previous_target").
pronoun_ref("them", "previous_targets").
pronoun_ref("this", "context_target").
pronoun_ref("that", "previous_target").
pronoun_ref("these", "previous_targets").
pronoun_ref("those", "previous_targets").
pronoun_ref("the file", "previous_file").
pronoun_ref("the files", "previous_files").
pronoun_ref("the code", "previous_code").
pronoun_ref("the function", "previous_function").
pronoun_ref("the changes", "previous_changes").
pronoun_ref("the results", "previous_output").
pronoun_ref("the output", "previous_output").
pronoun_ref("the findings", "previous_output").
pronoun_ref("the issues", "previous_issues").
pronoun_ref("the bugs", "previous_bugs").
pronoun_ref("the errors", "previous_errors").

# =========================================================================
# CONSTRAINT MARKERS - Words that modify scope
# =========================================================================

constraint_marker("but not", "exclusion").
constraint_marker("but skip", "exclusion").
constraint_marker("except", "exclusion").
constraint_marker("except for", "exclusion").
constraint_marker("excluding", "exclusion").
constraint_marker("without", "exclusion").
constraint_marker("don't touch", "exclusion").
constraint_marker("leave alone", "exclusion").
constraint_marker("skip", "exclusion").
constraint_marker("ignore", "exclusion").
constraint_marker("while keeping", "preservation").
constraint_marker("while preserving", "preservation").
constraint_marker("while maintaining", "preservation").
constraint_marker("preserving", "preservation").
constraint_marker("keeping", "preservation").
constraint_marker("maintaining", "preservation").
constraint_marker("without breaking", "preservation").
constraint_marker("without changing", "preservation").
constraint_marker("only", "inclusion").
constraint_marker("just", "inclusion").
constraint_marker("only the", "inclusion").
constraint_marker("just the", "inclusion").
constraint_marker("specifically", "inclusion").
constraint_marker("in particular", "inclusion").

# =========================================================================
# ITERATIVE MARKERS - Words that signal repetition
# =========================================================================

Decl iterative_marker(Marker, IterationType).

iterative_marker("each", "collection").
iterative_marker("every", "collection").
iterative_marker("all", "collection").
iterative_marker("all the", "collection").
iterative_marker("for each", "loop").
iterative_marker("for every", "loop").
iterative_marker("for all", "loop").
iterative_marker("one by one", "sequential_iteration").
iterative_marker("across all", "collection").
iterative_marker("throughout", "scope").
iterative_marker("everywhere", "scope").
iterative_marker("in all files", "file_scope").
iterative_marker("in all functions", "function_scope").
iterative_marker("in the entire", "full_scope").
iterative_marker("the whole", "full_scope").
iterative_marker("the entire", "full_scope").

# =========================================================================
# URGENCY/PRIORITY MARKERS
# =========================================================================

Decl priority_marker(Marker, PriorityLevel).

priority_marker("urgent", "high").
priority_marker("urgently", "high").
priority_marker("asap", "high").
priority_marker("immediately", "high").
priority_marker("right now", "high").
priority_marker("quickly", "medium").
priority_marker("soon", "medium").
priority_marker("when you can", "low").
priority_marker("eventually", "low").
priority_marker("at some point", "low").
priority_marker("critical", "critical").
priority_marker("blocking", "critical").
priority_marker("blocker", "critical").

# =========================================================================
# VERIFICATION MARKERS - Words that trigger verification steps
# =========================================================================

Decl verification_marker(Marker, VerificationType).

verification_marker("make sure", "verification_required").
verification_marker("ensure", "verification_required").
verification_marker("verify", "verification_required").
verification_marker("confirm", "verification_required").
verification_marker("check that", "verification_required").
verification_marker("validate", "verification_required").
verification_marker("test that", "test_verification").
verification_marker("run tests", "test_verification").
verification_marker("run the tests", "test_verification").
verification_marker("and test", "test_verification").
verification_marker("it works", "functional_verification").
verification_marker("it compiles", "build_verification").
verification_marker("it builds", "build_verification").
verification_marker("no errors", "error_verification").
verification_marker("no warnings", "warning_verification").

# =========================================================================
# INFERENCE RULES FOR MULTI-STEP DETECTION
# =========================================================================

# Check if two verbs can be composed
Decl can_compose(Verb1, Verb2).
can_compose(Verb1, Verb2) :-
    verb_composition(Verb1, Verb2, _, _).

# Get the default relation between two verbs
Decl verb_pair_relation(Verb1, Verb2, Relation).
verb_pair_relation(Verb1, Verb2, Relation) :-
    verb_composition(Verb1, Verb2, Relation, _).

# Check if a connector indicates a step boundary
Decl is_step_boundary(Connector).
is_step_boundary(Connector) :-
    step_connector(Connector, _, /true).

# Get connector type
Decl connector_type(Connector, Type).
connector_type(Connector, Type) :-
    step_connector(Connector, Type, _).

# Check if a word is an iterative marker
Decl is_iterative(Word).
is_iterative(Word) :-
    iterative_marker(Word, _).

# Check if a phrase needs verification
Decl needs_verification(Marker).
needs_verification(Marker) :-
    verification_marker(Marker, _).

# Resolve pronoun to reference type
Decl resolve_pronoun(Pronoun, RefType).
resolve_pronoun(Pronoun, RefType) :-
    pronoun_ref(Pronoun, RefType).
