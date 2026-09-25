# Tactile maintenance guidance

- Tactile executes effects; it does not decide constitutional permission.
- Keep capability probes bounded and cached. Repeated executor construction must
  not launch one external Docker probe per instance.
- Propagate caller-supplied `ExecutorConfig` into availability checks and backend
  construction; never probe defaults and execute with different configuration.
- Preserve platform containment and output limits, then run focused detector and
  factory tests plus the dependent VirtualStore route that constructs tactile.
- Analyzer and audit facts must use execution-scoped predicates whose exact
  arities and bounds are declared under `internal/core/defaults/`; do not reuse
  tester/world predicates with different meanings. Emit percentages as bounded
  integers and prove changed fact shapes through a real `RealKernel`, not only a
  callback mock.
- On-disk audit logs are a security boundary: keep owner-only permissions,
  redact environment/stdin values, bound captured output, and surface sink
  failures through metrics and logs.
- Every production `NewDirectExecutor*` outside this package is an exception to
  VirtualStore's governed route and must be registered in
  `direct_bypass.go#DirectBypassRegistry` with its permission proof and audit
  sink; `TestDirectBypassRegistryMatchesProductionConstructors` enforces it.
  Give a bypass an audit sink with `NewFactAuditedExecutor`, not a silent
  executor.
- Isolation backends are registered from host probes
  (`registerPlatformIsolation`); an explicit mode the host cannot provide stays
  unregistered and fails closed. Never fall back to direct for an explicit mode.
