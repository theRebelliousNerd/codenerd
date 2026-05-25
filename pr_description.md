💡 **What:** Refactored `error_classifier.go` to declare multiple `regexp.MustCompile` statements as global variables instead of inside individual function calls (`extractLineCol`, `ExtractPredicateFromError`).
🎯 **Why:** `regexp.MustCompile` is an expensive operation in Go. Running it repeatedly on every function invocation creates a clear performance anti-pattern. Pulling these to the global package scope ensures they compile exactly once at application startup.
📊 **Measured Improvement:** We wrote benchmark tests that confirm this modification brings substantial latency reduction:
- `ExtractPredicateFromError`: baseline ~8002 ns/op -> optimized ~3351 ns/op (~58% faster)
- `extractLineCol`: baseline ~8756 ns/op -> optimized ~1190 ns/op (~86% faster)
