//go:build ignore

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
		fmt.Printf("Error reading analysis: %v\n", err)
		return
	}

	var branches []BranchInfo
	if err := json.Unmarshal(data, &branches); err != nil {
		fmt.Printf("Error unmarshaling: %v\n", err)
		return
	}

	filesToWatch := []string{
		"internal/autopoiesis/traces.go",
		"internal/world/parser_factory_test.go",
		"internal/mangle/synth/schema_test.go",
		"internal/mangle/feedback/error_classifier.go",
		"internal/autopoiesis/thunderdome.go",
		"tests/e2e/mcp_virtualstore_integration_test.go",
	}

	for _, file := range filesToWatch {
		fmt.Printf("\n=== Analyzing branches modifying: %s ===\n", file)
		hashes := make(map[string][]string) // sha256 -> branch names

		for _, b := range branches {
			hasFile := false
			for _, f := range b.Files {
				if f == file {
					hasFile = true
					break
				}
			}
			if !hasFile {
				continue
			}

			// Show content hash of this file on this branch
			cmd := exec.Command("git", "show", b.Name+":"+file)
			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err == nil {
				h := sha256.Sum256(out.Bytes())
				hexStr := hex.EncodeToString(h[:])
				hashes[hexStr] = append(hashes[hexStr], b.Name)
			}
		}

		for hexStr, brs := range hashes {
			fmt.Printf("  Hash %s (used by %d branches):\n", hexStr[:8], len(brs))
			for _, br := range brs {
				fmt.Printf("    - %s\n", br)
			}
		}
	}
}
