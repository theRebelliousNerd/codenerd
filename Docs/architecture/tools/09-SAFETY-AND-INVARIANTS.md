# tools — Safety and Invariants

> Last verified: **2026-08-15**

## Layered safety model

```
┌──────────────────────────────────────────┐
│ Mangle permitted / modular_tool_allowed  │  policy layer
├──────────────────────────────────────────┤
│ Config AllowedTools                      │  runtime allowlist
├──────────────────────────────────────────┤
│ session checkSafety + executive gate     │  pre/post interactive
├──────────────────────────────────────────┤
│ Registry allowlist (SetAllowlist)        │  capability envelope
├──────────────────────────────────────────┤
│ Tool local invariants                    │  this package
│  - workspace path containment (all path  │
│    and working-dir arguments)            │
│  - timeouts / output caps                │
│  - shellquote (no shell -c for run_cmd)  │
│  - delete_file not for dirs              │
└──────────────────────────────────────────┘
```

Tools alone are **not** sufficient for constitutional safety.

## Local invariants (enforced in tools)

| Invariant | Where | Status |
|-----------|-------|--------|
| Non-empty Name + non-nil Execute | `Tool.Validate` | Enforced |
| Required args present | `validateArgs` | Enforced |
| Coarse arg types | `valueMatchesSchemaType` | Best-effort |
| Path inside workspace | `core` file ops, `core` search, all `codedom` path args | Enforced |
| Shell/git working_dir inside workspace | `shell.resolveWorkingDir` | Enforced |
| git pathspec inside workspace | `git_diff`, `git_log` | Enforced |
| Symlink escape rejection | `tools.ResolveWorkspacePath`; directory walks skip symlinks | Enforced |
| Separator normalization before the gate | `tools.normalizeSeparators` | Enforced |
| Boundary-aware root prefix (`/ws-evil` is not inside `/ws`) | `tools.containedIn` | Enforced |
| Tool outside capability envelope refused | `Registry.SetAllowlist` | Enforced |
| No directory delete via delete_file | `executeDeleteFile` | Enforced |
| run_command argv split not via shell | shellquote | Enforced |
| Command timeout | WithTimeout | Enforced |
| Output truncation shell 50k | executeRunCommand | Enforced |
| web_fetch body 2MB / length cap | web_fetch | Enforced |
| web_search max_results ≤ 30 | web_search | Enforced |
| Gemini URL context ≤ 20 | GroundingHelper | Enforced |
| git_operation whitelist | switch on operation | Enforced |
| commit requires message | git_operation | Enforced |

## Local invariants (missing / weak)

| Gap | Risk |
|-----|------|
| bash fallback on Windows may re-parse loosely | Depends on run_command path |
| research_cache no ACL | Any caller can clear/read keys |
| browser tools share process manager | Session ID guessability / no auth |
| FilterByIntent open fallback | Soft layer only. Narrowed: the fallback is intersected with the enforced allowlist, so it can never return more than the envelope permits |
| Registry allowlist not yet wired by session | The mechanism exists and is tested; `internal/session` must call `SetAllowlist` for it to bind |

## Concurrency invariants

1. Registry map mutations under mutex.  
2. Execute must not deadlock if a tool tries to Register (would need write lock while potentially holding nothing — OK today).  
3. ResearchCache Get/Set locked.  
4. Browser manager Once-init; concurrent Start should be safe in SessionManager (owned by browser package).

## Mangle surface (relevant Decls)

From `schemas_tools.mg` (not owned by tools package):

- `modular_tool_allowed(ToolName, Intent)`  
- `modular_tool_priority(ToolName, Priority)`  
- `tool_execution(ToolName, Success, Timestamp)`  
- related relevance/scoring predicates for shard tools  

Session safety asserts:

- `pending_action(ActionID, ActionType, Target, Payload, Timestamp)`  
- queries `permitted`  

Payload size cap **100 KB** before assert (session).

## Constitutional interaction

- Default deny is a **policy** property.  
- tools package defaults are "if called, run" **unless an allowlist is enforced**.  
- With `EnableSafetyGate=true` and nil kernel, session **fails closed**.  

### The empty-AllowedTools contract

`session.Executor.isToolAllowed` fails closed: `cfg == nil || len(cfg.AllowedTools) == 0` denies. The registry underneath it now states the same rule explicitly, because the registry is reachable process-globally through `tools.Execute`, and any caller that skips the session gate previously got the entire catalog.

`Registry.SetAllowlist(*Allowlist)` takes:

| `Enforced` | `Names` | Meaning |
|-----------|---------|---------|
| `false` | anything | No envelope configured. Unconstrained — the CLI / developer case. |
| `true` | non-empty | Only those tools may execute. |
| `true` | **empty** | The agent was granted **no** capability. **Deny everything.** |

The third row is the contract. An *absent* capability envelope is not a grant of every capability, and keeping `Enforced` as its own field means "not configured" and "configured empty" can never collapse into each other. `FilterByIntent`'s open fallback for a missing or hallucinated intent is intersected with the envelope, so a bad intent can widen the *intent* but never the *capability*.

Not yet bound: `internal/session` and `internal/core` must call `SetAllowlist` when the effective config changes. Until then this is a mechanism with no policy attached, and the session-layer gate remains the only enforcement point.

## Must-not rules for future edits

1. Do not add “always allow dangerous op” shortcuts inside tools when safety gate is off.  
2. Do not shell-out with unsanitized full command strings when argv form is possible.  
3. Do not log full secrets from env maps.  
4. Do not remove workspace_guard without replacement.  
   4b. Do not normalize a path for a security decision with `filepath.ToSlash`
   or `filepath.Clean` alone. Both are separator-*aware*: off Windows they treat
   a backslash as an ordinary filename character, so `..\..\etc\passwd` and
   `.nerd\config.json` survive them unchanged and walk straight through the
   gate. Rewrite separators unconditionally with `strings.ReplaceAll`, resolve
   to absolute, then compare with a trailing-separator-aware prefix check so
   `/ws-evil` does not read as inside `/ws`. Not hypothetical: this is how
   `.nerd\config.json` passed the nerd.md write-protection gate on Linux.  
5. Do not register tools whose Name conflicts with Ouroboros tools without intentional override policy.

## Exemption from the `internal/build` adoption mandate

`internal/build` is the repo's single source of truth for the environment handed
to `go build` / `go test` subprocesses, and
`internal/build/go_invocation_inventory_test.go` fails when a new `go`
invocation appears that neither uses it nor carries a written exemption.

**`internal/tools/shell` is exempt.** It executes arbitrary operator- and
agent-supplied commands, not the Go toolchain; its environment assembly is an
allowlist decision governed by the execution policy above (rule 3: do not log
full secrets from env maps). Narrowing it to a Go-toolchain env would break
every non-Go command, and widening the Go env into it would leak toolchain
paths into unrelated processes.

**`internal/tools/codedom/run_impacted_tests.go` is *not* exempt** — it is
recorded as `pending adoption` in `goSpawnExemptions`. It spawns `go test` in
the user's project root with no `cmd.Env`, so a project needing CGO headers
fails there with a compile error reported as a test failure. It should call
`build.GetBuildEnvForTest(userCfg, projectRoot)` and build its argv with
`build.AppendGoFlags`.

See `Docs/architecture/build/08-WIRING-AND-INTEGRATION.md` §7.
