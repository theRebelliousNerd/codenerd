Decl mission_tick(Mission, Cycles, Seconds, Spend, UnknownSpend, Stalls) bound [/string, /number, /number, /number, /name, /number].
Decl mission_context_size(Mission, Bytes) bound [/string, /number].
Decl mission_design(Mission, Disposition) bound [/string, /name].
Decl mission_observation(Mission, Cycle, Outcome, Witness, Before, After) bound [/string, /number, /name, /name, /string, /string].
Decl mission_stop_candidate(Mission, Rank, Reason) bound [/string, /number, /name].
Decl mission_stop_rank(Mission, Rank) bound [/string, /number].
Decl mission_stop(Mission, Reason) bound [/string, /name].
Decl mission_stopped(Mission) bound [/string].
Decl mission_continue(Mission, SecondsLeft) bound [/string, /number].
Decl mission_execute(Mission) bound [/string].
Decl mission_evidence(Mission, Cycle, Disposition) bound [/string, /number, /name].
Decl mission_write_candidate(Mission, Path, Scope, Protected) bound [/string, /string, /name, /name].
Decl mission_write_allowed(Mission, Path) bound [/string, /string].
Decl mission_parallelism(Mission, Tasks) bound [/string, /number].
Decl mission_source(Mission, Before, After, VisionBefore, VisionAfter) bound [/string, /string, /string, /string, /string].
Decl mission_fresh(Mission) bound [/string].
