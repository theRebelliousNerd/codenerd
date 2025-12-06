package core

import "context"

// LLMClient defines the minimal interface shards use to call an LLM.
// Mirrors perception.LLMClient to avoid import cycles.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// TracingClient extends LLMClient with context-setting for trace capture.
// Implemented by perception.TracingLLMClient.
type TracingClient interface {
	LLMClient
	SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string)
	ClearShardContext()
}

// ShardTraceAccessor provides shards with access to their own historical traces.
// Enables self-learning by querying past reasoning patterns.
type ShardTraceAccessor interface {
	GetMyTraces(limit int) ([]interface{}, error)
	GetMyFailedTraces(limit int) ([]interface{}, error)
	GetSimilarTasks(taskPattern string, limit int) ([]interface{}, error)
	GetSuccessfulPatterns(limit int) ([]interface{}, error)
}
