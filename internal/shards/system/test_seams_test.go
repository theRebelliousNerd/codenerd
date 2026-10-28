package system

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewWorldModelIngestorShard creates a new World Model Ingestor shard.
func NewWorldModelIngestorShard() *WorldModelIngestorShard {
	return NewWorldModelIngestorShardWithConfig(DefaultWorldModelConfig())
}
