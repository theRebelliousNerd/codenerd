# Prompt Budget — inventory, bounds, and enforcement

Date: 2026-09-09
Branch: `claude/codenerd-system-hardening-1xorkw`
Scope: every distinct source of text that can reach the outbound LLM prompt, on
every live path (session executor, articulation fallback, perception
classification, chat).

Governing principle: *"the right context will be presented to the llm at the
right time, and nothing more."*

The finding behind this document in one line: **the compiler measured the
budget and then shipped the prompt regardless.** `logCompilationStats`
(`internal/prompt/compiler.go:1239` before this pass) detected the overshoot,
logged `budget breach` at WARN, and returned the over-budget prompt. Nothing
downstream re-measured, and two of the largest text sources — `nerd.md` and the
holographic file context — were appended *after* the measurement.

---

## 1. Inventory

Cap status is one of:

- **bounded** — a hard limit is enforced in code (not a convention) and was
  already there before this pass.
- **bounded (this pass)** — a limit added here.
- **unbounded** — no limit; listed with the scenario that makes it explode.
- **n/a** — cannot reach the model on any live path.

### 1.1 Session executor path (`nerd <verb>`, subagents, campaign shards)

| # | Source | Where | Cap status |
|---|---|---|---|
| 1 | Corpus atoms (embedded / project DB / shard DB / evolved) | `prompt/compiler.go` `collectAtomsWithStats` → `budget.Fit` | bounded — per-category allocation, render-mode degradation, `maxAtomsInput` 100000, `maxAtomsLimit` 5000 |
| 2 | Atom truncation inside a category allocation | `prompt/budget.go` `truncateAtomToBudget` | bounded, but **silently** → **bounded (this pass)**: visible marker added |
| 3 | Mandatory atoms | `prompt/budget.go:499` | bounded — rejected when a single atom exceeds the whole budget |
| 4 | `injectable_context` kernel rows (incl. northstar mission/constraints/risks) | `prompt/compiler.go` `collectKernelInjectedAtoms` | **unbounded** → **bounded (this pass)** |
| 5 | `specialist_knowledge` kernel blocks | same | **unbounded** → **bounded (this pass)** |
| 6 | Project knowledge atoms (semantic + lexical) | `prompt/compiler_db.go` `collectKnowledgeAtoms` | bounded — 5 per source, 10 merged; `RetrievedContext` atoms are fit whole or dropped whole (never shredded) |
| 7 | Learning atoms (autopoiesis recall) | `prompt/compiler_db.go` `collectLearningAtoms` | bounded — 5 hits |
| 8 | `{{available_specialists}}` (`.nerd/agents.json`) | `prompt/compiler_specialists.go` `formatSpecialists` | **unbounded** → **bounded (this pass)** |
| 9 | `{{available_tools}}` | `prompt/assembler.go:411` | bounded by the allowed-tool envelope (kernel-derived, small) |
| 10 | Other template expansions (`{{language}}`, `{{frameworks}}`, …) | `prompt/assembler.go:324-426` | bounded — scalar context fields |
| 11 | **Assembled prompt total, post-expansion** | `prompt/compiler.go` `Compile` step 5 | **unbounded (detected, not enforced)** → **bounded (this pass)** |
| 12 | `nerd.md` rendered instructions | `projectdoc/facts.go` `PromptSection`, appended at `session/executor.go:920` via `executor_tools.go:863` | **unbounded** → **bounded (this pass)** |
| 13 | Holographic per-file context | `world/holographic.go:188` `PromptSection` | bounded — every list is count-capped (`maxSigs` 8 etc.). Individual kernel-derived strings (`SystemPurpose`) are uncapped; low risk, not changed (file is owned by another agent) |
| 14 | Current user input | `session/executor.go` → `generateResponse` | bounded at 50000 chars for classification (`perception/understanding_adapter.go:207`); **the generation call receives it whole** — see §4.1 |
| 15 | Prior conversation turns (replay window) | `session/executor.go` `priorTurnMessages` | bounded — 6 messages / 24000 chars; per-turn text **unbounded** → **bounded (this pass)** |
| 16 | Stored conversation history | `session/executor.go` `appendToHistory` | 50-turn cap; per-turn text **unbounded** → **bounded (this pass)** |
| 17 | Individual tool result | `session/executor_tools.go:1934` `truncateToolResult` | bounded — 16 KiB, UTF-8 safe |
| 18 | **Aggregate in-turn tool transcript** | `session/executor_tools.go:211-260` `history` | **unbounded** → helper added in `executor.go` (this pass); call site is a 1-line patch, see §5 |
| 19 | Tool definitions (name/description/schema) | `session/executor.go` `buildToolDefinitions` | bounded — sourced from the in-repo tool registry, reviewed source text |
| 20 | MCP tool descriptions / schemas | `mcp/renderer.go` | **n/a** — `CompileToolsForShard` has no non-test caller; MCP tools are not rendered into any prompt today. Renderer already has `maxSchemaLen`. Wiring gap, not a leak |
| 21 | Build/test/lint verifier output | `session/build_verify.go:160`, `test_verify.go:262`, `lsp_diagnostics.go:99` | bounded — head-only with a marker |
| 22 | Critic file bodies | `session/critic.go:282` | bounded — `criticMaxFileBytes` with a marker |
| 23 | Subagent summaries | `session/subagent.go:454` | bounded — 4096 chars |
| 24 | Pending-edit content asserted to the kernel | `session/executor_tools.go:1250` | bounded — 16 KiB, digest retained |

### 1.2 Articulation fallback path (JIT compilation failed)

| # | Source | Where | Cap status |
|---|---|---|---|
| 25 | Kernel shard template (`shard_prompt_base`) | `articulation/prompt_assembler.go:479` | bounded — reviewed `.mg` source |
| 26 | Embedded baseline prompt | `prompt.AssembleEmbeddedBaselinePrompt` | bounded — mandatory embedded atoms |
| 27 | Hardcoded fallback templates | `articulation/prompt_assembler.go:939-1038` | bounded — Go constants |
| 28 | `injectable_context` atoms (legacy render) | `articulation/prompt_assembler.go` `queryContextAtoms` | **unbounded** → **bounded (this pass)** |
| 29 | Blackboard session context (12 sections) | `articulation/prompt_assembler.go` `buildSessionContext` | element **count** capped at 20 per section; element **length** and section total **unbounded** → **bounded (this pass)** |
| 30 | `CompressedHistory` inside the blackboard | same, `< 1500` chars gate | bounded — already |
| 31 | Intent block | `buildIntentContext` | intent fields are short; covered by the new section total |
| 32 | Piggyback protocol suffix | `PiggybackProtocolSuffix` | bounded — Go constant |

### 1.3 Perception classification path (runs on EVERY turn, before any work)

| # | Source | Where | Cap status |
|---|---|---|---|
| 33 | Classification system prompt | `perception/understanding_adapter.go` `getUnderstandingPrompt` | bounded — JIT-compiled (now enforced) or embedded constant |
| 34 | Raw user input | `understanding_adapter.go:207` | bounded — 50000 chars |
| 35 | Ambient `SelectedText` (editor selection) | `perception/transducer_llm.go` `BuildPrompt` | **unbounded** → **bounded (this pass)** |
| 36 | Ambient diagnostics | same | **unbounded** → **bounded (this pass)** |
| 37 | Strategic context (campaign-injected) | same | **unbounded** → **bounded (this pass)** |
| 38 | Semantic exemplars (embedding recall) | same | **unbounded** → **bounded (this pass)** |
| 39 | Prior turns (last 5) + thought summaries | same | count bounded, **text unbounded** → **bounded (this pass)** |
| 40 | Semantic classifier input | `perception/semantic_classifier.go:312` | bounded — 32768 bytes |

### 1.4 Chat path (`cmd/nerd/chat`)

| # | Source | Where | Cap status |
|---|---|---|---|
| 41 | Kernel `final_system_prompt` + persona | `chat/process.go:817-828` | bounded — `.mg` source + Go constant |
| 42 | Replayed conversation turns | `chat/helpers_articulation.go:158-176` | bounded — 2000/500 chars per turn |
| 43 | Reviewer findings | `chat/helpers_articulation.go:196` | bounded — 40 findings |
| 44 | `context_to_inject` facts | `chat/helpers_articulation.go:252` | bounded — 80 facts / 12 KiB |
| 45 | `CompressedCtx` (Mangle context block) | `context/compressor.go` `GetContextString` → `serializer.go` `SerializeCompressedContext` | **unbounded** → **bounded (this pass)** |
| 46 | Core facts inside that block (`permitted`, `dangerous_action`, …) | `context/compressor.go` `getCoreFacts` | **unbounded** (every `permitted` row the kernel holds) → **bounded (this pass)** at the render boundary |
| 47 | Per-fact line length in that block | `context/serializer.go` | bounded in `serializeGrouped` only; `serializeFlat` and `SerializeScoredFacts` **unbounded** → **bounded (this pass)** |
| 48 | `LastShardResult` task/metrics, warnings | `chat/helpers_articulation.go:186-280` | **unbounded** — not changed, see §4.2 |

---

## 2. Fixes

Every cap is a named constant with a comment explaining the number. Every
truncation leaves a marker the model can see. Truncation is head+tail wherever
the tail can carry the conclusion.

### 2.1 Shared primitives — `internal/prompt/limits.go` (new)

`ClampText` (head+tail), `ClampHead`, `ClampLines`, `TruncationNotice`,
`IsClamped`. Marker prefix `[codenerd: truncated` is stable and is what tests
assert on. Head:tail is 2:1 (`clampTailDivisor`): framing is at the top,
verdicts are at the bottom. Below `minClampChars` (200) it degrades to
head-only, because below that the marker dominates the budget.

`internal/projectdoc` duplicates `clampDocText` locally rather than importing
this: `internal/prompt` already imports `internal/projectdoc`, so the
dependency cannot run the other way.

### 2.2 Total prompt budget is now ENFORCED (deliverable 4)

Before: `Compile` ran `Fit` → `Assemble` → returned. `Fit` charged per-atom
render-mode token counts; `Assemble` then expanded `{{…}}` placeholders and
joined sections. `{{available_specialists}}` interpolates the entire agent
registry over a 25-character placeholder, so the emitted prompt could exceed
the budget `Fit` had just certified. The breach was logged and shipped.

After: `Compile` step 5b calls `enforceAssembledBudget`
(`internal/prompt/compiler.go`), which:

1. Measures the **assembled** prompt.
2. If over, calls `TokenBudgetManager.ShedToFit`: drops **whole optional
   atoms**, lowest category priority first and lowest score first within a
   priority, re-assembling after each pass (`maxShedPasses` = 4). Whole-atom
   eviction is deliberate — half an exemplar is worse than none, because the
   model cannot tell which half is missing. Mandatory atoms (identity, safety,
   the kernel blocks) are never shed here. The final pass drains *every*
   remaining optional atom: reaching it means the per-atom estimate is badly
   wrong, and falling through to whole-prompt truncation with low-priority
   atoms still in the prompt would shred a high-priority section to keep an
   exemplar — the exact inversion the budget contract forbids
   (`TestShedToFit_FinalPassDrainsAllOptionalAtoms`).
3. If the mandatory skeleton alone still overflows, calls `truncatePrompt` as
   a visible last resort and logs a WARN naming the shard.
4. On any re-assembly error it returns the inputs unchanged: a budget overshoot
   is a degradation, a failed compile is an outage.

`fitted` is updated in place so the manifest and flight-recorder stats describe
what was actually emitted, not what was originally selected.

`truncatePrompt` (`internal/prompt/assembler.go`) was reachable only from
`AssembleWithOptions`, which has no non-test callers — a wiring gap, not dead
code. It is now the enforcement backstop, and was changed from head-only to
head+tail. Head-only was actively wrong here: `defaultCategoryOrder` puts
identity/safety first and **JIT working memory last**, so head-only truncation
removed exactly the part of the prompt describing the turn. Its marker now
names how much was removed.

Proof, two levels:

- `TestEnforceAssembledBudget_AdversarialInput` — the helper, on two adversarial
  atom sets at a 4096-token budget (the real subagent budget,
  `session/spawner.go:685`): result inside budget, and marked when it had to cut.
- `TestCompile_ResultAlwaysFitsBudget` — end-to-end through `Compile` with a
  real `EmbeddedCorpus` and kernel, at 1024 / 4096 / 8192 budgets, including a
  mandatory-only corpus 15x the budget. Asserts `EstimateTokens(result.Prompt)
  <= cc.AvailableTokens()` in every case. This is the contract: whatever the
  corpus and the kernel hand the compiler, the string it returns fits the budget
  it was given.

### 2.3 Kernel-injected atoms (`prompt/compiler.go`)

`injectable_context` and `specialist_knowledge` each become ONE synthetic atom
with `IsMandatory=true`, so `Fit` skips category allocation and only rejects
the merged atom if it exceeds the *entire* budget. Two failure modes, both
live:

- **Growth** — `prompt_northstar.mg` emits one row per mission constraint and
  per risk with no limit.
- **Cliff** — because it is one merged atom, crossing the budget dropped the
  *whole block* silently. The model lost the northstar with no marker.

New caps: `maxKernelContextRows` 60, `maxKernelContextRowChars` 1024,
`maxKernelInjectedAtomChars` 16 KiB, `maxSpecialistKnowledgeBlocks` 12,
`maxSpecialistTopicChars` 200, `maxSpecialistBlockChars` 4 KiB.
`TestRenderKernelContextBlock_SurvivesFitAtShardBudget` pins the cliff fix:
5000 rows still survive `Fit` at an 8192-token budget.

### 2.4 Atom truncation is no longer silent (`prompt/budget.go`)

`truncateAtomToBudget` cut content with no marker. It now appends
`[codenerd: truncated N of M chars from atom <id>]`, and reserves
`atomTruncationMarkerBudget` (120 bytes) *inside* the allocation so the marker
cannot push the atom back over the ceiling `Fit` is about to check.

### 2.5 Pass-1 global budget guard (`prompt/budget.go`)

`calculateAllocations` clamps every category to its `MinTokens` floor without
consulting what is left, so on a small budget the allocations **sum to more
than the budget**. Default floors total ~6100 tokens; subagents compile at
4096 (`session/spawner.go:685`). `Fit`'s pass 2 always checked
`availableBudget`; pass 1 checked only
`catTokens+tokens <= allocation`. Added `usedTokens+tokens <= availableBudget`
to all three render-mode attempts in pass 1, and clamped `remainingAlloc` to
the global remainder before truncating.
Pinned by `TestFit_SmallBudgetDoesNotOverAllocate` at 1024/2048/4096/8192.

### 2.6 Specialist roster (`prompt/compiler_specialists.go`)

`maxSpecialistEntries` 40, `maxSpecialistEntryChars` 240. `.nerd/agents.json`
grows monotonically and nothing prunes it; the roster is interpolated after
`Fit` and was therefore unaccounted by construction.

### 2.7 Perception classification prompt (`perception/transducer_llm.go`)

Caps added for the editor selection (4 KiB, head+tail), diagnostics (10 lines ×
300 chars), strategic context (4 KiB), exemplars (8 × 500 chars), prior turns
(`maxClassificationTurns` 5 × `maxClassificationTurnChars` 2000, head+tail)
and thought summaries (800 chars). The classification caps are named
`maxClassification*` rather than `maxHistory*` so they cannot be confused with
the session executor's differently-sized history caps. The exemplar cap
counts *written* entries, not scanned positions — otherwise a run of
low-similarity hits would suppress the strong hits behind them
(`TestBuildPrompt_ExemplarCapCountsWrittenNotScanned`).

### 2.8 Articulation blackboard (`articulation/prompt_assembler.go`)

`sessionContextLine` clamps every free-text line to 500 chars;
`maxSessionContextChars` (32 KiB, head+tail) caps the whole block; kernel
injected atoms in the legacy path get the same 60-row / 1 KiB caps as the JIT
path. This is the *fallback* assembler — it runs when JIT compilation already
failed, which is when the system is least able to absorb a context-window
error on top.

### 2.9 Mangle context block (`context/serializer.go`)

- `serializeFlat` and `SerializeScoredFacts` now apply the same per-fact
  `maxLineLength` cut `serializeGrouped` always had. Calling
  `WithGrouping(false)` used to remove the only per-fact bound in the block.
- `SerializeCompressedContext` caps the whole block at
  `maxContextBlockChars` (64 KiB, head+tail).

Why the block needed its own ceiling: `ContextBlockBuilder.Build` *measures*
the block (`TokenUsage`) but enforces nothing; `TokenBudget.Allocate` — the API
that would reject an over-budget category — **has no production caller**
(`recalcBudget` writes `tb.used.*` directly); and `CheckTotalBudget` runs at
the *start* of the next `BuildContext`, so an over-budget block is always
shipped once before anything notices.

### 2.10 `nerd.md` (`projectdoc/facts.go`)

Body capped at `maxPromptBodyChars` (32 KiB, head+tail), whole section at
`maxPromptSectionChars` (48 KiB), each restated frontmatter list at
`maxPromptListEntries` (50). This repo's `nerd.md` is 4.7 KiB, so the caps are
~8x typical and never fire in practice — the point is a ceiling on a
pathological file, not a budget that shapes a normal one.

The forbid-list truncation marker explicitly restates that the paths are
**still ENFORCED by the kernel**
(`TestPromptSection_ForbidTruncationSaysEnforcementIsUnaffected`). A truncated
forbid list is a documentation loss, never a protection loss, and the model
must not read a short list as a complete one.

Bounding at *render* rather than at *read* is deliberate: the frontmatter
becomes kernel facts and must be parsed whole; only the prompt projection needs
a ceiling.

### 2.11 Conversation history (`session/executor.go`)

`appendToHistory` clamps `Content` to `maxHistoryTurnChars` (8000, head+tail)
and `ThoughtSummary` to `maxHistoryThoughtChars` (2000).

The regression this removes is subtle and was live: `priorTurnMessages` evicts
whole messages oldest-first against a 24000-char budget, so **one** 200 KB turn
evicted *every* prior turn and the next turn started with no conversational
memory at all — a context loss that presented as amnesia rather than
truncation. 8000 is a third of `DefaultHistoryCharBudget`, so three capped
turns still fit the window.
Pinned by `TestPriorTurnMessages_OneHugeTurnDoesNotEvictTheWindow`.

### 2.12 In-turn tool transcript (`session/executor.go` helper)

`boundToolLoopHistory` caps the replayed transcript at
`maxToolLoopHistoryBytes` (256 KiB). Worst case before: 50 tool calls ×
16 KiB = 800 KB, replayed on each of ~24 provider round-trips.

Eviction **rewrites** the oldest tool results in place rather than deleting
their messages: Anthropic-style APIs reject a request whose `tool_use` block
has no matching `tool_result`, so dropping a message is a hard 400 mid-turn.
The newest tool-result message is never blanked — it is clamped head+tail —
because it is the one the next decision is made from. Idempotent
(`TestBoundToolLoopHistory_IsIdempotent`).

The helper lives in `executor.go` (owned) so it could be built and tested here;
the call site is a one-line patch in `executor_tools.go` (§5).

### 2.13 `CategoryBudget.CanExceedMax` removed (`prompt/budget.go`)

Documented as "allows this category to exceed MaxTokens if budget remains",
assigned `true` six times, read zero times. **Removed rather than
implemented**: `Fit`'s second pass already provides exactly that behaviour, and
provides it for *every* category — any atom rejected in pass 1 is retried
against the global remaining budget with no reference to its category ceiling.
Implementing the flag would have *narrowed* that to six categories: a
prompt-shrinking behaviour change wearing the costume of a bug fix. A comment
at the field's old location records the reasoning.

---

## 3. `atom_selector` / `jit_logic.mg` — do NOT wire it up

Investigated at the coordinator's request. Conclusion: **option 2 (delete the
inert rules), not option 1 (emit the facts).** The chain is dead at *both*
ends, which the original report only established for one end.

- **No producer.** `PromptAtom.ToSelectorFacts()` (`prompt/atoms.go:539`) is
  called only from tests. Confirmed.
- **No consumer.** `atom_has_shard_match` … `atom_has_model_match`
  (`jit_logic.mg:11-23`, `:29-87`) are declared and derived, and **nothing
  anywhere reads them**. `grep -rn "atom_has_" internal/` returns only their
  own Decls and rule heads. The claim that they feed `atom_matches_context`
  (`jit_logic.mg:90`) does not hold: that rule's body is
  `prompt_atom(...), atom_context_boost(AtomID, FinalScore)` — it never
  mentions `atom_has_*`.
- **The scoring predicate they would have fed is itself dead.**
  `atom_context_boost` has a `Decl` (`schemas_prompts.mg:450`) and is listed as
  a prompts-domain predicate (`shards/registration.go:271`), but **no Go code
  produces it**. So `atom_matches_context/2`'s non-mandatory rule never fires,
  and with it `atom_candidate`, `atom_loses_conflict`, `atom_loses_exclusion`,
  `is_excluded` and the score-derived `activation/2` in `jit_selection.mg`.
  None of those has a Go consumer either.

So emitting `atom_selector` would derive 13 predicates that nothing reads, at
a cost of 13 rules × N atoms × M context facts of join work on the hot compile
path, for zero behavioural change.

**Dimensional matching is not missing.** The live selector path is
`jit_compiler.mg`, which matches on `atom_tag(Atom, Dim, Tag)` against
`current_context(Dim, Tag)` via `has_constraint`/`satisfied_constraint`/
`blocked_by_context` (`jit_compiler.mg:62-100`, `:185-245`), producing
`selected_result/3` — which is exactly what `selector.go:877` and `:1044`
query, and exactly what `selector.go:1360-1378` emits tags for. The fail-closed
regime-dimension block that fixed the "114 mandatory atoms / 25 contradictory
identities" incident lives there. `jit_logic.mg`'s `atom_selector` vocabulary
is an older parallel design that was superseded and never removed.

**Recommendation for the `.mg` owner:** delete `jit_logic.mg:11-23` (the 13
`atom_has_*_match` Decls) and `:29-87` (their rules). Separately, decide
`atom_context_boost`: either implement the Go virtual predicate or delete
`jit_logic.mg:90-92` and the `jit_selection.mg` chain that depends on it. That
is a bigger call and is out of scope for this pass — but it should be recorded,
because right now `jit_selection.mg`'s conflict/exclusion resolution is inert
and the equivalent logic in `jit_compiler.mg` is what actually runs.

---

## 4. Found and deliberately NOT changed

### 4.1 Current-turn user input reaches generation uncapped

`session/executor.go` passes `input` to `generateResponse` verbatim.
Classification caps it at 50000 chars (`understanding_adapter.go:207`) but
generation does not. `nerd run "$(cat 50mb.log)"` sends 50 MB to the provider.

Not changed: truncating the user's actual request is a semantic decision, not a
hygiene one, and the failure mode is a clean provider-side 413 rather than a
model reasoning on a lie. It is now the only uncapped path into a live prompt,
and it is bounded on the *replay* side (item 15/16) so it can poison at most
one turn. Recommend a config-driven cap with an explicit CLI warning; not
something to decide inside a hardening pass.

### 4.2 Chat `LastShardResult.Task` / `Metrics` / `warnings`

`chat/helpers_articulation.go:186-280`. Uncapped, but every producer is
in-repo and short (`Task` is already truncated to 50 chars for the blackboard
listing at `:236`). `cmd/nerd/chat/**` is outside the owned set for this pass
and the risk is low. Recorded, not changed.

### 4.3 `world/holographic.go` free-text fields

`SystemPurpose`, `Role`, `Layer` are interpolated without a length cap. Every
list around them is count-capped, the strings are kernel-derived architecture
metadata, and the file is owned by another agent this pass. Recorded, not
changed.

### 4.4 `FactSerializer.truncateFact` arg truncation is uncounted

`context/serializer.go:153` cuts each fact argument at 50 chars and appends
`...`. The marker is visible but carries no count. Adding one would break the
Mangle notation the block is rendered in, and `...` inside a quoted argument is
already unambiguous. Left as is.

### 4.5 `TokenBudget.Allocate` has no production caller

`context/tokens.go:270`. `AllocateWithError` and `MustFitWithinBudget` likewise.
`recalcBudget` (`compressor_turns.go:247`) writes `tb.used.*` directly, so the
budget *is* maintained — the allocation API is a partially-wired alternative,
not a broken accounting path. Per `audit-before-delete` the wiring gap is
recorded rather than the code deleted. The enforcement gap it leaves is covered
by §2.9's render-boundary cap.

### 4.6 MCP tool rendering is not wired

`mcp.MCPIntegrationBridge.CompileToolsForShard` and `ToolRenderer` have no
non-test callers, so MCP tool descriptions and schemas cannot reach a prompt
today. The renderer already carries `maxSchemaLen`. Wiring gap; nothing to cap
until it is wired, and it should be capped *when* it is wired because MCP
descriptions are third-party text.

### 4.7 Scope finding: `internal/context` is chat-only

`internal/context` (spreading activation, semantic compression) is imported
only by `cmd/nerd/chat/**`, `cmd/nerd/cmd_context_stats.go` (a read-only stats
command) and `internal/testing/context_harness`. `internal/session` does not
import it at all.
So every `nerd <verb>` CLI run, every spawned subagent and every campaign shard
gets no compression — only the fixed 6-message / 24000-char window at
`executor.go` `priorTurnMessages`. Not restructured in this pass; recorded
because it means the "infinite context" machinery covers one of four entry
points.

---

## 5. Patches for files owned by the coordinator

### 5.1 `internal/session/executor_tools.go` — apply the tool-transcript bound

The helper `boundToolLoopHistory`, its constants and its tests already exist in
`internal/session/executor.go` and `internal/session/history_bounds_test.go`.
Only the call sites are needed, and there are exactly two — the two places that
put `history` on the wire (`grep -n CompleteWithToolResults executor_tools.go`
→ `:232`, `:523`).

**(a) `runToolLoop`, at `:210-214`** — after the tool_result turn is appended,
before the loop reaches `trp.CompleteWithToolResults` at `:232`:

```go
 		// Append the user tool_result turn to history and re-invoke.
 		history = append(history, types.Message{
 			Role:        "user",
 			ToolResults: toolResults,
 		})
+		// Each result is capped at 16 KiB (truncateToolResult), but `history`
+		// is append-only and is re-sent WHOLE on every round-trip: 50 tool
+		// calls x 16 KiB replayed across ~24 iterations. Blank the oldest
+		// payloads, keep the newest.
+		history = boundToolLoopHistory(history)
```

**(b) `forceDeadlineFinalAnswer`, at `:521`** — immediately before the final
`trp.CompleteWithToolResults(ctx, systemPrompt, *history, finalTools)` at
`:523`. This one call site covers every `*history` append in that function
(`:434`, `:502`, `:521`, `:584`):

```go
 	*history = append(*history, types.Message{Role: "user", Text: nudge})
+	// The forced-final call resends the whole transcript, including every
+	// tool result accumulated before the deadline fired.
+	*history = boundToolLoopHistory(*history)
 
 	final, err := trp.CompleteWithToolResults(ctx, systemPrompt, *history, finalTools)
```

`boundToolLoopHistory` is a no-op below the ceiling (every ordinary turn),
preserves message count, ordering and every `ToolUseID`, does not mutate its
input, and is idempotent — so it is safe at both sites and safe to add at more
if you prefer belt-and-braces.

### 5.2 `internal/session/executor_tools.go:1912` — retry loses `nerd.md`

`retryWithNoToolNudge` reissues with raw JIT output, dropping the
`withProjectInstructions` / `withFileContext` wrapping the first attempt had.
`nerd.md`'s write-protection rules vanish from the system prompt on precisely
the turn where the model has already shown it is confused about what it may do.

```go
-	return e.generateResponse(ctx, client, compileResult.Prompt, userInput, cfg)
+	// The retry must carry the SAME system prompt the first attempt did.
+	// compileResult.Prompt is raw JIT output; the live prompt is that plus
+	// nerd.md's rendered instructions and the target's holographic context
+	// (executor.go step 4b). Reissuing without them drops the project's
+	// write-protection rules from the retry — a safety regression, not just a
+	// context one.
+	retryPrompt := e.withProjectInstructions(compileResult.Prompt)
+	retryPrompt = e.withFileContext(ctx, retryPrompt, retryCtx.IntentTarget)
+	return e.generateResponse(ctx, client, retryPrompt, userInput, cfg)
```

### 5.3 `internal/core/defaults/policy/prompt_northstar.mg` — no producer-side cap needed

The consumer cap in §2.3 is sufficient and is the right layer: it bounds every
producer of `injectable_context`, present and future, and it converts the old
all-or-nothing cliff into a graduated loss with a marker. A producer-side limit
would also silently drop constraints from the *kernel*, where they are load-bearing
for policy, not just for the prompt. Recommend leaving the `.mg` as is.

### 5.4 `internal/core/defaults/policy/jit_logic.mg` — delete the inert selector rules

See §3. Delete lines 11-23 (the `atom_has_*_match` Decls) and 29-87 (their
rules). Do not add an `atom_selector` producer.

---

## 6. Verification

```
go build ./...   pass
go vet ./...     pass

go test ./internal/prompt/... ./internal/context/... ./internal/articulation/...
         ./internal/perception/... ./internal/projectdoc/... ./internal/session/...

ok  codenerd/internal/prompt
ok  codenerd/internal/prompt/marathon
ok  codenerd/internal/prompt/sync
ok  codenerd/internal/context
ok  codenerd/internal/articulation
ok  codenerd/internal/perception
ok  codenerd/internal/perception/xaioauth
ok  codenerd/internal/projectdoc
ok  codenerd/internal/session

# dependent packages (everything importing the six above)
go test ./cmd/nerd/... ./internal/autopoiesis/... ./internal/campaign/...
        ./internal/core/... ./internal/init/... ./internal/jit/...
        ./internal/shards/... ./internal/verification/... ./internal/tools/...
all pass
```

New tests:

| File | Covers |
|---|---|
| `internal/prompt/limits_test.go` | clamp primitives, UTF-8 boundaries, marker presence |
| `internal/prompt/budget_enforcement_test.go` | `ShedToFit` priority/score order, mandatory preservation, assembly-failure fallback, `Compile` end-to-end at 1024/4096/8192, small-budget over-allocation |
| `internal/prompt/kernel_injection_bounds_test.go` | kernel context/knowledge caps, the `Fit`-rejection cliff, specialist roster |
| `internal/perception/transducer_prompt_bounds_test.go` | all six classification-prompt sources, exemplar cap semantics |
| `internal/articulation/session_context_bounds_test.go` | blackboard per-line and total caps, ordinary-session passthrough |
| `internal/context/serializer_bounds_test.go` | per-fact bound on both serializer paths, context block total |
| `internal/projectdoc/prompt_section_bounds_test.go` | `nerd.md` body/list/section caps, enforcement-still-applies marker |
| `internal/session/history_bounds_test.go` | turn text caps, replay-window eviction regression, tool transcript bound + idempotence |
