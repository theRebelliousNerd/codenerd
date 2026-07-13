# LIVE_TEST_RESULTS

**Purpose:** Durable, in-workspace evidence from codeNERD live validation runs (2026-07).

Previously these artifacts were only under `%TEMP%\codenerd-*` session scratch. That was wrong for long-term auditability — they now live next to the polystack vehicle.

## Layout

| Subdir | Contents |
|--------|----------|
| `campaign_marathon/` | SuperGrok campaign start/resume stdout/stderr, `MARATHON_REPORT.md`, meta |
| `cli_matrix/` | Feature matrix `MATRIX.md`, app `SPEC.md`, summary |
| `subagent_reports/` | Parallel agent reports (gap audit, SuperGrok fix, hollow success, image routing, …) |
| `campaigns_snapshot/` | Copy of campaign JSON at collection time |

## How to add a new run

1. Create a dated folder: `LIVE_TEST_RESULTS/runs/YYYY-MM-DD_<name>/`
2. Drop `MATRIX.md` or `REPORT.md` + key `.out` logs
3. One-line entry in `INDEX.md` (create if missing)
4. Never store API keys or full `config.json` here

## Status snapshot (at promotion from TEMP)

- SuperGrok: engine `xai-oauth` probe ready (see auth fixes on main)
- Campaign `campaign_eae63fd5`: multi-phase polystack hardening; active mid-run when collected
- CLI matrix + subagent swarm reports: present under `subagent_reports/`

Parent marker: `../AGENTS.md`
