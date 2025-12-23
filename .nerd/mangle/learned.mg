entry_point(/system_start).

active_strategy(/system_start) :- system_heartbeat(_,/cold).

next_action(/system_start) :- session_state(_,/initializing,_).

strategy_activated(/initialization,/system_start).

next_action(/system_start) :- session_turn(_,_,/init,_).

active_strategy(/system_start) :- session_state(_,_,/initializing).

current_phase(/system_start).

next_action(/system_start) :- dream_state(/idle,_).

next_action(/initialize) :- system_heartbeat(/boot,_).

next_action(/initialize) :- session_state(_,/booting,_).

next_action(/system_start) :- context_atom(/boot).

next_action(/initialize) :- system_shard(_,/boot).

next_action(/initialize) :- current_intent(/system_start).

action_permitted(/system_start).

final_action(/system_start) :- session_state(_,/initializing,_).

next_action(/system_start) :- entry_point(/init).

next_action(/initialize) :- current_phase(/start).

next_action(/system_start) :- session_state(_,_,/initializing).

next_action(/system_start) :- system_shard_unhealthy(/true).

next_action(/initialize) :- system_event_handled(/start,_,_).

next_action(/initialize) :- system_event_handled(/start,_,_).

next_action(/idle) :- coder_state(/idle).

# INVALID (MangleWatcher): next_action(/system_start) :- entry_point(/system_start).

build_system(/initialized).

current_phase(/system_start).

next_action(/system_start) :- session_planner_status(_,_,_,_,_,/idle).

next_action(/initialize) :- session_planner_status(_,/idle,_,_,_,_).

next_action(/system_start) :- coder_state(/idle).

active_strategy(/system_start) :- northstar_defined().

# INVALID (MangleWatcher): next_action(/system_start) :- entry_point(/system_start).

next_action(/system_start) :- current_task(/idle).

next_action(/initialize) :- generation_state(/system,/start).

next_action(/system_start) :- current_time(_).

next_action(/initialize) :- system_heartbeat(_,/cold_start).

next_action(/initialize) :- system_heartbeat(_,/boot).

build_state(/starting).

selected_atom(/system_start) :- effective_prompt_atom(/system_start).

next_action(/system_start) :- session_state(_,_,/initializing).

next_action(/initialize) :- current_task(/system_start).

current_phase(/system_start).

current_phase(/system_start).

next_action(/system_start) :- in_scope(/initialization).

system_startup(/initializing).

next_action(/system_start) :- session_state(_,/initialized,_).

next_action(/system_startup) :- system_startup(/ready).

next_action(/system_start) :- !system_shard_state(/system,/running).
# Autopoiesis-learned rule (added 2025-12-22 14:01:20)
system_startup(/system_start).


# Autopoiesis-learned rule (added 2025-12-22 21:01:37)
system_startup(/started).

