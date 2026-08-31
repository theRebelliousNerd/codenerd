package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTracker_TrackAggregatesAndPersists(t *testing.T) {
	// TODO: What happens if model or provider is an empty string?
	// TODO: What happens if inputs cause integer overflow?
	// TODO: What happens if we provide extremely long string inputs for model or provider?
	// TODO: What happens if we have unbounded map growth for models or providers (unlike sessions)?
	// TODO: What happens if context cancellation occurs precisely during saveLocked()?
	// TODO: What happens if context keys are corrupted into non-string types causing panic?
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	// Avoid background autosave during the test (debounce uses AfterFunc).
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess_1")
	tracker.Track(ctx, "glm-4.6", "zai", 10, 5, "chat")
	tracker.Track(ctx, "glm-4.6", "zai", 2, 3, "chat")

	stats := tracker.Stats()
	if stats.TotalProject.Input != 12 || stats.TotalProject.Output != 8 || stats.TotalProject.Total != 20 {
		t.Fatalf("TotalProject=%+v, want input=12 output=8 total=20", stats.TotalProject)
	}
	if got := stats.ByProvider["zai"]; got.Total != 20 {
		t.Fatalf("ByProvider[zai]=%+v, want total=20", got)
	}
	if got := stats.ByModel["glm-4.6"]; got.Total != 20 {
		t.Fatalf("ByModel[glm-4.6]=%+v, want total=20", got)
	}
	if got := stats.ByShardType["coder"]; got.Total != 20 {
		t.Fatalf("ByShardType[coder]=%+v, want total=20", got)
	}
	if got := stats.ByOperation["chat"]; got.Total != 20 {
		t.Fatalf("ByOperation[chat]=%+v, want total=20", got)
	}
	if got := stats.BySession["sess_1"]; got.Total != 20 {
		t.Fatalf("BySession[sess_1]=%+v, want total=20", got)
	}

	if err := tracker.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
	if err != nil {
		t.Fatalf("read usage.json: %v", err)
	}
	var persisted UsageData
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("unmarshal usage.json: %v", err)
	}
	if persisted.Aggregate.TotalProject.Total != 20 {
		t.Fatalf("persisted total=%d, want 20", persisted.Aggregate.TotalProject.Total)
	}
}

func TestTracker_ContextHelpers(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctx := NewContext(context.Background(), tracker)
	if got := FromContext(ctx); got == nil {
		t.Fatalf("FromContext returned nil")
	}
	if got := FromContext(ctx); got != tracker {
		t.Fatalf("FromContext mismatch")
	}
}

func TestNewTracker(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker failed: %v", err)
		}
		if tracker == nil {
			t.Fatalf("tracker is nil")
		}

		expectedPath := filepath.Join(ws, ".nerd", "usage.json")
		if tracker.filePath != expectedPath {
			t.Errorf("got filePath %q, want %q", tracker.filePath, expectedPath)
		}

		// Ensure .nerd directory was created
		info, err := os.Stat(filepath.Join(ws, ".nerd"))
		if err != nil {
			t.Errorf("failed to stat .nerd dir: %v", err)
		} else if !info.IsDir() {
			t.Errorf(".nerd is not a directory")
		}

		// Verify maps are initialized
		if tracker.data.Aggregate.ByProvider == nil {
			t.Errorf("ByProvider map not initialized")
		}
		if tracker.data.Aggregate.ByModel == nil {
			t.Errorf("ByModel map not initialized")
		}
		if tracker.data.Aggregate.ByShardType == nil {
			t.Errorf("ByShardType map not initialized")
		}
		if tracker.data.Aggregate.ByOperation == nil {
			t.Errorf("ByOperation map not initialized")
		}
		if tracker.data.Aggregate.BySession == nil {
			t.Errorf("BySession map not initialized")
		}
	})

	t.Run("FailureMkdirAll", func(t *testing.T) {
		ws := t.TempDir()

		// Create a file named .nerd so MkdirAll fails
		err := os.WriteFile(filepath.Join(ws, ".nerd"), []byte("not a dir"), 0644)
		if err != nil {
			t.Fatalf("failed to write dummy .nerd file: %v", err)
		}

		tracker, err := NewTracker(ws)
		if err == nil {
			t.Fatalf("expected error when MkdirAll fails, got nil")
		}
		if tracker != nil {
			t.Fatalf("expected nil tracker, got %v", tracker)
		}
	})
}

func TestNewContext(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctx := context.Background()
	newCtx := NewContext(ctx, tracker)

	// Verify that the context now contains the tracker
	val := newCtx.Value(contextKey{})
	if val == nil {
		t.Fatalf("NewContext did not embed the tracker in the context")
	}

	retrievedTracker, ok := val.(*Tracker)
	if !ok {
		t.Fatalf("Embedded value is not of type *Tracker")
	}

	if retrievedTracker != tracker {
		t.Fatalf("Embedded tracker does not match the original tracker")
	}
}

func TestTracker_Load(t *testing.T) {
	t.Run("FileNotExist", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker: %v", err)
		}
		tracker.filePath = filepath.Join(ws, "nonexistent.json")

		err = tracker.Load()
		if err != nil {
			t.Errorf("expected no error for nonexistent file, got %v", err)
		}
	})

	t.Run("ReadError", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker: %v", err)
		}

		// Create a directory where a file is expected to cause a read error
		dirPath := filepath.Join(ws, "dir_instead_of_file")
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		tracker.filePath = dirPath

		err = tracker.Load()
		if err == nil {
			t.Errorf("expected read error, got nil")
		}
	})

	t.Run("JSONUnmarshalError", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker: %v", err)
		}

		filePath := filepath.Join(ws, "bad.json")
		if err := os.WriteFile(filePath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		tracker.filePath = filePath

		err = tracker.Load()
		if err == nil {
			t.Errorf("expected unmarshal error, got nil")
		}
	})

	t.Run("PartialDataMapInitialization", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker: %v", err)
		}

		// Write partial json (missing maps)
		filePath := filepath.Join(ws, "partial.json")
		if err := os.WriteFile(filePath, []byte(`{"version": "1.0", "aggregate": {"total_project": {"input": 10, "output": 20}}}`), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		tracker.filePath = filePath

		// Unset maps to ensure Load initializes them
		tracker.data.Aggregate.ByProvider = nil
		tracker.data.Aggregate.ByModel = nil
		tracker.data.Aggregate.ByShardType = nil
		tracker.data.Aggregate.ByOperation = nil
		tracker.data.Aggregate.BySession = nil

		err = tracker.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if tracker.data.Aggregate.ByProvider == nil {
			t.Errorf("ByProvider map not initialized")
		}
		if tracker.data.Aggregate.ByModel == nil {
			t.Errorf("ByModel map not initialized")
		}
		if tracker.data.Aggregate.ByShardType == nil {
			t.Errorf("ByShardType map not initialized")
		}
		if tracker.data.Aggregate.ByOperation == nil {
			t.Errorf("ByOperation map not initialized")
		}
		if tracker.data.Aggregate.BySession == nil {
			t.Errorf("BySession map not initialized")
		}

		if tracker.data.Aggregate.TotalProject.Input != 10 || tracker.data.Aggregate.TotalProject.Output != 20 {
			t.Errorf("TotalProject mismatch, got %+v", tracker.data.Aggregate.TotalProject)
		}
	})

	t.Run("Success", func(t *testing.T) {
		ws := t.TempDir()
		tracker, err := NewTracker(ws)
		if err != nil {
			t.Fatalf("NewTracker: %v", err)
		}

		// Write full json
		filePath := filepath.Join(ws, "full.json")
		fullJSON := `{
			"version": "1.0",
			"aggregate": {
				"total_project": {"input": 100, "output": 50, "total": 150},
				"by_provider": {"test-provider": {"input": 100, "output": 50, "total": 150}}
			}
		}`
		if err := os.WriteFile(filePath, []byte(fullJSON), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		tracker.filePath = filePath

		err = tracker.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if tracker.data.Aggregate.TotalProject.Total != 150 {
			t.Errorf("TotalProject mismatch")
		}
		if val, ok := tracker.data.Aggregate.ByProvider["test-provider"]; !ok || val.Total != 150 {
			t.Errorf("ByProvider mismatch")
		}
	})
}
