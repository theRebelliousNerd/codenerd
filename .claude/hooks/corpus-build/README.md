# corpus-build fleet hooks

Layer 2 (per-agent frontmatter) and Layer 3 (fleet telemetry) hooks for the
corpus-build v2 ecosystem. Design-of-record: `.claude/skills/PLAN-corpus-build.md`
SS5 ("Hook architecture") and SS9 ("Token economics"). This directory is
self-contained: every script is pure PowerShell (no external deps, no `jq`),
fails open on malformed input, and creates its own ledger directories on
demand.

Registration happens elsewhere (agent frontmatter for Layer 2, `settings.json`
for Layer 3 fleet-wide matchers) -- this README documents the wiring, it does
not perform it.

Run `powershell -NoProfile -File .claude/hooks/corpus-build/self-test.ps1`
after touching any script in this directory. It must exit 0 with one `PASS`
line per case.

---

## 1. block-oom-build.ps1

- **Event / matcher**: `PreToolUse`, matcher `Bash|PowerShell`
- **Enforces**: blocks a host `go build`/`go test`/`go vet` whose target
  includes `cmd/nerd`, or a whole-tree build/test/vet from
  the repo root (`./...`, which transitively compiles handlers). That package
  has frozen the dev machine's 128 GB host TWICE on a plain `go build`/`go test`.
  Docker invocations (`docker`, `docker-compose`, `make docker-*`) always pass
  -- that IS the required compile route, not a bypass.
- **Escape hatch**: a marker file `<cwd>/.corpus-build/.compile-grant-<agent_type>`
  lifts the block for that `agent_type`. Orchestrator-managed only: it creates
  the file before a deliberate exception and deletes it afterward. Workers must
  never create their own grant.
- **Exit codes**: `0` = allow (includes all fail-open/unparseable paths).
  `2` = deny; stderr carries a human-readable reason and stdout carries
  `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"..."}}`.

Agent-frontmatter registration snippet (per `PLAN-corpus-build.md` SS3.3):

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash|PowerShell"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/block-oom-build.ps1"
          timeout: 10
```

---

## 2. write-scope-guard.ps1

- **Event / matcher**: `PreToolUse`, matcher `Write|Edit`
- **Enforces** (in order):
  1. `.corpus-build/**` is always writable (the fleet's own ledger, intents,
     manifests, results).
  2. **Universal**: `Docs/architecture/**` is read-only for every `agent_type`
     except `corpus-doc-auditor`.
  3. **Universal**: reserved files (`internal/app/server/registration hubs (shards/registration.go, virtual_store routing, cmd/nerd main)`,
     `.nerd/config.json and internal/config`, `MCP/tool schemas**`) are blocked for `agent_type: corpus-builder`
     -- those changes go through the registration-intent pattern
     (`.corpus-build/intents/<WU>_intents.json`) instead of a direct edit.
  4. If a slice manifest exists at
     `.corpus-build/slices/current/<agent_type>.json`
     (shape `{"work_unit":"WU-NNN","files":["path1",...]}`), the write is
     scoped to exactly the files it lists (plus rule 1). No manifest for this
     `agent_type` = permissive: only rules 2-3 apply.
- **Exit codes**: `0` = allow. `2` = deny; stderr names the violated rule (and,
  for rule 4, the work unit id + the full allowed-file list); stdout carries
  the same `permissionDecision: deny` JSON shape as block-oom-build.ps1.

Agent-frontmatter registration snippet:

```yaml
hooks:
  PreToolUse:
    - matcher: "Write|Edit"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/write-scope-guard.ps1"
          timeout: 10
```

---

## 3. spec-context-injector.ps1

- **Event / matcher**: `PostToolUse`, matcher `Read`
- **Injects** (silent on every miss -- the index may not exist yet, or the
  read target may just not be covered):
  - Reads of `Docs/architecture/**/*.md` that ARE in
    `Docs/architecture/roadmap/33_corpus_context_index.json`'s `docs` map ->
    a one-line summary: subsystem, doc-class, feature ids + status/plane.
  - Reads of `Docs/architecture/**/*.md` that are NOT in the index -> checked
    for frontmatter directly on disk (first line `---`); if untagged, injects
    the tag-as-you-go reminder pointing at
    `Docs/architecture/roadmap/FEATURE_TAGGING_SCHEMA.md`. If the doc DOES
    have frontmatter, the index is merely stale -- stays silent.
  - Reads of `internal/**/*.go` -> longest-prefix match against the index's
    `packages` map -> owning subsystem, feature tags, declared wiring surfaces.
  - Dedupe: one injection per `(agent_type, target)` pair per session, cached
    at `$env:TEMP\corpus-injector-<session_id>.json`.
- **Exit codes**: always `0`. Output is either empty (no injection) or
  `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"..."}}`.

Agent-frontmatter registration snippet:

```yaml
hooks:
  PostToolUse:
    - matcher: "Read"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/spec-context-injector.ps1"
          timeout: 15
```

---

## 4. spec-attribution-check.ps1

- **Event / matcher**: `PostToolUse`, matcher `Write|Edit`
- **Warns** (never blocks): when a Write/Edit targets a NEW `.go` file under
  `internal/` (untracked per `git ls-files --error-unmatch`) whose content has
  no `// SPEC:` line, injects a reminder to add
  `// SPEC: <doc>#<section>` attribution near the symbol it implements.
  `_test.go` files are exempt.
- **Exit codes**: always `0`. Output is either empty or the same
  `additionalContext` JSON shape as the injector, WARN-level only -- this hook
  never denies a write.

Agent-frontmatter registration snippet (typically on `corpus-builder` only):

```yaml
hooks:
  PostToolUse:
    - matcher: "Write|Edit"
      hooks:
        - type: command
          command: "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/spec-attribution-check.ps1"
          timeout: 10
```

---

## 5. corpus-fleet-start.ps1

- **Event / matcher**: `SubagentStart`, matcher `corpus-.*` (Layer 3, fleet-wide)
- **Enforces / records**: appends `{ts, event:"start", run_id, phase,
  agent_type, agent_id}` to `.corpus-build/ledger/fleet_events.jsonl`. `run_id`
  and `phase` are read from `.corpus-build/ledger/<session_id>.active`
  (stamped by the orchestrator at Phase 0 and updated at each phase
  transition, shape `{"run_id":"...","phase":"...","skill":"corpus-build"}`).
  No active file for this session = untracked dispatch -> skip silently, no
  directory created.
- **Exit codes**: always `0`. Telemetry-only; mirrors
  `.claude/hooks/roadmap-grinder/subagent-start.ps1`'s contract.

`settings.json` registration snippet (Layer 3, fleet-wide matcher):

```json
{
  "hooks": {
    "SubagentStart": [
      {
        "matcher": "corpus-.*",
        "hooks": [
          {
            "type": "command",
            "command": "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/corpus-fleet-start.ps1",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

---

## 6. corpus-token-meter.ps1

- **Event / matcher**: `SubagentStop`, matcher `corpus-.*` (Layer 3, fleet-wide)
- **Enforces / records**: PowerShell port of
  `neurolog/.claude/hooks/skill-token-subagent.sh`, preserving its hard-won
  fixes:
  - the event's `transcript_path` is the MAIN session transcript, not the
    subagent's; the subagent's bounded transcript is resolved at
    `<dirname(transcript_path)>/<session_id>/subagents/agent-<agent_id>.jsonl`;
  - recency fallback (newest `agent-*.jsonl` in that dir modified within 5
    minutes) when `agent_id` has no on-disk correlate;
  - **hard guard**: never sums the main transcript -- an unresolved or
    main-transcript match is logged to
    `.corpus-build/ledger/skipped_subagent_stops.jsonl` instead;
  - dedupe by transcript filename stem, first stop wins;
  - sums `output_tokens`, `input_tokens`, `cache_creation_input_tokens`,
    `cache_read_input_tokens` per transcript line (`.message.usage // .usage`,
    skipping malformed lines); `billable_total = output + input +
    cache_creation` (cache_read excluded -- heavily discounted).
  - Gated on `.corpus-build/ledger/<session_id>.active`; no active file =
    untracked dispatch -> exit 0, no directories created.
- **Output**: `.corpus-build/ledger/token_runs.csv` (header
  `ts,run_id,phase,agent_type,agent_id,output,input,cache_creation,cache_read,billable_total`)
  plus a JSONL mirror `token_runs.jsonl` with the same fields.
- **Exit codes**: always `0`. Telemetry-only.

`settings.json` registration snippet (Layer 3, fleet-wide matcher):

```json
{
  "hooks": {
    "SubagentStop": [
      {
        "matcher": "corpus-.*",
        "hooks": [
          {
            "type": "command",
            "command": "powershell -NoProfile -ExecutionPolicy Bypass -File .claude/hooks/corpus-build/corpus-token-meter.ps1",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
```

---

## Ledger layout these hooks read/write

```
.corpus-build/
  .compile-grant-<agent_type>          # escape hatch marker (orchestrator-managed)
  slices/current/<agent_type>.json     # {"work_unit":"WU-NNN","files":[...]}
  intents/<WU>_intents.json            # registration-intent payloads for reserved files
  ledger/
    <session_id>.active                # {"run_id":"...","phase":"...","skill":"corpus-build"}
    fleet_events.jsonl                 # SubagentStart rows
    token_runs.csv / .jsonl            # SubagentStop token sums
    skipped_subagent_stops.jsonl       # hard-guard-triggered skips (diagnosis)
```

## Quality bar (applies to every script in this directory)

- Exit `0` on unexpected/malformed input -- never break a session.
- Tolerate missing files/directories; create ledger directories on demand,
  never eagerly (an untracked dispatch or ungated event must not create
  `.corpus-build/ledger/` just to find nothing to write).
- No external dependencies -- built-in PowerShell only (`ConvertFrom-Json`,
  not `jq`).
- ASCII-safe: avoid smart quotes/em-dashes/emoji in script bodies (Windows
  PowerShell 5.1 reads `.ps1` in the system codepage; a stray non-ASCII byte
  can corrupt the parse).
