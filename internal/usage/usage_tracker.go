package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type contextKey struct{}

// Tracker manages token usage recording and persistence.
type Tracker struct {
	mu            sync.Mutex
	data          UsageData
	filePath      string
	dirty         bool
	autoSaveTimer *time.Timer
}

// NewTracker creates a new usage tracker using the specified workspace persistence path.
func NewTracker(workspacePath string) (*Tracker, error) {
	nerdDir := filepath.Join(workspacePath, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .nerd dir: %w", err)
	}

	filePath := filepath.Join(nerdDir, "usage.json")
	t := &Tracker{
		filePath: filePath,
		data: UsageData{
			Version: "1.0",
			Aggregate: AggregatedStats{
				ByProvider:  make(map[string]TokenCounts),
				ByModel:     make(map[string]TokenCounts),
				ByShardType: make(map[string]TokenCounts),
				ByOperation: make(map[string]TokenCounts),
				BySession:   make(map[string]TokenCounts),
			},
		},
	}

	if err := t.Load(); err != nil {
		// Log error but continue with empty data if file is corrupt or missing
		// In a real logger we would log this.
	}

	return t, nil
}

// Load reads the usage data from disk.
func (t *Tracker) Load() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(t.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &t.data); err != nil {
		return err
	}

	// Ensure maps are initialized if file was empty/partial
	if t.data.Aggregate.ByProvider == nil {
		t.data.Aggregate.ByProvider = make(map[string]TokenCounts)
	}
	if t.data.Aggregate.ByModel == nil {
		t.data.Aggregate.ByModel = make(map[string]TokenCounts)
	}
	if t.data.Aggregate.ByShardType == nil {
		t.data.Aggregate.ByShardType = make(map[string]TokenCounts)
	}
	if t.data.Aggregate.ByOperation == nil {
		t.data.Aggregate.ByOperation = make(map[string]TokenCounts)
	}
	if t.data.Aggregate.BySession == nil {
		t.data.Aggregate.BySession = make(map[string]TokenCounts)
	}

	return nil
}

// Save writes the usage data to disk.
func (t *Tracker) Save() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.saveLocked()
}

func (t *Tracker) saveLocked() error {
	data, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.filePath, data, 0644)
}

// Track records a new usage event.
func (t *Tracker) Track(ctx context.Context, model, provider string, input, output int, operation string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Extract context metadata
	shardType := "unknown"
	if val := ctx.Value("shard_type"); val != nil {
		shardType = val.(string)
	}
	// Fallback to "ephemeral" if not specified but tracking is active?
	// Or maybe "chat" if it came from the main loop.

	shardName := "unknown"
	if val := ctx.Value("shard_name"); val != nil {
		shardName = val.(string)
	}

	sessionID := "unknown"
	if val := ctx.Value("session_id"); val != nil {
		sessionID = val.(string)
	}

	// Update Aggregates
	t.data.Aggregate.TotalProject.Add(input, output)

	addToMap(t.data.Aggregate.ByProvider, provider, input, output)
	addToMap(t.data.Aggregate.ByModel, model, input, output)
	addToMap(t.data.Aggregate.ByShardType, shardType, input, output)

	// Create composite key for shard name tracking if needed, or add a new map
	// For now, let's just log it if we want granular shard stats
	if shardName != "unknown" {
		// potential: t.data.Aggregate.ByShardName...
	}

	addToMap(t.data.Aggregate.ByOperation, operation, input, output)
	addToMap(t.data.Aggregate.BySession, sessionID, input, output)

	// Debounced auto-save
	if !t.dirty {
		t.dirty = true
		time.AfterFunc(5*time.Second, func() {
			t.Save()
			t.mu.Lock()
			t.dirty = false
			t.mu.Unlock()
		})
	}
}

// Stats returns a copy of the aggregated stats.
func (t *Tracker) Stats() AggregatedStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	stats := t.data.Aggregate
	stats.ByProvider = copyTokenCountsMap(stats.ByProvider)
	stats.ByModel = copyTokenCountsMap(stats.ByModel)
	stats.ByShardType = copyTokenCountsMap(stats.ByShardType)
	stats.ByOperation = copyTokenCountsMap(stats.ByOperation)
	stats.BySession = copyTokenCountsMap(stats.BySession)
	return stats
}

func copyTokenCountsMap(src map[string]TokenCounts) map[string]TokenCounts {
	if src == nil {
		return nil
	}
	dst := make(map[string]TokenCounts, len(src))
	for key, counts := range src {
		dst[key] = counts
	}
	return dst
}

func addToMap(m map[string]TokenCounts, key string, input, output int) {
	entry := m[key]
	entry.Add(input, output)
	m[key] = entry
}

// Context Helpers

// NewContext returns a new context carrying the tracker.
func NewContext(ctx context.Context, t *Tracker) context.Context {
	return context.WithValue(ctx, contextKey{}, t)
}

// FromContext retrieves the tracker from the context.
func FromContext(ctx context.Context) *Tracker {
	val := ctx.Value(contextKey{})
	if val == nil {
		return nil
	}
	return val.(*Tracker)
}

// WithShardContext adds shard metadata to the context.
func WithShardContext(ctx context.Context, name, typeName, sessionID string) context.Context {
	ctx = context.WithValue(ctx, "shard_name", name)
	ctx = context.WithValue(ctx, "shard_type", typeName)
	ctx = context.WithValue(ctx, "session_id", sessionID)
	return ctx
}
