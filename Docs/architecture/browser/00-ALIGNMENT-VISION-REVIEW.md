# 00 — Alignment & Vision Review: browser (`internal/browser`)

> Last verified against codebase: 2026-08-09
> Status: Living Reference Document — code-grounded  
> Source: `internal/browser/` (7 non-test ≈ 2.8k LOC; 10 tests) + companion schemas/policy under `internal/core/defaults/`

## 1. North-star statement

codeNERD treats the browser as a **physics engine for the world-model**, not a free-running scraper. Chrome is an external effect; the **Mangle kernel** is the executive. SessionManager and HoneypotDetector **transduce** page state into formal facts (`dom_node`, `css_property`, `react_component`, `is_honeypot`, …). Policy decides what is interactable; the model only proposes goals.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **4** | Go drives Rod; system facts land through `browserKernelSink` in the live executive kernel. Honeypot outcomes remain rule-derived (`honeypot.go`, `browser_honeypot.mg`). Progressive reasoning/query surfaces remain open. |
| Fact-flow fidelity | **4** | `captureDOMFacts`, event stream, `ReifyReact` emit schema-aligned predicates (`session_manager_dom.go`, `schemas_browser.mg`). CLI snapshot exports facts to `.nerd/browser/snapshots/*.mg`. |
| Constitutional safety | **3** | `safe_action(/browser_navigate\|screenshot\|read_dom)` in `constitution.mg`; tactile router marks navigate as `RequiresSafe: true`. Click/type are **not** constitutionally listed as safe actions — intentional high-risk. Wiring of browser into live OODA is partial (boot leaves `browserMgr` nil; VS `handleBrowse` refuses direct execution). |
| JIT / atom discipline | **2** | Package is effectful infrastructure, not prompt-facing. Prompt configs list `browser_navigate` as a tool name (`internal/prompt/config_*.go`) but browser itself has no prompt atoms — correct. |
| Observability | **4** | Dedicated `logging.CategoryBrowser` + convenience helpers + timers on start/create/navigate/screenshot/honeypot (`logger.go`, package sources). |
| Test grounding | **4** | Dense unit coverage of config, throttler, session map, persist/load, honeypot reason tables; lifecycle tests when Chrome present; integration tag for nav/interaction. |
| Wiring completeness | **4** | Cortex owns one manager for tactile + research; facts enter live kernel. Standalone CLI and legacy chat distinctions remain explicit; VS browse handler is still a hard redirect. |
| Isolation / multi-session | **4** | Shared profile tabs by default, explicit isolation, isolated forks, multi-browser bounds, redacted private metadata. |

**Overall alignment: 4 / 5** — runtime ownership and fact truth are repaired; residual risk is progressive surface and end-to-end parity incompleteness.

## 3. What “good” looks like (browser-specific)

| Good | Bad |
|------|-----|
| DOM/CSS/geometry asserted as Decl-bound facts | Scraping text only into chat without kernel |
| Honeypot via derived rules | Hardcoded “skip display:none” only in Go without policy |
| Explicit shared/isolation semantics + session IDs | Profile changes hidden at tool boundaries |
| Event throttle + max DOM nodes (200) | Unbounded fact storms on SPA mutation |
| On-demand Chrome (no boot spawn) | Every TUI session launches a browser |

## 4. Related corpora

- `Docs/architecture/core/` — kernel load of `schemas_browser.mg`, VirtualStore action types  
- `Docs/architecture/cli/` — `nerd browser` command tree  
- `Docs/architecture/shards/` — `TactileRouterShard.SetBrowserManager`  
- `Docs/architecture/tools/` (if present) — research modular browser tools  
- `Docs/architecture/mangle/` — Decl/rule discipline for honeypot rules  

## 5. Verdict

Treat `internal/browser` as **implemented and load-bearing**, not pre-product. Continue BPAR-2 progressive observe/act work, close the remaining VS/shard/tool reachability gaps, and assert `honeypot_suspicious_url` from Go (noted missing in policy comments).
