# System Configuration (EDB Facts)
# Extracted from system.mg

# System shards that must auto-start
shard_startup(/perception_firewall, /auto).
shard_startup(/executive_policy, /auto).
shard_startup(/constitution_gate, /auto).
shard_startup(/world_model_ingestor, /on_demand).
shard_startup(/tactile_router, /on_demand).
shard_startup(/session_planner, /on_demand).
# The three below start the way their Go profiles say (internal/shards,
# define*Profile) and had no row here, so the health policy did not know
# mangle_repair starts at every boot. TestSystemShardStartupModesAgreeWithTheKernel
# holds this table to the profiles.
shard_startup(/mangle_repair, /auto).
shard_startup(/campaign_runner, /on_demand).
shard_startup(/legislator, /on_demand).

# Routing table entries live in system_routing.mg.
# Keep configuration-only facts here so the routing source of truth stays singular.
