package core

// =============================================================================
// SHARD MANAGER - MODULARIZED ENTRY POINT
// =============================================================================
//
// This file serves as the primary entry point for the ShardManager system.
// The implementation has been split across multiple focused files:
//
// - shard_base.go           : BaseShardAgent implementation and type aliases
// - shard_config.go         : Configuration helper functions
// - shard_manager_core.go   : ShardManager struct and core operations
// - shard_manager_tools.go  : Intelligent tool routing (§40)
// - shard_manager_spawn.go  : Shard spawning and execution logic
// - shard_manager_facts.go  : Fact conversion and utilities
// - shard_manager_feedback.go : Reviewer feedback interface
//
// This modularization keeps each file under 1000 lines while maintaining
// package coherence and avoiding import cycles.
//
// =============================================================================

// All types, constants, and methods are defined in the modular files above.
// This file intentionally left minimal to serve as documentation.
