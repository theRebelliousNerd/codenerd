# cmd/test-research - Research Toolkit Test Harness

This tool provides a quick test harness for the ResearcherShard and research toolkit integrations.

## Usage

```bash
go run ./cmd/test-research
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Research toolkit test harness exercising ResearcherShard with Context7 and GitHub URL scraping. Tests explicit URL research, Context7 API integration, and knowledge atom extraction with configurable API key from env or config. |

## Test Scenarios

1. **Explicit URL Scraping**: Tests GitHub URL research path
2. **Context7 Integration**: Tests LLM-optimized documentation fetching
3. **Knowledge Atom Extraction**: Tests structured knowledge output

## Configuration

API key loaded from:
1. `CONTEXT7_API_KEY` environment variable
2. `.nerd/config.json` under `context7_api_key`

## Dependencies

- `internal/core` - RealKernel for research context
- `internal/perception` - Config loading
- `internal/shards/researcher` - ResearcherShard

## Building

```bash
go run ./cmd/test-research
```
