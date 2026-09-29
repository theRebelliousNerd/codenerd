config_param_required(/orient, /orient_embedding_hub_top_k_multiple).
config_param_required(/orient, /orient_embedding_batch_size).
config_param_required(/orient, /orient_embedding_concurrency).
config_param_required(/orient, /orient_embedding_retry_attempts).
config_param_required(/orient, /orient_reason_rarity_floor_permille).
config_param_required(/orient, /orient_near_duplicate_permille).
config_param_required(/orient, /orient_cohort_window_days).
config_param_required(/orient, /orient_cohort_share_ceiling_permille).
config_param_required(/orient, /orient_era_lull_median_permille).
config_param_required(/orient, /orient_read_weight_instructions).
config_param_required(/orient, /orient_read_weight_chain_latest).
config_param_required(/orient, /orient_read_weight_origin).
config_param_required(/orient, /orient_read_weight_cluster_rep).
config_param_required(/orient, /orient_read_weight_live_hub).
config_param_required(/orient, /orient_read_weight_link_hub).
config_param_required(/orient, /orient_read_weight_burst).
config_param_required(/orient, /orient_read_weight_cohort).
config_param_required(/orient, /orient_vision_weight_confidence).
config_param_required(/orient, /orient_vision_weight_role).
config_param_required(/orient, /orient_vision_weight_recent).
config_param_required(/orient, /orient_vision_weight_origin).
config_param_required(/orient, /orient_vision_weight_early).
config_param_required(/orient, /orient_vision_weight_middle).
config_param_required(/orient, /orient_vision_weight_burst).
config_param_required(/orient, /orient_vision_weight_cohort).
config_param_required(/orient, /orient_vision_weight_central).
config_param_required(/orient, /orient_vision_weight_live).

# Timeline judgments. Lineage (similarity, links, instructions) is lineage.mg.
# A shallow clone still has repo_month and repo_file_history; history_usable
# is the gate on every judgment that would treat the oldest fetched commit
# as the birth of the work.

history_usable() :-
    repo_span(_, _, _, /no).

# --- eras -------------------------------------------------------------------
# Monthly median includes empty calendar months. Ranking by month ordinal
# preserves repeated counts; averaging the two middle values stays exact.
month_measure(M, C) :- repo_month(M, _, C, _, _).
month_before(M, Other) :-
    month_measure(M, C), month_measure(Other, OC), OC < C.
month_before(M, Other) :-
    month_measure(M, C), month_measure(Other, C), Other < M.
month_has_before(M) :- month_before(M, _).
month_rank(M, R) :-
    month_before(M, Other)
    |> do fn:group_by(M), let R = fn:count().
month_rank(M, 0) :- month_measure(M, _), !month_has_before(M).
month_total(N) :-
    month_measure(M, C)
    |> do fn:group_by(), let N = fn:count().
month_middle(Lower, Upper) :-
    month_total(N), N > 0,
    Before = fn:minus(N, 1),
    Lower = fn:div(Before, 2), Upper = fn:div(N, 2).
month_median_twice(V) :-
    month_middle(Lower, Upper),
    month_rank(LM, Lower), month_measure(LM, LC),
    month_rank(UM, Upper), month_measure(UM, UC),
    V = fn:plus(LC, UC).
month_lull(M) :-
    month_measure(M, C), config_param(/orient_era_lull_commits, Floor),
    C < Floor.
month_lull(M) :-
    month_measure(M, C), month_median_twice(Median),
    config_param(/orient_era_lull_median_permille, Share),
    Scaled = fn:mult(C, 2000), Threshold = fn:mult(Median, Share),
    Scaled < Threshold.
month_kind(M, /lull) :- history_usable(), month_lull(M).
month_kind(M, /wave) :-
    history_usable(), month_measure(M, _), !month_lull(M).

has_later_month(M) :-
    month_kind(M, _),
    month_kind(N, _),
    N = fn:plus(M, 1).

era_start(M) :-
    month_kind(M, _),
    M = 0.

era_start(M) :-
    month_kind(M, K),
    month_kind(P, PK),
    M = fn:plus(P, 1),
    K != PK.

era_end(M) :-
    month_kind(M, K),
    month_kind(N, NK),
    N = fn:plus(M, 1),
    K != NK.

era_end(M) :-
    month_kind(M, _),
    !has_later_month(M).

era_start_reached(M, S) :-
    month_kind(M, _),
    era_start(S),
    S <= M.

era_starts_upto(M, C) :-
    era_start_reached(M, S)
    |> do fn:group_by(M), let C = fn:count().

era_index_of(M, I) :-
    era_starts_upto(M, C),
    I = fn:minus(C, 1).

repo_era(I, Start, End, Kind) :-
    era_start(Start),
    era_index_of(Start, I),
    era_end(End),
    era_index_of(End, I),
    month_kind(Start, Kind).

era_count(N) :-
    repo_era(I, Start, End, Kind)
    |> do fn:group_by(), let N = fn:count().

# --- where a document sits in the span --------------------------------------

doc_birth_month(Path, M) :-
    history_usable(),
    repo_file_history(Path, First, _, _, _),
    repo_month_span(M, Start, End),
    First >= Start,
    First <= End.

doc_touch_month(Path, M) :-
    history_usable(),
    repo_file_history(Path, _, Last, _, _),
    repo_month_span(M, Start, End),
    Last >= Start,
    Last <= End.

doc_birth_era(Path, I) :-
    doc_birth_month(Path, M),
    repo_era(I, Start, End, _),
    M >= Start,
    M <= End.

# One era has no "early" and "late" inside it. Position along repo_span does.
# Zero-width (every commit on one timestamp, or a span that does not advance)
# is an early birth; origin still requires a structural witness.

doc_at_permille(Path, P) :-
    history_usable(),
    era_count(1),
    repo_file_history(Path, Birth, _, _, _),
    repo_span(SpanFirst, SpanLast, _, /no),
    SpanLast > SpanFirst,
    Delta = fn:minus(Birth, SpanFirst),
    Width = fn:minus(SpanLast, SpanFirst),
    Scaled = fn:mult(Delta, 1000),
    P = fn:div(Scaled, Width).

doc_touch_permille(Path, P) :-
    history_usable(),
    era_count(1),
    repo_file_history(Path, _, Last, _, _),
    repo_span(SpanFirst, SpanLast, _, /no),
    SpanLast > SpanFirst,
    Delta = fn:minus(Last, SpanFirst),
    Width = fn:minus(SpanLast, SpanFirst),
    Scaled = fn:mult(Delta, 1000),
    P = fn:div(Scaled, Width).

doc_early_birth(Path) :-
    history_usable(),
    doc_birth_era(Path, 0),
    era_count(N),
    N > 1.

doc_generation(Path, /recent) :-
    history_usable(),
    doc_birth_era(Path, I),
    era_count(N),
    N > 1,
    Last = fn:minus(N, 1),
    I = Last.

doc_generation(Path, /early) :-
    history_usable(),
    doc_birth_era(Path, I),
    era_count(N),
    N > 1,
    I > 0,
    Last = fn:minus(N, 1),
    I < Last,
    Twice = fn:mult(I, 2),
    Twice < N.

doc_generation(Path, /middle) :-
    history_usable(),
    doc_birth_era(Path, I),
    era_count(N),
    N > 1,
    I > 0,
    Last = fn:minus(N, 1),
    I < Last,
    Twice = fn:mult(I, 2),
    Twice >= N.

doc_early_birth(Path) :-
    history_usable(),
    era_count(1),
    repo_file_history(Path, _, _, _, _),
    repo_span(A, B, _, /no),
    A >= B.

doc_early_birth(Path) :-
    doc_at_permille(Path, P),
    config_param(/orient_origin_span_permille, Cut),
    P < Cut.

doc_generation(Path, /early) :-
    doc_at_permille(Path, P),
    config_param(/orient_origin_span_permille, Origin),
    config_param(/orient_early_span_permille, Early),
    P >= Origin,
    P < Early.

doc_generation(Path, /middle) :-
    doc_at_permille(Path, P),
    config_param(/orient_early_span_permille, Early),
    config_param(/orient_middle_span_permille, Middle),
    P >= Early,
    P < Middle.

doc_generation(Path, /recent) :-
    doc_at_permille(Path, P),
    config_param(/orient_middle_span_permille, Middle),
    P >= Middle.

doc_has_generation(Path) :-
    doc_generation(Path, _).

# --- bursts and cohesive birth-window cohorts ------------------------------
document_path(Path) :- doc_file(Path, _, _, _).
document_count(N) :-
    document_path(Path)
    |> do fn:group_by(), let N = fn:count().
doc_birth_day(Path, Day) :-
    history_usable(), doc_file(Path, _, _, _),
    repo_file_history(Path, First, _, _, _),
    Day = fn:div(First, 86400).
doc_birth_date(Day) :- doc_birth_day(_, Day).
# Anchor short windows to actual births, so a calendar boundary cannot split
# a deliberate act. Window-local components cannot chain across those windows.
doc_birth_window(Path, Window) :-
    doc_birth_day(Path, Day), doc_birth_date(Window),
    config_param(/orient_cohort_window_days, Days),
    Day >= Window, End = fn:plus(Window, Days), Day < End.
cohort_directory_member(Dir, Window, Path) :-
    doc_birth_window(Path, Window), doc_subtree(Path, Dir).
cohort_directory_count(Dir, Window, N) :-
    cohort_directory_member(Dir, Window, Path)
    |> do fn:group_by(Dir, Window), let N = fn:count().
cohort_directory_good(Dir, Window) :-
    cohort_directory_count(Dir, Window, N), document_count(Total),
    config_param(/orient_cohort_min_docs, Min),
    config_param(/orient_cohort_share_ceiling_permille, Ceiling),
    N >= Min,
    Scaled = fn:mult(N, 1000), Limit = fn:mult(Total, Ceiling),
    Scaled <= Limit.
cohort_directory_document(Path, Window) :-
    cohort_directory_good(Dir, Window), cohort_directory_member(Dir, Window, Path).
cohort_directory_ord(Dir, Window, Ord) :-
    cohort_directory_good(Dir, Window),
    cohort_directory_member(Dir, Window, Path), doc_tie(Path, O)
    |> do fn:group_by(Dir, Window), let Ord = fn:min(O).
# Star connections avoid materializing every pair in a directory group.
cohort_connection(Path, Anchor, Window) :-
    cohort_directory_ord(Dir, Window, Ord), doc_tie(Anchor, Ord),
    cohort_directory_member(Dir, Window, Path), Path != Anchor.
cohort_connection(A, B, W) :-
    doc_link(A, B), doc_link(B, A),
    doc_birth_window(A, W), doc_birth_window(B, W), A != B,
    !cohort_directory_document(A, W), !cohort_directory_document(B, W).
cohort_connection(A, B, W) :-
    doc_similar(A, B, P),
    config_param(/orient_similarity_floor_permille, Floor), P >= Floor,
    doc_birth_window(A, W), doc_birth_window(B, W), A != B,
    !cohort_directory_document(A, W), !cohort_directory_document(B, W).
cohort_edge(A, B, W) :- cohort_connection(A, B, W).
cohort_edge(B, A, W) :- cohort_connection(A, B, W).
cohort_reach(A, A, W) :- cohort_edge(A, _, W).
cohort_reach(A, B, W) :-
    cohort_edge(A, B, W), doc_tie(A, AO), doc_tie(B, BO), BO < AO.
cohort_reach(A, C, W) :-
    cohort_edge(A, B, W), cohort_reach(B, C, W),
    doc_tie(A, AO), doc_tie(C, CO), CO < AO.
cohort_root_ord(Path, W, Ord) :-
    cohort_reach(Path, Other, W), doc_tie(Other, O)
    |> do fn:group_by(Path, W), let Ord = fn:min(O).
cohort_component(Path, Root, W) :-
    cohort_root_ord(Path, W, Ord), doc_tie(Root, Ord).
cohort_member_count(Root, W, N) :-
    cohort_component(Path, Root, W)
    |> do fn:group_by(Root, W), let N = fn:count().
cohort_good(Root, W) :-
    cohort_member_count(Root, W, N), document_count(Total),
    config_param(/orient_cohort_min_docs, Min),
    config_param(/orient_cohort_share_ceiling_permille, Ceiling),
    N >= Min,
    Scaled = fn:mult(N, 1000), Limit = fn:mult(Total, Ceiling),
    Scaled <= Limit.
cohort_overlap(Root, W, Other, OW) :-
    cohort_good(Root, W), cohort_component(Member, Root, W),
    cohort_component(Member, Other, OW), cohort_good(Other, OW).
cohort_blocked(Root, W) :-
    cohort_overlap(Root, W, Other, OW),
    cohort_member_count(Root, W, N), cohort_member_count(Other, OW, ON), ON > N.
cohort_blocked(Root, W) :-
    cohort_overlap(Root, W, Other, OW),
    cohort_member_count(Root, W, N), cohort_member_count(Other, OW, N), OW < W.
# Equal-size overlapping windows prefer the earlier act. All winning groups
# are bounded and disjoint; no member consumes two cohort slots.
cohort_member(Path, Root) :-
    cohort_component(Path, Root, W), cohort_good(Root, W), !cohort_blocked(Root, W).
doc_cohort(Path) :- cohort_member(Path, _).

file_has_days(Path) :- repo_file_day(Path, _).
cohort_days_missing(Root) :-
    cohort_member(Path, Root), repo_file_history(Path, _, _, _, Days),
    Days > 1, !file_has_days(Path).
cohort_touch_day(Root, Day) :-
    cohort_member(Path, Root), repo_file_day(Path, Day).
# A one-day summary is exact even when a synthetic fixture omits daily rows.
cohort_touch_day(Root, Day) :-
    cohort_member(Path, Root), !file_has_days(Path),
    repo_file_history(Path, _, _, _, 1), doc_birth_day(Path, Day).
cohort_active_days(Root, N) :-
    cohort_touch_day(Root, Day)
    |> do fn:group_by(Root), let N = fn:count().
cohort_commit_touches(Root, N) :-
    cohort_member(Path, Root), repo_file_history(Path, First, Last, C, Days)
    |> do fn:group_by(Root), let N = fn:sum(C).
cohort_burst(Root) :-
    cohort_active_days(Root, Days), cohort_commit_touches(Root, Commits),
    config_param(/orient_burst_max_days, MaxDays),
    config_param(/orient_burst_min_commits, MinCommits),
    Days <= MaxDays, Commits >= MinCommits, !cohort_days_missing(Root).
doc_burst(Path) :- cohort_member(Path, Root), cohort_burst(Root).
doc_burst(Path) :-
    history_usable(), doc_file(Path, _, _, _),
    repo_file_history(Path, _, _, Commits, Days),
    config_param(/orient_burst_max_days, MaxDays),
    config_param(/orient_burst_min_commits, MinCommits),
    Days <= MaxDays, Commits >= MinCommits.

cohort_in_link(Root, Path, From) :-
    cohort_member(Path, Root), doc_link(From, Path),
    cohort_member(From, Root), From != Path.
cohort_in_count(Root, Path, N) :-
    cohort_in_link(Root, Path, From)
    |> do fn:group_by(Root, Path), let N = fn:count().
cohort_has_in(Root, Path) :- cohort_in_count(Root, Path, _).
cohort_link_rank(Root, Path, N) :-
    cohort_in_count(Root, Path, N), N < 1000.
cohort_link_rank(Root, Path, 999) :-
    cohort_in_count(Root, Path, N), N >= 1000.
cohort_link_rank(Root, Path, 0) :-
    cohort_member(Path, Root), !cohort_has_in(Root, Path).
cohort_member_score(Root, Path, Score) :-
    cohort_link_rank(Root, Path, Links), read_cent_of(Path, C),
    read_tie_of(Path, Tie),
    LinkPart = fn:mult(Links, 1000000000000),
    CentPart = fn:mult(C, 1000000000),
    Sum = fn:plus(LinkPart, CentPart), Score = fn:plus(Sum, Tie).
cohort_best(Root, Score) :-
    cohort_member_score(Root, Path, S)
    |> do fn:group_by(Root), let Score = fn:max(S).
cohort_rep(Root, Path) :-
    cohort_member_score(Root, Path, S), cohort_best(Root, S).
is_cohort_rep(Path) :- cohort_rep(_, Path).

doc_has_history(Path) :- repo_file_history(Path, _, _, _, _).

# --- live, quiet, superseded ------------------------------------------------
# doc_live cannot be an input to doc_superseded: a successor is "live"
# because the old document went quiet, and the old document is quiet only
# when it is not the live one. live_base is the structural half (recent
# touch, or inbound links) with supersession still open. Quiet is the
# complement. Supersession needs a live_base successor and a quiet
# predecessor. doc_live is live_base that nobody superseded.

doc_recent_touch(Path) :-
    history_usable(),
    doc_touch_month(Path, M),
    repo_era(I, Start, End, _),
    era_count(N),
    N > 1,
    Last = fn:minus(N, 1),
    I = Last,
    M >= Start,
    M <= End.

doc_recent_touch(Path) :-
    doc_touch_permille(Path, P),
    config_param(/orient_middle_span_permille, Middle),
    P >= Middle.

doc_recent_touch(Path) :-
    history_usable(),
    era_count(1),
    repo_file_history(Path, _, _, _, _),
    repo_span(A, B, _, /no),
    A >= B.

doc_central(Path) :-
    link_in_count(Path, N),
    config_param(/orient_centrality_min_links, Min),
    N >= Min.

doc_live_base(Path) :-
    doc_recent_touch(Path).

doc_live_base(Path) :-
    doc_central(Path).

doc_quiet(Path) :-
    doc_file(Path, _, _, _),
    !doc_recent_touch(Path),
    !doc_central(Path).

doc_superseded(Old, By) :-
    doc_evolved_into(Old, By),
    doc_live_base(By),
    doc_quiet(Old).

doc_has_superseded(Path) :-
    doc_superseded(Path, _).

doc_live(Path) :-
    doc_live_base(Path),
    !doc_has_superseded(Path).

# The originals are the origin generation that something later did not
# replace. A superseded origin stays in the history; it is not a source.

origin_source(Path, /birth_era) :-
    doc_file(Path, _, _, _),
    doc_generation(Path, /origin),
    era_count(N),
    N > 1,
    !doc_has_superseded(Path).

origin_source(Path, /span_position) :-
    doc_file(Path, _, _, _),
    doc_generation(Path, /origin),
    era_count(1),
    repo_span(A, B, _, /no),
    B > A,
    !doc_has_superseded(Path).

origin_source(Path, /zero_span) :-
    doc_file(Path, _, _, _),
    doc_generation(Path, /origin),
    era_count(1),
    repo_span(A, B, _, /no),
    A >= B,
    !doc_has_superseded(Path).

# --- vision -----------------------------------------------------------------
# A draft is a vision claim too. Confidence and structural contributions
# are weighted by prevalence, then capped at 100. The why records an
# applicable structural shape, not an assertion that recency is quality.

vision_claim(Path, Conf) :- doc_role_claim(Path, /vision, Conf).
vision_claim(Path, Conf) :- doc_role_claim(Path, /north_star_draft, Conf).
vision_conf(Path, C) :-
    vision_claim(Path, Conf)
    |> do fn:group_by(Path), let C = fn:max(Conf).
vision_part_conf(Path, P) :-
    vision_conf(Path, C),
    config_param(/orient_vision_weight_confidence, Base),
    reason_rarity(/vision_role, R),
    Scaled = fn:mult(C, Base), Raw = fn:div(Scaled, 100),
    Weighted = fn:mult(Raw, R), P = fn:div(Weighted, 1000).
vision_part_role(Path, P) :-
    vision_conf(Path, _), config_param(/orient_vision_weight_role, Base),
    reason_rarity(/vision_role, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_gen(Path, P) :-
    vision_part_role(Path, _), doc_generation(Path, /recent),
    config_param(/orient_vision_weight_recent, Base), reason_rarity(/gen_recent, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_gen(Path, P) :-
    vision_part_role(Path, _), doc_generation(Path, /origin),
    config_param(/orient_vision_weight_origin, Base), reason_rarity(/gen_origin, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_gen(Path, P) :-
    vision_part_role(Path, _), doc_generation(Path, /early),
    config_param(/orient_vision_weight_early, Base), reason_rarity(/gen_early, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_gen(Path, P) :-
    vision_part_role(Path, _), doc_generation(Path, /middle),
    config_param(/orient_vision_weight_middle, Base), reason_rarity(/gen_middle, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_gen(Path, 0) :- vision_part_role(Path, _), !doc_has_generation(Path).
vision_part_burst(Path, P) :-
    vision_part_role(Path, _), doc_burst(Path),
    config_param(/orient_vision_weight_burst, Base), reason_rarity(/burst, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_burst(Path, P) :-
    vision_part_role(Path, _), doc_cohort(Path), !doc_burst(Path),
    config_param(/orient_vision_weight_cohort, Base), reason_rarity(/cohort, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_burst(Path, 0) :-
    vision_part_role(Path, _), !doc_burst(Path), !doc_cohort(Path).
vision_part_central(Path, P) :-
    vision_part_role(Path, _), doc_central(Path),
    config_param(/orient_vision_weight_central, Base), reason_rarity(/central, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_central(Path, 0) :- vision_part_role(Path, _), !doc_central(Path).
vision_part_live(Path, P) :-
    vision_part_role(Path, _), doc_live(Path),
    config_param(/orient_vision_weight_live, Base), reason_rarity(/live, R),
    Weighted = fn:mult(Base, R), P = fn:div(Weighted, 1000).
vision_part_live(Path, 0) :- vision_part_role(Path, _), !doc_live(Path).
vision_raw(Path, R) :-
    vision_part_conf(Path, C), vision_part_role(Path, Role),
    vision_part_gen(Path, G), vision_part_burst(Path, B),
    vision_part_central(Path, Cen), vision_part_live(Path, L),
    S1 = fn:plus(C, Role), S2 = fn:plus(S1, G),
    S3 = fn:plus(S2, B), S4 = fn:plus(S3, Cen), R = fn:plus(S4, L).

vision_why_rank(Path, 0) :-
    vision_part_role(Path, _),
    doc_generation(Path, /recent),
    doc_burst(Path),
    doc_role_claim(Path, /north_star_draft, _).

vision_why_rank(Path, 1) :-
    vision_part_role(Path, _),
    doc_generation(Path, /recent),
    doc_role_claim(Path, /north_star_draft, _),
    !doc_burst(Path).

vision_why_rank(Path, 2) :-
    vision_part_role(Path, _),
    doc_generation(Path, /origin),
    doc_role_claim(Path, /vision, _).

vision_why_rank(Path, 3) :-
    vision_part_role(Path, _),
    doc_live(Path).

vision_why_rank(Path, 4) :-
    vision_part_role(Path, _).

vision_why_best(Path, R) :-
    vision_why_rank(Path, Rank)
    |> do fn:group_by(Path), let R = fn:min(Rank).

vision_why(Path, /recent_burst_draft) :-
    vision_why_best(Path, 0).

vision_why(Path, /recent_draft) :-
    vision_why_best(Path, 1).

vision_why(Path, /origin_vision) :-
    vision_why_best(Path, 2).

vision_why(Path, /live_claim) :-
    vision_why_best(Path, 3).

vision_why(Path, /claimed) :-
    vision_why_best(Path, 4).

vision_source(Path, W, Why) :-
    vision_raw(Path, W),
    W < 100,
    vision_why(Path, Why).

vision_source(Path, 100, Why) :-
    vision_raw(Path, R),
    R >= 100,
    vision_why(Path, Why).

# --- what the model must read in full ---------------------------------------
# Structure only. A path is not selected because of the words in it.
# Weights sum. The key spaces the components so a higher score always beats
# centrality, centrality always beats the coarse last-touch bucket, and that
# bucket always beats the path ordinal:
#   integer score gap is at least 1, times 1e12
#   centrality is capped at 999, times 1e9
#   last-touch is committer unix / 1e7, times 1e6 (about 116 days)
#   tie is (999999 - ord), and ord is a unique lexicographic rank
# so an earlier path wins a tie without crossing into an older time bucket.
# The budget keeps every path with strictly fewer than B paths above it,
# and every path with nothing above it. orient_read_omitted is the rest,
# with the same reasons, so the cut is visible.

reason_weight(/instructions, W) :- config_param(/orient_read_weight_instructions, W).
reason_weight(/chain_latest, W) :- config_param(/orient_read_weight_chain_latest, W).
reason_weight(/origin, W) :- config_param(/orient_read_weight_origin, W).
reason_weight(/cluster_rep, W) :- config_param(/orient_read_weight_cluster_rep, W).
reason_weight(/live_hub, W) :- config_param(/orient_read_weight_live_hub, W).
reason_weight(/link_hub, W) :- config_param(/orient_read_weight_link_hub, W).
reason_weight(/burst, W) :- config_param(/orient_read_weight_burst, W).
reason_weight(/cohort, W) :- config_param(/orient_read_weight_cohort, W).
# Inverse prevalence is measured over distinct documents, not candidate rows.
orient_document(Path) :- doc_file(Path, _, _, _).
orient_document(Path) :- instruction_at_repo_root(Path).
orient_document(Path) :- instruction_at_scope_root(Path).
orient_document_count(N) :-
    orient_document(Path)
    |> do fn:group_by(), let N = fn:count().
signal_reason(Path, Reason) :- read_reason(Path, Reason).
signal_reason(Path, /vision_role) :- vision_claim(Path, _).
signal_reason(Path, /gen_recent) :- doc_generation(Path, /recent).
signal_reason(Path, /gen_origin) :- doc_generation(Path, /origin).
signal_reason(Path, /gen_early) :- doc_generation(Path, /early).
signal_reason(Path, /gen_middle) :- doc_generation(Path, /middle).
signal_reason(Path, /central) :- doc_central(Path).
signal_reason(Path, /live) :- doc_live(Path).
reason_document_count(Reason, N) :-
    signal_reason(Path, Reason), orient_document(Path)
    |> do fn:group_by(Reason), let N = fn:count().
reason_rarity(Reason, R) :-
    reason_document_count(Reason, N), orient_document_count(Total),
    config_param(/orient_reason_rarity_floor_permille, Floor), Total > 0,
    Scaled = fn:mult(N, 1000), Share = fn:div(Scaled, Total),
    R = fn:minus(1000, Share), R >= Floor.
reason_rarity(Reason, Floor) :-
    reason_document_count(Reason, N), orient_document_count(Total),
    config_param(/orient_reason_rarity_floor_permille, Floor), Total > 0,
    Scaled = fn:mult(N, 1000), Share = fn:div(Scaled, Total),
    R = fn:minus(1000, Share), R < Floor.

read_reason(Path, /origin) :-
    origin_source(Path, _).

read_reason(Path, /burst) :-
    doc_burst(Path).

read_reason(Path, /cohort) :-
    doc_cohort(Path), !doc_burst(Path).

read_reason(Path, /instructions) :-
    instruction_at_repo_root(Path).

read_reason(Path, /instructions) :-
    instruction_at_scope_root(Path).

read_reason(Path, /chain_latest) :-
    chain_latest(Path).

read_reason(Path, /cluster_rep) :-
    cluster_rep(Path).

read_reason(Path, /link_hub) :-
    link_hub(Path).

read_reason(Path, /live_hub) :-
    live_hub(Path).

read_contribution(Path, Reason, W) :-
    read_reason(Path, Reason), reason_weight(Reason, Base), reason_rarity(Reason, R),
    Scaled = fn:mult(Base, R), W = fn:div(Scaled, 1000).
read_score(Path, S) :-
    read_contribution(Path, Reason, W)
    |> do fn:group_by(Path), let S = fn:sum(W).

read_cent_of(Path, C) :-
    read_reason(Path, _),
    rank_centrality(Path, C).

read_cent_of(Path, 0) :-
    read_reason(Path, _),
    !has_rank_centrality(Path).

read_last_of(Path, Last) :-
    read_reason(Path, _),
    repo_file_history(Path, _, Last, _, _).

read_last_of(Path, 0) :-
    read_reason(Path, _),
    !doc_has_history(Path).

read_ord_of(Path, Ord) :-
    read_reason(Path, _),
    doc_tie(Path, Ord).

read_ord_of(Path, 0) :-
    read_reason(Path, _),
    !doc_has_tie(Path).

read_tie_of(Path, T) :-
    read_ord_of(Path, Ord),
    Ord <= 999999,
    T = fn:minus(999999, Ord).

read_tie_of(Path, 0) :-
    read_ord_of(Path, Ord),
    Ord > 999999.

read_member_key(Path, Key) :-
    read_score(Path, S),
    read_cent_of(Path, C),
    read_last_of(Path, Last),
    read_tie_of(Path, Tie),
    ScorePart = fn:mult(S, 1000000000000),
    CentPart = fn:mult(C, 1000000000),
    Coarse = fn:div(Last, 10000000),
    TimePart = fn:mult(Coarse, 1000000),
    K1 = fn:plus(ScorePart, CentPart),
    K2 = fn:plus(K1, TimePart),
    Key = fn:plus(K2, Tie).

read_unit_contribution(Path, Reason, W) :-
    read_unit_reason(Path, Reason), reason_weight(Reason, Base), reason_rarity(Reason, R),
    Scaled = fn:mult(Base, R), W = fn:div(Scaled, 1000).
read_unit_score(Path, S) :-
    read_unit_contribution(Path, Reason, W)
    |> do fn:group_by(Path), let S = fn:sum(W).
read_key(Path, Key) :-
    read_unit_score(Path, S), read_cent_of(Path, C),
    read_last_of(Path, Last), read_tie_of(Path, Tie),
    ScorePart = fn:mult(S, 1000000000000), CentPart = fn:mult(C, 1000000000),
    Coarse = fn:div(Last, 10000000), TimePart = fn:mult(Coarse, 1000000),
    K1 = fn:plus(ScorePart, CentPart), K2 = fn:plus(K1, TimePart), Key = fn:plus(K2, Tie).

read_rank_higher(Path, Other) :-
    read_key(Path, K),
    read_key(Other, OK),
    OK > K.

has_higher_rank(Path) :-
    read_rank_higher(Path, _).

higher_count(Path, N) :-
    read_rank_higher(Path, Other)
    |> do fn:group_by(Path), let N = fn:count().

orient_read_kept(Path) :-
    read_key(Path, _),
    !has_higher_rank(Path),
    config_param(/orient_read_candidate_budget, B),
    B > 0.

orient_read_kept(Path) :-
    higher_count(Path, N),
    config_param(/orient_read_candidate_budget, B),
    N < B.

orient_read_kept_path(Path) :-
    orient_read_kept(Path).

orient_read_candidate(Path, Reason) :-
    orient_read_kept(Path),
    read_unit_reason(Path, Reason).

orient_read_omitted(Path, Reason) :-
    read_reason(Path, Reason),
    !orient_read_kept_path(Path).
