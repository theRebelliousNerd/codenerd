# Integration Tests (AegisDB)

## Purpose
Integration tests validate that multiple subsystems work together as a coherent system. In AegisDB that means the hot/cold storage, graph, vector, deductive engine, wormhole pipeline, inference runtime, and protocol surfaces stay consistent under real flows.

## Where integration tests live
- `tests/integration/*.go` for cross-subsystem tests.
- `internal/.../*_test.go` for package-level integration that still runs in-process.

Existing examples to pattern match:
- `tests/integration/comprehensive_integration_test.go` (hybrid pipeline, telemetry parity).
- `tests/integration/wormhole_crystallization_test.go` (temporal integration).
- `tests/integration/api_test.go` (HTTP endpoints; uses build tag).

## Build tags and short mode
- Use `//go:build integration` for tests that need an external server or heavy dependencies.
- Gate long-running tests with `if testing.Short() { t.Skip(...) }`.
- Prefer in-process integration tests (fast) unless the test explicitly validates real sockets or external processes.

## Configuration reality check
AegisDB currently has multiple configuration shapes in the repo.
- `internal/core/config.go` is the canonical config structure for Go runtime.
- `configs/default.yaml` uses `database:` and sets server port to `8888`.
- `configs/testing.yaml` and `configs/development.yaml` use legacy keys (`storage`, `grpc_port`) that do not map to `internal/core.Config`.

Implications for tests:
- Do not blindly call `core.LoadConfig("configs/testing.yaml")` and expect the fields to map. You may get defaults instead.
- Prefer constructing `*core.Config` directly in tests so it is explicit and deterministic.
- If you need file-based config, ensure the YAML matches `internal/core/config.go` (fields under `server`, `database`, `protocols`, `inference`, etc).

## Ports and endpoints
Default ports are not consistent across docs and configs:
- REST is often assumed at `8080` but `configs/default.yaml` uses `8888`.
- gRPC is `9090` in config YAML but defaults to `50051` in code if not set.
- MCP defaults to `8081`, A2A to `8082`, metrics to `9091`.

For integration tests:
- Avoid hardcoding ports. Pass ports through config or environment.
- If starting servers in tests, prefer port `0` to select an available port and query the bound address.
- For HTTP tests against a running server, use an environment variable (example: `AEGISDB_BASE_URL`) and document it in the test.

## Minimal in-process database harness
To avoid external dependencies (Seaweed, Redis), set explicit config fields and use in-memory Badger.

```go
cfg := &core.Config{
    Database: core.DatabaseConfig{
        Badger: core.BadgerConfig{InMemory: true},
        Seaweed: core.SeaweedConfig{
            Endpoint: "",
            Bucket:   "",
        },
        Replication: core.ReplicationConfig{Enabled: false},
    },
    Inference: core.InferenceConfig{
        Embedding: core.EmbeddingConfig{Enabled: false},
    },
}

db, err := core.NewDatabase(cfg)
require.NoError(t, err)
t.Cleanup(func() { _ = db.Close() })
```

Why this matters:
- `core.NewDatabase` starts cold storage if `Seaweed` is configured. Empty endpoint/bucket disables it.
- `core.NewDatabase` will start a vector manager if embeddings are enabled.

## Full-stack service graph harness
The production server uses the managed service graph in `internal/app/server`. For integration tests, you usually want to build the same graph in-process but avoid external dependencies.
Key subsystems and dependencies from `internal/app/server/server.go`:
- `deductive-runtime` (Mangle engine) -> used by wormhole rule lookup.
- `hot-storage` (Badger) -> required.
- `cold-storage` (Seaweed) -> optional.
- `vector-plane` -> optional, enabled when embeddings are enabled.
- `database` -> depends on hot/cold/vector; attaches wormhole engine.
- `journal-retention` -> optional.
- `adk-page-agents` -> optional; uses `AEGISDB_API_BASE_URL`.
- `autonomous-agents` -> optional; depends on `adk-page-agents` and deductive.
- `api-server` -> REST/gRPC/A2A surface.

When you need the real server lifecycle, use `api.NewServer` or `rest.NewServer` with explicit config rather than `server.Run` (which binds real sockets and blocks).

## Integration focus areas by subsystem

### Storage (Badger hot + Seaweed cold)
- Validate that `core.NewDatabase` and `core.NewDatabaseWithServices` behave correctly when cold storage is disabled.
- For replication/cold-sync tests, enable replication and start `db.Start(ctx)` from `internal/core/cold_sync.go`.
- Use `db.ForceColdSync` and `db.ListColdCheckpoints` to validate cold-sync behavior.
- Ensure cold storage credentials are present when enabled; missing credentials cause `Start` to fail.

### Graph
- `internal/graph` and `internal/core` both manage graph nodes. Integration tests should confirm:
  - Node and edge creation results in consistent graph stats.
  - Subgraph definitions in config are honored.
  - Traversal yields correct results across edges.

### Vector and embeddings
- Vector indexes are created via `vector.Manager` or `db.VectorManager()`.
- Dimensions must match embeddings exactly; mismatches should produce errors.
- Integration tests should check vector search results and metadata preservation.

### Deductive (Mangle)
- Use `mangle.NewEngine(cfg)` or `deductive.NewEngine` for Mangle integration.
- Add facts with `AddFact` and validate query results.
- When Mangle feeds wormhole, wire `wormholebridge.RegisterScore` and confirm results flow into telemetry surfaces.

### Wormhole engine (trifecta + candidate service)
- Prefer `TrifectaType: "simple"` for tests to avoid ONNX model dependencies.
- Create a candidate service via `whcandidate.NewService(db, ttl)` and call `EnsureIndex(ctx)`.
- Ensure nodes contain embeddings; `ScorePair` fails when embeddings are missing.
- Validate that `wormhole.Engine.Report()` matches telemetry snapshots.

### Temporal integration and crystallization
- Temporal integration uses thresholds in `internal/wormhole/temporal/integration.go`.
- Crystallization requires:
  - `ValidationResult.Metrics.ObservationCount >= MinObservations`
  - `PersistenceScore` and `AdjustedScore` above thresholds
  - `alphaScore` derived from adjusted score, persistence, stability, and trend
- When testing crystallization, seed enough observations and ensure validation is enabled; otherwise callbacks will not fire.
- Use `require.Eventually` with a timeout instead of a fixed `time.Sleep`.

### Protocol surfaces (REST, gRPC, MCP, A2A)
- REST endpoints are registered in `internal/api/rest/server.go` and include:
  - `/health`, `/api/query`, `/api/nodes`, `/api/graph/*`, `/api/vector/*`, `/api/wormholes`, `/api/inference/*`, `/api/system/*`
- gRPC service is in `internal/api/grpc` and exposes health, graph, stats, telemetry, wormhole candidates.
- MCP and A2A servers live under `internal/protocols`.
- Integration tests should ensure telemetry parity across REST, gRPC, MCP, and A2A.

### MCP resources
- Use `MCPServer.SnapshotResourceForTest` to obtain `aegisdb://schema`, `aegisdb://stats`, `aegisdb://telemetry`, `aegisdb://capabilities` without a live WebSocket.
- Validate `schema` and `stats` payloads include graph, wormhole, and candidate counts.

### A2A telemetry
- Use `a2a.NewA2AServer` with `AttachTelemetry`.
- Validate `telemetry_update` broadcasts include wormhole mode, candidate index size, and last Mangle query.

### Inference
- REST inference endpoints are registered under `/api/inference/*`.
- If ONNX models are not present, tests should expect explicit error responses, not silent success.
- Use `rest.Server.inferenceOptions()` to confirm runtime configuration wiring.

### Security and auth
- REST auth is configured in `api.rest.auth`.
- Use `internal/security` manager for MCP tests where auth/rate limits are enabled.
- Test that protected endpoints reject invalid tokens and accept valid ones.

### Journals and plans
- Ensure `/api/journal/*`, `/api/plans/*`, `/api/hypotheses/*`, and tool registry endpoints are covered in at least one integration flow to validate DB persistence.

## Integration test patterns (from existing code)

### Hybrid pipeline example
`tests/integration/comprehensive_integration_test.go` wires:
- In-memory Badger database
- Candidate service and wormhole engine
- gRPC service for candidates and telemetry
- REST handler for wormhole surfaces
- MCP server for schema/stats snapshot

Use this pattern to validate end-to-end consistency without an external server.

### REST external endpoint example
`tests/integration/api_test.go` uses `baseURL := "http://localhost:8080"` and exercises HTTP endpoints.
For new tests:
- Use a configurable base URL instead of hardcoding `8080`.
- Add `//go:build integration` to ensure it runs only when a server is available.

### Temporal crystallization example
`tests/integration/wormhole_crystallization_test.go` exercises temporal validation.
Use it as a basis but ensure thresholds and observation counts are aligned to trigger crystallization reliably.

## Test data and invariants
- Node IDs should be stable and human readable (`entity:a`, `node-1`) to ease debugging.
- Embeddings must be consistent length across all nodes in a test.
- Use deterministic data to avoid nondeterministic results in similarity scoring.
- For base64-encoded IDs (wormhole surfaces), verify decode correctness and component ordering.

## Observability checks
- For REST, verify `/api/system/telemetry` and `/api/wormholes` reflect the wormhole engine report.
- For gRPC, check `GetTelemetry` and `GetStats` match expected counts.
- For MCP, validate resource snapshots contain the same counts and mode as gRPC/REST.
- For A2A, verify `telemetry_update` broadcast includes a consistent wormhole mode.

## Concurrency and cleanup
- Use `t.Cleanup` for database, servers, goroutines, and channels.
- For goroutine-based services, use contexts and wait groups, not `time.Sleep`.
- For HTTP or gRPC servers started in tests, ensure they stop within a timeout.

## Checklist for new integration tests
- [ ] Uses explicit config struct, avoids legacy YAML fields.
- [ ] All external dependencies disabled or mocked (unless explicitly testing them).
- [ ] Uses in-memory Badger or temp directories.
- [ ] Ensures vector dimensions match embeddings.
- [ ] Uses `require.Eventually` for async readiness.
- [ ] Validates telemetry parity across surfaces.
- [ ] Avoids hardcoded ports and sleeps.

## Example harness snippets

### In-process REST handler
```go
router := gin.New()
router.GET("/wormholes", handler.ListWormholes)
recorder := httptest.NewRecorder()
request := httptest.NewRequest(http.MethodGet, "/wormholes?limit=1&mode=chain", nil)
router.ServeHTTP(recorder, request)
require.Equal(t, http.StatusOK, recorder.Code)
```

### MCP snapshot
```go
msg, err := mcpServer.SnapshotResourceForTest("aegisdb://stats")
require.NoError(t, err)
```

### gRPC service
```go
svc := grpc.NewService(db, "test", candidateSvc, wormEngine, nil, nil, nil)
resp, err := svc.GetTelemetry(ctx, &pb.GetTelemetryRequest{})
require.NoError(t, err)
```

## Known integration hazards
- `core.NewDatabase` will attempt to start Seaweed if it is configured. Disable in tests.
- Temporal crystallization depends on validation metrics; low scores will never trigger callbacks.
- The default config server port is `8888`, while integration tests sometimes assume `8080`.
- `configs/testing.yaml` uses legacy keys and should not be relied upon without verification.

## When to use build tags vs in-process tests
- Use in-process tests when the goal is cross-component correctness (fast, deterministic).
- Use build-tagged tests when validating deployed server behavior or external dependencies.

## Suggested additions (future expansion)
- Dedicated integration harness helpers for shared setup (database + wormhole + mcp).
- Explicit test fixtures for telemetry parity across REST/gRPC/MCP/A2A.
- Structured integration test reports to capture coverage across subsystems.
