# CLI Command Gap Analysis — codeNERD Live Matrix

**Generated:** 2026-07-13  
**Sources:** `cmd/nerd/*.go` (cobra registration in `main.go` + cmd modules)  
**Evidence:** `%TEMP%\codenerd-live-matrix\MATRIX.md` phases 00–04 + `SUMMARY.md`  
**Workspace under test:** `C:\CodeProjects\codeNERD\.nerd\live_feature_matrix\polystack`

## Legend

| Class | Meaning |
|-------|---------|
| **exercised** | Invoked with meaningful args; RunE body ran; useful output or real work (PASS, hang-after-result, or empty-but-valid result all count) |
| **partial** | Parent help only, missing required args, wrong invocation, only one of several subcommands, or FAIL before useful work |
| **not** | Never appeared in MATRIX / harness |

**Exit hygiene note:** Several “PASS” LLM/direct-action runs still leave the process hung after printing Result (cortex.Close/maintenance). That is an exit-path bug, not “unexercised.”

---

## 1. Full cobra tree (registered)

Root: `nerd` (no args → interactive chat TUI)

| Command path | Kind | Matrix class | Matrix evidence | Notes |
|--------------|------|--------------|-----------------|-------|
| `nerd` (interactive) | TUI default | **not** | SUMMARY: TUI not proven | No Bubbletea smoke |
| `nerd run [instruction]` | direct OODA | **exercised** | `04_run_makefile` PASS+hang | |
| `nerd define-agent --name --topic` | agent | **partial** | `04_define_agent` FAIL | Missing required flags |
| `nerd spawn [type] [task]` | agent | **exercised** | `03_spawn_tester` PASS | |
| `nerd browser` | parent help | **partial** | `04_browser` help only | |
| `nerd browser launch` | sub | **not** | — | |
| `nerd browser session [url]` | sub | **not** | — | |
| `nerd browser snapshot [id]` | sub | **not** | — | |
| `nerd query [predicate]` | kernel | **partial** | `00_query` | Ran `permitted(A).` → no facts; syntax/form weak |
| `nerd status` | diag | **exercised** | `00_status` PASS | |
| `nerd init` | workspace | **exercised** | `01_init` PASS | |
| `nerd scan` | index | **exercised** | `01_scan`, `03_scan` PASS | |
| `nerd why [predicate]` | glassbox | **partial** | `00_why` | Pre-init “not initialized”; never re-run with predicate post-init |
| `nerd campaign` | parent | — | — | via subcommands |
| `nerd campaign start [goal]` | campaign | **exercised** | `04_campaign_start` PASS+hang | Start only; not full multi-phase |
| `nerd campaign status` | campaign | **exercised** | `04_campaign_status` PASS | |
| `nerd campaign list` | campaign | **exercised** | `00_campaign_list` PASS | |
| `nerd campaign pause` | campaign | **not** | — | |
| `nerd campaign resume` | campaign | **not** | — | |
| `nerd check-mangle [file...]` | mangle | **exercised** | `04_check_mangle` PASS | `00_*` FAIL no args |
| `nerd mangle-lsp` | LSP | **not** | — | Long-running server |
| `nerd auth` | parent | **not** | SUMMARY env note only | |
| `nerd auth grok` | auth | **not** | — | OAuth refresh known broken |
| `nerd auth claude` | auth | **not** | — | |
| `nerd auth codex` | auth | **not** | — | |
| `nerd auth status` | auth | **not** | — | High value, read-only |
| `nerd logs` | diag | **exercised** | `00_logs` PASS | |
| `nerd review <target>` | direct | **exercised** | `03_review` PASS | |
| `nerd fix <target>` | direct | **exercised** | `04_fix_typo` PASS+hang | |
| `nerd test <target>` | direct | **exercised** | `04_test_cmd` PASS+hang | |
| `nerd push [remote] [branch]` | git | **not** | — | |
| `nerd commit <message>` | git | **not** | — | |
| `nerd explain <target>` | direct | **exercised** | `03_explain` PASS | |
| `nerd create <description>` | direct | **exercised** | phase 02/02b/04 | Polyglot + hang probe |
| `nerd refactor <target>` | direct | **exercised** | `04_refactor_comment` PASS | |
| `nerd perception <input>` | perception | **exercised** | `04_perception` PASS; `00_*` TIMEOUT | |
| `nerd security <target>` | direct | **exercised** | `03_security` PASS | |
| `nerd analyze <target>` | direct | **exercised** | `03_analyze` PASS | |
| `nerd dream <scenario>` | advanced | **exercised** | `04_dream` PASS+hang | Not multi-turn marathon |
| `nerd shadow <action>` | advanced | **exercised** | `03_shadow` PASS | |
| `nerd whatif <change>` | advanced | **exercised** | `03_whatif` PASS | |
| `nerd logic [predicate]` | kernel | **exercised** | `00_logic` PASS | |
| `nerd agents` | diag | **exercised** | `00_agents` listed agents; exit hang FAIL | |
| `nerd tool list` | ouroboros | **exercised** | `04_tool_list` PASS (empty tools) | |
| `nerd tool run` | ouroboros | **not** | — | |
| `nerd tool info` | ouroboros | **not** | — | |
| `nerd tool generate` | ouroboros | **not** | — | SUMMARY gap |
| `nerd jit` | prompt | **exercised** | `00_jit` PASS | |
| `nerd dom` | parent help | **partial** | `04_dom` help only | |
| `nerd dom demo` | dom | **not** | — | Self-contained smoke |
| `nerd dom inspect <file>` | dom | **not** | — | |
| `nerd dom get <file> <ref>` | dom | **not** | — | |
| `nerd dom edit <file> <ref>` | dom | **not** | — | |
| `nerd dom apply <file> <plan>` | dom | **not** | — | |
| `nerd dom replace` | dom | **not** | — | |
| `nerd embedding stats` | embed | **exercised** | `00_embedding_stats` PASS | |
| `nerd embedding set` | embed | **not** | — | |
| `nerd embedding reembed` | embed | **not** | — | |
| `nerd northstar` | parent help | **partial** | `01_northstar` help only | |
| `nerd northstar show` | planning | **not** | — | |
| `nerd northstar summary` | planning | **not** | — | |
| `nerd northstar query` | planning | **not** | — | |
| `nerd northstar facts` | planning | **not** | — | |
| `nerd northstar export` | planning | **not** | — | |
| `nerd northstar stats` | planning | **not** | — | |
| `nerd mcp` | parent help | **partial** | `00_mcp` help | |
| `nerd mcp list` | mcp | **exercised** | `04_mcp_list` PASS (none connected) | |
| `nerd mcp tools` | mcp | **not** | — | |
| `nerd mcp status` | mcp | **not** | — | |
| `nerd autopoiesis` | parent help | **partial** | `00_autopoiesis` help | |
| `nerd autopoiesis status` | system | **not** | — | |
| `nerd autopoiesis learning` | system | **not** | — | |
| `nerd autopoiesis tools` | system | **not** | — | |
| `nerd memory` | parent help | **partial** | `00_memory` help | |
| `nerd memory status` | system | **not** | — | Only real subcommand |
| `nerd sessions` / `sessions list` | session | **exercised** | `00_sessions` PASS (none) | Default RunE = list |
| `nerd sessions load <id>` | session | **not** | — | Needs prior chat session |
| `nerd knowledge` / `knowledge list` | kb | **exercised** | `00_knowledge` empty list; `04_knowledge` FAIL empty | List path hit |
| `nerd knowledge search <q>` | kb | **not** | — | |
| `nerd glassbox` | introspect | **exercised** | `00_glassbox` PASS | |
| `nerd transparency` | introspect | **exercised** | `00_transparency` PASS | |
| `nerd reflection` | introspect | **exercised** | `00_reflection` PASS | |
| `nerd test-context` | harness | **partial** | `00_test_context` FAIL | Type assert CortexKernel vs RealKernel |

---

## 2. Rollup counts (leaf / invocable paths)

Counting invocable leaves (parent-only help paths counted once if they have no useful default RunE).

| Class | Approx count | Share |
|-------|--------------|-------|
| **exercised** | ~32 | ~45% |
| **partial** | ~12 | ~17% |
| **not** | ~27 | ~38% |
| **Total registered leaves+parents** | ~71 | |

Top-level registered root commands: **44** (+ default interactive).  
Of those top-level names, **~28** had at least one matrix row; **~16** top-level never meaningfully exercised beyond help or at all.

### Top-level never exercised (or help-only without real sub)

| Top-level | Status |
|-----------|--------|
| `auth` | **not** (all subcommands) |
| `push` | **not** |
| `commit` | **not** |
| `mangle-lsp` | **not** |
| `browser` | **partial** (help only) |
| `dom` | **partial** (help only) |
| `northstar` | **partial** (help only) |
| `autopoiesis` | **partial** (help only) |
| `memory` | **partial** (help only) |
| `define-agent` | **partial** (flag error) |
| `test-context` | **partial** (kernel type bug) |
| interactive `nerd` | **not** |

### Fully strong exercised top-level (real work)

`status`, `init`, `scan`, `create`, `spawn`, `analyze`, `explain`, `review`, `shadow`, `security`, `whatif`, `fix`, `test`, `refactor`, `run`, `dream`, `perception`, `logic`, `agents`, `jit`, `glassbox`, `transparency`, `reflection`, `logs`, `embedding stats`, `sessions` (list), `knowledge` (list), `campaign` (list/start/status), `check-mangle` (with file), `tool list`, `mcp list`

---

## 3. Partial inventory (what is missing)

| Path | Why partial | Fix for next wave |
|------|-------------|-------------------|
| `browser` | Parent help only | `launch` → `session <url>` → `snapshot` |
| `dom` | Parent help only | `dom demo` first; then `inspect` on polystack Go file |
| `northstar` | Parent help only | `show`, `facts`, `summary` after project has northstar |
| `mcp` / `autopoiesis` / `memory` | Parent help in phase 00 | Call real subcommands |
| `query` | Weak predicate | `query next_action`, `query permitted` |
| `why` | Pre-init only | `why next_action` post-init |
| `define-agent` | No flags | `--name LiveMatrixAgent --topic "Go HTTP APIs"` |
| `tool` | list only | `tool generate …` then `tool info` / `tool run` |
| `test-context` | Type bug | Fix CortexKernel assert OR run mock mode if available |
| `campaign` | start/status/list only | pause → resume lifecycle |
| `knowledge` | list only | `knowledge search polystack` (needs embeddings) |
| `embedding` | stats only | optional `reembed` (costly) |

---

## 4. Top 10 high-value live tests (next wave)

Prioritized by: product risk × surface novelty × cost (prefer short, non-hanging, non-destructive first).

| # | Command | Why high value | Suggested invocation | Risk / time |
|---|---------|----------------|----------------------|-------------|
| 1 | `nerd auth status` | Auth path never live-tested; explains engine/OAuth state after invalid_grant | `nerd auth status -w <polystack>` | Low / &lt;30s |
| 2 | `nerd memory status` | Memory tiers core to architecture; parent-only so far | `nerd memory status -w <polystack>` | Low / &lt;30s |
| 3 | `nerd autopoiesis status` | Self-mod / Ouroboros visibility unproven | `nerd autopoiesis status -w <polystack>` | Low / &lt;30s |
| 4 | `nerd northstar show` (+ `facts`) | Strategic planning surface only hit help | `nerd northstar show -w <polystack>` | Low / &lt;30s |
| 5 | `nerd query next_action` (+ `why next_action`) | Kernel query/why path incomplete; phase00 used bad form | `nerd query next_action -w <polystack>` | Low–med / hang risk |
| 6 | `nerd dom demo` | Entire Code DOM family unexercised; demo is self-contained | `nerd dom demo -w <polystack>` | Med / &lt;2m |
| 7 | `nerd tool generate …` | Ouroboros generation is SUMMARY explicit gap | `nerd tool generate "echo uppercaser for strings" -w <polystack>` | Med–high / hang + LLM |
| 8 | `nerd define-agent --name … --topic …` | Specialist creation never succeeded | `nerd define-agent --name PolyStackGo --topic "Go net/http status APIs" -w <polystack>` | High / long LLM |
| 9 | `nerd browser launch` → `session` → `snapshot` | Rod/browser stack unproven; SUMMARY gap | minimal URL e.g. backend `/health` once servers up | Med–high / deps |
| 10 | `nerd campaign pause` + `resume` | Campaign lifecycle incomplete after start | After start: pause then resume | Med / hang risk |

### Honorable mentions (wave 2)

| Command | Note |
|---------|------|
| `nerd commit` / `nerd push` | Git verbs never run; use throwaway branch only |
| `nerd mcp tools` / `mcp status` | After `mcp list` empty, still useful code paths |
| `nerd knowledge search …` | Needs working embeddings (Ollama was down in matrix) |
| `nerd embedding reembed` | Costly; only if Ollama/genai available |
| `nerd test-context --mode mock` | After type-assert fix; high CI value |
| `nerd sessions load <id>` | After interactive or saved session exists |
| Interactive `nerd` TUI smoke | Manual or scripted key sequence |
| `nerd mangle-lsp` | IDE path; short start+shutdown probe only |

---

## 5. Bug / harness findings tied to CLI surface

From MATRIX + SUMMARY (not re-litigated here):

1. **Hang after Result** on create/spawn/run/dream/campaign/test/fix — process does not exit cleanly (~90s post-result).
2. **`test-context`**: expects `*core.RealKernel`, gets `*core.CortexKernel`.
3. **`tool` / `check-mangle`**: require args; bare invocation is usage FAIL (expected).
4. **`define-agent`**: requires `--name` and `--topic`.
5. **Ollama down** during matrix → embedding warnings; knowledge/search/reembed limited.
6. **Auth**: SuperGrok refresh `invalid_grant` — matrix used API key path.

---

## 6. Gap summary table (quick)

| Area | Exercised | Partial | Not |
|------|-----------|---------|-----|
| Workspace (init/scan/status) | yes | — | — |
| Direct LLM verbs (create/fix/…) | yes | — | push, commit |
| Agents (spawn/define) | spawn | define-agent flags | — |
| Campaign | list/start/status | multi-phase depth | pause, resume |
| Kernel (query/logic/why/check-mangle) | logic, check-mangle | query form, why post-init | mangle-lsp |
| Introspection (jit/glassbox/…) | yes | — | — |
| Systems (mcp/memory/auto) | mcp list | parents | memory status, auto*, mcp tools/status |
| Northstar | — | parent help | all subs |
| Browser / DOM | — | parents | all leaf ops |
| Ouroboros tools | list | — | generate/run/info |
| Auth | — | — | all |
| Embeddings | stats | — | set, reembed |
| Sessions / knowledge | list | — | load, search |
| test-context | — | broken | — |
| Interactive TUI | — | — | yes not run |
