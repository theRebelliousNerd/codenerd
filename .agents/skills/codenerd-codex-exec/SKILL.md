---
name: codenerd-codex-exec
description: Develop, debug, or validate codeNERD's Codex exec backend. This skill should be used when working on codex-cli engine wiring, repo-skill injection, noninteractive auth probes, schema-constrained output, Codex-specific concurrency limits, or rate-limit troubleshooting inside codeNERD.
---

# codeNERD Codex Exec

Use this skill when Codex is being used as the backend model for `codeNERD` through `codex exec`.

## Purpose

Keep Codex in the correct role:

- `codeNERD` is the agent and orchestrator
- Codex is a stateless backend model
- JIT context and Piggyback remain the governing contract
- noninteractive `codex exec` is the supported transport

## Working rules

Treat Codex as a backend, not as an autonomous editor or planner.

Preserve explicit repo-skill injection. Do not rely on implicit skill matching for engine-critical behavior.

Preserve schema-first behavior. When the caller provides a JSON schema, return only schema-valid JSON and keep Piggyback fields faithful.

Preserve `codeNERD` ownership of tools, approvals, routing, and execution. Do not move those responsibilities into Codex skills.

Treat concurrency as an operational control. Start with the configured Codex-specific ceiling, keep it under the global scheduler ceiling, and lower it when subscription rate limits appear.

Prefer wiring gaps to removals. If a Codex config field, probe, or scheduler hook exists but is not consumed, wire it up instead of deleting it.

## Workflow

1. Read [references/wiring-map.md](references/wiring-map.md) to find the real engine/config/auth/scheduler touchpoints before editing.
2. Read [references/schema-jit-piggyback.md](references/schema-jit-piggyback.md) before changing prompt assembly, skill invocation, or schema handling.
3. Read [references/auth-and-probes.md](references/auth-and-probes.md) before changing `nerd auth codex`, status reporting, or raw `codex exec` probes.
4. Read [references/concurrency-and-rate-limits.md](references/concurrency-and-rate-limits.md) before changing Codex-specific concurrency defaults or rate-limit handling.
5. Use [scripts/codex_exec_probe.py](scripts/codex_exec_probe.py) for deterministic local checks:
   - `auth` for noninteractive readiness
   - `skill` for repo-skill discovery
   - `schema` for schema round-trip validation
   - `concurrency --parallelism N` for bounded smoke tests
6. Update the Go runtime first, then verify the user-facing setup surfaces, then update docs/tests so the skill and the code stay in sync.

## Verification expectations

Use `nerd auth codex` to validate install, login/subscription, skill discovery, and schema-constrained exec readiness.

Use `nerd auth status` to inspect current Codex model, sandbox, repo skill settings, effective concurrency, and current readiness probe result.

Use `python .agents/skills/codenerd-codex-exec/scripts/codex_exec_probe.py auth --use-skill` when debugging raw `codex exec` behavior outside the Go auth flow.

When changing runtime behavior, add or update focused tests in `internal/config` and `internal/perception` before relying on wider package test runs.

If the wider CLI package fails to compile because of unrelated subsystems, separate those failures from Codex-exec validation and report them explicitly.
