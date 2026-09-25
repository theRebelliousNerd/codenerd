# Retrieval Decisions
#
# Whether a turn runs the issue-driven sparse pass, and which of the files it
# found the model is handed as starting points, are the kernel's to decide
# (Decls: schemas_knowledge.mg section 52.5; Go: internal/retrieval
# decisions.go). Until 2026-09-25 the first was a verb switch in
# cmd/nerd/chat/process_seed.go that no other entry path consulted -- `nerd
# fix` never retrieved at all -- and the second was decided nowhere: the tiered
# files reached the chat compressor's activation scores and no model.

# The intent verbs whose request reads as an issue: a symptom to fix or debug,
# a finding to review or audit.
issue_retrieval_verb(/fix).
issue_retrieval_verb(/debug).
issue_retrieval_verb(/review).
issue_retrieval_verb(/security).

# Run the sparse pass for this intent. Asked for chat's /current_intent
# (process_seed.go) and for every task intent the session executor asserts
# (session/issue_retrieval.go).
issue_retrieval_wanted(Intent) :-
    user_intent(Intent, _, Verb, _, _),
    issue_retrieval_verb(Verb).

# The files handed to the model: every file the issue named, and a searched
# file only at or above retrieval.brief_min_relevance (percent). Without the
# threshold the second rule derives nothing and the brief holds the named
# files alone -- a floor fails closed, so it is not declared required (the
# working set refuses to build while any required key is missing).
retrieval_brief_file(Issue, File, /tier1, Relevance) :-
    tiered_context_file(Issue, File, /tier1, Relevance, _).

retrieval_brief_file(Issue, File, Tier, Relevance) :-
    tiered_context_file(Issue, File, Tier, Relevance, _),
    config_param(/retrieval_brief_min_relevance, Floor),
    Relevance >= Floor.
