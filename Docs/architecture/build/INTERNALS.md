# build — internals

Verified 2026-09-20 against commit `456e521` (`main`). All line ranges are
1-indexed file lines in `internal/build/env.go` unless stated.

## The environment pipeline

One pipeline with three entry points, all converging on `getBaseGoEnv`
(:403-449), which assembles PATH, GOCACHE (via `deriveGOCACHE`, :453-484),
CGO flags (via `detectCGOFlags`, :544-574), and the persisted
`config.BuildConfig` (via `loadBuildConfig`, :487-530):

1. `GetBuildEnv(userCfg, workspaceRoot) []string` (:126-165) — the base
   environment every `go` spawn should use.
2. `GetBuildEnvForModule(userCfg, moduleDir) []string` (:175-177) — the
   module-aware entry point. It resolves the module directory through
   `DetectionRootFor` (:187-209), which walks up to the detection root and
   stops at repo boundaries per `isRepoBoundary` (:213-220). It has no
   production callers; see WIRING-AND-NOT-BUILT.md.
3. `GetBuildEnvForTest(userCfg, workspaceRoot) []string` (:241-261) — the
   test specialization: GOTRACEBACK, `-count=1` folded into GOFLAGS, and the
   ambient CI/GORACE/GOMAXPROCS/GOTMPDIR propagation. It has no production
   callers; see WIRING-AND-NOT-BUILT.md.
4. `GetBuildEnvForCompile(userCfg, workspaceRoot, targetOS, targetArch)`
   (:277-289) — the cross-compile specialization; sets GOOS/GOARCH.

## Flag handling

- `AppendGoFlags(userCfg, workspaceRoot, args) []string` (:300-338)
  injects configured GoFlags into argv for flag-bearing subcommands only
  (gated by `goFlagSubcommands` at :304, flag names parsed by `goFlagName`
  at :342-350), skipping flags already present by name (:321-327). It has
  no production callers; see WIRING-AND-NOT-BUILT.md.
- `withCountOne(goflags string) string` (:265-273) is the `-count=1`
  folder used by the test specialization.

## Observability and merge helpers

- `SummarizeEnv(env) string` (:355-368) renders an environment for logs,
  redacting secrets via `redactEnvValue` (:371-382) and
  `redactedPlaceholder` (:384-389). Used by the package's own debug logging
  at :163 and :498.
- `MergeEnv(base, additional...) []string` (:601-613) merges env slices,
  later values winning via `setEnvKey` (:588-597); key lookup is
  `hasEnvKey` (:577-585) and value lookup is `envValue` (:392-400).
- `DefaultBuildConfig() *BuildConfig` (:46-49) delegates to
  `config.DefaultBuildConfig`; `BuildConfig` itself (:43) is an alias for
  `config.BuildConfig`, so there is exactly one struct definition. Its only
  production caller is `loadBuildConfig` at :488.
- `GetBuildEnvForModule`'s root walk is the only user of `envSliceFromMap`
  outside config loading (:534-540); both feed through the same merge path.

## Test tags

`TestTagsForWorkspace(workspaceRoot string) []string`
(`internal/build/tags.go:18-23`) returns `-tags sqlite_vec` for codeNERD
workspaces only, detected by `isCodeNERDWorkspace` (`tags.go:25-42`).
It has exactly two production callers; see WIRING-AND-NOT-BUILT.md.

## How verified

- Every range above ends at a `func`/`type` line confirmed by the file's
  own element index this turn; bodies were read for `AppendGoFlags`
  (:300-338), `MergeEnv`/`setEnvKey`/`hasEnvKey` (:577-614),
  `GetBuildEnvForTest`'s doc comment (:222-261), and the package comment
  (:1-23).
- Orphan status (no production callers) established by grepping each bare
  function name across the repo: hits are the definition and the package's
  own tests only (`internal/build/env_features_test.go`,
  `internal/build/env_gaps_test.go`).
- The `go`-spawn inventory is owned by
  `TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt`
  (`internal/build/go_invocation_inventory_test.go:61`), and the consumer
  list by
  `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented`
  (`go_invocation_inventory_test.go:142`); the exemption map lives at
  :32-52 and the non-test-files scope rule at :26-28.
- Tag behavior is pinned by `TestTestTagsForWorkspace_Detection`
  (`internal/build/tags_test.go:12`).
