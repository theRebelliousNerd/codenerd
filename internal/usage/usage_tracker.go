package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codenerd/internal/logging"
)

type contextKey struct{}

// shardMetaKey types the context keys carrying shard metadata. Raw string keys
// ("shard_type") collide across packages and are invisible to vet; a private
// struct key cannot be set or read by accident from outside this package.
type shardMetaKey struct{ name string }

var (
	shardNameKey = shardMetaKey{"shard_name"}
	shardTypeKey = shardMetaKey{"shard_type"}
	sessionIDKey = shardMetaKey{"session_id"}
)

// autoSaveDelay is how long Track waits before flushing a batch of mutations.
const autoSaveDelay = 5 * time.Second

// maxSessions bounds the BySession map. A long-lived workspace accumulates one
// entry per session forever otherwise, and usage.json grows without limit.
// When the cap is exceeded the smallest-spend sessions are folded into a single
// "(pruned)" bucket so totals still reconcile.
const maxSessions = 500

// prunedSessionKey is the bucket that absorbs pruned per-session rows.
const prunedSessionKey = "(pruned)"

// maxEvents bounds the retained raw event ring. The Events field previously
// implied a raw log but was never written; it is now a genuine bounded ring, so
// operators get recent history without unbounded file growth.
const maxEvents = 1000

// Tracker manages token usage recording and persistence.
type Tracker struct {
	mu       sync.Mutex
	data     UsageData
	filePath string

	// dirty means "there are mutations not yet on disk". saving means "a flush
	// is in flight". Both are needed: the old code cleared dirty after Save
	// returned, so any Track that landed during the write saw dirty==true,
	// skipped re-arming the timer, and was then marked clean — losing the
	// mutation until some later Track happened to re-arm.
	dirty  bool
	saving bool

	autoSaveTimer *time.Timer
	closed        bool

	// keepEvents enables the raw event ring. Off by default: most operators
	// only want aggregates, and the ring roughly doubles usage.json size.
	keepEvents bool
}

// Option configures a Tracker.
type Option func(*Tracker)

// WithEventLog enables retention of the most recent maxEvents raw events.
func WithEventLog() Option {
	return func(t *Tracker) { t.keepEvents = true }
}

// NewTracker creates a new usage tracker using the specified workspace persistence path.
func NewTracker(workspacePath string, opts ...Option) (*Tracker, error) {
	nerdDir := filepath.Join(workspacePath, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .nerd dir: %w", err)
	}

	filePath := filepath.Join(nerdDir, "usage.json")
	t := &Tracker{
		filePath: filePath,
		data:     newUsageData(),
	}
	for _, opt := range opts {
		opt(t)
	}

	if err := t.Load(); err != nil {
		// A corrupt or unreadable usage.json must not stop the session: usage
		// accounting is advisory. Start from empty aggregates and say so.
		logging.Get(logging.CategorySession).Warn(
			"usage: could not load %s, starting from empty aggregates: %v", filePath, err)
	}

	return t, nil
}

func newUsageData() UsageData {
	return UsageData{
		Version: "1.0",
		Aggregate: AggregatedStats{
			ByProvider:  make(map[string]TokenCounts),
			ByModel:     make(map[string]TokenCounts),
			ByShardType: make(map[string]TokenCounts),
			ByOperation: make(map[string]TokenCounts),
			BySession:   make(map[string]TokenCounts),
			ByShardName: make(map[string]TokenCounts),
		},
	}
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
		// Leave the tracker on fresh aggregates rather than a half-populated
		// struct from a partial unmarshal.
		t.data = newUsageData()
		return fmt.Errorf("parse %s: %w", t.filePath, err)
	}

	t.ensureMapsLocked()
	return nil
}

// ensureMapsLocked initializes any nil aggregate map. Caller holds mu.
func (t *Tracker) ensureMapsLocked() {
	a := &t.data.Aggregate
	if a.ByProvider == nil {
		a.ByProvider = make(map[string]TokenCounts)
	}
	if a.ByModel == nil {
		a.ByModel = make(map[string]TokenCounts)
	}
	if a.ByShardType == nil {
		a.ByShardType = make(map[string]TokenCounts)
	}
	if a.ByOperation == nil {
		a.ByOperation = make(map[string]TokenCounts)
	}
	if a.BySession == nil {
		a.BySession = make(map[string]TokenCounts)
	}
	if a.ByShardName == nil {
		a.ByShardName = make(map[string]TokenCounts)
	}
}

// Save writes the usage data to disk.
func (t *Tracker) Save() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.saveLocked()
}

// saveLocked serializes and atomically replaces usage.json. Caller holds mu.
//
// The previous implementation used os.WriteFile, which truncates in place: a
// crash or a full disk mid-write left a truncated, unparseable usage.json and
// the whole history was gone. Writing a sibling temp file and renaming means a
// reader sees either the old file or the new one, never a partial one.
func (t *Tracker) saveLocked() error {
	payload, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal usage data: %w", err)
	}

	dir := filepath.Dir(t.filePath)
	tmp, err := os.CreateTemp(dir, ".usage-*.json")
	if err != nil {
		return fmt.Errorf("create temp usage file: %w", err)
	}
	tmpName := tmp.Name()

	// Any failure past this point must not leave the temp file behind.
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temp usage file: %w", err)
	}
	// fsync before rename: without it the rename can land while the contents
	// are still only in the page cache, which is exactly the crash window the
	// temp file was meant to close.
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp usage file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp usage file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp usage file: %w", err)
	}
	if err := os.Rename(tmpName, t.filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename usage file into place: %w", err)
	}

	return nil
}

// Track records a new usage event.
//
// Negative token counts are rejected: they can only come from a provider bug or
// a bad cast, and letting them through silently corrupts every aggregate they
// touch with no way to reconstruct the truth.
func (t *Tracker) Track(ctx context.Context, model, provider string, input, output int, operation string) {
	if input < 0 || output < 0 {
		logging.Get(logging.CategorySession).Warn(
			"usage: rejecting negative token counts for model=%q provider=%q input=%d output=%d",
			model, provider, input, output)
		return
	}
	if input == 0 && output == 0 {
		return
	}

	shardName := shardMetaFromContext(ctx, shardNameKey, "shard_name")
	shardType := shardMetaFromContext(ctx, shardTypeKey, "shard_type")
	sessionID := shardMetaFromContext(ctx, sessionIDKey, "session_id")

	cost, priced := EstimateCost(model, int64(input), int64(output))

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	t.ensureMapsLocked()

	a := &t.data.Aggregate
	a.TotalProject.AddCost(input, output, cost)

	addToMap(a.ByProvider, provider, input, output, cost)
	addToMap(a.ByModel, model, input, output, cost)
	addToMap(a.ByShardType, shardType, input, output, cost)
	addToMap(a.ByShardName, shardName, input, output, cost)
	addToMap(a.ByOperation, operation, input, output, cost)
	addToMap(a.BySession, sessionID, input, output, cost)

	if !priced {
		a.UnpricedTokens += int64(input + output)
	}

	t.pruneSessionsLocked()

	if t.keepEvents {
		t.appendEventLocked(UsageEvent{
			Timestamp:     time.Now().UTC(),
			Model:         model,
			Provider:      provider,
			InputTokens:   input,
			OutputTokens:  output,
			ShardType:     shardType,
			ShardName:     shardName,
			SessionID:     sessionID,
			OperationType: operation,
			CostUSD:       cost,
		})
	}

	t.markDirtyLocked()
}

// appendEventLocked pushes onto the bounded event ring. Caller holds mu.
func (t *Tracker) appendEventLocked(ev UsageEvent) {
	t.data.Events = append(t.data.Events, ev)
	if overflow := len(t.data.Events) - maxEvents; overflow > 0 {
		// Copy down rather than reslice: reslicing keeps the whole original
		// backing array alive for the life of the process.
		t.data.Events = append(t.data.Events[:0], t.data.Events[overflow:]...)
	}
}

// pruneSessionsLocked folds the lowest-spend sessions into prunedSessionKey once
// the map exceeds maxSessions. Totals are preserved. Caller holds mu.
func (t *Tracker) pruneSessionsLocked() {
	sessions := t.data.Aggregate.BySession
	if len(sessions) <= maxSessions {
		return
	}

	type row struct {
		key   string
		total int64
	}
	rows := make([]row, 0, len(sessions))
	for k, v := range sessions {
		if k == prunedSessionKey {
			continue
		}
		rows = append(rows, row{k, v.Total})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].total != rows[j].total {
			return rows[i].total < rows[j].total
		}
		return rows[i].key < rows[j].key
	})

	// Keep the map strictly under the cap so we do not prune on every Track.
	target := len(sessions) - (maxSessions * 9 / 10)
	bucket := sessions[prunedSessionKey]
	for i := 0; i < target && i < len(rows); i++ {
		v := sessions[rows[i].key]
		bucket.Input += v.Input
		bucket.Output += v.Output
		bucket.Total += v.Total
		bucket.Cost += v.Cost
		delete(sessions, rows[i].key)
	}
	sessions[prunedSessionKey] = bucket
}

// markDirtyLocked arms the debounced auto-save. Caller holds mu.
func (t *Tracker) markDirtyLocked() {
	t.dirty = true
	if t.autoSaveTimer != nil || t.saving {
		// A flush is already scheduled or running; flushLocked re-arms if
		// mutations arrive while it is in flight.
		return
	}
	t.autoSaveTimer = time.AfterFunc(autoSaveDelay, t.autoSaveFlush)
}

// autoSaveFlush is the timer callback. It clears dirty *before* writing and
// re-arms if a mutation lands during the write, so no mutation is ever both
// unwritten and unscheduled.
func (t *Tracker) autoSaveFlush() {
	t.mu.Lock()
	t.autoSaveTimer = nil
	if t.closed || !t.dirty {
		t.mu.Unlock()
		return
	}
	t.dirty = false
	t.saving = true
	err := t.saveLocked()
	t.saving = false

	if err != nil {
		// The write failed, so the data is still only in memory. Restore dirty
		// and re-arm rather than dropping it.
		t.dirty = true
	}
	reArm := t.dirty && !t.closed && t.autoSaveTimer == nil
	if reArm {
		t.autoSaveTimer = time.AfterFunc(autoSaveDelay, t.autoSaveFlush)
	}
	t.mu.Unlock()

	if err != nil {
		logging.Get(logging.CategorySession).Warn("usage: auto-save failed, will retry: %v", err)
	}
}

// Flush writes pending mutations to disk immediately if there are any.
func (t *Tracker) Flush() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.flushLocked()
}

func (t *Tracker) flushLocked() error {
	if t.autoSaveTimer != nil {
		t.autoSaveTimer.Stop()
		t.autoSaveTimer = nil
	}
	if !t.dirty {
		return nil
	}
	if err := t.saveLocked(); err != nil {
		return err
	}
	t.dirty = false
	return nil
}

// Close flushes any pending mutations and stops the auto-save timer. It is safe
// to call more than once. Hosts must call this on Cortex close / chat shutdown,
// otherwise up to autoSaveDelay of usage is lost on exit.
func (t *Tracker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	err := t.flushLocked()
	t.closed = true
	if err != nil {
		logging.Get(logging.CategorySession).Error("usage: final flush failed, usage data lost: %v", err)
	}
	return err
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
	stats.ByShardName = copyTokenCountsMap(stats.ByShardName)
	return stats
}

// Events returns a copy of the retained raw event ring. It is empty unless the
// tracker was created WithEventLog.
func (t *Tracker) Events() []UsageEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.data.Events) == 0 {
		return nil
	}
	out := make([]UsageEvent, len(t.data.Events))
	copy(out, t.data.Events)
	return out
}

func copyTokenCountsMap(src map[string]TokenCounts) map[string]TokenCounts {
	if src == nil {
		return nil
	}
	dst := make(map[string]TokenCounts, len(src))
	maps.Copy(dst, src)
	return dst
}

func addToMap(m map[string]TokenCounts, key string, input, output int, cost float64) {
	if key == "" {
		key = "unknown"
	}
	entry := m[key]
	entry.AddCost(input, output, cost)
	m[key] = entry
}

// shardMetaFromContext reads shard metadata, preferring the typed key and
// falling back to the legacy raw string key so contexts built by older callers
// still resolve.
func shardMetaFromContext(ctx context.Context, key shardMetaKey, legacy string) string {
	if ctx == nil {
		return "unknown"
	}
	if val, ok := ctx.Value(key).(string); ok && val != "" {
		return val
	}
	//nolint:staticcheck // legacy raw-string key kept for one release for compatibility.
	if val, ok := ctx.Value(legacy).(string); ok && val != "" {
		return val
	}
	return "unknown"
}

// Context Helpers

// NewContext returns a new context carrying the tracker.
func NewContext(ctx context.Context, t *Tracker) context.Context {
	return context.WithValue(ctx, contextKey{}, t)
}

// FromContext retrieves the tracker from the context, or nil if absent.
func FromContext(ctx context.Context) *Tracker {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(contextKey{}).(*Tracker)
	return t
}

// WithShardContext adds shard metadata to the context.
func WithShardContext(ctx context.Context, name, typeName, sessionID string) context.Context {
	ctx = context.WithValue(ctx, shardNameKey, name)
	ctx = context.WithValue(ctx, shardTypeKey, typeName)
	ctx = context.WithValue(ctx, sessionIDKey, sessionID)
	return ctx
}

// TrackFromContext records usage against the tracker carried by ctx, if any.
// Every LLM client should call this rather than reaching for a tracker field,
// so a client used outside a tracked session simply records nothing.
func TrackFromContext(ctx context.Context, model, provider string, input, output int, operation string) {
	if t := FromContext(ctx); t != nil {
		t.Track(ctx, model, provider, input, output, operation)
	}
}
