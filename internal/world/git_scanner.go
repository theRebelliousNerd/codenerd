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

// ScanGitHistory performs a scan of git history to generate churn and history facts.
// It uses 'git log' to retrieve commit data.
func ScanGitHistory(ctx context.Context, root string, depth int) ([]core.Fact, error) {
	logging.World("Starting git history scan: %s (depth=%d)", root, depth)
	start := time.Now()

	// 1. Check if git is available and root is a git repo
	if err := checkGitRepo(ctx, root); err != nil {
		logging.WorldDebug("Skipping git scan (not a repo or git missing): %v", err)
		return nil, nil // Not an error, just skip
	}

	// 2. Fetch git log with numstat to calculate churn
	// Format: hash|author|timestamp|message
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("-n%d", depth),
		"--pretty=format:COMMIT:%H|%an|%ct|%s",
		"--numstat",
	)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	facts := make([]core.Fact, 0)
	churnMap := make(map[string]int)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentHash, currentAuthor, currentMsg string
	var currentTs int64

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "COMMIT:") {
			parts := strings.Split(strings.TrimPrefix(line, "COMMIT:"), "|")
			if len(parts) >= 4 {
				currentHash = parts[0]
				currentAuthor = parts[1]
				tsStr := parts[2]
				currentMsg = parts[3]

				if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					currentTs = ts
				}
			}
			continue
		}

		// Parse numstat lines: params added   deleted   file
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			// numstat output: "1	2	file.go"
			filePath := fields[2]

			// Generate git_history fact for this file/commit
			facts = append(facts, core.Fact{
				Predicate: "git_history",
				Args: []interface{}{
					filePath,
					currentHash,
					currentAuthor,
					currentTs,
					currentMsg,
				},
			})

			// Update churn count
			churnMap[filePath]++
		}
	}

	// 3. Generate churn_rate facts
	for file, count := range churnMap {
		facts = append(facts, core.Fact{
			Predicate: "churn_rate",
			Args: []interface{}{
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
