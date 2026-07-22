# System Shards Logic
# Section 21 of Cortex Executive Policy

# System Shard Health Monitoring

# System shard is healthy if heartbeat within threshold (30 seconds)
system_shard_healthy(ShardName) :-
    system_heartbeat(ShardName, _).

# Helper: check if shard has no recent heartbeat
shard_heartbeat_stale(ShardName) :-
    system_shard(ShardName, _),
    !system_shard_healthy(ShardName).

# Escalate if critical system shard is unhealthy
escalation_needed(/system_health, ShardName, "heartbeat_timeout") :-
    shard_heartbeat_stale(ShardName),
    shard_startup(ShardName, /auto).

# On-Demand Shard Activation

# Activate world_model_ingestor when files change
activate_shard(/world_model_ingestor) :-
    modified(_),
    !system_shard_healthy(/world_model_ingestor).

# Activate tactile_router when actions are pending
activate_shard(/tactile_router) :-
    action_ready_for_routing(_),
    !system_shard_healthy(/tactile_router).

# Activate session_planner for campaigns or complex goals
activate_shard(/session_planner) :-
    current_campaign(_),
    !system_shard_healthy(/session_planner).

activate_shard(/session_planner) :-
    user_intent(/current_intent, _, /plan, _, _),
    !system_shard_healthy(/session_planner).
