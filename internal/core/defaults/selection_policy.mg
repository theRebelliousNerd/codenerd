# Document selection policy for large-spec ingestion
# Leverages doc_metadata and campaign_goal to prune irrelevant sources

# Exclude experimental docs unless explicitly requested
is_irrelevant(Path) :-
    doc_tag(Path, /experimental).

# Base relevance: matches goal topics
is_relevant(Path) :-
    doc_metadata(CampID, Path, _, _, _),
    doc_tag(Path, Topic),
    goal_topic(CampID, Topic),
    !is_irrelevant(Path).

# Pull in referenced documents transitively
include_in_context(Path) :- is_relevant(Path).
