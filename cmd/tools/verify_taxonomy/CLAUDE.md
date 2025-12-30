# cmd/tools/verify_taxonomy - Mangle Taxonomy Verifier

This tool verifies that the Mangle taxonomy correctly classifies user intents by running test scenarios through the perception transducer.

## Usage

```bash
go run ./cmd/tools/verify_taxonomy
```

## File Index

| File | Description |
|------|-------------|
| `main.go` | Mangle taxonomy verification protocol running test scenarios through `perception.DebugTaxonomy()`. Tests keyword matching, hybrid intent resolution, and Mangle inference boosting for correct verb/category/shard routing. |

## Test Scenarios

| Input | Expected | Reason |
|-------|----------|--------|
| "review this code" | `/review` | Direct keyword match |
| "check for vulnerabilities" | `/security` | Keyword maps to security |
| "fix the security bug" | `/security` | Mangle boosts security context |
| "verify test coverage" | `/test` | Coverage boosts test |
| "debug this panic" | `/debug` | Stacktrace boosts debug |

## Output Format

```
>>> Input: "review this code"
    Goal:  /review (Direct keyword match)
    Result: /review [mutation] (Conf: 0.95) -> Shard: reviewer
    Status: ✅ PASS
```

## Integration

Tests `perception.DebugTaxonomy(input)` returning:
- `verb` - Classified intent verb
- `category` - Intent category
- `confidence` - Classification confidence
- `shard` - Target shard for routing

## Dependencies

- `internal/perception` - DebugTaxonomy function

## Building

```bash
go run ./cmd/tools/verify_taxonomy
```

---

**Remember: Push to GitHub regularly!**
