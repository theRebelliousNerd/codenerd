# Shard-agent knowledge: what to research again, what to prune.
#
# A shard agent's knowledge base is filled by typed sources (internal/
# knowledge: grounded web search, Context7, the pinned go doc, the
# repository's own use), each attempt recorded with its outcome, and is
# consumed when a compile pushes it into a prompt. Go measures the KBs and the
# sources that can run; policy/knowledge_refresh.mg decides what is stale,
# what is missing and what is dead weight; the background refresh
# (internal/system/knowledge_refresh.go) does what it derives, within the
# budget the policy sets.

# --- Measured by Go (EDB) ---------------------------------------------------
# Producer for all of these: internal/system/knowledge_refresh.go, from each
# roster agent's KB (knowledge.Measure) and the registry, at boot.

# knowledge_wanted(Agent, Subject) - a subject the agent should know: one of
# its research topics, or a package the workspace pins for one of its
# technologies (nerdinit.KnowledgeSubjects). Subject is the acquisition key.
Decl knowledge_wanted(Agent, Subject) bound [/name, /string].

# knowledge_outcome(Agent, Subject, Source, Status, AgeDays) - a source's last
# attempt on a subject: Source is "web", "context7", "godoc" or "repo/world";
# Status /acquired, /empty, /failed, /unavailable (the source could not run)
# or /inapplicable (the source reads no such subject); AgeDays whole days ago.
Decl knowledge_outcome(Agent, Subject, Source, Status, AgeDays) bound [/name, /string, /string, /name, /number].

# knowledge_acquired_version(Agent, Subject, Version) - the manifest version
# a source acquired the subject at (knowledge_acquisitions.version).
Decl knowledge_acquired_version(Agent, Subject, Version) bound [/name, /string, /string].

# dependency_pin(Module, Version) - a version a workspace manifest pins now: a
# go.mod require or a package.json dependency (nerdinit.ManifestPins). Two
# manifests may pin two versions of one module.
Decl dependency_pin(Module, Version) bound [/string, /string].

# knowledge_source_available(Source) - a source that can run in this session
# (a configured client offers grounded search, Context7 is wired, a go
# toolchain and workspace exist, a world model is loaded).
Decl knowledge_source_available(Source) bound [/string].

# knowledge_atom_held(Agent, Concept, Kind, AgeDays) - a knowledge row the
# agent's KB holds (placeholders excluded), by its producer kind ("web",
# "context7", "godoc", "repo", or "(none)" for rows with no provenance) and
# age.
Decl knowledge_atom_held(Agent, Concept, Kind, AgeDays) bound [/name, /string, /string, /number].

# knowledge_atom_retrieved(Agent, Concept, Count) - how many consultations
# retrieved the row (a compile pushing the agent's knowledge).
Decl knowledge_atom_retrieved(Agent, Concept, Count) bound [/name, /string, /number].

# knowledge_miss(Agent, Query, Count) - a task's question consultations of the
# agent found no knowledge for, and how often.
Decl knowledge_miss(Agent, Query, Count) bound [/name, /string, /number].

# knowledge_miss_recovered(Agent, Query) - a compile delivered the agent's
# knowledge rows for the question after its latest miss: the miss is history,
# not a current gap. Producer: internal/knowledge (Measure).
Decl knowledge_miss_recovered(Agent, Query) bound [/name, /string].

# --- Policy constants (policy/knowledge_refresh.mg) -------------------------

# knowledge_source_max_age(Source, Days) - an acquisition older than Days is
# stale. A source with no row never goes stale by age (go doc is re-read when
# the pinned version changes).
Decl knowledge_source_max_age(Source, Days) bound [/string, /number].

# knowledge_retry_after_days(Days) - a subject attempted fewer days ago than
# this is not attempted again by the unlearned/failed rules.
Decl knowledge_retry_after_days(Days) bound [/number].

# knowledge_refresh_budget(Subjects) - the most subjects one refresh run
# acquires (each costs at most one grounded search and one Context7 fetch).
Decl knowledge_refresh_budget(Subjects) bound [/number].

# knowledge_refresh_schedule(DelaySeconds, IntervalSeconds) - the first run
# starts DelaySeconds after boot (a short-lived process spends nothing), then
# one run every IntervalSeconds for as long as the session lives.
# Consumer: internal/system/knowledge_refresh_boot.go.
Decl knowledge_refresh_schedule(DelaySeconds, IntervalSeconds) bound [/number, /number].

# knowledge_refresh_weight(Reason, Weight) - how urgent a refresh reason is.
Decl knowledge_refresh_weight(Reason, Weight) bound [/name, /number].

# knowledge_prunable_kind(Kind) - a producer whose rows can be acquired again,
# so an unused one may be pruned. Ingested and authored knowledge is not.
Decl knowledge_prunable_kind(Kind) bound [/string].

# knowledge_prune_after_days(Days) / knowledge_prune_min_consultations(N) - an
# acquired row is pruned when older than Days and never retrieved by an agent
# consulted at least N times (so absence of use is evidence).
Decl knowledge_prune_after_days(Days) bound [/number].
Decl knowledge_prune_min_consultations(Count) bound [/number].

# --- Derived (IDB) ----------------------------------------------------------

# knowledge_acquired(Agent, Subject) - some source acquired the subject.
Decl knowledge_acquired(Agent, Subject) bound [/name, /string].

# knowledge_recently_attempted(Agent, Subject) - some source tried it within
# the retry window.
Decl knowledge_recently_attempted(Agent, Subject) bound [/name, /string].

# knowledge_agent_use(Agent, Retrievals) - the agent's total retrievals, for
# every agent with a KB measured (0 for one never consulted).
Decl knowledge_agent_use(Agent, Retrievals) bound [/name, /number].

# knowledge_agent_measured(Agent) - an agent whose KB was measured.
Decl knowledge_agent_measured(Agent) bound [/name].

# knowledge_agent_consulted(Agent) - an agent some consultation retrieved from.
Decl knowledge_agent_consulted(Agent) bound [/name].

# knowledge_concept_retrieved(Agent, Concept) - a row some consultation used.
Decl knowledge_concept_retrieved(Agent, Concept) bound [/name, /string].

# knowledge_refresh(Agent, Subject, Reason) - acquire the subject again.
# Reason: /version_changed (acquired at a version no manifest pins any more),
# /unlearned (wanted, never acquired), /stale (an acquisition past its
# source's age), /new_source (a source that could not run now can), /retry
# (a failed attempt past the retry window), /gap (a question consultations
# found nothing for).
Decl knowledge_refresh(Agent, Subject, Reason) bound [/name, /string, /name].

# knowledge_refresh_priority(Agent, Subject, Weight, Uses) - a refresh subject's
# most urgent reason's weight and its agent's use: the background refresh
# takes the budget's worth in (Weight, Uses) order, heavily used agents first.
# Consumer: internal/system/knowledge_refresh.go.
Decl knowledge_refresh_priority(Agent, Subject, Weight, Uses) bound [/name, /string, /number, /number].

# knowledge_prune(Agent, Concept, Reason) - delete the row. Reason
# /never_retrieved: acquired, old, and unused by a consulted agent.
# Consumer: internal/system/knowledge_refresh.go.
Decl knowledge_prune(Agent, Concept, Reason) bound [/name, /string, /name].
