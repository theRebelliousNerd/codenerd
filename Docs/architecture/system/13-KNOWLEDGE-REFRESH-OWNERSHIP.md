---
doc-class: shipped-with-future
subsystem: system
implementation-status: partial
last-verified: 2026-10-31
verified-against: 2e467c9f3150d627381e9367d1ef7caad25cb504
supersedes: []
---

# Knowledge refresh ownership

## Required boundary

The harness must enforce the owner's constraints before background acquisition
can call providers or mutate knowledge. Free-text mission instructions do not
grant background research. Bounded recurse missions suspend all knowledge
refresh, including explicit refresh, until their invocation ends. This is a
conservative lifecycle boundary, not a natural-language no-web parser or a new
permission grant. Existing model/tool authorization remains unchanged.

The CLI must suppress scheduling before mission Cortex boot, with the boot
restriction bound into cache identity. A mission using an already booted Cortex
must acquire a counted suspension lease: close refresh admission, cancel the
active acquisition, and join it before mission admission. Cancellation or a
missing host quiescence hook refuses admission. Releasing one lease must not
release another; releasing the last resumes ordinary scheduled refresh. A
different Cortex and non-refresh work remain independent.

Normal non-mission refresh retains its policy schedule and source selection.
Its context belongs to the Cortex service, with the Cortex usage tracker,
session scope, autopoiesis purpose and knowledge-refresh phase. It must not copy
an arbitrary boot caller's observer, task, or cancellation lifetime. Explicit
refresh retains its own caller's cancellation and nested usage observers.
Cancellation must stop subsequent sources and permit no refresh effects after
the suspension lease is returned. Unknown provider usage stays unknown in the
broker; suppressed calls are not billed to a mission.

## Current implementation and observed defect

An exploratory live probe at the verified base observed
two completed grounded searches and one canceled search before any repair
attempt; its mission ledger contained no usage reports. This is a boundary
failure, not qualification of recurse.

The fix attaches suppression before boot in `runCampaignRecurse`
(`cmd/nerd/cmd_campaign_recurse.go:151`) using `WithoutKnowledgeRefresh`
(`internal/system/knowledge_refresh_scope.go:15`). `getOrBootCortexWithConfig`
binds it to cache identity (`internal/system/factory.go:287`);
`initKnowledgeRefresh` omits the refresher when suppressed and otherwise
attaches the Cortex tracker, session scope and service purpose/phase
(`internal/system/knowledge_refresh_boot.go:20`). It deliberately does not
inherit the first boot caller's observer or request lifetime.

`SuspendKnowledgeRefresh` holds counted, idempotently released leases
(`internal/system/knowledge_refresh_scope.go:131`), and
`refreshAdmission.suspend` cancels and joins any active acquisition before
returning one (`internal/system/knowledge_refresh_scope.go:86`).
`knowledgeRefresher.serve` resumes on the last lease release
(`internal/system/knowledge_refresh.go:401`). `RecurseMission.begin` requires
the host barrier and retains its lease until the returned cancellation cleanup
(`internal/campaign/recurse_mission.go:99`). Both `newRecurseMission`
(`cmd/nerd/cmd_campaign_mission.go:40`) and chat's `Model.runRecurseLoop`
(`cmd/nerd/chat/campaign_recurse.go:187`) supply the Cortex barrier.

## Checkable exit: GAP-SYSTEM-REFRESH-OWNERSHIP

Use deterministic local tests, without live credentials: boot suppression and
cache separation; scheduled refresh canceled and joined before a no-web mission;
missing hook and canceled lease refuse; nested leases and a second Cortex stay
isolated; ordinary service receipts name their own session/purpose/phase;
explicit refresh preserves nested usage reports and cancellation; normal refresh
resumes after the final lease. Exercise CLI and chat wiring as well as the
underlying service. No test may widen safety or permission rules.

The ownership/usage/suspension integration and cache/cancellation controls are
`TestKnowledgeRefreshMissionSuspensionOwnsCancellationAndUsage`,
`TestKnowledgeRefreshSuspensionCancellationAndWaitingRun`, and
`TestKnowledgeRefreshBootSuppressionAndCacheIsolation`
(`internal/system/knowledge_refresh_scope_test.go:81`,
`internal/system/knowledge_refresh_scope_test.go:228`,
`internal/system/knowledge_refresh_scope_test.go:259`). Host
admission failures are checked by `TestMissionRequiresJoinedBackgroundLease`
(`internal/campaign/recurse_mission_policy_test.go:38`); the scripted production
cycle checks that its worker cannot invoke refresh
(`cmd/nerd/cmd_campaign_mission_test.go:128`,
`TestMissionRecurse_CommandAdapterThroughProductionCycle`). The focused suite
and the new boundary suite under `-race` passed. A broader race run found the
existing resident-shard shutdown race; the same test also failed with that
race on an untouched base checkout. No fix to that separate ownership path is
included, and no new live probe or R7 qualification is claimed.

## Verification receipt and publication limit

The implementation commit above passed the full offline sqlite_vec suite:
112 testable packages, 21996 passing tests and 80 skips; five other packages
have no tests. The sqlite_vec build and vet passed. The new boundary race suite
passed 11 tests. The broader refresh/mission race run failed one existing
shutdown test, and that same test failed once in ten baseline repetitions.
These are separate outcomes; the targeted pass does not erase the broader
failure. Existing shard shutdown remains a separate work item.

This change is local and publication is pending: the repository requires a
private-terms scanner for documentation changes, but no scanner or configured
denylist was available in the permitted repository/tool paths. Its check is
unknown, not passed. No merge, push, deployment or new live provider run is
part of this receipt. Free-text restrictions on other model tools are outside
this background-acquisition repair.
