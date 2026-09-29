# Orientation schema.
#
# This program is the executive for `nerd orient` and for init's picture of a
# foreign repository. Go asserts measurements (history, links, neighbour
# pairs, cluster ids, path ordinals, role claims). Every judgment — which
# months form an era, which document is live, which of a duplicate cluster
# is the one to read — is a rule here.
#
# config_param is declared in policy/config_params.mg, which the engine
# loads first. Do not redeclare it.
#
# Shared EDB below is declared once, in this file, so the engine loads
# before the lanes that assert those rows exist. Those lanes must not
# declare them again: a second Decl is a hard error in this fork.
#   I1 asserts agent_source/6, agent_source_digest/2, agent_source_topic/2,
#   agent_source_scope/2 and must not Decl them.
#   I2b asserts doc_role_claim/3 and doc_theme/2 and must not Decl them.
# I1's own derived predicates (agent_source_duplicate, agent_source_winner,
# orient_agent, orient_agent_knowledge) are not declared here; they arrive
# with I1's rules.

# --- thresholds -------------------------------------------------------------
# Every key the rules read is required. NewEngine refuses to start while
# config_param_missing(/orient, Key) derives, so a rule cannot quietly
# match nothing because a knob was absent.

config_param_required(/orient, /orient_similarity_floor_permille).
config_param_required(/orient, /orient_similar_top_k).
config_param_required(/orient, /orient_read_candidate_budget).
config_param_required(/orient, /orient_era_lull_commits).
config_param_required(/orient, /orient_burst_max_days).
config_param_required(/orient, /orient_burst_min_commits).
config_param_required(/orient, /orient_embedding_chunk_bytes).
config_param_required(/orient, /orient_cohort_min_docs).
config_param_required(/orient, /orient_centrality_min_links).
config_param_required(/orient, /orient_live_hub_min_links).
config_param_required(/orient, /orient_origin_span_permille).
config_param_required(/orient, /orient_early_span_permille).
config_param_required(/orient, /orient_middle_span_permille).

# --- measurements Go asserts ------------------------------------------------

Decl repo_file_history(Path, FirstUnix, LastUnix, Commits, ActiveDays) bound [/string, /number, /number, /number, /number].
Decl repo_month(MonthIndex, Label, Commits, FilesAdded, DocsAdded) bound [/number, /string, /number, /number, /number].
Decl repo_span(FirstUnix, LastUnix, TotalCommits, Shallow) bound [/number, /number, /number, /name].
# MonthIndex 0 is the month of the first commit. Spans abut and do not
# overlap (End + 1 = next Start) so a commit unix joins exactly one month.
# Go emits them because months are not equal length and fn:div of a unix
# timestamp is not a calendar month.
Decl repo_month_span(MonthIndex, StartUnix, EndUnix) bound [/number, /number, /number].

Decl doc_file(Path, Dir, Bytes, Headings) bound [/string, /string, /number, /number].
Decl doc_link(FromPath, ToPath) bound [/string, /string].
# A < B lexicographically. Only pairs Go kept: at or above the floor and
# inside some document's top-k. The floor is rechecked here.
Decl doc_similar(A, B, Permille) bound [/string, /string, /number].
# ClusterID is the lexicographic minimum path in the similarity component.
# Which member represents the cluster is rank_centrality, not this id.
Decl doc_cluster(Path, ClusterID) bound [/string, /string].
# Lexicographic ordinal of every path that can enter the read ranking.
# Unique, so a tied score still cuts the budget on an earlier path.
Decl doc_tie(Path, Ord) bound [/string, /number].

Decl agent_source(ID, Tool, Kind, Name, Path, Tracked) bound [/string, /name, /name, /string, /string, /name].
Decl agent_source_digest(ID, Digest) bound [/string, /string].
Decl agent_source_topic(ID, Topic) bound [/string, /string].
Decl agent_source_scope(ID, Dir) bound [/string, /string].

Decl doc_role_claim(Path, Role, ConfidencePct) bound [/string, /name, /number].
Decl doc_theme(Path, Theme) bound [/string, /string].

# --- judgments --------------------------------------------------------------

Decl history_usable().

Decl month_kind(MonthIndex, Kind) bound [/number, /name].
Decl has_later_month(MonthIndex) bound [/number].
Decl era_start(MonthIndex) bound [/number].
Decl era_end(MonthIndex) bound [/number].
Decl era_start_reached(MonthIndex, Start) bound [/number, /number].
Decl era_starts_upto(MonthIndex, Count) bound [/number, /number].
Decl era_index_of(MonthIndex, EraIndex) bound [/number, /number].
Decl era_count(N) bound [/number].
Decl repo_era(EraIndex, StartMonth, EndMonth, Kind) bound [/number, /number, /number, /name].

Decl doc_birth_month(Path, MonthIndex) bound [/string, /number].
Decl doc_touch_month(Path, MonthIndex) bound [/string, /number].
Decl doc_birth_era(Path, EraIndex) bound [/string, /number].
Decl doc_at_permille(Path, Permille) bound [/string, /number].
Decl doc_touch_permille(Path, Permille) bound [/string, /number].
Decl doc_generation(Path, Gen) bound [/string, /name].
Decl doc_has_generation(Path) bound [/string].

Decl doc_burst(Path) bound [/string].
Decl doc_birth_day(Path, Day) bound [/string, /number].
Decl cohort_size(Day, N) bound [/number, /number].
Decl doc_cohort(Path) bound [/string].

Decl doc_has_history(Path) bound [/string].
Decl doc_has_tie(Path) bound [/string].
Decl doc_in_cluster(Path) bound [/string].
Decl doc_has_theme(Path) bound [/string].
Decl shared_theme(A, B) bound [/string, /string].
Decl time_ordered_similar(Old, New) bound [/string, /string].
Decl doc_evolved_into(Old, New) bound [/string, /string].
Decl doc_has_successor(Path) bound [/string].
Decl doc_has_predecessor(Path) bound [/string].
Decl chain_latest(Path) bound [/string].

Decl link_in_count(Path, N) bound [/string, /number].
Decl has_link_in(Path) bound [/string].
Decl sim_neighbour(Path, Other) bound [/string, /string].
Decl sim_degree(Path, N) bound [/string, /number].
Decl has_sim_degree(Path) bound [/string].
Decl centrality(Path, N) bound [/string, /number].
Decl has_centrality(Path) bound [/string].
Decl rank_centrality(Path, C) bound [/string, /number].
Decl has_rank_centrality(Path) bound [/string].
Decl cluster_score(Path, Score) bound [/string, /number].
Decl cluster_scored(Cluster, Score) bound [/string, /number].
Decl cluster_best(Cluster, Best) bound [/string, /number].
Decl cluster_rep(Path) bound [/string].
Decl doc_central(Path) bound [/string].
Decl link_hub(Path) bound [/string].
Decl live_link(From, To) bound [/string, /string].
Decl live_link_in(Path, N) bound [/string, /number].
Decl live_hub(Path) bound [/string].

Decl instruction_path_has_slash(Path) bound [/string].
Decl instruction_at_repo_root(Path) bound [/string].
Decl instruction_at_scope_root(Path) bound [/string].

Decl doc_recent_touch(Path) bound [/string].
Decl doc_live_base(Path) bound [/string].
Decl doc_quiet(Path) bound [/string].
Decl doc_superseded(Old, By) bound [/string, /string].
Decl doc_has_superseded(Path) bound [/string].
Decl doc_live(Path) bound [/string].
Decl origin_source(Path, Why) bound [/string, /name].

Decl vision_claim(Path, Conf) bound [/string, /number].
Decl vision_conf(Path, Conf) bound [/string, /number].
Decl vision_part_conf(Path, Points) bound [/string, /number].
Decl vision_part_role(Path, Points) bound [/string, /number].
Decl vision_part_gen(Path, Points) bound [/string, /number].
Decl vision_part_burst(Path, Points) bound [/string, /number].
Decl vision_part_central(Path, Points) bound [/string, /number].
Decl vision_part_live(Path, Points) bound [/string, /number].
Decl vision_raw(Path, Points) bound [/string, /number].
Decl vision_why_rank(Path, Rank) bound [/string, /number].
Decl vision_why_best(Path, Rank) bound [/string, /number].
Decl vision_why(Path, Why) bound [/string, /name].
Decl vision_source(Path, WeightPct, Why) bound [/string, /number, /name].

# Reason weights are discernment, not a config threshold: the policy is what
# says an instruction outranks an origin. Summed when a document qualifies
# more than once.
Decl reason_weight(Reason, Weight) bound [/name, /number].
Decl read_reason(Path, Reason) bound [/string, /name].
Decl read_weighted(Path, Weight) bound [/string, /number].
Decl read_score(Path, Score) bound [/string, /number].
Decl read_cent_of(Path, C) bound [/string, /number].
Decl read_last_of(Path, Last) bound [/string, /number].
Decl read_ord_of(Path, Ord) bound [/string, /number].
Decl read_tie_of(Path, Tie) bound [/string, /number].
Decl read_key(Path, Key) bound [/string, /number].
Decl read_rank_higher(Path, Other) bound [/string, /string].
Decl has_higher_rank(Path) bound [/string].
Decl higher_count(Path, N) bound [/string, /number].
Decl orient_read_kept(Path) bound [/string].
Decl orient_read_kept_path(Path) bound [/string].
Decl orient_read_candidate(Path, Reason) bound [/string, /name].
# Same reasons as the candidate, for every qualifying document the budget
# did not keep. The count is orient.read_candidate_budget.
Decl orient_read_omitted(Path, Reason) bound [/string, /name].
