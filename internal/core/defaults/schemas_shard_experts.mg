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
#   /catalog        - the init catalog's key for it, from the saved orientation
#   /agent_name     - derived (policy/shard_experts.mg): a word of the agent's
#                     name is a language the extension table knows, a
#                     technology the vocabulary names, or a concept term
# Producer of the first three: Go. /agent_name rows are derived.
Decl shard_agent_domain(Agent, Technology, Source) bound [/name, /name, /name].

# shard_agent_name_part(Agent, Part) - a lower-case word of the agent's name
# ("mangle-logic-architect" -> /mangle, /logic, /architect), split by Go
# (nerdinit.AgentNameParts). Matching a word to a domain is the kernel's call.
# Producer: internal/system/shard_expert_facts.go at boot.
Decl shard_agent_name_part(Agent, Part) bound [/name, /name].

# domain_concept_term(Term, Domain) - a word that names a concept domain
# (/security, /testing): a concept no import or file extension names.
# domain_concept_verb(Domain, Verb) - a verb whose tasks need the concept's
# experts (/security -> /security).
# Producer: internal/system/shard_expert_facts.go from nerdinit.ConceptDomains.
Decl domain_concept_term(Term, Domain) bound [/name, /name].
Decl domain_concept_verb(Domain, Verb) bound [/name, /name].

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

# --- Consultation routing (per ask: one batch of consultations) --------------
# Producer of the EDB: internal/session (Executor.routeConsultations), which
# asserts them under one ask id, reads consult_route and retracts them.

# consult_ask(Ask, Agent, Mode) - a consultation expert_spawn derived.
Decl consult_ask(Ask, Agent, Mode) bound [/string, /name, /name].
# consult_ask_complex(Ask) - the task is complex (task_complexity /high).
Decl consult_ask_complex(Ask) bound [/string].
# consult_knowledge_delivered(Ask, Agent, Rows) - the compile's prompt carries
# Rows of the agent's knowledge rows.
Decl consult_knowledge_delivered(Ask, Agent, Rows) bound [/string, /name, /number].
# consult_card_cached(Ask, Agent, Disposition) - a card for the same question
# (agent, mode, verb, files, request) is held, with how it ended.
Decl consult_card_cached(Ask, Agent, Disposition) bound [/string, /name, /name].
# consult_card_escalated(Ask, Agent) - that card was asked under the stronger
# consultation profile.
Decl consult_card_escalated(Ask, Agent) bound [/string, /name].
# consult_card_file_digest(Ask, Agent, File, Digest) - a file's digest when the
# cached card was asked.
Decl consult_card_file_digest(Ask, Agent, File, Digest) bound [/string, /name, /string, /string].
# consult_file_digest(Ask, File, Digest) - the file's digest now.
Decl consult_file_digest(Ask, File, Digest) bound [/string, /string, /string].
# consult_escalation_available(Ask) - the stronger consultation profile is
# configured (shard_profiles.consultation_escalation).
Decl consult_escalation_available(Ask) bound [/string].

# Derived.
Decl consult_file_measured(Ask, File) bound [/string, /string].
Decl consult_card_stale(Ask, Agent) bound [/string, /name].
Decl consult_card_usable(Ask, Agent) bound [/string, /name].
Decl consult_card_unanswered(Ask, Agent) bound [/string, /name].
Decl consult_escalates(Ask, Agent) bound [/string, /name].
Decl consult_knowledge_suffices(Ask, Agent) bound [/string, /name].
Decl consult_settled(Ask, Agent) bound [/string, /name].
# consult_route(Ask, Agent, Route) - how the consultation is settled: /reuse,
# /knowledge, /escalate, /consult or /unresolved. Consumer: internal/session
# (Executor.withExpertConsultations).
Decl consult_route(Ask, Agent, Route) bound [/string, /name, /name].

# --- Consultation cards (per turn, dealt per planned step) ------------------

# turn_expert_card(Turn, Agent, Mode, Disposition) - a consultation this turn
# bought: the agent, its spawn mode (/lead, /advise) and how it ended
# (/answered, /failed, /empty). Producer: internal/session
# (Executor.withExpertConsultations).
Decl turn_expert_card(Turn, Agent, Mode, Disposition) bound [/name, /name, /name, /name].

# turn_expert_card_file(Turn, Agent, File) - a canonical task file the card's
# agent was selected for (experts.Need.Files). Producer: as turn_expert_card.
Decl turn_expert_card_file(Turn, Agent, File) bound [/name, /name, /string].

# turn_step_file(Turn, File) - a planned step of the turn edits File
# (canonical). Producer: internal/session (Executor.stepSystemPrompt).
Decl turn_step_file(Turn, File) bound [/name, /string].

# turn_card_dealt(Turn, File, Agent) - the agent's answered card goes into the
# prompt of the step that edits File. Consumer: internal/session
# (Executor.dealtCards).
Decl turn_card_dealt(Turn, File, Agent) bound [/name, /string, /name].

# turn_card_handoff(Turn, Agent, Idx, From, To) - the agent's card names a
# hand-off: a change in From obliges something of To (both canonical). Idx
# orders the card's hand-offs. Producer: internal/session
# (Executor.recordCards, from the card's HANDOFF lines).
Decl turn_card_handoff(Turn, Agent, Idx, From, To) bound [/name, /name, /number, /string, /string].

# turn_plan_file(Turn, File) - the turn's step plan edits File (canonical).
# Producer: internal/session (Executor.reconcileHandoffs).
Decl turn_plan_file(Turn, File) bound [/name, /string].

# turn_handoff_target_exists(Turn, File) - a hand-off's consumer is a file on
# disk. Producer: as turn_plan_file.
Decl turn_handoff_target_exists(Turn, File) bound [/name, /string].

# turn_handoff_answered(Turn, Agent, Idx, From, To) - a hand-off whose card
# answered.
Decl turn_handoff_answered(Turn, Agent, Idx, From, To) bound [/name, /name, /number, /string, /string].

# turn_handoff_dealt(Turn, File, Agent, Idx) - the hand-off goes into the
# prompt of the step on either of its ends. Consumer: Executor.dealtCards.
Decl turn_handoff_dealt(Turn, File, Agent, Idx) bound [/name, /string, /name, /number].

# turn_handoff_unplanned(Turn, Agent, Idx, To) - an answered hand-off leaves a
# planned file for a file the plan does not edit.
Decl turn_handoff_unplanned(Turn, Agent, Idx, To) bound [/name, /name, /number, /string].

# turn_handoff_step(Turn, Agent, Idx, To) - the unplanned consumer exists, so
# the plan gains a step on it. Consumer: Executor.reconcileHandoffs.
Decl turn_handoff_step(Turn, Agent, Idx, To) bound [/name, /name, /number, /string].

# turn_handoff_unresolved(Turn, Agent, Idx, To) - the unplanned consumer is not
# a file on disk: the hand-off is reported as unresolved, not guessed at.
# Consumer: Executor.reconcileHandoffs.
Decl turn_handoff_unresolved(Turn, Agent, Idx, To) bound [/name, /name, /number, /string].
