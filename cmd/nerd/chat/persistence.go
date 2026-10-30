// Package chat provides the interactive TUI chat interface for codeNERD.
// This file carries the session's focus into knowledge.db.
package chat

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// persistActivation carries the turn's focus into the next session: the
// kernel's activation of every fact that entered working memory
// (context_atom, policy/activation.mg), written to the activation log, which
// boot hydrates back as activation facts (VirtualStore.QueryActivations, the
// last hour's) so a resumed session starts where this one was looking.
//
// It replaces persistActivationScores, which nothing called: it logged the
// context compressor's scores under "predicate_[args]" ids on a 0-50 scale,
// which matched no activation id the kernel derives and no scale the
// hydration reads. The ids here are the kernel's own, the scale is the
// hydration's ratio, and which facts count is the policy's context_atom, not
// a threshold in Go.
func (m *Model) persistActivation() {
	if m.kernel == nil || m.localDB == nil {
		return
	}
	atoms, err := m.kernel.Query("context_atom")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("activation not persisted: query context_atom: %v", err)
		return
	}
	inMemory := make(map[string]bool, len(atoms))
	for _, f := range atoms {
		if len(f.Args) > 0 {
			inMemory[types.ExtractString(f.Args[0])] = true
		}
	}
	if len(inMemory) == 0 {
		return
	}
	rows, err := m.kernel.Query("activation")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("activation not persisted: query activation: %v", err)
		return
	}
	best := map[string]int64{}
	for _, f := range rows {
		if len(f.Args) <= 1 {
			continue
		}
		id := types.ExtractString(f.Args[0])
		score, ok := f.Args[1].(int64)
		if !ok || !inMemory[id] {
			continue
		}
		if prev, seen := best[id]; !seen || score > prev {
			best[id] = score
		}
	}
	for id, score := range best {
		// The kernel's percent scale as the log's ratio; PercentFromRatio
		// turns it back at hydration.
		if err := m.localDB.LogActivation(id, float64(min(score, 100))/100); err != nil {
			logging.Get(logging.CategoryContext).Warn("activation of %s not persisted: %v", id, err)
		}
	}
}
