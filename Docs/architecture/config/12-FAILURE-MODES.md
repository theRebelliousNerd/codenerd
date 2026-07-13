# 12 — Failure modes and recovery

## Failure taxonomy

| Failure | Current outcome | Honest recovery |
|---|---|---|
| JSON absent | **VERIFIED CURRENT** — empty aggregate; effective helpers default; feature registry is preserved. | First-run wizard/init or explicit env detection. |
| JSON malformed | **PARTIAL** — shared Cortex fails before perception and ambient fallback; secondary/campaign paths may still soften errors. | Stop boot; repair or restore last valid file; never route via ambient key. |
| unknown/invalid JSON field | **PARTIAL** — unknown/trailing JSON is rejected; most semantic invalidity still reaches consumers. | Full validation and typed diagnostic before constructors. |
| wizard rerun | **PARTIAL** — focused test proves representative unrelated fields survive; concurrent writer/version behavior remains undefined. | Add expected-version transaction and broaden mutator conformance. |
| interrupted save | **PARTIAL** — injected pre-rename failure leaves original bytes; no backup/version conflict or post-rename outcome receipt. | Recover last valid snapshot; do not synthesize defaults over damaged input. |
| permission exposure | **PARTIAL** — files request `0600` and trace defaults false; plaintext keys and unredacted opt-in trace remain. | Verify ACL/mode per platform, secret references and redaction. |
| config changed mid-run | **VERIFIED CURRENT** — disk changes; active logging/features/JIT/scheduler may remain old. | Restart or explicit full reload; partial hot mutation is unsupported. |
| execution block set | **PARTIAL** — shared Cortex validates/projects all fields; campaigns copy binaries/env/directory without shared timeout/containment and dormant legacy uses defaults. | Treat campaign/dormant parity as unproven; Mangle permission remains independently mandatory. |
| Codex isolation relaxed | **VERIFIED CURRENT** — backend ignores relaxation, forces read-only/shell-disabled and filters overrides; hostile-config regression passes. | Keep the backend-boundary regression mandatory. |
| MCP URL/protocol/timeout invalid | **PARTIAL** — conversion trusts strings; failure moves to connect time. | Validate before client construction; degrade optional server with named reason. |
| global state reused across workspace | **PARTIAL** — feature/logging/timeout state can outlive load identity. | Reinitialize under a workspace snapshot lifecycle or run separate process. |

## Cancellation and partial failure

Config decoding has no context because local reads are bounded only by file size
and the OS; there is no explicit maximum size. Save has a local atomic file stage
but no cancellation, idempotency key, version lock or backup. Shared boot
attributes load failure directly; softened secondary/campaign paths can still
make later partial failure hard to attribute to rejected input.

**PROPOSED UPLIFT.** Bound file size/depth, validate before side effects, make
save idempotent by expected snapshot/version, and reverse-unwind any boot stage
that accepted a projection before a later required stage fails.

## Recovery invariants

1. A present invalid file is not equivalent to absence.
2. Failed save leaves the prior valid bytes and active snapshot unchanged.
3. Failed reload leaves all consumers on the prior snapshot or none; never a mix.
4. Optional integration degradation is named and cannot alter provider or
   permission selection.
5. Secret exposure triggers rotation guidance; logs/config are not copied into a
   diagnostic packet.
6. Rollback chooses a complete prior schema/snapshot, not field-by-field defaults.

## Bounded triage

Confirm workspace identity, inspect parse/validation diagnostics, compare the
redacted snapshot ID to each consumer projection, then restore the last valid
file or use a future read-only migration command. The wizard now refuses a
malformed original because its load error is terminal; competing-writer recovery
is still undefined.
