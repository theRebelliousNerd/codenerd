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
#   gate_domain(S, Gate, Domain)           what the gate measured: root | packages
#   gate_comparison(Before, After)         the pair to judge
#
# Conclusions:
#   gate_change(B, A, Gate, /better | /worse | /same)   exactly one per compared gate
#   gate_regressed(B, A, Gate), gate_improved(B, A, Gate)
#   gate_unadmitted(B, A, Gate, /missing_after | /missing_before | /domain_changed)
#   gate_comparison_empty(B, A)            nothing was compared
#   gate_finding_new(B, A, Gate, Key), gate_finding_gone(B, A, Gate, Key)
#   cycle_keepable(B, A)                   every gate compared, none worse

# The Decls are in schemas_gates.mg, so a sibling policy file (recurse.mg)
# that reads these conclusions checks on its own.

# A status that is a measurement: the gate ran to a verdict.
gate_measurable(/pass).
gate_measurable(/fail).

# Admission: the pair answers one question per gate only when both sides
# measured it over one domain (audit gate-comparison-domain-admission). A
# gate measured on one side only used to leave the comparison silently, and
# G3 over one passing package read as an improvement on G3 over the whole
# module. A side that reached no count (unavailable, unverified) carries no
# findings, so its domain does not matter: pass -> unavailable is still worse.
gate_present(S, Gate) :-
    gate_measured(S, Gate, _, _).
gate_counted(S, Gate) :-
    gate_measured(S, Gate, Status, _),
    gate_measurable(Status).

gate_same_domain(B, A, Gate) :-
    gate_comparison(B, A),
    gate_domain(B, Gate, D),
    gate_domain(A, Gate, D).
gate_same_domain(B, A, Gate) :-
    gate_comparison(B, A),
    gate_present(B, Gate),
    gate_present(A, Gate),
    !gate_counted(B, Gate).
gate_same_domain(B, A, Gate) :-
    gate_comparison(B, A),
    gate_present(B, Gate),
    gate_present(A, Gate),
    !gate_counted(A, Gate).

gate_unadmitted(B, A, Gate, /missing_after) :-
    gate_comparison(B, A),
    gate_present(B, Gate),
    !gate_present(A, Gate).
gate_unadmitted(B, A, Gate, /missing_before) :-
    gate_comparison(B, A),
    gate_present(A, Gate),
    !gate_present(B, Gate).
gate_unadmitted(B, A, Gate, /domain_changed) :-
    gate_comparison(B, A),
    gate_present(B, Gate),
    gate_present(A, Gate),
    !gate_same_domain(B, A, Gate).

gate_compared(B, A, Gate, SB, SA, CB, CA) :-
    gate_comparison(B, A),
    gate_measured(B, Gate, SB, CB),
    gate_measured(A, Gate, SA, CA),
    gate_same_domain(B, A, Gate).

# A failure that named no finding: its count of zero enumerates nothing.
gate_opaque(S, Gate) :-
    gate_measured(S, Gate, /fail, 0).

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
# Named findings became a failure that names none: the gate can no longer
# show it did not get worse. Its zero is not fewer findings.
gate_regressed(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /fail, _, _),
    gate_opaque(A, Gate),
    !gate_opaque(B, Gate).

gate_improved(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /pass, _, _).
gate_improved(B, A, Gate) :-
    gate_compared(B, A, Gate, /fail, /fail, CB, CA),
    CA < CB,
    !gate_opaque(A, Gate).

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

gate_comparison_has_gate(B, A) :-
    gate_compared(B, A, _, _, _, _, _).
gate_comparison_empty(B, A) :-
    gate_comparison(B, A),
    !gate_comparison_has_gate(B, A).

gate_comparison_unadmitted(B, A) :-
    gate_unadmitted(B, A, _, _).
gate_comparison_unadmitted(B, A) :-
    gate_comparison_empty(B, A).

cycle_keepable(B, A) :-
    gate_comparison(B, A),
    !gate_comparison_regressed(B, A),
    !gate_comparison_unadmitted(B, A).
