package system

import (
	"testing"

	"codenerd/internal/config"
)

// The ingestor's default cap is the world section's default, not a literal
// left behind in this package.
func TestDefaultWorldModelConfig_MaxFilesPerScanComesFromWorldConfig(t *testing.T) {
	want := config.DefaultWorldConfig().ResolvedMaxFilesPerScan()
	if got := DefaultWorldModelConfig().MaxFilesPerScan; got != want {
		t.Fatalf("MaxFilesPerScan = %d, want world default %d", got, want)
	}
	if got := NewWorldModelIngestorShard().config.MaxFilesPerScan; got != want {
		t.Fatalf("constructed shard cap = %d, want %d", got, want)
	}
}

// Construction applies the loaded world section. Absent stays the default.
// Explicit 0 and negative stay unbounded.
func TestWorldModelConfigFor_ScanCapFollowsLoadedSection(t *testing.T) {
	if got := WorldModelConfigFor(config.WorldConfig{}).MaxFilesPerScan; got != 100 {
		t.Fatalf("absent cap = %d, want 100", got)
	}
	zero := 0
	if got := WorldModelConfigFor(config.WorldConfig{MaxFilesPerScan: &zero}).MaxFilesPerScan; got != 0 {
		t.Fatalf("explicit 0 = %d, want 0", got)
	}
	neg := -1
	if got := WorldModelConfigFor(config.WorldConfig{MaxFilesPerScan: &neg}).MaxFilesPerScan; got != -1 {
		t.Fatalf("explicit -1 = %d, want -1", got)
	}
	n := 7
	if got := WorldModelConfigFor(config.WorldConfig{MaxFilesPerScan: &n}).MaxFilesPerScan; got != 7 {
		t.Fatalf("explicit 7 = %d, want 7", got)
	}
}
