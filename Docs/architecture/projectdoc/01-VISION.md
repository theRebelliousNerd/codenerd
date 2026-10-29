# 01 — Vision: `nerd.md` Project-Instruction Subsystem

> **Corpus type:** Aspirational — target state for `internal/projectdoc` + Mangle + session + prompt.
> **North-star alignment:** Moves project instructions from prompt folklore (prose in `CLAUDE.md`) into typed facts (`project_*`) with Mangle `Decl`, bounded derivation, and observable `permitted`-gate denial.
> **Last synthesized:** 2026-05-13 (re-read 2026-08-07). Validated against `internal/projectdoc/nerdmd.go:1-293`, `internal/projectdoc/facts.go:1-250`, `internal/core/defaults/schemas_projectdoc.mg`, `internal/core/defaults/policy/projectdoc.mg`, `internal/session/executor_tools.go:406-445` where cited as `verified:`; claims beyond those files marked `planned:` / `ASSUMPTION`.
> **Uncertainty note:** JIT wiring for `PromptSection()` injection into `internal/prompt/compiler.go` atoms was not re-read this turn; described as `ASSUMPTION` pending verification. Policy `project_write_denied` consumption path likewise `ASSUMPTION`.

## 1. Target product vision

Any repository can drop a single optional file at its root — `nerd.md` (fallback `.nerd/nerd.md`) — and declare the instructions that must never be guessed:

* Canonical commands (`commands.build`, `commands.test`, `commands.lint`, `commands.run`, `commands.env`)
* Required environment (`commands.env` map — e.g. `CGO_CFLAGS`)
* Write-protected paths (`forbid: [{match, reason}]`)
* Non-negotiable steps (`require: [string]`)
* Named conventions (`conventions: [{id, rule}]`)

When that file exists, every assistant turn operates under **exactly** the frontmatter the author committed — no silently dropped key, no advisory-only prose masquerading as enforcement, no guess about build/test commands. When it is absent, behavior is unchanged (no file required).

**User-visible promise (target):**

* Author typos `projekt: go` → hard parse error naming the line (`verified: internal/projectdoc/nerdmd.go:168-175` via `decoder.KnownFields(true)`, `test: TestParse_UnknownKeyIsAHardError`).
* Author adds `forbid: [{match: .nerd/config.json, reason: "Live user-owned…"}]` → any `write_file`/`edit_lines`/`insert_lines`/`delete_lines` targeting that path is **denied before execution** with `blocked by nerd.md: <path> is write-protected (<reason>)` (`verified: internal/session/executor_tools.go:529` target) and costs the turn without mutating the filesystem. Model learns the rule from the prompt before wasting the turn (`verified: internal/projectdoc/facts.go:158-248`).
* Author declares `language: go`, `commands.test: go test ./...` → JIT prompt renders language + commands and the Mangle kernel derives `project_language/1`, `project_command/2`, `project_command_env/2` for downstream selection — model does not open source files to guess (`verified: internal/projectdoc/facts.go:63-93`).
* Markdown body after the second `---` remains **advisory only** (`verified: internal/projectdoc/facts.go:47-52`) but is rendered into the prompt as a single atom (`facts.go:158-248` `PromptSection`) so the model is not surprised by the subsequent hard denial.

## 2. Target architecture vision

```
workspace root: nerd.md  (or .nerd/nerd.md)
        │
        ├─ Find(workspace) ──► Path ("" if absent, not an error)  — verified: internal/projectdoc/nerdmd.go:131-140
        ├─ Load ──► os.ReadFile ──► Parse ──► splitFrontmatter (4 MiB buf, nerdmd.go:188-189)
        │                              └─► yaml KnownFields(true) (nerdmd.go:171) ──► validate (nerdmd.go:232-272)
        │                                      └─► Document{Path, Spec, Body}  (Path slash-normalised 153-158)
        │
        ├─ Facts() ──► project_forbidden_path/2, project_requirement/1, project_convention/2,
        │              project_command/2, project_command_env/2, project_language/1, has_project_doc/0
        │              (only frontmatter; Body never projected — verified: facts.go:47-52) ──► kernel overlay
        │                                                                           Decl in internal/core/defaults/schemas_projectdoc.mg:18-71
        │                                                                           (+ schemas_project.mg for project_language/1)
        ├─ PromptSection() ──► single JIT atom summarizing forbid/require/conventions/commands/body
        │                      (verified: facts.go:158-248) ──► internal/prompt/compiler.go JIT injection (ASSUMPTION: exact call site not re-read)
        │
        └─ Policy: policy/projectdoc.mg derives has_project_doc, project_has_command, project_write_denied/coder_block_write
                   ──► kernel permitted(Action,Target,Payload) gate ──► executor projectForbidsWrite (verified: executor_tools.go:406-445)
                                                                        (target: consume derived denial, not duplicate Contains logic)
```

**Executive vs creative split (load-bearing, preserved):**

| Half | Form | Projection | Enforcement |
|------|------|------------|-------------|
| YAML frontmatter (`---` fenced, `schema: nerd/v1` pinned `verified: nerdmd.go:36,43,238-245`) | Strict, `KnownFields(true)`, bounded | Kernel facts (`project_*`) | Mangle policy → `permitted` denial **before** tool runs |
| Markdown body (`TrimSpace` verbatim) | Free prose | `PromptSection` atom only | None — never facts |

## 3. Explicit non-goals (remain out of scope)

* **Do not** turn Markdown body into facts — `verified: facts.go:47-52` forbids pattern-matching prose in policy; natural language stays advisory via JIT only.
* **Do not** adopt glob semantics for `forbid.match` — substring on slash-normalised, lowercased paths is intentional so Go (`ForbidsPath` `verified: nerdmd.go:281-293`) and Mangle agree (`verified: schemas_projectdoc.mg:40-44`). A disagreeing glob creates a gate that sometimes opens.
* **Do not** range-check schema — `SchemaVersion` stays **pinned** to `nerd/v1` (`verified: nerdmd.go:38-43`); newer documents fail loudly (`unsupported schema "nerd/v9"; this build speaks "nerd/v1"` `verified: nerdmd.go:240-245`) rather than half-apply.
* **Do not** add a parallel subsystem for `require`/`conventions` — enforcement must reuse typed facts + Mangle + observable lifecycle gates.

## 4. Bounded evolution (two horizons)

**Safe truth-gap repair (next, small — `planned:`):**
* `require` → pre-handoff `handoff_blocked(Requirement)` derived from `project_requirement/1` and checked against tool history / test receipts; `permitted(handoff,…)` derivation + executor check parallel to `projectForbidsWrite`. `conventions` similarly surfaced via policy-only lint signal (requires decision on `permitted(handoff,…)` vs post-tool advisory check — see `OPEN-QUESTIONS.md`).
* Move executor live gate to consume dormant `project_write_denied` / `coder_block_write` (`verified: policy/projectdoc.mg:32-34` exists but not consumed) so policy is the single source of truth; requires proving equivalence of `ToLower(ToSlash(...))` + `Contains` in both layers (`planned:` needs normalisation proof artifact).

**Bounded longer-horizon option (`planned: ASSUMPTION`, needs decision):** Typed `commands.env` hermetic execution — executor materialises `project_command_env/2` when running canonical `build`/`test` (closing `CGO_CFLAGS` invisible-prerequisite gap `verified: nerdmd.go:100-103`, `facts.go:86-93`). No new config file, no ambient env capture. Whether opt-in per command or global is open.

## 5. How vision will be judged

* Every safety-relevant instruction is a `Decl`-typed fact with a bounded derivation and an observable denial (not prompt prose).
* Parse strictness thesis holds: `verified: nerdmd.go:23-27` — a dropped directive is worse than no directive.
* Existing verification stays green: `go test ./internal/projectdoc -run TestParse -count=1 -v`, `go vet ./internal/projectdoc`, `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py`.
* `PromptSection` restatement prevents surprise denial wasting a turn; enforcement remains kernel's (`verified: facts.go:158-248`).

## 6. Open choices (see OPEN-QUESTIONS.md)

* Whether `require` enforcement is `permitted(handoff,…)` vs post-tool advisory check.
* Whether `project_write_denied` consumption requires Go/Mangle normalisation proof artifact.
* Whether `commands.env` hermetic execution is opt-in per command or global — `PROPOSED UPLIFT`, needs decision before implementation.
* Exact `internal/prompt/compiler.go` injection point for `PromptSection()` (`ASSUMPTION` — not re-read this turn).

## 7. Risks if vision drifts

* Body pattern-matched as facts → Mangle natural-language matching (explicitly forbidden `verified: facts.go:47-52`).
* Glob semantics divergent between Go and Mangle → gate opens on some paths.
* Range-checked schema → newer document half-applied, author believes rule is live when it is not.
