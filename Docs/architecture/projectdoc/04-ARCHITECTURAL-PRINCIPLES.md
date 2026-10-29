# 04 — Architectural Principles: `nerd.md` Project-Instruction Subsystem

> **Corpus type:** Normative — invariants that constrain `internal/projectdoc` + `internal/session` interaction. Every claim cites `file:line` verified live in this turn via `read_file` (which prefixes each line with its real number and a tab).
> **Scope:** `internal/projectdoc/nerdmd.go`, `internal/projectdoc/facts.go`, `internal/session/executor_tools.go` as read 2026-08-08.

## 1. Principle: Frontmatter Is Strict, Body Is Advisory

`nerd.md` deliberately splits along the creative / executive line described at `internal/projectdoc/nerdmd.go:1-17`. The YAML frontmatter fenced by `---` is a strict schema that becomes enforcement; the Markdown body is prose that becomes advice.

**Why strict.** The thesis is stated explicitly at `internal/projectdoc/nerdmd.go:18-21`: a dropped directive is worse than no directive. An unknown key, bad schema version, or malformed entry is a hard error naming the line, never a silently ignored field.

Mechanically:

* `Parse` at `internal/projectdoc/nerdmd.go:170-189` splits frontmatter from body via `splitFrontmatter` and then decodes with `decoder.KnownFields(true)` at `internal/projectdoc/nerdmd.go:180`. The comment at `internal/projectdoc/nerdmd.go:178-179` is explicit: failing on an unknown key is the only way the author finds out the binary will not honour their directive. Decode failure is wrapped as `invalid frontmatter` at `internal/projectdoc/nerdmd.go:182`.
* After decode, `spec.validate()` at `internal/projectdoc/nerdmd.go:185` enforces the contract at `internal/projectdoc/nerdmd.go:236-273`: missing `schema` at `internal/projectdoc/nerdmd.go:237-238`, unsupported schema at `internal/projectdoc/nerdmd.go:240-244`, empty `forbid[].match` at `internal/projectdoc/nerdmd.go:248-249`, missing `forbid[].reason` at `internal/projectdoc/nerdmd.go:251-255`, empty convention id/rule at `internal/projectdoc/nerdmd.go:258-265`, and empty require at `internal/projectdoc/nerdmd.go:267-270`.
* `splitFrontmatter` at `internal/projectdoc/nerdmd.go:195-234` requires the first line to be `---` at `internal/projectdoc/nerdmd.go:205-210` and requires the block to be closed at `internal/projectdoc/nerdmd.go:221-223`, with an explicit 4 MiB buffer at `internal/projectdoc/nerdmd.go:200` so a long body line cannot masquerade as a frontmatter error.
* The schema itself is pinned: `SchemaVersion = "nerd/v1"` at `internal/projectdoc/nerdmd.go:43` with the comment at `internal/projectdoc/nerdmd.go:39-42` that it is pinned rather than range-checked so newer documents fail loudly. Adding a field to `Spec` at `internal/projectdoc/nerdmd.go:58-92` is defined at `internal/projectdoc/nerdmd.go:60-62` as a schema change that must bump that version.

**Why advisory.** The body half is the opposite kind by construction:

* `Document.Body` at `internal/projectdoc/nerdmd.go:53-55` is `Markdown prose after the frontmatter, verbatim and trimmed. Advisory only.`
* Only the frontmatter is projected to facts. The comment at `internal/projectdoc/facts.go:42-47` states the body belongs in the prompt, not the fact store, and `Facts()` at `internal/projectdoc/facts.go:51-54` projects `d.Spec` only.
* Rendering the body for the model is a separate path: `PromptSection()` at `internal/projectdoc/facts.go:156-242` builds prose for prompt injection, including the forbid/require/conventions/commands summaries and then the trimmed body at `internal/projectdoc/facts.go:236-238`. The section header at `internal/projectdoc/facts.go:162-164` stamps the source path so the injection is traceable.

**Consequence:** Writing a rule in prose does not create enforcement. Writing it in frontmatter does. The two containers never mix (`internal/projectdoc/facts.go:42-47` forbids asserting free text as fact).

## 2. Principle: Facts Go to the Kernel, Not a Go Struct

A `forbid` rule does not become a cached Go map consulted by the executor. It becomes a kernel fact that policy and every executor path query under the same name.

* The enforced predicate is declared at `internal/projectdoc/facts.go:32-33` as `PredForbiddenPath = "project_forbidden_path"` with the comment `the ENFORCED one` and doc `project_forbidden_path(Match, Reason)`. Sibling predicates for present/name/language/command/env/requirement/convention are at `internal/projectdoc/facts.go:12-39`.
* Projection is at `internal/projectdoc/facts.go:51-108`. `Facts()` emits `project_doc` at `internal/projectdoc/facts.go:56-58`, optional `project_name` at `internal/projectdoc/facts.go:60-62`, normalized `project_language` atom at `internal/projectdoc/facts.go:64-69` via `normalizeAtom` at `internal/projectdoc/facts.go:129-148`, `project_command` for `/build`/`/test`/`/lint`/`/run` at `internal/projectdoc/facts.go:71-84`, `project_command_env` at `internal/projectdoc/facts.go:86-91`, `project_forbidden_path` at `internal/projectdoc/facts.go:93-98`, `project_requirement` at `internal/projectdoc/facts.go:100-102`, and `project_convention` at `internal/projectdoc/facts.go:104-106`.
* The authoritative gate is `projectForbidsWrite` at `internal/session/executor_tools.go:487-524`. Its header comment at `internal/session/executor_tools.go:477-486` states the kernel is the authority, not a cached Go struct: facts are asserted at boot like any EDB so policy, `/query`, and the gate see exactly the same rules, and a parallel in-memory copy would drift after one refactor.
* `SetProjectDoc` at `internal/session/executor_tools.go:426-437` makes the structural implication explicit at `internal/session/executor_tools.go:426-432`: only the prose rendering is held on the executor; write protection is enforced by querying the kernel, so a subagent that never receives the pointer is still governed.
* Enforcement ordering in `executeToolCall` at `internal/session/executor_tools.go:669-700` is `isToolAllowed` → `checkSafety` → `projectForbidsWrite` → Dreamer preflight. The comment at `internal/session/executor_tools.go:684-694` notes `projectForbidsWrite` at `internal/session/executor_tools.go:695-700` intentionally sits after the constitutional gate and before simulation: constitutional rules outrank project rules, and there is no reason to simulate an action already denied. The denial is observable and turns into `blocked by nerd.md: <path> is write-protected (<reason>)` at `internal/session/executor_tools.go:698-699` with a `Warn` at `internal/session/executor_tools.go:696-697`.

Supporting details that keep the gate closed:

* Only write-mutation tools are gated, checked at `internal/session/executor_tools.go:488-489` via `isWriteMutationTool` defined at `internal/session/executor_tools.go:411-424` (which must cover every `VirtualStore` durable-write action).
* Target extraction checks every arg alias that names a path at `internal/session/executor_tools.go:463` (`projectDocPathArgs`) and `internal/session/executor_tools.go:466-475` (`projectDocTargetPath`), so `path` vs `file_path` vs `file` vs `filename` vs `target` vs `dest` vs `destination` cannot bypass the gate.
* Live display of the rule for learnability, without moving enforcement to the prompt, is `PromptSection()` at `internal/projectdoc/facts.go:200-212` for `Write-protected paths (ENFORCED)` plus the restatement in `withProjectInstructions` at `internal/session/executor_tools.go:445-456`.

**Consequence:** There is one representation of a `forbid` rule: the kernel fact. Go structs render it; they never re-decide it.

## 3. Principle: Absence of the File Is Not an Error

`nerd.md` is optional. An absent file means absent facts, not a failed run.

* `FileName` at `internal/projectdoc/nerdmd.go:36` is `"nerd.md"`. `Find` at `internal/projectdoc/nerdmd.go:131-141` searches `filepath.Join(workspace, FileName)` and then `filepath.Join(workspace, ".nerd", FileName)` and returns `""` when absent, documented at `internal/projectdoc/nerdmd.go:130` as `Returns "" when absent, which is not an error: nerd.md is optional.`
* `Load` at `internal/projectdoc/nerdmd.go:148-167` implements the `(nil, nil)` contract: at `internal/projectdoc/nerdmd.go:149-152` it calls `Find`, and if `path == ""` returns `nil, nil`. The doc comment at `internal/projectdoc/nerdmd.go:145-147` states `Returns (nil, nil) when the file does not exist. Returns an error only when the file exists and is invalid — an unreadable directive must never degrade to "no directive".`
* The fact and prompt projections are nil-safe on that path: `Facts()` at `internal/projectdoc/facts.go:51-54` returns `nil` for a nil document `so callers can pass the result of Load straight through without a nil check`, and `PromptSection()` at `internal/projectdoc/facts.go:156-159` returns `""` for nil. `ForbidsPath` at `internal/projectdoc/nerdmd.go:281-283` likewise returns `false` when `d == nil`.
* `withProjectInstructions` at `internal/session/executor_tools.go:445-456` calls `doc.PromptSection()` at `internal/session/executor_tools.go:450` and returns the input `systemPrompt` unchanged when `section == ""` at `internal/session/executor_tools.go:451-453`.

Errors are still surfaced when the file exists:

* `os.ReadFile` failure at `internal/projectdoc/nerdmd.go:153-156` returns `fmt.Errorf("read %s: %w", path, err)`.
* `Parse` failure at `internal/projectdoc/nerdmd.go:157-160` is wrapped as `fmt.Errorf("%s: %w", path, err)` so the line-precise message points at the file.

**Consequence:** A workspace without `nerd.md` behaves exactly as before adoption. A workspace with a malformed `nerd.md` stops loudly, never silently.

## 4. Principle: The `nerd.md` Gate Fails Open on Kernel Error — Loudly

When the kernel cannot be queried for `project_forbidden_path`, the `nerd.md` write gate allows the write and logs a warning. This is intentional and contrasts with the constitutional safety gate, which fails closed.

* `projectForbidsWrite` at `internal/session/executor_tools.go:487-524` queries `projectdoc.PredForbiddenPath` at `internal/session/executor_tools.go:499`. On error it executes the fail-open path at `internal/session/executor_tools.go:500-508`: `Warn` at `internal/session/executor_tools.go:505-506` with `nerd.md write protection could not be evaluated for %s (%v); allowing the write` and `return "", false` at `internal/session/executor_tools.go:507`. The comment at `internal/session/executor_tools.go:501-504` explains why: a transient query failure is not evidence that the path is protected, and blocking every write would make the agent unusable the moment the kernel hiccups.
* The nil-kernel path at `internal/session/executor_tools.go:495-497` similarly returns `("", false)` — no kernel means no derived `forbid` facts to enforce, so no denial fires.
* The read path is symmetric: `intentRequiresToolCall` at `internal/session/executor_tools.go:360-376` queries `intent_requires_tool_call(%s)` at `internal/session/executor_tools.go:367-368` and on error logs at `internal/session/executor_tools.go:370-372` and `return false` at `internal/session/executor_tools.go:373`, documented at `internal/session/executor_tools.go:353-359` as conservatively returning false so a missing kernel never blocks a final answer.
* Contrast with the constitutional gate `checkSafety` at `internal/session/executor_tools.go:798-923`: at `internal/session/executor_tools.go:810-817` it fails closed when `kernel == nil && EnableSafetyGate == true` with `return false` and `Error("Safety check failed closed...")`. The `nerd.md` gate is project policy (fail open on transient evaluation failure); the safety gate is constitutional (fail closed on missing kernel). Both choices are observable through structured logging (`Warn` vs `Error`).

**Consequence:** A kernel hiccup degrades `nerd.md` to a visible warning rather than a global write freeze, while safety invariants remain deny-by-default.

## 5. Supporting Invariants (Grounded)

* **Normalization agreement.** `ForbidsPath` at `internal/projectdoc/nerdmd.go:281-292` normalizes with `strings.ToLower(filepath.ToSlash(...))` at `internal/projectdoc/nerdmd.go:285` and `internal/projectdoc/nerdmd.go:287` before `strings.Contains` at `internal/projectdoc/nerdmd.go:287`. `projectForbidsWrite` at `internal/session/executor_tools.go:510-520` applies the identical normalization at `internal/session/executor_tools.go:510` and `internal/session/executor_tools.go:515` before `Contains` at `internal/session/executor_tools.go:519`. `match` is substring, not glob, by design at `internal/projectdoc/nerdmd.go:108-114` — a glob that disagrees across Go and Mangle is a gate that sometimes opens.
* **Reason required.** `validate` requires non-empty `Reason` at `internal/projectdoc/nerdmd.go:251-255` so a denial the agent cannot explain does not read as a malfunction that invites a workaround.
* **Language as atom.** `project_language` is emitted as `types.MangleAtom` at `internal/projectdoc/facts.go:68` via `normalizeAtom` at `internal/projectdoc/facts.go:129-148` because policy unifies on `/go`, not `"go"` (`internal/projectdoc/facts.go:65-67`).
* **Env-command separation.** `Commands.Env` at `internal/projectdoc/nerdmd.go:100-104` and its projection at `internal/projectdoc/facts.go:86-91` are typed separately from the command strings so `CGO_CFLAGS`-style prerequisites are not invisible in the command string.

## 6. How Conformance Is Judged

* Every `forbid` in frontmatter at `internal/projectdoc/nerdmd.go:80-83` is a `project_forbidden_path` fact at `internal/projectdoc/facts.go:93-98` and a live denial at `internal/session/executor_tools.go:695-700` before filesystem mutation.
* No body text at `internal/projectdoc/nerdmd.go:53-55` becomes a fact (`internal/projectdoc/facts.go:42-47` forbids it); it becomes prompt atom prose at `internal/projectdoc/facts.go:156-242`.
* `KnownFields(true)` at `internal/projectdoc/nerdmd.go:180` plus `validate` at `internal/projectdoc/nerdmd.go:236-273` mean no half-understood document is applied; schema skew at `internal/projectdoc/nerdmd.go:240-244` is a full rejection.
* Absent-file contract at `internal/projectdoc/nerdmd.go:130-131` and `internal/projectdoc/nerdmd.go:145-152` is preserved: absent means `nil` facts and unchanged prompt.
* Kernel-unavailable behavior is fail-open with a `Warn` at `internal/session/executor_tools.go:505-507`, never a silent allow or a global freeze.

## 7. Explicit Non-Goals

* Do not turn body prose into facts (`internal/projectdoc/facts.go:42-47`).
* Do not adopt glob for `forbid.match` (`internal/projectdoc/nerdmd.go:108-114` and `internal/projectdoc/nerdmd.go:281-292` vs `internal/session/executor_tools.go:510-520`).
* Do not range-check schema (`internal/projectdoc/nerdmd.go:39-43` and `internal/projectdoc/nerdmd.go:240-244` — stay pinned to `nerd/v1`).
* Do not cache `forbid` in a Go struct beside the kernel (`internal/session/executor_tools.go:477-486` and `internal/session/executor_tools.go:426-432`).
