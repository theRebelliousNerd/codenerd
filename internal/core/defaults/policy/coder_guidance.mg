# Coder Policy - What The Coder Is Told And What It May Touch
# The coder rules conclude things about the file the user is changing
# (coder_world.mg derives that file from the turn's intent). Three consumers act
# on them: the context compiler (which files are relevant), the prompt compiler
# (injectable_context rows, which the JIT compiler renders into the coder's
# prompt), and the constitution (which edits are barred).

# -----------------------------------------------------------------------------
# What the context compiler includes
# -----------------------------------------------------------------------------
# A file's coder priority becomes the compiler's relevance for it. Generated and
# vendored files never do: the model is not helped by reading them.

context_relevant(File, /p100) :- coder_context_priority(File, 100), coder_final_context_include(File).
context_relevant(File, /p80) :- coder_context_priority(File, 80), coder_final_context_include(File).
context_relevant(File, /p75) :- coder_context_priority(File, 75), coder_final_context_include(File).
context_relevant(File, /p70) :- coder_context_priority(File, 70), coder_final_context_include(File).
context_relevant(File, /p60) :- coder_context_priority(File, 60), coder_final_context_include(File).

# Files with build errors (the target's, then its importers') are what the
# coder most needs to see while the build is red.
context_relevant(File, /p95) :- priority_diagnostic(File, 100).
context_relevant(File, /p85) :- priority_diagnostic(File, 80).

# -----------------------------------------------------------------------------
# What the prompt says
# -----------------------------------------------------------------------------
# Each row is one sentence for the coder about the file it is changing. The
# prompt compiler reads injectable_context for the compiling shard ("coder") and
# renders the rows under its kernel-injected tier.

Decl coder_guidance(File, Line) bound [/string, /string].

# The language's conventions for the file being changed (the language_convention
# table in coder_language.mg): the file's extension names its language, and the
# language's rows are the sentences. Every premise shares the target.
coder_guidance(Target, Rule) :-
    coder_target(Target),
    file_extension(Target, Ext),
    extension_language(Ext, Language),
    language_convention(Language, _, Rule).

coder_guidance(Target, "Check and return every error; never discard one.") :-
    coder_target(Target),
    requires_error_handling(Target).

coder_guidance(Target, "Annotate parameters and return types.") :-
    coder_target(Target),
    requires_type_annotations(Target).

coder_guidance(Target, "The file already fails to build: fix those errors first.") :-
    coder_target(Target),
    diagnostic(/error, Target, _, _, _).

coder_guidance(Target, Line) :-
    coder_task(_, Action, Target, _),
    coder_action_guidance(Action, Line).

coder_guidance(File, Line) :-
    impact_warning(File, /high_dependent_count),
    Line = fn:string:concat(File, " is imported by many files; a change to its API reaches all of them."),
    coder_target(File).

coder_guidance(File, Line) :-
    impact_warning(File, /critical_file),
    Line = fn:string:concat(File, " is a core file or declares an interface; keep its contract stable."),
    coder_target(File).

coder_guidance(File, Line) :-
    impact_warning(File, /cross_package),
    Line = fn:string:concat(File, " is imported across package boundaries; check the importing packages still build."),
    coder_target(File).

coder_guidance(File, Line) :-
    needs_synchronization(File),
    Line = fn:string:concat(File, " is concurrent: synchronize shared state and wait for what you start.").

coder_guidance(File, Line) :-
    needs_context_propagation(File),
    Line = fn:string:concat(File, " is concurrent: pass the caller's context down and honour its cancellation.").

coder_guidance(File, Line) :-
    api_needs_validation(File),
    Line = fn:string:concat(File, " is an endpoint: validate its input before using it.").

coder_guidance(File, Line) :-
    api_needs_error_handling(File),
    Line = fn:string:concat(File, " is an endpoint: answer failures with explicit error responses.").

coder_guidance(File, Line) :-
    db_needs_pooling(File),
    Line = fn:string:concat(File, " touches the database: reuse the pooled connection and keep transactions short.").

coder_guidance(File, Line) :-
    edit_needs_tests(File),
    Line = fn:string:concat(File, " is new code: write its tests with it.").

coder_guidance(File, Line) :-
    edit_needs_docs(File),
    Line = fn:string:concat(File, " changes a public API with no documentation beside it: update the docs.").

injectable_context("coder", Line) :-
    coder_guidance(_, Line).
