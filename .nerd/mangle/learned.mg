
# Autopoiesis-learned rule (added 2025-12-09 15:38:31)
permitted(Action) :- Action = "system_start".

# Autopoiesis-learned rule (added 2025-12-10 10:35:56)
system_shard_state(/boot,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 11:45:05)
entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 16:26:54)
active_strategy(/system_start) :- system_heartbeat(_,/cold).


# Autopoiesis-learned rule (added 2025-12-10 16:29:52)
ooda_phase(/observe) :- system_startup(_,_).


# Autopoiesis-learned rule (added 2025-12-10 16:53:11)
system_startup(/start,/init).


# Autopoiesis-learned rule (added 2025-12-10 17:08:30)
next_action(/system_start) :- session_state(_,/initializing,_).


# Autopoiesis-learned rule (added 2025-12-10 17:10:39)
strategy_activated(/initialization,/system_start).


# Autopoiesis-learned rule (added 2025-12-10 17:11:58)
next_action(/system_start) :- session_turn(_,_,/init,_).


# Autopoiesis-learned rule (added 2025-12-10 17:15:53)
active_strategy(/system_start) :- session_state(_,_,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 17:33:05)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 17:33:54)
system_startup(/initialized,/core_services) :- !system_shard_healthy(/core_services).


# Autopoiesis-learned rule (added 2025-12-10 17:43:14)
next_action(/system_start) :- dream_state(/idle,_).


# Autopoiesis-learned rule (added 2025-12-10 17:44:10)
next_action(/initialize) :- system_heartbeat(/boot,_).


# Autopoiesis-learned rule (added 2025-12-10 17:46:49)
next_action(/initialize) :- session_state(_,/booting,_).


# Autopoiesis-learned rule (added 2025-12-10 17:47:37)
next_action(/system_start) :- context_atom(/boot).


# Autopoiesis-learned rule (added 2025-12-10 17:55:45)
next_action(/initialize) :- system_shard(_,/boot).


# Autopoiesis-learned rule (added 2025-12-10 18:13:55)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:18:12)
next_action(/initialize) :- current_intent(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:19:56)
action_permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 19:47:55)
next_action(/initialize) :- session_planner_status(_,_,/idle,_,_,_).


# Autopoiesis-learned rule (added 2025-12-10 19:49:40)
next_action(/initialize) :- system_startup(_,_).


# Autopoiesis-learned rule (added 2025-12-10 19:50:15)
final_action(/system_start) :- session_state(_,/initializing,_).


# Autopoiesis-learned rule (added 2025-12-10 19:57:39)
permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 19:59:27)
next_action(/system_start) :- entry_point(/init).

