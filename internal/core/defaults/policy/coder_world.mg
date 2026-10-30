# Coder Policy - Inputs Derived From The Live System
# The coder's working set is not a separate measurement. What the user is
# changing is the turn's intent (user_intent, a shared per-turn fact) and the
# edit in flight (pending_edit); everything else a coder rule asks about a file
# is a conclusion over the world model the scan keeps. Deriving them here keeps
# one measurement of each: a producer in Go would be a second copy that drifts.

# What the intent's verb means as a coder action.
Decl coder_verb_task(Verb, Action) bound [/name, /name].
coder_verb_task(/create, /create).
coder_verb_task(/implement, /create).
coder_verb_task(/fix, /fix).
coder_verb_task(/debug, /fix).
coder_verb_task(/refactor, /refactor).
coder_verb_task(/document, /document).
coder_verb_task(/modify, /modify).
coder_verb_task(/edit, /modify).
coder_verb_task(/update, /modify).
coder_verb_task(/integrate, /integrate).

coder_task("current_intent", Action, Target, Instruction) :-
    user_intent(/current_intent, /mutation, Verb, Target, Instruction),
    coder_verb_task(Verb, Action).

# The file under change: the intent's target when the world knows it, and any
# file with an edit in flight.
coder_target(File) :-
    coder_task(_, _, File, _),
    file_topology(File, _, _, _, _).

coder_target(File) :-
    pending_edit(File, _).

# How many files import a file: an aggregate over the import edges.
dependent_count(File, N) :-
    dependency_link(Importer, File, _),
    file_topology(File, _, _, _, _)
    |> do fn:group_by(File), let N = fn:count().

is_core_file(File) :-
    dependent_count(File, N),
    N > 10.

# What a file declares, from the CodeDOM elements the session has in view.
is_interface_file(File) :-
    code_element(_, /interface, File, _, _).

type_definition_file(File) :-
    code_element(_, /struct, File, _, _).

type_definition_file(File) :-
    code_element(_, /interface, File, _, _).

is_public_api(File) :-
    code_element(Ref, _, File, _, _),
    element_visibility(Ref, /public).
