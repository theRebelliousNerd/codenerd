---
doc-class: shipped
subsystem: orient
implementation-status: partial
last-verified: 2026-09-29
verified-against: working-tree-C2a
supersedes: []
---

# Wiring and remaining work

| Route | Source witness | Status |
|---|---|---|
| Init scan -> orientation -> profile -> agents/KBs | `internal/init/initializer.go:469` (`Initialize`), `internal/init/initializer.go:712` (`newPhaseRunner`) | Authored, runtime gate pending |
| Orientation -> northstar classify/draft/install | `internal/init/phase_orient.go:38` (`runOrientation`) | Calls the C2b-owned API; unverified |
| Orientation -> ecosystem agent/knowledge/prompt projection | `internal/init/phase_ecosystem.go:115` (`integrateEcosystem`) | One retained engine; unverified |
| Orientation -> strategic knowledge consumers | `internal/init/phase_orient.go:258` (`persistOrientationKnowledge`) | Existing categories retained; unverified |
| Snapshot -> boot facts | `internal/orient/snapshot.go:181` (`Projection`); `internal/core/kernel_init.go:309` (`loadMangleFiles`) | Refresh precedes loading; errors withhold stale projection |
| New commits -> delta measurements | `internal/orient/history_delta.go:13` (`historyDelta`); `internal/orient/snapshot.go:281` (`Refresh`) | One delta log pass; rename/rewind fallback; unverified |
| Incremental scan -> refresh | `internal/orient/snapshot.go:281` (`Refresh`) is available | **Not wired**; scan/consumer files are outside C2a scope |
| Changed documents -> new role claims | `internal/orient/freshness.mg:10` (`orient_role_pending`) | Old claims removed; LLM-capable resumption remains outside the boot path |
| Typed orient configuration -> global config loader/check | `internal/config/orient.go:97` (`LoadOrientConfig`) | Scoped reader present; global registration outside C2a scope |

GAP-ORIENT-23 is partial. Tests and builds were not run; authored test exits are
listed in [13-INIT-AND-REORIENTATION.md](13-INIT-AND-REORIENTATION.md). Neither
source inspection nor file creation establishes passing behavior.

The running-session hook can publish the exact public fact delta atomically through `RefreshResult.Apply` (`internal/orient/snapshot.go:258`). No caller in the incremental scanner was edited by C2a.
