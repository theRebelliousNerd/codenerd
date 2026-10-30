package config

import (
	"encoding/json"
	"testing"
)

// jit.vector_search_timeout_ms is read from config, defaults when absent and
// is refused when negative.
func TestJITConfig_VectorSearchTimeout(t *testing.T) {
	var cfg UserConfig
	if err := json.Unmarshal([]byte(`{"jit":{"vector_search_timeout_ms":2500}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetEffectiveJITConfig().VectorSearchTimeoutMs; got != 2500 {
		t.Fatalf("configured timeout = %d, want 2500", got)
	}
	if got := DefaultUserConfig().GetJITConfig().VectorSearchTimeoutMs; got != DefaultJITConfig().VectorSearchTimeoutMs || got <= 0 {
		t.Fatalf("absent key gives %d, want the positive default", got)
	}
	probs := (JITConfig{VectorSearchTimeoutMs: -1}).Check("jit")
	if len(probs) != 1 || probs[0].Path != "jit.vector_search_timeout_ms" {
		t.Fatalf("negative timeout not refused: %v", probs)
	}
}
