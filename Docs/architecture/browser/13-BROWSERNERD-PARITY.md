---
doc-class: governance
subsystem: browser
implementation-status: target-state
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: [61ed0677:Docs/architecture/browser/BROWSERNERD-PARITY.md]
---

# BrowserNERD 1.2.0 Parity Contract

> Restated 2026-09-29. Supersedes the initial parity contract from commit 61ed0677 (deleted 2026-09-20).
> Incorporates the BrowserNERD 1.2.0 delta (through commit 1396e5309) and the uncommitted process-reaper subsystem.

## 1. Upstream and Local Baselines

The authoritative upstream source of truth for BrowserNERD 1.2.0 capabilities is the CrossThread repository at `C:\CodeProjects\SybioGenv3\crossthread\dev_tools\BrowserNERD` at commit `0806144c6` (with the latest touching commit `1396e5309`), augmented with the twelve uncommitted working-tree files in that directory. The nested git repository at `dev_tools/BrowserNERD/.git` (HEAD `8b61e09`, dated 2026-08-01) is two months stale and is explicitly rejected as a source. The uncommitted upstream files represent the Chrome process tree reaper and its integration (`reaper.go`, `reaper_windows.go`, `reaper_unix.go`, `reaper_test.go`, and their wiring into `browser_instances.go`, `session_core.go`, `server.go`, and `server_doctor.go`).

The codeNERD baseline is commit `6597099c`. Previous parity reviews evaluated baseline codeNERD at `9170ad07` and `231cfa7`.

## 2. Meaning of Parity

Parity means that every operational and reasoning capability of BrowserNERD 1.2.0 is fully reachable through codeNERD's production runtime, safely bounded, governed by logic, and backed by verifiable automated tests.

Parity does not mean embedding BrowserNERD's standalone Model Context Protocol (MCP) server binary or running BrowserNERD's embedded secondary Mangle engine inside codeNERD. The architectural split adheres to the following binding adaptation rules:

- **Single Live Kernel**: Browser observations and events enter codeNERD's unified live kernel (`RealKernel` / `SystemKernel`). There is no second, detached Mangle engine or parallel reasoning reality.
- **Native Underscore Tools**: Agent-facing operations are exposed as native modular tools using codeNERD's snake_case convention: `browser_observe`, `browser_act`, `browser_reason`, `browser_audit`, `browser_mangle`, `browser_specs`, and `browser_test`. They are registered in the central tool catalog (`internal/tools/research/browser_progressive.go:14-95`, `internal/tools/research/browser_reasoning.go:99-178`).
- **JIT Prompt Atoms**: Guidance and capability disclosure are delivered via Just-In-Time prompt atoms (`internal/prompt/atoms/capability/browser_progressive.yaml:1-48`), not by bloating static tool descriptions.
- **Constitutional Governance**: All mutating browser actions remain subject to codeNERD's constitutional policy with default deny (`internal/core/defaults/policy/constitution.mg:150-175`). High-impact operations (such as file uploads or armed dialog accepts) require explicit, separate permission derivations.
- **Workspace Authority Root**: Browser configuration and persistent artifacts live strictly under `.nerd/` or configured workspace roots (`internal/browser/session_manager.go:94-130`). There is no independent `.browsernerd` configuration authority.
- **Attribution**: Source-derived code retains full attribution and notices under the Apache License, Version 2.0.
- **Production Gate**: A capability or gap row is classified as done only when unit test coverage passes and live Chrome execution confirms the observable behavior through codeNERD's production routes.

## 3. Scope Boundaries: What Is and Is Not Ported

To protect codeNERD from architectural bloat and duplicate logic, strict boundaries govern what is ported from BrowserNERD 1.2.0:

### Ported Components

- **First-Byte Event Stream**: Pre-navigation CDP subscription lifecycle, frame attachment, and comprehensive event capture.
- **Page Hooks and DOM Sensing**: Early script hooks for capturing route announcements, toast notifications, console events, unhandled exceptions, and page lifecycle failures before page scripts execute.
- **Progressive Observation**: Observation modes including compact summaries, navigation graphs, interactive element handles, data grids, hidden elements, raw text, and storage metadata (cookies and web storage with JWT expiration extraction).
- **Surgical Locators and Actions**: Validated sequential action batches supporting element references, visible text, labels, roles, and coordinates, with post-action verification and effects summaries.
- **Deductive Reasoning Rules**: Causal temporal diagnosis rules, bucketed request failure correlations, asset-type 4xx discrimination, and action candidate ranking.
- **Safe Process Lifecycle**: Rod launcher configuration with `Leakless(false)` to prevent antivirus quarantining, process-tree tracking, and automated orphan reaping at process boundaries.
- **Declarative Test Fixtures**: Portable, selector-free test execution and assertion fixtures.

### Explicitly Excluded Components

- **Secondary Mangle Engine**: Files implementing BrowserNERD's standalone deductive engine (`indexed_store.go`, `engine.go`, `closure.go`, `catalog.go`, `stats.go`, `query_check.go`, and `schemas/embed.go`) are not ported. Their deductive rules are ported directly into codeNERD's declarative corpus (`internal/core/defaults/schemas_browser.mg` and `internal/core/defaults/policy/browser.mg`).
- **Standalone MCP Server and Server Doctor**: Upstream MCP protocol wrappers in BrowserNERD (`mcp-server/cmd/server/main.go`, `mcp-server/internal/mcp/server.go`, `mcp-server/internal/mcp/server_doctor.go`) are not ported. Diagnostics are exposed through codeNERD's native Go health checks and tools.
- **Arbitrary JavaScript Execution (BP-19)**: Upstream or unconstrained JavaScript execution tools are intentionally omitted. Browser automation is restricted to closed, typed operations.
- **Repository Tracing and Contract Audit Expansion**: `internal/browser/repo_trace.go` and `internal/browser/contract_audit.go` are distinct capabilities under independent maintenance and are not modified by this port.

## 4. The "Even Better" Design Mandate

Rather than creating a verbatim copy of upstream Go code, codeNERD adopts twelve specific architectural enhancements where BrowserNERD's imperative Go logic is elevated into native Mangle deductive rules:

1. **Asset 4xx Discrimination in Logic**: Go asserts the raw CDP resource type in HTTP error facts. Mangle rules discriminate asset failures (images, fonts, stylesheets, media) from actionable API failures.
2. **Bucketed Causal Chain Reasoning**: Causal failure correlations are derived in Mangle using two-second time buckets, eliminating quadratic joins in Go.
3. **Derived Action Effects**: Go settles the browser and records timestamps; Mangle derives the causal effects of the action batch.
4. **Storage Expiry as Logic Facts**: Local and session storage inspection extracts keys and expiration timestamps without exposing raw credential values to the model, joining with authentication errors in logic.
5. **Deductive Action Candidate Prioritization**: Interactive element ranking is derived by Mangle rules; Go merely handles layout grouping.
6. **Default-Deny Dialog Policy**: Dialog answers default to dismiss unless an explicit kernel fact authorizes an accept.
7. **Constitutional Partitioning of Actions**: `browser_act` is partitioned so safe read-only or low-impact interactions remain permitted, while uploads and dialog accepts require distinct authorization.
8. **Signal-Aware Fact Retention**: When the fact buffer approaches saturation, low-signal polling events are evicted first, ensuring failure evidence is preserved.
9. **Lifecycle-Bounded Process Reaping**: Process cleanup runs exclusively during system startup and shutdown, never as an agent-callable tool.
10. **Unattended Navigation Tracking**: Navigation events occurring outside explicit model actions are reified as facts and surfaced in the subsequent observation.
11. **JIT Atom Token Discipline**: Tool descriptions remain concise; operational guidance is provided dynamically through prompt atoms, verified by token-budget tests.
12. **Structured Backend Correlation**: Container log matching correlates specific failed requests by endpoint path, and comprehensive container failure produces an explicit error.

## 5. Attribution and Licensing

BrowserNERD is developed by theRebelliousNerd and licensed under the Apache License, Version 2.0.

All ported files, adapted algorithms, and translated Mangle schemas must preserve the Apache-2.0 copyright notices. The central notice in `THIRD_PARTY_NOTICES.md:3-20` is maintained and updated to cite the expanded scope of adapted browser automation, observation, and reasoning capabilities.
