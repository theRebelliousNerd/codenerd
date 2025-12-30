# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# =============================================================================
# MODULARIZATION INDEX
# =============================================================================
# This file has been split into focused modular files for maintainability.
# Each file contains related predicate declarations organized by domain.
#
# File Structure (18 modules, all under 600 lines):
#
# schemas_intent.mg (26 lines)
#   - Sections 1-2: Intent Schema, Focus Resolution
#   - user_intent, focus_resolution, ambiguity_flag
#
# schemas_world.mg (45 lines)
#   - Sections 3-5: File Topology, Symbol Graph, Diagnostics
#   - file_topology, symbol_graph, dependency_link, diagnostic
#
# schemas_execution.mg (42 lines)
#   - Sections 9-10: TDD Repair Loop, Action & Execution
#   - test_state, next_action, action_executed, tool_invocation
#
# schemas_browser.mg (49 lines)
#   - Sections 8, 17: Browser Physics, Spatial Reasoning
#   - dom_element, spatial_relation, viewport_state
#
# schemas_project.mg (74 lines)
#   - Sections 18-20: Project Profile, User Preferences, Session State
#   - project_profile, user_preference, session_checkpoint
#
# schemas_dreamer.mg (108 lines)
#   - Sections 38, 48: Speculative Dreamer, Cross-Module Support
#   - dream_state, scenario_outcome, system_invariant_violated
#
# schemas_memory.mg (139 lines)
#   - Section 7: Memory Tiers & Knowledge (7A-7F)
#   - vector_recall, knowledge_link, learned_preference, activation
#
# schemas_knowledge.mg (142 lines)
#   - Sections 24-26, 44, 52: Knowledge Atoms, LSP, Semantic Matching, Sparse Retrieval
#   - knowledge_atom, lsp_completion, semantic_match, issue_keyword
#
# schemas_safety.mg (160 lines)
#   - Sections 11, 21-23: Constitution, Git Safety, Shadow Mode, Diff Approval
#   - permitted, git_commit, shadow_simulation, diff_approval
#
# schemas_analysis.mg (205 lines)
#   - Sections 12-16, 37, 39: Spreading Activation, Strategy, Impact, Autopoiesis
#   - activation_score, strategy_selected, impact_analysis, learning_signal
#
# schemas_misc.mg (217 lines)
#   - Sections 43, 49, 50: Northstar Vision, Continuation Protocol, Benchmarks
#   - northstar_goal, continuation_step, benchmark_result
#
# schemas_codedom.mg (238 lines)
#   - Section 34: Code DOM & Interactive Elements
#   - code_element, element_signature, element_modified
#
# schemas_testing.mg (240 lines)
#   - Sections 35-36, 51: Verification Loop, Reasoning Traces, Pytest
#   - verification_result, reasoning_trace, pytest_failure
#
# schemas_campaign.mg (295 lines)
#   - Section 27: Campaign Orchestration (Long-Running Goals)
#   - campaign, campaign_phase, campaign_task, campaign_dependency
#
# schemas_tools.mg (394 lines)
#   - Sections 28-29, 40: Ouroboros, Tool Learning, Intelligent Routing
#   - tool_registered, tool_capability, tool_execution, tool_routing
#
# schemas_prompts.mg (421 lines)
#   - Sections 42, 45: Dynamic Prompt Composition, JIT Compiler
#   - shard_prompt_base, prompt_atom, atom_selection_score
#
# schemas_reviewer.mg (426 lines)
#   - Sections 41, 47: Missing Declarations, Static Analysis Data Flow
#   - review_finding, data_flow_source, nil_deref_risk
#
# schemas_shards.mg (523 lines)
#   - Sections 6, 30-33: Shard Delegation & Coordination
#   - delegate_task, shard_profile, shard_result, multi_shard_review
#
# =============================================================================
# USAGE NOTES
# =============================================================================
# When loading schemas into the Mangle engine, you can either:
#
# 1. Load all modules (recommended for full functionality):
#    - Load all 18 schemas_*.mg files
#
# 2. Load selectively based on subsystem needs:
#    - For basic execution: intent, world, execution, safety
#    - For campaigns: + campaign, tools
#    - For review workflows: + reviewer, testing, codedom
#    - For full system: all modules
#
# The modular structure allows:
# - Faster development (smaller files, easier navigation)
# - Selective loading (only load what you need)
# - Clear boundaries (each domain is self-contained)
# - Easier testing (test individual subsystems)
#
# =============================================================================
# MIGRATION NOTES
# =============================================================================
# The original 3622-line schemas.mg has been preserved as schemas.mg.bak
# All predicate declarations remain unchanged - only the file organization
# has been improved.
#
# To verify completeness:
#   cat schemas_*.mg | grep "^Decl" | wc -l
#   # Should match the count from original schemas.mg
#
# =============================================================================
# CORE PREDICATES (Quick Reference)
# =============================================================================
# These are the most commonly used predicates across the system:

# Intent & Focus
#   user_intent(ID, Category, Verb, Target, Constraint) → schemas_intent.mg
#   focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence) → schemas_intent.mg

# File Topology & AST
#   file_topology(Path, Hash, Language, LastModified, IsTestFile) → schemas_world.mg
#   symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature) → schemas_world.mg
#   dependency_link(CallerID, CalleeID, ImportPath) → schemas_world.mg

# Execution
#   next_action(Action) → schemas_execution.mg
#   permitted(Action) → schemas_safety.mg

# Shard Coordination
#   delegate_task(ShardType, TaskDescription, Status) → schemas_shards.mg
#   shard_result(TaskID, Status, ShardType, TaskDescription, ResultSummary) → schemas_shards.mg

# Memory & Learning
#   learned_preference(Predicate, Args) → schemas_memory.mg
#   activation(FactID, Score) → schemas_memory.mg

# Campaign Management
#   campaign(CampaignID, Goal, Status, StartedAt) → schemas_campaign.mg
#   campaign_task(CampaignID, TaskID, Description, Status, Priority) → schemas_campaign.mg

# For detailed predicate documentation, see the individual modular files.
