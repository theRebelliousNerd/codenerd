# 04 — Architectural Principles: persist / factsnap

> Last verified against codebase: **2026-08-15**  
> These principles are **binding** for changes under `internal/persist/`.

## P1 — Facts only

Serialize `[]types.Fact` (and only that shape). Do not grow factsnap into a general filesystem, config, or chat-log store.

## P2 — Semantic codecs, not semantic forks

Gzip vs zstd (and Auto→gzip) must remain **semantically identical**. Any new codec must pass a parity test like `TestCodecParity`.

## P3 — Deterministic SimpleColumn

Always write with `SimpleColumn{Deterministic: true}` unless a measured, documented reason appears. Determinism enables reproducible artifacts and stable tests.

## P4 — Atomic publish

Never leave a partial final path. Pattern: unique temp in the destination
directory → sync → close → chmod → rename → fsync the directory; remove the temp
on failure. The temp name must be unique per call: a shared `<path>.tmp` lets two
writers interleave into one file and rename the mixture over a good snapshot.
Preserve this for every new write entrypoint, sidecars included.

## P4b — A snapshot and its digest move together

Anything that writes a snapshot writes (or removes) its `.sha256` in the same
call, under the same per-path lock. A stale sidecar is worse than none: it makes
every subsequent read fail on a file that is perfectly good.

## P5 — Extension is the contract for *finding* a file; content is the contract for *decoding* it

Canonical extensions (`.sc.gz`, `.sc.zst`) are part of the public API and
`snapshot.Resolve` depends on them. `CanonicalPath` / `ensureExt` own rewriting;
callers may pass logical basenames. On read, magic bytes outrank the suffix,
because a rename can invalidate a name but not a container header.

## P6 — Avoid core import cycles

Do not import `internal/core` from factsnap. Local `atomToFact` is intentional. If consolidation is needed, extract a **neutral** helper under `internal/types` or a tiny `internal/facts` convert package — not core→persist or persist→core.

## P7 — Preserve name-constant round-trips

Name constants must survive Write→Read as values that `ToAtom()` re-encodes as names (today: `types.MangleAtom`). Do not “simplify” to bare strings without updating tests and `ToAtom` symmetry.

## P8 — Policy lives outside

factsnap never implements `permitted(...)`. Loaders that assert into the kernel own safety.

## P9 — Prefer compression that earns its keep

Default gzip for portability; zstd for size. Keep `TestSizeComparison` (or successor) so regressions where SimpleColumn+gzip loses to JSON are caught.

## P10 — Wiring audit before deletion

Zero importers is **not** proof of dead code. This package spent a release as a
dormant integration point and then acquired a real caller. Audit callers, tests,
and design intent before removal.

## P13 — The workspace layout lives in `snapshot`, not in callers

No call site builds `.nerd/snapshots/...` by hand or invents a naming scheme.
`snapshot.Dir`, `SanitizeName`, `Export` and `Resolve` are the one place those
rules live, which is what lets a snapshot exported by one caller be found by
name from another.

## P14 — Loading a snapshot is an act, not a side effect

Nothing in this subsystem may assert snapshot facts implicitly — no boot hook,
no auto-import, no "helpfully" reloading the newest file. A snapshot is
untrusted the moment it leaves the process that wrote it, and a valid digest
proves only that the bytes are intact, not that their contents are true.

## P11 — Errors are wrapped and prefixed

All public errors use `factsnap: …` / `snapshot: …` prefixes with `%w` where
appropriate. Keep that for greppability, and keep sentinels (`ErrIntegrity`)
matchable with `errors.Is`.

## P12 — Library stays dependency-light

Default write path must not force zstd usage. Optional zstd is fine because the module already depends on klauspost; do not add heavy new deps for marginal format gains.
