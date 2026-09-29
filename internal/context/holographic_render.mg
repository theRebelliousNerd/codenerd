# Holographic render budget. The holographic section is pasted into prompt
# text, not a tool result, so the working ledger never archives it. Every
# display cap in it used to be a Go constant (maxCallers, maxSigs, maxTypes,
# maxRenderedImporters, maxOutlineElements). Each is now one decision: Go
# measures the pool the renderer holds (which dimension, the target, how many
# items, their mean rendered bytes) and the budget the session supplies, and
# the policy derives how many to render. The share is that dimension's
# config_param, /working_holographic_<dim>_share_percent. The renderer slices
# the existing ranking to that number and keeps the honest remainder line,
# which names the tool that reads the rest.
#
# The two count rules are mutually exclusive (Need <= Allowance against
# Need > Allowance), so one render derives exactly one N: the whole pool when
# it fits the share, else as many as the share holds. Allowance is that
# dimension's share of the render budget, in the bytes the renderer measures
# its lines in.
#
# /outline_signature is the other kind of decision. One outline entry's
# signature used to be cut at maxOutlineSignature (100 runes). The allowance
# is the outline block's bytes (already /outline's share of the budget)
# divided by the entries the count decision kept, and it is never below
# /working_holographic_outline_signature_floor. It is not a sixth share.

Decl holographic_count_dimension(Dim) bound [/name].
holographic_count_dimension(/callers).
holographic_count_dimension(/signatures).
holographic_count_dimension(/types).
holographic_count_dimension(/importers).
holographic_count_dimension(/outline).

Decl holographic_share_key(Dim, Key) bound [/name, /name].
holographic_share_key(/callers, /working_holographic_callers_share_percent).
holographic_share_key(/signatures, /working_holographic_signatures_share_percent).
holographic_share_key(/types, /working_holographic_types_share_percent).
holographic_share_key(/importers, /working_holographic_importers_share_percent).
holographic_share_key(/outline, /working_holographic_outline_share_percent).

config_param_required(/working, /working_holographic_callers_share_percent).
config_param_required(/working, /working_holographic_signatures_share_percent).
config_param_required(/working, /working_holographic_types_share_percent).
config_param_required(/working, /working_holographic_importers_share_percent).
config_param_required(/working, /working_holographic_outline_share_percent).
config_param_required(/working, /working_holographic_outline_signature_floor).

Decl holographic_render_share(Dim, Percent) bound [/name, /number].
holographic_render_share(Dim, P) :-
    holographic_share_key(Dim, Key),
    config_param(Key, P).

Decl holographic_outline_signature_floor(N) bound [/number].
holographic_outline_signature_floor(N) :-
    config_param(/working_holographic_outline_signature_floor, N).

Decl holographic_render_pool(Dim, Target, Total) bound [/name, /string, /number].
Decl holographic_render_bytes(Dim, Target, Avg) bound [/name, /string, /number].
Decl holographic_render_budget(Target, Bytes) bound [/string, /number].

Decl holographic_render_allowance(Dim, Target, Bytes) bound [/name, /string, /number].
holographic_render_allowance(Dim, Target, Allowance) :-
    holographic_render_budget(Target, Budget),
    holographic_render_share(Dim, Share),
    Allowance = fn:div(fn:mult(Budget, Share), 100).

Decl holographic_to_render(Dim, Target, N) bound [/name, /string, /number].
holographic_to_render(Dim, Target, Total) :-
    holographic_count_dimension(Dim),
    holographic_render_pool(Dim, Target, Total),
    holographic_render_allowance(Dim, Target, Allowance),
    holographic_render_bytes(Dim, Target, Avg),
    Need = fn:mult(Total, Avg),
    Need <= Allowance.
holographic_to_render(Dim, Target, N) :-
    holographic_count_dimension(Dim),
    holographic_render_pool(Dim, Target, Total),
    holographic_render_allowance(Dim, Target, Allowance),
    holographic_render_bytes(Dim, Target, Avg),
    Need = fn:mult(Total, Avg),
    Need > Allowance,
    N = fn:div(Allowance, Avg).

# Entries is the count the outline decision kept, passed back as the pool.
# The two rules split on the floor (Floor <= Per against Floor > Per) so
# exactly one character allowance is derived. Go never asserts a zero entry
# count: fn:div by zero aborts the fixpoint.
holographic_to_render(/outline_signature, Target, Per) :-
    holographic_render_pool(/outline_signature, Target, Entries),
    Entries > 0,
    holographic_render_allowance(/outline, Target, Allowance),
    holographic_outline_signature_floor(Floor),
    Per = fn:div(Allowance, Entries),
    Floor <= Per.
holographic_to_render(/outline_signature, Target, Floor) :-
    holographic_render_pool(/outline_signature, Target, Entries),
    Entries > 0,
    holographic_render_allowance(/outline, Target, Allowance),
    holographic_outline_signature_floor(Floor),
    Per = fn:div(Allowance, Entries),
    Floor > Per.
