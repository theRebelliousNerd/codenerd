// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains task parsing and file expansion logic.
package reviewer

import (
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// TASK PARSING
// =============================================================================

// ReviewerTask represents a parsed review task.
type ReviewerTask struct {
	Action            string   // "review", "security_scan", "style_check", "complexity", "diff"
	Files             []string // Files to review
	DiffRef           string   // Git diff reference (e.g., "HEAD~1")
	Options           map[string]string
	EnableEnhancement bool // --andEnhance flag enables creative suggestions (Steps 8-12)
}

// parseTask extracts action and parameters from task string.
func (r *ReviewerShard) parseTask(task string) (*ReviewerTask, error) {
	parsed := &ReviewerTask{
		Action:  "review",
		Files:   make([]string, 0),
		Options: make(map[string]string),
	}

	parts := strings.Fields(task)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty task")
	}

	// First token is the action
	action := strings.ToLower(parts[0])
	switch action {
	case "review", "check":
		parsed.Action = "review"
	case "security_scan", "security", "scan":
		parsed.Action = "security_scan"
	case "style_check", "style", "lint":
		parsed.Action = "style_check"
	case "complexity", "metrics":
		parsed.Action = "complexity"
	case "diff":
		parsed.Action = "diff"
	default:
		// Assume review if action is a file path
		if strings.Contains(action, ".") || strings.Contains(action, "/") {
			parsed.Action = "review"
			parsed.Files = append(parsed.Files, action)
		}
	}

	// Parse key:value pairs
	for _, part := range parts[1:] {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.ToLower(kv[0])
			value := kv[1]

			switch key {
			case "file":
				parsed.Files = append(parsed.Files, value)
			case "files":
				// Comma-separated list
				for _, f := range strings.Split(value, ",") {
					if f = strings.TrimSpace(f); f != "" {
						parsed.Files = append(parsed.Files, f)
					}
				}
			case "diff":
				parsed.DiffRef = value
				parsed.Action = "diff"
			case "pr":
				// PR files format: pr:files:a.go,b.go
				if strings.HasPrefix(value, "files:") {
					files := strings.TrimPrefix(value, "files:")
					for _, f := range strings.Split(files, ",") {
						if f = strings.TrimSpace(f); f != "" {
							parsed.Files = append(parsed.Files, f)
						}
					}
				}
			default:
				parsed.Options[key] = value
			}
		} else if strings.HasPrefix(part, "--") {
			// Handle double-dash flags
			flag := strings.TrimPrefix(part, "--")
			switch strings.ToLower(flag) {
			case "andenhance", "enhance":
				parsed.EnableEnhancement = true
				logging.ReviewerDebug("Enhancement mode enabled via --%s flag", flag)
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - treat as file
			parsed.Files = append(parsed.Files, part)
		}
	}

	return parsed, nil
}

func (r *ReviewerShard) expandTaskFiles(task *ReviewerTask) {
	if task == nil || task.Action == "diff" {
		return
	}

	explicit := make([]string, 0, len(task.Files))
	broadRequested := false
	for _, file := range task.Files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		if isBroadTargetToken(trimmed) {
			broadRequested = true
			continue
		}
		explicit = append(explicit, trimmed)
	}

	if len(explicit) > 0 && !broadRequested {
		task.Files = explicit
		return
	}
	task.Files = explicit

	if r.kernel == nil {
		if len(task.Files) == 0 {
			logging.ReviewerDebug("Skipping file expansion: kernel unavailable")
		}
		return
	}

	facts, err := r.kernel.Query("file_topology")
	if err != nil {
		logging.ReviewerDebug("file_topology query failed: %v", err)
		return
	}

	expanded := make([]string, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		path := fmt.Sprintf("%v", fact.Args[0])
		isTest := fmt.Sprintf("%v", fact.Args[4])
		if isTest == "/true" {
			continue
		}
		if !isCodeFile(path) {
			continue
		}
		expanded = append(expanded, path)
	}

	if len(expanded) == 0 {
		return
	}

	task.Files = dedupeFiles(append(task.Files, expanded...))
	if len(task.Files) > 50 {
		task.Files = task.Files[:50]
	}
}

func isBroadTargetToken(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "all", "codebase", "*", ".", "repo", "project", "workspace":
		return true
	default:
		return false
	}
}

func isCodeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h", ".cs", ".rb", ".php":
		return true
	default:
		return false
	}
}

func dedupeFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	result := make([]string, 0, len(files))
	for _, file := range files {
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		result = append(result, file)
	}
	return result
}
