---
doc-class: shipped
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Implemented membership contract

This record describes the uncommitted L1/C1 working tree. Integrated sqlite-vec verification remains open.

| Contract | Implementation witness |
|---|---|
| Cached authority per resolved root and ordered patterns; Windows cache keys fold case | For/Open/cacheKey/canonicalRoot, `internal/workspace/membership.go:61`, `internal/workspace/membership.go:73`, `internal/workspace/membership.go:106`, `internal/workspace/membership.go:118` |
| Tracked and nonignored untracked Git paths; NUL parsing preserves whitespace | loadSnapshot/acceptGitPath, `internal/workspace/git.go:98`, `internal/workspace/git.go:182` |
| Read-only Git commands clear repository redirection, preserve discovery ceilings, set GIT_OPTIONAL_LOCKS=0 | gitEnv/gitRun, `internal/workspace/git.go:17`, `internal/workspace/git.go:39` |
| Gitlinks and nested repos are boundary members; descendants/descent are refused and Files omits boundaries | loadSnapshot/Includes/IncludesDir/snapshotFiles, `internal/workspace/git.go:98`, `internal/workspace/membership.go:254`, `internal/workspace/membership.go:292`, `internal/workspace/membership.go:367` |
| Unknown paths use batched check-ignore and a mutex-protected cache; checks serialize with refresh and return their request's answer | ask/askBatch/checkIgnore, `internal/workspace/membership.go:471`, `internal/workspace/membership.go:477`, `internal/workspace/git.go:247` |
| Explicit refresh always reloads; queries detect control changes; nested ignore edits to known members need explicit refresh | Refresh/ensureFresh/stampCached, `internal/workspace/membership.go:184`, `internal/workspace/membership.go:207`, `internal/workspace/git.go:312` |
| User patterns apply in Git and fallback modes: names at any depth, root-relative slash patterns, **, directory-only trailing /, ordered ! negation | compilePattern/excluded/blocksDir, `internal/workspace/patterns.go:39`, `internal/workspace/patterns.go:111`, `internal/workspace/patterns.go:118` |
| Missing Git/non-worktree roots use patterns; confirmed Git failures fail closed | newMembership/askBatch, `internal/workspace/membership.go:138`, `internal/workspace/membership.go:477` |
| Only .git/.nerd are always excluded; hidden directories otherwise use ordinary membership | alwaysExcluded, `internal/workspace/membership.go:540` |
| For reads target-root ignore settings; absent/empty/invalid settings use the configuration default; Open accepts explicit patterns | loadIgnorePatterns/Open/DefaultWorldConfig, `internal/workspace/membership.go:582`, `internal/workspace/membership.go:73`, `internal/config/world.go:41` |
| Walk batches unknown siblings, checks cancellation and never opens rejected directories | Walk/walk, `internal/workspace/walk.go:18`, `internal/workspace/walk.go:46` |
| Files returns a sorted copy; new files appear after refresh; fallback lists are cached until refresh | Files, `internal/workspace/membership.go:335` |
| Admit translates rejected directories to SkipDir and rejected files to skipped callbacks, including relative and aliased-root paths | Gate/Admit/relOf, `internal/workspace/membership.go:391`, `internal/workspace/membership.go:415`, `internal/workspace/membership.go:429` |

The full caller census is [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md). Default scanners read scanned-root patterns; configured scanners use supplied patterns (NewScanner/membership, `internal/world/fs.go:33`, `internal/world/fs.go:41`). Factory wiring already supplies loaded patterns (initFinalExecutors, `internal/system/factory.go:2239`).

Campaign briefing/snapshot/rollback use campaign-root membership (writeSetBriefing/snapshotDirectoryFiles/rollbackTaskExecutionSnapshot, `internal/campaign/orchestrator_task_results.go:217`, `internal/campaign/orchestrator_task_transaction.go:388`, `internal/campaign/orchestrator_task_transaction.go:487`). Init gates direct reads and module subtree walks against the initializer root (detectEntryPointsForRoot, `internal/init/scanner.go:264`).

## Evidence boundary

Workspace, projectdoc, gates and Mangle tests passed in the requested native test run and in a supplemental CGO-disabled run, with repository-contained temp directories. Membership witnesses include TestMembershipGit, TestWalkNeverEntersBigIgnoredDir and the whitespace/concurrency/alias regressions (`internal/workspace/membership_test.go:43`, `internal/workspace/membership_test.go:162`, `internal/workspace/membership_regression_test.go:12`, `internal/workspace/membership_regression_test.go:40`, `internal/workspace/membership_regression_test.go:70`).

The required sqlite-vec build cannot execute the configured clang compiler. CGO-dependent callers have no passing result. Policy guards report both concurrent wiring failures and a literal-budget baseline reduction (chat 181 versus 183 after skip-list removal); GAP-WS-06 retains both obligations. Campaign/init/retrieval regressions are authored, not passing evidence (`internal/campaign/workspace_membership_test.go:10`, `internal/init/scanner_membership_test.go:10`, `internal/retrieval/backend_membership_test.go:11`).

Treatment and orientation-artifact ingestion remain planned (GAP-WS-05).