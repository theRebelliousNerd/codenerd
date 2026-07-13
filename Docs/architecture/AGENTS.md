# Architecture corpus guidance

This directory explains and improves codeNERD's live architecture. Source and
tests outrank prose. Keep current truth, target design, and open questions visibly
separate.

## Required contract

- Every realized corpus owns the exclusive roots in `corpus.toml`; register it in
  `portfolio.toml`. Do not invent padded corpora for child or governance surfaces.
- Preserve the canonical 18 documents. Put optional deep dives behind explicit
  README links; never let a redirect satisfy a canonical responsibility.
- Make each README a plain-language front door with the seven sections defined in
  `_rebuild/SUPERSTAR_CORPUS_STANDARD.md` and a concrete end-to-end journey.
- Use only the standard claim labels. A current claim needs `path#symbol-or-predicate`
  plus behavioral evidence. Dates and file existence are freshness, not proof.
- Put authoritative `NERD_FEATURE` cards only in `TODO.md`. Proposed code and
  paths must be labeled `planned:` or `example:`.
- Cover every applicability lane or give a package-specific, evidenced `N-A`.
- Classify and harvest legacy content before deletion. Preserve incident history,
  rejected alternatives, and unique rationale in canonical destinations.
- Verified product bugs may be fixed only as bounded packets with reproduction,
  causal scope, regression, wiring check, corpus reconciliation, and rollback.

## Working safely

Record the dirty worktree before a packet, re-read targets before patching, and
preserve unrelated user or Grok work. One writer owns a corpus; serialize shared
portfolio, index, validator, and ledger edits.

Validate changed corpora structurally, then perform source-grounded semantic
review. Machine validation does not prove prose truth or runtime wiring.
