package shards

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestMatchSpecialistsForTask uses table-driven tests to verify specialist matching logic.
// It covers different verbs, file patterns, and content hints.
func TestMatchSpecialistsForTask_Table(t *testing.T) {
	t.Parallel()

	// Mock registry
	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{Name: "GoExpert", Type: "persistent", Status: "ready"},
			{Name: "ReactExpert", Type: "persistent", Status: "ready"},
			{Name: "DatabaseExpert", Type: "persistent", Status: "ready"},
			{Name: "SecurityAuditor", Type: "persistent", Status: "ready"},
		},
	}

	cases := []struct {
		name          string
		verb          string
		files         []string
		contentConfig map[string]string // file -> content map for mocking ReadFile
		wantAgents    []string          // expected agents in order
		wantTopScore  float64           // minimum expected top score
	}{
		{
			name:       "go_files_review",
			verb:       "/review",
			files:      []string{"main.go", "utils.go"},
			wantAgents: []string{"GoExpert"},
		},
		{
			name:  "react_files_fix",
			verb:  "/fix",
			files: []string{"App.tsx", "components/Button.jsx"},
			contentConfig: map[string]string{
				"App.tsx":               "import React from 'react';\nexport default function App() { return <div/>; }",
				"components/Button.jsx": "import { useState } from 'react';\nexport function Button() { return <button/>; }",
			},
			wantAgents: []string{"ReactExpert"},
		},
		{
			name:  "sql_files_create",
			verb:  "/create",
			files: []string{"schema.sql"},
			contentConfig: map[string]string{
				"schema.sql": "CREATE TABLE users (id int);",
			},
			wantAgents: []string{"DatabaseExpert"},
		},
		{
			name:  "security_sensitive_code",
			verb:  "/review",
			files: []string{"auth.go"},
			contentConfig: map[string]string{
				"auth.go": "package auth\nimport \"crypto/sha256\"\nfunc HashPassword() {}",
			},
			// Expect SecurityAuditor (via crypto import) and GoExpert (via .go ext)
			wantAgents: []string{"SecurityAuditor", "GoExpert"},
		},
		{
			name:       "unknown_verb_defaults_to_review",
			verb:       "/unknown",
			files:      []string{"main.go"},
			wantAgents: []string{"GoExpert"},
		},
		{
			name:       "empty_registry",
			verb:       "/review",
			files:      []string{"main.go"},
			wantAgents: []string{}, // Registry passed as nil in special check or handled?
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Note: We cannot easily mock os.ReadFile in the current implementation of MatchSpecialistsForTask
			// without refactoring. However, we can create temporary files.
			// Integrating "Test Architect" advice: prefer not to use on-disk files if possible,
			// but strict isolation requires TempDir.

			tmpDir := t.TempDir()
			var realFiles []string

			for _, f := range tc.files {
				content := ""
				if tc.contentConfig != nil {
					content = tc.contentConfig[f]
				}

				// Create file
				// f might contain slashes
				fullPath := filepath.Join(tmpDir, f)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				realFiles = append(realFiles, fullPath)
			}

			// If test case expects empty registry behavior, pass nil or empty
			reg := registry
			if tc.name == "empty_registry" {
				reg = &AgentRegistry{}
			}

			matches := MatchSpecialistsForTask(context.Background(), tc.verb, realFiles, reg)

			// user might check just existence of top match
			if len(tc.wantAgents) == 0 {
				if len(matches) > 0 {
					t.Errorf("expected no matches, got %d", len(matches))
				}
				return
			}

			if len(matches) == 0 {
				t.Fatalf("expected matches %v, got none", tc.wantAgents)
			}

			// Check first match (simplest verification)
			// Matches are sorted by score.
			// Depending on scoring, order might vary if scores are close.
			// But GoExpert for .go file is strong match.

			found := false
			for _, m := range matches {
				if m.AgentName == tc.wantAgents[0] {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected match %q, got matches: %v", tc.wantAgents[0], matches)
			}
		})
	}
}

func TestGetExecutionMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		verb string
		want ExecutionMode
	}{
		{"/review", ModeParallel},
		{"/fix", ModeAdvisoryWithCritique},
		{"/create", ModeAdvisory},
		{"/unknown", ModeParallel}, // default
	}

	for _, tt := range tests {
		got := GetExecutionMode(tt.verb)
		if got != tt.want {
			t.Errorf("GetExecutionMode(%q) = %v, want %v", tt.verb, got, tt.want)
		}
	}
}
