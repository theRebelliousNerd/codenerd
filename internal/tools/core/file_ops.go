package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// ReadFileTool returns a tool for reading file contents.
func ReadFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Category:    tools.CategoryCode,
		Priority:    90,
		Execute:     executeReadFile,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "The file path to read",
				},
				"start_line": {
					Type:        "integer",
					Description: "Starting line number (1-indexed, optional)",
				},
				"end_line": {
					Type:        "integer",
					Description: "Ending line number (inclusive, optional)",
				},
			},
		},
	}
}

func executeReadFile(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	logging.VirtualStoreDebug("read_file: path=%s", path)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	result := string(content)

	// Handle line range if specified
	startLine, hasStart := args["start_line"].(int)
	endLine, hasEnd := args["end_line"].(int)

	if hasStart || hasEnd {
		lines := strings.Split(result, "\n")

		if !hasStart {
			startLine = 1
		}
		if !hasEnd {
			endLine = len(lines)
		}

		// Convert to 0-indexed
		startLine--
		if startLine < 0 {
			startLine = 0
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}

		result = strings.Join(lines[startLine:endLine], "\n")
	}

	logging.VirtualStore("read_file completed: %s (%d bytes)", path, len(result))
	return result, nil
}

// WriteFileTool returns a tool for writing content to a file.
func WriteFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "write_file",
		Description: "Write content to a file, creating it if it doesn't exist",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeWriteFile,
		Schema: tools.ToolSchema{
			Required: []string{"path", "content"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "The file path to write",
				},
				"content": {
					Type:        "string",
					Description: "The content to write",
				},
				"create_dirs": {
					Type:        "boolean",
					Description: "Create parent directories if they don't exist (default: true)",
					Default:     true,
				},
			},
		},
	}
}

func executeWriteFile(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, _ := args["content"].(string)

	createDirs := true
	if cd, ok := args["create_dirs"].(bool); ok {
		createDirs = cd
	}

	logging.VirtualStoreDebug("write_file: path=%s, size=%d", path, len(content))

	// Create parent directories if needed
	if createDirs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	logging.VirtualStore("write_file completed: %s (%d bytes)", path, len(content))
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
}

// EditFileTool returns a tool for editing files with search/replace.
func EditFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "edit_file",
		Description: "Edit a file by replacing text",
		Category:    tools.CategoryCode,
		Priority:    85,
		Execute:     executeEditFile,
		Schema: tools.ToolSchema{
			Required: []string{"path", "old_text", "new_text"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "The file path to edit",
				},
				"old_text": {
					Type:        "string",
					Description: "The text to find and replace",
				},
				"new_text": {
					Type:        "string",
					Description: "The replacement text",
				},
				"replace_all": {
					Type:        "boolean",
					Description: "Replace all occurrences (default: false, replaces first only)",
					Default:     false,
				},
			},
		},
	}
}

func executeEditFile(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	oldText, _ := args["old_text"].(string)
	if oldText == "" {
		return "", fmt.Errorf("old_text is required")
	}

	newText, _ := args["new_text"].(string)

	replaceAll := false
	if ra, ok := args["replace_all"].(bool); ok {
		replaceAll = ra
	}

	logging.VirtualStoreDebug("edit_file: path=%s, old_len=%d, new_len=%d", path, len(oldText), len(newText))

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, oldText) {
		return "", fmt.Errorf("old_text not found in file")
	}

	var newContent string
	var count int
	if replaceAll {
		count = strings.Count(contentStr, oldText)
		newContent = strings.ReplaceAll(contentStr, oldText, newText)
	} else {
		count = 1
		newContent = strings.Replace(contentStr, oldText, newText, 1)
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	logging.VirtualStore("edit_file completed: %s (%d replacements)", path, count)
	return fmt.Sprintf("Replaced %d occurrence(s) in %s", count, path), nil
}

// DeleteFileTool returns a tool for deleting files.
func DeleteFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "delete_file",
		Description: "Delete a file (requires explicit permission)",
		Category:    tools.CategoryCode,
		Priority:    50,
		Execute:     executeDeleteFile,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "The file path to delete",
				},
			},
		},
	}
}

func executeDeleteFile(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	logging.VirtualStoreDebug("delete_file: path=%s", path)

	// Safety check - don't delete directories
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("cannot delete directory with delete_file, use dedicated command")
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("failed to delete file: %w", err)
	}

	logging.VirtualStore("delete_file completed: %s", path)
	return fmt.Sprintf("Deleted %s", path), nil
}

// ListFilesTool returns a tool for listing directory contents.
func ListFilesTool() *tools.Tool {
	return &tools.Tool{
		Name:        "list_files",
		Description: "List files in a directory",
		Category:    tools.CategoryCode,
		Priority:    85,
		Execute:     executeListFiles,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "The directory path to list",
				},
				"recursive": {
					Type:        "boolean",
					Description: "List recursively (default: false)",
					Default:     false,
				},
				"include_hidden": {
					Type:        "boolean",
					Description: "Include hidden files (default: false)",
					Default:     false,
				},
			},
		},
	}
}

func executeListFiles(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	recursive := false
	if r, ok := args["recursive"].(bool); ok {
		recursive = r
	}

	includeHidden := false
	if ih, ok := args["include_hidden"].(bool); ok {
		includeHidden = ih
	}

	logging.VirtualStoreDebug("list_files: path=%s, recursive=%v", path, recursive)

	var files []string

	if recursive {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			name := info.Name()
			if !includeHidden && strings.HasPrefix(name, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			relPath, _ := filepath.Rel(path, p)
			if relPath == "." {
				return nil
			}

			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if !includeHidden && strings.HasPrefix(name, ".") {
				continue
			}

			if entry.IsDir() {
				files = append(files, name+"/")
			} else {
				files = append(files, name)
			}
		}
	}

	logging.VirtualStore("list_files completed: %s (%d entries)", path, len(files))
	return strings.Join(files, "\n"), nil
}
