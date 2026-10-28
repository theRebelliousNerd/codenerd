# Self problems: codeNERD's own problems log as evidence.
#
# While /debug-self is on the chat reads the session's aggregated problems log
# (.nerd/logs/<run>_problems.log, internal/problemlog) and replaces these facts
# with its current contents: one self_problem per distinct problem, grouped by
# level, category and a normalized signature (ids, paths and numbers stripped
# so repeats collapse). The main agent and every shard can then query their own
# failures and reason over them instead of asking the user to paste log lines.
#
# Sample is the first occurrence's whole message, never shortened. The kernel
# holds at most problemlog.DefaultMaxProblems problems, most urgent first
# (errors, then count, then recency); what the cut left out is
# self_problem_omitted, never silently dropped.

Decl self_problem(Signature, Level, Category, Count, Sample) bound [/string, /name, /string, /number, /string].
Decl self_problem_seen(Signature, Level, Category, FirstUnix, LastUnix) bound [/string, /name, /string, /number, /number].
Decl self_problem_omitted(Problems, Entries) bound [/number, /number].
# The chat asserts it while /debug-self is on; the lines below are injected
# only then.
Decl debug_self_active() bound [].

# A problem worth diagnosing: every error, and a warning that repeated at least
# logging.self_problem_warn_repeats times. Without that threshold a warning is
# never hot.
Decl self_problem_hot(Signature, Level, Category) bound [/string, /name, /string].

config_param_required(/logging, /logging_self_problem_warn_repeats).

self_problem_hot(Signature, /error, Category) :-
    self_problem(Signature, /error, Category, Count, Sample).
self_problem_hot(Signature, /warn, Category) :-
    self_problem(Signature, /warn, Category, Count, Sample),
    config_param(/logging_self_problem_warn_repeats, Limit),
    Count >= Limit.

# One compact line per hot problem; the full sample stays in self_problem.
injectable_context("*", Line) :-
    debug_self_active(),
    self_problem_hot(Signature, /error, Category),
    self_problem(Signature, /error, Category, Count, Sample),
    Line = fn:string:concat("Own log ERROR [", Category, "] x", fn:number:to_string(Count), ": ", Signature, " (full message: self_problem)").
injectable_context("*", Line) :-
    debug_self_active(),
    self_problem_hot(Signature, /warn, Category),
    self_problem(Signature, /warn, Category, Count, Sample),
    Line = fn:string:concat("Own log WARN [", Category, "] x", fn:number:to_string(Count), ": ", Signature, " (full message: self_problem)").
