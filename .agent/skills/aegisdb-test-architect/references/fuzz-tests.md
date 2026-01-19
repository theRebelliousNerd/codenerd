# Fuzz Tests

## Purpose
Discover crashes and invariant violations with randomized inputs.

## Targets
- Parsers and decoders
- Mangle rules and query ingestion
- Config loaders
- Protocol payload parsing

## Structure
- Use FuzzXxx with *testing.F.
- Seed with realistic corpus inputs via f.Add.
- Validate invariants; never allow panics.

## Invariants
- No panic or crash
- Output length bounds
- Round trip encode/decode
- Idempotent normalization

## Corpus
- Keep seeds small and representative.
- Store generated crashers under testdata/fuzz when useful.

## Performance
- Keep a single call cheap.
- Avoid external services or long-running operations.

## Pitfalls
- Do not ignore errors.
- Do not mutate global state.
