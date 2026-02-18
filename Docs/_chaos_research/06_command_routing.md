# 06 - Command Routing & Slash Commands

## 1. Routing Architecture

### handleSubmit: The Entry Gate

The command routing pipeline begins at `handleSubmit` (`model_handlers.go:77`). User input is trimmed via `strings.TrimSpace()` at line 78. The critical routing decision occurs at line 112:

```go
if strings.HasPrefix(input, "/") {
    return m.handleCommand(input)
}
```

This is a **single-character prefix check** with no further validation. Any input starting with `/` is routed to the command handler. There is no length check, no character validation, and no sanitization before dispatch. Empty input is caught earlier (line 79) but `"/"` alone passes the `HasPrefix` check and enters `handleCommand`.

Notable: Before the `/` check, two special modes are tested:
- **Patch ingestion mode** (`awaitingPatch`, line 89): If active, ALL input is captured until `--END--` sentinel.
- **Pending subtasks** (line 81): Empty Enter acts as continuation confirmation.

### handleCommand: The Dispatcher

`handleCommand` (`commands.go:53-2173`) receives the raw input string and immediately parses it:

```go
parts := strings.Fields(input)
cmd := parts[0]
```

`strings.Fields` splits on whitespace, which means:
- `"/test  foo   bar"` → `["/test", "foo", "bar"]` (multiple spaces collapsed)
- `"/test\tfoo"` → `["/test", "foo"]` (tabs treated as separators)
- `"/ "` → `["/"]` (single slash, space stripped)

The command token `parts[0]` is matched in a **2173-line monolithic switch statement** with ~50+ case branches. The `default` case at line 2162 emits an "Unknown command" message using `fmt.Sprintf` with the raw `cmd` value injected into the response.

### Switch Statement Structure

Commands are organized by category (documented at lines 12-25):
- **Session**: `/quit`, `/exit`, `/continue`, `/usage`, `/clear`, `/reset`, `/new-session`, `/sessions`
- **Help**: `/help`, `/status`
- **Init**: `/init`, `/scan`, `/refresh-docs`, `/scan-path`, `/scan-dir`
- **Config**: `/config`, `/embedding`
- **Files**: `/read`, `/mkdir`, `/write`, `/search`, `/patch`, `/edit`, `/append`, `/pick`
- **Agents**: `/define-agent`, `/northstar`, `/learn`, `/agents`, `/spawn`, `/ingest`
- **Analysis**: `/review`, `/security`, `/analyze`, `/test`, `/fix`, `/refactor`
- **Campaigns**: `/legislate`, `/clarify`, `/launchcampaign`, `/campaign`
- **Query**: `/query`, `/why`, `/logic`, `/glassbox`, `/transparency`, `/shadow`, `/whatif`
- **Review**: `/approve`, `/reject-finding`, `/accept-finding`, `/review-accuracy`
- **Tools**: `/tool`, `/jit`, `/cleanup-tools`
- **Evolution**: `/evolve`, `/evolution-stats`, `/evolved-atoms`, `/promote-atom`, `/reject-atom`, `/strategies`

### CommandRegistry (Decoupled Metadata)

`command_categories.go` defines a `CommandRegistry` (line 36) — a `[]CommandInfo` slice that holds metadata (Name, Aliases, Description, Usage, Category, ShowInHelp) for help rendering. This registry is **decoupled** from the actual switch dispatch — the switch in `commands.go` is the canonical handler, while `CommandRegistry` is only used for `/help` rendering and `FindCommand` lookups. Commands can exist in the switch without registry entries and vice versa — a wiring gap risk.

`FindCommand` (`command_categories.go:550`) does exact string matching against `Name` and `Aliases` — no normalization, no case folding.

## 2. Argument Parsing & Validation

### Argument Extraction Patterns

Commands extract arguments from the `parts` slice (produced by `strings.Fields`) using three main patterns:

**Pattern A: `len(parts) < N` guard** — Most commands check minimum argument count before accessing `parts[1]`, `parts[2]`, etc. If insufficient, they emit a usage hint. Examples:
- `/load-session` (`commands.go:176`): `if len(parts) < 2` → "Usage: `/load-session <session-id>`"
- `/promote-atom` (`commands.go:2095`): `if len(parts) < 2` → "Usage: `/promote-atom <atom-id>`"
- `/reject-atom` (`commands.go:2129`): `if len(parts) < 2` → "Usage: `/reject-atom <atom-id>`"

**Pattern B: Optional args with `strings.Join`** — Some commands join remaining parts into a freeform string:
- `/help` (`commands.go:193-195`): `arg = strings.Join(parts[1:], " ")` — accepts arbitrary text after `/help`

**Pattern C: Direct index access** — Some commands access `parts[1]` directly after the length check:
- `/load-session` (`commands.go:187`): `sessionID := parts[1]` — raw, unsanitized user string used as session identifier
- `/promote-atom` (`commands.go:2108`): `atomID := parts[1]` — raw atom ID passed directly to `PromoteAtom()`
- `/reject-atom` (`commands.go:2142`): `atomID := parts[1]` — raw atom ID passed directly to `RejectAtom()`

### Commands Accepting Arbitrary User Strings

Several commands pass user-provided strings directly to subsystems without sanitization:
- `/load-session <id>` — session ID used for file path construction
- `/query <predicate>` — passed to Mangle engine parser
- `/why <fact>` — passed to Mangle explanation engine
- `/write <path> <content>` — path and content used for filesystem writes
- `/read <path>` — path used for filesystem reads
- `/search <pattern>` — pattern used for codebase search
- `/mkdir <path>` — path used for directory creation
- `/spawn <type> <task>` — type and task description sent to shard system
- `/fix <description>` — description sent to LLM as part of prompt
- `/campaign start <goal>` — goal text injected into campaign orchestrator
- `/legislate <constraint>` — constraint text sent to Legislator shard
- `/reject-finding <file>:<line> <reason>` — file path and reason stored as facts

### Flag Parsing

Flag handling is ad-hoc and command-specific, not centralized:
- `/scan` (`command_categories.go:58`): Accepts `--deep` or `-d`
- `/init` (`command_categories.go:471`): Accepts `--force`
- `/refresh-docs` (`command_categories.go:486`): Accepts `--force` or `-f`
- `/review` (`command_categories.go:65`): Accepts `--andEnhance` and `-- <passthrough flags>`

There is no standard flag parser (no `flag` package, no `pflag`). Flags are checked via string comparison on `parts` elements, meaning `--force` works but `--force=true` or `-force` may not be handled.

### Missing/Extra Arguments

- **Missing**: Generally handled with `len(parts) < N` guard → usage message. But the guard is not universal — some commands may panic on index-out-of-bounds if the guard is missing.
- **Extra**: Universally ignored. No command validates maximum argument count. `/quit extra garbage here` works identically to `/quit`.

## 3. Edge Cases & Existing Tests

### Special Input Handling

| Input | Behavior | Location |
|-------|----------|----------|
| `"/"` alone | `strings.Fields("/")` → `["/"]`, switch gets `"/"`, falls to `default` → "Unknown command: /" | `commands.go:2162` |
| `"/ "` (slash+space) | `strings.TrimSpace` reduces to `"/"`, then same as above | `model_handlers.go:78` |
| `"//"` | `strings.Fields("//")` → `["//"]`, switch gets `"//"`, falls to `default` → "Unknown command: //" | `commands.go:2162` |
| `"/command\x00arg"` | `strings.Fields` does NOT split on null bytes → `parts[0]` = `"/command\x00arg"`, falls to `default` case. Null byte preserved in error message. | `commands.go:54,2165` |
| `"/QUIT"` | Case-sensitive switch: falls to `default`, not matched to `/quit` | `commands.go:57` |

### Existing Test Coverage (`commands_test.go`)

Tests cover:
- **Quit/Exit/Q**: All three aliases tested for `tea.Quit` return (`TestCommand_Quit`, line 17)
- **Continue (no pending)**: Verifies "No pending tasks" message (`TestCommand_Continue_NoPending`, line 37)
- **Continue (with pending)**: Verifies `isLoading=true`, batch command returned, subtasks consumed (`TestCommand_Continue_WithPending`, line 55)
- **Usage**: Verifies `viewMode` switches to `UsageView` (`TestCommand_Usage`, line 82)
- **Clear**: Verifies history emptied (`TestCommand_Clear`, line 94)
- **New-session**: Verifies new session ID generated, message about new session (`TestCommand_NewSession`, line 115)
- **Help**: Verifies assistant message contains "Commands"/"command" (`TestCommand_Help`, line 142)
- **Status**: Skipped — requires kernel (`TestCommand_Status`, line 165)
- **Config**: Verifies wizard or config display (`TestCommand_Config`, line 182)

### What's NOT Tested

- No test for `"/"` alone, `"//"`, or `"/ "` edge cases
- No test for null bytes or control characters in command names
- No test for unicode command names (e.g., `/テスト`)
- No test for extremely long command strings
- No test for the `default` case (unknown commands)
- No test for command argument overflow (extra args ignored?)
- No test for concurrent command execution
- No test for commands with path traversal arguments (`../../`)
- No test for `/query` or `/why` with malicious Mangle syntax
- No test for `/write` or `/read` with path injection attempts
- No tests for any file-operation commands (`/read`, `/write`, `/mkdir`, `/search`, `/edit`, `/append`)
- No tests for any shard-spawning commands (`/review`, `/test`, `/fix`, `/spawn`)
- No tests for any campaign commands
- No tests for flag parsing correctness

## 4. CHAOS FAILURE PREDICTIONS

### P1: Null Byte Injection in Command Name — **HIGH**

**Input**: `"/\x00\x00\x00"`  
**Path**: `model_handlers.go:112` → `commands.go:54` → `commands.go:2165`  
**Prediction**: `strings.Fields` preserves null bytes. The command `"/\x00\x00\x00"` reaches the `default` case and is injected into `fmt.Sprintf("Unknown command: %s", cmd)` at `commands.go:2165`. The null bytes appear in the Bubbletea viewport via `addMessage` → `renderHistory`. Depending on the terminal emulator, null bytes could corrupt rendering, cause truncated display, or interact unexpectedly with TUI state. The `FindCommand` lookup (`command_categories.go:550`) will also fail silently since no registry entry matches.  
**Blast radius**: UI corruption, potential terminal state issues.

### P2: Memory Exhaustion via Oversized Command — **CRITICAL**

**Input**: `"/"` followed by 10MB of text (no spaces)  
**Path**: `model_handlers.go:78` → `strings.TrimSpace` on 10MB string → `model_handlers.go:112` HasPrefix check passes → `commands.go:54` `strings.Fields` allocates `parts` slice → `commands.go:55` `parts[0]` = entire 10MB string → `commands.go:2165` `fmt.Sprintf` creates another 10MB+ string → `addMessage` stores in `history` slice → `renderHistory` attempts to render 10MB into viewport.  
**Prediction**: No input length limit exists anywhere in the pipeline. The 10MB string is copied multiple times (TrimSpace, Fields, Sprintf, history storage, rendering). Total memory impact: ~50MB+ per submission. Repeated submissions could OOM the process. The `textarea` Bubbletea component itself may impose a soft limit, but `handleSubmit` reads `m.textarea.Value()` which has no documented max.  
**Blast radius**: Process OOM, terminal hang, potential panic from Bubbletea rendering.

### P3: Unicode Normalization Bypass — **MEDIUM**

**Input**: `"/ｑｕｉｔ"` (fullwidth Unicode) or `"/qu\u0300it"` (combining diacritics)  
**Path**: `commands.go:54-57`  
**Prediction**: The switch statement does exact byte comparison. Fullwidth `/ｑｕｉｔ` will NOT match `"/quit"`. The command falls to `default` and is reported as unknown. However, if any downstream system normalizes Unicode (e.g., filesystem paths via `/read /ｅｔｃ/ｐａｓｓｗｄ`), the mismatch between command routing and subsystem normalization creates a bypass vector. The `FindCommand` registry also does exact matching with no normalization (`command_categories.go:553`).  
**Blast radius**: Command spoofing in logs, potential filesystem normalization exploits via file-operation commands.

### P4: Rapid-Fire Command Flooding — **HIGH**

**Input**: Rapid sequential submission of `/test`, `/review`, `/fix` within milliseconds  
**Path**: `model_handlers.go:77` → each spawns shard via `tea.Batch` → concurrent goroutines  
**Prediction**: Each analysis command (`/review`, `/test`, `/fix`) spawns shard goroutines and sets `m.isLoading = true`. However, there is no guard preventing a second command while `isLoading` is true — `handleSubmit` checks `isLoading` nowhere. If the Bubbletea model receives rapid input before processing previous commands, multiple concurrent shards could be spawned simultaneously, competing for the same kernel, filesystem, and LLM API resources. The `pendingSubtasks` slice could be overwritten or corrupted if two command handlers try to set it concurrently.  
**Blast radius**: Resource exhaustion, kernel fact store corruption from concurrent writes, race conditions in shard execution.

### P5: Shell Injection via `/write` Path Arguments — **CRITICAL**

**Input**: `/write "$(rm -rf /)" malicious content`  
**Path**: `commands.go` `/write` handler → path passed to filesystem operations  
**Prediction**: The `/write` command takes `parts[1]` as a file path and remaining parts as content. If the path is passed to any shell-based execution (e.g., via `os/exec.Command` through a VirtualStore predicate), shell metacharacters could be interpreted. Even without shell injection, path traversal via `/write ../../etc/cron.d/evil content` could write outside the workspace. The constitutional gate (`permitted(Action)`) should block dangerous paths, but the command handler may bypass the kernel entirely for simple file operations.  
**Blast radius**: Arbitrary file write, potential RCE if path reaches shell execution.

### P6: Mangle Injection via `/query` Arguments — **CRITICAL**

**Input**: `/query user_intent(X,Y,Z,W,V) :- shell_exec("rm -rf /", _).`  
**Path**: `commands.go` `/query` handler → Mangle parser → kernel evaluation  
**Prediction**: The `/query` command passes the user-provided predicate string directly to the Mangle parser. If the parser accepts rule definitions (not just queries), a user could inject arbitrary Mangle rules that bind to virtual predicates with side effects (e.g., `file_write`, `shell_exec`). Even if the parser rejects rules, malformed Mangle syntax could trigger parser panics. The Mangle `parse.Parse` function's error handling is the only defense.  
**Blast radius**: Arbitrary logic injection, potential RCE via virtual predicates, kernel state corruption.

### P7: Path Traversal via `/read` — **HIGH**

**Input**: `/read ../../../../etc/passwd` or `/read C:\Windows\System32\config\SAM`  
**Path**: `commands.go` `/read` handler → file read operation  
**Prediction**: The `/read` command takes `parts[1]` as a path. If the handler does `filepath.Join(workspace, parts[1])` without canonicalization and containment check, `../` sequences escape the workspace. Even with `filepath.Clean`, symbolic links could bypass containment. On Windows, UNC paths (`\\server\share`) and device paths (`\\.\PhysicalDrive0`) add additional attack surface.  
**Blast radius**: Arbitrary file read, credential/secret exposure, information disclosure.

### P8: Default Case Response Injection — **MEDIUM**

**Input**: `/notacommand <script>alert('xss')</script>` or `/notacommand \x1b[2J\x1b[H` (ANSI escape)  
**Path**: `commands.go:2162-2165`  
**Prediction**: The default case injects the raw command into the response string: `fmt.Sprintf("Unknown command: %s. Type /help...", cmd)`. The command name is not escaped. If the rendered output is displayed in a context that interprets ANSI escapes (terminal) or HTML (web view), injection is possible. ANSI escapes like `\x1b[2J` (clear screen) or `\x1b[31m` (color) could corrupt the TUI display. Since Bubbletea renders via Lipgloss, some ANSI sequences may be interpreted.  
**Blast radius**: TUI display corruption, misleading output to user, potential screen clearing to hide malicious activity.

### P9: Command-Registry Desync — **MEDIUM**

**Input**: Any command that exists in the switch but not in `CommandRegistry` (or vice versa)  
**Path**: `commands.go` switch vs `command_categories.go:36` CommandRegistry  
**Prediction**: The switch statement and `CommandRegistry` are maintained independently. A command added to the switch but not the registry will work but be invisible in `/help`. A command in the registry but not the switch will appear in `/help` but hit `default` case when executed. This desync is a maintenance burden that grows with each new command. The `FindCommand` function (`command_categories.go:550`) is used for help lookups but never for actual dispatch, creating two independent truth sources.  
**Blast radius**: User confusion, hidden functionality, "ghost commands" in help that don't work.

### P10: Session ID Injection via `/load-session` — **HIGH**

**Input**: `/load-session ../../.env` or `/load-session sess_$(whoami)`  
**Path**: `commands.go:187` → `sessionID := parts[1]` → `m.loadSelectedSession(sessionID)`  
**Prediction**: The session ID from user input is passed directly to `loadSelectedSession` without validation. If this function constructs a file path from the session ID (e.g., `.nerd/sessions/<sessionID>.json`), path traversal could read arbitrary JSON files. If the session ID is used in `fmt.Sprintf` for logging or kernel assertions, format string injection is possible. The session ID `"sess_%d"` format (from `/new-session` at `commands.go:129`) suggests the loader expects numeric suffixes, but no validation enforces this.  
**Blast radius**: Arbitrary file read via path traversal, potential deserialization of attacker-controlled JSON.

### P11: Concurrent Model Mutation Race Condition — **HIGH**

**Input**: Simultaneous `/clear` and `/new-session` via programmatic input  
**Path**: `commands.go:97` (clear history) and `commands.go:128` (reset history + new session)  
**Prediction**: Bubbletea's MVU architecture processes messages sequentially, which should prevent true races. However, `tea.Batch` commands spawned by shard-executing commands (`/test`, `/review`) return async `tea.Msg` values. If a user submits `/clear` while a shard is completing and the shard's completion message arrives between `handleCommand`'s state mutations and the return, the model state could be inconsistent — cleared history with a shard result message referencing now-deleted context.  
**Blast radius**: Panic from nil/empty slice access, orphaned shard results, state corruption.

### P12: Atom ID Injection via `/promote-atom` and `/reject-atom` — **MEDIUM**

**Input**: `/promote-atom ../../../etc/passwd` or `/promote-atom ; DROP TABLE atoms;--`  
**Path**: `commands.go:2108` → `m.promptEvolver.PromoteAtom(atomID)`  
**Prediction**: The raw `parts[1]` string is passed directly to `PromoteAtom()`. If the prompt evolver uses the atom ID to construct file paths (reading/writing YAML atoms from disk) or SQL queries (updating atom status in SQLite), injection is possible. The atom ID format is never validated against a pattern like `^[a-z0-9_-]+$`. Depending on `PromoteAtom`'s implementation, this could lead to path traversal or SQL injection.  
**Blast radius**: File system manipulation, database corruption, promotion of unintended content.

### Summary Table

| # | Attack Vector | Severity | Entry Point |
|---|---------------|----------|-------------|
| P1 | Null byte injection in command name | HIGH | `commands.go:54,2165` |
| P2 | 10MB command string memory exhaustion | CRITICAL | `model_handlers.go:78,112` |
| P3 | Unicode normalization bypass | MEDIUM | `commands.go:54-57` |
| P4 | Rapid-fire command flooding (no isLoading guard) | HIGH | `model_handlers.go:77,112` |
| P5 | Shell injection via `/write` path | CRITICAL | `commands.go` /write handler |
| P6 | Mangle injection via `/query` | CRITICAL | `commands.go` /query handler |
| P7 | Path traversal via `/read` | HIGH | `commands.go` /read handler |
| P8 | ANSI escape injection in default case | MEDIUM | `commands.go:2162-2165` |
| P9 | Command-registry desync (ghost commands) | MEDIUM | `commands.go` vs `command_categories.go:36` |
| P10 | Session ID path traversal via `/load-session` | HIGH | `commands.go:187` |
| P11 | Concurrent model mutation race | HIGH | `commands.go:97,128` + shard async |
| P12 | Atom ID injection via `/promote-atom` | MEDIUM | `commands.go:2108` |
