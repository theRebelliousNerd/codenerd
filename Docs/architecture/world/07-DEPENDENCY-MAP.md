# world — Dependency Map

> Last verified: **2026-07-13**

## Upstream (what world imports)

| Package | Why |
|---------|-----|
| `codenerd/internal/types` | Fact/MangleAtom/GraphQuery aliases (cycle break) |
| `codenerd/internal/core` | `core.Fact`, `MangleAtom`, `RealKernel` queries (holographic) |
| `codenerd/internal/store` | LocalStore world file/fact cache |
| `codenerd/internal/logging` | CategoryWorld |
| `codenerd/internal/features` | Fast scan workers / AST max bytes |
| `codenerd/internal/mangle` | LSP engine/server (subpackage) |
| `codenerd/internal/tools/codedom` | TestDependencyAnalyzer interface |
| stdlib | `go/ast`, `go/parser`, `go/token`, `os/exec` (git), sync, path |
| `github.com/smacker/go-tree-sitter` (+ lang grammars) | Fast AST / multi-lang parsers |

**Avoided / indirect:** world does not import `session`, `cli`, `campaign` (callers import world).

## Downstream (who imports world)

### cmd

| Path | Usage |
|------|-------|
| `cmd/nerd/cmd_init_scan.go` | Scanner |
| `cmd/nerd/cmd_instruction.go` | ApplyIncrementalResult |
| `cmd/nerd/cmd_campaign.go` | Scanner, HolographicProvider |
| `cmd/nerd/cmd_mangle_lsp.go` | `world/lsp` |
| `cmd/nerd/chat/helpers_scan.go` | Incremental, deep facts |
| `cmd/nerd/chat/session_boot.go` | Scanner |
| `cmd/nerd/chat/process_sync.go` | Apply incremental |
| `cmd/nerd/chat/process_dream_delegation.go` | Apply incremental |
| `cmd/nerd/chat/campaign.go` / `campaign_assault.go` | HolographicProvider |
| `cmd/nerd/chat/model_types.go` | Holds `*world.Scanner` |

### internal

| Path | Usage |
|------|-------|
| `internal/init/*` | Scanner during init/profile/knowledge |
| `internal/system/factory.go` | ScannerWithConfig in boot |
| `internal/system/holographic_code_scope.go` | FileScope + EnsureDeepFacts + Cartographer |
| `internal/campaign/intelligence_gatherer.go` | Scanner, holographic |
| `internal/campaign/edge_case_detector.go` | Scanner |
| `internal/campaign/intelligence_gathering_methods.go` | world APIs |
| `internal/shards/system/world_model.go` | ASTParser ingestor |
| `internal/shards/system/campaign_runner.go` | Scanner, holographic |

### core (indirect)

`internal/core/virtual_store.go` / `virtual_store_types.go` reference CodeScope interfaces implemented by world-backed bridges — **core does not import world**.

## Schema dependency

| Artifact | Relation |
|----------|----------|
| `internal/core/defaults/schemas_world.mg` | Decl for topology, symbols, LSP |
| Policy rules elsewhere in core corpus | Consume `file_topology`, `code_defines`, etc. |

## External process dependency

| Tool | Used by |
|------|---------|
| `git` | `ScanGitHistory` |

## Dependency risk notes

1. **`world → core` + holographic RealKernel** couples agent context to concrete kernel type (`*core.RealKernel`). Interface-narrowing would reduce coupling.
2. **tree-sitter CGO/native** cost: large binary / platform sensitivity for polyglot parse.
3. **store API** shape (`WorldFactInput`, `ReplaceWorldFactsForFile`) is a contract — version carefully.
