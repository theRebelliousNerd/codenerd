# config — authoritative feature cards

> This is the sole `NERD_FEATURE` authority for the config corpus. Proposed or
> deferred cards are not current behavior.

## P0: Preserve configuration as a secret-bearing transaction

<!-- NERD_FEATURE
id: config-safe-persistence-v1
owner: config
status: in_progress
kind: truth-gap
depends_on: []
affects: [config, cli, init, observability]
-->

**Value.** A wizard or auth command cannot erase unrelated settings, leak API
keys to other local users, or leave a truncated config after interruption.

**Evidence and observed gap.** `internal/config/persistence.go#writePrivateFileAtomically`
now performs same-directory write, `0600`, sync and rename for JSON and YAML;
`cmd/nerd/chat/config_wizard_steps.go#Model.saveConfigWizard` now loads and merges
its fields. `internal/config/config_security_test.go` proves pre-rename
preservation, Unix `0600`, and round-trip; `cmd/nerd/chat/config_wizard_save_test.go`
proves representative unowned fields survive. Concurrent-writer, backup,
cross-platform permission and secret-diagnostic contracts remain; values are
plaintext and there is no expected snapshot/version.

**Desired behavior.** Load and merge under one workspace-scoped transaction,
validate before commit, write and sync a sibling temporary file, atomically
replace, preserve a bounded backup/recovery receipt, and use owner-only
permissions where the platform supports them. Secret references are preferred
over copied secret values.

**Non-goals.** Do not make config a credential vault, preserve unknown invalid
fields forever, or allow a failed save to mutate active runtime state.

**Affected contracts.** UserConfig and legacy Config persistence, wizard/auth/init
mutators, permissions, crash recovery, feature activation.

**Positive acceptance.** Tests prove unrelated nested fields survive wizard and
auth edits, a forced pre-rename failure leaves the old file byte-identical, a
successful replace reloads, and Unix permission checks observe `0600`.

**Negative acceptance.** Raw keys never enter logs or receipts; concurrent
writers cannot interleave JSON; malformed existing files are not silently
replaced; Windows ACL limitations are reported rather than claimed secure.

**Rollback.** Retain the last valid config and disable the mutating command if
atomic replacement fails; never fall back to direct truncating writes.

## P0: Strictly decode and validate one immutable boot snapshot

<!-- NERD_FEATURE
id: config-strict-snapshot-v1
owner: config
status: in_progress
kind: truth-gap
depends_on: [config-safe-persistence-v1]
affects: [config, system, perception, core, prompt, shards]
-->

**Value.** A typo or corrupt file fails before codeNERD selects a provider,
opens stores, or creates an effect adapter.

**Evidence and observed gap.** `internal/config/persistence.go#decodeStrictJSON`
now rejects unknown fields and trailing JSON, and `LoadUserConfig` uses it. There
is still no schema migration or full semantic validator. Shared Cortex now
propagates present-invalid load errors and refuses ambient rescue of an explicit
unusable LLM selection, with regressions in
`internal/system/factory_execution_test.go` and
`internal/config/config_security_test.go`. Secondary consumers still soften some
errors. Validation on legacy `Config` covers only provider/key and a subset of
limits when called explicitly.

**Desired behavior.** Decode with unknown-field rejection into a versioned raw
shape, migrate, normalize, validate cross-field invariants, then publish an
immutable snapshot and typed redacted diagnostics. Missing-file first run remains
a distinct explicit state; malformed or invalid present files fail closed.

**Non-goals.** Do not ban forward migration, infer invalid values with an LLM,
or turn warnings into silent defaults for security-sensitive fields.

**Affected contracts.** provider/engine selection, context percentages, JIT
reserve math, core/scheduler floors, URLs/durations/paths, features, boot order.

**Positive acceptance.** Table and fuzz tests cover every field family, unknown
keys, cross-field bounds, migrations, and deterministic diagnostics; integration
tests prove no client/store/executor constructor runs after rejection.

**Negative acceptance.** An explicit provider never falls through to an ambient
key; invalid allowlists, negative budgets, bad URLs and impossible percentages
do not reach consumers; diagnostics contain no values classified secret.

**Rollback.** Offer a read-only migration/dry-run command and a time-bounded
compatibility decoder that still validates; never restore malformed-file boot.

## P0: Project effective execution config into every effect path

<!-- NERD_FEATURE
id: config-execution-projection-v1
owner: config
status: in_progress
kind: truth-gap
depends_on: [config-strict-snapshot-v1]
affects: [config, core, tactile, system, cli, campaign]
-->

**Value.** The same allowlist, environment, working-directory containment and
timeout bounds govern interactive chat, shared Cortex, campaigns, and one-shot
commands.

**Evidence and observed gap.** Shared Cortex resolves, contains and projects
binaries, env, directory and timeout through
`internal/system/factory_execution.go#executionLayerConfigs`, with positive and
hostile regressions in `internal/system/factory_execution_test.go`. Campaign
start/resume copy binaries/env/directory without the shared timeout/containment
helper and soften load errors; dormant
`cmd/nerd/chat/session_boot.go#performSystemBootLegacy` uses defaults.

**Desired behavior.** Resolve one typed effective execution projection at boot,
validate it against the workspace boundary, pass it to each VirtualStore/tactile
constructor, and record only non-secret digests in the boot receipt. Mangle
authorization remains an independent mandatory gate.

**Non-goals.** Config does not grant `permitted/3`, add binaries automatically,
or weaken an explicitly requested isolation backend.

**Affected contracts.** VirtualStoreConfig, tactile executor, campaign and chat
boot, environment filtering, timeout/cancellation, path containment.

**Positive acceptance.** Cross-surface integration tests use a non-default
allowlist, env set, directory and timeout and observe the same effective values
and denial behavior in all constructors.

**Negative acceptance.** An unlisted binary, env var, escaped working directory,
or stale projection is denied; missing config cannot broaden defaults; projection
mismatch cannot be repaired by skipping constitutional permission.

**Rollback.** Disable effect construction and report the invalid projection;
fall back only to an equal-or-stricter reviewed profile.

## P1: Emit a redacted configuration provenance receipt

<!-- NERD_FEATURE
id: config-provenance-receipt-v1
owner: config
status: proposed
kind: leverage
depends_on: [config-strict-snapshot-v1, config-execution-projection-v1]
affects: [config, observability, transparency, system]
-->

**Value.** An operator can answer which file/default/environment/migration
selected each effective behavior without printing the configuration or secrets.

**Evidence and observed gap.** `internal/config/config.go#Config.applyEnvOverrides`
has order-dependent YAML overrides, `UserConfig.GetContext7APIKey` has separate
precedence, and boot logs mostly provider/model or feature summaries. There is no
field-level provenance or snapshot identity.

**Desired behavior.** Emit a versioned snapshot ID, schema version, source
digests, field origin classes, normalization/validation decisions, consumer
projection IDs, and redacted warnings with bounded retention and correlation.

**Non-goals.** Do not store raw file contents, keys, OAuth tokens, prompts, full
paths outside policy, or use the receipt as authorization.

**Affected contracts.** config loader, env policy, boot logs, system factory,
transparency, support diagnostics.

**Positive acceptance.** File, default, env, migration and rejection fixtures
produce deterministic receipts that correlate with consumer projections and let
an operator explain the active provider and every bound.

**Negative acceptance.** Seeded secrets and prompt text never appear; receipt
failure is visible but cannot change the selected config or allow an effect.

**Rollback.** Disable durable receipt storage while retaining validation and a
bounded in-memory snapshot ID; never fall back to full-config logging.

## P3: Compare migrations and defaults in a no-effect laboratory

<!-- NERD_FEATURE
id: config-migration-shadow-lab-v1
owner: config
status: deferred
kind: moonshot
depends_on: [config-provenance-receipt-v1]
affects: [config, verification, system, campaign]
-->

**Value.** Maintainers can see how a schema/default/precedence change would alter
real redacted workspace behavior before promotion.

**Evidence and observed gap.** `DefaultConfig` and `DefaultUserConfig` already
drift on shard concurrency, execution timeout, context window and trace defaults;
tests do not compare effective consumer projections across a migration corpus.

**Desired behavior.** Replay versioned, redacted config fixtures through current
and candidate decoders without constructing providers, stores or executors; diff
effective projections and require reviewed promotion/rollback evidence.

**Non-goals.** No live credentials, network calls, tool effects, automatic
promotion, or claim that platform-specific permissions are equivalent.

**Affected contracts.** schema migration, defaults, provenance receipts,
cross-platform tests, release governance.

**Positive acceptance.** Fixed fixtures produce deterministic semantic diffs,
resource bounds are enforced, and promotion cites expected positive/negative
changes plus compatibility policy.

**Negative acceptance.** The lab cannot call any client or executor, cannot read
unredacted production config, and cannot publish a candidate automatically.

**Rollback.** Delete lab artifacts and disable candidate evaluation; runtime
decode and the last verified migration remain unchanged.
