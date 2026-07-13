# Integration Surface Checklist

Dynamic 9-category checklist with 102 potential surfaces. The wiring-auditor agent evaluates each surface as REQUIRED / OPTIONAL / N-A based on the subsystem spec, then verifies REQUIRED surfaces pass.

## How It Works

1. **verify_surfaces.py** (or the agent directly) DISCOVERS candidate surfaces by scanning directories: internal/*/, internal/*/, cmd/*/, internal/mcp/*/, cmd/nerd/*/, web/dashboard/
2. The **corpus-wiring-auditor agent** CLASSIFIES each discovered surface as REQUIRED/OPTIONAL/N-A by reading the subsystem spec. This is an architectural judgment, not a mechanical check.
3. The agent VERIFIES classified-as-REQUIRED surfaces with file:line evidence.

## Categories

### A: Core Engine Wiring (internal/) -- 50 surfaces

Every new or modified subsystem in internal/<subsystem>/ must check: storage layer (sqlite store key prefixes via internal/store/), cache layer (L1/L2), core database registration, config structs, app server lifecycle, deductive engine predicates, graph engine registration, vector engine embeddings, attention-routing policy, codeNERDRAG entity types, codeNERDRAP ingestion, inference models, observability metrics, telemetry spans, security constitutional safety (permitted), backup paths, migration scripts, scheduler tasks, consolidation proposals, ontology types, knowledge base entries, system corpus docs, testing infra, testing remediation, ingest enrichment, memory management, blob handlers, geo spatial, training data, learning auto-tuner, replication events, conflict resolution, domain models, bridge handlers, capabilities declaration, gatekeeper policies, ADK tools (internal/adktools/), permanent agents, plugin center hooks, AQL operators, query ops handlers, experiments flags, utils, testutil helpers, adaptive delivery, HTAP hooks, tool surface declaration, vision processing, sidecar communication, distributed coordination.

Key verification patterns:
- A1 (Storage): Check internal/store/migrations.go for key prefix registration
- A3 (Core): Check internal/core/ for NewDatabase constructor wiring
- A6 (Deductive): Check internal/mangle/ for external predicate registration
- A37 (ADK tools): Check internal/adktools/shared/ for tool registration

### B: Protocol Layer -- 8 surfaces

REST API (cmd/nerd/), REST middleware, GraphQL, gRPC, WebSocket/Realtime, MCP protocol (internal/mcp/), A2A protocol (internal/mcp/), ADK tools (internal/adktools/ -- note: NOT internal/tools/ which does not exist).

Key verification: route registration in registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main), MCP tool handler, A2A capability card.

### C: Codegen and Artifacts -- 6 surfaces

OpenAPI spec (docs/api/openapi.v1.json via OpenAPI gen if present), API-client codegen API client (web/dashboard/src/services/generated/ via go generate / client scripts if present), WebSocket TS types (via go generate-ws-client), Protobuf codegen (MCP/tool schemas via go generate), Mock generation (via go generate-mocks), TS constants (cmd/generate-ts-constants/).

### D: Client Libraries (internal/) -- 5 surfaces

Go client (internal/perception/go/), HTTP client, CLI commands (cmd/nerd/), SDK types (internal/sdk/), Skills (.agents/skills/).

### E: Binaries (cmd/) -- 5 surfaces

Main server (cmd/codenerd/), dedicated server (cmd/codenerd-server/), CLI tool (cmd/nerd/), seed tool (cmd/codenerd-seed/), Mangle LSP (cmd/codenerd-manglelsp/).

### F: Frontend (web/dashboard/) -- 7 surfaces

React components, pages/routes, API hooks (React Query + API-client codegen), state management (Zustand), config panel (hot-reload), E2E tests (Playwright), unit tests (Vitest).

### G: Configuration -- 7 surfaces

4 YAML config files (configs/{default,development,testing,production}.yaml), Viper hot-reload, Docker compose (deployments/docker/docker-compose.dev.yml), Makefile targets.

### H: Documentation and Corpus -- 4 surfaces

Architecture docs (Docs/architecture/<subsystem>/ -- update Section 3 status only), agent corpus (Docs/architecture/docs/agents/), API docs (docs/api/), Mangle rules documentation.

### I: Testing and Quality -- 10 surfaces

Unit tests exist, race detector clean, benchmarks exist, vet clean, lint clean, coverage >70%, Mangle anti-pattern check, OpenAPI parity, API client parity, cyber torture (if security-relevant).

## Summary

| Category | Count | Scope |
|----------|-------|-------|
| A: Core Engine | 50 | 1-to-1 with internal/ directories |
| B: Protocol Layer | 8 | REST, GraphQL, gRPC, WS, MCP, A2A, ADK |
| C: Codegen | 6 | OpenAPI, API-client codegen, Tygo, tool/MCP schema, mocks, TS |
| D: Client Libraries | 5 | Go client, HTTP, CLI, SDK, skills |
| E: Binaries | 5 | Main server, dedicated, CLI, seed, LSP |
| F: Frontend | 7 | Components, pages, hooks, state, config, E2E, unit |
| G: Configuration | 7 | 4 YAML, Viper, Docker, Makefile |
| H: Documentation | 4 | Arch docs, agent corpus, API docs, Mangle |
| I: Testing | 10 | Unit, race, bench, vet, lint, coverage, Mangle, OpenAPI, client, cyber |
| **TOTAL** | **102** | |

Not every surface applies to every subsystem. The wiring-auditor classifies each as REQUIRED/OPTIONAL/N-A.

## Source

Full surface-by-surface detail with directory paths and verification commands: `.claude/skills/PLAN-corpus-build-wiring-checklist.md`
