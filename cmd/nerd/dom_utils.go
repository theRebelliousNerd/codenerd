package main

import (
	"context"
	"fmt"
	"os/exec"
)

func runGoFmtFiles(ctx context.Context, ws string, files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Chunk to avoid command line length limits on Windows.
	const chunkSize = 40
	for start := 0; start < len(files); start += chunkSize {
		end := start + chunkSize
		if end > len(files) {
			end = len(files)
		}
		args := append([]string{"-w"}, files[start:end]...)
		gofmtCmd := exec.CommandContext(ctx, "gofmt", args...)
		gofmtCmd.Dir = ws
		if out, err := gofmtCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofmt failed: %w\n%s", err, string(out))
		}
	}
	return nil
}

