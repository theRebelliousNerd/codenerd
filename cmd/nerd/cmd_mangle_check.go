// Package main implements the codeNERD CLI commands.
// This file contains the check-mangle command for validating Mangle (.mg) files.
//
// Symbiogen Product Requirements Document (PRD)
//
// File: cmd/nerd/cmd_mangle_check.go
// Author: Gemini
// Date: 2025-12-05
//
// Recommended Model: Gemini 3 Flash
//
// Overview:
// This file implements the `check-mangle` command for the codeNERD CLI.
// Its primary responsibility is to validate the syntax of Google Mangle (.mg) files.
//
// Key Features & Business Value:
// - Syntax Validation: Parse .mg files and report syntax errors using the official parser.
// - Glob Support: Process multiple files via shell globs or recursive directory scanning.
// - CI/CD Integration: Return non-zero exit codes on failure for pipeline compatibility.
//
// Architectural Context:
// - Component Type: CLI Command
// - Deployment: Built into `nerd` binary.
// - Dependencies: Relies on `github.com/google/mangle/parse` (via `internal/mangle` wrapper if avail).
//
// Dependencies & Dependents:
// - Dependencies: `github.com/spf13/cobra`, `internal/mangle`.
// - Is a Dependency for: None (Leaf command).
//
// Deployment & Operations:
// - CI/CD: Standard go build.
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling and clear logging.
//
// Functions / Classes:
// - `runCheckMangle()`:
//   - **Purpose:** Execute the syntax check logic.
//   - **Logic:** Iterate args, read files, parse, print verification status.
//
// Usage:
// `nerd check-mangle internal/mangle/*.mg`
//
// References:
// - internal/mangle/grammar.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/mangle"

	"github.com/spf13/cobra"
)

// =============================================================================
// CHECK-MANGLE COMMAND - Mangle syntax validation
// =============================================================================

var checkMangleCmd = &cobra.Command{
	Use:   "check-mangle [file...]",
	Short: "Check Mangle syntax in .mg files",
	Long:  `Validates the syntax of Google Mangle (Datalog) logic files.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCheckMangle,
}

func runCheckMangle(cmd *cobra.Command, args []string) error {
	hasError := false

	for _, pattern := range args {
		// Handle glob expansion (if shell didn't already)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Error processing pattern %s: %v\n", pattern, err)
			hasError = true
			continue
		}

		if len(matches) == 0 {
			// If no glob match, maybe it's a specific file (Glob returns nil if no match but no error)
			if _, err := os.Stat(pattern); err == nil {
				matches = []string{pattern}
			} else {
				fmt.Printf("No files found matching: %s\n", pattern)
				continue
			}
		}

		for _, file := range matches {
			if err := checkFile(file); err != nil {
				fmt.Printf("ERROR in %s: %v\n", file, err)
				hasError = true
			} else {
				fmt.Printf("OK: %s\n", file)
			}
		}
	}

	if hasError {
		os.Exit(1)
	}
	return nil
}

func checkFile(path string) error {
	// Create a new engine for isolation
	tmpEngine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return err
	}

	// Load all schema files (modular design: schemas.mg, schemas_intent.mg, etc.)
	// The canonical location is internal/core/defaults/
	searchPaths := []string{
		"internal/core/defaults",
		".",
		"../internal/core/defaults",
		"../../internal/core/defaults",
	}

	schemasLoaded := 0
	for _, basePath := range searchPaths {
		// Check if this directory exists
		if _, err := os.Stat(basePath); err != nil {
			continue
		}

		// Find all schema*.mg files in this directory
		schemaPattern := filepath.Join(basePath, "schemas*.mg")
		schemaFiles, err := filepath.Glob(schemaPattern)
		if err != nil {
			continue
		}

		for _, schemaFile := range schemaFiles {
			// Skip if we're checking this exact file
			if filepath.Base(path) == filepath.Base(schemaFile) {
				continue
			}
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				continue
			}
			if err := tmpEngine.LoadSchemaString(string(data)); err != nil {
				// Warn but continue with other schema files
				fmt.Printf("WARNING: Failed to load %s: %v\n", filepath.Base(schemaFile), err)
			} else {
				schemasLoaded++
			}
		}

		// Load non-modular schema helpers that live under defaults/schema.
		extraSchemas := []string{
			filepath.Join(basePath, "schema", "learning.mg"),
		}
		for _, schemaFile := range extraSchemas {
			if _, err := os.Stat(schemaFile); err != nil {
				continue
			}
			data, err := os.ReadFile(schemaFile)
			if err != nil {
				continue
			}
			if err := tmpEngine.LoadSchemaString(string(data)); err != nil {
				fmt.Printf("WARNING: Failed to load %s: %v\n", filepath.Base(schemaFile), err)
			} else {
				schemasLoaded++
			}
		}
		if schemasLoaded > 0 {
			break // Found schemas in this directory, stop searching
		}
	}

	// Also load MCP schemas if available
	mcpSchemaPath := "internal/mcp/schemas_mcp.mg"
	if _, err := os.Stat(mcpSchemaPath); err == nil {
		if data, err := os.ReadFile(mcpSchemaPath); err == nil {
			if err := tmpEngine.LoadSchemaString(string(data)); err == nil {
				schemasLoaded++
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return tmpEngine.LoadSchemaString(string(data))
}
