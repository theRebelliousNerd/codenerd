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

# Link evidence stays distinct from embedding centrality in read reasons.
# doc_central also recognizes embedding hubs below.
link_hub(Path) :-
    link_in_count(Path, N), config_param(/orient_centrality_min_links, Min), N >= Min.

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

# C3: early birth is a candidate, not an origin verdict.
doc_origin_evidence(Path) :- doc_has_successor(Path).
doc_origin_evidence(Path) :- doc_central(Path).
doc_origin_evidence(Path) :- instruction_at_repo_root(Path).
doc_origin_evidence(Path) :- instruction_at_scope_root(Path).
doc_generation(Path, /origin) :- doc_early_birth(Path), doc_origin_evidence(Path).
doc_generation(Path, /early) :- doc_early_birth(Path), !doc_origin_evidence(Path).

# Embedding hubs preserve old central sources even with sparse Markdown links.
doc_central(Path) :-
    sim_degree(Path, N), config_param(/orient_similar_top_k, K),
    config_param(/orient_embedding_hub_top_k_multiple, Multiple),
    Min = fn:mult(K, Multiple), N >= Min.

# Duplicate and cohort components share a read slot. Minimum labels propagate
# through every component; greater labels need not reach a smaller node.
# Digest stars use one
# canonical ordinal instead of every pair of equal bodies.
digest_first_ord(Digest, Ord) :-
    doc_body_digest(Path, Digest), doc_tie(Path, O)
    |> do fn:group_by(Digest), let Ord = fn:min(O).
read_connection(Path, Anchor) :-
    digest_first_ord(Digest, Ord), doc_tie(Anchor, Ord),
    doc_body_digest(Path, Digest), Path != Anchor.
read_connection(A, B) :-
    doc_similar(A, B, P), config_param(/orient_near_duplicate_permille, Floor),
    P >= Floor, A != B.
read_connection(Path, Root) :- cohort_member(Path, Root), Path != Root.
read_edge(A, B) :- read_connection(A, B).
read_edge(B, A) :- read_connection(A, B).
read_has_connection(Path) :- read_edge(Path, _).
read_reach(A, A) :- read_edge(A, _).
read_reach(A, B) :-
    read_edge(A, B), doc_tie(A, AO), doc_tie(B, BO), BO < AO.
read_reach(A, C) :-
    read_edge(A, B), read_reach(B, C),
    doc_tie(A, AO), doc_tie(C, CO), CO < AO.
read_component_ord(Path, Ord) :-
    read_reach(Path, Other), doc_tie(Other, O)
    |> do fn:group_by(Path), let Ord = fn:min(O).
read_component(Path, Root) :- read_component_ord(Path, Ord), doc_tie(Root, Ord).
read_has_reason(Path) :- read_reason(Path, _).
# A cohort's internally linked representative wins within its read unit.
# The bonus sits above every validated raw read score and ranking suffix.
read_rep_score(Root, Path, S) :-
    read_component(Path, Root), read_member_key(Path, K), is_cohort_rep(Path),
    S = fn:plus(K, 10000000000000000).
read_rep_score(Root, Path, K) :-
    read_component(Path, Root), read_member_key(Path, K), !is_cohort_rep(Path).
read_rep_best(Root, S) :-
    read_rep_score(Root, Path, Score)
    |> do fn:group_by(Root), let S = fn:max(Score).
read_slot_rep(Path) :-
    read_rep_score(Root, Path, S), read_rep_best(Root, S).
read_slot_rep(Path) :-
    read_has_reason(Path), !read_has_connection(Path).
read_unit_reason(Rep, Reason) :-
    read_slot_rep(Rep), read_component(Rep, Root),
    read_component(Member, Root), read_reason(Member, Reason).
read_unit_reason(Path, Reason) :-
    read_slot_rep(Path), !read_has_connection(Path), read_reason(Path, Reason).
orient_read_member(Member, Rep) :-
    read_slot_rep(Rep), read_component(Rep, Root),
    read_component(Member, Root), Member != Rep.
