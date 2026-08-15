# TODO — Autopoiesis

> Last verified against codebase: **2026-08-15**
> Prioritized engineering backlog

## P0

- [x] Route all production tool creation through `ExecuteOuroborosLoop` (chat `generate_tool`, `ExecuteAction`).
      `Orchestrator.GenerateTool`, `executeToolGeneration` and `GenerateToolWithTracing` now run the full
      pipeline; `WriteAndRegisterTool` is the only unaudited seam and has no production callers.
      Enforced by `internal/autopoiesis/tool_creation_routing_test.go` (AST inventory + exemption list).
- [x] Fail closed when `go_safety.mg` fails to load (no empty policy).
      `loadGoSafetyPolicy` records the error, an empty policy counts as a failure, and `SafetyChecker.Check`
      returns a blocking `ViolationPolicy` before touching the code. `checker_failclosed_test.go`.
- [x] Audit default `AllowExec: true` — document or tighten for untrusted workspaces.
      **Decision: deny by default.** `DefaultOuroborosConfig` and the orchestrator-built config now set
      `AllowExec: false`; grant per workspace via `Config.AllowToolExec`. Rationale recorded at both sites.

## P1

- [x] Parity check post-boot: registry tool count vs `tool_registered` facts.
      `Orchestrator.VerifyKernelToolParity` runs automatically at the end of `syncExistingToolsToKernel` and
      logs an error on mismatch. `kernel_parity_test.go`.
- [x] Confirm `StartKernelListener` started on all interactive boot paths; document poll interval.
      Both paths (`session_boot.go`, `session_shared_boot.go`) start it at 2s, now named
      `autopoiesis.DefaultKernelPollInterval`. `kernel_listener_wiring_test.go` fails if a chat file wires an
      Orchestrator into a session without starting the listener, or uses a different cadence.
- [x] Expand e2e: scripted multi-stage Ouroboros (safety fail → regen → thunderdome survive).
      `ouroboros_multistage_e2e_test.go` drives the real state machine: unsafe proposal → policy rejection →
      regeneration with violation feedback → real PanicMaker attacks in a compiled arena → transition gate →
      compile (running the generated tests) → register → execute. Plus the inverse rejection case.
- [x] Campaign pregen always uses same safety depth as chat Ouroboros helpers.
      Verified: `internal/campaign/tool_pregenerator.go` calls `ouroboros.Execute`, the same entry point the
      chat helpers use. Pinned by `TestCampaignPregen_WhenGeneratingTools_ShouldRunFullOuroborosLoop` and by
      the safety-bearing-method inventory in the same file.

## P2

- [x] Unify Yaegi vs binary execution policy (config switch + docs).
      **Decision: compiled binaries are the product default** — the only mode with a process boundary, a
      scrubbed environment and a hard context kill. `OuroborosConfig.ExecutionMode` /
      `AllowCompilationFallback` is the single switch; the duplicate `ToolExecutionConfig` (which declared
      the opposite default and was read by nothing) is deleted. The interpreter now derives its import
      allowlist from the `SafetyChecker` and accepts the pipeline's own entry-point contract, which it
      previously could not. `execution_policy_test.go`.
- [x] Human-in-the-loop default for SPL auto-promote.
      `DefaultEvolverConfig().AutoPromote` is now false, matching `LearningCandidateAutoPromote` in
      `internal/shards/system/perception.go`. `GetPendingAtoms` + `PromoteAtom`/`RejectAtom` are the review
      surface. `prompt_evolution/promotion_gate_test.go`.
- [x] Agent spec → runtime scheduler ownership decision (shards vs autopoiesis).
      **Decision: shards own scheduling; autopoiesis authors.** Boot runs
      `system.SyncAgentRegistryFromDisk` → `DiscoverAgentsOnDisk` → `shardMgr.DefineProfile`. Discovery keys
      off `prompts.yaml`, which `writeAgentSpec` never wrote — so every autopoiesis-created agent was
      invisible to its own runtime. Fixed: the spec writer now emits `prompts.yaml`, with
      `SetAgentDefinitionWriter` as the seam for boot to install `system.WriteAgentDefinition` (a direct
      import would close a cycle). `agent_handoff_test.go`.
- [x] Export optional metrics (generation latency, reject rates).
      `Orchestrator.ExportMetrics()` → `AutopoiesisMetrics`: mean/max generation latency, reject rate,
      safety-violation rate, panic rate, Thunderdome kill/entry rates. Latency is recorded for rejected runs
      too. `metrics.go`, `metrics_test.go`.
- [x] Golden suite per `ViolationType`.
      `checker_failclosed_test.go` pins a provoking sample for every reachable type and an explicit reason
      for the two the checker cannot produce. Writing it surfaced that
      `ViolationUnsafePointer`/`Reflection`/`CGO`/`Exec` were never emitted by anything;
      `classifyImportViolation` now produces them so regeneration feedback can name the real hazard.

## P3 / hygiene

- [x] Refresh package `internal/autopoiesis/README.md` date/architecture version to match 2026 corpus.
- [x] Remove or redirect legacy architecture filenames if still present beside this corpus.
      Verified: all nine legacy files are already one-line redirects to the current documents.
- [ ] Reduce dual templates vs JIT prompt residual prose over time.
      Still open. `tool_templates.go` (592 lines) and the legacy prompt strings in `tool_generation.go`
      coexist with the JIT path; the JIT path is only taken when a `PromptAssembler` is attached and
      `JITReady()`. Collapsing them needs the JIT corpus to cover every Ouroboros stage first.

## Found and fixed while working through the above

- `RuntimeTool.Execute` declared the wrapper's `output` field as a Go `string`, but the generated wrapper
  emits it as `json.RawMessage` and passes valid JSON through verbatim. Any tool whose result parsed as
  JSON — a count, a bool, a JSON document — compiled, passed safety, survived the Thunderdome, registered,
  and then failed on its first call with "cannot unmarshal number into Go struct field .output".
- Target OS/arch defaulted to the literals `windows`/`amd64` (both in `DefaultConfig` via
  `config.DefaultToolGenerationConfig` and in `NewOuroborosLoop`'s fallback). Ouroboros compiles a tool and
  then executes the binary itself, so on any non-Windows host every generated tool cross-compiled to
  Windows and died with "exec format error". Now `runtime.GOOS`/`runtime.GOARCH`, with an explicit user
  setting still winning.
- Both compile sites passed `nil` for `*config.UserConfig`, so tools and arenas built without the
  operator's CGO flags, GOFLAGS and allowlisted env. Threaded through `Config` → `OuroborosConfig` →
  `ToolCompiler`/`ThunderdomeConfig`. The compiler's build-env *detection* root was the throwaway temp
  module rather than the workspace, which also broke tools importing `codenerd` via the replace directive.
