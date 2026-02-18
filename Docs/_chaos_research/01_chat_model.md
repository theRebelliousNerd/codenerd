# 01 - Chat Model & Input Handling

> Chaos Testing Research Document - Chat Input Pipeline Analysis
> Source: `cmd/nerd/chat/model_types.go` (646 lines)

## 1. Model Struct - Input-Relevant Fields

### UI Input Components
| Field | Type | Purpose |
|-------|------|---------|
| `textarea` | `textarea.Model` | Primary text input widget (Bubbles) |
| `viewport` | `viewport.Model` | Chat history display |
| `errorVP` | `viewport.Model` | Error panel viewport |
| `spinner` | `spinner.Model` | Loading indicator |

### State Flags (Input Blocking)
| Field | Type | Purpose | Chaos Relevance |
|-------|------|---------|-----------------|
| `isLoading` | `bool` | Blocks input during LLM calls | Race: can be toggled mid-keystroke |
| `isBooting` | `bool` | Blocks input during startup | If never cleared, input is permanently dead |
| `ready` | `bool` | Set after first WindowSizeMsg | Input before ready = undefined behavior |
| `isInterrupted` | `bool` | User pressed Ctrl+X | May not reset properly |

### Awaiting Flags (LEGACY - scattered state)
These are the OLD flags, still present alongside the unified `inputMode`:
| Field | Type | Notes |
|-------|------|-------|
| `awaitingClarification` | `bool` | Dual with `inputMode == InputModeClarification` |
| `awaitingPatch` | `bool` | Dual with `inputMode == InputModePatch` |
| `awaitingAgentDefinition` | `bool` | Dual with `inputMode == InputModeAgentWizard` |
| `awaitingConfigWizard` | `bool` | Dual with `inputMode == InputModeConfigWizard` |
| `awaitingNorthstar` | `bool` | NO corresponding InputMode enum! |
| `awaitingOnboarding` | `bool` | Dual with `inputMode == InputModeOnboarding` |
| `awaitingKnowledge` | `bool` | NO corresponding InputMode enum! |
| `launchClarifyPending` | `bool` | Dual with `inputMode == InputModeCampaignLaunch` |

**CHAOS INSIGHT**: `awaitingNorthstar` and `awaitingKnowledge` have NO corresponding `InputMode` value. This means the "unified" InputMode state machine is INCOMPLETE. Setting `awaitingKnowledge=true` without changing `inputMode` could cause inconsistent routing.

### Unified Input Mode (replacement, incomplete migration)
| Value | Constant | Description |
|-------|----------|-------------|
| 0 | `InputModeNormal` | Default chat input |
| 1 | `InputModeClarification` | Awaiting clarification |
| 2 | `InputModePatch` | Awaiting patch (--END-- terminated) |
| 3 | `InputModeAgentWizard` | Agent definition wizard |
| 4 | `InputModeConfigWizard` | Config wizard |
| 5 | `InputModeCampaignLaunch` | Campaign launch clarification |
| 6 | `InputModeOnboarding` | Onboarding wizard |

### ViewMode Enum
| Value | Constant | Notes |
|-------|----------|-------|
| 0 | `ChatView` | Primary input view |
| 1 | `ListView` | Session list |
| 2 | `FilePickerView` | File picker |
| 3 | `UsageView` | Usage dashboard |
| 4 | `CampaignPage` | Campaign dashboard |
| 5 | `PromptInspector` | JIT inspector |
| 6 | `AutopoiesisPage` | Autopoiesis dashboard |
| 7 | `ShardPage` | Shard console |

### BootStage Enum
| Value | Constant |
|-------|----------|
| 0 | `BootStageBooting` |
| 1 | `BootStageScanning` |

### ContinuationMode Enum
| Value | Constant | Description |
|-------|----------|-------------|
| 0 | `ContinuationModeAuto` | Fully automatic |
| 1 | `ContinuationModeConfirm` | Pause after each step |
| 2 | `ContinuationModeBreakpoint` | Auto reads, pause mutations |

## All tea.Msg Types (lines 511-645)

| Type | Kind | Description |
|------|------|-------------|
| `responseMsg` | `string` | Raw LLM response text |
| `errorMsg` | `error` | Error propagation |
| `windowSizeMsg` | `tea.WindowSizeMsg` | Terminal resize |
| `clarificationMsg` | `ClarificationState` | Request clarification from user |
| `clarificationReply` | `string` | User's clarification answer |
| `ShardResultPayload` | struct | Completed shard execution |
| `ClarifyUpdate` | struct | Auto-clarification state update |
| `assistantMsg` | struct | Rich response (Surface + ShardResult + ClarifyUpdate + DreamHypothetical) |
| `memUsageMsg` | struct | Memory sampling (Alloc, Sys bytes) |
| `campaignStartedMsg` | struct | Campaign started (campaign, orch, channels) |
| `campaignProgressMsg` | `*campaign.Progress` | Campaign progress update |
| `campaignEventMsg` | `OrchestratorEvent` | Orchestrator event |
| `campaignCompletedMsg` | `*campaign.Campaign` | Campaign done |
| `campaignErrorMsg` | struct | Campaign error |
| `continueMsg` | struct | Next continuation step |
| `continuationInitMsg` | struct | Start continuation chain |
| `interruptMsg` | struct | Ctrl+X pressed |
| `confirmContinueMsg` | struct | Enter to continue |
| `continuationDoneMsg` | struct | All steps done |
| `northstarDocsAnalyzedMsg` | struct | Northstar doc analysis result |
| `initCompleteMsg` | struct | Init complete (result + learning store) |
| `bootCompleteMsg` | struct | System boot complete (components + err) |
| `statusMsg` | `string` | Background process status |
| `glassBoxEventMsg` | `GlassBoxEvent` | Glass Box inline event |
| `traceUpdateMsg` | struct | Mangle derivation trace |
| `onboardingCompleteMsg` | struct | Onboarding wizard finished |
| `onboardingCheckMsg` | struct | First-run detection trigger |
| `knowledgeGatheredMsg` | struct | Specialist consultations complete |
| `alignmentCheckMsg` | struct | Northstar alignment check result |

### Shutdown Coordination
| Field | Type | Chaos Relevance |
|-------|------|-----------------|
| `shutdownOnce` | `*sync.Once` | Pointer to allow Model copy - what if nil? |
| `shutdownCtx` | `context.Context` | Root context for all background ops |
| `shutdownCancel` | `context.CancelFunc` | Cancels shutdownCtx |
| `goroutineWg` | `*sync.WaitGroup` | Tracks background goroutines |

### Channel Fields (potential deadlock vectors)
| Field | Type | Buffer? |
|-------|------|---------|
| `statusChan` | `chan string` | Unknown buffer size |
| `campaignProgressChan` | `chan campaign.Progress` | Unknown buffer size |
| `campaignEventChan` | `chan campaign.OrchestratorEvent` | Unknown buffer size |
| `glassBoxEventChan` | `<-chan transparency.GlassBoxEvent` | Receive-only |
| `toolEventChan` | `<-chan transparency.ToolEvent` | Receive-only |
| `autopoiesisListenerCh` | `<-chan struct{}` | Closed when listener stops |

## 2. Update() Flow & KeyEnter Handling

> Source: `cmd/nerd/chat/model_update.go` (650+ lines read)

### Update() Entry Point (line 24)

The `Update()` function is the Bubbletea message handler. It receives value-copy `Model` (not pointer receiver). Performance instrumented with 100ms warning threshold (lines 25-37).

### KeyEnter Path (lines 236-260)

```
tea.KeyMsg received
  -> line 236: case tea.KeyEnter
     -> line 238: if msg.Alt => break (let textarea insert newline)
     -> line 244: if msg.Paste => break (bracketed paste, insert newline)
     -> line 249: if splitPane.FocusRight && logicPane != nil => toggle expand, return
     -> line 255: if !m.isLoading:
         -> line 256: if m.awaitingClarification => handleClarificationResponse()
         -> line 259: else => handleSubmit()
     -> line 260: (implicit fall-through if isLoading=true: SILENTLY DROPPED)
```

**CRITICAL**: When `isLoading=true`, Enter keypress is silently swallowed at line 255. There is no feedback to the user that their input was ignored.

### Input Blocking When isLoading=true

- **Line 255**: `if !m.isLoading` - gates KeyEnter from reaching handleSubmit
- **Line 464**: `if !m.isLoading` - gates ALL regular key input from reaching textarea.Update()

This means when `isLoading=true`:
1. Enter does nothing (silent drop)
2. Typing does nothing (textarea doesn't receive keystrokes)
3. User has zero feedback that input is blocked

### How Keyboard Events Reach the Textarea

1. `tea.KeyMsg` enters `Update()` at line 46
2. Error panel focus check (lines 50-62) - if focused, swallows most keys
3. Global keybindings (lines 65-118) - Ctrl+C, Ctrl+X, Shift+Tab, Esc
4. ViewMode routing (lines 121-231) - if not ChatView, keys go to sub-views
5. ChatView key handling (lines 234-323) - Enter, Up, Down, Tab
6. Alt-key bindings (lines 326-461) - feature toggles
7. **Line 464**: `if !m.isLoading { m.textarea, tiCmd = m.textarea.Update(msg) }` - finally reaches textarea

### Race Conditions in State Transitions

1. **isLoading toggle race (line 73-78 vs line 581/608)**: Ctrl+X sets `isLoading=false` at line 78. An in-flight goroutine returning `assistantMsg` also sets `isLoading=false` at line 581. If both fire, no crash but `isInterrupted` may not get cleared.

2. **awaitingClarification vs inputMode (lines 256 vs 363)**: `handleClarificationResponse()` is gated by legacy `awaitingClarification` bool (line 256) but the newer `inputMode` field is NOT checked here. If `inputMode != InputModeClarification` but `awaitingClarification == true`, the system follows the legacy path.

3. **Value-copy Model pattern**: `Update()` uses value receiver `(m Model)`. Every branch must return the modified `m`. If a deep branch forgets to return, stale state propagates.

4. **clarificationReply handling (line 548-550)**: Accesses `m.clarificationState.PendingIntent` and `m.clarificationState.Context` without nil-checking `m.clarificationState`. If `clarificationState` is nil when a `clarificationReply` msg arrives, this is a **nil pointer dereference panic**.

5. **Ctrl+X interrupt during continuation (line 73-91)**: Sets `isLoading=false` and `isInterrupted=true` but does NOT clear `pendingSubtasks`, `continuationStep`, or `continuationTotal`. A subsequent `/continue` command could resume stale state.

## 3. handleSubmit() & Input Validation

> Source: `cmd/nerd/chat/model_handlers.go` (lines 77-191)

### Validation Performed

1. **Line 78**: `input := strings.TrimSpace(m.textarea.Value())` - Only validation: trim whitespace
2. **Line 79**: `if input == ""` - Empty check after trim. If empty AND `pendingSubtasks > 0`, treats Enter as continuation confirmation (line 81-84). Otherwise returns nil (line 85).

### What is NOT Validated (Chaos Attack Surface)

| Missing Validation | Impact | Chaos Test |
|--------------------|--------|------------|
| **No size limit** | Textarea has CharLimit but raw value is unchecked | Send 1MB+ string |
| **No encoding check** | Invalid UTF-8, null bytes, control chars pass through | Send `\x00\xff` sequences |
| **No content sanitization** | Mangle injection, format string injection | Send `%s%n` or `/atom` syntax |
| **No rate limiting** | Rapid-fire submits while isLoading races | Spam Enter at 100Hz |
| **No input history limit** | `inputHistory` grows unbounded (line 132-134) | Send 100K unique inputs |
| **No pendingPatchLines limit** | Patch accumulation is unbounded (line 106) | Stay in patch mode, send 100K lines |
| **No regex/pattern validation** | Commands parsed with simple string prefix | Send `/` followed by megabytes |

### Command Routing Path

```
handleSubmit() line 77
  -> TrimSpace + empty check (lines 78-86)
  -> Patch mode check (lines 89-109)
     -> awaitingPatch: accumulate until "--END--"
     -> NO limit on accumulated lines
     -> applyPatchResult() called with raw patch string
  -> Command check (line 112): strings.HasPrefix(input, "/")
     -> handleCommand(input) - routes to command handlers
  -> Campaign clarification accumulation (lines 118-123)
     -> launchClarifyPending: appends input to launchClarifyAnswers
     -> NO limit on accumulated answers
  -> Add to history (lines 126-135)
  -> Textarea reset (line 138)
  -> Wizard routing cascade (lines 145-162):
     -> awaitingAgentDefinition -> handleAgentWizardInput
     -> awaitingConfigWizard -> handleConfigWizardInput
     -> awaitingNorthstar -> handleNorthstarWizardInput
     -> awaitingOnboarding -> handleOnboardingInput
  -> Set isLoading=true (line 164) *** AFTER wizard checks ***
  -> Negative feedback check (lines 167-169)
  -> Dream confirmation/correction/execution checks (lines 172-184)
  -> processInput(input) in background goroutine (lines 187-190)
```

### Critical Observations

1. **isLoading set AFTER wizard routing (line 164)**: If a wizard handler returns without setting isLoading, the model stays in a non-loading state. This is correct for wizards but means there is a window between lines 126-163 where the user message is added to history but isLoading is not yet true.

2. **Multiple awaiting checks are sequential, not exclusive (lines 145-162)**: If somehow BOTH `awaitingAgentDefinition` AND `awaitingConfigWizard` are true, only the first match executes. The second flag remains set, causing the next submit to go to the wrong wizard.

3. **processInput runs as fire-and-forget goroutine (line 189)**: The `tea.Batch(spinner.Tick, processInput(input))` launches processInput as a Bubbletea command. It returns a `tea.Msg` to the Update loop. If processInput panics in the goroutine, the panic propagates to the Bubbletea runtime and crashes the entire TUI.

4. **Input history dedup is weak (line 132)**: Only checks if the LAST entry matches. Sending alternating "a", "b", "a", "b" fills the unbounded history.

## 4. processInput() & OODA Loop

> Source: `cmd/nerd/chat/process.go` (lines 49-300)

### Goroutine Pattern

`processInput()` returns a `tea.Cmd` (line 49), which is a closure `func() tea.Msg` (line 50). Bubbletea executes this closure in its own goroutine. The function accesses the Model's fields via the captured value-copy `m`. This is a **fire-and-forget** pattern - the function returns a single `tea.Msg` when done, but there is NO mechanism to cancel it mid-execution beyond the context timeout.

```
processInput(input) -> tea.Cmd closure -> runs in Bubbletea goroutine
   -> returns one of: assistantMsg | errorMsg | clarificationMsg
```

### Key Guards (lines 52-57)

- `m.transducer == nil` -> returns `errorMsg`
- `m.client == nil` -> returns `errorMsg`

No guard for `m.kernel == nil` at entry, though kernel is nil-checked later at line 81/179.

### Context & Timeout (line 59)

```go
ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().OODALoopTimeout)
```

**CRITICAL**: Uses `context.Background()`, NOT `m.shutdownCtx`. This means:
1. The goroutine's context is independent of shutdown
2. If the user presses Ctrl+C, `performShutdown()` cancels `shutdownCtx`, but this goroutine's context continues until its own timeout expires
3. The goroutine may complete AFTER the TUI has exited, potentially causing issues

### How Errors Propagate Back to the UI

Errors return as `errorMsg` type (line 53, 56, 156). In `model_update.go`, the `errorMsg` case handler sets `m.err` and `m.showError`. The error is displayed in the error panel viewport.

**However**: If the goroutine panics (not returns an error), the panic propagates through Bubbletea's goroutine runner, crashing the entire TUI with an unrecoverable panic.

### Timeout Handling

- **OODALoopTimeout** (line 59): Configurable via `config.GetLLMTimeouts()`. If this fires, the context is cancelled and the LLM call should return a context deadline exceeded error.
- **No per-step timeout**: Individual steps (perception, kernel seeding, reflection, workspace scan, articulation) share the single OODA timeout. A slow perception step could starve subsequent steps.
- **No explicit timeout feedback**: If the context deadline fires, the user sees a generic error, not a specific "timeout" indication.

### OODA Loop Steps (within processInput)

1. **Follow-up detection** (line 95-99): Pre-perception check against `m.lastShardResult`
2. **Perception** (lines 101-161): Parses intent via PerceptionFirewall shard or direct transducer
3. **Kernel seeding** (lines 178-246): Asserts `user_intent` facts, retracts stale facts
4. **Reflection** (lines 248-255): Optional self-reflection on input
5. **Memory operations** (lines 259-272): Process `promote_to_long_term`, `forget` instructions
6. **Dream state** (line 276-279): If verb is `/dream`, delegates to `handleDreamState`
7. **Assault campaign** (lines 283-287): If input matches assault pattern
8. **Auto-clarification** (lines 290-309): If request looks like campaign/plan

### Stale State Access

The goroutine captures a **value-copy** of `Model` (since `processInput` is on value receiver). This means:
- `m.launchClarifyPending`, `m.launchClarifyAnswers` (line 68-73) are READ from the snapshot at call time and WRITTEN to the copy. These writes are lost - they never propagate back to the real Model.
- `m.lastReflection` (line 249) is also written to the copy, not the real Model.
- Only the returned `tea.Msg` actually updates the real Model in `Update()`.

**CHAOS INSIGHT**: Line 68-73 modifies `m.launchClarifyAnswers` inside the goroutine, but this modification is lost because it's on a value copy. The same accumulation logic also exists in `handleSubmit()` (model_handlers.go:118-123), creating a dual-write with only the handleSubmit version persisting.

## 5. InitChat() & Configuration

> Source: `cmd/nerd/chat/session.go` (lines 70-206)

### Textarea Configuration (lines 84-90)

```go
ta := textarea.New()
ta.Placeholder = "System initializing..."
ta.Prompt = "┃ "
ta.CharLimit = 0     // *** UNLIMITED ***
ta.SetWidth(80)
ta.SetHeight(3)      // 3 lines default
ta.ShowLineNumbers = false
```

### CharLimit Setting

**Line 87: `ta.CharLimit = 0`** - This sets the character limit to **ZERO**, which in the Bubbles textarea library means **UNLIMITED**. There is no upper bound on input size.

**CHAOS IMPLICATION**: A user (or automated test) can type or paste an arbitrarily large string. The textarea will accept it, `handleSubmit()` will `TrimSpace` it, and `processInput()` will pass it to the LLM transducer. There is no size guard at any point in the pipeline.

### Other Input-Related Config

| Setting | Value | Location | Notes |
|---------|-------|----------|-------|
| `CharLimit` | 0 (unlimited) | session.go:87 | No input size protection |
| `Width` | 80 (initial) | session.go:88 | Resized on WindowSizeMsg |
| `Height` | 3 lines | session.go:89 | Fixed input area height |
| `ShowLineNumbers` | false | session.go:90 | Clean input UI |
| `statusChan` | buffered(10) | session.go:192 | Status updates channel |
| `mouseEnabled` | true | session.go:196 | Mouse capture on by default |
| `isBooting` | true | session.go:190 | Blocks input until boot complete |
| `showError` | true | session.go:177 | Error panel visible by default |

### Initialization State

At `InitChat()` completion, the following are **nil/uninitialized**:
- `kernel` (line 187)
- `shardMgr` (line 188)
- `client` (line 189)
- `transducer` (not set)
- `executor` (not set)
- `emitter` (not set)
- `virtualStore` (not set)
- `compressor` (not set)
- `autopoiesis` (not set)

All backend components are set during `performSystemBoot()` which runs asynchronously. The `isBooting=true` flag prevents input, but there is NO guard on individual component nil-ness beyond the transducer/client checks in `processInput()`.

### Shutdown Coordination

- `shutdownOnce`: `&sync.Once{}` (line 198) - pointer-based to survive Model value copies
- `shutdownCtx`/`shutdownCancel`: Created from `context.Background()` (line 146)
- `goroutineWg`: `&sync.WaitGroup{}` (line 201) - pointer-based

## 6. CHAOS FAILURE PREDICTIONS

Based on comprehensive analysis of the 5 files above, here are specific chaos failure predictions:

### CRITICAL Severity

| # | Prediction | File:Line | Details |
|---|-----------|-----------|---------|
| 1 | **Nil pointer panic on clarificationReply** | `model_update.go:550` | If `clarificationReply` msg arrives when `m.clarificationState` is nil, accessing `.PendingIntent` and `.Context` causes panic. No nil guard. Trigger: send `clarificationReply` msg type to Update() when no clarification is active. | 
| 2 | **Unbounded input causes OOM** | `session.go:87` + `model_handlers.go:78` | `CharLimit=0` (unlimited). Paste 100MB string -> textarea accepts -> TrimSpace allocates copy -> processInput sends to LLM. No size guard anywhere in pipeline. |
| 3 | **Goroutine panic kills TUI** | `process.go:50` | `processInput()` runs in Bubbletea goroutine with no `recover()`. Any panic in perception, kernel seeding, or articulation propagates through Bubbletea and crashes the entire terminal UI. Trigger: send input that causes nil deref in transducer. |
| 4 | **processInput uses context.Background(), not shutdownCtx** | `process.go:59` | On Ctrl+C shutdown, `performShutdown()` cancels `shutdownCtx`, but the processInput goroutine uses an independent `context.Background()`. The goroutine continues running after TUI exit, potentially writing to closed channels. |

### HIGH Severity

| # | Prediction | File:Line | Details |
|---|-----------|-----------|---------|
| 5 | **Dual-state inconsistency: awaiting flags vs inputMode** | `model_types.go:203-215,361-363` | `awaitingNorthstar` and `awaitingKnowledge` have NO corresponding `InputMode` enum value. Setting these flags creates split-brain state where `inputMode` says "Normal" but the flag says "awaiting". New code checking `inputMode` will route incorrectly. |
| 6 | **Stale continuation state after Ctrl+X** | `model_update.go:73-91` | Ctrl+X sets `isLoading=false` and `isInterrupted=true` but does NOT clear `pendingSubtasks`, `continuationStep`, or `continuationTotal`. Next `/continue` resumes from stale state. |
| 7 | **Lost writes in processInput goroutine** | `process.go:68-73` | `m.launchClarifyAnswers` is modified inside goroutine on a value-copy of Model. These writes are silently lost. The same logic in `handleSubmit()` (model_handlers.go:118-123) does persist but creates data inconsistency. |
| 8 | **Multiple wizard flags can be true simultaneously** | `model_handlers.go:145-162` | Sequential `if` checks mean if both `awaitingAgentDefinition` AND `awaitingConfigWizard` are true, only the first executes. The second flag remains stuck forever, hijacking future submits after the first wizard completes. |

### MEDIUM Severity

| # | Prediction | File:Line | Details |
|---|-----------|-----------|---------|
| 9 | **Unbounded input history growth** | `model_handlers.go:132-134` | `inputHistory` is appended without limit. Dedup only checks last entry. Alternating distinct inputs grow the slice forever. Over a long session (thousands of turns), this leaks memory. |
| 10 | **Unbounded patch accumulation** | `model_handlers.go:89-108` | In patch mode (`awaitingPatch=true`), lines accumulate in `pendingPatchLines` with no limit. Stay in patch mode and send 100K lines -> OOM. No timeout on patch mode either - it persists across turns indefinitely. |
| 11 | **Silent input drop when isLoading** | `model_update.go:255,464` | When `isLoading=true`, both Enter and regular typing are silently swallowed. User gets zero feedback. In chaos testing, rapid input during loading appears to "disappear" unpredictably. |
| 12 | **Renderer re-creation on every resize** | `model_update.go:536-542` | Every `WindowSizeMsg` creates a new `glamour.TermRenderer` and re-renders ALL history. Rapid resize events (terminal window dragging) cause O(N) re-renders per event, causing visible lag. |
| 13 | **No encoding validation on input** | `model_handlers.go:78` | Only `strings.TrimSpace()` is applied. Null bytes (`\x00`), invalid UTF-8 sequences, or control characters pass through to the LLM transducer and Mangle kernel. Could cause parsing failures or assertion errors deep in the stack. |
| 14 | **statusChan buffer of 10 can block** | `session.go:192` | `statusChan` has buffer size 10. If the consumer (UI Update loop) is slow and producers (processInput, system shards) emit >10 status messages rapidly, the 11th send blocks the goroutine until a slot opens. |
