# tools — Current State

> Last verified: **2026-07-13**  
> Inventory of what exists on disk under `internal/tools/`

## Package tree

```
internal/tools/
├── types.go
├── registry.go
├── errors.go
├── README.md
├── registry_test.go
├── registry_extra_test.go
├── registry_boundary_test.go
├── core/
│   ├── doc.go
│   ├── register.go
│   ├── file_ops.go
│   ├── search.go
│   ├── workspace_guard.go
│   └── *_test.go
├── shell/
│   ├── doc.go
│   ├── register.go
│   ├── execute.go
│   └── *_test.go
├── codedom/
│   ├── doc.go
│   ├── register.go
│   ├── elements.go
│   ├── lines.go
│   ├── run_impacted_tests.go
│   └── *_test.go
└── research/
    ├── doc.go
    ├── register.go
    ├── context7.go
    ├── web_search.go
    ├── web_fetch.go
    ├── browser.go
    ├── cache.go
    ├── grounding.go
    ├── thinking.go
    └── *_test.go
```

## Counts

| Kind | Count |
|------|------:|
| Non-test Go sources | 25 |
| Test Go files | 21 |
| Local Mangle | 0 |
| Subpackages | 4 |

## Hotspots (edit risk)

| Hotspot | Why |
|---------|-----|
| `shell/execute.go` | Largest; OS/exec + Windows bash + git |
| `core/file_ops.go` + `workspace_guard.go` | Safety-critical path resolution |
| `registry.go` | Shared concurrency + arg validation |
| `codedom/run_impacted_tests.go` | DI + kernel query + go test spawn |
| `research/browser.go` | Shared browser process lifecycle |
| `core/search.go` | Unbounded walk if called outside workspace |

## Behavioral maturity by subsystem

| Subsystem | Maturity | Notes |
|-----------|----------|-------|
| Registry core | High | Well tested boundaries |
| File ops + guard | High | Containment present |
| Search tools | Medium | Functional; weaker safety |
| Shell/git | Medium-High | Mockable; real OS variance |
| CodeDOM elements | Medium | Regex approximation |
| Line edit tools | Medium | Correct line math; no guard |
| Impact tests | Medium | Depends on provider wiring |
| Context7 / web | Medium | Network-dependent |
| Browser tools | Medium | Depends on Rod/Chrome |
| Cache tools | High (simple) | In-memory only |
| Grounding/Thinking | Medium | Provider-interface gated |

## Process globals present today

1. `tools.globalRegistry`  
2. `codedom.globalTestProvider`  
3. `research.defaultCache`  
4. `research.browserMgr` (+ Once)

## Related code outside package (not owned here)

- `internal/core/virtual_store_tools.go` — hydrate  
- `internal/session/executor_tools.go` — execute + safety  
- `internal/mangle/intent_routing.mg` — allow rules  
- `internal/jit/config` — AllowedTools on EffectiveAgentRuntimeConfig  
- `internal/system/factory.go` — boot call site  

## Honest “is it implemented?”

**Yes.** This is not a stub package. Full hydrate path is live; e2e suites exercise allowlists and modular execution. Gaps are safety parity and catalog sync, not missing registry.
