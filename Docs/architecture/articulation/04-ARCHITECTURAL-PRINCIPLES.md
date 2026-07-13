# 04 — Architectural Principles: articulation

> Binding principles for `internal/articulation/`. Reviewers should reject PRs that violate these without explicit exception notes.

## P1 — Dual channel, single envelope

Every protocol-compliant model response is a **PiggybackEnvelope**: `control_packet` + `surface_response`. Do not invent parallel ad-hoc formats for the same job (e.g. “tools in prose, facts in markdown”) inside this package.

## P2 — Thought-first ordering

Control is **logical commitment** before surface speech. Struct field order, JSON schema property order, and `PiggybackProtocolSuffix` must keep control first. Streaming may show surface incrementally only after locating the surface string; it must not invent control from surface prose.

## P3 — Parse is total for user display

`Process` / `ProcessLLMResponse*` should not leave operators with empty screens when the model produced text. Prefer salvage and friendly truncation over hard failure unless `RequireValidJSON` is intentionally set.

## P4 — Confidence and method are API

Callers must be able to branch on `ParseMethod` and `Confidence`. Changing method strings or confidence semantics is a **breaking contract** with chat, session, and shards.

## P5 — Caps before trust

No control list is unbounded. Surface, mangle updates, memory ops, tools, knowledge, and reasoning traces all have caps. New control fields need caps in `applyCaps`.

## P6 — Filter untrusted mangle syntax here; authorize elsewhere

Articulation rejects malformed / shell-dangerous atom strings. **Permission** (`permitted`, VirtualStore gates) stays in core/session/policy. Do not re-implement full policy inside the parser.

## P7 — JIT-first prompts; legacy is emergency

New LLM-facing behavior lands as prompt atoms and selectors (`internal/prompt/atoms/...`). Growing hard-coded templates in `prompt_assembler.go` is a last resort and must be temporary.

## P8 — Kernel dependency is an interface

`KernelQuerier` keeps the assembler testable and decoupled from concrete kernel types. Prefer query predicates (`injectable_context`, `shard_prompt_base`) over smuggling full kernel handles.

## P9 — Import-cycle discipline

Use `PromptAssemblerAdapter` and map-based DI (`mapToPromptContext`) rather than importing perception/autopoiesis into this package. Store `*PromptAssembler` behind `any`/`interface{}` at shard boundaries when needed.

## P10 — Structured tools over provider function-calling monopoly

`tool_requests` in control packet is the canonical multi-provider tool surface. Native provider tool APIs may still exist, but Piggyback must remain the portable representation.

## P11 — Adversarial input is expected

Embedded JSON extraction assumes decoys, depth bombs, and oversized candidates. Prefer state-machine scanners with hard limits over regex. Last-match-wins for rich envelopes.

## P12 — Observability by default

Hot paths use `logging.CategoryArticulation`, timers, and explicit fallback error logs. Silent parse failures are a bug.

## P13 — No Vectryx product vocabulary

This is codeNERD substrate documentation. Do not introduce unrelated product terms into this package’s architecture docs or APIs.
