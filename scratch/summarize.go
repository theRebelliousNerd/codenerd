//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	var branches []BranchInfo
	if err := json.Unmarshal(data, &branches); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		return
	}

	// Track which branches modify which files
	fileToBranches := make(map[string][]string)
	for _, b := range branches {
		for _, file := range b.Files {
			fileToBranches[file] = append(fileToBranches[file], b.Name)
		}
	}

	// Sort files by number of branches modifying them
	type FileStat struct {
		File     string
		Branches []string
	}
	var stats []FileStat
	for file, brs := range fileToBranches {
		stats = append(stats, FileStat{File: file, Branches: brs})
	}
	sort.Slice(stats, func(i, j int) bool {
		return len(stats[i].Branches) > len(stats[j].Branches)
	})

	fmt.Println("Top overlapping files across branches:")
	count := 0
	for _, stat := range stats {
		if len(stat.Branches) > 1 {
			fmt.Printf("- %s (modified by %d branches):\n", stat.File, len(stat.Branches))
			for _, br := range stat.Branches {
				fmt.Printf("  * %s\n", br)
			}
			count++
			if count >= 10 {
				fmt.Println("... and more files.")
				break
			}
		}
	}
	if count == 0 {
		fmt.Println("No files are shared/modified by multiple branches!")
	}
}
