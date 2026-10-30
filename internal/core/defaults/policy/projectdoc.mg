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

# A path a project rule forbids is refused on the constitutional path too
# (coder_guidance.mg coder_edit_barred reads project_forbidden_path directly), so
# the denial holds beside the Go gate (internal/projectdoc/gate.go) and does not
# depend on a derived predicate being reachable.
