package prompt

import (
	"strings"
	"sync"

	"codenerd/internal/types"
)

// Candidate facts are the selector's description of every candidate atom:
// atom/1, atom_category/2, atom_priority/2, prompt_atom/5, atom_tag/3,
// atom_requires/2, atom_conflicts/2, atom_requires_tool/2 and is_mandatory/1.
// A compile emits them for the whole candidate set -- tens of thousands of
// rows for the shipped corpus -- and they used to be rendered as Mangle text
// and parsed back by the kernel adapter on every compile, although all but
// the mandatory flag depend only on the atom.
//
// They are now typed Fact values (strings stay Go strings, name constants are
// types.MangleAtom, numbers int64: exactly what the adapter's parser used to
// produce), and the static ones are built once per immutable atom and reused.
// is_mandatory and prompt_atom carry the per-compile forced-mandatory
// override and are built per compile. Mangle remains the selector; only the
// encoding of its input changed.

// candidateFactCacheLimit bounds the cache. Corpus atoms are long-lived
// pointers (embedded corpus, per-generation catalogs); a reset when the bound
// is crossed drops entries for atoms no longer in use.
const candidateFactCacheLimit = 16384

type candidateFactCache struct {
	mu      sync.Mutex
	entries map[*PromptAtom][]any
}

func (cache *candidateFactCache) get(atom *PromptAtom) []any {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.entries[atom]
}

func (cache *candidateFactCache) put(atom *PromptAtom, facts []any) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil || len(cache.entries) >= candidateFactCacheLimit {
		cache.entries = make(map[*PromptAtom][]any)
	}
	cache.entries[atom] = facts
}

// cacheableCandidate reports whether an atom outlives one compile. Kernel
// injections and retrieved context are rebuilt every compile; caching them
// would only churn the cache.
func cacheableCandidate(atom *PromptAtom) bool {
	return !atom.KernelInjected && !atom.RetrievedContext
}

// appendCandidateFacts appends the selector facts of every candidate.
func (s *AtomSelector) appendCandidateFacts(facts []any, atoms []*PromptAtom, forcedMandatory map[string]struct{}) []any {
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		isMandatory := atom.IsMandatory
		if forcedMandatory != nil {
			if _, ok := forcedMandatory[atom.ID]; ok {
				isMandatory = true
			}
		}
		if isMandatory {
			facts = append(facts, Fact{Predicate: "is_mandatory", Args: []any{atom.ID}})
		}
		// Schema (schemas_prompts.mg):
		//   Decl prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
		//                    /string  /name    /number   /number    /name
		// Position 3 is the TokenCount, never the content hash: a hex string
		// there broke every fn:plus / fn:max over the slot.
		if category := candidateCategoryName(atom); category != "" {
			mandatory := types.MangleAtom("/false")
			if isMandatory {
				mandatory = types.MangleAtom("/true")
			}
			facts = append(facts, Fact{Predicate: "prompt_atom", Args: []any{
				atom.ID, category, int64(atom.Priority), int64(atom.TokenCount), mandatory,
			}})
		}

		static := s.candidateFacts.get(atom)
		if static == nil {
			static = staticCandidateFacts(atom)
			if cacheableCandidate(atom) {
				s.candidateFacts.put(atom, static)
			}
		}
		facts = append(facts, static...)
	}
	return facts
}

// candidateCategoryName is the category as a name constant (e.g. /identity).
func candidateCategoryName(atom *PromptAtom) types.MangleAtom {
	category := string(atom.Category)
	if !strings.HasPrefix(category, "/") {
		category = "/" + category
	}
	return types.MangleAtom(mangleNormalizeNameConst(category))
}

// staticCandidateFacts builds the facts that depend only on the atom.
func staticCandidateFacts(atom *PromptAtom) []any {
	id := atom.ID
	facts := []any{
		Fact{Predicate: "atom", Args: []any{id}},
		Fact{Predicate: "atom_category", Args: []any{id, string(atom.Category)}},
		Fact{Predicate: "atom_priority", Args: []any{id, int64(atom.Priority)}},
	}

	// Tags use name constants for dimension and value: current_context
	// (/shard, /coder) must match atom_tag(ID, /shard, /coder), and the string
	// 'shard' is a different Mangle type from the name /shard.
	addTags := func(dim string, values []string) {
		atomDim := mangleNormalizeNameConst("/" + strings.TrimPrefix(dim, "/"))
		for _, v := range values {
			value := v
			if !strings.HasPrefix(value, "/") {
				value = "/" + value
			}
			atomVal := mangleNormalizeNameConst(value)
			if atomDim == "" || atomVal == "" {
				continue
			}
			facts = append(facts, Fact{Predicate: "atom_tag", Args: []any{
				id, types.MangleAtom(atomDim), types.MangleAtom(atomVal),
			}})
		}
	}
	addTags("mode", atom.OperationalModes)
	addTags("phase", atom.CampaignPhases)
	addTags("layer", atom.BuildLayers)
	addTags("init_phase", atom.InitPhases)
	addTags("northstar_phase", atom.NorthstarPhases)
	addTags("ouroboros_stage", atom.OuroborosStages)
	addTags("intent", atom.IntentVerbs)
	// Emit both /shard and /shard_type because policy uses both
	// (jit_selection.mg reads /shard in one rule and /shard_type in another).
	addTags("shard", atom.ShardTypes)
	addTags("shard_type", atom.ShardTypes)
	addTags("lang", atom.Languages)
	addTags("framework", atom.Frameworks)
	addTags("state", atom.WorldStates)
	// Pin dimensions. Both are regime dimensions in jit_compiler.mg, so
	// emitting the tag is what arms the fail-closed block: an atom with a
	// /provider or /model tag is admitted only on a compile whose
	// current_context carries a matching one.
	addTags("provider", atom.Providers)
	addTags("model", atom.Models)

	// Dependencies - needed for atom_requires() in jit_compiler.mg
	for _, dep := range atom.DependsOn {
		if dep != "" {
			facts = append(facts, Fact{Predicate: "atom_requires", Args: []any{id, dep}})
		}
	}
	// Conflicts - needed for atom_conflicts() in jit_compiler.mg
	for _, conflict := range atom.ConflictsWith {
		if conflict != "" {
			facts = append(facts, Fact{Predicate: "atom_conflicts", Args: []any{id, conflict}})
		}
	}
	// Capability requirements - needed for blocked_by_missing_tool in
	// jit_selection.mg. Tool names are strings, not name constants. Never
	// widen authority here; the policy omits the atom when any required tool
	// is absent.
	for _, tool := range atom.RequiresTools {
		if strings.TrimSpace(tool) == "" {
			continue
		}
		facts = append(facts, Fact{Predicate: "atom_requires_tool", Args: []any{id, tool}})
	}
	return facts
}
