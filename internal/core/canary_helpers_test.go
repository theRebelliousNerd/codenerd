package core

import (
	"sync"
	"testing"
)

// safeActionCanaryBaseline returns the number of safe_action rows a pristine
// kernel derives, caching the count across the package's canary tests.
//
// The canary baseline drifts whenever a permitted action is added to the
// constitution (for example safe_action(/browser_audit)), so pinning a literal
// count in each test guarantees a future false red. A single shared helper
// keeps the three canaries (dependency reachability, intent identity, shards
// decl) anchored to whatever the current constitution derives.
//
// The kernel is booted with NewRealKernel, the same constructor the canary
// tests use for their pristine kernel. The count must stay at or above 100 so
// a degraded pristine kernel still fails loudly instead of silently lowering
// the baseline.
var (
	safeActionCanaryOnce  sync.Once
	safeActionCanaryCount int
	safeActionCanaryErr   error
)

func safeActionCanaryBaseline(t *testing.T) int {
	t.Helper()
	safeActionCanaryOnce.Do(func() {
		k, err := NewRealKernel()
		if err != nil {
			safeActionCanaryErr = err
			return
		}
		rows, err := k.Query("safe_action(A)")
		if err != nil {
			safeActionCanaryErr = err
			return
		}
		safeActionCanaryCount = len(rows)
	})
	if safeActionCanaryErr != nil {
		t.Fatalf("safeActionCanaryBaseline: pristine kernel query failed: %v", safeActionCanaryErr)
	}
	if safeActionCanaryCount < 100 {
		t.Fatalf("safeActionCanaryBaseline = %d rows, want at least 100 — pristine constitution analysis is degraded",
			safeActionCanaryCount)
	}
	return safeActionCanaryCount
}
