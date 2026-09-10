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
	"codenerd/internal/observation"
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

// contentSearch is a content search whose every path decision has already been
// made: the root is resolved and contained, the pattern is compiled, the caps
// are fixed.
//
// It exists because grep and search_code must walk identically. They read the
// same arguments and answer the same question, differing only in how the answer
// is shaped, and a second copy of this walk is how one verb ends up with the
// containment guard and the other without it — which is the precise shape of
// the hole searchBase was added to close.
type contentSearch struct {
	// root is the workspace root used to shorten displayed paths, or "" when it
	// could not be resolved and paths must be shown absolute.
	root string
	// path is the contained search root: a directory to walk or a single file.
	path         string
	pattern      string
	re           *regexp.Regexp
	filePattern  string
	contextLines int
	maxResults   int
	// missingRoot records that the search root does not exist. That is zero
	// matches, not a failure: see the walk below for the live incident.
	missingRoot bool
}

func parseContentSearch(ctx context.Context, args map[string]any, defaultMaxResults int) (contentSearch, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return contentSearch{}, fmt.Errorf("pattern is required")
	}

	rawPath := ""
	if p, ok := args["path"].(string); ok {
		rawPath = p
	}
	path, err := searchBase(ctx, rawPath)
	if err != nil {
		return contentSearch{}, err
	}

	search := contentSearch{path: path, pattern: pattern, maxResults: defaultMaxResults}

	if fp, ok := args["file_pattern"].(string); ok {
		search.filePattern = fp
	}
	if v, ok := argInt(args, "context_lines"); ok {
		search.contextLines = v
	}
	if v, ok := argInt(args, "max_results"); ok && v > 0 {
		search.maxResults = v
	}

	compiled := pattern
	if ic, ok := args["ignore_case"].(bool); ok && ic {
		compiled = "(?i)" + compiled
	}
	search.re, err = regexp.Compile(compiled)
	if err != nil {
		return contentSearch{}, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Display paths are reported relative to the workspace root: containment
	// resolves every search root to an absolute path, and echoing those back
	// would fill the model's context with the same long prefix on every line
	// and teach it to cite files by absolute path. ResolveWorkspaceDir("")
	// yields the symlink-resolved root, which is the form every match path is
	// already in — filepath.Rel against an unresolved root fails wherever the
	// root traverses a link (/tmp on macOS), and the display would silently
	// fall back to absolute.
	if root, rootErr := tools.ResolveWorkspaceDir(ctx, "", ""); rootErr == nil {
		search.root = root
	}

	if _, statErr := os.Stat(path); statErr != nil {
		// F-GREP-1: a search over a path that does not exist yields zero
		// matches, not a hard failure. Returning an error here propagates as a
		// shard/task failure and can cascade to "too many failures -> replan ->
		// pause" (observed live, run 14 phase 2: a reviewer grepped
		// vendor/github.com/smacker/go-tree-sitter, which the module-based
		// build does not vendor). Report no matches so the agent recovers and
		// retargets.
		if os.IsNotExist(statErr) {
			search.missingRoot = true
			return search, nil
		}
		return contentSearch{}, fmt.Errorf("path not found: %w", statErr)
	}
	return search, nil
}

// run walks the contained root and returns the matches, capped.
func (s contentSearch) run() ([]GrepMatch, error) {
	var files []string
	info, err := os.Stat(s.path)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		walkErr := filepath.Walk(s.path, func(p string, info os.FileInfo, err error) error {
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
					if p != s.path {
						return filepath.SkipDir
					}
				} else if name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check file pattern
			if s.filePattern != "" {
				matched, _ := filepath.Match(s.filePattern, info.Name())
				if !matched {
					return nil
				}
			}

			files = append(files, p)
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", walkErr)
		}
	} else {
		files = []string{s.path}
	}

	var matches []GrepMatch
	for _, file := range files {
		if len(matches) >= s.maxResults {
			break
		}

		fileMatches, err := searchFile(file, s.re, s.contextLines, s.maxResults-len(matches))
		if err != nil {
			continue // Skip files with errors
		}

		matches = append(matches, fileMatches...)
	}
	return matches, nil
}

// display shortens an absolute match path for presentation.
func (s contentSearch) display(file string) string {
	if s.root == "" {
		return file
	}
	// ToSlash here is cosmetic — this is display text, not a containment
	// decision. Containment was decided by searchBase before any file was
	// opened.
	if rel, err := filepath.Rel(s.root, file); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return file
}

func executeGrep(ctx context.Context, args map[string]any) (string, error) {
	search, err := parseContentSearch(ctx, args, 50)
	if err != nil {
		return "", err
	}

	logging.ToolsDebug("grep: pattern=%s, path=%s", search.pattern, search.path)

	if search.missingRoot {
		logging.Tools("grep: path does not exist: %s (0 matches)", search.path)
		return fmt.Sprintf("No matches found for pattern: %s (path does not exist: %s)", search.pattern, search.path), nil
	}

	matches, err := search.run()
	if err != nil {
		return "", err
	}

	logging.Tools("grep completed: %s (%d matches)", search.pattern, len(matches))

	if len(matches) == 0 {
		return "No matches found for pattern: " + search.pattern, nil
	}

	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", search.display(m.File), m.LineNumber, m.Line))
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

// SearchCodeTool returns the structural code-search verb.
//
// It runs the same contained walk as grep and shapes the answer differently,
// which is the whole distinction between the two: grep reports the lines that
// matched, search_code reports what those lines MEAN — which symbols the hits
// landed in, which of them is the declaration, and what now depends on what.
//
// The shaping is the point. A wall of matching lines is the most expensive way
// to convey the least structure: it charges the model for text it must then
// re-read to work out which symbols these are, on every turn the transcript
// survives. The lines are not lost — they are retained under the handle in the
// result and readable with search_expand, which never re-runs the search.
func SearchCodeTool() *tools.Tool {
	tool := GrepTool()
	tool.Name = "search_code"
	tool.Description = "Search source files and get back the symbols the matches landed in and the dependency edges between them, not the matching lines. Returns each symbol with its file, extent, hit count and whether the match was its declaration, plus reference and import edges. The raw matching lines are retained under the handle in the result: read them with search_expand, which never re-runs the search. Use grep instead when the literal matching lines are what you need."
	tool.Priority = 86
	tool.Execute = executeSearchCode

	// grep's schema is inherited, minus the argument that would be a lie here:
	// context lines never reach a projection, so advertising the knob would
	// spend the model's tokens on a control that does nothing. The result cap
	// is restated because search_code's is twice grep's.
	delete(tool.Schema.Properties, "context_lines")
	tool.Schema.Properties["max_results"] = tools.Property{
		Type:        "integer",
		Description: "Maximum number of matches to project (default: 100)",
		Default:     100,
	}
	return tool
}

func executeSearchCode(ctx context.Context, args map[string]any) (string, error) {
	// 100 rather than grep's 50: the result is projected to symbols and edges,
	// so a wider search costs the reader roughly what a narrow one does, and a
	// half-seen call graph is the failure mode worth avoiding here.
	search, err := parseContentSearch(ctx, args, 100)
	if err != nil {
		return "", err
	}

	logging.ToolsDebug("search_code: pattern=%s, path=%s", search.pattern, search.path)

	if search.missingRoot {
		logging.Tools("search_code: path does not exist: %s (0 matches)", search.path)
		return fmt.Sprintf("No matches found for pattern: %s (path does not exist: %s)", search.pattern, search.path), nil
	}

	matches, err := search.run()
	if err != nil {
		return "", err
	}

	observed := observation.Search{
		Query:     search.pattern,
		Truncated: len(matches) >= search.maxResults,
	}
	// sources maps each displayed path back to the absolute file the walk
	// actually visited. Projection reads only through this map, so it can only
	// open files the contained walk already opened — a path the caller supplies
	// later can never steer it somewhere else.
	sources := make(map[string]string, len(matches))
	for _, m := range matches {
		shown := search.display(m.File)
		sources[shown] = m.File
		observed.Match = append(observed.Match, observation.Match{
			File: shown,
			Line: m.LineNumber,
			Text: m.Line,
		})
	}

	result := observation.Shared().Encode(observed, func(file string) ([]byte, error) {
		abs, ok := sources[file]
		if !ok {
			return nil, fmt.Errorf("%s was not part of this search", file)
		}
		return os.ReadFile(abs)
	}, observation.Limits{})

	logging.Tools("search_code completed: %s (%d matches, %d symbols, %d edges)",
		search.pattern, result.Matches, len(result.Symbols), len(result.Edges))

	if result.Matches == 0 {
		return "No matches found for pattern: " + search.pattern, nil
	}
	return result.Text(SearchExpandToolName), nil
}

// SearchExpandToolName is named once so that every producer of a handle names
// the same redemption verb. A result that tells the model to call a verb by the
// wrong name publishes a handle nobody can redeem, and the model has no way to
// tell that from an expired one.
const SearchExpandToolName = "search_expand"

// SearchExpandTool returns the verb that redeems a search_code handle.
//
// It reads the retained observation and nothing else. That is not a convenience
// — it is the property that makes the elision safe. Re-running the search to
// expand it would answer from a world that has moved since the reasoning was
// built: an agent that cited a symbol at line 40 and then expanded to find
// different content there has been handed a contradiction it cannot diagnose.
func SearchExpandTool() *tools.Tool {
	return &tools.Tool{
		Name:          SearchExpandToolName,
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "Read the raw matching lines behind a search_code handle. This never re-runs the search, so the lines it returns are exactly the ones the symbols and edges were derived from. Narrow with 'file' to one path, and walk long results with 'offset'. A handle that has expired means running search_code again.",
		Category:      tools.CategoryCode,
		Priority:      84,
		Execute:       executeSearchExpand,
		Schema: tools.ToolSchema{
			Required: []string{"handle"},
			Properties: map[string]tools.Property{
				"handle": {
					Type:        "string",
					Description: "Handle reported by a previous search_code result",
				},
				"file": {
					Type:        "string",
					Description: "Limit the expansion to matches in this file, as the result displayed it",
				},
				"offset": {
					Type:        "integer",
					Description: "Skip this many retained matches (use the offset a previous expansion reported)",
					Default:     0,
				},
				"max_lines": {
					Type:        "integer",
					Description: "Maximum matching lines to return (default 40, hard cap 200)",
					Default:     40,
				},
			},
		},
	}
}

func executeSearchExpand(ctx context.Context, args map[string]any) (string, error) {
	// Arguments are validated before the store is consulted: a missing handle
	// is wrong whether or not anything is retained, and reporting "not found"
	// for it would send the caller off re-running a search over a typo.
	handle, _ := args["handle"].(string)
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return "", fmt.Errorf("handle is required; it is reported by search_code")
	}

	window := observation.Window{}
	if file, ok := args["file"].(string); ok {
		window.File = strings.TrimSpace(file)
	}
	if v, ok := argInt(args, "offset"); ok && v > 0 {
		window.Offset = v
	}
	if v, ok := argInt(args, "max_lines"); ok && v > 0 {
		window.Limit = v
	}

	hydrated, err := observation.Shared().Hydrate(handle, window)
	if err != nil {
		return "", err
	}
	logging.Tools("search_expand: handle=%s returned %d of %d retained match(es)",
		handle, len(hydrated.Match), hydrated.Total)
	return hydrated.Text(), nil
}
