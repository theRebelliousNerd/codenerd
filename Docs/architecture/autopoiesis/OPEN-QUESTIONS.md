# OPEN QUESTIONS — Autopoiesis

> Last verified against codebase: **2026-08-15**
> Only questions that remain real after reading the code

## Q3 — How much internal Ouroboros Mangle state should surface?

Parent kernel gets durable outcomes. Should halt reasons, `battle_hardened`, or iteration counts be
queryable session-wide for glass-box UX?

**Impact:** the loop's internal engine holds `panic_maker_verdict`, `battle_hardened`,
`thunderdome_result`, `retry_attempt` and `error_history`; the parent kernel receives only
`tool_registered` / `tool_hash` / `tool_capability` / `tool_hot_loaded`. A tool that barely survived the
arena is indistinguishable from one that sailed through, at the level where routing decisions are made.

## Q7 — Campaign coupling

Complexity analysis recommends campaigns but does not start them. Should autopoiesis assert a kernel fact
(`needs_campaign`) instead of returning Go-only actions for chat to interpret?

## Q8 — Tool identity and capability naming

`normalizeCapabilityName(tool.Name)` equates name and capability. Will multi-capability tools need richer
`tool_capability` fan-out as first-class design?

## Q9 — Offline / air-gapped compile

`go mod tidy` during compile assumes module access. What is the supported offline story for generated tools
with non-stdlib imports? (Partial answer since 2026-08-15: `ExecutionMode = ExecuteInterpreted` runs tools
with no toolchain at all, but only stdlib-only tools, and with weaker isolation.)

## Q10 — Cross-repo (Vectryx) memory

Should successful tool schemas or learnings eventually consolidate into Vectryx, or remain workspace-local
by north-star scope discipline?

## Q11 — Dangerous-call rules in `go_safety.mg`

`ViolationDangerousCall` exists and `astFactEmitter.handleAssignment` already tracks aliases to
`os.RemoveAll` / `os.Remove` / `unsafe.Pointer`, but the policy has no rule that consumes them: under
`AllowFileSystem` (on by default) a generated tool calling `os.RemoveAll` produces no violation at all. The
import allowlist is currently the entire call-level story. Adding the rule means deciding what a
filesystem-permitted tool may legitimately do — and note that a negated literal containing `_` filters
nothing in this Mangle fork (see `internal/core/bound_negation_test.go`), so any new rule must project into
a bound helper first.

---

## Resolved

### Q1 — Should light generation paths remain at all? *(resolved 2026-08-15 — no)*

They were accidental dual maintenance, not a deliberate latency path. `Orchestrator.GenerateTool`,
`ExecuteAction` and `GenerateToolWithTracing` all run the full Ouroboros pipeline;
`tool_creation_routing_test.go` fails the build if a new bypass appears.

### Q2 / Q6 — Long-term sandbox, and the `AllowExec` default *(resolved 2026-08-15)*

Compiled binary + `go_safety.mg` + Thunderdome is the product sandbox: it is the only mode with a process
boundary, a scrubbed environment (`toolExecutionEnv`) and a hard context kill. Yaegi is a supported
alternate for hosts without a Go toolchain, selected by `OuroborosConfig.ExecutionMode`, and now shares the
SafetyChecker's import allowlist instead of maintaining a second one.

`AllowExec` defaults to **false**. `go_safety.mg` gates imports and nothing else, so an allowlisted
`os/exec` is an unrestricted shell running with the user's workspace as its working directory — a strictly
larger capability than anything else autopoiesis grants. Grant it per workspace with
`Config.AllowToolExec`. (Whether exec is worth granting at all is now Q11's problem: without call-level
rules the grant is all-or-nothing.)

### Q4 — Who schedules persistent agents? *(resolved 2026-08-15 — shards)*

Chat boot runs `system.SyncAgentRegistryFromDisk` → `DiscoverAgentsOnDisk` → `shardMgr.DefineProfile`, so a
discovered agent directory becomes a spawnable shard profile. Autopoiesis authors specs and does not grow a
scheduler. The handoff artifact is `prompts.yaml` (what discovery keys off); `writeAgentSpec` now emits it,
with `SetAgentDefinitionWriter` as the seam for boot to install `system.WriteAgentDefinition`.

### Q5 — SPL promotion authority? *(resolved 2026-08-15 — human first)*

`AutoPromote` now defaults to false. A promoted atom edits the agent's own system prompt for every
subsequent shard invocation, and `ShouldPromote` can be satisfied by three uses. The pending queue plus
`PromoteAtom`/`RejectAtom` is the review surface. Deriving `permitted(promote_atom, …)` in Mangle remains
the more ambitious answer and is not blocked by this change — the gate is one boolean at one call site.

---

When a question is decided, move the decision into IMPLEMENTED_SPEC / principles and record it here.
