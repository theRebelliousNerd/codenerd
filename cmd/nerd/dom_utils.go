package main

import (
	"context"
	"fmt"
	"os/exec"

	"codenerd/internal/processutil"
)

func runGoFmtFiles(ctx context.Context, ws string, files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Chunk to avoid command line length limits on Windows.
	const chunkSize = 40
	for start := 0; start < len(files); start += chunkSize {
		end := min(start+chunkSize, len(files))
		// Two independent guards against a filename being read as a flag:
		// "--" ends gofmt's flag parsing, and the "./" prefix means a leading
		// dash never reaches the command line even if a tool ignores "--".
		safeFiles := make([]string, 0, end-start)
		for _, f := range files[start:end] {
			if len(f) > 0 && f[0] == '-' {
				f = "./" + f
			}
			safeFiles = append(safeFiles, f)
		}
		args := append([]string{"-w", "--"}, safeFiles...)
		gofmtCmd := processutil.NonInteractive(exec.CommandContext(ctx, "gofmt", args...))
		gofmtCmd.Dir = ws
		if out, err := gofmtCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofmt failed: %w\n%s", err, string(out))
		}
	}
	return nil
}
