# tools — Dependency Map

> Last verified: **2026-07-13**

## Upstream (what tools imports)

### Root `internal/tools`

| Import | Use |
|--------|-----|
| stdlib `context`, `fmt`, `reflect`, `sort`, `sync`, `time` | Registry |
| `codenerd/internal/logging` | ToolsDebug |

### `core`

| Import | Use |
|--------|-----|
| stdlib `os`, `path/filepath`, `bufio`, `regexp`, … | FS/search |
| `codenerd/internal/logging` | VirtualStore* logs |
| `codenerd/internal/tools` | Tool types |

### `shell`

| Import | Use |
|--------|-----|
| `os/exec`, `runtime`, … | Process spawn |
| `github.com/kballard/go-shellquote` | Safe argv split |
| `codenerd/internal/logging`, `codenerd/internal/tools` | |

### `codedom`

| Import | Use |
|--------|-----|
| stdlib + `os/exec` | Elements, line edit, go test |
| `codenerd/internal/logging`, `codenerd/internal/tools` | |

### `research`

| Import | Use |
|--------|-----|
| `net/http`, `golang.org/x/net/html` | Fetch/search |
| `codenerd/internal/browser` | SessionManager |
| `codenerd/internal/config` | Context7 API key |
| `codenerd/internal/logging` | Researcher/Browser |
| `codenerd/internal/types` | LLM grounding/thinking interfaces |
| `codenerd/internal/tools` | Tool registration |

**No import of:** kernel, session, mangle, world (avoids cycles; DI for impact).

## Downstream (who imports tools)

| Consumer | Import path | Role |
|----------|-------------|------|
| `internal/core` | tools + all subpackages | modularTools, hydrate |
| `internal/session` | tools | Global().Execute |
| `internal/system` | via VirtualStore | boot hydrate |
| `internal/init` | tools, tools/research | knowledge / agents |
| `internal/campaign` | tools/research | research helpers |
| `internal/autopoiesis` | tools/research | grounding |
| `internal/world` | tools/codedom | implements analyzer side |
| `tests/e2e/*` | tools | register dummy tools |

### Grep command for refresh

```powershell
rg "codenerd/internal/tools" -g "*.go" --glob "!*_test.go"
```

## External Mangle (logical dependency)

| Artifact | Role |
|----------|------|
| `internal/mangle/intent_routing.mg` | modular_tool_allowed rules |
| `internal/core/defaults/schemas_tools.mg` | Decls for modular tools + tool_execution |

Tools package does not load these files; kernel does at boot.

## Dependency direction rules

```
session/core ──► tools ──► browser/config/types/logging
world ──implements──► codedom interfaces (not tools → world)
```

If a new tool needs kernel Query, add an interface in tools and inject from core — do not import core from tools.
