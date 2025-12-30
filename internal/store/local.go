package store

// This file serves as the main entry point for the LocalStore implementation.
// The actual implementation has been modularized into the following files:
//
// - local_core.go:         Core types, initialization, and utility functions
// - local_world.go:        World model cache (Fast + Deep AST facts)
// - local_vector.go:       Vector store operations (Shard B)
// - local_graph.go:        Knowledge graph operations (Shard C)
// - local_cold.go:         Cold storage and archival tier (Shard D)
// - local_session.go:      Session management, activation logs, compressed state
// - local_verification.go: Verification records and reasoning traces
// - local_knowledge.go:    Knowledge atoms for agent knowledge bases
// - local_prompt.go:       Prompt atoms for JIT Prompt Compiler
// - local_review.go:       Review findings storage
//
// This modularization keeps each file under 1000 lines while maintaining
// all functionality in the same package (store) with full access to private members.
