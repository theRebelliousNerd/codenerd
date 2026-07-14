//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type BranchInfo struct {
	Name        string   `json:"name"`
	Hash        string   `json:"hash"`
	Date        string   `json:"date"`
	Author      string   `json:"author"`
	Subject     string   `json:"subject"`
	Mergeable   bool     `json:"mergeable"`
	ConflictErr string   `json:"conflict_err,omitempty"`
	Files       []string `json:"files"`
}

func main() {
	// 1. Get list of unmerged branches
	cmd := exec.Command("git", "branch", "-r", "--no-merged", "origin/main")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error listing branches: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(out.String(), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "origin/HEAD") || line == "origin/main" {
			continue
		}
		branches = append(branches, line)
	}

	fmt.Printf("Analyzing %d unmerged remote branches...\n", len(branches))

	var results []BranchInfo
	for i, b := range branches {
		info := BranchInfo{Name: b}

		// Get metadata
		logCmd := exec.Command("git", "log", "-1", "--format=%H|%cI|%an|%s", b)
		var logOut bytes.Buffer
		logCmd.Stdout = &logOut
		if err := logCmd.Run(); err == nil {
			parts := strings.SplitN(strings.TrimSpace(logOut.String()), "|", 4)
			if len(parts) == 4 {
				info.Hash = parts[0]
				info.Date = parts[1]
				info.Author = parts[2]
				info.Subject = parts[3]
			}
		}

		// Get changed files
		// Find the merge base first to avoid argument concatenation vulnerability
		mbCmd := exec.Command("git", "merge-base", "origin/main", b)
		var mbOut bytes.Buffer
		mbCmd.Stdout = &mbOut
		if err := mbCmd.Run(); err == nil {
			mergeBase := strings.TrimSpace(mbOut.String())

			// Use the merge base as a separate argument to avoid injection
			diffCmd := exec.Command("git", "diff", "--name-only", mergeBase, b)
			var diffOut bytes.Buffer
			diffCmd.Stdout = &diffOut
			if err := diffCmd.Run(); err == nil {
				fileLines := strings.Split(diffOut.String(), "\n")
				for _, fl := range fileLines {
					fl = strings.TrimSpace(fl)
					if fl != "" {
						info.Files = append(info.Files, fl)
					}
				}
			}
		}

		// Check mergeability
		mergeCmd := exec.Command("git", "merge-tree", "origin/main", b)
		var mergeErr bytes.Buffer
		mergeCmd.Stderr = &mergeErr
		if err := mergeCmd.Run(); err != nil {
			info.Mergeable = false
			// If it exited with status 1, it's just a conflict, otherwise it's a real error.
			info.ConflictErr = strings.TrimSpace(mergeErr.String())
			if info.ConflictErr == "" {
				info.ConflictErr = err.Error()
			}
		} else {
			info.Mergeable = true
		}

		results = append(results, info)
		if (i+1)%10 == 0 || i == len(branches)-1 {
			fmt.Printf("  Progress: %d/%d branches analyzed...\n", i+1, len(branches))
		}
	}

	// Write results to JSON file
	resBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("scratch/branch_analysis.json", resBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Analysis complete. Results written to scratch/branch_analysis.json")
}
