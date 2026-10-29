# Deep dive: constitutional safety at the composition root

This optional deep dive separates policy authorship from boot wiring. The
canonical invariant summary remains [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).

## What system owns

System registers the policy kernel shard and assigns it the complete
authorization envelope:

```text
pending_action(ActionID, Action, Target, Payload, Timestamp)
permitted_action(ActionID, Action, Target, Payload, Reason)
permission_check_result(ActionID, Permitted, Reason, Timestamp)
permitted(Action, Target, Payload)
```

The exact declarations and rules are core-owned. Co-location is system-owned
because a CortexKernel routes predicates to separate RealKernel stores.

## Why exactness matters

A policy grant for `/read_file`, `a.go`, and one canonical payload cannot permit
`b.go` or another encoding payload. `safe_action/1` may classify a verb but does
not bind target or payload, so it cannot authorize a request.

`internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`
asserts a pending action against the default layout, proves exact permission,
and proves both target and payload substitution deny.

## Defense in depth after policy

VirtualStore also validates the constitution, requires Dreamer simulation for
mapped destructive actions, dispatches the effect, runs post-action validators,
and emits correlated result facts. These checks are independent: a positive
policy result does not compensate for an unavailable Dreamer, and simulation
does not grant permission.

## Known bypass

`sessionVirtualStoreAdapter.ReadFile` and `.WriteFile` call the OS directly.
That adapter must gain a typed policy-preserving capability before this corpus
can claim every system-constructed file path is constitutional.
