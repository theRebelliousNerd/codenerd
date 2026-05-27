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

	// Define categories based on files/names
	categories := []struct {
		Name     string
		Matches  func(BranchInfo) bool
		Branches []BranchInfo
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
		{
			Name: "Others",
			Matches: func(b BranchInfo) bool {
				return true
			},
		},
	}

	categorized := make(map[string]bool)

	for _, b := range branches {
		for i := range categories {
			if categories[i].Matches(b) && !categorized[b.Name] {
				categories[i].Branches = append(categories[i].Branches, b)
				categorized[b.Name] = true
				break
			}
		}
	}

	for _, cat := range categories {
		if len(cat.Branches) == 0 {
			continue
		}

		// Sort by date descending
		sort.Slice(cat.Branches, func(i, j int) bool {
			t1, _ := time.Parse(time.RFC3339, cat.Branches[i].Date)
			t2, _ := time.Parse(time.RFC3339, cat.Branches[j].Date)
			return t1.After(t2)
		})

		fmt.Printf("\n=== Category: %s (%d branches) ===\n", cat.Name, len(cat.Branches))
		for idx, b := range cat.Branches {
			mark := " "
			if idx == 0 {
				mark = "*" // Latest in category
			}
			fmt.Printf("  %s [%s] %s (%s) - %s\n", mark, b.Date, b.Name, b.Hash[:8], b.Subject)
		}
	}
}
