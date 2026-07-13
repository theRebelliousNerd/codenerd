# Live CLI Stress REPORT — polystack

- **When:** 2026-07-13T05:45:41.8842975-04:00
- **App vehicle:** `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack`
- **Binary:** `C:\CodeProjects\codeNERD\nerd.exe` (Cortex 1.5.0)
- **Evidence dir:** `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\live_cli`
- **LLM:** engine=api provider=xai (config); image model gemini-3.1-flash-image assumed present
- **Execution:** serial only; `taskkill /F /IM nerd.exe` between steps

## Pass/Fail matrix

See [MATRIX.md](MATRIX.md) for the full table.

| Step | Result |
|------|--------|
| 1 status | PASS |
| 2 scan | PASS |
| 3 tool generate | PASS* (claimed create, not persisted) |
| 4 tool list | PASS* (empty after generate) |
| 5 browser help (+ snapshot help) | PASS |
| 6 campaign list/status | PASS |
| 7 spawn image_generator | **TIMEOUT/HANG** |
| 8 spawn researcher | PASS (ENDPOINTS.md) |
| 9 run dev.ps1 | PASS* (no file written) |
| 10 shadow | PASS |
| 11 whatif | **TIMEOUT/HANG** |
| 12 check-mangle | PASS |
| 13 define-agent poly-health | PASS |
| 14 perception | PASS (no real panic) |
| 15 security frontend | PASS* (no actual audit of frontend/) |

**Headline:** 13/15 exited or timed under harness control without process panic; **2 hangs**; **several silent functional bugs** on tool persistence, `nerd run` not completing file create, and security not targeting workspace `frontend/`.

## Bugs found

### BUG-1: `nerd tool generate` success not reflected in `tool list` / disk
- Generate printed: `Generated self-tool echo_upper...` (exit 0, ~59s).
- Immediately after, `nerd tool list` → "No tools generated yet."
- `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack\.nerd\tools\available_tools.json` is `[]`; no `echo_upper` file under app or repo search.
- **Severity:** High — Ouroboros self-tool loop appears non-durable.

### BUG-2: `nerd spawn image_generator` hang
- Logged spawn start on stderr JSON; **0 bytes stdout** for ~10+ minutes; CPU nearly flat.
- No `IMAGE_NOTE.md` written; taskkill required.
- Shard type `image_generator` is **not listed** in `nerd spawn --help` (help lists generalist/specialist/coder/researcher/reviewer/tester only).
- **Severity:** High — hang + undocumented/invalid shard type may not fail fast.

### BUG-3: `nerd whatif` hangs after first kernel line
- Prints What-If header and `derives_from_hypothetical("add redis cache").` then stalls until 180s kill.
- No further analysis, no clean exit.
- **Severity:** High — incomplete feature / stuck LLM or kernel path.

### BUG-4: `nerd run` reports success without fulfilling mutation
- Instruction: ensure `scripts/dev.ps1` exists documenting 4 services.
- Intent correctly `/create` target `dev.ps1`; derived `next_action(/delegate_coder)`.
- Surface: `Result: Next action: /delegate_coder` and exit 0.
- **File never created** (`scripts/` missing).
- **Severity:** High — CLI instruction mode does not complete coder delegation end-to-end.

### BUG-5: `nerd security frontend` does not use workspace target
- Exit 0, spawns security shard, but response says tools unavailable / asks for frontend path again despite `frontend` arg and `-w APP` containing `frontend/`.
- **Severity:** Medium — command UX / tool wiring gap.

### BUG-6: Perception "panic" false-positive in harness only
- Exit 0, correct routing to coder. Matrix `panic=True` is from user prompt substring `"fix a panic..."`, not a Go runtime panic.
- **No real panic/FATAL** observed across steps.

### Harness note (not product bug)
- PowerShell automatic variable `` as a function parameter silently drops real argv → launches interactive chat (hangs after boot). Fixed by renaming to `` / using cmd arg strings.

## Artifacts created (this run)

### Evidence dir (`C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\live_cli`)
- `MATRIX.md`, `REPORT.md`, `run_remaining.ps1`
- Per-step: `NN_*.out`, `NN_*.err`, `NN_*.meta`, `NN_*.ec` for steps 01–15 (+ 05a/b, 06a/b/c, 07pre)

### App workspace (non-`.nerd`)
Created/updated by live run:
- **`ENDPOINTS.md`** (NEW, researcher) — 10-line API summary for Go backend :8080

Pre-existing non-`.nerd` tree (not created by this matrix; listed for inventory):
- `README.md`, `SPEC.md`
- hang probes: `hang3.txt`, `hangprobe.txt`, `hangprobe2.txt`, `hang_residual_probe.txt`
- `backend/`: `main.go`, `main_test.go`, `go.mod`, `polystack-backend.exe`
- `frontend/`: `package.json`, `package-lock.json`, `vite.config.ts`, `tsconfig*.json`, `index.html`, `src/*`, `dist/`, `node_modules/`
- `sidecars/python/`: `main.py`, `requirements.txt`
- `sidecars/rust/`: `Cargo.toml`, `Cargo.lock`, `src/main.rs`, `target/` (debug/release builds)

### App `.nerd` changes of interest
- `.nerd/mangle/scan.mg` — from scan (step 2)
- `.nerd/agents/poly-health/` + `health_aggregation_facts.mg` — from define-agent (step 13)
- `.nerd/tools/available_tools.json` remains `[]` (tool generate did not stick)

### Not created (expected by steps)
- `IMAGE_NOTE.md` (step 7 hang)
- `scripts/dev.ps1` (step 9 incomplete)

## Panic / FATAL
- **None** in process output across all steps.
- Step 14 contains the word "panic" only as user intent text.

## Command evidence paths
All under: `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\live_cli`

| Step | out | err | meta |
|------|-----|-----|------|
| 01 | 01_status.out | 01_status.err | 01_status.meta |
| 02 | 02_scan.out | 02_scan.err | 02_scan.meta |
| 03 | 03_tool_generate.out | 03_tool_generate.err | 03_tool_generate.meta |
| 04 | 04_tool_list.out | 04_tool_list.err | 04_tool_list.meta |
| 05 | 05a_browser_help.out, 05b_browser_snapshot_help.out | … | … |
| 06 | 06a_campaign_list.out, 06b_campaign_status.out | … | … |
| 07 | 07_spawn_image.out (empty) | 07_spawn_image.err | 07_spawn_image.meta |
| 08–15 | 08_… through 15_security.* | | |

## Recommendations (brief)
1. Fail-fast unknown spawn shard types (`image_generator`); wire image path or document.
2. Fix whatif completion / timeout so it exits after partial kernel derivation.
3. Persist Ouroboros tools into workspace `.nerd/tools` and make list read same store.
4. `nerd run` / instruction path: either execute `/delegate_coder` fully or exit non-zero when file not written.
5. `nerd security <path>` should resolve relative to `-w` without re-prompting.
