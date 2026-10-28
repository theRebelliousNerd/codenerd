package retrieval

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/workspace"
)

// =============================================================================
// NATIVE SCAN
// =============================================================================

// maxDefaultParallelism caps the default file-reader count. Each reader holds
// one file and its lower-cased copy (up to maxScanFileSize each), so the cap
// bounds memory on a many-core host; the walk is I/O-bound well before it.
const maxDefaultParallelism = 32

// Scan resource bounds.
//
// The scanner reads every non-excluded file in the workspace into memory, so
// each of these is load-bearing on a real repository rather than a tidiness
// preference.
const (
	// maxScanFileSize is the per-file read cutoff. Source files are far below
	// it; anything larger is a build artifact, dataset, or media blob whose
	// keyword matches would be noise anyway.
	maxScanFileSize = 4 << 20 // 4 MiB

	// maxHitsPerFile caps matches recorded for a single file. Minified and
	// generated files can hold hundreds of thousands of matches, and ranking
	// only needs to know the file matched heavily.
	maxHitsPerFile = 500

	// maxHitsPerKeyword caps total matches for one keyword across the repo.
	// Ranking keeps only maxResults files, so scanning past this is waste.
	// A result that reaches it is an overflow: true hits, not every hit.
	maxHitsPerKeyword = 20_000

	// binarySniffBytes is how much of a file is inspected for a NUL byte.
	binarySniffBytes = 8 << 10
)

// workerBudget returns how many file-reading workers a single keyword search
// may start, dividing the retriever's total parallelism across the keyword
// searches currently in flight. It never returns less than one.
func (r *SparseRetriever) workerBudget() int {
	total := r.parallelism
	if total < 1 {
		total = 1
	}
	inFlight := int(r.activeSearches.Load())
	if inFlight < 1 {
		inFlight = 1
	}
	if w := total / inFlight; w >= 1 {
		return w
	}
	return 1
}

// isBinaryContent reports whether data looks like a binary payload, using the
// conventional NUL-byte-in-the-first-block heuristic.
func isBinaryContent(data []byte) bool {
	head := data
	if len(head) > binarySniffBytes {
		head = head[:binarySniffBytes]
	}
	return bytes.IndexByte(head, 0x00) >= 0
}

// membershipFor is the one workspace-membership authority a scan may ask.
// Failing closed is deliberate: walking when git cannot answer would visit
// everything .gitignore exists to hide.
func membershipFor(root string) (*workspace.Membership, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	m, err := workspace.For(root)
	if err != nil {
		return nil, err
	}
	if err := m.Refresh(); err != nil {
		return nil, err
	}
	return m, nil
}

// searchSingleKeyword searches for one keyword with the configured backend.
func (r *SparseRetriever) searchSingleKeyword(ctx context.Context, keyword string) ([]KeywordHit, error) {
	ctx, cancel := context.WithTimeout(ctx, r.searchTimeout)
	defer cancel()

	// An external backend (ripgrep) replaces the native scan entirely when one
	// is configured; the bounds below only apply to the in-process walk.
	if backend := r.backend; backend != nil {
		searchStart := time.Now()
		r.metrics.searches.Add(1)
		r.activeSearches.Add(1)
		defer r.activeSearches.Add(-1)
		hits, berr := backend.Search(ctx, r.workDir, keyword, r.excludePatterns)
		r.metrics.hits.Add(int64(len(hits)))
		r.metrics.searchNanos.Add(int64(time.Since(searchStart)))
		if berr != nil {
			r.metrics.errors.Add(1)
		}
		return hits, berr
	}

	found, err := r.searchKeywordsNative(ctx, []string{keyword})
	return found[keyword], err
}

// excludedPath reports whether a purpose-filter glob excludes path: its own
// name, or the name of any directory under the workspace it sits in (the walk
// skips such a directory whole).
func (r *SparseRetriever) excludedPath(path string) bool {
	rel, err := filepath.Rel(r.workDir, path)
	if err != nil {
		rel = path
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		for _, pattern := range r.excludePatterns {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}
	return false
}

// scanFile reads one file once and matches it against every keyword: per
// keyword index, its word-bounded hits (at most maxHitsPerFile), each
// carrying its running count in the file. ok is false when the file is
// skipped: missing, oversized, unreadable or binary.
func (r *SparseRetriever) scanFile(path string, keywords []string, lowered [][]byte, want func(int) bool) (hits [][]KeywordHit, ok bool) {
	if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() > maxScanFileSize {
		r.metrics.filesSkipped.Add(1)
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil || isBinaryContent(data) {
		r.metrics.filesSkipped.Add(1)
		return nil, false
	}
	r.metrics.filesScanned.Add(1)
	lowerData := bytes.ToLower(data)

	hits = make([][]KeywordHit, len(keywords))
	var lines [][]byte
	var lineOffsets []int
	for i, kwBytes := range lowered {
		if want != nil && !want(i) {
			continue
		}
		offsets := ScanBuffer(lowerData, kwBytes)
		if len(offsets) == 0 {
			continue
		}
		if lines == nil {
			lines = bytes.Split(data, []byte("\n"))
			lineOffsets = make([]int, len(lines))
			current := 0
			for li, line := range lines {
				lineOffsets[li] = current
				current += len(line) + 1
			}
		}
		for _, offset := range offsets {
			if len(hits[i]) >= maxHitsPerFile {
				break
			}
			if !isWordBoundary(lowerData, offset, len(kwBytes)) {
				continue
			}
			lineIdx := sort.SearchInts(lineOffsets, offset+1) - 1
			if lineIdx < 0 {
				lineIdx = 0
			}
			hits[i] = append(hits[i], KeywordHit{
				FilePath: path,
				Keyword:  keywords[i],
				Line:     lineIdx + 1,
				Column:   offset - lineOffsets[lineIdx] + 1,
				Context:  strings.TrimSpace(strings.TrimRight(string(lines[lineIdx]), "\r\n")),
				Count:    len(hits[i]) + 1,
			})
		}
	}
	return hits, true
}

// searchKeywordsNative scans the workspace once for every keyword: each file
// is admitted, size- and binary-checked, read and lower-cased once, then
// matched against each keyword still under its hit cap. Per keyword the hits
// are exactly what a walk for that keyword alone records: word-bounded
// matches, at most maxHitsPerFile per file and maxHitsPerKeyword in all, each
// carrying its file's running count.
func (r *SparseRetriever) searchKeywordsNative(ctx context.Context, keywords []string) (map[string][]KeywordHit, error) {
	searchStart := time.Now()
	r.metrics.searches.Add(int64(len(keywords)))
	r.activeSearches.Add(1)
	defer r.activeSearches.Add(-1)
	defer func() { r.metrics.searchNanos.Add(int64(time.Since(searchStart))) }()

	lowered := make([][]byte, len(keywords))
	for i, kw := range keywords {
		lowered[i] = []byte(strings.ToLower(kw))
	}
	found := make(map[string][]KeywordHit, len(keywords))
	var mu sync.Mutex
	full := func(i int) bool { return len(found[keywords[i]]) >= maxHitsPerKeyword }

	files := make(chan string, 1000)
	var wg sync.WaitGroup
	for range r.workerBudget() {
		wg.Go(func() {
			// open is which keywords are still under their cap, read once per
			// file under one lock rather than once per keyword.
			open := make([]bool, len(keywords))
			for path := range files {
				if ctx.Err() != nil {
					return
				}
				anyOpen := false
				mu.Lock()
				for i := range keywords {
					open[i] = !full(i)
					anyOpen = anyOpen || open[i]
				}
				mu.Unlock()
				if !anyOpen {
					return
				}
				perKeyword, ok := r.scanFile(path, keywords, lowered, func(i int) bool { return open[i] })
				if !ok {
					continue
				}
				mu.Lock()
				for i, local := range perKeyword {
					for _, h := range local {
						if full(i) {
							break
						}
						found[keywords[i]] = append(found[keywords[i]], h)
					}
				}
				mu.Unlock()
			}
		})
	}

	// Walk directory. The cancellation check and the select on the send are
	// both required: workers return once ctx is done, and a walk blocked on a
	// full channel would never close it, so wg.Wait would never return.
	mem, memErr := membershipFor(r.workDir)
	if memErr != nil {
		close(files)
		wg.Wait()
		r.metrics.errors.Add(1)
		return found, fmt.Errorf("%w: %w", errMembership, memErr)
	}
	var walked int64
	err := filepath.WalkDir(r.workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		member, admErr := mem.Admit(path, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member {
			return nil
		}
		for _, pattern := range r.excludePatterns {
			if matched, _ := filepath.Match(pattern, d.Name()); matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if !d.IsDir() {
			walked++
			select {
			case files <- path:
			case <-ctx.Done():
				return filepath.SkipAll
			}
		}
		return nil
	})
	close(files)
	wg.Wait()
	r.metrics.filesWalked.Add(walked)
	total := 0
	for _, hits := range found {
		total += len(hits)
	}
	r.metrics.hits.Add(int64(total))

	if cerr := ctx.Err(); cerr != nil {
		r.metrics.timeouts.Add(1)
		if cerr == context.DeadlineExceeded {
			return found, fmt.Errorf("search timeout for %d keyword(s): %w", len(keywords), cerr)
		}
		return found, fmt.Errorf("search for %d keyword(s) canceled: %w", len(keywords), cerr)
	}
	if err != nil {
		r.metrics.errors.Add(1)
	}
	return found, err
}

// errMembership marks a scan that could not ask the workspace which files are
// members, so it walked nothing.
var errMembership = fmt.Errorf("workspace membership unavailable")

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
