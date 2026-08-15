# tools — Safety and Invariants

> Last verified: **2026-07-13**

## Layered safety model

```
┌──────────────────────────────────────────┐
│ Mangle permitted / modular_tool_allowed  │  policy layer
├──────────────────────────────────────────┤
│ Config AllowedTools                      │  runtime allowlist
├──────────────────────────────────────────┤
│ session checkSafety + executive gate     │  pre/post interactive
├──────────────────────────────────────────┤
│ Tool local invariants                    │  this package
│  - workspace path containment (partial)  │
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
| Path inside workspace | `core` file ops | Enforced |
| Symlink escape rejection | `resolveWorkspacePath` | Enforced |
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
| glob/grep paths unconstrained | Read outside workspace |
| codedom line tools unconstrained | Read/write outside workspace |
| shell working_dir unconstrained | Command runs anywhere FS allows |
| bash fallback on Windows may re-parse loosely | Depends on run_command path |
| research_cache no ACL | Any caller can clear/read keys |
| browser tools share process manager | Session ID guessability / no auth |
| FilterByIntent open fallback | Soft layer only |

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
- tools package defaults are “if called, run.”  
- With `EnableSafetyGate=true` and nil kernel, session **fails closed**.  
- With empty AllowedTools, session **fails open** at allowlist layer — critical contract.

## Must-not rules for future edits

1. Do not add “always allow dangerous op” shortcuts inside tools when safety gate is off.  
2. Do not shell-out with unsanitized full command strings when argv form is possible.  
3. Do not log full secrets from env maps.  
4. Do not remove workspace_guard without replacement.  
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
