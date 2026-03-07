# System Configuration (EDB Facts)
# Extracted from system.mg

# System shards that must auto-start
shard_startup(/perception_firewall, /auto).
shard_startup(/executive_policy, /auto).
shard_startup(/constitution_gate, /auto).
shard_startup(/world_model_ingestor, /on_demand).
shard_startup(/tactile_router, /on_demand).
shard_startup(/session_planner, /on_demand).

# Routing table entries live in system_routing.mg.
# Keep configuration-only facts here so the routing source of truth stays singular.
