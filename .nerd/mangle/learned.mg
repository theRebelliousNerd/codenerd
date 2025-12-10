
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

