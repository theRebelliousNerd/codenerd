# Speculative Dreamer (Precognition Layer)
# Section 26 of Cortex Executive Policy


# Enumerate critical files that must never disappear
critical_file("go.mod").
critical_file("go.sum").

# Panic if a projected action would remove a critical file
panic_state(Action, "critical_file_missing") :-
    projected_fact(Action, /file_missing, File),
    critical_file(File).

# Panic on obviously dangerous exec commands
panic_state(Action, "dangerous_exec") :-
    projected_fact(Action, /exec_danger, _).

# Panic when deleting a file whose symbols are covered by tests
panic_state(Action, "deletes_tested_symbol") :-
    projected_fact(Action, /file_missing, _),
    projected_fact(Action, /impacts_test, _).

# Panic when Dreamer flags critical path hits
panic_state(Action, "critical_path_missing") :-
    projected_fact(Action, /critical_path_hit, _).

# Block actions the Dreamer marks as panic states
dream_block(Action, Reason) :-
    panic_state(Action, Reason).
