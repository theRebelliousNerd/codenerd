# 00 — Alignment & Vision Review: browser (`internal/browser`)

> Last verified against codebase: 2026-08-09
> Status: Living Reference Document — code-grounded  
> Source: `internal/browser/` (10 non-test ≈ 4.4k LOC; 12 tests) + companion schemas/policy under `internal/core/defaults/`

## 1. North-star statement

codeNERD treats the browser as a **physics engine for the world-model**, not a free-running scraper. Chrome is an external effect; the **Mangle kernel** is the executive. SessionManager and HoneypotDetector **transduce** page state into formal facts (`dom_node`, `css_property`, `react_component`, `is_honeypot`, …). Policy decides what is interactable; the model only proposes goals.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **4** | Go drives Rod; system facts land through `browserKernelSink` in the live executive kernel. Honeypot outcomes remain rule-derived (`honeypot.go`, `browser_honeypot.mg`). Progressive reasoning/query surfaces remain open. |
| Fact-flow fidelity | **4** | `captureDOMFacts`, event stream, `ReifyReact` emit schema-aligned predicates (`session_manager_dom.go`, `schemas_browser.mg`). CLI snapshot exports facts to `.nerd/browser/snapshots/*.mg`. |
| Constitutional safety | **4** | All registered browser spellings, including observe/act, still require exact `pending_action` → `permitted(action,target,payload)` derivation. Browser session/browser/target handles are recognized safety targets. |
| JIT / atom discipline | **4** | Research/test intent configs select observe/act; `capability/browser_progressive` and its safety dependency teach the bounded ref-first protocol without inventing authority. |
| Observability | **4** | Dedicated `logging.CategoryBrowser` + convenience helpers + timers on start/create/navigate/screenshot/honeypot (`logger.go`, package sources). |
| Test grounding | **5** | Unit/race coverage plus live modular-registry Chrome proof for observations, refs, actions, secret redaction, screenshot confinement, DOM and React modes. |
| Wiring completeness | **4** | Cortex owns one manager for tactile + research; facts enter live kernel. Standalone CLI and legacy chat distinctions remain explicit; VS browse handler is still a hard redirect. |
| Isolation / multi-session | **4** | Shared profile tabs by default, explicit isolation, isolated forks, multi-browser bounds, redacted private metadata. |

**Overall alignment: 4 / 5** — runtime truth and BPAR-2 observe/act are production-routed; reason/audit/spec/evidence parity remains incomplete.

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

Treat `internal/browser` as **implemented and load-bearing**, not pre-product. Continue BPAR-3 reasoning/wait work, close the remaining VS/shard/tool reachability gaps, and assert `honeypot_suspicious_url` from Go (noted missing in policy comments).
