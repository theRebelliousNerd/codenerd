# World Model Updates (World Model Ingestor)
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl world_model_stale(File).
# Decl modified(File).
# Decl file_topology(File, PackageName, Imports, Types, Deps).
# Decl next_action(Action).
# Decl system_shard_healthy(ShardName).
# Decl file_in_project(File).
# Decl symbol_reachable(From, To).
# Decl dependency_link(From, To, Type).
# Decl symbol_reachable_bounded(From, To, MaxDepth).
# Decl symbol_reachable_safe(From, To).

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

# Safe bounded reachability using bottom-up path length generation.
# Replaces unsafe top-down budget logic.

Decl path_of_length(From, To, Len).

path_of_length(From, To, 1) :-
    dependency_link(From, To, _).

path_of_length(From, To, Len) :-
    dependency_link(From, Mid, _),
    path_of_length(Mid, To, SubLen),
    Len = fn:plus(SubLen, 1),
    Len <= 15.

symbol_reachable_safe(From, To) :-
    path_of_length(From, To, _).

# symbol_reachable_bounded deprecated/removed to ensure safety.
# If something calls it, it will fail at compile time, which is better than runtime safety error.
# If I wanted to keep it, I'd need a generator for MaxDepth or to bind it.
# symbol_reachable_bounded(From, To, MaxDepth) :-
#    symbol_reachable_safe(From, To),
#    MaxDepth = 15.
