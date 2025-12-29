package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

const countLinesDescription = "Counts the number of lines in a given file."

// countLines takes a file path and returns the number of lines in the file.
func countLines(ctx context.Context, path string) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading file %s: %w", path, err)
	}

	return count, nil
}

// RegisterCountLines registers the countLines tool with the provided ToolRegistry.
func RegisterCountLines(registry ToolRegistry) error {
	return registry.Register("countLines", countLinesDescription, countLines)
}
