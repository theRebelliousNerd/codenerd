# browser internals

> Verified 2026-09-20 against `main` (`231cfa7`). All line references are
> to `internal/browser/session_manager.go` unless noted.

## Config and defaults

`Config` (94-130) is a flat JSON-tagged struct: debugger URL, launch
flags, headless, viewport size, navigation timeout, session store, event
logging level, DOM/header ingestion switches, event throttle, multi-tab
default, tab/browser caps, idle-tab timeout, sensitive-key extras,
workspace and writable roots, evidence toggles and ceilings, spec config,
header-ingestion mode, honeypot guard, epoch fact cap, and container
correlation settings. (Its doc comment is duplicated verbatim at lines
92-93.) Behavior is read through getters, never the fields: headless,
viewport, navigation timeout, multi-tab default, tab/browser caps, and
idle timeout (`IsHeadless` 262-264 through `GetIdleTabTimeout` 312-317);
header ingestion mode and guard (`GetHeaderIngestionMode` 164-175,
`ShouldIngestHeaders` 178-180, `GetHoneypotGuard` 183-193);
evidence enablement and ceilings (`IsEvidenceEnabled` 235-237,
`GetMaxEvidenceFiles` 240-248, `GetMaxEvidenceFileBytes` 251-259).
`DefaultConfig` (205-230) supplies the baseline; `boolPointer` (232) is
its pointer helper.

## Event throttling

`eventThrottler` (60-64) gates one key at a time behind `interval` using
`last` timestamps. `newEventThrottler` (66-74) returns nil for non-positive
milliseconds, and `Allow` (76-90) returns true for a nil receiver — so a
zero throttle disables throttling by construction rather than by error.
A nil `*bool` multi-tab default follows the same pattern via
`IsMultiTabDefault` (291-293).

## The session model

Public metadata and private handles are split deliberately. `Session`
(25-35) and `BrowserInstance` (46-52) are JSON-serializable metadata.
`sessionRecord` (37-43) pairs a `Session` with its `*rod.Page`, an
optional isolated `*rod.Browser`, a `streamCancel` func, and an
`*ElementRegistry`; `browserRecord` (54-58) pairs a `BrowserInstance`
with its `*rod.Browser` and cancel func. Cancellation lives next to the
handle it must release — the code shape of the manager-owned-lifetime
rule in `agents.md:13-14`.

## Manager lifecycle

`SessionManager` (348-370) is built by `NewSessionManager` (373-379), by
`NewSessionManagerWithSink` (382-384) when the caller supplies the fact
sink, or by the shared `newSessionManager` (386-422). `Start` (488-495)
connects guarded by `ensureStarted` (497-505); `ControlURL` (508-512) and
`IsConnected` (515-519) expose connection state; `Shutdown` (522-524)
tears it down. Sessions are created and attached (`CreateSession`
573-575, `Attach` 578-580), enumerated (`List` 527-542, `ListSessions`
548-562, `DefaultSessionID` 566-570), inspected (`Page` 583-592,
`GetSession` 607-615, `UpdateMetadata` 595-604), and forked into
isolation (`ForkSession` 801-868). Tab actions run through `Navigate`
(871-920), `Click` (923-953), `Type` (956-984), and `Screenshot`
(987-1016). `ReifyReact` (639-798) is the DOM/React-to-facts pass;
`Registry` (619-630) exposes the element registry and
`invalidateElementReferences` (632-636) drops stale refs.

## Container correlation (BP-25)

`CorrelateContainerErrors` (428-448) consults `CorrelationContainers`
(121-124): empty disables correlation entirely. `DockerPath` (125-129)
is the resolved docker executable, or `""` when Docker is unauthorized
or absent — resolved via `LookupDockerBinary` so the operator's
`execution.allowed_binaries` stays the authority, with empty disabling
correlation without failing anything.

## Contract audit

`DiscoverContract` (`contract_audit.go:336-370`) validates the repo root
(`validateAuditRoot` 209-222), derives search needles (`auditNeedles`
144-188, with noise/numeric guards `isAuditNoise` 50-53, `isNumericOnly`
55-65, `shouldDropAuditTerm` 67-78), walks URL paths (`extractURLPath`
104-118, `splitPath` 90-102, `stripQueryFragment` 80-88,
`pathSegments` 120-129, `urlPathSegments` 131-138), caps sources
(`cappedSources` 265-272), and builds sorted findings (`findingForNeedle`
274-300, `buildFindings` 302-323, `sortAuditFindings` 256-263), with a
skipped-discovery path (`buildSkippedDiscovery` 224-254). Inputs and
outputs are `ContractAuditInput` (191-198), `AuditFinding` (31-36), and
`ContractAuditDiscovery` (201-207). Fact emission
(`contract_audit_facts.go`) indexes matches (`buildAuditMatchIndexes`
21-34, `auditLineIndex` 16-19), relativizes paths (`auditRelativePath`
36-56, `auditSourceLine` 58-69), and appends findings, needles, and
sources (71-127) through `AuditDiscoveryFacts` (131-148) and
`AssertAuditDiscovery` (153-166).

## How it is verified

- `TestAuditDiscoveryFacts_ShouldNeverEmitHazard`
  (`contract_audit_facts_test.go:59-73`), `..._ShouldScopeEveryFactToSession`
  (32-57), `..._ShouldEmitRelativePathsOnly` (125-147), and
  `..._ShouldCapTotalFacts` (107-123) pin the emission invariants;
  `TestAssertAuditDiscovery_ShouldRedactThroughAddFacts` (149-195) pins
  redaction at the sink boundary.
- `TestSessionManager_Navigation_Integration`,
  `..._Interaction_Integration`, and
  `..._NavigateThenScreenshot_NoFrameRace`
  (`browser_integration_test.go:41-233`) pin end-to-end tab behavior
  against a live page.
