---
trigger: always_on
description: Run it before claiming it's done
---

# Verify Before Done

> **If you didn't run it, you didn't ship it.**

## Mandatory Verification

### Go Code

```bash
go build ./...           # Compiles?
go test ./...            # Tests pass?
go test -race ./...      # Race conditions?
```

### Python Code

```bash
pytest tests/            # Tests pass?
ruff check .             # Lint clean?
```

## Done Checklist

1. ✅ Build succeeded
2. ✅ Tests passed  
3. ✅ Ran the specific changed feature
4. ✅ Checked for regressions

> If your "completed" work fails on first user verification, you failed.
