package prompt

import (
	"maps"
	"slices"
)

// CompilationResult holds the output of a prompt compilation.
type CompilationResult struct {
	// The compiled prompt text
	Prompt string

	// Atoms included in the prompt (for debugging/audit)
	IncludedAtoms []*PromptAtom

	// Token usage statistics
	TotalTokens int
	BudgetUsed  float64 // Percentage of budget used
	// MandatoryCount / OptionalCount hold the skeleton / flesh split by
	// CATEGORY (isSkeletonCategory), which is what the stats derived from them
	// are labelled as. The names predate the skeleton/flesh vocabulary; they do
	// NOT count the IsMandatory flag, and classifying on that flag is what made
	// the JIT summary report flesh=0 while flesh atoms were being selected.
	MandatoryCount int
	OptionalCount  int

	// Categories breakdown
	CategoryTokens map[AtomCategory]int

	// Compilation metadata
	CompilationTimeMs int64
	AtomsCandidates   int // Total atoms considered
	AtomsSelected     int // Atoms that matched context
	AtomsIncluded     int // Atoms that fit in budget

	// Compilation manifest (Flight Recorder)
	Manifest *PromptManifest

	// Comprehensive compilation statistics
	Stats *CompilationStats
}

// clonePromptAtom deep-copies an atom's mutable slices.
func clonePromptAtom(atom *PromptAtom) *PromptAtom {
	if atom == nil {
		return nil
	}
	clone := *atom
	clone.OperationalModes = slices.Clone(atom.OperationalModes)
	clone.CampaignPhases = slices.Clone(atom.CampaignPhases)
	clone.BuildLayers = slices.Clone(atom.BuildLayers)
	clone.InitPhases = slices.Clone(atom.InitPhases)
	clone.NorthstarPhases = slices.Clone(atom.NorthstarPhases)
	clone.OuroborosStages = slices.Clone(atom.OuroborosStages)
	clone.IntentVerbs = slices.Clone(atom.IntentVerbs)
	clone.ShardTypes = slices.Clone(atom.ShardTypes)
	clone.Languages = slices.Clone(atom.Languages)
	clone.Frameworks = slices.Clone(atom.Frameworks)
	clone.Models = slices.Clone(atom.Models)
	clone.Providers = slices.Clone(atom.Providers)
	clone.WorldStates = slices.Clone(atom.WorldStates)
	clone.DependsOn = slices.Clone(atom.DependsOn)
	clone.ConflictsWith = slices.Clone(atom.ConflictsWith)
	clone.RequiresTools = slices.Clone(atom.RequiresTools)
	clone.Embedding = slices.Clone(atom.Embedding)
	return &clone
}

// cloneCompilationResult copies the mutable object graph exposed by result
// APIs. Cache and last-result state remain private when a consumer edits atoms,
// maps, manifests, or statistics returned by an observer.
func cloneCompilationResult(res *CompilationResult) *CompilationResult {
	if res == nil {
		return nil
	}
	out := *res
	if res.IncludedAtoms != nil {
		out.IncludedAtoms = make([]*PromptAtom, len(res.IncludedAtoms))
		for i, atom := range res.IncludedAtoms {
			out.IncludedAtoms[i] = clonePromptAtom(atom)
		}
	}
	if res.CategoryTokens != nil {
		out.CategoryTokens = maps.Clone(res.CategoryTokens)
	}
	if res.Manifest != nil {
		manifest := *res.Manifest
		manifest.Selected = slices.Clone(res.Manifest.Selected)
		manifest.Dropped = slices.Clone(res.Manifest.Dropped)
		out.Manifest = &manifest
	}
	if res.Stats != nil {
		stats := *res.Stats
		out.Stats = &stats
	}
	return &out
}

// privateResult returns an independent copy for cache miss, hit, and
// singleflight callers. cacheHit belongs to this caller, so changing it on
// the copy cannot rewrite the stored result or another caller's statistics.
func privateResult(res *CompilationResult, cacheHit bool) *CompilationResult {
	out := cloneCompilationResult(res)
	if out != nil && out.Stats != nil {
		// Cache status is per caller, so it must not rewrite the stored result.
		out.Stats.CacheHit = cacheHit
	}
	return out
}
