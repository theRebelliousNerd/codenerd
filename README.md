# codeNERD

Logic-first, neuro-symbolic CLI/TUI agent powered by Google Mangle (Datalog) with an LLM-only perception layer. The assistant reasons through a policy kernel, renders a “Glass Box” view for traceability, and exposes both CLI commands and an interactive chat TUI.

## Quickstart

### Install & Build
1. Install Go 1.22+.
2. From the repo root:
   ```bash
   go build ./...
   go build -o nerd.exe ./cmd/nerd
   ```

### Authenticate
Provide one of:
- Env var: `ZAI_API_KEY` (preferred) or `GEMINI_API_KEY`
- Config file: `~/.codenerd/config.json` (auto-created by the CLI)

### Initialize a workspace
Run once per project:
```bash
nerd init
```
This creates `.nerd/`, scans the codebase, builds a profile, and preloads facts into the kernel.

### Core commands
- `nerd` → interactive chat TUI (Bubble Tea)
- `nerd run "<instruction>"` → single-shot OODA loop
- `nerd query <predicate>` → query derived facts
- `nerd why [predicate]` → explain derivations
- `nerd status` → show system status
- `nerd spawn <shard> <task>` → invoke specialist shards
- `nerd browser launch|session|snapshot` → browser automation (snapshot persistence not yet implemented; see Notes)

In chat mode, type `/help` for commands and keybindings.

### Safety modes
- Logic-first policy in Google Mangle (`internal/mangle/policy.gl`)
- Shadow Mode simulations (`/shadow`, `/whatif`) to project effects before acting
- Interactive diff/approval flow (`/approve`) for guarded changes

## Known incomplete areas
- Browser snapshot persistence is currently stubbed and does not retain sessions across commands; planned: persistent session registry + fact export.
- Deep research for Type 4 specialist agents kicks off but lacks persisted knowledge ingestion status reporting; planned: emit progress facts and surface in `/agents`.

## Testing
Light smoke tests are included for core CLI behaviors. Run:
```bash
go test ./...
```

## Troubleshooting
- Missing API key: set `ZAI_API_KEY` or `GEMINI_API_KEY`, or `/config set-key <key>` in chat.
- Workspace scan warnings: chat mode will surface non-fatal scan failures; rerun with a clean workspace or adjust permissions.

