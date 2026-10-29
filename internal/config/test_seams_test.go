package config

import "time"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// EmbeddingPullTimeout is the installed Ollama model-pull bound. Without a
// load it is the default (30m).
func EmbeddingPullTimeout() time.Duration {
	if n := activeEmbeddingPullTimeout.Load(); n > 0 {
		return time.Duration(n)
	}
	d, err := DefaultEmbeddingConfig().ResolvedPullTimeout()
	if err != nil {
		panic("config: default embedding.pull_timeout: " + err.Error())
	}
	return d
}
