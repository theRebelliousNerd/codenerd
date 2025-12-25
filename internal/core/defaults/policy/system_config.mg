# System Configuration (EDB Facts)
# Extracted from system.mg

# System shards that must auto-start
shard_startup(/perception_firewall, /auto).
shard_startup(/executive_policy, /auto).
shard_startup(/constitution_gate, /auto).
shard_startup(/world_model_ingestor, /on_demand).
shard_startup(/tactile_router, /on_demand).
shard_startup(/session_planner, /on_demand).

# Routing Table (Tactile Router)
# Default routing table entries (can be extended via Autopoiesis)
routing_table(/fs_read, /read_file, /low).
routing_table(/fs_write, /write_file, /medium).
routing_table(/exec_cmd, /execute_command, /high).
routing_table(/browser, /browser_action, /high).
routing_table(/code_graph, /analyze_code, /low).
