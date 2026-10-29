package embedding

import "context"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewGenAIEngine creates a new GenAI embedding engine.
func NewGenAIEngine(apiKey, model, taskType string) (*GenAIEngine, error) {
	return NewGenAIEngineWithContext(context.Background(), apiKey, model, taskType)
}
