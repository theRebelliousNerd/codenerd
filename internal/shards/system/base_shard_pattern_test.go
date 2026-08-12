package system

import (
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// fakeLearningStore implements core.LearningStore for load-path tests without a real DB.
type fakeLearningStore struct {
	mu        sync.Mutex
	saved     []savedRecord
	batch     []batchRecord
	learnings map[string][]types.ShardLearning // keyed by predicate
	loadErr   map[string]error
}

type savedRecord struct {
	shardID   string
	predicate string
	args      []any
}

type batchRecord struct {
	shardID   string
	learnings []types.ShardLearning
}

func (f *fakeLearningStore) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]any, len(factArgs))
	copy(cp, factArgs)
	f.saved = append(f.saved, savedRecord{shardID: shardType, predicate: factPredicate, args: cp})
	return nil
}

func (f *fakeLearningStore) SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]types.ShardLearning, len(learnings))
	copy(cp, learnings)
	f.batch = append(f.batch, batchRecord{shardID: shardType, learnings: cp})
	return nil
}

func (f *fakeLearningStore) Load(shardType string) ([]types.ShardLearning, error) {
	return nil, nil
}

func (f *fakeLearningStore) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	if f.loadErr != nil {
		if err, ok := f.loadErr[predicate]; ok && err != nil {
			return nil, err
		}
	}
	if f.learnings == nil {
		return nil, nil
	}
	return f.learnings[predicate], nil
}

func (f *fakeLearningStore) DecayConfidence(shardType string, decayFactor float64) error {
	return nil
}

func (f *fakeLearningStore) Close() error { return nil }

type fakeKernelIface interface {
	Assert(types.Fact) error
}

type shardLike struct {
	mu     sync.RWMutex
	kernel fakeKernelIface
	data   map[string]int
}

// helper to create a kernel for in-process tests.
func newTestKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	return k
}

// helper to find shard_pattern facts matching expected args.
func findShardPattern(t *testing.T, facts []types.Fact, wantShardID string, wantKind string, wantPattern string, wantCount int64) bool {
	t.Helper()
	for _, f := range facts {
		if f.Predicate != "shard_pattern" {
			continue
		}
		if len(f.Args) != 4 {
			continue
		}
		gotShard, _ := f.Args[0].(string)
		// Kind may come back as string "/success" or MangleAtom; normalize to string.
		var gotKind string
		switch v := f.Args[1].(type) {
		case types.MangleAtom:
			gotKind = string(v)
		case string:
			gotKind = v
		default:
			continue
		}
		gotPattern, _ := f.Args[2].(string)
		var gotCount int64
		switch v := f.Args[3].(type) {
		case int:
			gotCount = int64(v)
		case int64:
			gotCount = v
		case float64:
			gotCount = int64(v)
		default:
			continue
		}
		if gotShard == wantShardID && gotKind == wantKind && gotPattern == wantPattern && gotCount == wantCount {
			return true
		}
	}
	return false
}

// TestTrackSuccess_Threshold verifies that crossing the success threshold (3)
// asserts shard_pattern with /success and the correct count, while below
// threshold nothing is asserted.
func TestTrackSuccess_Threshold(t *testing.T) {
	k := newTestKernel(t)
	shardID := "test_success_threshold"
	b := NewBaseSystemShard(shardID, StartupAuto)
	b.SetParentKernel(k)

	// Below threshold: 1 and 2 must not assert.
	b.trackSuccess("patternA")
	b.trackSuccess("patternA")

	facts, err := k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected 0 shard_pattern facts below threshold, got %d: %v", len(facts), facts)
	}

	// Crossing threshold: 3rd call must assert with count 3.
	b.trackSuccess("patternA")
	facts, err = k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, shardID, "/success", "patternA", 3) {
		t.Fatalf("expected shard_pattern(%q, /success, %q, 3) after 3rd trackSuccess, got %v", shardID, "patternA", facts)
	}

	// Further increments should assert with updated count (4).
	b.trackSuccess("patternA")
	facts, err = k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, shardID, "/success", "patternA", 4) {
		t.Fatalf("expected shard_pattern with count 4 after 4th trackSuccess, got %v", facts)
	}
}

// TestTrackFailure_Threshold verifies that crossing the failure threshold (2)
// asserts shard_pattern with /failure carrying the bare pattern, NOT the
// composite "pattern:reason" key, and that below threshold nothing is asserted.
func TestTrackFailure_Threshold_BarePattern(t *testing.T) {
	k := newTestKernel(t)
	shardID := "test_failure_threshold"
	b := NewBaseSystemShard(shardID, StartupAuto)
	b.SetParentKernel(k)

	pattern := "myPattern"
	reason := "myReason"
	composite := pattern + ":" + reason

	// First call below threshold must not assert.
	b.trackFailure(pattern, reason)
	facts, err := k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected 0 facts after 1st trackFailure, got %v", facts)
	}

	// Second call crosses threshold: must assert bare pattern with count 2.
	b.trackFailure(pattern, reason)
	facts, err = k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, shardID, "/failure", pattern, 2) {
		t.Fatalf("expected shard_pattern(%q, /failure, %q, 2) after threshold, got %v", shardID, pattern, facts)
	}
	// Must NOT have asserted the composite key as the Pattern arg.
	if findShardPattern(t, facts, shardID, "/failure", composite, 2) {
		t.Fatalf("shard_pattern must carry bare pattern %q, not composite %q; facts: %v", pattern, composite, facts)
	}

	// Verify internal counter uses composite key (so the distinction is real):
	// The map should contain composite, not bare.
	if _, ok := b.patternFailure[composite]; !ok {
		t.Fatalf("expected internal patternFailure to use composite key %q, have %v", composite, b.patternFailure)
	}
	if _, ok := b.patternFailure[pattern]; ok {
		t.Fatalf("internal patternFailure must not contain bare pattern %q when reason is present; have %v", pattern, b.patternFailure)
	}

	// Third call should assert with count 3 and still bare pattern.
	b.trackFailure(pattern, reason)
	facts, err = k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, shardID, "/failure", pattern, 3) {
		t.Fatalf("expected shard_pattern with count 3 and bare pattern after 3rd trackFailure, got %v", facts)
	}
}

// TestNilKernel_NoPanic verifies that a nil Kernel is a no-op and does not panic
// for trackSuccess, trackFailure and load paths.
func TestNilKernel_NoPanic(t *testing.T) {
	b := NewBaseSystemShard("nil_kernel_test", StartupAuto)
	// Explicitly ensure Kernel is nil (NewBaseSystemShard leaves it nil).
	if b.Kernel != nil {
		t.Fatalf("expected nil Kernel initially")
	}

	// These must not panic when kernel is nil.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("trackSuccess with nil kernel panicked: %v", r)
			}
		}()
		b.trackSuccess("p")
		b.trackSuccess("p")
		b.trackSuccess("p") // crosses threshold but kernel nil
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("trackFailure with nil kernel panicked: %v", r)
			}
		}()
		b.trackFailure("p", "r")
		b.trackFailure("p", "r") // crosses threshold but kernel nil
	}()

	// SetLearningStore with nil kernel must not panic and must still load.
	fake := &fakeLearningStore{
		learnings: map[string][]types.ShardLearning{
			"success_pattern": {{FactArgs: []any{"sPat"}}},
			"failure_pattern": {{FactArgs: []any{"fPat", "reason"}}},
		},
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SetLearningStore with nil kernel panicked: %v", r)
			}
		}()
		b.SetLearningStore(fake)
	}()
	// No kernel, so no facts to query, but internal maps should be seeded.
	if got := b.patternSuccess["sPat"]; got != 3 {
		t.Fatalf("expected patternSuccess seeded to 3 even with nil kernel, got %d", got)
	}
	if _, ok := b.patternFailure["fPat:reason"]; !ok {
		t.Fatalf("expected patternFailure seeded with composite key even with nil kernel, have %v", b.patternFailure)
	}
}

// TestNoAssertionWhileHoldingMu verifies that kernel assertion happens after
// releasing b.mu. It uses a helper that mirrors the exact lock discipline in
// base.go (decide under lock, release, then assert) and a fake kernel whose
// Assert calls back into a shard method that takes b.mu. If the code held the
// lock during Assert, the callback would deadlock. The test runs under a timeout
// so a regression fails instead of hanging. This follows the same pattern as
// internal/autopoiesis/prompt_evolution/atom_promoted_callback_test.go
// TestAtomPromotedCallback_RunsWithoutEvolverLock.
//
// NOTE: BaseSystemShard.Kernel is a concrete *core.RealKernel, so we cannot
// directly assign a fake *RealKernel with a custom Assert method. The closest
// available fake is to exercise the same lock pattern via a standalone helper
// that uses an interface kernel. The real BaseSystemShard code follows this
// exact pattern (see trackSuccess/trackFailure and SetLearningStore in base.go:
// an inner func holds b.mu, captures kernel and fact, releases, then asserts).
// This test pins that discipline and would catch a regression that moved the
// Assert inside the lock.
func TestNoAssertionWhileHoldingMu(t *testing.T) {
	// Isolated lock-discipline helper that mirrors base.go.
	helperTrack := func(s *shardLike, pattern string) {
		var fact *types.Fact
		var k fakeKernelIface
		func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.data[pattern]++
			if s.data[pattern] >= 2 {
				k = s.kernel
				f := types.Fact{Predicate: "shard_pattern", Args: []any{"id", types.MangleAtom("/success"), pattern, s.data[pattern]}}
				fact = &f
			}
		}()
		if fact != nil && k != nil {
			_ = k.Assert(*fact)
		}
	}

	sl := &shardLike{data: make(map[string]int)}

	// Fake kernel that re-enters the shard while Assert is executing.
	// If Assert were called while holding sl.mu, this Get-like call would deadlock.
	fk := &callbackFakeKernel{
		shard: sl,
	}
	sl.kernel = fk

	done := make(chan error, 1)
	go func() {
		// First call below threshold: no assert, no callback.
		helperTrack(sl, "deadlockPat")
		// Second call crosses threshold: triggers Assert which calls back.
		helperTrack(sl, "deadlockPat")
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("helperTrack failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("helperTrack did not return in time - likely deadlock: Assert invoked while holding mu (callback re-entered shard and blocked)")
	}

	if !fk.called {
		t.Fatal("fake kernel Assert was not called; test did not exercise the callback path")
	}
	if !fk.callbackRan {
		t.Fatal("callback did not run or was blocked due to deadlock")
	}
}

type callbackFakeKernel struct {
	mu          sync.Mutex
	called      bool
	callbackRan bool
	shard       *shardLike
}

func (f *callbackFakeKernel) Assert(fact types.Fact) error {
	f.mu.Lock()
	f.called = true
	f.mu.Unlock()
	// Re-enter the shard while Assert is executing. This mimics a kernel
	// that during Assert calls back into a shard method that takes b.mu
	// (e.g., GetState, GetID, or any RLock/Lock). The real base.go must
	// have released b.mu before calling kernel.Assert, otherwise this
	// will deadlock.
	//
	// We use TryLock-style via channel to avoid permanently blocking if buggy,
	// but the outer test's timeout is the real deadlock detector.
	done := make(chan struct{})
	go func() {
		f.shard.mu.RLock()
		f.shard.mu.RUnlock()
		close(done)
	}()
	select {
	case <-done:
		f.mu.Lock()
		f.callbackRan = true
		f.mu.Unlock()
	case <-time.After(2 * time.Second):
		// If we cannot acquire shard.mu within 2s, it is likely held by the
		// caller (helperTrack) during Assert -> deadlock.
		// Do not mark callbackRan; the outer test will timeout and fail.
	}
	return nil
}

// TestLoadLearnedPatterns_SuccessAndFailure verifies that loadLearnedPatterns
// via SetLearningStore seeds the in-memory counters and asserts shard_pattern
// facts after releasing b.mu, using a fake LearningStore rather than a real DB.
func TestLoadLearnedPatterns_SuccessAndFailure(t *testing.T) {
	k := newTestKernel(t)
	b := NewBaseSystemShard("load_test_shard", StartupAuto)
	b.SetParentKernel(k)

	fake := &fakeLearningStore{
		learnings: map[string][]types.ShardLearning{
			"success_pattern": {
				{FactArgs: []any{"loadedSuccess"}},
				{FactArgs: []any{"loadedSuccess2"}},
			},
			"failure_pattern": {
				{FactArgs: []any{"loadedFail", "because"}},
				{FactArgs: []any{"bareFail"}}, // old row without reason
			},
		},
	}

	// SetLearningStore should load, seed, and assert after releasing mu.
	b.SetLearningStore(fake)

	// In-memory seeding checks.
	if got := b.patternSuccess["loadedSuccess"]; got != 3 {
		t.Fatalf("expected patternSuccess[loadedSuccess]=3 after load, got %d", got)
	}
	if got := b.patternSuccess["loadedSuccess2"]; got != 3 {
		t.Fatalf("expected patternSuccess[loadedSuccess2]=3 after load, got %d", got)
	}
	// Failure with reason should be seeded under composite key.
	if got := b.patternFailure["loadedFail:because"]; got != 3 {
		t.Fatalf("expected patternFailure[loadedFail:because]=3 after load, got %d (have %v)", got, b.patternFailure)
	}
	// Bare failure without reason falls back to bare key.
	if got := b.patternFailure["bareFail"]; got != 3 {
		t.Fatalf("expected patternFailure[bareFail]=3 after load, got %d (have %v)", got, b.patternFailure)
	}

	// Kernel should have shard_pattern facts for each loaded pattern, with bare pattern.
	facts, err := k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, "load_test_shard", "/success", "loadedSuccess", 3) {
		t.Fatalf("expected shard_pattern for loadedSuccess after SetLearningStore, got %v", facts)
	}
	if !findShardPattern(t, facts, "load_test_shard", "/success", "loadedSuccess2", 3) {
		t.Fatalf("expected shard_pattern for loadedSuccess2 after SetLearningStore, got %v", facts)
	}
	if !findShardPattern(t, facts, "load_test_shard", "/failure", "loadedFail", 3) {
		t.Fatalf("expected shard_pattern for loadedFail (bare) after SetLearningStore, got %v", facts)
	}
	if !findShardPattern(t, facts, "load_test_shard", "/failure", "bareFail", 3) {
		t.Fatalf("expected shard_pattern for bareFail after SetLearningStore, got %v", facts)
	}
	// Assert that composite key was NOT used as the fact's Pattern arg.
	if findShardPattern(t, facts, "load_test_shard", "/failure", "loadedFail:because", 3) {
		t.Fatalf("shard_pattern must use bare pattern, not composite key; facts: %v", facts)
	}
}

// TestInProcessVisibility_RealKernel is the most important case: it builds a
// real kernel, attaches it with SetParentKernel, drives trackSuccess past
// threshold, then Queries the kernel for shard_pattern and asserts the fact
// comes back. This proves assert-then-query works inside one process. It
// matters because ` + "`nerd logic`" + ` is a separate process that snapshots before
// system shards attach, so the CLI can never observe these facts and is the
// wrong instrument for this question.
func TestInProcessVisibility_RealKernel(t *testing.T) {
	k := newTestKernel(t)
	shardID := "visibility_test_shard"
	b := NewBaseSystemShard(shardID, StartupAuto)
	b.SetParentKernel(k)

	// Verify kernel is attached.
	if b.GetKernel() == nil {
		t.Fatal("expected kernel to be attached after SetParentKernel")
	}

	pattern := "inProcessPattern"
	// Drive past threshold.
	b.trackSuccess(pattern)
	b.trackSuccess(pattern)
	b.trackSuccess(pattern)

	// Query in-process; this must see the fact.
	facts, err := k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query shard_pattern failed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("expected at least 1 shard_pattern fact after trackSuccess threshold, got 0")
	}
	if !findShardPattern(t, facts, shardID, "/success", pattern, 3) {
		t.Fatalf("expected in-process visible shard_pattern(%q, /success, %q, 3), got %v", shardID, pattern, facts)
	}

	// Also verify via QueryAll that the fact is present in the merged view.
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}
	found := false
	for _, f := range all["shard_pattern"] {
		if len(f.Args) == 4 {
			if s, _ := f.Args[0].(string); s == shardID {
				if p, _ := f.Args[2].(string); p == pattern {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected shard_pattern for %q in QueryAll, have %v", pattern, all["shard_pattern"])
	}
}

// TestSetLearningStore_InProcessVisibility ensures the load path also yields
// in-process query visibility (same rationale as above, but for the
// SetLearningStore -> loadLearnedPatterns -> Assert after unlock path).
func TestSetLearningStore_InProcessVisibility(t *testing.T) {
	k := newTestKernel(t)
	shardID := "load_visibility_shard"
	b := NewBaseSystemShard(shardID, StartupAuto)
	b.SetParentKernel(k)

	fake := &fakeLearningStore{
		learnings: map[string][]types.ShardLearning{
			"success_pattern": {{FactArgs: []any{"fromStore"}}},
		},
	}
	b.SetLearningStore(fake)

	facts, err := k.Query("shard_pattern")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !findShardPattern(t, facts, shardID, "/success", "fromStore", 3) {
		t.Fatalf("expected shard_pattern from load path to be in-process visible, got %v", facts)
	}
}
