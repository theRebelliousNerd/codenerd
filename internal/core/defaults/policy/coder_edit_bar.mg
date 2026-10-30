# Edits the constitution refuses
# permitted/3 (constitution.mg) carries the negation of coder_edit_barred/2.
#
# A generated file is regenerated, not edited: the next generation erases the
# edit. A vendored file belongs to someone else. A path the project's nerd.md
# forbids is forbidden here too, so the denial holds on this path as well as in
# the Go gate (internal/projectdoc/gate.go) and does not depend on a derived
# predicate being reachable. A file is "generated" when the scan found the
# generator's own marker in it (generated_code), not when its name says so.
#
# The constitution declares coder_edit_barred itself because it is loaded
# without the rest of the coder policy (per-agent kernels and the approval tests
# do); this file adds the rules where the world facts they read exist.

Decl coder_write_action(Action) bound [/name].
coder_write_action(/write_file).
coder_write_action(/fs_write).
coder_write_action(/edit_file).
coder_write_action(/fs_edit).
coder_write_action(/edit_lines).
coder_write_action(/insert_lines).
coder_write_action(/delete_lines).
coder_write_action(/edit_element).

Decl vendor_dir_marker(Marker) bound [/string].
vendor_dir_marker("vendor/").
vendor_dir_marker("node_modules/").

# (declared in constitution.mg)
coder_edit_barred(Action, Target) :-
    pending_action(_, Action, Target, _, _),
    coder_write_action(Action),
    generated_code(Target, _, _).

coder_edit_barred(Action, Target) :-
    pending_action(_, Action, Target, _, _),
    coder_write_action(Action),
    vendor_dir_marker(Marker),
    :string:contains(Target, Marker).

coder_edit_barred(Action, Target) :-
    pending_action(_, Action, Target, _, _),
    coder_write_action(Action),
    project_forbidden_path(Match, _),
    :string:contains(Target, Match).
