# articulation internals

How the package works inside. What it is and who calls it: README.md.
What is wired, dead, or assumed: WIRING-AND-NOT-BUILT.md.

## Inbound: text to envelope

1. `ResponseProcessor.Process` (`emitter.go:178-360`) tries three parses in
   order: `parseJSON` (`emitter.go:533-603`) for a bare envelope,
   `parseMarkdownWrappedJSON` (`emitter.go:606-629`) for fenced JSON, and
   `extractEmbeddedJSON` (`emitter.go:632-687`), which scans candidates
   with `findJSONCandidates` (`json_scanner.go:37-132`, called at
   `emitter.go:638`) and locates keys outside string literals with
   `findKeyOutsideStrings` (`json_scanner.go:140-171`).
2. Unknown-field rejection: `checkUnknownFields` (`emitter.go:503-530`)
   against `schemaAllowedKeys` (`emitter.go:458-499`).
3. Caps and filters: `applyCaps` (`emitter.go:370-444`) bounds surface
   length (`truncateUTF8Bytes`, `emitter.go:446-455`), mangle-update count
   and single-update length, and drops syntactically bad atoms.
4. The outcome is an `ArticulationResult` (`emitter.go:147-161`); counters
   go to `ProcessorStats` (`emitter.go:138-144`) via `updateStats`
   (`emitter.go:708-713`), readable with `GetStats`
   (`emitter.go:690-698`) and cleared with `ResetStats`
   (`emitter.go:701-706`).

Wire tolerance lives in the types, not the processor:
`ControlPacket.UnmarshalJSON` (`protocol_types.go:49-78`),
`ToolRequest.UnmarshalJSON` (`protocol_types.go:100-138`), and
`IntentClassification.UnmarshalJSON` (`protocol_types.go:166-205`).

Placeholder and refusal detection is a separate pre-pass:
`isPlaceholderSurface` (`emitter.go:1033-1042`) and
`isPlaceholderFeedback` (`emitter.go:1048-1059`) match the patterns at
`emitter.go:1023-1027`, recorded on `ProcessedLLMResponse`
(`emitter.go:1012-1018`).

Truncated output is salvaged, not dropped: `looksLikePartialEnvelope`
(`emitter.go:19-27`) detects a cut-off envelope,
`salvageSurfaceFromPartial` (`emitter.go:34-64`) recovers the surface, and
`truncatedEnvelopeMessage` (`emitter.go:70-81`) labels the fallback.

The package's own LLM entries never require strict JSON:
`processLLMResponse` (`emitter.go:1086-1132`) hardcodes
`RequireValidJSON=false` (`emitter.go:1090`); the strict field on
`ResponseProcessor` (`emitter.go:126-135`, built by `NewResponseProcessor`,
`emitter.go:164-174`) exists but is unwired on this path.

## Outbound: context to prompt

`PromptAssembler.AssembleSystemPrompt` (`prompt_assembler.go:367-519`):

1. Normalise input: `toCompilationContext`
   (`prompt_assembler.go:101-319`); map-form callers convert via
   `mapToPromptContext` (`prompt_assembler_adapter.go:51-137`, invoked at
   `prompt_assembler.go:381`).
2. Load the shard template: `queryShardTemplate`
   (`prompt_assembler.go:537-570`); unknown shard types fall back to
   `getFallbackTemplate` (`prompt_assembler.go:940-953`).
3. Inject kernel context: `getInjectableContextFacts`
   (`prompt_assembler.go:574-600`) and `queryContextAtoms`
   (`prompt_assembler.go:604-633`); the standalone form is
   `BuildContextSection` (`kernel_context.go:35-59`), which joins
   `queryAndFormatContext` (`kernel_context.go:62-94`) and
   `queryAndFormatSpecialistKnowledge` (`kernel_context.go:97-128`).
4. Render the session blackboard: `buildSessionContext`
   (`prompt_assembler.go:636-885`), one line per fact via
   `sessionContextLine` (`prompt_assembler.go:915-917`), intent via
   `buildIntentContext` (`prompt_assembler.go:920-937`).
5. Append the piggyback suffix when `shouldAppendPiggybackProtocol`
   (`prompt_assembler.go:521-533`) says so; or compile through JIT when
   `JITReady` (`prompt_assembler.go:1208-1212`) reports a compiler, with
   budgets from `SetJITBudgets` (`prompt_assembler.go:1255-1281`) via
   `getBudgetConfig` (`prompt_assembler.go:1283-1287`).
6. Prompt language: `inferLanguageFromTarget`
   (`prompt_assembler.go:323-343`) unless `shouldForceMangleLanguage`
   (`prompt_assembler.go:346-359`).

Blackboard growth is capped: `maxSessionContextLineChars`
(`prompt_assembler.go:900`), `maxSessionContextChars`
(`prompt_assembler.go:903`), `maxInjectedContextAtoms`
(`prompt_assembler.go:908`), `maxInjectedContextAtomChars`
(`prompt_assembler.go:911`).

Supporting entries: `EnableJIT` (`1216-1225`), `SetJITCompiler`
(`1229-1238`), `GetJITCompiler` (`1241-1245`), `IsJITEnabled`
(`1248-1252`), `AssembleQuickPrompt` (`1160-1172`), and the context
builders `WithSessionContext` (`1175-1178`), `WithIntent` (`1181-1184`),
`WithCampaign` (`1187-1190`), `WithSemanticQuery` (`1193-1199`).

## Streaming

`StreamParser.ProcessChunk` (`stream_parser.go:45-187`) accumulates text;
`GetFullBuffer` (`191-193`) reads it back, `IsComplete` (`196-198`)
reports a balanced envelope, `Reset` (`201-212`) clears state.
String and escape correctness comes from `scanJSONString*`
(`json_scanner.go:181-255`), `decodeUnicodeEscape` (`260-279`),
`decodeStreamEscape` (`291-316`), and `parseHex4` (`319-340`).

Verified: 2026-09-20 · branch `main`, working tree clean · commit `456e5217`.
