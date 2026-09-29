# Timeline judgments. Lineage (similarity, links, instructions) is lineage.mg.
# A shallow clone still has repo_month and repo_file_history; history_usable
# is the gate on every judgment that would treat the oldest fetched commit
# as the birth of the work.

history_usable() :-
    repo_span(_, _, _, /no).

# --- eras -------------------------------------------------------------------
# A month quieter than the lull threshold is a lull, consecutive months of
# the same kind are one era, and the calendar gap Go stored as a 0-commit
# month splits a wave. Threshold 1 means only an empty month is a lull.

month_kind(M, /lull) :-
    history_usable(),
    repo_month(M, _, Commits, _, _),
    config_param(/orient_era_lull_commits, T),
    Commits < T.

month_kind(M, /wave) :-
    history_usable(),
    repo_month(M, _, Commits, _, _),
    config_param(/orient_era_lull_commits, T),
    Commits >= T.

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
    repo_era(I, _, _, _)
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
# is all origin: there is no later generation to invent.

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

doc_generation(Path, /origin) :-
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

doc_generation(Path, /origin) :-
    history_usable(),
    era_count(1),
    repo_file_history(Path, _, _, _, _),
    repo_span(A, B, _, /no),
    A >= B.

doc_generation(Path, /origin) :-
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

# --- burst and cohort -------------------------------------------------------
# A burst is many commits on very few days: one document worked in a
# concentrated act. A cohort is many documents born on one day: a landing,
# not a file that happened to be touched. Neither is "recent".

doc_burst(Path) :-
    history_usable(),
    doc_file(Path, _, _, _),
    repo_file_history(Path, _, _, Commits, Days),
    config_param(/orient_burst_max_days, MaxDays),
    config_param(/orient_burst_min_commits, MinCommits),
    Days <= MaxDays,
    Commits >= MinCommits.

doc_birth_day(Path, Day) :-
    history_usable(),
    doc_file(Path, _, _, _),
    repo_file_history(Path, First, _, _, _),
    Day = fn:div(First, 86400).

cohort_size(Day, N) :-
    doc_birth_day(Path, Day)
    |> do fn:group_by(Day), let N = fn:count().

doc_cohort(Path) :-
    doc_birth_day(Path, Day),
    cohort_size(Day, N),
    config_param(/orient_cohort_min_docs, Min),
    N >= Min.

doc_has_history(Path) :-
    repo_file_history(Path, _, _, _, _).

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
# A /north_star_draft is not a weaker /vision. A draft written in the recent
# generation, in one burst, is the strongest source: that is where the
# north star is being written, including while it is still a draft.
# Points, then a cap at 100. The why is the strongest applicable label
# (lowest rank), recorded beside the weight.

vision_claim(Path, Conf) :-
    doc_role_claim(Path, /vision, Conf).

vision_claim(Path, Conf) :-
    doc_role_claim(Path, /north_star_draft, Conf).

vision_conf(Path, C) :-
    vision_claim(Path, Conf)
    |> do fn:group_by(Path), let C = fn:max(Conf).

vision_part_conf(Path, P) :-
    vision_conf(Path, C),
    Scaled = fn:mult(C, 40),
    P = fn:div(Scaled, 100).

vision_part_role(Path, 10) :-
    vision_conf(Path, _).

vision_part_gen(Path, 20) :-
    vision_part_role(Path, _),
    doc_generation(Path, /recent).

vision_part_gen(Path, 10) :-
    vision_part_role(Path, _),
    doc_generation(Path, /origin).

vision_part_gen(Path, 8) :-
    vision_part_role(Path, _),
    doc_generation(Path, /early).

vision_part_gen(Path, 5) :-
    vision_part_role(Path, _),
    doc_generation(Path, /middle).

vision_part_gen(Path, 0) :-
    vision_part_role(Path, _),
    !doc_has_generation(Path).

# Burst outranks cohort, and a document that is both was one act, not two.
vision_part_burst(Path, 20) :-
    vision_part_role(Path, _),
    doc_burst(Path).

vision_part_burst(Path, 15) :-
    vision_part_role(Path, _),
    doc_cohort(Path),
    !doc_burst(Path).

vision_part_burst(Path, 0) :-
    vision_part_role(Path, _),
    !doc_burst(Path),
    !doc_cohort(Path).

vision_part_central(Path, 10) :-
    vision_part_role(Path, _),
    doc_central(Path).

vision_part_central(Path, 0) :-
    vision_part_role(Path, _),
    !doc_central(Path).

vision_part_live(Path, 15) :-
    vision_part_role(Path, _),
    doc_live(Path).

vision_part_live(Path, 0) :-
    vision_part_role(Path, _),
    !doc_live(Path).

vision_raw(Path, R) :-
    vision_part_conf(Path, C),
    vision_part_role(Path, Role),
    vision_part_gen(Path, G),
    vision_part_burst(Path, B),
    vision_part_central(Path, Cen),
    vision_part_live(Path, L),
    S1 = fn:plus(C, Role),
    S2 = fn:plus(S1, G),
    S3 = fn:plus(S2, B),
    S4 = fn:plus(S3, Cen),
    R = fn:plus(S4, L).

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
#   score gap is at least 5, times 1e12
#   centrality is capped at 999, times 1e9
#   last-touch is committer unix / 1e7, times 1e6 (about 116 days)
#   tie is (999999 - ord), and ord is a unique lexicographic rank
# so an earlier path wins a tie without crossing into an older time bucket.
# The budget keeps every path with strictly fewer than B paths above it,
# and every path with nothing above it. orient_read_omitted is the rest,
# with the same reasons, so the cut is visible.

reason_weight(/instructions, 100).
reason_weight(/chain_latest, 90).
reason_weight(/origin, 80).
reason_weight(/cluster_rep, 70).
reason_weight(/live_hub, 65).
reason_weight(/link_hub, 60).
reason_weight(/burst, 55).
reason_weight(/cohort, 50).

read_reason(Path, /origin) :-
    origin_source(Path, _).

read_reason(Path, /burst) :-
    doc_burst(Path).

read_reason(Path, /cohort) :-
    doc_cohort(Path).

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

read_weighted(Path, W) :-
    read_reason(Path, Reason),
    reason_weight(Reason, W).

read_score(Path, S) :-
    read_weighted(Path, W)
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

read_key(Path, Key) :-
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
    read_reason(Path, Reason).

orient_read_omitted(Path, Reason) :-
    read_reason(Path, Reason),
    !orient_read_kept_path(Path).
