package world

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a temporary git repository for testing.
// Returns the repository path and a cleanup function.
func setupGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Set git config for commits
	configCmds := [][]string{
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@example.com"},
	}

	for _, args := range configCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to set git config %v: %v", args, err)
		}
	}

	// Create a test file and commit it
	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	// Ensure stable commit time by setting GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=1700000000 +0000", "GIT_COMMITTER_DATE=1700000000 +0000")

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Modify the file and commit again
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add second: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Second commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=1700000060 +0000", "GIT_COMMITTER_DATE=1700000060 +0000")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit second: %v", err)
	}

	return dir, func() {} // TempDir is automatically cleaned up
}

func TestScanGitHistory(t *testing.T) {
	ctx := context.Background()
	repoDir, _ := setupGitRepo(t)

	t.Run("ValidGitRepo", func(t *testing.T) {
		facts, err := ScanGitHistory(ctx, repoDir, 10)
		if err != nil {
			t.Fatalf("ScanGitHistory failed: %v", err)
		}

		// Expecting at least 1 git_history fact (for test.go) and 1 churn_rate fact.
		// Since we have 2 commits touching test.go, we should have 2 git_history facts
		// and 1 churn_rate fact.
		if len(facts) != 3 {
			t.Errorf("Expected 3 facts, got %d", len(facts))
		}

		var historyCount, churnCount int
		for _, f := range facts {
			switch f.Predicate {
			case "git_history":
				historyCount++
				if len(f.Args) != 5 {
					t.Errorf("Expected 5 args for git_history, got %d", len(f.Args))
				}
				if f.Args[0] != "test.go" {
					t.Errorf("Expected filename 'test.go', got '%v'", f.Args[0])
				}
			case "churn_rate":
				churnCount++
				if len(f.Args) != 2 {
					t.Errorf("Expected 2 args for churn_rate, got %d", len(f.Args))
				}
				if f.Args[0] != "test.go" {
					t.Errorf("Expected filename 'test.go', got '%v'", f.Args[0])
				}
				// churn_rate is number of commits touching the file, so it should be 2
				if f.Args[1] != 2 {
					t.Errorf("Expected churn count 2, got '%v'", f.Args[1])
				}
			default:
				t.Errorf("Unexpected predicate: %s", f.Predicate)
			}
		}

		if historyCount != 2 {
			t.Errorf("Expected 2 git_history facts, got %d", historyCount)
		}
		if churnCount != 1 {
			t.Errorf("Expected 1 churn_rate fact, got %d", churnCount)
		}
	})

	t.Run("NotAGitRepo", func(t *testing.T) {
		// Use a temporary directory that is NOT a git repo
		emptyDir := t.TempDir()

		facts, err := ScanGitHistory(ctx, emptyDir, 10)
		if err != nil {
			t.Fatalf("ScanGitHistory failed: %v", err)
		}

		if len(facts) != 0 {
			t.Errorf("Expected 0 facts for non-git repo, got %d", len(facts))
		}
	})

	t.Run("DepthLimit", func(t *testing.T) {
		// Test with depth 1
		facts, err := ScanGitHistory(ctx, repoDir, 1)
		if err != nil {
			t.Fatalf("ScanGitHistory failed: %v", err)
		}

		// 1 history fact + 1 churn fact = 2 facts total
		if len(facts) != 2 {
			t.Errorf("Expected 2 facts with depth=1, got %d", len(facts))
		}

		var historyCount int
		for _, f := range facts {
			if f.Predicate == "git_history" {
				historyCount++
			}
		}

		if historyCount != 1 {
			t.Errorf("Expected 1 git_history fact with depth=1, got %d", historyCount)
		}
	})
}
