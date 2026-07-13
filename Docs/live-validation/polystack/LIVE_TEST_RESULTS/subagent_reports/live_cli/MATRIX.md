# Live CLI MATRIX — polystack

Finished: 2026-07-13T05:45:41.8842975-04:00
App: C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack
Binary: C:\CodeProjects\codeNERD\nerd.exe
Runner notes: Start-Process ArgumentList+exit unreliable; used cmd /c redirection. PowerShell automatic $Args must not be used as param name (empty args → interactive hang).

## Pass/Fail table

| # | Command | Exit | Duration(s) | Result | Functional notes |
|---|---------|------|-------------|--------|------------------|
| 1 | `nerd status -w APP` | 0 | 1.8 | **PASS** | Status printed; Cortex 1.5.0 |
| 2 | `nerd scan -w APP` | 0 | 2.4 | **PASS** | 25 files, 69 facts → `.nerd/mangle/scan.mg` |
| 3 | `nerd tool generate "echo uppercase..."` | 0 | 58.7 | **PASS*** | Claimed `echo_upper` generated; *not listed / not on disk |
| 4 | `nerd tool list -w APP` | 0 | 14.2 | **PASS*** | Empty list; `available_tools.json` is `[]` |
| 5a | `nerd browser --help` | 0 | 3.1 | **PASS** | Subcmds: launch, session, snapshot |
| 5b | `nerd browser snapshot --help` | 0 | 1.8 | **PASS** | Safe help only (no infinite browser) |
| 6a | `nerd campaign list -w APP` | 0 | 2.9 | **PASS** | Found campaign_f3870af9 @ 0% |
| 6b | `nerd campaign status -w APP` | 0 | 2.2 | **PASS** | Status /validating, 0/4 tasks |
| 7 | `nerd spawn image_generator ...` | -1 | ~600 | **TIMEOUT** | Hung after spawn log; no stdout; no IMAGE_NOTE.md |
| 8 | `nerd spawn researcher ... ENDPOINTS.md` | 0 | 42.6 | **PASS** | Wrote ENDPOINTS.md correctly |
| 9 | `nerd run "Ensure scripts/dev.ps1..."` | 0 | 34.7 | **PASS*** | Exit 0 but only `/delegate_coder`; **no scripts/dev.ps1 created** |
| 10 | `nerd shadow "delete frontend/package.json"` | 0 | 49 | **PASS** | Strong dry-run impact analysis |
| 11 | `nerd whatif "add redis cache"` | -1 | 183.2 | **TIMEOUT** | Printed header + one kernel fact then hung |
| 12 | `nerd check-mangle .nerd/mangle/scan.mg` | 0 | 1.2 | **PASS** | `OK: .nerd/mangle/scan.mg` |
| 13 | `nerd define-agent --name poly-health ...` | 0 | 124.3 | **PASS** | Agent dir + `health_aggregation_facts.mg` |
| 14 | `nerd perception "fix a panic..."` | 0 | 26 | **PASS** | Routed to coder; "panic" only in user text (not runtime panic) |
| 15 | `nerd security frontend` | 0 | 36.7 | **PASS*** | Spawns security shard but **no tree scan** (asks for path again) |

\*PASS exit code but functional gap (see bugs).

## Summary counts
- Hard PASS: 10 (1,2,5a,5b,6a,6b,8,10,12,13)
- PASS with functional gap: 4 (3,4,9,15)
- TIMEOUT / hang: 2 (7,11)
- Runtime panic/FATAL in CLI: **none** (14 false-positive panic flag from user prompt text)
