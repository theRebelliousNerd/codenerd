# 07 — Dependency Map: Autopoiesis

> Last verified against codebase: **2026-07-13**

## 1. Upstream (what autopoiesis imports)

| Package | Why |
|---------|-----|
| `codenerd/internal/types` | `KernelFact`, `KernelInterface`, `ToolInfo`, LLM tool types |
| `codenerd/internal/logging` | CategoryAutopoiesis timers/logs |
| `codenerd/internal/config` | Default tool-gen OS/arch, user config load |
| `codenerd/internal/core` | Embedded defaults (`go_safety.mg`, `schemas_state.mg`) |
| `codenerd/internal/mangle` | Local Ouroboros engine, DifferentialEngine, Fact |
| `codenerd/internal/mangle/transpiler` | Sanitizer for embedded Mangle in tools |
| `codenerd/internal/tools/research` | GroundingHelper / ThinkingHelper wrappers |
| `codenerd/internal/build` | Build helpers (Thunderdome/compiler) |
| `codenerd/internal/prompt` | (prompt_evolution) atom/manifest types |
| stdlib | `go/ast`, `go/parser`, `os/exec`, `sync`, etc. |
| `github.com/traefik/yaegi/*` | Interpreted execution path |
| `gopkg.in/yaml.v3` | prompt_evolution strategy/config |

**Avoided direct imports (by design):** `articulation` concrete types → `PromptAssembler` interface; full `core.Kernel` → `KernelInterface` / bridge.

## 2. Downstream (who imports autopoiesis)

| Consumer | Import use |
|----------|------------|
| `internal/system/factory.go` | Construct Orchestrator, SetKernel, VirtualStore tool hooks |
| `cmd/nerd/chat/*` | Orchestrator field, QuickAnalyze, tools, evolution, listener |
| `cmd/nerd/ui/autopoiesis_page.go` | `DetectedPattern`, `ToolLearning` display |
| `cmd/nerd/cmd_instruction.go` | `ProcessKernelDelegations` |
| `cmd/nerd/cmd_systems.go` | Status / systems copy |
| `internal/campaign/tool_pregenerator.go` | Tool generation for campaigns |
| `internal/campaign/intelligence_gatherer.go` | Capability/intelligence |
| `internal/verification/verifier.go` | Verification reuse |
| `tests/e2e/*` | Kernel contract / smoke |

Scratch scripts under `scratch/` may list file paths for merge analysis; not runtime deps.

## 3. Dependency direction diagram

```
perception/LLM clients
        │
        ▼
internal/autopoiesis  ──uses──►  mangle, core defaults, config, logging, types
        │
        ├── boot wired by ──► internal/system (Cortex)
        ├── driven by ──────► cmd/nerd/chat, cmd/nerd/ui
        ├── facts to ───────► core.Kernel via AutopoiesisBridge
        ├── tools to ───────► VirtualStore (generator/executor)
        └── used by ────────► campaign, verification
```

## 4. Cycle-prevention pattern

```
autopoiesis  --interface-->  PromptAssembler  <--implements--  articulation
autopoiesis  --alias------>  types.KernelInterface <--impl-- core.AutopoiesisBridge
```

Do **not** add `import articulation` or `import core` for kernel methods beyond embedded content helpers already used.

## 5. Policy / schema external files

| Asset | Location | Loaded by |
|-------|----------|-----------|
| `go_safety.mg` | core embedded defaults | `checker.go` init |
| `schemas_state.mg` | core embedded defaults | `NewOuroborosLoop` |
| (kernel init also lists both) | `internal/core/kernel_init.go` | session kernel separately |

## 6. Third-party risk notes

- **Yaegi:** version/stdlib symbol surface; keep whitelist tight.  
- **`go build` for tools:** requires local Go toolchain and network for `go mod tidy` unless fully offline modules — operational dependency, not a Go import.
