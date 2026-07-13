# Workflow: Workspace Isolation (`-w` / CODENERD_WORKSPACE_ROOT)

## What It Stresses

- `nerd -w <workspace>` flag propagation through Cortex boot
- Modular file tools (`write_file`, `read_file`, `edit_file`) path containment
- VirtualStore `WorkingDir` vs process CWD alignment
- Env contract: `CODENERD_WORKSPACE_ROOT` set by `resolveWorkspaceRoot`

## Why This Exists (2026-07 regression)

`nerd -w <app> create …` booted Cortex against `<app>` but modular tools wrote under the monorepo CWD. VirtualStore logs showed the correct `workingDir` while `write_file completed:` paths pointed at the parent repo.

Root cause: tools call `workspaceRoot()` which prefers `CODENERD_WORKSPACE_ROOT` then `os.Getwd()` — boot never set the env. Fixed in `internal/system/factory.go` (`resolveWorkspaceRoot` sets the env).

## Severity Levels

| Level | Action |
|-------|--------|
| **Conservative** | Empty temp workspace + create one file; assert path under `-w` only |
| **Aggressive** | Polyglot multi-file create under nested `.nerd/live_feature_matrix/polystack` |
| **Chaos** | Concurrent creates into two different `-w` workspaces (serial recommended — SQLite) |
| **Hybrid** | Create + scan + spawn tester in same `-w` app tree |

## Conservative Procedure

```powershell
$APP = Join-Path $env:TEMP "nerd-ws-isolation-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $APP | Out-Null
# copy or init minimal .nerd/config.json with working LLM
nerd create "Write hello.txt with content hello-workspace" -w $APP --timeout 15m

# MUST pass:
Test-Path (Join-Path $APP "hello.txt")          # true
Test-Path (Join-Path $PWD "hello.txt")          # false (no monorepo leak)

# Log proof:
Select-String -Path "$APP\.nerd\logs\*virtual_store*" -Pattern "write_file completed"
# every path must start with $APP
```

## Pass Criteria

- [ ] Every `write_file completed:` path is under the `-w` root
- [ ] No new project files appear in monorepo root
- [ ] `CODENERD_WORKSPACE_ROOT` equals abs path of `-w` during tool execution
- [ ] No `debug_program_ERROR.mg`

## Related Surfaces

- `nerd create`, `nerd spawn`, `nerd fix`, `nerd run`
- `internal/tools/core/workspace_guard.go`
- `internal/system/factory.go` → `resolveWorkspaceRoot`
