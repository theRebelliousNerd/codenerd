# Semantic treatment of member directories (GAP-ORIENT-09 / GAP-WS-05).
#
# Go measures a bounded census of member directories (treatment.go): names,
# parent links, depth, file and byte counts, extension mix, and a counted
# sample of headers carrying generated-code markers. The operator's
# overrides arrive as treatment_override rows. Every judgment below - which
# role a directory plays, which treatment it gets, which claim governs a
# descendant, and where equally authoritative claims disagree - is a rule.
#
# Ancestry is the parent relation Go asserts per path segment, never string
# prefix: a rule on "data" cannot reach "database".
#
# Inference never derives /exclude. A directory without a governing claim
# has no tree_treatment row and keeps current behavior.
#
# This file is evaluated alone with schema.mg for treatment refresh and is
# also part of the full orientation program, so it declares its own
# predicates exactly once.

Decl tree_dir(Dir) bound [/string].
Decl tree_parent(Dir, Parent) bound [/string, /string].
Decl tree_depth(Dir, Depth) bound [/string, /number].
Decl tree_name(Dir, Base) bound [/string, /string].
Decl tree_stats(Dir, Files, Bytes) bound [/string, /number, /number].
Decl tree_ext(Dir, Ext, Count) bound [/string, /string, /number].
Decl tree_header_sample(Dir, Read, Generated) bound [/string, /number, /number].
Decl treatment_override(Dir, Treatment, Basis) bound [/string, /name, /name].

Decl treatment_name_convention(Base, Role) bound [/string, /name].
Decl treatment_data_extension(Ext) bound [/string].
Decl treatment_source_extension(Ext) bound [/string].
Decl treatment_role_treatment(Role, Treatment) bound [/name, /name].
Decl treatment_role_data_like(Role) bound [/name].
Decl treatment_authority_rank(Authority, Rank) bound [/name, /number].
Decl treatment_restriction_strictness(Treatment, Level) bound [/name, /number].

Decl tree_scope(Dir, Scope) bound [/string, /string].
Decl tree_data_files(Dir, Count) bound [/string, /number].
Decl tree_source_files(Dir, Count) bound [/string, /number].
Decl tree_role(Dir, Role, Why) bound [/string, /name, /name].
Decl treatment_claim(Scope, Treatment, Authority, Why) bound [/string, /name, /name, /name].
Decl treatment_claim_applies(Dir, Scope, Treatment, Authority, Why, Rank, Depth) bound [/string, /string, /name, /name, /name, /number, /number].
Decl treatment_top_rank(Dir, Rank) bound [/string, /number].
Decl treatment_restriction_level(Dir, Level) bound [/string, /number].
Decl treatment_restricted(Dir, Treatment) bound [/string, /name].
Decl treatment_restriction_depth(Dir, Depth) bound [/string, /number].
Decl treatment_open_claim(Dir, Scope, Treatment, Authority, Why, Depth) bound [/string, /string, /name, /name, /name, /number].
Decl treatment_nearest_depth(Dir, Depth) bound [/string, /number].
Decl treatment_nearest_claim(Dir, Scope, Treatment, Authority, Why) bound [/string, /string, /name, /name, /name].
Decl tree_treatment_conflict(Dir, Scope, Treatment) bound [/string, /string, /name].
Decl tree_treatment_conflicted(Dir, Scope) bound [/string, /string].
Decl tree_treatment(Dir, Treatment, Reason) bound [/string, /name, /name].
Decl tree_treatment_source(Dir, Scope, Authority) bound [/string, /string, /name].
Decl treatment_override_unmatched(Dir) bound [/string].

config_param_required(/orient, /orient_treatment_source_share_permille).
config_param_required(/orient, /orient_treatment_data_share_permille).
config_param_required(/orient, /orient_treatment_data_min_bytes).
config_param_required(/orient, /orient_treatment_generated_share_permille).
config_param_required(/orient, /orient_treatment_generated_min_samples).

# --- conventions -------------------------------------------------------------

treatment_name_convention("vendor", /vendored).
treatment_name_convention("third_party", /vendored).
treatment_name_convention("node_modules", /vendored).
treatment_name_convention("testdata", /fixtures).
treatment_name_convention("fixtures", /fixtures).
treatment_name_convention("__fixtures__", /fixtures).
treatment_name_convention("__snapshots__", /fixtures).

treatment_data_extension(".json").
treatment_data_extension(".jsonl").
treatment_data_extension(".ndjson").
treatment_data_extension(".csv").
treatment_data_extension(".tsv").
treatment_data_extension(".parquet").
treatment_data_extension(".arrow").
treatment_data_extension(".avro").
treatment_data_extension(".npy").
treatment_data_extension(".npz").
treatment_data_extension(".pkl").
treatment_data_extension(".h5").
treatment_data_extension(".hdf5").
treatment_data_extension(".sqlite").
treatment_data_extension(".db").
treatment_data_extension(".bin").
treatment_data_extension(".dat").
treatment_data_extension(".gz").
treatment_data_extension(".zip").
treatment_data_extension(".tar").

treatment_source_extension(".go").
treatment_source_extension(".mg").
treatment_source_extension(".py").
treatment_source_extension(".js").
treatment_source_extension(".jsx").
treatment_source_extension(".mjs").
treatment_source_extension(".cjs").
treatment_source_extension(".ts").
treatment_source_extension(".tsx").
treatment_source_extension(".rs").
treatment_source_extension(".java").
treatment_source_extension(".kt").
treatment_source_extension(".scala").
treatment_source_extension(".c").
treatment_source_extension(".h").
treatment_source_extension(".cc").
treatment_source_extension(".cpp").
treatment_source_extension(".hpp").
treatment_source_extension(".cs").
treatment_source_extension(".rb").
treatment_source_extension(".php").
treatment_source_extension(".swift").
treatment_source_extension(".lua").
treatment_source_extension(".ex").
treatment_source_extension(".exs").
treatment_source_extension(".hs").
treatment_source_extension(".dart").

# Roles that describe data rather than code. Source-language evidence
# contests them; vendored and generated trees are code by nature, so the
# same evidence does not.
treatment_role_treatment(/vendored, /index_names_only).
treatment_role_treatment(/fixtures, /index_names_only).
treatment_role_treatment(/generated, /index_names_only).
treatment_role_treatment(/seed_data, /index_names_only).
treatment_role_data_like(/fixtures).
treatment_role_data_like(/seed_data).

# Authority is resolved before specificity.
treatment_authority_rank(/access_restriction, 3).
treatment_authority_rank(/operator, 2).
treatment_authority_rank(/inferred, 1).

# Hard restrictions compose: the strictest applicable one holds.
treatment_restriction_strictness(/index_names_only, 1).
treatment_restriction_strictness(/exclude, 2).

# --- ancestry ----------------------------------------------------------------

tree_scope(D, D) :- tree_dir(D).
tree_scope(D, S) :- tree_parent(D, P), tree_scope(P, S).

# --- roles -------------------------------------------------------------------

tree_data_files(D, N) :- tree_ext(D, E, C), treatment_data_extension(E)
    |> do fn:group_by(D), let N = fn:sum(C).
tree_source_files(D, N) :- tree_ext(D, E, C), treatment_source_extension(E)
    |> do fn:group_by(D), let N = fn:sum(C).

tree_role(D, Role, /name_convention) :- tree_name(D, Base), treatment_name_convention(Base, Role).

tree_role(D, /seed_data, /data_extension_mix) :-
    tree_stats(D, Files, Bytes), tree_data_files(D, N),
    config_param(/orient_treatment_data_min_bytes, MinBytes), Bytes >= MinBytes,
    config_param(/orient_treatment_data_share_permille, Share),
    Scaled = fn:mult(N, 1000), Floor = fn:mult(Share, Files), Scaled >= Floor.

tree_role(D, /source, /source_extension_mix) :-
    tree_stats(D, Files, Bytes), tree_source_files(D, N),
    config_param(/orient_treatment_source_share_permille, Share),
    Scaled = fn:mult(N, 1000), Floor = fn:mult(Share, Files), Scaled >= Floor.

tree_role(D, /generated, /generated_headers) :-
    tree_header_sample(D, Read, Generated),
    config_param(/orient_treatment_generated_min_samples, Min), Read >= Min,
    config_param(/orient_treatment_generated_share_permille, Share),
    Scaled = fn:mult(Generated, 1000), Floor = fn:mult(Share, Read), Scaled >= Floor.

# --- claims ------------------------------------------------------------------

treatment_claim(D, T, /inferred, Why) :-
    tree_role(D, Role, Why), treatment_role_treatment(Role, T).

# Source evidence contests a data-like judgment on the same directory: real
# implementation under fixtures/ is a conflict, not a silent names-only.
treatment_claim(D, /index_and_parse, /inferred, /source_extension_mix) :-
    tree_role(D, /source, /source_extension_mix),
    tree_role(D, Role, Why), treatment_role_data_like(Role).

treatment_claim(D, T, /access_restriction, /operator_access_restriction) :-
    treatment_override(D, T, /access_restriction).
treatment_claim(D, T, /operator, /operator_override) :-
    treatment_override(D, T, /operational).

treatment_override_unmatched(D) :- treatment_override(D, T, B), !tree_dir(D).

# --- resolution --------------------------------------------------------------

treatment_claim_applies(D, S, T, A, Why, Rank, Depth) :-
    tree_scope(D, S), treatment_claim(S, T, A, Why),
    treatment_authority_rank(A, Rank), tree_depth(S, Depth).

treatment_top_rank(D, Max) :- treatment_claim_applies(D, S, T, A, Why, Rank, Depth)
    |> do fn:group_by(D), let Max = fn:max(Rank).

# A hard restriction dominates every operational or inferred claim; among
# restrictions the strictest holds, credited to its nearest scope.
treatment_restriction_level(D, Max) :-
    treatment_claim_applies(D, S, T, /access_restriction, Why, Rank, Depth),
    treatment_restriction_strictness(T, Level)
    |> do fn:group_by(D), let Max = fn:max(Level).
treatment_restricted(D, T) :-
    treatment_restriction_level(D, Level), treatment_restriction_strictness(T, Level).
treatment_restriction_depth(D, Max) :-
    treatment_restricted(D, T),
    treatment_claim_applies(D, S, T, /access_restriction, Why, Rank, Depth)
    |> do fn:group_by(D), let Max = fn:max(Depth).
tree_treatment(D, T, Why) :-
    treatment_restricted(D, T), treatment_restriction_depth(D, Depth),
    treatment_claim_applies(D, S, T, /access_restriction, Why, Rank, Depth).
tree_treatment_source(D, S, /access_restriction) :-
    treatment_restricted(D, T), treatment_restriction_depth(D, Depth),
    treatment_claim_applies(D, S, T, /access_restriction, Why, Rank, Depth).

# Otherwise the most authoritative claims compete, and the nearest scope
# among them governs: an operator's child override refines its parent's,
# and a nearer inference refines a farther one.
treatment_open_claim(D, S, T, A, Why, Depth) :-
    treatment_top_rank(D, Top), Top < 3,
    treatment_claim_applies(D, S, T, A, Why, Top, Depth).
treatment_nearest_depth(D, Max) :- treatment_open_claim(D, S, T, A, Why, Depth)
    |> do fn:group_by(D), let Max = fn:max(Depth).
treatment_nearest_claim(D, S, T, A, Why) :-
    treatment_nearest_depth(D, Depth), treatment_open_claim(D, S, T, A, Why, Depth).

# Equally authoritative claims on one scope that disagree are a conflict for
# that scope and everything inheriting from it. No row: current behavior.
tree_treatment_conflict(D, S, T1) :-
    treatment_nearest_claim(D, S, T1, A1, W1),
    treatment_nearest_claim(D, S, T2, A2, W2), T1 != T2.
tree_treatment_conflicted(D, S) :- tree_treatment_conflict(D, S, T).

tree_treatment(D, T, Why) :-
    treatment_nearest_claim(D, S, T, A, Why), !tree_treatment_conflicted(D, S).
tree_treatment_source(D, S, A) :-
    treatment_nearest_claim(D, S, T, A, Why), !tree_treatment_conflicted(D, S).
