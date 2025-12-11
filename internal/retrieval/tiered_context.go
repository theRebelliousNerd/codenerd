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
	retriever   *SparseRetriever
	workDir     string
	mu          sync.RWMutex

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
	FilePath       string   `json:"file_path"`
	Tier           int      `json:"tier"`
	RelevanceScore float64  `json:"relevance_score"`
	SelectionReason string  `json:"selection_reason"`
	Keywords       []string `json:"keywords,omitempty"`
	ImportedBy     []string `json:"imported_by,omitempty"`
	Content        string   `json:"content,omitempty"` // Populated on demand
}

// TieredContext represents the complete context built from all tiers.
type TieredContext struct {
	IssueText string        `json:"issue_text"`
	Keywords  *IssueKeywords `json:"keywords"`
	Files     []ContextFile `json:"files"`

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
		IssueText: issueText,
		Keywords:  keywords,
		Files:     make([]ContextFile, 0),
	}

	// Track files already added to avoid duplicates
	addedFiles := make(map[string]bool)

	// Tier 1: Explicitly mentioned files
	tier1Files := b.extractMentionedFiles(ctx, keywords, addedFiles)
	tc.Files = append(tc.Files, tier1Files...)
	tc.Tier1Count = len(tier1Files)

	logging.Context("TieredContextBuilder: Tier 1 - %d explicitly mentioned files", tc.Tier1Count)

	// Tier 2: Keyword match files
	tier2Files, err := b.searchKeywordFiles(ctx, keywords, addedFiles)
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

// extractMentionedFiles finds files explicitly mentioned in the issue.
func (b *TieredContextBuilder) extractMentionedFiles(ctx context.Context, keywords *IssueKeywords, addedFiles map[string]bool) []ContextFile {
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

		if addedFiles[foundPath] {
			continue
		}
		addedFiles[foundPath] = true

		files = append(files, ContextFile{
			FilePath:       foundPath,
			Tier:           1,
			RelevanceScore: 1.0,
			SelectionReason: fmt.Sprintf("Explicitly mentioned in issue: %s", mentioned),
		})
	}

	return files
}

// findFile attempts to locate a file by partial path.
func (b *TieredContextBuilder) findFile(ctx context.Context, partial string) string {
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
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "__pycache__" ||
				name == ".venv" || name == "venv" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if the path ends with our partial
		if strings.HasSuffix(path, partial) {
			found = path
		}
		return nil
	})

	return found
}

// =============================================================================
// TIER 2: KEYWORD MATCHES
// =============================================================================

// searchKeywordFiles uses the SparseRetriever to find keyword matches.
func (b *TieredContextBuilder) searchKeywordFiles(ctx context.Context, keywords *IssueKeywords, addedFiles map[string]bool) ([]ContextFile, error) {
	// Get candidate files from retriever
	candidates, err := b.retriever.FindRelevantFiles(ctx, "", b.maxTier2*2) // Request more, filter later
	if err != nil {
		return nil, err
	}

	// Use the pre-extracted keywords for a direct search
	hits, err := b.retriever.SearchKeywords(ctx, keywords)
	if err != nil {
		return nil, err
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
			FilePath:       candidate.FilePath,
			Tier:           2,
			RelevanceScore: candidate.RelevanceScore,
			SelectionReason: fmt.Sprintf("Matches %d keywords: %s", candidate.UniqueKeywords, strings.Join(candidate.Keywords, ", ")),
			Keywords:       candidate.Keywords,
		})
	}

	// Also include candidates from the direct issue search
	for _, candidate := range candidates {
		if len(files) >= b.maxTier2 {
			break
		}

		if addedFiles[candidate.FilePath] {
			continue
		}
		addedFiles[candidate.FilePath] = true

		files = append(files, ContextFile{
			FilePath:       candidate.FilePath,
			Tier:           2,
			RelevanceScore: candidate.RelevanceScore,
			SelectionReason: fmt.Sprintf("Matches %d keywords: %s", candidate.UniqueKeywords, strings.Join(candidate.Keywords, ", ")),
			Keywords:       candidate.Keywords,
		})
	}

	return files, nil
}

// =============================================================================
// TIER 3: IMPORT NEIGHBORS
// =============================================================================

// expandImportGraph adds files that import or are imported by Tier 1-2 files.
func (b *TieredContextBuilder) expandImportGraph(ctx context.Context, existingFiles []ContextFile, addedFiles map[string]bool) []ContextFile {
	var newFiles []ContextFile

	// Collect imports for each existing file
	for _, file := range existingFiles {
		if len(newFiles) >= b.maxTier3 {
			break
		}

		imports := b.extractImports(file.FilePath)
		for _, imp := range imports {
			if len(newFiles) >= b.maxTier3 {
				break
			}

			// Try to resolve import to file path
			resolvedPath := b.resolveImport(imp, file.FilePath)
			if resolvedPath == "" {
				continue
			}

			if addedFiles[resolvedPath] {
				continue
			}
			addedFiles[resolvedPath] = true

			newFiles = append(newFiles, ContextFile{
				FilePath:       resolvedPath,
				Tier:           3,
				RelevanceScore: 0.5,
				SelectionReason: fmt.Sprintf("Imported by: %s", filepath.Base(file.FilePath)),
				ImportedBy:     []string{file.FilePath},
			})
		}
	}

	return newFiles
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

// semanticExpansion uses vector similarity to find related files.
// This is a placeholder - full implementation requires embedding service.
func (b *TieredContextBuilder) semanticExpansion(ctx context.Context, issueText string, keywords *IssueKeywords, addedFiles map[string]bool) []ContextFile {
	// Placeholder: In production, this would:
	// 1. Generate embedding for the issue text
	// 2. Query vector database for similar file embeddings
	// 3. Return top matches not already in context

	// For now, use heuristic expansion based on symbol names
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
				FilePath:       defFile,
				Tier:           4,
				RelevanceScore: 0.3,
				SelectionReason: fmt.Sprintf("May define symbol: %s", symbol),
			})
		}
	}

	return files
}

// findSymbolDefinitions searches for files that might define a symbol.
func (b *TieredContextBuilder) findSymbolDefinitions(ctx context.Context, symbol string) []string {
	// Use ripgrep to find class/function definitions
	patterns := []string{
		fmt.Sprintf("^class %s", symbol),
		fmt.Sprintf("^def %s", symbol),
		fmt.Sprintf("^    def %s", symbol), // Method definition
	}

	var files []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		hits, err := b.retriever.searchSingleKeyword(ctx, pattern)
		if err != nil {
			continue
		}

		for _, hit := range hits {
			if !seen[hit.FilePath] {
				seen[hit.FilePath] = true
				files = append(files, hit.FilePath)
			}
		}
	}

	return files
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
