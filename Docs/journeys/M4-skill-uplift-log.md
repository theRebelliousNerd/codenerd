# M4 — Skill uplift log: mangle-programming, from syntax reference to programming model

## Status

- last updated: 2026-09-19 03:00
- done: 1 Before; 2 Decisions; 3 After; 4 Examples run; 5 Mirror; 6 Next uplift (hand-written by the merger after the uplift agent stalled twice); 7 Second pass: every example run and fixed, the tools checked against the engine (v1.1.0)
- open: the rest of the prompt atoms (section 7, "What the next pass should do", item 1: 344 left)
- NOTE (merger, 2026-09-18 15:15): the uplift agent stalled at 10:30 after writing section 1 and touched nothing in the skill directory (verified by mtime). Sections 2-6 are owed by the S11 relaunch, which starts from M0/M2/M3 as revised.

Branch: `dogfood/c2-closure`. Repo: `C:/CodeProjects/codeNERD`. Scratch: `C:/Temp/mangle-study/`.
Rules followed: no repo writes outside this file and the skill directory (+ its `.codex` mirror); no git write operations.

## 1. Before: file list with sizes

Measured 2026-09-18 with `find -type f -printf '%s	%p'`: **57 files, 1,933,976 bytes** (the brief said 55 / ~1 MB; the two extra files are `960-FORK_FEATURES_v0.5.1.md` and `SHARDING_STRATEGIES.md`, which exist only in `.claude/` and not yet in the `.codex/` mirror). `scripts/mangle-cli.js` alone is 1,202,742 bytes.

The `.codex/skills/mangle-programming` mirror **exists** (55 files) and is **not** byte-identical today: `diff -rq` reports 21 differing files plus the two `.claude`-only files above (recorded before any edit).

| bytes | path (under `.claude/skills/mangle-programming/`) |
|---:|---|
| 6,924 | `assets/codenerd-schemas.mg` |
| 5,983 | `assets/examples/access-control.mg` |
| 8,963 | `assets/examples/aggregation-patterns.mg` |
| 4,513 | `assets/examples/vulnerability-scanner.mg` |
| 113 | `assets/go-integration/go.mod` |
| 5,298 | `assets/go-integration/main.go` |
| 2,781 | `assets/README.md` |
| 5,252 | `assets/starter-policy.mg` |
| 3,999 | `assets/starter-schema.mg` |
| 7,024 | `references/000-ORIENTATION.md` |
| 11,676 | `references/100-FUNDAMENTALS.md` |
| 28,701 | `references/150-AI_FAILURE_MODES.md` |
| 16,893 | `references/200-SYNTAX_REFERENCE.md` |
| 9,812 | `references/250-BUILTINS_COMPLETE.md` |
| 14,416 | `references/300-PATTERN_LIBRARY.md` |
| 4,127 | `references/400-RECURSION_MASTERY.md` |
| 37,534 | `references/450-PROMPT_ATOM_PREDICATES.md` |
| 3,338 | `references/500-AGGREGATION_TRANSFORMS.md` |
| 3,668 | `references/600-TYPE_SYSTEM.md` |
| 2,609 | `references/700-OPTIMIZATION.md` |
| 2,709 | `references/800-THEORY.md` |
| 4,498 | `references/900-ECOSYSTEM.md` |
| 23,262 | `references/950-ADVANCED_ARCHITECTURE.md` |
| 13,411 | `references/960-FORK_FEATURES_v0.5.1.md` |
| 7,165 | `references/ADVANCED_PATTERNS.md` |
| 5,853 | `references/CLI_BATCH_API.md` |
| 8,665 | `references/CLI_COMMANDS.md` |
| 8,787 | `references/CLI_ERROR_CODES.md` |
| 5,449 | `references/CLI_OUTPUT_SCHEMAS.md` |
| 56,867 | `references/context7-mangle.md` |
| 11,310 | `references/EXAMPLES.md` |
| 19,556 | `references/GO_API_REFERENCE.md` |
| 8,497 | `references/PRODUCTION.md` |
| 21,904 | `references/SHARDING_STRATEGIES.md` |
| 4,364 | `references/SYNTAX.md` |
| 10,254 | `references/VALIDATION_TOOLS.md` |
| 25,692 | `scripts/analyze_module.py` |
| 25,470 | `scripts/dead_code.py` |
| 29,250 | `scripts/diagnose_stratification.py` |
| 9,335 | `scripts/EXAMPLES.md` |
| 4,446 | `scripts/examples/performance_antipatterns.mg` |
| 28,879 | `scripts/explain_derivation.py` |
| 20,574 | `scripts/generate_stubs.py` |
| 4,400 | `scripts/generate_template.py` |
| 1,202,742 | `scripts/mangle-cli.js` |
| 35,732 | `scripts/profile_rules.py` |
| 4,765 | `scripts/QUICKSTART.md` |
| 4,244 | `scripts/README.md` |
| 7,354 | `scripts/README_ANALYZER.md` |
| 8,122 | `scripts/README_explain_derivation.md` |
| 2,771 | `scripts/README_trace_query.md` |
| 248 | `scripts/sizes.json` |
| 7,487 | `scripts/STUB_GENERATOR_README.md` |
| 31,272 | `scripts/trace_query.py` |
| 30,949 | `scripts/validate_go_mangle.py` |
| 39,553 | `scripts/validate_mangle.py` |
| 14,516 | `SKILL.md` |


## 2. Decisions: kept / harvested-and-updated / deleted, per file

| file | decision | why |
|---|---|---|
| `SKILL.md` | **rewritten** (v1.0.0) | It described "Google's Mangle 0.4.0 (November 2024)" and opened with syntax. It now opens with the programming model (witness vs judgement, five sentences), a pattern picker keyed to live corpus examples, the engine truths verified on the pinned commit, a one-screen syntax block, the working checklist for adding a decision to codeNERD's policy, and how the prompt corpus's 119 `/mangle` atoms reach codeNERD's own agents. The CLI, scripts and asset sections were kept verbatim and renumbered. |
| `references/010-PROGRAMMING_MODEL.md` | **new** | The seven patterns distilled from M2, each with what M3's verification changed and where it is live in the corpus. Points at M2/M3 by repository path; does not copy them (one source of record). |
| `references/020-ENGINE_TRUTHS_v0.5.1.md` | **new** | Sixteen behaviours established by running programs on the pinned engine or by a production failure, with the mechanism: silent negation deletion, constant-only external inputs, the single-atom aggregation wildcard, the created-fact limit's misleading message, no in-engine retraction, the `ToAtom` string/name heuristic. |
| `references/context7-mangle.md` (57 KB) | **deleted** | A dump of the downstream 0.4.0 documentation: 116 "Google" mentions, describing an engine this repository does not run. Its one inbound link (960) now points at 020. |
| `references/SYNTAX.md` | **deleted** | Duplicate of `200-SYNTAX_REFERENCE.md`, listed as "being migrated" since 2025. Its inbound link (000) was rewritten. |
| `references/000-ORIENTATION.md` | **updated** | The "legacy files being migrated" block replaced with the entry point to 010/020. |
| `references/960-FORK_FEATURES_v0.5.1.md` | **updated** | See-also repointed. |
| `150-AI_FAILURE_MODES`, `450-PROMPT_ATOM_PREDICATES`, `SHARDING_STRATEGIES`, the numbered 100-950 references, `GO_API_REFERENCE`, `PRODUCTION`, `ADVANCED_PATTERNS`, `EXAMPLES`, `VALIDATION_TOOLS`, `CLI_*`, `scripts/`, `assets/` | **kept** | Harvest list from the plan; none carries the 0.4.0 framing beyond a passing mention. Not re-verified line by line in this pass (section 6). |

## 3. After: file list with sizes

57 files (55 before the two additions and two deletions net to 57 with the two `.claude`-only files now mirrored). New: `references/010-PROGRAMMING_MODEL.md`, `references/020-ENGINE_TRUTHS_v0.5.1.md`. Gone: `references/context7-mangle.md` (56,867 bytes), `references/SYNTAX.md` (4,364 bytes). `SKILL.md` is 319 lines (was 376).

## 4. Examples run

The one-screen syntax block in `SKILL.md` section 4, completed with its `Decl`s, was checked with the production binary: `nerd.exe check-mangle skill_example.mg` -> `OK` (parse, analysis, stratification on the pinned engine; binary built from `f8290327`). The statements in 020 are not new examples: each cites the M3 probe or the production commit that established it. The numbered references' older examples were **not** re-run in this pass.

## 5. Mirror

`.codex/skills/mangle-programming` was deleted and regenerated as a copy of `.claude/skills/mangle-programming`; `diff -rq` reports no differences (it reported 21 differing files and two missing ones before). The mirror is never edited on its own.

## 6. What the next uplift should do

1. Re-run every example in the numbered references (100-950, EXAMPLES, ADVANCED_PATTERNS) through `nerd check-mangle` and delete or fix what fails; this pass verified only what it wrote.
2. Put each of 020's truths into the prompt corpus as a `/mangle` atom where one does not exist (the skill teaches the development agent; the atoms teach the agent inside the harness), and check the 119 existing atoms for 0.4.0-era claims the way `context7-mangle.md` was.
3. Add the dropped-negation lint (negated-atom count per clause before and after analysis) to `nerd check-mangle`; 020 item 1 is still caught only by reading.
4. When the upstream external-predicate fix lands (020 item 5), rewrite Pattern A's section and `policy/knowledge.mg`'s comment.
5. Teach provenance (Pattern F) with a runnable `DerivationRecorder` example once `nerd why` uses it.

## 7. Second pass (2026-09-19): every example run, the tools checked against the engine

Method: a scratch Go probe loaded every fenced `mangle` block in the skill through a fresh pinned
engine (the path `nerd check-mangle` uses) and sorted failures into unmarked, marked-as-wrong and
fragment; then every marked and fragment failure was read too, because the heuristic buckets hid
broken "CORRECT" blocks. Each replacement was run first with `nerd check-mangle --standalone
--eval` (added in `a978b7b8`) and checked for the facts it should derive, not only for loading.

| | before | after |
|---|---|---|
| fenced Mangle blocks | 261 | 220 (listings that are not programs became `text`) |
| load on the pinned engine | 59 | 206 |
| fail without saying so | 115 | 0 |
| marked wrong-way, failing for the reason their text gives | some, often for another reason | 14 |
| skill size | 2.0 MB | 522 KB |

What was wrong, by class: `not` for negation (the syntax reference's summary table taught it);
predicates used with no `Decl` (85 blocks, fixed by declaring what the engine named); Prolog
`[H|T]` lists; `fn:filter`, `fn:divide`, `fn:multiply`, `fn:mod`, `fn:len`, `fn:concat`,
`fn:map_get`; `do A, do B` in one stage and let-only stages; unbound variables in comparisons,
heads and `:match_field`; float comparisons; underscores in variables; a name constant swallowing
the period; wildcard negations (the starter template had five, each silently deleted); a
flagship example that joined `/log4j` against `"log4j"` and could never derive; "CORRECT" type
declarations with one bound for two columns; a claim that there are no list/map/union/any bound
types (there are, capitalised). New engine truths 17-28 in `020-ENGINE_TRUTHS_v0.5.1.md`,
including: negation is reordered and comparisons are not; source facts are not checked against
bounds; the engine's own spec spells `bounds`, which does not parse; struct literals in a premise
do not destructure; temporal literals cannot evaluate in codeNERD (no temporal store); `fundep` +
`merge` lattices load and never finish evaluating.

Tools: `mangle-cli.js` (an error on the canonical `bound [/name, /name]` form), `validate_mangle.py`
(VALID for a duplicate Decl), `trace_query.py` and `explain_derivation.py` (no result, and a
negation read as a positive premise), `analyze_module.py`, `dead_code.py`,
`validate_go_mangle.py`, `generate_stubs.py` (imports `github.com/google/mangle`) and
`generate_template.py` were deleted with their docs (`CLI_*.md`, `VALIDATION_TOOLS.md`, READMEs).
`diagnose_stratification.py` and `profile_rules.py` stayed as advisory helpers after agreeing
with the engine. `EXAMPLES.md` (the first half of `300-PATTERN_LIBRARY.md`, byte for byte) and
`assets/codenerd-schemas.mg` (a stale copy of the kernel's own schema) were deleted.

In the prompt corpus: the legislator atom taught `blocked(X) :- candidate_action(X),
!permitted(X, _, _).` as CORRECT (run, it blocks every action); fixed with the LSP cheatsheet's
two-wildcard orphan rule, and pinned by `TestEmbeddedCorpus_TeachesNoWildcardNegation`.

The skill stays gitignored with the rest of `.claude/skills/` (Steve, 2026-09-19: keep ignored
paths ignored); this log is the tracked record of what changed in it. The `.codex` copy is
regenerated from the `.claude` one.

### What the next pass should do

1. The prompt atoms carry the same classes of error: `not` in at least 12 places, `fn:divide` /
   `fn:multiply` / `fn:filter` in `mangle/builtins_complete.yaml`, let-only reducer stages in the
   antipattern atoms, and 654 fenced examples that fail to load (the ratchet in
   `cmd/tools/validate_prompt_atoms`). Run the same method there: it is what codeNERD's own agents
   learn from. **2026-09-19, `f26140e1`:** the method ran over the atoms -- 296 missing Decls added
   from the engine's diagnostics, 71 `not` rewritten to `!`, 35 wildcard negations rewritten as
   negations of a projection; 972 of 2,102 examples load and the ratchet is 344. Left: top-level
   `=`, queries written as clauses, `fn:divide`/`fn:multiply`, comparisons over unbound variables,
   `external` Decl syntax, let-only stages, three wildcard negations, and the 625 examples the
   ratchet skips as marked wrong-way (unaudited: in the skill, some of those were broken CORRECT
   blocks).
2. Add the dropped-negation check to `nerd check-mangle` itself (compare negated-atom counts per
   clause before and after analysis), so it is caught in any file, not only by the corpus guard.
3. Items 4 and 5 of section 6 stand.
