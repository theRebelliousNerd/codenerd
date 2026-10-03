# Architecture documentation: current reality plus the north-star specification

## What these documents are

`Docs/architecture/` is codeNERD's living specification and architectural north
star: **where we are, plus a human-readable and LLM coding-agent-usable
specification that pushes implementation toward the behavior and capabilities
we want.** Both halves are necessary. Neither may replace the other.

The corpus explains each subsystem's purpose, intended behavior, requirements,
design, contracts, actual implementation, and the work needed to close the gap.
It must help a human understand the system and let a coding agent build, wire,
and test the intended behavior without inventing missing requirements.

"Current" includes the current engineering intent and accepted design. It does
NOT mean "only the code that happens to exist today." A capability belongs in
the specification while its implementation is missing, incomplete, or unwired.

This is neither a frozen wishlist nor a historical record. Update the design
deliberately as our understanding changes. Drive implementation toward the
specification; do not reduce the specification to excuse incomplete code. The
north star describes the behavior the finished system exhibits and continuously
applies pressure to the distance between that behavior and present evidence.
It is not a milestone to check off or a claim that the project is complete.
Closing a gap establishes a specific capability; it does not end that pressure.

Every subsystem's documentation must answer:

1. What are we trying to accomplish, and why does this subsystem exist?
2. What behavior, capabilities, and guarantees are required?
3. How should the architecture make those requirements hold?
4. What actually works today, and what evidence establishes that?
5. What remains missing, incorrect, unwired, or undecided?
6. What implementation and tests would close that distance?

## What these documents are not

- Not a historical record, run diary, agent handoff, or changelog.
- Not a transcript of source inspection or a catalogue of functions and files.
- Not a stenographic summary of whatever the current implementation does.
- Not a collection of test receipts masquerading as a design.
- Not marketing prose or an uncheckable list of ambitions.
- Not an excuse to discard an intended capability because it is absent.
- Not a place to manufacture confidence, implementation status, or completeness.

Git holds revision history. Operational logs and execution artifacts belong
outside the architecture corpus. Keep a decision's rationale when it explains
the current design; do not turn the main documents into a chronology of runs.

## Keep target, implementation, and gaps distinct

- Vision, capability specifications, principles, internal architecture, API
  contracts, safety requirements, and testing requirements define the intended
  system. Preserve their substantive design, including unimplemented parts.
- `02-CURRENT-STATE.md`, `IMPLEMENTED_SPEC.md`, and equivalent explicitly
  implementation-focused documents describe what runs now. These may be concise,
  but claims need actual source and appropriately scoped behavioral evidence.
- Gap analysis compares intended behavior with implementation. Each significant
  gap needs a named requirement, actual deficiency, intended outcome, dependencies,
  and a checkable exit criterion.
- Wiring documents distinguish intended integration from verified reachability.
  "Code exists" is not "the feature works through its production entry path."
- `TODO.md` is the current build queue, not a diary. Keep authoritative
  `NERD_FEATURE` cards there, traceable to requirements and gap IDs.
- Open questions identify unresolved design decisions. They do not authorize
  erasing the capability that depends on those decisions.

Evidence decides whether implementation meets a requirement. It does not decide
whether an absent requirement should be removed from the specification.

Make the path from intent to implementation explicit. For each significant
capability, a human or coding agent must be able to identify the required behavior,
the relevant contracts and constraints, the present implementation and evidence,
the remaining gap, and observable acceptance conditions. Keep these linked across
the existing documents rather than duplicating the entire specification in each.
When today's behavior violates the intended contract, describe it as a deficiency
to fix, not as a new architectural requirement merely because the code does it.

## Non-negotiable editing rules

1. Do not shorten, flatten, or replace design prose merely to make the corpus
   smaller, easier to validate, or closer to today's code.
2. Do not replace substantial architecture documents with thin current-state
   summaries, tables of source pointers, or redirects.
3. Do not remove future capabilities, invariants, rationale, diagrams, contracts,
   failure handling, or planned tests because they are not implemented yet.
4. Moving intended design into a `legacy/` archive does not preserve its place in
   the current specification. Keep active requirements in their proper documents.
5. Retire or replace a requirement only through an explicit design decision.
   Explain the reason, replacement, and affected contracts; absence of code is
   not that decision. Ask the user when the decision is not established.
6. A rewrite must retain or improve every substantive requirement. Review the
   diff for lost design and acceptance obligations, not just broken citations.
7. Separate proposed APIs, paths, predicates, and examples from shipped claims.
   Label them as planned or example material; do not delete them to avoid labeling.
8. Update implementation status without rewriting the target to manufacture a
   pass. A focused test proves its named scope, not whole-feature completion.
9. Never declare dormant code dead before tracing intended and actual wiring.
10. Make specifications implementable: explain responsibilities, state and data
    shapes, interfaces, Go/Mangle boundaries, lifecycle, errors, safety, integration,
    and tests in enough detail to build the intended behavior.

## codeNERD architectural requirements

Preserve the north star: the model is the creative center; Mangle is the executive.
Planning, orchestration, context selection, safety, and completion obligations are
derived by the kernel, not quietly transferred to model discretion or Go heuristics.
Go supplies the FFI, drivers, and typed tools. New LLM-facing behavior is JIT-first,
with prompt atoms, piggybacking, and control-packet-aware integration. Context must
be bounded, revision-aware, and recoverable; effects require exact default-deny
authorization. Specify the guarantees and the distance still to close.

## Ownership and organization

- Implementation agents are code-only unless the user explicitly assigns a
  documentation task. They must not generate, condense, or rewrite Markdown as a
  side effect of coding. The orchestrator owns necessary specification updates.
- Do not invoke corpus-generation workflows to normalize away substantive design.
- Preserve established document responsibilities. If a corpus uses the canonical
  18-document layout, each document must fulfill its responsibility. Do not create
  padded files merely to satisfy a file-count check.
- Keep existing `corpus.toml` source ownership exclusive and `portfolio.toml`
  registration accurate. Serialize shared index and registry changes.
- Follow the target/implementation distinction in
  `Docs/journeys/09-architecture-doc-standard.md`. Structural standards are aids,
  not authority to delete the design they are meant to organize.
- Establish ownership before editing shared work. Preserve existing user changes;
  integrate deliberately rather than resetting, overwriting, or shelving them.
- After large documentation reorganizations, update scoped `AGENTS.md` guidance
  so it reinforces these rules instead of granting stale rewrite mandates.

Success is a clearer, more complete, currently relevant specification that drives
implementation and testing toward the north star. Smaller documents are not a
success criterion. Describing today's implementation alone is not completion.
