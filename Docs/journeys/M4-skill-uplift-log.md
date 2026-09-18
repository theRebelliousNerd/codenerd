# M4 — Skill uplift log: mangle-programming, from syntax reference to programming model

## Status

- last updated: 2026-09-18 10:08
- done: 1 Before
- open: 2 Decisions; 3 After; 4 Examples run; 5 Mirror; 6 Next uplift
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

(pending)

## 3. After: file list with sizes

(pending)

## 4. Examples run

(pending)

## 5. Mirror

(pending)

## 6. What the next uplift should do

(pending)
