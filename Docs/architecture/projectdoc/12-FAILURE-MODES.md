# 12 — Failure Modes — projectdoc (nerd.md)

> **Scope:** `internal/projectdoc` (parsing, validation, fact projection) and `internal/session/executor_tools.go` (enforcement, prompt injection). Every `file:line` below was verified against `read_file` output that prefixes each line with its real line number and a tab. No line numbers are estimated. A prior revision incorrectly cited `ForbidsPath` in `internal/projectdoc/facts.go` and `SetProjectDoc` in `internal/session/spawner.go`; the correct locations are `internal/projectdoc/nerdmd.go:281` and `internal/session/executor_tools.go:433` respectively.

## How nerd.md splits

* Strict YAML frontmatter → kernel facts (enforced). Advisory Markdown body → prompt atom. See package doc `internal/projectdoc/nerdmd.go:1` and `internal/projectdoc/facts.go:42` (“Only the frontmatter is projected…”).
* Schema is pinned: `internal/projectdoc/nerdmd.go:43` `SchemaVersion = "nerd/v1"`. Unknown keys are hard errors, not silent drops.

---

## FM-01 — Missing file is “no directive”

**What breaks:** All write protections disappear.

**Trigger:** No file at either candidate path.

**Observable symptom:** Writes that should be denied succeed; `project_doc` fact absent from kernel; no error returned.

**Handling / failing code:**
* `internal/projectdoc/nerdmd.go:131` `func Find(workspace string) string` checks `filepath.Join(workspace, FileName)` then `filepath.Join(workspace, ".nerd", FileName)` at `internal/projectdoc/nerdmd.go:132`–`internal/projectdoc/nerdmd.go:135` and returns `""` when neither exists at `internal/projectdoc/nerdmd.go:140`.
* `internal/projectdoc/nerdmd.go:148` `func Load(workspace string) (*Document, error)` returns `(nil, nil)` when `Find` returns `""` at `internal/projectdoc/nerdmd.go:149`–`internal/projectdoc/nerdmd.go:152`.
* `internal/projectdoc/facts.go:51` `func (d *Document) Facts() []types.Fact` returns `nil` for nil `d` at `internal/projectdoc/facts.go:52`–`internal/projectdoc/facts.go:54`.
* `internal/projectdoc/nerdmd.go:281` `func (d *Document) ForbidsPath` returns `"", false` for nil `d` at `internal/projectdoc/nerdmd.go:282`–`internal/projectdoc/nerdmd.go:283`.
* `internal/session/executor_tools.go:445` `func (e *Executor) withProjectInstructions` delegates to `doc.PromptSection()` at `internal/session/executor_tools.go:450`; `internal/projectdoc/facts.go:156` `PromptSection` returns `""` for nil `d` at `internal/projectdoc/facts.go:157`–`internal/projectdoc/facts.go:159`.

---

## FM-02 — Unreadable file (permissions / I/O error)

**What breaks:** Load fails; agent never starts with project rules.

**Trigger:** File exists per `Find` but `os.ReadFile` fails.

**Observable symptom:** `Load` returns wrapped error `read <path>: %w`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:153` `data, err := os.ReadFile(path)` and `internal/projectdoc/nerdmd.go:154`–`internal/projectdoc/nerdmd.go:156` `return nil, fmt.Errorf("read %s: %w", path, err)`.

---

## FM-03 — File empty or first line not `---`

**What breaks:** Entire document rejected before any rule is evaluated.

**Trigger:** Empty file, or file whose first non-space line is not the fence.

**Observable symptom:** `Parse` error: `file is empty; expected a "---" frontmatter block` or `first line must be "---" to open the frontmatter block (got "…")`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:192` `const frontmatterFence = "---"`.
* `internal/projectdoc/nerdmd.go:195` `func splitFrontmatter` — scanner setup at `internal/projectdoc/nerdmd.go:196`, buffer override at `internal/projectdoc/nerdmd.go:200` (`64*1024` initial, `4*1024*1024` max).
* Empty check at `internal/projectdoc/nerdmd.go:202`–`internal/projectdoc/nerdmd.go:204`.
* Fence open check at `internal/projectdoc/nerdmd.go:205`–`internal/projectdoc/nerdmd.go:210` (`strings.TrimSpace(scanner.Text()) != frontmatterFence`).

---

## FM-04 — Frontmatter opened but never closed

**What breaks:** Body is never parsed; frontmatter contents are ambiguous.

**Trigger:** Missing closing `---` fence.

**Observable symptom:** Error `frontmatter block opened with "---" was never closed`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:212`–`internal/projectdoc/nerdmd.go:220` loop collecting `frontLines` until closing fence sets `closed = true` at `internal/projectdoc/nerdmd.go:216`.
* Failure at `internal/projectdoc/nerdmd.go:221`–`internal/projectdoc/nerdmd.go:223` `if !closed { return …, fmt.Errorf("frontmatter block opened with %q was never closed", frontmatterFence) }`.

---

## FM-05 — Scan / read error inside frontmatter or body

**What breaks:** Large single-line bodies or I/O errors abort parsing with a generic read error that masks the true location.

**Trigger:** A body line > 4 MiB exceeds the overridden buffer, or underlying `Scanner.Err`.

**Observable symptom:** Error `read: %w` with no frontmatter context.

**Handling code:**
* Body scan loop at `internal/projectdoc/nerdmd.go:225`–`internal/projectdoc/nerdmd.go:228`.
* Error check at `internal/projectdoc/nerdmd.go:229`–`internal/projectdoc/nerdmd.go:231` `if scanErr := scanner.Err(); scanErr != nil`.
* Earlier buffer comment at `internal/projectdoc/nerdmd.go:197`–`internal/projectdoc/nerdmd.go:200` explains why default 64 KiB was raised.

---

## FM-06 — Unknown YAML key (strict schema)

**What breaks:** Any typo or forward-compatible field is a hard parse error.

**Trigger:** Frontmatter contains a key not in `Spec` (`timeout:`, `forbidden:` vs `forbid:`, etc.).

**Observable symptom:** `invalid frontmatter: yaml: … field … not found in type projectdoc.Spec`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:177` `decoder := yaml.NewDecoder(bytes.NewReader(front))`.
* `internal/projectdoc/nerdmd.go:180` `decoder.KnownFields(true)` with comment at `internal/projectdoc/nerdmd.go:178`–`internal/projectdoc/nerdmd.go:180` explaining fail-loud intent.
* `internal/projectdoc/nerdmd.go:181`–`internal/projectdoc/nerdmd.go:183` `if err := decoder.Decode(&spec); err != nil { return nil, fmt.Errorf("invalid frontmatter: %w", err) }`.
* Schema extensibility note at `internal/projectdoc/nerdmd.go:60`–`internal/projectdoc/nerdmd.go:62` (“Adding a field here is a schema change: bump SchemaVersion”).

---

## FM-07 — Invalid YAML syntax

**What breaks:** Same hard failure as FM-06 but for malformed YAML.

**Trigger:** Bad indentation, missing colon, unclosed quote in frontmatter.

**Observable symptom:** Same `invalid frontmatter: %w` at `internal/projectdoc/nerdmd.go:182`.

**Handling code:** Identical decode path `internal/projectdoc/nerdmd.go:177`–`internal/projectdoc/nerdmd.go:183`.

---

## FM-08 — Missing or wrong `schema` (pin mismatch)

**What breaks:** Valid-looking rules are rejected because contract version does not match binary.

**Trigger:** `schema:` absent/empty or not exactly `nerd/v1`.

**Observable symptom:** `frontmatter is missing the required "schema" key; expected "nerd/v1"` or `unsupported schema "…" ; this build speaks "nerd/v1". Refusing to half-apply…`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:236` `func (s *Spec) validate() error`.
* Empty check at `internal/projectdoc/nerdmd.go:237`–`internal/projectdoc/nerdmd.go:239`.
* Mismatch check at `internal/projectdoc/nerdmd.go:240`–`internal/projectdoc/nerdmd.go:245` (`if s.Schema != SchemaVersion`).
* Pin definition at `internal/projectdoc/nerdmd.go:43` `const SchemaVersion = "nerd/v1"` and strictness comment at `internal/projectdoc/nerdmd.go:40`–`internal/projectdoc/nerdmd.go:42`.
* Post-decode validation call at `internal/projectdoc/nerdmd.go:185`–`internal/projectdoc/nerdmd.go:187`.

---

## FM-09 — `forbid` rule with empty `match` or missing `reason`

**What breaks:** Parse fails before any fact is emitted.

**Trigger:** `forbid:` entry with `match: ""`/`match: "   "` or `reason: ""`.

**Observable symptom:** `forbid[0] has an empty "match"; a rule that matches every path would deny every write` or `forbid[0] (match "…") has no "reason"; a denial the agent cannot explain reads as a malfunction…`.

**Handling code:**
* Type at `internal/projectdoc/nerdmd.go:108` `type ForbidRule struct` with `Match` at `internal/projectdoc/nerdmd.go:115` and `Reason` at `internal/projectdoc/nerdmd.go:120`.
* Substring semantics comment at `internal/projectdoc/nerdmd.go:109`–`internal/projectdoc/nerdmd.go:114`.
* Loop at `internal/projectdoc/nerdmd.go:247` `for i, rule := range s.Forbid`.
* Empty match guard at `internal/projectdoc/nerdmd.go:248`–`internal/projectdoc/nerdmd.go:250`.
* Empty reason guard at `internal/projectdoc/nerdmd.go:251`–`internal/projectdoc/nerdmd.go:255`.

---

## FM-10 — `conventions` / `require` entries empty

**What breaks:** Same hard validation as FM-09; prevents vacuous rules.

**Trigger:** `conventions:` with empty `id`/`rule` or `require:` with empty string.

**Observable symptom:** `conventions[0] has an empty "id"`, `conventions[0] (id "…") has an empty "rule"`, `require[0] is empty`.

**Handling code:**
* `internal/projectdoc/nerdmd.go:258`–`internal/projectdoc/nerdmd.go:265` conventions loop (`c.ID` check at `internal/projectdoc/nerdmd.go:259`, `c.Rule` check at `internal/projectdoc/nerdmd.go:262`).
* `internal/projectdoc/nerdmd.go:267`–`internal/projectdoc/nerdmd.go:271` require loop.
* Type at `internal/projectdoc/nerdmd.go:124` `type Convention struct`.

---

## FM-11 — Forbidden-path matching is substring + slash-normalized + case-insensitive

**What breaks:** Rule author intent and enforcement diverge.

**Trigger:** Author writes `match: ".git"` expecting glob/exact directory, or `match: "secrets/"` but file is `Secrets\Keys.txt` on Windows, or `match: "config"` matching `myconfig.json`.

**Observable symptom:** False positive (unexpected denial) or false negative (expected denial does not fire). `ForbidsPath` both under- and over-matches relative to author model.

**Handling code (Go helper):**
* `internal/projectdoc/nerdmd.go:281` `func (d *Document) ForbidsPath(target string) (reason string, forbidden bool)` — **this is the correct location; `ForbidsPath` does not exist in `internal/projectdoc/facts.go`**.
* Nil/empty guard at `internal/projectdoc/nerdmd.go:282`–`internal/projectdoc/nerdmd.go:283`.
* Normalization at `internal/projectdoc/nerdmd.go:285` `strings.ToLower(filepath.ToSlash(target))`.
* Substring check at `internal/projectdoc/nerdmd.go:287` `strings.Contains(normalized, strings.ToLower(filepath.ToSlash(rule.Match)))` and return reason at `internal/projectdoc/nerdmd.go:288`.

**Handling code (enforcement path — kernel, not helper):**
* `internal/session/executor_tools.go:487` `func (e *Executor) projectForbidsWrite(call ToolCall) (string, bool)` — the executor gate, not `ForbidsPath`.
* Write-tool guard at `internal/session/executor_tools.go:488`–`internal/session/executor_tools.go:490` (`if !isWriteMutationTool(call.Name) { return "", false }`).
* Target extraction at `internal/session/executor_tools.go:491`–`internal/session/executor_tools.go:494` via `internal/session/executor_tools.go:466` `func projectDocTargetPath`.
* Kernel query at `internal/session/executor_tools.go:499` `facts, err := e.kernel.Query(projectdoc.PredForbiddenPath)` where `internal/projectdoc/facts.go:33` `PredForbiddenPath = "project_forbidden_path"`.
* Fail-open on query error at `internal/session/executor_tools.go:500`–`internal/session/executor_tools.go:508` (Warn `nerd.md write protection could not be evaluated…; allowing the write`).
* Same substring logic mirrored at `internal/session/executor_tools.go:510` `normalized := strings.ToLower(filepath.ToSlash(target))` and `internal/session/executor_tools.go:515`/`internal/session/executor_tools.go:519` `strings.Contains(normalized, match)`.

**Note on divergence:** `ForbidsPath` and `projectForbidsWrite` implement identical semantics but are separate code paths. A fix to one without the other silently reintroduces the mismatch.

---

## FM-12 — Fact projection drops or mangles fields

**What breaks:** Frontmatter is accepted but kernel facts or prompt prose do not reflect it; policy or model acts on stale view.

**Triggers and handling:**

* **Nil document:** `internal/projectdoc/facts.go:52`–`internal/projectdoc/facts.go:54` returns `nil`; `internal/projectdoc/facts.go:157`–`internal/projectdoc/facts.go:159` `PromptSection` returns `""`. Symptom: silent no-op.
* **Body not projected:** `internal/projectdoc/facts.go:42`–`internal/projectdoc/facts.go:47` (“Only the frontmatter is projected…”). Body appears only via `internal/projectdoc/facts.go:236`–`internal/projectdoc/facts.go:239`. Symptom: policy cannot match on body text (by design); expecting policy to enforce body prose fails.
* **Language normalization:** `internal/projectdoc/facts.go:64`–`internal/projectdoc/facts.go:69` calls `internal/projectdoc/facts.go:129` `func normalizeAtom`. Inside `normalizeAtom`, lowercasing and slash-prefix at `internal/projectdoc/facts.go:130`, rune filtering at `internal/projectdoc/facts.go:136`–`internal/projectdoc/facts.go:142` (`"-"/" "/" ."` → `"_"`), empty check at `internal/projectdoc/facts.go:144`–`internal/projectdoc/facts.go:147`. Trigger: `language: "  "` or `language: "***"` → returns `""` at `internal/projectdoc/facts.go:131` or `internal/projectdoc/facts.go:145`, so no `project_language` fact emitted. Symptom: language-gated prompt atoms never activate, no error.
* **Empty commands:** `internal/projectdoc/facts.go:71`–`internal/projectdoc/facts.go:84` iterates `map[string]string{"/build":…, "/test":…, "/lint":…, "/run":…}` and `continue`s on `strings.TrimSpace(command)==""` at `internal/projectdoc/facts.go:77`. Symptom: missing command is zero facts, not an error. `CommandCount` mirrors at `internal/projectdoc/facts.go:112`–`internal/projectdoc/facts.go:123` with same `TrimSpace` guard at `internal/projectdoc/facts.go:118`.
* **Predicate constants:** `internal/projectdoc/facts.go:15` `PredPresent = "project_doc"` emitted at `internal/projectdoc/facts.go:56`–`internal/projectdoc/facts.go:58` as `(Path, Schema)`; `internal/projectdoc/facts.go:18` `PredName`; `internal/projectdoc/facts.go:22` `PredLanguage` (emitted as `types.MangleAtom` at `internal/projectdoc/facts.go:68`); `internal/projectdoc/facts.go:26` `PredCommand`; `internal/projectdoc/facts.go:30` `PredCommandEnv` (loop at `internal/projectdoc/facts.go:86`); `internal/projectdoc/facts.go:33` `PredForbiddenPath` (loop at `internal/projectdoc/facts.go:93`); `internal/projectdoc/facts.go:36` `PredRequirement`; `internal/projectdoc/facts.go:39` `PredConvention`.

---

## FM-13 — Enforcement gate bypasses and fail-open cases

**What breaks:** A write that matches a `project_forbidden_path` fact is not denied.

**Triggers → symptom → exact line:**

* **Non-write tool:** any read/search tool targeting a protected path. Symptom: allowed (by design) but author may expect denial for reads. Gate at `internal/session/executor_tools.go:488` `if !isWriteMutationTool(call.Name)`.
* **Empty target:** tool call with no recognized path arg or `target: ""`. Symptom: write proceeds. Guard at `internal/session/executor_tools.go:491`–`internal/session/executor_tools.go:494` (`target == ""` → `return "", false`).
* **Unrecognized arg name:** model uses a novel arg key not in `internal/session/executor_tools.go:463` `var projectDocPathArgs = []string{"path", "file_path", "filepath", "file", "filename", "target", "dest", "destination"}`. Symptom: `internal/session/executor_tools.go:466` `func projectDocTargetPath` returns `""` at `internal/session/executor_tools.go:474`, so `projectForbidsWrite` bails.
* **Missing tool in `isWriteMutationTool`:** a registered durable-write action not listed at `internal/session/executor_tools.go:411` `func isWriteMutationTool`. Symptom: write mutates protected path without hitting gate *and* `checkHollowSuccess` miscounts. Current list at `internal/session/executor_tools.go:412`–`internal/session/executor_tools.go:420` (`write_file`, `edit_file`, `delete_file`, `edit_lines`, `insert_lines`, `delete_lines`, `edit_element`, `fs_write` plus defensive aliases `apply_patch` etc.).
* **Nil kernel:** Symbol `e.kernel == nil` at `internal/session/executor_tools.go:495`–`internal/session/executor_tools.go:497`. Symptom: all writes allowed, no log.
* **Kernel query error:** transient failure at `internal/session/executor_tools.go:499`. Symptom: fail-open with Warn at `internal/session/executor_tools.go:505`–`internal/session/executor_tools.go:507` `nerd.md write protection could not be evaluated for %s (%v); allowing the write`.
* **Case/slash mismatch / malformed fact:** `len(fact.Args) < 2` skip at `internal/session/executor_tools.go:512`–`internal/session/executor_tools.go:514`, empty `match` skip at `internal/session/executor_tools.go:515`–`internal/session/executor_tools.go:518`.
* **Call site ordering:** `internal/session/executor_tools.go:669` `func (e *Executor) executeToolCall` checks `isToolAllowed` at `internal/session/executor_tools.go:673`, safety gate at `internal/session/executor_tools.go:678`–`internal/session/executor_tools.go:681`, then `projectForbidsWrite` at `internal/session/executor_tools.go:695`–`internal/session/executor_tools.go:700` (Warn at `internal/session/executor_tools.go:696`–`internal/session/executor_tools.go:697` and return `blocked by nerd.md: %s is write-protected (%s)` at `internal/session/executor_tools.go:698`–`internal/session/executor_tools.go:700`). Symptom if ordering wrong: Dreamer preflight at `internal/session/executor_tools.go:707`–`internal/session/executor_tools.go:712` would simulate a denied action, or safety gate would be bypassed.

---

## FM-14 — `SetProjectDoc` wiring vs kernel authority

**What breaks:** Developer assumes prompt wiring governs enforcement; a mis-wired executor appears protected (prose injected) but is not enforced, or vice versa.

**Trigger:** `SetProjectDoc` not called on a subagent executor, or `projectDoc` overwritten without kernel re-assertion.

**Observable symptom:** Prompt shows “Write-protected paths (ENFORCED)” but writes succeed, or writes are denied but prompt never mentioned protection (wasted turn).

**Handling code (correct locations):**
* `internal/session/executor_tools.go:433` `func (e *Executor) SetProjectDoc(doc *projectdoc.Document)` — **the canonical definition; not `internal/session/spawner.go`**. Body at `internal/session/executor_tools.go:434` `e.mu.Lock()` and `internal/session/executor_tools.go:436` `e.projectDoc = doc`.
* Comment at `internal/session/executor_tools.go:426`–`internal/session/executor_tools.go:432` (“Only the prose rendering is held here. Write protection is enforced by querying the kernel…so a subagent that never receives this pointer is still governed…”).
* Prompt injection at `internal/session/executor_tools.go:445` `func (e *Executor) withProjectInstructions` — copies `e.projectDoc` under `RLock` at `internal/session/executor_tools.go:446`–`internal/session/executor_tools.go:448`, renders via `internal/projectdoc/facts.go:156` `PromptSection` at `internal/session/executor_tools.go:450`, injects at `internal/session/executor_tools.go:454`–`internal/session/executor_tools.go:455`.
* Enforcement does **not** read `e.projectDoc`; it queries `internal/session/executor_tools.go:499` `e.kernel.Query(projectdoc.PredForbiddenPath)` where `internal/projectdoc/facts.go:33` defines the predicate.

---

## FM-15 — `withProjectInstructions` silent no-op

**What breaks:** Project prose expected in prompt is absent, model violates convention then claims it was never told.

**Trigger:** `projectDoc` is nil or `PromptSection` is `""` (empty doc).

**Observable symptom:** No log, no error; prompt unchanged.

**Handling code:**
* `internal/session/executor_tools.go:450` `section := doc.PromptSection()` where `doc` may be nil; `internal/projectdoc/facts.go:157`–`internal/projectdoc/facts.go:159` guards nil → `""`.
* Early return at `internal/session/executor_tools.go:451`–`internal/session/executor_tools.go:453` `if section == "" { return systemPrompt }`.
* Log only on inject at `internal/session/executor_tools.go:454` `logging.Session("Injected %s instructions …", doc.Path, len(section))` — absence is not logged.

---

## FM-16 — `intentRequiresToolCall` / hollow-success interaction masks nerd.md denial

**What breaks:** A write denied by nerd.md is counted as “no tool call completed,” so `checkHollowSuccess` fails the run with `hollow success blocked:` instead of the clearer `blocked by nerd.md` error, confusing the user about which gate fired.

**Trigger:** Model issues only a write to a protected path; `projectForbidsWrite` denies it; `SuccessfulWriteTools` stays 0.

**Observable symptom:** Run fails with `hollow success blocked: write-oriented intent … completed without write_file/edit_file (tool_calls=…)` rather than the per-tool `blocked by nerd.md` message. Tool-level denial was logged but top-level error is hollow-success.

**Handling code:**
* `internal/session/executor_tools.go:360` `func (e *Executor) intentRequiresToolCall(verb string) bool` — kernel nil guard at `internal/session/executor_tools.go:361`–`internal/session/executor_tools.go:362`, query at `internal/session/executor_tools.go:367`–`internal/session/executor_tools.go:368` `fmt.Sprintf("intent_requires_tool_call(%s)", verb)`.
* `internal/session/executor_tools.go:381` `func isWriteOrientedIntent` switch at `internal/session/executor_tools.go:382`–`internal/session/executor_tools.go:388`.
* `internal/session/executor_tools.go:339`–`internal/session/executor_tools.go:341` `SuccessfulWriteTools++` only on `isWriteMutationTool` success (so denied writes do not increment).
* `internal/session/executor_tools.go:540` `func (e *Executor) checkHollowSuccess` — `requiresTools` at `internal/session/executor_tools.go:552`–`internal/session/executor_tools.go:555`, zero-call check at `internal/session/executor_tools.go:557`–`internal/session/executor_tools.go:562`, write-tool zero check at `internal/session/executor_tools.go:566`–`internal/session/executor_tools.go:571`.
* Per-tool denial originates at `internal/session/executor_tools.go:695`–`internal/session/executor_tools.go:700`.

---

## Cross-reference to prior citation errors (do not repeat)

* `ForbidsPath` is declared at `internal/projectdoc/nerdmd.go:281`. It does not exist in `internal/projectdoc/facts.go` (which defines `Facts` at `internal/projectdoc/facts.go:51`, `normalizeAtom` at `internal/projectdoc/facts.go:129`, `PromptSection` at `internal/projectdoc/facts.go:156`).
* `SetProjectDoc` is declared at `internal/session/executor_tools.go:433`. It is not declared in `internal/session/spawner.go`. `executor_tools.go` also owns `projectForbidsWrite` at `internal/session/executor_tools.go:487`, `projectDocTargetPath` at `internal/session/executor_tools.go:466`, and `withProjectInstructions` at `internal/session/executor_tools.go:445`.

---

## Verification note

All `file:line` citations above were taken from `read_file` output where each line is prefixed `<line>\t<content>`. Fixes to this document should re-page the three source files (`internal/projectdoc/nerdmd.go`, `internal/projectdoc/facts.go`, `internal/session/executor_tools.go`) and copy the prefix number verbatim.
