---
implementation-status: proposed-refinement
verification-method: static-source-inspection
date: 2026-10-29
---

# Specialist roster workspace identity

## Finding and scope

The chat prompt can advertise a different specialist registry from the one its session uses. Interactive startup from a project subdirectory preserves that process working directory, while chat initialization discovers the canonical project root. The JIT compiler later resolves an unspecified roster workspace through `os.Getwd()`, not the session root. A missing subdirectory registry therefore produces a built-in-only listing even when custom specialists are registered and ready at the project root.

This is a source-supported prompt-discovery defect, not a measured consultation failure. Prompt impact requires successful chat need derivation, compilation, and admission of the stock knowledge-discovery atom within budget. It does not establish that every chat prompt is affected, that a model will miss a consultation, or that authorization or isolation has failed.

Code citations pin main commit `9dcb01b9736bf4763aa3ac391d29f133d73c6611`. The ten witness files also match the inspected Mangle, session, and OAuth heads listed below. Research head `0fa6fd9f52cc6338445a498895a2052699bcc3b9`, inspected at 18:09 UTC, adds documentation-extraction and system-execution documentation. No runtime execution, tests, model inference, or performance measurements were performed. The witness is a source prediction, not an observed incident.

## Source path to the prompt

Interactive `RunE` chooses the supplied workspace or current directory, then changes into that directory. Without a workspace flag, launching from `P/pkg` leaves the process there. Chat initialization separately calls `FindWorkspaceRoot` and stores the result in its model. That helper returns the nearest ancestor containing `go.mod`; otherwise it prefers the deepest discovered `.nerd` directory, then the original directory. It is not a topmost-module search. [Startup][startup] [Root discovery][root] [Chat initialization][session]

The canonical root already reaches shared boot and JIT initialization. Boot helpers synchronize and read its agent registry and define profiles. The system factory binds the actual tool root without changing process CWD; its project database and JIT setup do not give the compiler a roster-workspace option. This is therefore not evidence of missing knowledge-base registration or wholly broken workspace initialization. [Shared boot][boot] [Registry boot][profiles] [System factory][factory] [JIT construction][jitfactory]

The split occurs at context construction. `buildChatCompilationContext` fills ordinary request fields, available tools, and the semantic query, but not `AvailableSpecialists`. It asks for the chat-articulation consumer's needs. Policy derives `envelope_knowledge_requests` for that consumer, and the stock knowledge-discovery atom is mandatory when that world state holds. The atom labels its interpolated roster as the current project's specialists. [Chat context and compile][persona] [Consumer policy][needs] [Discovery atom][atom]

The compiler clones the context and, when its roster is blank, calls `InjectAvailableSpecialists(cc, "")` before hashing or cache lookup. The helper resolves that blank workspace to process CWD and reads `workspace/.nerd/agents.json`. A failed stat produces the formatted empty-registry result, including core built-ins, returns no error, and negatively caches that path. Neither TTL expiration nor a fresh compiler changes the selected directory. [Compiler entry][pipeline] [Roster resolution][roster]

The template returns that context field. The current render session charges expanded text and substitutes it once; the context hash already includes the roster string. Those mechanisms can correctly budget and cache the wrong listing. The older comment describing uncharged roster expansion is not an accurate description of current rendering. [Rendering][render] [Template field][template] [Context hash][hash]

On a healthy compile, chat uses the compiled system prompt in its articulation request; otherwise it can fall back to persona-only text. Returned knowledge requests enter the consultation handler. Automatic `_any_specialist` selection reads the registry under `m.workspace`, so root-based routing can rescue discovery omissions. Explicit names, existing project retrieval, and other tools can also compensate. The source establishes a mismatch in advertised names, not a necessary change in model choice or task quality. [Prompt delivery][delivery] [Request handling][handling] [Consultation routing][routing]

## Minimal source witness

A future reproduction should use a valid Go workspace `P` with `P/go.mod`, a valid root registry, and one uniquely named ready custom expert with working profile and knowledge-base boot. Keep the complete roster below the 40-entry limit. Launch the normal interactive TUI from `P/pkg`, with no nested module, no nested agent registry, and no workspace flag.

Hold the prompt body and other context stable, avoid other sources naming the expert, derive the chat knowledge-request need, and allow normal compilation budget. The source predicts that session boot and routing use `P`, while the injected roster uses the missing registry under `P/pkg`. When the stock atom is admitted, its actual listing omits the expert.

Launching at `P` or explicitly selecting `P` is a control, not a separate CLI prompt-impact claim. A correctly supplied nonempty `AvailableSpecialists` field also bypasses the fallback. A nested registry could instead advertise unrelated names, but that conditional extension is unnecessary to establish the primary omission.

## Proposed contract and implementation boundary

Resolve workspace identity once at the boot owner and carry it explicitly to roster discovery. Prefer a compiler option bound to the canonical workspace or a roster provider with an explicit workspace identity. Do not repair this through global `Chdir`, another environment-variable guess, or independent ancestor walks in each subsystem: those approaches obscure ownership and complicate multiple workspaces in one process.

A narrow chat fix can populate the roster from `m.workspace` before compilation. That covers the demonstrated call path, but it is not a complete compiler contract until other callers and the assembler's blank-field fallback are audited. Shared compiler instances must have unambiguous workspace ownership rather than accepting whichever process directory happens to be current.

Capture one immutable roster snapshot per request, with workspace identity, registry identity or revision, and resolution outcome available for diagnostics. Keep the existing formatted string in context and preserve its hash participation, entry bounds, and render-session consistency. This finding does not automatically require new hash machinery or a redesign of registry freshness.

Distinguish a genuine empty registry or optional absent registry from an unreadable, malformed, or incorrectly selected registry. Avoid presenting resolution failure as evidence that the canonical project has no custom experts. Any typed outcome should preserve an intentional degraded path rather than make all optional registry absence fatal.

Advertising an expert must not register it, manufacture a missing knowledge base, widen tool grants, or bypass readiness and dispatch checks. The proposed contract aligns discovery with the already selected workspace; it does not make the prompt roster an authority source.

## Expected costs and tradeoffs

All cost effects remain hypotheses. Passing an existing root and normalizing an identifier should be small work, but provider wiring, cache migration, filesystem operations, parsing, allocations, and latency need measurement. Correct path selection requires no new embedding or provider calls.

A correct roster can be longer than the incorrect built-in-only one. Its charged tokens may displace other context, and successful discovery may legitimately increase consultation calls. Preserve the 40-entry and 240-character bounds and full-prompt budget enforcement. Evaluate useful consultation and answer quality alongside tokens and cost; a cheaper empty listing is not automatically better. More explicit read failures could also change retries or backoff. Existing TTL and freshness policy remain a separate design question.

## Future verification and acceptance criteria

Start with a scripted, no-inference integration fixture that follows production root selection through chat initialization, boot, persona construction, actual compilation, and capture of the mock articulation request. Testing the roster helper with an explicit directory alone cannot establish the production contract.

Compare root and nested launches of the same workspace. Under the same ordinary budget, the unique ready expert must appear in the stock listing in both actual requests, with no wrong-workspace names. Use explicit process fixtures or injected ownership rather than changing global CWD inside parallel tests.

Cover these boundaries:

- Workspace-flag control, a misleading nested registry, and ancestor or module-boundary cases.
- Genuine empty registry, optional absence, read or parse failure, and ready versus not-ready entries.
- Caller-supplied nonempty roster, cold and warm caches, expiry after five seconds, and forced fresh compilation.
- Two explicitly bound workspace compiler instances sharing the same process CWD.
- No chat consumer need and non-chat CLI paths as negative controls; insufficient budget as a separate compilation-refusal case.
- Root-based automatic selection and explicit-name dispatch, while retaining correct profile and knowledge-base registration and unchanged authority checks.

Existing specialist benchmark definitions exercise explicit-directory loading and mtime/TTL refresh, not workspace discovery. Existing chat prompt tests express intended protocol inclusion. They are useful regression starting points, but their definitions were inspected rather than executed. [Specialist tests][specialisttests] [Chat protocol tests][chattests]

Record roster coverage, incorrect names, actual request bytes and tokens, filesystem operations, allocations, latency, and compiled-cache hits. Only a later matched-budget, held-out evaluation can establish effects on successful consultation, factual accuracy, turns, token use, cache behavior, and failed calls. No improvement percentage is claimed.

## Relationship to existing audits

This refinement concerns ownership of the roster's workspace, not a new general caching or consultation program. The generation-aware audit already owns source vectors, preflight, expanded-token charging, and single substitution. Prompt-budget hardening owns roster bounds. Consultation coordination owns deduplication, outcome handling, and reuse; the campaign retrieval contract concerns a different knowledge provider. Preserve those boundaries rather than duplicating their remedies. [Generation-aware audit][generation] [Prompt-budget audit][budget] [Consultation audit][consultation] [Campaign contract][campaign]

The inspected audit inventory and bounded searches did not identify this precise roster-root mismatch. The only documentation search hit for InjectAvailableSpecialists was the prompt architecture specification; that is not a claim of repository-wide novelty or runtime proof.

## Revision parity

The checked witness set comprises `compiler_specialists.go`, `compiler_pipeline.go`, `assembler.go`, and `context.go` under `internal/prompt`; the knowledge-discovery atom; `cmd/nerd/main.go`; chat `session.go`, `persona.go`, and `process_knowledge.go`; and `internal/config/user_config.go`.

Their blobs match main at Mangle `a53cfc308e8836c72370adaa46deba3f7b00180d`, session `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and OAuth `aa7988dec4c62116656ea722ff73b687a8509908`. This parity supports the same narrow source conclusion on those inspected heads, not an assertion about every branch or future revision. The main, three active implementation heads, and research head were rechecked unchanged at 18:18 UTC on October 29, 2026.

[startup]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/main.go#L171-L204
[root]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/config/user_config.go#L568-L600
[session]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session.go#L112-L203
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L38-L113
[profiles]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_boot_helpers.go#L136-L156
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L223-L243
[persona]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/persona.go#L318-L543
[needs]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/jit_needs.mg#L99-L102
[atom]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/atoms/capability/knowledge_discovery.yaml#L5-L21
[pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_pipeline.go#L76-L112
[roster]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_specialists.go#L42-L205
[render]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/assembler.go#L203-L273
[template]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/assembler.go#L504-L511
[hash]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/context.go#L740
[delivery]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L913-L970
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_knowledge.go#L178-L350
[specialisttests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/specialist_benchmark_test.go#L39-L104
[chattests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/chat_jit_prompt_test.go#L412-L442
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md#L199-L218
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/2026-09-09-system-hardening/02-PROMPT-BUDGET.md#L207-L213
[consultation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/consultation-coordination-and-safe-reuse.md#L116-L139
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/campaign-specialist-retrieval-contract.md#L15-L48
[jitfactory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1797-L1831
[handling]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L1050-L1055

