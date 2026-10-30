config_param_required(/routing, /deliberation_round_cap).
config_param_required(/routing, /deliberation_no_progress).
config_param_required(/routing, /deliberation_tool_calls).
config_param_required(/routing, /deliberation_candidate_cap).
config_param_required(/routing, /deliberation_context_bytes).

deliberation_valid_kind(/data).
deliberation_valid_kind(/values).
deliberation_valid_kind(/design).
deliberation_valid_severity(/critical).
deliberation_valid_severity(/important).
deliberation_invalid(T, R) :- deliberation_item(T, R, _, _, _, K), !deliberation_valid_kind(K).
deliberation_invalid(T, R) :- deliberation_item(T, R, _, _, S, _), !deliberation_valid_severity(S).
deliberation_critical(T, R, C) :- deliberation_item(T, R, C, _, /critical, _).
deliberation_nondata(T, R, C) :- deliberation_item(T, R, C, _, /critical, K), K != /data .
deliberation_nonvalue(T, R, C) :- deliberation_item(T, R, C, _, /critical, K), K != /values .

deliberation_eligible(T, R, /ready, C, Rank) :-
    deliberation_candidate(T, R, C, Rank), deliberation_brief(T, R, C, _, _),
    deliberation_citation(T, R, C, _), deliberation_success(T, R, C), !deliberation_critical(T, R, C).
deliberation_eligible(T, R, /empirical, C, Rank) :-
    deliberation_candidate(T, R, C, Rank), deliberation_critical(T, R, C), !deliberation_nondata(T, R, C).
deliberation_eligible(T, R, /values_fork, C, Rank) :-
    deliberation_candidate(T, R, C, Rank), deliberation_brief(T, R, C, _, _),
    deliberation_success(T, R, C), deliberation_critical(T, R, C), !deliberation_nonvalue(T, R, C).

deliberation_lower(A, B, A) :- A <= B.
deliberation_lower(A, B, B) :- B < A.
deliberation_best(T, R, K, Rank) :- deliberation_eligible(T, R, K, _, Rank).
deliberation_pick(T, R, K, C) :- deliberation_best(T, R, K, Rank), deliberation_eligible(T, R, K, C, Rank).
deliberation_has_ready(T, R) :- deliberation_pick(T, R, /ready, _).
deliberation_has_empirical(T, R) :- deliberation_pick(T, R, /empirical, _).
deliberation_has_values(T, R) :-
    deliberation_eligible(T, R, /values_fork, C, _),
    deliberation_eligible(T, R, /values_fork, D, _), C != D,
    deliberation_recommended(T, R, C).

deliberation_stop(T, R, /round_cap) :- deliberation_tick(T, R, _), config_param(/deliberation_round_cap, Cap), R > Cap.
deliberation_stop(T, R, /no_progress) :- deliberation_tick(T, R, N), config_param(/deliberation_no_progress, Cap), N >= Cap,
    config_param(/deliberation_round_cap, RCap), R <= RCap.
deliberation_stopped(T, R) :- deliberation_stop(T, R, _).

deliberation_decision(T, R, /ready, C) :- deliberation_round(T, R), deliberation_pick(T, R, /ready, C),
    !deliberation_invalid(T, R), !deliberation_stopped(T, R).
deliberation_decision(T, R, /empirical, C) :- deliberation_round(T, R), deliberation_pick(T, R, /empirical, C),
    !deliberation_has_ready(T, R), !deliberation_invalid(T, R), !deliberation_stopped(T, R).
deliberation_decision(T, R, /values_fork, C) :- deliberation_round(T, R), deliberation_has_values(T, R), deliberation_recommended(T, R, C), deliberation_eligible(T, R, /values_fork, C, _),
    !deliberation_has_ready(T, R), !deliberation_has_empirical(T, R), !deliberation_invalid(T, R), !deliberation_stopped(T, R).
deliberation_decision(T, R, /needs_work, "") :- deliberation_round(T, R),
    !deliberation_has_ready(T, R), !deliberation_has_empirical(T, R), !deliberation_has_values(T, R),
    !deliberation_stopped(T, R).

deliberation_disposition(T, R, /proceed, C) :- deliberation_decision(T, R, /ready, C).
deliberation_disposition(T, R, /proceed_assuming, C) :- deliberation_decision(T, R, /values_fork, C), deliberation_mode(T, /yolo).
deliberation_disposition(T, R, /ask_once, C) :- deliberation_decision(T, R, /values_fork, C),
    deliberation_mode(T, M), M != /yolo, !deliberation_questioned(T).
deliberation_disposition(T, R, /proceed, C) :- deliberation_decision(T, R, /values_fork, _),
    deliberation_answer(T, R, C), deliberation_candidate(T, R, C, _), deliberation_success(T, R, C), !deliberation_nonvalue(T, R, C).

deliberation_agent_continue(A) :- deliberation_agent_tick(A, N), config_param(/deliberation_tool_calls, Cap), N < Cap.

# The deliberation roles read: the /core envelope every persona carries
# (reads, recall, search and the MCP plane, whose calls policy_mcp.mg judges
# by risk) and the structural CodeDOM reads. No edit envelope.
deliberation_read_tool(T) :- envelope_tool(/core, T).
deliberation_read_tool(T) :- envelope_tool(/codedom_read, T).
verb_persona(/deliberation_proposer, /deliberation_proposer).
verb_persona(/requirements_interrogator, /requirements_interrogator).
persona_tool_allowed(/deliberation_proposer, T) :- deliberation_read_tool(T).
persona_tool_allowed(/requirements_interrogator, T) :- deliberation_read_tool(T).

deliberation_disposition(T, R, /stop_values, C) :- deliberation_decision(T, R, /values_fork, C),
    deliberation_mode(T, M), M != /yolo, deliberation_questioned(T), !deliberation_has_answer(T, R).
deliberation_has_answer(T, R) :- deliberation_answer(T, R, _).
