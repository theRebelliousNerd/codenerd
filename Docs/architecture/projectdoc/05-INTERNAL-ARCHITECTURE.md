# 05 — Internal Architecture: `nerd.md` Subsystem

> **Scope:** Internal architecture of the `nerd.md` subsystem. Every claim cites `file:line` using exact line numbers as prefixed by `read_file` (line number + tab). Sources directly re-read for this revision: `internal/projectdoc/nerdmd.go`, `internal/projectdoc/facts.go`, `internal/session/executor_tools.go`.

## 1. Package thesis and constants

* Package comment at `internal/projectdoc/nerdmd.go:1-21` defines `nerd.md` as the per-project instruction file and states the split: YAML frontmatter is a **strict schema** that becomes kernel facts (executive), Markdown body is **prose** that becomes an advisory prompt atom. Explicit comparison to `CLAUDE.md` / `AGENTS.md` at `internal/projectdoc/nerdmd.go:3-11`.
* Strictness rationale at `internal/projectdoc/nerdmd.go:18-21`: an unknown key, bad schema version, or malformed entry is a hard parse error naming the line, not a silently ignored field — a dropped directive is worse than no directive.
* Canonical filename at `internal/projectdoc/nerdmd.go:36`: `const FileName = "nerd.md"`.
* Schema version pin at `internal/projectdoc/nerdmd.go:43`: `const SchemaVersion = "nerd/v1"`. Pinned rather than range-checked per comment at `internal/projectdoc/nerdmd.go:38-42`: a document written for a newer binary must fail loudly with a message naming the expected vs. actual version.

## 2. Domain model: `Document`, `Spec`, and related types

### 2.1 `Document`

Defined at `internal/projectdoc/nerdmd.go:46-56`:

* `Path string` at `internal/projectdoc/nerdmd.go:48` — file the document was read from, relative to workspace when known. Normalized at load time via `filepath.Rel` + `filepath.ToSlash` at `internal/projectdoc/nerdmd.go:161-165`.
* `Spec Spec` at `internal/projectdoc/nerdmd.go:51` — the strict machine-readable frontmatter.
* `Body string` at `internal/projectdoc/nerdmd.go:55` — Markdown prose after the frontmatter, verbatim and trimmed (`strings.TrimSpace(body)` at `internal/projectdoc/nerdmd.go:189`).

### 2.2 `Spec`

Defined at `internal/projectdoc/nerdmd.go:58-92`. Every field is optional except `Schema` per comment at `internal/projectdoc/nerdmd.go:60-62`:

* `Schema string` at `internal/projectdoc/nerdmd.go:65` with tag `yaml:"schema"` — required, must equal `SchemaVersion`.
* `Project string` at `internal/projectdoc/nerdmd.go:68` with tag `yaml:"project,omitempty"` — human-readable project name surfaced in prompts.
* `Language string` at `internal/projectdoc/nerdmd.go:73` with tag `yaml:"language,omitempty"` — primary language tag (e.g. `go`, `python`); seeds JIT compilation context `/lang` dimension.
* `Commands Commands` at `internal/projectdoc/nerdmd.go:78` with tag `yaml:"commands,omitempty"` — canonical build/test/lint/run invocations.
* `Forbid []ForbidRule` at `internal/projectdoc/nerdmd.go:83` with tag `yaml:"forbid,omitempty"` — paths the agent must not write to; enforced as `project_forbidden_path` facts (see §4 and §6).
* `Require []string` at `internal/projectdoc/nerdmd.go:88` with tag `yaml:"require,omitempty"` — non-negotiable steps, projected as `project_requirement` facts.
* `Conventions []Convention` at `internal/projectdoc/nerdmd.go:91` with tag `yaml:"conventions,omitempty"` — named, checkable project rules.

### 2.3 `Commands`

Defined at `internal/projectdoc/nerdmd.go:94-105`:

* `Build string` at `internal/projectdoc/nerdmd.go:96` (`yaml:"build,omitempty"`).
* `Test string` at `internal/projectdoc/nerdmd.go:97` (`yaml:"test,omitempty"`).
* `Lint string` at `internal/projectdoc/nerdmd.go:98` (`yaml:"lint,omitempty"`).
* `Run string` at `internal/projectdoc/nerdmd.go:99` (`yaml:"run,omitempty"`).
* `Env map[string]string` at `internal/projectdoc/nerdmd.go:104` (`yaml:"env,omitempty"`). Comment at `internal/projectdoc/nerdmd.go:100-103` explains the need: `CGO_CFLAGS`-style prerequisites are invisible in the command string and their absence produces a confusing failure far from its cause.

### 2.4 `ForbidRule`

Defined at `internal/projectdoc/nerdmd.go:107-121`:

* `Match string` at `internal/projectdoc/nerdmd.go:115` (`yaml:"match"`). Substring semantics by design, explained at `internal/projectdoc/nerdmd.go:108-114`: substring, not glob, so semantics are obvious to the author and identical in Go and Mangle; a glob engine that disagrees across layers would be a safety gate that sometimes opens.
* `Reason string` at `internal/projectdoc/nerdmd.go:120` (`yaml:"reason"`). Required; comment at `internal/projectdoc/nerdmd.go:117-119` says a denial the agent cannot explain looks like a malfunction and invites a workaround.

### 2.5 `Convention`

Defined at `internal/projectdoc/nerdmd.go:123-127`:

* `ID string` at `internal/projectdoc/nerdmd.go:125` (`yaml:"id"`).
* `Rule string` at `internal/projectdoc/nerdmd.go:126` (`yaml:"rule"`).

## 3. Parse pipeline: `Find` → `Load` → `Parse` → `splitFrontmatter` → `validate`

### 3.1 `Find`

`func Find(workspace string) string` at `internal/projectdoc/nerdmd.go:131-141`:

* Searches two candidates in order at `internal/projectdoc/nerdmd.go:132-135`: `filepath.Join(workspace, FileName)` then `filepath.Join(workspace, ".nerd", FileName)` (workspace root preferred).
* Tests each with `os.Stat` + `!info.IsDir()` at `internal/projectdoc/nerdmd.go:136`.
* Returns the first existing file or `""` when absent at `internal/projectdoc/nerdmd.go:140`. Absence is not an error: `nerd.md` is optional.

### 3.2 `Load`

`func Load(workspace string) (*Document, error)` at `internal/projectdoc/nerdmd.go:148-167`:

* Calls `Find` at `internal/projectdoc/nerdmd.go:149`.
* Returns `(nil, nil)` when absent at `internal/projectdoc/nerdmd.go:150-152` — no file is not a failure.
* Reads the file with `os.ReadFile(path)` at `internal/projectdoc/nerdmd.go:153`, wrapping errors with `fmt.Errorf("read %s: %w", path, err)` at `internal/projectdoc/nerdmd.go:155`.
* Delegates to `Parse` at `internal/projectdoc/nerdmd.go:157`, wrapping parse errors with `fmt.Errorf("%s: %w", path, err)` at `internal/projectdoc/nerdmd.go:159`.
* Normalizes `doc.Path` to slash form relative to workspace at `internal/projectdoc/nerdmd.go:161-165`: `filepath.Rel(workspace, path)` then `filepath.ToSlash`, falling back to `filepath.ToSlash(path)` on `Rel` error.

Comment at `internal/projectdoc/nerdmd.go:143-147` states the invariant: an unreadable directive must never degrade to "no directive" — only absence is `(nil, nil)`.

### 3.3 `Parse`

`func Parse(data []byte) (*Document, error)` at `internal/projectdoc/nerdmd.go:170-190`:

* Calls `splitFrontmatter(data)` at `internal/projectdoc/nerdmd.go:171` to separate `front` and `body`.
* Creates a YAML decoder with `yaml.NewDecoder(bytes.NewReader(front))` at `internal/projectdoc/nerdmd.go:177`.
* Enables strict known-fields checking with `decoder.KnownFields(true)` at `internal/projectdoc/nerdmd.go:180`. Comment at `internal/projectdoc/nerdmd.go:178-179` states the rationale: an unknown key means the author wrote a directive this binary will not honour; failing here is the only way they find out.
* Decodes into `Spec` at `internal/projectdoc/nerdmd.go:181`, wrapping errors as `fmt.Errorf("invalid frontmatter: %w", err)` at `internal/projectdoc/nerdmd.go:182`.
* Calls `spec.validate()` at `internal/projectdoc/nerdmd.go:185`, returning its error directly at `internal/projectdoc/nerdmd.go:186`.
* Returns `&Document{Spec: spec, Body: strings.TrimSpace(body)}` at `internal/projectdoc/nerdmd.go:189`.

### 3.4 `splitFrontmatter`

`const frontmatterFence = "---"` at `internal/projectdoc/nerdmd.go:192`.

`func splitFrontmatter(data []byte) (front []byte, body string, err error)` at `internal/projectdoc/nerdmd.go:195-234`:

* Scanner with enlarged buffer at `internal/projectdoc/nerdmd.go:200`: `scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)`. Rationale at `internal/projectdoc/nerdmd.go:197-199`: bodies routinely embed long lines (tables, command strings) and the default 64 KiB would truncate mid-document.
* Empty-file guard at `internal/projectdoc/nerdmd.go:202-204`: `if !scanner.Scan() { return nil, "", fmt.Errorf("file is empty; expected a %q frontmatter block", frontmatterFence) }`.
* First-line fence check at `internal/projectdoc/nerdmd.go:205-210`: must be `---` after `TrimSpace`; error message includes `truncate(scanner.Text(), 40)` at `internal/projectdoc/nerdmd.go:209` and explicitly contrasts `nerd.md` (requires frontmatter) with `CLAUDE.md`.
* Front accumulation loop at `internal/projectdoc/nerdmd.go:212-220`: collects lines until a closing `---` sets `closed = true` at `internal/projectdoc/nerdmd.go:216`.
* Unclosed-fence error at `internal/projectdoc/nerdmd.go:221-223`: `fmt.Errorf("frontmatter block opened with %q was never closed", frontmatterFence)`.
* Body accumulation at `internal/projectdoc/nerdmd.go:225-228` and scanner error propagation at `internal/projectdoc/nerdmd.go:229-231`: `fmt.Errorf("read: %w", scanErr)`.
* Returns `[]byte(strings.Join(frontLines, "\n")), strings.Join(bodyLines, "\n")` at `internal/projectdoc/nerdmd.go:233`.

Helper `func truncate(s string, n int) string` at `internal/projectdoc/nerdmd.go:294-299` is used only for error-message truncation at `internal/projectdoc/nerdmd.go:209`.

### 3.5 `validate`

`func (s *Spec) validate() error` at `internal/projectdoc/nerdmd.go:236-274`:

* Missing `Schema` at `internal/projectdoc/nerdmd.go:237-239`: `fmt.Errorf("frontmatter is missing the required %q key; expected %q", "schema", SchemaVersion)`.
* Pinned-version mismatch at `internal/projectdoc/nerdmd.go:240-245`: `if s.Schema != SchemaVersion` emits `unsupported schema` with both the got and expected values.
* `Forbid` loop at `internal/projectdoc/nerdmd.go:247-256`:
  * Empty `Match` rejected at `internal/projectdoc/nerdmd.go:248-249` with `forbid[%d] has an empty "match"; a rule that matches every path would deny every write`.
  * Empty `Reason` rejected at `internal/projectdoc/nerdmd.go:251-255` with `forbid[%d] (match %q) has no "reason"`.
* `Conventions` loop at `internal/projectdoc/nerdmd.go:258-265`:
  * Empty `ID` at `internal/projectdoc/nerdmd.go:259-260`.
  * Empty `Rule` at `internal/projectdoc/nerdmd.go:261-263`.
* `Require` loop at `internal/projectdoc/nerdmd.go:267-271`: empty string rejected at `internal/projectdoc/nerdmd.go:268-269` with `require[%d] is empty`.

### 3.6 `ForbidsPath` (Go-side substring matcher)

`func (d *Document) ForbidsPath(target string) (reason string, forbidden bool)` at `internal/projectdoc/nerdmd.go:281-292`:

* Nil / empty guard at `internal/projectdoc/nerdmd.go:282-284`: returns `("", false)` when `d == nil` or `TrimSpace(target) == ""`.
* Normalization at `internal/projectdoc/nerdmd.go:285`: `strings.ToLower(filepath.ToSlash(target))` — slash-normalized and case-insensitive per comment at `internal/projectdoc/nerdmd.go:279-280`.
* Substring match at `internal/projectdoc/nerdmd.go:286-289`: `strings.Contains(normalized, strings.ToLower(filepath.ToSlash(rule.Match)))`.
* Returns `rule.Reason, true` at `internal/projectdoc/nerdmd.go:288` on first match.

## 4. Fact projection: `Facts()`

### 4.1 Predicate constants

All predicates declared at `internal/projectdoc/facts.go:12-40` (with declaration sites noted at `internal/projectdoc/facts.go:9-11` as `internal/core/defaults/schemas_projectdoc.mg` and policy at `internal/core/defaults/policy/projectdoc.mg`):

* `PredPresent = "project_doc"` at `internal/projectdoc/facts.go:15` — `project_doc(Path, Schema)` per comment at `internal/projectdoc/facts.go:13-14`.
* `PredName = "project_name"` at `internal/projectdoc/facts.go:18` — `project_name(Name)` at `internal/projectdoc/facts.go:17-18`.
* `PredLanguage = "project_language"` at `internal/projectdoc/facts.go:22` — `project_language(Lang)` as a Mangle atom at `internal/projectdoc/facts.go:20-22`.
* `PredCommand = "project_command"` at `internal/projectdoc/facts.go:26` — `project_command(Kind, Command)` where `Kind` is `/build`, `/test`, `/lint`, `/run` at `internal/projectdoc/facts.go:24-26`.
* `PredCommandEnv = "project_command_env"` at `internal/projectdoc/facts.go:30` — `project_command_env(Name, Value)` at `internal/projectdoc/facts.go:28-30`.
* `PredForbiddenPath = "project_forbidden_path"` at `internal/projectdoc/facts.go:33` — `project_forbidden_path(Match, Reason)` at `internal/projectdoc/facts.go:32-33`, marked `ENFORCED` at `internal/projectdoc/facts.go:32`.
* `PredRequirement = "project_requirement"` at `internal/projectdoc/facts.go:36` — `project_requirement(Text)` at `internal/projectdoc/facts.go:35-36`.
* `PredConvention = "project_convention"` at `internal/projectdoc/facts.go:39` — `project_convention(ID, Rule)` at `internal/projectdoc/facts.go:38-39`.

### 4.2 `Facts()` implementation

`func (d *Document) Facts() []types.Fact` at `internal/projectdoc/facts.go:51-109`:

* Nil guard at `internal/projectdoc/facts.go:52-54`: `if d == nil { return nil }` so callers can pass the result of `Load` straight through without a nil check (comment at `internal/projectdoc/facts.go:49-50`).
* Always emits `PredPresent` at `internal/projectdoc/facts.go:56-58`: `{Predicate: PredPresent, Args: []any{d.Path, d.Spec.Schema}}`.
* `PredName` when `TrimSpace(d.Spec.Project) != ""` at `internal/projectdoc/facts.go:60-62`.
* `PredLanguage` via `normalizeAtom` at `internal/projectdoc/facts.go:64-69`: `lang := normalizeAtom(d.Spec.Language)` then `types.MangleAtom(lang)` at `internal/projectdoc/facts.go:68`. Comment at `internal/projectdoc/facts.go:65-67` explains it must be a Mangle name constant (atom), not a quoted string, because `current_context(/lang, /go)` and language-gated atoms are disjoint types in Mangle.
* `PredCommand` loop at `internal/projectdoc/facts.go:71-84`: iterates `map[string]string{"/build": ..., "/test": ..., "/lint": ..., "/run": ...}` at `internal/projectdoc/facts.go:71-76`, skipping empty `TrimSpace(command)` at `internal/projectdoc/facts.go:77-79`, emitting `{Predicate: PredCommand, Args: []any{types.MangleAtom(kind), command}}` at `internal/projectdoc/facts.go:80-83`.
* `PredCommandEnv` loop at `internal/projectdoc/facts.go:86-91`: iterates `d.Spec.Commands.Env` at `internal/projectdoc/facts.go:86`, skipping empty `TrimSpace(name)` at `internal/projectdoc/facts.go:87-89`, emitting `PredCommandEnv` with `name, value` at `internal/projectdoc/facts.go:90`.
* `PredForbiddenPath` loop at `internal/projectdoc/facts.go:93-98`: iterates `d.Spec.Forbid` at `internal/projectdoc/facts.go:93`, emitting each `rule.Match, rule.Reason` at `internal/projectdoc/facts.go:94-97`.
* `PredRequirement` loop at `internal/projectdoc/facts.go:100-102`: iterates `d.Spec.Require` at `internal/projectdoc/facts.go:100`, emitting each `req` at `internal/projectdoc/facts.go:101`.
* `PredConvention` loop at `internal/projectdoc/facts.go:104-106`: iterates `d.Spec.Conventions` at `internal/projectdoc/facts.go:104`, emitting `c.ID, c.Rule` at `internal/projectdoc/facts.go:105`.
* Body exclusion: comment at `internal/projectdoc/facts.go:42-47` states only frontmatter is projected; body is prose and belongs in the prompt, not the fact store, to avoid policy pattern-matching natural language.

Returns the accumulated slice at `internal/projectdoc/facts.go:108`.

### 4.3 `CommandCount`

`func (d *Document) CommandCount() int` at `internal/projectdoc/facts.go:112-123`:

* Nil → 0 at `internal/projectdoc/facts.go:113-115`.
* Counts non-empty `TrimSpace` among `Build/Test/Lint/Run` at `internal/projectdoc/facts.go:116-121`.

### 4.4 `normalizeAtom`

`func normalizeAtom(raw string) string` at `internal/projectdoc/facts.go:129-148`:

* Trims, strips leading `/`, lowercases at `internal/projectdoc/facts.go:130`: `strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "/")))`.
* Returns `""` on empty at `internal/projectdoc/facts.go:131-133`.
* Writes leading `/` at `internal/projectdoc/facts.go:135`, then maps runes at `internal/projectdoc/facts.go:136-142`: `[a-z0-9_]` kept, `-`, space, `.` become `_`, others dropped.
* Returns `""` when only `/` survives at `internal/projectdoc/facts.go:144-146` (so caller emits no fact rather than an unparseable one per comment at `internal/projectdoc/facts.go:127-128`).
* Returns `b.String()` at `internal/projectdoc/facts.go:147`.

## 5. Prompt rendering: `PromptSection()`

`func (d *Document) PromptSection() string` at `internal/projectdoc/facts.go:156-242`:

* Nil → `""` at `internal/projectdoc/facts.go:157-159`.
* Header at `internal/projectdoc/facts.go:161-164`: `## Project Instructions (<Path>)` (path from `d.Path`).
* Project name stanza at `internal/projectdoc/facts.go:166-170`: `**Project**: <name>` when `TrimSpace(d.Spec.Project) != ""`.
* Canonical commands section at `internal/projectdoc/facts.go:172-198`:
  * Guarded by `c.Build != "" || c.Test != "" || c.Lint != "" || c.Run != ""` at `internal/projectdoc/facts.go:172`.
  * Header `### Canonical commands` at `internal/projectdoc/facts.go:173`.
  * Instruction `Use these exactly. Do not infer a build or test command.` at `internal/projectdoc/facts.go:174`.
  * Iterates `[build,test,lint,run]` pairs at `internal/projectdoc/facts.go:175-176`, skipping empty `TrimSpace(pair[1])` at `internal/projectdoc/facts.go:178-180`, writing `` - `kind`: `command` `` at `internal/projectdoc/facts.go:181-185`.
  * Required environment block at `internal/projectdoc/facts.go:187-196`: `Required environment for those commands:` then each `- `name`=`value`` at `internal/projectdoc/facts.go:189-194`.
* Write-protected paths at `internal/projectdoc/facts.go:200-212`:
  * Guarded by `len(d.Spec.Forbid) > 0` at `internal/projectdoc/facts.go:200`.
  * Header `### Write-protected paths (ENFORCED)` at `internal/projectdoc/facts.go:201`.
  * Disclaimer at `internal/projectdoc/facts.go:202-203`: `These are denied by the kernel before the tool runs, not by your judgement. Attempting one costs a turn and changes nothing.`
  * Each rule rendered as `- any path containing `<Match>` — <Reason>` at `internal/projectdoc/facts.go:204-209`.
* Required steps at `internal/projectdoc/facts.go:214-222`: header `### Required steps` at `internal/projectdoc/facts.go:215`, each `- <req>` at `internal/projectdoc/facts.go:216-219`.
* Conventions at `internal/projectdoc/facts.go:224-234`: header `### Conventions` at `internal/projectdoc/facts.go:225`, each `- **<ID>**: <Rule>` at `internal/projectdoc/facts.go:226-231`.
* Verbatim body at `internal/projectdoc/facts.go:236-239`: `TrimSpace(d.Body)` appended with trailing newline when non-empty.
* Returns `b.String()` at `internal/projectdoc/facts.go:241`.

Rationale comment at `internal/projectdoc/facts.go:150-155`: frontmatter is restated in prose alongside the body because the model cannot read the fact store directly; enforcement remains the kernel's, but the model should not be surprised by a denial mid-edit.

## 6. Executor gate: how the kernel facts are consumed

### 6.1 Storing the document vs. querying the kernel

* `func (e *Executor) SetProjectDoc(doc *projectdoc.Document)` at `internal/session/executor_tools.go:433-437`: locks `e.mu` at `internal/session/executor_tools.go:434`, sets `e.projectDoc = doc` at `internal/session/executor_tools.go:436`. Comment at `internal/session/executor_tools.go:426-432` states only the prose rendering is held here; write protection is enforced by querying the kernel so a subagent that never receives this pointer is still governed.
* `func (e *Executor) withProjectInstructions(systemPrompt string) string` at `internal/session/executor_tools.go:445-456`: `RLock`s and reads `e.projectDoc` at `internal/session/executor_tools.go:446-448`, calls `doc.PromptSection()` at `internal/session/executor_tools.go:450`, returns early when `section == ""` at `internal/session/executor_tools.go:451-453`, otherwise logs injection at `internal/session/executor_tools.go:454` and returns `systemPrompt + "\n\n" + section` at `internal/session/executor_tools.go:455`. Comment at `internal/session/executor_tools.go:439-444` notes this restates frontmatter in prose so a protected path is known before the denied turn.

### 6.2 Write-mutation recognition

`func isWriteMutationTool(name string) bool` at `internal/session/executor_tools.go:411-424`:

* Case-insensitive, trimmed switch at `internal/session/executor_tools.go:412`: `strings.ToLower(strings.TrimSpace(name))`.
* Registered VirtualStore write actions at `internal/session/executor_tools.go:413-417`: `write_file`, `edit_file`, `delete_file`, `edit_lines`, `insert_lines`, `delete_lines`, `edit_element`, `fs_write`.
* Defensive aliases at `internal/session/executor_tools.go:418-419`: `apply_patch`, `str_replace`, `create_file`, `replace_in_file`, `multi_edit` — harmless to accept, keeps gate closed if one is ever added. Comment at `internal/session/executor_tools.go:391-410` explains the list must cover every durable-write `ActionType` in `internal/core/virtual_store_types.go` and previously drifted from generic LLM tool vocabulary.
* Returns `true` at `internal/session/executor_tools.go:420`, otherwise `false` at `internal/session/executor_tools.go:422`.

This predicate drives two gates simultaneously per comment at `internal/session/executor_tools.go:393-396`: `checkHollowSuccess` reporting and `projectForbidsWrite` denial.

### 6.3 Target path extraction

`var projectDocPathArgs = []string{...}` at `internal/session/executor_tools.go:463` enumerates `path`, `file_path`, `filepath`, `file`, `filename`, `target`, `dest`, `destination`. Comment at `internal/session/executor_tools.go:458-463` explains tools disagree on arg name, so the gate checks all of them.

`func projectDocTargetPath(args map[string]any) string` at `internal/session/executor_tools.go:466-475`: iterates `projectDocPathArgs` at `internal/session/executor_tools.go:467`, returns the first non-empty `string` value at `internal/session/executor_tools.go:469-470`, otherwise `""` at `internal/session/executor_tools.go:474`.

### 6.4 Kernel query gate

`func (e *Executor) projectForbidsWrite(call ToolCall) (string, bool)` at `internal/session/executor_tools.go:487-524`:

* Non-write tools bypass at `internal/session/executor_tools.go:488-490`: `if !isWriteMutationTool(call.Name) { return "", false }`.
* Extracts `target := projectDocTargetPath(call.Args)` at `internal/session/executor_tools.go:491`; no target → `false` at `internal/session/executor_tools.go:492-494`.
* No kernel → `false` at `internal/session/executor_tools.go:495-497`.
* Queries the kernel at `internal/session/executor_tools.go:499`: `facts, err := e.kernel.Query(projectdoc.PredForbiddenPath)` where `PredForbiddenPath` is `"project_forbidden_path"` at `internal/projectdoc/facts.go:33`. Comment at `internal/session/executor_tools.go:477-484` states the kernel is the authority, not a cached Go struct — `nerd.md` facts are asserted at boot like any other EDB.
* Fail-open on query error at `internal/session/executor_tools.go:500-508`: logs a `Warn` at `internal/session/executor_tools.go:505` and returns `("", false)` — a transient kernel hiccup must not block all writes, but the degraded state must be visible.
* Normalizes the target at `internal/session/executor_tools.go:510`: `strings.ToLower(filepath.ToSlash(target))`.
* Iterates facts at `internal/session/executor_tools.go:511-522`: skips facts with `< 2` args at `internal/session/executor_tools.go:512-514`, normalizes `match` via `strings.ToLower(filepath.ToSlash(types.ExtractString(fact.Args[0])))` at `internal/session/executor_tools.go:515`, skips empty `match` at `internal/session/executor_tools.go:516-518`, and checks `strings.Contains(normalized, match)` at `internal/session/executor_tools.go:519`, returning `types.ExtractString(fact.Args[1]), true` at `internal/session/executor_tools.go:520` (the `Reason`) on first match. No match → `("", false)` at `internal/session/executor_tools.go:523`.

Only write-mutation tools are gated; reading a protected file is allowed per comment at `internal/session/executor_tools.go:485-486`.

### 6.5 Enforcement point in `executeToolCall`

`func (e *Executor) executeToolCall(ctx context.Context, call ToolCall, cfg *config.EffectiveAgentRuntimeConfig) (string, error)` at `internal/session/executor_tools.go:669-712` (signature at `internal/session/executor_tools.go:669`):

* JIT allowlist check at `internal/session/executor_tools.go:673-675`: `if !e.isToolAllowed(call.Name, cfg)` with comment at `internal/session/executor_tools.go:670-672` that registry membership does not grant capability.
* Constitutional safety gate at `internal/session/executor_tools.go:678-682`: `if e.config.EnableSafetyGate && !e.checkSafety(call)`.
* **Project write protection** at `internal/session/executor_tools.go:684-700`:
  * Comment at `internal/session/executor_tools.go:684-694` states this is what makes `nerd.md` frontmatter different in kind from `CLAUDE.md`: prose is a request, a `project_forbidden_path` fact is checked before the tool runs and no model conviction gets past it. It sits after `checkSafety` and before the Dreamer preflight because constitutional rules outrank project rules and there is no reason to simulate an already-denied action.
  * Check at `internal/session/executor_tools.go:695`: `if reason, denied := e.projectForbidsWrite(call); denied`.
  * Logs `nerd.md BLOCKED %s on %s: %s` at `internal/session/executor_tools.go:696-697`.
  * Returns `fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)", projectDocTargetPath(call.Args), reason)` at `internal/session/executor_tools.go:698-699`.
* Dreamer preflight at `internal/session/executor_tools.go:702-712`: `gate.PreflightDestructiveToolCall` for `InteractiveExecutiveGate` implementations.

Ordering is load-bearing: `isToolAllowed` → `checkSafety` → `projectForbidsWrite` → `PreflightDestructiveToolCall` → actual tool execution (modular registry at `internal/session/executor_tools.go:720-721`).

## 7. Invariants and failure modes

* Unknown frontmatter keys fail via `decoder.KnownFields(true)` at `internal/projectdoc/nerdmd.go:180` — an author who believes a directive is in force must be told it is not.
* Schema mismatch fails at `internal/projectdoc/nerdmd.go:240-245` — older binaries refuse to half-apply a document written for a newer contract.
* Write protection is fail-open on kernel query error at `internal/session/executor_tools.go:500-508` but fail-closed on explicit `project_forbidden_path` match at `internal/session/executor_tools.go:519-520`; the warning at `internal/session/executor_tools.go:505-506` makes the degraded open state auditable.
* Path matching is substring on slash-normalized, lowercased paths both in Go (`ForbidsPath` at `internal/projectdoc/nerdmd.go:285-288`) and in the kernel gate (`projectForbidsWrite` at `internal/session/executor_tools.go:510-519`); the parallel ensures Go and Mangle agree and prevents case/ separator bypasses.
* Body is never asserted as a fact per `internal/projectdoc/facts.go:42-47` — policy must not pattern-match free text.
* `Facts()` nil-safety at `internal/projectdoc/facts.go:52-54` and `PromptSection()` nil-safety at `internal/projectdoc/facts.go:157-159` allow `Load`'s `(nil, nil)` absence signal to flow without extra checks.
