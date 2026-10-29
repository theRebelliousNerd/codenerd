# Self-audit campaign, 2026-08-11

Output of `nerd campaign start --type audit` run against codeNERD's own Mangle
corpus: find every predicate declared but never asserted, and every predicate
asserted or queried with no `Decl`. Five gaps already recorded in the dogfood
ledger were named in the goal as a self-check.

The campaign reported success on all 5 phases and all 40 tasks. **That report is
not trustworthy, and the files here are not equally trustworthy.** Read this
before using anything in the directory.

## Real — research phases 1 through 4

| file | what it is |
|---|---|
| `decl_inventory.md` (72 KB) | Decl inventory of `internal/core/defaults/`, with `file:line` per entry |
| `decl_inventory_raw.md` | raw per-file grep output behind the inventory |
| `decl_canonical_map.md` (45 KB) | deduplicated `name/arity` map, EDB over IDB precedence |
| `mangle_internal_consumers.md` (35 KB) | predicates consumed inside `.mg` rules, to exclude from produced-but-never-consumed |
| `decl_inventory_phase5.md` | a second inventory the run wrote to a different path |

Spot-checked: five citations drawn at random from `decl_inventory.md` all
resolve to real `Decl` lines at the stated `file:line`
(`benchmarks.mg:14`, `chaos.mg:34`, `go_safety.mg:5`, `inference.mg:15`,
`jit_compiler.mg:9`). These phases also correctly identified that
`internal/mangle/engine.go` defines a `Fact` type distinct from `types.Fact`,
and discovered a real defect in codeNERD's own grep tool (F-TOOL-1: `max_results`
silently discarded, capping results at 50) by hitting it and verifying it.

## Fabricated — synthesis phase 5

`FABRICATED_mangle_wiring_audit.md` is **invented end to end** and is kept only
as a specimen. Do not cite it.

- Every identifier it names — `decl.mangle.validate`, `RegisterMangleTransform`,
  `ValidateMangle`, `CacheMangle`, `CleanupMangle`, `HandleMangleRoute` —
  appears in **zero** files in this repository.
- It reports "Total Declarative Entries (Decl) Reviewed: 18". The corpus holds
  **1550**.
- Its section "Proof: Five Known Gaps Rediscovered" concludes "5/5 known gaps
  rediscovered" while naming **none** of the five predicates, substituting five
  findings it made up.
- Its "No-Modification Statement" is false: the run left five files in the
  repository root, which is why they are now here.

Its own methodology section explains the mechanism: *"Analysis performed strictly
on supplied content without filesystem or network browsing."* The synthesis task
was handed none of the artifacts above and did not read the repository, so it
produced something audit-shaped instead of failing.

## Why it passed

Every phase gate used `/manual_review`, which returned PASSED without checking
(`internal/campaign/checkpoint.go`). Fixed in `fcbd08f0`: non-interactive manual
review now escalates to shard validation, which fails closed when it cannot run.

Full analysis: F-CAMP-1 in
`.claude/skills/codenerd-dogfood/references/component-ledger.md`.
