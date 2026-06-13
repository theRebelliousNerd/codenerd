// Package retrieval provides efficient file discovery for large codebases.
// SparseRetriever uses keyword-based search (ripgrep) to quickly identify
// relevant files without loading the entire repository into memory.
package retrieval

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"codenerd/internal/logging"
)

// =============================================================================
// SPARSE RETRIEVER - Keyword-based file discovery for large repos
// =============================================================================

// SparseRetriever provides fast keyword-based file discovery using ripgrep.
// Designed for SWE-bench scenarios with 50,000+ file repositories.
type SparseRetriever struct {
	workDir string
	cache   *KeywordHitCache
	mu      sync.RWMutex

	// Configuration
	maxResults      int           // Max files to return
	searchTimeout   time.Duration // Per-search timeout
	parallelism     int           // Number of parallel ripgrep processes
	excludePatterns []string      // Patterns to exclude from search
}

// SparseRetrieverConfig holds configuration for the retriever.
type SparseRetrieverConfig struct {
	WorkDir         string
	MaxResults      int
	SearchTimeout   time.Duration
	Parallelism     int
	ExcludePatterns []string
	CacheSize       int
	CacheTTL        time.Duration
}

// DefaultSparseRetrieverConfig returns sensible defaults.
func DefaultSparseRetrieverConfig(workDir string) *SparseRetrieverConfig {
	return &SparseRetrieverConfig{
		WorkDir:       workDir,
		MaxResults:    100,
		SearchTimeout: 30 * time.Second,
		Parallelism:   4,
		ExcludePatterns: []string{
			"*.pyc", "__pycache__", ".git", "node_modules",
			"*.egg-info", ".tox", ".pytest_cache", "*.min.js",
			"vendor", "dist", "build", ".venv", "venv",
		},
		CacheSize: 1000,
		CacheTTL:  5 * time.Minute,
	}
}

// NewSparseRetriever creates a new retriever with the given config.
func NewSparseRetriever(cfg *SparseRetrieverConfig) *SparseRetriever {
	if cfg == nil {
		cfg = DefaultSparseRetrieverConfig(".")
	}

	parallelism := cfg.Parallelism
	if parallelism <= 0 {
		parallelism = runtime.NumCPU()
	}

	return &SparseRetriever{
		workDir:         cfg.WorkDir,
		cache:           NewKeywordHitCache(cfg.CacheSize, cfg.CacheTTL),
		maxResults:      cfg.MaxResults,
		searchTimeout:   cfg.SearchTimeout,
		parallelism:     parallelism,
		excludePatterns: cfg.ExcludePatterns,
	}
}

// =============================================================================
// ISSUE KEYWORDS
// =============================================================================

// IssueKeywords represents extracted keywords from a problem statement.
type IssueKeywords struct {
	// Primary keywords are the most important (e.g., error names, function names)
	Primary []string

	// Secondary keywords are supporting terms (e.g., class names, module names)
	Secondary []string

	// Tertiary keywords are contextual (e.g., action verbs, generic terms)
	Tertiary []string

	// Weights map keywords to importance scores (0.0-1.0)
	Weights map[string]float64

	// MentionedFiles are file paths explicitly mentioned in the issue
	MentionedFiles []string

	// MentionedSymbols are code symbols (functions, classes) mentioned
	MentionedSymbols []string
}

var (
	// Patterns for extraction
	filePathPattern     = regexp.MustCompile(`(?:^|\s)([a-zA-Z_][a-zA-Z0-9_\\/]*\.(?:py|go|js|ts|rs|java|rb|cpp|c|h))(?:\s|$|:)`)
	pythonSymbolPattern = regexp.MustCompile(`\b([A-Z][a-zA-Z0-9_]*(?:Error|Exception|Warning)?)\b`)
	functionPattern     = regexp.MustCompile(`\b([a-z_][a-z0-9_]*)\s*\(`)
	methodPattern       = regexp.MustCompile(`\.([a-z_][a-z0-9_]*)\s*\(`)
	classPattern        = regexp.MustCompile(`\bclass\s+([A-Z][a-zA-Z0-9_]*)`)
	quotedPattern       = regexp.MustCompile(`["'\x60]([a-zA-Z_][a-zA-Z0-9_]*)["'\x60]`)
)

// ExtractKeywords extracts keywords from issue text using heuristics.
// For production use, this could be enhanced with NLP/LLM processing.

func ExtractKeywords(issueText string) *IssueKeywords {
	kw := &IssueKeywords{
		Weights:          make(map[string]float64),
		MentionedFiles:   make([]string, 0),
		MentionedSymbols: make([]string, 0),
	}

	// Fast-path: empty or whitespace-only input has no extractable keywords.
	// Cheap guard even though the regex passes would short-circuit naturally —
	// avoids six regex invocations on every blank call.
	if strings.TrimSpace(issueText) == "" {
		return kw
	}

	// Extract file paths
	for _, match := range filePathPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 {
			filePath := normalizePathSeparators(match[1])
			kw.MentionedFiles = append(kw.MentionedFiles, filePath)
			kw.Weights[filePath] = 1.0 // Highest weight for explicit files
		}
	}

	// Extract error types and class names (Primary)
	errorTypes := make(map[string]bool)
	for _, match := range pythonSymbolPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 {
			sym := match[1]
			if !isCommonWord(sym) && !errorTypes[sym] {
				errorTypes[sym] = true
				kw.MentionedSymbols = append(kw.MentionedSymbols, sym)
				kw.Primary = append(kw.Primary, sym)
				kw.Weights[sym] = 0.9
			}
		}
	}

	// Extract function names (Secondary)
	functions := make(map[string]bool)
	for _, match := range functionPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 {
			fn := match[1]
			if len(fn) > 2 && !isCommonWord(fn) && !functions[fn] {
				functions[fn] = true
				kw.Secondary = append(kw.Secondary, fn)
				kw.Weights[fn] = 0.7
			}
		}
	}

	// Extract method names (Secondary)
	for _, match := range methodPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 {
			method := match[1]
			if len(method) > 2 && !isCommonWord(method) && !functions[method] {
				functions[method] = true
				kw.Secondary = append(kw.Secondary, method)
				kw.Weights[method] = 0.7
			}
		}
	}

	// Extract explicit class definitions (Primary)
	for _, match := range classPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 && !errorTypes[match[1]] {
			kw.Primary = append(kw.Primary, match[1])
			kw.Weights[match[1]] = 0.85
		}
	}

	// Extract quoted strings as potential identifiers (Tertiary)
	for _, match := range quotedPattern.FindAllStringSubmatch(issueText, -1) {
		if len(match) > 1 {
			quoted := match[1]
			if len(quoted) > 2 && !isCommonWord(quoted) {
				kw.Tertiary = append(kw.Tertiary, quoted)
				kw.Weights[quoted] = 0.5
			}
		}
	}

	// Deduplicate
	kw.Primary = uniqueStrings(kw.Primary)
	kw.Secondary = uniqueStrings(kw.Secondary)
	kw.Tertiary = uniqueStrings(kw.Tertiary)
	kw.MentionedFiles = uniqueStrings(kw.MentionedFiles)
	kw.MentionedSymbols = uniqueStrings(kw.MentionedSymbols)

	return kw
}

// AllKeywords returns all keywords in priority order.
func (kw *IssueKeywords) AllKeywords() []string {
	all := make([]string, 0, len(kw.Primary)+len(kw.Secondary)+len(kw.Tertiary))
	all = append(all, kw.Primary...)
	all = append(all, kw.Secondary...)
	all = append(all, kw.Tertiary...)
	return all
}

// =============================================================================
// KEYWORD HIT
// =============================================================================

// KeywordHit represents a file matching a keyword search.
type KeywordHit struct {
	FilePath string
	Keyword  string
	Line     int
	Column   int
	Context  string // Line content
	Count    int    // Number of matches in file
}

// CandidateFile represents a file ranked by keyword relevance.
type CandidateFile struct {
	FilePath       string
	TotalHits      int          // Total keyword matches
	UniqueKeywords int          // Number of distinct keywords matched
	RelevanceScore float64      // Weighted relevance score
	Tier           int          // Context tier (1-4)
	Hits           []KeywordHit // Individual matches
	Keywords       []string     // Keywords that matched
}

// =============================================================================
// SEARCH METHODS
// =============================================================================

// SearchKeywords performs parallel keyword search using ripgrep.
func (r *SparseRetriever) SearchKeywords(ctx context.Context, keywords *IssueKeywords) ([]KeywordHit, error) {
	if keywords == nil || (len(keywords.AllKeywords()) == 0 && len(keywords.MentionedFiles) == 0) {
		return nil, nil
	}

	// Route files-only keyword search fallback candidates correctly
	if len(keywords.AllKeywords()) == 0 && len(keywords.MentionedFiles) > 0 {
		var hits []KeywordHit
		for _, file := range keywords.MentionedFiles {
			hits = append(hits, KeywordHit{
				FilePath: file,
				Keyword:  "[mentioned]",
				Line:     1,
				Column:   1,
				Context:  "Explicitly mentioned file",
				Count:    1,
			})
		}
		return hits, nil
	}

	logging.Context("SparseRetriever: searching %d keywords", len(keywords.AllKeywords()))

	allKeywords := keywords.AllKeywords()
	results := make(chan []KeywordHit, len(allKeywords))
	errors := make(chan error, len(allKeywords))

	// Limit parallelism
	semaphore := make(chan struct{}, r.parallelism)
	var wg sync.WaitGroup

	for _, keyword := range allKeywords {
		// Check cache first
		if cached, ok := r.cache.Get(keyword); ok {
			results <- cached
			continue
		}

		kw := keyword
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hits, err := r.searchSingleKeyword(ctx, kw)
			if err != nil {
				errors <- err
				return
			}

			// Cache the result
			r.cache.Set(kw, hits)
			results <- hits
		})
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	var allHits []KeywordHit
	for hits := range results {
		allHits = append(allHits, hits...)
	}

	// Check for errors
	for err := range errors {
		if err != nil {
			logging.Context("SparseRetriever: search error: %v", err)
		}
	}

	logging.Context("SparseRetriever: found %d total hits", len(allHits))
	return allHits, nil
}

// searchSingleKeyword uses native Go scanning to search for a single keyword.
func (r *SparseRetriever) searchSingleKeyword(ctx context.Context, keyword string) ([]KeywordHit, error) {
	ctx, cancel := context.WithTimeout(ctx, r.searchTimeout)
	defer cancel()

	var hits []KeywordHit
	hitCounts := make(map[string]int)
	var mu sync.Mutex

	kwBytes := []byte(strings.ToLower(keyword))

	// Channel for files to process
	files := make(chan string, 1000)

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < r.parallelism; i++ {
		wg.Go(func() {
			for path := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				// Convert to lower for case-insensitive search
				lowerData := bytes.ToLower(data)
				offsets := ScanBuffer(lowerData, kwBytes)

				if len(offsets) == 0 {
					continue
				}

				// Map byte offsets to lines and columns
				lines := bytes.Split(data, []byte("\n"))
				lineOffsets := make([]int, len(lines))
				currentOffset := 0
				for i, line := range lines {
					lineOffsets[i] = currentOffset
					currentOffset += len(line) + 1 // +1 for \n
				}

				var localHits []KeywordHit
				for _, offset := range offsets {
					// Word boundary check
					if !isWordBoundary(lowerData, offset, len(kwBytes)) {
						continue
					}

					// Find line
					lineIdx := sort.SearchInts(lineOffsets, offset+1) - 1
					if lineIdx < 0 {
						lineIdx = 0
					}

					colNum := offset - lineOffsets[lineIdx] + 1
					contextLine := strings.TrimRight(string(lines[lineIdx]), "\r\n")
					contextLine = strings.TrimSpace(contextLine)

					localHits = append(localHits, KeywordHit{
						FilePath: path,
						Keyword:  keyword,
						Line:     lineIdx + 1,
						Column:   colNum,
						Context:  contextLine,
					})
				}

				if len(localHits) > 0 {
					mu.Lock()
					for _, h := range localHits {
						hitCounts[h.FilePath]++
						h.Count = hitCounts[h.FilePath]
						hits = append(hits, h)
					}
					mu.Unlock()
				}
			}
		})
	}

	// Walk directory
	err := filepath.WalkDir(r.workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Check exclusions
		for _, pattern := range r.excludePatterns {
			matched, _ := filepath.Match(pattern, d.Name())
			if matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !d.IsDir() {
			files <- path
		}
		return nil
	})

	close(files)
	wg.Wait()

	// Surface context cancellation/timeout as an error so callers can
	// distinguish an interrupted search from a genuinely empty result set.
	if cerr := ctx.Err(); cerr != nil {
		if cerr == context.DeadlineExceeded {
			return hits, fmt.Errorf("search timeout for keyword %q", keyword)
		}
		return hits, fmt.Errorf("search for keyword %q canceled: %w", keyword, cerr)
	}

	return hits, err
}

// parseRipgrepOutput parses grep/ripgrep "--vimgrep" style output of the form
// "path:line:col:content" into KeywordHits. Lines that do not contain at least
// the four colon-separated fields are skipped. Line and column numbers are
// parsed best-effort (defaulting to 0 when non-numeric) so that partially
// malformed matches are still surfaced rather than silently dropped. Each hit
// carries a per-file 1-based occurrence Count, enabling downstream ranking to
// weight files with denser keyword coverage.
//
// Note: Windows drive-letter paths (e.g. "C:\\repo\\file.go:1:2:text") are
// counted as matches but the drive letter is treated as the leading field;
// callers that need exact Windows path fidelity should normalize upstream.
func (r *SparseRetriever) parseRipgrepOutput(output, keyword string) []KeywordHit {
	var hits []KeywordHit
	counts := make(map[string]int)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// path:line:col:content — keep content intact even if it embeds colons.
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		filePath := parts[0]
		lineNum, _ := strconv.Atoi(parts[1])
		colNum, _ := strconv.Atoi(parts[2])
		counts[filePath]++
		hits = append(hits, KeywordHit{
			FilePath: filePath,
			Keyword:  keyword,
			Line:     lineNum,
			Column:   colNum,
			Context:  strings.TrimRight(parts[3], "\r\n"),
			Count:    counts[filePath],
		})
	}
	return hits
}

func isWordBoundary(data []byte, offset, kwLen int) bool {
	if offset > 0 {
		prev := data[offset-1]
		if isAlphanumeric(prev) {
			return false
		}
	}
	if offset+kwLen < len(data) {
		next := data[offset+kwLen]
		if isAlphanumeric(next) {
			return false
		}
	}
	return true
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// RankFiles ranks files by keyword relevance.
func (r *SparseRetriever) RankFiles(hits []KeywordHit, keywords *IssueKeywords, limit int) []CandidateFile {
	if len(hits) == 0 {
		return nil
	}

	// Group hits by file
	fileHits := make(map[string][]KeywordHit)
	for _, hit := range hits {
		fileHits[hit.FilePath] = append(fileHits[hit.FilePath], hit)
	}

	// Calculate scores for each file
	candidates := make([]CandidateFile, 0, len(fileHits))
	for filePath, hits := range fileHits {
		// Count unique keywords
		keywordSet := make(map[string]bool)
		for _, hit := range hits {
			keywordSet[hit.Keyword] = true
		}

		// Calculate weighted score
		var score float64
		var keywordList []string
		for kw := range keywordSet {
			keywordList = append(keywordList, kw)
			weight := keywords.Weights[kw]
			if weight == 0 {
				weight = 0.3 // Default weight
			}
			score += weight
		}

		// Boost for multiple unique keywords
		if len(keywordSet) > 1 {
			score *= (1.0 + float64(len(keywordSet)-1)*0.2)
		}

		// Determine tier based on score and file type
		tier := r.determineTier(filePath, score, keywords)

		candidates = append(candidates, CandidateFile{
			FilePath:       filePath,
			TotalHits:      len(hits),
			UniqueKeywords: len(keywordSet),
			RelevanceScore: score,
			Tier:           tier,
			Hits:           hits,
			Keywords:       keywordList,
		})
	}

	// Sort by relevance score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RelevanceScore > candidates[j].RelevanceScore
	})

	// Apply limit
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates
}

// determineTier assigns a context tier (1-4) to a file.
func (r *SparseRetriever) determineTier(filePath string, score float64, keywords *IssueKeywords) int {
	normalizedFilePath := normalizePathSeparators(filePath)

	// Tier 1: Explicitly mentioned files
	for _, mentioned := range keywords.MentionedFiles {
		normalizedMentioned := normalizePathSeparators(mentioned)
		if strings.HasSuffix(normalizedFilePath, normalizedMentioned) || strings.Contains(normalizedFilePath, normalizedMentioned) {
			return 1
		}
	}

	// Tier 2: High-relevance keyword matches
	if score >= 2.0 {
		return 2
	}

	// Tier 3: Medium relevance
	if score >= 1.0 {
		return 3
	}

	// Tier 4: Low relevance (tertiary matches only)
	return 4
}

// FindRelevantFiles is a convenience method that combines extraction, search, and ranking.
func (r *SparseRetriever) FindRelevantFiles(ctx context.Context, issueText string, limit int) ([]CandidateFile, error) {
	keywords := ExtractKeywords(issueText)

	logging.Context("SparseRetriever: extracted keywords - primary=%d, secondary=%d, tertiary=%d, files=%d",
		len(keywords.Primary), len(keywords.Secondary), len(keywords.Tertiary), len(keywords.MentionedFiles))

	hits, err := r.SearchKeywords(ctx, keywords)
	if err != nil {
		return nil, err
	}

	if limit == 0 {
		limit = r.maxResults
	}

	return r.RankFiles(hits, keywords, limit), nil
}

// =============================================================================
// CACHE
// =============================================================================

// KeywordHitCache caches keyword search results.
type KeywordHitCache struct {
	entries map[string]*cacheEntry
	list    *list.List
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	key       string
	hits      []KeywordHit
	timestamp time.Time
	element   *list.Element
}

// NewKeywordHitCache creates a new cache.
func NewKeywordHitCache(maxSize int, ttl time.Duration) *KeywordHitCache {
	return &KeywordHitCache{
		entries: make(map[string]*cacheEntry),
		list:    list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves cached hits for a keyword.
func (c *KeywordHitCache) Get(keyword string) ([]KeywordHit, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[keyword]
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.timestamp) > c.ttl {
		c.list.Remove(entry.element)
		delete(c.entries, keyword)
		return nil, false
	}

	// LRU promotion
	c.list.MoveToFront(entry.element)

	// Return a copy to prevent cache aliasing corruption
	cloned := make([]KeywordHit, len(entry.hits))
	copy(cloned, entry.hits)
	return cloned, true
}

// Set stores hits for a keyword.
func (c *KeywordHitCache) Set(keyword string, hits []KeywordHit) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If entry already exists, update and promote it
	if entry, ok := c.entries[keyword]; ok {
		cloned := make([]KeywordHit, len(hits))
		copy(cloned, hits)
		entry.hits = cloned
		entry.timestamp = time.Now()
		c.list.MoveToFront(entry.element)
		return
	}

	// Evict if at capacity
	if c.list.Len() >= c.maxSize {
		c.evictOldest()
	}

	cloned := make([]KeywordHit, len(hits))
	copy(cloned, hits)

	entry := &cacheEntry{
		key:       keyword,
		hits:      cloned,
		timestamp: time.Now(),
	}
	elem := c.list.PushFront(entry)
	entry.element = elem
	c.entries[keyword] = entry
}

// evictOldest removes the oldest cache entry in O(1) time.
func (c *KeywordHitCache) evictOldest() {
	elem := c.list.Back()
	if elem != nil {
		c.list.Remove(elem)
		entry := elem.Value.(*cacheEntry)
		delete(c.entries, entry.key)
	}
}

// Clear empties the cache.
func (c *KeywordHitCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.list.Init()
}

// =============================================================================
// HELPERS
// =============================================================================

// isCommonWord returns true if the word is too common to be useful.
func isCommonWord(word string) bool {
	common := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "up": true, "down": true, "out": true,
		"and": true, "but": true, "or": true, "nor": true, "so": true, "yet": true,
		"if": true, "then": true, "else": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "every": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "not": true, "only": true,
		"own": true, "same": true, "than": true, "too": true, "very": true,
		"can": true, "just": true, "now": true, "new": true, "old": true,
		"get": true, "set": true, "make": true, "see": true, "know": true,
		"take": true, "come": true, "think": true, "look": true, "want": true,
		"give": true, "use": true, "find": true, "tell": true, "ask": true,
		"work": true, "seem": true, "feel": true, "try": true, "leave": true,
		"call": true, "good": true, "first": true, "last": true, "long": true,
		"great": true, "little": true,
		"right": true, "big": true, "high": true, "different": true, "small": true,
		"large": true, "next": true, "early": true, "young": true, "important": true,
		"public": true, "bad": true, "able": true,
		"this": true, "that": true, "these": true, "those": true,
		"it": true, "its": true, "i": true, "you": true, "he": true, "she": true,
		"we": true, "they": true, "my": true, "your": true, "his": true, "her": true,
		"our": true, "their": true, "me": true, "him": true, "us": true, "them": true,
		// Python keywords that aren't useful as search terms
		"def": true, "class": true, "import": true, "return": true,
		"self": true, "none": true, "true": true, "false": true,
		// Common code terms
		"test": true, "tests": true, "data": true, "file": true, "value": true,
		"name": true, "type": true, "error": true, "result": true,
	}

	lower := strings.ToLower(word)

	// Too short
	if len(word) <= 2 {
		return true
	}

	// All uppercase single letters
	if len(word) == 1 && unicode.IsUpper(rune(word[0])) {
		return true
	}

	return common[lower]
}

// uniqueStrings removes duplicates from a string slice.
func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func normalizePathSeparators(path string) string {
	// Normalize Windows-style paths so mentions and ripgrep results match regardless
	// of OS-specific separators.
	return strings.ReplaceAll(path, "\\", "/")
}
