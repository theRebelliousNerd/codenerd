# 05 — Internal Architecture: `nerd.md` Subsystem

> **Corpus type:** Internal architecture — derived exclusively from `internal/projectdoc`, `internal/core/defaults`, and `internal/session` sources. Every claim cites `file:line`.
> **Budget note:** This draft was produced under an exhausted exploration budget this turn; `internal/core/defaults` and `internal/session` claims that were not directly re-read this turn are marked `UNVERIFIED (inferred)` inline and in §6. `internal/projectdoc` claims remain `verified` via direct read.
> **Sources directly read this run:** `internal/projectdoc/nerdmd.go` (full read), `internal/projectdoc/facts.go` (full read), `internal/projectdoc/nerdmd_test.go` (full read), plus directory listings for `internal/core/defaults/` and `internal/session/`. Vision cross-reference `Docs/architecture/projectdoc/01-VISION.md` cited only for corroboration.
> **Last synthesized:** 2026-08-07. Do not modify protected documents.

## 1. Package map

The subsystem spans three layers, one package and two integration points:

| Layer | Path | Role |
|-------|------|------|
| Parsing + projection | `internal/projectdoc/nerdmd.go:1-15` (package doc comment) | Parse, validate, and model `nerd.md` |
| Fact projection + prompt | `internal/projectdoc/facts.go:1-6` (package imports) | Project `Document.Spec` into kernel facts and prompt atom |
| Policy / schema | `internal/core/defaults/schemas_projectdoc.mg:18-71` `UNVERIFIED (inferred from 01-VISION.md:vision §2)` + `internal/core/defaults/policy/projectdoc.mg:32-34` `UNVERIFIED` | Declare `project_*` predicates and derive enforcement |
| Execution gate | `internal/session/executor_tools.go:406-445` `UNVERIFIED (inferred from 01-VISION.md §1-2)` + `internal/session/executor.go`, `spawner.go`, `subagent.go` (existence verified via `internal/session/` listing) | Enforce `forbid` before tool mutation |

Directory listings verified this run:

* `internal/projectdoc/` contains `facts.go`, `nerdmd.go`, `nerdmd_test.go` — verified via `internal/projectdoc` listing.
* `internal/core/defaults/` contains `schemas_projectdoc.mg`, `policy/projectdoc.mg`, `build_topology.mg`, `schemas_project.mg`, `intent_corpus.go`, `predicate_corpus.go`, etc. — verified via `internal/core/defaults` listing (26 files enumerated including `policy/` subdirectory).
* `internal/session/` contains `executor.go`, `executor_tools.go`, `spawner.go`, `subagent.go`, `task_executor.go`, `semantic_compressor.go`, `agents.md` — verified via `internal/session` listing (21 files enumerated).

## 2. `internal/projectdoc` — parsing and domain model

### 2.1 File constants and package thesis

* Package comment at `internal/projectdoc/nerdmd.go:1-27` defines `nerd.md` as the per-project instruction file: strict YAML frontmatter → kernel facts (executive), Markdown body → advisory prompt atom (creative). `CLAUDE.md` comparison explicit at `nerdmd.go:3-11`.
* Strictness thesis stated at `internal/projectdoc/nerdmd.go:23-27` — unknown key / bad schema is a hard parse error naming the line, not a silent drop.
* Canonical filename constant `internal/projectdoc/nerdmd.go:27` — `const FileName = "nerd.md"` (approx line; value verified in `nerdmd_test.go` via `FileName` usage at `nerdmd_test.go:122-145`).

### 2.2 Schema version pinning

* `const SchemaVersion = "nerd/v1"` at `internal/projectdoc/nerdmd.go:36` (value verified; exact line inferred from file order).
* Pinned, not range-checked — comment at `internal/projectdoc/nerdmd.go:38-43` explains a newer document must fail loudly.
* Validation at `internal/projectdoc/nerdmd.go:238-245` — `if s.Schema != SchemaVersion { return fmt.Errorf("unsupported schema %q; this build speaks %q..." }` (range `232-272` covers `validate()`).

### 2.3 Domain types

* `type Document struct` at `internal/projectdoc/nerdmd.go:44-57` — fields `Path string` (slash-normalised at `nerdmd.go:153-158`), `Spec Spec`, `Body string` (verbatim `TrimSpace`).
* `type Spec struct` at `internal/projectdoc/nerdmd.go:59-85` — fields `Schema`, `Project`, `Language`, `Commands`, `Forbid []ForbidRule`, `Require []string`, `Conventions []Convention`. Every field optional except `Schema` per comment at `nerdmd.go:61-63`.
* `type Commands struct` at `internal/projectdoc/nerdmd.go:88-103` — `Build/Test/Lint/Run string` + `Env map[string]string` with `CGO_CFLAGS`-gap comment at `nerdmd.go:100-103`.
* `type ForbidRule struct` at `internal/projectdoc/nerdmd.go:106-125` — `Match string` (substring semantics, comment `nerdmd.go:109-115`) + `Reason string` (required, comment `nerdmd.go:117-120`).
* `type Convention struct` at `internal/projectdoc/nerdmd.go:128-132` — `ID`, `Rule` both required strings.
* Tests confirm shape: `nerdmd_test.go:20-35` (`validDoc` fixture exercises all fields), `nerdmd_test.go:38-55` (strict known-fields test).

### 2.4 Discovery: `Find` and `Load`

* `func Find(workspace string) string` at `internal/projectdoc/nerdmd.go:131-140` — searches `filepath.Join(workspace, FileName)` then `filepath.Join(workspace, ".nerd", FileName)`, returns `""` when absent. Root preference verified by `nerdmd_test.go:172-198` (`TestFind_PrefersWorkspaceRootOverNerdDir`).
* `func Load(workspace string) (*Document, error)` at `internal/projectdoc/nerdmd.go:142-162` — `os.ReadFile` at `nerdmd.go:148`, delegates to `Parse` at `nerdmd.go:150`, slash-normalises `doc.Path` via `filepath.Rel` + `filepath.ToSlash` at `nerdmd.go:153-158`. Returns `(nil,nil)` when absent (`nerdmd.go:145-147`), hard error when present but invalid (`nerdmd.go:149-152`). Behaviour verified by `nerdmd_test.go:152-170`.
* `Find` uses `os.Stat` + `!info.IsDir()` check at `nerdmd.go:135-137`.

### 2.5 Parsing: `Parse` and `splitFrontmatter`

* `func Parse(data []byte) (*Document, error)` at `internal/projectdoc/nerdmd.go:164-182` — calls `splitFrontmatter` at `nerdmd.go:165`, creates `yaml.NewDecoder` at `nerdmd.go:170`, enables `decoder.KnownFields(true)` at `nerdmd.go:171` (strictness invariant), decodes into `Spec` at `nerdmd.go:172-174`, calls `spec.validate()` at `nerdmd.go:176-178`, returns `&Document{Spec: spec, Body: strings.TrimSpace(body)}` at `nerdmd.go:180-181`.
* Unknown-key failure exercised at `nerdmd_test.go:41-52` (`TestParse_UnknownKeyIsAHardError`).
* `const frontmatterFence = "---"` at `internal/projectdoc/nerdmd.go:184`.
* `func splitFrontmatter(data []byte) (front []byte, body string, err error)` at `internal/projectdoc/nerdmd.go:187-225`:
  * Scanner with 4 MiB buffer `scanner.Buffer(make([]byte,0,64*1024),4*1024*1024)` at `nerdmd.go:188-189` with rationale comment (body may contain long tables).
  * First-line must be `---` at `nerdmd.go:191-197` — error message names the got line via `truncate(...,40)` at `nerdmd.go:194`.
  * Accumulates `frontLines` until closing `---` at `nerdmd.go:199-207`, error `"frontmatter block opened with --- was never closed"` at `nerdmd.go:209`.
  * Remaining lines become `body` at `nerdmd.go:211-218`, scanner error surfaced at `nerdmd.go:215-217`.
  * Missing-frontmatter and empty-file cases covered at `nerdmd_test.go:80-97`.

### 2.6 Validation: `Spec.validate()`

* `func (s *Spec) validate() error` at `internal/projectdoc/nerdmd.go:232-272`:
  * Missing `Schema` check at `nerdmd.go:233-235` — `TestParse_SchemaVersionIsPinned` missing case at `nerdmd_test.go:55-68`.
  * Pinned-version check at `nerdmd.go:236-245`.
  * `Forbid` loop at `nerdmd.go:247-260` — empty `Match` rejected at `nerdmd.go:248-250` (`nerdmd_test.go:63-68`), empty `Reason` rejected at `nerdmd.go:251-258` (`nerdmd_test.go:60-68`).
  * `Conventions` loop at `nerdmd.go:262-269` — empty `ID`/`Rule` rejected.
  * `Require` loop at `nerdmd.go:270-272` — empty string rejected.

### 2.7 Path protection: `ForbidsPath`

* `func (d *Document) ForbidsPath(target string) (string,bool)` at `internal/projectdoc/nerdmd.go:281-293`:
  * Nil / empty guard at `nerdmd.go:282-284`.
  * Normalisation `strings.ToLower(filepath.ToSlash(target))` at `nerdmd.go:285` — case-insensitive, slash-normalised per comment at `nerdmd.go:280-281`.
  * Substring match `strings.Contains(normalized, strings.ToLower(filepath.ToSlash(rule.Match)))` at `nerdmd.go:286-288` — verified by `nerdmd_test.go:99-135` (exact, absolute, windows-separator, different-case cases) and `nerdmd_test.go:139-158` (separator/case bypass resistance).
  * Returns `rule.Reason` at `nerdmd.go:287` — a denial must carry its reason per `nerdmd_test.go:128-131`.

### 2.8 Helper

* `func truncate(s string,n int) string` at `internal/projectdoc/nerdmd.go:295-301` — used only for error-message truncation.

## 3. `internal/projectdoc/facts.go` — fact projection and prompt rendering

### 3.1 Predicate constants

All predicates declared at `internal/projectdoc/facts.go:8-40` with line-mapped definitions:

* `PredPresent = "project_doc"` at `facts.go:14` — `project_doc(Path, Schema)` (`facts.go:15-16`).
* `PredName = "project_name"` at `facts.go:18` — `project_name(Name)` (`facts.go:19`).
* `PredLanguage = "project_language"` at `facts.go:22` — `project_language(Lang)` atom (`facts.go:23-24`).
* `PredCommand = "project_command"` at `facts.go:27` — `project_command(Kind, Command)` where `Kind ∈ {/build,/test,/lint,/run}` (`facts.go:28-29`).
* `PredCommandEnv = "project_command_env"` at `facts.go:32` — `project_command_env(Name, Value)` (`facts.go:33-34`).
* `PredForbiddenPath = "project_forbidden_path"` at `facts.go:37` — ENFORCED (`facts.go:37`), `project_forbidden_path(Match, Reason)` (`facts.go:37`).
* `PredRequirement = "project_requirement"` at `facts.go:40` — `project_requirement(Text)` (`facts.go:40`).
* `PredConvention = "project_convention"` at `facts.go:42` — `project_convention(ID, Rule)` (`facts.go:42`).

Comment at `facts.go:1-4` notes declaration site `internal/core/defaults/schemas_projectdoc.mg` and policy site `internal/core/defaults/policy/projectdoc.mg`.

### 3.2 `Facts()`

* `func (d *Document) Facts() []types.Fact` at `internal/projectdoc/facts.go:44-95`:
  * Nil guard `if d==nil {return nil}` at `facts.go:45-47` (verified by `nerdmd_test.go:193-205`).
  * Always emits `PredPresent` with `d.Path, d.Spec.Schema` at `facts.go:49-51` — verified by `nerdmd_test.go:160-168` (`byPred[PredPresent]` must be non-empty).
  * `PredName` when `TrimSpace(d.Spec.Project)!=""`at `facts.go:53-55`.
  * `PredLanguage` via `normalizeAtom(d.Spec.Language)` → `types.MangleAtom(lang)` at `facts.go:57-62` — atom not string, with disjointness comment at `facts.go:58-60`. Tested at `nerdmd_test.go:177-190`: arg must be `types.MangleAtom("/go")`, lowercased, slash-prefixed.
  * `PredCommand` loop over `map[string]string{"/build":..., "/test":..., "/lint":..., "/run":...}` at `facts.go:63-76`, skipping empty `TrimSpace(command)` at `facts.go:71-73`.
  * `PredCommandEnv` loop over `d.Spec.Commands.Env` at `facts.go:78-84`, skipping empty name at `facts.go:79-81`.
  * `PredForbiddenPath` loop over `d.Spec.Forbid` at `facts.go:86-91`.
  * `PredRequirement` loop over `d.Spec.Require` at `facts.go:93-95` (range `93-95`).
  * `PredConvention` loop over `d.Spec.Conventions` at `facts.go:97-99`.
  * Free-text `Body` is never projected — explicit prohibition at `facts.go:47-52`: "Only the frontmatter is projected … asserting free text as a fact would invite policy to pattern-match natural language."
  * Every emitted fact must survive `ToAtom()` at `facts.go:192-204` (`TestFacts_AllConvertToAtoms` at `nerdmd_test.go:192-204`) — otherwise silently evicted at kernel boundary.

### 3.3 `CommandCount` and `normalizeAtom`

* `func (d *Document) CommandCount() int` at `internal/projectdoc/facts.go:101-113` — nil→0 at `facts.go:102-104`, counts non-empty `TrimSpace` among `Build/Test/Lint/Run` at `facts.go:106-111`.
* `func normalizeAtom(raw string) string` at `internal/projectdoc/facts.go:115-138`:
  * `strings.ToLower(TrimSpace(TrimPrefix(TrimSpace(raw),"/")))` at `facts.go:116`.
  * Drop non-`[a-z0-9_]` chars; `'-'/' '/' '.'` → `'_'` at `facts.go:121-128`.
  * Returns `""` when only `/` survives (`facts.go:129-132`) so caller emits no fact rather than unparseable one (`facts.go:118-120`).

### 3.4 `PromptSection()`

* `func (d *Document) PromptSection() string` at `internal/projectdoc/facts.go:150-248`:
  * Nil→`""` at `facts.go:151-153` (`nerdmd_test.go:193-205`).
  * Header `## Project Instructions (<Path>)` at `facts.go:155-158`.
  * Project name stanza `**Project**: <name>` at `facts.go:160-165`.
  * Canonical commands section `### Canonical commands` at `facts.go:167-189` — phrase `"Use these exactly. Do not infer a build or test command."` at `facts.go:169`. Iterates `[build,test,lint,run]` pairs at `facts.go:170-180`, then required env at `facts.go:181-189`.
  * Write-protected paths `### Write-protected paths (ENFORCED)` at `facts.go:191-206` — enforcement disclaimer `"These are denied by the kernel before the tool runs, not by your judgement."` at `facts.go:192-194`, plus `"Attempting one costs a turn and changes nothing."` Format `"- any path containing `<Match>` — <Reason>"` at `facts.go:197-202`.
  * Required steps `### Required steps` at `facts.go:208-217`.
  * Conventions `### Conventions` at `facts.go:219-230` — format `"- **<ID>**: <Rule>"` at `facts.go:221-226`.
  * Verbatim body `TrimSpace(d.Body)` appended at `facts.go:232-236`.
  * Advisory+enforced split rationale at `facts.go:240-244`: frontmatter restated in prose so model is not surprised by kernel denial.
  * Content verified by `nerdmd_test.go:207-230` (`TestPromptSection_StatesThatProtectionIsEnforced` checks for build command, `CGO_CFLAGS`, `.nerd/config.json`, `ENFORCED`, `conventional-commits`, and prose body).

## 4. `internal/core/defaults` — schemas and policy

> **Uncertainty disclosure:** `internal/core/defaults` Mangle sources were not directly read this turn due to budget exhaustion. Claims in §4.1–4.3 are `UNVERIFIED (inferred)` from `Docs/architecture/projectdoc/01-VISION.md` (which itself marks JIT injection `ASSUMPTION`), the confirmed existence of `schemas_projectdoc.mg` + `policy/projectdoc.mg` in the directory listing, and the predicate declarations in `internal/projectdoc/facts.go:1-4`. Treat file:line citations in this section as asserted and requiring re-verification before relying on exact line numbers.

### 4.1 Declared predicates (schemas)

* `internal/core/defaults/schemas_projectdoc.mg:18-71` `UNVERIFIED` — declares `Decl` for `project_doc/2`, `project_name/1`, `project_language/1`, `project_command/2`, `project_command_env/2`, `project_forbidden_path/2`, `project_requirement/1`, `project_convention/2` (and derived `has_project_doc/0` per `01-VISION.md §2`). `has_project_doc` derivation separates "doc present" from "no doc" without requiring callers to inspect `project_doc`.
* `internal/core/defaults/schemas_project.mg` `UNVERIFIED (existence confirmed via listing — `schemas_project.mg` present)` — co-declares `project_language/1` used as JIT `/lang` dimension alongside `schemas_projectdoc.mg`. Referenced in `01-VISION.md §2` arch diagram.
* `internal/core/defaults/schemas_projectdoc.mg:40-44` `UNVERIFIED` — forbid match typed as substring on slash-normalised, lowercased paths, intentionally paralleling `ForbidsPath` (`nerdmd.go:281-293`) so Go and Mangle agree. Glob semantics explicitly rejected (see `01-VISION.md §3`).
* Supporting corpora: `internal/core/defaults/intent_corpus.go`, `predicate_corpus.go`, `prompt_corpus.go` (existence verified via listing; schema overlap `UNVERIFIED`).

### 4.2 Policy derivations

* `internal/core/defaults/policy/projectdoc.mg:32-34` `UNVERIFIED` — derives `project_write_denied` / `coder_block_write` (naming per `01-VISION.md §2, §4`). Currently dormant (exists but not consumed by executor per `01-VISION.md §4`).
* `internal/core/defaults/policy/projectdoc.mg` (full file) `UNVERIFIED` — derives `has_project_doc` (from `project_doc`), `project_has_command(Kind)` (from `project_command`), and the write-denial predicate consumed by `permitted(Action,Target,Payload)` gate. Bounded derivation required per `01-VISION.md §5`.
* `require`/`conventions` enforcement `planned:` per `01-VISION.md §4` — intended derivations `handoff_blocked(Requirement)` from `project_requirement/1` and lint signals from `project_convention/2`, gated by `permitted(handoff,…)` derivation (open choice per `01-VISION.md §6`).

### 4.3 Other defaults in scope

* Existence verified, content `UNVERIFIED`: `build_topology.mg`, `benchmarks.mg`, `campaign_rules.mg`, `chaos.mg`, `doc_taxonomy.mg`, `go_safety.mg`, `inference.mg`, `jit_compiler.mg`, `reviewer.mg`, `selection_policy.mg`, `taxonomy.mg`, `tester.mg`, `topology_planner.mg`, plus `schemas_*.mg` family (19 files) and `policy/` subdirectory. These were not re-read; they are listed to bound the corpus scope, not to assert projectdoc semantics.

## 5. `internal/session` — execution and the write gate

> **Uncertainty disclosure:** `internal/session` implementation files were not directly read this turn. Claims in §5 are `UNVERIFIED (inferred)` from the `internal/session/` directory listing, `internal/projectdoc/facts.go`/`nerdmd.go` call sites described in `01-VISION.md`, and the `executor_tools.go` API described there. File:line citations are asserted from prior synthesis and require re-verification.

### 5.1 Executor

* `internal/session/executor.go` (existence verified) — orchestrates tool lifecycle; calls the projectdoc gate before mutating tools.
* `internal/session/executor_tools.go:406-445` `UNVERIFIED` — implements `projectForbidsWrite(target string) (reason string, forbidden bool)` (name per `01-VISION.md §2` diagram: `executor projectForbidsWrite`). Wraps `Document.ForbidsPath` (`nerdmd.go:281-293`) for live write-mutation tools (`write_file`, `edit_lines`, `insert_lines`, `delete_lines`).
* `internal/session/executor_tools.go:529` `UNVERIFIED` — denial message string `blocked by nerd.md: <path> is write-protected (<reason>)`, emitted when `ForbidsPath` returns true. Costs the turn without mutating the filesystem (`01-VISION.md §1`).
* `internal/session/executor_tools.go:406-445` gate is called from `permitted(Action,Target,Payload)`-equivalent check before tool execution — `01-VISION.md §2` marks the intended state as consuming derived `project_write_denied` rather than duplicating `Contains` logic (open proof obligation per `§6`).

### 5.2 Boot and fact injection

* `internal/session/spawner.go` / `internal/session/task_executor.go` / `internal/session/subagent.go` (existence verified) `UNVERIFIED` content — at boot, `projectdoc.Load(workspace)` (`nerdmd.go:142-162`) result is (a) projected via `Document.Facts()` (`facts.go:44-99`) into the kernel overlay and (b) rendered via `Document.PromptSection()` (`facts.go:150-248`) into a single JIT prompt atom.
* `internal/session/semantic_compressor.go` (existence verified) `UNVERIFIED` — may compress or route the JIT atom; exact injection point in `internal/prompt/compiler.go` is `ASSUMPTION` per `01-VISION.md` header.

### 5.3 Other session components

* `internal/session/executor_test.go`, `executor_boundary_test.go`, `executor_budget_exhaustion_test.go`, `executor_capability_test.go`, `executor_mangle_test.go`, `executor_planner_routing_test.go`, `executor_process_test.go`, `executor_projectdoc_test.go` — existence verified via listing; `executor_projectdoc_test.go` is expected to exercise `projectForbidsWrite` but was not read this turn (`UNVERIFIED`).
* `internal/session/spawner_config_test.go`, `spawner_gaps_test.go`, etc., plus `subagent_test.go`, `task_executor_test.go`, `write_mutation_tool_test.go` — existence verified, content `UNVERIFIED`.

## 6. End-to-end data flow (with provenance)

```
workspace root: nerd.md  (or .nerd/nerd.md)
        │  Find at nerdmd.go:131-140  (verified)
        ├─ Load  at nerdmd.go:142-162  ──► ReadFile + Parse + Rel/ToSlash  (verified)
        │         └─ splitFrontmatter at nerdmd.go:187-225  (verified, 4 MiB buf at 188-189)
        │              └─ yaml KnownFields(true) at nerdmd.go:171  (verified) ──► validate at 232-272 (verified)
        │                       └─ Document{Path,Spec,Body} at nerdmd.go:44-57 / 180-181 (verified)
        │
        ├─ Facts() at facts.go:44-99 (verified) ──► project_{doc,name,language,command,command_env,forbidden_path,requirement,convention}
        │        only frontmatter; Body never projected per facts.go:47-52 (verified)
        │        Decl in schemas_projectdoc.mg:18-71 UNVERIFIED ──► kernel overlay
        │
        ├─ PromptSection() at facts.go:150-248 (verified) ──► single JIT atom (verified per facts.go:1-4 comment)
        │        injected via internal/prompt/compiler.go ASSUMPTION (per 01-VISION.md header)
        │
        └─ Policy: policy/projectdoc.mg:32-34 derives project_write_denied / coder_block_write UNVERIFIED
                   ──► permitted(Action,Target,Payload) gate UNVERIFIED
                        ──► executor_tools.go:406-445 projectForbidsWrite UNVERIFIED
                             (intended to consume derived denial, not duplicate ForbidsPath Contains logic)
```

Executive vs creative split load-bearing (verified):

| Half | Form | Projection | Enforcement | Source |
|------|------|------------|-------------|--------|
| YAML frontmatter `---` fenced, `schema: nerd/v1` pinned | Strict, `KnownFields(true)` | Kernel facts `project_*` | Mangle `permitted` denial before tool runs | `nerdmd.go:36,43,171,238-245` + `facts.go:44-99` (verified) |
| Markdown body `TrimSpace` verbatim | Free prose | `PromptSection` JIT atom only | None — never facts | `facts.go:47-52,150-248` (verified) |

## 7. Enforcement and safety properties

* **Substring, not glob** — `ForbidRule.Match` comment at `nerdmd.go:109-115` and `ForbidsPath` `Contains` at `nerdmd.go:286-287` intentionally avoid glob so Go and Mangle (`schemas_projectdoc.mg:40-44` `UNVERIFIED`) agree. A disagreeing glob is a gate that sometimes opens (`01-VISION.md §3`, `nerdmd.go:109-115`).
* **Case and separator normalisation** — `ToLower(ToSlash(...))` at `nerdmd.go:285-287` ensures `Secrets/Prod` matches `secrets/prod`, `SECRETS\PROD`, `./app/Secrets/Prod` (`nerdmd_test.go:139-158` verified). Mangle-side equivalence needs a normalisation proof artifact (`planned:` per `01-VISION.md §4`).
* **Reason-required denials** — `Spec.validate()` rejects empty `Reason` at `nerdmd.go:251-258`; `ForbidsPath` returns reason at `nerdmd.go:287`; `PromptSection` prints it at `facts.go:197-202`; `executor_tools.go:529` `UNVERIFIED` surfaces `blocked by nerd.md: <path> is write-protected (<reason>)`. An unexplained denial reads as malfunction and invites workaround (`nerdmd.go:251-253` comment).
* **Schema pinning** — `nerd/v1` pinned at `nerdmd.go:36-43`; unsupported schema error at `nerdmd.go:240-245` cites both document and binary versions (`nerdmd_test.go:55-68`).
* **Tool coverage** — `01-VISION.md §1` asserts gate applies to `write_file`, `edit_lines`, `insert_lines`, `delete_lines`; `executor_tools.go:406-445` `UNVERIFIED` is the enforcement site.

## 8. Non-goals (preserved from vision)

* Do not project Markdown body as facts — `facts.go:47-52` forbids pattern-matching prose in policy (verified).
* Do not adopt glob — `nerdmd.go:109-115` + `schemas_projectdoc.mg:40-44` `UNVERIFIED` (verified Go side).
* Do not range-check schema — `nerdmd.go:38-43` pinned (verified).
* Do not add parallel subsystem for `require`/`conventions` — reuse `project_requirement/1` + `project_convention/2` facts (`facts.go:40,42` verified) plus Mangle derivations (policy `UNVERIFIED`).

## 9. Verification

Commands that must stay green per `01-VISION.md §5`:

* `go test ./internal/projectdoc -run TestParse -count=1 -v` — exercises `nerdmd_test.go:24-97`.
* `go vet ./internal/projectdoc` — static check.
* `go test ./internal/projectdoc -run TestFacts -count=1 -v` — exercises `facts.go:44-99` via `nerdmd_test.go:160-204` (including `ToAtom` conversion at `nerdmd_test.go:192-204`).
* `go test ./internal/projectdoc -run TestPromptSection -count=1 -v` — `nerdmd_test.go:207-230`.
* `go test ./internal/projectdoc -run TestForbidsPath -count=1 -v` — `nerdmd_test.go:99-158`.

`internal/session` and `internal/core/defaults` tests not re-run this turn due to budget (`UNVERIFIED`); expected suites include `executor_projectdoc_test.go` and `schemas_projectdoc` `Decl` validation.

## 10. Open questions and risks (carried from 01-VISION.md §6-7)

* `require` enforcement as `permitted(handoff,…)` vs post-tool advisory check — open (see `OPEN-QUESTIONS.md` per `01-VISION.md §6`).
* `project_write_denied` consumption requires Go/Mangle normalisation proof for `ToLower(ToSlash(...))` + `Contains` equivalence — open.
* `commands.env` hermetic execution (materialise `project_command_env/2` for canonical `build`/`test`) — `PROPOSED UPLIFT`, opt-in vs global not decided.
* `PromptSection()` injection point in `internal/prompt/compiler.go` — `ASSUMPTION` per `01-VISION.md` header; needs re-read to confirm.
* Risks if vision drifts: body pattern-matched as facts → natural-language policy matching (`facts.go:47-52` forbids); glob divergence → gate opens; range-checked schema → half-applied document.

## 11. Source index (every claim traces here)

* `internal/projectdoc/nerdmd.go` — directly read; line ranges cited per §2 (Find `131-140`, Load `142-162`, Parse `164-182`, splitFrontmatter `187-225`, validate `232-272`, ForbidsPath `281-293`).
* `internal/projectdoc/facts.go` — directly read; predicate block `8-42`, `Facts()` `44-99`, `CommandCount` `101-113`, `normalizeAtom` `115-138`, `PromptSection` `150-248`.
* `internal/projectdoc/nerdmd_test.go` — directly read; fixtures and coverage cited per §2-3.
* `internal/core/defaults/` — directory listing verified; `schemas_projectdoc.mg` + `policy/projectdoc.mg` existence confirmed, content `UNVERIFIED` this turn (asserted lines `18-71`, `32-34` require re-read).
* `internal/session/` — directory listing verified; `executor_tools.go:406-445,529` `UNVERIFIED` this turn (asserted from prior synthesis).
* `Docs/architecture/projectdoc/01-VISION.md` — corroboration only; its own header flags JIT/predicate paths as `ASSUMPTION` pending verification.
