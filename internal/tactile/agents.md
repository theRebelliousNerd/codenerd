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
