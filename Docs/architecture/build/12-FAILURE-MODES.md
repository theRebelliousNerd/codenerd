# 12 — Failure Modes: `internal/build`

> Last verified: **2026-08-15**  
> Concrete failures tied to this package’s behavior and its callers.

---

## FM-01 — GOCACHE undefined in subprocess

| | |
|--|--|
| **Symptom** | `go: GOCACHE is not defined` or cache-related fatal from toolchain |
| **Cause** | Parent env lacks GOCACHE and all of LOCALAPPDATA/USERPROFILE/HOME/TEMP/TMP/TMPDIR empty |
| **Package behavior** | `deriveGOCACHE` returns `""`; no GOCACHE entry |
| **Mitigation** | Ensure HOME or LOCALAPPDATA exists for agent process; or set GOCACHE in Build.EnvVars |
| **Detection** | BuildDebug “Derived GOCACHE” absent; final env missing key |

---

## FM-02 — Missing CGO_CFLAGS for host project build

| | |
|--|--|
| **Symptom** | CGO compile fails finding `sqlite3.h` |
| **Cause** | `workspaceRoot` does not contain `sqlite_headers`; config lacks CGO_CFLAGS; detect finds nothing |
| **Package behavior** | Silent omission of CGO_CFLAGS |
| **Mitigation** | Pass real monorepo root; create `sqlite_headers`; set `UserConfig.Build.EnvVars` |
| **Note** | Root Agents.md manual export still works for human shells outside this package |

---

## FM-03 — Auto sqlite_vec tag not applied

| | |
|--|--|
| **Symptom** | Binary lacks sqlite-vec features; tests skip vec paths |
| **Cause** | Process already has `GOFLAGS`; or user config set custom GOFLAGS; or headers dir missing |
| **Package behavior** | Skips injecting `-tags=sqlite_vec` |
| **Mitigation** | Clear ambient GOFLAGS or set desired tags explicitly in Build.EnvVars |

---

## FM-04 — Config appears ignored

| | |
|--|--|
| **Symptom** | `.nerd` build.env_vars do not affect Ouroboros/Thunderdome compiles |
| **Cause** | Callers pass `userCfg=nil` |
| **Package behavior** | Correctly skips config when nil |
| **Mitigation** | Thread UserConfig into ToolCompiler/Thunderdome; treat as wiring gap G-02 |

---

## FM-05 — Headers not detected for tool sandbox builds

| | |
|--|--|
| **Symptom** | Expectation that tool compile gets monorepo CGO flags |
| **Cause** | `workspaceRoot` argument is tmp/arena without headers; CGO also forced off |
| **Package behavior** | No headers; CGO disabled by MergeEnv |
| **Mitigation** | **Usually desired** for portable tools. If CGO tools are required, redesign: pass workspace root, do not force CGO_ENABLED=0, and vendor headers carefully |

---

## FM-06 — Duplicate environment keys

| | |
|--|--|
| **Symptom** | Hard-to-debug override (wrong GOPATH/GOFLAGS) |
| **Cause** | Base includes process GOFLAGS; whitelist re-appends; Build.EnvVars appends again without setEnvKey |
| **Package behavior** | Multiple `KEY=` entries possible |
| **Mitigation** | Prefer `MergeEnv`/`setEnvKey` in a future normalize step; callers inspect full env |
| **Go runtime note** | Typically last occurrence wins in process env |

---

## FM-07 — Cross-compile still uses host CGO toolchain

| | |
|--|--|
| **Symptom** | Cross `GOOS`/`GOARCH` build fails or links wrong |
| **Cause** | GetBuildEnvForCompile only sets GOOS/GOARCH; does not set CC/CXX; autopoiesis forces CGO=0 |
| **Package behavior** | By design limited |
| **Mitigation** | For true CGO cross-compile, set CC in Build.EnvVars and do not force CGO off |

---

## FM-08 — PATH missing → go not found

| | |
|--|--|
| **Symptom** | `executable file not found in %PATH%` |
| **Cause** | Parent process has empty PATH (stripped agent env) |
| **Package behavior** | Only copies PATH if set; cannot invent go location |
| **Mitigation** | Ensure agent process PATH includes Go toolchain |

---

## FM-09 — Relative workspace root resolves wrong

| | |
|--|--|
| **Symptom** | Headers not found though they exist “here” |
| **Cause** | Relative root + unexpected process CWD; Abs may resolve differently |
| **Package behavior** | Attempts Abs; on failure uses original string |
| **Mitigation** | Callers pass absolute workspace paths |

---

## FM-10 — MergeEnv silent skip of bad entries

| | |
|--|--|
| **Symptom** | Expected overlay missing (`CGO_ENABLED0` typo without `=`) |
| **Cause** | Entry lacks `=` |
| **Package behavior** | Skip without log |
| **Mitigation** | Always pass `KEY=value`; add tests when adding overlays |

---

## FM-11 — Dead GoFlags expectation

| | |
|--|--|
| **Symptom** | Config `go_flags: ["-race"]` has no effect |
| **Cause** | ~~Package never applies GoFlags to argv~~ — **fixed 2026-08-15**: `AppendGoFlags(userCfg, root, args)` injects them after the subcommand. Still a failure mode for any caller that builds argv by hand instead of calling it. |
| **Mitigation** | Put flags on command line at caller, or use GOFLAGS env via EnvVars |

---

## FM-12 — Thunderdome / forge compile failure (downstream)

| | |
|--|--|
| **Symptom** | Arena binary or tool binary fails to compile |
| **Cause** | Often invalid generated Go, timeout, missing go.mod — not env |
| **Package role** | Env may be fine; check autopoiesis stderr |
| **Mitigation** | Inspect CombinedOutput; AutopoiesisDebug compile logs |

---

## Failure mode summary table

| ID | Severity (typical) | In package? | Silent? |
|----|--------------------|-------------|---------|
| FM-01 | High for CI agents | Partial | Yes until go fails |
| FM-02 | High for host CGO | Yes | Yes |
| FM-03 | Medium | Yes | Yes |
| FM-04 | Medium | Wiring | Yes |
| FM-05 | Low if CGO=0 intended | Wiring | Yes |
| FM-06 | Medium | Yes | Yes |
| FM-07 | Medium | Design limit | — |
| FM-08 | High | Environmental | Until exec |
| FM-09 | Medium | Caller | Yes |
| FM-10 | Low | Yes | Yes |
| FM-11 | Low/Medium | Design gap | Yes |
| FM-12 | Variable | Downstream | No (stderr) |
