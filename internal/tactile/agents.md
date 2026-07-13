# Tactile maintenance guidance

- Tactile executes effects; it does not decide constitutional permission.
- Keep capability probes bounded and cached. Repeated executor construction must
  not launch one external Docker probe per instance.
- Propagate caller-supplied `ExecutorConfig` into availability checks and backend
  construction; never probe defaults and execute with different configuration.
- Preserve platform containment and output limits, then run focused detector and
  factory tests plus the dependent VirtualStore route that constructs tactile.
