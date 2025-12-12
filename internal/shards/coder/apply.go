package coder

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/world"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// =============================================================================
// EDIT APPLICATION
// =============================================================================

// applyEdits writes the code changes via VirtualStore.
func (c *CoderShard) applyEdits(ctx context.Context, edits []CodeEdit) error {
	logging.Coder("Applying %d edits", len(edits))

	tx := NewFileTransaction()
	applied := make([]CodeEdit, 0, len(edits))
	committed := false
	defer func() {
		if !committed {
			logging.Coder("ApplyEdits failed after %d/%d edits; rolling back filesystem state", len(applied), len(edits))
			tx.Rollback()
			// Best-effort retract of modified facts for rolled-back edits.
			if c.kernel != nil {
				for _, e := range applied {
					_ = c.kernel.RetractFact(core.Fact{
						Predicate: "modified",
						Args:      []interface{}{e.File},
					})
				}
			}
		} else {
			tx.Commit()
		}
	}()

	var tsParser *world.TreeSitterParser
	defer func() {
		if tsParser != nil {
			tsParser.Close()
		}
	}()

	for i, edit := range edits {
		logging.CoderDebug("Edit[%d]: type=%s, file=%s, content_len=%d",
			i, edit.Type, edit.File, len(edit.NewContent))

		fullPath := c.resolvePath(edit.File)

		if err := tx.Stage(fullPath); err != nil {
			return fmt.Errorf("failed to stage transaction for %s: %w", edit.File, err)
		}

		// Pre-apply verification for modifications.
		if edit.Type == "modify" || edit.Type == "refactor" || edit.Type == "fix" {
			// If old content was provided, require an exact match to avoid clobbering unexpected changes.
			if edit.OldContent != "" {
				current, err := os.ReadFile(fullPath)
				if err != nil {
					return fmt.Errorf("failed to read existing file for verification (%s): %w", edit.File, err)
				}
				if string(current) != edit.OldContent {
					return fmt.Errorf("old_content mismatch for %s; refusing to apply edit", edit.File)
				}

				// Minimal-edit guard: block large rewrites unless explicitly refactoring.
				if edit.Type == "modify" || edit.Type == "fix" {
					ratio, total := estimateLineChangeRatio(edit.OldContent, edit.NewContent)
					if total > 30 && ratio > 0.6 {
						return fmt.Errorf("edit to %s changes %.0f%% of lines; refusing large rewrite without refactor intent", edit.File, ratio*100)
					}
				}
			}

		}

		// Lightweight syntax gates for supported languages (including creates).
		if edit.Type != "delete" {
			lang := strings.ToLower(edit.Language)
			ext := strings.ToLower(filepath.Ext(fullPath))

			// Go syntax gate via stdlib
			if lang == "go" || ext == ".go" {
				fset := token.NewFileSet()
				if _, err := parser.ParseFile(fset, fullPath, edit.NewContent, parser.AllErrors); err != nil {
					return fmt.Errorf("go syntax check failed for %s: %w", edit.File, err)
				}
			}

			// Tree-sitter gates for other first-class languages
			if tsParser == nil && (lang == "python" || lang == "rust" || lang == "typescript" || lang == "javascript" ||
				ext == ".py" || ext == ".rs" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx") {
				tsParser = world.NewTreeSitterParser()
			}
			if tsParser != nil {
				src := []byte(edit.NewContent)
				switch {
				case lang == "python" || ext == ".py":
					if _, err := tsParser.ParsePython(fullPath, src); err != nil {
						return fmt.Errorf("python syntax check failed for %s: %w", edit.File, err)
					}
				case lang == "rust" || ext == ".rs":
					if _, err := tsParser.ParseRust(fullPath, src); err != nil {
						return fmt.Errorf("rust syntax check failed for %s: %w", edit.File, err)
					}
				case lang == "typescript" || ext == ".ts" || ext == ".tsx":
					if _, err := tsParser.ParseTypeScript(fullPath, src); err != nil {
						return fmt.Errorf("typescript syntax check failed for %s: %w", edit.File, err)
					}
				case lang == "javascript" || ext == ".js" || ext == ".jsx":
					if _, err := tsParser.ParseJavaScript(fullPath, src); err != nil {
						return fmt.Errorf("javascript syntax check failed for %s: %w", edit.File, err)
					}
				}
			}
		}

		c.mu.Lock()
		c.editHistory = append(c.editHistory, edit)
		c.mu.Unlock()

		if c.virtualStore == nil {
			// No virtual store, write directly
			logging.CoderDebug("No VirtualStore, writing directly to filesystem")
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to create directory for %s: %v", fullPath, err)
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(edit.NewContent), 0644); err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to write file %s: %v", fullPath, err)
				return fmt.Errorf("failed to write file: %w", err)
			}
			logging.CoderDebug("Direct write successful: %s", fullPath)
			applied = append(applied, edit)
			continue
		}

		// Route unsafe mutations through the kernel OODA pipeline when available.
		var actionType string
		payload := map[string]interface{}{}
		switch edit.Type {
		case "delete":
			logging.CoderDebug("Deleting file: %s", edit.File)
			actionType = "/delete_file"
			payload["confirmed"] = true
		default:
			// create/modify/refactor/fix all map to write_file with content payload
			logging.CoderDebug("Writing file (%s): %s", edit.Type, edit.File)
			actionType = "/write_file"
			payload["content"] = edit.NewContent
		}

		// If kernel is attached, go through executive->constitution->router.
		if c.kernel != nil {
			actionID := fmt.Sprintf("action-%d", time.Now().UnixNano())
			payload["action_id"] = actionID

			nextAction := core.Fact{
				Predicate: "next_action",
				Args:      []interface{}{actionType, edit.File, payload},
			}

			editTimer := logging.StartTimer(logging.CategoryCoder, fmt.Sprintf("ApplyEdit:%s", edit.File))
			if err := c.kernel.Assert(nextAction); err != nil {
				editTimer.Stop()
				return fmt.Errorf("failed to assert next_action for %s: %w", edit.File, err)
			}

			_, err := c.waitForRoutingResult(ctx, actionID)
			editTimer.Stop()
			if err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to apply edit to %s via kernel: %v", edit.File, err)
				return fmt.Errorf("failed to apply edit to %s: %w", edit.File, err)
			}
			logging.CoderDebug("Successfully applied edit via kernel pipeline: %s", edit.File)
		} else {
			// Fallback to direct VirtualStore routing (legacy/testing).
			action := core.Fact{
				Predicate: "next_action",
				Args:      []interface{}{actionType, edit.File, payload},
			}
			editTimer := logging.StartTimer(logging.CategoryCoder, fmt.Sprintf("ApplyEdit:%s", edit.File))
			_, err := c.virtualStore.RouteAction(ctx, action)
			editTimer.Stop()
			if err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to apply edit to %s: %v", edit.File, err)
				return fmt.Errorf("failed to apply edit to %s: %w", edit.File, err)
			}
			logging.CoderDebug("Successfully applied edit to: %s", edit.File)
		}

		// Inject modified fact
		if c.kernel != nil {
			_ = c.kernel.Assert(core.Fact{
				Predicate: "modified",
				Args:      []interface{}{edit.File},
			})
			logging.CoderDebug("Asserted modified fact for: %s", edit.File)
		}

		applied = append(applied, edit)
	}

	logging.Coder("All %d edits applied successfully", len(edits))
	committed = true
	return nil
}

// estimateLineChangeRatio returns (ratioChanged, totalLines) for old vs new content.
// This is a conservative heuristic to detect accidental whole-file rewrites.
func estimateLineChangeRatio(oldContent, newContent string) (float64, int) {
	if oldContent == "" && newContent == "" {
		return 0, 0
	}
	if oldContent == newContent {
		lines := len(strings.Split(oldContent, "\n"))
		return 0, lines
	}
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	oldLen := len(oldLines)
	newLen := len(newLines)
	total := oldLen
	if newLen > total {
		total = newLen
	}
	if total == 0 {
		return 0, 0
	}
	minLen := oldLen
	if newLen < minLen {
		minLen = newLen
	}
	changed := 0
	for i := 0; i < minLen; i++ {
		if oldLines[i] != newLines[i] {
			changed++
		}
	}
	changed += total - minLen
	return float64(changed) / float64(total), total
}

// waitForRoutingResult blocks until a routing_result for actionID appears.
// Returns the tool output on success.
func (c *CoderShard) waitForRoutingResult(ctx context.Context, actionID string) (string, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if c.kernel == nil {
				return "", fmt.Errorf("kernel not attached")
			}

			results, err := c.kernel.Query("routing_result")
			if err != nil {
				continue
			}

			for _, f := range results {
				if len(f.Args) < 2 {
					continue
				}
				id := fmt.Sprintf("%v", f.Args[0])
				if id != actionID {
					continue
				}
				status := fmt.Sprintf("%v", f.Args[1])
				switch status {
				case "success", "/success":
					if len(f.Args) > 2 {
						return fmt.Sprintf("%v", f.Args[2]), nil
					}
					return "", nil
				case "failure", "/failure":
					reason := ""
					if len(f.Args) > 2 {
						reason = fmt.Sprintf("%v", f.Args[2])
					}
					return "", fmt.Errorf("routing failed for %s: %s", actionID, reason)
				}
			}
		}
	}
}
