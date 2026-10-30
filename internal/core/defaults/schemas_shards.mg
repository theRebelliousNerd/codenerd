# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: SHARDS
# Sections: 6, 30, 31, 32, 33

# =============================================================================
# SECTION 6: SHARD DELEGATION (§7.0)
# =============================================================================

# delegate_task(ShardType, TaskDescription, Status)
# ShardType: /researcher, /coder, /reviewer, /tester, /generalist, /specialist
# Status: /pending, /in_progress, /completed, /failed
Decl delegate_task(ShardType, TaskDescription, Status) bound [/name, /string, /name].

# delegation_candidate(IntentID, ShardType, Confidence) - EDB asserted by Go.
# Step 4: Go computes the verb->shard lookup (GetShardTypeForVerb, which reads
# the perception taxonomy corpus that is intentionally siloed from this kernel)
# and the perception confidence, then passes BOTH into the kernel as a fact so
# the DELEGATION DECISION (the confidence gate) can live in policy. ShardType is
# /none when no shard maps to the verb. Confidence is a 0-100 integer (Go scales
# the 0.0-1.0 float, matching the action_verified convention).
Decl delegation_candidate(IntentID, ShardType, Confidence) bound [/name, /name, /number].

# should_delegate(ShardType) - derived: the current intent passes the
# delegation gate for ShardType (confidence at or above
# routing.delegation_min_confidence). Read through route_decision; Go has no
# copy of the gate.
Decl should_delegate(ShardType) bound [/name].

# delegation_persona(Candidate, Persona) - derived: the persona that takes the
# turn for a delegation candidate. A /query whose candidate's persona_verb is
# write-oriented goes to /reviewer; every other candidate is its own persona.
Decl delegation_persona(Candidate, Persona) bound [/name, /name].
# query_needs_reader(Candidate) - derived: the current /query would run as a
# write if handed to Candidate.
Decl query_needs_reader(Candidate) bound [/name].

# -----------------------------------------------------------------------------
# 6.0.1 Routing Arbitration (single decision point per turn)
# Rules live in policy/routing_arbitration.mg. Go asserts the per-turn EDB
# (user_intent, intent_signal, delegation_candidate, multi_step_signal), queries
# route_decision once, and executes the one lane that derived, or none. This replaces the old
# scattered Go gates (follow-up substring detector, stability bypass, ad-hoc
# confidence booleans) that made identical inputs route differently depending
# on hidden session state.
# -----------------------------------------------------------------------------

# route_decision(Route, ShardType) - derived: the single routing outcome for
# the current turn. Route: /perception_answer, /dream, /assault,
# /respond_directly, /clarify, /multi_step, /delegate, /hold,
# /approve_pending, /gather_evidence.
# ShardType carries the target shard for /delegate and is /none otherwise.
Decl route_decision(Route, ShardType) bound [/name, /name].

# route_verifies() - derived: the /delegate lane runs the verification loop
# (the persona's attempts, each judged) rather than one plain run.
Decl route_verifies() bound [].

# wants_direct_answer() - derived: the user is asking for an answer (question or
# conversation), so the turn must terminate in prose, not shard work.
Decl wants_direct_answer() bound [].

# needs_workspace_evidence() - derived: the turn names a concrete file or
# perception named tools it needs, so the answer must be read from the
# workspace, not produced from prose knowledge.
Decl needs_workspace_evidence() bound [].

# -----------------------------------------------------------------------------
# 6.0.2 Chat delegation attempts (policy/delegation.mg)
# A chat request handed to a persona runs attempts until the kernel accepts
# one or the persona's cap is reached. Go asserts, per delegation Root: the
# request, each attempt's turn verdict (the executor's kernel verdict, carried
# back typed), and the LLM judge's verdict on an attempt that ended done; it
# asks delegation_move. Retracted when the delegation ends.
# -----------------------------------------------------------------------------

# persona_verb(Persona, Verb) - the verb a persona's turn runs: the one persona
# table (policy/delegation.mg).
Decl persona_verb(Persona, Verb) bound [/name, /name].
# verb_isolated(Verb) - a verb whose turn runs as an isolated subagent.
Decl verb_isolated(Verb) bound [/name].

# delegation_request(Root, Persona, Cap) - the persona and its attempt cap
# (shard_profiles.<persona>.max_retries).
Decl delegation_request(Root, Persona, Cap) bound [/string, /name, /number].
# delegation_attempt(Root, Attempt, Outcome) - the attempt's turn verdict:
# /done, /unverified, /hollow, /failed, or /none when the turn returned none.
Decl delegation_attempt(Root, Attempt, Outcome) bound [/string, /number, /name].
# judge_verdict(Root, Attempt, Verdict, Confidence) - the LLM judge on a done
# attempt, held to the judge/v2 reply contract and grounded in the attempt's
# evidence packet: /pass, /defect or /insufficient_evidence, with its
# confidence as a percent. A reply that broke the contract asserts nothing.
Decl judge_verdict(Root, Attempt, Verdict, Confidence) bound [/string, /number, /name, /number].
# judge_round(Root, Attempt, Round) - each round the judge was asked about the
# attempt, from 1; a later round supplies evidence an earlier one requested.
Decl judge_round(Root, Attempt, Round) bound [/string, /number, /number].
# judge_evidence_requested(Root, Attempt) - the latest round requested packet
# evidence the host can supply.
Decl judge_evidence_requested(Root, Attempt) bound [/string, /number].
# judge_evidence_applicability(Root, Attempt, Applicability) - the host's
# measurement, after the latest judge round, of whether the attempt's evidence
# still describes the workspace: /current, /superseded or /unknown.
Decl judge_evidence_applicability(Root, Attempt, Applicability) bound [/string, /number, /name].
Decl judge_evidence_current(Root, Attempt) bound [/string, /number].
Decl judge_evidence_historical(Root, Attempt) bound [/string, /number].
Decl judge_unresolved(Root, Attempt) bound [/string, /number].
Decl judge_last_round(Root, Attempt, Round) bound [/string, /number, /number].
Decl judge_may_acquire(Root, Attempt) bound [/string, /number].
Decl delegation_attempt_failed(Root, Attempt) bound [/string, /number].
Decl delegation_judge_due(Root, Attempt) bound [/string, /number].
Decl analysis_persona(Persona) bound [/name].
Decl delegation_judge_rubric(Root, Rubric) bound [/string, /name].
Decl judge_rejects(Root, Attempt) bound [/string, /number].
Decl delegation_attempt_accepted(Root, Attempt) bound [/string, /number].
Decl delegation_attempt_count(Root, N) bound [/string, /number].
# delegation_move(Root, Attempt, Move) - derived: /accept, /retry, /escalate,
# /acquire_evidence (another judge round over the same packet) or
# /unverifiable (the judge cannot settle it; fail closed, no worker rerun).
Decl delegation_move(Root, Attempt, Move) bound [/string, /number, /name].

# -----------------------------------------------------------------------------
# 6.0.3 Which file a fixer is sent to, and which topology a verb runs
# (policy/delegation.mg). Go asserts the citations and the verb being routed.
# The rank order and the topology are facts and rules, not a Go switch.
# -----------------------------------------------------------------------------

# review_finding_citation(ReviewID, File, Severity, Index) - EDB. One row per
# review finding that names a file. Index is that finding's position in the
# review, so two files cannot share an index. Severity is /critical, /high,
# /medium, /low, or /unknown.
Decl review_finding_citation(ReviewID, File, Severity, Index) bound [/name, /string, /name, /number].

# severity_rank(Severity, Rank) - the reviewer's ladder as numbers. /unknown
# is 0, below /low: a word the ladder does not name is missing information.
Decl severity_rank(Severity, Rank) bound [/name, /number].

Decl citation_ranked(ReviewID, File, Rank, Index) bound [/name, /string, /number, /number].
Decl file_citation_summary(ReviewID, File, Count, MaxRank, MinIndex) bound [/name, /string, /number, /number, /number].
Decl citation_count_value(ReviewID, Count) bound [/name, /number].
Decl best_citation_count(ReviewID, Best) bound [/name, /number].
Decl count_winner(ReviewID, File, MaxRank, MinIndex) bound [/name, /string, /number, /number].
Decl citation_rank_value(ReviewID, MaxRank) bound [/name, /number].
Decl best_citation_rank(ReviewID, Best) bound [/name, /number].
Decl rank_winner(ReviewID, File, MinIndex) bound [/name, /string, /number].
Decl citation_index_value(ReviewID, MinIndex) bound [/name, /number].
Decl best_citation_index(ReviewID, Best) bound [/name, /number].

# delegation_target_file(ReviewID, File) - derived: the one file the fixer is
# sent to. Most citations, then the worst severity among those, then the
# earliest index. The index is unique per citation, so exactly one file wins.
Decl delegation_target_file(ReviewID, File) bound [/name, /string].

# configured_execution_mode(Verb, Mode) - the coordination topology for a verb
# that has one. Mode: /parallel, /advisory, /advisory_with_critique.
Decl configured_execution_mode(Verb, Mode) bound [/name, /name].
Decl has_configured_execution_mode(Verb) bound [/name].

# asked_delegation_verb(Verb) - EDB: the verb spawnShardWithSpecialists is
# routing. Binds the default so a verb with no configured row still derives.
Decl asked_delegation_verb(Verb) bound [/name].

# execution_mode(Verb, Mode) - derived: the topology for Verb. A verb that was
# asked and has no configured row derives /parallel, which is what the Go map
# returned on a miss.
Decl execution_mode(Verb, Mode) bound [/name, /name].

# multi_step_lane() - derived: the turn decomposes (a multi-step mutation that
# is not answered directly). Negated by the delegate and clarify lanes, so
# route_decision holds for one lane at most.
Decl multi_step_lane() bound [].

# multi_step_plan_ready() - derived: the decomposition (multi_step_plan_step)
# has two steps or more and each names a shard. multi_step_lane requires it.
Decl multi_step_plan_ready() bound [].

# multi_step_plan_unrouted() - derived: a step of the decomposition names no
# shard.
Decl multi_step_plan_unrouted() bound [].

# conversational_verb(Verb) - vocabulary: verbs whose outcome is always prose.
Decl conversational_verb(Verb) bound [/name].

# workhorse_verb(Verb) - vocabulary: verbs that delegate to a shard even when
# the request is phrased as a question ("can you review my code?").
Decl workhorse_verb(Verb) bound [/name].

# The early lanes (routing_arbitration.mg): perception already answered, a
# hypothetical to consult on, or an adversarial campaign on the workspace.
# Each excludes every later lane.
Decl perception_answer_verb(Verb) bound [/name].
Decl perception_answer_target(Verb, Target) bound [/name, /string].
Decl perception_answer_lane() bound [].
Decl dream_lane() bound [].
Decl assault_lane() bound [].
Decl early_lane() bound [].

# The clarify lane: needs_clarification says why a turn must ask first
# (uncertain, untargeted, or a question the kernel already has); clarify_lane
# applies the precedence and the loop guard, and the delegate lane yields to it.
Decl clarify_lane() bound [].

# Work admission (routing_arbitration.mg). work_mode(Mode) is the chat's
# autonomy switch (/yolo, /propose, /explicit; config work_mode);
# work_proposal_pending() is asserted by chat while held work awaits a go;
# intent_signal(/user_go_ahead) marks the turn that carries the go-ahead.
Decl work_mode(Mode) bound [/name].
# route_background(): the delegation only reads, so it runs off the turn
# (chat background.go); background_job(JobID, Shard, Task, Status) is one
# such job, Status /running, /done or /failed.
Decl route_background() bound [].
Decl background_job(JobID, Shard, Task, Status) bound [/string, /string, /string, /name].
# background_job_progress(JobID, Text): one line per agent working on a running
# background job -- its elapsed time, model calls, tool results, last tool and
# what it is waiting on -- from the transcripts the harness writes. Refreshed
# every chat turn.
Decl background_job_progress(JobID, Text) bound [/string, /string].
Decl work_proposal_pending() bound [].
Decl work_hold_requested() bound [].
Decl work_admitted() bound [].
Decl delegate_lane_ready(ShardType) bound [/name].

# delegate_lane_derived() - derived: some shard would take the turn
# (delegate_lane_ready for any shard); the nullary form lets a rule negate it.
Decl delegate_lane_derived() bound [].
# evidence_lane() - derived: a read-only turn that needs the workspace and
# has no shard, clarification or early lane: the host reads what it names.
Decl evidence_lane() bound [].
Decl work_awaiting_go() bound [].
Decl approve_lane() bound [].
Decl needs_clarification() bound [].
Decl acts_on_target(Verb) bound [/name].
Decl target_names_nothing(Target) bound [/string].

# Conversation acts (policy/conversation_acts.mg): what the user is doing with
# the turn -- asking for work, replying, asking about codeNERD, giving
# feedback, correcting it, sharing material, stopping it, chatting -- beside
# the task verb perception reads. Jev's Choice conversation_act names it
# (jev_chosen, jev_evidence.mg).
# conversation_act_option(Option, Act): the Choice's option name and its act.
Decl conversation_act_option(Option, Act) bound [/string, /name].
# act_asks_no_work(Act): the act carries no task of its own.
Decl act_asks_no_work(Act) bound [/name].
# act_answers_last_turn(Act): the act answers or corrects codeNERD's last turn;
# a task it carries is the one perception reads in it.
Decl act_answers_last_turn(Act) bound [/name].
# act_answered_from_state(Act): the answer is codeNERD's own operating state,
# which the main agent is given every turn, not the workspace.
Decl act_answered_from_state(Act) bound [/name].
# conversation_act(Act): the act the kernel reads this turn.
Decl conversation_act(Act) bound [/name].
# conversation_act_applies(Act): the act routes this turn (no work is held:
# held work is answered by the go-ahead judgments).
Decl conversation_act_applies(Act) bound [/name].
# conversation_act_route(Act, Disposition): what the act did to this turn's
# routing, for the log and the Glass Box: /answer_in_conversation or
# /ask_first. No row: perception's reading routes the turn.
Decl conversation_act_route(Act, Disposition) bound [/name, /name].
# conversation_act_option_known(Option) / conversation_act_unmapped(Option): an
# option Jev answered that no row maps; a test holds it empty.
Decl conversation_act_option_known(Option) bound [/string].
Decl conversation_act_unmapped(Option) bound [/string].

# spawn_subagent(Persona) - derived: subagent should be spawned
# Persona is the closed persona vocabulary shared with persona/1
# (/researcher, /tester, /nemesis) - see policy/intent_routing_rules.mg.
Decl spawn_subagent(Persona) bound [/name].

# shard_profile(AgentName, Type, KnowledgePath)
Decl shard_profile(AgentName, Type, KnowledgePath) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 6.1 Specialist/Advisor Predicates
# -----------------------------------------------------------------------------

# task_complexity(ComplexityLevel) - current task complexity level (1-arg)
# ComplexityLevel: /simple, /complex, /critical
Decl task_complexity(ComplexityLevel) bound [/name].

# task_complexity(Task, ComplexityLevel) - task-specific complexity (2-arg)
Decl task_complexity(Task, ComplexityLevel) bound [/string, /name].

# specialist_classification(Agent, AgentType, SpecialistType) - agent type classification
# Agent: closed specialist vocabulary (/goexpert, /mangleexpert, /northstar, ...)
# AgentType: /advisor, /executor
# SpecialistType: /strategic, /technical
Decl specialist_classification(Agent, AgentType, SpecialistType) bound [/name, /name, /name].

# specialist_knowledge_db(Specialist, DBPath) - specialist's knowledge database
# Specialist is the same closed vocabulary as specialist_classification/3 arg 1;
# DBPath is a filesystem path, so it stays a string.
Decl specialist_knowledge_db(Specialist, DBPath) bound [/name, /string].

# specialist_campaign_role(Specialist, Role) - specialist's role in campaigns
# Role: /phase_executor, /plan_reviewer, /alignment_guardian
Decl specialist_campaign_role(Specialist, Role) bound [/name, /name].

# specialist_can_execute(Specialist) - derived: specialist can execute tasks
# Projection of specialist_classification/3 arg 1, so it carries name constants.
Decl specialist_can_execute(Specialist) bound [/name].


# specialist_should_execute(Specialist, Task) - derived: specialist should execute task directly
Decl specialist_should_execute(Specialist, Task) bound [/name, /string].

# specialist_should_advise(Specialist, Task) - derived: specialist should advise on task
Decl specialist_should_advise(Specialist, Task) bound [/name, /string].

# strategic_advisor_required(Task) - derived: task requires strategic advisor
Decl strategic_advisor_required(Task) bound [/string].

# specialist_context_source(Specialist, DBPath) - derived: route to specialist knowledge DB
# Straight projection of specialist_knowledge_db/2.
Decl specialist_context_source(Specialist, DBPath) bound [/name, /string].

# activate_specialist_for_phase(Specialist, Phase) - derived: activate specialist for campaign phase
# Specialist comes from specialist_campaign_role/2 (name constant); Phase is
# campaign_phase/6 arg 1, which is a PhaseID string, not a name constant.
Decl activate_specialist_for_phase(Specialist, Phase) bound [/name, /string].

# specialist_assists(Advisor, Executor) - derived: advisor can assist executor
# Both slots are specialist_classification/3 arg 1.
Decl specialist_assists(Advisor, Executor) bound [/name, /name].


# specialist_allowed_tools(Specialist, Tool) - derived: tools specialist can use
# Specialist is specialist_classification/3 arg 1; Tool is the /write_file,
# /edit_file, /run_command ... tool vocabulary.
Decl specialist_allowed_tools(Specialist, Tool) bound [/name, /name].

# persona_tool_allowed(Persona, Tool) - derived: tools allowed for a persona
Decl persona_tool_allowed(Persona, Tool) bound [/name, /name].

# =============================================================================
# SECTION 30: CODER SHARD HELPERS
# =============================================================================

# file_content(FilePath, Content) - cached file content
Decl file_content(FilePath, Content) bound [/string, /string].

# coder_state(State) - current coder state
# State: /idle, /context_ready, /code_generated, /edit_applied, /build_passed, /build_failed
Decl coder_state(State) bound [/name].

# pending_edit(FilePath, Content) - pending edits
Decl pending_edit(FilePath, Content) bound [/string, /string].

# coder_block_write(FilePath, Reason) - derived: write is blocked
Decl coder_block_write(FilePath, Reason) bound [/string, /string].

# coder_safe_to_write(FilePath) - derived: safe to write
Decl coder_safe_to_write(FilePath) bound [/string].

# file_package(FilePath, PackageName) - maps file to its Go package
Decl file_package(FilePath, PackageName) bound [/string, /string].

# tdd_state(State) - current TDD phase state
# State: /red (tests failing), /green (tests passing), /refactor
Decl tdd_state(State) bound [/name].

# function_metrics(FilePath, FuncName, Lines, Complexity) - function metrics
Decl function_metrics(FilePath, FuncName, Lines, Complexity) bound [/string, /string, /number, /number].

# function_params(FilePath, FuncName, ParamCount) - parameter count
Decl function_params(FilePath, FuncName, ParamCount) bound [/string, /string, /number].

# function_nesting(FilePath, FuncName, Depth) - nesting depth
Decl function_nesting(FilePath, FuncName, Depth) bound [/string, /string, /number].

# diagnostic_count(FilePath, Severity, Count) - diagnostics per file
Decl diagnostic_count(FilePath, Severity, Count) bound [/string, /name, /number].

# path_contains(Path, Pattern) - checks if file path contains substring
Decl path_contains(Path, Pattern) bound [/string, /string].

# is_test_file(FilePath) - file is a test file (e.g., *_test.go)
# NOTE: Also declared in tester.mg for module separation
Decl is_test_file(FilePath) bound [/string].

# extension_language(Extension, Language) - the language a lower-case file
# extension (with its dot) names (policy/coder_language.mg).
Decl extension_language(Extension, Language) bound [/string, /name].

# testable_language(Language) - language supports automated testing
Decl testable_language(Language) bound [/name].

# test_file_for(TestFile, SourceFile) - TestFile contains tests for SourceFile
Decl test_file_for(TestFile, SourceFile) bound [/string, /string].

# build_state(State) - current build state
# State: /passing, /failing, /unknown
Decl build_state(State) bound [/name].

# build_result(Success, Output) - result of build action (added for Bug 10)
Decl build_result(Success, Output) bound [/name, /string].

# =============================================================================
# SECTION 31: REVIEWER SHARD HELPERS
# =============================================================================

# file_line_count(FilePath, LineCount) - line count per file
Decl file_line_count(FilePath, LineCount) bound [/string, /number].

# finding_count(Severity, Count) - count of findings by severity
Decl finding_count(Severity, Count) bound [/name, /number].

# style_rule(RuleID, RuleName, Threshold) - style rule definitions
# Threshold is the numeric budget the rule allows (max line length, max
# nesting depth, 0 for "none permitted"). Regex-shaped rules carry their
# pattern in style_rule_pattern/2, NOT in this slot.
Decl style_rule(RuleID, RuleName, Threshold) bound [/string, /string, /number].

# style_rule_pattern(RuleID, Pattern) - regex a style rule matches on
# Split out of style_rule/3 because a pattern is not a threshold: STY003 had
# "TODO|FIXME" sitting in the /number Threshold slot, which is the exact
# shape that aborts whole-program evaluation the moment anything compares it.
Decl style_rule_pattern(RuleID, Pattern) bound [/string, /string].

# permission_denied(Action, Reason) - derived: why permission was denied (Bug 12)
Decl permission_denied(Action, Reason) bound [/name, /string].

# checks_passed() - derived: positive confirmation of checks (Bug 10)
Decl checks_passed() bound [].

# safe_to_commit() - derived: all checks pass, safe to commit
Decl safe_to_commit() bound [].

# =============================================================================
# SECTION 32: SAFE NEGATION HELPERS
# =============================================================================
# These helpers support safe negation patterns in Mangle rules

# has_block_commit() - helper: true if any block_commit exists
Decl has_block_commit() bound [].

# has_active_refinement() - helper: true if any refinement in progress
Decl has_active_refinement() bound [].

# has_eligible_phase() - helper: true if any phase is eligible
Decl has_eligible_phase() bound [].

# has_next_campaign_task() - helper: true if there's a next task
Decl has_next_campaign_task() bound [].

# has_in_progress_phase() - helper: true if any phase in progress
Decl has_in_progress_phase() bound [].

# has_incomplete_phase(CampaignID) - helper: campaign has incomplete phases
Decl has_incomplete_phase(CampaignID) bound [/string].

# has_incomplete_phase_task(PhaseID) - helper: phase has incomplete tasks
Decl has_incomplete_phase_task(PhaseID) bound [/string].

# has_incomplete_hard_dep(TaskID) - helper: task has incomplete hard dependencies
Decl has_incomplete_hard_dep(TaskID) bound [/string].

# has_earlier_phase(PhaseID) - helper: there are earlier phases to complete
Decl has_earlier_phase(PhaseID) bound [/string].

# has_earlier_task(TaskID, PhaseID) - helper: a runnable higher-priority task in the phase goes first
Decl has_earlier_task(TaskID, PhaseID) bound [/string, /string].

# all_phase_tasks_complete(PhaseID) - derived: all tasks in phase are complete
Decl all_phase_tasks_complete(PhaseID) bound [/string].

# campaign_complete(CampaignID) - derived: entire campaign is complete
Decl campaign_complete(CampaignID) bound [/string].

# campaign_phases_done(CampaignID) - derived: every phase completed or skipped
# (the orchestrator then settles the campaign's acceptance)
Decl campaign_phases_done(CampaignID) bound [/string].

# -----------------------------------------------------------------------------
# 32.2 Campaign Derived Predicates (from policy.mg Section 19)
# -----------------------------------------------------------------------------

# failed_campaign_task(CampaignID, TaskID) - derived: task failed during campaign
Decl failed_campaign_task(CampaignID, TaskID) bound [/string, /string].

# priority_higher(PriorityA, PriorityB) - priority ordering helper
# Returns true if PriorityA is higher than PriorityB.
# The six ground facts in policy/campaign_tasks.mg are name constants
# (/critical, /high, /normal, /low) and they join task_priority/2, whose Go
# producer emits campaign.TaskPriority values that already carry the leading
# slash ("/critical") and therefore land as name constants. This is an
# ordering table over an enum, not arithmetic - nothing sums or compares it.
Decl priority_higher(PriorityA, PriorityB) bound [/name, /name].

# has_blocking_task_dep(TaskID) - helper: task has incomplete blocking dependencies
Decl has_blocking_task_dep(TaskID) bound [/string].

# task_conflict_active(TaskID) - helper: task conflicts with an in-progress task
Decl task_conflict_active(TaskID) bound [/string].

# has_passed_checkpoint(PhaseID, CheckType) - helper: phase has passed checkpoint
Decl has_passed_checkpoint(PhaseID, CheckType) bound [/string, /name].

# phase_success_pattern(PhaseType) - tracks successful phase types for learning
Decl phase_success_pattern(PhaseType) bound [/name].

# =============================================================================
# SECTION 33: SYSTEM SHARD COORDINATION
# =============================================================================
# Predicates for coordinating the 6 Type 1 system shards:
# - perception_firewall: NL → atoms transduction
# - world_model_ingestor: file_topology, symbol_graph maintenance
# - executive_policy: next_action derivation
# - constitution_gate: safety enforcement
# - tactile_router: action → tool routing
# - session_planner: agenda/campaign orchestration

# -----------------------------------------------------------------------------
# 33.1 System Shard Registry
# -----------------------------------------------------------------------------

# system_shard(ShardName, Type) - registered system shards with type
Decl system_shard(ShardName, Type) bound [/string, /name].

# system_shard_state(ShardName, State) - current shard state
# State: /idle, /starting, /running, /stopping, /stopped, /error
Decl system_shard_state(ShardName, State) bound [/string, /name].

# system_heartbeat(ShardName, Timestamp) - last heartbeat from shard
# ShardName is the system shard's TYPE name from the fixed registry
# (/perception_firewall, /executive_policy, /constitution_gate, /tactile_router,
# /session_planner, /world_model_ingestor, ...), NOT a spawn instance id. It is
# the same closed vocabulary that activate_shard/1 and shard_startup/2 hold, so
# it is /name. BaseSystemShard.EmitHeartbeat wraps its plain ID accordingly.
Decl system_heartbeat(ShardName, Timestamp) bound [/name, /number].

# Campaign Runner supervision facts
Decl campaign_runner_heartbeat(Timestamp) bound [/number].
Decl campaign_runner_active(CampaignID, Timestamp) bound [/string, /number].
Decl campaign_runner_success(CampaignID, Timestamp) bound [/string, /number].
Decl campaign_runner_failure(CampaignID, Reason, Timestamp) bound [/string, /string, /number].

# -----------------------------------------------------------------------------
# 33.2 Intent Processing Flow
# -----------------------------------------------------------------------------

# processed_intent(IntentID) - intent has been processed by perception
# IntentID is the same /name value user_intent/5 carries: every producer
# (chat/process.go, chat/process_seed.go, shards/system/perception.go) asserts
# it alongside user_intent from the same intentID variable, "/current_intent",
# which Fact.ToAtom promotes to a name constant.
Decl processed_intent(IntentID) bound [/name].

# executive_processed_intent(IntentID) - intent has been consumed by the executive
# (i.e., executive_policy emitted the first action envelope for it).
# Same /name intent id. shards/system/executive.go asserts the literal
# "/current_intent"; the 37 negated body literals in policy/capabilities.mg all
# match !executive_processed_intent(/current_intent).
Decl executive_processed_intent(IntentID) bound [/name].

# pending_intent(IntentID) - derived: intent waiting to be processed
# IntentID is the singleton name constant /current_intent: the only rule head
# is pending_intent(/current_intent) and the Go producers of user_intent/5
# emit "/current_intent", which Fact.ToAtom turns into a name constant.
Decl pending_intent(IntentID) bound [/name].

# -----------------------------------------------------------------------------
# 33.3 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# pending_action(ActionID, ActionType, Target, Payload, Timestamp) - action awaiting permission check
# ActionID is a unique string generated by the executive (or shard) for correlation.
# Payload may be a map or additional args to pass to VirtualStore.
Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp) bound [/string, /name, /string, /string, /number].

# permitted_action(ActionID, ActionType, Target, Payload, Timestamp) - action cleared by constitution gate
# This is the primary action stream for the tactile router today.
Decl permitted_action(ActionID, ActionType, Target, Payload, Timestamp) bound [/string, /name, /string, /string, /number].

# pending_permission_check(ActionID) - derived: action needs constitution check
Decl pending_permission_check(ActionID) bound [/string].

# action_permitted(ActionID) - action passed constitution gate (derived from permission_check_result)
Decl action_permitted(ActionID) bound [/string].

# ready_for_routing(ActionID) - derived: action ready for router
Decl ready_for_routing(ActionID) bound [/string].

# route_to(Target, Reason) - Routing decision record
# Records where an action was routed and why (for transparency display)
Decl route_to(Target, Reason) bound [/name, /string].

# exec_request(ToolName, Target, Timeout, CallID, Timestamp) - router output
Decl exec_request(ToolName, Target, Timeout, CallID, Timestamp) bound [/string, /string, /number, /string, /number].

# -----------------------------------------------------------------------------
# 33.4 Safety & Violations
# -----------------------------------------------------------------------------

# security_violation(ActionType, Reason, Timestamp) - blocked by constitution
Decl security_violation(ActionType, Reason, Timestamp) bound [/name, /string, /number].

# escalation_needed(Target, Subject, Reason) - needs human intervention
# Target: system component, a closed vocabulary of name constants
#         (/system_health, /session_planner, /campaign,
#          /constitution_gate). Go producers must use types.MangleAtom.
# Subject: entity being escalated - an open identifier (ItemID, PhaseID) or a
#          composite like "write_file:/some/path.go" from escalationSubject().
#          Open text, so /string.
# Reason: why escalation is needed - free text
Decl escalation_needed(Target, Subject, Reason) bound [/name, /string, /string].

# rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp)
Decl rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp) bound [/string, /string, /string, /number, /number].

# -----------------------------------------------------------------------------
# 33.5 System Health
# -----------------------------------------------------------------------------

# system_shard_healthy(ShardName) - derived: shard heartbeat recent
# ShardName is inherited from system_heartbeat/2, so it is the /name-shaped
# shard type. The on-demand activation rules in policy/system_shards.mg negate
# this with literals (!system_shard_healthy(/session_planner)); while this was
# declared /string and Go asserted a bare "session_planner", the negation could
# never be satisfied and every activate_shard guard was permanently open.
Decl system_shard_healthy(ShardName) bound [/name].

# system_shard_unhealthy(ShardName) - derived: shard heartbeat stale
Decl system_shard_unhealthy(ShardName) bound [/string].

# world_model_heartbeat(ShardID, FileCount, Timestamp) - world model status
Decl world_model_heartbeat(ShardID, FileCount, Timestamp) bound [/string, /number, /number].

# world_model_updating(UpdateType, Scope) - trigger for incremental world model updates
Decl world_model_updating(UpdateType, Scope) bound [/name, /name].

# session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp)
Decl session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp) bound [/number, /number, /number, /number, /number, /number].

# plan_task(TaskID, Description, Status, ProgressPct) - individual task state
# Status: /pending, /in_progress, /completed, /blocked
Decl plan_task(TaskID, Description, Status, ProgressPct) bound [/string, /string, /name, /number].

# plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) - overall plan progress
Decl plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) bound [/string, /number, /number, /number].

# -----------------------------------------------------------------------------
# 33.6 Routing & Tool Management
# -----------------------------------------------------------------------------

# routing_error(ActionType, Reason, Timestamp) - router couldn't find handler
Decl routing_error(ActionType, Reason, Timestamp) bound [/name, /string, /number].

# route_added(ActionPattern, ToolName, Timestamp) - new route added via autopoiesis
Decl route_added(ActionPattern, ToolName, Timestamp) bound [/string, /string, /number].

# system_event_handled(ActionType, Target, Timestamp) - internal kernel lifecycle event acknowledged
Decl system_event_handled(ActionType, Target, Timestamp) bound [/name, /string, /number].

# -----------------------------------------------------------------------------
# 33.7 Agenda & Planning
# -----------------------------------------------------------------------------

# agenda_item(ItemID, Description, Priority, Status, Timestamp)
Decl agenda_item(ItemID, Description, Priority, Status, Timestamp) bound [/string, /string, /number, /name, /number].

# session_checkpoint(CheckpointID, ItemsRemaining, Timestamp)
Decl session_checkpoint(CheckpointID, ItemsRemaining, Timestamp) bound [/string, /number, /number].

# task_completed(TaskID) - task marked complete
Decl task_completed(TaskID) bound [/string].

# task_blocked(TaskID) - task is blocked
Decl task_blocked(TaskID) bound [/string].

# -----------------------------------------------------------------------------
# 33.8 Perception Errors & Stats
# -----------------------------------------------------------------------------

# perception_error(Message, Timestamp) - perception shard error
Decl perception_error(Message, Timestamp) bound [/string, /number].

# world_model_error(Message, Timestamp) - world model shard error
Decl world_model_error(Message, Timestamp) bound [/string, /number].

# executive_error(Message, Timestamp) - executive shard error
Decl executive_error(Message, Timestamp) bound [/string, /number].

# executive_trace(Action, FromRule, Rationale, Timestamp) - debug trace
Decl executive_trace(Action, FromRule, Rationale, Timestamp) bound [/name, /string, /string, /number].

# strategy_activated(StrategyName, Timestamp) - strategy change
Decl strategy_activated(StrategyName, Timestamp) bound [/string, /number].

# executive_blocked(Reason, Timestamp) - executive policy blocked
Decl executive_blocked(Reason, Timestamp) bound [/string, /number].

# -----------------------------------------------------------------------------
# 33.8b Tactile Execution Audit Facts
# -----------------------------------------------------------------------------
# Facts generated by tactile/audit.go for execution event tracking

# execution_started(SessionID, RequestID, Binary, Timestamp) - command started
Decl execution_started(SessionID, RequestID, Binary, Timestamp) bound [/string, /string, /string, /number].

# execution_command(RequestID, CommandString) - full command string
Decl execution_command(RequestID, CommandString) bound [/string, /string].

# execution_working_dir(RequestID, WorkingDir) - working directory
Decl execution_working_dir(RequestID, WorkingDir) bound [/string, /string].

# execution_completed(RequestID, ExitCode, DurationMs, Timestamp) - command finished
Decl execution_completed(RequestID, ExitCode, DurationMs, Timestamp) bound [/string, /number, /number, /number].

# execution_output(RequestID, StdoutLen, StderrLen) - output lengths
Decl execution_output(RequestID, StdoutLen, StderrLen) bound [/string, /number, /number].

# execution_success(RequestID) - successful execution (exit code 0)
Decl execution_success(RequestID) bound [/string].

# execution_nonzero(RequestID, ExitCode) - non-zero exit code
Decl execution_nonzero(RequestID, ExitCode) bound [/string, /number].

# execution_failure(RequestID, Error) - infrastructure failure
Decl execution_failure(RequestID, Error) bound [/string, /string].

# execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) - resource metrics
Decl execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) bound [/string, /number, /number].

# execution_io(RequestID, ReadBytes, WriteBytes) - I/O metrics
Decl execution_io(RequestID, ReadBytes, WriteBytes) bound [/string, /number, /number].

# execution_sandbox(RequestID, SandboxMode) - sandbox mode used
Decl execution_sandbox(RequestID, SandboxMode) bound [/string, /name].

# execution_killed(RequestID, Reason, DurationMs) - command was killed
Decl execution_killed(RequestID, Reason, DurationMs) bound [/string, /string, /number].

# execution_error(RequestID, ErrorMessage) - execution error
Decl execution_error(RequestID, ErrorMessage) bound [/string, /string].

# execution_blocked(RequestID, Reason, Timestamp) - command was blocked by policy/sandbox
Decl execution_blocked(RequestID, Reason, Timestamp) bound [/string, /string, /number].

# Structured summaries parsed from test and build output. These names are
# deliberately execution-scoped so they cannot collide with tester/world facts
# that use test_result, failed_test, test_coverage, build_result, or diagnostic
# with different meanings and arities.
Decl execution_test_summary(RequestID, Passed, Failed, Skipped) bound [/string, /number, /number, /number].
Decl execution_test_state(RequestID, State) bound [/string, /name].
Decl execution_failed_test(RequestID, TestName) bound [/string, /string].
Decl execution_test_coverage(RequestID, CoveragePercent) bound [/string, /number].
Decl execution_build_summary(RequestID, Success, Errors, Warnings) bound [/string, /name, /number, /number].
Decl execution_diagnostic(RequestID, Severity, FilePath, Line, Column, Message) bound [/string, /name, /string, /number, /number, /string].

# -----------------------------------------------------------------------------
# 33.9 Policy Derived Predicates (Section 21 Support)
# -----------------------------------------------------------------------------

# Intent processing derived predicates
Decl intent_processed(IntentID) bound [/string].
Decl focus_needs_resolution(Ref) bound [/string].
# IntentID matches pending_intent/1: the singleton name constant /current_intent.
Decl intent_ready_for_executive(IntentID) bound [/name].

# Action flow derived predicates
Decl action_pending_permission(ActionID) bound [/string].
Decl permission_checked(ActionID) bound [/string].
Decl permission_check_result(ActionID, Result, Reason, Timestamp) bound [/string, /name, /string, /number].
Decl action_blocked(ActionID, Reason) bound [/string, /string].
Decl action_routed(ActionID) bound [/string].
Decl routing_result(ActionID, Result, Details, Timestamp) bound [/string, /name, /string, /number].
Decl routing_succeeded(ActionID) bound [/string].
Decl routing_failed(ActionID, Error) bound [/string, /string].

# Health monitoring derived predicates
Decl shard_heartbeat_stale(ShardName) bound [/string].

# Safety derived predicates
Decl block_all_actions(Reason) bound [/string].
Decl security_anomaly(AnomalyID, Type, Details) bound [/string, /name, /string].
Decl anomaly_investigated(AnomalyID) bound [/string].
Decl investigation_result(AnomalyID, Result) bound [/string, /string].
Decl violation_count(Pattern, Count) bound [/string, /number].
Decl propose_safety_rule(Pattern) bound [/string].
Decl repeated_violation_pattern(Pattern) bound [/string].
Decl safety_violation(ViolationID, Pattern, ActionType, Timestamp) bound [/string, /string, /name, /number].

# World model derived predicates
Decl world_model_stale(File) bound [/string].
Decl file_in_project(File) bound [/string].
# Priority: 70
Decl symbol_reachable_bounded(From, To, MaxDepth) bound [/string, /string, /number].
# Depth generator for bounded recursion (1-20)
Decl depth_option(Depth) bound [/number].

# Routing derived predicates
Decl routing_table(ActionType, Tool, RiskLevel) bound [/name, /name, /name].
Decl tool_allowlist(Tool, Timestamp) bound [/name, /number].
Decl tool_allowed(Tool, ActionType) bound [/name, /name].
Decl route_action(ActionID, Tool) bound [/string, /name].
Decl action_type(ActionID, ActionType) bound [/string, /name].
Decl routing_blocked(ActionID, Reason) bound [/string, /string].
Decl has_tool_for_action(ActionType) bound [/name].

# Agenda/Planning derived predicates
Decl agenda_item_ready(ItemID) bound [/string].
Decl has_incomplete_dependency(ItemID) bound [/string].
Decl agenda_dependency(ItemID, DepID) bound [/string, /string].
Decl next_agenda_item(ItemID) bound [/string].
Decl has_higher_priority_item(ItemID) bound [/string].
Decl checkpoint_due() bound [].
Decl last_checkpoint_time(Timestamp) bound [/number].
Decl agenda_item_escalate(ItemID, Reason) bound [/string, /string].
Decl item_retry_count(ItemID, Count) bound [/string, /number].

# Shard activation derived predicates
# ShardName is the closed system-shard vocabulary (/perception_firewall,
# /executive_policy, /constitution_gate, /world_model_ingestor,
# /tactile_router, /session_planner). The Go consumer of activate_shard
# (ShardManager.StartSystemShards) normalizes with TrimLeft("/"), so it reads
# name constants correctly.
Decl activate_shard(ShardName) bound [/name].
Decl system_startup(State) bound [/name].
Decl shard_startup(ShardName, Mode) bound [/name, /name].

# Autopoiesis derived predicates
Decl unhandled_case_count(ShardName, Count) bound [/string, /number].
Decl unhandled_cases(ShardName, Cases) bound [/string, /string].
# ShardName here is a policy-module / shard identity name constant. Nothing
# derives it: the system_autopoiesis.mg rule binds it from system_shard/2,
# which nothing (Go or .mg) asserts, and the policy/verification.mg rule
# (deleted 2026-09-23) read quality violations nothing asserted either.
Decl propose_new_rule(ShardName) bound [/name].
Decl proposed_rule(RuleID, ShardName, MangleCode, Confidence) bound [/string, /string, /string, /number].
Decl rule_needs_approval(RuleID) bound [/string].
Decl auto_apply_rule(RuleID) bound [/string].
Decl rule_applied(RuleID) bound [/string].
Decl applied_rule(RuleID, Timestamp) bound [/string, /number].
Decl learning_signal(SignalType, RuleID) bound [/name, /string].
Decl rule_outcome(RuleID, Outcome, Details) bound [/string, /name, /string].

# OODA loop derived predicates
Decl ooda_phase(Phase) bound [/name].
Decl has_next_action() bound [].
Decl current_ooda_phase(Phase) bound [/name].

# Builtin helper predicates
# Note: time_diff removed - use fn:minus(Now, Timestamp) inline in rules instead.
# Note: list_length is DEPRECATED - use fn:list:length(List) in transform pipelines instead.
