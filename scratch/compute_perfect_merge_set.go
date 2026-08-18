//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
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
	data, err := os.ReadFile("scratch/branch_analysis.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var branches []BranchInfo
	if err := json.Unmarshal(data, &branches); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Files we want to deduplicate branches for (only keep the latest branch that modifies them)
	dedupeFiles := []string{
		"internal/autopoiesis/traces.go",
		"internal/mangle/feedback/error_classifier.go",
		"internal/world/parser_factory_test.go",
		"internal/mangle/synth/schema_test.go",
		"internal/autopoiesis/thunderdome.go",
		"tests/e2e/mcp_virtualstore_integration_test.go",
	}

	// Map to track the latest branch for each deduped file
	latestForFile := make(map[string]BranchInfo)
	for _, file := range dedupeFiles {
		var matching []BranchInfo
		for _, b := range branches {
			for _, f := range b.Files {
				if f == file {
					matching = append(matching, b)
					break
				}
			}
		}

		if len(matching) > 0 {
			sort.Slice(matching, func(i, j int) bool {
				t1, _ := time.Parse(time.RFC3339, matching[i].Date)
				t2, _ := time.Parse(time.RFC3339, matching[j].Date)
				return t1.After(t2)
			})
			latestForFile[file] = matching[0]
			fmt.Printf("For file [%s]: keeping latest branch: %s (%s)\n", file, matching[0].Name, matching[0].Date)
		}
	}

	// Now build the perfect merge set
	var mergeSet []BranchInfo
	discardedCount := 0

	for _, b := range branches {
		discard := false
		for _, file := range dedupeFiles {
			// Check if this branch touches the deduped file
			touches := false
			for _, f := range b.Files {
				if f == file {
					touches = true
					break
				}
			}

			if touches {
				// If this is NOT the latest branch for this file, discard it
				latestB := latestForFile[file]
				if b.Name != latestB.Name {
					discard = true
					break
				}
			}
		}

		if discard {
			discardedCount++
			continue
		}
		mergeSet = append(mergeSet, b)
	}

	// Sort final merge set by date ascending (oldest first)
	sort.Slice(mergeSet, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, mergeSet[i].Date)
		t2, _ := time.Parse(time.RFC3339, mergeSet[j].Date)
		return t1.Before(t2)
	})

	fmt.Printf("\nFiltered out %d redundant branches.\n", discardedCount)
	fmt.Printf("Perfect Merge Set size: %d branches\n", len(mergeSet))

	for _, b := range mergeSet {
		fmt.Printf("  - %s (%s)\n", b.Name, b.Date)
	}

	mBytes, err := json.MarshalIndent(mergeSet, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	os.WriteFile("scratch/merge_set.json", mBytes, 0644)
	fmt.Println("Perfect Merge Set written to scratch/merge_set.json")
}
