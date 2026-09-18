# S14 — The architect's chat persona, delivered and never lost

## Status

- **Phase:** Study complete, decision taken, implementation landed, tests green.
- **Worktree:** `C:\CodeProjects\codeNERD\.claude\worktrees\agent-a343f2ad01ad9d148`, branch
  `worktree-agent-a343f2ad01ad9d148`, fast-forwarded to `dogfood/c2-closure` @ `ce05e8fe`
  before any edit (the worktree was 43 commits behind and the incoming diffs touched
  `internal/prompt/selector.go`, `internal/session/`, `internal/core/` — all files this seam reads).
- **Decision:** the persona does **not** become a JIT atom. It stays a Go constant, behind one
  delivery seam, pinned by tests. Reasoning in the Decision section; it rests on a fact the brief
  did not have: the main chat turn never compiles a JIT prompt at all.

## Study

Every claim below is `path:line` in this worktree at `ce05e8fe`, read directly. Nothing here is
taken from `Docs/architecture/`.

### (1) How an atom gets from a YAML file into the embedded corpus

- **Source of truth:** `internal/prompt/atoms/<category>/*.yaml`. The YAML schema is
  `AtomDefinition` at `cmd/tools/prompt_builder/main.go:43-81`; `is_mandatory` is
  `main.go:57`, `priority` is `:56`, the contextual selectors (`intent_verbs`, `shard_types`,
  `operational_modes`, `models`, …) are `:63-76`, and content is either inline `content:` or
  `content_file:` (`:79-80`).
- **The build step is a manual command, not `go generate`.** It is documented in the tool's own
  package comment at `cmd/tools/prompt_builder/main.go:6`:
  `go run ./cmd/tools/prompt_builder`, with `-input internal/prompt/atoms` and
  `-output internal/core/defaults/prompt_corpus.db` as defaults (`main.go:92-94`). With
  embeddings it requires `GEMINI_API_KEY` (`main.go:107-115`); `-skip-embeddings` (`main.go:94`)
  builds a corpus without them. **`grep -rn "go:generate"` over `internal/core/defaults`,
  `internal/prompt` and `cmd/tools` returns nothing** — the `.db` is a committed binary artifact
  that a human regenerates.
- **The embedded DBs are three, not seven.** Every `go:embed` of a `.db` in the repository:
  - `internal/core/defaults/prompt_corpus.go:22-23` → `prompt_corpus.db` (14,327,808 bytes)
  - `internal/core/defaults/intent_corpus.go:23-24` → `intent_corpus.db` (18,075,648 bytes)
  - `internal/core/defaults/predicate_corpus.go:26` → `predicate_corpus.db` (1,359,872 bytes)
  (The brief's "7 embedded DBs" does not match the tree.)
- **Runtime path:** `prompt.MaterializeDefaultPromptCorpus` (`internal/prompt/default_corpus.go:18`)
  writes the embedded bytes to a workspace file *only if that file does not already exist*
  (`:24-28`) — it never clobbers. The compiler then merges atom sources with the embedded corpus
  winning every duplicate ID: embedded → project → shard → evolved → strategy
  (`internal/prompt/compiler.go:1220-1252`, precedence rationale at `:1185-1188`).

So adding a mandatory atom is: write the YAML, run `go run ./cmd/tools/prompt_builder`, commit
both the YAML and the regenerated 14 MB `prompt_corpus.db`.

### (2) How the chat path's `final_system_prompt` is compiled — it is not

This is the finding the decision turns on.

- `cmd/nerd/chat/process.go` (pre-change `:822-829`) queried the kernel for `final_system_prompt`
  and used row 0 arg 0 as the system prompt.
- **Nothing in this repository produces `final_system_prompt`.** It is listed in the repo's own
  never-produced baseline: `internal/core/defaults/testdata/query_only_predicates.txt:22` reads
  `final_system_prompt<TAB>cmd/nerd/chat/process.go`. That file's header states the property
  exactly: *"Predicates Go queries that nothing produces. Each of these returns zero rows on every
  call."* The gate is `TestGoQueriedPredicateBudget`
  (`internal/core/defaults/query_only_predicate_test.go:41`), which computes the set from the
  Mangle corpus plus an over-approximated Go producer scan (`:61-109`). **I ran it: PASS**, i.e.
  the baseline is accurate and `final_system_prompt` has no producer today.
- Its only other appearance anywhere in the tree is the declaration at
  `internal/core/defaults/schemas_reviewer.mg:144-145`.
- **Consequence:** on every main-chat turn `systemPrompt` was `""`, and `process.go:832`
  (`systemPrompt += "\n\n" + stevenMoorePersona`) made the persona the *entire* system prompt.
  This is not inference — the new test prints it: with the persona delivery removed, the
  main-chat system prompt is **0 bytes**.
- That string is passed as the real system message to the model at
  `cmd/nerd/chat/helpers_articulation.go:323` / `:327` (streaming) and `:383`
  (`CompleteWithSystem`), and is additionally echoed into the user message at `:145-149`.
- **The main chat turn never calls the JIT compiler.** The only `jitCompiler.Compile` calls under
  `cmd/nerd/chat` are `northstar_llm.go:110` and `process_dream_delegation.go:413`.
  `process.go:559` and `process.go:911` only read `GetLastResult()` for glass-box telemetry and a
  manifest hash — telemetry about a compile some *other* path did.
- Therefore the brief's question "would a mandatory chat-persona atom be at the top of
  `final_system_prompt`?" has a harder answer than expected: **it would not be in it at all**, and
  neither would any other atom, because no atom-selection or budget machinery runs on this path.
- Adjacent, same cause, not this seam's to fix: `context_to_inject` (`process.go:818`) is in the
  same never-produced baseline (`query_only_predicates.txt:19`), so the chat turn's `contextFacts`
  are also always empty. `shard_prompt_base` (`internal/articulation/prompt_assembler.go`) is in
  it too.
- **A published claim to correct:** `Docs/journeys/03-knowledge-delivery-and-window.md:224` relays,
  from `00-journey-map.md` stage 20, that `final_system_prompt` "is itself the kernel-relayed JIT
  output (i.e. a fact asserted from the same compiled-prompt pipeline …)". That is false. No rule
  derives it, no Go asserts it, and the repo's own gate says so.

### (3) Are `.nerd/agents/` atoms loadable today

- The loading code exists: `internal/prompt/loader.go:489` reads
  `.nerd/agents/<agent>/prompts.yaml`, `:585` walks `.nerd/agents`, `:529`
  (`LoadProjectPrompts`) reads `.nerd/prompts/*.yaml`, and `internal/prompt/sync/synchronizer.go:39`
  is rooted at `.nerd/agents`. The compiler has a live project-DB source with its own count
  (`compiler.go:1225-1233`, `breakdown.project`).
- I could not re-measure the 2026-09-17 `project=0` here: **this worktree has no `.nerd/`
  directory at all**, so there is nothing for the loader to find. That measurement needs the main
  checkout.
- It does not change this seam either way: a perfectly working project channel feeds a compile
  that the chat path does not run.

### (4) Is the persona wanted on the shard paths (coder/tester)

No, and it is not there today. `grep -rn "stevenMoorePersona" --include=*.go .` returned exactly
four hits, all in `cmd/nerd/chat` (the constant plus three call sites). Shard and session prompts
come from `internal/session/executor.go:1004-1046` (`jitCompiler.Compile`) then
`withProjectInstructions` / `withFileContext` (`:1064-1069`); nothing in `internal/session`,
`internal/core/shards` or `internal/shards` mentions the persona. **Not added there.**

### (5) What happens to a mandatory atom when the budget is tight

Two separate mechanisms, and the second is the dangerous one.

- **It is never silently cut.** The fitter shrinks renderings, sheds every optional atom, and then
  *refuses*: `internal/prompt/budget.go:552` ("prompt budget of %d tokens cannot hold the mandatory
  skeleton…") and `internal/prompt/compiler.go:1753-1757` ("refusing to compile a cut prompt"). The
  stated reasoning — a cut identity atom is a different constitution — is the right one.
- **But a refusal is downgraded to a prompt with no atoms in it at all.** The refusal is a plain
  `fmt.Errorf`, indistinguishable from "the corpus DB is locked", and
  `internal/session/executor.go:1038-1042` catches it and runs the turn on the 52-character string
  `"You are an AI assistant helping with software development."`. The in-file comment at
  `:1014-1037` flags exactly this contradiction and explicitly leaves the decision to whoever owns
  the loop. `cmd/nerd/chat/process_dream_delegation.go:413-421` has the same shape with a different
  fallback.
  **So a mandatory atom cannot be truncated, but it can be entirely absent from the prompt that
  actually runs — and precisely in the tight-budget case.** For a persona atom that is loss.
- **There is also a live cap that does drop mandatory atoms**, `internal/prompt/selector.go:396-479`
  (`mangleMandatoryTokenCap = 900000`, `mangleMandatoryAtomCap = 600`,
  `mangleMandatoryBudgetRatio = 0.90`, applied at `:443-469`). It is gated to
  `isMangleMandatoryContext` (`:487-496`: shard `legislator` or `mangle_repair` **and** language
  `mangle`), so it could not reach a chat persona atom — but it is proof that "mandatory" is not
  an absolute in this codebase.

### (6) Why `process_dream_delegation.go:421` uses the persona as a translator system prompt

- `buildShardInterpretationPrompt` (`cmd/nerd/chat/process_dream_delegation.go:379`) builds the
  system+user prompt for `interpretShardOutput` (`:462`), a single `CompleteWithSystem` call whose
  output `formatInterpretedResult` (`:502`) shows the user **as the shard's answer in chat**, with
  the raw output folded into a `<details>` block (`:513`). Callers: `process.go:611`,
  `process_dream.go:746`, `process_dream_delegation.go:274`, `:286`, `:364`.
- So the persona is there because this is user-facing chat prose, not an internal artifact. That is
  coherent.
- **The incoherence is which branch gets it.** When `m.jitCompiler != nil` and the
  `/analysis_translator` compile succeeds (`:413`), the system prompt is the JIT prompt and the
  persona is *absent*. Only when the compiler is nil or the compile fails/returns empty does `:421`
  return the persona. The architect's voice therefore appears on this surface **only when the JIT
  fails**, which is backwards. Per the brief I kept the behaviour byte-identical and recorded the
  question rather than deciding it — see Open.

## Decision

Three lines, as asked:

1. **The atom route is unsafe, so the persona is not moved.** The main chat turn compiles no JIT
   prompt: `final_system_prompt` has no producer anywhere in the repo
   (`internal/core/defaults/testdata/query_only_predicates.txt:22`, gate
   `TestGoQueriedPredicateBudget` green), the base prompt is empirically 0 bytes, and the persona is
   already the *entire* main-chat system prompt — so publishing it as a mandatory atom would not
   reposition it, it would delete it from chat. Even on the paths that *do* compile, a mandatory
   atom survives the fitter only to be discarded wholesale when a tight-budget refusal is
   downgraded to a fallback prompt (`internal/session/executor.go:1038-1042`).
2. **The constant stays, but there is now exactly one way to deliver it**:
   `cmd/nerd/chat/persona.go`. `withArchitectPersona(base)` puts the persona at byte offset 0 and
   the caller's role framing after it; all three call sites were repointed and no other file in the
   package may name the constant (enforced by a test). The old constant and its old home are gone —
   no forwarding alias, no second copy.
3. **The cost is counted and the position recorded.** `architectPersonaTokens()` measures the
   persona with `internal/prompt.EstimateTokens` — the same estimator the JIT budget is fitted with
   — and `recordArchitectPersonaDelivery` logs `N tokens at offset 0 of an M-token system prompt`
   on every delivery. Measured: **1,616 tokens**. The brief's "deducted from the JIT budget before
   compilation" has **no site today**: no persona-carrying path compiles a JIT prompt, so there is
   no compilation budget to subtract from, and I did not invent one. The bytes *are* already
   charged where it counts financially — the broker meters the assembled request at the outbound
   boundary and names this very append as the reason it counts there
   (`internal/broker/types.go:78-85`, `internal/broker/integrity_test.go:11-18`).

## Changes

All in this worktree; `go build ./...` clean and `go vet ./cmd/nerd/chat/` clean.

- **`cmd/nerd/chat/persona.go`** (new, 284 lines). Holds the persona text (moved byte-for-byte),
  a header explaining why it is a constant and not an atom, and the seam:
  - `architectPersona` — the text, unchanged, not edited by a single byte.
  - `architectPersonaOffset = 0` — the position, as a value a test can assert.
  - `withArchitectPersona(base string) string` — persona first, framing second; empty or
    whitespace-only base yields the persona alone (which is every main-chat turn today).
  - `architectPersonaTokens() int` — cost via `prompt.EstimateTokens`.
  - `recordArchitectPersonaDelivery(where, systemPrompt string)` — logs count and offset.
  - `(m Model) articulationSystemPrompt() string` — the production main-chat path, extracted from
    the middle of `processInput`'s closure so a test can call *the code that ships*.
- **`cmd/nerd/chat/process_knowledge.go`** — the 149-line `stevenMoorePersona` constant deleted
  from here (lines 269–418); the knowledge-synthesis call site now uses
  `withArchitectPersona(...)`, which also moves the persona from *after* the role sentence to
  *before* it. That reorder is deliberate: position is the invariant.
- **`cmd/nerd/chat/process.go`** — the twelve lines of inline kernel query + append replaced by
  `systemPrompt := m.articulationSystemPrompt()`.
- **`cmd/nerd/chat/process_dream_delegation.go`** — `return stevenMoorePersona, fallbackPrompt`
  becomes `return withArchitectPersona(""), fallbackPrompt`: byte-identical output, routed through
  the one seam, with the open question for the architect recorded in a comment beside it.
- **`cmd/nerd/chat/testdata/architect_persona.golden`** (new, 6,461 bytes) — the golden copy.
  `.gitattributes:12` already pins `*.golden text eol=lf`.
- **`.gitattributes`** — pins `cmd/nerd/chat/persona.go text eol=lf` as well. `core.autocrlf` is
  `true` on this box, so without it every newline *inside the raw string literal* becomes CRLF on
  checkout while the golden stays LF, and the gate on the architect's own words goes red over line
  endings. Verified with `git check-attr text eol`: both files report `eol: lf`.

Not done, on purpose: no atom YAML written, no `prompt_corpus.db` regenerated (nothing to
regenerate — no atom was added), nothing touched on the shard paths, `.nerd/config.json` untouched,
`nerd.exe` never run.

## Tests

`cmd/nerd/chat/persona_test.go` (new).

- **`TestMainChatSystemPrompt_AlwaysCarriesArchitectPersona`** — the mandated invariant. Builds a
  `Model` with a **real kernel** (`core.NewRealKernel()`) and runs the production
  `articulationSystemPrompt()` under two configs: a normal budget (1 M context window, 200 k JIT
  budget) and **the tightest the config schema allows** (`context_window.max_tokens = 1`,
  `jit.token_budget = 1`, `reserved_tokens = 1`, which `GetEffectiveJITConfig` clamps to
  budget 1 / reserve 0). Asserts the persona is present, that its index is exactly
  `architectPersonaOffset` (0), that the delivered bytes equal the source bytes, and that the token
  cost is counted. Logged from the run: `persona: 1616 tokens at offset 0; effective JIT budget:
  1 tokens (reserve 0)` — a persona 1,616× the entire configured budget, delivered whole.
  **Proven to fail:** with the `withArchitectPersona` call removed from `articulationSystemPrompt`,
  both subtests fail with *"main-chat system prompt does not carry the architect persona at all
  (prompt is 0 bytes)"* — which is also the empirical proof that `final_system_prompt` yields
  nothing.
- **`TestPersonaWordingUnchanged`** — pins `architectPersona` byte-for-byte against the golden, and
  reports the first differing line by number when it breaks. **Proven to fail:** changing
  "caffeinated chaos gremlin energy" to "…vibes" in the golden produced a line-5 diff; the golden
  was restored from a scratchpad copy and re-verified at 6,461 bytes.
- **`TestArchitectPersonaSurvivesEverySeam`** — the other two surfaces: the knowledge-synthesis
  prompt (persona leads, framing retained) and the translator fallback (persona alone, byte-identical
  to before), plus empty/whitespace bases.
- **`TestArchitectPersonaHasOneDeliverySeam`** — scans every `.go` file in `cmd/nerd/chat` and fails
  if any file other than `persona.go`/`persona_test.go` names the `architectPersona` constant
  (word-boundary regex, so the sanctioned `withArchitectPersona` / `architectPersonaTokens` /
  `architectPersonaOffset` API is allowed). This is the structural guarantee: a second delivery path
  is how the persona gets lost, so a second delivery path cannot be added silently.

Targeted suites, all green:
`go test ./cmd/nerd/chat/ ./internal/prompt/... ./internal/core/... ./internal/session/` →
`cmd/nerd/chat 45.9s ok`, `internal/prompt 21.3s ok`, `internal/prompt/marathon ok`,
`internal/prompt/sync ok`, `internal/core 283.1s ok`, `internal/core/defaults ok`,
`internal/core/defaults/policy ok`, `internal/core/shards ok`, `internal/session 204.3s ok`.

## Full test run

`go test ./...` from the worktree root: **85 `ok`, 4 `[no test files]`, 2 packages failing** —
`internal/autopoiesis` and `internal/perception`. Neither is touched by this change and neither
imports `cmd/nerd/chat`. All three failing tests are environmental:

- `TestOuroborosLoop_HotReload_LockedBinary` — *"timed out waiting for hotreload test execution"*,
  then a Windows `Access is denied` unlinking `hotreload_tool.exe` in `TempDir` cleanup.
- `TestThunderdome_SingleArgFunctionHandling` — *"Survived:false … Failure:timeout (context)"*
  after 16s.
  **Both pass when re-run on their own**: `go test ./internal/autopoiesis/ -run
  'TestOuroborosLoop_HotReload_LockedBinary|TestThunderdome_SingleArgFunctionHandling'` →
  `ok codenerd/internal/autopoiesis 3.248s`. They are compile-a-tool-and-exec timeouts that lose
  their race under whole-suite load on this box, not regressions.
- `TestCodexCLIClient_RunHealthProbe_Success` — *"codex CLI timed out after 5m0s"*. Codex is
  documented as down; this probe cannot pass in this environment.

Before the change the same three were not measured in this worktree, so I am reporting what I
observed rather than asserting a clean baseline: the two autopoiesis failures are proven flaky by
the isolated re-run, and the Codex probe is proven external by the outage.

Also run, all green: `go build ./...`; `go vet ./cmd/nerd/chat/`;
`go test ./cmd/nerd/chat/ ./internal/prompt/... ./internal/core/... ./internal/session/`
(9 packages, `internal/core` 283s, `internal/session` 204s, `cmd/nerd/chat` 46s).

## Open

Ordered by how much they cost the vision, not by effort.

1. **`final_system_prompt` has no producer, so the main TUI chat turn runs with no JIT prompt, no
   atoms, no mandatory skeleton, no ordering and no budget.** The architect's own chat surface is
   the one surface the JIT does not reach. This is a seam of its own and much larger than S14; S14
   only made the persona's delivery safe *given* that fact. Same file, same cause:
   `context_to_inject` is also never produced, so the chat turn's context facts are always empty.
2. **`Docs/journeys/00-journey-map.md` stage 20 and `03-knowledge-delivery-and-window.md:224` state
   that `final_system_prompt` is kernel-relayed JIT output.** It is not. Those two documents should
   be corrected before anyone plans against them.
3. **The translator asymmetry** (`process_dream_delegation.go:397-421`): the architect's voice
   reaches the user-facing shard-interpretation text only when the JIT compile *fails*. Should the
   `/analysis_translator` prompt carry the persona too, or should the fallback drop it? His call —
   behaviour left identical.
4. **A refused JIT compile is downgraded to a prompt with no constitution in it**
   (`internal/session/executor.go:1014-1042`, self-flagged in the code). Until that is decided, no
   identity or safety content is genuinely safe as a mandatory atom on the compiling paths either.
5. **`project=0` for `.nerd/agents/` could not be re-measured** — this worktree has no `.nerd/`
   directory. Needs the main checkout.
6. **The prompt corpus has no `go generate`.** `prompt_corpus.db` is a 14 MB committed binary
   regenerated only by remembering to run `go run ./cmd/tools/prompt_builder` with a Gemini key. A
   YAML edit that is not followed by that command ships as a no-op.
7. **The persona is 1,616 tokens on every chat turn and every knowledge-synthesis turn**, now
   measured and logged for the first time. Whether that is the right spend is a question the number
   makes askable; it was not askable before.
