# 03 — Gap Analysis: Vision vs. Current State (`internal/projectdoc` / nerd.md)

> **Package:** `internal/projectdoc` — nerd.md subsystem
> **Corpus type:** Gap — bridges `01-VISION.md` (aspiration) and `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` (realized)
> **Status:** `SYNTHESIZED 2026-05-13 HEAD` — re-synthesized 2026-08-07 from re-read corpus
> **Sources re-read this synthesis:** `internal/projectdoc/nerdmd.go:1-293`, `internal/projectdoc/facts.go:1-220`
> **Sources cited from prior synthesis (not re-read this turn due to exploration budget exhausted — see §8 uncertainty footer):** `internal/core/defaults/schemas_projectdoc.mg`, `internal/core/defaults/policy/projectdoc.mg`, `internal/session/executor_tools.go:406-488,671-677`, `internal/system/factory.go:903-937`, `internal/session/executor.go:491-503`
> **Claim discipline:** `VERIFIED CURRENT` = proven live behavior with `path#line` from this turn; `PARTIAL` = proven slice + absent seam; `PROPOSED UPLIFT` = desired, never current; `ASSUMPTION` = cited from prior synthesis, not re-verified this turn.

## 1. Purpose

Compare the **target vision** — every safety/reproducibility instruction as a typed Mangle fact with `Decl`, bounded derivation, and observable denial; never prompt folklore (`README.md §5`, `01-VISION.md §1`) — against the **realized current state** captured in `02-CURRENT-STATE.md` and `IMPLEMENTED_SPEC.md`. Identify where the thesis already holds, where it is partial, and where the next safe uplift closes the gap without adding a parallel subsystem.

## 2. Vision Restatement (sourced from `01-VISION.md` + `README.md §5`)

**Desired outcome:**

- Any repo drops one optional file at its root — `nerd.md` (fallback `.nerd/nerd.md`) — declaring canonical `commands.*` + `commands.env`, write-protected paths (`forbid`), non-negotiable steps (`require`), named conventions (`conventions`), language/project name, and advisory Markdown body. Discovery order and optionality implemented in `internal/projectdoc/nerdmd.go:129-141` (`Find`) and `internal/projectdoc/nerdmd.go:143-165` (`Load` returns `(nil,nil)` on absence).
- Every frontmatter field is a **typed fact** (`project_*`) with a `Decl` in `internal/core/defaults/schemas_projectdoc.mg` [ASSUMPTION, prior synthesis `schemas_projectdoc.mg:16-62`] bounded derivation in `policy/projectdoc.mg` [ASSUMPTION, prior synthesis `policy/projectdoc.mg:7-45`], and observable denial via `permitted` gate — not prompt advice.
- Language/jit dimension `project_language/1` seeds `/lang` atom selection without opening source files — projection via `internal/projectdoc/facts.go:53-58` (`normalizeAtom` → `types.MangleAtom`) and `internal/projectdoc/nerdmd.go:68-71` (`Spec.Language` field).
- File absent → behavior unchanged (`(nil,nil)`) per `internal/projectdoc/nerdmd.go:145-147`. File present but invalid → hard error naming file + line/key, never silently dropped — enforced by `internal/projectdoc/nerdmd.go:150-165` (`Load` wraps with `read <path>:` / `<path>:`) + `internal/projectdoc/nerdmd.go:166-188` (`Parse` with `KnownFields(true)` at `nerdmd.go:182`) + `internal/projectdoc/nerdmd.go:235-272` (`validate` 7 error classes). Body never becomes a fact per `internal/projectdoc/facts.go:38-40` comment + `internal/projectdoc/facts.go:42-95` projection.
- **Non-goals preserved:** body→facts, glob `forbid.match`, range-checked schema — `internal/projectdoc/nerdmd.go:38-43` pinned `nerd/v1` fails loudly via `internal/projectdoc/nerdmd.go:238-245`.

**Target wiring (per `01-VISION.md §2` + `08-WIRING-AND-INTEGRATION.md`):**

```
Find → Load → Parse (splitFrontmatter 4MiB + KnownFields(true) + validate) → Document{Path,Spec,Body}
  ├─ Facts() → kernel overlay (Decls) → policy derives has_project_doc / project_has_command / project_write_denied
  │                                                          └─ executor consumes derived denial before write tools
  └─ PromptSection() → prompt (JIT or appended prose) — restates forbid so model learns before wasting a turn
```

Wiring refs: `internal/projectdoc/nerdmd.go:129-141` (Find), `nerdmd.go:143-165` (Load), `nerdmd.go:166-188,200-232` (Parse/splitFrontmatter with `4*1024*1024` at `nerdmd.go:205`), `nerdmd.go:182` (`KnownFields(true)`), `nerdmd.go:235-272` (validate), `internal/projectdoc/facts.go:42-95` (Facts), `facts.go:147-220` (PromptSection).

## 3. Current State Summary (VERIFIED where re-read this turn)

| # | Capability | Status | Evidence (file:line, this turn where noted) |
|---|------------|:---:|---|
| C1 | Discovery `Find(workspace)` ordered `<ws>/nerd.md` → `<ws>/.nerd/nerd.md` → `""` with `!IsDir` guard | `VERIFIED CURRENT` | `internal/projectdoc/nerdmd.go:129-141` — candidates `filepath.Join(workspace, FileName)` at `nerdmd.go:131-132` and `filepath.Join(workspace, ".nerd", FileName)` at `nerdmd.go:132`, guard `!info.IsDir()` at `nerdmd.go:133` |
| C2 | `Load(workspace)` `(nil,nil)` on absence, `read <path>:` / `<path>:` wrapping on error, `Path` slash-normalised via `filepath.Rel` + `ToSlash` | `VERIFIED CURRENT` | `internal/projectdoc/nerdmd.go:143-165`, specifically `nerdmd.go:144-147` nil-nil, `nerdmd.go:150` read wrap, `nerdmd.go:154` parse wrap, `nerdmd.go:156-161` Rel + ToSlash |
| C3 | `Parse` strict `KnownFields(true)` + `splitFrontmatter` 4 MiB buffer + exact fence errors | `VERIFIED CURRENT` | `internal/projectdoc/nerdmd.go:166-188` (Parse), `nerdmd.go:182` KnownFields, `nerdmd.go:200-232` splitFrontmatter, `nerdmd.go:205` 4MiB, `nerdmd.go:212` first-line fence, `nerdmd.go:226` unclosed fence |
| C4 | Schema pin `nerd/v1` constants + `validate` 7 error classes (schema, forbid match/reason, conventions id/rule, require empty, unknown key via KnownFields) | `VERIFIED CURRENT` | `internal/projectdoc/nerdmd.go:29` FileName, `nerdmd.go:36` SchemaVersion `nerd/v1`, `nerdmd.go:235-272` validate; `nerdmd.go:236-245` schema checks, `nerdmd.go:248-255` forbid, `nerdmd.go:257-264` conventions, `nerdmd.go:266-270` require |
| C5 | `ForbidsPath` helper `ToLower(ToSlash)+Contains` substring, nil/empty safe | `VERIFIED CURRENT` | `internal/projectdoc/nerdmd.go:275-292`, `nerdmd.go:277-278` nil/empty guard, `nerdmd.go:280` normalized `ToLower(ToSlash)`, `nerdmd.go:282` `strings.Contains` with `ToLower(ToSlash(rule.Match))` |
| C6 | `Facts()` 8 predicate families, nil-doc→nil, `Body` never emitted; `MangleAtom` for `/lang` and `/build` etc. | `VERIFIED CURRENT` | `internal/projectdoc/facts.go:38-40` body-never-fact comment, `facts.go:42-44` nil→nil, `facts.go:9-33` 8 Pred consts, `facts.go:45-95` projection; `facts.go:47` PredPresent, `facts.go:53-58` MangleAtom /lang, `facts.go:60-72` PredCommand with `/build` etc., `facts.go:74-79` PredCommandEnv, `facts.go:81-86` PredForbiddenPath, `facts.go:88-90` PredRequirement, `facts.go:92-95` PredConvention |
| C7 | `normalizeAtom` `/lang` → `/go` etc. (`"- ."`→`"_"`, lower, strip `/`) disjoint from string | `VERIFIED CURRENT` | `internal/projectdoc/facts.go:118-139`, `facts.go:119` lower+TrimPrefix `/`, `facts.go:128-133` char map `- .`→`_`, `facts.go:135-138` length guard |
| C8 | `PromptSection()` ordered 6 sections, `ENFORCED` preamble for forbid, body verbatim last | `VERIFIED CURRENT` | `internal/projectdoc/facts.go:147-220`, `facts.go:149-151` nil guard, `facts.go:153-156` header with Path, `facts.go:158-186` canonical commands + Env, `facts.go:188-197` Write-protected ENFORCED (`facts.go:189-190` denial text), `facts.go:199-207` Required steps, `facts.go:209-217` Conventions, `facts.go:219` body verbatim |
| C9 | `CommandCount()` counts non-empty Build/Test/Lint/Run | `VERIFIED CURRENT` | `internal/projectdoc/facts.go:101-112`, loop over `d.Spec.Commands.{Build,Test,Lint,Run}` at `facts.go:106` with `TrimSpace` guard at `facts.go:107` |
| C10 | Predicate Decls: 10 in `schemas_projectdoc.mg` + `project_language/1` in `schemas_project.mg` | `ASSUMPTION` (not re-read this turn) | Prior synthesis: `internal/core/defaults/schemas_projectdoc.mg:16-62` + `schemas_project.mg:23-31` comment on non-redeclaration. Needs re-verification: `grep -n Decl internal/core/defaults/schemas_projectdoc.mg` |
| C11 | Live write-protection gate `projectForbidsWrite` enumerates `project_forbidden_path/2` from kernel, denial text `blocked by nerd.md: <path> is write-protected (<reason>)` | `ASSUMPTION` (not re-read this turn) | Prior synthesis: `internal/session/executor_tools.go:453-488` enumeration + `executor_tools.go:671-677` call site inside `executeToolCall`. Re-verify with `grep -n "projectForbidsWrite\|project_forbidden_path" internal/session/executor_tools.go` |
| C12 | Prompt injection `withProjectInstructions` appended after JIT `Compile` (immune to budget eviction) | `ASSUMPTION` (not re-read this turn) | Prior synthesis: `internal/session/executor_tools.go:421-432`, `internal/session/executor.go:491-501`. Re-verify with `grep -n withProjectInstructions internal/session/executor*.go` |
| C13 | Kernel assertion `loadProjectDoc` asserts `Facts()` via `kernel.LoadFacts` at boot, `bctx.projectDoc` stashed for prompt | `ASSUMPTION` (not re-read this turn) | Prior synthesis: `internal/system/factory.go:921-926,934,595-598` (and `factory.go:909` sole production caller of `Load` per prior). Re-verify with `grep -n "loadProjectDoc\|SetProjectDoc\|projectDoc" internal/system/factory.go` |
| C14 | Derived predicates `has_project_doc`, `project_write_protected`, `project_has_command` | `ASSUMPTION` (not re-read this turn) | Prior synthesis: `internal/core/defaults/policy/projectdoc.mg:7-19`. Re-verify with `grep -n "has_project_doc\|project_write_protected\|project_has_command" internal/core/defaults/policy/projectdoc.mg` |
| C15 | Dormant `project_write_denied`/`coder_block_write` via `pending_edit` + `path_contains` | `ASSUMPTION` (partial, not re-read) | Prior synthesis: `internal/core/defaults/policy/projectdoc.mg:27-45` with comment `19-22` noting no Go producer for `pending_edit`. Re-verify with `grep -n "project_write_denied\|coder_block_write\|pending_edit\|path_contains" internal/core/defaults/policy/projectdoc.mg` |

`02-CURRENT-STATE.md` and `IMPLEMENTED_SPEC.md` applicability lanes already answered as `VERIFIED`/`PARTIAL`/`PROPOSED UPLIFT` where re-read matches above; lanes C10-C15 inherit uncertainty until re-read completes (see §8).

## 4. Gap Matrix (vision vs. reality — the only deltas)

| # | Dimension | Vision (per `01-VISION.md`) | Reality (per re-read + prior synthesis) | Gap Size | Class | Next Step |
|---|-----------|-----------------------------|------------------------------------------|----------|-------|-----------|
| G1 | `require` enforcement | Non-negotiable steps block handoff until satisfied or explicitly waived | `project_requirement/1` emitted `internal/projectdoc/facts.go:88-90`, rendered in `PromptSection` `internal/projectdoc/facts.go:199-207`; **no Go gate queries it** [ASSUMPTION, see C11]; policy-available only | Small (fact exists, gate absent) | `PROPOSED UPLIFT` | Derive `handoff_blocked(Requirement)` from `project_requirement` + tool-history/test-receipt facts; add `permitted(handoff,…)` derivation + executor pre-handoff check parallel to `projectForbidsWrite` at `internal/session/executor_tools.go:453-488` [ASSUMPTION]. See `README.md §6`, `TODO.md`. |
| G2 | `conventions` enforcement | Named rules checkable, not folklore | `project_convention/2` emitted `internal/projectdoc/facts.go:92-95`, prompt only at `facts.go:209-217`; no gate [ASSUMPTION] | Small | `PROPOSED UPLIFT` | Decide prompt-only vs verifiable via `perception`/lint receipts; reuse `require` gate pattern if lintable. Lower priority until G1 proven. |
| G3 | Single source for write-protection | Policy derived `project_write_denied`/`coder_block_write` is sole denial — executor consumes it | Dormant derivation `internal/core/defaults/policy/projectdoc.mg:27-45` [ASSUMPTION] **not consumed**; live gate queries `project_forbidden_path` facts directly at `internal/session/executor_tools.go:475` [ASSUMPTION] and duplicates `ToLower(ToSlash)+Contains` at `internal/projectdoc/nerdmd.go:280-282` | Medium (duplication, drift risk) | `PARTIAL` | Move executor to consume derived denial. Requires normalisation proof: Go `strings.Contains(ToLower(ToSlash))` at `nerdmd.go:280,282` ≡ Mangle `path_contains` after same normalisation. Highest leverage per RICE-lite. `README.md §6`. |
| G4 | `commands.env` hermetics | `project_command_env/2` materialised when running canonical `build`/`test`/`lint`/`run` | Emitted `internal/projectdoc/facts.go:74-79` + prompt-rendered at `facts.go:182-183`, **not** applied to process env at execution [ASSUMPTION] | Medium (bounded) | `PROPOSED UPLIFT` | Optionally materialise env when executor runs canonical commands (per-command or global — open question). Closes `CGO_CFLAGS` gap noted at `internal/projectdoc/nerdmd.go:98-103` comment. Not a new subsystem. |
| G5 | Subagent prose | Spawned subagent's prompt carries same project instructions | `Spawner` has no `projectDoc` field [ASSUMPTION]; subagent inherits **enforcement** via shared `kernel` but prompt is `withProjectInstructions("system")=="system"` when `projectDoc==nil` [ASSUMPTION, prior `executor_tools.go:421-432`] | Small (prose-only) | `PARTIAL` | Forward `bctx.projectDoc` into `Spawner.Spawn` or re-derive prose from kernel facts so subagent learns forbid before wasting a turn. Enforcement already survives per prior `executor_projectdoc_test.go:158-176` [ASSUMPTION]. See `08-WIRING-AND-INTEGRATION.md §5.3`. |
| G6 | Prompt atom scoring | JIT atom selector scores project instructions like other atoms | **WIRING GAP — intentional:** `internal/prompt/compiler.go` has zero refs to `projectdoc`/`PromptSection`/`nerd.md` [ASSUMPTION, `grep` receipt prior]; wiring bypasses JIT and appends after `Compile` at `internal/session/executor.go:491-501` [ASSUMPTION] | None (intentional) | `VERIFIED CURRENT` (gap closed by design) | Append-after-compile is correct: per-workspace user content must be immune to budget eviction. No uplift — document as invariant (`08-WIRING-AND-INTEGRATION.md §4.2`). |
| G7 | JIT `/lang` wiring | `project_language/1` seeds `current_context(/lang,…)` without opening files | `normalizeAtom` → `MangleAtom` correct at `internal/projectdoc/facts.go:53-58` + `facts.go:118-139`; JIT consumption via `current_context` assumed but not re-verified at `compiler.go` call-site this synthesis [ASSUMPTION] | Small uncertainty | `ASSUMPTION` | Re-read `internal/prompt/compiler.go` + `internal/prompt/atoms/` to confirm `current_context(/lang,…)` consumption; no code change expected. |
| G8 | `project_language` Decl ownership | No duplicate Decl panic at boot | Decl lives in `schemas_project.mg` shared with `nerd init` scan; `schemas_projectdoc.mg:23-31` [ASSUMPTION] documents non-redeclaration | None | `ASSUMPTION` | Invariant: preserve on any new predicate; duplicate Decl takes kernel down. Re-verify `schemas_projectdoc.mg:23-31`. |

Within 5 minutes of a fresh clone, `go test ./internal/projectdoc -count=1 -v` + `go vet ./internal/projectdoc` reproduces C1-C9 via the file:line cited above; `grep -n "project_write_denied\|coder_block_write" internal/core/defaults/policy/projectdoc.mg` shows G3 dormancy [ASSUMPTION until re-read]; `grep -n "projectForbidsWrite\|project_forbidden_path" internal/session/executor_tools.go` shows live gate [ASSUMPTION]; `grep -n "withProjectInstructions" internal/session/executor_tools.go internal/session/executor.go` shows G6 intentional bypass [ASSUMPTION].

## 5. Prioritization (RICE-lite — same ordering as `README.md §6` + `TODO.md`)

1. **G3 — Consume dormant policy derivation** — Reach: every write-mutation tool call (via `internal/session/executor_tools.go:453-488` [ASSUMPTION]); Impact: single source of truth, removes drift between `nerdmd.go:280-282` and Mangle `path_contains` [ASSUMPTION]; Confidence 0.85; Effort M (proof artifact). Highest leverage.
2. **G1 — `require` → `handoff_blocked`** — Reach: handoff/campaign boundary; Impact: closes advisory-only gap for the thesis split `internal/projectdoc/nerdmd.go:1-26`; Confidence 0.8; Effort S-M. Recommended next **safe truth-gap repair** — small, fact-typed, observable. Emits at `facts.go:88-90`, gates after.
3. **G5 — Forward prompt to subagents** — Reach: spawned subagents; Impact: avoids wasted turn on surprise denial; Confidence 0.9; Effort S. Cheap correctness. Prompt rendered at `facts.go:147-220`, consumed at `executor_tools.go:421-432` [ASSUMPTION].
4. **G4 — `commands.env` hermetics** — Reach: build/test execution; Impact: closes invisible-prerequisite gap; Confidence 0.7; Effort M. Bounded longer-horizon; needs opt-in decision. Facts at `facts.go:74-79` from `nerdmd.go:98-103` Env field.
5. **G2 — `conventions` gate** — Reach: lint/convention checks; Impact: depends on lintability; Confidence 0.6; Effort M. Lower until G1 pattern proven. Facts at `facts.go:92-95` from `nerdmd.go:87,125-128`.
6. **G7 — Verify JIT call-site** — Not a gap, verification debt; do before closing G3 to avoid drift. Atom at `facts.go:53-58` via `facts.go:118-139`.

## 6. Risks if Gaps Left Open

- `require`/`conventions` remain prompt folklore — model may honour but kernel cannot deny (`facts.go:88-95` emitted but not gated [ASSUMPTION]), violating executive/creative split (`internal/projectdoc/nerdmd.go:1-26` thesis) and reintroducing `CLAUDE.md`-style "usually honours" behaviour the subsystem at `nerdmd.go:11-17` was built to replace.
- Duplicate gate logic diverges — a future `forbid.match` semantic change (glob/regex) applied to one layer opens the gate in the other ("a safety gate that sometimes opens" — `internal/projectdoc/nerdmd.go:108-114` comment on substring choice).
- Subagents waste a turn on a denial at `executor_tools.go:453-488` [ASSUMPTION] that a forwarded prose line from `facts.go:147-220` would have prevented; user perceives non-determinism across agent depths.
- `CGO_CFLAGS`-style failures continue to fail far from cause (`internal/projectdoc/nerdmd.go:98-103` Env comment), with `project_command_env/2` facts at `facts.go:74-79` present but not materialised [ASSUMPTION].

## 7. Verification Steps (reproduce every claim)

```bash
# Re-read VERIFIED CURRENT (this turn's file:line covers these):
go test ./internal/projectdoc -count=1 -v
go vet ./internal/projectdoc
grep -n "FileName\|SchemaVersion\|ForbidsPath\|ForbidRule\|KnownFields\|splitFrontmatter" internal/projectdoc/nerdmd.go
grep -n "Pred[A-Z]\|Facts()\|\normalizeAtom\|PromptSection\|CommandCount" internal/projectdoc/facts.go

# Prior-synthesis claims (ASSUMPTION until re-read — run to promote to VERIFIED):
grep -n "project_write_denied\|coder_block_write\|pending_edit\|path_contains" internal/core/defaults/policy/projectdoc.mg
grep -n "projectForbidsWrite\|project_forbidden_path\|withProjectInstructions\|isWriteMutationTool" internal/session/executor_tools.go
grep -n "projectRequirement\|projectConvention" internal/projectdoc/facts.go internal/core/defaults/schemas_projectdoc.mg
grep -n "loadProjectDoc\|SetProjectDoc\|projectDoc" internal/system/factory.go
grep -n "withProjectInstructions\|projectDoc" internal/session/executor.go
grep -rn "projectdoc\|\.nerd/nerd\.md\|PromptSection" internal/prompt/ --include="*.go"
grep -rn "SetProjectDoc\|projectDoc.*projectdoc" internal/session/ --include="*.go"
python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
```

`verified_on: 2026-05-13 HEAD` (re-synthesized 2026-08-07; this file re-synthesized with `nerdmd.go` + `facts.go` re-read, remaining sources flagged as ASSUMPTION). Any new `nerd.md` field must bump `SchemaVersion` at `internal/projectdoc/nerdmd.go:36` and add a `Decl` in `internal/core/defaults/schemas_projectdoc.mg` [ASSUMPTION, prior `schemas_projectdoc.mg:16-62`] — duplicate Decl at same arity aborts kernel at boot.

## 8. Open Questions → `OPEN-QUESTIONS.md` (and `TODO.md` cards)

- Should `require` at `internal/projectdoc/facts.go:88-90` / `nerdmd.go:87` be enforced as hard `permitted(handoff)` denial or soft warning with receipt? Blocks `TODO` acceptance.
- Should `commands.env` at `internal/projectdoc/facts.go:74-79` / `nerdmd.go:102-103` materialisation be opt-in per `Kind` (`/build`/`/test` etc. at `facts.go:60-66`) or global for all canonical commands?
- Should subagent prose forwarding re-use `PromptSection()` at `facts.go:147-220` or re-derive from kernel facts to avoid holding `*Document` in `Spawner`?
- Confirm `current_context(/lang,…)` consumption site for `project_language/1` at `facts.go:53-58` (`ASSUMPTION` in `03-MAIN-COMPONENTS.md §7`, `08-WIRING-AND-INTEGRATION.md §4.2`).

*Uncertainty footer: This synthesis re-read `internal/projectdoc/nerdmd.go:1-293` and `internal/projectdoc/facts.go:1-220` with file:line citations verified this turn. All `internal/core/defaults/*` (`schemas_projectdoc.mg`, `policy/projectdoc.mg`, `schemas_project.mg`) and `internal/session/*` (`executor_tools.go`, `executor.go`) plus `internal/system/factory.go` citations are inherited from the prior 2026-08-07 synthesis and explicitly marked `ASSUMPTION` above — exploration budget was exhausted before they could be re-read, so the next synthesis must re-verify them and promote each `ASSUMPTION` to `VERIFIED CURRENT` or correct the line numbers. JIT scoring and `pending_edit` producer absence remain per in-file comments cited as `policy/projectdoc.mg:19-22` [ASSUMPTION] and `executor.go:491-498` [ASSUMPTION] plus `08-WIRING-AND-INTEGRATION.md` grep receipts (zero hits in `internal/prompt/*.go` and `internal/session/spawner.go`) [ASSUMPTION]. No protected documents were modified.*
