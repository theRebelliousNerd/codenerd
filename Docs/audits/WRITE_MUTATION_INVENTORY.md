# Inventory: Write-Mutation Tools in `internal/core` — Execution / Dispatch Path

**Generated:** 2026-08-08 (Researcher Shard, encyclopedic mode)  
**Scope:** `internal/core` only. Evidence cited is **already-observed** source; gaps flagged explicitly rather than inferred.  
**Sources read:** `virtual_store_types.go`, `virtual_store.go`, `virtual_store_actions.go`, `virtual_store_file_actions.go`, `virtual_store_codedom.go`, `virtual_store_tools.go`, `tool_registry.go`, `transaction_manager.go`, `kernel_types.go`

---

## 1. Summary

`internal/core` does **not** expose standalone binaries for writes. All write-mutations are **VirtualStore ActionHandlers** dispatched by `ActionType` (`virtual_store_types.go:9-152`) and executed via three distinct substrates:

1. **Raw filesystem** (`os.WriteFile` / `os.Remove` / `strings.Replace`) — `virtual_store_file_actions.go`
2. **Code DOM / FileEditor** (`tactile.FileEditor` interface, `types.FileEditor`) — `virtual_store_codedom.go` → `kernel_types.go:271-294`
3. **Tactile executor / shell** (`tactile.Executor` + `tactile.Command`) — `virtual_store_actions.go`
4. **Registered external tools** (`exec.CommandContext`) — `tool_registry.go:158-207`
5. **Transactional 2PC batch** (`TransactionManager` + `ShadowMode`) — `transaction_manager.go`

> **Uncertainty:** The top-level router (`VirtualStore.RouteAction` / `Dispatch` / `HandleAction`) was **not located** in the files read — `grep RouteAction` returned only bus comments in `virtual_store.go:117,135,391`. The dispatch switch on `ActionType` is inferred from handler naming (`handleWriteFile`, `handleEditElement`, etc.) and existing tests, but its exact location/signature was not observed. See §6.

---

## 2. Canonical Write-Mutation Inventory

### 2.1 Raw Filesystem Mutations — `virtual_store_file_actions.go`

| ActionType (const) | Handler | File + Symbol | Substrate | Facts Emitted | Gate / Validation |
|---|---|---|---|---|---|
| `ActionWriteFile = "write_file"` (`virtual_store_types.go:15`) + alias `ActionFSWrite = "fs_write"` (`:73`) | `handleWriteFile` | `virtual_store_file_actions.go:186` | `os.MkdirAll` + `os.WriteFile(path, []byte(content), 0644)`; `extractCodeBlockForFile` fence-stripping; `sha256` hash | `file_written(path, hash, sessionID, timestamp)`, `modified(path)` | `validator_file.go:23` (`CanValidate: ActionWriteFile, ActionFSWrite`), `validator_dir.go:22`, `validator_paranoid.go:41` via `ValidatorRegistry` (`virtual_store.go:287-296`) |
| `ActionEditFile = "edit_file"` (`virtual_store_types.go:16`) | `handleEditFile` | `virtual_store_file_actions.go:248` | `os.ReadFile` → `strings.Contains` → `strings.Replace(...,1)` → `os.WriteFile` | `file_edited(path)`, `modified(path)` on success; `edit_failed(path, "pattern_not_found")` on miss | `validator_file.go:147`, `validator_edit_enhanced.go:45`, `validator_paranoid.go:43` |
| `ActionDeleteFile = "delete_file"` (`virtual_store_types.go:17`) | `handleDeleteFile` | `virtual_store_file_actions.go:314` | `os.Remove(path)` — **requires** `payload["confirmed"] == true` else `delete_blocked(path, "no_confirmation")` | `file_deleted(path)` / `delete_blocked(path, ...)` | `validator_file.go:252` |

Evidence anchors:
- `handleWriteFile` extracts `payload["content"]` (must be `string`), resolves via `v.resolvePath(req.Target)`, truncates nowhere; error path asserts `file_write_error(path, err)` (`:225`).
- `handleEditFile` extracts `payload["old"]` + `payload["new"]` strings; single-occurrence replace only.
- `handleDeleteFile` hard gate: no `confirmed:true` → `Success:false`.

### 2.2 Code DOM / Semantic Line Mutations — `virtual_store_codedom.go`

All require `codeScope CodeScope` + `fileEditor FileEditor` (`virtual_store.go:76-78`, `kernel_types.go:203-294`). All refresh scope via `scope.RefreshWithRetry(3)` and replace kernel Code DOM facts via `v.clearCodeDOMFacts()` + `scope.ScopeFacts()`.

| ActionType | Handler | Substrate (`FileEditor` method) | Additional Guards | Facts |
|---|---|---|---|---|
| `ActionEditElement = "edit_element"` (`virtual_store_types.go:54`) | `handleEditElement` (`virtual_store_codedom.go:155`) | `editor.ReplaceElement(file, startLine, endLine, newContent)` (`kernel_types.go:293`) | `scope.GetCoreElement(ref)` existence; `scope.VerifyFileHash(file)` → on mismatch `RefreshWithRetry` then re-fetch; emits `element_edit_blocked(ref, "hash_verification_failed"/"concurrent_modification")`, `element_stale` | `element_modified(ref, sessionID, timestamp)`, `modified(file)` + `result.Facts` from editor + refreshed `ScopeFacts` |
| `ActionEditLines = "edit_lines"` (`virtual_store_types.go:57`) | `handleEditLines` (`:325`) | `editor.EditLines(path, startLine, endLine, newLines)` | `payload["start_line"/"end_line"]` as `float64`; `scope.IsInScope(path)` gate for refresh | `result.Facts` + refreshed `ScopeFacts` |
| `ActionInsertLines = "insert_lines"` (`:58`) | `handleInsertLines` (`:394`) | `editor.InsertLines(path, afterLine, newLines)` | `payload["after_line"] float64`, `payload["content"] string` (non-empty); `scope.IsInScope` refresh | same |
| `ActionDeleteLines = "delete_lines"` (`:59`) | `handleDeleteLines` (`:457`) | `editor.DeleteLines(path, startLine, endLine)` | same line-range payload | same |
| Scope lifecycle (non-mutating but state-changing) | `handleOpenFile` (`:22`), `handleRefreshScope` (`:264`), `handleCloseScope` (`:298`) | `scope.Open(path)` / `scope.Refresh()` / `scope.Close()` | `scope == nil` → error | `scope_open_failed`, `scope_closed`, full `ScopeFacts` replacement |

Validator: `validator_codedom.go:43` (`ActionEditElement || ActionEditLines || ActionInsertLines || ActionDeleteLines`)

### 2.3 Shell / Executor Mutations — `virtual_store_actions.go`

These are **arbitrary** write-capable; policy can use them to mutate files indirectly.

| ActionType | Handler | Substrate |
|---|---|---|
| `ActionExecCmd = "exec_cmd"` (`virtual_store_types.go:13`), `ActionRunCommand`, `ActionBash`, `ActionRunTests`, `ActionBuildProject`, `ActionGitOperation`, `ActionShowDiff` | `handleExecCmd` (`virtual_store_actions.go:81`) → legacy `v.executor.Execute` or modern `handleExecCmdModern` (`:194`) via `v.modernExecutor` | `tactile.Command{Binary:"bash", Arguments:["-c", req.Target], WorkingDirectory, Environment:getAllowedEnv(), Limits:{TimeoutMs}}` → `executor.Execute(ctx, cmd)` |
| Direct API (bypasses `ActionRequest`) | `VirtualStore.Exec` (`virtual_store_actions.go:20`) | Same tactile path, but `filterCallerEnv(env)` + `getAllowedEnv()` merged; `isBinaryAllowed(binary)` allowlist; `strings.Contains(cmd,"..")` traversal guard |

Facts: `cmd_succeeded(binary, output)` / `cmd_failed(binary, err)` (legacy) or audit-logger facts via `injectTactileFact` → `kernel.Assert` (`virtual_store.go:333-359`) in modern path. Success = `result.Success && result.ExitCode==0` (`:167,217`).

Gate: `isBinaryAllowed` allowlist (`virtual_store.go:71`) default `{bash,sh,pwsh,powershell,cmd,go,git,grep,ls,mkdir,cp,mv,npm,npx,node,python,python3,pip,cargo,rustc,make,cmake}` (`:155-160`).

### 2.4 Registered / Generated Tool Mutations — `tool_registry.go`

| Capability | API | Substrate |
|---|---|---|
| `ActionExecTool = "exec_tool"` (`virtual_store_types.go:48`) | `ToolRegistry.ExecuteTool(ctx, toolName, input)` (`tool_registry.go:159`) → `ExecuteRegisteredTool` (`:165`) | `exec.CommandContext(ctx, tool.Command, args...)` with `cmd.Dir = workDir`; `secureValidateCommand` + `secureValidateArgs` pre-check; `tool.ExecuteCount++`; kernel facts: `registered_tool(name,command,affinity)`, `tool_registered`, `tool_hash`, `tool_capability` via `collectToolFacts` (`:253-280`) + `injectToolFacts` → `kernel.AssertBatch` |

Hydration paths (`virtual_store_tools.go`):
- `HydrateModularTools()` (`:45`) → `core.RegisterAll`, `shell.RegisterAll`, `codedom.RegisterAll`, `research.RegisterAll` to both `v.modularTools` (`tools.Registry`) and `tools.Global()` — registers filesystem/shell/codedom/research tools for any shard.
- `HydrateToolsFromDisk(nerdDir)` (`:140`) → `ToolRegistry.RestoreFromDisk(.nerd/tools/.compiled)` + `SyncFromOuroboros(toolExecutor)`
- `HydrateStaticTools(defs)` (`:184`) → `RestoreFromStaticDefs` (from `available_tools.json`)

### 2.5 Transactional Batch Mutations (2PC + Shadow Validation) — `transaction_manager.go`

Not a standalone `ActionType` but the **atomic** path for multi-file edits used by higher layers and the `ShadowMode` simulation.

```
Begin(ctx, description)          // txns[txnID]={Status:pending, Snapshots:{}}
  └─ creates txnID = fmt.Sprintf("txn_%d", time.Now().UnixNano())

AddEdit(ctx, FileEdit{FilePath, OldHash, NewHash, Content, EditType})  // :138
  ├─ snapshots original via os.ReadFile (unless EditTypeCreate)
  ├─ kernel.Assert(pending_mutation(mutationID, filePath, oldContent[:200], newContent[:200]))
  └─ txn.Edits = append

Prepare(ctx) → *ShadowValidationResult // Phase 1  :200
  ├─ txn.Status = preparing
  ├─ shadowMode.StartSimulation(ctx, description)   // forked RealKernel
  ├─ per-edit: hash conflict check (OldHash vs currentHash), then
  │            shadowMode.SimulateAction(SimulatedAction{Type:ActionTypeFileWrite})
  ├─ query shadow kernel: deny_edit(Ref,Reason) → SafetyBlock
  ├─ shadowMode.AbortSimulation("validation complete")
  └─ txn.Status = Ready | Aborted; ValidDuration recorded

Commit(ctx) error               // Phase 2  :337
  ├─ requires txn.Status == Ready
  ├─ per-edit: MkdirAll + WriteFile or Remove; on any error → rollback() restores Snapshots + Abort
  ├─ txn.Status = Committed; activeTxnID = ""
  └─ kernel.Assert(file_written(path, newHash, txnID, timestamp)) per non-delete edit

Abort(ctx, reason)              // :410  — sets Aborted, clears activeTxnID
IsTransactionActive() / GetActiveTransaction() / GetTransaction(id)
```

Types: `TransactionStatus` = `pending|preparing|ready|committing|committed|aborted` (`:45-52`); `EditType` = `modify|create|delete` (`:67-71`).

---

## 3. Exact Execution / Dispatch Path (as observed)

### 3.1 Kernel → VirtualStore

1. **Policy derives `next_action`** from Mangle rules (observed samples: `defaults/policy/capabilities.mg:18-24` → `next_action(/write_file)`, `next_action(/edit_element)` etc.; `defaults/policy/autopoiesis.mg:30` etc.). `FactCategoryAction = "action"` includes `next_action, permitted` (`README.md:255`).
2. **CortexKernel / RealKernel** evaluates IDB; `next_action` facts are routed to the `VirtualStore` FFI (`kernel_types.go:80-84` RealKernel struct). Direction: `RealKernel` holds `virtualStore *VirtualStore` (`kernel_types.go:80`), and `VirtualStore` holds `kernel Kernel` (`virtual_store.go:65`). `SetKernel` wires both (`virtual_store.go:418-446`).
3. **Dispatch to handler** — **not observed verbatim** (see uncertainty §6). By convention every `ActionType` maps 1:1 to `VirtualStore.handle*` via a switch on `req.Type` (`handleWriteFile`, `handleEditFile`, `handleDeleteFile`, `handleEditElement`, `handleEditLines`, `handleInsertLines`, `handleDeleteLines`, `handleExecCmd`, `handleExecCmdModern`, `ExecuteTool`). Evidence: handler signatures are uniform `func (v *VirtualStore) handleX(ctx context.Context, req ActionRequest) (ActionResult,error)`.
4. **Pre-execution gates** (`VirtualStore`):
   - `bootGuardActive` (`virtual_store.go:109,389-406`) — all `RouteAction` blocked until `DisableBootGuard()` (first user message).
   - Constitutional `permittedCache` (`:99-100,430`) + `constitution []ConstitutionalRule` (`:63`).
   - `isBinaryAllowed` + `secureValidateCommand/Args` for exec/tool paths.
5. **Substrate execution** (see §2 tables): `os.*` / `FileEditor.*` / `tactile.Executor.Execute` / `exec.CommandContext`.
6. **Post-execution kernel feedback** — every handler returns `ActionResult{FactsToAdd: []Fact}` which the dispatcher asserts to the kernel (observed pattern: handlers build `FactsToAdd`; `VirtualStore.processValidationResults` asserts `ValidationResult.ToFacts()`; `injectTactileFact` asserts tactile facts). Concrete audit: `handleWriteFile → file_written+modified`; `handleEditLines → result.Facts + ScopeFacts`; `TransactionManager.Commit → file_written`.
7. **Post-action validation** — `ValidatorRegistry` (`virtual_store.go:112-113,284-296`) initialized via `RegisterAllValidators` (`validator_registry.go`). Each `ActionType` has typed validators:
   - `validator_file.go:23/147/252` (write/edit/delete)
   - `validator_dir.go:22` (write/list)
   - `validator_codedom.go:43` (edit_element/lines)
   - `validator_edit_enhanced.go:45` (edit_file)
   - `validator_paranoid.go:41` (write/edit)
   - Results injected via `processValidationResults` (`virtual_store.go:298-330`) → `vr.ToFacts()` → `kernel.Assert` → `action_verified` / `action_validation_failed` + `action_weakly_validated` etc. (seen in `predicate_corpus.db`).

### 3.2 Tactile Executor Instantiation

`VirtualStore.initModernExecutor()` (`virtual_store.go:256-282`):
```
DefaultExecutorConfig{DefaultWorkingDir, AllowedEnvironment}
→ CompositeExecutorWithConfig
→ AuditLogger{FactCallback: injectTactileFact}
→ Composite.SetAuditCallback(auditLogger.Log)
→ v.modernExecutor = composite; v.useModernExecutor = true
```
Selection at dispatch: `v.mu.RLock(); useModern = v.useModernExecutor && v.modernExecutor != nil` (`virtual_store_actions.go:55-60,126-133`).

### 3.3 Tool Registry Instantiation

`NewVirtualStoreWithConfig` (`virtual_store.go:171-205`) creates:
```
toolRegistry = NewToolRegistry(config.WorkingDir)   // map[string]*Tool + Kernel ref
modularTools = tools.NewRegistry()
```
`SetKernel(k)` propagates `toolRegistry.SetKernel(k)` (`virtual_store.go:442-445`). Facts injected via `collectToolFacts` (`tool_registry.go:253`) → `kernel.AssertBatch` (single Mangle fixpoint).

---

## 4. Write-Tool Surface Summary (count)

| Subsystem | Mutating ActionTypes | Handler Count |
|---|---|---|
| Raw filesystem | `write_file`, `fs_write`, `edit_file`, `delete_file` | 3 handlers |
| Code DOM | `edit_element`, `edit_lines`, `insert_lines`, `delete_lines` | 4 handlers |
| Exec (indirect writes) | `exec_cmd`, `run_command`, `bash`, `run_tests`, `build_project`, `git_operation`, `show_diff`, `exec_tool` | 2 handlers (`handleExecCmd`, `ExecuteTool`) |
| Transactional batch | (no ActionType, uses `pending_mutation` + `file_written`) | `TransactionManager` (Begin/AddEdit/Prepare/Commit) |
| **Total distinct write-capable ActionTypes in `internal/core`** | **12+ aliases** (see `virtual_store_types.go:12-152` full list) | **9 direct write handlers + 2 exec paths + TM** |

All other constants in `virtual_store_types.go` (`read_file`, `list_files`, `glob`, `grep`, `search_code`, `get_elements`, `get_element`, `open_file`, `refresh_scope`, `close_scope`, `delegate_*`, campaign/*, python/*, research/*) are **read or delegation**, not direct filesystem mutators.

---

## 5. Policy & Safety Hooks

- **Constitutional deny:** `defaults/policy/constitution.mg:198-453` defines `dangerous_action(ActionType) :- requires_permission(...)`, `has_active_override`, `appeal_*` predicates. Not fully read; cited for location.
- **ShadowMode / deny_edit:** `TransactionManager.Prepare` queries `deny_edit` in shadow kernel (`transaction_manager.go:298-314`). `ShadowMode` (`shadow_mode.go`, not read) forks kernel for simulation.
- **Validator Registry:** `virtual_store.go:287-296` + `validator_registry.go` (not read) centralizes `CanValidate(ActionType) bool` dispatch. Prior work: `validator_file.go`, `validator_codedom.go`, etc.

---

## 6. Gaps & Uncertainty (must verify before acting)

1. **Router location/signature** — `grep "RouteAction|route|Handle.*Action"` returned no handler switch in scanned files. **Did not observe** the `func (v *VirtualStore) RouteAction(...)` or `Dispatch` implementation; cannot cite its file/line. The dispatch path in §3.1 step 3 is reconstructed from handler signatures and `GlassBox` comments (`virtual_store.go:117`) and should be confirmed by reading `virtual_store.go` tail / `virtual_store_routing.go` / `dreamer.go:438,461`.
2. **Full ActionType → handler table** — inferred from `virtual_store_types.go` constants and `handle*` names; the authoritative `switch req.Type` block was not read. Aliases (`fs_write`, `search_files`, `analyze_code`, `delegate_*`) may map differently.
3. **Modular tools' write set** — `internal/tools/core`, `shell`, `codedom`, `research` registries were not enumerated; `HydrateModularTools` proves they exist (`virtual_store_tools.go:58-97`) but individual tool names/capabilities not listed here.
4. **Mangle `next_action` derivation** — only sampled `capabilities.mg`/`autopoiesis.mg`; full derivation graph (`schemas_*.mg`) not traced.
5. **Executor security allowlists** — `isBinaryAllowed`, `secureValidateCommand/Args`, `filterCallerEnv`, `getAllowedEnv` implementations not read; cited only by call-site.

**Recommended next read (budget allowing):**
- `internal/core/virtual_store.go` (tail, routing switch) + `virtual_store_routing.go` + `virtual_store_constitution.go`
- `internal/core/validator_registry.go` + each `validator_*.go`
- `internal/tools/core/*.go`, `internal/tools/shell/*.go`, `internal/tools/codedom/*.go` to enumerate modular write tools
- `internal/core/shadow_mode.go` + `dreamer.go` for shadow/2PC integration

---

## 7. How to Use This Inventory

- To add a new write-mutation: add `ActionType` const in `virtual_store_types.go`, add `handle*` in `virtual_store_{file_actions|codedom|actions}.go`, register validator in `validator_registry.go`, and if batched, go through `TransactionManager.AddEdit` + `Prepare` + `Commit`.
- To audit blast radius: grep `FactsToAdd` predicates (`file_written`, `modified`, `element_modified`, `cmd_succeeded`) — these are the kernel signals downstream policy reacts to.
- To test: `virtual_store_*_test.go`, `transaction_manager_test.go`, `tool_registry_test.go`, `validator_*_test.go` cover each path.

