# The error-free gates, compared: is a cycle keepable?
#
# Rung R7 keeps a cycle only when it left no gate worse. Go measures the
# ladder's error-free gates (G1 build .. G10 gofmt) into snapshots
# (internal/gates, `nerd gates measure`) and asserts two of them; these rules
# judge the pair. Consumers: `nerd gates diff`, and the recurse ratchet, which
# refuses a cycle whose snapshots this file says regressed (policy/recurse.mg,
# recurse_cycle_snapshots).
#
# Inputs (asserted by internal/gates.Compare and the recurse loop):
#   gate_snapshot(S)                       a snapshot taken
#   gate_measured(S, Gate, Status, Count)  Status /pass /fail /unavailable /unverified
#   gate_finding(S, Gate, Key)             a distinct finding (file | check | message)
#   gate_comparison(Before, After)         the pair to judge
#
# Conclusions:
#   gate_change(B, A, Gate, /better | /worse | /same)   exactly one per gate
#   gate_regressed(B, A, Gate), gate_improved(B, A, Gate)
#   gate_finding_new(B, A, Gate, Key), gate_finding_gone(B, A, Gate, Key)
#   cycle_keepable(B, A)                   no gate got worse

# The Decls are in schemas_gates.mg, so a sibling policy file (recurse.mg)
# that reads these conclusions checks on its own.

# A status that is a measurement: the gate ran to a verdict.
gate_measurable(/pass).
gate_measurable(/fail).

gate_compared(B, A, Gate, SB, SA, CB, CA) :-
    gate_comparison(B, A),
    gate_measured(B, Gate, SB, CB),
    gate_measured(A, Gate, SA, CA).

# Worse: a pass became a fail; a measured gate can no longer be measured (a
# gate that cannot show it did not get worse is not kept as unchanged); or a
# failing gate's findings grew. A failing gate whose findings changed but did
# not grow is not worse: fixing one finding may reword another.
gate_regressed(B, A, Gate) :-
    gate_compared(B, A, Gate, /pass, /fail, _, _).
gate_regressed(B, A, Gate) :-
    gate_compared(B, A, Gate, SB, /unverified, _, _),
    gate_measurable(SB).
gate_regressed(B, A, Gate) :-
    gate_compared(B, A, Gate, SB, /unavailable, _, _),
    gate_measurable(SB).
gate_regressed(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /fail, CB, CA),
    CA > CB.

gate_improved(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /pass, _, _).
gate_improved(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /fail, CB, CA),
    CA < CB.

gate_change(B, A, Gate, /worse) :-
    gate_regressed(B, A, Gate).
gate_change(B, A, Gate, /better) :-
    gate_improved(B, A, Gate).
gate_change(B, A, Gate, /same) :-
    gate_compared(B, A, Gate, _, _, _, _),
    !gate_regressed(B, A, Gate),
    !gate_improved(B, A, Gate).

gate_finding_new(B, A, Gate, Key) :-
    gate_comparison(B, A),
    gate_finding(A, Gate, Key),
    !gate_finding(B, Gate, Key).

gate_finding_gone(B, A, Gate, Key) :-
    gate_comparison(B, A),
    gate_finding(B, Gate, Key),
    !gate_finding(A, Gate, Key).

gate_comparison_regressed(B, A) :-
    gate_regressed(B, A, _).

cycle_keepable(B, A) :-
    gate_comparison(B, A),
    !gate_comparison_regressed(B, A).
