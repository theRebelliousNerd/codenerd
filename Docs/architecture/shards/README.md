# shards — Architecture Corpus (`internal/shards`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary packages: `internal/shards/`, `internal/shards/system/`  
> Sibling lifecycle owner: `internal/core/shards` (ShardManager)

## Scope

This corpus documents **shard registration, Type-1 system shards, specialist matching, consultation, and background observers**. Domain persona shards (coder/tester/reviewer/…) are **gone as Go packages**; they live as JIT personas + `session.Executor`. What remains here is the **OODA executive plumbing** and **specialist orchestration helpers**.

It is **not** the ShardManager implementation (`Docs/architecture/core/` / `internal/core/shards/`), **not** the session JIT executor (`Docs/architecture/session/`), and **not** policy Mangle sources (`internal/core/defaults/policy/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | On-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, OODA fact pipeline, lifecycle |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported surface |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot / factory / chat registration |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Constitution gate, cost guard, boot guard |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, heartbeats, glass box |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Build / verify

```powershell
# Targeted tests (package + system subpackage)
go test ./internal/shards/...

# With sqlite-vec CGO flags when full binary needed
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
go test ./internal/shards/...
```

## One-paragraph architecture

```
user → perception_firewall → user_intent
  → Mangle policy derives next_action
  → executive_policy → pending_action
  → constitution_gate → permitted_action | deny/security_violation
  → tactile_router → exec_request / tool dispatch → VirtualStore
```

System shards are factories registered via `RegisterAllShardFactories` (`registration.go`) or inline in `cmd/nerd/chat/session_boot.go` / `internal/system/factory.go`. Profiles define Auto vs OnDemand startup. CostGuard + StrictMode constitution + executive boot guard bound runaway loops.

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring journals, honest gaps — **not** auto-generated file tables alone. The package README at `internal/shards/README.md` still describes a 2024 “everything deleted” migration; **treat this corpus as authoritative for current code**.
