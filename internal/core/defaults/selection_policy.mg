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

# Hard dependencies: propagate relevance through references
is_relevant(Path) :-
    doc_metadata(CampID, Path, _, _, _),
    doc_metadata(CampID, Parent, _, _, _),
    doc_reference(Parent, Path),
    is_relevant(Parent),
    !is_irrelevant(Path).

# Pull in referenced documents transitively
include_in_context(Path) :- is_relevant(Path).
include_in_context(Dep) :-
    include_in_context(Parent),
    doc_reference(Parent, Dep).
