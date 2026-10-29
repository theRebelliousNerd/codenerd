# Shard experts: the harness decides which shard agent a task needs.
#
# A shard agent's knowledge (its prompts.yaml atoms and its knowledge base)
# reaches a turn when this file says the turn needs it, not when the model
# thinks to ask. Go measures the roster and each agent's technologies at boot
# (shard_agent, shard_agent_domain, technology_import_exact/_prefix) and, for each task,
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

turn_expert_actionable(Turn, Idx) :-
    turn_expert_finding(Turn, _, Idx, Sev),
    severity_rank(Sev, Rank),
    severity_rank(/medium, Bar),
    Rank >= Bar.
