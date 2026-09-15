package shards

import (
	"context"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// OnDemandTriggerPredicates is the closed vocabulary of kernel predicates
// whose assertion can newly derive activate_shard/1 for an on-demand system
// shard. It mirrors the rule bodies in
// internal/core/defaults/policy/system_shards.mg:
//
//	activate_shard(/world_model_ingestor) :- modified(_), ...
//	activate_shard(/tactile_router)       :- action_ready_for_routing(_), ...
//	activate_shard(/session_planner)      :- current_campaign(_), ...
//	activate_shard(/session_planner)      :- user_intent(.../plan...), ...
//
// When a rule body gains a new positive predicate, add it here or the
// runtime watcher (internal/core.StartOnDemandWatcher) will not wake for it.
var OnDemandTriggerPredicates = []string{
	"modified",
	"action_ready_for_routing",
	"current_campaign",
	"user_intent",
}

// EnsureOnDemandShards re-queries activate_shard/1 and spawns every derived
// on-demand system shard that is not already active. It returns the type
// names it spawned.
//
// Why this exists: StartSystemShards queries activate_shard exactly once, at
// boot — before any modified/1, action_ready_for_routing/1, or
// current_campaign/1 fact can exist. Without a runtime re-query, the
// on-demand rules (world_model_ingestor, tactile_router, session_planner)
// derive into a void: the kernel activates shards nobody ever starts. The
// watcher subscribes to OnDemandTriggerPredicates and calls this on every
// trigger burst, closing that loop.
//
// Only StartupOnDemand profiles are spawned here. Auto shards are the boot
// path's responsibility; restarting a dead auto shard is the escalation
// path's job, not this function's.
func (sm *ShardManager) EnsureOnDemandShards(ctx context.Context) []string {
	sm.mu.RLock()
	kernel := sm.kernel
	sm.mu.RUnlock()
	if kernel == nil {
		return nil
	}

	activated, err := kernel.Query("activate_shard")
	if err != nil {
		logging.Get(logging.CategoryShards).Warn("EnsureOnDemandShards: failed to query activate_shard: %v", err)
		return nil
	}

	var spawned []string
	for _, fact := range activated {
		if len(fact.Args) == 0 {
			continue
		}
		name := normalizeShardTypeName(types.ExtractString(fact.Args[0]))
		if name == "" {
			continue
		}

		sm.mu.RLock()
		cfg, ok := sm.profiles[name]
		_, disabled := sm.disabled[name]
		sm.mu.RUnlock()
		if !ok || disabled || cfg.Type != types.ShardTypeSystem {
			continue
		}
		if cfg.StartupMode != types.StartupOnDemand {
			continue
		}
		if sm.hasActiveShardOfType(name) {
			continue
		}

		if _, err := sm.SpawnAsync(ctx, name, "on_demand_activation"); err != nil {
			logging.Get(logging.CategoryShards).Error("EnsureOnDemandShards: failed to start on-demand shard %s: %v", name, err)
			continue
		}
		logging.Shards("EnsureOnDemandShards: started on-demand shard %s", name)
		spawned = append(spawned, name)
	}
	return spawned
}

// hasActiveShardOfType reports whether a live agent carries the profile name.
// Agents are spawned from their profile config, so GetConfig().Name is the
// type identity — not the unique instance ID.
func (sm *ShardManager) hasActiveShardOfType(typeName string) bool {
	for _, agent := range sm.GetActiveShards() {
		if agent == nil {
			continue
		}
		if normalizeShardTypeName(agent.GetConfig().Name) == typeName {
			return true
		}
	}
	return false
}
