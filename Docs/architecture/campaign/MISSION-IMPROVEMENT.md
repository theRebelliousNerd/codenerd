# Mission-linked recurse

The mission mode of recurse keeps an owner-supplied purpose under continuous
pressure while bounding each invocation. It extends the existing dependency
sweep and ratchet rather than introducing another scheduler. Start it with
`nerd campaign recurse --mission "<desired behaviour>"` or
`/recurse --mission "<desired behaviour>"`. Existing subsystem and pass options
still apply. Ordinary recurse retains its current behaviour.

## Required contract

### Guard-source ownership

GAP-CAMPAIGN-GUARD-SOURCES is closed locally for four named guard units in
`6e3506acc0f4c3e5d06672c7539d1f55536e236e`. This is a host-authored restriction.
The decoded usage sensor (`internal/perception/usage_track.go`), broker capture
(`internal/broker/broker.go`) and refresh suspension/boot units
(`internal/system/knowledge_refresh_scope.go`,
`internal/system/knowledge_refresh_boot.go`) constrain the mission and remain
owner-controlled source. Default mission protection and the repository's
`nerd.md` forbid rules must refuse model writes to all four, including when they
are within the admitted node. Existing protections remain in force. Ordinary
source writes and protected-file reads remain available under their existing
permissions. A writer that bypasses the tool guard must still be refused by the
ratchet, with its owned writes restored. No recovery or permission-widening
change belongs to this repair. Before closure, deterministic controls must show
refusal before mutation, rollback after a bypass and ordinary-write success.
The default list is supplied by `internal/config/mission.go:22`. The real-kernel
in-scope control is `internal/campaign/recurse_guard_source_test.go:19`; the
shipped project-document tool refusal and read/ordinary-write controls are
`internal/session/guard_source_protection_test.go:18`. The real recurse ratchet
refuses and restores owned bypass writes in
`internal/campaign/recurse_guard_source_test.go:65`. These controls and their
race checks pass locally; byte-identical tests fail against the pinned parent.
The tool fixture isolates the document gate with the existing mock
constitutional seam; existing permission/allowlist precedence and unrelated
write isolation remain separately verified. This is not a universal guard
inventory or live qualification. Publication requires the currently unavailable
private-terms scanner.

### Mission admission

Before mission admission, the host must close knowledge-refresh admission,
cancel and join its active refresh, and hold that suspension until mission end.
The CLI also suppresses scheduling during mission Cortex boot. Missing or failed
host quiescence refuses the mission. All refresh is suppressed during a bounded
mission rather than interpreting free-text no-web instructions as executable
policy. Ordinary refresh resumes after the final suspension lease; unrelated
Cortex services and caller usage observers remain isolated. The ownership
contract is specified in
[system/13-KNOWLEDGE-REFRESH-OWNERSHIP.md](../system/13-KNOWLEDGE-REFRESH-OWNERSHIP.md).

Mangle owns run admission, budget exhaustion, stalled progress, and whether a
deliberated attempt may execute. The existing proposer and requirements
interrogator shape each selected repair or improvement. Their input preserves
the complete mission, the selected node, measured finding or improvement angle,
North Star context and previous outcome. They cannot choose another write scope,
replace a gate, or confer constitutional permission. A values fork pauses the
mission with its saved question; it never silently selects a preference.

Execution remains a normal recurse attempt campaign, including its North Star
observer/Guardian, write guard, acceptance witness, regression ratchet and owned
compensation. Model prose is never a passing witness. Mission outcomes are
recorded separately from mission satisfaction: resolving a measured finding can
support that gate deficit at that source snapshot; moving a metric cannot
establish an arbitrary strategic requirement. No automatic North Star waiver,
risk mitigation, prompt promotion, deployment or restart follows.

The `mission` configuration section supplies positive finite limits:
`max_cycles` (8), `max_seconds` (3600), `max_spend_microusd` (5000000),
`no_progress_cycles` (2), `concurrency` (1), and `context_bytes` (65536).
Deliberation rounds, candidates and evidence calls use the existing `routing`
settings; campaign retries, acceptance rounds and tools use their owning
settings. Concurrency other than one is rejected until isolated adoption is
implemented. All executive thresholds are published as `config_param` facts.

Time includes discovery and verification. Cancellation joins the existing
attempt/compensation path. Spend is the sum of versioned provider price estimates,
not a billing guarantee; admission stops between paid phases when it reaches
the limit. A paid phase may cross that limit. Missing, partial or unpriced usage
blocks further work rather than being counted as free. Preserve the existing
usage tracker and observers. An invocation has a fresh finite envelope; starting
another is explicit owner action.

Usage uncertainty applies to attempts inside one provider call as well as to
separate calls. A successful retry without token metadata cannot inherit the
preceding attempt's accounting. The provider must preserve the known counts
and mark the missing attempt unknown through nested observers; the next mission
admission then derives `/unknown_spend`. A fully reported retry remains eligible
under the existing limits. GAP-PERCEPTION-RETRY-USAGE owns this adapter repair.

An inference request whose response was lost must retain the same uncertainty
even when the transport retries internally and returns a counted success. The
Anthropic adapter must notify incomplete observers at that transport boundary;
the mission retains known successful spend and refuses further admission via
`/unknown_spend`. A local server that reads the first request before closing
without headers is the required control, with clean success still admitted.
GAP-PERCEPTION-ANTHROPIC-LOST-RESPONSE owns this bounded adapter strengthening;
mission gate and recovery implementations must remain unchanged.

That adapter boundary is closed locally in
`271336b1b43d9feeda0450ae1b76fdac00caf617`. The actual single-key production
factory and real-kernel mission control in
`internal/campaign/recurse_mission_lost_response_test.go:21` retains 2,000
micro-USD of reported spend after a lost response and derives `/unknown_spend`.
A clean 2,000 micro-USD call stays admitted; fully reported 4,000 micro-USD under
a 3,000 micro-USD cap derives `/spend`. Saved mission state retains counts and
stop status. The local server consumes the inference request before losing
its response; this proves uncertainty, not an actual provider charge. Gate and
recovery code is unchanged; publication requires the private-terms scan.

The bounded local candidate `a6aaaf0f1cbc429dac34f57b3d92bba2486c8fca` exercises
the decoded retry through the provider adapter, broker, mission meter and real
kernel in `internal/campaign/recurse_mission_retry_usage_test.go:16`.
Missing usage stops admission and persists uncertainty while preserving the
known 2,000 micro-USD; the fully reported control retains 3,000 micro-USD and
remains admitted. This offline fixture is not live R7 qualification. Unknown
transport failures and malformed replies remain separate accounting obligations.

The meter uses the typed `usage.MicroUSDPerUSD` currency conversion from
`internal/usage/pricing.go` for both overflow detection and rounded accounting.
The spend limit remains `mission.max_spend_microusd`; the currency unit is fixed.
Chat command prefixes are token sequences whose lengths define their grammar
in `cmd/nerd/chat/campaign_recurse.go`, independent of routing budgets.

Anthropic's incomplete-message retries and pause/resume parts must obey the
same accounting contract. Each decoded attempt is observed before validation,
and the accepted exchange aggregate is not counted again. Missing usage stays
unknown even when another attempt reports. Local adapter candidate
`ac50802fe1441b05d1530c76df6d31868596d14f` is controlled through the broker,
mission meter and real kernel in
`internal/campaign/recurse_mission_anthropic_usage_test.go:20`: under a 3,000
micro-USD limit, 4,000 micro-USD of counted retries derives `/spend`, a missing
attempt with 2,000 micro-USD known derives `/unknown_spend`, and a clean 2,000
micro-USD call remains admitted. The saved mission retains the accounting and
stop status. Admission and recovery implementations are unchanged. Recovered
wire failures, server-tool per-use fees and hard billing guarantees remain
outside this bounded local result; publication requires the private-terms scan.

## Durable state and recovery

Mission snapshots live under `.nerd/sessions/<session>/missions/<id>.json` with
owner-only permissions and atomic replacement. They preserve exact mission and
limits, selected design/thread, admitted node, witness identity, source snapshot,
observed ratchet outcome, commit identity and uncertainty. Save intent before
execution. Keep outcome history on disk and give subsequent deliberations an
explicit reference plus the last outcome; never silently truncate evidence.
The existing recurse journal remains authoritative for adoption and recovery.
An interrupted mission is not automatically replayed or reported accepted.
Reconcile its pending receipt against the recurse journal before attempting
additional effects; ambiguity requires owner inspection.

The implementation checks pending intents across session folders before another
mission starts. Sidecars under `missions/<id>/` retain full verification-input
digests, returned owned-write receipts, and complete measured check output and
comparison domains. Existing Go tests, module files,
testdata and `nerd.md` gate declarations are pinned through
`internal/evidence/change.go:VerificationInputs`; additional protected paths
come from `mission.protected_paths`. New tests may be added. Modifying a pinned
oracle requires a separate work item. Missing live oracle metadata refuses
writes. The ratchet also rejects out-of-scope, protected and unattributed
changes, including paths whose current bytes differ from their last owned
postimages. These checks do not claim an atomic transaction against arbitrary
external editors between the check and Git adoption.

Ordinary recurse also refuses recovery of a mission-marked in-flight attempt
without a settled verdict. It cannot bypass the mission receipt by invoking
the older difference-from-HEAD recovery route. Errors before adoption compensate
owned writes; an ambiguous adoption acknowledgement retains the durable intent
for reconciliation.

## Design rulings and acceptance

Repair priority and dependency order remain recurse policy. Deliberation chooses
the design within that admitted work item, including green-node exploration.
Learning means relaying observed outcomes and the existing retry evidence; it
does not mean changing prompts or promotion standards. Whole-snapshot validity
is conservative: an old source receipt is historical after any source change.
Unknown remains unknown. Debugging, runtime symptom resolution, holdout-based
prompt evaluation and isolated concurrent experiments retain the stronger
contracts proposed in `Docs/innovations/mission-driven-forever-improvement.md`;
this mode must not manufacture evidence for them.

The production-path acceptance fixture must use a scripted model, real kernel,
registered read tools, normal campaign execution and a real host witness. It
must observe a failing gate before repair, a passing witness afterwards, durable
mission state and a kernel-derived configured stop. Negative controls cover
budget zero-crossings, unknown spend, values forks, worker failure, cancellation,
source drift and recovery ambiguity. Tests and the disposable Mangle lab are
authored for execution by the validating session; no runtime success is claimed
until that session runs them.

## Authored integration and remaining evidence

Local verification of the refresh barrier passed the focused mission suite,
the new boundary race controls, the full offline sqlite_vec suite, build and
vet. `TestMissionRecurse_CommandAdapterThroughProductionCycle` now also checks
that its admitted worker cannot call the suspended refresher
(`cmd/nerd/cmd_campaign_mission_test.go:128`).
`TestMissionRequiresJoinedBackgroundLease` covers missing/failed hooks,
missing leases and canceled admission
(`internal/campaign/recurse_mission_policy_test.go:38`). A broader refresh race
run found a separate resident-shard shutdown race that also reproduced on the
untouched base. This local fix is not live R7 qualification, and publication
is pending the required unavailable private-terms scanner.

`internal/campaign/recurse_mission.go` ferries observations between recurse and
`internal/deliberation`. Its policy, persistence, metering and scope adapters are
separate files. `internal/core/defaults/schemas_mission.mg` is explicitly loaded
by the kernel schema inventory; `policy/mission.mg` derives stopping, execution,
freshness, concurrency and scoped gate evidence. Mission inputs are homed on the
campaign shard in `internal/shards/registration.go`, filtered at quiescent boot
and protected from model updates and learned rules.

`cmd/nerd/cmd_campaign_mission.go` and `cmd/nerd/chat/campaign_recurse.go` connect
the same loop to registered roles, the campaign orchestrator and live delegated
progress. The campaign carries the mission independently of task ordinals and
injects it on every worker dispatch, including remediation and refinement.
Existing Guardian/risk preflight and acceptance remain on that path. Optional
error-free gate measurement retains existing CLI flag behaviour; chat does not
silently acquire that guarantee.

`cmd/nerd/cmd_campaign_mission_test.go` defines the full scripted-model fixture,
including an observed failing witness before repair. Its persisted config uses
the accepted Ollama provider without an embedding model, so boot skips embedding
initialization locally. The fixture reloads that file through `LoadUserConfig`
before boot, matching the campaign adapter's checked configuration path, and
asserts the exact repaired source remains after the cycle. Policy fixtures cover
stop bounds, unknown usage, write scope, historical evidence and interrupted
intent refusal. Nested usage captures preserve the enclosing mission meter
through broker captures (`internal/usage/observer.go`). No build, test, lab or
live-model result is asserted here. Prompt atoms changed; embedded-corpus
regeneration and execution validation remain required.

The larger innovation remains the target: explicit requirement IDs and reviewed
behavioral contracts, dependency-based evidence validity, runtime symptom
receipts, controlled holdouts and isolated concurrent experiments need their
own production evidence. This path supplies mission-shaped selection and
historical gate-deficit receipts; it keeps the mission open.


## Accepted response body-read uncertainty

Local candidate `567d7926565626a24082a7416d11e4144fb03abe` extends uncertainty
observation to unreadable HTTP 200 bodies in compatible non-streaming chat.
`internal/campaign/recurse_mission_body_read_test.go:16` verifies an accepted
truncated body followed by a successful retry through the real broker, mission
meter and kernel. The answer and known 2,000 micro-USD survive, uncertainty is
persisted and the next admission stops `/unknown_spend`; clean responses remain
admitted. This is a passing offline control, not proof the failed attempt was
billed, a universal transport guarantee or a hard billing cap. Existing phase
limits, ownership and recovery remain unchanged. See GAP-PERCEPTION-ACCEPTED-READ.
