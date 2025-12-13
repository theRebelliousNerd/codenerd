
# Autopoiesis-learned rule (added 2025-12-09 15:38:31)
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(Action) :- Action = "system_start".

# Autopoiesis-learned rule (added 2025-12-10 10:35:56)
# SELF-HEALED: learned rule defines protected predicate "system_shard_state": produced by system shard supervisor
# system_shard_state(/boot,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 11:45:05)
entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 16:26:54)
active_strategy(/system_start) :- system_heartbeat(_,/cold).


# Autopoiesis-learned rule (added 2025-12-10 16:29:52)


# Autopoiesis-learned rule (added 2025-12-10 16:53:11)


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
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:18:12)
next_action(/initialize) :- current_intent(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 18:19:56)
action_permitted(/system_start).


# DISABLED: Always-true rules cause action derivation loops (stress-tester 2025-12-11)
# next_action(/initialize) :- session_planner_status(_,_,/idle,_,_,_).



# Autopoiesis-learned rule (added 2025-12-10 19:50:15)
final_action(/system_start) :- session_state(_,/initializing,_).


# Autopoiesis-learned rule (added 2025-12-10 19:57:39)
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 19:59:27)
next_action(/system_start) :- entry_point(/init).


# Autopoiesis-learned rule (added 2025-12-10 20:15:45)
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 20:25:42)
next_action(/initialize) :- current_phase(/start).


# DISABLED: Unconditional rule causes infinite loop (stress-tester 2025-12-11)
# next_action(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 20:36:01)
next_action(/system_start) :- session_state(_,_,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 21:25:16)
next_action(/system_start) :- system_shard_unhealthy(/true).


# Autopoiesis-learned rule (added 2025-12-10 22:47:53)
next_action(/initialize) :- system_event_handled(/start,_,_).


# Autopoiesis-learned rule (added 2025-12-10 22:49:29)
next_action(/initialize) :- system_event_handled(/start,_,_).


# Autopoiesis-learned rule (added 2025-12-10 22:50:04)
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(/system_start) :- current_task(/initialization).


# Autopoiesis-learned rule (added 2025-12-10 22:50:10)


# Autopoiesis-learned rule (added 2025-12-10 22:50:44)
next_action(/idle) :- coder_state(/idle).


# Autopoiesis-learned rule (added 2025-12-10 22:54:04)


# Autopoiesis-learned rule (added 2025-12-10 22:57:13)
# SELF-HEALED: learned rule defines protected predicate "permitted": constitutional permission is core-owned (do not learn permissions)
# permitted(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 22:57:39)
next_action(/system_start) :- entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 23:23:29)
build_system(/initialized).


# Autopoiesis-learned rule (added 2025-12-10 23:29:30)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-10 23:33:36)
next_action(/system_start) :- session_planner_status(_,_,_,_,_,/idle).


# Autopoiesis-learned rule (added 2025-12-10 23:34:39)


# Autopoiesis-learned rule (added 2025-12-10 23:36:11)
# next_action(/initialize) :- system_heartbeat(_,_), !suppressed(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 00:36:38)
next_action(/initialize) :- session_planner_status(_,/idle,_,_,_,_).


# Autopoiesis-learned rule (added 2025-12-11 12:48:59)
next_action(/system_start) :- coder_state(/idle).


# Autopoiesis-learned rule (added 2025-12-11 12:51:32)
active_strategy(/system_start) :- northstar_defined().


# Autopoiesis-learned rule (added 2025-12-11 12:51:54)
next_action(/system_start) :- entry_point(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 12:51:57)
next_action(/system_start) :- current_task(/idle).


# Autopoiesis-learned rule (added 2025-12-11 12:52:00)


# Autopoiesis-learned rule (added 2025-12-11 12:54:40)
next_action(/initialize) :- generation_state(/system,/start).


# Autopoiesis-learned rule (added 2025-12-11 12:55:24)
next_action(/system_start) :- current_time(_).


# Autopoiesis-learned rule (added 2025-12-11 12:55:26)
# SELF-HEALED: infinite loop risk: unconditional next_action for system action will fire every tick
# next_action(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 12:55:38)


# Autopoiesis-learned rule (added 2025-12-11 12:55:42)


# Autopoiesis-learned rule (added 2025-12-11 12:55:57)


# Autopoiesis-learned rule (added 2025-12-11 12:56:12)


# Autopoiesis-learned rule (added 2025-12-11 12:56:24)


# Autopoiesis-learned rule (added 2025-12-11 12:56:29)
next_action(/initialize) :- system_heartbeat(_,/cold_start).


# Autopoiesis-learned rule (added 2025-12-11 12:56:36)
next_action(/initialize) :- system_heartbeat(_,/boot).


# Autopoiesis-learned rule (added 2025-12-11 12:56:38)
build_state(/starting).


# Autopoiesis-learned rule (added 2025-12-11 13:46:07)
selected_atom(/system_start) :- effective_prompt_atom(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 13:49:51)
next_action(/system_start) :- session_state(_,_,/initializing).


# Autopoiesis-learned rule (added 2025-12-11 13:51:47)
next_action(/initialize) :- current_task(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 13:53:13)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 14:11:01)
current_phase(/system_start).


# Autopoiesis-learned rule (added 2025-12-11 14:29:56)
next_action(/system_start) :- in_scope(/initialization).


# Autopoiesis-learned rule (added 2025-12-13 04:11:28)
system_startup(/initializing).


# Autopoiesis-learned rule (added 2025-12-13 04:17:22)
next_action(/system_start) :- session_state(_,/initialized,_).


# Autopoiesis-learned rule (added 2025-12-13 04:22:25)
next_action(/system_startup) :- system_startup(/ready).

