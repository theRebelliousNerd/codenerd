package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"

	"github.com/spf13/cobra"
)

var (
	domReplaceFrom        string
	domReplaceTo          string
	domReplaceRegex       bool
	domReplaceScope       string
	domReplaceRoot        string
	domReplaceIncludeTest bool
	domReplaceGoFmt       bool
	domReplaceGoTest      bool
	domReplaceDryRun      bool
	domReplaceAllowLarge  bool
	domReplaceMaxFiles    int
)

var domReplaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "Search/replace across a 1-hop scope or the workspace",
	Args:  cobra.NoArgs,
	RunE:  runDomReplace,
}

func init() {
	domCmd.AddCommand(domReplaceCmd)
	domReplaceCmd.Flags().StringVar(&domReplaceFrom, "from", "", "Pattern to replace (required)")
	domReplaceCmd.Flags().StringVar(&domReplaceTo, "to", "", "Replacement text")
	domReplaceCmd.Flags().BoolVar(&domReplaceRegex, "regex", false, "Treat --from as a regular expression")
	domReplaceCmd.Flags().StringVar(&domReplaceScope, "scope", "workspace", "Replacement scope: workspace or one-hop")
	domReplaceCmd.Flags().StringVar(&domReplaceRoot, "root", "", "Root file for one-hop scope (required when --scope=one-hop)")
	domReplaceCmd.Flags().BoolVar(&domReplaceIncludeTest, "include-tests", true, "Include *_test.go files (workspace mode; and package-local tests for one-hop)")
	domReplaceCmd.Flags().BoolVar(&domReplaceGoFmt, "gofmt", false, "Run gofmt on modified files")
	domReplaceCmd.Flags().BoolVar(&domReplaceGoTest, "test", false, "Run `go test` in workspace after replacement")
	domReplaceCmd.Flags().BoolVar(&domReplaceDryRun, "dry-run", false, "Only report matches; do not modify files")
	domReplaceCmd.Flags().BoolVar(&domReplaceAllowLarge, "allow-large", false, "Allow modifying more than --max-files files")
	domReplaceCmd.Flags().IntVar(&domReplaceMaxFiles, "max-files", 200, "Safety cap: max files to modify unless --allow-large")
	domReplaceCmd.Flags().IntVar(&domDemoDeepWorker, "deep-workers", 0, "Deep fact workers (0=auto)")
}

func runDomReplace(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(domReplaceFrom) == "" {
		return fmt.Errorf("--from is required")
	}
	if domReplaceScope != "workspace" && domReplaceScope != "one-hop" {
		return fmt.Errorf("--scope must be one of: workspace, one-hop")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
	defer cancel()

	var re *regexp.Regexp
	if domReplaceRegex {
		compiled, err := regexp.Compile(domReplaceFrom)
		if err != nil {
			return fmt.Errorf("compile --from regex: %w", err)
		}
		re = compiled
	}

	ws := workspace
	var absRoot string
	if domReplaceScope == "one-hop" {
		if strings.TrimSpace(domReplaceRoot) == "" {
			return fmt.Errorf("--root is required when --scope=one-hop")
		}
		var err error
		absRoot, err = filepath.Abs(domReplaceRoot)
		if err != nil {
			return fmt.Errorf("resolve --root path: %w", err)
		}
		if ws == "" {
			if root, err := findGoWorkspaceRoot(absRoot); err == nil && root != "" {
				ws = root
			} else {
				ws, _ = os.Getwd()
			}
		}
	} else {
		if ws == "" {
			ws, _ = os.Getwd()
		}
	}
	ws, _ = filepath.Abs(ws)

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	files, err := collectReplaceFiles(ctx, ws, absRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("No candidate files found in scope=%s\n", domReplaceScope)
		return nil
	}

	type hit struct {
		path  string
		count int
	}

	// First pass: count and cap before writing anything.
	hits := make([]hit, 0)
	totalMatches := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		content := string(b)
		var n int
		if domReplaceRegex {
			n = len(re.FindAllStringIndex(content, -1))
		} else {
			n = strings.Count(content, domReplaceFrom)
		}
		if n > 0 {
			hits = append(hits, hit{path: f, count: n})
			totalMatches += n
		}
	}

	if len(hits) == 0 {
		fmt.Printf("Matches: 0\n")
		return nil
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].path < hits[j].path })

	fmt.Printf("Workspace: %s\n", ws)
	fmt.Printf("Scope:     %s\n", domReplaceScope)
	if domReplaceScope == "one-hop" {
		fmt.Printf("Root:      %s\n", absRoot)
	}
	fmt.Printf("Files:     %d matched\n", len(hits))
	fmt.Printf("Matches:   %d\n", totalMatches)

	if domReplaceDryRun {
		for _, h := range hits {
			fmt.Printf("  - %s (%d)\n", h.path, h.count)
		}
		fmt.Printf("dry-run: OK (no files written)\n")
		return nil
	}

	if !domReplaceAllowLarge && len(hits) > domReplaceMaxFiles {
		return fmt.Errorf("refusing to modify %d files (cap=%d). Re-run with --allow-large", len(hits), domReplaceMaxFiles)
	}

	editor := tactile.NewFileEditor()
	editor.SetWorkingDir(ws)

	modifiedFiles := make([]string, 0, len(hits))
	for _, h := range hits {
		b, err := os.ReadFile(h.path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		content := string(b)

		var newContent string
		if domReplaceRegex {
			newContent = re.ReplaceAllString(content, domReplaceTo)
		} else {
			newContent = strings.ReplaceAll(content, domReplaceFrom, domReplaceTo)
		}
		if newContent == content {
			continue
		}

		lines := []string{}
		if newContent != "" {
			lines = strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
		}
		if _, err := editor.WriteFile(h.path, lines); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		modifiedFiles = append(modifiedFiles, h.path)
	}

	if domReplaceGoFmt && len(modifiedFiles) > 0 {
		if err := runGoFmtFiles(ctx, ws, modifiedFiles); err != nil {
			return err
		}
		fmt.Printf("gofmt: OK\n")
	}

	if domReplaceGoTest {
		fmt.Printf("Running: go test -count=1 ./cmd/nerd/... ./internal/... (workspace)\n")
		testCmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./cmd/nerd/...", "./internal/...")
		testCmd.Dir = ws
		out, err := testCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go test failed: %w\n%s", err, string(out))
		}
		fmt.Printf("go test: PASS\n")
	}

	fmt.Printf("Modified:  %d files\n", len(modifiedFiles))
	return nil
}

func collectReplaceFiles(ctx context.Context, ws, absRoot string) ([]string, error) {
	if domReplaceScope == "one-hop" {
		_, vs, scope, err := newDOMHarness(ws, domDemoDeepWorker)
		if err != nil {
			return nil, err
		}

		if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", absRoot}}); err != nil {
			return nil, fmt.Errorf("open_file failed: %w", err)
		}

		files := scope.GetInScopeFiles()

		// Optionally include package-local tests for the root package.
		if domReplaceIncludeTest && absRoot != "" {
			pkgDir := filepath.Dir(absRoot)
			if entries, err := os.ReadDir(pkgDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if !strings.HasSuffix(name, "_test.go") {
						continue
					}
					files = append(files, filepath.Join(pkgDir, name))
				}
			}
		}

		return filterGoFiles(files, domReplaceIncludeTest), nil
	}

	// Workspace mode: walk recursively.
	var files []string
	err := filepath.WalkDir(ws, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			switch base {
			case ".git", ".nerd", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !domReplaceIncludeTest && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func filterGoFiles(files []string, includeTests bool) []string {
	out := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(f, "_test.go") {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
