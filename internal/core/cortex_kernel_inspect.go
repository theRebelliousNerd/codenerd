package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeberg.org/TauCeti/mangle-go/provenance"

	"codenerd/internal/mangle"
)

// Inspection on the Cortex: /why, /explain and provenance read the shards a
// Query of the same predicate reads (readShards). A derivation happens inside
// one shard -- no rule sees another shard's store -- so each shard's trace or
// proof is a whole derivation, and the Cortex's answer is theirs merged.
// Until 2026-09-23 these existed only on RealKernel, and the chat session ran
// on the catch-all shard's kernel to reach them.

// TraceQuery traces query in every shard a Query of it reads and merges the
// traces: one root per derived fact, deduplicated across shards as Query
// deduplicates its rows.
func (c *CortexKernel) TraceQuery(ctx context.Context, query string) (*mangle.DerivationTrace, error) {
	start := time.Now()
	shards, _, err := c.readShards(query)
	if err != nil {
		return nil, err
	}
	merged := &mangle.DerivationTrace{
		Query:     query,
		RootNodes: make([]*mangle.DerivationNode, 0),
		AllNodes:  make([]*mangle.DerivationNode, 0),
		Timestamp: start,
	}
	seen := make(map[string]struct{})
	for _, s := range shards {
		trace, err := s.kernel.TraceQuery(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("[cortex] trace in shard %s: %w", s.domain, err)
		}
		for _, root := range trace.RootNodes {
			key := factKey(Fact{Predicate: root.Fact.Predicate, Args: root.Fact.Args})
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			merged.RootNodes = append(merged.RootNodes, root)
			merged.AllNodes = append(merged.AllNodes, derivationSubtree(root)...)
		}
	}
	merged.Duration = time.Since(start)
	return merged, nil
}

// derivationSubtree lists a node and every node under it.
func derivationSubtree(node *mangle.DerivationNode) []*mangle.DerivationNode {
	nodes := []*mangle.DerivationNode{node}
	for _, child := range node.Children {
		nodes = append(nodes, derivationSubtree(child)...)
	}
	return nodes
}

// Explain returns the proofs of goal from the first shard, among those a
// Query of its predicate reads, that recorded one. provenance.ErrNoProof when
// none did; ErrProvenanceDisabled when recording is off.
func (c *CortexKernel) Explain(goal string, opts ExplainOptions) ([]*provenance.ProofNode, error) {
	shards, _, err := c.readShards(goal)
	if err != nil {
		return nil, err
	}
	for _, s := range shards {
		proofs, err := s.kernel.Explain(goal, opts)
		if errors.Is(err, provenance.ErrNoProof) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return proofs, nil
	}
	return nil, provenance.ErrNoProof
}

// EnableProvenance turns proof recording on in every shard: a goal is proved
// in whichever shard derives it.
func (c *CortexKernel) EnableProvenance() {
	for _, s := range c.shardList() {
		s.kernel.EnableProvenance()
	}
}

// DisableProvenance turns proof recording off in every shard.
func (c *CortexKernel) DisableProvenance() {
	for _, s := range c.shardList() {
		s.kernel.DisableProvenance()
	}
}

// IsProvenanceEnabled reports whether every shard records proofs.
func (c *CortexKernel) IsProvenanceEnabled() bool {
	shards := c.shardList()
	if len(shards) == 0 {
		return false
	}
	for _, s := range shards {
		if !s.kernel.IsProvenanceEnabled() {
			return false
		}
	}
	return true
}

// AssertString parses one fact and asserts it through the Cortex, so it lands
// in the shard that owns its predicate (or every shard, when shared).
func (c *CortexKernel) AssertString(factStr string) error {
	fact, err := ParseFactString(factStr)
	if err != nil {
		return fmt.Errorf("AssertString: failed to parse %q: %w", factStr, err)
	}
	return c.Assert(fact)
}

// shardList snapshots the shards that have a kernel.
func (c *CortexKernel) shardList() []*KernelShard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	shards := make([]*KernelShard, 0, len(c.shards))
	for _, s := range c.shards {
		if s != nil && s.kernel != nil {
			shards = append(shards, s)
		}
	}
	return shards
}

// ShadowKernel is one kernel over every shard's facts: the primary's program
// -- every shard evaluates the same program -- with the base facts of every
// shard, a shared predicate's rows once. It is what a simulation copies
// (ShadowParent): a rule whose premises different shards own never derives in
// production, but it does here, which is what a what-if is asked for. A clone
// of the catch-all shard alone held only the facts no other shard owns.
func (c *CortexKernel) ShadowKernel() (*RealKernel, error) {
	primary := c.GetPrimaryRealKernel()
	if primary == nil {
		return nil, fmt.Errorf("cortex has no primary shard to copy")
	}
	shadow := primary.Clone()
	var rest []Fact
	for _, shard := range c.shardList() {
		shard.mu.RLock()
		k := shard.kernel
		shard.mu.RUnlock()
		if k == nil || k == primary {
			continue
		}
		for f := range k.GetFactsSnapshotSeq() {
			rest = append(rest, f)
		}
	}
	if len(rest) > 0 {
		if err := shadow.AssertBatch(rest); err != nil {
			return nil, fmt.Errorf("copy the shards' facts into the shadow kernel: %w", err)
		}
	}
	return shadow, nil
}
