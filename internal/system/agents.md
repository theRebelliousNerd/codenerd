# System wiring guidance

- Keep the complete authorization envelope (`pending_action`, `permitted_action`,
  `permission_check_result`, and `permitted`) owned by the policy Cortex shard so
  Mangle can join one exact request.
- Routers must preserve the executive-issued action ID and canonical payload;
  never mint a replacement correlation ID at the effect boundary.
- Tests that call `VirtualStore.RouteAction` directly must assert the exact
  pending-action envelope. Do not add wildcard or `safe_action` test bypasses.
- After boot/factory or router changes, run focused Cortex correlation tests,
  `go test ./internal/shards/system`, and bounded `go test ./internal/system`.
- Treat Cortex boot as a resource transaction. New boot steps must put acquired
  handles on `bootContext`, transfer ownership in `cortexFromBootContext`, and
  remain safe under the shared rollback/`Cortex.Close` path.
- Cortex cache identity must include every boot option that changes live wiring.
  Normalize set-like options before both hashing and applying them; never place
  credentials or raw option material in the cache key.
- Project `execution` settings must configure both the tactile executor and the
  VirtualStore allowlists. Resolve working directories inside the active
  workspace and reject invalid durations or path escapes during boot.
- The production prompt `KernelAdapter` must create a private RealKernel clone
  for each compilation. Never let JIT selector facts mutate the live Cortex.
- The Cortex owns the process browser manager. Bind modular research tools to
  that manager and adapt browser facts into `SystemKernel.AssertBatch`; a
  private browser-only Mangle engine makes runtime evidence invisible.
