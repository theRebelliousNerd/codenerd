package coder

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// =============================================================================
// EDIT APPLICATION
// =============================================================================

// applyEdits writes the code changes via VirtualStore.
func (c *CoderShard) applyEdits(ctx context.Context, edits []CodeEdit) error {
	logging.Coder("Applying %d edits", len(edits))

	for i, edit := range edits {
		logging.CoderDebug("Edit[%d]: type=%s, file=%s, content_len=%d",
			i, edit.Type, edit.File, len(edit.NewContent))

		c.mu.Lock()
		c.editHistory = append(c.editHistory, edit)
		c.mu.Unlock()

		if c.virtualStore == nil {
			// No virtual store, write directly
			logging.CoderDebug("No VirtualStore, writing directly to filesystem")
			fullPath := c.resolvePath(edit.File)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to create directory for %s: %v", fullPath, err)
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(edit.NewContent), 0644); err != nil {
				logging.Get(logging.CategoryCoder).Error("Failed to write file %s: %v", fullPath, err)
				return fmt.Errorf("failed to write file: %w", err)
			}
			logging.CoderDebug("Direct write successful: %s", fullPath)
			continue
		}

		// Use VirtualStore for proper action routing
		var action core.Fact
		switch edit.Type {
		case "create":
			logging.CoderDebug("Creating new file: %s", edit.File)
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "modify", "refactor", "fix":
			logging.CoderDebug("Modifying file (%s): %s", edit.Type, edit.File)
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "delete":
			logging.CoderDebug("Deleting file: %s", edit.File)
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/delete_file",
					edit.File,
					map[string]interface{}{"confirmed": true},
				},
			}
		default:
			logging.CoderDebug("Default action (write) for file: %s", edit.File)
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		}

		editTimer := logging.StartTimer(logging.CategoryCoder, fmt.Sprintf("ApplyEdit:%s", edit.File))
		_, err := c.virtualStore.RouteAction(ctx, action)
		editTimer.Stop()
		if err != nil {
			logging.Get(logging.CategoryCoder).Error("Failed to apply edit to %s: %v", edit.File, err)
			return fmt.Errorf("failed to apply edit to %s: %w", edit.File, err)
		}
		logging.CoderDebug("Successfully applied edit to: %s", edit.File)

		// Inject modified fact
		if c.kernel != nil {
			_ = c.kernel.Assert(core.Fact{
				Predicate: "modified",
				Args:      []interface{}{edit.File},
			})
			logging.CoderDebug("Asserted modified fact for: %s", edit.File)
		}
	}

	logging.Coder("All %d edits applied successfully", len(edits))
	return nil
}
