# Document lineage: similarity across time, link and embedding centres,
# instruction files, the latest member of an evolution chain.
# Theme compatibility is three positive shapes. A negated pair
# (!shared_theme(A, B), or !doc_theme(A, _)) does not exclude in this fork
# when a variable is wild or only bound on one side, so a document with no
# theme is a single-arg doc_has_theme projection instead.

doc_has_theme(Path) :-
    doc_theme(Path, _).

shared_theme(A, B) :-
    doc_theme(A, T),
    doc_theme(B, T),
    A != B.

# doc_similar is stored with A < B, which is not time order. Both
# orientations are tried; equal birth timestamps are neither.
time_ordered_similar(Old, New) :-
    history_usable(),
    doc_similar(Old, New, P),
    config_param(/orient_similarity_floor_permille, Floor),
    P >= Floor,
    repo_file_history(Old, FirstOld, _, _, _),
    repo_file_history(New, FirstNew, _, _, _),
    FirstOld < FirstNew.

time_ordered_similar(Old, New) :-
    history_usable(),
    doc_similar(New, Old, P),
    config_param(/orient_similarity_floor_permille, Floor),
    P >= Floor,
    repo_file_history(Old, FirstOld, _, _, _),
    repo_file_history(New, FirstNew, _, _, _),
    FirstOld < FirstNew.

doc_evolved_into(Old, New) :-
    time_ordered_similar(Old, New),
    doc_file(Old, _, _, _),
    doc_file(New, _, _, _),
    shared_theme(Old, New).

doc_evolved_into(Old, New) :-
    time_ordered_similar(Old, New),
    doc_file(Old, _, _, _),
    doc_file(New, _, _, _),
    !doc_has_theme(Old).

doc_evolved_into(Old, New) :-
    time_ordered_similar(Old, New),
    doc_file(Old, _, _, _),
    doc_file(New, _, _, _),
    !doc_has_theme(New).

doc_has_successor(Path) :-
    doc_evolved_into(Path, _).

doc_has_predecessor(Path) :-
    doc_evolved_into(_, Path).

# The latest member of a chain has a predecessor and no successor. A fork
# (one older document, two later ones) keeps both tips.
chain_latest(Path) :-
    doc_has_predecessor(Path),
    !doc_has_successor(Path).

# --- centres ----------------------------------------------------------------

link_in_count(Path, N) :-
    doc_link(From, Path)
    |> do fn:group_by(Path), let N = fn:count().

has_link_in(Path) :-
    link_in_count(Path, _).

sim_neighbour(Path, Other) :-
    doc_similar(Path, Other, _).

sim_neighbour(Path, Other) :-
    doc_similar(Other, Path, _).

sim_degree(Path, N) :-
    sim_neighbour(Path, Other)
    |> do fn:group_by(Path), let N = fn:count().

has_sim_degree(Path) :-
    sim_degree(Path, _).

centrality(Path, N) :-
    sim_degree(Path, S),
    link_in_count(Path, L),
    N = fn:plus(S, L).

centrality(Path, S) :-
    sim_degree(Path, S),
    !has_link_in(Path).

centrality(Path, L) :-
    link_in_count(Path, L),
    !has_sim_degree(Path).

has_centrality(Path) :-
    centrality(Path, _).

# Capped so the read-rank key can give centrality its own digits without
# running into the score. Missing centrality is zero only for a document
# that is actually in a cluster; everyone else stays unbound here and the
# read rank treats "no rank" as zero on its own.
rank_centrality(Path, C) :-
    centrality(Path, N),
    N < 1000,
    C = N.

rank_centrality(Path, 999) :-
    centrality(Path, N),
    N >= 1000.

rank_centrality(Path, 0) :-
    doc_in_cluster(Path),
    !has_centrality(Path).

has_rank_centrality(Path) :-
    rank_centrality(Path, _).

doc_in_cluster(Path) :-
    doc_cluster(Path, _).

doc_has_tie(Path) :-
    doc_tie(Path, _).

# Within a component the representative is the highest capped centrality,
# then the earlier path. 1e8 sits strictly above the ordinal complement
# (0..99999999), so one more inbound neighbour beats any path order.
cluster_score(Path, Score) :-
    doc_cluster(Path, _),
    rank_centrality(Path, C),
    doc_tie(Path, Ord),
    Ord <= 99999999,
    Scaled = fn:mult(C, 100000000),
    Inv = fn:minus(99999999, Ord),
    Score = fn:plus(Scaled, Inv).

cluster_score(Path, Score) :-
    doc_cluster(Path, _),
    rank_centrality(Path, C),
    doc_tie(Path, Ord),
    Ord > 99999999,
    Score = fn:mult(C, 100000000).

cluster_scored(Cluster, Score) :-
    doc_cluster(Path, Cluster),
    cluster_score(Path, Score).

cluster_best(Cluster, Best) :-
    cluster_scored(Cluster, Score)
    |> do fn:group_by(Cluster), let Best = fn:max(Score).

cluster_rep(Path) :-
    doc_cluster(Path, Cluster),
    cluster_score(Path, Score),
    cluster_best(Cluster, Score).

# The same inbound-link threshold as doc_central. One rule, so the two
# names cannot drift. link_hub is the read reason; doc_central is what
# "quiet" is the complement of.
link_hub(Path) :-
    doc_central(Path).

live_link(From, To) :-
    doc_live(From),
    doc_link(From, To),
    From != To.

live_link_in(Path, N) :-
    live_link(From, Path)
    |> do fn:group_by(Path), let N = fn:count().

live_hub(Path) :-
    live_link_in(Path, N),
    config_param(/orient_live_hub_min_links, Min),
    N >= Min.

# --- instruction roots ------------------------------------------------------
# No path vocabulary. A root instruction is an agent_source of kind
# /instructions whose path contains no slash (the repository root), or one
# whose doc_file directory is the scope directory (a subtree root). A
# nested instruction that is not itself a document has no directory
# measurement this policy can join, so it is not a subtree root.

instruction_path_has_slash(Path) :-
    agent_source(_, _, /instructions, _, Path, _),
    :string:contains(Path, "/").

instruction_at_repo_root(Path) :-
    agent_source(_, _, /instructions, _, Path, _),
    !instruction_path_has_slash(Path).

instruction_at_scope_root(Path) :-
    agent_source(ID, _, /instructions, _, Path, _),
    instruction_path_has_slash(Path),
    doc_file(Path, Dir, _, _),
    agent_source_scope(ID, Dir).
