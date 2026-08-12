package codedom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tactile"
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	// Contain the path to the workspace.
	//
	// These three tools called os.ReadFile/os.WriteFile on the caller-supplied
	// string directly — no root, no symlink resolution, no escape check — while
	// the file_ops family next door routed every path through this same guard.
	// Found by codeNERD's own security review of internal/tools/core/file_ops.go,
	// which named the asymmetry as its highest finding. An absolute path also
	// slips past the constitution's path_traversal_protection rule, which tests
	// for a literal ".." and finds none in "C:\Users\...".
	path, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
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
	content, err := projectdoc.ReadFileForTool(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	ending := tactile.DetectLineEnding(content)
	lines := strings.Split(tactile.NormalizeLineEnding(string(content), "\n"), "\n")

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
	newLines := strings.Split(tactile.NormalizeLineEnding(newContent, "\n"), "\n")

	// Build new content
	var result []string
	result = append(result, lines[:startIdx]...)
	result = append(result, newLines...)
	result = append(result, lines[endIdx:]...)

	// Refuse an edit that drops a delimiter the replaced range was holding.
	// Observed repeatedly: a replacement range ends on a closing brace, the new
	// content omits it, and the file silently stops parsing several
	// declarations later. Failing here costs one retry; not failing costs a
	// corrupted file that looks like a successful write.
	if err := checkDelimiterBalance(path, lines[startIdx:endIdx], newLines); err != nil {
		return "", err
	}

	// Write back
	output := tactile.NormalizeLineEnding(strings.Join(result, "\n"), ending)
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	linesReplaced := endLine - startLine + 1
	logging.VirtualStore("edit_lines completed: %s (replaced %d lines with %d)", path, linesReplaced, len(newLines))
	return fmt.Sprintf("Replaced lines %d-%d (%d lines) with %d new lines in %s.%s",
		startLine, endLine, linesReplaced, len(newLines), path,
		lineShiftNotice(startLine, len(newLines)-linesReplaced, len(result))), nil
}

// checkDelimiterBalance refuses a replacement whose net brace/bracket/paren
// balance differs from the text it replaces.
//
// The failure this prevents: an edit_lines range that ends on a closing brace,
// replaced by content that omits it. The write succeeds, the tool reports
// success, and the file stops compiling somewhere far below the edit -- so the
// error surfaces detached from its cause, often after several more edits have
// been layered on top. That is the worst shape of bug for an unattended run.
//
// Only applied to source files whose delimiters are structural. Comments and
// string literals are skipped so a brace inside them cannot trip the check.
func checkDelimiterBalance(path string, oldLines, newLines []string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".java", ".c", ".h", ".cpp", ".hpp", ".cs", ".rs", ".js", ".jsx", ".ts", ".tsx", ".kt", ".swift", ".scala":
	default:
		return nil // not a brace-structured language; nothing to check
	}

	oldNet := netDelimiters(strings.Join(oldLines, "\n"))
	newNet := netDelimiters(strings.Join(newLines, "\n"))

	var offenders []string
	for _, d := range []struct {
		name  string
		open  rune
		close rune
	}{{"braces", '{', '}'}, {"brackets", '[', ']'}, {"parens", '(', ')'}} {
		if oldNet[d.open] != newNet[d.open] {
			offenders = append(offenders, fmt.Sprintf(
				"%s: replaced text had net %+d, new content has net %+d",
				d.name, oldNet[d.open], newNet[d.open]))
		}
	}
	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf(
		"refusing edit: it changes delimiter balance in %s (%s).\n"+
			"The lines you replaced were holding a delimiter your new content does not reproduce, "+
			"which would leave the file unparseable below the edit.\n"+
			"Re-read the exact range with get_element or read_file, include every closing delimiter "+
			"the range contained, and retry. If the imbalance is intentional (you are deliberately "+
			"moving a block), make the matching edit in the same call or widen the range to cover both ends",
		path, strings.Join(offenders, "; "))
}

// netDelimiters counts opens minus closes per delimiter, ignoring anything
// inside a string, rune, raw literal, or comment.
func netDelimiters(src string) map[rune]int {
	net := map[rune]int{'{': 0, '[': 0, '(': 0}

	var inLine, inBlock, inStr, inRune, inRaw, esc bool
	runes := []rune(src)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			if c == '*' && next == '/' {
				inBlock = false
				i++
			}
		case inStr, inRune:
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if (inStr && c == '"') || (inRune && c == '\'') {
				inStr, inRune = false, false
			}
		case inRaw:
			if c == '`' {
				inRaw = false
			}
		default:
			switch {
			case c == '/' && next == '/':
				inLine = true
				i++
			case c == '/' && next == '*':
				inBlock = true
				i++
			case c == '"':
				inStr = true
			case c == '\'':
				inRune = true
			case c == '`':
				inRaw = true
			case c == '{', c == '[', c == '(':
				net[c]++
			case c == '}':
				net['{']--
			case c == ']':
				net['[']--
			case c == ')':
				net['(']--
			}
		}
	}
	return net
}

// lineShiftNotice reports how a mutation moved every line below it, so the
// caller's next edit does not land at coordinates the previous edit invalidated.
//
// This exists because the tool result is the only feedback an LLM gets between
// two edits to the same file. Without it the model reuses line numbers from an
// earlier get_elements, and a multi-edit session silently duplicates or shreds
// declarations — observed live: two edits to spawner.go produced duplicate
// SetProjectDoc and SpawnSpecialist definitions from stale offsets.
//
// firstShifted is the first line number whose position changed; delta is how
// far every line at or after it moved (negative means the file got shorter).
func lineShiftNotice(firstShifted, delta, newTotal int) string {
	if delta == 0 {
		return fmt.Sprintf(" File is still %d lines; line numbers are unchanged.", newTotal)
	}

	correction := "subtract"
	magnitude := -delta
	if delta > 0 {
		correction = "add"
		magnitude = delta
	}

	return fmt.Sprintf(
		" File is now %d lines (%+d). WARNING: line numbers at or after %d from any earlier"+
			" get_elements/get_element/read_file are now STALE — %s %d to reuse them, or"+
			" re-run get_elements on this file before the next edit.",
		newTotal, delta, firstShifted, correction, magnitude)
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	// Contain the path to the workspace.
	//
	// These three tools called os.ReadFile/os.WriteFile on the caller-supplied
	// string directly — no root, no symlink resolution, no escape check — while
	// the file_ops family next door routed every path through this same guard.
	// Found by codeNERD's own security review of internal/tools/core/file_ops.go,
	// which named the asymmetry as its highest finding. An absolute path also
	// slips past the constitution's path_traversal_protection rule, which tests
	// for a literal ".." and finds none in "C:\Users\...".
	path, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
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
	content, err := projectdoc.ReadFileForTool(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	ending := tactile.DetectLineEnding(content)
	lines := strings.Split(tactile.NormalizeLineEnding(string(content), "\n"), "\n")

	// Validate line number
	if afterLine < 0 || afterLine > len(lines) {
		return "", fmt.Errorf("after_line %d out of range (file has %d lines)", afterLine, len(lines))
	}

	// Split insert content into lines
	newLines := strings.Split(tactile.NormalizeLineEnding(insertContent, "\n"), "\n")

	// Build new content
	var result []string
	result = append(result, lines[:afterLine]...)
	result = append(result, newLines...)
	result = append(result, lines[afterLine:]...)

	// Write back
	output := tactile.NormalizeLineEnding(strings.Join(result, "\n"), ending)
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	logging.VirtualStore("insert_lines completed: %s (inserted %d lines after line %d)", path, len(newLines), afterLine)
	return fmt.Sprintf("Inserted %d lines after line %d in %s.%s",
		len(newLines), afterLine, path,
		lineShiftNotice(afterLine+1, len(newLines), len(result))), nil
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	// Contain the path to the workspace.
	//
	// These three tools called os.ReadFile/os.WriteFile on the caller-supplied
	// string directly — no root, no symlink resolution, no escape check — while
	// the file_ops family next door routed every path through this same guard.
	// Found by codeNERD's own security review of internal/tools/core/file_ops.go,
	// which named the asymmetry as its highest finding. An absolute path also
	// slips past the constitution's path_traversal_protection rule, which tests
	// for a literal ".." and finds none in "C:\Users\...".
	path, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
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
	content, err := projectdoc.ReadFileForTool(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	ending := tactile.DetectLineEnding(content)
	lines := strings.Split(tactile.NormalizeLineEnding(string(content), "\n"), "\n")

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
	output := tactile.NormalizeLineEnding(strings.Join(result, "\n"), ending)
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	linesDeleted := endLine - startLine + 1
	logging.VirtualStore("delete_lines completed: %s (deleted %d lines)", path, linesDeleted)
	return fmt.Sprintf("Deleted lines %d-%d (%d lines) from %s.%s",
		startLine, endLine, linesDeleted, path,
		lineShiftNotice(startLine, -linesDeleted, len(result))), nil
}
