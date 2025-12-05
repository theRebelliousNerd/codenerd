package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/mangle"

	"github.com/spf13/cobra"
)

// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
// This File is best written for Gemini 2.5 Pro.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for cmd/nerd/check_mangle.go
//
// File: cmd/nerd/check_mangle.go
// Author: Gemini
// Date: 2025-12-05
//
// Recommended Model: 2.5 Flash
//
// Overview:
// This file implements the `check-mangle` command for the codeNERD CLI.
// Its primary responsibility is to validate the syntax of Google Mangle (.gl) files.
//
// Key Features & Business Value:
// - Syntax Validation: Parse .gl files and report syntax errors using the official parser.
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
// `nerd check-mangle internal/mangle/*.gl`
//
// References:
// - internal/mangle/grammar.go
//
// --- END OF PRD HEADER ---

var checkMangleCmd = &cobra.Command{
	Use:   "check-mangle [file...]",
	Short: "Check Mangle syntax in .gl files",
	Long:  `Validates the syntax of Google Mangle (Datalog) logic files.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCheckMangle,
}

func runCheckMangle(cmd *cobra.Command, args []string) error {
	hasError := false
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to initialize mangle engine: %w", err)
	}

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
			if err := checkFile(engine, file); err != nil {
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

func checkFile(engine *mangle.Engine, path string) error {
	// Create a new engine for isolation
	tmpEngine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return err
	}

	// Try to load schemas.gl first if it exists, to provide context
	// We assume a standard location or relative path; for now hardcode likely location
	// In a real tool, this would be a flag --schema or --include
	excludePath := "internal/mangle/schemas.gl"
	if _, err := os.Stat(excludePath); err == nil && filepath.Base(path) != "schemas.gl" {
		schemaData, err := os.ReadFile(excludePath)
		if err == nil {
			if err := tmpEngine.LoadSchemaString(string(schemaData)); err != nil {
				// If the schema itself is broken, we should probably warn but proceed
				fmt.Printf("WARNING: Failed to load context from %s: %v\n", excludePath, err)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return tmpEngine.LoadSchemaString(string(data))
}
