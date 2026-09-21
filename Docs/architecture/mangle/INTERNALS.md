# mangle internals — how the package works

Verified 2026-09-21 against commit `0f7253b`. Every section cites the code it
describes; anything without a citation is the author's inference, not the code.

## Engine lifecycle (`engine.go`, 1,410 lines)

`NewEngine` (`engine.go:152-163`) takes a `Config` (`engine.go:34-52`) and
returns a store with an empty program: no schemas, no facts, no derivations.
`LoadSchema` / `LoadSchemaString` (`engine.go:285-317`) add source text and call
`rebuildProgramLocked` (`engine.go:320-377`), which re-parses the whole program
and re-derives. `WarmFromPersistence` (`engine.go:380-421`) replays stored facts
without re-parsing.

`AddFact` / `AddFacts` / `AddFactsContext` (`engine.go:424-486`) insert base
facts through `insertFactLocked` (`engine.go:630-656`), which converts each
value with `convertValueToTypedTerm` (`engine.go:735-864`) and may trigger
evaluation. `ReplaceFactsForFile` and its hash variant (`engine.go:489-496`)
delegate to `replaceFactsForFileImpl` (`engine.go:499-537`); `ReplaceControlFacts`
(`engine.go:572-601`) swaps every base fact of the named predicates at once.
`PushFact` (`engine.go:1332-1334`) is the append path the session uses for
streamed facts.

Evaluation is `evalWithGasLimit` (`engine.go:220-267`), which runs the
upstream fixpoint and then caps the derived set. The two entry points differ
in what they promise: `RecomputeRules` (`engine.go:180-215`) re-runs
derivation without touching the store; `Evaluate` (`engine.go:550-558`)
re-runs derivation as a named operation and returns `errNoSchemas` when no
program is loaded — the name suggests a fuller pipeline than the one line it
contains, but both funnel into `evalWithGasLimit` (556). `EvaluateRule`
(`engine.go:1407-1409`), a single-rule evaluation helper, closes the file.

`Query` (`engine.go:869-1016`) parses the query shape (`parseQueryShape`,
`engine.go:1169-1205`), requires the queried predicate to have a `Decl` and
errors otherwise (`engine.go:884-888`), then merges stored rows with fresh
`EvalQuery` derivations, dedups, and sorts (`engine.go:961-999`), with a 5s
default timeout (`engine.go:904-917`). Repeated variables must agree
(`repeatedVariablesAgree`, `engine.go:1210-1230`). The read-only surface is
`GetFacts` (`engine.go:1019-1047`), `QueryFacts` (`engine.go:1340-1371`),
`GetFactsSeq` (`engine.go:1374-1404`), `AtomStrings` (`engine.go:1054-1071`),
`GetStats` (`engine.go:1074-1094`), and `GetProgramInfo` (`engine.go:1153-1157`);
`Clear` / `Reset` / `Close` (`engine.go:1097-1149`) tear down in that order.

## Validation gates (`schema_validator.go`, 598 lines)

`NewSchemaValidator` (`schema_validator.go:35-42`) starts empty.
`LoadDeclaredPredicates` (`schema_validator.go:45-61`) scans source text for
`Decl` forms via `extractDeclsFromText` (`schema_validator.go:64-82`, with the
`balancedArgs` paren matcher at 86-99) and records arities; head predicates
found without a `Decl` come from `extractHeadPredicatesFromText` (103-116).

`ValidateRule` (`schema_validator.go:125-156`) is the per-rule entry point.
`ValidateLearnedRule` (`schema_validator.go:164-208`) adds the learned-rule
gates on top: reject heads in `forbiddenLearnedHeads` (174-177), atom-syntax
check on whatever sits in head position even when the head regex misses
(185-189), require a `Decl` for learned *facts* while letting learned *rules*
derive new predicates (197-199), validate head arity against the schema (202),
then delegate to `ValidateRule` (207). Arity counting is quote- and
nesting-aware (`countTopLevelArgs`, 283-309). `HotLoadRule`
(`schema_validator.go:212-224`) parses the candidate with `ParseUnit` (220),
auto-appends a missing trailing period (217-219), and runs
`ValidateLearnedRule` (223) — all in-process. There is no sandbox here; the
sandbox belongs to the kernel path described in WIRING-AND-NOT-BUILT.md.
`ValidateRules` (409-423) and `ValidateProgram` (446-483) are the batch forms;
`unsourcedPremisePredicates` (488-513) reports body predicates with no
producer. Introspection is `IsDeclared` (561-563), `GetDeclaredPredicates`
(566-568), `GetArity` (571-576), `CheckArity` (580-591), and
`SetPredicateArity` (595-597).

## Grammar and repair (`grammar.go`, 794 lines)

`AtomValidator.loadCorePredicates` (`grammar.go:90-238`) and
`loadCoreNameConstants` (`grammar.go:303-387`) hard-code the known predicate
and `/name` shapes; `UpdateFromProgramInfo` (`grammar.go:243-272`) extends
them from the live program. `ValidateAtom` (`grammar.go:390-484`) checks one
atom; `validateArg` (`grammar.go:487-538`) checks one argument with
`boundToArgType` (276-300), `inferArgType` (630-659), `isNumeric` (665-670),
`compatibleTypes` (673-678), and `typeString` (681-696); `attemptRepair`
(541-560) tries `fixUnquotedStrings` (701-711). `ValidateAtoms` (563-569) is
the batch form.

`RepairLoop` (`grammar.go:719-722`) holds a validator plus program info
(`NewRepairLoop`, 725-730; `UpdateFromProgramInfo`, 733-735).
`ValidateAndRepair` (744-766) validates, and for each invalid atom
`generateRepairPrompt` (769-793) builds the "MANGLE SYNTAX ERROR" prompt that
goes back to the model, with the syntax rules restated inline (785-790).

## Learner loop (`synth/`, `feedback/`, `transpiler/`)

`synth.CompileSpec` (`synth/synth.go`) renders a pattern IR into Mangle text
through deliberate stages (`synth/planner.go`, `normalize.go`); `sanity.go`
(156-299) rejects empty and duplicate patterns before planning, and
`semantics.go:CheckPatternSemantics` (98-200) rejects patterns whose Mangle
cannot mean what the author intended. `transpiler/sanitizer.go:sanitizeFile`
(28-168) scrubs model-pasted Mangle in stages before it reaches any of this.

`feedback/loop.go:GenerateAndValidate` normalizes the candidate
(`NormalizeRuleInput`, Phase 0), runs the engine-native validator
(`SchemaValidator.HotLoadRule`, Phase 1), calls the injected model client
(`llmClient.Complete`, `loop.go:199`), decodes the reply
(`SpecDecode.DecodeSpec`, `loop.go:222`), and compiles the result
(`synth.CompileSpec`, `loop.go:224`). `FixRule` (`feedback/autocorrect.go`)
repairs a rejected candidate; `feedback/config.go` holds the retry defaults;
`BuildEnhancedSystemPrompt` (`feedback/prompts.go`, consumed at
`internal/core/kernel_policy.go:267`) and `mangleRuleSystemPrompt`
(`feedback/system_prompt.go`) are the prompt surface.

## Proof trees (`proof_tree.go`, 502 lines)

`ProofTreeTracer` (`proof_tree.go:51-59`) indexes rules by head predicate
(`IndexRules`, 82-132), derives one node per fact (`buildDerivationNode`,
223-253), classifies EDB vs IDB (`classifyFact`, 256-281), finds premises
(`findPremises`, 306-340), and caches recent traces FIFO
(`traces`/`traceOrder`/`maxCache`, with `ClearCache` at 391-399 and
`GetCachedTrace` at 402-406). `TraceQuery` (139-220) is the entry point;
`MaterializeToFacts` (352-375) reifies a trace as `derivation_trace` /
`proof_tree_node` facts; `RenderASCII` (413-456) and `RenderJSON` (459-501)
render it. The only production caller of the tracer is `nerd why`
(`cmd/nerd/cmd_query.go:308-310`); the kernel's own `TraceQuery` method is a
separate path — see WIRING-AND-NOT-BUILT.md.

## LSP indexing (`lsp.go`, 1,019 lines)

`NewLSPServer` (`lsp.go:105-114`) stores the engine handle and builds empty
indexes. `indexDocumentLocked` (`lsp.go:162-221`) extracts declarations,
definitions, references, and hover text with line regexes — it never consults
the engine. `OpenDocument` (133-147) stores text and (re-)indexes; there is no
separate update method, re-opening is the update path. `CloseDocument`
(150-155) drops the entry. `IndexWorkspace` (676-715) walks the tree for
`.mg` / `.mangle` files and opens each one through `OpenDocument` (call at
710), skipping `node_modules`, `.git`, `vendor`, `.nerd/cache`, the
`debug_program*.mg` dumps (688-703), and files it cannot read (704-707).
`GetCompletions` (513-589) completes predicates, `/names`, and keywords from
the snapshot. `GetHover` (438-487) resolves the word under the cursor with
`getWordAtPosition` (952), falls back through the hover and definition maps,
and renders the predicate's declarations with arity counted by `countArity`
(992); prefix completion uses `getWordPrefixAtPosition` (971) over
`isWordChar` (984). `ValidateCode` (656-669) replays the validator and
publishes `Diagnostic` values. The stdio server (`ServeStdio`, 744-802)
speaks JSON-RPC over stdin/stdout.

## Concurrency and platform notes

The upstream parser is not goroutine-safe: `parse_lock.go:ParseUnitGuard`
(71 lines) serializes parser entry and every load path goes through it.
`Engine` guards its own state with a mutex (`engine.go:62-81`); `WarmFromPersistence`
documents the lock ordering where the two meet. `simd.go` (29 lines) is a
build-tagged probe — the scalar path does the work on every platform this
repository builds on.
