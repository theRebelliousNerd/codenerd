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

func getUnmergedFiles() ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var unmerged []string
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		isUnmerged := status == "DD" || status == "AA" || strings.Contains(status, "U")
		if isUnmerged {
			unmerged = append(unmerged, line[3:])
		}
	}
	return unmerged, nil
}

func isSafeToAutoResolve(file string) bool {
	file = strings.TrimSpace(file)
	safeExts := []string{".json", ".py", ".sh", ".patch", ".txt", ".md", ".orig"}
	for _, ext := range safeExts {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	if strings.Contains(file, ".nerd/campaigns") || strings.Contains(file, "sessions/sess_") {
		return true
	}
	return false
}

func resolveTransientConflicts(unmerged []string, branchName string) bool {
	for _, file := range unmerged {
		file = strings.TrimSpace(file)
		if !isSafeToAutoResolve(file) {
			fmt.Printf("Core file is unmerged: %s\n", file)
			return false
		}
	}

	fmt.Printf("Conflicts are only in transient/script/doc files. Resolving automatically by choosing '--ours'...\n")
	for _, file := range unmerged {
		file = strings.TrimSpace(file)
		checkoutCmd := exec.Command("git", "checkout", "--ours", file)
		_ = checkoutCmd.Run()

		var addCmd *exec.Cmd
		if _, err := os.Stat(file); err == nil {
			addCmd = exec.Command("git", "add", file)
		} else {
			addCmd = exec.Command("git", "rm", file)
		}

		if err := addCmd.Run(); err != nil {
			fmt.Printf("Failed to resolve %s: %v\n", file, err)
			return false
		}
	}

	// Commit the merge
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("merge: auto-resolve transient conflicts from %s", branchName))
	var out bytes.Buffer
	commitCmd.Stdout = &out
	commitCmd.Stderr = &out
	if err := commitCmd.Run(); err != nil {
		fmt.Printf("Failed to commit merge resolution: %v\nOutput:\n%s\n", err, out.String())
		return false
	}

	fmt.Println("Conflict resolved and committed successfully.")
	return true
}

func main() {
	data, err := os.ReadFile("scratch/merge_set.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var mergeSet []BranchInfo
	if err := json.Unmarshal(data, &mergeSet); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Find the current merge state starting from index 0
	startIdx := 0
	for i := 0; i < len(mergeSet); i++ {
		cmd := exec.Command("git", "merge-base", "--is-ancestor", mergeSet[i].Name, "HEAD")
		if err := cmd.Run(); err == nil {
			fmt.Printf("Branch %s is already merged. Skipping.\n", mergeSet[i].Name)
			startIdx = i + 1
		} else {
			break
		}
	}

	fmt.Printf("Resuming sequential merge of remaining %d branches starting at index %d...\n", len(mergeSet)-startIdx, startIdx)

	for i := startIdx; i < len(mergeSet); i++ {
		b := mergeSet[i]
		fmt.Printf("[%d/%d] Merging %s: %q...\n", i, len(mergeSet)-1, b.Name, b.Subject)

		// First ensure clean state
		resetCmd := exec.Command("git", "reset", "--hard")
		if err := resetCmd.Run(); err != nil {
			fmt.Printf("ERROR: git reset --hard failed: %v\n", err)
			os.Exit(1)
		}

		cmd := exec.Command("git", "merge", "--no-edit", b.Name)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			// Check if we can auto-resolve conflicts
			unmerged, statErr := getUnmergedFiles()
			if statErr != nil {
				fmt.Printf("ERROR: git merge failed and status check failed: %v\nOutput:\n%s\n", statErr, out.String())
				os.Exit(1)
			}

			if len(unmerged) > 0 && resolveTransientConflicts(unmerged, b.Name) {
				fmt.Printf("Branch %s merged with auto-resolution.\n", b.Name)
				continue
			}

			fmt.Printf("ERROR: Failed to merge %s: %v\nOutput:\n%s\n", b.Name, err, out.String())
			os.Exit(1)
		}
		fmt.Printf("Successfully merged %s.\n", b.Name)
	}

	fmt.Println("All branches merged successfully!")
}
