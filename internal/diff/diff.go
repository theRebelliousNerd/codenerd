// Package diff provides robust diff computation using the sergi/go-diff library.
// This replaces the manual LCS implementation with a battle-tested diff engine.
package diff

import (
	"strings"
	"sync"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// LineType represents the type of diff line
type LineType int

const (
	LineContext LineType = iota // Unchanged context line
	LineAdded                   // Added line
	LineRemoved                 // Removed line
	LineHeader                  // Diff header line
)

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
	cache sync.Map // Cache for identical input pairs
}

// cacheKey is used for caching LCS/diff results
type cacheKey struct {
	oldHash uint64
	newHash uint64
}

// NewEngine creates a new diff engine with optimal settings
func NewEngine() *Engine {
	dmp := diffmatchpatch.New()
	// Optimize for code diffs
	dmp.DiffTimeout = 0 // Disable timeout for accuracy
	return &Engine{
		dmp:   dmp,
		cache: sync.Map{},
	}
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

	// Check cache
	oldHash := hash(oldContent)
	newHash := hash(newContent)
	key := cacheKey{oldHash, newHash}

	if cached, ok := e.cache.Load(key); ok {
		if cachedDiff, ok := cached.(*FileDiff); ok {
			// Clone cached result with updated paths
			result := *cachedDiff
			result.OldPath = oldPath
			result.NewPath = newPath
			return &result
		}
	}

	// Compute diffs using sergi/go-diff
	// Use a line-level reduction to avoid newline boundary artifacts when converting to line ops.
	a, b, lineArray := e.dmp.DiffLinesToChars(oldContent, newContent)
	diffs := e.dmp.DiffMain(a, b, false)
	diffs = e.dmp.DiffCleanupSemantic(diffs)
	diffs = e.dmp.DiffCharsToLines(diffs, lineArray)

	// Convert to hunks
	hunks := e.convertToHunks(diffs, 3) // 3 lines of context
	fileDiff.Hunks = hunks

	// Cache result
	e.cache.Store(key, fileDiff)

	return fileDiff
}

// ComputeDiff is a convenience function using the default engine
func ComputeDiff(oldPath, newPath, oldContent, newContent string) *FileDiff {
	return DefaultEngine.ComputeDiff(oldPath, newPath, oldContent, newContent)
}

// convertToHunks converts diffmatchpatch diffs to our Hunk format with context grouping
func (e *Engine) convertToHunks(diffs []diffmatchpatch.Diff, contextLines int) []Hunk {
	if len(diffs) == 0 {
		return nil
	}

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
				start := i - contextLines
				if start < 0 {
					start = 0
				}

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

// hash computes a simple hash for caching (FNV-1a algorithm)
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

// ClearCache clears the diff cache
func (e *Engine) ClearCache() {
	e.cache = sync.Map{}
}

// ComputeWordLevelDiff computes word-level differences within a line
// This is useful for highlighting specific changes within modified lines
func (e *Engine) ComputeWordLevelDiff(oldLine, newLine string) []diffmatchpatch.Diff {
	diffs := e.dmp.DiffMain(oldLine, newLine, false)
	diffs = e.dmp.DiffCleanupSemantic(diffs)
	return diffs
}
