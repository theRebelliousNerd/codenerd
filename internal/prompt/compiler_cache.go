package prompt

import (
	"strconv"
)

// promptCacheEntry holds a cached compilation result and its key.
type promptCacheEntry struct {
	key    string
	result *CompilationResult
}

type promptCompilationOutcome struct {
	result   *CompilationResult
	cacheHit bool
}

func promptCacheKey(contextHash string, configRevision uint64) string {
	if configRevision == 0 {
		return contextHash
	}
	return HashContent(contextHash + "\x00compiler_config_revision=" + strconv.FormatUint(configRevision, 10))
}
