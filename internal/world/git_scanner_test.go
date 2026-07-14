package world

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScanGitHistory(t *testing.T) {
	// 1. Setup a temporary directory for the git repository
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	// Configure git author for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	// Create a file and commit it
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package test\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add test file to git: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create a second commit to test churn
	if err := os.WriteFile(filePath, []byte("package test\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add modified test file to git: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Second commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// 2. Call ScanGitHistory
	ctx := context.Background()
	facts, err := ScanGitHistory(ctx, tmpDir, 10)
	if err != nil {
		t.Fatalf("ScanGitHistory failed: %v", err)
	}

	// 3. Verify results
	// We expect git_history facts and a churn_rate fact
	var historyFactsCount int
	var churnFactsCount int

	for _, fact := range facts {
		switch fact.Predicate {
		case "git_history":
			historyFactsCount++
			if len(fact.Args) != 5 {
				t.Errorf("expected 5 args for git_history fact, got %d", len(fact.Args))
			}
			// Args: filePath, currentHash, currentAuthor, currentTs, currentMsg
			if file, ok := fact.Args[0].(string); !ok || file != "test.go" {
				t.Errorf("unexpected file in git_history fact: %v", fact.Args[0])
			}
			if author, ok := fact.Args[2].(string); !ok || author != "Test User" {
				t.Errorf("unexpected author in git_history fact: %v", fact.Args[2])
			}
		case "churn_rate":
			churnFactsCount++
			if len(fact.Args) != 2 {
				t.Errorf("expected 2 args for churn_rate fact, got %d", len(fact.Args))
			}
			// Args: file, count
			if file, ok := fact.Args[0].(string); !ok || file != "test.go" {
				t.Errorf("unexpected file in churn_rate fact: %v", fact.Args[0])
			}
			if count, ok := fact.Args[1].(int); !ok || count != 2 {
				t.Errorf("expected churn count of 2, got %v", fact.Args[1])
			}
		default:
			t.Errorf("unexpected fact predicate: %s", fact.Predicate)
		}
	}

	if historyFactsCount != 2 {
		t.Errorf("expected 2 git_history facts, got %d", historyFactsCount)
	}
	if churnFactsCount != 1 {
		t.Errorf("expected 1 churn_rate fact, got %d", churnFactsCount)
	}
}

func TestScanGitHistory_NotARepo(t *testing.T) {
	// Setup a temporary directory without git initialized
	tmpDir := t.TempDir()

	ctx := context.Background()
	facts, err := ScanGitHistory(ctx, tmpDir, 10)
	if err != nil {
		t.Fatalf("ScanGitHistory should not fail on non-repo: %v", err)
	}
	if facts != nil {
		t.Errorf("expected nil facts for non-repo, got %v", facts)
	}
}
