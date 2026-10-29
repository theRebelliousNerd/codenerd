# Architectural principles: shards

## 1. Creativity proposes; logic and code commit

LLMs may classify, decompose, advise, and propose Mangle. Core Mangle policy
decides executive facts. Constitution and VirtualStore enforce effects. A prompt,
profile, confidence score, or readiness fact cannot grant authority.

## 2. Preserve the exact envelope

Action ID, type, target, and canonical payload are one contract from executive
through constitution, router, VirtualStore, and terminal receipts. Never mint a
new action ID at an adapter boundary or authorize only an action class.

## 3. Default deny, including degraded dependencies

A missing kernel, missing exact permission, malformed envelope, missing required
route, or unavailable required safety participant is denial. Optional creative
work may degrade; effect authority may not.

## 4. One registration authority, explicit enrichers

Factories, profiles, predicate ownership, startup, and dependencies need one
typed source. Browser/UI/store enrichers are named adapter layers. A second
hard-coded table is debt and must have parity tests until removed.

## 5. Auto-start is a safety contract

Only the minimum OODA/safety spine starts automatically. Campaign, planner,
world, router, and legislator remain on demand unless a policy and product
decision changes their lifecycle. Submission is not readiness.

## 6. One terminal outcome

Every spawn, consultation, observation generation, permission consumption, and
route ends once. Partial results retain their failures. Cancellation propagates.
Retries require idempotency and never repeat an external effect blindly.

## 7. JIT atoms own stable model behavior

New system behavior becomes atoms and typed selection. User/task data may remain
inline as bounded payload. Compatibility fallback is explicit per call and
observable; deterministic executive behavior never falls back to an LLM.

## 8. Repair before persistence

Candidate Mangle must parse, use declared predicates/arity, bind negation,
stratify, and avoid protected control-plane heads before hot-load or persistence.
The retry budget is finite and rejection is a valid terminal result.

## 9. State has an owner and generation

ShardManager owns active/result state; concrete shards own local state; kernel
owns facts; stores own durable learning. Restartable managers create a fresh
generation. Returned views do not expose mutable internal pointers.

## 10. Bounds are part of correctness

Queue size, workers, model calls, validation attempts, event buffers, cache
entries, output bytes, fact retention, scan scope, and timeouts must remain
finite and diagnosable.

## 11. Evidence outranks optimistic wiring

Registration, a constructor, or a passing build is not reachability. Prove boot,
dispatch, teardown, negative safety, and user-visible terminal behavior. Search
all boot and adapter paths before deleting apparently unused code.

## 12. Personas stay declarative

Domain coding agents belong to prompt atoms, agent databases, and session
execution. `internal/shards` should remain a thin system spine plus collaboration
libraries, not grow another bespoke persona class tree.
