# Shard experts: the harness decides which shard agent a task needs.
#
# A shard agent's knowledge (its prompts.yaml atoms and its knowledge base)
# reaches a turn when this file says the turn needs it, not when the model
# thinks to ask. Go measures the roster and each agent's technologies at boot
# (shard_agent, shard_agent_domain, technology_import_exact/_prefix) and, for each task,
# its verb and targets (expert_task, expert_task_target, expert_task_extension);
# everything below is the decision. Consumers (internal/experts):
#   - the session executor's compile: every expert_needed agent's KB is pushed
#     into the prompt (JIT first), whatever entry point runs the turn;
#   - chat delegation: specialist_match, derived here, drives
#     specialist_should_execute / specialist_should_advise (policy/shards.mg)
#     and the spawn topology (execution_mode, policy/delegation.mg).
#
# This replaces a Go table of file-name, import and content substrings
# (CoreTechnologyPatterns) that ran only on two chat paths, so `nerd fix`,
# `nerd run` and campaign tasks never selected an expert.

# --- The task's files --------------------------------------------------------

expert_task_file(Task, File) :-
    expert_task_target(Task, File).

# A package directory target: its scanned files.
expert_task_file(Task, File) :-
    expert_task_target(Task, Dir),
    file_dir(File, Dir).

# --- The technologies those files use ----------------------------------------

# The target's language from its measured extension, through the extension
# table (policy/coder_language.mg).
task_technology(Task, File, Lang, /language) :-
    expert_task_extension(Task, File, Ext),
    extension_language(Ext, Lang).

# A scanned file's language from the world model.
task_technology(Task, File, Lang, /language) :-
    expert_task_file(Task, File),
    file_topology(File, _, Lang, _, _).

# An import of a technology, exactly or under its prefix. The world model
# records every import a Go, Python, JavaScript, TypeScript or Rust file makes
# as dependency_link(File, _, Import). expert_task_file binds File first, so
# the scan is the task's own imports, not the workspace's dependency_link.
task_technology(Task, File, Tech, /import) :-
    expert_task_file(Task, File),
    dependency_link(File, _, Import),
    technology_import_exact(Import, Tech).

task_technology(Task, File, Tech, /import) :-
    expert_task_file(Task, File),
    dependency_link(File, _, Import),
    technology_import_prefix(Prefix, Tech),
    :string:starts_with(Import, Prefix).

# --- The verb ----------------------------------------------------------------

# Explaining code reads it, so it needs the domain like changing it does.
reads_code_verb(/explain).

expert_blind_verb(Verb) :-
    conversational_verb(Verb),
    !reads_code_verb(Verb).

# --- The domains an agent's name states ---------------------------------------

# A concept agent (mangle-logic-architect, SecurityAuditor, TestArchitect) is
# defined by what it is for, not by a technology its init created it for, so
# its registry entry carries no domains. Its name says them: a word of it that
# is a language the extension table knows, a technology the import vocabulary
# names, or a concept term is a domain it holds. Go splits the name; only the
# kernel decides what a word names.
shard_agent_domain(Agent, Lang, /agent_name) :-
    shard_agent(Agent),
    shard_agent_name_part(Agent, Lang),
    extension_language(_, Lang).

shard_agent_domain(Agent, Tech, /agent_name) :-
    shard_agent(Agent),
    shard_agent_name_part(Agent, Tech),
    technology_import_exact(_, Tech).

shard_agent_domain(Agent, Domain, /agent_name) :-
    shard_agent(Agent),
    shard_agent_name_part(Agent, Term),
    domain_concept_term(Term, Domain).

# --- The decision ------------------------------------------------------------

expert_needed(Task, Agent, Tech) :-
    expert_task(Task, Verb),
    !expert_blind_verb(Verb),
    task_technology(Task, _, Tech, _),
    shard_agent_domain(Agent, Tech, _),
    shard_agent(Agent).

# A concept domain has no import or extension to match: the verb the task runs
# as names it (a /security task needs the security expert).
expert_needed(Task, Agent, Domain) :-
    expert_task(Task, Verb),
    !expert_blind_verb(Verb),
    domain_concept_verb(Domain, Verb),
    shard_agent_domain(Agent, Domain, _),
    shard_agent(Agent).

expert_file(Task, Agent, File) :-
    expert_needed(Task, Agent, Tech),
    task_technology(Task, File, Tech, _).

# A framework import names the expert's exact subject: strong enough to spawn
# the expert (specialist_should_execute above 80 for an executor,
# specialist_should_advise otherwise). A language match is the expert's broad
# field: its knowledge is pushed into the turn (JIT first) and nothing is
# spawned -- at 60 it advised, which on a Go repository consulted the Go
# expert on every turn for what its pushed atoms already carry.
expert_evidence_confidence(/import, 90).
expert_evidence_confidence(/language, 40).

# A verb that names a concept is the user asking for the domain, but it names
# no code: the expert's knowledge is pushed into the turn like a language
# match, and nothing is spawned.
expert_evidence_confidence(/verb, 40).

expert_evidence(Task, Agent, Tech, Confidence) :-
    expert_needed(Task, Agent, Tech),
    task_technology(Task, _, Tech, Via),
    expert_evidence_confidence(Via, Confidence).

expert_evidence(Task, Agent, Domain, Confidence) :-
    expert_needed(Task, Agent, Domain),
    expert_task(Task, Verb),
    domain_concept_verb(Domain, Verb),
    expert_evidence_confidence(/verb, Confidence).

expert_confidence(Task, Agent, Confidence) :-
    expert_evidence(Task, Agent, Tech, C)
    |> do fn:group_by(Task, Agent), let Confidence = fn:max(C).

specialist_match(Agent, Task, Confidence) :-
    expert_confidence(Task, Agent, Confidence).

# --- Spawn second ----------------------------------------------------------------

# A task already running as a shard agent (a consultation, `nerd spawn <agent>`)
# spawns no one: the consultation an expert answers must not consult again.
# The speaker is whatever user-agent name the verb addresses; only a roster
# agent counts, so a verb outside the persona taxonomy ("/explain") that
# merely looks like an agent name does not silence the spawn.
expert_task_spoken(Task) :-
    expert_task_speaker(Task, Agent),
    shard_agent(Agent).

expert_spawn(Task, Agent, /lead) :-
    specialist_should_execute(Agent, Task),
    !expert_task_spoken(Task).

expert_spawn(Task, Agent, /advise) :-
    specialist_should_advise(Agent, Task),
    !expert_task_spoken(Task).

# --- Recording derived domains ------------------------------------------------

shard_agent_domain_to_record(Agent, Tech) :-
    shard_agent(Agent),
    !shard_agent_recorded(Agent),
    shard_agent_domain(Agent, Tech, _).

# --- Task complexity ----------------------------------------------------------
# What makes a task complex is measured, not read from its wording: its verb,
# how many files it spans, and how much those files define in the world model.
# A keyword match over the task text ("complex", "security", "refactor") made
# this call in Go until it moved here.

complex_verb(/refactor).
complex_verb(/security).
complex_verb(/audit).
complex_verb(/migrate).

expert_task_file_count(Task, N) :-
    expert_task_file(Task, File)
    |> do fn:group_by(Task), let N = fn:count().

expert_task_file_symbol(Task, File, Symbol) :-
    expert_task_file(Task, File),
    symbol_graph(Symbol, _, _, File, _).

expert_task_file_symbols(Task, File, N) :-
    expert_task_file_symbol(Task, File, Symbol)
    |> do fn:group_by(Task, File), let N = fn:count().

expert_task_complex(Task) :-
    expert_task(Task, Verb),
    complex_verb(Verb).

# More than three files: a change that spans packages.
expert_task_complex(Task) :-
    expert_task_file_count(Task, N),
    N > 3.

# A file defining more than sixty symbols: a change inside a large surface.
expert_task_complex(Task) :-
    expert_task_file_symbols(Task, _, N),
    N > 60.

task_complexity(Task, /high) :- expert_task_complex(Task).

task_complexity(Task, /normal) :-
    expert_task(Task, _),
    !expert_task_complex(Task).

# --- Critique ------------------------------------------------------------------
# A verb whose topology is advise, execute, critique (/fix, /refactor) has the
# shard agents it spawned review the change before the turn's verdict, through
# the same consultation path. The review is a post-edit round
# (/expert_critique, policy/turn_rounds.mg); its on-change findings at or above
# the critic's bar go back to the model as one repair round, exactly as the
# build critic's do (turn_critic_actionable, coder_safety.mg).

expert_critique(Task, Agent) :-
    expert_spawn(Task, Agent, _),
    expert_task(Task, Verb),
    execution_mode(Verb, /advisory_with_critique).

# --- The consultation routing ladder ------------------------------------------
# expert_spawn says a task needs an expert's own reading; it does not say a
# model call must buy it. Completed advice has no evidence receipts and is
# never reused across results. Failure history applies only to a nonempty
# scope whose complete before/after/current observations agree:
#   /unresolved a valid card that went unanswered under the stronger profile:
#               the gap is kept, not asked again on the same files;
#   /escalate   a valid unanswered card not yet escalated, or a complex task's
#               lead, when the stronger profile is configured;
#   /knowledge  an adviser whose knowledge rows the compile already carried,
#               on a task that is not complex;
#   /consult    everything else.

consult_ask_scoped(Ask, Agent) :- consult_ask_file(Ask, Agent, _).

consult_file_stable(Ask, Agent, File) :-
    consult_file_observation(Ask, Agent, File, /before, /read, Digest),
    consult_file_observation(Ask, Agent, File, /after, /read, Digest),
    consult_file_observation(Ask, Agent, File, /current, /read, Digest).

consult_card_stale(Ask, Agent) :-
    consult_ask_file(Ask, Agent, File),
    !consult_file_stable(Ask, Agent, File).

consult_card_unanswered(Ask, Agent) :-
    consult_card_cached(Ask, Agent, /failed),
    consult_ask_scoped(Ask, Agent),
    !consult_card_stale(Ask, Agent).

consult_card_unanswered(Ask, Agent) :-
    consult_card_cached(Ask, Agent, /empty),
    consult_ask_scoped(Ask, Agent),
    !consult_card_stale(Ask, Agent).

consult_route(Ask, Agent, /unresolved) :-
    consult_ask(Ask, Agent, _),
    consult_card_unanswered(Ask, Agent),
    consult_card_escalated(Ask, Agent).

consult_escalates(Ask, Agent) :-
    consult_ask(Ask, Agent, _),
    consult_card_unanswered(Ask, Agent),
    !consult_card_escalated(Ask, Agent),
    consult_escalation_available(Ask).

consult_escalates(Ask, Agent) :-
    consult_ask(Ask, Agent, /lead),
    consult_ask_complex(Ask),
    consult_escalation_available(Ask),
    !consult_card_unanswered(Ask, Agent).

consult_route(Ask, Agent, /escalate) :- consult_escalates(Ask, Agent).

consult_knowledge_suffices(Ask, Agent) :-
    consult_ask(Ask, Agent, /advise),
    consult_knowledge_delivered(Ask, Agent, Rows),
    Rows > 0,
    !consult_ask_complex(Ask),
    !consult_card_unanswered(Ask, Agent).

consult_route(Ask, Agent, /knowledge) :- consult_knowledge_suffices(Ask, Agent).

consult_settled(Ask, Agent) :-
    consult_card_unanswered(Ask, Agent),
    consult_card_escalated(Ask, Agent).
consult_settled(Ask, Agent) :- consult_escalates(Ask, Agent).
consult_settled(Ask, Agent) :- consult_knowledge_suffices(Ask, Agent).

consult_route(Ask, Agent, /consult) :-
    consult_ask(Ask, Agent, _),
    !consult_settled(Ask, Agent).

# Dealing a turn's consultations to its planned steps. A lead's answer lays
# out the whole work, so every step holds it; an adviser's answer is about the
# files it was asked about, so it goes to the steps on those files. A card
# that did not answer is dealt to no step (the step names it instead).
turn_card_dealt(Turn, File, Card) :-
    turn_step_file(Turn, File),
    turn_expert_card(Turn, Card, _, /lead, /answered).

turn_card_dealt(Turn, File, Card) :-
    turn_step_file(Turn, File),
    turn_expert_card(Turn, Card, _, /advise, /answered),
    turn_expert_card_file(Turn, Card, File).

# Edge-bound advice: a hand-off belongs to the relationship between two files,
# so it goes to the step on either end, whichever card carried it.
turn_handoff_answered(Turn, Card, Idx, From, To) :-
    turn_card_handoff(Turn, Card, Idx, From, To),
    turn_expert_card(Turn, Card, _, _, /answered).

turn_handoff_dealt(Turn, File, Card, Idx) :-
    turn_step_file(Turn, File),
    turn_handoff_answered(Turn, Card, Idx, File, _).

turn_handoff_dealt(Turn, File, Card, Idx) :-
    turn_step_file(Turn, File),
    turn_handoff_answered(Turn, Card, Idx, _, File).

# A hand-off out of a planned file into one the plan does not edit is missing
# work: the plan gains a step on an existing consumer, and a consumer that is
# not on disk is reported unresolved rather than invented.
turn_handoff_unplanned(Turn, Card, Idx, To) :-
    turn_handoff_answered(Turn, Card, Idx, From, To),
    turn_plan_file(Turn, From),
    !turn_plan_file(Turn, To).

turn_handoff_step(Turn, Card, Idx, To) :-
    turn_handoff_unplanned(Turn, Card, Idx, To),
    turn_handoff_target_exists(Turn, To).

turn_handoff_unresolved(Turn, Card, Idx, To) :-
    turn_handoff_unplanned(Turn, Card, Idx, To),
    !turn_handoff_target_exists(Turn, To).

# The repair stage. After the planned steps each repair (a build, test, vet,
# coverage or pin repair, and each advisory round) runs on the turn's own
# prompt, which carries the consultations bought before the plan but none a
# step bought. Per repair: a step-born answered lead goes to it (a lead's plan
# covers the work); a step-born adviser goes to it only when that repair's
# subject names one of the files it was asked about. A repair whose subject
# names none of the turn's files gets no adviser card, and says so.
turn_repair_card_dealt(Turn, Repair, Card) :-
    turn_repair(Turn, Repair),
    turn_card_step_born(Turn, Card),
    turn_expert_card(Turn, Card, _, /lead, /answered).

turn_repair_card_dealt(Turn, Repair, Card) :-
    turn_repair(Turn, Repair),
    turn_card_step_born(Turn, Card),
    turn_expert_card(Turn, Card, _, /advise, /answered),
    turn_expert_card_file(Turn, Card, File),
    turn_repair_file(Turn, Repair, File).

# Mandates. An expert's REQUIRE line is a proposal; it becomes an obligation
# only when the user's registry grants that agent /mandate authority. An
# admitted mandate names a test, and only that test passing in the turn's
# own run (turn_test_case, coder_safety.mg) discharges it; the turn is not
# verified while one is unmet (turn_verified, coder_safety.mg). Neither the
# expert's confidence nor the model's word can admit or discharge one.
turn_mandate_admitted(Turn, Test) :-
    turn_card_mandate(Turn, Card, _, Test),
    turn_expert_card(Turn, Card, Agent, _, /answered),
    shard_agent_authority(Agent, /mandate).

turn_mandate_discharged(Turn, Test) :-
    turn_mandate_admitted(Turn, Test),
    turn_test_case(Turn, _, Test, /pass, _).

turn_mandate_unmet(Turn, Test) :-
    turn_mandate_admitted(Turn, Test),
    !turn_mandate_discharged(Turn, Test).

turn_has_unmet_mandate(Turn) :- turn_mandate_unmet(Turn, _).

turn_expert_actionable(Turn, Idx) :-
    turn_expert_finding(Turn, _, Idx, Sev),
    severity_rank(Sev, Rank),
    severity_rank(/medium, Bar),
    Rank >= Bar.
