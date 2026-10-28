# Code families: shared obligations over independently located code
# (Docs/audits/codedom-governed-code-replication.md).
#
# The operator declares a family in .nerd/code_families.json (internal/
# codefamily): its member element refs and the requirements every member must
# keep satisfying. When a turn changes one member, every other member owes a
# review against each shared requirement. A review is settled when the peer is
# changed in the same turn, or when the model records an explicit no-change
# decision with its reason (the family_no_change tool, asserted by the host as
# turn_family_no_change). An unsettled review withholds turn_verified
# (coder_safety.mg) and is named to the model below.
#
# Completion is closed over the DECLARED family only: an undeclared duplicate
# is residual risk, never an implied peer.

Decl code_family(Family) bound [/string].
Decl code_family_member(Family, Ref) bound [/string, /string].
Decl code_family_requirement(Family, Requirement) bound [/string, /string].
Decl turn_family_no_change(Turn, Family, Member, Reason) bound [/name, /string, /string, /string].

Decl turn_family_member_changed(Turn, Family, Ref) bound [/name, /string, /string].
Decl turn_family_obligation(Turn, Family, Member, Requirement) bound [/name, /string, /string, /string].
Decl turn_family_member_settled(Turn, Family, Member) bound [/name, /string, /string].
Decl turn_family_open(Turn, Family, Member) bound [/name, /string, /string].
Decl turn_has_open_family_obligation(Turn) bound [/name].

turn_family_member_changed(Turn, Family, Ref) :-
    turn_changed_element(Turn, Ref),
    code_family_member(Family, Ref).

turn_family_obligation(Turn, Family, Member, Requirement) :-
    turn_family_member_changed(Turn, Family, Changed),
    code_family_member(Family, Member),
    Member != Changed,
    code_family_requirement(Family, Requirement).

turn_family_member_settled(Turn, Family, Member) :-
    turn_family_obligation(Turn, Family, Member, Requirement),
    turn_changed_element(Turn, Member).
turn_family_member_settled(Turn, Family, Member) :-
    turn_family_obligation(Turn, Family, Member, Requirement),
    turn_family_no_change(Turn, Family, Member, Reason).

turn_family_open(Turn, Family, Member) :-
    turn_family_obligation(Turn, Family, Member, Requirement),
    !turn_family_member_settled(Turn, Family, Member).

turn_has_open_family_obligation(Turn) :- turn_family_open(Turn, Family, Member).

# Name each open review to the model: the peer, the family, and what it must
# keep satisfying.
injectable_context("*", Line) :-
    turn_family_obligation(Turn, Family, Member, Requirement),
    turn_family_open(Turn, Family, Member),
    Line = fn:string:concat("Code family ", Family, ": review ", Member, " for \"", Requirement, "\" — edit it, or call family_no_change with the reason it needs no change.").
