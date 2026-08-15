package retrieval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/logging"
)

// =============================================================================
// TIERED CONTEXT BUILDER
// =============================================================================

// TieredContextBuilder progressively builds context through 4 tiers.
// This enables efficient context collection for large repositories.
//
// Tier 1 (30%): Explicitly mentioned files from issue text
// Tier 2 (40%): Files matching extracted keywords
// Tier 3 (20%): Import neighbors of Tier 1-2 files
// Tier 4 (10%): Semantic expansion (vector similarity - requires embedding)
type TieredContextBuilder struct {
	retriever *SparseRetriever
	workDir   string

	// mu guards findCache. findFile walks the entire workspace for every
	// unresolved mention, and Tier 1 resolves each mention while Tier 3/4 may
	// ask for the same names again; without the memo a five-mention issue paid
	// five full tree walks.
	mu        sync.RWMutex
	findCache map[string]string

	// semantic is the optional embedding backend for Tier 4. Nil falls back to
	// the definition-scan heuristic.
	semantic SemanticSearcher

	// Budget allocation (percentages)
	tier1Budget float64
	tier2Budget float64
	tier3Budget float64
	tier4Budget float64

	// Max files per tier
	maxTier1 int
	maxTier2 int
	maxTier3 int
	maxTier4 int
}

// TieredContextConfig holds configuration for the builder.
type TieredContextConfig struct {
	WorkDir     string
	Retriever   *SparseRetriever
	Tier1Budget float64
	Tier2Budget float64
	Tier3Budget float64
	Tier4Budget float64
	MaxTotal    int

	// Semantic, when set, makes Tier 4 a real vector expansion instead of the
	// definition-scan heuristic. Optional by design: the retriever must work in
	// a session with no embedding backend configured.
	Semantic SemanticSearcher
}

// DefaultTieredContextConfig returns sensible defaults.
func DefaultTieredContextConfig(workDir string) *TieredContextConfig {
	return &TieredContextConfig{
		WorkDir:     workDir,
		Tier1Budget: 0.30,
		Tier2Budget: 0.40,
		Tier3Budget: 0.20,
		Tier4Budget: 0.10,
		MaxTotal:    50,
	}
}

// NewTieredContextBuilder creates a new builder.
func NewTieredContextBuilder(cfg *TieredContextConfig) *TieredContextBuilder {
	if cfg == nil {
		cfg = DefaultTieredContextConfig(".")
	}

	retriever := cfg.Retriever
	if retriever == nil {
		retriever = NewSparseRetriever(DefaultSparseRetrieverConfig(cfg.WorkDir))
	}

	maxTotal := cfg.MaxTotal
	if maxTotal == 0 {
		maxTotal = 50
	}

	return &TieredContextBuilder{
		retriever:   retriever,
		workDir:     cfg.WorkDir,
		findCache:   make(map[string]string),
		semantic:    cfg.Semantic,
		tier1Budget: cfg.Tier1Budget,
		tier2Budget: cfg.Tier2Budget,
		tier3Budget: cfg.Tier3Budget,
		tier4Budget: cfg.Tier4Budget,
		maxTier1:    int(float64(maxTotal) * cfg.Tier1Budget),
		maxTier2:    int(float64(maxTotal) * cfg.Tier2Budget),
		maxTier3:    int(float64(maxTotal) * cfg.Tier3Budget),
		maxTier4:    int(float64(maxTotal) * cfg.Tier4Budget),
	}
}

// =============================================================================
// CONTEXT FILE
// =============================================================================

// ContextFile represents a file selected for context injection.
type ContextFile struct {
	FilePath        string   `json:"file_path"`
	Tier            int      `json:"tier"`
	RelevanceScore  float64  `json:"relevance_score"`
	SelectionReason string   `json:"selection_reason"`
	Keywords        []string `json:"keywords,omitempty"`
	ImportedBy      []string `json:"imported_by,omitempty"`
	Content         string   `json:"content,omitempty"` // Populated on demand
}

// TieredContext represents the complete context built from all tiers.
type TieredContext struct {
	IssueText string         `json:"issue_text"`
	Keywords  *IssueKeywords `json:"keywords"`
	Files     []ContextFile  `json:"files"`

	// Candidates is the ranked Tier 2 keyword evidence, kept alongside the
	// selected files because candidate_file/2 and keyword_hit/3 need the scores
	// and per-keyword counts that ContextFile discards.
	Candidates []CandidateFile `json:"candidates,omitempty"`

	// ResolvedMentions maps each mention from the issue text to the real path it
	// resolved to, so callers can assert file_mentioned against a joinable path
	// rather than the raw spelling.
	ResolvedMentions map[string]string `json:"resolved_mentions,omitempty"`

	// Statistics
	Tier1Count int `json:"tier1_count"`
	Tier2Count int `json:"tier2_count"`
	Tier3Count int `json:"tier3_count"`
	Tier4Count int `json:"tier4_count"`
	TotalFiles int `json:"total_files"`
}

// =============================================================================
// BUILD CONTEXT
// =============================================================================

// BuildContext builds a tiered context from issue text.
func (b *TieredContextBuilder) BuildContext(ctx context.Context, issueText string) (*TieredContext, error) {
	keywords := ExtractKeywords(issueText)

	tc := &TieredContext{
		IssueText:        issueText,
		Keywords:         keywords,
		Files:            make([]ContextFile, 0),
		ResolvedMentions: make(map[string]string, len(keywords.MentionedFiles)),
	}

	// Track files already added to avoid duplicates
	addedFiles := make(map[string]bool)

	// Tier 1: Explicitly mentioned files
	tier1Files := b.extractMentionedFiles(ctx, keywords, addedFiles, tc.ResolvedMentions)
	tc.Files = append(tc.Files, tier1Files...)
	tc.Tier1Count = len(tier1Files)

	logging.Context("TieredContextBuilder: Tier 1 - %d explicitly mentioned files", tc.Tier1Count)

	// Tier 2: Keyword match files
	tier2Files, candidates, err := b.searchKeywordFiles(ctx, keywords, addedFiles)
	tc.Candidates = candidates
	if err != nil {
		logging.Context("TieredContextBuilder: Tier 2 search error: %v", err)
	} else {
		tc.Files = append(tc.Files, tier2Files...)
		tc.Tier2Count = len(tier2Files)
	}

	logging.Context("TieredContextBuilder: Tier 2 - %d keyword match files", tc.Tier2Count)

	// Tier 3: Import neighbors
	tier3Files := b.expandImportGraph(ctx, tc.Files, addedFiles)
	tc.Files = append(tc.Files, tier3Files...)
	tc.Tier3Count = len(tier3Files)

	logging.Context("TieredContextBuilder: Tier 3 - %d import neighbor files", tc.Tier3Count)

	// Tier 4: Semantic expansion (placeholder - requires embedding service)
	tier4Files := b.semanticExpansion(ctx, issueText, keywords, addedFiles)
	tc.Files = append(tc.Files, tier4Files...)
	tc.Tier4Count = len(tier4Files)

	logging.Context("TieredContextBuilder: Tier 4 - %d semantic expansion files", tc.Tier4Count)

	tc.TotalFiles = len(tc.Files)

	return tc, nil
}

// =============================================================================
// TIER 1: MENTIONED FILES
// =============================================================================

// extractMentionedFiles finds files explicitly mentioned in the issue and
// records each resolution in resolved, so the caller can assert file_mentioned
// against a real workspace path instead of the raw spelling from the issue text.
func (b *TieredContextBuilder) extractMentionedFiles(ctx context.Context, keywords *IssueKeywords, addedFiles map[string]bool, resolved map[string]string) []ContextFile {
	var files []ContextFile

	for _, mentioned := range keywords.MentionedFiles {
		if len(files) >= b.maxTier1 {
			break
		}

		// Try to find the file in the repository
		foundPath := b.findFile(ctx, mentioned)
		if foundPath == "" {
			continue
		}
		if resolved != nil {
			resolved[mentioned] = foundPath
		}

		if addedFiles[foundPath] {
			continue
		}
		addedFiles[foundPath] = true

		files = append(files, ContextFile{
			FilePath:        foundPath,
			Tier:            1,
			RelevanceScore:  1.0,
			SelectionReason: fmt.Sprintf("Explicitly mentioned in issue: %s", mentioned),
		})
	}

	return files
}

// ResolveFile locates a workspace file from a partial path (the exported form of
// findFile). Callers that need to resolve a mention outside a full build — the
// issue seed path, for one — use this so both sides agree on what a mention
// points at.
func (b *TieredContextBuilder) ResolveFile(ctx context.Context, partial string) string {
	return b.findFile(ctx, partial)
}

// findFile attempts to locate a file by partial path.
//
// Results are memoized because the walk is a full workspace traversal, and it
// honors ctx: it used to ignore cancellation entirely, so an expired seed budget
// still paid for every remaining tree walk before returning.
func (b *TieredContextBuilder) findFile(ctx context.Context, partial string) string {
	if partial == "" {
		return ""
	}
	// Existing callers pass a nil context; the cancellation checks below would
	// panic on one, and a resolver is not the place to start being strict.
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.RLock()
	cached, ok := b.findCache[partial]
	b.mu.RUnlock()
	if ok {
		return cached
	}

	found := b.locateFile(ctx, partial)

	// A miss caused by cancellation is not a real answer; caching it would make
	// the whole session believe the file does not exist.
	if found != "" || ctx.Err() == nil {
		b.mu.Lock()
		if b.findCache == nil {
			b.findCache = make(map[string]string)
		}
		b.findCache[partial] = found
		b.mu.Unlock()
	}
	return found
}

func (b *TieredContextBuilder) locateFile(ctx context.Context, partial string) string {
	// Try exact path first
	fullPath := filepath.Join(b.workDir, partial)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// Try finding by filename
	var found string
	filepath.Walk(b.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return err
		}
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "__pycache__" ||
				name == ".venv" || name == "venv" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// filepath.Walk yields OS-separated paths while callers write partials with
		// forward slashes, so compare both in slash form. The leading separator
		// keeps the match on a path boundary: a bare suffix test also accepts
		// ".../unnested/alpha.go" for "nested/alpha.go".
		slashPath := filepath.ToSlash(path)
		slashPartial := filepath.ToSlash(partial)
		if slashPath == slashPartial || strings.HasSuffix(slashPath, "/"+slashPartial) {
			found = path
		}
		return nil
	})

	return found
}

// =============================================================================
// TIER 2: KEYWORD MATCHES
// =============================================================================

// searchKeywordFiles uses the SparseRetriever to find keyword matches. It
// returns the selected Tier 2 files and the full ranked candidate list, which
// the caller needs to assert candidate_file/2 and keyword_hit/3 — those carry
// scores and per-keyword counts that ContextFile does not model.
func (b *TieredContextBuilder) searchKeywordFiles(ctx context.Context, keywords *IssueKeywords, addedFiles map[string]bool) ([]ContextFile, []CandidateFile, error) {
	// The previous revision also called FindRelevantFiles(ctx, "", …) here.
	// With an empty issue text ExtractKeywords yields nothing, so that call
	// ran a whole second keyword sweep of the repository that could only
	// return an empty ranking — pure cost, no candidates. The direct search
	// below already uses the keywords the caller extracted.
	hits, err := b.retriever.SearchKeywords(ctx, keywords)
	if err != nil {
		return nil, nil, err
	}

	// Rank the files
	ranked := b.retriever.RankFiles(hits, keywords, b.maxTier2*2)

	var files []ContextFile
	for _, candidate := range ranked {
		if len(files) >= b.maxTier2 {
			break
		}

		if addedFiles[candidate.FilePath] {
			continue
		}
		addedFiles[candidate.FilePath] = true

		files = append(files, ContextFile{
			FilePath:        candidate.FilePath,
			Tier:            2,
			RelevanceScore:  candidate.RelevanceScore,
			SelectionReason: fmt.Sprintf("Matches %d keywords: %s", candidate.UniqueKeywords, strings.Join(candidate.Keywords, ", ")),
			Keywords:        candidate.Keywords,
		})
	}

	return files, ranked, nil
}

// =============================================================================
// TIER 3: IMPORT NEIGHBORS
// =============================================================================

// expandImportGraph adds files that import or are imported by Tier 1-2 files.
func (b *TieredContextBuilder) expandImportGraph(ctx context.Context, existingFiles []ContextFile, addedFiles map[string]bool) []ContextFile {
	var newFiles []ContextFile

	// Collect imports for each existing file
	for _, file := range existingFiles {
		if len(newFiles) >= b.maxTier3 || ctx.Err() != nil {
			break
		}

		for _, resolvedPath := range b.importNeighbors(file.FilePath) {
			if len(newFiles) >= b.maxTier3 {
				break
			}

			if addedFiles[resolvedPath] {
				continue
			}
			addedFiles[resolvedPath] = true

			newFiles = append(newFiles, ContextFile{
				FilePath:        resolvedPath,
				Tier:            3,
				RelevanceScore:  0.5,
				SelectionReason: fmt.Sprintf("Imported by: %s", filepath.Base(file.FilePath)),
				ImportedBy:      []string{file.FilePath},
			})
		}
	}

	return newFiles
}

// importNeighbors resolves one file's imports to workspace files, dispatching on
// language. Tier 3 handled Python only, so on a Go repository — this one — the
// whole import tier was empty and the builder's 20% import budget went unused.
func (b *TieredContextBuilder) importNeighbors(filePath string) []string {
	if strings.EqualFold(filepath.Ext(filePath), ".go") {
		return b.goImportNeighbors(filePath)
	}

	var out []string
	for _, imp := range b.extractImports(filePath) {
		if resolved := b.resolveImport(imp, filePath); resolved != "" {
			out = append(out, resolved)
		}
	}
	return out
}

// extractImports extracts import statements from a Python file.
func (b *TieredContextBuilder) extractImports(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var imports []string
	importRegex := regexp.MustCompile(`^(?:from\s+([a-zA-Z0-9_.]+)\s+import|import\s+([a-zA-Z0-9_.]+))`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		matches := importRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			if matches[1] != "" {
				imports = append(imports, matches[1])
			}
			if matches[2] != "" {
				imports = append(imports, matches[2])
			}
		}
	}

	return imports
}

// resolveImport attempts to resolve a Python import to a file path.
func (b *TieredContextBuilder) resolveImport(importPath, currentFile string) string {
	// Convert import path to potential file paths
	parts := strings.Split(importPath, ".")
	currentDir := filepath.Dir(currentFile)

	// Try relative import first
	candidates := []string{
		filepath.Join(currentDir, strings.Join(parts, string(os.PathSeparator))+".py"),
		filepath.Join(currentDir, strings.Join(parts, string(os.PathSeparator)), "__init__.py"),
	}

	// Try from repo root
	candidates = append(candidates,
		filepath.Join(b.workDir, strings.Join(parts, string(os.PathSeparator))+".py"),
		filepath.Join(b.workDir, strings.Join(parts, string(os.PathSeparator)), "__init__.py"),
	)

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// =============================================================================
// TIER 4: SEMANTIC EXPANSION
// =============================================================================

// semanticExpansion finds files related to the issue by meaning rather than by
// spelling. It prefers a real embedding pass when one is configured and falls
// back to scanning for symbol definitions otherwise — a fallback, not a
// placeholder: the definition scan is the only thing that works in a session
// with no embedding backend, which is the common case.
func (b *TieredContextBuilder) semanticExpansion(ctx context.Context, issueText string, keywords *IssueKeywords, addedFiles map[string]bool) []ContextFile {
	if b.semantic != nil && b.maxTier4 > 0 {
		matches, err := b.semantic.SimilarFiles(ctx, issueText, b.maxTier4*2)
		if err != nil {
			logging.Context("TieredContextBuilder: Tier 4 semantic search failed, falling back to definition scan: %v", err)
		} else if files := semanticMatchFiles(matches, b.maxTier4, addedFiles); len(files) > 0 {
			return files
		}
	}

	// Heuristic expansion based on symbol names.
	var files []ContextFile

	for _, symbol := range keywords.MentionedSymbols {
		if len(files) >= b.maxTier4 {
			break
		}

		// Search for files defining this symbol
		definitionFiles := b.findSymbolDefinitions(ctx, symbol)
		for _, defFile := range definitionFiles {
			if len(files) >= b.maxTier4 {
				break
			}

			if addedFiles[defFile] {
				continue
			}
			addedFiles[defFile] = true

			files = append(files, ContextFile{
				FilePath:        defFile,
				Tier:            4,
				RelevanceScore:  0.3,
				SelectionReason: fmt.Sprintf("May define symbol: %s", symbol),
			})
		}
	}

	return files
}

// definitionKeywords are the language keywords that introduce a definition.
// They are matched literally, not as regexes — see findSymbolDefinitions.
var definitionKeywords = []string{
	"class",     // Python, Java, C++, TS
	"def",       // Python, Ruby
	"func",      // Go, Swift
	"function",  // JS, TS, PHP
	"fn",        // Rust
	"type",      // Go, TS
	"struct",    // Go, Rust, C
	"interface", // Go, TS, Java
	"impl",      // Rust
}

// findSymbolDefinitions searches for files that might define a symbol.
//
// This used to build patterns like "^class Foo" and pass them to
// searchSingleKeyword, which performs a literal byte scan — so it searched for
// a caret character in the source text and matched nothing, ever. Tier 4 was
// silently empty for every symbol.
//
// The patterns are now literal ("class Foo"), and the line-start intent the
// caret was reaching for is applied afterwards against each hit's column,
// which the scanner already reports. The keyword list also covers Go, Rust, JS
// and TS rather than Python alone.
func (b *TieredContextBuilder) findSymbolDefinitions(ctx context.Context, symbol string) []string {
	var files []string
	seen := make(map[string]bool)

	for _, keyword := range definitionKeywords {
		pattern := keyword + " " + symbol

		hits, err := b.retriever.searchSingleKeyword(ctx, pattern)
		if err != nil {
			continue
		}

		for _, hit := range hits {
			// A definition sits at the start of its line, allowing for
			// indentation (methods, nested types). Requiring column 1 would
			// drop every method; not checking at all would match the symbol
			// in the middle of an expression.
			if !isLineLeading(hit.Context, pattern) {
				continue
			}
			if !seen[hit.FilePath] {
				seen[hit.FilePath] = true
				files = append(files, hit.FilePath)
			}
		}
	}

	return files
}

// isLineLeading reports whether pattern begins the line, ignoring leading
// whitespace and any modifiers that legitimately precede a definition keyword
// (pub, export, async, public, static, ...). Comparison is case-insensitive to
// match the scanner's own case-folding.
func isLineLeading(line, pattern string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	target := strings.ToLower(pattern)

	if strings.HasPrefix(lower, target) {
		return true
	}
	for _, modifier := range definitionModifiers {
		rest := strings.TrimSpace(strings.TrimPrefix(lower, modifier+" "))
		if rest != lower && strings.HasPrefix(rest, target) {
			return true
		}
	}
	return false
}

// definitionModifiers may precede a definition keyword on the same line.
var definitionModifiers = []string{
	"pub", "export", "async", "public", "private", "protected",
	"static", "final", "abstract", "default", "const",
}

// =============================================================================
// CONTEXT HELPERS
// =============================================================================

// GetFilesByTier returns files filtered by tier.
func (tc *TieredContext) GetFilesByTier(tier int) []ContextFile {
	var files []ContextFile
	for _, f := range tc.Files {
		if f.Tier == tier {
			files = append(files, f)
		}
	}
	return files
}

// GetTopFiles returns the top N files by relevance score.
func (tc *TieredContext) GetTopFiles(n int) []ContextFile {
	// Sort by relevance score
	sorted := make([]ContextFile, len(tc.Files))
	copy(sorted, tc.Files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RelevanceScore > sorted[j].RelevanceScore
	})

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// GetFilePaths returns just the file paths for all context files.
func (tc *TieredContext) GetFilePaths() []string {
	paths := make([]string, len(tc.Files))
	for i, f := range tc.Files {
		paths[i] = f.FilePath
	}
	return paths
}

// LoadContent loads file content for all files up to maxBytes total.
func (tc *TieredContext) LoadContent(maxBytes int64) error {
	var totalBytes int64

	for i := range tc.Files {
		if totalBytes >= maxBytes {
			break
		}

		content, err := os.ReadFile(tc.Files[i].FilePath)
		if err != nil {
			continue
		}

		tc.Files[i].Content = string(content)
		totalBytes += int64(len(content))
	}

	return nil
}
