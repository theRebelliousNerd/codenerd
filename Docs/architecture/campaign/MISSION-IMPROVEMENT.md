# Mission-linked recurse

The mission mode of recurse keeps an owner-supplied purpose under continuous
pressure while bounding each invocation. It extends the existing dependency
sweep and ratchet rather than introducing another scheduler. Start it with
`nerd campaign recurse --mission "<desired behaviour>"` or
`/recurse --mission "<desired behaviour>"`. Existing subsystem and pass options
still apply. Ordinary recurse retains its current behaviour.

## Required contract

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

The meter uses the typed `usage.MicroUSDPerUSD` currency conversion from
`internal/usage/pricing.go` for both overflow detection and rounded accounting.
The spend limit remains `mission.max_spend_microusd`; the currency unit is fixed.
Chat command prefixes are token sequences whose lengths define their grammar
in `cmd/nerd/chat/campaign_recurse.go`, independent of routing budgets.

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
