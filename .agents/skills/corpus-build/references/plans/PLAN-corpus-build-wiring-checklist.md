# corpus-build: Integration Surface Registry (human-readable source)

**Date**: 2026-03-22 (v1) · **Uplifted in place**: 2026-07-08 (v2)
**Purpose**: The complete catalog of integration surfaces a newly-built subsystem must wire into.

> **v2 rule: the machine-readable registry is authoritative.** This document is the
> human-readable SOURCE that `surfaces.yaml` is authored from; the registry +
> `verify_surfaces.py --registry` produce the verdicts. Hand-counted totals are gone —
> counts are derived from the registry. When this doc and the registry disagree, fix the
> registry first, then this doc.

---

## Registry design (`.agents/skills/corpus-build/references/surfaces.yaml`)

Every surface below becomes one registry entry:

```yaml
- id: B6-mcp-tool
  category: protocol
  name: MCP tool registration
  paths: [internal/mcp/]
  applicability:                # predicates over the feature manifest / tag index
    any:
      - feature_tag: agent-accessible     # NERD_FEATURE topic/plane match
      - manifest_field: integration_points contains "mcp"
      - new_public_operation: true        # heuristic: new exported service methods
  detection:                    # mechanical evidence gathering (grep/glob)
    - grep: 'RegisterTool.*<subsystem>' in internal/mcp/
  verification:                 # command evidence when detection alone is weak
    - cmd: 'go test ./internal/mcp/ -run TestToolRegistry -count=1'
  fix_owner: corpus-comms-plumber
  evidence_required: file:line
```

`verify_surfaces.py --registry` evaluates every entry → `PASS` / `FAIL` /
`N-A` (applicability false) / `AMBIGUOUS` (applicability true, detection inconclusive)
with evidence attached. Agents adjudicate only AMBIGUOUS; humans see only the verdict
table. The dynamic spec injector (§6 of PLAN-corpus-build.md) uses the same registry to
tell a worker, at Read/Write time, which surfaces its package declares.

`fix_owner` routing: `corpus-comms-plumber` (protocol), `corpus-defense-auditor`
(security/telemetry/observation), `corpus-consumables-keeper` (internal/), `corpus-builder`
(engine), orchestrator (codegen gate — never an agent).

---

## Category A: Core Engine Wiring (internal/)

Every new or modified subsystem in `internal/<subsystem>/` must check these:

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| A1 | Storage layer | `internal/store/` | Key prefixes, serialization, migrations | Keys in migrations.go prefix table |
| A2 | Cache layer | `internal/cache/` or `internal/attention-routing/cache/` | L1/L2 cache entries + invalidation | Cache Get/Set calls present |
| A3 | Core database | `internal/core/` | Register with core.Database if subsystem is a first-class engine component | Constructor wiring in NewDatabase |
| A4 | Config structs | `internal/config/` | Go config struct + YAML section in all 4 config files | `configs/{default,development,testing,production}.yaml` |
| A5 | App server init | `internal/app/` | Lifecycle Start/Stop, dependency injection | Registered in server bootstrap |
| A6 | Deductive engine | `internal/mangle/` | External predicates if subsystem exposes Mangle-queryable data | Registered in external_provider*.go |
| A7 | Graph engine | `internal/graph/` | Edge/node type registration if subsystem creates graph entities | Registered in graph service |
| A8 | Vector engine | `internal/vector/` | Embedding registration if subsystem produces embeddable content | Registered in vector manager |
| A9 | Attention-routing engine | `internal/attention-routing/` | Attention-routing policy if subsystem participates in attention scoring | Policy deck registration |
| A10 | codeNERDRAG | `internal/retrieval/` | Entity type + relationship registration for retrieval | Pipeline source registration |
| A11 | codeNERDRAP | `internal/retrieval/` | Ingestion handlers if subsystem produces persistable data | Publisher registration |
| A12 | Inference | `internal/inference/` | ONNX model registration if subsystem uses ML | Model registry entry |
| A13 | Observability | `internal/observability/` | Metrics, traces, health checks | Prometheus collectors registered |
| A14 | Telemetry | `internal/logging/` | Structured logging fields, span creation | structured logging fields + trace spans |
| A15 | Security | `internal/core/defaults/policy/` | constitutional safety (permitted) permissions for new operations | Permission definitions |
| A16 | Backup | `internal/backup/` | New storage paths included in backup scope | Backup scanner paths |
| A17 | Migration | `internal/migration/` | Schema migration if new storage format | Migration script registered |
| A18 | Scheduler | `internal/scheduler/` | Periodic tasks if subsystem needs background work | Task registration |
| A19 | Sleepcycle | `internal/consolidation/` | Consolidation proposals if subsystem benefits from off-peak optimization | Proposal generator hook |
| A20 | Ontology | `internal/ontology/` | Type definitions if subsystem introduces new entity types | Ontology registration |
| A21 | Knowledge | `internal/knowledge/` | Knowledge base entries if subsystem has self-describing capabilities | KB registration |
| A22 | System corpus | `Docs/architecture/` | Agent corpus docs (mission.md, interfaces.md) if subsystem has agents | Corpus directory exists |
| A23 | Testing infra | `internal/testing/` | Test fixtures, test helpers | testutil helpers registered |
| A24 | Testing remediation | `internal/testing/remediation/` | Auto-fix pipeline hookup for the subsystem's failure classes | Remediation handler registered |
| A24a | **Observation collector** | `internal/testing/remediation/observation/collectors/` | Runtime failure collector so the self-fixing Jules pipeline can SEE this subsystem's errors (~30 collectors exist today; every new subsystem with runtime failure modes adds one) + entry in the `subsystemTestCommands` verification map in `observation/adapter.go` | Collector file + registration in `internal/app/server/subsystem_observation.go::buildObservationSystem` |
| A25 | Ingest | `internal/ingest/` | Enrichment step handlers if subsystem processes ingested data | Step handler registration |
| A26 | Memory | `internal/memory/` | Memory management hooks if subsystem has significant state | Memory tracker registration |
| A27 | Blob | `internal/blob/` | Blob handlers if subsystem processes binary files | Blob type handler |
| A28 | Geo | `internal/geo/` | Spatial index registration if subsystem has location data | Spatial index hook |
| A29 | Training | `internal/training/` | Training data export if subsystem produces trainable data | Training pipeline hook |
| A30 | Learning | `internal/learning/` | Auto-tuner parameters if subsystem has tunable knobs | Tunable parameter registration |
| A31 | Replication | `internal/replication/` | Event sourcing if subsystem state must replicate | Event emitter registration |
| A32 | Conflict | `internal/conflict/` | Conflict resolution if subsystem supports concurrent writes | Resolution strategy |
| A33 | Domain | `internal/domain/` | Domain model registration | Domain type registry |
| A34 | Bridge | `internal/system/` | Cross-subsystem bridge if needed | Bridge handler |
| A35 | Capabilities | `internal/capabilities/` | Capability declaration for agent discovery | Capability manifest |
| A36 | Gatekeeper | `internal/gatekeeper/` | Rate limiting / access control rules | Gatekeeper policy |
| A37 | ADK tools | `internal/adktools/` | Shared ADK tool wrappers if subsystem exposes agent tools | Tool registration in shared/ |
| A38 | Agents (permanent) | `internal/shards/` | Page agent if subsystem is user-facing | Agent directory + spec.go |
| A39 | Plugins runtime | `internal/plugins/` | Runtime hook dispatch, lifecycle registration, hook metrics, or cross-system plugin observability if the subsystem emits or consumes hooks | Hook system wiring, runtime registration, metrics/inspection surface |
| A39a | Plugin center | `internal/plugincenter/` | Control-plane domain if subsystem needs persisted operator configuration, managed-domain coordination, or dashboard/MCP overview exposure | Domain adapter + REST/MCP/operator surface wired |
| A40 | AQL | `internal/aql/` | Query language extensions if subsystem adds query surface | AQL operator registration |
| A41 | Query ops | `internal/queryops/` | Query operation handlers | Query handler registration |
| A42 | Experiments | `internal/experiments/` | Feature flag if subsystem has experimental features | Experiment registration |
| A43 | Utils | `internal/utils/` | Shared utilities (only if creating genuinely reusable helpers) | — |
| A44 | Testutil | `internal/internal/testing/` | Test utilities for other packages to use | Test helper functions |
| A45 | Adaptive delivery | `internal/adaptivedelivery/` | Delivery strategy if subsystem serves agent responses | Strategy registration |
| A46 | HTAP | `internal/htap/` | Hybrid transactional-analytical hooks if applicable | HTAP registration |
| A47 | Toolsurface | `internal/toolsurface/` | Tool surface declaration for MCP/ADK discovery | Tool manifest |
| A48 | Vision | `internal/vision/` | Vision processing hooks if subsystem handles images | Vision handler |
| A49 | Sidecar | `internal/sidecar/` | Sidecar communication if subsystem has external processes | Sidecar protocol |
| A50 | Distributed | `internal/distributed/` | Distributed coordination if subsystem spans nodes | Coordination handler |

**Extension note:** `internal/plugins/` and `internal/plugincenter/` are adjacent but distinct.
Use `plugins` for runtime hook/lifecycle integration and `plugincenter` for operator-facing
configuration/control-plane integration. Many subsystems touch one, not both.

## Category B: Protocol Layer (internal/mcp/ + cmd/nerd/)

Primary fix owner: `corpus-comms-plumber`. Every agent-facing or user-facing capability
must answer for ALL of B1–B8 (mostly N-A verdicts are fine; silent absence is not).

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| B1 | REST API | `cmd/nerd/` | Handler file + route registration in registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main) — full route→handler→bind-struct trace; typed GETs need the hand-registered contract in openapi_spec_contracts*.go | Route registered, OpenAPI updated |
| B2 | REST middleware | `cmd/nerd/middleware/` | Auth, rate limiting, CORS for new endpoints | Middleware chain applied |
| B3 | GraphQL | `cmd/nerd/graphql/` | Schema + resolver if subsystem exposes GraphQL | Schema type registered |
| B4 | gRPC | `cmd/nerd/grpc/` | Service definition + handler | Proto compiled, service registered |
| B5 | Realtime/WebSocket | `cmd/nerd/realtime/` | WS handlers if subsystem pushes real-time updates | WS route registered + Tygo types regenerated |
| B6 | MCP protocol | `internal/mcp/` | MCP tool handler for agent-accessible operations | Tool registered in MCP server |
| B7 | A2A protocol | `internal/mcp/` | A2A capability card + handler | Capability exposed |
| B8 | ADK protocol | `internal/tools/` | ADK tool declarations | Tool declared |

## Category C: Codegen & Artifacts

**v2: this category is a serial orchestrator-owned PHASE (5.5 CODEGEN GATE), not agent
work.** Ordering matters: protoc → route snapshot → OpenAPI → API-client codegen → WS types →
TS constants → mocks → parity checks. The OpenAPI regen runs in the host go test/build (scoped packages)
(handlers.test OOMs on host).

| # | Surface | Directory/File | What to Generate | Command |
|---|---------|---------------|-----------------|---------|
| C1 | OpenAPI spec | `docs/api/openapi.v1.json` | Regenerate from live routes | `OpenAPI gen if present` (container path) |
| C2 | API-client codegen API client | `web/dashboard/src/services/generated/` | TypeScript client + Zod schemas | `go generate / client scripts if present` |
| C3 | WebSocket TS types | `web/dashboard/src/services/generated/websockets.ts` | Tygo-generated TypeScript types | `go generate-ws-client` |
| C4 | Protobuf codegen | `MCP/tool schemas{api,mcp,a2a,forge}/*.proto` | Regenerate Go stubs; **Python/TS client bindings currently generated ONLY from `MCP/tool schemasapi/codenerd.proto`** — a new/changed proto in the other 11 files gets Go-only bindings unless explicitly extended | `go generate`, `go generate-python-client`, `go generate-ts-client` |
| C5 | Mock generation | Various `*_mock_test.go` | Regenerate test mocks | `go generate-mocks` |
| C6 | TS constants | `internal/perception/typescript/constants.ts` + `web/dashboard/src/constants/generated/` | TypeScript constants from Go domain constants | `go generate-ts-constants` |
| C7 | ADK tool codegen | `internal/shards/*/tools_generated_*.go` | Page-specialist ADK tools | `go generate-adk-tools` |

**Known CI gaps (as of 2026-07-08, verified against `.github/workflows/ci.yml`):**
OpenAPI (`api-openapi-spec`), API-client codegen (`dashboard-api-client`), route changelog
(`api-route-changelog`), and Go↔Python↔TS client parity (`client-parity`) ARE CI-gated.
**Proto regen and `generate-ts-constants` are NOT** — no job regenerates and diffs them.
Closing these two is work item W11 in PLAN-corpus-build.md.

## Category D: Client Consumables (internal/)

Primary fix owner: `corpus-consumables-keeper`. The standing rule: a customer-visible
surface change is NOT DONE until internal/ reflects it (the 5th ship dimension).

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| D1 | Go reference client | `internal/perception/go/` | Client methods for new API endpoints (Go is the parity reference) | Methods match REST handlers |
| D2 | Python + TS clients | `internal/perception/{python,typescript}/` | Parity with Go reference | `scripts/check-client-parity.sh` green (CI-gated) |
| D3 | Long-tail clients | `internal/perception/{java,csharp,ruby,rust}/` + `internal/perception/missions/` | Parity with Go reference | `consumables_parity.py` (v2 — extends parity to all 7 languages; NOT CI-gated today, W11) |
| D4 | CLI | `cmd/nerd/` + `cmd/nerd/` | New commands/subcommands for new capabilities | Commands registered |
| D5 | SDK integrations | `internal/sdk/` (LangChain, LangGraph, AutoGen, Google-ADK — Python) | Tool/toolkit surface exposes new capabilities | Integration modules updated; auth-style trifecta respected (Basic=ADK, Bearer=langchain/langgraph/autogen/go, X-API-Key=python/ts) |
| D6 | Customer skills | `.agents/skills/codenerd-*/` (~39 skills) | Skill references/scripts teach the new surface — written for ZERO-codebase-access customers, including how to OBTAIN credentials | `consumables_parity.py --skills`: endpoint mentions in skill references diffed against live route list |
| D7 | Stray-module hygiene | `internal/` top level | internal/ contains ONLY consumables (skills, sdk, cli, client). `internal/styling/` is a known stray (single attention-routing_shadow.go) pending relocation by the architect | Registry flags non-consumable top-level dirs |

## Category E: Binaries (cmd/)

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| E1 | Main server | `cmd/codenerd/` | Subsystem initialization in main | Init call present |
| E2 | Dedicated server | `cmd/codenerd-server/` | Same as E1 if separate server binary | Init call present |
| E3 | CLI tool | `cmd/nerd/` | New subcommands for subsystem operations | Command registered |
| E4 | Seed tool | `cmd/codenerd-seed/` | Seed data for new entity types | Seed function present |
| E5 | Mangle LSP | `cmd/codenerd-manglelsp/` | LSP completions if new Mangle predicates added | Completion provider updated |

## Category F: Frontend (web/dashboard/)

Frontend work follows the object-oriented UI archetype (codenerd-viz-orchestrator owns
conformance; graphcad/graphNERD naming is fixed). Fix routing: viz-scaffolder /
graphcad pipeline agents, NOT corpus-builder.

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| F1 | React components | `web/dashboard/src/components/` | UI components for subsystem, archetype-conformant (WorkspaceSlice<T>, ObjectUtil<T>, JSONForms inspector) | Components render; viz-archetype-auditor clean |
| F2 | Pages/routes | `web/dashboard/src/pages/` | Dashboard pages for subsystem | Routes registered |
| F3 | API hooks | `web/dashboard/src/hooks/` | React Query hooks using generated API-client codegen client | Hooks call correct endpoints |
| F4 | State management | `web/dashboard/src/stores/` | Zustand/context stores if needed | State flows correctly |
| F5 | Config panel | `web/dashboard/` (settings page) | Config hot-reload: dashboard settings → REST config endpoint → Viper watch | Config changes apply without restart |
| F6 | E2E tests | `web/dashboard/e2e/` | Playwright specs for new pages | E2E tests pass |
| F7 | Vitest tests | `web/dashboard/src/**/*.test.tsx` | Unit tests for new components | Tests pass |
| F8 | Pagekit agent controllability | `internal/shards/<agent>/spec.go` | Every user-facing feature controllable by its page agent through shard-UI.Spec — tools declared, handlers implemented, DOM selectors mapped | Page agent can invoke every subsystem operation |
| F9 | Page agent tools | `internal/shards/<agent>/tools.go` | ADK function tools wrapping subsystem operations | All tools build, return structured results |
| F10 | Page agent corpus | `Docs/architecture/docs/agents/<agent>/` | mission.md, interfaces.md, playbook.md | Corpus passes prompt-architect / corpus-doc-auditor audit (`TestAgentCorpora_ExistsForAllDiscoveredAgents`) |

## Category G: Configuration

| # | Surface | File(s) | What to Wire | Verification |
|---|---------|---------|-------------|-------------|
| G1 | Default config | `.nerd/config.json` | New config section with sane defaults | Section present |
| G2 | Dev config | `configs/development.yaml` | Dev-appropriate overrides | Section present |
| G3 | Test config | `configs/testing.yaml` | Test-appropriate values | Section present |
| G4 | Prod config | `configs/production.yaml` | Production-hardened values | Section present |
| G5 | Viper hot-reload | Go code | `viper.WatchConfig()` + callback for dynamic knobs | Hot-reload tested |
| G6 | Docker compose | `deployments/docker/docker-compose.dev.yml` | Environment variables, volume mounts if needed | Container starts |
| G7 | Makefile | `Makefile` | New targets if subsystem has custom build/test/codegen steps | Target works |
| G8 | Config struct parity | `internal/config/` | Every YAML key has a Go struct field and vice versa (registry detection: struct-tag ↔ YAML-key diff) | No orphan keys in either direction |

## Category H: Documentation & Corpus

Primary fix owner: `corpus-doc-auditor` (the only fleet agent with Docs/architecture write access).

| # | Surface | Directory/File | What to Wire | Verification |
|---|---------|---------------|-------------|-------------|
| H1 | Architecture docs | `Docs/architecture/<subsystem>/` | IMPLEMENTED_SPEC §Implementation Status reflects shipped reality | Status matches gate evidence |
| H2 | Feature tags + frontmatter | same | `NERD_FEATURE` tags current (plane flips gap→current); doc frontmatter stamped (tag-as-you-go) | build_tag_index.py runs clean; registers 31/32 regenerated |
| H3 | Context index | `Docs/architecture/roadmap/33_corpus_context_index.json` | Regenerated (machine-owned) | Committed index matches regen output |
| H4 | Agent corpus | `Docs/architecture/docs/agents/` | mission.md, interfaces.md for new agents | Corpus files exist |
| H5 | API docs | `docs/api/` | OpenAPI spec current | `make test-openapi-spec` passes |
| H6 | Mangle rules | `rules/` or inline `.mg` | New deductive rules documented | Rules stratify cleanly (`make mangle-antipattern-check`) |

## Category I: Testing & Quality

| # | Surface | What to Verify | Command |
|---|---------|---------------|---------|
| I1 | Unit tests exist | `*_test.go` for every new `.go` file (five-case table: happy, nil/empty, error, boundary, concurrency) | `go test ./internal/<subsystem>/...` |
| I2 | Race detector clean | No data races under concurrent access | `go test -race ./internal/<subsystem>/...` |
| I3 | Benchmarks exist | At least 1 benchmark per key function | `go test -bench=. ./internal/<subsystem>/...` |
| I4 | Vet clean | No vet warnings | `go vet ./internal/<subsystem>/...` |
| I5 | Lint clean | No lint warnings | golangci-lint |
| I6 | Coverage threshold | >70% line coverage for new code | `spot_check_coverage.py` per WU; full profile at gate |
| I7 | Mangle anti-pattern check | No stratification violations | `make mangle-antipattern-check` |
| I8 | OpenAPI parity | Generated spec matches live routes | `make test-openapi-spec` |
| I9 | API client parity | Generated client matches spec | `client parity checks if present` |
| I10 | Client-language parity | Go↔Python↔TS (CI) + long-tail languages (v2 script) | `check-client-parity.sh`, `consumables_parity.py` |
| I11 | Cyber torture (if security-relevant) | Passes protocol conformance | `make test-cyber-torture-protocol` |
| I12 | OOM discipline | No host build/test of `cmd/nerd` anywhere in the run | block-oom-build hook log empty of overrides |

---

## Verdict model

Counts are registry-derived — no more hand-maintained totals. Every surface is evaluated
per run as:

- **PASS** — applicable, evidence found (file:line or command output attached)
- **FAIL** — applicable, evidence absent → dispatched to `fix_owner`
- **N-A** — applicability predicates false (recorded, not silent)
- **AMBIGUOUS** — applicable but detection inconclusive → `corpus-wiring-auditor` adjudicates

Intentional skips of applicable surfaces require a skip record
(`.corpus-build/skips.jsonl` with reason) — the run report shows skips next to passes.
