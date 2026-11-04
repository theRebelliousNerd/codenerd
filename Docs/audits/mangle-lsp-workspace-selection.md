---
doc-class: deep-dive
subsystem: mangle-editor-workspace
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: e9d8d60148d6ec8452bf524a7157dcd732b4eedd
verification-method: static-source-inspection
supersedes: []
---

# Mangle LSP replaces the selected workspace with its launch directory

## Finding and required outcome

**An explicit `nerd mangle-lsp --workspace` selects the process's current directory instead of the directory supplied by the caller.** The command reads its local workspace flag, then replaces every value other than the literal `.` with `os.Getwd()`. The replacement is passed to the manager and used for the startup Mangle source index. A valid absolute path is affected just as a relative path is. [Flag and replacement][command-root] [Manager initialization][manager-init]

This can produce a plausible but wrong navigation answer, not merely an empty index. When the server starts in project A while the caller explicitly selects project B, A's definitions enter the server-wide symbol maps. Opening a B source file afterward adds that document without retiring A's definitions. A definition query for a shared symbol in B can then return A's file. Sending B's `rootUri` in the LSP initialize request does not repair the selection: the handler does not read its parameters. [Indexing][index] [Document update][document] [Lookup][lookup] [Initialize][initialize]

**Required contract:** the explicitly selected workspace must survive resolution, manager construction and indexing as the same directory identity. Relative paths need resolution against the launch directory; they must not be replaced by it. A failure to resolve or admit that selected root must remain a failure rather than causing an implicit fallback scan of another project. Preserve the existing membership boundary and the separation between editor assistance and the Mangle execution kernel.

This is a static, conditional defect at [main `e9d8d60148d6ec8452bf524a7157dcd732b4eedd`][pin], rechecked unchanged at 16:48 UTC on November 4, 2026. Referenced local source copies were checked against the pinned tree's Git blob identities. The finding establishes wrong-root indexing and possible cross-workspace editor navigation. It does not establish a production incident, a compiler/test false pass, a model permission bypass, source modification, remote transmission or measured performance loss. No application, language server, build, test, gate, model inference, training or benchmark ran. Only read-only source research and scratch documentation were performed.

## Intent, current status and scope

The command describes its flag as the workspace root to index. World guidance requires resolving workspace identity before scans and using that identity for file labels. Root guidance calls for source-grounded behavior through production entry paths. The architecture corpus distinguishes the implemented regex-based editor indexer from semantic execution; the current manager explicitly says its former world-fact projection path was removed. This report concerns the live editor route and does not revive the older README's generic shard/world-projection claims. [Command contract][command-root] [World guidance][world-guidance] [Root guidance][root-guidance] [Architecture guidance][architecture-guidance] [Mangle architecture][architecture] [Manager scope][manager-scope]

| Capability | Status at the pin | Evidence boundary |
|---|---|---|
| Register and dispatch `nerd mangle-lsp` | Implemented | Root registration selects the subcommand's `RunE`. [Registration][registration] [Command][command] |
| Read an explicit command-local workspace flag | Implemented | `GetString("workspace")` supplies the requested string. [Selection][command-root] |
| Preserve a non-dot selected root | Incorrect | The value is overwritten with the launch directory. [Replacement][command-root] |
| Pass a chosen root through manager initialization | Implemented | The manager stores its argument and passes it to `IndexWorkspace`; it does not reinterpret a discarded flag. [Manager][manager-init] |
| Apply workspace membership during source indexing | Implemented | Membership is obtained/refreshed for the supplied root and consulted during the walk. It protects that root, which may be the wrong one. [Walker][index] |
| Load client workspace identity during LSP initialization | Not implemented in this handler | It returns capabilities without decoding `rootUri`, `rootPath` or `workspaceFolders`. [Initialize][initialize] |
| Answer symbol requests from opened/indexed documents | Implemented | Definitions and references are server-wide maps; queries select the word in the requested document and retrieve matching symbols. [Index maps][maps] [Lookup][lookup] |
| Correct cross-directory CLI behavior and selected-root failure handling | Proposed, unverified | Acceptance requirements below. No new implementation or executed test receipt is claimed. |

The LSP standard carries workspace identity in initialization, including `rootUri` and, for supporting clients, `workspaceFolders`. It does not prescribe this application's CLI precedence. That precedence needs an explicit product contract; the narrow defect is already established by the existing workspace flag being discarded. [LSP initialization][lsp-initialize]

## Producer-to-consumer trace

### 1. The caller's root is replaced, not normalized

`mangleLSPCmd` installs a local `--workspace`/`-w` string with default `.`. `runMangleLSP` obtains that value, then enters its “Resolve to absolute path” branch whenever it is not exactly `.`. The branch calls `os.Getwd()` without using the selected string and assigns its result back to `workspace`. [Command selection][command-root]

Go's `Getwd` returns the current directory. `filepath.Abs`, by contrast, resolves the supplied path and joins a relative spelling with the current directory. Neither lexical absolute conversion nor current-directory lookup alone proves a unique filesystem identity. These API contracts make the substitution precise; this is not a disputed interpretation of a path-cleaning operation. [Go current directory][getwd] [Go absolute resolution][abs]

The root command also has a persistent workspace flag, but the LSP subcommand owns a local flag of that name and reads it directly. Cobra supports local flags and shadowing of inherited flags. More importantly, **the root command's `os.Chdir` is inside the interactive root `RunE`, not its inherited pre-run hook**. The pre-run initializes logging and returns; the LSP subcommand invokes its own runner. The interactive chat's directory change therefore does not rescue this route. [Global flag and root runner][root-runner] [Pre-run][pre-run] [Cobra flag semantics][cobra-flags]

This audit does not infer a single logging/configuration destination from that code. Those paths have separate early and inherited boot handling. The witness needs ordinary boot to succeed and follows only the root given to the LSP manager.

### 2. The wrong root becomes the index's source

The command constructs `NewManager(workspace)`, calls `Initialize`, then starts stdio serving. The manager stores the supplied root and constructs a Mangle engine plus `LSPServer`. It calls `IndexWorkspace(ctx, m.workspaceRoot)` before exposing the server over stdin/stdout. [Command lifecycle][command-lifecycle] [Manager lifecycle][manager-init] [Delegation][manager-serve]

`IndexWorkspace` obtains and refreshes membership for its root, walks that root, and opens admitted `.mg` and `.mangle` files. It retains the existing debug-dump exclusion and skips unreadable source files. The primary witness uses fully readable admitted files, so neither incomplete-read behavior nor an ignore-rule defect is required. Membership cannot recover a different root that the command no longer supplies. [Index walk][index]

### 3. The protocol does not replace the startup index

Once serving begins, an `initialize` request receives capabilities without parameter decoding or a call to rebuild the index. Consequently, a client that consistently names project B in both the command and `rootUri` still cannot correct the already indexed A through this request. The same is true of a supplied `workspaceFolders` field in this handler. This is a source statement about codeNERD, not a claim that every LSP server ignores those fields. [Initialize handler][initialize]

### 4. A correctly opened B document can retrieve an A definition

`didOpen` forwards the document's URI and text to `OpenDocument`. Reindexing clears definitions and references only for that document's file path, then adds its own entries. It does not erase the separate startup file from A. [Open handler][did-open] [Document indexing][document] [Per-file retirement][retire]

`GoToDefinition` reads the requested document, extracts the word at the cursor and looks it up in the server's `definitions` map by symbol. The response serializes the first returned definition's file path. With exactly one indexed definition of the selected symbol, belonging to A, the result points at A even when the cursor document belongs to B. No ordering race or ambiguous duplicate-definition tie is needed. [Definition lookup][lookup] [Response][definition-response]

This global symbol map is not independently condemned: it is useful for a single project-wide index. The causal error is populating it from a different workspace before serving the intended project.

## Minimal conditional witness

The following is a proposed fixture and source deduction, not an executed reproduction. Use ordinary absolute POSIX paths without spaces, URI escapes, symlinks or non-ASCII text. Both directories exist, are distinct and readable, and allow the ordinary process bootstrap. Their Mangle files pass membership. Project B's definition file is not separately opened by the editor before the query.

```text
/ws/a/defs.mg       Decl shared(X).
/ws/b/defs.mg       Decl shared(X).
/ws/b/main.mg       main(X) :- shared(X).
```

Each line above names a file and its relevant contents. The two declarations intentionally share a symbol: the returned file identity is the oracle, not whether the declaration's displayed text happens to look reasonable.

1. Launch from `/ws/a` with the conceptual invocation `nerd mangle-lsp --workspace /ws/b`.
2. The runner reads `/ws/b`, replaces it with `/ws/a`, and the manager indexes `/ws/a/defs.mg`.
3. Send a valid initialization request with `rootUri: "file:///ws/b"`, wait for its response, send the once-only `initialized` notification with `{}` parameters, then send `didOpen` for `file:///ws/b/main.mg` with the text above.
4. Send `textDocument/definition` for that URI at zero-based line `0`, character `13`, within `shared`.
5. The character is ordinary ASCII; word extraction returns `shared`. The only indexed declaration of that symbol is A's. The handler returns `file:///ws/a/defs.mg`, with its declaration location, rather than B's intended declaration.

Use the currently supported simple framing: a correct `Content-Length` header followed directly by the blank separator and JSON body. That avoids the separately documented multi-header transport defect. Correct URI spelling and ASCII positions likewise exclude the neighboring URI and UTF-16 issues. The witness requires no source edits in either project, no failed file read and no malicious input. [Transport boundary][transport] [Existing header audit][prior-header]

Two controls are essential:

- **Same directory:** launch from B and select B explicitly. The replacement accidentally preserves the intended directory, so this particular defect disappears.
- **Different directory, corrected root handling:** launch from A but index the selected B. With the same file set and messages, B's declaration must be returned and A's file must not be acquired by the startup scan.

If the editor later opens B's definition file, it adds B's declaration, but does not establish a clean B index: A's unrelated document remains unless separately retired. In this exact witness A remains first, so the same lookup still selects A. Conversely, if A contains no matching declaration, the symptom may be an absent definition rather than a wrong one. A's startup indexing can also fail and prevent the server from serving at all. These are different outcomes under different initial conditions, not contradictory results.

## Required design contract

### Preserve selection through one explicit root decision

Define a single effective root for the existing single-workspace server:

- With an explicit CLI workspace, resolve that supplied path against the launch directory only when relative. Preserve absolute paths. Validate the resolved target and pass the same identity to membership, the walk and the manager.
- Without an explicit override, use a documented default. Supporting client-provided initialization roots as an alternative would require changing startup order; it is not necessary to fix the explicit-flag defect.
- Keep path spelling, canonical filesystem identity and URI serialization separate. Reuse the project's established root normalization policy where appropriate rather than adding a competing ad hoc alias rule. Successful `Abs` is not a symlink identity proof.
- If a selected root is missing, not a directory, unreadable or otherwise inadmissible, report that selected-root failure. Do not scan the launch directory as recovery.
- Define how an explicit CLI root and a disagreeing client root are handled. A documented rejection or explicit precedence is preferable to silently indexing a union. Multi-root support is a separate capability, not a reason to broaden this repair.

The CLI should not need a process-wide `Chdir` to preserve a directory argument. Passing a resolved root keeps the operation local and avoids altering unrelated relative paths. A narrowly scoped adapter fix plus an end-to-end CLI witness is preferable to moving editor root selection into model discretion or creating new Mangle executive policy for an ordinary host-path operation.

### Preserve valid editor behavior and bounded failure

Keep document synchronization, membership exclusions, per-file index retirement, symbol lookup and existing protocol responses intact. Do not “fix” the wrong startup index by deleting all symbols whenever a document opens: that would destroy cross-file navigation. Nor does the finding justify banning all documents outside a root; editor-provided untitled/external documents and any eventual multi-root behavior need their own explicit contract.

For the present capability, initialization must not claim a different indexed workspace than the one actually acquired. If future root replacement is supported, retire the old generation before serving the replacement as current, and do not keep a silent mixture after a failed scan. That lifecycle is proposed future work, not an existing requirement to rebuild the server during this bounded fix.

The result remains editor assistance. This path's regex index does not execute the Mangle program and is not a compiler, gate witness or authorization decision. Improving root fidelity must not be advertised as semantic validation of the returned definition. [Architecture boundary][architecture] [Current manager boundary][manager-scope]

## Existing evidence and nonduplication

The current main tree and `Docs/audits` inventory, audit queue, relevant neighboring reports, Mangle architecture, command/source wiring and LSP test definitions were screened. Current GitHub searches for `runMangleLSP`, the root replacement, LSP workspace terms and position encoding returned source and adjacent records at this pin. The bounded screen found no dedicated report of this command's root replacement. It does not prove that no historical branch, private conversation or unrelated document mentioned it.

- **Mangle LSP header boundary** concerns loss of otherwise valid protocol frames. Here all request frames reach the handler and the wrong source index already exists before the first frame. Its transport repair does not preserve a discarded workspace argument. [Header report][prior-header]
- **Critic LSP URI identity** concerns native path/URI conversion and correlation in the external Python/TypeScript client. Here the in-process editor server is selected, with plain absolute POSIX paths and exact URI spellings. [URI report][prior-uri]
- **LSP QA journal** already identifies UTF-16/byte-coordinate problems and workspace scale/coverage concerns. Those were not repackaged as the new finding. The reviewed journal does not identify the command's non-dot root replacement. [QA journal][qa]
- **Project-document logical root identity** concerns the logical role of the supported `.nerd/nerd.md` fallback inside an already selected workspace. Here one valid unrelated directory is substituted for another before the index is constructed; no fallback document, symlink or alias is involved. [Logical-root report][prior-project-root]
- **Specialist roster workspace identity** makes the related general argument against substituting process CWD for an owned workspace, but its producer is JIT roster discovery and its consumer is the chat prompt. The LSP command discards an explicit nonempty argument before manager construction; a roster-context repair cannot change this independent command path. [Roster report][prior-roster]

The inspected LSP tests exercise direct server construction, document open/close, per-file retirement, reference isolation and request handlers. They provide useful controls but do not establish that the production command honors a workspace different from its launch directory. `UPLIFT_LEDGER` lists the command as inspected; a checked path is not a receipt for this cross-directory behavior. These test definitions were read, not run. [Basic tests][basic-tests] [Uplift tests][uplift-tests] [Ledger][ledger]

## Unrun validation and full costs

### Acceptance matrix

All evaluations below are proposed. Run them only under separate implementation/test authorization, in disposable fixtures with isolated configuration/log locations and no model/provider activity.

| Case | Required observation |
|---|---|
| Absolute B selected while process launches in A | Manager and membership receive B; startup acquisition reads B's admitted sources and no A source; navigation returns B's declaration. |
| Relative `../b` selected from A | Resolve the selected spelling to B; do not replace it with A. |
| B selected from B; omitted flag; explicit `.` | Preserve the documented default and same-directory behavior without using these as the only root tests. |
| Same symbol in A and B; only B's use file opened | Opening the use file cannot conceal a wrong startup census; assert returned URI and source acquisition, not merely a nonempty result. |
| Initialize carries the same B root | Selected identity remains B from startup through serving. |
| Initialize disagrees with the explicit root | Assert the chosen documented precedence or error; never silently merge unrelated source indexes. |
| Missing/file-valued/unreadable selected root | Selected-root failure, no launch-directory fallback. Where filesystem permissions cannot be reliably induced, use a clearly identified injected error rather than claiming an untested OS result. |
| Membership exclusions inside B | Continue honoring exclusions while indexing ordinary B sources; do not restore a private skip list. |
| Existing open/change/close, references, completion and diagnostics cases | Preserve their current lifecycle behavior; root repair must not erase other legitimate files in the selected index. |
| Platforms and aliases | Exercise supported Windows/Unix absolute and relative paths; test symlink identities only where supported. Keep URI correctness as a separate measured boundary. |

A helper test can establish resolution, but it is insufficient alone. The release criterion must include the normal CLI dispatch into manager initialization and a framed initialize/open/definition exchange. Capture the actual root consumed and the returned URI. Do not test only `NewManager(B)` or `IndexWorkspace(B)`: those bypass the producer that currently loses B. Ensure setup and teardown restore any test-owned working directory; serialize process-global directory tests or use a subprocess fixture rather than racing unrelated tests.

### What to measure

Measure correctness first: selected-root identity, admitted source-file census, unexpected A files acquired, missing B files, definition URI precision, completion contamination and failures attributed to the selected root. Measure startup time, files/bytes read, indexing CPU, peak retained document/index memory, query latency and restart/retry counts. Use identical trees and message sequences across baseline and candidate. Include both small and large launch directories, and reverse their relative sizes.

Wrong-root indexing consumes the work of traversing and indexing A while failing to supply B's project context. Opening additional B files manually can add work and retained state without removing A. The bounded correction may reduce or increase raw startup resource use depending on which tree is larger; scanning fewer files by selecting the wrong project is not an efficiency win. Path validation/canonicalization has filesystem and error-handling cost, and an optional future initialization-root negotiation has compatibility, startup-order and index-retirement cost.

No LLM call is needed on the demonstrated editor path. Therefore this report claims no direct token or inference savings. If a later user study measures reduced developer searching, mistaken navigation or repeated restarts, report those separately from host resource costs and do not infer them from a unit test. No such measurements exist in this audit.

**Exit criterion:** from a launch directory different from the explicitly selected workspace, the normal editor command indexes the selected workspace and resolves its unopened-file definitions without acquiring or returning unrelated launch-directory definitions. Unsupported or failed selected-root resolution remains explicit. Existing membership and editor lifecycle controls continue to hold.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/e9d8d60148d6ec8452bf524a7157dcd732b4eedd
[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/agents.md#L17-L23
[world-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/world/agents.md#L1-L24
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/architecture/AGENTS.md#L48-L74
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/architecture/mangle/README.md#L39-L63
[command]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/cmd_mangle_lsp.go#L74-L105
[command-root]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/cmd_mangle_lsp.go#L102-L126
[command-lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/cmd_mangle_lsp.go#L123-L158
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/main.go#L278-L297
[pre-run]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/main.go#L122-L168
[root-runner]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/cmd/nerd/main.go#L177-L210
[manager-scope]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/world/lsp/manager.go#L12-L18
[manager-init]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/world/lsp/manager.go#L29-L64
[manager-serve]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/world/lsp/manager.go#L79-L92
[index]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L603-L645
[maps]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L26-L35
[document]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L135-L203
[retire]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L226-L271
[lookup]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L381-L433
[transport]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L675-L742
[initialize]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L787-L804
[did-open]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L806-L818
[definition-response]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp.go#L903-L930
[basic-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp_test.go#L7-L100
[uplift-tests]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/internal/mangle/lsp_uplift_test.go#L13-L154
[prior-header]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/audits/mangle-lsp-header-boundary.md
[prior-uri]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/audits/lsp-document-uri-identity-and-diagnostic-grounding.md
[prior-project-root]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/audits/projectdoc-logical-root-identity.md
[prior-roster]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/audits/specialist-roster-workspace-identity.md
[qa]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/.quality_assurance/lsp_qa_journal_20261010_002712.md
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/e9d8d60148d6ec8452bf524a7157dcd732b4eedd/Docs/audits/UPLIFT_LEDGER.md#L243
[getwd]: https://pkg.go.dev/os#Getwd
[abs]: https://pkg.go.dev/path/filepath#Abs
[cobra-flags]: https://cobra.dev/docs/how-to-guides/working-with-flags/
[lsp-initialize]: https://raw.githubusercontent.com/microsoft/language-server-protocol/gh-pages/_specifications/lsp/3.17/general/initialize.md
