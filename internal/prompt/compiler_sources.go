package prompt

import (
	"context"

	"codenerd/internal/logging"
)

// sourceBreakdown tracks atom counts by source.
type sourceBreakdown struct {
	embedded int
	project  int
	shard    int
	evolved  int
	strategy int
}

// collectAtomsWithStats gathers atoms and tracks source breakdown. It reads
// only the handles in src, the same ones the source vector was captured
// from; database atoms come from the typed catalog when their generation is
// current, and a failed optional source marks the compile degraded.
func (c *JITPromptCompiler) collectAtomsWithStats(
	ctx context.Context,
	cc *CompilationContext,
	src compileSources,
	vector sourceVector,
	pending *pendingCatalogs,
	degraded *degradation,
) ([]*PromptAtom, sourceBreakdown, error) {
	var breakdown sourceBreakdown
	embeddedCorpus := src.embedded

	// Earlier sources are authoritative. In particular, the embedded corpus is
	// the canonical source for built-in atoms; the project database may contain
	// an older synchronized copy when embedding refresh is unavailable. Letting
	// both copies through can place contradictory instructions in one prompt.
	embeddedCount := 0
	if embeddedCorpus != nil {
		embeddedCount = embeddedCorpus.Count()
	}
	allAtoms := make([]*PromptAtom, 0, embeddedCount+100)
	seen := make(map[string]string, embeddedCount+100)
	appendSource := func(source string, atoms []*PromptAtom) (added, duplicates int) {
		for _, atom := range atoms {
			if atom == nil || atom.ID == "" {
				// Preserve existing invalid-atom handling in the selector/resolver.
				allAtoms = append(allAtoms, atom)
				added++
				continue
			}
			if _, exists := seen[atom.ID]; exists {
				duplicates++
				continue
			}
			seen[atom.ID] = source
			allAtoms = append(allAtoms, atom)
			added++
		}
		if duplicates > 0 {
			logging.Get(logging.CategoryJIT).Debug(
				"Skipped %d duplicate %s atom(s); earlier sources take precedence",
				duplicates, source,
			)
		}
		return added, duplicates
	}

	// 1. Embedded corpus: canonical built-in atoms always win duplicate IDs.
	if embeddedCorpus != nil {
		breakdown.embedded, _ = appendSource("embedded", embeddedCorpus.All())
	}

	// 2. Project database: adds project-only atoms, never stale built-in copies.
	if src.projectDB != nil {
		projectAtoms, err := c.loadCatalogAtoms(ctx, "project", src.projectDB, vector.parts["project"], pending)
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("Failed to load project atoms: %v", err)
			degraded.add("project")
		} else {
			breakdown.project, _ = appendSource("project", projectAtoms)
		}
	}

	// 3. Selected expert database: adds scoped atoms with unique IDs. Only the
	// selected expert is read, and only its generation is in the vector.
	if src.shardDB != nil {
		source := "expert:" + src.shardKey
		shardAtoms, err := c.loadCatalogAtoms(ctx, source, src.shardDB, vector.parts[source], pending)
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("Failed to load shard atoms: %v", err)
			degraded.add(source)
		} else {
			breakdown.shard, _ = appendSource("shard", shardAtoms)
		}
	}

	// 4. Evolved atoms from SPL (System Prompt Learning).
	evolvedAtoms := c.collectEvolvedAtoms(cc)
	breakdown.evolved, _ = appendSource("evolved", evolvedAtoms)

	// 5. Learned strategies for this problem type. Last, so a strategy never
	// shadows a built-in atom that happens to share an ID — the same
	// precedence every source below the embedded corpus already has.
	breakdown.strategy, _ = appendSource("strategy", c.collectStrategyAtoms(cc))

	return allAtoms, breakdown, nil
}
