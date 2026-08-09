package specs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	markdownLinkPattern  = regexp.MustCompile(`\[[^\]]+\]\(([^)#?]+\.md)(?:#[^)]+)?\)`)
	errCatalogLimit      = errors.New("browser spec catalog file limit reached")
	errCatalogEntryLimit = errors.New("browser spec catalog traversal limit reached")
)

// Catalog loads configured Markdown sources without escaping the workspace.
type Catalog struct {
	workspace     string
	realWorkspace string
	config        Config
}

// NewCatalog creates a read-only, workspace-confined catalog.
func NewCatalog(workspace string, config Config) (*Catalog, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("browser spec workspace is required")
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve browser spec workspace: %w", err)
	}
	realWorkspace, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve browser spec workspace symlinks: %w", err)
	}
	normalized := cloneConfig(config.Normalize())
	return &Catalog{workspace: filepath.Clean(absolute), realWorkspace: filepath.Clean(realWorkspace), config: normalized}, nil
}

// Config returns the normalized catalog configuration.
func (c *Catalog) Config() Config {
	if c == nil {
		return Config{}
	}
	return cloneConfig(c.config)
}

func cloneConfig(config Config) Config {
	if config.Enabled != nil {
		enabled := *config.Enabled
		config.Enabled = &enabled
	}
	config.Sources = append([]Source(nil), config.Sources...)
	for index := range config.Sources {
		config.Sources[index].Roots = append([]string(nil), config.Sources[index].Roots...)
		config.Sources[index].Indexes = append([]string(nil), config.Sources[index].Indexes...)
		config.Sources[index].Include = append([]string(nil), config.Sources[index].Include...)
		config.Sources[index].Exclude = append([]string(nil), config.Sources[index].Exclude...)
	}
	return config
}

// Load scans configured indexes first and roots second under hard bounds.
func (c *Catalog) Load(ctx context.Context) (LoadResult, error) {
	if c == nil || !c.config.IsEnabled() {
		return LoadResult{}, fmt.Errorf("browser specs are disabled")
	}
	result := LoadResult{}
	candidates := make([]specCandidate, 0, c.config.MaxFiles)
	seen := make(map[string]struct{})
	maxEntries := c.config.MaxFiles * 20
	if maxEntries > hardMaxCatalogEntries {
		maxEntries = hardMaxCatalogEntries
	}
	for _, source := range c.config.Sources {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		source.Name = strings.TrimSpace(source.Name)
		if source.Name == "" {
			addCatalogWarning(&result, "ignored browser spec source without a name")
			continue
		}
		truncated, warnings := c.collectSource(ctx, source, &candidates, seen, c.config.MaxFiles, &result.EntriesScanned, maxEntries)
		for _, warning := range warnings {
			addCatalogWarning(&result, warning)
		}
		result.Truncated = result.Truncated || truncated
		if len(candidates) >= c.config.MaxFiles {
			result.Truncated = true
			break
		}
	}

	result.Specs = make([]Spec, 0, len(candidates))
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.FilesScanned++
		info, err := os.Stat(candidate.path)
		if err != nil {
			addCatalogWarning(&result, fmt.Sprintf("inspect browser spec %s: %v", candidate.relative, err))
			continue
		}
		if info.IsDir() || info.Size() > c.config.MaxFileBytes {
			if info.Size() > c.config.MaxFileBytes {
				addCatalogWarning(&result, fmt.Sprintf("browser spec %s exceeds max_file_bytes", candidate.relative))
			}
			continue
		}
		if result.BytesLoaded+info.Size() > hardMaxCatalogBytes {
			result.Truncated = true
			addCatalogWarning(&result, fmt.Sprintf("browser spec catalog reached hard byte limit (%d)", hardMaxCatalogBytes))
			break
		}
		content, err := os.ReadFile(candidate.path)
		if err != nil {
			addCatalogWarning(&result, fmt.Sprintf("read browser spec %s: %v", candidate.relative, err))
			continue
		}
		result.BytesLoaded += int64(len(content))
		doc, err := Parse(candidate.relative, content)
		if err != nil {
			addCatalogWarning(&result, err.Error())
			continue
		}
		doc.Corpus = candidate.corpus
		result.Specs = append(result.Specs, doc)
	}
	return result, nil
}

type specCandidate struct {
	path     string
	relative string
	corpus   string
}

func (c *Catalog) collectSource(ctx context.Context, source Source, candidates *[]specCandidate, seen map[string]struct{}, limit int, entriesScanned *int, maxEntries int) (bool, []string) {
	warnings := make([]string, 0)
	truncated := false
	appendCandidate := func(path, patternRelative string) {
		if len(*candidates) >= limit {
			truncated = true
			return
		}
		resolved, relative, err := c.resolveReadPath(path)
		if err != nil {
			warnings = appendBoundedWarning(warnings, err.Error())
			return
		}
		if !strings.EqualFold(filepath.Ext(resolved), ".md") || !sourceAllows(source, relative, patternRelative) {
			return
		}
		key := canonicalKey(resolved)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		*candidates = append(*candidates, specCandidate{path: resolved, relative: relative, corpus: source.Name})
	}

	for _, configuredIndex := range source.Indexes {
		if err := ctx.Err(); err != nil {
			return truncated, appendBoundedWarning(warnings, err.Error())
		}
		indexPath, indexRelative, err := c.resolveReadPath(configuredIndex)
		if err != nil {
			warnings = appendBoundedWarning(warnings, err.Error())
			continue
		}
		info, err := os.Stat(indexPath)
		if err != nil || info.IsDir() || info.Size() > c.config.MaxFileBytes {
			warnings = appendBoundedWarning(warnings, fmt.Sprintf("invalid browser spec index %s", indexRelative))
			continue
		}
		content, err := os.ReadFile(indexPath)
		if err != nil {
			warnings = appendBoundedWarning(warnings, fmt.Sprintf("read browser spec index %s: %v", indexRelative, err))
			continue
		}
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
			(*entriesScanned)++
			if *entriesScanned > maxEntries {
				truncated = true
				warnings = appendBoundedWarning(warnings, errCatalogEntryLimit.Error())
				break
			}
			if len(match) > 1 {
				appendCandidate(filepath.Join(filepath.Dir(indexPath), filepath.FromSlash(match[1])), filepath.FromSlash(match[1]))
			}
			if len(*candidates) >= limit {
				truncated = true
				break
			}
		}
	}

	for _, configuredRoot := range source.Roots {
		if len(*candidates) >= limit {
			return true, warnings
		}
		root, rootRelative, err := c.resolveReadPath(configuredRoot)
		if err != nil {
			warnings = appendBoundedWarning(warnings, err.Error())
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			warnings = appendBoundedWarning(warnings, fmt.Sprintf("browser spec root is not a directory: %s", rootRelative))
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				warnings = appendBoundedWarning(warnings, fmt.Sprintf("scan browser spec root %s: %v", rootRelative, walkErr))
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			(*entriesScanned)++
			if *entriesScanned > maxEntries {
				return errCatalogEntryLimit
			}
			relativeToRoot, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			if entry.IsDir() {
				if path != root && sourceExcluded(source, rootRelative, relativeToRoot) {
					return filepath.SkipDir
				}
				return nil
			}
			appendCandidate(path, relativeToRoot)
			if len(*candidates) >= limit {
				return errCatalogLimit
			}
			return nil
		})
		if errors.Is(walkErr, errCatalogLimit) || errors.Is(walkErr, errCatalogEntryLimit) {
			truncated = true
			if errors.Is(walkErr, errCatalogEntryLimit) {
				warnings = appendBoundedWarning(warnings, errCatalogEntryLimit.Error())
			}
			break
		}
		if walkErr != nil {
			if ctx.Err() != nil {
				return truncated, appendBoundedWarning(warnings, ctx.Err().Error())
			}
			warnings = appendBoundedWarning(warnings, fmt.Sprintf("scan browser spec root %s: %v", rootRelative, walkErr))
		}
	}
	return truncated, warnings
}

func (c *Catalog) resolveReadPath(configured string) (string, string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "", "", fmt.Errorf("empty browser spec path")
	}
	path := configured
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.workspace, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve browser spec path %q: %w", configured, err)
	}
	absolute = filepath.Clean(absolute)
	if !pathWithin(c.workspace, absolute) {
		return "", "", fmt.Errorf("browser spec path %q is outside workspace", configured)
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve browser spec path %q: %w", configured, err)
	}
	realPath = filepath.Clean(realPath)
	if !pathWithin(c.realWorkspace, realPath) {
		return "", "", fmt.Errorf("browser spec path %q escapes workspace through a symlink", configured)
	}
	relative, err := filepath.Rel(c.realWorkspace, realPath)
	if err != nil {
		return "", "", fmt.Errorf("relativize browser spec path %q: %w", configured, err)
	}
	return realPath, filepath.ToSlash(relative), nil
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func canonicalKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func sourceAllows(source Source, workspaceRelative, rootRelative string) bool {
	if sourceExcluded(source, workspaceRelative, rootRelative) {
		return false
	}
	if len(source.Include) == 0 {
		return true
	}
	for _, pattern := range source.Include {
		if globMatch(pattern, workspaceRelative) || globMatch(pattern, rootRelative) {
			return true
		}
	}
	return false
}

func sourceExcluded(source Source, values ...string) bool {
	for _, pattern := range source.Exclude {
		for _, value := range values {
			if globMatch(pattern, value) {
				return true
			}
		}
	}
	return false
}

func globMatch(pattern, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(filepath.Clean(value))
	if pattern == "" {
		return false
	}
	var expression strings.Builder
	expression.WriteByte('^')
	for index := 0; index < len(pattern); index++ {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index++
				if index+1 < len(pattern) && pattern[index+1] == '/' {
					index++
					expression.WriteString("(?:.*/)?")
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[index])))
		}
	}
	expression.WriteByte('$')
	matched, err := regexp.MatchString(expression.String(), value)
	return err == nil && matched
}

func addCatalogWarning(result *LoadResult, warning string) {
	result.Warnings = appendBoundedWarning(result.Warnings, warning)
}

func appendBoundedWarning(warnings []string, warning string) []string {
	if strings.TrimSpace(warning) != "" && len(warnings) < hardMaxCatalogWarnings {
		warnings = append(warnings, warning)
	}
	return warnings
}

// MatchSpecs ranks documents by exact bindings and relevant terms.
func MatchSpecs(specs []Spec, input MatchInput, maxExcerptBytes int) []Match {
	input.Max = boundedInt(input.Max, defaultMaxResults, hardMaxResults)
	maxExcerptBytes = boundedInt(maxExcerptBytes, defaultMaxExcerptBytes, hardMaxExcerptBytes)
	terms := normalizedTerms(input)
	type rankedSpec struct {
		doc   Spec
		score int
	}
	ranked := make([]rankedSpec, 0, len(specs))
	hasFilter := matchInputHasFilter(input)
	for _, doc := range specs {
		score := specScore(doc, input, terms)
		if hasFilter && score <= 0 {
			continue
		}
		ranked = append(ranked, rankedSpec{doc: doc, score: score})
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		return ranked[left].doc.Path < ranked[right].doc.Path
	})
	if len(ranked) > input.Max {
		ranked = ranked[:input.Max]
	}
	result := make([]Match, 0, len(ranked))
	for _, item := range ranked {
		doc := item.doc
		result = append(result, Match{
			Name: doc.Name, Title: doc.Title, Path: doc.Path, Corpus: doc.Corpus,
			Summary: truncateText(doc.Summary, maxExcerptBytes/2), ReadWhen: doc.ReadWhen,
			DocType: doc.DocType, Subsystem: doc.Subsystem, Bindings: doc.Bindings,
			InvariantCount: len(doc.Invariants), Score: item.score,
			Excerpt: truncateText(relevantExcerpt(doc.Body, terms), maxExcerptBytes),
		})
	}
	return result
}

// CountMatchingSpecs returns the exact in-memory match count before result caps.
func CountMatchingSpecs(specs []Spec, input MatchInput) int {
	terms := normalizedTerms(input)
	hasFilter := matchInputHasFilter(input)
	count := 0
	for _, doc := range specs {
		if score := specScore(doc, input, terms); !hasFilter || score > 0 {
			count++
		}
	}
	return count
}

// SelectInvariants returns bounded invariants matching file/binding/term scope.
func SelectInvariants(specs []Spec, input MatchInput, max int) ([]SelectedInvariant, bool) {
	max = boundedInt(max, 100, 100)
	result := make([]SelectedInvariant, 0)
	truncated := false
	terms := normalizedTerms(input)
	for _, doc := range specs {
		if input.Corpus != "" && !strings.EqualFold(doc.Corpus, input.Corpus) {
			continue
		}
		if invariantDocumentFilter(input) && specScore(doc, input, terms) <= 0 {
			continue
		}
		for _, invariant := range doc.Invariants {
			if input.File != "" {
				if input.From > 0 && input.To > 0 {
					if !invariant.Covers(input.File, input.From, input.To) {
						continue
					}
				} else if !invariant.InFile(input.File) {
					continue
				}
			}
			if len(result) >= max {
				truncated = true
				return result, truncated
			}
			result = append(result, SelectedInvariant{
				Spec: doc.Name, Path: doc.Path, Corpus: doc.Corpus,
				Bindings: append([]Binding(nil), doc.Bindings...), Invariant: invariant,
			})
		}
	}
	return result, truncated
}

func matchInputHasFilter(input MatchInput) bool {
	return input.Corpus != "" || input.File != "" || input.Component != "" || input.Route != "" || input.Selector != "" || len(input.Terms) > 0
}

func invariantDocumentFilter(input MatchInput) bool {
	return input.Corpus != "" || input.Component != "" || input.Route != "" || input.Selector != "" || len(input.Terms) > 0
}

func normalizedTerms(input MatchInput) []string {
	values := append([]string(nil), input.Terms...)
	values = append(values, input.File, input.Component, input.Route, input.Selector)
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, value := range values {
		for _, term := range strings.FieldsFunc(strings.ToLower(value), func(char rune) bool {
			return (char < 'a' || char > 'z') && (char < '0' || char > '9')
		}) {
			if len(term) < 3 {
				continue
			}
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			result = append(result, term)
		}
	}
	return result
}

func specScore(doc Spec, input MatchInput, terms []string) int {
	if input.Corpus != "" && !strings.EqualFold(doc.Corpus, input.Corpus) {
		return 0
	}
	score := 0
	for _, binding := range doc.Bindings {
		switch binding.Kind {
		case "component":
			if input.Component != "" && strings.EqualFold(binding.Target, input.Component) {
				score += 100
			}
		case "route":
			if input.Route != "" && routeMatches(binding.Target, input.Route) {
				score += 100
			}
		case "selector":
			if input.Selector != "" && binding.Target == input.Selector {
				score += 100
			}
		}
	}
	if input.File != "" && (sameFile(doc.Source, input.File) || sameFile(doc.Path, input.File)) {
		score += 100
	}
	haystack := strings.ToLower(strings.Join([]string{
		doc.Name, doc.Title, doc.Summary, doc.ReadWhen, doc.DocType, doc.Subsystem,
		strings.Join(doc.Tags, " "), doc.Path, doc.Body,
	}, " "))
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			score += 5
		}
	}
	if input.Corpus != "" && strings.EqualFold(doc.Corpus, input.Corpus) {
		score++
	}
	return score
}

func routeMatches(binding, route string) bool {
	binding = strings.TrimSuffix(strings.TrimSpace(binding), "/")
	route = strings.TrimSuffix(strings.TrimSpace(route), "/")
	return binding == route || binding != "" && strings.HasPrefix(route, binding+"/")
}

func relevantExcerpt(body string, terms []string) string {
	body = strings.TrimSpace(body)
	if body == "" || len(terms) == 0 {
		return body
	}
	lower := strings.ToLower(body)
	best := -1
	for _, term := range terms {
		if index := strings.Index(lower, term); index >= 0 && (best < 0 || index < best) {
			best = index
		}
	}
	if best <= 0 {
		return body
	}
	start := best - 240
	if start < 0 {
		start = 0
	}
	for start < len(body) && !utf8.RuneStart(body[start]) {
		start++
	}
	return body[start:]
}

func truncateText(value string, maxBytes int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	if maxBytes <= 3 {
		end := maxBytes
		for end > 0 && !utf8.RuneStart(value[end]) {
			end--
		}
		return value[:end]
	}
	end := maxBytes - 3
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "..."
}
