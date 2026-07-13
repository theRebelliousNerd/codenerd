# 06 — TUI Chat Surface (`cmd/nerd/chat`)

> Last verified: 2026-07-13  
> Stack: Bubble Tea (`tea.Model`), Glamour markdown, Lipgloss styles, activity pulse UI.

## 1. Role

The chat package is the **primary product UX**. It owns:

- Multi-turn conversation state
- Slash command routing
- System boot / Cortex wiring for interactive sessions
- Delegation, multistep decomposition, dream state
- Wizards (config, north star, agents)
- Glass-box visibility of ongoing work

## 2. Control loop

```
User keystrokes
  → model_key_handler / Update (model_update.go)
  → processInput (process.go)   // panic-recovered
  → slash command? → commands.go switch → handlers
  → else perception/intent → kernel/session path
  → view.go render (messages, spinners, glass box, panes)
```

### Panic recovery

`processInput` wraps work in a deferred `recover`, logs stack via `logging.API`, and returns `errorMsg` so `isLoading` cannot stick forever after a panic (`chat/process.go` header + recover block).

## 3. Slash command catalog (router)

Defined primarily in `chat/commands.go` (non-exhaustive of every alias):

### Session / meta

`/quit` `/exit` `/q`, `/continue` `/resume`, `/usage`, `/clear`, `/reset`, `/model`,  
`/new-session`, `/sessions`, `/load-session`, `/help` `/h` `/?`, `/status`

### Knowledge & planning

`/knowledge`, `/legislate`, `/clarify`, `/launchcampaign`, `/northstar` `/vision` `/spec`,  
`/learn`, `/agents`, `/alignment` `/align`, `/spawn`, `/ingest`

### Workspace / files

`/init`, `/scan`, `/refresh-docs` `/scan-docs`, `/scan-path`, `/scan-dir`,  
`/read`, `/mkdir`, `/write`, `/search`, `/patch`, `/edit`, `/append`, `/pick`

### Coding verbs

`/review`, `/security`, `/analyze`, `/test`, `/fix`, `/refactor`, `/explain`, `/explain-off`

### Kernel / safety exploration

`/query`, `/why`, `/logic`, `/shadow`, `/whatif`, `/approve`,  
`/reject-finding`, `/accept-finding`, `/review-accuracy`

### Systems

`/config`, `/embedding`, `/glassbox`, `/transparency`, `/campaign`, `/tool`,  
`/define-agent` `/agent`, `/reflection`

Handlers live across `commands_handlers.go`, `commands_handlers_files.go`,  
`commands_handlers_analysis.go`, `commands_handlers_evolution.go`, etc.

## 4. Major subsystems inside chat/

| Concern | Primary files |
|---------|----------------|
| Boot | `session_boot.go`, `session_shared_boot.go`, `session.go` |
| Input / OODA | `process.go`, `process_follow_up.go`, `process_continuation.go`, `process_knowledge.go`, `process_sync.go` |
| Dream | `process_dream.go`, `process_dream_delegation.go` |
| Multistep | `multistep_corpus.go`, `multistep_decomposer.go` |
| Delegation | `delegation.go`, `delegation_modes.go` |
| Model | `model_types.go`, `model_update.go`, `model_handlers.go`, `model_helpers.go`, `model_key_handler.go`, `model_session_context.go` |
| View | `view.go`, `glass_box.go`, activity pulse helpers |
| Campaigns | `campaign.go` |
| Wizards | `config_wizard.go`, `config_wizard_steps.go`, `northstar_wizard.go` |
| Review | `review_aggregator.go` |
| Reflection | `reflection.go` |

## 5. Dependencies (chat → internal)

From `session_boot.go` imports (illustrative, not exclusive):

`config`, `core`, `core/shards`, `perception`, `prompt`, `retrieval`, `session`, `shards`, `shards/system`, `store`, `embedding`, `browser`, `autopoiesis` (+ prompt_evolution), `northstar`, `transparency`, `verification`, `world`, `tactile`, `ux`, `features`, `logging`, `types`, `context` (compression), `sqlpragmas`, `system`.

This import set is why CLI boot failures often present as “chat won’t start” even when the root cause is an internal package.

## 6. Design notes / hazards

1. **File gravity** — several 800–1000+ line files; refactors should be mechanical splits with tests.
2. **Legacy vs shared boot** — both `performSystemBootLegacy` and newer shared boot exist; prefer consolidating call sites carefully.
3. **Domain shard comment** — coder/tester/reviewer imports removed toward JIT atoms; ensure slash verbs still resolve to working paths.
4. **Bubble Tea concurrency** — cmds run async; shared model mutation must stay message-driven.
