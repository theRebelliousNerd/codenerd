# `internal/projectdoc` — `nerd.md` Project-Instruction Subsystem

> **Corpus type:** Realized — evidence-grounded. All behavioral claims cite `path#line` or `test#Name`.
> **Verification:** `go test ./internal/projectdoc -run TestParse -count=1 -v` , `go vet ./internal/projectdoc`
> **Last verified:** 2026-05-13 (HEAD) — re-synthesized 2026-08-07 from `internal/projectdoc/nerdmd.go`, `facts.go`, `schemas_projectdoc.mg`, `policy/projectdoc.mg`, `executor_tools.go`
> **Source inventory:** 3 Go files / ~900 lines in `internal/projectdoc/` + 2 Mangle surfaces in `internal/core/defaults/`

---

## 1. In one minute

**Feature:** `nerd.md` — one optional file at the workspace root (`nerd.md`, fallback `.nerd/nerd.md`) that lets a project declare its own instructions: canonical `build`/`test`/`lint` commands, required environment, write-protected paths, non-negotiable steps, and named conventions.

**User:** Any team that has lost time to a guessed build command, a silently overwritten config, or a `CLAUDE.md` line the model treated as a suggestion.

**Problem:** `CLAUDE.md`/`AGENTS.md` are prose appended to a prompt. The model *usually* honours them. A single write to `.nerd/config.json` or `.nerd/mangle/learned` has already destroyed ~160 lines of user-owned state — and prose cannot reliably prevent the next one.

**Visible outcome:** Frontmatter the author believes is in force is *exactly* what the kernel enforces. An unknown key, bad schema, or empty `forbid.match` is a **hard parse error naming the line** — never silently dropped. A forbidden path is **denied before the tool runs** with `blocked by nerd.md: <path> is write-protected (<reason>)` (`internal/session/executor_tools.go:529`), costs a turn, and changes nothing. The advisory Markdown body is rendered into the prompt so the model is not surprised.

---

## 2. Its place in codeNERD

```
Natural language → perception transducer → Mangle atoms → kernel derives next_action → VirtualStore → articulation
                         ▲                                      ▲
                         │                                      │
              nerd.md frontmatter ── Facts() ──► kernel overlay    │
              nerd.md body ────── PromptSection() ──► JIT prompt ──┘
```

**Boundary:**

- **Inside:** Discovery (`Find`), loading (`Load`), strict parsing (`Parse` + `splitFrontmatter` + `validate`), fact projection (`Facts`, `normalizeAtom`, `CommandCount`), prompt rendering (`PromptSection`), in-process helper (`ForbidsPath`) — all in `internal/projectdoc/` (`internal/projectdoc/nerdmd.go:1-293`, `facts.go:1-250`).
- **Outside:** Predicate `Decl`s (`internal/core/defaults/schemas_projectdoc.mg` + `schemas_project.mg` for `project_language/1`), derived policy (`policy/projectdoc.mg`), **live enforcement** (`internal/session/executor_tools.go:406-445` `projectForbidsWrite`), JIT language dimension (`internal/prompt/compiler.go` / `internal/prompt/atoms/`), lifecycle wiring at boot (`internal/core/kernel_*.go`, `internal/session/`).

**Creative-center / executive split (load-bearing):**

| Half of `nerd.md` | Form | Semantics | Projection | Enforcement |
|---|---|---|---|---|
| YAML frontmatter (`---` fenced, `nerd/v1`) | Strict schema, `KnownFields(true)` | Machine-readable | Kernel facts (`project_*`) | Mangle policy → `permitted` / denial **before** tool execution |
| Markdown body (after second `---`) | Free prose, `TrimSpace` verbatim | Advisory | Single prompt atom | None — never projected as facts (`facts.go:47-52`) |

> `nerdmd.go:23-27`: *“A directive the user believes is in force but which the parser dropped is worse than no directive at all.”* Strictness is the thesis.

---

## 3. A representative journey

**Author writes `nerd.md`:**

```yaml
---
schema: nerd/v1
project: codeNERD
language: go
commands:
  build: go build -o nerd.exe ./cmd/nerd
  test: go test ./...
  env: { CGO_CFLAGS: -IC:/CodeProjects/codeNERD/sqlite_headers }
forbid:
  - {match: .nerd/config.json, reason: "Live user-owned runtime config..."}
require: ["Run `go test ./...` before handoff."]
conventions: [{id: conventional-commits, rule: "Subjects use feat/fix..."}]
---
Body prose here — advisory.
```

**Boot:**

1. `projectdoc.Find(workspace)` checks `<ws>/nerd.md` then `<ws>/.nerd/nerd.md` (`nerdmd.go:131-140`). Returns `""` if absent — **not an error**.
2. `Load` → `os.ReadFile` → `Parse` → `splitFrontmatter` (4 MiB scanner, `nerdmd.go:188-189`) → strict YAML `KnownFields(true)` (`nerdmd.go:171`) → `validate` (`nerdmd.go:232-272`) → `Document{Path, Spec, Body}` with `Path` slash-normalised (`nerdmd.go:153-158`).
3. `Facts()` projects only frontmatter (`facts.go:54-131`). `Body` never becomes a fact.
4. Facts asserted into kernel overlay (Decls in `schemas_projectdoc.mg:18-71`; `project_language/1` lives in `schemas_project.mg` — duplicate Decl would panic at boot).
5. `PromptSection()` renders prose summary (`facts.go:158-248`) for JIT injection so the model learns the forbid **before** it wastes a turn.

**Tool call — write to `.nerd/config.json`:**

1. Executor's `projectForbidsWrite` enumerates `project_forbidden_path/2` facts (`executor_tools.go:422`), normalises both target and `Match` with `filepath.ToSlash` + `strings.ToLower` + `strings.Contains` (same as `ForbidsPath` helper `nerdmd.go:281-293`).
2. Match → denied: `blocked by nerd.md: .nerd/config.json is write-protected (Live user-owned...)` (`executor_tools.go:529`). No file modified.

**Failure — author typos `projekt: go`:**
`Parse` fails `invalid frontmatter: yaml: unmarshal errors: line N: field projekt not found in type projectdoc.Spec` (`nerdmd.go:172-174`, proven `TestParse_UnknownKeyIsAHardError`). Author learns immediately; older binary with `nerd/v9` fails `unsupported schema "nerd/v9"; this build speaks "nerd/v1"` (`nerdmd.go:240-245`).

---

## 4. What exists today

| Capability | Status | Evidence |
|---|---|---|
| Discovery `Find` | `VERIFIED CURRENT` | `nerdmd.go:131-140`, `TestFind_PrefersWorkspaceRootOverNerdDir` |
| Load `Load(workspace)` | `VERIFIED CURRENT` | `nerdmd.go:145-160`, `TestLoad_AbsentFileIsNotAnError`, `TestLoad_MalformedFileIsAnError` |
| Strict parse `Parse` + `splitFrontmatter` | `VERIFIED CURRENT` | `nerdmd.go:163-220`, `TestParse_RequiresFrontmatter`, `TestParse_UnknownKeyIsAHardError`, `TestParse_SchemaVersionIsPinned` |
| Schema pin `nerd/v1` | `VERIFIED CURRENT` | `nerdmd.go:36,43,238-245` |
| `ForbidsPath` helper | `VERIFIED CURRENT` | `nerdmd.go:281-293`, `TestForbidsPath*` |
| Fact projection `Facts()` + `normalizeAtom` + `CommandCount` | `VERIFIED CURRENT` | `facts.go:54-156`, `TestFacts`, `TestFacts_AllConvertToAtoms` |
| Prompt rendering `PromptSection` | `VERIFIED CURRENT` | `facts.go:158-248`, `TestPromptSection_StatesThatProtectionIsEnforced` |
| Mangle predicate Decls (10 predicates) | `VERIFIED CURRENT` | `schemas_projectdoc.mg:18,21,37,42,49,53,56,63,68,71` + `schemas_project.mg` for `project_language/1` |
| Live write-protection gate | `VERIFIED CURRENT` | `executor_tools.go:406-445` `projectForbidsWrite` + `facts.go:95-101` `PredForbiddenPath` |
| Body advisory non-projection | `VERIFIED CURRENT` | `facts.go:47-52` + `PromptSection` as sole body consumer |
| Policy derived predicates | `PARTIAL` | `policy/projectdoc.mg:7,13,32-34` — `has_project_doc`, `project_has_command` live; `project_write_denied`/`coder_block_write` bridge **dormant** (not consumed by executor) |
| `require`/`conventions` enforcement | `PROPOSED UPLIFT` | Facts emitted (`facts.go:103-124`) but no Go gate queries them; policy-available only |

**Inventory:** 3 Go files, ~900 lines, 0 `.mg` in-package (Mangle surfaces intentionally in `internal/core/defaults/`). 15+ tests in `nerdmd_test.go`. No floating `N-A` lanes — every lane answered with `APPLICABLE` and cited evidence (see `09-SAFETY-AND-INVARIANTS.md`, `08-WIRING-AND-INTEGRATION.md`).

**Honest gaps:** `require` and `conventions` are surfaced to the model and to policy but not enforced by a tool gate today; the Mangle `project_write_denied` → `coder_block_write` derivation exists but the executor's live gate still queries facts directly rather than consuming the derived denial. Wiring `require` into a verifiable gate is the next safe uplift (see `TODO.md`).

---

## 5. North star

**Desired outcome:** Every project instruction that matters for safety or reproducibility is a **typed fact** with a `Decl`, a bounded derivation, and an observable denial — never prompt folklore. Language selection, canonical commands, and write-protection are resolved from `nerd.md` without opening a source file or guessing.

**Explicit non-goals:**

- Turning the Markdown body into facts. Body is natural language; `facts.go:47-52` forbids asserting free text as facts to avoid pattern-matching prose in policy.
- Glob semantics for `forbid.match`. Substring on slash-normalised, case-insensitive paths is deliberate so Go (`ForbidsPath`) and Mangle (`contains`-style) **agree** (`nerdmd.go:109-114`, `schemas_projectdoc.mg:40-44`). A disagreeing glob is a gate that sometimes opens.
- Range-checked schema. `SchemaVersion` is **pinned** (`nerdmd.go:38-43`); newer documents fail loudly rather than half-apply.

---

## 6. Improvement frontier

**Safe truth-gap repair (next):** Wire `project_requirement` and `project_convention` into verifiable lifecycle gates without adding a parallel subsystem.

- `require` → pre-handoff check: derive `handoff_blocked(Requirement)` from `project_requirement` and check tool history / test receipts; surface in `PromptSection` already, but add kernel `permitted(handoff,…)` derivation and executor check parallel to `projectForbidsWrite`. Small, fact-typed, observable.
- Move executor's live `projectForbidsWrite` to consume the existing dormant `project_write_denied` / `coder_block_write` derivation (`policy/projectdoc.mg:32-34`) so there is **one** source of truth (policy) rather than Go duplicating the `Contains` logic. Requires proving equivalence of `ToLower(ToSlash(...))` in both layers.

**Bounded longer-horizon option:** Typed `commands.env` → hermetic command execution. Today `Env` is rendered into the prompt and emitted as `project_command_env/2`; the executor could materialise those env vars when running the canonical `build`/`test` commands, closing the `CGO_CFLAGS`-style invisible-prerequisite gap (`nerdmd.go:100-103`).

Both keep executive control in typed facts, Mangle policy, and JIT selection — not a new subsystem.

---

## 7. Choose a reading route

**90-second orientation:** This README (§1-§4) + `Docs/architecture/projectdoc.md` (§1-§5 tables).

**10-minute tour:**

1. `02-CURRENT-STATE.md` — precise file/line inventory and status table.
2. `03-MAIN-COMPONENTS.md` — seven components with function signatures and wiring.
3. `06-PUBLIC-API-AND-TYPES.md` — exported types and error strings verbatim.
4. `09-SAFETY-AND-INVARIANTS.md` — permission, default-deny, trust boundaries.
5. `TODO.md` — one truth-gap card + one bounded north-star card.

**Deep implementation:**

- `IMPLEMENTED_SPEC.md` — flagship realized-truth spec (≥400 lines, every claim cited).
- `05-INTERNAL-ARCHITECTURE.md` — parse pipeline, fact projection, prompt rendering, gate ordering.
- `07-DEPENDENCY-MAP.md` — upstream/downstream with import evidence.
- `08-WIRING-AND-INTEGRATION.md` — boot order, kernel overlay, executor gate, JIT atom.
- `12-FAILURE-MODES.md` — concrete failures, detection, mitigation (includes 4 MiB buffer, float abort, duplicate Decl).
- `_progress.md` — verification date, commit, dirty-tree fingerprint, rubric score.

**Verify locally:**

```bash
go test ./internal/projectdoc -count=1 -v
go vet ./internal/projectdoc
python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
```

---

## Document map

| Doc | Role |
|---|---|
| `README.md` | Human front door (this file) |
| `IMPLEMENTED_SPEC.md` | Flagship realized-truth spec — every claim cited or marked `ASSUMPTION` |
| `02-CURRENT-STATE.md` | Citation-backed inventory (companion to IMPLEMENTED_SPEC) |
| `03-MAIN-COMPONENTS.md` | Seven components with signatures and wiring |
| `00-ALIGNMENT-VISION-REVIEW.md` | Scored dimensions vs north star |
| `01-VISION.md` | Target product/architecture vision |
| `03-GAP-ANALYSIS.md` | Spec vs reality matrix |
| `04-ARCHITECTURAL-PRINCIPLES.md` | Binding principles |
| `05-INTERNAL-ARCHITECTURE.md` | Components, data flow, state |
| `06-PUBLIC-API-AND-TYPES.md` | Exported API and types |
| `07-DEPENDENCY-MAP.md` | Upstream/downstream |
| `08-WIRING-AND-INTEGRATION.md` | Boot, kernel, executor, JIT wiring |
| `09-SAFETY-AND-INVARIANTS.md` | Safety, concurrency, Mangle invariants |
| `10-TESTING-ALIGNMENT.md` | Tests and gaps |
| `11-OBSERVABILITY.md` | Signals and diagnosis |
| `12-FAILURE-MODES.md` | Failure modes |
| `TODO.md` | Authoritative feature cards |
| `OPEN-QUESTIONS.md` | Open choices |
| `_progress.md` | Rebuild verification record |
| `corpus.toml` | Machine ownership |

Related: `nerd.md` (workspace root, canonical example), `Docs/architecture/projectdoc.md` (architecture entry), `Docs/Spec/internal/projectdoc/` (Spec corpus — should mirror this architecture corpus).
