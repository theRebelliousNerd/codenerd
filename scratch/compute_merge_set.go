//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
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

	// Categories and their matchers
	categories := []struct {
		Name    string
		Matches func(BranchInfo) bool
	}{
		{
			Name: "MCP & VirtualStore E2E Tests",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "mcp") || strings.Contains(b.Name, "VirtualStore")
			},
		},
		{
			Name: "Parser Factory Tests",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "parser-factory")
			},
		},
		{
			Name: "Regex Optimization in Traces",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "regex-traces") || strings.Contains(b.Name, "perf-traces") || strings.Contains(b.Name, "optimize-regex-traces") || strings.Contains(b.Name, "performance-traces-regex") || strings.Contains(b.Name, "optimize-regex-autopoiesis") || strings.Contains(b.Name, "perf-autopoiesis-regex")
			},
		},
		{
			Name: "Error Classifier Optimization",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "error-classifier")
			},
		},
		{
			Name: "Autopoiesis Package Normalization (Thunderdome)",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "normalize-package") || strings.Contains(b.Name, "thunderdome")
			},
		},
		{
			Name: "Schema / Synth Tests",
			Matches: func(b BranchInfo) bool {
				return strings.Contains(b.Name, "schema")
			},
		},
	}

	// Group branches
	groups := make(map[string][]BranchInfo)
	var others []BranchInfo
	categorized := make(map[string]bool)

	for _, b := range branches {
		matched := false
		for _, cat := range categories {
			if cat.Matches(b) {
				groups[cat.Name] = append(groups[cat.Name], b)
				categorized[b.Name] = true
				matched = true
				break
			}
		}
		if !matched {
			others = append(others, b)
		}
	}

	// Select latest for each group
	var mergeSet []BranchInfo
	for groupName, brs := range groups {
		sort.Slice(brs, func(i, j int) bool {
			t1, _ := time.Parse(time.RFC3339, brs[i].Date)
			t2, _ := time.Parse(time.RFC3339, brs[j].Date)
			return t1.After(t2)
		})
		fmt.Printf("Group [%s]: Selected latest: %s (%s)\n", groupName, brs[0].Name, brs[0].Date)
		mergeSet = append(mergeSet, brs[0])
	}

	// Add others
	for _, b := range others {
		mergeSet = append(mergeSet, b)
	}

	// Sort final merge set by date ascending (oldest first so we build history naturally)
	sort.Slice(mergeSet, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, mergeSet[i].Date)
		t2, _ := time.Parse(time.RFC3339, mergeSet[j].Date)
		return t1.Before(t2)
	})

	fmt.Printf("\nTotal branches in target Merge Set: %d\n", len(mergeSet))
	for _, b := range mergeSet {
		fmt.Printf("  - %s (%s)\n", b.Name, b.Date)
	}

	// Write this merge set to json
	mBytes, err := json.MarshalIndent(mergeSet, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	os.WriteFile("scratch/merge_set.json", mBytes, 0644)
	fmt.Println("\nMerge set written to scratch/merge_set.json")
}
