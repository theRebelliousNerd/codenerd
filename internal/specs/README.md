# Spec pairing (`internal/specs`)

## What pairing is
A `Link` (`model.go`) joins a Markdown `Section` (`Ref` like `Docs/x.md:Install > Linux`) to a code `Element` (`Ref` like `internal/core.RealKernel.Evaluate`). `Relation` is `cites`, `specifies`, or `implements`; `Source` is `doc_citation`, `code_comment`, or `annotation`. `Result` (`resolve.go`) holds `Links`, `Sections`, `Elements`, `Confirmed`, and refused `Problem`s. `Result.Facts` (`facts.go`) projects them as `spec_section`, `spec_element`, `spec_link`, `spec_confirmed`, and `spec_changed_since` facts (`schemas_spec.mg`).

## How links are found
`ScanDoc` (`doc_scan.go`) reads each section's own text via `ownSpans`/`ownEnd`, skipping fenced code blocks. `DocScan.scanSpan` finds path hits with `CodeLexer` plus backticked symbols, `pathSelectors` attaches `#Sym`/`:Sym` selectors, and `takeAnnotations` turns `<!-- implements: ... -->` into explicit links. `ScanCode` (`code_scan.go`) reads `spec: doc.md Heading` lines from doc comments (gated by `MayCiteSpecs`). `Resolve` (`resolve.go`) matches them through `CodeIndex` (`NewCodeIndex`): `resolver.docCite` resolves section citations to elements (`inFile`/`global`, stale refs kept as written), `resolver.codeCite` resolves `spec:` lines to sections, and `resolver.result` adds link targets plus the exported universe of reached packages.

## Fingerprints
Identity of content is `codemodel.Fingerprint`: layout folded away, tokens kept. A section print covers its own text after the heading (code blocks included, nested sections excluded) in `ScanDoc`; an element print covers its span including its leading comment (a header's is the whole file) in `Element.Fingerprint` (`model.go`). Reflowing text does not move a print; rewording does.

## Confirmation store
`confirm.go` defines `LinkKey`, `Confirmation` (`SpecFingerprint`, `CodeFingerprint`, `ConfirmedAt`, `SpecChangedAt`, `CodeChangedAt`), and `Confirmations`. `Reconcile` advances the set per measurement: first sight confirms at-now, both sides moved re-confirms at-now, one side moved keeps the old confirmation and stamps first-seen time, a side back at confirmed clears its stamp, a gone link drops its row. `Confirm` records a reviewed agreement. `store.go` persists this in `StorePath` (`.nerd/specs/links.json`) via `OpenStore`, `Store.Load`, and `Store.Update` (read-modify-write under one lock). The world scan (`world/spec_facts.go`) loads the store, calls `Resolve` with `Confirmed`/`Now`, and writes back `Result.Confirmed`.

## Drift rules
Judgment lives in `internal/core/defaults/policy/spec_pairing.mg`. `spec_drift` holds when the element print differs from `spec_confirmed` while the section print matches; `spec_stale_code` is the reverse. `spec_drift_since`/`spec_stale_since` read the first-seen time from `spec_changed_since` (`/code`/`/spec`). `spec_drift_last_commit`/`spec_drift_commit` attribute drift to the latest `git_history` commit to the element's file at or after `ConfirmedAt`. `dangling_spec_link` flags a link whose element or section is gone; `unspecced_element`/`spec_coverage` track reached packages' uncovered exports.

## Holographic view
`world/holographic_specs.go` renders drift where you already look. `loadSpecView` queries `spec_link`, `spec_drift`, `spec_stale_code`, `spec_drift_since`, `spec_stale_since`, `spec_drift_commit`, and `dangling_spec_link`; `specBlock` lists a file's links (`fromCode`/`fromSection`), `specEntryLine` marks `DRIFT` vs `STALE CODE`, and `specChangeDetail` adds first-seen time, latest commit, and the fix-up call.

## Confirming a link
Read both sides first. If neither needs an edit (each moved after the other, or the change is intended), run `confirm_spec_link` (`tools/codedom/spec_links.go`, `ConfirmSpecLinkTool`). `world.SpecLinkConfirmer.ConfirmSpecLink` (`world/spec_confirm.go`, wired by `system/spec_confirmer.go`) checks `spec_link`, reads current prints from `spec_section`/`spec_element`, calls `Confirm`, and rewrites the store plus the link's `spec_confirmed` row. Editing code after section (or section after code) re-confirms on the next scan without the tool.
