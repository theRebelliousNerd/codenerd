# Shard experts: the harness decides which shard agent a task needs.
#
# A shard agent's knowledge (its prompts.yaml atoms and its knowledge base)
# reaches a turn when this file says the turn needs it, not when the model
# thinks to ask. Go measures the roster and each agent's technologies at boot
# (shard_agent, shard_agent_domain, technology_import_root) and, for each task,
# its verb and targets (expert_task, expert_task_target, file_extension);
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

# The target's language from its measured extension (policy/coder_language.mg).
task_technology(Task, File, Lang, /language) :-
    expert_task_file(Task, File),
    detected_language(File, Lang).

# A scanned file's language from the world model.
task_technology(Task, File, Lang, /language) :-
    expert_task_file(Task, File),
    file_topology(File, _, Lang, _, _).

# An import under a technology's root. expert_task_file binds File first, so
# the scan is the task's own imports, not the workspace's dependency_link.
task_technology(Task, File, Tech, /import) :-
    expert_task_file(Task, File),
    dependency_link(File, _, Import),
    technology_import_root(Root, Tech),
    :string:starts_with(Import, Root).

# --- The verb ----------------------------------------------------------------

# Explaining code reads it, so it needs the domain like changing it does.
reads_code_verb(/explain).

expert_blind_verb(Verb) :-
    conversational_verb(Verb),
    !reads_code_verb(Verb).

# --- The decision ------------------------------------------------------------

expert_needed(Task, Agent, Tech) :-
    expert_task(Task, Verb),
    !expert_blind_verb(Verb),
    task_technology(Task, _, Tech, _),
    shard_agent_domain(Agent, Tech, _),
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

expert_evidence(Task, Agent, Tech, Confidence) :-
    expert_needed(Task, Agent, Tech),
    task_technology(Task, _, Tech, Via),
    expert_evidence_confidence(Via, Confidence).

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
