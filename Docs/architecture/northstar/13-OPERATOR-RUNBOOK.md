# Operator Runbook — how a vision becomes kernel facts

> Last verified against codebase: 2026-08-15
> Answers the P0 corpus item "Document operator path: after the wizard, how do
> facts get into the Guardian DB?"

## 1. The authority, stated once

There is **one** durable record of the project vision and **one** thing that
decides with it:

| Artifact | Role | Written by | Read by |
|----------|------|-----------|---------|
| Mangle kernel `northstar_*` facts | **Executive.** Everything that decides reads these. | `Guardian.refreshKernelFacts` (from the store) | policy in `internal/core/defaults/policy/prompt_northstar.mg`, JIT context injection, `nerd query` |
| `.nerd/northstar_knowledge.db` | **Durable record.** The facts are projected from here. | `Store.SaveVision` / `Guardian.UpdateVision` | `Guardian`, `/alignment`, campaign risk gate, `nerd northstar *` |
| `.nerd/northstar.json` | **Import + export surface.** Human/wizard editable. | wizard, `nerd northstar load`, every export | `northstar.LoadVisionJSON` during reconciliation |
| `.nerd/northstar.mg` | **Export surface only.** Rendered from `Vision.ToFacts`. | every export | humans, `nerd northstar export mangle` |

`.nerd/northstar.json` and `.nerd/northstar.mg` are **never** an authority that
anything reads in order to decide. They exist so a human can read, diff, and
hand-edit the vision, and so the wizard has somewhere to write.

Implementation: `internal/northstar/bridge.go` (`SyncVisionAuthority`).

## 2. The path, end to end

```
  /northstar wizard  ──┐
  hand-edited JSON  ───┼──►  .nerd/northstar.json
  nerd northstar load ─┘             │
                                     │  SyncVisionAuthority (mtime vs updated_at)
                                     ▼
                        .nerd/northstar_knowledge.db   ◄── Guardian.UpdateVision
                                     │
                                     │  Vision.ToFacts()
                                     ▼
                          Mangle kernel EDB (northstar_*)
                                     │
                                     ▼
                 injectable_context / campaign risk gate / /alignment
```

Reconciliation runs inside `Guardian.Initialize()`, so it happens on **every**
path that builds a guardian: chat boot, shared chat boot, `BuildCampaignObserver`,
and every `nerd northstar` read command. No call site has to remember to do it.

## 3. Reconciliation rules

`SyncVisionAuthority(store, nerdDir)` applies, in order:

1. Neither surface has a vision → **noop**.
2. Only the JSON has one → **import** it into the store, refresh `.mg`.
3. Only the store has one → **export** JSON and `.mg`.
4. Both, semantically equal (timestamps ignored) → **noop**. Nothing is
   rewritten, so repeated boots do not churn mtimes.
5. Both, different → **newer wins**: the JSON file's mtime against the store's
   `updated_at`. Ties go to the store, because the store is what the kernel
   projects. `created_at` always survives.

A JSON document with an empty `Mission` is treated as a half-finished wizard run
and is **not** imported — importing it would flip `northstar_defined()` on and
start gating campaigns against nothing.

An unreadable/corrupt `northstar.json` logs a warning and leaves the store as
sole authority; it never fails the boot.

## 4. Operator recipes

### After running the `/northstar` wizard in chat

Nothing to do. `saveNorthstar` calls `Guardian.UpdateVision`, which writes the
store, re-projects kernel facts, and refreshes both export surfaces in one step.
Confirm with:

```
nerd northstar state      # Vision defined: true
nerd northstar facts      # exactly what the kernel holds
```

### After hand-editing `.nerd/northstar.json`

```
nerd northstar sync       # reports: Imported <json> into <db>
```

Or just start any session — `Guardian.Initialize` reconciles on boot. The `sync`
command exists so the operator can see which direction it moved before booting.

### Loading a vision from a file

```
nerd northstar load path/to/vision.json
```

Writes the **store first** (the authority), then derives JSON and `.mg`. If the
store write fails nothing is exported, so a visible vision the kernel never
received is not possible.

### Checking what the guardian has actually been doing

```
nerd northstar state             # rollup: total checks, blocked rate, mean score, open drift
nerd northstar history --limit 50 # every alignment check, newest first
nerd northstar drift             # unresolved drift
nerd northstar drift --all       # including resolved, with resolutions
```

### Confirming the kernel really received the vision

```
nerd query northstar_defined
nerd query northstar_mission
nerd query northstar_serves      # capability -> persona links
```

If `northstar_defined` is empty but `nerd northstar state` says the vision is
defined, the guardian's kernel wire is missing on that boot path — see
`TestChatBootPaths_ShouldWireGuardianKernelIdentically`, which exists precisely
because `session_boot.go` shipped without `SetParentKernel` for a while.

## 5. Fact shape notes for operators

- Persona IDs are `persona_<Name>`. Link fields (`serves`, `supports`,
  `addresses`) accept either the bare name or the encoded ID.
- Links are only emitted when their target exists in the same vision. A
  `serves: ["Ghost"]` on a capability is dropped rather than emitted as a
  dangling fact, because `unserved_persona`/`orphan_capability` would otherwise
  be silently wrong.
- Mitigations emit **two** facts: `northstar_mitigation(RiskID, /mit_<slug>_<hash>)`
  (the `/name`-typed strategy slot) and `northstar_mitigation_text(RiskID, Text)`
  (the operator's own words). The slug+hash keeps distinct mitigations distinct;
  before this, every mitigation was the same constant `/mitigation`.

## 6. Guardian lifetime

One Guardian per `.nerd` directory per process, refcounted by
`northstar.AcquireGuardian` / `ReleaseGuardian`. Chat boot holds one for the
session; `/alignment`, the campaign observer and the wizard acquire and release
around their work. This is why `/alignment` no longer opens a fresh SQLite handle
on every invocation, and why its recorded checks are visible to the boot
guardian's periodic-check scheduling instead of leaving it stale.
