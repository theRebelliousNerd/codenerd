// Package retrieval provides efficient file discovery for large codebases.
// SparseRetriever uses keyword-based search (ripgrep) to quickly identify
// relevant files without loading the entire repository into memory.
package retrieval

import (
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// =============================================================================
// SPARSE RETRIEVER - Keyword-based file discovery for large repos
// =============================================================================

// SparseRetriever provides fast keyword-based file discovery using ripgrep.
// Designed for SWE-bench scenarios with 50,000+ file repositories.
type SparseRetriever struct {
	workDir string
	cache   *KeywordHitCache

	// mu guards the invalidation cursor and the exact file_written facts at its
	// timestamp. Writers currently record Unix seconds, so a new fact can share
	// the cursor timestamp; retaining only that boundary's identities avoids
	// replaying those facts forever without keeping unbounded write history.
	mu              sync.RWMutex
	lastWriteCursor int64
	lastWriteEvents map[string]struct{}
	// lastUnstamped are the timestampless change facts (file_deleted,
	// file_modified_externally) present at the previous pass.
	lastUnstamped map[string]struct{}

	// activeSearches counts keyword searches currently in flight so each one
	// can take a fair share of the shared file-worker budget instead of every
	// search starting a full pool of its own.
	activeSearches atomic.Int32

	// backend, when set, replaces the in-process scan with an external one.
	backend ScanBackend

	metrics retrieverMetrics

	// Configuration
	maxResults      int           // Max files to return
	searchTimeout   time.Duration // Per-search timeout
	parallelism     int           // Number of concurrent keyword searches
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

	// Backend selects the scanning engine. Nil means the native in-process
	// scan; see backend.go for the ripgrep alternative and why native is the
	// default.
	Backend ScanBackend
}

// DefaultSparseRetrieverConfig returns sensible defaults.
func DefaultSparseRetrieverConfig(workDir string) *SparseRetrieverConfig {
	return &SparseRetrieverConfig{
		WorkDir:       workDir,
		MaxResults:    100,
		SearchTimeout: 30 * time.Second,
		// Zero: one file reader per core, up to maxDefaultParallelism.
		Parallelism: 0,
		// Directory membership is workspace.For. These globs are a purpose
		// filter (bytecode, minified bundles), not a second ignore list.
		ExcludePatterns: []string{
			"*.pyc", "*.min.js", "*.egg-info",
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
		parallelism = min(runtime.NumCPU(), maxDefaultParallelism)
	}

	// Zero means default throughout this config: an unset cache size would
	// otherwise disable eviction while a zero TTL expired every entry on
	// read — an unbounded map that never hits.
	cacheSize := cfg.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1000
	}
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}

	return &SparseRetriever{
		workDir:         cfg.WorkDir,
		cache:           NewKeywordHitCache(cacheSize, cacheTTL),
		backend:         cfg.Backend,
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

// filePathExtensions is the alternation used by filePathPattern.
//
// Order is load-bearing: Go's regexp prefers the leftmost alternative that
// allows an overall match, so every extension that is a prefix of another
// ("ts" of "tsx", "c" of "cc"/"cpp", "m" of "mm") must come after the longer
// one. With the old ordering a mention of "Button.tsx" was not merely missed —
// nothing matched at all, because "ts" left a stray "x" where the delimiter had
// to be.
const filePathExtensions = `pyi|py|go|jsx|js|mjs|cjs|tsx|ts|rs|java|kts|kt|rb|swift|` +
	`cs|php|scala|vue|svelte|dart|ex|exs|cpp|cc|hpp|hh|c|h|mm|m|sh|sql|proto|mg`

var (
	// Patterns for extraction.
	//
	// The path body accepts '-' and '.' because real repositories are full of
	// "my-pkg/foo.test.ts", and the trailing set accepts ordinary sentence
	// punctuation so a file named at the end of a clause ("see main.go.") is
	// still extracted.
	filePathPattern = regexp.MustCompile(
		`(?:^|\s)([a-zA-Z_][a-zA-Z0-9_.\-\\/]*\.(?:` + filePathExtensions + `))(?:[\s:,;)\]}."'\x60]|$)`)
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
//
// It is a package function rather than a method because RipgrepBackend, not the
// retriever, is what actually produces this output — see backend.go.
func parseRipgrepOutput(output, keyword string) []KeywordHit {
	var hits []KeywordHit
	counts := make(map[string]int)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// path:line:col:content — keep content intact even if it embeds colons.
		// A Windows volume contains a colon before the vimgrep fields.
		prefix := ""
		if len(line) >= 3 && line[1] == ':' && (line[2] == '\\' || line[2] == '/') {
			prefix, line = line[:2], line[2:]
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		filePath := prefix + parts[0]
		lineNum, lineErr := strconv.Atoi(parts[1])
		colNum, colErr := strconv.Atoi(parts[2])
		if lineErr != nil || colErr != nil || lineNum < 1 || colNum < 1 {
			continue
		}
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

// RankFiles ranks files by keyword relevance.
func (r *SparseRetriever) RankFiles(hits []KeywordHit, keywords *IssueKeywords, limit int) []CandidateFile {
	if len(hits) == 0 {
		return nil
	}
	if keywords == nil {
		keywords = &IssueKeywords{}
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
		keywordList := make([]string, 0, len(keywordSet))
		for kw := range keywordSet {
			keywordList = append(keywordList, kw)
		}
		// The keyword list renders into Tier 2 selection reasons; map order
		// would make those strings (and the prompt bytes) nondeterministic.
		sort.Strings(keywordList)
		for _, kw := range keywordList {
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

	// Sort by relevance score (descending), breaking ties by path: equal
	// scores are routine (single-keyword files share one weight), and an
	// unstable order would make the limit below keep a different subset.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].RelevanceScore != candidates[j].RelevanceScore {
			return candidates[i].RelevanceScore > candidates[j].RelevanceScore
		}
		return candidates[i].FilePath < candidates[j].FilePath
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

// =============================================================================
// HELPERS
// =============================================================================

// commonWords is the stop-word set for keyword extraction. Package-level:
// rebuilding a 120-entry map on every extracted token was pure allocation.
var commonWords = map[string]bool{
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

// isCommonWord returns true if the word is too common to be useful.
func isCommonWord(word string) bool {
	// Too short
	if len(word) <= 2 {
		return true
	}

	// All uppercase single letters
	if len(word) == 1 && unicode.IsUpper(rune(word[0])) {
		return true
	}

	return commonWords[strings.ToLower(word)]
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
