package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/mangle"
)

// RunTool is the entry point for the tool's core logic.
// It accepts a primary input string and additional command-line arguments.
func RunTool(input string, args []string) (string, error) {
	var filesToProcess []string

	// If input is provided, treat it as the first file/pattern
	if input != "" {
		filesToProcess = append(filesToProcess, input)
	}
	// Append any additional CLI arguments as file patterns
	filesToProcess = append(filesToProcess, args...)

	if len(filesToProcess) == 0 {
		return "", fmt.Errorf("no file patterns provided. Usage: <file_pattern> [file_pattern ...]")
	}

	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return "", fmt.Errorf("critical Error: Failed to initialize Mangle engine: %w", err)
	}

	var outputBuffer bytes.Buffer
	hasError := false

	for _, pattern := range filesToProcess {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Fprintf(&outputBuffer, "Error: Invalid glob pattern '%s': %v\n", pattern, err)
			hasError = true
			continue
		}

		if len(matches) == 0 {
			// If no glob match, maybe it's a specific file (Glob returns nil if no match but no error)
			if info, err := os.Stat(pattern); err == nil && !info.IsDir() {
				matches = []string{pattern}
			} else {
				fmt.Fprintf(&outputBuffer, "Warning: No files found matching '%s'\n", pattern)
				continue
			}
		}

		for _, file := range matches {
			if err := checkMangleFile(engine, file); err != nil {
				fmt.Fprintf(&outputBuffer, "FAIL: %s\n  -> %v\n", file, err)
				hasError = true
			} else {
				fmt.Fprintf(&outputBuffer, "PASS: %s\n", file)
			}
		}
	}

	if hasError {
		return outputBuffer.String(), fmt.Errorf("one or more files failed Mangle syntax check")
	}
	return outputBuffer.String(), nil
}

// checkMangleFile validates a single Mangle file.
func checkMangleFile(engine *mangle.Engine, path string) error {
	// Create a fresh engine instance to avoid pollution between files
	tmpEngine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return err
	}

	// Try to load schemas.mg first if it exists, to provide context
	// We assume a standard location relative to the project root
	schemasPath := "internal/mangle/schemas.mg"
	if _, err := os.Stat(schemasPath); err == nil && filepath.Base(path) != "schemas.mg" {
		schemaData, err := os.ReadFile(schemasPath)
		if err == nil {
			if err := tmpEngine.LoadSchemaString(string(schemaData)); err != nil {
				// Warn but proceed - if the schema itself is being checked and is broken, it will fail later
				return fmt.Errorf("failed to load context from %s: %w", schemasPath, err)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	// Parse and Load - LoadSchemaString performs syntax validation and basic type checking
	return tmpEngine.LoadSchemaString(string(data))
}
