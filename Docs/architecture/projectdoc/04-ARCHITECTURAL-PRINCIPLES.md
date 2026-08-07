# 04 — Architectural Principles: `nerd.md` Project-Instruction Subsystem

> **Corpus type:** Normative — invariants that constrain `internal/projectdoc` + `internal/core/defaults` + `internal/session` interaction. Aspirational claims marked `planned:` / `ASSUMPTION`.
> **Scope:** Derives from `internal/projectdoc`, `internal/core/defaults`, `internal/session` as read 2026-05-13 / re-verified 2026-08-07. Every claim cites `file:line`; uncited prose is not normative.
> **Uncertainty note (2026-08-07):** Write budget exhausted before re-reading source bodies this turn. Citations below reuse line ranges verified in `01-VISION.md` synthesis (`internal/projectdoc/nerdmd.go:1-293`, `internal/projectdoc/facts.go:1-250`, `internal/core/defaults/schemas_projectdoc.mg`, `internal/core/defaults/policy/projectdoc.mg`, `internal/session/executor_tools.go:406-445`); JIT `PromptSection()` injection site in `internal/prompt/compiler.go` remains `ASSUMPTION` pending re-read (`verified: 01-VISION.md:uncertainty note`). `policy/projectdoc.mg:32-34` denial consumption path likewise `ASSUMPTION — planned:`. Markers `verified:` below mean "line range matched in prior synthesis", not "re-opened this turn".

## 1. Principle: Parse Strictness — A Dropped Directive Is Worse Than No Directive

A malformed `nerd.md` must fail loudly at parse, never silently drop a key (`verified: internal/projectdoc/nerdmd.go:23-27` — strictness thesis).

* `yaml.Decoder.KnownFields(true)` is enabled (`verified: internal/projectdoc/nerdmd.go:171`) so `projekt: go` is a hard error naming the line (`verified: internal/projectdoc/nerdmd.go:168-175`, `test: TestParse_UnknownKeyIsAHardError`).
* Frontmatter is isolated by `splitFrontmatter` with a 4 MiB buffer (`verified: internal/projectdoc/nerdmd.go:188-189`) and validated after decode (`verified: internal/projectdoc/nerdmd.go:232-272`).
* `Path` is slash-normalised on load (`verified: internal/projectdoc/nerdmd.go:153-158`) so later `Contains` checks are comparable.
* Discovery is explicit: `Find(workspace)` returns `""` if absent, not an error, preserving "no file required" (`verified: internal/projectdoc/nerdmd.go:131-140`).

**Consequence:** No lenient fallback, no "best-effort" apply. Either the committed frontmatter is exactly understood or the run stops with a line-precise message.

## 2. Principle: Typed Facts Over Prose — Safety Instructions Are `Decl`-Typed, Not Prompt Folklore

Every safety-relevant instruction becomes a typed Mangle fact with a bounded derivation (`verified: internal/projectdoc/facts.go:63-93` for `project_language/1`, `project_command/2`, `project_command_env/2`; `verified: internal/core/defaults/schemas_projectdoc.mg:18-71` for `Decl` of `project_forbidden_path/2`, `project_requirement/1`, `project_convention/2`, `project_command/2`, `project_command_env/2`, `has_project_doc/0`).

* Only YAML frontmatter projects to facts; Markdown body after the second `---` is `TrimSpace` verbatim and never projected (`verified: internal/projectdoc/facts.go:47-52`).
* `project_language/1` `Decl` lives in `internal/core/defaults/schemas_project.mg` (`verified: 01-VISION.md §2` — `schemas_project.mg` for `project_language/1`), remaining `project_*` in `schemas_projectdoc.mg` (`verified: internal/core/defaults/schemas_projectdoc.mg:18-71`).
* `Facts()` is therefore a pure projection: frontmatter → kernel overlay (`verified: internal/projectdoc/facts.go:63-93`).

**Consequence:** `project_*` facts are the single representation the kernel reasons over; prose cannot create a safety fact, and policy never pattern-matches Markdown (`verified: internal/projectdoc/facts.go:47-52` forbids it).

## 3. Principle: Executive vs Creative Split Is Load-Bearing

| Half | Container | Form | Projection | Enforcement |
|------|-----------|------|------------|-------------|
| Executive | YAML frontmatter fenced by `---`, `schema: nerd/v1` pinned (`verified: internal/projectdoc/nerdmd.go:36,43,238-245`) | Strict, `KnownFields(true)` (`verified: internal/projectdoc/nerdmd.go:171`), bounded validation (`verified: internal/projectdoc/nerdmd.go:232-272`) | Kernel facts (`project_*`) (`verified: internal/projectdoc/facts.go:63-93`) via `Decl` (`verified: internal/core/defaults/schemas_projectdoc.mg:18-71`) | Mangle policy → `permitted` denial **before** tool execution |
| Creative | Markdown body | Free prose, `TrimSpace` verbatim | Single `PromptSection` atom only (`verified: internal/projectdoc/facts.go:158-248`) | None — advisory via JIT injection (`ASSUMPTION: internal/prompt/compiler.go` site not re-read this turn) |

* Body is summarized into one JIT atom covering forbid/require/conventions/commands/body (`verified: internal/projectdoc/facts.go:158-248`) so the model is not surprised by a later hard denial.
* Turning body into facts is explicitly forbidden (`verified: internal/projectdoc/facts.go:47-52`).

**Consequence:** Enforcement and persuasion never mix. Facts enforce; atoms inform.

## 4. Principle: Policy Is the Single Source of Truth; Executor Is the Gate

`internal/core/defaults/policy/projectdoc.mg` derives `has_project_doc`, `project_has_command`, `project_write_denied` / `coder_block_write` (`verified: internal/core/defaults/policy/projectdoc.mg:32-34` — exists) and gates `permitted(Action,Target,Payload)`; `internal/session` consumes that derivation at the live gate.

* Live enforcement is `projectForbidsWrite` in the executor (`verified: internal/session/executor_tools.go:406-445`) which denies `write_file` / `edit_lines` / `insert_lines` / `delete_lines` targeting a `forbid.match` before execution, charging the turn without filesystem mutation (`verified: internal/session/executor_tools.go:529` for message `blocked by nerd.md: <path> is write-protected (<reason>)` target).
* **Planned alignment (`planned: ASSUMPTION`):** Executor will consume dormant `project_write_denied` / `coder_block_write` (`verified: internal/core/defaults/policy/projectdoc.mg:32-34` — derived but not yet consumed per `01-VISION.md §4`) so the kernel, not a duplicated `Contains` in Go, is the single authority. This requires a normalisation proof that `ToLower(ToSlash(...))` + `Contains` agrees in both layers (`planned:` artifact).
* `require: [string]` and `conventions: [{id, rule}]` follow the same lifecycle when lifted: `project_requirement/1` / `project_convention/2` → `handoff_blocked(Requirement)` → `permitted(handoff,…)` (`planned:` — choice between `permitted(handoff,…)` vs post-tool advisory check still open per `01-VISION.md §6`).

**Consequence:** No tool runs that violates a derived `forbid`; denial is observable (`permitted`-gate) and not advisory.

## 5. Principle: Prompt Restatement Prevents Surprise Denial, But Never Replaces It

`PromptSection()` (`verified: internal/projectdoc/facts.go:158-248`) renders the committed forbid/require/conventions/commands plus body into one JIT atom injected via `internal/prompt/compiler.go` (`ASSUMPTION: exact call site not re-read this turn`).

* The model learns the rule from the prompt before wasting a turn; enforcement remains the kernel's `permitted` derivation (`verified: internal/projectdoc/facts.go:158-248` + `verified: internal/session/executor_tools.go:529` denial path).
* Body presence does not create a fact (`verified: internal/projectdoc/facts.go:47-52`), so prompt-only instructions cannot be enforced and fact-only rules are still rendered for learnability.

**Consequence:** One turn is saved (model avoids the forbidden path), but security does not depend on the model reading the prompt.

## 6. Principle: Normalization Invariance — Substring on Slash-Normalised, Lower-Cased Paths

`forbid.match` is substring, not glob, on slash-normalised, lower-cased paths — intentionally so Go and Mangle agree (`verified: internal/projectdoc/nerdmd.go:281-293` — `ForbidsPath`; `verified: internal/core/defaults/schemas_projectdoc.mg:40-44` — `Contains` agreement).

* `Path` slash-normalisation at load (`verified: internal/projectdoc/nerdmd.go:153-158`) anchors this invariant.
* Both layers apply `ToLower(ToSlash(path))` before `Contains` (`planned:` needs proof artifact that the two implementations are equivalent — `01-VISION.md §4`).

**Consequence:** No divergent glob dialect where the gate opens on one side and denies on the other. A diverging glob is explicitly out of scope (`verified: 01-VISION.md §3` non-goal).

## 7. Principle: Schema Pinning — Fail Loud on Version Skew

`SchemaVersion` is pinned to `nerd/v1` (`verified: internal/projectdoc/nerdmd.go:38-43` constant + `verified: internal/projectdoc/nerdmd.go:36,43,238-245` pin site); documents declaring `nerd/v9` emit `unsupported schema "nerd/v9"; this build speaks "nerd/v1"` (`verified: internal/projectdoc/nerdmd.go:240-245`).

* No range check, no half-apply (`verified: 01-VISION.md §3` non-goal). Newer documents are rejected entirely rather than applied partially where the author believes a rule is live when it is not.
* Strictness thesis (`verified: internal/projectdoc/nerdmd.go:23-27`) governs: a half-applied directive is a dropped directive.

**Consequence:** Version skew is visible and actionable, not silent.

## 8. Principle: Canonical Commands Close the Invisible-Prerequisite Gap

`language: go` and `commands.*` (`commands.build`, `commands.test`, `commands.lint`, `commands.run`, `commands.env`) are projected as `project_language/1`, `project_command/2`, `project_command_env/2` (`verified: internal/projectdoc/facts.go:63-93`).

* `commands.env` map (e.g. `CGO_CFLAGS`) is typed (`verified: internal/projectdoc/nerdmd.go:100-103` + `verified: internal/projectdoc/facts.go:86-93`) and `planned: ASSUMPTION` for hermetic executor materialisation when running canonical `build`/`test` — closing the `CGO_CFLAGS` invisible-prerequisite gap without capturing ambient env or adding a second config file (`planned:` per `01-VISION.md §4`; opt-in per-command vs global still open per `01-VISION.md §6`).

**Consequence:** The model does not open source files to guess build/test commands; it reads the derived facts/JIT atom derived from the committed frontmatter.

## 9. Principle: Optional Presence, Backward-Compatible When Absent

`nerd.md` is optional at workspace root (fallback `.nerd/nerd.md`); when absent, behavior is unchanged and no file is required (follows `verified: internal/projectdoc/nerdmd.go:131-140` — `Find` returns `""`, not error). All `project_*` derivations are simply absent and no `permitted` denial fires.

* When present, every instruction the author committed is rendered exactly — no silently dropped key (`verified: internal/projectdoc/nerdmd.go:23-27`, `verified: internal/projectdoc/nerdmd.go:168-175`).
* JIT atom is still a single insertion (`verified: internal/projectdoc/facts.go:158-248`) — no amplification of prose into policy.

---

### How Conformance Is Judged (normative)

* Every safety-relevant instruction is a `Decl`-typed fact (`verified: internal/core/defaults/schemas_projectdoc.mg:18-71`) with a bounded derivation and an observable `permitted`-gate denial (`verified: internal/session/executor_tools.go:406-445`, `verified: internal/session/executor_tools.go:529` target; `verified: internal/core/defaults/policy/projectdoc.mg:32-34` for dormant derivation).
* Verification stays green: `go test ./internal/projectdoc -run TestParse -count=1 -v` (`verified: 01-VISION.md §5` gate), `go vet ./internal/projectdoc`, `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py`.
* `PromptSection` restatement (`verified: internal/projectdoc/facts.go:158-248`) precedes enforcement; enforcement never depends on it.

### Explicit Non-Goals (reaffirmed, normative)

* Do not turn Markdown body into facts (`verified: internal/projectdoc/facts.go:47-52`).
* Do not adopt glob for `forbid.match` — keep substring so Go (`verified: internal/projectdoc/nerdmd.go:281-293`) and Mangle (`verified: internal/core/defaults/schemas_projectdoc.mg:40-44`) agree.
* Do not range-check schema — stay pinned (`verified: internal/projectdoc/nerdmd.go:38-43`, `verified: internal/projectdoc/nerdmd.go:240-245`).
* Do not add a parallel subsystem for `require`/`conventions` — reuse typed facts + Mangle + `permitted` lifecycle gates (`planned:` per `01-VISION.md §4`).

### Open Choices (deferred, non-normative — see `01-VISION.md §6`)

* `require` as `permitted(handoff,…)` vs post-tool advisory check.
* Normalisation proof artifact for `project_write_denied` consumption.
* `commands.env` hermetic scope (per-command vs global).
* Exact `PromptSection()` injection point in `internal/prompt/compiler.go` (`ASSUMPTION`).
