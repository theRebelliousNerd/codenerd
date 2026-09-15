package core

// =============================================================================
// MODULARIZATION NOTICE
// =============================================================================
// The RealKernel implementation has been modularized into several files:
//
// - kernel_types.go: Type definitions, constants, data structures (RealKernel, Fact aliases)
// - kernel_init.go: Constructors (NewRealKernel, workspace setup, corpus boot)
// - kernel_facts.go: Fact management (Assert, Retract, LoadFacts)
// - kernel_facts_intern.go: Predicate interning for the fact path
// - kernel_fact_decl.go: Decl-directed fact encoding at the source boundary
// - kernel_query.go: Query execution and pattern matching
// - kernel_eval.go: Policy evaluation, full/differential fixpoint
// - kernel_validation.go: Schema validation and safety checks
// - kernel_policy.go: Policy/schema loading, HotLoad paths
// - kernel_virtual.go: VirtualStore attachment (SetVirtualStore/GetVirtualStore)
// - kernel_accessors.go: Lock-guarded field accessors
// - kernel_provenance.go: Derivation provenance tracking
// - kernel_safe_action.go: safe_action/1 permission surface
// - kernel_shard.go: Shard-scoped kernel views
// - kernel_transactions.go: Transactor support for atomic updates
// - kernel_undeclared.go: Undeclared-predicate handling
// - kernel_utils.go: Shared kernel helpers
// - kernel_sysfacts.go: System facts (wall clock, git state) refreshed into the EDB
//
// All functionality has been moved to the appropriate modular files.
// This file remains as a package marker and documentation reference.
