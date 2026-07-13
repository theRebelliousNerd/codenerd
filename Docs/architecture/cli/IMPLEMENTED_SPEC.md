# codeNERD CLI — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Binary: `nerd` / `nerd.exe`  
> Primary sources: `cmd/nerd/`, `cmd/nerd/chat/`, `cmd/nerd/ui/`  
> Scale: **113** non-test Go files ≈ **38,110** lines; **55** test files ≈ **16,755** lines  

## 1. Overview

The codeNERD CLI is the operator and user interface for a **logic-first neuro-symbolic coding agent**. Unlike a thin remote-API client, this binary **hosts** the agent runtime: it boots Cortex (Mangle kernel, VirtualStore, system shards, perception/articulation, stores, embeddings) and drives multi-turn work.

It is implemented as **three tightly coupled surfaces**:

1. **Cobra command tree** (`cmd/nerd/*.go`) — scriptable verbs, automation, focused tools.  
2. **Interactive Bubble Tea chat** (`cmd/nerd/chat/`) — default UX, slash commands, wizards, multistep, campaigns.  
3. **UI component library** (`cmd/nerd/ui/`) — panes, diff view, campaign/JIT/shard pages.

### Key characteristics

| Property | Value |
|----------|-------|
| Default mode | Interactive chat (`rootCmd.RunE`) |
| Command framework | Cobra |
| TUI framework | Bubble Tea + Lipgloss/Glamour |
| Workspace | `--workspace` / CWD + `.nerd/` via `init` |
| API key | `--api-key` or `ZAI_API_KEY` |
| Default timeout | 25m (config-overridable) |
| Logic kernel | Google Mangle via `internal/core` |
| Vector/sqlite | CGO build with `sqlite_headers` |
| Architecture slogan | “Logic determines Reality; the Model merely describes it.” (`main.go` Long) |

### High-level control flow

```
nerd [subcommand]
   │
   ├─ (no args) ──► chat.RunInteractiveChat ──► boot Cortex ──► multi-turn OODA
   │
   └─ subcommand ──► RunE ──► GetOrBootCortex / specialized runner ──► print
```

Fact-flow (always):

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation → TUI/stdout
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Cobra root + global flags | **Implemented** | `main.go` |
| Interactive chat TUI | **Implemented** | `chat/` large |
| System boot assembly | **Implemented** | `session_boot.go` (+ shared boot) |
| Slash command router | **Implemented** | `chat/commands.go` |
| Direct action verbs | **Implemented** | `cmd_direct_actions.go` |
| Campaign CLI + chat | **Implemented** | `cmd_campaign.go`, chat campaign |
| Auth multi-engine | **Implemented** | `cmd_auth.go` |
| Browser automation cmds | **Implemented** | `cmd_browser.go` |
| Mangle check + LSP | **Implemented** | `cmd_mangle_*.go` |
| DOM surgical commands | **Implemented** | `dom_*.go` |
| Embedding management | **Implemented** | `embedding_cmd.go` |
| Glass box / transparency | **Implemented** | transparency + glass_box |
| Interactive refine loop | **Implemented** | `cmd_interactive.go` |
| Panic recovery in chat | **Implemented** | `process.go` |
| Full Cobra↔slash parity | **Partial** | documented asymmetries |
| Hot-path test density | **Partial** | see testing doc |
| Domain shard direct imports | **Removed/legacy** | JIT path comments in boot |

**Overall:** living production CLI — **not** pre-implementation.

---

## 3. Source inventory (largest files)

### 3.1 Package layout

```
cmd/nerd/
  main.go                 # rootCmd, flags, AddCommand hub
  cmd_*.go                # cobra implementations by domain
  dom_*.go                # CodeDOM CLI
  embedding_cmd.go
  campaign_jit_provider.go
  system_results.go, stats.go, cli_test.go
  chat/                   # Bubble Tea application
  ui/                     # shared UI widgets/pages
```

### 3.2 Top non-test sources (line counts ≈)

| Path | Lines | Purpose |
|------|------:|---------|
| `cmd/nerd/chat/process.go` | 1085 | Input processing, recover, intent pipeline |
| `cmd/nerd/chat/session_boot.go` | 1015 | System boot / Cortex assembly |
| `cmd/nerd/cmd_campaign.go` | 944 | Campaign cobra tree |
| `cmd/nerd/chat/model_update.go` | 933 | tea.Update |
| `cmd/nerd/chat/multistep_corpus.go` | 929 | Multistep corpus |
| `cmd/nerd/chat/review_aggregator.go` | 925 | Review aggregation |
| `cmd/nerd/chat/model_session_context.go` | 869 | Session context |
| `cmd/nerd/chat/commands_handlers.go` | 865 | Slash handlers |
| `cmd/nerd/ui/splitpane.go` | 864 | Split panes |
| `cmd/nerd/chat/northstar_wizard.go` | 743 | North-star wizard |
| `cmd/nerd/chat/model_handlers.go` | 710 | Model handlers |
| `cmd/nerd/ui/diffview.go` | 687 | Diff UI |
| `cmd/nerd/chat/multistep_decomposer.go` | 683 | Decomposition |
| `cmd/nerd/chat/view.go` | 662 | View |
| `cmd/nerd/chat/model_types.go` | 657 | Types |
| `cmd/nerd/chat/process_dream.go` | 653 | Dream state |
| `cmd/nerd/chat/delegation.go` | 611 | Delegation |
| `cmd/nerd/main.go` | ~350+ | Entry + registration (file index extensive) |
| `cmd/nerd/cmd_advanced.go` | 488 | dream/shadow/whatif/… |
| `cmd/nerd/cmd_northstar.go` | 474 | northstar tree |
| `cmd/nerd/cmd_auth.go` | 387 | auth tree |
| `cmd/nerd/dom_cmd.go` | 542 | DOM commands |

### 3.3 File index (from `main.go` package comment)

The entry file documents the intentional split:

- Core: `cmd_instruction.go`, `cmd_spawn.go`, `cmd_init_scan.go`
- Direct actions: `cmd_direct_actions.go`
- Advanced: `cmd_advanced.go`
- Browser: `cmd_browser.go`
- Mangle: `cmd_mangle_check.go`, `cmd_mangle_lsp.go`
- Query: `cmd_query.go`
- Campaign: `cmd_campaign.go`
- Auth: `cmd_auth.go`
- Context test: `cmd_test_context.go`
- Helpers: `system_results.go`, `stats.go`

---

## 4. Command surface (Cobra)

### 4.1 Global flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-v, --verbose` | false | Debug logging |
| `--api-key` | env `ZAI_API_KEY` | LLM provider key |
| `-w, --workspace` | CWD | Project root |
| `--timeout` | 25m | Operation timeout |
| `--disable-system-shard` | (run) | Disable named Type-1 shards |

### 4.2 Command groups

See [05-COMMAND-ARCHITECTURE.md](05-COMMAND-ARCHITECTURE.md) for the full table. Summary groups:

- **Lifecycle:** init, scan, run, status, logs, test-context  
- **Kernel:** query, why, logic  
- **Agents:** define-agent, spawn, agents  
- **Direct verbs:** review, fix, test, explain, create, refactor, security, analyze, perception, push, commit  
- **Speculative:** dream, shadow, whatif  
- **Campaign:** start/status/pause/resume/list  
- **Auth:** claude, codex, grok, status  
- **Browser:** launch, session, snapshot  
- **Mangle tools:** check-mangle, mangle-lsp  
- **Systems:** mcp, autopoiesis, memory  
- **Transparency:** glassbox, transparency, reflection  
- **Sessions / knowledge / northstar / dom / embedding**

### 4.3 Interactive meta-commands (direct actions)

When `-i/--interactive` is set on selected direct actions (`cmd_interactive.go`):

`refine`, `redo`, `approve`, `quit`, `help` — multi-turn loop with Cortex held open (30m timeout, SIGINT cancel).

---

## 5. Chat slash surface

Router: `chat/commands.go`. Categories include session control, knowledge/planning, filesystem ops, coding verbs, kernel inspection, safety exploration, systems, campaigns, tools.

Full catalog and file map: [06-TUI-CHAT-SURFACE.md](06-TUI-CHAT-SURFACE.md).

Notable long-horizon:

- `/campaign assault …` and natural language assault intents (README + campaign docs)
- Artifacts under `.nerd/campaigns/<campaign>/assault/`

---

## 6. Boot & Cortex assembly

### 6.1 Interactive boot

`chat/session_boot.go` constructs the running system: logging categories, config, stores (sqlite pragmas), core kernel, system shards, perception clients, prompt/JIT pieces, retrieval, embedding engines, browser manager, autopoiesis, northstar, transparency, world model hooks, etc.

Comments record migration away from direct domain-shard imports toward **JIT prompt atoms**.

### 6.2 Non-interactive boot

Many Cobra commands call `internal/system` helpers such as `GetOrBootCortex` (see `cmd_interactive.go` pattern) with workspace + API key + optional config.

### 6.3 Failure modes

| Failure | User symptom | Where to look |
|---------|--------------|---------------|
| Bad workspace | chdir errors with suggestions | `main.go` RunE |
| Logging init fail | warning; continue | PersistentPreRunE |
| Cortex boot fail | chat error / command error | session_boot, system factory |
| Panic in turn | error message, UI stays up | process.go recover |
| Mangle program error | possible `debug_program_ERROR.mg` dump | kernel load |

---

## 7. UI subsystem

See [07-UI-PAGES-AND-OUTPUT.md](07-UI-PAGES-AND-OUTPUT.md).

Critical components: `splitpane`, `diffview`, campaign/JIT/shard/autopoiesis pages, styles, render cache.

---

## 8. Safety model (CLI edge)

See [09-CONSTITUTIONAL-SAFETY.md](09-CONSTITUTIONAL-SAFETY.md).

Summary: CLI exposes power tools; **authorization** remains kernel `permitted` + Dreamer + policy. CLI contributes timeouts, cancels, shadow/dream/whatif, approve flows, and panic isolation.

---

## 9. Observability

See [12-TELEMETRY-OBSERVABILITY.md](12-TELEMETRY-OBSERVABILITY.md).

Zap + categorized logs + glass box + activity pulse + reflection.

---

## 10. Testing

See [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

```powershell
go test ./cmd/nerd/...
go test -race ./cmd/nerd/chat/...
```

---

## 11. Build

```powershell
if (Test-Path .\nerd.exe) { Remove-Item .\nerd.exe -ErrorAction SilentlyContinue }
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

Portable usage: drop binary → `nerd init` → `nerd` (`cmd/nerd/README.md`).

---

## 12. Integration map

| System | CLI attachment |
|--------|----------------|
| core / mangle | boot, query, why, logic, all agent actions |
| perception / articulation | chat process, run instruction |
| prompt JIT | jit cmd, campaign JIT provider, chat `/jit` |
| shards | spawn, agents, boot system shards |
| campaign | campaign cmd + chat |
| store / embedding | boot, embedding cmd |
| browser | browser cmd |
| autopoiesis | tool cmd, systems autopoiesis |
| transparency | glassbox/transparency/reflection |
| northstar | northstar cmd + wizard |
| world / codedom | scan, dom commands |

Deep journal: [11-CROSS-SYSTEM-WIRING-JOURNAL.md](11-CROSS-SYSTEM-WIRING-JOURNAL.md).

---

## 13. Gaps & non-goals

Gaps: [03-GAP-ANALYSIS-CLI.md](03-GAP-ANALYSIS-CLI.md).

Non-goals of this document:

- Full narrative of every chat handler branch  
- Replacing Docs/Spec 18-file product docs  
- Kernel algorithm deep-dives (see core/mangle corpora)

---

## 14. Related documents

- `cmd/nerd/README.md` — operator quickstart  
- `Docs/architecture/INDEX.md` — corpus index  
- Root `AGENTS.md` — build/test contract  

---

## 15. Deep dive: `nerd run` OODA path

Source: `cmd/nerd/cmd_instruction.go`

### 15.1 Command contract

| Field | Value |
|-------|-------|
| Use | `run [instruction]` |
| Args | `cobra.MinimumNArgs(1)` |
| Long help | Documents Perception → Orient → Decide → Act |

### 15.2 Runtime sequence

```
runInstruction
  ctx = WithTimeout(timeout)
  signal.Notify SIGINT/SIGTERM → cancel
  userInput = joinArgs(args)
  key = --api-key || ZAI_API_KEY
  cortex = system.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
  defer cortex.Close()
  ctx = usage.NewContext(ctx, cortex.UsageTracker)  // if present
  baselines = systemResultBaselines(cortex.Kernel)
  emitter = articulation.NewEmitter()
  // Perception: Transduce Input -> Intent
  // … kernel orient/decide …
  // Act via VirtualStore / session execution
  // Emit articulated response
```

### 15.3 Design notes

- Uses the **same Cortex boot** family as interactive mode (`internal/system`), not a toy kernel.
- Supports system-shard disable list for debugging degraded modes.
- Shutdown is cooperative via context cancel on signals.
- Usage tracking is ambient via context when the tracker exists.

### 15.4 Coupling

| Import | Why |
|--------|-----|
| `internal/system` | GetOrBootCortex |
| `internal/core` | Kernel baselines / facts |
| `internal/articulation` | Emitter for surface response |
| `internal/types` | Shared types |
| `internal/world` | World model participation when scanning/acting |
| `internal/usage` | Token/accounting context |

---

## 16. Deep dive: Campaign CLI

Source: `cmd/nerd/cmd_campaign.go` (~944 lines), `campaign_jit_provider.go`

### 16.1 Parent command

`campaign` long help positions campaigns as multi-phase, multi-session goals:

- greenfield from specs
- large features
- audits
- migrations

### 16.2 Subcommands

| Subcommand | Handler | Purpose |
|------------|---------|---------|
| `start [goal]` | `runCampaignStart` | Decompose + start; `--docs`, `--type` |
| `status` | `runCampaignStatus` | Current campaign |
| `pause` | `runCampaignPause` | Pause |
| `resume` | `runCampaignResume` | Resume |
| `list` | `runCampaignList` | List campaigns |

### 16.3 Types (from flags in `main.go`)

| Flag | Applies to | Values |
|------|------------|--------|
| `--docs` | start | string array of spec paths |
| `--type` | start | `greenfield`, `feature`, `audit`, `migration`, `remediation` (default `feature`) |

### 16.4 JIT adapter

`CampaignJITProvider` (`campaign_jit_provider.go`) implements campaign role prompt retrieval by bridging `campaign.CampaignRole` into the prompt/JIT subsystem — ensuring campaign agents get compiled prompts rather than hard-coded monoliths.

### 16.5 Internal dependencies

`campaign`, `session`, `core`, `coreshards`, `perception`, `prompt`, `store`, `tactile`, `world`, `articulation`, `config`, `types`, plus mangle-go analysis import for logic-adjacent checks.

---

## 17. Deep dive: Chat input processing

Source: `cmd/nerd/chat/process.go` (package index documents split across process_*.go)

### 17.1 File index (from package header)

| File | Concern |
|------|---------|
| `process.go` | `processInput`, seeds |
| `process_dream.go` | Dream state consultation |
| `process_follow_up.go` | Recent turns / agent creation from prompt |
| `process_continuation.go` | Continuation protocol / subtasks |
| `process_knowledge.go` | Knowledge requests / specialist match |
| `process_sync.go` | Workspace sync |

### 17.2 Panic contract

`processInput` returns a named result and installs `defer recover`:

- Logs `logging.API("PANIC in processInput (recovered): …")` + stack
- Converts panic to `errorMsg` so UI returns to idle (`isLoading` cannot stick)

This is a **production reliability requirement**, not a nice-to-have.

### 17.3 Branching model

```
processInput(input)
  if slash command → handleCommand (commands.go)
  else → perception transduction → intent → kernel/session execution path
  dream / knowledge / continuation helpers as specialized branches
```

---

## 18. Deep dive: System boot (chat)

Source: `cmd/nerd/chat/session_boot.go`, `session_shared_boot.go`

### 18.1 Responsibilities

1. Initialize categorized logging (`logging.Initialize(workspace)`).
2. Load / accept `config.UserConfig`.
3. Open stores with sqlite pragmas.
4. Construct kernel + load defaults.
5. Register/start system shards (unless disabled).
6. Wire perception clients, prompt compiler pieces, retrieval, embeddings.
7. Optional: browser, autopoiesis, northstar, transparency, world, tactile, verification.
8. Emit boot timing steps to TTY (`logStep`).

### 18.2 Domain shard migration note

`session_boot.go` comments list removed direct imports:

- coder, nemesis, researcher, reviewer, tester, tool_generator  

Replaced by **JIT clean loop / prompt atoms**. Any regression where `/review` etc. no-ops is a wiring bug until proven otherwise.

### 18.3 Legacy dual path

- `performSystemBootLegacy` — pre-cutover assembly (still present).
- `performSystemBoot` — preferred shared path (`session_shared_boot.go`).

Call sites should converge; dual maintenance is technical debt tracked in OPEN-QUESTIONS.

---

## 19. Error handling patterns

| Layer | Pattern |
|-------|---------|
| Cobra RunE | `return fmt.Errorf("…: %w", err)` |
| Workspace chdir | Typed suggestions for permission / not exist (`main.go`) |
| Logging init | Soft-fail with stderr warning; continue |
| Chat panics | recover → errorMsg |
| Interactive mode | SIGINT cancel context |
| Campaign / long ops | Context timeouts + status commands |

JSON error envelopes (Vectryx-style) are **not** the primary codeNERD CLI contract; human text + TUI messages dominate. Scripted consumers should prefer explicit subcommands with stable exit codes.

---

## 20. Configuration touchpoints

| Source | Consumed by |
|--------|-------------|
| `.nerd/config.yaml` / config loader | timeouts, engines (`main.go` PersistentPreRunE, boot) |
| Global flags | override API key, workspace, verbose, timeout |
| Env `ZAI_API_KEY` | fallback API key |
| Feature flags | `internal/features` during boot |
| Engine auth state | `nerd auth *` commands |

Detailed schema: see `Docs/architecture/config/` and skill `codenerd-config-expert`.

---

## 21. Extension playbook (new CLI feature)

1. Decide door: Cobra only, slash only, or both (document if asymmetric).
2. Implement pure logic in `internal/<pkg>` when possible; keep `cmd/nerd` thin.
3. Wire boot if new subsystem requires init.
4. Add RunE + tests; add slash case in `commands.go` + handler.
5. Emit logs/glass-box events for multi-step work.
6. Update this IMPLEMENTED_SPEC section + command architecture table.
7. Run `go test ./cmd/nerd/...` and targeted package tests.

---

## 22. Historical notes (honest)

| Date | Note |
|------|------|
| 2026-07-13 | First architecture corpus attempt was auto-inventory stubs (~inventory only). Rejected as inadequate vs Vectryx depth. |
| 2026-07-13 | Deep rewrite: dual-surface architecture, command/slash catalogs, boot/wiring journals, OODA/campaign deep dives. |

Still thinner than Vectryx’s largest CLI IMPLEMENTED_SPEC (that corpus includes multi-year command-by-command hardening narratives). Further expansion should attach **shipped change logs** per command family as they land, not invent history.
