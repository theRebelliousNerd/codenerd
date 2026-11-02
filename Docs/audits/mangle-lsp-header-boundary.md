---
title: Mangle editor transport must recognize a complete LSP header block
subsystem: mangle-lsp-stdio
status: source-supported; not reproduced; proposed contract
source_commit: bc298237af7f3e4eae53026b25b6d8e69b33a603
review_date: 2026-11-02
---

# Mangle editor transport must recognize a complete LSP header block

## Finding and scope

The production `nerd mangle-lsp` reader treats the line after `Content-Length` as an unconditional separator. A valid `Content-Type` header in that position is consumed instead, leaving the actual separator in the body. With a compact JSON request, the parser reads the separator plus all but the last two body bytes, silently rejects the truncated JSON, and leaves those bytes in the stream. Subsequent valid frames can then be lost as well. This is a header-boundary defect, not a bad request, incorrect length supplied by the editor, or concurrent writer requirement. [Reader][server-read]

The supported impact is **editor interoperability, availability and document-state freshness**. An initialization request can receive no response; an already initialized session can lose document updates or diagnostic clearing. It is not evidence of a Mangle execution-policy bypass, remote code execution, lost source files, or a compromised coding-agent verdict. No particular editor's default header emission or real incident was established.

**Evidence status.** All codeNERD source links pin **`bc298237af7f3e4eae53026b25b6d8e69b33a603`**. Public source, existing test definitions, documentation and primary standards were inspected. No application, CLI/server process, test, build, benchmark, model call or failure injection was run. The examples below are static byte derivations, not execution receipts. Only this standalone Markdown draft was written.

## Intended boundary and actual production path

The repository asks for deterministic grounding and validation through real entry paths, and distinguishes architectural intent from demonstrated wiring. Here the appropriate responsibility is a Go transport driver recognizing bytes before dispatch, not a model deciding which headers matter or new executive logic in Mangle. [Root guidance][root-guidance] [Architecture guidance][architecture-guidance]

The command is live:

1. The root command registers `mangleLSPCmd`; its command definition selects `runMangleLSP` and advertises JSON-RPC over standard input/output. [Registration][cli-register] [Command][cli-command]
2. The runner creates an LSP manager, creates a cancellation context, initializes the manager and calls `manager.ServeStdio(ctx)`. Initialization failure prevents reaching the transport; the witness assumes a normally indexable workspace and successful initialization. [Runner][cli-serve]
3. The manager constructs a Mangle engine and `mangle.NewLSPServer`, indexes the workspace, then delegates its stdio entry directly to that server. No intermediary normalizes inbound headers. [Initialization][manager-init] [Delegation][manager-serve]
4. `LSPServer.ServeStdio` binds a buffered reader to `os.Stdin`. Only after a successful JSON decode does it call `handleRequest`, write a response if present, and publish applicable document diagnostics. [Reader][server-read] [Dispatch][server-dispatch]

The neighboring `internal/world/lsp.Client` is a different direction of communication. Its `readFrame` loops until an empty header line, matching Content-Length case-insensitively. That is useful local design precedent but **does not protect this server**. The current manager explicitly distinguishes its editor-facing Mangle server from external language servers used directly by the critic. Older world-model projection prose is not needed to establish reachability. [Client reader][client-frame] [Current manager scope][manager-scope] [Wiring record][wiring-doc]

## Protocol contract

The official LSP 3.17 base protocol defines an ASCII header section, a CRLF separator, a required Content-Length measured in bytes, and a supported Content-Type whose default is the UTF-8 JSON-RPC media type. Content-Type is not a forbidden extra line. Header-field structure follows HTTP semantics; neither LSP nor those semantics require Content-Length to be the final field. The HTTP standard makes the relative order of differently named fields insignificant. These rules support both orders below, without invoking an unknown extension or unsupported encoding. [Official LSP base protocol][lsp-spec] [HTTP field order][http-order]

JSON-RPC distinguishes requests, which require correlated responses, from notifications, which do not. The repair must not invent success responses for document notifications merely to make loss visible. Valid framing should deliver their bodies; invalid framing needs a deliberate transport/error policy. [JSON-RPC request and notification contracts][jsonrpc]

## Static byte witness

Let `J` be this compact, UTF-8 initialization body, with no trailing whitespace, and let `N` be its byte length:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}
```

The following is a wire-layout notation, not a command or executed reproduction. Each `\r\n` denotes two actual bytes; the symbolic `N` is replaced by the correct decimal byte count.

```text
Content-Length: N\r\n
Content-Type: application/vscode-jsonrpc; charset=utf-8\r\n
\r\n
J
```

The code's state transitions are sufficient to locate the defect:

| Step | Actual consumption | Consequence |
|---|---|---|
| First `ReadString('\n')` | Content-Length line | Exact prefix matches; decimal N is accepted. |
| Unchecked “Skip blank line” read | Entire Content-Type line | The program never verifies that this was blank. |
| `io.ReadFull` into N bytes | `CR LF` followed by `J[0:N-2]` | The body is shifted; two final bytes of J remain unread. |
| `json.Unmarshal` | Leading whitespace plus an object missing its last two closing braces | Decode fails and `continue` silently bypasses dispatch. |
| Next header read | Starts at the leftover closing braces | It is no longer positioned at a frame boundary. |

These are byte positions, not rune counts. A complete correct sender frame already supplies enough bytes for the erroneous N-byte read, so the first loss does not depend on fragmentation, short reads, packet timing or an incorrect Content-Length. Go's documented `ReadString` and `ReadFull` behavior explains the consumption; buffering cannot repair the wrong starting position. [Reader implementation][server-read] [Go buffered reads][go-readstring] [Go exact-length reads][go-readfull]

Had the same body reached `handleRequest`, its initialize branch would produce a capabilities response with the request ID. In this layout neither that response nor any notification is generated because the decode fails earlier. The loop does not return an error for that failure. If the peer keeps stdin open waiting for the initialize response, the next line read can wait for additional bytes rather than producing a diagnostic. No universal editor timeout or permanent hang is asserted. [Initialize handler][initialize] [Decode and dispatch][server-read]

### Loss can cross a frame boundary

Suppose the next frame begins immediately with `Content-Length: M\r\n`. The reader sees one line beginning with the leftover `}}` from J, followed by that next length field. The prefix does not match, so the **next frame's actual length line is discarded too**. Its blank separator is skipped on another iteration. A subsequent compact body without a newline can then be read together with the following frame's first header line. A stream of compact, length-first frames can remain out of alignment; the code has no explicit resynchronization state or trustworthy marker search. [Loop branches][server-read]

This is conditional, not a claim that recovery is impossible:

- At a clean boundary, **Content-Type before Content-Length happens to work**: the first header is ignored, the next iteration recognizes length, and the subsequent blank line is consumed correctly. Ignoring the field's semantics is not full header validation, but it avoids this shift.
- If a body includes trailing CRLF in its counted length, the misread can contain leading CRLF plus the entire JSON object. JSON decoding can succeed; the leftover CRLF can then be skipped before the next length field. Thus “every Content-Type frame fails” is false.
- Two trailing spaces can likewise leave a decodable object, yet those spaces can prefix and spoil the next length line. Successful current dispatch alone is not proof of correct frame consumption.
- A newline in the remaining suffix, a later body newline, or a next frame with Content-Type before Content-Length can incidentally restore alignment. Such cases do not recover requests or notifications already discarded.
- With EOF after the remaining suffix, the header read takes the EOF branch and returns nil. Clean termination is therefore not a receipt that every complete sender frame was processed.

All of these follow from the same line-scan/prefix branches; no additional bug or concurrent execution is needed. They belong in negative controls so an evaluation cannot accidentally select an alignment-friendly fixture and declare the transport sound. [Framing loop][server-read]

## Consequences after initialization

The server's full-sync didOpen/didChange handlers call `OpenDocument`, which replaces the document and reindexes it. didClose removes it and clears file entries. Diagnostics are published only after successful decoding and handling of the relevant notification. Dropping one of those frames leaves the preceding state or publication intact, rather than applying the user's current document transition. [Synchronization handlers][sync-handlers] [Document state][document-state] [Diagnostic publication][diagnostics]

A concrete proposed freshness witness is: initialize successfully with length-only frames; open version 1; send a compact version-2 didChange with Content-Type after length; then send a query with Content-Type before length, allowing the scanner to regain alignment. The update is lost while the later query can dispatch against version 1. An existing indexed file may also have a disk-derived document from startup, so a dropped didOpen does **not** necessarily mean there is no document at all. [Startup indexing][workspace-index]

This does not prove a stale response from any named editor or a false clean diagnostic in every case. It proves the lost update has no state effect, and describes an unrun route to observing that loss. A lost exit notification can also fail to set the loop's shutdown flag; cancellation is checked only between reads, so repairing framing alone should not be sold as solving every blocked-I/O lifecycle issue. [Exit handler][exit] [Serve-loop checks][server-read]

## Prior work and nonduplication

| Screened material | Existing concern and distinction |
|---|---|
| Mangle LSP QA journal | Already identifies inadequate ServeStdio coverage and asks for malformed JSON/type tests. It does not identify Content-Type ordering, the unchecked separator, or valid-frame loss. Credit the test-gap observation; the new witness uses valid headers and a valid body. [Coverage discussion][qa-gap] [JSON/type discussion][qa-json] |
| LSP document-URI audit | Covers outbound critic-client URI identity and diagnostic correlation, plus terminal read-loop/error attribution. This report concerns inbound Mangle editor-server framing before a method sees the body. They do not share the defective reader. [Scope][uri-audit] [Residual terminal issue][uri-terminal] |
| Stream-completion audit | Already advocates proper SSE event framing and avoiding silent event omission. The transferable principle is acknowledged; SSE multi-line events and LLM completion are not this length-prefixed LSP ingress. [Earlier framing analysis][sse-audit] |
| Architecture, journey and audit records | Mangle wiring is documented. The external-audit routing record and general audit queue were screened; their status labels are not behavioral proof of this transport. No exact optional-header finding was located. [Wiring][wiring-doc] [Journey record][journey-audit] [Queue][audit-queue] |

The current basic tests exercise construction and open/close methods. The uplift test calls `handleRequest` and `notificationsFor` directly; it proves useful method behavior but never crosses the stdio parser. Do not repeat the QA journal's historical blanket statement that no handler coverage exists as a description of current code. [Basic tests][basic-tests] [Uplift test][uplift-tests]

Repository searches included Content-Length, Content-Type, ServeStdio, framing and LSP audit terms, followed by pinned source reads. This is a bounded nonduplication review, not an exhaustive claim about every branch, issue or historical discussion.

## Proposed contract and implementation direction — not built

**For each admitted frame, consume exactly its complete header section and exactly its declared body bytes once; dispatch only that body, and leave the reader at the next frame's first byte. Equivalent supported header orders must have equivalent behavior.**

A focused repair should:

1. **Separate framing from dispatch.** Add a reader over injected `io.Reader`/buffered input that reads the whole header block before body allocation. Keep `ServeStdio` as the production adapter, so tests can exercise the actual parser without replacing process-global stdin.
2. **Parse a header block, not a preferred layout.** Handle both supported fields, case-insensitive field names, the declared charset/default, and the terminating blank line. Preserve correct UTF-8 byte lengths; do not trim, reserialize or normalize body bytes to compensate for offsets.
3. **Define malformed-header and resource behavior.** Require an unambiguous valid length and select documented bounds for header bytes, header count and body allocation. Explicitly decide duplicate/conflicting length handling and unsupported charset errors. These are protective design requirements, not additional reproduced defects or reasons to reject the valid witness.
4. **Separate recoverable body errors from lost framing.** After a complete known-length frame, an invalid JSON body can receive the applicable protocol error and the next frame still has a known boundary. If length or header termination is untrustworthy, stop the transport with an attributed error rather than scan arbitrary body text for a header-looking substring.
5. **Keep observations safe.** Record a bounded error category and stage, not the whole document body. Propagate framing/read/write termination to the CLI; reserve stdout for framed protocol messages. Keep notification semantics and existing method handlers unchanged.
6. **Evaluate sharing, not duplication by copy-paste.** The generic client reader demonstrates the required header loop, but presently ignores Content-Type and lacks explicit bounds in that function. Extracting a common parser may reduce drift, yet needs parity for both directions and must avoid package dependency cycles. Reusing that helper unchanged is not a complete contract. [Client precedent][client-frame]

Header reordering on the client is at most a temporary workaround. Special-casing one extra read for Content-Type simply creates another fixed-layout assumption. Logging `json.Unmarshal` failures makes symptoms visible but does not restore body ownership. Retrying notifications is unsafe as a universal remedy and cannot replace reliable delivery within an established stream.

## Costs, tradeoffs and prioritization

The expected work is a small deterministic transport boundary plus integration tests, not a new model feature. For H header bytes and B body bytes, a single-pass parser should take O(H+B) time and bounded header storage plus O(B) body storage, similar in order to the existing allocation. Actual allocation count, buffering overhead and latency are unmeasured.

Explicit limits trade resource safety against large full-document updates; choose them from supported workloads and expose rejection, rather than silently truncate. A shared framing package adds ownership and API review across client/server consumers. Closing on untrustworthy framing sacrifices that session's availability, but avoids presenting arbitrary later bytes as valid messages. Injectable I/O and lifecycle-aware termination have implementation costs; they also avoid fragile tests that mutate global file descriptors.

Priority is warranted because a single legal optional field can prevent an editor session starting or silently omit a document transition. Severity remains an interoperability/freshness assessment, not a quantified security score. Default-client prevalence, user impact and repair performance require evidence not collected here.

## Fair evaluation plan — all unrun

Compare the pinned reader with the proposed parser using identical bodies, workspace content and handler semantics. First use deterministic in-memory framing; then exercise the real CLI entry with controlled stdin/stdout and a bounded test supervisor. Report actual bytes consumed, decoded request IDs, dispatch counts, stored document versions, published diagnostics, terminal errors and allocations—not merely whether a process exits.

| Case | Required oracle and purpose |
|---|---|
| Length-only initialization | One capabilities response; baseline compatibility preserved. |
| Identical initialization with Content-Type before/after length | Same body and response exactly once; next-frame position identical. |
| Compact affected frame followed by compact length-only frames | Every frame delivered once after repair; exposes cascading loss rather than just first decode failure. |
| Counted trailing CRLF, two spaces, pretty JSON and compact JSON | All valid encodings preserve boundaries; include current accidental-success controls. |
| Header/body split at every byte; several frames in one read | Chunking-independent result, including UTF-8 sequences split across reads. |
| Full-sync update followed by an alignment-friendly query | Version 2 is stored and queried, not version 1; headers must not change state semantics. |
| didClose after published diagnostics | Exactly the expected clearing publication; no invented notification response. |
| Missing/invalid/conflicting length, unsupported charset, excess limits | Documented bounded failure, no panic, no partial dispatch and no silent scan into later data. |
| Correctly framed invalid JSON followed by valid request | Chosen body-error response and intact next boundary; distinguish this from framing loss. |
| EOF within header/body, idle cancellation and output failure | Explicit lifecycle outcomes; no claim framing repair alone guarantees cancellation of blocking I/O. |
| Client reader sharing, if chosen | Existing external-client request correlation/diagnostic behavior remains unchanged. |

Use ordinary valid requests before malformed cases, unique IDs, finite supervision and independent expected byte counts. Include a small hand-authored wire fixture: an encoder paired with its own decoder can share the same layout assumption. Run the CLI path in a normal supported build environment; do not bypass initialization and then claim end-to-end compatibility. No performance or reliability gain should be reported until measured.

## Primary references and what they establish

- [Microsoft's official LSP 3.17 specification][lsp-spec] establishes the supported header/body format, not the emission frequency of any editor.
- [RFC 9110 §5.3][http-order] supplies the HTTP field-order semantics referenced by the LSP design; this audit does not import HTTP chunking or request routing into LSP.
- [Go `bufio.Reader.ReadString`][go-readstring] and [`io.ReadFull`][go-readfull] establish byte-consumption behavior, not a reproduction of codeNERD.
- [JSON-RPC 2.0][jsonrpc] supplies response/notification/error distinctions. It is not a stream resynchronization algorithm.
- Momot, Bratus, Hallberg and Patterson, [*The Seven Turrets of Babel*, IEEE SecDev 2016][langsec], distinguish incomplete input recognition and parser disagreement. The application here is architectural: recognize the entire frame before use rather than interleaving layout guesses and dispatch. The cited institutional publication record/abstract supports that framing; its security examples do not establish an exploit in this repository.

## Acceptance boundary

Close this finding only when valid supported header orders preserve byte boundaries and production dispatch, document transitions are not lost, malformed input has an explicit bounded outcome, and the normal editor command is covered. Passing direct handler tests, receiving one response from an alignment-friendly payload, or documenting a preferred sender order is insufficient.

[root-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/agents.md#L17-L29
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/architecture/AGENTS.md#L48-L74
[cli-register]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/cmd/nerd/main.go#L278-L297
[cli-command]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/cmd/nerd/cmd_mangle_lsp.go#L74-L105
[cli-serve]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/cmd/nerd/cmd_mangle_lsp.go#L123-L158
[manager-init]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/world/lsp/manager.go#L29-L64
[manager-serve]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/world/lsp/manager.go#L79-L92
[manager-scope]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/world/lsp/manager.go#L12-L18
[server-read]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L674-L724
[server-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L726-L742
[initialize]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L786-L804
[sync-handlers]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L806-L847
[document-state]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L135-L158
[diagnostics]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L744-L783
[exit]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L960-L971
[workspace-index]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp.go#L603-L645
[client-frame]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/world/lsp/client.go#L236-L264
[basic-tests]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp_test.go#L7-L100
[uplift-tests]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/internal/mangle/lsp_uplift_test.go#L104-L154
[qa-gap]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/.quality_assurance/lsp_qa_journal_20261010_002712.md#L6-L12
[qa-json]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/.quality_assurance/lsp_qa_journal_20261010_002712.md#L149-L165
[uri-audit]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/audits/lsp-document-uri-identity-and-diagnostic-grounding.md#L16-L44
[uri-terminal]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/audits/lsp-document-uri-identity-and-diagnostic-grounding.md#L114-L130
[sse-audit]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/audits/stream-completion-contract.md#L46-L60
[wiring-doc]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/architecture/mangle/WIRING-AND-NOT-BUILT.md#L66-L82
[journey-audit]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/journeys/07-external-audit-2026-09-19.md#L1-L25
[audit-queue]: https://github.com/theRebelliousNerd/codenerd/blob/bc298237af7f3e4eae53026b25b6d8e69b33a603/Docs/audits/AUDIT.md#L105-L118
[lsp-spec]: https://github.com/microsoft/language-server-protocol/blob/gh-pages/_specifications/lsp/3.17/specification.md#base-protocol
[http-order]: https://www.rfc-editor.org/rfc/rfc9110.html#section-5.3
[jsonrpc]: https://www.jsonrpc.org/specification
[go-readstring]: https://pkg.go.dev/bufio#Reader.ReadString
[go-readfull]: https://pkg.go.dev/io#ReadFull
[langsec]: https://iti.illinois.edu/credc/publications/seven-turrets-babel-taxonomy-langsec-errors-and-how-expunge-them

