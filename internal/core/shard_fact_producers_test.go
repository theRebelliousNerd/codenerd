package core

import (
	"errors"
	"testing"

	"codenerd/internal/core/shards"

	"codeberg.org/TauCeti/mangle-go/ast"
	"github.com/stretchr/testify/require"
)

// admitsExactly reports whether some bound alternative of f's Decl accepts
// every argument without the bare-string /name tolerance. A fact the
// tolerance admits lands as a string no /name rule can match.
func admitsExactly(t *testing.T, k *RealKernel, f Fact) {
	t.Helper()
	atom, err := f.ToAtom()
	require.NoError(t, err)
	k.mu.Lock()
	defer k.mu.Unlock()
	pa, err := k.admissionForLocked(atom.Predicate)
	require.NoError(t, err)
	require.NotNil(t, pa, "%s/%d has no Decl", f.Predicate, len(f.Args))
	adapted, err := pa.adapt(atom, f.Args)
	require.NoError(t, err)
	for _, handles := range pa.handles {
		exact := true
		for j, h := range handles {
			c, ok := adapted.Args[j].(ast.Constant)
			if !ok || !h.HasType(c) {
				exact = false
				break
			}
		}
		if exact {
			return
		}
	}
	t.Errorf("%s is admitted only by the bare-string /name tolerance (or not at all): %v", f.Predicate, f.Args)
}

func productionKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.Evaluate())
	return k
}

// The 2026-10-28 dogfood log reported shard_executed/4 arg 1,
// last_shard_execution/3 arg 1 and recent_shard_context/4 arg 0 as bare Go
// strings in /name slots: ResultToFacts passed its shardType parameter
// through unchanged.
func TestResultToFacts_ShardTypeIsAName(t *testing.T) {
	k := productionKernel(t)
	sm := shards.NewShardManager()
	facts := append(
		sm.ResultToFacts("coder-1", "coder", "fix bug", "done", nil),
		sm.ResultToFacts("coder-2", "/coder", "fix bug", "", errors.New("boom"))...,
	)
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Predicate] = true
		admitsExactly(t, k, f)
	}
	for _, p := range []string{"shard_executed", "last_shard_execution", "recent_shard_context"} {
		require.True(t, seen[p], "ResultToFacts emitted no %s", p)
	}
	require.NoError(t, k.AssertBatch(facts))
	rows, err := k.Query("shard_executed")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		require.Equal(t, "/coder", r.Args[1], "both spellings of the type land as the same name")
	}
}

// ToFacts emitted shard_profile/2 against a shard_profile/3 Decl, so the
// specialist rules (campaign_rules.mg, policy/shards.mg) read
// shard_profile(Name, /specialist, _) over a predicate nothing had ever filled.
func TestShardManagerToFacts_ShardProfileMatchesItsDecl(t *testing.T) {
	k := productionKernel(t)
	sm := shards.NewShardManager()
	sm.DefineProfile("coder", shards.DefaultGeneralistConfig("coder"))
	sm.DefineProfile("dbexpert", shards.DefaultSpecialistConfig("dbexpert", "/kb/db.sqlite"))

	facts := sm.ToFacts()
	require.Len(t, facts, 2)
	for _, f := range facts {
		require.Len(t, f.Args, 3)
		admitsExactly(t, k, f)
	}
	require.NoError(t, k.AssertBatch(facts))

	rows, err := k.Query("shard_profile")
	require.NoError(t, err)
	kinds := map[string]string{}
	for _, r := range rows {
		kinds[r.Args[0].(string)] = r.Args[1].(string)
	}
	require.Equal(t, map[string]string{"coder": "/ephemeral", "dbexpert": "/specialist"}, kinds)
}
