# articulation

`internal/articulation/` is the LLM I/O boundary. Inbound it parses raw model
text into a `PiggybackEnvelope` (display `Surface` plus typed
`ControlPacket`); outbound it assembles shard system prompts from templates,
kernel context, and session state.

- Envelope types: `PiggybackEnvelope` (`protocol_types.go:19-22`),
  `ControlPacket` (`protocol_types.go:25-46`), `ToolRequest`
  (`protocol_types.go:83-97`), `KnowledgeRequest`
  (`protocol_types.go:143-154`), `IntentClassification`
  (`protocol_types.go:157-163`), `MemoryOperation`
  (`protocol_types.go:208-212`), `SelfCorrection`
  (`protocol_types.go:215-218`), `ContextFeedback`
  (`protocol_types.go:224-239`).
- Inbound pipeline: `ResponseProcessor.Process` (`emitter.go:178-360`) tries
  `parseJSON` (`emitter.go:533-603`), markdown-wrapped JSON
  (`emitter.go:606-629`), and embedded-JSON extraction
  (`emitter.go:632-687`), then enforces `applyCaps`
  (`emitter.go:370-444`). How it works: INTERNALS.md.
- Lenient entries for callers that accept plain text: `ProcessLLMResponse`
  (`emitter.go:1072-1074`), `ProcessLLMResponseAllowPlain`
  (`emitter.go:1078-1080`), `MustExtractSurface` (`emitter.go:1136-1139`),
  `ExtractSurfaceOnly` (`emitter.go:951-969`).
- Outbound pipeline: `PromptAssembler.AssembleSystemPrompt`
  (`prompt_assembler.go:367-519`) over a `PromptContext`
  (`prompt_assembler.go:32-42`). How it works: INTERNALS.md.
- Kernel augmentation without an import cycle:
  `PromptAssembler.BuildContextSection` (`kernel_context.go`).
- Cycle-breaking entry for perception: `NewPromptAssemblerAdapter`
  (`prompt_assembler_adapter.go:22-24`); map-form contexts convert via
  `mapToPromptContext` (`prompt_assembler_adapter.go:51-137`).
- Schema text for the model: `GetPiggybackSchema`
  (`schema.go:235-240`).
- Streaming: `StreamParser` (`stream_parser.go:21-35`) with `ProcessChunk`
  (`stream_parser.go:45-187`), `IsComplete` (`stream_parser.go:196-198`),
  and `Reset` (`stream_parser.go:201-212`).
- Kernel safety hook on parsed envelopes: `ApplyConstitutionalOverride`
  (`emitter.go:854-902`); its only production caller is the session
  executor (`internal/session/executor.go:1740`). See
  WIRING-AND-NOT-BUILT.md.

Production consumers: session executor, `cmd/nerd/cmd_instruction.go`
(`NewEmitter` + `Emit`), campaign builders, shard registration, the system
factory, and the perception transducer. Full caller list:
WIRING-AND-NOT-BUILT.md.

Further reading: INTERNALS.md (how it works), WIRING-AND-NOT-BUILT.md
(what is wired, what is dead, what the design assumes but the code
does not do).

Verified: 2026-09-20 · branch `main`, working tree clean · commit `456e5217`.
