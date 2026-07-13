# 00 — Alignment & Vision Review: browser (`internal/browser`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/browser/` (3 non-test ≈ 1.9k LOC; 6 tests) + companion schemas/policy under `internal/core/defaults/`

## 1. North-star statement

codeNERD treats the browser as a **physics engine for the world-model**, not a free-running scraper. Chrome is an external effect; the **Mangle kernel** is the executive. SessionManager and HoneypotDetector **transduce** page state into formal facts (`dom_node`, `css_property`, `react_component`, `is_honeypot`, …). Policy decides what is interactable; the model only proposes goals.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **4** | Go drives Rod; facts land in `EngineSink` → Mangle. Honeypot outcomes come from `EvaluateRule("is_honeypot")` + policy files, not ad-hoc LLM judgments (`honeypot.go`, `browser_honeypot.mg`). Gap: research tools create a package-level singleton manager with **nil engine**, so that path skips fact reification. |
| Fact-flow fidelity | **4** | `captureDOMFacts`, event stream, `ReifyReact` emit schema-aligned predicates (`session_manager_dom.go`, `schemas_browser.mg`). CLI snapshot exports facts to `.nerd/browser/snapshots/*.mg`. |
| Constitutional safety | **3** | `safe_action(/browser_navigate\|screenshot\|read_dom)` in `constitution.mg`; tactile router marks navigate as `RequiresSafe: true`. Click/type are **not** constitutionally listed as safe actions — intentional high-risk. Wiring of browser into live OODA is partial (boot leaves `browserMgr` nil; VS `handleBrowse` refuses direct execution). |
| JIT / atom discipline | **2** | Package is effectful infrastructure, not prompt-facing. Prompt configs list `browser_navigate` as a tool name (`internal/prompt/config_*.go`) but browser itself has no prompt atoms — correct. |
| Observability | **4** | Dedicated `logging.CategoryBrowser` + convenience helpers + timers on start/create/navigate/screenshot/honeypot (`logger.go`, package sources). |
| Test grounding | **4** | Dense unit coverage of config, throttler, session map, persist/load, honeypot reason tables; lifecycle tests when Chrome present; integration tag for nav/interaction. |
| Wiring completeness | **3** | Real CLI (`cmd_browser.go`), research tools (`tools/research/browser.go`), tactile field + routes, schemas loaded in `kernel_init.go`. Chat boot **intentionally** does not construct SessionManager at start. VS browse handler is a hard redirect error. Dual managers (research singleton vs chat field) risk divergence. |
| Isolation / multi-session | **4** | Incognito contexts per session; fork copies cookies/storage; session store JSON under `.nerd/browser/`. |

**Overall alignment: 3.5 / 5** — solid “Browser Physics” core and Mangle surface; residual risk is **integration partiality** (who owns the live manager, when facts hit the Cortex kernel vs a throwaway engine).

## 3. What “good” looks like (browser-specific)

| Good | Bad |
|------|-----|
| DOM/CSS/geometry asserted as Decl-bound facts | Scraping text only into chat without kernel |
| Honeypot via derived rules | Hardcoded “skip display:none” only in Go without policy |
| Incognito + explicit session IDs | Shared cookie jar across agent tasks silently |
| Event throttle + max DOM nodes (200) | Unbounded fact storms on SPA mutation |
| On-demand Chrome (no boot spawn) | Every TUI session launches a browser |

## 4. Related corpora

- `Docs/architecture/core/` — kernel load of `schemas_browser.mg`, VirtualStore action types  
- `Docs/architecture/cli/` — `nerd browser` command tree  
- `Docs/architecture/shards/` — `TactileRouterShard.SetBrowserManager`  
- `Docs/architecture/tools/` (if present) — research modular browser tools  
- `Docs/architecture/mangle/` — Decl/rule discipline for honeypot rules  

## 5. Verdict

Treat `internal/browser` as **implemented and load-bearing**, not pre-product. Align future work on: single shared SessionManager with Cortex engine sink; close VS↔shard↔tool execution path; assert `honeypot_suspicious_url` from Go (noted missing in policy comments).
