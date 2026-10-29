# Shard-agent knowledge is never static: this file decides, over what Go
# measured of every agent KB (schemas_kb_refresh.mg), what to research again
# and what to prune. The background refresh (internal/system/
# knowledge_refresh.go) does what it derives, within knowledge_refresh_budget.

# --- Constants ----------------------------------------------------------------

# Web answers age fastest; Context7 follows library releases; the repository's
# own importers follow the code. go doc has no age: it is re-read when the
# version go.mod pins changes.
knowledge_source_max_age("web", 30).
knowledge_source_max_age("context7", 90).
knowledge_source_max_age("repo/world", 7).

knowledge_retry_after_days(1).
knowledge_refresh_budget(4).
# Two minutes in, then hourly: a one-shot run is over before it starts, a
# session keeps its experts current while it works.
knowledge_refresh_schedule(120, 3600).

knowledge_refresh_weight(/gap, 5).
knowledge_refresh_weight(/unlearned, 4).
knowledge_refresh_weight(/new_source, 3).
knowledge_refresh_weight(/stale, 2).
knowledge_refresh_weight(/retry, 1).

knowledge_prunable_kind("web").
knowledge_prunable_kind("context7").
knowledge_prunable_kind("godoc").
knowledge_prunable_kind("repo").
knowledge_prune_after_days(60).
knowledge_prune_min_consultations(20).

# --- What the KBs hold ----------------------------------------------------------

knowledge_acquired(Agent, Subject) :-
    knowledge_outcome(Agent, Subject, _, /acquired, _).

knowledge_recently_attempted(Agent, Subject) :-
    knowledge_outcome(Agent, Subject, _, _, Age),
    knowledge_retry_after_days(Window),
    Age < Window.

knowledge_agent_measured(Agent) :- knowledge_wanted(Agent, _).
knowledge_agent_measured(Agent) :- knowledge_atom_held(Agent, _, _, _).

knowledge_concept_retrieved(Agent, Concept) :-
    knowledge_atom_retrieved(Agent, Concept, _).

knowledge_agent_consulted(Agent) :-
    knowledge_atom_retrieved(Agent, _, _).

knowledge_agent_use(Agent, Uses) :-
    knowledge_atom_retrieved(Agent, _, Count)
    |> do fn:group_by(Agent), let Uses = fn:sum(Count).

knowledge_agent_use(Agent, 0) :-
    knowledge_agent_measured(Agent),
    !knowledge_agent_consulted(Agent).

# --- What to research ------------------------------------------------------------

# Wanted and never learned, not tried today.
knowledge_refresh(Agent, Subject, /unlearned) :-
    knowledge_wanted(Agent, Subject),
    !knowledge_acquired(Agent, Subject),
    !knowledge_recently_attempted(Agent, Subject).

# An acquisition past its source's age.
knowledge_refresh(Agent, Subject, /stale) :-
    knowledge_outcome(Agent, Subject, Source, /acquired, Age),
    knowledge_source_max_age(Source, Max),
    Age > Max.

# A source that could not run on the subject can now.
knowledge_refresh(Agent, Subject, /new_source) :-
    knowledge_outcome(Agent, Subject, Source, /unavailable, _),
    knowledge_source_available(Source).

# A failed attempt, past the retry window.
knowledge_refresh(Agent, Subject, /retry) :-
    knowledge_outcome(Agent, Subject, Source, /failed, Age),
    knowledge_source_available(Source),
    knowledge_retry_after_days(Window),
    Age >= Window.

# A question a consultation found nothing for, not yet answered by any
# source, not tried today.
knowledge_refresh(Agent, Query, /gap) :-
    knowledge_miss(Agent, Query, _),
    !knowledge_acquired(Agent, Query),
    !knowledge_recently_attempted(Agent, Query).

knowledge_refresh_priority(Agent, Subject, Weight, Uses) :-
    knowledge_refresh(Agent, Subject, Reason),
    knowledge_refresh_weight(Reason, W),
    knowledge_agent_use(Agent, Uses)
    |> do fn:group_by(Agent, Subject, Uses), let Weight = fn:max(W).

# --- What to prune ---------------------------------------------------------------

# An acquired row nobody used: old, never retrieved, from an agent consulted
# often enough that the silence means something. It can be acquired again.
knowledge_prune(Agent, Concept, /never_retrieved) :-
    knowledge_atom_held(Agent, Concept, Kind, Age),
    !knowledge_concept_retrieved(Agent, Concept),
    knowledge_prunable_kind(Kind),
    knowledge_prune_after_days(Max),
    Age > Max,
    knowledge_agent_use(Agent, Uses),
    knowledge_prune_min_consultations(Min),
    Uses >= Min.
