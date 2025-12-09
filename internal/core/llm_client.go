package core

import "context"

// LLMClient defines the minimal interface shards use to call an LLM.
// Mirrors perception.LLMClient to avoid import cycles.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// SchemaCapableLLMClient extends LLMClient with JSON Schema validation.
// This is an optional capability - use AsSchemaCapable() to check and convert.
// Currently implemented by: ClaudeCodeCLIClient (via --json-schema flag)
//
// JSON Schema validation ensures the LLM returns structured output conforming
// to the schema. This is critical for the Piggyback Protocol where we need
// guaranteed {control_packet, surface_response} structure.
type SchemaCapableLLMClient interface {
	LLMClient
	CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
}

// AsSchemaCapable checks if an LLMClient supports JSON Schema validation.
// Returns the SchemaCapableLLMClient and true if supported, nil and false otherwise.
//
// Example usage:
//
//	if schemaClient, ok := core.AsSchemaCapable(llmClient); ok {
//	    response, err := schemaClient.CompleteWithSchema(ctx, sys, user, schema)
//	}
func AsSchemaCapable(client LLMClient) (SchemaCapableLLMClient, bool) {
	sc, ok := client.(SchemaCapableLLMClient)
	return sc, ok
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
