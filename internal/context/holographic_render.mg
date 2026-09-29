# Holographic caller render budget. The holographic section is pasted into
# prompt text, not a tool result, so the working ledger never archives it and
# every display cap in it used to be a Go constant (maxCallers 8). This is the
# first of those caps to become a policy decision: Go measures the pool the
# renderer holds (the target, how many callers, their mean rendered bytes)
# and the budget the session supplies for this render, and the policy derives
# how many to render. The renderer slices the existing ranking to that number
# and keeps the honest remainder line ("and N more callers; callers_of ...").
#
# The two to_render rules are mutually exclusive (Need <= Allowance against
# Need > Allowance), so one render derives exactly one N: the whole pool when
# it fits the share, else the top N of the existing ranking. Allowance is the
# render budget's caller share in bytes, the same unit the renderer measures
# its lines in.
Decl holographic_caller_pool(Target, Total) bound [/string, /number].
Decl holographic_caller_bytes(Target, Avg) bound [/string, /number].
Decl holographic_render_budget(Target, Bytes) bound [/string, /number].
Decl holographic_caller_share(Percent) bound [/number].
holographic_caller_share(P) :- config_param(/working_holographic_caller_share_percent, P).
config_param_required(/working, /working_holographic_caller_share_percent).
Decl holographic_caller_allowance(Target, Bytes) bound [/string, /number].
holographic_caller_allowance(Target, Allowance) :-
    holographic_render_budget(Target, Budget),
    holographic_caller_share(Share),
    Allowance = fn:div(fn:mult(Budget, Share), 100).
Decl holographic_callers_to_render(Target, N) bound [/string, /number].
holographic_callers_to_render(Target, Total) :-
    holographic_caller_pool(Target, Total),
    holographic_caller_allowance(Target, Allowance),
    holographic_caller_bytes(Target, Avg),
    Need = fn:mult(Total, Avg),
    Need <= Allowance.
holographic_callers_to_render(Target, N) :-
    holographic_caller_pool(Target, Total),
    holographic_caller_allowance(Target, Allowance),
    holographic_caller_bytes(Target, Avg),
    Need = fn:mult(Total, Avg),
    Need > Allowance,
    N = fn:div(Allowance, Avg).
