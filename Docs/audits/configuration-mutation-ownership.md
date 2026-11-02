# Configuration mutation ownership across restart-only and live setters

**Repository:** `theRebelliousNerd/codenerd`  
**Inspected revision:** `99d87a96ea0a2d11e8b0e539f4fe9bd223abe682` (the supplied main snapshot)  
**Inspection date:** 2026-11-02  
**Status:** statically supported, not dynamically reproduced; proposed contracts and evaluations below are not implemented or run by this audit.

## Finding

A successful `/embedding set` can be silently undone by the next successful `/yolo` change or Shift+Tab in the same chat session. The embedding command loads and saves a separate `UserConfig` but leaves `m.Config` unchanged. The work-mode setter then saves **the entire old `m.Config`**, not just `work_mode`. This overwrites the newly persisted embedding choice even though both commands can report success. No concurrent process, interrupted write, invalid configuration, model call, or failed disk operation is required. [Embedding setter][embedding] [Work-mode setter][work-mode] [Whole-config save][save]

This is a **sequential lost update between configuration owners**. The atomic private writer already protects complete replacement and owner-only permissions; it does not know that its complete input snapshot predates a different acknowledged change. Preserve that writer. The missing contract is preservation of fields outside the current command's write set, plus a distinction between persisted desired settings and settings actually applied to runtime components. [Atomic writer][atomic] [Config maintenance guidance][guidance]

The existing “Restart to apply changes” notice is not itself a defect: rebuilding an embedding engine may legitimately require restart. The defect is that the requested next-start state can disappear before that restart. This audit does not claim the active engine switches immediately, that wrong vectors have been served, or that a security boundary has been bypassed.

## 1. Pinned production chain

| Boundary | Inspected behavior | Evidence |
|---|---|---|
| Chat construction | `InitChat` loads `config.GlobalConfig()`, defaults if nil, and stores that pointer in `Model.Config`. | [Initial load][boot-load], [model installation][boot-model] |
| Normal slash submission | A normal slash-prefixed submission routes directly to `handleCommand`. Its explicit cases dispatch `/embedding` and `/yolo` to their handlers. | [Submit routing][submit], [embedding dispatch][embedding-route], [yolo dispatch][yolo-route] |
| Embedding mutation | `handleEmbeddingCommand` calls `config.GlobalConfig()` again, changes the returned embedding block, checks `MissingModel`, saves to `m.userConfigPath()`, and reports a restart-required success. Its returned model has no `m.Config = cfg` assignment. | [Embedding setter][embedding] |
| Fresh allocation, not a cache | `GlobalConfig` calls `LoadUserConfig(DefaultUserConfigPath())`; `LoadUserConfig` allocates a new `UserConfig` before reading and decoding. It is not a singleton pointer alias to the model's original object. | [Global loader][global], [allocation and decode][load] |
| Work-mode slash mutation | Bare `/yolo`, `/yolo on`, and `/yolo off` reach `setYolo`, which delegates to `setWorkMode`. `/yolo status` returns before mutation. | [Yolo handler and setter][yolo] |
| Work-mode keyboard mutation | The reachable Shift+Tab branch calls `setWorkMode(m.workMode().Next())`. | [Keyboard branch][keyboard] |
| Persistence sink | `setWorkMode` changes `m.Config.WorkMode`, mirrors mode facts, then invokes `m.Config.Save(m.userConfigPath())`. | [Work-mode setter][work-mode] |
| Save semantics | `Save` marshals the entire receiver and atomically replaces the target. It performs no reload, field merge, base-revision check, or compare-and-swap. | [Save][save], [atomic writer][atomic] |
| Destination | Both setters write the model's active workspace `.nerd/config.json`, falling back only if that workspace is empty. | [Workspace-bound write path][path] |

The ordinary same-workspace case is enough: the process-discovered config path and the model's workspace path point to the same file. The embedding read uses the former and its save uses the latter. A read/write workspace mismatch is an adjacent path-identity concern, not a required premise or an independently demonstrated production incident here. [Path discovery][path-discovery]

### Deterministic source-level witness — not an executed transcript

Start a normally initialized, idle chat in workspace W. Let A and B be two distinct, explicitly supplied valid Ollama model names. Assume the initial file is valid, both subsequent saves return nil, and no intervening action refreshes `m.Config`.

1. Initial durable configuration is `D0 = { embedding: A, work_mode: propose, other: K }`.
2. Chat retains object `M0` decoded from D0.
3. Submit `/embedding set ollama B`.
4. The handler decodes another object, changes its embedding choice, and saves `D1 = { embedding: B, work_mode: propose, other: K }`. It reports that the embedding provider was set and restart is required.
5. The returned model still holds M0, whose embedding choice is A.
6. Submit `/yolo on`. The mode handler changes only `M0.work_mode`, then serializes **all of M0**.
7. Durable configuration becomes `D2 = { embedding: A, work_mode: yolo, other: K }`.

A restart now reads A, not the acknowledged B. A reachable Shift+Tab transition produces the same embedding reversion while choosing its own next mode (from propose, explicit). This is a state-transition deduction from the linked code, not a report that the UI was exercised. The embedding setter permits the chosen model string without constructing a new engine; provider availability is not needed to explain the persistence transition. [Embedding setter][embedding] [Missing-model check][missing-model] [Mode cycle][mode-cycle]

| Point | Persisted desired embedding | `m.Config.Embedding` | Existing embedding engine |
|---|---|---|---|
| Initial session | A | A | Initialized through normal boot, if available |
| After successful embedding set | B | A | Not replaced by this handler |
| After successful work-mode save | A | A | Not replaced by this handler |
| Next restart | A is loaded | New session loads A | Subject to ordinary startup checks |

The same structure can remove a newly introduced embedding block when the model's original snapshot had no embedding block. The stronger minimal demonstration uses a pre-existing valid A, so it does not rely on missing-file behavior or boot degradation.

## 2. Boundaries and counterexamples

- **Restart first:** immediately restarting after the embedding save, before any stale whole-snapshot writer, can load B. The defect is not that every embedding change fails.
- **Same value:** setting A to A has no visible embedding loss.
- **Read-only yolo status:** `/yolo status` does not call `setWorkMode` and is a necessary negative control.
- **Modal keys:** approval review intercepts keys before the global Shift+Tab branch. A focused error panel also swallows most keys. The witness uses ordinary chat state, not a claim that every Shift+Tab always persists. [Key-routing guards][key-guards]
- **Nil model config:** `setWorkMode` returns without saving. Normal initialization supplies a config; nil-config test seams do not refute the initialized-session witness. [Work-mode setter][work-mode] [Yolo tests][yolo-tests]
- **Save failure:** the primary witness assumes nil error. A failure is separately surfaced as session-only mode behavior. Runtime mode is mutated before persistence, so a failed mode save must not be reported as proof that no in-memory change occurred. [Work-mode setter][work-mode]
- **Post-replacement failure:** the private writer performs a final chmod after replacement. Therefore an error from `Save` is not universally proof that the old on-disk bytes remain. Its documented old-file guarantee is for **pre-replacement** failures. Fault evaluations must distinguish these stages rather than assert an unconditional rollback. [Atomic writer][atomic]
- **No concurrency needed:** serializing UI event handlers or adding a lock solely around rename cannot repair this witness. M0 was already stale before the second write began.
- **Not a hot-reload demand:** desired persistence and live application may differ legitimately. The contract must retain that distinction rather than pretend a new pointer makes all existing clients current.

Two adjacent observations help scope future ownership work but are not needed to establish the finding:

1. `setDebugSelf` also saves the entire model config after changing its own logging field. It is another inspected stale-snapshot sink; fixing only the work-mode caller would not establish a universal preservation contract. [Debug-self setter][debug-self]
2. The config wizard loads the current workspace file and merges owned fields before saving, which is a useful existing preservation pattern. Its inspected save helper returns only an error and does not synchronize `m.Config`. The exact wizard/UI sequence is not used as this audit's reproduction. [Wizard merge entry][wizard-entry], [wizard save return][wizard-save]

The embedding setter also discards its load error and falls back to defaults if it receives nil. The primary finding does not require that branch. Do not conflate the healthy sequential lost update with a separate invalid/unreadable-file overwrite analysis. A new shared mutation path should nevertheless avoid carrying that fallback into its contract. [Embedding setter][embedding]

## 3. Existing work and duplication check

The repository was searched at the supplied main snapshot for `setWorkMode`, `embedding set`, `m.Config.Save`, `lost update`, and stale-config combinations. The closest relevant documents were read, and their claims were compared with current source. This is a targeted duplicate check, not an assertion that every historical issue, branch, or audit paragraph was exhaustively reviewed.

| Existing artifact | What it covers | Why this finding is different |
|---|---|---|
| [Embedding representation and migration][representation] | Vector provenance, compatibility, input coverage, conditional vector publication, restart-safe migration; the document includes implementation-status updates. | This finding loses the **requested configuration before activation**. It does not reopen already repaired vector-identity work. |
| [Embedding stats observation boundary][stats-audit] | An observation command performing engine/index maintenance. | This is write-owner composition across two setters, not stats side effects. |
| [Development policy reload adoption][policy-audit] | Identified adoption of hot-reloaded Mangle programs and associated state. | Related desired/applied distinction, but a different object, loader, and publication path. |
| [Config wiring/current-status document][config-wiring] | Config accessor wiring and prior repaired default-publication gaps. | These are not evidence of fresh-base preservation across chat setters; historical claims were not treated as current behavior. |
| [Config-wizard preservation test][wizard-test] | A single wizard save preserves execution, logging and integration fields already on disk. | Useful regression precedent, but does not compose an independent disk-only mutation with a subsequent stale model save. |
| [Yolo tests][yolo-tests] and [Shift+Tab test][key-test] | Yolo toggle/persistence, status/usage/nil cases, mode cycle and session flag semantics. | The inspected tests do not first persist a distinct embedding update and then assert that it survives the mode save. The Shift+Tab test checks model mode, not unrelated saved fields. |

**Status at the pin:** the exact fresh-object embedding producer and whole-model work-mode consumer are present and reachable. No repair is claimed. No repository code, tests, branch, issue, or remote document was changed by this audit.

## 4. Required mutation contracts

### 4.1 Field ownership

For a command whose authorized write set is S, successful persistence must preserve the current durable values outside S, except for explicitly documented schema normalization. In the work-mode case S contains only `work_mode`; in the embedding command it is the documented provider/model/key/endpoint choices.

A valid implementation may reconstruct complete JSON for atomic publication. “Whole file written” is not inherently wrong. The problem is reconstructing from an obsolete whole-file authority rather than the current base plus the intended delta.

For two successful commands with disjoint write sets, their composed durable result should retain both changes. This property is stronger and more maintainable than adding one embedding-specific copy to one caller.

### 4.2 Desired versus applied state

Represent or expose at least:

- the workspace and durable desired config identity;
- the owned requested delta;
- whether persistence succeeded, failed before publication, or has an uncertain post-publication outcome;
- whether the changed section is active now or pending restart;
- the active engine/provider identity when reporting live operation.

Do not eagerly label the engine B merely because the config file now requests B. Equally, do not reuse the live engine's old config snapshot as authority to overwrite desired B.

### 4.3 Error and base handling

Load/parse/validation failures should refuse persistence with a visible reason, not erase the document by starting from defaults. Missing-file creation remains a separate legitimate case.

Any optimistic freshness check must cover the interval through publication. A read, digest comparison, and later uncoordinated replacement still leaves a race against another writer. Decide and state which writers participate: a single event-loop owner, cooperating processes, or arbitrary external editors are different guarantees.

A config reader used only to prepare a candidate should not accidentally activate unrelated settings. This matters because current `LoadUserConfig` also installs process-wide feature and timeout policy after successful validation. Reusing it inside every setter is not a side-effect-free “just read JSON” operation. Preserve those established boot effects while considering a separate validated, non-applying snapshot load. [Loader policy publication][load-publish]

### 4.4 Confidentiality

Configuration can contain API keys and OAuth material. Preserve owner-only replacement, redact field values in diagnostics, and never include raw before/after configs in public test output or audit telemetry. Change identities or field names usually suffice; any persisted diagnostic snapshot needs an explicit private-data policy. [Atomic writer][atomic] [Config guidance][guidance]

## 5. Alternatives and delivery order

| Alternative | What it can establish | Important limitation/cost |
|---|---|---|
| Copy the newly saved embedding config into `m.Config` | Smallest candidate repair for the demonstrated same-session sequence. | Does not preserve external edits or every other producer; may change runtime consumers before engine restart. Requires explicit desired/applied semantics, not blind assignment. |
| Reload the current workspace config and patch only `work_mode` before saving | Closes the demonstrated stale-mode overwrite while preserving pending settings already on disk. | A naive reload has global-policy side effects, and load-to-save races remain with uncoordinated writers. Other stale sinks still need review. |
| A small shared owned-field mutation helper, with current-base loading and explicit publication disposition | Consistent preservation, path selection, errors and desired/applied status across interactive setters. | More integration work. Must avoid turning a local integrity fix into a new general configuration platform. |
| Revision-checked mutation across cooperating writers | Detects stale bases and supports safe rebase/refusal where multiwriter behavior is required. | Lock/CAS protocol, contention, retry policy and external-writer limitations must be specified and tested. |
| Immediate automatic engine rebuild after every setter | Can make some live components reflect desired state. | Not necessary to prevent data loss; adds provider/network work, lifecycle, migration and rollback obligations. Not the default recommendation. |

Start with the two-command preservation contract and honest runtime status. Extend to the inspected debug-self sink and other whole-config producers only under an explicit field-ownership inventory. Avoid “fixing” this by removing atomic replacement, ignoring config validation, automatically uploading content to a new provider, or performing unsolicited reembedding.

## 6. Costs and expected benefit

**Current direct cost:** the setter is deterministic local work. For serialized config size B, whole-config JSON serialization and replacement use O(B) work/output and O(B) serialization memory, plus filesystem and synchronization latency. No LLM inference is required by the demonstrated two setters. These are source-derived complexity statements, not benchmark measurements. [Save][save] [Atomic writer][atomic]

**Repair cost:** reloading and validating a current base adds O(B) read/parse work and filesystem latency; a detached candidate can add O(B) memory. Coordination adds acquisition/wait time and conflict handling. A digest or revision receipt adds comparison/accounting work. None inherently requires an LLM, embedding call, scan or index migration.

**Potential avoided cost:** restoring a lost setting, repeating diagnosis/restart, and possibly repeating maintenance under the wrong configuration. No incidence, saved tokens, elapsed-time reduction, inference spend, or retrieval-quality improvement has been measured. If users run expensive work after the setting is lost, that cost is conditional and must not be presented as observed.

Preservation and truthful status are the primary acceptance criteria. A marginal latency reduction cannot compensate for silently losing a successful change; conversely, a large architecture should not be justified by an invented performance win.

## 7. Fair evaluations — proposed and entirely unrun

### Deterministic functional matrix

Use a temporary workspace and non-secret synthetic configuration. Exercise production handlers/normal Update routing, not a replacement model of the persistence algorithm. Avoid concurrent tests that change process cwd; either isolate the process or establish controlled path discovery before each fixture.

| Evaluation | Required observation |
|---|---|
| `/embedding set ollama B` then `/yolo on` | B remains in saved config; yolo persists; unowned sentinel fields remain equal. Verify both intermediate and final files. |
| Same embedding change then Shift+Tab via `Update` | B survives and the correct next mode is selected/persisted. |
| Reverse command order | Both changes survive; avoids favoring only one tested order. |
| `/yolo status` after embedding save | No config write and B stays intact. |
| Restart immediately after embedding save | B is loaded as desired; no claim about provider readiness without a controlled engine fixture. |
| Valid new block where initial embedding was absent | Work-mode save does not remove the new block. |
| A equals B | No false conflict or misleading “changed” claim. |
| Mode cycle from each mode, plus session `--yolo` override | Existing mode and override semantics remain intact. |
| Approval modal/error focus | Shift+Tab behavior respects existing routing guards; no accidental persistence through a modal. |
| Subsequent debug-self mutation | The same field-preservation contract holds for the other inspected whole-model sink if included in repair scope. |
| Valid unrelated on-disk change before mode mutation | Preserve it or visibly refuse a stale base according to the selected contract. |
| Malformed/unreadable existing file | Refuse destructive replacement; distinguish missing-file creation. |
| Workspace A active, cwd resolves B | Reads and writes honor one explicitly selected workspace; no cross-workspace contamination. This is an adjacent contract test, not the minimal bug witness. |

Compare **parsed semantic configuration** for preservation; byte comparison alone is unfair where canonical indentation or key ordering changes. Also inspect explicit zero, absent-pointer, empty string and default semantics so a merge does not silently normalize a meaningful setting away. Use canary strings rather than actual credentials.

### Failure and interruption matrix

Inject failures at temp creation, write, sync, close, replacement and post-replacement chmod. Verify the actual disk outcome and reported disposition at each point. Pre-replacement failure should retain the previous target. A post-replacement error must not induce a blind retry or falsely assert that the old config is still present.

For the live mode, separately check model state, kernel-visible mode, saved mode, and user-facing warning. Existing intentional session-only behavior can remain; the evaluation must not require accidental rollback of a legitimate session override.

If revision checks are introduced, deterministically hold a writer after reading, publish a second writer, then release the first. Require either a valid merge of disjoint fields or visible conflict, never silent overwrite. Test same-field disagreement separately from disjoint updates. Test the declared cooperating-writer protocol rather than claim arbitrary editor safety from an in-process mutex.

### Performance and operational comparison

Compare unchanged baseline, minimal targeted repair and shared mutation helper on the same config corpus and storage class. Record config bytes, reads/writes, allocations, command latency (including sync), conflict/retry counts, and errors; include small and large valid config objects, cold and warm filesystem conditions, and repeated randomized command order.

Retain all runs, including save failures and refused conflicts. Do not call a scheme faster by excluding its retries or moving persistence outside the measured interval. Keep network/model work disabled in deterministic preservation tests. A separate controlled restart/provider test may establish engine application, but cannot be used to substitute for the saved-state invariant.

### Release exit

The change is complete when:

1. the actual slash and keyboard entry paths preserve the acknowledged embedding choice;
2. unrelated config values remain unchanged by mode-only mutations;
3. desired state and live application are not confused;
4. failure outcomes and unsupported multiwriter guarantees are explicit;
5. existing privacy, atomic-write and mode-routing behavior is preserved.

A unit test that only sees `work_mode = yolo` is insufficient: that is already compatible with the reported loss.

## 8. Primary literature and transferable reasoning

- **Kung and Robinson (1981), “On Optimistic Methods for Concurrency Control.”** The original paper separates local candidate work from validation and publication; it also explains the need to keep final validation and write indivisible in its serial-validation construction. The applicable lesson is current-base validation before accepting a candidate, not that codeNERD needs a database transaction engine. Retry/starvation costs matter if the chosen repair expands to concurrent writers. [Author-hosted original paper](https://www.eecs.harvard.edu/~htk/publication/1981-tods-kung-robinson.pdf), especially pp. 215 and 220–222.
- **RFC 9110, section 13.1.1 (If-Match).** The standard's conditional-update contract rejects a modification when the current representation does not match the caller's base. It is a useful analogy for an expected-config-revision receipt and an explicit stale-update outcome. HTTP headers are not proposed for a local file. [RFC 9110 §13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1).

These sources inform repair design. They do not establish the repository defect; the pinned allocation, routing and save chain above does. The minimal failure is sequential, so its first repair should not be delayed on a general concurrency redesign.

## Evidence and execution disclosure

Completed: read-only public GitHub inspection at the pinned commit; static route/dataflow tracing; targeted duplicate searches and related-document review; inspection of existing relevant tests; primary-literature lookup; this local Markdown research artifact.

Not performed: application startup, reproduction, test execution, benchmarks, provider calls, code changes, remote publication, issue/PR creation, or performance measurement. Local source copies were retained only as audit evidence.

[embedding]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/commands_handlers.go#L277-L340
[work-mode]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/work_mode.go#L14-L41
[save]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/user_config.go#L801-L813
[atomic]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/persistence.go#L35-L92
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/agents.md#L1-L16
[boot-load]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/session.go#L53-L65
[boot-model]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/session.go#L198-L214
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/model_handlers.go#L108-L132
[embedding-route]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/commands.go#L132-L140
[yolo-route]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/commands.go#L240-L243
[global]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/user_config.go#L1765-L1768
[load]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/user_config.go#L617-L671
[yolo]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/yolo.go#L15-L56
[keyboard]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/model_key_handler.go#L89-L94
[path]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/config_path.go#L10-L18
[path-discovery]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/user_config.go#L539-L615
[missing-model]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/memory.go#L169-L188
[mode-cycle]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/work_mode.go#L19-L55
[key-guards]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/model_key_handler.go#L20-L59
[yolo-tests]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/yolo_test.go#L12-L64
[debug-self]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/self_debug.go#L543-L557
[wizard-entry]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/config_wizard_steps.go#L509-L525
[wizard-save]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/config_wizard_steps.go#L605-L613
[representation]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/Docs/audits/embedding-representation-and-migration.md
[stats-audit]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/Docs/audits/embedding-stats-observation-boundary.md
[policy-audit]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/Docs/audits/dev-policy-reload-adoption.md
[config-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/Docs/architecture/config/WIRING-AND-NOT-BUILT.md
[wizard-test]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/config_wizard_save_test.go#L12-L74
[key-test]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/cmd/nerd/chat/commands_test.go#L792-L811
[load-publish]: https://github.com/theRebelliousNerd/codenerd/blob/99d87a96ea0a2d11e8b0e539f4fe9bd223abe682/internal/config/user_config.go#L698-L740

