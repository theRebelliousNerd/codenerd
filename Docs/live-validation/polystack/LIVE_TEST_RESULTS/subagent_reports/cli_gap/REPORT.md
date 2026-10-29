# REPORT — nerd CLI vs Live Matrix coverage

**Agent:** cli_gap (read-only audit)  
**Date:** 2026-07-13  
**Repo:** `C:\CodeProjects\codeNERD`  
**Cmd sources:** `cmd/nerd/main.go` + `cmd_*.go`, `dom_*.go`, `embedding_cmd.go`  
**Matrix:** `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\MATRIX.md`  
**Detail:** [GAP.md](./GAP.md)

## Method

1. Enumerated cobra tree from `rootCmd.AddCommand` / `*.AddCommand` and every `Use:` in `cmd/nerd`.
2. Parsed MATRIX phases 00–04 plus SUMMARY for exercised command names and outcomes.
3. Cross-checked a sample of `*.out`/`*.err` (help-only vs real RunE).
4. Classified each leaf: **exercised** / **partial** / **not**.
5. Ranked next-wave tests by novelty + architectural value + cost.

Did **not** run long `nerd create` for discovery.

## Inventory size

| Metric | Value |
|--------|------:|
| Top-level root commands (excl. default TUI) | 44 |
| Subcommands / tool pseudo-subs (tool list\|run\|info\|generate, etc.) | ~27 |
| Approx invocable paths classified | ~71 |
| Paths **exercised** | ~32 (~45%) |
| Paths **partial** | ~12 (~17%) |
| Paths **not** | ~27 (~38%) |

## Gap summary table

| Status | Commands / areas |
|--------|------------------|
| **Exercised** | `status`, `init`, `scan`, `create` (polyglot), `spawn`, `analyze`, `explain`, `review`, `shadow`, `security`, `whatif`, `fix`, `test`, `refactor`, `run`, `dream`, `perception`, `logic`, `agents`, `jit`, `glassbox`, `transparency`, `reflection`, `logs`, `embedding stats`, `sessions` (list), `knowledge` (list), `campaign` {list,start,status}, `check-mangle` (with file), `tool list`, `mcp list` |
| **Partial** | `browser`/`dom`/`northstar`/`mcp`/`autopoiesis`/`memory` (parent help), `query` (weak pred), `why` (pre-init), `define-agent` (missing flags), `tool` (list only; bare FAIL), `test-context` (kernel type assert), `campaign` (no pause/resume), `knowledge` (no search) |
| **Not exercised** | Interactive TUI; `auth` {grok,claude,codex,status}; `push`; `commit`; `mangle-lsp`; `browser` {launch,session,snapshot}; all `dom` leaves; all `northstar` leaves; `memory status`; `autopoiesis` {status,learning,tools}; `mcp` {tools,status}; `tool` {run,info,generate}; `embedding` {set,reembed}; `sessions load`; `knowledge search`; `campaign` {pause,resume} |

## Strengths of the live matrix

- Strong coverage of **coding-agent direct verbs** and polyglot `create`.
- Core **diag** surfaces (status, jit, glassbox, transparency, reflection, logic) exercised.
- Campaign **start/list/status** and one **spawn** path covered.
- `check-mangle` recovered in phase 4 with a real file.

## Weak spots

1. **Parent-only smoke**: many systems commands “passed” only because cobra printed help (`mcp`, `memory`, `autopoiesis`, `northstar`, `browser`, `dom`).
2. **Entire families dark**: auth, git push/commit, browser/rod, Code DOM, northstar content, Ouroboros generate.
3. **Exit hangs** contaminate PASS semantics for LLM verbs.
4. **test-context** is broken (`CortexKernel` vs `RealKernel`) — CI harness surface unusable until fixed.
5. **Embeddings/Ollama** offline limited knowledge search / reembed value.

## Top 10 remaining high-value live tests

1. `nerd auth status`
2. `nerd memory status`
3. `nerd autopoiesis status`
4. `nerd northstar show` (+ `facts`)
5. `nerd query next_action` + `nerd why next_action`
6. `nerd dom demo`
7. `nerd tool generate <desc>`
8. `nerd define-agent --name … --topic …`
9. `nerd browser launch` → `session` → `snapshot`
10. `nerd campaign pause` + `resume`

Full rationale and suggested args: **GAP.md §4**.

## Recommended next-wave order (practical)

```
# Fast read-only (minutes)
nerd auth status -w <ws>
nerd memory status -w <ws>
nerd autopoiesis status -w <ws>
nerd northstar show -w <ws>
nerd northstar facts -w <ws>
nerd query next_action -w <ws>
nerd why next_action -w <ws>
nerd mcp tools -w <ws>
nerd mcp status -w <ws>

# Medium structural
nerd dom demo -w <ws>
nerd campaign pause -w <ws>   # if campaign active
nerd campaign resume -w <ws>

# Expensive LLM / env
nerd tool generate "..." -w <ws>
nerd define-agent --name PolyStackGo --topic "Go net/http" -w <ws>
# browser only if rod + URL available
```

## Deliverables

| File | Path |
|------|------|
| GAP.md | `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\cli_gap\GAP.md` |
| REPORT.md | `C:\Users\smoor\AppData\Local\Temp\codenerd-live-matrix\subagents\cli_gap\REPORT.md` |

## Conclusion

Live matrix **solidly exercised ~half of invocable CLI surface**, concentrated on create/analyze/diag. **~38% of leaves never ran**, including high-value auth, memory/autopoiesis status, northstar content, DOM, browser, tool generation, and git verbs. Phase-00 “PASS” on several parents is **help-only** and should not be counted as feature proof. Next wave should fix that with short status/subcommand smokes first, then DOM demo, Ouroboros generate, and define-agent.
