package world

import (
	"bufio"
	"bytes"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gitFieldSep separates the fields of a commit header. A commit message or an
// author name may contain any printable character; the unit separator it cannot.
const gitFieldSep = "\x1f"

// ScanGitHistory performs a scan of git history to generate churn and history facts.
// It uses 'git log' to retrieve commit data.
//
//	git_history(File, Commit, Author, CommitTime, Message)  one row per commit and file;
//	                                                         CommitTime is unix seconds
//	churn_rate(File, Commits)                                commits in the window touching File
//
// Paths are relative to root and use forward slashes, the spelling file_topology
// uses, so a rule can join history to the world model. Renames are read as a
// delete and an add: both paths keep exactly the commits that touched them.
// CommitTime is a measurement, not an age: age is judged in Mangle against
// current_time (policy/git_safety.mg), so the same facts replay at any time.
func ScanGitHistory(ctx context.Context, root string, depth int) ([]core.Fact, error) {
	logging.World("Starting git history scan: %s (depth=%d)", root, depth)
	start := time.Now()

	// 1. Check if git is available and root is a git repo
	if err := checkGitRepo(ctx, root); err != nil {
		logging.WorldDebug("Skipping git scan (not a repo or git missing): %v", err)
		return nil, nil // Not an error, just skip
	}

	// 2. Fetch git log with numstat to calculate churn
	cmd := exec.CommandContext(ctx, "git", "-c", "core.quotepath=false", "log",
		fmt.Sprintf("-n%d", depth),
		"--relative", "--no-renames",
		"--pretty=format:COMMIT:%H%x1f%an%x1f%ct%x1f%s",
		"--numstat",
	)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	facts := make([]core.Fact, 0)
	churnMap := make(map[string]int64)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var currentHash, currentAuthor, currentMsg string
	var currentTs int64

	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "COMMIT:"); ok {
			parts := strings.SplitN(after, gitFieldSep, 4)
			if len(parts) == 4 {
				currentHash = parts[0]
				currentAuthor = parts[1]
				currentMsg = parts[3]
				if ts, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
					currentTs = ts
				}
			}
			continue
		}

		// numstat line: "<added>\t<deleted>\t<path>"; a binary file reports "-".
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || fields[2] == "" {
			continue
		}
		filePath := fields[2]

		facts = append(facts, core.Fact{
			Predicate: "git_history",
			Args: []any{
				filePath,
				currentHash,
				currentAuthor,
				currentTs,
				currentMsg,
			},
		})
		churnMap[filePath]++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("git log output: %w", err)
	}

	// 3. Generate churn_rate facts
	for file, count := range churnMap {
		facts = append(facts, core.Fact{
			Predicate: "churn_rate",
			Args: []any{
				file,
				count,
			},
		})
	}

	logging.World("Git history scan complete: %d facts generated in %v", len(facts), time.Since(start))
	return facts, nil
}

func checkGitRepo(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run()
}
