package codedom

import (
	"context"
	"fmt"
	"os"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// EditLinesTool returns a tool for editing specific lines in a file.
func EditLinesTool() *tools.Tool {
	return &tools.Tool{
		Name:        "edit_lines",
		Description: "Replace specific lines in a file",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeEditLines,
		Schema: tools.ToolSchema{
			Required: []string{"path", "start_line", "end_line", "new_content"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File path to edit",
				},
				"start_line": {
					Type:        "integer",
					Description: "Starting line number (1-indexed)",
				},
				"end_line": {
					Type:        "integer",
					Description: "Ending line number (inclusive)",
				},
				"new_content": {
					Type:        "string",
					Description: "New content to replace the lines with",
				},
			},
		},
	}
}

func executeEditLines(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	startLine, ok := args["start_line"].(int)
	if !ok {
		// Try float64 (JSON numbers)
		if f, ok := args["start_line"].(float64); ok {
			startLine = int(f)
		} else {
			return "", fmt.Errorf("start_line is required")
		}
	}

	endLine, ok := args["end_line"].(int)
	if !ok {
		if f, ok := args["end_line"].(float64); ok {
			endLine = int(f)
		} else {
			return "", fmt.Errorf("end_line is required")
		}
	}

	newContent, _ := args["new_content"].(string)

	logging.VirtualStoreDebug("edit_lines: path=%s, start=%d, end=%d", path, startLine, endLine)

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line numbers
	if startLine < 1 || startLine > len(lines) {
		return "", fmt.Errorf("start_line %d out of range (file has %d lines)", startLine, len(lines))
	}
	if endLine < startLine || endLine > len(lines) {
		return "", fmt.Errorf("end_line %d out of range", endLine)
	}

	// Convert to 0-indexed
	startIdx := startLine - 1
	endIdx := endLine

	// Split new content into lines
	newLines := strings.Split(newContent, "\n")

	// Build new content
	var result []string
	result = append(result, lines[:startIdx]...)
	result = append(result, newLines...)
	result = append(result, lines[endIdx:]...)

	// Write back
	if err := os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	linesReplaced := endLine - startLine + 1
	logging.VirtualStore("edit_lines completed: %s (replaced %d lines with %d)", path, linesReplaced, len(newLines))
	return fmt.Sprintf("Replaced lines %d-%d with %d new lines in %s", startLine, endLine, len(newLines), path), nil
}

// InsertLinesTool returns a tool for inserting lines at a position.
func InsertLinesTool() *tools.Tool {
	return &tools.Tool{
		Name:        "insert_lines",
		Description: "Insert lines at a specific position in a file",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeInsertLines,
		Schema: tools.ToolSchema{
			Required: []string{"path", "after_line", "content"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File path to edit",
				},
				"after_line": {
					Type:        "integer",
					Description: "Insert after this line number (0 to insert at beginning)",
				},
				"content": {
					Type:        "string",
					Description: "Content to insert",
				},
			},
		},
	}
}

func executeInsertLines(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	afterLine := 0
	if al, ok := args["after_line"].(int); ok {
		afterLine = al
	} else if f, ok := args["after_line"].(float64); ok {
		afterLine = int(f)
	}

	insertContent, _ := args["content"].(string)
	if insertContent == "" {
		return "", fmt.Errorf("content is required")
	}

	logging.VirtualStoreDebug("insert_lines: path=%s, after=%d", path, afterLine)

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line number
	if afterLine < 0 || afterLine > len(lines) {
		return "", fmt.Errorf("after_line %d out of range (file has %d lines)", afterLine, len(lines))
	}

	// Split insert content into lines
	newLines := strings.Split(insertContent, "\n")

	// Build new content
	var result []string
	result = append(result, lines[:afterLine]...)
	result = append(result, newLines...)
	result = append(result, lines[afterLine:]...)

	// Write back
	if err := os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	logging.VirtualStore("insert_lines completed: %s (inserted %d lines after line %d)", path, len(newLines), afterLine)
	return fmt.Sprintf("Inserted %d lines after line %d in %s", len(newLines), afterLine, path), nil
}

// DeleteLinesTool returns a tool for deleting lines from a file.
func DeleteLinesTool() *tools.Tool {
	return &tools.Tool{
		Name:        "delete_lines",
		Description: "Delete a range of lines from a file",
		Category:    tools.CategoryCode,
		Priority:    75,
		Execute:     executeDeleteLines,
		Schema: tools.ToolSchema{
			Required: []string{"path", "start_line", "end_line"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File path to edit",
				},
				"start_line": {
					Type:        "integer",
					Description: "Starting line number (1-indexed)",
				},
				"end_line": {
					Type:        "integer",
					Description: "Ending line number (inclusive)",
				},
			},
		},
	}
}

func executeDeleteLines(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	startLine, ok := args["start_line"].(int)
	if !ok {
		if f, ok := args["start_line"].(float64); ok {
			startLine = int(f)
		} else {
			return "", fmt.Errorf("start_line is required")
		}
	}

	endLine, ok := args["end_line"].(int)
	if !ok {
		if f, ok := args["end_line"].(float64); ok {
			endLine = int(f)
		} else {
			return "", fmt.Errorf("end_line is required")
		}
	}

	logging.VirtualStoreDebug("delete_lines: path=%s, start=%d, end=%d", path, startLine, endLine)

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line numbers
	if startLine < 1 || startLine > len(lines) {
		return "", fmt.Errorf("start_line %d out of range (file has %d lines)", startLine, len(lines))
	}
	if endLine < startLine || endLine > len(lines) {
		return "", fmt.Errorf("end_line %d out of range", endLine)
	}

	// Convert to 0-indexed
	startIdx := startLine - 1
	endIdx := endLine

	// Build new content (skip the deleted lines)
	var result []string
	result = append(result, lines[:startIdx]...)
	result = append(result, lines[endIdx:]...)

	// Write back
	if err := os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	linesDeleted := endLine - startLine + 1
	logging.VirtualStore("delete_lines completed: %s (deleted %d lines)", path, linesDeleted)
	return fmt.Sprintf("Deleted lines %d-%d from %s", startLine, endLine, path), nil
}
