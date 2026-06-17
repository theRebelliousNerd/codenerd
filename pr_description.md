🎯 What: Refactored `parseManglePredicateAndArity` in `internal/world/code_elements_mangle.go` into three smaller, focused helper functions: `extractManglePredicateName`, `extractMangleArgsString`, and `calculateMangleArity`.

💡 Why: The original function was over 130 lines long, making it hard to follow. By separating the logic to extract the predicate name, match parenthesis for the argument string, and parse that string for arity into helpers, the code becomes more maintainable, testable, and easier to understand.

✅ Verification: Verified syntax through `gofmt -w` and ran the full suite via `go test ./internal/world/...`, specifically asserting that `TestCodeElementParser_ParseMangleFile` passed with the refactored logic intact.

✨ Result: The core parsing function is significantly shorter and more declarative, effectively decoupling concerns without altering its previous behavior.
