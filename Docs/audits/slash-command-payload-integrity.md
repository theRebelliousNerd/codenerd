---
doc-class: deep-dive
subsystem: cli
implementation-status: implemented except tab transport through the input widget (see As-built status)
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
supersedes: []
---

# Preserve file payloads across chat slash command ingestion

## Decision

Separate command parsing from file-content transport. `/write`, `/append`, and patch ingestion currently treat source text like conversational command tokens, changing meaningful whitespace before the writer receives it.

Preserve the user's payload exactly within an explicitly supported text format, or reject unsupported input before mutation. Keep command recognition, terminal-display protection, path authorization and execution policy separate from payload normalization. This is a repair to existing human-facing file commands, not another model planner or editing tool.

**Evidence status:** static source inspection. The transformations below follow from the inspected operations; their examples are proposed deterministic witnesses, not executed reproductions. No tests, builds, model calls, benchmarks, or failure injection were performed. No claim is made that these defects caused a particular user incident.

## 1 Scope and source freeze

The inspected heads were main `356f65e0e6bb639adaa90ab36212daa714114871`, feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`, and research `9a1a91286370674eec26f03ac2184bb6c9ad2658`. All three were unchanged when rechecked at approximately 14:13 UTC on 2026-10-28. Feature was 27 commits ahead of main and zero behind. Links below use the feature pin unless explicitly marked main.

The affected route is the interactive TUI's **human slash commands**. It is not the native `write_file`/`edit_file` tool path or CodeDOM batching. Current root guidance calls for tools that reduce turns and offload deterministic work; it does not justify silently changing source supplied by the person. [Repository guidance][guidance]

The inspected research corpus already covers:

- [Edit observation precondition continuity][precondition-study]: revision/read preconditions through batching.
- [Tool effect receipts and validation completeness][receipt-study]: mutations surviving failed validation.
- [Workspace search navigation fidelity][search-study]: search acquisition and navigation identity.
- [TUI evidence turn ownership][evidence-study]: acquisition, coverage and delivery of file observations.
- [Chat session memory ownership and recovery][session-study]: selected-session memory and persistence.
- [Tool argument coercion][coercion-study]: integer argument extraction.

Those boundaries should remain intact. The question here is earlier and narrower: **are the file bytes reaching a direct command's writer the bytes the user supplied?** No matching slash-payload finding was found in these inspected adjacent audits; this is not a claim that every historical document has been exhaustively reviewed.

The user-facing registry describes `/write` as writing a file, `/append` as appending to a file, and `/edit` as an inline-editing entry point. It does not advertise prose whitespace normalization. [Command registry][registry]

## 2 Finding P1 The write and append commands reconstruct a different payload

The source chain is explicit:

1. `handleSubmit` calls `strings.TrimSpace` on the textarea value before dispatching a slash command. This already removes trailing payload whitespace at the submission boundary. [Submission][submission]
2. `handleCommand` sanitizes the input, then runs `strings.Fields(input)` over the **entire command**, including its payload. [Dispatcher][dispatcher]
3. `/write` takes `parts[1]` as the path and reconstructs content using `strings.Join(parts[2:], " ")`. `/append` performs the same reconstruction and adds a newline. [Write handler][write-handler], [append handler][append-handler]
4. `writeFileContent` directly passes those reconstructed bytes to `os.WriteFile`. The append helper passes them to `WriteString`. Neither can reconstruct whitespace discarded upstream. [Writer][writer], [appender][appender]

**Source-derived witness:** `/write sample.txt alpha  beta`, containing two spaces between the words, produces a writer payload containing one. Quotes do not protect payload spans from `strings.Fields`; a JSON string such as `"alpha  beta"` is also changed internally. Multiline source reaching this dispatcher loses its newlines and indentation through the same mechanism.

That is a semantic change, not merely prettier formatting. It can alter literal strings, indentation-sensitive code, structured data and exact-match fixtures. When the IO call succeeds, the handler reports that the file was written or appended; it does not disclose that it transformed the content. These are conditional IO-success observations from source, not a reproduced successful write.

### Preserve the feature branch read fix

The feature branch correctly strengthened `/edit` to refuse a partial read rather than offer a prefix for overwriting. It instead directs the person to `/read` with a range or `/append`. Preserve that improvement and do not report main's former prefix-overwrite invitation as current feature behavior. However, the feature's **whole-file** branch still instructs the person to save through `/write`, so whole observation coverage does not imply a lossless edit round trip. [Current edit behavior][edit-handler]


The whitespace reconstruction defect exists on main as well. [Main dispatcher][main-dispatcher], [main writer handler][main-write-handler]

## 3 Finding P2 Patch ingestion trims syntax before the patch parser sees it

`/patch` explicitly asks the person to paste a patch **line by line**, ending with `--END--`. [Patch-mode entry][patch-entry]

However, `handleSubmit`:

- trims the submission before checking patch mode;
- handles an empty trimmed submission before patch ingestion;
- appends the already-trimmed string to `pendingPatchLines`;
- later joins those strings with newlines.

[Patch ingestion boundary][submission]

For the documented line-by-line workflow, a unified-diff context line beginning with a space loses that marker. Trailing spaces on additions, deletions or context lines are also removed. A whitespace-only submission is not retained as patch data. GNU's primary format description specifies the leading space for a line common to both files, contrasting it with `+` and `-` indicators for additions and removals. The removed leading character therefore carries syntax, not decorative indentation. [GNU unified-format specification][gnu-unified]

**Precise proposed witness:** submit a nonempty context line whose first character is a single space, for example ` unchanged`. The buffer entry becomes `unchanged`. Separately, a line consisting only of the context marker trims to empty and leaves through the empty-input branch before patch accumulation. No backend assumptions are needed to establish those transformations.

**Bounded conclusion:** the buffer delivered downstream can differ syntactically and semantically from the supplied patch before any backend parses it. This inspection did not execute `git apply` or `patch`, so it does not establish a particular backend error or assert that every pasted patch fails. Multiline submissions have a different loss pattern: trimming applies at each submitted chunk's boundaries, not every interior line.

Main has the same ingestion behavior. [Main submission][main-submission]

### What the backend documentation establishes

The inspected helper invokes `git apply --whitespace=nowarn`, then may fall back to `patch -p0`. [Backend invocation][patch-backend] Git documents `nowarn` as suppressing trailing-whitespace warnings; it is not the `fix` option. Git also documents `--check` as checking applicability without applying. These support two separate qualification stages: verify lossless input first, then check backend applicability against the intended source tree. [Git apply manual][git-apply]

Neither the manual nor a proposed successful `--check` proves exact final application under subsequent concurrent source changes, and neither can reconstruct bytes already discarded by chat ingestion. This research did not inspect an installed Git/patch version, run a check, or validate the helper's wrapping/fallback behavior end to end. Patch-format selection and backend compatibility remain explicit future acceptance work, not additional demonstrated defects in this note.

## 4 Existing controls to preserve

The sanitizer deliberately retains newline, carriage-return and tab characters, and its inspected unit test checks that behavior. The later `Fields`/`Join` composition destroys distinctions that this local helper preserves. A passing sanitizer test therefore would not establish command-to-file fidelity. The tests were read, not run. [Sanitizer][sanitizer], [preservation test][sanitizer-test]

Do not simply remove all input protection. Terminal rendering and accepted file representation need separate contracts. Unsupported control bytes should receive an explicit rejection or documented transformation, rather than an unnoticed mutation of the file payload. Exact binary transport is outside the demonstrated text-command capability.

Do not conflate this finding with authorization. Human slash commands and model-proposed operations have different entry points; this note establishes a transformation defect, not a model permission bypass. Any repair must preserve the applicable workspace and policy boundaries rather than treating literal-payload acceptance as permission to write.

## 5 Proposed repair contract

These are design requirements, not shipped APIs:

1. **Parse only the command envelope.** Identify verb and path with an explicit grammar, then retain the unparsed payload. Define path quoting and the delimiter between envelope and body.
2. **Preserve raw submission for content modes.** Use a separate normalized view to recognize commands and the patch terminator; do not use that normalized view as file data.
3. **Specify append semantics.** Decide whether append means exact bytes or a line append. Preserve all internal content either way, and make newline insertion explicit.
4. **Define patch framing.** State accepted patch formats, chunk/newline handling, cancellation and how a literal terminator is represented. Do not change formats silently.
5. **Validate before mutation.** Unsupported framing, encoding or size should refuse the operation without touching the destination. Keep authorization and containment requirements independent of parser acceptance.
6. **Report the actual operation.** Success should identify the admitted payload and bytes written. Transformation, refusal and partial/error outcomes must not masquerade as an exact save.

A focused parser repair need not invent a new editing subsystem. An explicit multiline entry mode may be clearer than an increasingly complicated inline syntax. Retiring a misleading inline workflow is also an option, but requires truthful help and a usable supported replacement.

No executive heuristic is required to decide what the user's source bytes mean. Go should preserve and validate the transport; policy still governs which operations may occur. If a repair introduces model-facing explanations, follow the repository's JIT-first requirement rather than adding an unrelated prompt path.

## 6 Verification program not executed

Exercise the real submission → dispatcher → handler → writer route, not only helpers:

| Fixture | Required observation |
|---|---|
| Repeated spaces inside ordinary text and JSON strings | Destination preserves exact payload bytes. |
| Tabs, blank lines, multiline code and trailing whitespace | Preservation matches the declared format; unsupported input refuses before write. |
| Edit whole-file display followed by unchanged save | Supported text round-trips without semantic modification. |
| Feature edit of a file above its display budget | Existing partial-read refusal remains; no invitation to replace the file with a prefix. |
| Append with and without a final newline | Exact declared newline behavior; existing file prefix unchanged. |
| Patch supplied line by line | Context markers and trailing spaces survive ingestion. |
| Same patch supplied in differently sized chunks | Equivalent admitted patch under the declared framing rules. |
| Blank patch line, literal terminator and canceled mode | No accidental execution or loss of legitimate data. |
| Rejected controls, malformed path or oversized body | Clear refusal and unchanged destination. |
| Patch backend unavailable, applicability failure or source changed after preflight | Explicit outcome and no false exact-save claim. |

Capture the admitted payload and inspect persisted bytes. Then qualify patch application using a deterministic fixture and the supported backend. Check both expected final content and the changed-path set. UI success text alone is insufficient. Keep parser fixtures independent of a model; only a later user-journey study should estimate reduced repair conversations or agent turns.

## 7 Costs and checkable exit

The immediate objective is correctness, not a claimed token reduction. Potentially avoided repair conversations and rereads are hypotheses requiring measurement.

Compare raw-suffix parsing, explicit multiline entry and removal of the unsafe inline workflow. Account for parsing/allocation cost, payload bounds, UI complexity, migration of existing command syntax, diagnostic clarity and human recovery effort. Keeping raw and display-safe representations can require extra memory; bound that deliberately without silent clipping. Backend preflight adds latency and IO and does not remove the need for coherent effect handling.

Measure exact-byte preservation, refused-input behavior, user correction steps, UI latency, temporary bytes, backend attempts and end-to-end completion. Any claimed token benefit must include extra explanations, rereads, retries and verification rather than count only a shorter successful command. A smaller prompt that writes the wrong literal is not an efficiency improvement.

Proposed checkable exits for the owning CLI spec:

| Capability | Exit |
|---|---|
| Literal file payload transport | The real slash workflow preserves every supported payload fixture and refuses unsupported representations before mutation. |
| Lossless patch ingestion | Line-by-line and chunked fixtures preserve the admitted patch representation, including context markers and trailing whitespace. |
| Honest effect reporting | Successful reports agree with persisted bytes and changed paths; failure does not claim exact completion. |
| Feature regression preservation | Partial edit reads still refuse replacement; whole-file reads do not bypass payload fidelity. |

**Completion criterion:** a person can supply supported file content or a supported patch through the advertised TUI workflow and either obtain the declared exact effect or receive an explicit refusal before mutation. Source integrity must survive the complete input pipeline, not merely the read helper or final filesystem call.

## Source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star
[precondition-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/edit-observation-precondition-continuity.md
[receipt-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/tool-effect-receipts-and-validation-completeness.md
[search-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/workspace-search-navigation-fidelity.md
[evidence-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/tui-evidence-turn-ownership.md
[session-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/chat-session-memory-ownership-and-recovery.md
[coercion-study]: https://github.com/theRebelliousNerd/codenerd/blob/9a1a91286370674eec26f03ac2184bb6c9ad2658/Docs/audits/tool-arg-coercion.md
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/command_categories.go#L145-L178
[submission]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L104-L143
[dispatcher]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands.go#L42-L52
[write-handler]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_analysis.go#L349-L377
[append-handler]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_analysis.go#L478-L506
[writer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_files.go#L307-L313
[appender]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_files.go#L36-L48
[edit-handler]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_analysis.go#L438-L475
[main-dispatcher]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/commands.go#L45-L52
[main-write-handler]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/commands_handlers_analysis.go#L349-L377
[patch-entry]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_analysis.go#L422-L435
[main-submission]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L104-L135
[patch-backend]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_files.go#L51-L80
[sanitizer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_helpers.go#L21-L40
[sanitizer-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_helpers_test.go#L236-L241
[gnu-unified]: https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html
[git-apply]: https://git-scm.com/docs/git-apply

## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212`. Every finding was still present on main; the conversation-brief merge did not touch this route.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| P1: `handleSubmit` trims the submission before dispatch | Real, fixed | Was `model_handlers.go:106`; now the raw value reaches `handleCommand` with only leading whitespace dropped (`model_handlers.go:106`, `:128`) |
| P1: `handleCommand` runs `strings.Fields` over the whole command; `/write` and `/append` rebuild content with `strings.Join(parts[2:], " ")` | Real, fixed | `handleCommand` keeps `raw` for the two payload commands (`commands.go:45`, `:146`, `:158`); `splitPayloadCommand` parses only verb, path and one separator (`command_payload.go:31`) |
| P1: success is reported without disclosing the rewrite | Real, fixed | Replies now state the bytes written (`commands_handlers_analysis.go:352`, `:474`) |
| P1: quoted JSON spans are also collapsed | Real, fixed | `TestSlashWrite_SubmissionReachesTheFileByteForByte/json_string_spacing` |
| `/edit` refuses a partial read (feature-branch fix) | Confirmed present on main, kept | `commands_handlers_analysis.go` `handleCmdEdit` (`coverageSpan` branch) |
| `/edit` whole-file branch sends the person to `/write` | Real; now a lossless route except tabs (below), and the hint says how to send multiline content | `commands_handlers_analysis.go:461` |
| P2: patch mode trims each submission, drops empty submissions, joins trimmed strings | Real, fixed | `handleSubmit` hands patch mode the raw value before any trim or empty check (`model_handlers.go:108-110`); `patchChunk` drops exactly one trailing line ending (`command_payload.go:74`) |
| P2: the context marker of ` unchanged` and of a blank context line is lost | Real, fixed | `TestPatchMode_LineByLineKeepsContextMarkersAndApplies` |
| Backend: `git apply --whitespace=nowarn`, fallback `patch -p0` | Confirmed. Not in the audit: the backend ran in the process working directory, not the workspace, and a failed `git apply` was reported as "git apply and patch both unavailable" with its output discarded | `runApplyPatch(workspace, ...)` sets `cmd.Dir` and returns git's and patch's own output (`helpers_files.go:71`) |
| Sanitizer keeps `\n`, `\r`, `\t` | Confirmed; it still guards command recognition only | `model_helpers.go:25` |
| Not in the audit: the input widget rewrites content before `handleSubmit` | Real, partly addressed. The bubbles v1.0.0 textarea expands tabs to four spaces, turns each CR into a newline and drops controls on every insert (`runeutil.Sanitizer`, not configurable: `rsan` is unexported), and cuts a paste at `CharLimit` (100,000, `session.go:69`) or 10,000 lines without notice | A cut paste now produces a notice and nothing is sent silently (`droppedInputNotice`, `command_payload.go`; `model_key_handler.go`). Tab expansion is not fixable in bubbles v1; see left open |

### What was built

- Rulings: (1) the envelope is `<verb> <path><sep><payload>` with `<path>` one whitespace-free token and `<sep>` exactly one space, tab, `\n` or `\r\n`; no path quoting. (2) An empty payload is a usage error, so a stray trailing space cannot truncate a file. (3) Control characters other than tab/CR/LF (and DEL) are refused before the file is opened, not stripped. (4) `/append` is a line append: payload bytes, then `\n` unless already present. (5) Patch mode ends only on a whole submission of exactly `--END--`; a multi-line chunk is always data, so a removal line `--END--` can be pasted. (6) The `*** Begin Patch` wrapper is kept: git apply and patch both skip it as preamble.
- `cmd/nerd/chat/command_payload.go`: `splitPayloadCommand`, `checkTextPayload`, `patchChunk`, `ingestPatchSubmission`, `droppedInputNotice`.
- Registry descriptions for `/write` and `/append` state the byte-for-byte contract (`command_categories.go`).
- Spec: `Docs/architecture/cli/03-GAP-ANALYSIS.md` `GAP-CLI-SLASH-PAYLOAD`.

### Tests

`cmd/nerd/chat/command_payload_test.go`: repeated spaces, JSON spacing, trailing whitespace and multiline content reach the file exactly through `handleSubmit`; tabs and CRLF exact at the dispatcher; a control byte, an empty and a missing payload leave an existing file unchanged and are not reported as writes; append newline rule; a line-by-line patch with context markers, a blank context line and trailing spaces applies to the workspace while the test's working directory is elsewhere; chunked and line-by-line ingestion admit the same patch; a terminator with no lines applies nothing; a paste beyond the limit is reported. The existing patch-mode e2e and command tests pass; `go test ./cmd/nerd/chat/` passes.

### Left open

- Tab transport through the input box: bubbles v1.0.0 offers no sanitizer option; keeping tabs needs the bubbles v2 / bubbletea v2 migration (a repository-wide TUI change, not this lane) or a raw paste shadow buffer. Dispatcher and writer are exact, so the fix is confined to the widget.
- Section 6 rows not built: concurrent source change after a `git apply --check` preflight (no preflight was added; the backend's own atomic check-then-apply is relied on) and an oversized-body bound (the widget's limit is the bound and is now disclosed).
- `patch -p0` fallback does not match git's `a/`/`b/` prefixes; it only runs after git apply fails, and both failures are now reported.
- Path containment for `/write` and `/append` is unchanged: these are the person's own commands and absolute paths are accepted, as before.
