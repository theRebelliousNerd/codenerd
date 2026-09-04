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

	"codenerd/internal/atomicfile"
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

// tracker is the shared recording/persistence state. It is never handed to a
// caller directly: callers hold a *Tracker handle over it (see Tracker), which
// is what makes one owner's Close independent of another's.
type tracker struct {
	mu       sync.Mutex
	data     UsageData
	filePath string

	baseline AggregatedStats // aggregates as of the last successful sync with disk; see saveLocked. It is the snapshot used to compute this process's own contribution, so a concurrent writer's totals are merged rather than overwritten.

	// dirty means "there are mutations not yet on disk". saving means "a flush
	// is in flight". Both are needed: the old code cleared dirty after Save
	// returned, so any Track that landed during the write saw dirty==true,
	// skipped re-arming the timer, and was then marked clean — losing the
	// mutation until some later Track happened to re-arm.
	dirty  bool
	saving bool

	autoSaveTimer *time.Timer
	closed        bool

	// refs counts live owners. A tracker handed out by Shared is owned by every
	// caller that asked for it (Cortex and the chat model, typically), and the
	// first Close must not stop metering for the others. NewTracker starts at 1
	// so a privately constructed tracker closes on its first Close.
	refs int

	// sharedKey is the registry key when this tracker came from Shared, so the
	// final Close can unregister it. Empty for private trackers.
	sharedKey string

	// keepEvents enables the raw event ring. Off by default: most operators
	// only want aggregates, and the ring roughly doubles usage.json size.
	keepEvents bool
}

// sharedTrackers maps an absolute usage.json path to the one Tracker allowed to
// own it in this process.
//
// Two trackers over the same file is not a cosmetic duplication: each holds its
// own in-memory aggregates loaded at construction time, and each atomic save
// replaces the file wholesale — so whichever flushes last silently erases every
// token the other counted. Cortex and the interactive chat model both used to
// call NewTracker on the same workspace, which is exactly that bug.
var (
	sharedMu       sync.Mutex
	sharedTrackers = make(map[string]*tracker)
)

// Shared returns the process-wide tracker for workspacePath, creating it on
// first use and handing back the same instance afterwards. Every owner must
// Close its handle; only the last Close flushes and shuts the tracker down.
//
// Options are honored only when the tracker is created; a later caller joins
// the existing tracker as-is rather than reconfiguring a tracker other owners
// are already using.
func Shared(workspacePath string, opts ...Option) (*Tracker, error) {
	key, err := filepath.Abs(workspacePath)
	if err != nil {
		key = workspacePath
	}

	sharedMu.Lock()
	defer sharedMu.Unlock()

	if t, ok := sharedTrackers[key]; ok {
		t.mu.Lock()
		if !t.closed {
			t.refs++
			t.mu.Unlock()
			// A fresh handle, not the one the previous owner holds: each owner
			// must be able to release exactly its own reference.
			return &Tracker{tracker: t}, nil
		}
		t.mu.Unlock()
		// A fully closed tracker cannot record anything; replace it.
		delete(sharedTrackers, key)
	}

	h, err := NewTracker(workspacePath, opts...)
	if err != nil {
		return nil, err
	}
	h.sharedKey = key
	sharedTrackers[key] = h.tracker
	return h, nil
}

// releaseShared drops key from the registry if it still maps to t.
func releaseShared(key string, t *tracker) {
	if key == "" {
		return
	}
	sharedMu.Lock()
	if sharedTrackers[key] == t {
		delete(sharedTrackers, key)
	}
	sharedMu.Unlock()
}

// Tracker is one owner's handle on a usage tracker.
//
// It embeds the shared state, so every recording method (Track, Stats, Flush,
// …) reads and writes the one set of aggregates — which is the whole point of
// Shared: two trackers over one usage.json would each hold their own in-memory
// totals and the last flush would erase the other's.
//
// Close is the exception, and the reason a handle exists at all. Ownership was
// previously counted on the shared state itself, with every owner holding the
// same pointer — so the count could not tell "owner A closing a second time"
// from "owner B closing for the first time". A caller with the ordinary
// `defer tracker.Close()` plus an explicit shutdown call therefore consumed a
// reference belonging to someone else, and the next Close shut the tracker
// down while a live owner was still metering into it: every subsequent Track
// was silently dropped. A handle can answer that question, because releasing
// is a property of the handle rather than of the shared state, and closeOnce
// makes a repeat Close by the same owner the no-op the doc comment always
// claimed it was.
type Tracker struct {
	*tracker

	closeOnce sync.Once
	closeErr  error
}

// Close releases this owner's handle, and only this owner's. Calling it more
// than once is a genuine no-op: the second call returns the first call's error
// without touching the shared reference count.
//
// The last handle to close flushes and shuts the tracker down; earlier ones
// flush what is pending and leave metering running for whoever still holds it.
// Hosts must call this on Cortex close / chat shutdown, otherwise up to
// autoSaveDelay of usage is lost on exit.
func (h *Tracker) Close() error {
	h.closeOnce.Do(func() { h.closeErr = h.tracker.release() })
	return h.closeErr
}

// Option configures a Tracker.
type Option func(*tracker)

// WithEventLog enables retention of the most recent maxEvents raw events.
func WithEventLog() Option {
	return func(t *tracker) { t.keepEvents = true }
}

// NewTracker creates a new usage tracker using the specified workspace persistence path.
func NewTracker(workspacePath string, opts ...Option) (*Tracker, error) {
	nerdDir := filepath.Join(workspacePath, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .nerd dir: %w", err)
	}

	filePath := filepath.Join(nerdDir, "usage.json")
	t := &tracker{
		filePath: filePath,
		data:     newUsageData(),
		refs:     1,
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

	return &Tracker{tracker: t}, nil
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
// Load reads the usage data from disk.
func (t *tracker) Load() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(t.filePath)
	if os.IsNotExist(err) {
		t.baseline = cloneAggregates(t.data.Aggregate)
		return nil
	}
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &t.data); err != nil {
		// Leave the tracker on fresh aggregates rather than a half-populated
		// struct from a partial unmarshal.
		t.data = newUsageData()
		t.baseline = cloneAggregates(t.data.Aggregate)
		return fmt.Errorf("parse %s: %w", t.filePath, err)
	}

	t.ensureMapsLocked()
	t.baseline = cloneAggregates(t.data.Aggregate)
	return nil
}

// ensureMapsLocked initializes any nil aggregate map. Caller holds mu.
func (t *tracker) ensureMapsLocked() {
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

func cloneAggregates(a AggregatedStats) AggregatedStats {
	out := AggregatedStats{
		TotalProject:   a.TotalProject,
		UnpricedTokens: a.UnpricedTokens,
		ByProvider:     copyTokenCountsMap(a.ByProvider),
		ByModel:        copyTokenCountsMap(a.ByModel),
		ByShardType:    copyTokenCountsMap(a.ByShardType),
		ByOperation:    copyTokenCountsMap(a.ByOperation),
		BySession:      copyTokenCountsMap(a.BySession),
		ByShardName:    copyTokenCountsMap(a.ByShardName),
	}
	if out.ByProvider == nil {
		out.ByProvider = make(map[string]TokenCounts)
	}
	if out.ByModel == nil {
		out.ByModel = make(map[string]TokenCounts)
	}
	if out.ByShardType == nil {
		out.ByShardType = make(map[string]TokenCounts)
	}
	if out.ByOperation == nil {
		out.ByOperation = make(map[string]TokenCounts)
	}
	if out.BySession == nil {
		out.BySession = make(map[string]TokenCounts)
	}
	if out.ByShardName == nil {
		out.ByShardName = make(map[string]TokenCounts)
	}
	return out
}

func addTokenCounts(dst *TokenCounts, delta TokenCounts) {
	dst.Input += delta.Input
	dst.Output += delta.Output
	dst.Total += delta.Total
	dst.Cost += delta.Cost
}

func contributionSince(current, baseline AggregatedStats) AggregatedStats {
	diff := func(cur, base TokenCounts) TokenCounts {
		var d TokenCounts
		// Never emit negative values: if a subtraction would go below zero
		// (which can only happen if the file was truncated or reset underneath
		// us), clamp that component to zero and treat current as the
		// contribution so we don't discard the tokens this process just
		// counted.
		if cur.Input >= base.Input {
			d.Input = cur.Input - base.Input
		} else {
			d.Input = cur.Input
		}
		if cur.Output >= base.Output {
			d.Output = cur.Output - base.Output
		} else {
			d.Output = cur.Output
		}
		if cur.Total >= base.Total {
			d.Total = cur.Total - base.Total
		} else {
			d.Total = cur.Total
		}
		if cur.Cost >= base.Cost {
			d.Cost = cur.Cost - base.Cost
		} else {
			d.Cost = cur.Cost
		}
		return d
	}

	out := AggregatedStats{
		ByProvider:  make(map[string]TokenCounts, len(current.ByProvider)),
		ByModel:     make(map[string]TokenCounts, len(current.ByModel)),
		ByShardType: make(map[string]TokenCounts, len(current.ByShardType)),
		ByOperation: make(map[string]TokenCounts, len(current.ByOperation)),
		BySession:   make(map[string]TokenCounts, len(current.BySession)),
		ByShardName: make(map[string]TokenCounts, len(current.ByShardName)),
	}
	out.TotalProject = diff(current.TotalProject, baseline.TotalProject)
	if current.UnpricedTokens >= baseline.UnpricedTokens {
		out.UnpricedTokens = current.UnpricedTokens - baseline.UnpricedTokens
	} else {
		// Same clamp as above: file truncated, treat current as contribution.
		out.UnpricedTokens = current.UnpricedTokens
	}

	for k, curVal := range current.ByProvider {
		if baseVal, ok := baseline.ByProvider[k]; ok {
			out.ByProvider[k] = diff(curVal, baseVal)
		} else {
			out.ByProvider[k] = curVal
		}
	}
	for k, curVal := range current.ByModel {
		if baseVal, ok := baseline.ByModel[k]; ok {
			out.ByModel[k] = diff(curVal, baseVal)
		} else {
			out.ByModel[k] = curVal
		}
	}
	for k, curVal := range current.ByShardType {
		if baseVal, ok := baseline.ByShardType[k]; ok {
			out.ByShardType[k] = diff(curVal, baseVal)
		} else {
			out.ByShardType[k] = curVal
		}
	}
	for k, curVal := range current.ByOperation {
		if baseVal, ok := baseline.ByOperation[k]; ok {
			out.ByOperation[k] = diff(curVal, baseVal)
		} else {
			out.ByOperation[k] = curVal
		}
	}
	for k, curVal := range current.BySession {
		if baseVal, ok := baseline.BySession[k]; ok {
			out.BySession[k] = diff(curVal, baseVal)
		} else {
			out.BySession[k] = curVal
		}
	}
	for k, curVal := range current.ByShardName {
		if baseVal, ok := baseline.ByShardName[k]; ok {
			out.ByShardName[k] = diff(curVal, baseVal)
		} else {
			out.ByShardName[k] = curVal
		}
	}
	return out
}

func mergeAggregates(base, delta AggregatedStats) AggregatedStats {
	out := cloneAggregates(base)
	addTokenCounts(&out.TotalProject, delta.TotalProject)
	out.UnpricedTokens += delta.UnpricedTokens

	if out.ByProvider == nil {
		out.ByProvider = make(map[string]TokenCounts)
	}
	for k, v := range delta.ByProvider {
		cur := out.ByProvider[k]
		addTokenCounts(&cur, v)
		out.ByProvider[k] = cur
	}
	if out.ByModel == nil {
		out.ByModel = make(map[string]TokenCounts)
	}
	for k, v := range delta.ByModel {
		cur := out.ByModel[k]
		addTokenCounts(&cur, v)
		out.ByModel[k] = cur
	}
	if out.ByShardType == nil {
		out.ByShardType = make(map[string]TokenCounts)
	}
	for k, v := range delta.ByShardType {
		cur := out.ByShardType[k]
		addTokenCounts(&cur, v)
		out.ByShardType[k] = cur
	}
	if out.ByOperation == nil {
		out.ByOperation = make(map[string]TokenCounts)
	}
	for k, v := range delta.ByOperation {
		cur := out.ByOperation[k]
		addTokenCounts(&cur, v)
		out.ByOperation[k] = cur
	}
	if out.BySession == nil {
		out.BySession = make(map[string]TokenCounts)
	}
	for k, v := range delta.BySession {
		cur := out.BySession[k]
		addTokenCounts(&cur, v)
		out.BySession[k] = cur
	}
	if out.ByShardName == nil {
		out.ByShardName = make(map[string]TokenCounts)
	}
	for k, v := range delta.ByShardName {
		cur := out.ByShardName[k]
		addTokenCounts(&cur, v)
		out.ByShardName[k] = cur
	}
	return out
}

// Save writes the usage data to disk.
func (t *tracker) Save() error {
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
// saveLocked serializes and atomically replaces usage.json. Caller holds mu.
//
// The previous implementation used os.WriteFile, which truncates in place: a
// crash or a full disk mid-write left a truncated, unparseable usage.json and
// the whole history was gone. Writing a sibling temp file and renaming means a
// reader sees either the old file or the new one, never a partial one.
func (t *tracker) saveLocked() error {
	// Cross-process coordination: read-merge-write under an advisory file lock.
	// Each process keeps a full UsageData in memory and saveLocked previously
	// wrote that whole struct, so two processes on one workspace overwrote each
	// other (last-writer-wins). A lock alone does not fix this because each
	// process would still write its own complete copy; the write must become
	// read-merge-write. Every aggregate is an additive counter, so onDisk +
	// (current - baseline) is exactly the right answer. Track a baseline
	// snapshot so a concurrent writer's totals are merged rather than overwritten.
	lockPath := t.filePath + ".lock"
	var lk *fileLock
	if l, err := acquireFileLock(lockPath); err != nil {
		logging.Get(logging.CategorySession).Debug("usage: could not acquire cross-process lock %s: %v", lockPath, err)
	} else {
		lk = l
		defer func() {
			if err := lk.release(); err != nil {
				logging.Get(logging.CategorySession).Debug("usage: failed to release cross-process lock %s: %v", lockPath, err)
			}
		}()
		// With the lock held, read the current on-disk aggregates.
		// The Events ring is explicitly documented as non-exhaustive with
		// aggregates as the durable record, so do NOT attempt to merge event
		// rings across processes. Keep writing this process's own Events
		// exactly as today.
		var onDiskAgg AggregatedStats
		haveOnDisk := false
		parseFailed := false
		if raw, err := os.ReadFile(t.filePath); err == nil {
			var onDisk UsageData
			if err := json.Unmarshal(raw, &onDisk); err != nil {
				logging.Get(logging.CategorySession).Warn("usage: could not parse %s, writing own data without merge: %v", t.filePath, err)
				parseFailed = true
			} else {
				onDiskAgg = onDisk.Aggregate
				haveOnDisk = true
			}
		} else if os.IsNotExist(err) {
			// No file yet; treat on-disk aggregates as empty. mergeAggregates
			// handles nil maps in base.
			haveOnDisk = true
		} else {
			logging.Get(logging.CategorySession).Warn("usage: could not read %s for merge, writing own data without merge: %v", t.filePath, err)
			parseFailed = true
		}
		if haveOnDisk && !parseFailed {
			contribution := contributionSince(t.data.Aggregate, t.baseline)
			merged := mergeAggregates(onDiskAgg, contribution)
			t.data.Aggregate = merged
		}
	}

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
	if err := atomicfile.Replace(tmpName, t.filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename usage file into place: %w", err)
	}

	t.baseline = cloneAggregates(t.data.Aggregate)
	return nil
}

// Track records a new usage event.
//
// Negative token counts are rejected: they can only come from a provider bug or
// a bad cast, and letting them through silently corrupts every aggregate they
// touch with no way to reconstruct the truth.
func (t *tracker) Track(ctx context.Context, model, provider string, input, output int, operation string) {
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
func (t *tracker) appendEventLocked(ev UsageEvent) {
	t.data.Events = append(t.data.Events, ev)
	if overflow := len(t.data.Events) - maxEvents; overflow > 0 {
		// Copy down rather than reslice: reslicing keeps the whole original
		// backing array alive for the life of the process.
		t.data.Events = append(t.data.Events[:0], t.data.Events[overflow:]...)
	}
}

// pruneSessionsLocked folds the lowest-spend sessions into prunedSessionKey once
// the map exceeds maxSessions. Totals are preserved. Caller holds mu.
func (t *tracker) pruneSessionsLocked() {
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
func (t *tracker) markDirtyLocked() {
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
func (t *tracker) autoSaveFlush() {
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
func (t *tracker) Flush() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.flushLocked()
}

func (t *tracker) flushLocked() error {
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

// release drops one reference. It is called exactly once per handle, from
// Tracker.Close under that handle's sync.Once — which is what makes the
// reference count trustworthy. Do not call it from anywhere else: an unbalanced
// release here shuts the tracker down under a live owner, which is the bug the
// handle type exists to prevent.
func (t *tracker) release() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	if t.refs > 1 {
		t.refs--
		err := t.flushLocked()
		t.mu.Unlock()
		if err != nil {
			logging.Get(logging.CategorySession).Warn("usage: flush on handle release failed: %v", err)
		}
		return err
	}

	err := t.flushLocked()
	t.refs = 0
	t.closed = true
	key := t.sharedKey
	t.mu.Unlock()

	releaseShared(key, t)

	if err != nil {
		logging.Get(logging.CategorySession).Error("usage: final flush failed, usage data lost: %v", err)
	}
	return err
}

// Stats returns a copy of the aggregated stats.
func (t *tracker) Stats() AggregatedStats {
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
func (t *tracker) Events() []UsageEvent {
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

// WithSessionID tags ctx with the session identity only, leaving any shard
// name/type already on the context untouched. The session executor uses it so
// a turn's spend lands in BySession under its own ID instead of "unknown",
// which is what a per-turn cost delta must read: TotalProject is merged across
// processes and moves whenever another nerd process spends tokens.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionTokens returns the accumulated counts recorded under sessionID, or
// zero counts when the session has no entry yet.
func (h *Tracker) SessionTokens(sessionID string) TokenCounts {
	if h == nil || h.tracker == nil {
		return TokenCounts{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensureMapsLocked()
	return h.data.Aggregate.BySession[sessionID]
}

// TrackFromContext records usage against the tracker carried by ctx, if any.
// Every LLM client should call this rather than reaching for a tracker field,
// so a client used outside a tracked session simply records nothing.
func TrackFromContext(ctx context.Context, model, provider string, input, output int, operation string) {
	if t := FromContext(ctx); t != nil {
		t.Track(ctx, model, provider, input, output, operation)
	}
}
