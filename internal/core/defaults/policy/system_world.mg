# System Shard Coordination - World Model Logic
# Domain: World Model Maintenance and Topology

Decl world_model_stale(File.Type<string>).
Decl modified(File.Type<string>).
Decl file_topology(File.Type<string>, Size.Type<int64>, Type.Type<string>, Hash.Type<string>, Deps.Type<string>).
Decl next_action(Action.Type<atom>).
Decl system_shard_healthy(ShardName.Type<atom>).
Decl file_in_project(File.Type<string>).
Decl symbol_reachable(From.Type<string>, To.Type<string>).
Decl dependency_link(From.Type<string>, To.Type<string>, Type.Type<string>).
Decl symbol_reachable_bounded(From.Type<string>, To.Type<string>, MaxDepth.Type<int64>).
Decl symbol_reachable_safe(From.Type<string>, To.Type<string>).
Decl activate_shard(ShardName.Type<atom>).

# World Model Updates (World Model Ingestor)

# File change triggers world model update
world_model_stale(File) :-
    modified(File),
    file_topology(File, _, _, _, _).

# Trigger ingestor when world model is stale
next_action(/update_world_model) :-
    world_model_stale(_),
    system_shard_healthy(/world_model_ingestor).

# File topology derived from filesystem
file_in_project(File) :-
    file_topology(File, _, _, _, _).

# Symbol graph connectivity (uses dependency_link for edges)
# WARNING: This unbounded version can loop forever if dependency_link has cycles.
# Use symbol_reachable_bounded/3 with explicit depth limit for safety.
symbol_reachable(From, To) :-
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    dependency_link(From, Mid, _),
    symbol_reachable(Mid, To).

# Depth-bounded variant to prevent infinite recursion in cyclic graphs.
# MaxDepth should typically be 10-20 for most codebases.
symbol_reachable_bounded(From, To, MaxDepth) :-
    MaxDepth > 0,
    dependency_link(From, To, _).

symbol_reachable_bounded(From, To, MaxDepth) :-
    MaxDepth > 0,
    dependency_link(From, Mid, _),
    NextDepth = fn:minus(MaxDepth, 1),
    symbol_reachable_bounded(Mid, To, NextDepth).

# Convenience predicate with default depth limit of 15.
# Safe to use in place of symbol_reachable.
symbol_reachable_safe(From, To) :-
    symbol_reachable_bounded(From, To, 15).

# Activate world_model_ingestor when files change
activate_shard(/world_model_ingestor) :-
    modified(_),
    !system_shard_healthy(/world_model_ingestor).
