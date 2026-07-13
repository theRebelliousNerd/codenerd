# Work Unit Types and Agent Routing

11 work unit types produced by the corpus-judge build plan. The orchestrator dispatches each to the agent listed below.

## Agent Routing Table

| Type | Description | Agent | Model | Isolation | Post-Action |
|------|------------|-------|-------|-----------|-------------|
| 1 | New Go package/file | corpus-builder | sonnet | worktree | go build |
| 2 | Complete partial implementation | corpus-builder | sonnet | worktree | go build |
| 3 | Unit tests | go-architect / test-forge unit | sonnet | worktree | go test -race |
| 4 | Integration tests | test-forge integration | sonnet | worktree | go test -race |
| 5 | Cross-system tests | cross-system-test-architect | opus | worktree | go test -race |
| 6 | REST API endpoints | corpus-comms-plumber | sonnet | worktree | OpenAPI gen if present |
| 7 | Frontend page agent updates | corpus-builder | sonnet | worktree | prompt-architect / corpus-doc-auditor |
| 8 | Mangle rules | corpus-builder + mangle-programming | sonnet | worktree | mangle stratification check |
| 9 | System corpus docs | prompt-architect / corpus-doc-auditor (automated) | sonnet | main | corpus file check |
| 10 | Wiring verification | corpus-wiring-auditor | sonnet | main | surface pass/fail report |
| 11 | Protocol handlers (MCP/A2A/gRPC) | corpus-builder | sonnet | worktree | go generate (tool/MCP schema) |

## Type Details

### Type 1: New Go Package/File
**Input**: Spec section describing the feature, interface signatures, data types, invariants
**Output**: New .go files with full implementations (no stubs)
**Constraint**: Match interfaces defined in spec foundation docs. Read existing code for patterns.

### Type 2: Complete Partial Implementation
**Input**: Existing file + spec for what is missing + TODO.md items
**Output**: Completed implementation in existing files
**Constraint**: Match existing code style. Do not break existing tests.

### Type 3: Unit Tests
**Input**: Newly created Go functions from Type 1/2 work units
**Output**: Table-driven tests with edge cases
**Trigger**: Automatically after each Type 1/2 completes in the same level or next level

### Type 4: Integration Tests
**Input**: Cross-component interactions defined in spec
**Output**: Integration test files exercising multi-component flows
**Trigger**: After all Type 1/2 units for a dependency cluster complete

### Type 5: Cross-System Tests
**Input**: Subsystem interaction patterns from spec deep-dives
**Output**: Chaos/adversarial/contract tests
**Trigger**: After integration tests pass, for subsystems touching 3+ others
**Note**: Reserved for complex cross-cutting scenarios. Uses opus for architectural reasoning.

### Type 6: REST API Endpoints
**Input**: Spec-defined API surface, existing handler patterns in cmd/nerd/
**Output**: Handler file, route registration (via intent file for registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)), OpenAPI spec updates
**Post-Action**: Regenerate API-client codegen client per feedback_regen_apis.md

### Type 7: Frontend Page Agent Updates
**Input**: New tools/capabilities the page agent needs from the subsystem
**Output**: Updated tools.go, spec.go, corpus docs in internal/shards/
**Post-Action**: Run prompt-architect / corpus-doc-auditor to update mission.md, interfaces.md

### Type 8: Mangle Rules
**Input**: Deductive reasoning requirements from spec
**Output**: .mg rule files, external predicate registrations in internal/mangle/
**Constraint**: Must pass stratification check. Read mangle-programming skill context.

### Type 9: System Corpus Docs
**Input**: New/modified agent capabilities from Types 1-8
**Output**: Updated mission.md, interfaces.md, playbook.md in Docs/architecture/docs/agents/
**Automation**: Invoked via prompt-architect / corpus-doc-auditor skill, not dispatched as a subagent

### Type 10: Wiring Verification
**Input**: All completed code from Types 1-9
**Output**: Wiring verification checklist with PASS/FAIL/SKIP per surface
**Agent**: corpus-wiring-auditor reads references/02-integration-surface-checklist.md

### Type 11: Protocol Handlers (MCP/A2A/gRPC)
**Input**: Spec-defined agent-facing operations
**Output**: Handler registration in internal/mcp/{mcp,a2a}/, tool/MCP schema definitions if gRPC
**Post-Action**: go generate for tool/MCP schema codegen if .proto files modified

## Agent Selection Rationale

- **corpus-builder** (new) for Types 1, 2, 7, 8, 11: Requires reading spec and building new code. Existing agents are purpose-built for mutation or test-writing, not spec-to-code generation.
- **Existing agents** for Types 3-6: go-architect / test-forge unit, test-forge integration, cross-system-test-architect, and corpus-comms-plumber already handle these exact tasks. Reusing avoids duplicating proven prompts.
- **corpus-wiring-auditor** (new) for Type 10: Purpose-built for the dynamic surface checklist.
- **prompt-architect / corpus-doc-auditor** (existing skill) for Type 9: Already handles system corpus lifecycle.

### Routing cautions (learned corpusbuild_graphcad_20260709)

- **corpus-doc-auditor's charter is Docs/architecture/ ONLY.** Never route bulk doc
  normalization outside that tree (Docs/architecture/, repo-root docs, internal/skills)
  to it — it will (correctly) refuse the dispatch. Route such work to a general-purpose
  implementer. A plan's `agent_routing_note` is judge output, not charter authority;
  the charter wins.
- **Slice manifests are exact-path matched.** A directory entry (with or without a
  trailing slash) does NOT grant writes to files inside it — every file the worker
  will touch must appear as its own exact path in
  `.corpus-build/slices/current/<agent_type>.json`. Workers may self-extend their own
  manifest (append the exact path first, then edit); expect concurrent-edit rejects
  if the orchestrator writes the same manifest mid-flight.

## Cost Estimates by Type

| Type | Model | Est. Input Tokens | Est. Cost |
|------|-------|-------------------|-----------|
| 1 | sonnet | ~50K | ~$0.15 |
| 2 | sonnet | ~40K | ~$0.12 |
| 3 | sonnet | ~30K | ~$0.09 |
| 4 | sonnet | ~50K | ~$0.15 |
| 5 | opus | ~100K | ~$1.50 |
| 6 | sonnet | ~40K | ~$0.12 |
| 7 | sonnet | ~30K | ~$0.09 |
| 8 | sonnet | ~40K | ~$0.12 |
| 9 | sonnet | ~20K | ~$0.06 |
| 10 | sonnet | ~80K | ~$0.24 |
| 11 | sonnet | ~40K | ~$0.12 |

Estimated total for a medium build (14 work units): ~$5-8
