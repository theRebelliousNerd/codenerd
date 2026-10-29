# Orientation schema.
#
# This program is the executive for init and the OODA picture of a
# foreign repository. Go asserts measurements (history, links, neighbour
# pairs, cluster ids, path ordinals, role claims). Every judgment — which
# months form an era, which document is live, which of a duplicate cluster
# is the one to read — is a rule here.
#
# This isolated program owns its configuration declarations. It must not
# import the action kernel: the kernel loads its persisted projection.
Decl config_param(Key, Value) bound [/name, /number].
Decl config_param_required(Section, Key) bound [/name, /name].
Decl has_config_param(Key) bound [/name].
Decl config_param_missing(Section, Key) bound [/name, /name].
has_config_param(Key) :- config_param(Key, Value).
config_param_missing(Section, Key) :- config_param_required(Section, Key), !has_config_param(Key).
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
Decl repo_file_day(Path, Day) bound [/string, /number].
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

# Reason weights come from orient.read_weight_* (config_param rows). Each is
# scaled by reason_rarity, which falls as more documents hold the reason, so a
# signal most documents share cannot decide the ranking. Summed when a document
# qualifies more than once.
Decl reason_weight(Reason, Weight) bound [/name, /number].
Decl read_reason(Path, Reason) bound [/string, /name].
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

# C3 measurements and signal-quality judgments. Existing Decls are unchanged.
Decl doc_body_digest(Path, Digest) bound [/string, /string].
Decl doc_subtree(Path, Dir) bound [/string, /string].
Decl doc_embedding_status(Path, Status) bound [/string, /name].
Decl doc_embedding_omitted(Path, Kind, Detail) bound [/string, /name, /string].
Decl month_measure(Month, Commits) bound [/number, /number].
Decl month_before(Month, Other) bound [/number, /number].
Decl month_has_before(Month) bound [/number].
Decl month_rank(Month, Rank) bound [/number, /number].
Decl month_total(Count) bound [/number].
Decl month_middle(Lower, Upper) bound [/number, /number].
Decl month_median_twice(Value) bound [/number].
Decl month_lull(Month) bound [/number].
Decl doc_early_birth(Path) bound [/string].
Decl doc_origin_evidence(Path) bound [/string].
Decl document_path(Path) bound [/string].
Decl document_count(Count) bound [/number].
Decl doc_birth_window(Path, Window) bound [/string, /number].
Decl cohort_window_offset(K) bound [/number].
Decl cohort_directory_member(Dir, Window, Path) bound [/string, /number, /string].
Decl cohort_directory_count(Dir, Window, Count) bound [/string, /number, /number].
Decl cohort_directory_good(Dir, Window) bound [/string, /number].
Decl cohort_directory_ord(Dir, Window, Ord) bound [/string, /number, /number].
Decl cohort_connection(A, B, Window) bound [/string, /string, /number].
Decl cohort_edge(A, B, Window) bound [/string, /string, /number].
Decl cohort_reach(A, B, Window) bound [/string, /string, /number].
Decl cohort_root_ord(Path, Window, Ord) bound [/string, /number, /number].
Decl cohort_component(Path, Root, Window) bound [/string, /string, /number].
Decl cohort_member_count(Root, Window, Count) bound [/string, /number, /number].
Decl cohort_good(Root, Window) bound [/string, /number].
Decl cohort_member(Path, Root) bound [/string, /string].
Decl file_has_days(Path) bound [/string].
Decl cohort_touch_day(Root, Day) bound [/string, /number].
Decl cohort_active_days(Root, Days) bound [/string, /number].
Decl cohort_commit_touches(Root, Commits) bound [/string, /number].
Decl cohort_burst(Root) bound [/string].
Decl cohort_in_link(Root, Path, From) bound [/string, /string, /string].
Decl cohort_in_count(Root, Path, Count) bound [/string, /string, /number].
Decl cohort_has_in(Root, Path) bound [/string, /string].
Decl cohort_link_rank(Root, Path, Rank) bound [/string, /string, /number].
Decl cohort_member_score(Root, Path, Score) bound [/string, /string, /number].
Decl cohort_best(Root, Score) bound [/string, /number].
Decl cohort_rep(Root, Path) bound [/string, /string].
Decl is_cohort_rep(Path) bound [/string].
Decl orient_document(Path) bound [/string].
Decl orient_document_count(Count) bound [/number].
Decl signal_reason(Path, Reason) bound [/string, /name].
Decl reason_document_count(Reason, Count) bound [/name, /number].
Decl reason_rarity(Reason, Factor) bound [/name, /number].
Decl read_contribution(Path, Reason, Weight) bound [/string, /name, /number].
Decl read_has_reason(Path) bound [/string].
Decl read_member_key(Path, Key) bound [/string, /number].
Decl digest_first_ord(Digest, Ord) bound [/string, /number].
Decl read_connection(A, B) bound [/string, /string].
Decl read_edge(A, B) bound [/string, /string].
Decl read_has_connection(Path) bound [/string].
Decl read_reach(A, B) bound [/string, /string].
Decl read_component_ord(Path, Ord) bound [/string, /number].
Decl read_component(Path, Root) bound [/string, /string].
Decl read_rep_score(Root, Path, Score) bound [/string, /string, /number].
Decl read_rep_best(Root, Score) bound [/string, /number].
Decl read_slot_rep(Path) bound [/string].
Decl read_unit_reason(Path, Reason) bound [/string, /name].
Decl read_unit_contribution(Path, Reason, Weight) bound [/string, /name, /number].
Decl read_unit_score(Path, Score) bound [/string, /number].
Decl orient_read_member(Path, Representative) bound [/string, /string].

Decl cohort_days_missing(Root) bound [/string].

Decl doc_birth_date(Day) bound [/number].
Decl cohort_directory_document(Path, Window) bound [/string, /number].
Decl cohort_overlap(Root, Window, Other, OtherWindow) bound [/string, /number, /string, /number].
Decl cohort_blocked(Root, Window) bound [/string, /number].
