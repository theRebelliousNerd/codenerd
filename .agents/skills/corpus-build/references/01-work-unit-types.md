# Work Unit Types and Agent Routing (codeNERD)

Work units produced by corpus-judge. Orchestrator dispatches each to the agent below.

| Type | Description | Agent | Isolation | Post-Action |
|------|-------------|-------|-----------|-------------|
| 1 | New Go package/file | corpus-builder | worktree | go test / build package |
| 2 | Complete partial implementation | corpus-builder | worktree | go test package |
| 3 | Unit tests | corpus-builder or test-forge unit | worktree | go test -race |
| 4 | Integration tests | corpus-builder or test-forge integration | worktree | go test -race |
| 5 | Cross-system tests | go-architect / cross-system agent | worktree | go test -race |
| 6 | CLI / cmd surface | corpus-builder | worktree | go build ./cmd/nerd |
| 7 | Shard registration / lifecycle | corpus-builder (+ intents) | worktree | registration compile |
| 8 | Mangle rules / schemas / policy | corpus-builder + mangle-logic-architect | worktree | mangle check / boot |
| 9 | Prompt atoms / JIT selection | corpus-builder + prompt-jit | worktree | atom load tests |
| 10 | Wiring verification | corpus-wiring-auditor | main | verify_surfaces.py |
| 11 | VirtualStore / tools / MCP | corpus-builder (+ intents) | worktree | route tests |
| 12 | Spec / architecture status docs | corpus-doc-auditor | main | status reconcile |

## Type Notes

### Type 1–2 — Go implementation
Match existing package style. Context first, never ignore errors, race-safe.

### Type 3–5 — Tests
Five-case discipline: happy, nil/empty, error, boundary, concurrency.
Do not delete failing product tests to force green.

### Type 6 — CLI
`cmd/nerd/` handlers, flags, chat commands. Prefer intent files for shared registration.

### Type 7 — Shards
`internal/shards/`, manager, registration.go via intents.

### Type 8 — Mangle
`Decl` required. Atoms `/lowercase`. Variables Uppercase. Negation safety. Aggregation `|>`.
Read `internal/mangle/agents.md` and skill `mangle-programming`.

### Type 9 — Prompt atoms
`internal/prompt/atoms/<category>/`. JIT-first. No ad-hoc shard prose dumps.

### Type 10 — Wiring
Consumes `references/surfaces.yaml` + intents. Verdicts PASS/FAIL/N-A/AMBIGUOUS.

### Type 11 — VirtualStore / tools
Routing tables, tool registration, MCP bridges.

### Type 12 — Docs
Only corpus-doc-auditor writes architecture status rows from gate evidence.
