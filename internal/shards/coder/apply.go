package coder

import (
	"codenerd/internal/core"
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
	for _, edit := range edits {
		c.mu.Lock()
		c.editHistory = append(c.editHistory, edit)
		c.mu.Unlock()

		if c.virtualStore == nil {
			// No virtual store, write directly
			fullPath := c.resolvePath(edit.File)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(edit.NewContent), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			continue
		}

		// Use VirtualStore for proper action routing
		var action core.Fact
		switch edit.Type {
		case "create":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "modify", "refactor", "fix":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "delete":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/delete_file",
					edit.File,
					map[string]interface{}{"confirmed": true},
				},
			}
		default:
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		}

		_, err := c.virtualStore.RouteAction(ctx, action)
		if err != nil {
			return fmt.Errorf("failed to apply edit to %s: %w", edit.File, err)
		}

		// Inject modified fact
		if c.kernel != nil {
			_ = c.kernel.Assert(core.Fact{
				Predicate: "modified",
				Args:      []interface{}{edit.File},
			})
		}
	}

	return nil
}
