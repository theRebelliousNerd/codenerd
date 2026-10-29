# 03 — Gap Analysis: Vision vs. Current State (`internal/projectdoc` / nerd.md)

> **Package:** `internal/projectdoc` — nerd.md subsystem  
> **Corpus type:** Gap — bridges `01-VISION.md` (aspiration) and `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` (realized)  
> **Generated:** 2026-08-07 — from-scratch rewrite (prior version miscited `SchemaVersion` at `:36` vs real `:43` and `KnownFields` at `:182` vs real `:180`)  
> **Sources re-read this synthesis (every `file:line` below is the tab-prefixed line number returned by `read_file` this turn):** `internal/projectdoc/nerdmd.go:1-300`, `internal/projectdoc/facts.go:1-243`, `internal/session/executor_tools.go:1-764`, `internal/core/defaults/policy/projectdoc.mg:1-42`  
> **Claim discipline:** `ENFORCED` = blocks before tool runs; `FACT ONLY` = kernel fact + prompt prose, no gate; `WIRING GAP` = promised wiring point absent in code.

## 1. Purpose

Compare what the nerd.md subsystem **promises** — strict YAML frontmatter becomes kernel facts and policy derives enforcement so a forbidden path is a denied tool call, not prompt folklore (`internal/projectdoc/nerdmd.go:9-16`, `internal/projectdoc/nerdmd.go:80-83`, `internal/session/executor_tools.go:684-700`) — against what it **actually enforces today** as proven by the four files re-read this turn. Every gap states its consequence and the exact `file:line` that proves it; where a wiring point is missing it says `WIRING GAP` explicitly.

## 2. Vision Restatement (what the comments and types claim)

The thesis (`internal/projectdoc/nerdmd.go:3-16`):

- YAML frontmatter is a **strict schema** (`internal/projectdoc/nerdmd.go:18-21`). Unknown key, bad schema version, or malformed entry is a hard parse error naming the line, never silently dropped.
- The Markdown body is advisory prose, exactly like `CLAUDE.md` (`internal/projectdoc/nerdmd.go:15-16`, `internal/projectdoc/facts.go:42-48`).
- A forbidden path is a `project_forbidden_path` fact (`internal/projectdoc/nerdmd.go:80-83`, `internal/projectdoc/facts.go:32-33`) and the executor refuses any write-mutation tool whose target matches **before the tool runs** (`internal/session/executor_tools.go:684-700`).
- `require` and `conventions` are also facts available to policy (`internal/projectdoc/nerdmd.go:85-91`, `internal/projectdoc/facts.go:35-39`), implying gateable non-negotiable steps and checkable rules.
- `commands` + `commands.env` are canonical build/test/lint/run + env (`internal/projectdoc/nerdmd.go:94-105`) that stop the agent from guessing.
- Pinned schema `nerd/v1` at `internal/projectdoc/nerdmd.go:43` fails loudly on mismatch via `internal/projectdoc/nerdmd.go:236-244`.

The intended wiring (`01-VISION.md` §2) was:

```
Find (internal/projectdoc/nerdmd.go:131) → Load (internal/projectdoc/nerdmd.go:148) → Parse (internal/projectdoc/nerdmd.go:170) → splitFrontmatter 4 MiB buffer (internal/projectdoc/nerdmd.go:200) + KnownFields(true) (internal/projectdoc/nerdmd.go:180) + validate (internal/projectdoc/nerdmd.go:236)
  ├─ Facts() (internal/projectdoc/facts.go:51) → kernel overlay → policy derives has_project_doc / project_write_protected / project_has_command / project_write_denied (internal/core/defaults/policy/projectdoc.mg:7-41)
  │                                                              └─ executor consumes derived denial before write tools (intended)
  └─ PromptSection() (internal/projectdoc/facts.go:156) → prompt so model learns forbid before wasting a turn (internal/session/executor_tools.go:439-456)
```

## 3. What is actually true today (verified by re-read)

All `VERIFIED` lines below are the `read_file` tab numbers this turn — not estimates.

| # | Capability | Status | Exact evidence |
|---|------------|:------:|---|
| C1 | Discovery `Find` ordered `<ws>/nerd.md` → `<ws>/.nerd/nerd.md` → `""` with `!IsDir` guard | VERIFIED | `internal/projectdoc/nerdmd.go:131-141` (`filepath.Join(workspace, FileName)` at :132, `filepath.Join(workspace, ".nerd", FileName)` at :134, `!info.IsDir()` at :136) |
| C2 | `Load` returns `(nil,nil)` on absence, wraps errors as `read <path>:` / `<path>:` | VERIFIED | `internal/projectdoc/nerdmd.go:148-167` (`:149-151` nil-nil, `:153` read wrap, `:159` parse wrap) |
| C3 | `Parse` splits frontmatter, strict `KnownFields(true)`, 4 MiB scanner buffer, exact fence errors | VERIFIED | `internal/projectdoc/nerdmd.go:170-189` (decoder at `:177`, `decoder.KnownFields(true)` at `:180`, `Decode` at `:181`), `splitFrontmatter` at `:195-234` with `scanner.Buffer(..., 4*1024*1024)` at `:200`, fence checks at `:205` and `:215` |
| C4 | Schema pin `nerd/v1` and `validate` 7 error classes | VERIFIED | `internal/projectdoc/nerdmd.go:43` `const SchemaVersion = "nerd/v1"`, `internal/projectdoc/nerdmd.go:236-274` (missing/unsupported schema :237-244, `forbid` match/reason :247-255, conventions id/rule :258-265, require empty :267-270) |
| C5 | File identity constants and types | VERIFIED | `internal/projectdoc/nerdmd.go:36` `FileName = "nerd.md"`, `internal/projectdoc/nerdmd.go:46` `type Document`, `internal/projectdoc/nerdmd.go:63` `type Spec`, `internal/projectdoc/nerdmd.go:95` `type Commands`, `internal/projectdoc/nerdmd.go:108` `type ForbidRule`, `internal/projectdoc/nerdmd.go:124` `type Convention` |
| C6 | `ForbidsPath` helper case-insensitive substring, nil/empty safe | VERIFIED | `internal/projectdoc/nerdmd.go:281-292` (nil guard :282, `strings.ToLower(filepath.ToSlash(target))` at :285, `strings.Contains(..., strings.ToLower(filepath.ToSlash(rule.Match)))` at :287) |
| C7 | `Facts()` emits 8 predicate families, nil→nil, body never becomes fact, `MangleAtom` for `/lang` and `/build` etc. | VERIFIED | `internal/projectdoc/facts.go:15` `PredPresent`, `:18` `PredName`, `:22` `PredLanguage`, `:26` `PredCommand`, `:30` `PredCommandEnv`, `:33` `PredForbiddenPath`, `:36` `PredRequirement`, `:39` `PredConvention`; `Facts()` at `:51-109` (nil→nil :52-53, `PredPresent` at :57, `MangleAtom` at :68, `/build` map at :71-83, `PredCommandEnv` at :86-91, `PredForbiddenPath` at :93-98, `PredRequirement` at :100-102, `PredConvention` at :104-106) |
| C8 | `normalizeAtom` `/lang` → `/go` (lowercase, strip `/`, `- .`→`_`) | VERIFIED | `internal/projectdoc/facts.go:129-148` (`TrimPrefix "/"` at :130, char map at :137-141, length guard at :144) |
| C9 | `PromptSection()` 6 ordered sections, `ENFORCED` preamble for forbid, body verbatim last | VERIFIED | `internal/projectdoc/facts.go:156-242` (header at :162-164, commands at :172-198, `Write-protected paths (ENFORCED)` at :200-212 with denial text at :202-203, Required at :214-222, Conventions at :224-234, body at :236-239) |
| C10 | Live write-protection gate queries kernel `project_forbidden_path` before write tools, substring case-insensitive | VERIFIED | `internal/session/executor_tools.go:487-524` (`isWriteMutationTool` guard at :488, `projectDocTargetPath` at :491, `e.kernel.Query(projectdoc.PredForbiddenPath)` at :499, normalized compare at :510-519, `strings.Contains(normalized, match)` at :519), enforcement callsite `if reason, denied := e.projectForbidsWrite(call); denied` at :695 inside `executeToolCall` at :669 (after safety gate at :679-682, before Dreamer preflight at :707-712) |
| C11 | Write-mutation identity and path-arg extraction | VERIFIED | `internal/session/executor_tools.go:411-424` `isWriteMutationTool` (registered writes at :413-417), `internal/session/executor_tools.go:463` `projectDocPathArgs` list, `internal/session/executor_tools.go:466-475` `projectDocTargetPath` |
| C12 | Prompt injection appends after JIT | VERIFIED | `internal/session/executor_tools.go:439-456` `withProjectInstructions` (`section := doc.PromptSection()` at :450, append at :455), `SetProjectDoc` at :433-437, `CommandCount()` at `internal/projectdoc/facts.go:112-123` |
| C13 | Derived predicates `has_project_doc`, `project_write_protected`, `project_has_command` exist | VERIFIED | `internal/core/defaults/policy/projectdoc.mg:7-14` (`has_project_doc()` at :7, `project_write_protected()` at :10, `project_has_command(Kind)` at :13) |
| C14 | Dormant `project_write_denied` / `coder_block_write` via `pending_edit` + `path_contains` | VERIFIED | `internal/core/defaults/policy/projectdoc.mg:26-41` (`Decl project_write_denied` at :32, rule at :33-36, `coder_block_write` at :40-41) with dormancy comment at :27-31 |

## 4. Gap Matrix — promise vs. enforcement (the only deltas)

| # | Dimension | What is promised (comment/type) | What actually happens (file:line) | Consequence | Class | Next step |
|---|-----------|----------------------------------|------------------------------------|-------------|-------|-----------|
| G1 | `require` enforcement | Non-negotiable steps (`internal/projectdoc/nerdmd.go:85-88` `Require []string "non-negotiable"`, `internal/projectdoc/facts.go:35-36` `project_requirement`) block handoff until satisfied | Emitted as `project_requirement` at `internal/projectdoc/facts.go:100-102` and rendered in `PromptSection` at `internal/projectdoc/facts.go:214-222`; **no Go gate queries `project_requirement`** in `internal/session/executor_tools.go:669-700` and **no policy `handoff_blocked` derivation** exists in `internal/core/defaults/policy/projectdoc.mg:1-42` | Agent can handoff without running `go test ./...`; requirement is folklore despite being a typed fact | **WIRING GAP** — `PROPOSED UPLIFT` | Derive `handoff_blocked(Req)` from `project_requirement` + tool-history/receipt facts in policy; add executor pre-handoff check parallel to `projectForbidsWrite` at `internal/session/executor_tools.go:487-524` |
| G2 | `conventions` enforcement | Named, checkable rules (`internal/projectdoc/nerdmd.go:90-91`, `internal/projectdoc/facts.go:38-39` `project_convention`) | Emitted at `internal/projectdoc/facts.go:104-106`, rendered at `internal/projectdoc/facts.go:224-234`; **no gate or lint derivation** anywhere in `internal/session/executor_tools.go` or `internal/core/defaults/policy/projectdoc.mg:1-42` | Conventions never block; "checkable" is unenforceable prose | **WIRING GAP** — `PROPOSED UPLIFT` | Decide prompt-only vs verifiable via `perception`/lint receipts; reuse G1 gate pattern if lintable. Lower priority than G1 |
| G3 | `commands` / `commands.env` hermetics | Canonical commands + env are ground truth (`internal/projectdoc/nerdmd.go:94-105`, comment at :98-103 `CGO_CFLAGS`-style prereqs invisible in command string) | Emitted as `project_command` at `internal/projectdoc/facts.go:71-83` and `project_command_env` at `internal/projectdoc/facts.go:86-91`, prompt-rendered at `internal/projectdoc/facts.go:172-198`; **never applied to process env** when executor runs tools — no `os.Setenv` or `cmd.Env` wiring in `internal/session/executor_tools.go:669-730` | Build prerequisites documented but not materialized; absence produces confusing failure far from cause — exactly the failure `nerdmd.go:98-103` warns about | **WIRING GAP** — `PROPOSED UPLIFT` | Materialize `project_command_env` when running canonical `build/test/lint/run` (per-command or global — open question). Closes gap noted at `internal/projectdoc/nerdmd.go:98-103`. Not a new subsystem |
| G4 | Single source for write-protection (policy vs direct query) | Policy derived `project_write_denied`/`coder_block_write` is intended sole denial (`internal/core/defaults/policy/projectdoc.mg:32-41`, comment at :16-31) | `project_write_denied` at `:33-36` depends on `pending_edit(Path,_)` which **has no Go producer** (comment at `:27-29` says so) so derivation is dormant and `coder_block_write` at `:40-41` never fires. Live gate queries `project_forbidden_path` facts directly at `internal/session/executor_tools.go:499` and re-implements normalization (`ToLower(ToSlash)+Contains` duplicated at `nerdmd.go:285-287` and `executor_tools.go:510-519`) | Two sources, one dead — safety depends on Go string matching agreeing with Mangle `path_contains` after identical normalization; drift would be silent | **WIRING GAP** (intended derivation dormant) — `PARTIAL` | Move executor to consume derived `project_write_denied`/`coder_block_write` once `pending_edit` is wired (or replace Mangle `path_contains` with proven-equivalent normalized check). Requires proof that Go `strings.Contains(ToLower(ToSlash))` at `nerdmd.go:285,287` ≡ Mangle `path_contains` at `policy/projectdoc.mg:36`. Highest leverage |
| G5 | `project_language` / JIT `/lang` wiring | `Spec.Language` (`internal/projectdoc/nerdmd.go:70-73`) seeds JIT compilation context `/lang` dimension so language atoms selected without opening files (`internal/projectdoc/facts.go:64-68` comment at :65-67) | `normalizeAtom` → `types.MangleAtom(lang)` correct at `internal/projectdoc/facts.go:64-68` and `:129-148`; facts `project_language/1` emitted. **No `current_context(/lang, …)` derivation or ToolDefinitions wiring verified** in re-read corpus (no `current_context` in `policy/projectdoc.mg:1-42`; not visible in `executor_tools.go:360-424` allowlist logic which only gates by name at `:673-675`) | Fact exists but JIT language selection still likely file-scan–driven; promise of zero-file language seeding unverified | **WIRING GAP** (unverified derivation) | Verify consumption in `internal/prompt/compiler.go` + atoms; if absent, derive `current_context(/lang, Lang)` from `project_language` |
| G6 | Subagent inheritance | Spawned subagent inherits same project rules | `SetProjectDoc` at `internal/session/executor_tools.go:433-437` holds prose on parent executor; `withProjectInstructions` at `:445-456` is nil-safe (`doc.PromptSection() == ""` → return prompt at :450-452) so subagent with `projectDoc==nil` gets **enforcement via shared kernel** (facts are overlay) but **loses prose nudge** and costs a turn to learn by denial. No `projectDoc` field found on Spawner in re-read corpus | Enforcement survives, UX does not — subagent wastes a turn discovering forbid by being blocked | `PARTIAL` (enforcement ok, prose missing) | Forward `bctx.projectDoc` into spawner or re-render prose from kernel `project_forbidden_path` facts so subagent prompt includes forbid without extra wiring |
| G7 | `ForbidsPath` helper vs kernel gate divergence | Helper at `internal/projectdoc/nerdmd.go:281` is the documented API for path checks | `ForbidsPath` at `:281-292` and `projectForbidsWrite` at `internal/session/executor_tools.go:487-524` duplicate substring case-insensitive logic independently; kernel query failure at `:499-507` **fails open loudly** (warn and allow write) | Two independent implementations of the same safety predicate; kernel hiccup degrades from "deny" to "allow" — correct per availability rationale at `:501-504` but inconsistent with Document API expectations | `PARTIAL` (intentional fail-open, but duplication) | Collapse to one normalizer (`filepath.ToSlash`+`ToLower`+`Contains`) shared by Go helper and policy `path_contains`, and decide fail-open vs fail-closed explicitly in one place |

## 5. What is NOT a gap (intentionally out of scope, correctly implemented)

- **Body never becomes a fact** — by design at `internal/projectdoc/facts.go:42-48` (`Only the frontmatter is projected. The Markdown body … belongs in the prompt, not the fact store`). Prose as facts would invite pattern-matching natural language in Mangle, which is explicitly forbidden.
- **Strictness** — `KnownFields(true)` at `internal/projectdoc/nerdmd.go:180` (not `:182` as prior version claimed) makes unknown keys a hard error; `SchemaVersion` pin at `internal/projectdoc/nerdmd.go:43` (not `:36`) fails loudly at `internal/projectdoc/nerdmd.go:238-244`. `validate` at `:236-274` covers 7 classes: missing/unsupported schema, `forbid` match/reason, conventions id/rule, require empty, unknown key via KnownFields. An unreadable directive never degrades to "no directive" (`internal/projectdoc/nerdmd.go:143-167`).
- **Discovery fallback** — `Find` at `internal/projectdoc/nerdmd.go:131-141` checks `nerd.md` then `.nerd/nerd.md` (not `docs/` etc.), `Load` returns `(nil,nil)` on absence at `:149-151` so repo without nerd.md behaves unchanged.
- **Prompt append after JIT** — `withProjectInstructions` at `internal/session/executor_tools.go:439-456` appends `PromptSection` at `:455` after compilation so project instructions are immune to atom budget eviction. This bypass of the JIT atom scorer is intentional, not a missing wire.

## 6. Prioritization (same ordering as `README.md` §6 + `TODO.md` — highest leverage first)

1. **G4 — Consume dormant policy derivation** — Reach: every write-mutation tool call via `internal/session/executor_tools.go:487-524` and `:695`; Impact: single source of truth, removes drift between `nerdmd.go:285-287` and `policy/projectdoc.mg:36` `path_contains`; Effort: M (normalization equivalence proof).
2. **G1 — `require` → `handoff_blocked`** — Reach: handoff boundary; Impact: closes "non-negotiable means nothing" gap; Effort: M (needs tool-history truth source for "requirement satisfied").
3. **G3 — `commands.env` materialization** — Reach: canonical command invocations; Impact: hermetic `CGO_CFLAGS` gap closed; Effort: S.
4. **G2 / G5 — conventions & language context** — Lower until G1/G4 proven; effort S each.
5. **G6 — Subagent prose forwarding** — Polish; no safety impact since kernel enforcement already survives.

## 7. How to verify (reproducible from re-read corpus, 5 minutes of fresh clone)

```bash
# Correct citations (tab numbers from read_file):
grep -n "SchemaVersion" internal/projectdoc/nerdmd.go          # → :43  const SchemaVersion = "nerd/v1"  (prior doc said :36 — wrong)
grep -n "KnownFields" internal/projectdoc/nerdmd.go            # → :180 decoder.KnownFields(true)       (prior doc said :182 — wrong)
grep -n "Frontmatter\|Load\|Find\|ForbidsPath" internal/projectdoc/nerdmd.go  # :131 Find, :148 Load, :170 Parse, :281 ForbidsPath

# Projection vs enforcement:
grep -n "Pred.*project_" internal/projectdoc/facts.go          # :15,18,22,26,30,33,36,39 — Facts() at :51-109, PromptSection at :156
grep -n "projectForbidsWrite\|PredForbiddenPath" internal/session/executor_tools.go  # live gate at :487-524, query at :499, enforcement at :695
grep -n "project_write_denied\|coder_block_write\|pending_edit\|path_contains" internal/core/defaults/policy/projectdoc.mg  # dormant at :32-41, pending_edit has no producer per :27-29
grep -n "has_project_doc\|project_write_protected\|project_has_command" internal/core/defaults/policy/projectdoc.mg  # :7, :10, :13 — derived but no consumer for write_denied
```

`go test ./internal/projectdoc -count=1 -v` plus `go vet ./internal/projectdoc` reproduces C1-C9 via the `file:line` above; the three `grep` receipts above show G1-G4 dormancy and the live gate.

## 8. Risks if left as-is

- `require`/`conventions` remain prompt folklore — exactly what the Mangle thesis (`internal/projectdoc/nerdmd.go:3-16`) claims to replace. A future maintainer edits `nerd.md` to add `require: ["run go vet ./..."]` and assumes it blocks handoff; it does not.
- Dormant `project_write_denied` means the only defense is the Go `strings.Contains(ToLower(ToSlash))` duplicate. If either normalizer drifts, the safety gate opens without changing any test (no single test asserts both paths).
- `commands.env` invisibility reproduces the `CGO_CFLAGS` failure at a confusing distance from its cause — the comment at `internal/projectdoc/nerdmd.go:98-103` already predicts the failure mode.

---
*Every `file:line` above is the real `read_file` line number (prefix before the tab) from this turn. Prior synthesis line estimates were not re-used.*
