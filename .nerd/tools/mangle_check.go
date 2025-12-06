package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/mangle"
)

// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
// This File is best written for Gemini 2.5 Pro.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for .nerd/tools/mangle_check.go
//
// File: .nerd/tools/mangle_check.go
// Author: Gemini
// Date: 2025-12-05
//
// Recommended Model: 2.5 Pro (due to integration with internal Mangle)
//
// Overview:
// This file implements the standalone `mangle-check` tool, designed to validate Google Mangle logic files (.mg).
// It adheres to the new flexible interface for Ouroboros-generated tools, functioning as both a CLI application
// and an agent-integrated tool. This demonstrates the enhanced capabilities of the Ouroboros tool builder.
//
// Key Features & Business Value:
// - Static Binary: Compiled to a single, portable executable.
// - Robust Validation: Leverages the official `codenerd/internal/mangle` parser for accurate syntax and basic semantic checks.
// - Dual Interface: Automatically detects if it's being run via agent (JSON payload) or directly as a CLI.
// - Cross-Platform: Built with `CGO_ENABLED=0` and configurable `GOOS`/`GOARCH` for broad compatibility.
//
// Architectural Context:
// - Component Type: Autopoiesis-Generated Tool (Go executable)
// - Location: .nerd/tools/ (Source), .nerd/tools/.compiled/ (Binary)
// - Integration: Orchestrated by `autopoiesis.OuroborosLoop`, executed by `core.VirtualStore`.
//
// Dependencies & Dependents:
// - Dependencies: `codenerd/internal/mangle` (resolved during compilation via `go.mod` replace directive).
// - Is a Dependency for: Agent workflows needing Mangle syntax checks and the Autopoiesis self-improvement system.
//
// Deployment & Operations:
// - CI/CD: Built via `go build -ldflags="-s -w"`, with `CGO_ENABLED=0` and `GOOS`/`GOARCH` support.
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling and clear logging.
//
// Functions / Classes:
// - `RunTool(input string, args []string)`:
//   - Purpose: Main entry point for the tool's logic, supporting both agent (JSON) and CLI invocation.
//   - Logic: Interprets `input` and `args` as file patterns, performs Mangle validation, and returns results.
// - `checkMangleFile(engine *mangle.Engine, path string)`:
//   - Purpose: Validates a single Mangle file.
//   - Logic: Initializes a Mangle engine, pre-loads `schemas.mg`, and attempts to load the target file.
//
// Usage (Agent execution):
// The agent will call this tool via the `core.VirtualStore`'s `ActionExecCmd`, passing JSON input
// (e.g., `{"input": "path/to/file.mg", "args": []}`).
//
// Usage (Standalone CLI):
// `mangle-check.exe path/to/file.mg internal/mangle/*.mg`
//
// --- END OF PRD HEADER ---

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
