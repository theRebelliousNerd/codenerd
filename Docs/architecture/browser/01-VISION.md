# 01 — Vision: Browser Physics

> Last verified against codebase: 2026-07-13  
> Status: Target architecture vision (informed by existing code, not greenfield fiction)

## 1. Product role

The browser package turns the live web into a **reified world fragment** the kernel can reason about:

1. **Act** — navigate, click, type, screenshot under explicit session IDs.  
2. **Observe** — CDP streams (nav, console, network) + DOM snapshots + optional React Fiber walk.  
3. **Judge** — Mangle derives honeypots, safe interactables, spatial relations (`left_of` / `above`).  
4. **Persist** — session metadata (and CLI control URL / snapshots) under `.nerd/browser/`.

This is Cortex §9.0 “Browser Physics”: geometry and CSS are first-class inputs to safety policy, not opaque screenshots alone.

## 2. Target control loop

```
┌─────────────┐     next_action      ┌──────────────────┐
│ Mangle      │ ───────────────────► │ VirtualStore /   │
│ kernel      │                      │ modular tools /  │
│ permitted() │ ◄─── facts ───────── │ tactile router   │
└─────────────┘                      └────────┬─────────┘
                                              │
                                              ▼
                                    ┌──────────────────┐
                                    │ SessionManager   │
                                    │ (Rod + Chrome)   │
                                    └────────┬─────────┘
                                              │
                         DOM/React/events ────┤
                                              ▼
                                    EngineSink.AddFacts
```

**Invariant:** no browser mutation without a traceable session; no “safe click” claim without either policy derivation or explicit human override path.

## 3. Desired properties

| Property | Target |
|----------|--------|
| Engine coupling | One live `SessionManager` per Cortex instance, sink = production `mangle.Engine` |
| Multi-session | N tabs/contexts, listable, forkable, detach/reattach via TargetID |
| Fact budget | Throttled events; bounded DOM capture (already max 200 nodes) |
| Safety | Click/type only on `safe_interactable` or explicit permit; honeypot deny by default |
| Headless/headful | Config-driven; default headful in `DefaultConfig` for operator visibility |
| Attach mode | Connect to existing Chrome via `DebuggerURL` (CLI control file pattern) |
| React SPA | Fiber reification for component tree when available |
| Operator tools | Full Cobra parity for launch/session/snapshot; optional TUI status |

## 4. Non-goals

- Full Playwright-class test runner or visual regression suite  
- General web scraping product with proxy rotation / CAPTCHA solving  
- Replacing Context7/web_fetch for static docs (browser is for JS-rendered / interactive surfaces)  
- Embedding Chromium — rely on system Chrome via go-rod launcher  
- sibling-platform product concepts (foreign-product-surface, foreign-agent-kit, etc.) — out of scope  

## 5. Success criteria (vision)

1. Research tools, CLI, and agent OODA share **one** manager and **one** fact sink.  
2. Kernel program always includes `schemas_browser.mg` + honeypot policy (already loaded).  
3. Agent cannot click honeypots when policy is active and DOM facts are fresh.  
4. Integration tests green with Chrome in CI when tagged; unit tests always green without Chrome.  
5. Snapshots and session store are enough for post-mortem of a failed browse campaign.

## 6. North-star mapping

| North star | Browser realization |
|------------|---------------------|
| LLM = creative center | Model proposes “inspect this SPA” / selectors; does not invent safety |
| Kernel = executive | `is_honeypot`, `safe_interactable`, `safe_action` gate effects |
| Transduction | Rod JS eval + CDP → `mangle.Fact` atoms |
| Default deny | Constitution lists only navigate/screenshot/read_dom as safe browser actions |
| Wiring before deletion | Field `BrowserManager` and research singleton both real — audit before removing either path |
