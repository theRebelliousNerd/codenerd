package embedding

import (
	"sync/atomic"
	"time"
)

// pullTimeout is one Ollama model-pull HTTP call's bound, in nanoseconds.
// internal/config publishes it from embedding.pull_timeout
// (SetEmbeddingPullTimeout). This package cannot import config: config
// imports embedding. Zero means nothing has been published; the pull
// client's Timeout is then 0 and the caller's context is the only bound.
var pullTimeout atomic.Int64

// SetPullTimeout installs the bound for one Ollama model pull (POST /api/pull).
// config.SetEmbeddingPullTimeout is the production caller.
func SetPullTimeout(d time.Duration) {
	if d <= 0 {
		panic("embedding: pull timeout must be positive")
	}
	pullTimeout.Store(int64(d))
}

// PullTimeout is the installed Ollama model-pull bound.
// Zero means config has not published one.
func PullTimeout() time.Duration {
	return time.Duration(pullTimeout.Load())
}
