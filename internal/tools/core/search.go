package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// GlobTool returns a tool for finding files matching a pattern.
func GlobTool() *tools.Tool {
	return &tools.Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern",
		Category:    tools.CategoryCode,
		Priority:    85,
		Execute:     executeGlob,
		Schema: tools.ToolSchema{
			Required: []string{"pattern"},
			Properties: map[string]tools.Property{
				"pattern": {
					Type:        "string",
					Description: "Glob pattern (e.g., '**/*.go', 'src/*.ts')",
				},
				"base_path": {
					Type:        "string",
					Description: "Base directory for search (default: current directory)",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results (default: 100)",
					Default:     100,
				},
			},
		},
	}
}

func executeGlob(ctx context.Context, args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	basePath := "."
	if bp, ok := args["base_path"].(string); ok && bp != "" {
		basePath = bp
	}

	maxResults := 100
	if mr, ok := args["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}

	logging.VirtualStoreDebug("glob: pattern=%s, base=%s", pattern, basePath)

	var matches []string

	// Handle ** patterns (recursive)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := ""
		if len(parts) > 1 {
			suffix = strings.TrimPrefix(parts[1], "/")
		}

		searchPath := basePath
		if prefix != "" {
			searchPath = filepath.Join(basePath, prefix)
		}

		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if len(matches) >= maxResults {
				return filepath.SkipAll
			}

			if info.IsDir() {
				return nil
			}

			// Check suffix match
			if suffix != "" {
				matched, _ := filepath.Match(suffix, info.Name())
				if !matched {
					// Try matching the full relative path suffix
					relPath, _ := filepath.Rel(searchPath, path)
					matched, _ = filepath.Match(suffix, relPath)
				}
				if matched {
					relPath, _ := filepath.Rel(basePath, path)
					matches = append(matches, relPath)
				}
			} else {
				relPath, _ := filepath.Rel(basePath, path)
				matches = append(matches, relPath)
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Simple glob
		fullPattern := filepath.Join(basePath, pattern)
		globMatches, err := filepath.Glob(fullPattern)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern: %w", err)
		}

		for i, m := range globMatches {
			if i >= maxResults {
				break
			}
			relPath, _ := filepath.Rel(basePath, m)
			matches = append(matches, relPath)
		}
	}

	logging.VirtualStore("glob completed: %s (%d matches)", pattern, len(matches))

	if len(matches) == 0 {
		return "No files found matching pattern: " + pattern, nil
	}

	return strings.Join(matches, "\n"), nil
}

// GrepTool returns a tool for searching file contents.
func GrepTool() *tools.Tool {
	return &tools.Tool{
		Name:        "grep",
		Description: "Search for a pattern in file contents",
		Category:    tools.CategoryCode,
		Priority:    85,
		Execute:     executeGrep,
		Schema: tools.ToolSchema{
			Required: []string{"pattern"},
			Properties: map[string]tools.Property{
				"pattern": {
					Type:        "string",
					Description: "Regular expression pattern to search for",
				},
				"path": {
					Type:        "string",
					Description: "File or directory to search (default: current directory)",
				},
				"file_pattern": {
					Type:        "string",
					Description: "Glob pattern for files to search (e.g., '*.go')",
				},
				"context_lines": {
					Type:        "integer",
					Description: "Number of context lines before and after match (default: 0)",
					Default:     0,
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of matches (default: 50)",
					Default:     50,
				},
				"ignore_case": {
					Type:        "boolean",
					Description: "Case insensitive search (default: false)",
					Default:     false,
				},
			},
		},
	}
}

// GrepMatch represents a single grep match.
type GrepMatch struct {
	File       string
	LineNumber int
	Line       string
	Context    []string
}

func executeGrep(ctx context.Context, args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	filePattern := ""
	if fp, ok := args["file_pattern"].(string); ok {
		filePattern = fp
	}

	contextLines := 0
	if cl, ok := args["context_lines"].(int); ok {
		contextLines = cl
	}

	maxResults := 50
	if mr, ok := args["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}

	ignoreCase := false
	if ic, ok := args["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	logging.VirtualStoreDebug("grep: pattern=%s, path=%s", pattern, path)

	// Compile regex
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	var matches []GrepMatch

	// Collect files to search
	var files []string
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				// Skip hidden and common excluded directories
				name := info.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check file pattern
			if filePattern != "" {
				matched, _ := filepath.Match(filePattern, info.Name())
				if !matched {
					return nil
				}
			}

			files = append(files, p)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		files = []string{path}
	}

	// Search each file
	for _, file := range files {
		if len(matches) >= maxResults {
			break
		}

		fileMatches, err := searchFile(file, re, contextLines, maxResults-len(matches))
		if err != nil {
			continue // Skip files with errors
		}

		matches = append(matches, fileMatches...)
	}

	logging.VirtualStore("grep completed: %s (%d matches)", pattern, len(matches))

	if len(matches) == 0 {
		return "No matches found for pattern: " + pattern, nil
	}

	// Format output
	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", m.File, m.LineNumber, m.Line))
		for _, ctx := range m.Context {
			sb.WriteString(fmt.Sprintf("  %s\n", ctx))
		}
	}

	return sb.String(), nil
}

func searchFile(path string, re *regexp.Regexp, contextLines, maxMatches int) ([]GrepMatch, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []GrepMatch
	var lines []string

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)

		if re.MatchString(line) {
			match := GrepMatch{
				File:       path,
				LineNumber: lineNum,
				Line:       strings.TrimSpace(line),
			}

			// Add context lines if requested
			if contextLines > 0 {
				start := len(lines) - contextLines - 1
				if start < 0 {
					start = 0
				}
				for i := start; i < len(lines)-1; i++ {
					match.Context = append(match.Context, fmt.Sprintf("-%d: %s", len(lines)-1-i, strings.TrimSpace(lines[i])))
				}
			}

			matches = append(matches, match)

			if len(matches) >= maxMatches {
				break
			}
		}

		// Keep only enough lines for context
		if contextLines > 0 && len(lines) > contextLines+1 {
			lines = lines[1:]
		}
	}

	return matches, scanner.Err()
}

// SearchCodeTool is an alias for grep with code-focused defaults.
func SearchCodeTool() *tools.Tool {
	tool := GrepTool()
	tool.Name = "search_code"
	tool.Description = "Search for code patterns in source files"
	return tool
}
