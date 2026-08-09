# init — Safety and Invariants

> Last verified: 2026-08-09

## Scope of safety

Init is a **local filesystem / DB bootstrapper**. It is not the constitutional action gate (`permitted(...)`). Safety here means: do not corrupt workspaces, do not block forever, do not invent privileged remote actions without tools, and leave Mangle templates that reinforce default-deny for later sessions.

## Invariants

### I1 — Workspace locality

All durable writes target `{Workspace}/.nerd/**` (plus reading the workspace tree for detection). Init must not treat arbitrary paths outside the workspace as profile root.

### I2 — Initialized marker

`IsInitialized` ⇔ `.nerd/profile.json` exists. Other artifacts may exist without this; treat profile as the canonical marker.

### I3 — Batch fact load

Scan facts enter the kernel via `LoadFacts`, not per-file `Assert` loops (performance + consistency invariant).

### I4 — Upgrade idempotence (best-effort)

Agent KB upgrade uses content-hash sets (`buildAtomHashSet` / `appendKnowledgeAtom`) so re-init does not infinitely duplicate identical atoms.

User Mangle overlays are stronger than best-effort: `extensions.mg` and
`policy_overrides.mg` use atomic create-if-absent and are never replaced by
force init. The same rule protects the user-customizable `.nerd/.gitignore`.

### I5 — Shared-before-specialist

Shared pool creation precedes Type-3 KB creation in the happy path so inheritance is meaningful.

### I6 — Non-blocking progress

Progress channel sends use `select`/`default`; a stuck UI consumer cannot deadlock init.

### I7 — Context cancellation

`Initialize` applies `InitConfig.Timeout` to every caller, preserves a shorter
parent deadline, and checks cancellation between phases. Long research also
derives child timeouts from that bounded context.

### I8 — Mangle constant safety

Generated name constants pass `sanitizeForMangle`; string args escaped in `generateFactsFile`.

### I9 — Validation observability

Successful init runs `ValidateAllAgentDBs` and labels that result structural.
Required artifact failures make `Success=false`; optional LLM failures are
counted separately and visibly degrade the summary.

### I10 — Embedding requirement honesty

If sqlite-vec path requires embeddings, failure to create embedding engine is a **hard error** after DB path (return error), not a silent empty vector index.

## Concurrency safety

| Resource | Guard |
|----------|--------|
| Grounding sources / LLM call metrics | `Initializer.mu` |
| ETA maps | `ETATracker.mu` |
| Parallel agent KBs | Separate DB paths per agent; worker pool results channel |
| ProgressChan | Non-blocking send |

## Permissions model (metadata only)

`RecommendedAgent.Permissions` and `types.ShardPermission` on registered profiles are **declarative for later spawn**. Init does not enforce them when writing files.

## Constitutional / policy surface

| Artifact | Safety role |
|----------|-------------|
| `mangle/policy_overrides.mg` | User may add `permitted` extensions; default empty comments only |
| `mangle/extensions.mg` | Schema extensions Decls only |
| Core policy | Loaded at session boot, not rewritten by init |

Init must **not** inject broad `permitted(...)` facts that weaken default deny.

## Secrets handling

- API keys come from env / user config; not written into `profile.json` or agent YAML by default.
- Default `config.json` may include provider settings; treat as sensitive local file (gitignored patterns partially cover DBs; config may be committed if user chooses — operator awareness).

## Threat notes (local tool)

| Threat | Mitigation |
|--------|------------|
| Malicious workspace files confuse detectors | Heuristic only; no remote code exec from profile.json |
| Research tool network fetch | Requires registry + keys; SkipResearch available |
| Prompt injection via README into strategic knowledge | LLM-filtered docs; still untrusted content → atoms; downstream retrieval should not treat as policy |
| Path traversal in agent names | Type U alphanumeric validation; generated paths use `strings.ToLower(agent.Name)` |

## Mangle Decl note

Init generates **facts**, not a full Decl corpus. Consumers must ensure predicates like `project_language`, `entry_point` are Declared in core schemas before loading `profile.mg` at boot (core responsibility, not init’s Decl file).
