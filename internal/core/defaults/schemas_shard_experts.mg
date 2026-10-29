# Shard experts: which shard agent a task needs, decided by the kernel.
#
# A shard agent is a user-defined specialist: .nerd/agents/<name>/prompts.yaml
# synced into .nerd/shards/<name>_knowledge.db and listed in .nerd/agents.json.
# Go measures the roster, each agent's declared technologies, the task's verb
# and the files it targets; policy/shard_experts.mg derives expert_needed, and
# the executor and chat delegation obey it (internal/experts).

# --- Measured by Go (EDB) ---------------------------------------------------

# shard_agent(Agent) - a ready shard agent from .nerd/agents.json. Agent is the
# lower-cased registry name as a name constant (/goexpert), the same identity
# specialist_classification and specialist_match key on.
# Producer: internal/system/shard_expert_facts.go at boot.
Decl shard_agent(Agent) bound [/name].

# shard_agent_domain(Agent, Technology, Source) - a technology the agent is
# expert in. Technology is a lower-case technology key as a name constant
# (/go, /mangle, /bubbletea): the same keys extension_language names and the
# init catalog creates agents for. Source says where the claim came from:
#   /registry       - the agent's `domains` in .nerd/agents.json (init writes
#                     the catalog key that created the agent)
#   /knowledge_base - a languages:/frameworks: tag on an atom in the agent's KB
# Producer: internal/system/shard_expert_facts.go at boot.
Decl shard_agent_domain(Agent, Technology, Source) bound [/name, /name, /name].

# technology_import_exact(Import, Technology) - an import spelled exactly so
# names Technology ("github.com/spf13/cobra", "react", "django", "tokio").
# technology_import_prefix(Prefix, Technology) - an import starting with
# Prefix names it: the exact spelling plus its ecosystem's separator
# ("github.com/spf13/cobra/", "next/", "django.", "tokio::"), so a root never
# matches a longer name ("react" is not "react-dom").
# Producer: internal/system/shard_expert_facts.go from init's technology
# vocabulary (nerdinit.TechnologyImportRoots, internal/init/
# technology_vocabulary.go) -- Go, npm, Python and Rust -- so the key a
# manifest created an agent for is the key a file's import matches.
Decl technology_import_exact(Import, Technology) bound [/string, /name].
Decl technology_import_prefix(Prefix, Technology) bound [/string, /name].

# expert_task(Task, Verb) - a task about to run, by an id unique to it, and the
# verb it runs as. Asserted for the duration of one selection, then retracted.
# Producer: internal/experts.Open.
Decl expert_task(Task, Verb) bound [/string, /name].

# expert_task_target(Task, Path) - a workspace path the task names: a file or
# a package directory, in canonical (workspace-relative, slash) form, the
# identity file_topology, file_dir and dependency_link carry.
# Producer: internal/experts.Open.
Decl expert_task_target(Task, Path) bound [/string, /string].

# expert_task_extension(Task, File, Ext) - the lower-case extension (with its
# dot) of a task target that has one, measured by Go. Keyed by the task like
# every other selection input, so a selection's cleanup removes only what it
# asserted: an untagged file_extension(File, Ext) was one tuple shared by every
# selection on that file, and the first to finish retracted it from the others.
# Producer: internal/experts.Select.
Decl expert_task_extension(Task, File, Ext) bound [/string, /string, /string].

# --- Derived (IDB, policy/shard_experts.mg) ----------------------------------

# expert_task_file(Task, File) - a file the task is about: the target itself,
# or a scanned file in the target directory.
Decl expert_task_file(Task, File) bound [/string, /string].

# task_technology(Task, File, Technology, Via) - a technology a task file uses.
# Via: /language (the file's language) or /import (an import under a
# technology_import_prefix).
Decl task_technology(Task, File, Technology, Via) bound [/string, /string, /name, /name].

# reads_code_verb(Verb) - a conversational verb whose answer still reads the
# target's code (explaining a policy file needs the engine's rules).
Decl reads_code_verb(Verb) bound [/name].

# expert_blind_verb(Verb) - a verb whose answer reads no code, so no shard
# agent's knowledge is needed for it whatever its target.
Decl expert_blind_verb(Verb) bound [/name].

# expert_needed(Task, Agent, Reason) - the task needs this shard agent. Reason
# is the technology that joins the task's files to the agent's domain.
# Consumers: internal/experts (session executor compile, chat delegation).
Decl expert_needed(Task, Agent, Reason) bound [/string, /name, /name].

# expert_file(Task, Agent, File) - a task file the agent's domain covers.
Decl expert_file(Task, Agent, File) bound [/string, /name, /string].

# expert_evidence_confidence(Via, Confidence) - how strongly one kind of
# evidence ties a file to an expert, 0-100. A framework import names the
# expert's exact subject; a language match is the expert's broad field.
Decl expert_evidence_confidence(Via, Confidence) bound [/name, /number].

# expert_evidence(Task, Agent, Technology, Confidence) - one piece of evidence
# for expert_needed, scored.
Decl expert_evidence(Task, Agent, Technology, Confidence) bound [/string, /name, /name, /number].

# expert_confidence(Task, Agent, Confidence) - the strongest evidence for the
# expert on this task. specialist_match (schemas_tools.mg) is its projection.
Decl expert_confidence(Task, Agent, Confidence) bound [/string, /name, /number].

# --- Recording derived domains (boot) -----------------------------------------

# shard_agent_recorded(Agent) - the agent's agents.json entry carries a
# `domains` field (possibly empty): its domains were decided once and are read
# back as /registry rows. Producer: internal/system/shard_expert_facts.go.
Decl shard_agent_recorded(Agent) bound [/name].

# shard_agent_domain_to_record(Agent, Technology) - a domain boot derived for
# a roster agent whose entry has no `domains` yet (the init catalog's key from
# the saved orientation, a KB tag). Boot writes these back to agents.json, so
# the derivation runs once. Consumer: internal/system/shard_expert_facts.go.
Decl shard_agent_domain_to_record(Agent, Technology) bound [/name, /name].

# --- Spawning (per task) --------------------------------------------------------

# expert_task_speaker(Task, Agent) - the task already runs as this shard agent
# (verb /consult/<agent> or /<agent>). Producer: internal/experts.Select.
Decl expert_task_speaker(Task, Agent) bound [/string, /name].

# expert_task_spoken(Task) - the task runs as some shard agent.
Decl expert_task_spoken(Task) bound [/string].

# expert_spawn(Task, Agent, Mode) - the harness consults this shard agent
# before the task's turn runs, and the answer enters the turn. Mode:
#   /lead   - an executor expert routed to execute: it lays out the work
#   /advise - an advisor routed to advise: it says what must hold
# Consumer: internal/session (Executor.consultExperts) through the
# JITExecutor's consultation spawner.
Decl expert_spawn(Task, Agent, Mode) bound [/string, /name, /name].

# --- Task complexity (per task) -------------------------------------------------
# task_complexity(Task, Level) (schemas_shards.mg) is derived here from the
# task's measured shape; strategic_advisor_required (policy/shards.mg) reads it.

# complex_verb(Verb) - a verb whose work is complex whatever its size.
Decl complex_verb(Verb) bound [/name].

# expert_task_file_count(Task, N) - how many files the task is about.
Decl expert_task_file_count(Task, N) bound [/string, /number].

# expert_task_file_symbol(Task, File, Symbol) - a symbol the world model says a
# task file defines.
Decl expert_task_file_symbol(Task, File, Symbol) bound [/string, /string, /string].

# expert_task_file_symbols(Task, File, N) - how many symbols a task file defines.
Decl expert_task_file_symbols(Task, File, N) bound [/string, /string, /number].

# expert_task_complex(Task) - some measure makes the task complex.
Decl expert_task_complex(Task) bound [/string].

# --- Critique (per task, then per turn) -------------------------------------

# expert_critique(Task, Agent) - a shard agent the task spawns must also
# review the change before the turn's verdict: the verb's execution mode is
# /advisory_with_critique (policy/delegation.mg). Read by internal/experts.
Decl expert_critique(Task, Agent) bound [/string, /name].

# turn_expert_critic(Turn, Agent) - this turn owes the agent's review: the
# executor records it from the turn's selection (expert_critique) when the
# turn starts. Producer: internal/session (Executor.recordExpertCritics).
Decl turn_expert_critic(Turn, Agent) bound [/name, /name].

# turn_expert_finding(Turn, Agent, Idx, Severity) - one finding of an agent's
# review that cites a line the turn changed, by its index in the round.
# Producer: internal/session (Executor.reviewWithShardExperts).
Decl turn_expert_finding(Turn, Agent, Idx, Severity) bound [/name, /name, /number, /name].

# turn_expert_actionable(Turn, Idx) - a finding the repair round carries.
Decl turn_expert_actionable(Turn, Idx) bound [/name, /number].
