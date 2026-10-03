# Generated-tool contributor guidance

- Generated registration is host-owned: pin workspace, binary path, digest, and
  protocol. Never infer an execution protocol from a failed invocation or retry
  under a different argument convention.
- Keep process, validation, and feedback outcomes distinct. Durable learning is
  acknowledged only after successful atomic publication to the actual store.
- Live duplicate calls share one execution receipt. Feedback repair must not
  rerun the process. A persisted identity without a recoverable live receipt is
  ambiguous after restart and requires reconciliation.
- Preserve execution statistics when synchronizing catalogs. Corrupt learning
  state must not be silently replaced with an empty successful history.
- Tests must observe native effects, real validation, reopened learning, failed
  publication, and cancellation/drain; a fabricated receipt proves none of these.
