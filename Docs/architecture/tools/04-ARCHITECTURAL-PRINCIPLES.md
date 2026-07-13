# tools — Architectural Principles

> Last verified: **2026-07-13**  
> Binding principles for work inside `internal/tools/**`.

## P1 — Tools are effects, not policy

A tool must not decide constitutional permission. It may enforce **local physical invariants** (path containment, timeouts, size caps, refuse directory delete). Global “may this agent do this?” belongs to Mangle `permitted` / ConfigFactory / session gates.

## P2 — Name is the contract

`Tool.Name` is the stable ID shared by:

- LLM tool_call names  
- `AllowedTools` entries  
- Mangle atoms (`/name` form at safety boundary)  
- logs and metrics  

Renames require multi-surface updates; treat as breaking.

## P3 — Schema-required before Execute

`Registry.Execute` validates required args and coarse types. Tool bodies may re-check and coerce (JSON float64). Prefer central coerce helpers over silent zero values.

## P4 — RegisterAll is idempotent

`if registry.Has(name) { continue }` is required so hydrate can run against Global that already has tools. Never assume empty registry.

## P5 — Prefer pure handlers + injectable OS

Shell uses `execCommandContext` vars for tests. Network tools should accept context deadlines. Avoid untestable package-level HTTP clients when adding tools (DefaultClient is current pragmatism).

## P6 — Contain every filesystem root

Any tool that takes `path` / `base_path` / `working_dir` must resolve against workspace root (or document an explicit escape hatch and who authorizes it). Today this is incomplete — treat completion as principle, not suggestion.

## P7 — Output is string, bounded

Tools return LLM-facing strings. Cap large outputs (shell 50k; web_fetch max_length; session truncates tool results at 16k). Prefer “useful summary + truncation marker” over raw dumps.

## P8 — Categories guide filtering, not capability

`ToolCategory` is a soft index for `FilterByIntent` / docs. Capability is Name + Schema. Do not invent parallel permission systems inside categories.

## P9 — Break cycles with interfaces

codedom’s `TestImpactProvider` / `KernelQuerier` pattern is the model for tools that need kernel or world: define interfaces in tools, implement in core/world, register at boot.

## P10 — Helpers ≠ tools

LLM-client-coupled features (grounding, thinking) are helpers under `research/`. Do not register them as tools unless they have a pure string Execute path without holding a client.

## P11 — Dual registry is temporary truth

Until session reads VS modularTools only, both registries must stay in sync via HydrateModularTools. New boot paths must not register into only one.

## P12 — Wiring before deletion

If a tool “looks unused,” grep: RegisterAll, intent_routing, AllowedTools configs, prompt atoms, e2e names, VirtualStore routes. Prefer wiring audit over delete.
