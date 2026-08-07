# Project Document Policy (nerd.md)
#
# Derivations over the facts declared in schemas_projectdoc.mg.
#
# Schema declarations live in schemas_projectdoc.mg; do not redeclare here.

has_project_doc() :-
    project_doc(_, _).

project_write_protected() :-
    project_forbidden_path(_, _).

project_has_command(Kind) :-
    project_command(Kind, _).

# --- Write protection -------------------------------------------------------
#
# The Go gate in internal/session/executor_tools.go queries
# project_forbidden_path directly before running any write-mutation tool, so
# denial does not depend on a derived predicate being reachable. That is
# deliberate: a safety gate that silently stops firing because an upstream rule
# stopped deriving is worse than no gate, because it still looks present.
#
# These derivations exist for policy and reporting on top of that gate.

# project_write_denied(Path, Reason)
# A pending edit that a project rule forbids. pending_edit has no Go producer
# today (the whole coder_safety.mg block family is dormant on that account), so
# this rule derives nothing yet and is NOT what enforces nerd.md. It is written
# now so that wiring pending_edit later turns nerd.md protection on across the
# transaction path for free, rather than requiring this to be discovered again.
Decl project_write_denied(Path, Reason) bound [/string, /string].
project_write_denied(Path, Reason) :-
    pending_edit(Path, _),
    project_forbidden_path(Match, Reason),
    path_contains(Path, Match).

# Fold nerd.md denials into the existing coder block aggregation so
# coder_safe_to_write/1 accounts for them wherever it is consulted.
coder_block_write(Path, Reason) :-
    project_write_denied(Path, Reason).
