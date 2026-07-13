# Vision: a policy-governed specialist mesh

## Product outcome

The user asks one question and experiences one coherent codeNERD, even when
multiple specialists contribute. The system should choose only the participants
that add value, give each a bounded creative role, make readiness and failure
visible, and preserve one deterministic effect authority.

## Target architecture

### One typed registry

One descriptor names each shard's factory, profile, dependencies, startup mode,
owned predicates, prompt selectors, resource limits, readiness, and teardown.
Runtime-specific enrichers attach browser, ToolStore, Glass Box, or campaign
manager dependencies without rewriting the base factory graph.

### One correlated operation story

Ingress, consultation, spawn, permission, route, and completion preserve stable
identities. Every operation terminates as success, partial success, denial,
cancellation, timeout, or failure. No missing response or retained fact is
allowed to masquerade as success.

### Policy-certified activation

Mangle derives a finite activation plan from capabilities, dependencies,
configuration, and resource facts. Go owns actual spawning and effects. A boot
generation distinguishes fresh readiness from stale heartbeats. Required safety
participants must be ready for the generation before a dependent effect; an
optional advisor may degrade without deadlocking the session.

### JIT-native cognition

Stable model-facing behavior lives in validated prompt atoms. Task/user data
stays bounded and separate. Each call declares required-JIT, optional-JIT, or a
specific compatibility fallback. A redacted receipt shows atom IDs, budget,
truncation, and fallback without storing secrets.

### Evidence-aware specialists

Heuristic and semantic retrieval may nominate specialists. Typed classification,
readiness, permission, cost, and task contracts decide eligibility. Evidence
freshness and confidence are visible; recommendation never equals authority.

## Non-goals

- a distributed actor platform;
- permanent processes for every persona;
- model-authored permission or readiness;
- moving core policy and declarations into `internal/shards`;
- reintroducing coder/reviewer/tester/researcher Go hierarchies;
- requiring every optional specialist before responding;
- recording raw prompts, source bodies, or secrets in lifecycle receipts.

## Measurable success

| Outcome | Falsifiable target |
|---|---|
| registration parity | every boot enumerates one descriptor set; duplicate ownership and incomplete authorization envelope fail validation |
| action integrity | one action ID/type/target/payload reaches one terminal effect or denial; zero permissive replay |
| collaboration honesty | every requested specialist has a response or named failure; total failure cannot return nil error |
| lifecycle honesty | Start/Stop/Start either works by generation or is explicitly rejected; public liveness matches worker liveness |
| readiness | required failure blocks only dependent effects; optional failure degrades visibly and does not deadlock |
| JIT discipline | every LLM call maps to an atom family and explicit fallback policy |
| operational containment | receipts are versioned, redacted, size-capped, and retention-bounded |

## First slices

1. Lock manifest/production parity and preserve the policy authorization join.
2. Repair batch, observer, and router terminal semantics with negative tests.
3. Inventory and normalize JIT call policies.
4. Add activation generations only after one registry and terminal outcomes exist.
