# codeNERD CLI Command Reference

This reference lists the available commands for the `nerd` binary.

## Core Commands

- **`nerd chat`**: Start the interactive chat session (Main entry point).
- **`nerd init`**: Initialize a new codeNERD project/profile.
- **`nerd scan`**: Scan the current directory to populate the World Model (`file_topology`, `symbol_graph`).
- **`nerd query <predicate>`**: Query the Mangle knowledge base directly.
- **`nerd auth`**: Manage authentication.

## Development & Debugging

- **`nerd debug`**: Debugging tools.
- **`nerd mangle check`**: Validate Mangle files and schema.
- **`nerd mangle lsp`**: Start the Mangle Language Server.
- **`nerd transparency`**: View internal state/reasoning.

## Advanced Features

- **`nerd campaign`**: Manage long-running campaigns.
- **`nerd session`**: Manage chat sessions.
- **`nerd spawn <shard>`**: Manually spawn a shard (e.g., `nerd spawn coder`).
- **`nerd systems`**: Manage system shards and autopoiesis.
- **`nerd knowledge`**: Interact with the knowledge graph/vector store.
- **`nerd browser`**: Browser automation tools (Rod integration).
- **`nerd northstar`**: Strategic planning and goal tracking.

## Usage

```powershell
# Build
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd

# Run
./nerd.exe <command> [flags]
```
