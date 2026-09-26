package shards

import "codenerd/internal/types"

// Test conveniences; spawn calls clientForShardTypeLocked under its own lock.

// clientForShardType picks LLM for a shard: image family → imageLLMClient
// (Gemini Nano Banana 2), everything else → default llmClient (worker Ollama).
// Image shard types never fall back to the worker/main client — that would
// silently send Nano Banana work to Ollama (FM15). When the image client is
// unset, returns nil so spawn leaves the agent without a mis-wired client.
func (sm *ShardManager) clientForShardType(typeName string) types.LLMClient {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.clientForShardTypeLocked(typeName)
}
