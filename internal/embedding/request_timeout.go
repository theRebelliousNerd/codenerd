package embedding

import (
	"sync/atomic"
	"time"
)

// embedRequestTimeout is one Ollama embed HTTP call's bound, in nanoseconds.
// internal/config publishes it from embedding.request_timeout
// (SetEmbeddingRequestTimeout). This package cannot import config: config
// imports embedding. Zero means nothing has been published; the Ollama
// client's Timeout is then 0 and the caller's context is the only bound.
var embedRequestTimeout atomic.Int64

// SetEmbedRequestTimeout installs the bound for one Ollama embed HTTP call.
// config.SetEmbeddingRequestTimeout is the production caller.
func SetEmbedRequestTimeout(d time.Duration) {
	if d <= 0 {
		panic("embedding: request timeout must be positive")
	}
	embedRequestTimeout.Store(int64(d))
}

// EmbedRequestTimeout is the installed Ollama embed-call bound.
// Zero means config has not published one.
func EmbedRequestTimeout() time.Duration {
	return time.Duration(embedRequestTimeout.Load())
}
