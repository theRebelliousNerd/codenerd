# Recurse: the improve-everything loop's decisions, derived
#
# Recurse walks the workspace's own dependency graph bottom to top, forever
# (Docs/journeys/10-forever-loop.md). Go is the driver: it runs the
# workspace's gates, asserts what they reported, runs one attempt, and asserts
# what re-measuring found. Every decision about what to attempt and whether an
# attempt is kept is a rule here:
#
#   recurse_next(FindingID)                  what the visit attempts next
#   finding_stalled(FindingID)               a finding the loop stops retrying
#   recurse_ratchet(Cycle, /keep | /revert | /refuse)
#                                            what happens to an attempt's writes
#   recurse_ratchet_kind(Kind)               which workspace-wide gates every
#                                            attempt is re-measured against
#
# EDB (asserted by internal/campaign/recurse_cycle.go):
#   recurse_visit(Node)                          the node being visited
#   recurse_visit_attempted(ID)                  attempted during this visit
#   recurse_finding(ID, Node, Gate, Kind, Target, Signature)
#                                                an open finding, as last measured
#   recurse_regression(ID)                       absent when the pass began
#   recurse_attempt(ID, Node, Cycle, Outcome, Signature)
#                                                every past attempt (journal)
#   recurse_node_kept(Node, Cycle)               a kept change touched Node
#   recurse_ratchet_gate(Cycle, Gate, Before, After, BeforeCount, AfterCount)
#   recurse_ratchet_target(Cycle, ID, /resolved | /open)
#   recurse_ratchet_changed(Cycle, /yes | /no)
#   recurse_ratchet_forbidden(Cycle, Path)       the attempt wrote a path nerd.md
#                                                forbids

Decl recurse_visit(Node) bound [/string].
Decl recurse_visit_attempted(ID) bound [/string].
Decl recurse_finding(ID, Node, Gate, Kind, Target, Signature) bound [/string, /string, /string, /name, /string, /string].
Decl recurse_regression(ID) bound [/string].
Decl recurse_attempt(ID, Node, Cycle, Outcome, Signature) bound [/string, /string, /number, /name, /string].
Decl recurse_node_kept(Node, Cycle) bound [/string, /number].
Decl recurse_ratchet_gate(Cycle, Gate, Before, After, BeforeCount, AfterCount) bound [/number, /string, /name, /name, /number, /number].
Decl recurse_ratchet_target(Cycle, ID, State) bound [/number, /string, /name].
Decl recurse_ratchet_changed(Cycle, Changed) bound [/number, /name].
Decl recurse_ratchet_forbidden(Cycle, Path) bound [/number, /string].

Decl recurse_kind_rank(Kind, Rank) bound [/name, /number].
Decl recurse_ratchet_kind(Kind) bound [/name].
Decl recurse_attempt_failed(Outcome) bound [/name].
Decl recurse_node_kept_after(Node, Cycle) bound [/string, /number].
Decl finding_stalled(ID) bound [/string].
Decl finding_refused(ID) bound [/string].
Decl recurse_finding_rank(ID, Rank) bound [/string, /number].
Decl recurse_candidate(ID, Rank) bound [/string, /number].
Decl recurse_best_rank(Rank) bound [/number].
Decl recurse_next(ID) bound [/string].
Decl recurse_gate_worse(Cycle, Gate) bound [/number, /string].
Decl recurse_cycle_worse(Cycle) bound [/number].
Decl recurse_cycle_refused(Cycle) bound [/number].
Decl recurse_cycle_keeps(Cycle) bound [/number].
Decl recurse_ratchet(Cycle, Verdict) bound [/number, /name].

# =============================================================================
# What to attempt
# =============================================================================
# A red build first: nothing else in the node can be judged while it does not
# compile. Then red tests. Then a lint or audit finding the pass itself
# introduced -- the loop cleans up after itself before it polishes. Then the
# rest.
recurse_kind_rank(/build, 1).
recurse_kind_rank(/test, 2).
recurse_kind_rank(/lint, 4).
recurse_kind_rank(/audit, 5).

recurse_finding_rank(ID, R) :-
    recurse_finding(ID, Node, Gate, Kind, Target, Sig),
    recurse_kind_rank(Kind, R),
    !recurse_regression(ID).

recurse_finding_rank(ID, R) :-
    recurse_finding(ID, Node, Gate, Kind, Target, Sig),
    recurse_kind_rank(Kind, R),
    recurse_regression(ID),
    R < 3.

recurse_finding_rank(ID, 3) :-
    recurse_finding(ID, Node, Gate, Kind, Target, Sig),
    recurse_kind_rank(Kind, R),
    recurse_regression(ID),
    R > 3.

# A stalled finding is one the loop has shown it cannot fix as things stand:
# two failed attempts that ended the same way, with no kept change to its node
# since the first of them. It is not a counter -- a third try that fails
# differently is new evidence, and a kept change to the node lifts the stall,
# because the finding may be fixable now.
recurse_attempt_failed(/reverted).
recurse_attempt_failed(/unverified).

recurse_node_kept_after(Node, C1) :-
    recurse_attempt(ID, Node, C1, Outcome, Sig),
    recurse_node_kept(Node, C),
    C1 < C.

finding_stalled(ID) :-
    recurse_attempt(ID, Node, C1, O1, Sig),
    recurse_attempt(ID, Node, C2, O2, Sig),
    C1 < C2,
    recurse_attempt_failed(O1),
    recurse_attempt_failed(O2),
    !recurse_node_kept_after(Node, C1).

# A refusal is the owner's to lift: the attempt reached for a path nerd.md
# forbids, and retrying it would only reach again.
finding_refused(ID) :-
    recurse_attempt(ID, Node, Cycle, /refused, Sig).

# A visit attempts each finding once. What an attempt left open waits for the
# next pass, when the node is visited again with whatever the pass changed;
# retrying it now, on the same code, is how a visit never ends.
recurse_candidate(ID, R) :-
    recurse_visit(Node),
    recurse_finding(ID, Node, Gate, Kind, Target, Sig),
    recurse_finding_rank(ID, R),
    !finding_stalled(ID),
    !finding_refused(ID),
    !recurse_visit_attempted(ID).

recurse_best_rank(Min) :-
    recurse_candidate(ID, R) |> do fn:group_by(), let Min = fn:min(R).

# Every candidate at the best rank. The driver takes the lowest ID among them,
# so the choice is the same on every run over the same measurement.
recurse_next(ID) :-
    recurse_candidate(ID, R),
    recurse_best_rank(R).

# =============================================================================
# Whether an attempt is kept
# =============================================================================
# Every attempt is re-measured on its node's gates and on the workspace-wide
# gates of these kinds. Tests and audits across the whole workspace run once
# per pass, where their findings become next pass's regressions.
recurse_ratchet_kind(/build).
recurse_ratchet_kind(/lint).

# Worse is any gate that passed and no longer does, any gate that now cannot
# give a verdict, or a failing gate with more findings than before. A failing
# gate whose findings changed but did not grow is not worse: fixing one
# compile error routinely uncovers the next.
recurse_gate_worse(Cycle, Gate) :-
    recurse_ratchet_gate(Cycle, Gate, /pass, /fail, B, A).

recurse_gate_worse(Cycle, Gate) :-
    recurse_ratchet_gate(Cycle, Gate, Before, /unverified, B, A).

recurse_gate_worse(Cycle, Gate) :-
    recurse_ratchet_gate(Cycle, Gate, /fail, /fail, B, A),
    A > B.

recurse_cycle_worse(Cycle) :-
    recurse_gate_worse(Cycle, Gate).

recurse_cycle_refused(Cycle) :-
    recurse_ratchet_forbidden(Cycle, Path).

recurse_cycle_keeps(Cycle) :-
    recurse_ratchet_changed(Cycle, /yes),
    recurse_ratchet_target(Cycle, ID, /resolved),
    !recurse_cycle_worse(Cycle),
    !recurse_cycle_refused(Cycle).

recurse_ratchet(Cycle, /refuse) :-
    recurse_cycle_refused(Cycle).

recurse_ratchet(Cycle, /keep) :-
    recurse_cycle_keeps(Cycle).

# Everything else is reverted: the target is still open, something got worse,
# or the attempt changed nothing at all.
recurse_ratchet(Cycle, /revert) :-
    recurse_ratchet_changed(Cycle, Changed),
    !recurse_cycle_keeps(Cycle),
    !recurse_cycle_refused(Cycle).
