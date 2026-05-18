1. Modify `internal/autopoiesis/ouroboros.go` at the beginning of `func (rt *RuntimeTool) Execute` to add validation:
```go
	cleanPath := filepath.Clean(rt.BinaryPath)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("tool binary path must be absolute for security: %s", cleanPath)
	}

	// Verify binary still exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return "", fmt.Errorf("tool binary not found: %s", cleanPath)
	}
```
2. Replace `cmd := exec.CommandContext(ctx, rt.BinaryPath)` with `cmd := exec.CommandContext(ctx, cleanPath)`.
3. Add `path/filepath` to imports if it's not already there (it should be since `filepath.Join` is used in the same file).
4. Since `Execute` is part of `RuntimeTool` and only expects to execute valid generated binaries, ensuring the path is absolute and clean protects against path traversal that might inadvertently just resolve to a system binary like `bash` or `../../../../bin/bash` relative to some working directory. Since Ouroboros paths are usually built via `filepath.Join(WorkspaceRoot, ...)`, they will naturally be absolute paths, so this strict validation enforces the existing design securely.
5. Create a test in `internal/autopoiesis/ouroboros_test.go` to test that `Execute` rejects relative paths (e.g., `bash`).
6. Run `go test ./internal/autopoiesis/...` to verify nothing is broken.
7. Run `go vet` and format checks.
8. Request plan review.
