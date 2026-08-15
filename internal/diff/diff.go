// Package diff provides robust diff computation using the sergi/go-diff library.
// This replaces the manual LCS implementation with a battle-tested diff engine.
package diff

import (
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// diffTimeout bounds the cost of pathological diff inputs while remaining
// generous enough for typical code review diffs. Set to 0 to disable.
const diffTimeout = 5 * time.Second

// defaultContextLines is the fallback number of context lines for hunks.
const defaultContextLines = 3

// maxContextLines clamps absurd contextLines values from callers (or fuzzers).
const maxContextLines = 1000

// containsNullByte reports whether s contains a NUL byte, which is the
// conventional sentinel used to flag binary payloads in diff tooling.
func containsNullByte(s string) bool {
	return strings.IndexByte(s, 0x00) >= 0
}

// clampContextLines bounds contextLines to [0, maxContextLines] so callers
// cannot drive groupIntoHunks into a degenerate or pathological state.
func clampContextLines(n int) int {
	if n < 0 {
		return 0
	}
	if n > maxContextLines {
		return maxContextLines
	}
	return n
}

// LineType represents the type of diff line
type LineType int

const (
	LineContext LineType = iota // Unchanged context line
	LineAdded                   // Added line
	LineRemoved                 // Removed line

	// LineHeader is never produced by this engine and never will be: hunk
	// framing lives in the Hunk fields (OldStart/OldCount/NewStart/NewCount),
	// so a renderer composes its own "@@ -a,b +c,d @@" rather than receiving
	// one as a Line. It is kept — rather than deleted — because it is the
	// declared type for the header rows a renderer synthesizes and mixes into
	// its own line list, which cmd/nerd/ui does. Treat it as "UI-owned member
	// of the enum"; the engine emitting one would be a bug, and
	// TestComputeDiff_WhenAnyInput_ShouldNeverEmitLineHeader enforces that.
	LineHeader
)

// SpanType classifies a run of text inside a word-level comparison.
type SpanType int

const (
	SpanEqual  SpanType = iota // present on both sides
	SpanDelete                 // present only on the old side
	SpanInsert                 // present only on the new side
)

// WordSpan is one run of a word-level comparison between two lines.
//
// This replaces the raw diffmatchpatch.Diff slice ComputeWordLevelDiff used to
// return. Handing a third-party struct out of a public API forced every
// consumer to import sergi/go-diff to do anything with the result, and made the
// library's type layout part of codeNERD's API surface — the UI took the
// coward's way out and typed the parameter as `any`, which is why word-level
// highlighting sat unimplemented for so long.
type WordSpan struct {
	Type SpanType
	Text string
}

// Line represents a single line in the diff
type Line struct {
	LineNum int
	Content string
	Type    LineType
}

// Hunk represents a group of changes
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

// FileDiff represents changes to a single file
type FileDiff struct {
	OldPath  string
	NewPath  string
	Hunks    []Hunk
	IsNew    bool
	IsDelete bool
	IsBinary bool
}

// Engine provides diff computation with caching
type Engine struct {
	dmp   *diffmatchpatch.DiffMatchPatch
	cache *diffCache
	opts  Options
}

// cacheKey identifies a cached diff.
//
// Two independent 64-bit hashes plus both content lengths, rather than one hash
// per side: a single FNV-1a collision would serve one file's hunks as another
// file's diff, and the caller has no way to notice. Widening the key is free
// (both hashes come from one pass over the content) and drops the collision
// probability to the point where it stops being a correctness argument.
type cacheKey struct {
	oldHash      uint64
	oldHash2     uint64
	oldLen       int
	newHash      uint64
	newHash2     uint64
	newLen       int
	contextLines int
}

// contentFingerprint is the per-side half of a cacheKey.
type contentFingerprint struct {
	primary   uint64
	secondary uint64
	length    int
}

// fingerprint hashes s twice in one pass: FNV-1a and a differently seeded
// FNV-1a variant that also mixes position, so inputs that collide under one are
// overwhelmingly unlikely to collide under both.
func fingerprint(s string) contentFingerprint {
	const (
		offset64  = 14695981039346656037
		prime64   = 1099511628211
		offsetAlt = 1469598103934665603
		primeAlt  = 31
	)
	h1 := uint64(offset64)
	h2 := uint64(offsetAlt)
	for i := 0; i < len(s); i++ {
		c := uint64(s[i])
		h1 ^= c
		h1 *= prime64
		h2 = (h2+c+uint64(i))*primeAlt ^ (h2 >> 29)
	}
	return contentFingerprint{primary: h1, secondary: h2, length: len(s)}
}

// Options tunes an Engine. The zero value is valid and selects the defaults
// used before this struct existed, so NewEngine() and NewEngineWith(Options{})
// behave identically.
type Options struct {
	// ContextLines is the number of unchanged lines kept around each change.
	// Zero means defaultContextLines; use a negative value for genuinely zero
	// context.
	ContextLines int

	// DisableCache turns off result caching entirely. Counters still advance so
	// Stats remains meaningful.
	DisableCache bool

	// MaxCacheEntries bounds resident entries. Zero means defaultMaxCacheEntries.
	MaxCacheEntries int

	// MaxCacheBytes bounds approximate resident payload. Zero means
	// defaultMaxCacheBytes.
	MaxCacheBytes int64

	// Timeout bounds a single diffmatchpatch computation. Zero means
	// diffTimeout; use a negative value to disable the bound.
	Timeout time.Duration

	// VerifyCacheContent makes the engine retain the exact inputs alongside each
	// cached diff and byte-compare them on a hit, treating any mismatch as a
	// miss (counted in Stats.Collisions).
	//
	// Off by default because it roughly doubles cache memory for a hazard the
	// widened key already makes negligible. Turn it on where a wrong diff would
	// be applied rather than merely displayed.
	VerifyCacheContent bool
}

// contextLines resolves the configured context width to a concrete, clamped value.
func (o Options) contextLines() int {
	if o.ContextLines == 0 {
		return defaultContextLines
	}
	return clampContextLines(o.ContextLines)
}

// timeout resolves the configured diff timeout to a concrete value.
func (o Options) timeout() time.Duration {
	switch {
	case o.Timeout == 0:
		return diffTimeout
	case o.Timeout < 0:
		return 0 // diffmatchpatch treats 0 as "no timeout"
	default:
		return o.Timeout
	}
}

// NewEngine creates a new diff engine with optimal settings
func NewEngine() *Engine {
	return NewEngineWith(Options{})
}

// NewEngineWith creates a diff engine from opts. The zero Options value yields
// the same engine as NewEngine.
func NewEngineWith(opts Options) *Engine {
	dmp := diffmatchpatch.New()
	// Bound pathological inputs (e.g., massive minified single-line files)
	// while remaining generous enough for typical code diffs.
	dmp.DiffTimeout = opts.timeout()
	return &Engine{
		dmp:   dmp,
		cache: newDiffCache(opts.MaxCacheEntries, opts.MaxCacheBytes),
		opts:  opts,
	}
}

// Stats returns cumulative cache counters for this engine.
func (e *Engine) Stats() Stats {
	return e.cache.stats()
}

// DefaultEngine is a singleton engine for general use
var DefaultEngine = NewEngine()

// ComputeDiff creates a FileDiff from old and new content strings
// This function uses the sergi/go-diff library for robust diff computation
// and includes caching for identical input pairs.
func (e *Engine) ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff {
	fileDiff := &FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Hunks:   make([]Hunk, 0),
	}

	if oldContent == "" {
		fileDiff.IsNew = true
	}
	if newContent == "" {
		fileDiff.IsDelete = true
	}

	// Short-circuit binary content: a NUL byte in either side is the standard
	// signal that this is not a text payload. Sending it through diffmatchpatch
	// yields garbage hunks and (for very large blobs) ruinous memory/time use,
	// so we flag IsBinary=true and return an empty hunk list instead.
	if containsNullByte(oldContent) || containsNullByte(newContent) {
		fileDiff.IsBinary = true
		e.cache.markBinary()
		return fileDiff
	}

	contextLines := e.opts.contextLines()

	// Check cache. The key includes contextLines because hunk grouping depends
	// on it: two engines sharing content but not context width must not read
	// each other's entries.
	oldFP := fingerprint(oldContent)
	newFP := fingerprint(newContent)
	key := cacheKey{
		oldHash: oldFP.primary, oldHash2: oldFP.secondary, oldLen: oldFP.length,
		newHash: newFP.primary, newHash2: newFP.secondary, newLen: newFP.length,
		contextLines: contextLines,
	}

	if !e.opts.DisableCache {
		if cached := e.cache.get(key, oldContent, newContent); cached != nil {
			// get returns a deep copy, so retargeting the paths here cannot
			// disturb the cached entry or any diff handed to another caller.
			cached.OldPath = oldPath
			cached.NewPath = newPath
			return cached
		}
	}

	// Compute diffs using sergi/go-diff
	// Use a line-level reduction to avoid newline boundary artifacts when converting to line ops.
	e.cache.markCompute()
	a, b, lineArray := e.dmp.DiffLinesToChars(oldContent, newContent)
	diffs := e.dmp.DiffMain(a, b, false)
	diffs = e.dmp.DiffCleanupSemantic(diffs)
	diffs = e.dmp.DiffCharsToLines(diffs, lineArray)

	// Convert to hunks (configured context, clamped to safe bounds)
	fileDiff.Hunks = e.convertToHunks(diffs, contextLines)

	// Cache a copy; the caller keeps sole ownership of fileDiff.
	if !e.opts.DisableCache {
		e.cache.put(key, fileDiff, oldContent, newContent, e.opts.VerifyCacheContent)
	}

	return fileDiff
}

// ComputeDiff is a convenience function using the default engine
func ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff {
	return DefaultEngine.ComputeDiff(oldPath, newPath, oldContent, newContent)
}

// convertToHunks converts diffmatchpatch diffs to our Hunk format with context grouping.
// contextLines is clamped to [0, maxContextLines] to guard against pathological inputs
// (negative values, MaxInt, fuzzer-generated extremes).
func (e *Engine) convertToHunks(diffs []diffmatchpatch.Diff, contextLines int) []Hunk {
	if len(diffs) == 0 {
		return nil
	}

	contextLines = clampContextLines(contextLines)

	// Convert diffs to lines with types
	operations := e.diffsToOperations(diffs)
	if len(operations) == 0 {
		return nil
	}

	// Group operations into hunks
	return e.groupIntoHunks(operations, contextLines)
}

// operation represents a single line operation
type operation struct {
	typ     LineType
	oldLine int
	newLine int
	content string
}

// diffsToOperations converts diffmatchpatch diffs to line-based operations
func (e *Engine) diffsToOperations(diffs []diffmatchpatch.Diff) []operation {
	operations := make([]operation, 0)
	oldLine := 0
	newLine := 0

	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")

		// Handle empty diff edge case
		if len(lines) == 1 && lines[0] == "" && diff.Type != diffmatchpatch.DiffEqual {
			continue
		}

		// Remove trailing empty line from split
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		for i, line := range lines {
			// Skip empty lines at the end unless they're the only line
			if i == len(lines)-1 && line == "" && len(lines) > 1 {
				continue
			}

			switch diff.Type {
			case diffmatchpatch.DiffEqual:
				operations = append(operations, operation{
					typ:     LineContext,
					oldLine: oldLine,
					newLine: newLine,
					content: line,
				})
				oldLine++
				newLine++

			case diffmatchpatch.DiffDelete:
				operations = append(operations, operation{
					typ:     LineRemoved,
					oldLine: oldLine,
					newLine: -1,
					content: line,
				})
				oldLine++

			case diffmatchpatch.DiffInsert:
				operations = append(operations, operation{
					typ:     LineAdded,
					oldLine: -1,
					newLine: newLine,
					content: line,
				})
				newLine++
			}
		}
	}

	return operations
}

// groupIntoHunks groups operations into hunks with context
func (e *Engine) groupIntoHunks(ops []operation, contextLines int) []Hunk {
	if len(ops) == 0 {
		return nil
	}

	hunks := make([]Hunk, 0)
	var currentHunk *Hunk
	lastChangeIdx := -1

	for i, op := range ops {
		isChange := op.typ != LineContext

		if isChange {
			// Start a new hunk if needed
			if currentHunk == nil {
				currentHunk = &Hunk{
					Lines: make([]Line, 0),
				}

				// Add leading context
				start := max(i-contextLines, 0)

				for j := start; j < i; j++ {
					if ops[j].typ == LineContext {
						currentHunk.Lines = append(currentHunk.Lines, Line{
							LineNum: ops[j].oldLine + 1,
							Content: ops[j].content,
							Type:    LineContext,
						})
					}
				}

				// Set start positions
				if start < len(ops) {
					currentHunk.OldStart = ops[start].oldLine + 1
					currentHunk.NewStart = ops[start].newLine + 1
					// Handle edge cases where we start with an addition or deletion
					if ops[start].oldLine < 0 {
						currentHunk.OldStart = 0
					}
					if ops[start].newLine < 0 {
						currentHunk.NewStart = 0
					}
				}
			}
			lastChangeIdx = i
		}

		// Add the current operation to the hunk
		if currentHunk != nil {
			lineNum := op.oldLine + 1
			if op.typ == LineAdded {
				lineNum = op.newLine + 1
			}
			currentHunk.Lines = append(currentHunk.Lines, Line{
				LineNum: lineNum,
				Content: op.content,
				Type:    op.typ,
			})

			// Check if we should close the hunk (too much context after changes)
			if op.typ == LineContext && i-lastChangeIdx > contextLines {
				// Trim trailing context to contextLines
				trimTo := len(currentHunk.Lines) - (i - lastChangeIdx - contextLines)
				if trimTo > 0 && trimTo < len(currentHunk.Lines) {
					currentHunk.Lines = currentHunk.Lines[:trimTo]
				}

				// Count old and new lines
				e.computeHunkCounts(currentHunk)
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}

	// Close final hunk
	if currentHunk != nil && len(currentHunk.Lines) > 0 {
		e.computeHunkCounts(currentHunk)
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// computeHunkCounts calculates OldCount and NewCount for a hunk
func (e *Engine) computeHunkCounts(hunk *Hunk) {
	for _, line := range hunk.Lines {
		if line.Type == LineRemoved || line.Type == LineContext {
			hunk.OldCount++
		}
		if line.Type == LineAdded || line.Type == LineContext {
			hunk.NewCount++
		}
	}
}

// hash computes a simple FNV-1a hash. Retained as the primary half of
// fingerprint's output so existing hash-behavior tests keep their meaning.
func hash(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	hash := uint64(offset64)
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}

// ClearCache drops every cached diff. Unlike the previous implementation it
// does not reassign the cache field, so it is safe to call concurrently with
// ComputeDiff. Cumulative Stats counters are preserved.
func (e *Engine) ClearCache() {
	e.cache.clear()
}

// ComputeWordLevelDiff computes word-level differences within a line pair,
// returned as codeNERD spans in old-then-new reading order: a renderer walks
// the slice once, painting SpanEqual plus SpanDelete for the removed line and
// SpanEqual plus SpanInsert for the added one.
//
// Results are not cached: word diffs are computed per visible line pair, are
// cheap relative to a file diff, and caching them would key on content the
// caller already holds.
func (e *Engine) ComputeWordLevelDiff(oldLine, newLine string) []WordSpan {
	diffs := e.dmp.DiffMain(oldLine, newLine, false)
	diffs = e.dmp.DiffCleanupSemantic(diffs)

	spans := make([]WordSpan, 0, len(diffs))
	for _, d := range diffs {
		if d.Text == "" {
			continue
		}
		var typ SpanType
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			typ = SpanEqual
		case diffmatchpatch.DiffDelete:
			typ = SpanDelete
		case diffmatchpatch.DiffInsert:
			typ = SpanInsert
		}
		spans = append(spans, WordSpan{Type: typ, Text: d.Text})
	}
	return spans
}

// ComputeWordLevelDiff computes word-level spans using the default engine.
func ComputeWordLevelDiff(oldLine, newLine string) []WordSpan {
	return DefaultEngine.ComputeWordLevelDiff(oldLine, newLine)
}
