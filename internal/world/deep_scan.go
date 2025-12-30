package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// DeepResult describes a deep (Cartographer) scan.
type DeepResult struct {
	NewFacts     []core.Fact
	RetractFacts []core.Fact
	FilesParsed  int
	Duration     time.Duration
}

// EnsureDeepFacts ensures deep world facts for the given file paths.
// Only Go files are deep-parsed today; others are ignored.
// Cached deep facts (depth="deep") are reused when fingerprints match.
func EnsureDeepFacts(ctx context.Context, paths []string, db *store.LocalStore, workers int) (*DeepResult, error) {
	start := time.Now()
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 8 {
			workers = 8
		}
		if workers < 2 {
			workers = 2
		}
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	loadFacts := make([]core.Fact, 0)
	retractFacts := make([]core.Fact, 0)
	parsed := 0

	for _, p := range paths {
		path := p
		if filepath.Ext(path) != ".go" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := os.Stat(path)
			if err != nil {
				return
			}
			fp := fileFingerprint(info)

			// Load cached deep facts for retraction and/or reuse.
			var cachedFacts []core.Fact
			cachedFp := ""
			if db != nil {
				oldInputs, oldFp, loadErr := db.LoadWorldFactsForFile(path, "deep")
				if loadErr == nil && len(oldInputs) > 0 {
					cachedFp = oldFp
					cachedFacts = make([]core.Fact, 0, len(oldInputs))
					for _, in := range oldInputs {
						cachedFacts = append(cachedFacts, core.Fact{Predicate: in.Predicate, Args: in.Args})
					}
					// Always retract existing deep facts for this file to avoid duplicates.
					mu.Lock()
					retractFacts = append(retractFacts, cachedFacts...)
					mu.Unlock()
				}
			}

			if len(cachedFacts) > 0 && cachedFp == fp {
				// Reuse cached deep facts.
				mu.Lock()
				loadFacts = append(loadFacts, cachedFacts...)
				mu.Unlock()
				return
			}

			// Parse deep facts.
			c := NewCartographer()
			deepFacts, parseErr := c.MapFile(path)
			if parseErr != nil || len(deepFacts) == 0 {
				return
			}

			mu.Lock()
			parsed++
			mu.Unlock()
			if db != nil {
				inputs := make([]store.WorldFactInput, 0, len(deepFacts))
				for _, f := range deepFacts {
					inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
				}
				_ = db.ReplaceWorldFactsForFile(path, "deep", fp, inputs)
			}

			mu.Lock()
			loadFacts = append(loadFacts, deepFacts...)
			mu.Unlock()
		}()
	}

	wg.Wait()
	logging.WorldDebug("Deep scan complete: parsed=%d facts=%d duration=%v", parsed, len(loadFacts), time.Since(start))

	return &DeepResult{
		NewFacts:     loadFacts,
		RetractFacts: retractFacts,
		FilesParsed:  parsed,
		Duration:     time.Since(start),
	}, nil
}
