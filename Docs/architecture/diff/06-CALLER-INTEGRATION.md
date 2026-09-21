---
doc-class: shipped-with-future
subsystem: diff
implementation-status: partial
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 06 Caller Integration — internal/diff

This file specifies how the diff engine attaches to its callers. Shipped
seams below are claims the code bears out today, each cited with
repo-relative path plus symbol and line. Planned behaviour, data flow,
failure modes, proving tests, vision trace, and gap IDs follow in later
sections.

## 1. Seams in callers (shipped)

Two production seams exist. No other production importer was witnessed.

### 1.1 Agent repair loop — `internal/session/turn_diff.go`

- `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) renders the
  turn's edits file by file, returning `""` when nothing was written.
- `func renderFileDiff` (`internal/session/turn_diff.go:55-76`)
  short-circuits identical content (`internal/session/turn_diff.go:59-61`).
- Seam call `fd := diff.ComputeDiff(path, path, before, after)` at
  (`internal/session/turn_diff.go:62`), which is the `DefaultEngine`
  convenience wrapper `func ComputeDiff`
  (`internal/diff/diff.go:310-312`) over `var DefaultEngine`
  (`internal/diff/diff.go:238`).
- Nil or zero-hunk results map to `""`
  (`internal/session/turn_diff.go:63-64`).
- The caller composes hunk framing itself with
  `@@ -%d,%d +%d,%d @@` (`internal/session/turn_diff.go:69`) and renders
  markers via `diffMarker(l.Type)` (`internal/session/turn_diff.go:71`).
- `func newFileNote` (`internal/session/turn_diff.go:78-83`) marks only
  `IsNew` (`struct FileDiff` at `internal/diff/diff.go:98-105`).
- `func diffMarker` (`internal/session/turn_diff.go:85-94`) maps `LineAdded`
  to `+`, `LineRemoved` to `-`, default to space.

### 1.2 TUI diff view — `cmd/nerd/ui/diffview.go`

- `var uiDiffEngine = diff.NewEngine()`
  (`cmd/nerd/ui/diffview.go:907`) with the single-engine rationale
  (`cmd/nerd/ui/diffview.go:900-906`); `func NewEngine`
  (`internal/diff/diff.go:214-216`) equals
  `NewEngineWith(Options{})` (`internal/diff/diff.go:220-230`), so this
  engine runs zero-`Options`.
- `func CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:910-912`)
  delegates to `uiDiffEngine.ComputeDiff` at
  (`cmd/nerd/ui/diffview.go:911`).
- `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) reports
  `uiDiffEngine.Stats()` at (`cmd/nerd/ui/diffview.go:916`).
- Each view holds the package engine: field `diffEngine *diff.Engine`
  (`cmd/nerd/ui/diffview.go:146`), assigned as `diffEngine: uiDiffEngine`
  in the constructor (`cmd/nerd/ui/diffview.go:181`).
- The side-by-side word path calls
  `d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)`
  (`cmd/nerd/ui/diffview.go:970`) on
  `func (e *Engine) ComputeWordLevelDiff`
  (`internal/diff/diff.go:530-551`).
- The single-engine invariant is pinned by
  `func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
  (`cmd/nerd/ui/word_highlight_test.go:141-162`).

## 2. Data flow (shipped)

### 2.1 Agent repair loop — before/after strings to prompt text

- `func turnDiffSection` (`internal/session/turn_diff.go:24-52`)
  iterates the files the turn wrote and concatenates one
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) block per
  file; when nothing was written it returns `""`.
- Per file, `func renderFileDiff` (`internal/session/turn_diff.go:55-76`)
  calls `fd := diff.ComputeDiff(path, path, before, after)` at
  (`internal/session/turn_diff.go:62`), which is the `DefaultEngine`
  convenience wrapper `func ComputeDiff`
  (`internal/diff/diff.go:310-312`) over `var DefaultEngine`
  (`internal/diff/diff.go:238`).
- Identical content short-circuits before rendering
  (`internal/session/turn_diff.go:59-61`); nil or zero-hunk results map to
  `""` (`internal/session/turn_diff.go:63-64`).
- On a non-empty `struct FileDiff` (`internal/diff/diff.go:98-105`), the
  caller owns framing: it composes `@@ -%d,%d +%d,%d @@`
  (`internal/session/turn_diff.go:69`) from `struct Hunk`
  (`internal/diff/diff.go:89-95`), renders each `struct Line`
  (`internal/diff/diff.go:82-86`) via `diffMarker(l.Type)`
  (`internal/session/turn_diff.go:71`), and appends `func newFileNote`
  (`internal/session/turn_diff.go:78-83`) for `IsNew`.
- `func diffMarker` (`internal/session/turn_diff.go:85-94`) maps `LineAdded`
  to `+`, `LineRemoved` to `-`, default to space, so the prompt receives
  plain `+/−/space` lines under caller-composed `@@` headers.

### 2.2 TUI diff view — strings to rendered hunks and word spans

- `func CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:910-912`)
  delegates to `uiDiffEngine.ComputeDiff` at
  (`cmd/nerd/ui/diffview.go:911`) on `var uiDiffEngine = diff.NewEngine()`
  (`cmd/nerd/ui/diffview.go:907`), which is
  `NewEngineWith(Options{})` (`internal/diff/diff.go:214-216`,
  `internal/diff/diff.go:220-230`).
- The returned `struct FileDiff` (`internal/diff/diff.go:98-105`) carries
  `struct Hunk` (`internal/diff/diff.go:89-95`) lists the view renders;
  the engine never emits `LineHeader`
  (`internal/diff/diff.go:48-56`), so `@@` framing stays in the caller.
- The side-by-side word path calls
  `d.diffEngine.ComputeWordLevelDiff(line.Content, nextLine.Content)`
  (`cmd/nerd/ui/diffview.go:970`) on
  `func (e *Engine) ComputeWordLevelDiff`
  (`internal/diff/diff.go:530-551`), which returns `struct WordSpan`
  (`internal/diff/diff.go:76-79`) spans the view highlights.
- `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) reports
  `uiDiffEngine.Stats()` at (`cmd/nerd/ui/diffview.go:916`) from
  `method Engine.Stats` (`internal/diff/diff.go:233-235`).

### 2.3 Engine internals — cache lookup around deterministic compute

- `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) flags
  empty sides `IsNew` (`internal/diff/diff.go:250-252`) and `IsDelete`
  (`internal/diff/diff.go:253-255`), then short-circuits NUL-bearing
  inputs detected by `func containsNullByte`
  (`internal/diff/diff.go:24-26`) to `IsBinary` with empty hunks
  (`internal/diff/diff.go:261-265`).
- Otherwise it builds `struct cacheKey` (`internal/diff/diff.go:114-129`)
  via `func fingerprint` (`internal/diff/diff.go:141-157`), probes
  `method diffCache.get` (`internal/diff/cache.go:96-121`), and on miss
  runs `DiffLinesToChars/DiffMain/DiffCleanupSemantic/DiffCharsToLines`
  (`internal/diff/diff.go:293-296`) inside
  `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`).
- Raw diffs become line operations via `method Engine.diffsToOperations`
  (`internal/diff/diff.go:343-400`), then hunks via
  `method Engine.convertToHunks` (`internal/diff/diff.go:317-332`) and
  `method Engine.groupIntoHunks` (`internal/diff/diff.go:403-486`).
- `method diffCache.put` (`internal/diff/cache.go:126-172`) stores a
  `method FileDiff.Clone` (`internal/diff/cache.go:229-246`) copy sized by
  `method FileDiff.approxSize` (`internal/diff/cache.go:251-264`), and
  `method diffCache.get` (`internal/diff/cache.go:96-121`) returns a clone,
  so callers never share cached backing arrays.
- `func (e *Engine) ComputeWordLevelDiff`
  (`internal/diff/diff.go:530-551`) runs uncached per-line spans in its own
  `struct WordSpan` (`internal/diff/diff.go:76-79`) type.

## 3. Failure modes (shipped behaviour + planned hardening)

Shipped behaviour below is cited with repo-relative path plus symbol and
line. Planned handling uses future tense and ends in a gap ID. Vision
tags (V1-V4) are defined in the gap derivation
(`.nerd/campaigns/b4eeb4a4/artifacts/task_b4eeb4a4_3_0.md`).

### 3.1 FM-01 — cache key collision serves wrong hunks silently

- Shipped: `struct cacheKey` (`internal/diff/diff.go:114-129`) plus
  `func fingerprint` (`internal/diff/diff.go:141-157`) feed
  `method diffCache.get` (`internal/diff/cache.go:96-121`), which verifies
  only when `Options.VerifyCacheContent`
  (`internal/diff/diff.go:183-190`) is set and counts `Collisions` in
  `struct Stats` (`internal/diff/cache.go:28-43`, `Collisions` at
  `internal/diff/cache.go:42`). Both production engines run zero-`Options`
  (`var DefaultEngine` at `internal/diff/diff.go:238` via `func ComputeDiff`
  at `internal/diff/diff.go:310-312` reached from
  `internal/session/turn_diff.go:62`; `var uiDiffEngine` at
  `cmd/nerd/ui/diffview.go:907` via `func NewEngine` at
  `internal/diff/diff.go:214-216`), so verification is off on both paths.
- Effect on callers: either seam (`internal/session/turn_diff.go:62`,
  `cmd/nerd/ui/diffview.go:911`) will render the wrong file's hunks without
  signalling, which breaks V4 (a wrong diff is never silently served).
- Planned: any path where a diff is applied will miss on collision and
  increment `Stats.Collisions` rather than serve, or an ADR will record the
  trusted-key risk as `accepted-not-implemented` — see GAP-DIFF-01.

### 3.2 FM-02 — binary edit is invisible to the repair round

- Shipped: `func containsNullByte` (`internal/diff/diff.go:24-26`) routes
  NUL-bearing inputs to `IsBinary` with empty hunks
  (`internal/diff/diff.go:261-265`), and `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) maps nil or zero-hunk results to
  `""` (`internal/session/turn_diff.go:63-64`). No binary branch was
  witnessed in `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`) or beyond the guard in
  `func renderFileDiff` (`internal/session/turn_diff.go:55-61`).
- Effect on callers: the agent seam (`internal/session/turn_diff.go:62`)
  returns `""`, so the model re-diagnoses an edit it cannot see, which
  breaks V2 (repair round sees its own edits, reduce turns).
- Planned: a binary write will produce a one-line marker (e.g.
  `(binary, not shown)`) in both `renderFileDiff` and `turnDiffSection`
  output — see GAP-DIFF-02.

### 3.3 FM-03 — deleted file gets no note while created file does

- Shipped: `func newFileNote` (`internal/session/turn_diff.go:78-83`)
  marks only `IsNew`, while `IsDelete` exists in `struct FileDiff`
  (`internal/diff/diff.go:98-105`) and is set at
  (`internal/diff/diff.go:253-255`) with no witnessed note branch.
- Effect on callers: the agent seam reports creates but stays silent on
  deletes, so a deleting turn looks like a no-op in the prompt (V2).
- Planned: a deleting turn will get a symmetric note (e.g.
  `deleted by this turn`) mirroring create — see GAP-DIFF-03.

### 3.4 FM-04 — unbounded diff budget floods the repair prompt

- Shipped: the witnessed ranges `method Engine.ComputeDiff`
  (`internal/diff/diff.go:243-307`), `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) composing
  `@@ -%d,%d +%d,%d @@` (`internal/session/turn_diff.go:69`) with
  `func diffMarker` (`internal/session/turn_diff.go:85-94`), and
  `func turnDiffSection` (`internal/session/turn_diff.go:24-52`)
  concatenating one block per file contain no truncation or budget symbol.
- Effect on callers: a large edit passes through the agent seam
  (`internal/session/turn_diff.go:62`) unbounded, which breaks V3 (prompt
  stays bounded, condense the search space).
- Planned: per-file plus per-turn byte/line budgets with an explicit
  truncation marker will bound output, with the budget constant cited by
  symbol and line — see GAP-DIFF-04.

## 4. Proving tests

Shipped pins below are witnessed with repo-relative path plus symbol and
line. Planned checks use future tense, trace to vision tags V1-V4 defined
in the gap derivation
(`.nerd/campaigns/b4eeb4a4/artifacts/task_b4eeb4a4_3_0.md`), and end in a
gap ID.

### 4.1 Shipped — caller-integration pins

- `func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView`
  (`cmd/nerd/ui/word_highlight_test.go:141-162`) pins that
  `func CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:910-912`)
  shares `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`) with the view:
  `DiffEngineStats` before / `CreateDiffFromStrings` / after at
  (`cmd/nerd/ui/word_highlight_test.go:150/151/152`), `Computes` routing
  check (`cmd/nerd/ui/word_highlight_test.go:154-156`), and
  `view.diffEngine != uiDiffEngine` check
  (`cmd/nerd/ui/word_highlight_test.go:158-161`).
- No other caller-integration pin is cited here: `internal/diff/*_test.go`
  bodies were not opened with symbol and line this turn, so per Rule 1a
  they are not named as proving tests.

### 4.2 Planned — gap-closing checks (future tense)

- PT-01 (GAP-DIFF-01, V4): a collision-forcing test will assert that
  `method diffCache.get` (`internal/diff/cache.go:96-121`) misses and
  `Collisions` in `struct Stats` (`internal/diff/cache.go:28-43`,
  `Collisions` at `internal/diff/cache.go:42`) equals 1 with the entry
  dropped, or an ADR witness will record the trusted-key risk as
  `accepted-not-implemented`.
- PT-02 (GAP-DIFF-02, V2): a binary-input test will assert that
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) on
  `IsBinary` empty hunks (`internal/diff/diff.go:261-265`) will return
  non-empty output containing a `binary` marker, and that
  `func turnDiffSection` (`internal/session/turn_diff.go:24-52`)
  will include it instead of `""`.
- PT-03 (GAP-DIFF-03, V2): an `IsDelete` test will assert that
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) on a
  `struct FileDiff` (`internal/diff/diff.go:98-105`) with `IsDelete` set
  (`internal/diff/diff.go:253-255`) will contain a delete note mirroring
  `func newFileNote` (`internal/session/turn_diff.go:78-83`).
- PT-04 (GAP-DIFF-04, V3): a synthetic large-diff test will assert that
  `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) composing
  `@@ -%d,%d +%d,%d @@` (`internal/session/turn_diff.go:69`) with
  `func diffMarker` (`internal/session/turn_diff.go:85-94`) inside
  `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) output
  will stay within a cited budget constant and will contain an explicit
  truncation marker.
- PT-05 (GAP-DIFF-05): where justified, one seam
  (`internal/session/turn_diff.go:62` or
  `cmd/nerd/ui/diffview.go:911`) will construct
  `func NewEngineWith` (`internal/diff/diff.go:220-230`) with non-zero
  `struct Options` (`internal/diff/diff.go:162-191`), and a test will pin
  the differing behaviour; otherwise an ADR witness will mark tuning
  `accepted-not-implemented`.
- PT-06 (GAP-DIFF-06): either a lifecycle call-site for
  `method Engine.ClearCache` (`internal/diff/diff.go:518-520`) will be
  added with a test asserting counters survive concurrent compute, or a
  witness will retire it as test-only.
- PT-07 (GAP-DIFF-07): either a production caller of
  `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) over
  `method Engine.Stats` (`internal/diff/diff.go:233-235`) will be added
  with a test asserting `struct Stats`
  (`internal/diff/cache.go:28-43`) counters advance, or a witness will
  mark it test-only.

## 5. Vision trace (planned)

This section traces each caller seam to the north-star vision sentence at
`agents.md:43-46` ("Tools exist for exactly three things: condense the
search space, reduce the turns to complete the task, and offload cognition
to deterministic code"). Vision tags V1-V4 below are the inferred targets
defined in the gap derivation
(`.nerd/campaigns/b4eeb4a4/artifacts/task_b4eeb4a4_3_0.md`). Shipped anchors
are cited with repo-relative path plus symbol and line; planned behaviour
uses future tense and ends in a gap ID.

- V1 (offload diff computation): both seams will keep delegating all diff
  work to deterministic Go rather than LLM hand-edit — the agent seam
  `fd := diff.ComputeDiff(path, path, before, after)` at
  (`internal/session/turn_diff.go:62`) via `func ComputeDiff`
  (`internal/diff/diff.go:310-312`) over `var DefaultEngine`
  (`internal/diff/diff.go:238`), and the TUI seam
  `func CreateDiffFromStrings` (`cmd/nerd/ui/diffview.go:910-912`) via
  `uiDiffEngine.ComputeDiff` at (`cmd/nerd/ui/diffview.go:911`) over
  `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`). The compute chain
  `DiffLinesToChars/DiffMain/DiffCleanupSemantic/DiffCharsToLines`
  (`internal/diff/diff.go:293-296`) inside `method Engine.ComputeDiff`
  (`internal/diff/diff.go:243-307`) will remain the single place hunks are
  produced, so future callers will add seams rather than reimplement it.
- V2 (repair round sees its own edits, reduce turns): `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`) plus `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) will show the turn every edit it
  made, including the binary path (`func containsNullByte` at
  `internal/diff/diff.go:24-26` to `IsBinary` empty hunks at
  `internal/diff/diff.go:261-265`) and the delete path (`IsDelete` at
  `internal/diff/diff.go:253-255` via `func newFileNote` at
  `internal/session/turn_diff.go:78-83`), so a repair round will not spend
  turns re-diagnosing an invisible edit — see GAP-DIFF-02, GAP-DIFF-03.
- V3 (prompt stays bounded, condense the search space): the bounded-cost
  anchors `const diffTimeout` (`internal/diff/diff.go:14`),
  `defaultContextLines` (`internal/diff/diff.go:17`), `maxContextLines`
  (`internal/diff/diff.go:20`), `func clampContextLines`
  (`internal/diff/diff.go:30-38`), and `method Options.contextLines/timeout`
  (`internal/diff/diff.go:194-211`) applied at `func NewEngineWith`
  (`internal/diff/diff.go:220-230`) will extend to a per-file plus per-turn
  byte/line budget with an explicit truncation marker in `func renderFileDiff`
  (`internal/session/turn_diff.go:55-76`) and `func turnDiffSection`
  (`internal/session/turn_diff.go:24-52`), so the shown diff will condense
  rather than flood the search space under large edits — see GAP-DIFF-04.
  Per-caller tuning via `func NewEngineWith`
  (`internal/diff/diff.go:220-230`) with non-zero `struct Options`
  (`internal/diff/diff.go:162-191`) at one seam
  (`internal/session/turn_diff.go:62` or `cmd/nerd/ui/diffview.go:911`)
  will be adopted where justified or explicitly declined — see GAP-DIFF-05.
- V4 (diffs are trustworthy where applied): the opt-in trust anchors
  `Options.VerifyCacheContent` (`internal/diff/diff.go:183-190`),
  `method diffCache.get` verification (`internal/diff/cache.go:96-121`),
  and `Collisions` in `struct Stats` (`internal/diff/cache.go:28-43`,
  `Collisions` at `internal/diff/cache.go:42`) will harden so that a
  collision will miss rather than silently serve wrong hunks on any path
  where a diff is applied — see GAP-DIFF-01. Lifecycle
  (`method Engine.ClearCache` at `internal/diff/diff.go:518-520`) and
  observability (`func DiffEngineStats` at
  `cmd/nerd/ui/diffview.go:915-917` over `method Engine.Stats` at
  `internal/diff/diff.go:233-235`) will either gain a production caller or
  be retired as test-only by witness, so trust claims will stay attached to
  a live seam — see GAP-DIFF-06, GAP-DIFF-07.

## 6. Gap IDs (caller-integration slice, planned)

This section collects the caller-integration slice of the gap matrix from
the gap derivation
(`.nerd/campaigns/b4eeb4a4/artifacts/task_b4eeb4a4_3_0.md`). Each row anchors
to a shipped seam cited with repo-relative path plus symbol and line,
traces to a vision tag V1-V4 defined in that derivation, and ends in a
machine-checkable exit. No row claims shipped behaviour beyond its cited
seam; target states use future tense.

| Gap ID | Capability (target) | Current seam (shipped, cited) | Target state | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-DIFF-01 | Proven cache keys (V4) | `struct cacheKey` (`internal/diff/diff.go:114-129`) plus `func fingerprint` (`internal/diff/diff.go:141-157`); `method diffCache.get` (`internal/diff/cache.go:96-121`) verifies only when `Options.VerifyCacheContent` (`internal/diff/diff.go:183-190`) is set; both prod engines run zero-`Options` (`var DefaultEngine` at `internal/diff/diff.go:238` via `internal/session/turn_diff.go:62`; `var uiDiffEngine` at `cmd/nerd/ui/diffview.go:907`) | Collision on any apply path will miss and increment `Collisions` (`internal/diff/cache.go:42`) rather than serve, or an ADR witness will record the trusted-key risk as `accepted-not-implemented` | Medium (High if diffs ever drive apply) | Hardening (Phase 3) | `internal/diff/diff.go:114-157` plus `internal/diff/cache.go:96-136` | Test forcing key-collision will assert miss plus `Stats.Collisions==1` with entry dropped; or ADR witness marked `accepted-not-implemented` |
| GAP-DIFF-02 | Binary edits visible to repair round (V2) | NUL sentinel `func containsNullByte` (`internal/diff/diff.go:24-26`) to `IsBinary` empty hunks (`internal/diff/diff.go:261-265`) to `""` via nil/zero-hunk guard (`internal/session/turn_diff.go:63-64`); no binary branch in `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) | Binary write will produce a one-line marker (e.g. `(binary, not shown)`) in `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) and `func turnDiffSection` output | Medium | Prompt-correctness (Phase 1) | `internal/session/turn_diff.go:24-52,55-76` plus `internal/diff/diff.go:261-265` | Test on binary inputs will assert `renderFileDiff` returns non-empty containing `binary` and `turnDiffSection` will include it instead of `""` |
| GAP-DIFF-03 | Delete note symmetric with create note (V2) | `func newFileNote` (`internal/session/turn_diff.go:78-83`) marks only `IsNew`; `IsDelete` in `struct FileDiff` (`internal/diff/diff.go:98-105`) set at (`internal/diff/diff.go:253-255`) with no witnessed note branch | Deleting turn will get a note (e.g. `deleted by this turn`) mirroring create | Low | Prompt-correctness (Phase 1) | `internal/session/turn_diff.go:78-83` plus `internal/diff/diff.go:253-255` | Test on `IsDelete` `struct FileDiff` will assert note string present in `renderFileDiff` output |
| GAP-DIFF-04 | Bounded repair-prompt diff budget (V3) | `method Engine.ComputeDiff` (`internal/diff/diff.go:243-307`) plus `func renderFileDiff` (`internal/session/turn_diff.go:55-76`) composing `@@ -%d,%d +%d,%d @@` (`internal/session/turn_diff.go:69`) with `func diffMarker` (`internal/session/turn_diff.go:85-94`) contain no truncation symbol; `func turnDiffSection` (`internal/session/turn_diff.go:24-52`) concatenates per file with no budget check | Per-file plus per-turn byte/line budget with explicit truncation marker will bound output | High | Prompt-budget (Phase 2) | `internal/diff/diff.go:243-307` plus `internal/session/turn_diff.go:24-76` | Test with synthetic large diff will assert output length within cited budget constant and containing truncation marker |
| GAP-DIFF-05 | Per-caller engine tuning | `struct Options` non-defaults (`internal/diff/diff.go:162-191`) via `func NewEngineWith` (`internal/diff/diff.go:220-230`); prod builds only `var DefaultEngine` (`internal/diff/diff.go:238` at `internal/session/turn_diff.go:62`) and `func NewEngine` (`internal/diff/diff.go:214-216` at `cmd/nerd/ui/diffview.go:907`) — both zero-`Options` | TUI vs agent loop will differ in context width/timeout/cache bounds where justified, or an ADR will explicitly decline tuning | Low | Tuning (Phase 3+) | Both seams `internal/session/turn_diff.go:62` plus `cmd/nerd/ui/diffview.go:907/911/970` | One seam will construct `NewEngineWith` non-zero with a test pinning behaviour; or ADR witness marked `accepted-not-implemented` |
| GAP-DIFF-06 | Cache lifecycle wired or retired | `method Engine.ClearCache` (`internal/diff/diff.go:518-520`) exists with no witnessed caller at `internal/session/turn_diff.go:62` or `cmd/nerd/ui/diffview.go:911/916/970` | Long sessions will either clear on a lifecycle event or the LRU will be documented as sufficient with `ClearCache` marked test-only | Low | Lifecycle (Phase 3+) | Session lifecycle or explicit retire decision | Either a call-site plus test (clear during concurrent compute preserves counters) will land, or a doc/ADR witness will retire it |
| GAP-DIFF-07 | Cache observability wired or retired | `func DiffEngineStats` (`cmd/nerd/ui/diffview.go:915-917`) over `method Engine.Stats` (`internal/diff/diff.go:233-235`); only witnessed caller is the pinning test `func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView` (`cmd/nerd/ui/word_highlight_test.go:141-162`) | `struct Stats` (`internal/diff/cache.go:28-43`) counters will feed a log/gate/policy, or stats will be declared test-only | Low | Observability (Phase 3+) | `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`) | A prod caller of `DiffEngineStats` with a test asserting counters advance will land; or a witness will mark it test-only |



