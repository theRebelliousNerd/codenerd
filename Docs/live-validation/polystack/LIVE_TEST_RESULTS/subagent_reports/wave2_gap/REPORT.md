# Wave 2 Gap Live CLI Report

**Date:** 2026-07-13  
**Repo:** `C:\CodeProjects\codeNERD`  
**Binary:** `nerd.exe` (present, built)  
**Workspace APP:** `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack`  
**Evidence:** `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\wave2_gap\`  
**Constraint:** Serial only; `taskkill /F /IM nerd.exe` between each command.

---

## Executive summary

Second-wave gap probes exercised 12 CLI surfaces. Most **status/query** commands do **real work** (not stubs). Hard failures: **`why` mode declarations**, **campaign pause/resume consistency**, and **northstar missing data** on APP. Help-only validated for **browser session** and **commit** (no monorepo commit performed). Suspected **memory vs embedding stats** inconsistency (465 vs 0).

---

## Per-command detail

### 1. `nerd auth status` — PASS / real work

```
Current engine: api
Backend: HTTP API
  Provider: zai
  Model: glm-4.7
```

Exit 0, ~1.2s. Global (no `-w` needed). Auth path live.

Evidence: `01_auth_status.txt`

---

### 2. `nerd memory status -w APP` — PASS / real work

```
RAM (Working Memory):  0 facts
Vector (Embeddings):   465 entries
Graph (Relationships): 0 edges
Cold (Long-term):      0 entries
Compressed Contexts:   0
```

Exit 0, ~4.2s. Real tier read against APP workspace.

Evidence: `02_memory_status.txt`

---

### 3. `nerd autopoiesis status -w APP` — PASS / real work

```
Ouroboros Loop:    0 tools generated
Prompt Evolution:  0 evolutions
Learning Store:    0 patterns
Thunderdome:       0 battles
Orchestrator: Active
```

Exit 0, ~4.5s. Counters empty but status pipeline live.

Evidence: `03_autopoiesis_status.txt`

---

### 4. `nerd northstar show -w APP` — FAIL (data) / real work attempt

Also tried: `northstar facts`, `northstar summary`.

```
Error: northstar not defined - run '/northstar' in interactive mode or 'nerd northstar wizard'
```

facts: `northstar.mg not found`

Exit 1. CLI wiring present; APP has no northstar definition. Not a help-only stub.

Evidence: `04_northstar_show.txt`, `04b_northstar_facts.txt`, `04c_northstar_summary.txt`

---

### 5. `nerd query next_action -w APP` — PASS (empty) / real work

```
No facts found for predicate 'next_action'
```

Exit 0, ~5s. Kernel query path works; no derived `next_action` in current APP state (expected outside OODA turn).

Evidence: `05_query_next_action.txt`

---

### 6. `nerd why next_action -w APP` — FAIL / real work broken

```
Explaining derivation for: next_action
Tracing query: next_action(Var0)
Error: trace failed: predicate next_action has no modes declared
```

Also `why blocked` fails parse: `mismatched input '.' expecting '('`.

Exit 1. Glass-box surface exists but tracing is broken for these predicates.

Evidence: `06_why_next_action.txt`, `06b_why_blocked.txt`

---

### 7. Campaign pause → resume — PARTIAL / inconsistent

**pause** (exit 0):
```
Campaign paused. Run 'nerd campaign resume' to continue.
```

**resume** (exit 0):
```
No paused campaigns found.
```

**status** (before/after):
- Campaign: `Polystack README Structured Logging Notes` (`/campaign_f3870af9`)
- Status: **`/validating`** (never showed paused in status)
- Progress 0%, Tasks 0/4, Phases 0/2

**list** briefly showed pause emoji after pause, but resume cannot find paused campaigns. Pause appears to be a **message-only or wrong-state transition**.

Evidence: `07a_campaign_pause.txt`, `07b_campaign_resume.txt`, `07c_campaign_status.txt`, `07d_campaign_list.txt`, `07e_campaign_status_after_resume.txt`

---

### 8. `nerd tool list` / `tool info` — PASS empty / real work

**list** (exit 0):
```
No tools generated yet.
```

**info** without name (exit 1):
```
usage: nerd tool info <name>
```

Empty Ouroboros inventory is consistent with autopoiesis (0 tools). `info` correctly requires a name — documented empty, not help-only for list.

Evidence: `08a_tool_list.txt`, `08b_tool_info.txt`

---

### 9. `nerd embedding stats -w APP` — PASS / real work (+ skew)

```
Embedding Statistics:
  Total Vectors: 0
  With Embeddings: 0
  Without Embeddings: 0
  Engine: ollama:embeddinggemma:300m
  Dimensions: 768
```

Exit 0. **Skew:** memory status reported **465** vector entries; embedding stats **0**. Likely different DB path, count definition, or knowledge.db vs memory store split — flag as gap.

Evidence: `09_embedding_stats.txt` (compare `02_memory_status.txt`)

---

### 10. `nerd sessions` / `sessions list -w APP` — PASS empty / real work

```
No saved sessions found.
```

Exit 0. List path works; APP has no saved sessions. `load` not exercised (nothing to load).

Evidence: `10a_sessions.txt`, `10b_sessions_list.txt`

---

### 11. `nerd browser session --help` — PASS / help-only

Subcommands: `launch`, `session [url]`, `snapshot`.  
No `browser list` subcommand exists. Help-only as instructed (did not launch browser).

Evidence: `11a_browser_session_help.txt`, `11b_browser_help.txt`

---

### 12. `nerd commit --help` — PASS / help-only

```
Usage: nerd commit <message> [flags]
```

No `--dry-run` flag in help. **Did not** run an actual commit against the monorepo.

Evidence: `12_commit_help.txt`

---

## Gap ranking (actionable)

| Priority | Issue | Impact |
|----------|--------|--------|
| P0 | Campaign pause/resume state machine | Long-horizon control unreliable |
| P0 | `why` / mode declarations for core predicates | Glass-box explainability broken |
| P1 | Memory (465) vs embedding stats (0) skew | Ops/diagnostics misleading |
| P1 | Northstar missing on polystack APP | Strategic context unavailable for campaigns |
| P2 | `commit` lacks `--dry-run` | Unsafe CLI for automation |
| P2 | No browser session inventory | Hard to manage live browser state |
| P3 | Empty tools/sessions/next_action | Environmental, not necessarily product bugs |

---

## Method notes

- All runs sequential; process killed between tests.
- Stdout/stderr captured per command under evidence dir.
- Help text for all targets saved as `help_*.txt`.
- No concurrent `nerd.exe`; no monorepo commit; no browser launch; no tool generate.
