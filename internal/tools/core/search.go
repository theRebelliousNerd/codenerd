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

// argInt extracts an integer tool argument, tolerating the numeric types that
// actually arrive at runtime.
//
// The four private copies this used to be one of are now a single shared
// helper; see tools.CoerceInt for why the bare args[key].(int) assertion this
// replaced silently discarded every caller-supplied limit.
func argInt(args map[string]any, key string) (int, bool) {
	return tools.ArgInt(args, key)
}

// searchBase resolves a search root argument to an absolute path inside the
// workspace.
//
// glob and grep took base_path/path straight from the caller and handed it to
// filepath.Walk, so `grep pattern=. path=/etc` read outside the workspace and
// `base_path=../../` walked the parent tree — while the file_ops family next
// door routed every path through the containment guard. The asymmetry is the
// gap: an agent that cannot read /etc/shadow with read_file could still find
// and print its contents with grep.
func searchBase(ctx context.Context, raw string) (string, error) {
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	return tools.ResolveWorkspaceDir(ctx, root, raw)
}

// skipUncontained reports whether a walk entry must not be visited: symlinks
// are never followed, because filepath.Walk reports them via Lstat and opening
// one reads whatever it points at — which is how a link planted inside the
// workspace turns a contained walk into an arbitrary read.
func skipUncontained(info os.FileInfo) bool {
	return info != nil && info.Mode()&os.ModeSymlink != 0
}

// GlobTool returns a tool for finding files matching a pattern.
func GlobTool() *tools.Tool {
	return &tools.Tool{
		Name:          "glob",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryAttack, tools.CategoryGeneral},
		Description:   "Find files matching a glob pattern",
		Category:      tools.CategoryCode,
		Priority:      85,
		Execute:       executeGlob,
		Schema: tools.ToolSchema{
			Required: []string{"pattern"},
			Properties: map[string]tools.Property{
				"pattern": {
					Type:        "string",
					Description: "Glob pattern (e.g., '**/*.go', 'src/*.ts')",
				},
				"base_path": {
					Type:        "string",
					Description: "Base directory for search, relative to the workspace root (default: workspace root)",
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

	rawBase := ""
	if bp, ok := args["base_path"].(string); ok {
		rawBase = bp
	}
	basePath, err := searchBase(ctx, rawBase)
	if err != nil {
		return "", err
	}

	maxResults := 100
	if v, ok := argInt(args, "max_results"); ok && v > 0 {
		maxResults = v
	}

	logging.ToolsDebug("glob: pattern=%s, base=%s", pattern, basePath)

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
			// The prefix comes out of the caller's pattern, so it is just as
			// untrusted as base_path: "../../**/*.pem" put the walk root above
			// the workspace before this check existed.
			resolved, err := tools.ResolveWorkspacePath(ctx, basePath, prefix)
			if err != nil {
				return "", err
			}
			searchPath = resolved
		}

		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if len(matches) >= maxResults {
				return filepath.SkipAll
			}

			if skipUncontained(info) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
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

		for _, m := range globMatches {
			if len(matches) >= maxResults {
				break
			}
			// filepath.Join collapsed any ".." in the pattern before Glob ran,
			// so a match can legitimately sit outside basePath. Drop those
			// instead of failing the whole call: a wildcard that happens to
			// straddle the boundary should return what it may return.
			if _, err := tools.ResolveWorkspacePath(ctx, basePath, m); err != nil {
				continue
			}
			relPath, _ := filepath.Rel(basePath, m)
			matches = append(matches, relPath)
		}
	}

	logging.Tools("glob completed: %s (%d matches)", pattern, len(matches))

	if len(matches) == 0 {
		return "No files found matching pattern: " + pattern, nil
	}

	return strings.Join(matches, "\n"), nil
}

// GrepTool returns a tool for searching file contents.
func GrepTool() *tools.Tool {
	return &tools.Tool{
		Name:          "grep",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryAttack, tools.CategoryGeneral},
		Description:   "Search for a pattern in file contents",
		Category:      tools.CategoryCode,
		Priority:      85,
		Execute:       executeGrep,
		Schema: tools.ToolSchema{
			Required: []string{"pattern"},
			Properties: map[string]tools.Property{
				"pattern": {
					Type:        "string",
					Description: "Regular expression pattern to search for",
				},
				"path": {
					Type:        "string",
					Description: "File or directory to search, relative to the workspace root (default: workspace root)",
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

	rawPath := ""
	if p, ok := args["path"].(string); ok {
		rawPath = p
	}
	path, err := searchBase(ctx, rawPath)
	if err != nil {
		return "", err
	}

	filePattern := ""
	if fp, ok := args["file_pattern"].(string); ok {
		filePattern = fp
	}

	contextLines := 0
	if v, ok := argInt(args, "context_lines"); ok {
		contextLines = v
	}

	maxResults := 50
	if v, ok := argInt(args, "max_results"); ok && v > 0 {
		maxResults = v
	}

	ignoreCase := false
	if ic, ok := args["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	logging.ToolsDebug("grep: pattern=%s, path=%s", pattern, path)

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
		// F-GREP-1: a search over a path that does not exist yields zero matches,
		// not a hard failure. Returning an error here propagates as a shard/task
		// failure and can cascade to "too many failures -> replan -> pause"
		// (observed live, run 14 phase 2: a reviewer grepped
		// vendor/github.com/smacker/go-tree-sitter, which the module-based build
		// does not vendor). Report no matches so the agent recovers and retargets.
		if os.IsNotExist(err) {
			logging.Tools("grep: path does not exist: %s (0 matches)", path)
			return fmt.Sprintf("No matches found for pattern: %s (path does not exist: %s)", pattern, path), nil
		}
		return "", fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if skipUncontained(info) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				// Skip hidden and common excluded directories
				name := info.Name()
				if strings.HasPrefix(name, ".") {
					if p != path {
						return filepath.SkipDir
					}
				} else if name == "node_modules" || name == "vendor" {
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

	logging.Tools("grep completed: %s (%d matches)", pattern, len(matches))

	if len(matches) == 0 {
		return "No matches found for pattern: " + pattern, nil
	}

	// Format output. Paths are reported relative to the workspace root:
	// containment resolves every search root to an absolute path, and echoing
	// those back would fill the model's context with the same long prefix on
	// every line and teach it to cite files by absolute path.
	// ResolveWorkspaceDir("") yields the symlink-resolved root, which is the
	// form every match path is already in — filepath.Rel against an
	// unresolved root fails wherever the root traverses a link (/tmp on
	// macOS), and the display would silently fall back to absolute.
	root, rootErr := tools.ResolveWorkspaceDir(ctx, "", "")
	var sb strings.Builder
	for _, m := range matches {
		display := m.File
		if rootErr == nil {
			// ToSlash here is cosmetic — this is display text, not a
			// containment decision. Containment was decided above, by
			// searchBase, before any file was opened.
			if rel, err := filepath.Rel(root, m.File); err == nil && !strings.HasPrefix(rel, "..") {
				display = filepath.ToSlash(rel)
			}
		}
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", display, m.LineNumber, m.Line))
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
				start := max(len(lines)-contextLines-1, 0)
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
