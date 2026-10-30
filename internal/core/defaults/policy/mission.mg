config_param_required(/mission, /mission_max_cycles).
config_param_required(/mission, /mission_max_seconds).
config_param_required(/mission, /mission_max_spend_microusd).
config_param_required(/mission, /mission_no_progress_cycles).
config_param_required(/mission, /mission_concurrency).
config_param_required(/mission, /mission_context_bytes).

mission_stop_candidate(M, 0, /concurrency) :- mission_tick(M, _, _, _, _, _), config_param(/mission_concurrency, N), N != 1.
mission_stop_candidate(M, 1, /cycles) :- mission_tick(M, N, _, _, _, _), config_param(/mission_max_cycles, Cap), N >= Cap.
mission_stop_candidate(M, 2, /time) :- mission_tick(M, _, N, _, _, _), config_param(/mission_max_seconds, Cap), N >= Cap.
mission_stop_candidate(M, 3, /unknown_spend) :- mission_tick(M, _, _, _, /yes, _).
mission_stop_candidate(M, 4, /spend) :- mission_tick(M, _, _, N, _, _), config_param(/mission_max_spend_microusd, Cap), N >= Cap.
mission_stop_candidate(M, 5, /no_progress) :- mission_tick(M, _, _, _, _, N), config_param(/mission_no_progress_cycles, Cap), N >= Cap.
mission_stop_candidate(M, 6, /context) :- mission_context_size(M, N), config_param(/mission_context_bytes, Cap), N > Cap.
mission_stop_candidate(M, 7, /values) :- mission_design(M, /ask_once).
mission_stop_candidate(M, 8, /design_unresolved) :- mission_design(M, /stopped).
mission_stop_candidate(M, 9, /source_changed) :- mission_source(M, Before, After, _, _), Before != After.
mission_stop_candidate(M, 10, /vision_changed) :- mission_source(M, _, _, Before, After), Before != After.
mission_stop_rank(M, Rank) :- mission_stop_candidate(M, R, Reason) |> do fn:group_by(M), let Rank = fn:min(R).
mission_stop(M, Reason) :- mission_stop_rank(M, Rank), mission_stop_candidate(M, Rank, Reason).
mission_stopped(M) :- mission_stop(M, _).
mission_continue(M, Remaining) :- mission_tick(M, _, Seconds, _, _, _),
    config_param(/mission_max_seconds, Cap), Remaining = fn:minus(Cap, Seconds), !mission_stopped(M).
mission_execute(M) :- mission_continue(M, _), mission_design(M, /ready), mission_fresh(M).

mission_evidence(M, C, /supported_gate_deficit) :- mission_observation(M, C, /kept, /resolved, Before, After), Before != After.
mission_evidence(M, C, /unknown) :- mission_observation(M, C, O, W, Before, After), O != /kept .
mission_evidence(M, C, /unknown) :- mission_observation(M, C, /kept, W, Before, After), W != /resolved .
mission_evidence(M, C, /unknown) :- mission_observation(M, C, /kept, /resolved, Same, Same).
mission_fresh(M) :- mission_source(M, Source, Source, Vision, Vision).
mission_write_allowed(M, P) :- mission_write_candidate(M, P, /inside, /no), mission_execute(M).
mission_parallelism(M, N) :- mission_execute(M), config_param(/mission_concurrency, N).
