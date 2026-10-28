package retrieval

import (
	"context"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/workspace"
)

// =============================================================================
// SEARCH METHODS
// =============================================================================

// backendName names the keyword tier's scanning engine.
func (r *SparseRetriever) backendName() string {
	if r.backend != nil {
		return BackendRipgrep
	}
	return BackendNative
}

// searchDomain identifies what a cached keyword result is a claim about: the
// workspace's member set (workspace.Membership.Revision -- root, ignore
// patterns, and in git HEAD, its ref, the index and the ignore files), the
// purpose-filter globs, and the backend's matching semantics. A result
// computed over another domain is not reused.
func (r *SparseRetriever) searchDomain() (string, error) {
	root := r.workDir
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	m, err := workspace.For(root)
	if err != nil {
		return "", err
	}
	return r.backendName() + "\x00" + strings.Join(r.excludePatterns, "\x00") + "\x00" + m.Revision(), nil
}

// SearchKeywords searches the workspace for every keyword and reports how
// complete the search was. Hits from a keyword whose search failed or
// overflowed are still returned -- they are true -- but the outcome says the
// result is partial, so no caller mistakes it for "nothing else matches".
// Only a keyword's whole result is cached, and only into the domain epoch
// its scan started in.
func (r *SparseRetriever) SearchKeywords(ctx context.Context, keywords *IssueKeywords) ([]KeywordHit, TierOutcome) {
	out := TierOutcome{Tier: TierKeyword, Status: StatusComplete, Reason: ReasonNone, Attempted: r.backendName(), Used: r.backendName()}
	if keywords == nil || (len(keywords.AllKeywords()) == 0 && len(keywords.MentionedFiles) == 0) {
		return nil, out
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
		return hits, out
	}

	allKeywords := keywords.AllKeywords()
	out.Queries = len(allKeywords)
	logging.Context("SparseRetriever: searching %d keywords", len(allKeywords))

	// A domain that cannot be identified cannot be cached against: search,
	// but neither read nor publish.
	domain, domainErr := r.searchDomain()
	cacheable := domainErr == nil
	var epoch uint64
	if cacheable {
		epoch = r.cache.Begin(domain)
	}

	var allHits []KeywordHit
	var missing []string
	var overflowed []string
	for _, keyword := range allKeywords {
		if cacheable {
			if cached, ok := r.cache.Get(keyword); ok {
				r.metrics.cacheHits.Add(1)
				out.CacheHits++
				allHits = append(allHits, cached.hits...)
				if cached.overflow {
					overflowed = append(overflowed, keyword)
				}
				continue
			}
		}
		r.metrics.cacheMisses.Add(1)
		missing = append(missing, keyword)
	}

	var failed []string
	var searchErr error
	if len(missing) > 0 {
		var found map[string][]KeywordHit
		failedSet := map[string]bool{}
		if r.backend != nil {
			found, failedSet, searchErr = r.searchEachKeyword(ctx, missing)
		} else {
			// Every missing keyword in one walk: each file is read and
			// lower-cased once for all of them. One walk per keyword read the
			// whole workspace once per keyword, and a task brief carries
			// dozens (measured 2026-10-27: the 10 s retrieval budget expired
			// with 5448 hits still coming).
			sctx, cancel := context.WithTimeout(ctx, r.searchTimeout)
			found, searchErr = r.searchKeywordsNative(sctx, missing)
			cancel()
			if searchErr != nil {
				// One walk serves every keyword: if it stopped, none of them
				// is whole.
				for _, keyword := range missing {
					failedSet[keyword] = true
				}
			}
		}
		if searchErr != nil {
			logging.Context("SparseRetriever: search error: %v", searchErr)
		}
		for _, keyword := range missing {
			hits := found[keyword]
			allHits = append(allHits, hits...)
			if failedSet[keyword] {
				failed = append(failed, keyword)
				continue
			}
			// The external backend applies no hit cap, so it never
			// overflows; the native walk stops a keyword at the cap.
			overflow := r.backend == nil && len(hits) >= maxHitsPerKeyword
			if overflow {
				overflowed = append(overflowed, keyword)
			}
			if cacheable && !r.cache.Set(keyword, keywordResult{hits: hits, overflow: overflow}, epoch) {
				logging.ContextDebug("SparseRetriever: %q not cached; the workspace changed while it was searched", keyword)
			}
		}
	}

	out.Incomplete = append(append([]string(nil), failed...), overflowed...)
	switch {
	case len(failed) > 0 && len(failed) == len(allKeywords) && len(allHits) == 0:
		out.Status, out.Reason = StatusUnavailable, errorReason(searchErr)
	case len(failed) > 0:
		out.Status, out.Reason = StatusPartial, errorReason(searchErr)
	case len(overflowed) > 0:
		out.Status, out.Reason = StatusPartial, ReasonOverflow
	}
	if searchErr != nil {
		out.Detail = searchErr.Error()
	}
	logging.Context("SparseRetriever: found %d total hits (%s)", len(allHits), out)
	return allHits, out
}

// searchEachKeyword runs the configured external backend once per keyword,
// up to r.parallelism at a time. failed names the keywords whose search
// errored; err is the first such error.
func (r *SparseRetriever) searchEachKeyword(ctx context.Context, keywords []string) (found map[string][]KeywordHit, failed map[string]bool, err error) {
	found = make(map[string][]KeywordHit, len(keywords))
	failed = make(map[string]bool)
	var mu sync.Mutex
	semaphore := make(chan struct{}, max(r.parallelism, 1))
	var wg sync.WaitGroup
	for _, keyword := range keywords {
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			hits, kerr := r.searchSingleKeyword(ctx, keyword)
			mu.Lock()
			defer mu.Unlock()
			found[keyword] = hits
			if kerr != nil {
				failed[keyword] = true
				if err == nil {
					err = kerr
				}
			}
		})
	}
	wg.Wait()
	return found, failed, err
}
