# Preserve delegated verdicts in the next task's handoff

**Status:** source-derived, independently reviewed; not reproduced. Main `356f65e0e6bb639adaa90ab36212daa714114871` and feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` were inspected. This concerns a conditional *subsequent handoff*, not the original user-facing escalation, which is correctly reported.

## The discrepancy

A coder delegation can be rejected by the verifier and shown to the user as **Task Escalation Required**. If that remains the prior shard result, a later contextual task can reconstruct its nonempty prose as an `observation.Return` without the verifier disposition. The projection defaults that incomplete reconstruction to `completed`. The next task text can therefore say `coder returned completed` for a result whose delegation was rejected.

This is not evidence that the next worker trusts the label, that a mutation bypasses verification, or that the underlying code passed. It is a source-level disagreement between the prior verified disposition and the next task's summary.

## Main-branch trace

1. On the verifier's escalation branch, it returns output, the rejected verdict and `ErrMaxRetriesExceeded`: [verifier](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L297-L306).
2. The foreground chat result carries the output in a shard payload while the surface accurately reports escalation: [processing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L496-L510), [escalation formatter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers.go#L391-L417).
3. [Storage](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_lifecycle.go#L263-L296) keeps output, findings, metrics and facts, but not the verifier disposition and corresponding error, checks or snapshot.
4. A subsequent contextual `/test` with a prior coder result calls [`priorShardContext`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L129-L138), which [reconstructs the return](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L164-L204) from those partial fields.
5. For nonempty output and absent failure/outcome, the [projection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/subagent.go#L569-L589) defaults to completed; its [text](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/observation/subagent.go#L1043-L1063) is placed directly in the task. Expanding its handle cannot recover fields discarded before encoding.

## Feature branch: a narrower route

Feature's background query `/test` uses a special `run_tests` task and **does not use this prior-context formatter**: [background](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/background.go#L233-L242). Do not claim that the normal background query follows the main witness. A foreground mutation `/test` can still take the contextual formatter. Other formatter branches pass prior tester/reviewer returns to `/fix` or `/debug`, but their specific outcomes and reachability require their own evidence.

The in-memory `buildPriorShardSummaries` also assigns success broadly, but normal JIT compilation clears the associated session-context baggage; legacy renderers alone are not evidence that ordinary JIT workers see that separate success label. The finding here is the directly embedded task string on the stated route.

## Proposed contract

Retain a typed handoff combining the worker's observation **and** the delegation/verifier disposition. Worker `/done` alone is not acceptance when the judge rejected it. Carry owner/request identity, outcome, verification stage and verdict, snapshot, observed checks, changed paths, citations and the original output or durable handle. Missing legacy fields must render as unknown rather than successful. Keep the original escalation display, permission rules and independent verification gates.

A typed receipt with provenance is analogous to preserving an execution history when rebuilding state; [Temporal's event-history documentation](https://docs.temporal.io/encyclopedia/event-history) illustrates why recovery needs the events that establish state. It is not a recommendation to adopt that system.

## Evaluation and costs

In a future deterministic TUI fixture, produce identical nonempty worker output under accepted, rejected/escalated, failed, canceled and unverified dispositions; follow with contextual `/test` and capture the actual next task text. A rejected case must retain its rejected status even when the worker reported done. Check feature background query separately, foreground mutation, legacy records with absent status, long-output handle expansion, multiple sessions and newer results replacing older ones. Assert truthful escalation display and unchanged write/verification permissions.

Measure serialized receipt size, context tokens, extra lookups, retained payload and cleanup, duplicate summaries, worker retry and task completion quality against matched tasks. Compare structured compact receipts with current prose and keep detail retrievable progressively. No observed production error, measured speedup or accuracy gain is claimed.
