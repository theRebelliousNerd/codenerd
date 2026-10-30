# Read continuation must identify the first unread position

## Finding and scope

The TUI `/read` command discloses its 16,000-byte body cap, but the continuation it prints can skip content that was never supplied. The reader records the line at which truncation occurred as `last`, even if it returned only part of that line or none of it. The handler then instructs the user to resume at `last + 1`. A user following that instruction can therefore miss an unread line suffix or an entire line.

The narrow repair is to make the displayed continuation describe the **first unread position**, not the next line after the last line the reader considered. Preserve the existing cap, range selection, containment, explicit partial disclosure, and answering-turn handoff. A whole-line strategy can repair ordinary short-line pagination, but it needs an explicit policy for a line larger than the page budget; moving the hint back one line alone does not make that case resumable.

This report is pinned to public main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), revalidated on 2026-10-30 at 09:42 UTC. The evidence is static source and test-definition inspection. No codeNERD execution, test, build, inference, benchmark, or failure injection was performed. Examples below are source-derived witnesses, not reproduced incidents or measured answer-quality failures.

## Continuation of the existing evidence audit

The [TUI evidence turn ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/tui-evidence-turn-ownership.md#L248-L266) records real improvements: typed whole, partial, and unavailable observations; bounded residual reads; actual body delivery into articulation; and a workspace-evidence route when no worker was selected. It expressly leaves multipage continuation, shared observation handles, and revision checks open. This report refines that open boundary with a concrete defect in the existing `/read` continuation advice; it does not claim the broader need was previously unidentified.

The slash command also already fixes earlier behavior that silently clipped a file, presented it as assistant prose, ignored the documented range, and did not start an answering turn. The current handler records the supplied contents as user evidence and schedules `processInput` on the explicit question or an earlier-question fallback. Those are capabilities to retain, not defects to report again. [Handler history and current handoff](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L223-L296).

The residual evidence renderer still recommends `/read` with a line range for the rest of a partial file. Thus the command is a recovery affordance as well as a direct input route. This audit's primary witness uses the command directly; it does not assume a model will select it or that every natural-language file request reaches this path. [Partial-evidence rendering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/turn_evidence.go#L181-L194).

## Production path and the mismatched meanings of last

Ordinary slash submission enters `handleCommand`, whose `/read` arm invokes `handleCmdRead`. The handler accepts a path, optional line range, and optional question. It passes a fixed `maxReadBytes = 16000` to `readFileRange`. No classifier decision is required to reach the file read. [Submission dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_handlers.go#L109-L132), [command arm](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands.go#L139-L140), [range and cap](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L247-L260).

The helper resolves the path through `tools.ResolveWorkspacePath`, opens it, and reads newline-delimited chunks. For an in-range chunk that would exceed the cap, it computes `remaining = maxBytes - out.Len()`. If `remaining > 0`, it appends only that prefix. Whether or not any bytes were appended, it sets `truncated = true` and `last = lineNo`. It continues reading to count the file's total lines. [Reader and truncation branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_files.go#L106-L154).

The handler uses `last` in two different claims: the supplied range header says `lines first-last`, and the partial notice says the range continues past `last`, with a command starting at `last + 1`. That assumes the reader supplied all of line `last`. The truncation branch guarantees no such thing. [Header and continuation construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L272-L280).

The distinction is not cosmetic. The body and generated advice enter the same user-evidence message, and the command schedules an answering turn. A later `/read` executes the displayed range literally; it has no byte offset or retained partial-line cursor from the earlier call. [Evidence and answering turn](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L277-L296), [line-range parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L200-L220).

## Exact boundary witness with only short lines

Assume a readable, unchanged file inside the workspace and ordinary idle command submission. Construct its logical contents as follows; this describes a future fixture and was not executed:

| Lines | Contents | Relevant size |
|---|---|---|
| 1 through 2,000 | `aaaaaaa` followed by LF on each line | 2,000 × 8 = 16,000 bytes |
| 2,001 | `MISSING_SENTINEL` followed by LF | Shorter than the page cap |
| 2,002 | `after` followed by LF | Shorter than the page cap |

Submit `/read file.log`. After line 2,000 the output builder contains exactly 16,000 bytes. Reading line 2,001 takes the overflow branch with zero remaining capacity. No byte of that line is appended, but `last` becomes 2,001. The final total is 2,002 lines.

The first evidence body therefore contains only lines 1 through 2,000. Nevertheless, its header names lines 1 through 2,001, and its continuation is `/read file.log 2002:2002`. Following that command returns the final line, not the omitted sentinel. The two advertised pages do not cover the source even though every line is small. This follows directly from the [zero-remaining branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_files.go#L133-L144) and [next-line hint](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L272-L275).

When capacity instead runs out inside a line, the first page includes that line's prefix and the hint skips its suffix. A line longer than 16,000 bytes exposes a stronger usability limit: asking `/read` to start at that line again starts from the same line beginning and applies the same cap. A line-only retry cannot advance through the suffix. Other tools or manually chosen short-line ranges may recover missing content; this is not a claim that the underlying file was damaged or that all access paths are incapable of reading it.

## Controls and limits to preserve

- **Partial disclosure exists.** The command says it was truncated. The defect is the inaccurate continuation position and range claim, not an absence of every warning.
- **Containment remains enforced.** The helper uses the shared workspace resolver before opening the file. Repairing pagination must not create a cursor that bypasses path authorization or opens an arbitrary external path. [Resolution and open](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_files.go#L106-L119).
- **Text boundary handling exists.** After building the body, the helper trims until it is valid UTF-8. Preserve valid-rune boundaries, and ensure any new cursor points after the bytes actually returned, not after a larger prefix that was subsequently trimmed. [Final body handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_files.go#L155-L163).
- **Useful current evidence is retained.** The first page is supplied and an answering turn is scheduled. The model may honestly qualify its answer or obtain another observation. The source does not prove an incorrect final answer, lost persisted data, or a permission bypass.
- **No source-change race is needed.** The witness holds for an unchanged file. Cross-page revision checks are a related open contract, not a prerequisite for this failure.

This helper uses **`bufio.Reader.ReadBytes('\n')`, not `bufio.Scanner`**. There is no Scanner default-token-limit explanation for this result. The 256 KiB reader buffer is not a maximum line size; the source asks for the whole newline-delimited chunk before clipping what it returns. The loop also continues to EOF for `totalLines`. Consequently, 16,000 is a returned-body cap, not a total filesystem-read, longest-line-allocation, or full model-request budget. Do not “repair” this report by increasing a nonexistent Scanner token limit. [Actual reader loop](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_files.go#L125-L154).

## Repair options and recommended contract

### Whole line pages with an explicit large line outcome

For ordinary lines, append a line only when it fits in full. Track the last fully returned line separately from the first unread line. A full page followed by another line must advertise that unread line, not advance past it. If the next line fits in an empty page, retrying from its beginning makes progress without cutting it.

A line larger than the cap needs a separate outcome. Either offer a supported byte/chunk continuation or return an explicit oversized-line limitation and a real alternative. Do not emit the same no-progress line-range retry indefinitely, silently drop the remainder, or raise the cap without a bound. This option is a small compatibility repair for common logs; without the large-line mechanism it is not complete bounded resumption for arbitrary text.

### Byte position continuation

A byte-oriented continuation can return a bounded prefix of a large line and name the exact next unread byte in the same source. Its display should distinguish complete lines from a partial line, while its transport uses an unambiguous offset. For valid UTF-8, the returned end and next offset must account for rune-boundary trimming. A page that returns no bytes cannot claim successful progress unless it reports a terminal or blocked outcome.

The cursor should retain the requested range and source identity. If the underlying source changes, reject or explicitly restart/rebase according to a documented policy rather than concatenating incompatible snapshots as one complete observation. Preserve containment when redeeming it. A content digest, retained snapshot, or existing observation handle can supply identity, but each has costs; the immediate short-line repair need not build a new archival system.

### Reuse the existing observation protocol where practical

The prior evidence audit already points to shared observation handles and revision checks as the larger integration direction. Prefer one established coverage/cursor representation over competing ad hoc protocols if it can support the slash command without changing its useful “supply evidence and answer” interaction. This is a proposal, not a claim that the slash helper currently uses that protocol. [Prior open scope](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/tui-evidence-turn-ownership.md#L260-L266).

The minimum common contract is:

1. Returned span metadata describes bytes actually supplied, including whether the last line is partial.
2. The next position is the first unread position within the requested source and range.
3. Every admitted continuation either advances coverage or reports an explicit terminal, stale, unavailable, or blocked outcome.
4. Following the advertised sequence on an unchanged source leaves no unreported gaps. Deliberate overlap is identified rather than mistaken for additional source content.
5. Bounded projection and workspace authorization remain in force; coverage does not imply permission to read a different source or write anything.

Document that contract in the affected CLI/evidence specifications before implementation, following the repository's [spec-first requirement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md#L92-L110). Do not close the broader revision-aware observation work solely because the next-line arithmetic is repaired.

## Evaluation through normal command entry

The existing `TestCommand_Read_RangeAndTruncationMarker` checks that `2:3` selects the requested lines and that a large-file response contains a truncation notice and a `/read` hint. It never redeems that hint. Those are useful tests of disclosure and range selection, but they cannot establish coverage conservation. Their definitions were inspected, not run. [Existing regression](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/read_command_test.go#L72-L103).

A future deterministic regression should submit the command through normal TUI dispatch, inspect the generated evidence message, follow its actual advertised continuation, and compare the ordered returned spans with the requested source. Do not compute a corrected cursor in the test independently of what the user receives. Also drive the scheduled answering turn with a controlled perception/provider dependency and confirm the relevant evidence and partial status reach its input; source coverage alone does not prove prompt delivery or answer correctness.

| Case | Required observation |
|---|---|
| The exact 16,000-byte short-line witness | The next page begins at the wholly unread sentinel line |
| Cap falls inside a short line | No suffix lost; either the full line moves to the next page or a precise partial-line cursor resumes it |
| One line exceeds the cap, with and without final LF | Finite progress through supported chunks, or an honest explicit limitation with no false retry promise |
| A valid multibyte rune crosses the boundary | No invalid returned text and no skipped rune bytes on resumption |
| CRLF, empty lines, empty file, exact EOF, and one byte over | Correct span, terminal status, and continuation behavior |
| Explicit start and end range | Continuation remains within that request; completion does not require reading unrelated trailing lines |
| Missing or outside-workspace path | Existing refusal/diagnostic behavior retained |
| Source replaced or edited between pages | Outcome matches the chosen source-identity contract; no silent mixed-source completeness claim |
| Repeated cursor, stale cursor, or empty-result page | No fabricated progress, skipped data, or endless identical recommendation |
| Healthy small file and explicit question | Existing user-evidence role and exactly the intended answering-turn handoff retained |

For multi-page comparisons, inspect source-position coverage, not simply concatenated rendered Markdown: fences, headers, deliberate overlap, and line terminators require an explicit comparison policy. Keep the byte budget fixed across baseline and treatments. A larger page or a whole-file read can hide the witness while weakening the resource contract.

After transport invariants hold, evaluate actual questions whose answers lie before, at, and after page boundaries. Include honest partial answers and requests for more evidence as legitimate outcomes. Measure unsupported completeness and incorrect answers separately from mere absence of the sentinel in one prompt. No behavioral improvement is claimed by the static trace.

## Costs and acceptance criteria

Whole-line packing can leave unused page space and increase the number of pages and user interactions. Byte cursors add parsing, metadata, source-validation, and error-handling complexity. Snapshot retention or content hashing adds storage or read work; an index can reduce repeated scanning but needs invalidation. Reusing a shared observation protocol has integration and compatibility costs even when it avoids duplicating semantics.

The present helper rescans and counts the file to EOF on every call. Multiple line-range pages can therefore reread the same file repeatedly, and `ReadBytes` can retain a large single-line chunk before returning a small prefix. A cursor alone does not remove those costs if the implementation still performs the same full count. Conversely, bounded chunk reading may need to report unknown total lines until they are measured. Make that tradeoff explicit rather than inventing a total.

Each successful `/read` also schedules an answering turn. More accurate pagination can increase immediate calls compared with skipping content. Fewer misleading answers and corrective retries are possible benefits to evaluate, not established results. Compare equivalent covered workloads, counting source bytes read, peak retained bytes, returned body/header bytes, page count, model calls and tokens, user interactions, end-to-end useful-answer latency, and task accuracy. Do not score a treatment as efficient because it silently supplies less evidence. No cost saving or reliability percentage has been measured here.

The narrow exit is that the exact-boundary and midline witnesses cannot lose content when the user follows the emitted continuation. The stronger exit adds bounded progress for large lines and a declared cross-page source-identity policy. Preserve partial disclosure and containment throughout, and record these exits separately from the broader evidence audit's still-open work.
