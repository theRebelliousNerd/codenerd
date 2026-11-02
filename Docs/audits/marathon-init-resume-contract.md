---
doc-class: deep-dive
subsystem: cli-init-prompt-marathon-config
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-02
verified-against: 60bb283d8a81656f99d4bab1b1dd504192d219ff
verification-method: static-source-inspection
supersedes: []
---

# Marathon initialization and resume contract

## Decision

Make an explicit marathon request survive the boundary between ordinary workspace initialization and the optional corpus-optimization pass. Both stages must consume correctly resolved configuration, and completion of the first stage must not silently cancel the second stage on a retry.

At the pinned revision, the normal CLI path has a concrete wiring failure. After successful standard initialization, `runMarathon` receives the **workspace root directory** and passes it to `buildMarathonEmbedder`. That helper passes the directory directly to `LoadUserConfig`, whose argument is the filename to read, not a directory to search. On an ordinary existing workspace directory, the read fails and the error returns before construction of the marathon embedding engine, admission configuration, or invocation of `marathon.Run`. The separately defined admission helper contains the same argument mismatch, but it is downstream of the first failure. This is not evidence that normal CLI execution silently disabled TypeSafe and emitted unjudged variants. [CLI ordering][cli-run] [Both helpers][cli-config] [Loader contract][config-load]

The retry boundary compounds that failure. Standard initialization has already saved `.nerd/profile.json`. `IsInitialized` tests only whether that path can be statted; a later `nerd init --marathon` without `--force` returns `nil` from the already-initialized branch before either initialization or marathon runs. `--marathon-restart` does not bypass that branch. `--force` bypasses it but reruns standard initialization and reaches the same bad configuration load if initialization succeeds again. [Already-initialized gate][cli-gate] [Profile publication][profile-write] [Initialized predicate][initialized]

The recommended repair is deliberately bounded:

1. Resolve and validate the intended configuration **file** once at a clearly owned boundary; carry the effective configuration, or an explicit filename with equivalent validation, into both marathon adapters.
2. Separate ordinary-init readiness from the requested marathon stage. Preserve the ordinary `nerd init` no-op behavior where appropriate, without treating that no-op as completion of an explicitly requested marathon.
3. Preserve all existing model-identity, grounded-research, citation, atom-size, optional TypeSafe, and persistence checks. Do not restore reachability by suppressing errors, accepting a nil embedder, inventing model defaults, or bypassing admission.
4. Make retry, resume, restart, and force-reinitialization distinct operational intents. Do not advertise `--force` as a proven universal overlay reset without establishing its row-ownership and checkpoint behavior.
5. Qualify the change through the normal command lifecycle with scripted dependencies before spending on live model quality experiments.

These are proposed changes, not an implemented fix.

## Evidence and scope

All repository links in this document pin public repository `theRebelliousNerd/codenerd` at **`60bb283d8a81656f99d4bab1b1dd504192d219ff`**. The entry source, configuration loader, initializer phases, marathon package, admission logic, relevant guidance, architecture, and selected previous audits were read directly at that pin. No repository program, build, test, benchmark, model inference, training, provider request, or failure-injection experiment was run. Only fetched source caches and this standalone Markdown artifact were written; no repository was edited.

“Source-predicted” below means an execution consequence derived from control flow under stated preconditions. It is not a reproduced CLI incident. “Package control” means a direct library/helper call which does not establish normal command reachability. “Proposed” describes an acceptance condition or design choice not built by this audit.

The principal witness assumes:

- ordinary CLI startup, argument parsing, logging, and workspace resolution succeed;
- the selected workspace is an existing directory, with readable, valid configuration at the intended location;
- the configured standard-init dependencies are usable, standard initialization reports success, and its retained main/worker client is nonnil;
- the workspace and configuration are not concurrently replaced during this trace;
- cleanup-only mode is not requested.

These conditions isolate the marathon handoff. Earlier startup, validation, orientation, embedding, or initialization failures are real alternate outcomes, not evidence against the handoff defect. The executable performs startup configuration checks before command execution, and root pre-run logging can also fail; this audit does not bypass those gates to claim an unconditional command result. [Executable startup][startup] [Initializer success gate][cli-success]

Scope stops at faithful delivery of an admitted marathon request into the existing package, plus the public retry/resume meaning surrounding that delivery. The downstream package is inspected to preserve its controls and design a fair acceptance plan. This document does not certify live grounding, model semantic quality, vector retrieval, compiled-prompt inclusion, cost savings, overlay-reset completeness, or crash-safe agreement between the database and checkpoint.

## Architectural intent and related work

The root guidance treats the model as the creative center and Mangle as the executive, requires production-entry validation, and warns against inferring runtime reachability from the mere existence of a helper. Configuration guidance treats explicit configuration as executable policy rather than something to replace silently with defaults. Init guidance distinguishes required failures from optional degraded enrichment, preserves user-owned artifacts, and favors incremental reinitialization. Prompt guidance keeps atom schema, tags, selection, capability gating, and lifecycle ownership aligned. The relevant architecture rules require intended capability, actual implementation, and verification gaps to remain distinct. [Root guidance][root-guidance] [Configuration guidance][config-guidance] [Init guidance][init-guidance] [Prompt guidance][prompt-guidance] [Architecture rules][arch-guidance]

Marathon's architecture describes a post-init pass which researches the serving model and creates model-pinned variants, with resumable progress and no changes to shipped YAML. That is a useful target. It is not proof that the CLI reaches the package. Its older “one LLM call per atom,” fixed corpus-size, restart, and force-reset descriptions also need to be read against current source rather than treated as measured current behavior. The current init overview supplies the relevant phase ordering and artifact consumers, while its wiring document explicitly labels historical sections superseded by newer status tables. [Marathon target][marathon-doc] [Current init overview][init-readme] [Init wiring history][init-wiring]

Bounded duplicate screening covered the 201 file paths under the pinned `Docs/audits/` tree, the audit queue, and the following selected related reports. This is not a claim to have reviewed every audit body, historical branch, deleted ref, or private issue.

| Related work | Existing concern | Boundary owned here |
|---|---|---|
| Document extraction coverage resumption | Partial chunk acquisition granting unchanged-document skip eligibility | An ordinary-init profile makes a separately requested marathon stage unreachable on retry. No chunk extraction is needed. |
| Prompt evolution lifecycle and evidence accounting | Candidate quarantine, trials, publication, evidence claims, and served-candidate attribution, including as-built repairs | Marathon CLI admission into its package, not automatic evolution, candidate promotion, or exposure credit. |
| Prompt evolution screening cohort | Cohort identity and screening expense before generation; the report has implemented status | Config-path and stage-entry continuity, not repeated Jev screen purchases. |
| Exact model pin tag identity | Lossless exact serving identity and selector/evidence scope | The normal command fails before serving identity is queried. Marathon's family-pin policy is preserved as a separate existing decision, not relabeled exact pinning. |
| Embedding representation and migration | Vector identity, compatibility, partial migration, and as-built repairs | Correct delivery of configured embedding construction at the CLI handoff, not vector-space migration or proof of semantic retrieval. |

[Audit queue][audit-queue] [Extraction audit][prior-extraction] [Evolution lifecycle][prior-evolution] [Screening cohort][prior-screening] [Exact-model audit][prior-model] [Embedding audit][prior-embedding]

No selected report establishes this complete entry-and-retry contract. That limited observation supports a focused refinement, not a universal novelty claim or reopening of their repaired baselines.

## The actual command lifecycle

### The ordinary initialization gate precedes marathon

`initCmd.RunE` calls `runInit`, which calls `runInitWithLLMConfigurer` with the production `configureInitLLM`. The command derives its operation context, resolves the workspace, and handles cleanup-only mode. It then checks `IsInitialized(cwd) && !forceInit` and returns successfully if that condition holds. Neither `marathonInit` nor `marathonRestart` participates in this condition. [Command registration and flags][cli-flags] [Early gate][cli-gate]

If the branch does not return, the command constructs `DefaultInitConfig(cwd)`, obtains ordinary-init application configuration through `loadCampaignConfig(filepath.Join(cwd, ".nerd"))`, validates custom-agent and preference flags, and configures clients. The production configurer selects the worker slot when usable, otherwise the main client, and separately supplies the planner when configured. The marathon receives `config.LLMClient`, not `PlannerClient`. A repair should retain that serving-client provenance unless the architecture deliberately changes the policy. [Init setup][cli-setup] [Client routing][cli-clients]

`loadCampaignConfig` expects a **nerd directory** and appends `config.json` before calling `LoadUserConfig`. Its source also discards loader errors and substitutes defaults when the returned pointer is nil. That existing behavior is not a reason to reuse it as a proposed strict marathon preflight: the desired repair must preserve explicit error disposition rather than merely copy the filename join along with error suppression. Nor does this helper's existence imply that every production startup validation is skipped. [Ordinary-init helper][campaign-config] [Executable startup][startup]

### Standard initialization has real side effects before marathon

The initializer is created, its `Close` is deferred, and `Initialize(ctx)` runs. A returned error or a false `InitResult.Success` prevents marathon. Only after both checks pass does the command branch on `marathonInit`. This preserves the intended “after standard init, not instead of it” ordering. [Success and handoff][cli-success]

The ordinary pipeline includes migrations, directory and database setup, scanning, orientation, profile creation, facts, project atoms, corpus database initialization, agent creation, project tools, preferences, session state, registry work, prompt synchronization, and final validation. These phases can read or write durable workspace state and can make configured model or research requests. They are not a read-only marathon preflight. [Pipeline order][init-pipeline] [Phase side effects][init-phases]

The retry-relevant side effect is especially direct: phase 4 writes `.nerd/profile.json`; `IsInitialized` later returns true whenever `os.Stat` of that path succeeds. It does not inspect marathon flags, checkpoint existence, checkpoint completion, a marathon failure, or even the profile's contents. This is a readiness proxy for the first stage, not a receipt for the combined request. [Profile write][profile-write] [Initialized predicate][initialized]

Phase 5c initializes `.nerd/prompts/corpus.db`, and finalization computes success after validation. Consequently, the source-predicted “standard init succeeded; marathon configuration failed” outcome is **partially completed at the command-request level**, not a rolled-back initialization. No rollback of the successful profile or other standard-init effects is performed by `runMarathon`'s configuration error path. [Corpus setup][init-corpus] [Finalization][init-finalize] [Marathon error return][cli-run]

This is narrower than claiming every failed initialization creates the profile. Directory or orientation failure can stop before profile creation. Conversely, a later standard-init failure can occur after profile creation; that broader ordinary-init readiness issue is not required for the marathon witness.

### Three similar path parameters have different contracts

| Value | Intended meaning | Actual consumer |
|---|---|---|
| `cwd` / `workspace` in `runMarathon` | Project root directory | Used for the overlay and checkpoint under `.nerd/prompts` |
| `nerdDir` in `loadCampaignConfig` | Workspace's `.nerd` directory | Helper appends `config.json` |
| `path` in `LoadUserConfig` | Exact configuration file path | Passed directly to `os.ReadFile` |

The first two are directory-valued strings. The last is file-valued. No overloading in `LoadUserConfig` detects a directory, searches its children, or automatically adds `.nerd/config.json`. The actual workspace resolver returns an absolute workspace path; it does not reinterpret a marathon workspace as its configuration file. [Workspace resolution][workspace-resolve] [Loader][config-load] [Correct ordinary join][campaign-config] [Marathon workspace use][marathon-open]

Go's `os.ReadFile` contract is to read the named file and report a path error on failure. It is not a configuration-discovery API. The application-level mismatch is therefore visible without relying on a provider, model, database, or elaborate race. The exact operating-system error text is not part of this audit's acceptance condition. [Go ReadFile documentation][go-readfile]

### The first marathon failure is a configuration read

The actual order inside `runMarathon` is:

1. Reject a nil client.
2. Print the marathon heading.
3. Call `buildMarathonEmbedder(workspace)`; return on error.
4. Call `buildMarathonAdmission(workspace)`; return on error.
5. Invoke `marathon.Run` with the workspace, client, engine, admission, atom cap, and `Resume: !marathonRestart`.
6. Translate package sentinel errors or print its returned summary.

Both setup helpers call `LoadUserConfig(workspace)`. For the ordinary directory witness, the embedding helper returns `--marathon: load config: ...`. It does not reach `GetEmbeddingConfig` or `embedding.NewEngine`; the admission helper is not called at all. [Run ordering][cli-run] [Helper bodies][cli-config]

This yields precise negative conclusions for this attempt:

- no **marathon** engine construction or engine-readiness request occurs;
- no marathon TypeSafe client is resolved;
- no marathon model identity is queried;
- no marathon grounded-research or atom-optimization request occurs;
- no marathon checkpoint is loaded or saved;
- no variant is emitted by this marathon call.

Those are stage-local conclusions. Standard initialization has already had its own embedding, model, research, corpus, and filesystem opportunities. “No marathon work” must not be reported as “no model calls,” “no network activity,” “no writes,” or “no expense” for the whole command.

### The admission mismatch is latent but real

Fixing only the first helper's path would expose the second helper's identical directory/file mismatch, assuming engine construction then succeeds. Admission is therefore part of the same handoff repair, not an optional follow-on clean-up. [Both helper calls][cli-config]

The existing admission construction is meaningful and should be retained. It resolves a TypeSafe client from the configured section and supplies the effective JIT atom-size bound to `atomjudge.AdmissionFor`. An absent TypeSafe section deliberately yields no client; an invalid present section or unavailable named credential is an error, not a silently disabled client. With a usable client and atom checks enabled, the admission carries the judge and threshold. With no judge, a within-bound candidate is explicitly `Unjudged`; the size check still applies. Judge-reading failure also has an explicit unjudged disposition in the existing policy. [TypeSafe adapter][typesafe-adapter] [TypeSafe resolution][typesafe-resolve] [Admission construction and checks][admission]

This audit neither upgrades that policy to universal fail-closed semantic judgment nor claims that optional unjudged admission is a new bypass. A path repair must honor the user's present section, `atom_checks`, threshold, and `jit.atom_max_chars`; it must not accidentally turn a configured judge into the absent-section control.

### Missing-path controls are not normal CLI witnesses

`LoadUserConfig` creates a nonnil empty `UserConfig` and returns it without error when the named path does not exist. It also installs several default process policies in that branch. The helpers' `if appCfg == nil` fallback is therefore not taken for this case. [Missing-file behavior][config-load]

A direct call to `buildMarathonAdmission` with a nonexistent pathname can consequently construct size-only admission from default JIT settings and a nil TypeSafe section. That would be a useful helper control, **not** evidence that an actual existing workspace with an explicit TypeSafe configuration is silently treated that way by the normal CLI. The observed source path is an existing directory read error first. [JIT defaults][jit-defaults] [TypeSafe adapter][typesafe-adapter]

Likewise, passing a configuration filename directly as the helper's nominal `workspace` can make that isolated helper read the right bytes; it does not repair `runMarathon`'s workspace contract. Passing that filename as the whole marathon workspace would redirect subsequent `.nerd/prompts` path construction under a file-valued path. A test that succeeds by hand-building package configuration or substituting a filename has bypassed the defect it needs to test. [Marathon output paths][marathon-open]

## Retry resume restart and force

### Source predicted state transitions

Let `I` mean `IsInitialized(workspace)` returns true, not that all requested work has completed. Let `M` mean a marathon was explicitly requested. Let `F` mean `--force`. The following matrix concerns the inspected command branch after successful startup; all outcomes are unrun source predictions.

| Prior state and request | First relevant branch | Marathon consequence |
|---|---|---|
| `I=false, M=true, F=false`; ordinary init succeeds, client nonnil | Standard init then marathon setup | Workspace-directory config read fails before `Run` |
| Same workspace after that failure; `M=true, F=false` | Existing profile triggers already-initialized return | Returns `nil`; no marathon attempt or checkpoint read |
| `I=true, M=true, F=false`, plus `--marathon-restart` | Same already-initialized return | Restart flag does not reach `Resume: false` |
| `I=true, M=true, F=true`; reinit succeeds | Force bypasses the profile gate and reruns ordinary init | Same incorrect directory read remains |
| `I=true, M=false, F=false` | Ordinary already-initialized return | Normal init no-op; no marathon was requested |
| Earlier ordinary-init error or false success | Error/success gate before marathon | Marathon correctly does not start |
| Standard init succeeds but retained client is nil | `runMarathon` nil-client gate | Explicit client error before either helper |
| Cleanup-only requested, even with marathon flags | Cleanup branch returns before readiness test | Cleanup is standalone; marathon is not reached |

[Command gates][cli-gate] [Success checks][cli-success] [Run configuration][cli-run]

The second row is the operational problem: a second process can exit successfully after the first process failed the marathon handoff, without attempting the requested remaining stage. The CLI does print that the project is already initialized and suggests force; it does not explicitly announce a completed marathon. The defect is skipped requested work and ambiguous combined-operation success, not fabrication of a marathon success line.

### Package resume exists but is downstream

Once `marathon.Run` is reached, its package has real resume machinery:

- It validates workspace, client, and a positive admission bound before doing research.
- It resolves serving identity, checks grounded-search support, and researches before opening the overlay or reading the checkpoint.
- It loads eligible, unpinned shipped bases and sorts them by ID.
- With `Resume=true`, it reads `marathon_checkpoint.json`; absent or unreadable state starts fresh, with a warning for malformed JSON.
- It compares the checkpoint's **model pin**, not the full raw provider/model pair, to the new profile pin. `modelPinToken` deliberately prefers a family token when available.
- For a matching checkpoint, only base IDs whose recorded content hash equals the current base hash are skipped.
- Successful unchanged decisions, admitted emitted variants, and refused variants are marked decided; optimization, admission errors, and emit failures are not.
- It attempts a checkpoint save after each processed atom and logs save failure rather than returning it as a fatal package error.

[Package entry and loop][marathon-run] [Checkpoint implementation][checkpoint] [Family-pin selection][research] [Existing package tests][package-tests]

These facts prevent two overclaims. The package is not devoid of resume protections, and a package resume does not mean “no repeated research”: the three-query research phase precedes checkpoint loading on every reached run. The fixed corpus-size claim in older documentation is not used as a current work estimate. The selected corpus size depends on current embedded contents and filtering. [Research queries][research] [Base selection][select-bases]

Package `Resume` is a bool; the CLI explicitly sets it to the inverse of `marathonRestart`. A direct caller omitting that bool obtains Go's zero value, false, regardless of the field comment's “default true” wording. The CLI default is clear, but it cannot matter until command dispatch reaches `Run`. [Config shape][marathon-config] [CLI adapter][cli-run]

### Force is not a dedicated resume or proven universal reset

At the command boundary, force has a definite effect: it bypasses the already-initialized return. The same initializer is then constructed and run; no separate marathon-only force branch appears here. Incremental reuse can reduce actual reinit work, but that is not equivalent to skipping the standard-init lifecycle. User-owned configuration, templates, preferences, and curated prompts retain their existing protection contracts. [Force branch][cli-gate] [Init guidance][init-guidance]

The CLI completion text and marathon architecture say force discards the overlay. This audit does **not** rely on that broad statement as a proven current reset contract. The inspected corpus initializer seeds an absent database without overwriting an existing one, then reconciles canonical embedded rows and ingests project rows. Reconciliation deletes nonembedded IDs only when their `source_file` is nonempty; NULL or blank ownership is retained. It also handles errors as documented at the caller. Establishing exactly which historical or newly emitted marathon rows are reset requires tracing their persisted ownership and the reset's interaction with the checkpoint, not merely quoting the completion message. [Init corpus setup][init-corpus] [Reconciler ownership][reconciler] [Reset message][cli-summary]

Accordingly, do not instruct an operator to delete the profile, config, corpus database, or checkpoint as a casual workaround. Such deletion changes readiness, user-owned state, or resume evidence and may repeat paid work. Nor does `--marathon-restart` mean “rollback”: at the package boundary it starts a fresh in-memory decision map; its broader overlay and prior-state effect must remain separately specified. [Checkpoint fresh path][checkpoint]

## Existing controls that a repair must preserve

| Control | Current source meaning | What it does not prove |
|---|---|---|
| Ordinary-init error and success checks | Marathon follows successful ordinary init | No rollback or combined-operation completion receipt |
| Nonnil LLM client | Marathon cannot start without a supplied client | That the client supports grounding or reports a sufficient identity |
| Positive admission bound | `Run` rejects absent/nonpositive `MaxChars` | Universal semantic approval |
| Serving identity | Client must implement the identity interface; both provider and model cannot be blank | Exact immutable model-version pinning; the implementation does not require both strings independently nonblank |
| Grounding capability | Required interface and positive support flag | Availability or accuracy of every search result |
| Profile documentation | Nonempty guidance plus at least one citation; optimizer checks again | Official-source authenticity, semantic coverage of every rewrite, or independent quality validation |
| Size and TypeSafe admission | Size is enforced; configured enabled judgment is used, with explicit unjudged policy | A new mandatory judge or permission grant |
| Model selectors and new variant ID | Rewrites are represented as variants, preserving other cloned atom metadata and clearing obsolete short renderings | Guaranteed selection, retrieval, or useful downstream behavior |
| Content-hash checkpoint | Matching decided bases can be skipped after reaching the package | Atomic agreement with database writes, complete CLI resumption, or proof every saved decision still has its output |

[CLI gates][cli-success] [Package preconditions][marathon-run] [Identity and grounding][research] [Documentation condition][marathon-types] [Optimizer and variant construction][optimizer] [Admission][admission] [Checkpoint][checkpoint]

The final row matters to scope. Database publication and JSON checkpoint saving are separate operations; checkpoint-save errors are warnings in the inspected loop. That is enough to require careful recovery acceptance, not enough to claim a reproduced loss or prescribe a new distributed transaction subsystem. The entry defect can be repaired without pretending downstream crash recovery has been established.

## Proposed bounded operational contract

The following is a design contract for review, not a new configuration schema, importable API, or shipped command.

### Configuration identity belongs to the operation

The orchestration boundary should establish three distinct values: resolved workspace root, resolved configuration source, and validated effective configuration. Directory-valued and filename-valued parameters should be visibly different in names or types. Both marathon adapters must receive the same intended configuration observation, preserving explicit embedding endpoint/model/task settings, TypeSafe enablement and credentials, and JIT admission limits.

Two reasonable implementation choices are available:

- **Small path repair:** resolve `workspace/.nerd/config.json` explicitly in both adapters through an error-preserving helper. This fixes the concrete mismatch but leaves two reads and the possibility of configuration changing between them.
- **Shared resolved configuration:** resolve once and pass a read-only effective configuration or narrow validated embedding/admission settings. This avoids accidental adapter divergence and duplicate parse/publication effects, but requires an explicit lifetime and ownership contract because the existing loader also publishes process-wide policy.

The second option is preferable where the entry already owns an effective configuration, provided it does not accidentally carry a stale pre-init snapshot over an intentional configuration change. It is not a demand to serialize secrets or log a full config. If a nonsecret configuration generation is recorded, use an appropriate protected digest or selected nonsecret fields and never expose credentials.

Define absence, malformed bytes, wrong path type, unreadable file, and invalid explicit sections separately. Do not translate every error into “not configured.” Do not substitute global or ambient settings without the owning configuration precedence contract. Standard init has its own workspace/global embedding resolution; “mirror init” must be proved from the effective choice, not asserted in a comment. [Init embedding resolution][init-embedding] [Loader effects][config-load] [Config guidance][config-guidance]

### Readiness and requested work are separate

An already-initialized workspace should be eligible for an explicitly requested marathon after the required corpus/readiness checks, without forcing unrelated reinitialization solely to make the request reachable. Existing `nerd init` without marathon can keep its ordinary no-op semantics.

A bounded state model is enough:

- ordinary init is needed, complete, or failed;
- marathon is not requested, not started, running, partial/retryable, or complete for a declared processing scope;
- an explicitly requested restart differs from continuation;
- a force reinitialization differs from resetting marathon artifacts.

The exact representation can be a typed result and durable stage record where required; this audit does not mandate a new database or framework. A profile's existence must never be the sole proof that the marathon stage completed. A failed marathon setup must remain retryable without erasing ordinary-init achievements.

If the product instead chooses to reject marathon on already-initialized workspaces, it should return an explicit non-success disposition naming the requested stage and a supported, non-destructive continuation route. Silently returning ordinary-init success is the weak option. A new standalone marathon command is a possible API design, not assumed current functionality.

### Preflight should prevent avoidable cost without reordering publication

Cheap checks that can be performed before expensive standard initialization should identify obvious marathon ineligibility: valid configuration source, required configured embedding model, candidate client identity/capabilities, and valid explicit admission settings. A preflight should not issue a paid documentation search merely to check a type assertion, and it must not open or publish the overlay before the required ordinary-init corpus lifecycle.

Keep stage outcomes explicit if prerequisites can only be established after init. A pure eligibility check, live engine readiness, research success, per-atom judgment, persistence, and actual prompt inclusion are separate observations. In particular, engine construction is not proof of a healthy vector path: the inspected Ollama constructor treats an initial ensure failure as retryable at first embed. The normal mismatch fails even before that constructor. [Embedding bootstrap behavior][embedding-engine]

The existing command context preserves owner cancellation and an explicitly supplied whole-command `--timeout`; there is no default whole-command deadline in the inspected operation-context helper. A repair should preserve that policy and bound individual external requests through their owners rather than invent a permanent arbitrary run cutoff. The marathon helper currently uses the contextless embedding constructor; cancellation coverage of any refactored setup needs its own explicit acceptance case. [Operation context][operation-context] [Embedding guidance][embedding-guidance]

### Resume and restart need receipts that match their claims

At minimum, report:

- which stages were requested and which ran;
- whether ordinary initialization was reused or rerun;
- configuration source and nonsecret effective engine/admission identity;
- raw serving identity and the actual pin granularity used;
- checkpoint disposition: absent, resumed, incompatible, unreadable, ignored for restart, or save failed;
- count of attempted, unchanged, refused, emitted, failed, and resumed bases;
- whether the requested atom scope completed, stopped at the explicit cap, or remains retryable;
- completed durable effects and the next supported action on failure.

Do not call a bounded pass a completed whole corpus merely because the function returned no error. Do not count a refused atom as emitted. Do not turn unknown token accounting into zero. The present package already carries several useful counts, while its CLI summary prints a subset; completing reporting should conserve that information rather than inventing new success criteria. [Result shape][marathon-types] [Loop accounting][marathon-run] [CLI summary][cli-summary]

For restart, state whether prior overlay rows are reused, replaced, or retired and when the checkpoint becomes authoritative. For reset, define ownership and deletion boundaries and preserve unrelated project atoms and user files. For cancellation or a crash between atom storage and checkpoint publication, reconcile existing outputs before claiming “no repeated work” or “exactly once.” The smallest entry fix need not solve every downstream issue, but its documentation must not overclaim those guarantees.

The primary retry lesson from Amazon's Builders' Library is useful here: absence of a duplicate side effect is weaker than a response that faithfully represents the caller's intended operation, and retry identities must distinguish different intent. That motivates stage-specific outcomes for this CLI; it does not import AWS's API or transaction design as a mandatory implementation. [Making retries safe with idempotent APIs][aws-retries]

## Failure controls and operational costs

### Preserve helpful failures

A corrected path must still refuse a malformed or invalid explicit configuration. A present unusable TypeSafe section must not become the absent-section case. Missing model identity, unavailable grounded search, and a profile without citations must still stop before variant publication. Oversize candidates must remain refused independently of optional judgment. A nil client must still be explicit. These are protections, not obstacles to make a test green. [Config parsing][config-load] [TypeSafe resolve][typesafe-resolve] [Package gates][marathon-run] [Research][research] [Admission][admission]

A failed setup should name the actual failed stage and the configuration source without leaking keys. Ordinary-init achievements should remain visible, because they affect the correct next action. Repeating the same unchanged deterministic path failure automatically is not useful recovery; it repeats initialization expense without creating new evidence. Retry may be appropriate after an operator or code/config change removes that specific blocker.

### Cost is a whole request property

For the source-predicted first attempt, marathon search, optimization, and per-variant judgment expenses are zero because those stages are not reached. Standard-init expense is not zero and is not measured here. It can include scanning, orientation, configured model requests, topic research, embedding work, schema/corpus operations, and durable writes. The exact bill depends on the workspace, configuration, cache state, and optional phases. [Init pipeline][init-pipeline]

A forced retry repeats entry into that lifecycle. Existing incremental reuse can avoid some provider work, so “repeats every call” would be inaccurate; it nevertheless incurs reinitialization checks and whichever phases do not reuse their previous outputs. A plain retry can be cheap precisely because it skips the requested marathon, so latency or spend alone would reward the wrong behavior.

After reachability is repaired, cost should be separated into:

- ordinary-init work required or rerun;
- configuration/preflight and engine construction;
- documentation research, currently three queries per reached package run;
- optimizer calls for nonresumed bases;
- optional TypeSafe judgments and failed/unjudged attempts;
- embedding work for emitted variants, including provider retries;
- SQLite writes, checkpoint serialization/atomic replacement, and recovery work;
- later validation and any task-quality experiment.

[Research query loop][research] [Optimization call][optimizer] [Per-atom loop][marathon-run]

`--marathon-max-atoms` bounds newly processed loop outcomes—optimized, unchanged, refused, or failed—not the preceding standard-init work or documentation research. Resumed entries do not consume this counter. A small cap is therefore not a whole-command spend cap, and many small resumed runs can repeat the research cost. A zero cap means the whole selected corpus; no current atom count or dollar estimate was measured here. [Budget condition][marathon-run]

The proposed shared configuration boundary adds a small resolution/validation cost and potentially a protected identity record. Stage receipts add schema/versioning, read/write, and migration responsibilities. Resume without forced reinit can reduce redundant work but also requires readiness validation and clearer ownership. More precise recovery can spend I/O and complexity to avoid repeated inference. None of those tradeoffs establishes a net saving until measured on matched requests.

## Fair verification plan

All cases in this section are proposed and **unrun**. Implementing or executing them requires a separate authorized engineering task.

### First establish the entry contract without live inference

Use a disposable, file-backed workspace with explicit configuration and controlled model, embedding, and judgment transports. Keep the real workspace resolver, flag dispatch, configuration loader, initializer success gate, marathon adapter, and package entry in the path under test. Script only external responses and unavoidable test dependencies. Do not hand-build an admission object as the sole proof that the CLI used the requested TypeSafe/JIT policy.

The repository already contains a normal-init fixture with an HTTP fake serving chat and embedding routes and recording phase requests. That is a useful starting pattern, not evidence of marathon coverage: the inspected fixture concerns planner routing, and a full marathon fixture also needs actual supported grounding behavior or an explicit capability-preserving boundary seam. Tests using a nil LLM configurer can establish filesystem behavior but cannot prove the nonnil-client marathon path. [Existing initializer test seam][cli-entry] [Planner-entry fixture][entry-fixture]

The existing package test directly invokes `Run` with fake clients, explicit `Admission{MaxChars: 1500}`, and no embedding engine. It checks pin persistence and then directly invokes `Run` again with `Resume: true`. Those are useful package assertions. They bypass `IsInitialized`, both setup helpers, filename resolution, configured engine construction, configured admission, and normal CLI retry. Keep them; add the missing command witness rather than treating them as an end-to-end command pass. [Package overlay/resume test][package-tests]

### Acceptance matrix

| Case | Required evidence |
|---|---|
| Fresh initialized-through-command workspace, marathon requested | Intended config file was read; configured engine and admission reached the package; workspace root remained the output root |
| Existing workspace, marathon requested without force | Requested marathon is attempted or explicitly refused with a stage-specific non-success result; no silent ordinary-init-only success |
| Standard init succeeds, marathon setup fails, then dependency/config is repaired | Same ordinary request can continue under the documented contract; completed init is not erased or falsely reported as marathon completion |
| Existing workspace with restart flag | Restart intent reaches the documented marathon operation instead of being swallowed by the profile gate |
| Force plus marathon | Actual reinit is visible; all user-owned preservation rules hold; force is not mistaken for a cheap resume |
| Valid file versus directory passed to loader/helper | Filename succeeds where valid; wrong path kind produces a clear configuration error, not default fallback |
| Missing file, malformed JSON, unreadable file, invalid explicit section | Each follows its declared precedence/error policy; absent and invalid are not conflated |
| Workspace A selected while process CWD/config belongs to B | All source/output/config observations belong to A; no ambient-workspace substitution |
| Distinct custom embedding endpoint/model/task and nondefault atom bound | The actual adapter receives the intended values; no hidden provider/model defaults |
| TypeSafe absent, atom checks off, enabled valid section, present invalid section | Deliberate unjudged controls remain distinct from configured judgment and configuration failure |
| Nil client, identity unavailable, grounding unsupported, uncited prose | Existing rejection boundaries remain before variant storage |
| Base changed, unchanged decided base, incompatible pin, malformed checkpoint | Existing package reuse/invalidations are preserved and reported at the command boundary |
| One optimization failure or refused variant within a bounded run | Failed work remains retryable; refused work is not emitted; cap and outcome counts remain truthful |
| Cancellation and checkpoint-save error | Retained durable effects, incomplete state, and next action are explicit; no unsupported exactly-once claim |
| Force/reset with historical and new variants plus unrelated project atoms | Row ownership and checkpoint interaction are measured before publishing a blanket reset guarantee |

The first three rows are the core exit criteria. The remaining controls keep a reachability fix from weakening the surrounding contracts.

Record actual calls and arguments, stage transitions, returned error/disposition, profile and checkpoint state, corpus rows and ownership, and filesystem differences. Preserve logs as evidence but use direct state observations as the oracle. Assert semantic error categories rather than a Linux-specific “is a directory” string.

A useful negative control deliberately restores either wrong-path call and expects the normal-entry fixture to fail. Another restores the old unconditional initialized return and expects the retry fixture to fail. These establish that the new tests are sensitive to the audited boundary, not merely able to run a fake `marathon.Run`.

### Compare behavior before optimizing cost

Compare at least these source/implementation treatments:

1. pinned baseline;
2. file-path repair only;
3. complete configuration and requested-stage retry repair.

Use identical workspace inputs, configuration, scripted external outcomes, selected corpus, atom cap, and existing checkpoint state. The path-only treatment is important: it may make a first run reach the package while still swallowing the next ordinary retry. Do not use its first-run success as proof that the resumability contract is repaired.

Measure stage reachability and correct terminal disposition first. Then measure standard-init repetitions, research/optimization/judgment/embedding attempts, tokens where actually reported, filesystem/SQLite/checkpoint writes, CPU, allocations, retained state, and wall time. Account for failed requests and recovery, not just successful emitted variants. Label absent usage measurements unknown.

Only after this contract is established should an authorized live study compare shipped atoms and marathon variants on matched held-out tasks with the same model, tools, policy, context budget, and workspace. Include semantic-constraint preservation, retrieval and actual compiled inclusion, verified task outcome, total provider expense, and human review burden. Separate generation data from evaluation data. A variant being stored or a self-reported rationale sounding plausible is not proof of better behavior.

## Exit criteria and remaining limits

The bounded repair is acceptable when the normal CLI uses the intended configuration file for both marathon adapters, an explicit marathon request is not swallowed by ordinary-init readiness, a setup failure can be retried under a documented non-destructive route, and the existing guards continue to produce their distinct outcomes. No repository program was run to establish those exits in this audit.

The outcome would be faithful operation entry and continuation—not proof that every model supports grounded search, every rewrite preserves meaning, every vector is retrievable, every variant is selected, force resets every overlay, or checkpoint/database recovery is complete. Those are separable obligations. Restoring the requested stage without weakening its controls is sufficient scope for this proposal.

## Pinned sources

[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/agents.md
[config-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/config/agents.md
[init-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/agents.md
[embedding-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/embedding/agents.md
[arch-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/architecture/AGENTS.md
[marathon-doc]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/architecture/prompt/15-MARATHON-INIT.md
[init-readme]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/architecture/init/README.md
[init-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/architecture/init/WIRING-AND-NOT-BUILT.md
[audit-queue]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/ASTRA_QUEUE.md
[prior-extraction]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/document-extraction-coverage-resumption.md
[prior-evolution]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[prior-screening]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/prompt-evolution-screening-cohort.md
[prior-model]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/exact-model-pin-tag-identity.md
[prior-embedding]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/Docs/audits/embedding-representation-and-migration.md
[cli-flags]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L36-L104
[cli-entry]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L150-L168
[cli-gate]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L169-L191
[cli-setup]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L193-L228
[cli-success]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L230-L259
[cli-run]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L261-L319
[cli-summary]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L321-L333
[cli-config]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L335-L378
[cli-clients]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L380-L410
[workspace-resolve]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_init_scan.go#L596-L617
[campaign-config]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/cmd_campaign.go#L30-L64
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/main.go#L122-L478
[config-load]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/config/user_config.go#L617-L670
[init-embedding]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/initializer.go#L466-L501
[init-pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/initializer.go#L510-L654
[init-phases]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/phases.go#L58-L209
[profile-write]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/phases.go#L173-L193
[initialized]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/profile.go#L791-L800
[init-finalize]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/phases.go#L527-L548
[init-corpus]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/init/profile.go#L857-L910
[reconciler]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/reconciler.go#L27-L43
[typesafe-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/typesafe/from_config.go#L7-L24
[typesafe-resolve]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/config/typesafe.go#L351-L384
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/atomjudge/admission.go#L16-L147
[jit-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/config/jit.go#L117-L145
[embedding-engine]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/embedding/engine.go#L101-L149
[operation-context]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/operation_context.go#L12-L64
[marathon-config]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon.go#L23-L62
[marathon-run]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon.go#L89-L240
[select-bases]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon.go#L242-L273
[marathon-open]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon.go#L275-L304
[checkpoint]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon.go#L306-L363
[research]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/research.go#L25-L158
[marathon-types]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/types.go#L67-L135
[optimizer]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/optimize.go#L22-L175
[package-tests]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/internal/prompt/marathon/marathon_test.go#L363-L495
[entry-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/60bb283d8a81656f99d4bab1b1dd504192d219ff/cmd/nerd/init_planner_entry_test.go#L23-L84
[go-readfile]: https://pkg.go.dev/os#ReadFile
[aws-retries]: https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-APIs/

