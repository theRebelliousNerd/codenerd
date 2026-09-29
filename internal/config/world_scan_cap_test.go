package config

import (
	"encoding/json"
	"testing"
)

func TestWorldConfig_MaxFilesPerScanDefaultIs100(t *testing.T) {
	def := DefaultWorldConfig()
	if def.MaxFilesPerScan == nil || *def.MaxFilesPerScan != 100 {
		t.Fatalf("default max_files_per_scan = %v, want 100", def.MaxFilesPerScan)
	}
	if got := def.ResolvedMaxFilesPerScan(); got != 100 {
		t.Fatalf("resolved default = %d, want 100", got)
	}
	if got := (WorldConfig{}).ResolvedMaxFilesPerScan(); got != 100 {
		t.Fatalf("absent key resolved = %d, want 100", got)
	}
}

func TestGetWorldConfig_MaxFilesPerScanAbsentAndExplicit(t *testing.T) {
	if got := (*UserConfig)(nil).GetWorldConfig().ResolvedMaxFilesPerScan(); got != 100 {
		t.Fatalf("nil config cap = %d, want 100", got)
	}

	var absent UserConfig
	if err := json.Unmarshal([]byte(`{"world":{"fast_workers":3}}`), &absent); err != nil {
		t.Fatal(err)
	}
	got := absent.GetWorldConfig()
	if got.FastWorkers != 3 {
		t.Fatalf("fast_workers = %d, want 3", got.FastWorkers)
	}
	if got.MaxFilesPerScan != nil {
		t.Fatalf("absent max_files_per_scan = %d, want nil", *got.MaxFilesPerScan)
	}
	if n := got.ResolvedMaxFilesPerScan(); n != 100 {
		t.Fatalf("absent key resolved = %d, want 100", n)
	}

	var zero UserConfig
	if err := json.Unmarshal([]byte(`{"world":{"max_files_per_scan":0}}`), &zero); err != nil {
		t.Fatal(err)
	}
	if n := zero.GetWorldConfig().ResolvedMaxFilesPerScan(); n != 0 {
		t.Fatalf("explicit 0 resolved = %d, want 0 (unbounded)", n)
	}

	var neg UserConfig
	if err := json.Unmarshal([]byte(`{"world":{"max_files_per_scan":-4}}`), &neg); err != nil {
		t.Fatal(err)
	}
	if n := neg.GetWorldConfig().ResolvedMaxFilesPerScan(); n != -4 {
		t.Fatalf("explicit -4 resolved = %d, want -4 (unbounded)", n)
	}

	var set UserConfig
	if err := json.Unmarshal([]byte(`{"world":{"max_files_per_scan":25}}`), &set); err != nil {
		t.Fatal(err)
	}
	if n := set.GetWorldConfig().ResolvedMaxFilesPerScan(); n != 25 {
		t.Fatalf("explicit 25 resolved = %d, want 25", n)
	}
}
