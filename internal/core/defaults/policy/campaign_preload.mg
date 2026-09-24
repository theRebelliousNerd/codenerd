# Campaign Preload: the code a task's brief names, pushed before the model asks
#
# Measured on campaign 7b853890 (report 03, C7; report 02, G8): 84% of the
# model's read_file calls read a file its anchor named, a turn that writes
# first writes at median round 8 of 19, and the same structural questions were
# asked again and again across tasks (package_outline internal/features 11
# times, callers_of features.IsProvenanceEnabled 4 times). The harness knew
# those files before the first round. Now the brief carries their structure:
#
#   task_preload(Task, Target, /outline | /count | /element | /signature)
#
#   /outline    a Go or Mangle file, or a Go package directory, as the structure
#               index lists it (file:span kind ref rev signature doc per
#               declaration), when it has at most
#               campaign.preload_outline_max_rows declarations
#   /count      the same, larger: named with its declaration count, for the
#               model to page with package_outline
#   /element    an element the brief names, its source line-numbered, when it
#               spans at most campaign.preload_element_max_lines lines
#   /signature  the same, longer: its outline row, for get_element part=
#
# What is preloaded: the code files and packages the brief names, the code the
# task writes (its own targets), the code files the tasks it depends on wrote,
# and the elements the brief names that resolve to exactly one declaration.
# Every row carries the element's revision, which the edit verbs take as their
# precondition: an element the turn has since changed is refused, never
# silently overwritten from a stale copy.
#
# Go measures and asserts (the campaign orchestrator, task_preload.go):
#   code_outline(Path, Shape, Rows)
#       the structure index parses Path as a /file or a /package with Rows
#       declarations (measured for the asked task's candidates only)
#   task_brief_element(Task, Ref, Lines, Revision)
#       an identifier of the asked task's brief resolves to exactly one element
# and reuses task_brief_names / task_output_path (campaign_evidence.mg) for the
# paths the brief names and the task writes.

Decl code_outline(Path, Shape, Rows) bound [/string, /name, /number].
Decl task_brief_element(TaskID, Ref, Lines, Revision) bound [/string, /string, /number, /string].

config_param_required(/campaign, /campaign_preload_outline_max_rows).
config_param_required(/campaign, /campaign_preload_element_max_lines).

# The code a task works from or on.
Decl task_preload_path(TaskID, Path) bound [/string, /string].

task_preload_path(TaskID, Path) :-
    task_brief_names(TaskID, Path),
    code_outline(Path, Shape, Rows).

task_preload_path(TaskID, Path) :-
    task_output_path(TaskID, Path),
    code_outline(Path, Shape, Rows).

task_preload_path(TaskID, Path) :-
    task_dependency(TaskID, Producer),
    campaign_task(Producer, PhaseID, Desc, /completed, Type),
    task_artifact_on_disk(Producer, Path, ArtType, Ext, Bytes),
    code_outline(Path, Shape, Rows).

Decl task_preload(TaskID, Target, Form) bound [/string, /string, /name].

task_preload(TaskID, Path, /outline) :-
    task_preload_path(TaskID, Path),
    code_outline(Path, Shape, Rows),
    config_param(/campaign_preload_outline_max_rows, Max),
    Rows <= Max.

task_preload(TaskID, Path, /count) :-
    task_preload_path(TaskID, Path),
    code_outline(Path, Shape, Rows),
    config_param(/campaign_preload_outline_max_rows, Max),
    Rows > Max.

task_preload(TaskID, Ref, /element) :-
    task_brief_element(TaskID, Ref, Lines, Revision),
    config_param(/campaign_preload_element_max_lines, Max),
    Lines <= Max.

task_preload(TaskID, Ref, /signature) :-
    task_brief_element(TaskID, Ref, Lines, Revision),
    config_param(/campaign_preload_element_max_lines, Max),
    Lines > Max.
