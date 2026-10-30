Decl deliberation_tick(Thread, Round, Unchanged) bound [/string, /number, /number].
Decl deliberation_round(Thread, Round) bound [/string, /number].
Decl deliberation_candidate(Thread, Round, Candidate, Rank) bound [/string, /number, /string, /number].
Decl deliberation_success(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_item(Thread, Round, Candidate, Item, Severity, Kind) bound [/string, /number, /string, /string, /name, /name].
Decl deliberation_recommended(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_answer(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_questioned(Thread) bound [/string].
Decl deliberation_mode(Thread, Mode) bound [/string, /name].
Decl deliberation_critical(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_nondata(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_nonvalue(Thread, Round, Candidate) bound [/string, /number, /string].
Decl deliberation_eligible(Thread, Round, Kind, Candidate, Rank) bound [/string, /number, /name, /string, /number].
Decl deliberation_best(Thread, Round, Kind, Rank)
    descr [fundep([Thread, Round, Kind], [Rank]), merge([Rank], "deliberation_lower")]
    bound [/string, /number, /name, /number].
Decl deliberation_lower(A, B, Best) descr [mode('+', '+', '-'), deferred()] bound [/number, /number, /number].
Decl deliberation_pick(Thread, Round, Kind, Candidate) bound [/string, /number, /name, /string].
Decl deliberation_has_ready(Thread, Round) bound [/string, /number].
Decl deliberation_has_empirical(Thread, Round) bound [/string, /number].
Decl deliberation_has_values(Thread, Round) bound [/string, /number].
Decl deliberation_stop(Thread, Round, Reason) bound [/string, /number, /name].
Decl deliberation_stopped(Thread, Round) bound [/string, /number].
Decl deliberation_decision(Thread, Round, Verdict, Candidate) bound [/string, /number, /name, /string].
Decl deliberation_disposition(Thread, Round, Action, Candidate) bound [/string, /number, /name, /string].
Decl deliberation_valid_kind(Kind) bound [/name].
Decl deliberation_valid_severity(Severity) bound [/name].
Decl deliberation_invalid(Thread, Round) bound [/string, /number].
Decl deliberation_agent_tick(Agent, Calls) bound [/string, /number].
Decl deliberation_agent_continue(Agent) bound [/string].
Decl deliberation_read_tool(Tool) bound [/name].
Decl deliberation_has_answer(Thread, Round) bound [/string, /number].
Decl deliberation_brief(Thread, Round, Candidate, Brief, SuccessTest) bound [/string, /number, /string, /string, /string].
Decl deliberation_citation(Thread, Round, Candidate, Citation) bound [/string, /number, /string, /string].
Decl deliberation_probe(Thread, Round, Key, Tool, Digest) bound [/string, /number, /string, /string, /string].
