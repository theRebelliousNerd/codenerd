---
doc-class: deep-dive
subsystem: browser-evidence
implementation-status: proposed
last-verified: 2026-11-02
verified-against: 95d2b28ac68163984fd1bec57b52085c789531a9
verification-method: static-source-inspection
supersedes: []
---

# Browser evidence URL userinfo redaction

## Decision

Make URL authority redaction a structural part of the browser evidence projection, independent of query parameters and credential-shaped text. Keep the execution locator distinct from its safe display and persistence representation. Preserve browser admission, manager ownership, live-kernel integration, bounded evidence storage and exact sanitized-fact retirement.

The concrete residual gap is small but crosses several intended evidence boundaries. The browser redactor removes sensitive query values, authorization-shaped text and common assignments, but does not remove the parsed URL userinfo component. A credential-bearing requested URL can consequently survive `SanitizeForEvidence`, reach the arguments of browser logging calls, and be returned by `browser_navigate` after that tool succeeds. Successful progressive navigation also has an original-request-derived action-evidence path into the flight recorder. Neither conclusion requires Chrome to retain credentials in its reported document URL.

**Evidence status:** this is a source-derived, conditional diagnosis, not a reproduced incident, credential-validity finding, browser exploit, unauthorized disclosure or measured failure rate. All example credentials below are fabricated markers. No browser navigation, codeNERD execution, provider inference, tests or benchmarks were run. No repository files were changed; this document is a scratch-only proposal.

## Source freeze and distinct scope

All implementation links below are pinned to main [95d2b28](https://github.com/theRebelliousNerd/codenerd/commit/95d2b28ac68163984fd1bec57b52085c789531a9). The inspected [browser guidance](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/agents.md#L19-L31) explicitly puts sanitization before facts and evidence sinks, alongside private permissions and writable-root confinement. The [research guidance](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/agents.md#L15-L40) likewise requires sanitization before bounded extraction and preserves the bound runtime's ownership.

This is not a reopening of the repaired [research browser ownership and evidence retirement audit](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/Docs/audits/research-browser-ownership-and-evidence-retirement.md#L14-L67): runtime leases, inherited redaction configuration, cancellation and exact sanitized retraction keys remain valuable. Nor is it the [research observation identity and admission audit](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/Docs/audits/research-observation-identity-and-admission.md#L12-L38), which concerns acquisition meaning and downstream source identity. This report concerns which URL components a browser evidence sanitizer removes, with a direct requested-URL witness. Existing research link projection already handles userinfo separately, as discussed below.

## The production route and its limits

Registration alone is not permission to execute a tool. The trace below starts only when the effective tool catalog and host authorization admit the requested operation.

| Boundary | Verified behavior |
|---|---|
| Boot wiring | The system factory copies operator browser settings, extra sensitive keys, workspace and evidence configuration into a manager backed by the live kernel, attaches its fact querier/retractor, and binds it through `research.SetBrowserRuntime`. [factory.go 2023–2069](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/system/factory.go#L2023-L2069). |
| Registered tools | Research registration includes `browser_navigate`, `browser_act` and `browser_evidence`. [register.go 8–48](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/register.go#L8-L48). |
| Bound ownership | Browser tools use the boot-bound interactive manager; `requireBoundBrowser` returns an error rather than constructing a fallback when none exists. [browser_runtime.go 106–129](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/browser_runtime.go#L106-L129). |
| Sanitization | The manager constructs `security.NewRedactor(cfg.ExtraSensitiveKeys)`; `SanitizeForEvidence` delegates to its `SanitizeString`. [session_manager.go 400–424](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/session_manager.go#L400-L424), [489–492](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/session_manager.go#L489-L492). |
| Requested URL projection | `executeBrowserNavigate` computes `safeURL` from the original `args["url"]`, supplies it to a debug log before browser start, and uses the same value in the success log and returned success string. [browser.go 44–89](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/browser.go#L44-L89). |
| Actual navigation | Existing-session navigation checks startup/session availability, invalidates element refs before attempting the move, uses the caller context and configured navigation timeout, and waits for document parsing on success. Metadata prefers the browser-reported URL when available. [session_manager.go 885–938](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/session_manager.go#L885-L938). |

The distinction between requested and observed URLs matters. Chrome might reject userinfo, normalize it away, or report a different document URL. This report does not establish its behavior. The tool's returned `safeURL` is not replaced by that observed URL. For the simplest success witness, use the existing-session branch and condition on `Start`, `Navigate` and `GetSession` succeeding. The earlier logging call receives `safeURL` even if startup later fails, but whether a configured logger emits or retains that message is a separate condition.

## Why the current redactor leaves this component intact

The implementation performs these operations in order:

1. Apply `RedactURL` to the whole input string.
2. Apply it again to embedded HTTP or HTTPS URL matches.
3. Replace bearer/basic authorization-shaped values.
4. Replace JWT-shaped values.
5. Replace values after selected secret-assignment labels.

[Patterns and sequence](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/security/redactor.go#L17-L78).

However, `RedactURL` returns the raw string immediately when parsing fails **or when `RawQuery` is empty**. Otherwise it only changes query values whose keys match the sensitive-key policy. It never inspects or clears `parsed.User`. When a sensitive query is changed, `parsed.String()` therefore still serializes any userinfo. [redactor.go 81–107](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/security/redactor.go#L81-L107).

Consider this fabricated input, as a static example only:

```text
https://reader:orchid@example.test/path
```

Its URL has a userinfo component and no query. Both URL passes return it unchanged. Neither `reader` nor `orchid` is an authorization prefix, JWT or selected assignment label, so the later text passes do not repair the omission. Putting the same URL inside ordinary text invokes the same embedded-URL transformation. Adding a sensitive query such as `?token=synthetic-marker` can redact that query value without removing `reader:orchid@`.

This is not evidence that all credential-shaped text survives. Recognized query keys, bearer/basic tokens, JWT patterns, sensitive field names and input descriptors have working redaction paths. Some particular userinfo strings may be incidentally altered by the later regular expressions; that does not establish component coverage. Adding the already-known key `password` cannot make the sanitizer recognize arbitrary data in `URL.User`.

Recursive evidence sanitization does not close the gap: it copies maps, slices and structs, masks sensitive field names, and applies `SanitizeString` to other string leaves. The ordinary `url` field is not a default sensitive key. A custom policy that masks an enclosing field can suppress a particular structured occurrence, but it does not change direct `SanitizeForEvidence(string)` handling. [Default keys](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/security/redactor.go#L29-L59), [recursive handling](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/security/redactor.go#L138-L198).

## A separate original request path into recorded evidence

Progressive actions make the persistence claim precise without assuming anything about CDP URL normalization:

1. For a nonempty `navigate` operation on a selected session, `ExecuteActions` calls `m.Navigate` with `operation.URL`. Its result map contains `m.SanitizeForEvidence(operation.URL)`. If navigation succeeds, its portable action also contains that original requested URL after the same sanitizer. [progressive_action.go 109–119](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/progressive_action.go#L109-L119).
2. A successful portable action is passed to `recordActionIntent`, which records an `action_intent` event when a recorder and session ID exist. [268–279](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/progressive_action.go#L268-L279), [495–500](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/progressive_action.go#L495-L500).
3. `FlightRecorder.Record` recursively sanitizes the event data, marshals it, enforces its row limit and appends it to a private evidence file. Repeating the same sanitizer does not remove this userinfo. [flight_recorder.go 95–163](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/flight_recorder.go#L95-L163).
4. `browser_evidence read` returns selected stored event bodies through JSON serialization. Read ceilings, filtering and retention determine which events are returned. [flight_recorder.go 192–246](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/flight_recorder.go#L192-L246), [browser_evidence.go 53–68](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/browser_evidence.go#L53-L68).

The tool also records an `act` event containing the unabridged execution results. **Default compact output removes each step's `Result`; summary output omits the result list.** Only a full view that includes the relevant page directly returns that result URL. The recorder call still receives `execution.Results` independently of these display reductions. [browser_progressive.go 113–166](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/browser_progressive.go#L113-L166).

Persistence therefore requires enabled and successfully constructed recording, a valid session, a fitting event row, successful filesystem operations and retention of the record. Oversized data can be replaced by a summary/hash; recorder errors are ignored by the action-intent helper, so tool success alone is not proof of a durable row. This report makes no crash-durability guarantee and no claim that an unauthorized party can read owner-only files.

## Existing safeguards and important counterevidence

- **Navigation authority is unchanged.** An evidence redaction omission does not bypass tool approval, session checks, browser policy or host permissions. No synthetic URL was dereferenced. Credential validity, authentication behavior, network transmission and access to another user's data remain unestablished.
- **Browser-observed facts are conditional surfaces.** Fact publication sanitizes string arguments before recording and sending them to the live sink. If a producer supplies userinfo, the same gap applies; this does not establish that Chrome emits userinfo in network or document facts. Preserve the exact sanitized-key retirement fix rather than changing fact identity after tracking. [fact_redaction.go 8–36](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/fact_redaction.go#L8-L36).
- **Private and bounded evidence remains private and bounded.** Recorder construction resolves its directory through the path policy and makes it private. Appending checks existing-file privacy and protects new files; bounded reads and rotation still constrain retention. Those controls reduce access and volume, not the meaning of a surviving string. [flight_recorder.go 70–163](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/flight_recorder.go#L70-L163).
- **Default headless research is not the same recorder configuration.** `researchHeadlessConfig` disables evidence recording, clears workspace/session-store settings, disables header ingestion and inherits extra sensitive keys. Do not generalize the interactive action-recorder witness to every default `web_fetch` read. [browser_runtime.go 329–357](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/browser_runtime.go#L329-L357).
- **Research link projection already removes userinfo.** For successfully parsed links, `web_fetch.resolveLink` resolves against the base and sets `ref.User = nil`. Its redirect notice also clears userinfo from a successfully parsed document URL. These are distinct working controls, not callers of the deficient browser helper. Malformed links have a raw fallback, and the notice has other input fields; neither fact licenses claiming every research output is sanitized. [web_fetch.go 340–365](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/tools/research/web_fetch.go#L340-L365).
- **The session-log URL redactor is different.** It replaces `u.User` and returns a redacted sentinel on parse failure, in addition to query treatment. It is not the implementation called by browser `SanitizeForEvidence`. This is evidence of an existing local design, not proof that all browser output later traverses it. [sessionlog/redact.go 51–75](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/sessionlog/redact.go#L51-L75).
- **Form credentials already have another control.** Portable input recording replaces sensitive values with environment-variable references. Preserve that behavior; URL authority credentials are a different component. [progressive_action.go 457–466](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/progressive_action.go#L457-L466).

## The contract to implement

URI authority has a userinfo subcomponent separate from host, path and query. RFC 3986 deprecates the user-and-password form and discourages clear-text display/storage of password-bearing userinfo. RFC 9110 further prohibits generating userinfo in HTTP(S) message target URIs or field values and recommends treating its presence in untrusted references as an error. These are reasons to define structural handling; they do not establish what this installed browser sends on the wire. [RFC 3986 sections 3.2 and 3.2.1](https://www.rfc-editor.org/rfc/rfc3986.html#section-3.2), [RFC 9110 section 4.2.4](https://www.rfc-editor.org/rfc/rfc9110.html#section-4.2.4).

Proposed invariants:

1. **Evidence URL projection removes all userinfo by default.** Remove it or replace the entire component with a stable nonsecret marker before any no-query fast return. Decide explicitly whether usernames may ever be retained. Go's `URL.Redacted()` masks only the password in `u.User`; it is not an all-userinfo or query-secret sanitizer. [Go net/url](https://pkg.go.dev/net/url#URL.Redacted).
2. **Execution and evidence are different values.** A sanitizer must not accidentally rewrite the locator passed to navigation. Separately decide whether ingress should reject HTTP(S) userinfo altogether; rejection is a compatibility/authority decision, not a substitute for sanitizing error and diagnostic output.
3. **Parse failure cannot certify a URL as safe.** In typed URL evidence fields, prefer a bounded nonsecret invalid-URL representation over the raw value. Mixed prose needs a separate extraction/fallback contract so malformed URL-like tokens cannot simply be echoed, without replacing all ordinary prose whenever whole-string URL parsing fails. Never put a raw parse error containing the input into the fallback.
4. **Encoding and punctuation are parsed, not guessed.** Cover encoded delimiters, username-only and empty-password forms, IPv6 hosts, ports, relative references and embedded URLs. Do not split blindly on `@`, decode an entire string before component parsing, or mistake an email address/path character for authority userinfo. Preserve nonsecret host and path meaning and the existing query-key policy.
5. **Sanitization is stable and precedes persistence.** Applying it twice should not reintroduce credentials or continually change the display. Fixing future writes does not remove old files, archives, exports or already-delivered output; any cleanup requires a separate bounded retention decision and authorization.

The minimal repair belongs in the common browser URL projection, with caller contracts checked at typed URL and mixed-text boundaries. Reusing semantics from the session-log helper may reduce divergence, but wholesale substitution would need review of differing key policies, malformed-input behavior and output conventions.

## Costs and failure modes

Structural userinfo removal does not require another model request, browser navigation or network round trip. The existing path already parses URLs and scans text; component handling adds implementation and regression work rather than a demonstrated latency win. Measure CPU, allocations and output length on realistic evidence sizes instead of assuming the added work is free.

Removing usernames sacrifices some diagnostic differentiation. Rejecting credential-bearing navigation could break legacy workflows; keeping raw execution input requires careful confinement even when display copies are safe. Aggressive malformed-token fallback can conceal useful diagnostics or over-redact prose. Re-serialization can alter escaping or query ordering, so evidence identity and tests must distinguish safe semantic preservation from byte-for-byte equality. The existing exact sanitized fact tracking must continue to match what was published.

A shared redactor reduces policy drift but increases the blast radius of a mistake. Changing only returned output leaves recorder writes and log arguments exposed; changing only persistence leaves direct responses untouched. Read-time redaction can reduce future display of historical rows but cannot undo old exports and is not a substitute for pre-write handling. Cleanup, compatibility guidance and migration checks are real costs. None of these changes proves arbitrary secrets in page bodies, paths or fragments have been detected.

## Fair evaluation proposal

**All checks below are proposed and unrun.** The inspected [redactor tests](https://github.com/theRebelliousNerd/codenerd/blob/95d2b28ac68163984fd1bec57b52085c789531a9/internal/browser/security/redactor_test.go#L8-L72) cover query, embedded-query, authorization, assignment, JWT, recursive-input and custom-key behavior; their displayed cases do not establish URL-userinfo coverage.

| Check | Fair control and observable outcome |
|---|---|
| Component matrix | Use synthetic distinct markers for username/password; test queryless, sensitive-query, benign-query, encoded, username-only, empty-password, IPv6/port, relative and malformed forms. Compare against the same ordinary URLs without userinfo. Assert marker absence and retained nonsecret identity, not merely the presence of a redaction marker. |
| Existing policy regression | Keep current query/header/JWT/assignment/input tests and custom keys. Include benign `@` outside authority and prose punctuation. Reject a repair that “passes” by dropping every URL or all evidence. |
| String and recursive paths | Exercise whole URL, embedded URL, map/struct URL fields and repeated sanitization. Compare idempotence and ensure strings are sanitized before truncation or serialization. |
| Requested versus observed URL | In a bounded host fixture, let the browser seam report a credential-free actual URL while the requested URL carries synthetic userinfo. Verify navigation success output and logging arguments use the safe projection. Separately exercise startup/navigation failure and unbound-manager rejection. Do not count a refused navigation as proof of successful-output sanitization. |
| Progressive recording | With recording enabled and a small retained event, inspect successful `action_intent` and `act` rows, full-view output and evidence readback. Preserve the fact that compact output omits `Result`. Repeat with recording disabled, filesystem failure, oversized rows and retention limits to avoid confusing absent data with successful redaction. |
| Real integration if later authorized | Use a controlled fixture and fabricated markers only. Observe the actual boot-bound route, not an isolated helper alone. Record browser acceptance separately from evidence projection; do not authenticate to a real account or infer remote transmission from a returned string. |
| Cost and utility | Compare identical workloads, evidence sizes, modes and retention settings before/after. Report redaction coverage, false positives, useful nonsecret URL detail, allocations/CPU and bytes, including errors and dropped/truncated cases. Do not substitute model quality or a claimed security/performance win for these measurements. |

The acceptance condition is narrow and checkable: supported browser evidence paths must not retain URL userinfo merely because a query is absent or its labels are unfamiliar, while existing authority, lifecycle, privacy, boundedness and nonsecret diagnostic behavior remain intact.

