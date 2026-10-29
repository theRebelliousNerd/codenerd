# Wave 2 Gap CLI Matrix — Live Tests

| # | Command | Exit | Result | Real work vs help-only | Notes |
|---|---------|------|--------|------------------------|-------|
| 1 | `nerd auth status` | 0 | **PASS** | **Real work** | Engine `api`, provider `zai`, model `glm-4.7` |
| 2 | `nerd memory status -w APP` | 0 | **PASS** | **Real work** | RAM 0 / Vector **465** / Graph 0 / Cold 0 |
| 3 | `nerd autopoiesis status -w APP` | 0 | **PASS** | **Real work** | All counters 0; Orchestrator Active |
| 4 | `nerd northstar show -w APP` | 1 | **FAIL** (data) | **Real work** (attempt) | `northstar not defined`; facts/summary same |
| 5 | `nerd query next_action -w APP` | 0 | **PASS** (empty) | **Real work** | `No facts found for predicate 'next_action'` |
| 6 | `nerd why next_action -w APP` | 1 | **FAIL** | **Real work** (broken) | `predicate next_action has no modes declared` |
| 7a | `nerd campaign pause -w APP` | 0 | **PASS?** | **Partial / suspect** | Prints "Campaign paused"; status still `/validating` |
| 7b | `nerd campaign resume -w APP` | 0 | **FAIL** (logic) | **Partial / broken** | `No paused campaigns found` after pause claimed success |
| 8 | `nerd tool list` / `tool info` | 0 / 1 | **PASS** (list empty) | **Real work** | List empty; `info` needs name (usage error expected) |
| 9 | `nerd embedding stats -w APP` | 0 | **PASS** | **Real work** | 0 vectors; engine `ollama:embeddinggemma:300m` dim 768 |
| 10 | `nerd sessions` / `sessions list -w APP` | 0 | **PASS** (empty) | **Real work** | `No saved sessions found` |
| 11 | `nerd browser session --help` | 0 | **PASS** | **Help-only** | Subcmds: launch, session, snapshot (no list) |
| 12 | `nerd commit --help` | 0 | **PASS** | **Help-only** | No dry-run flag; did **not** commit monorepo |

**Workspace (APP):** `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack`  
**Binary:** `C:\CodeProjects\codeNERD\nerd.exe`  
**Evidence dir:** `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\wave2_gap\`  
**Mode:** Serial only; `taskkill /F /IM nerd.exe` between each invocation.

## Scorecard

| Category | Count |
|----------|------:|
| PASS (real work, healthy) | 6 |
| PASS (empty but functional) | 3 |
| PASS (help-only as requested) | 2 |
| FAIL / broken / suspect | 4 (northstar data, why modes, campaign pause/resume pair) |
| **Commands exercised** | **12 primary + diagnostics** |

## Notable gaps / bugs

1. **Campaign pause/resume mismatch** — pause claims success; resume finds nothing; status remains `/validating`. List briefly showed pause emoji but resume is a no-op.
2. **`why next_action`** — command surface exists but fails: `no modes declared` on `next_action`.
3. **Northstar unconfigured** on polystack APP — CLI works; data missing (`northstar.mg` not found).
4. **Memory vs embedding stats skew** — memory reports **465** vector entries; embedding stats reports **0** total vectors (possible dual DB / path mismatch).
5. **No browser list** — only launch/session/snapshot; no session inventory subcommand.
6. **`commit` has no `--dry-run`** — help-only validated; unsafe for monorepo without that flag.
