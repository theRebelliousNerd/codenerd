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
		Name: "read_file",
		Description: "Read the contents of a file. Each line is returned prefixed with its " +
			"1-indexed line number and a tab, so you can cite file:line accurately. " +
			"The prefix is NOT part of the file: strip it before passing any content to " +
			"write_file, edit_file, edit_lines or any other tool that matches against the " +
			"real text.",
		Category: tools.CategoryCode,
		Priority: 90,
		Execute:  executeReadFile,
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}

	root, err := workspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	path, err := resolveWorkspacePath(ctx, root, rawPath)
	if err != nil {
		return "", err
	}

	logging.VirtualStoreDebug("read_file: path=%s", path)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	result := string(content)

	// Handle line range if specified.
	// LLM tool args may arrive as float64 via JSON; coerce robustly.
	startLine, hasStart := coerceInt(args["start_line"])
	endLine, hasEnd := coerceInt(args["end_line"])

	if hasStart || hasEnd {
		lines := strings.Split(result, "\n")
		totalLines := len(lines)

		if !hasStart {
			startLine = 1
		}
		if !hasEnd {
			endLine = totalLines
		}

		// Clamp to valid 1-indexed range
		if startLine < 1 {
			startLine = 1
		}
		if startLine > totalLines {
			startLine = totalLines
		}
		if endLine < startLine {
			endLine = startLine
		}
		if endLine > totalLines {
			endLine = totalLines
		}

		// Convert to 0-indexed slice bounds
		result = strings.Join(lines[startLine-1:endLine], "\n")
	} else {
		startLine = 1
	}

	result = numberLines(result, startLine)

	logging.VirtualStore("read_file completed: %s (%d bytes)", path, len(result))
	return result, nil
}

// numberLines prefixes each line with its 1-indexed number and a tab.
//
// Without this the model has to count lines by eye to cite anything, and it
// counts badly. Measured on the architecture docs codeNERD wrote about its own
// projectdoc package: the claims were correct but the citations drifted between
// one and forty-two lines, and one pointed into an unrelated function. This repo
// asks every architectural claim to carry a file:line, so an uncountable read
// tool makes that convention unsatisfiable.
//
// startAt keeps ranged reads honest: slicing lines 200-240 and numbering them
// from 1 would be worse than no numbers at all, because it looks authoritative.
func numberLines(content string, startAt int) string {
	if content == "" {
		return content
	}
	if startAt < 1 {
		startAt = 1
	}

	lines := strings.Split(content, "\n")
	var b strings.Builder
	// Rough preallocation: original text plus a short numeric prefix per line.
	b.Grow(len(content) + len(lines)*8)
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d\t%s", startAt+i, line)
	}
	return b.String()
}

// stripLineNumberPrefixes removes the "N\t" prefix numberLines adds, but only
// when every line has one. Returns ok=false otherwise, so a genuine edit to a
// file of tab-separated numeric data is never silently rewritten.
func stripLineNumberPrefixes(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	lines := strings.Split(s, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		tab := strings.IndexByte(line, '\t')
		if tab <= 0 {
			return s, false
		}
		for _, r := range line[:tab] {
			if r < '0' || r > '9' {
				return s, false
			}
		}
		out[i] = line[tab+1:]
	}
	return strings.Join(out, "\n"), true
}

// coerceInt accepts an int or a JSON-decoded float64 and returns an int.
// LLM tool args arrive via JSON, so integers commonly decode as float64.
func coerceInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	}
	return 0, false
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}

	// Reject non-string content rather than silently writing an empty file.
	rawContent, ok := args["content"]
	if !ok {
		return "", fmt.Errorf("content is required")
	}
	content, ok := rawContent.(string)
	if !ok {
		return "", fmt.Errorf("content must be a string, got %T", rawContent)
	}

	createDirs := true
	if cd, ok := args["create_dirs"].(bool); ok {
		createDirs = cd
	}

	root, err := workspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	path, err := resolveWorkspacePath(ctx, root, rawPath)
	if err != nil {
		return "", err
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}

	oldText, _ := args["old_text"].(string)
	if oldText == "" {
		return "", fmt.Errorf("old_text is required")
	}

	// Reject a non-string (or absent) new_text rather than silently coercing
	// it to "" — that would turn the edit into a deletion of old_text without
	// any error. Mirrors write_file's content type check. An explicit empty
	// string is still a valid deletion and passes.
	rawNewText, ok := args["new_text"]
	if !ok {
		return "", fmt.Errorf("new_text is required")
	}
	newText, ok := rawNewText.(string)
	if !ok {
		return "", fmt.Errorf("new_text must be a string, got %T", rawNewText)
	}

	replaceAll := false
	if ra, ok := args["replace_all"].(bool); ok {
		replaceAll = ra
	}

	root, err := workspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	path, err := resolveWorkspacePath(ctx, root, rawPath)
	if err != nil {
		return "", err
	}

	logging.VirtualStoreDebug("edit_file: path=%s, old_len=%d, new_len=%d", path, len(oldText), len(newText))

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, oldText) {
		// read_file now returns "N\tline" so the model can cite file:line. The
		// tool description says to strip that prefix before editing, but a model
		// that pastes what it read would otherwise get a bare "old_text not
		// found" and burn the turn re-reading. Recover only when EVERY line
		// carries the prefix, so this cannot mangle a genuine edit whose text
		// happens to start with digits.
		if stripped, ok := stripLineNumberPrefixes(oldText); ok && strings.Contains(contentStr, stripped) {
			logging.VirtualStoreWarn("edit_file: old_text carried read_file line-number prefixes; "+
				"stripped them and matched. path=%s", path)
			oldText = stripped
		} else {
			return "", fmt.Errorf("old_text not found in file")
		}
	}

	var newContent string
	var count int
	if replaceAll {
		count = strings.Count(contentStr, oldText)
		newContent = strings.ReplaceAll(contentStr, oldText, newText)
	} else {
		// Refuse an ambiguous edit: replacing only the first of several
		// matches silently can corrupt the wrong site (the caller believes it
		// targeted a specific location). Require disambiguation with more
		// surrounding context, or an explicit replace_all opt-in.
		if n := strings.Count(contentStr, oldText); n > 1 {
			return "", fmt.Errorf("old_text is not unique: found %d occurrences; add surrounding context to make it unique or set replace_all=true", n)
		}
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}

	root, err := workspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	path, err := resolveWorkspacePath(ctx, root, rawPath)
	if err != nil {
		return "", err
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		rawPath = "."
	}

	recursive := false
	if r, ok := args["recursive"].(bool); ok {
		recursive = r
	}

	includeHidden := false
	if ih, ok := args["include_hidden"].(bool); ok {
		includeHidden = ih
	}

	root, err := workspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	path, err := resolveWorkspacePath(ctx, root, rawPath)
	if err != nil {
		return "", err
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

			// Defensive containment: skip anything that resolves outside the
			// workspace root (e.g. a symlink pointing out of tree).
			if _, guardErr := resolveWorkspacePath(ctx, root, p); guardErr != nil {
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
