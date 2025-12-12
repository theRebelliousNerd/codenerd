// Package prompt implements the JIT Prompt Compiler for codeNERD.
// This system compiles optimal system prompts from atomic prompt fragments
// based on the current compilation context (operational mode, campaign phase,
// shard type, language, etc.).
//
// The JIT compiler achieves "infinite" effective prompt length through:
// 1. Atomic decomposition - prompts are stored as small, reusable atoms
// 2. Context-aware selection - only relevant atoms are selected
// 3. Vector-augmented retrieval - semantic search for domain-specific atoms
// 4. Budget-constrained assembly - fits within token limits
package prompt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// AtomCategory represents the category of a prompt atom.
// Categories organize atoms by their role in the system prompt.
type AtomCategory string

const (
	// CategoryIdentity defines who the agent is and its core capabilities.
	CategoryIdentity AtomCategory = "identity"

	// CategoryProtocol defines operational protocols (Piggyback, OODA, etc.).
	CategoryProtocol AtomCategory = "protocol"

	// CategorySafety defines constitutional safety constraints.
	CategorySafety AtomCategory = "safety"

	// CategoryMethodology defines how to approach problems (TDD, etc.).
	CategoryMethodology AtomCategory = "methodology"

	// CategoryHallucination contains anti-hallucination guardrails.
	CategoryHallucination AtomCategory = "hallucination"

	// CategoryLanguage contains language-specific guidance (Go, Python, etc.).
	CategoryLanguage AtomCategory = "language"

	// CategoryFramework contains framework-specific guidance (Bubbletea, etc.).
	CategoryFramework AtomCategory = "framework"

	// CategoryDomain contains domain-specific knowledge (project context).
	CategoryDomain AtomCategory = "domain"

	// CategoryCampaign contains campaign/goal-specific context.
	CategoryCampaign AtomCategory = "campaign"

	// CategoryInit contains initialization phase guidance.
	CategoryInit AtomCategory = "init"

	// CategoryNorthstar contains northstar planning guidance.
	CategoryNorthstar AtomCategory = "northstar"

	// CategoryOuroboros contains self-improvement/tool-generation guidance.
	CategoryOuroboros AtomCategory = "ouroboros"

	// CategoryContext contains dynamic context atoms (files, symbols, etc.).
	CategoryContext AtomCategory = "context"

	// CategoryExemplar contains few-shot examples and demonstrations.
	CategoryExemplar AtomCategory = "exemplar"

	// CategoryReviewer contains reviewer-specific atoms for code review enhancement.
	CategoryReviewer AtomCategory = "reviewer"

	// CategoryKnowledge contains knowledge extraction and management atoms.
	CategoryKnowledge AtomCategory = "knowledge"

	// CategoryBuildLayer contains build layer guidance (scaffold, domain_core, etc.).
	CategoryBuildLayer AtomCategory = "build_layer"

	// CategoryIntent contains intent-specific guidance for user intent classification.
	CategoryIntent AtomCategory = "intent"

	// CategoryWorldState contains world state awareness atoms (diagnostics, errors, etc.).
	CategoryWorldState AtomCategory = "world_state"
)

// AllCategories returns all defined atom categories.
func AllCategories() []AtomCategory {
	return []AtomCategory{
		CategoryIdentity,
		CategoryProtocol,
		CategorySafety,
		CategoryMethodology,
		CategoryHallucination,
		CategoryLanguage,
		CategoryFramework,
		CategoryDomain,
		CategoryCampaign,
		CategoryInit,
		CategoryNorthstar,
		CategoryOuroboros,
		CategoryContext,
		CategoryExemplar,
		CategoryReviewer,
		CategoryKnowledge,
		CategoryBuildLayer,
		CategoryIntent,
		CategoryWorldState,
	}
}

// PromptAtom represents a single atomic prompt fragment.
// Atoms are the fundamental building blocks of compiled prompts.
// Each atom is self-contained and can be independently selected,
// composed, and assembled into a final prompt.
type PromptAtom struct {
	// Unique identifier for this atom (e.g., "go-error-handling-v2")
	ID string `json:"id"`

	// Version number for tracking updates
	Version int `json:"version"`

	// The actual prompt content (markdown/text)
	Content string `json:"content"`

	// Estimated token count (cached for budget calculations)
	TokenCount int `json:"token_count"`

	// SHA256 hash of Content for deduplication and cache invalidation
	ContentHash string `json:"content_hash"`

	// Classification
	Category    AtomCategory `json:"category"`
	Subcategory string       `json:"subcategory,omitempty"`

	// =========================================================================
	// Contextual Selectors (JSON arrays in DB)
	// These define WHEN this atom should be included in a prompt.
	// An atom matches if ANY selector in a non-empty list matches.
	// Empty lists mean "always match" for that dimension.
	// =========================================================================

	// OperationalModes: /active, /dream, /debugging, /creative, /scaffolding, /shadow, /tdd_repair
	OperationalModes []string `json:"operational_modes,omitempty"`

	// CampaignPhases: /planning, /decomposing, /validating, /active, /completed, /paused, /failed
	CampaignPhases []string `json:"campaign_phases,omitempty"`

	// BuildLayers: /scaffold, /domain_core, /data_layer, /service, /transport, /integration
	BuildLayers []string `json:"build_layers,omitempty"`

	// InitPhases: /migration, /setup, /scanning, /analysis, /profile, /facts, /agents, /kb_*
	InitPhases []string `json:"init_phases,omitempty"`

	// NorthstarPhases: /doc_ingestion, /problem, /vision, /requirements, /architecture, /roadmap
	NorthstarPhases []string `json:"northstar_phases,omitempty"`

	// OuroborosStages: /detection, /specification, /safety_check, /simulation, /codegen, /testing
	OuroborosStages []string `json:"ouroboros_stages,omitempty"`

	// IntentVerbs: /fix, /debug, /refactor, /test, /review, /create, /research, /explain
	IntentVerbs []string `json:"intent_verbs,omitempty"`

	// ShardTypes: /coder, /tester, /reviewer, /researcher, /librarian, /planner, /custom
	ShardTypes []string `json:"shard_types,omitempty"`

	// Languages: /go, /python, /typescript, /rust, /java, /javascript, /mangle
	Languages []string `json:"languages,omitempty"`

	// Frameworks: /bubbletea, /gin, /react, /django, /rod, /lipgloss
	Frameworks []string `json:"frameworks,omitempty"`

	// WorldStates: failing_tests, diagnostics, large_refactor, security_issues, new_files, high_churn
	WorldStates []string `json:"world_states,omitempty"`

	// =========================================================================
	// Composition Rules
	// =========================================================================

	// Priority determines ordering within a category (higher = earlier)
	Priority int `json:"priority"`

	// IsMandatory means this atom MUST be included if it matches context
	IsMandatory bool `json:"is_mandatory"`

	// IsExclusive is an exclusion group ID. Only one atom per group is selected.
	IsExclusive string `json:"is_exclusive,omitempty"`

	// DependsOn lists atom IDs that must be present for this atom to be selected
	DependsOn []string `json:"depends_on,omitempty"`

	// ConflictsWith lists atom IDs that cannot be present with this atom
	ConflictsWith []string `json:"conflicts_with,omitempty"`

	// =========================================================================
	// Embedding for Vector Search
	// =========================================================================

	// Embedding is the vector representation (3072-dim for Gemini)
	// Not serialized to JSON to avoid bloating API responses
	Embedding []float32 `json:"-"`

	// EmbeddingTask records which task type was used for embedding
	EmbeddingTask string `json:"embedding_task,omitempty"`

	// Description used for semantic search (embedded instead of Content)
	Description string `json:"description"`

	// Concise version for tight token budgets
	ContentConcise string `json:"content_concise,omitempty"`

	// Minimal version (emergency fallback)
	ContentMin string `json:"content_min,omitempty"`

	// CreatedAt tracks when this atom was created
	CreatedAt time.Time `json:"created_at"`
}

// EstimateTokens estimates the token count for content using chars/4 approximation.
// This is a fast heuristic; actual tokenization may vary by model.
func EstimateTokens(content string) int {
	if content == "" {
		return 0
	}
	// chars/4 is a reasonable approximation for English text
	// Adjust for code which tends to be more token-dense
	return (len(content) + 3) / 4
}

// HashContent computes a SHA256 hash of content for deduplication.
func HashContent(content string) string {
	if content == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// NewPromptAtom creates a new PromptAtom with computed fields.
func NewPromptAtom(id string, category AtomCategory, content string) *PromptAtom {
	return &PromptAtom{
		ID:          id,
		Version:     1,
		Category:    category,
		Content:     content,
		TokenCount:  EstimateTokens(content),
		ContentHash: HashContent(content),
		CreatedAt:   time.Now(),
	}
}

// MatchesContext checks if this atom should be included for the given context.
// Returns true if the atom matches ALL non-empty selector dimensions.
// Empty selector lists are treated as "match any".
func (a *PromptAtom) MatchesContext(cc *CompilationContext) bool {
	if cc == nil {
		return true
	}

	// Check each selector dimension
	// For each dimension: if the selector list is non-empty, the context value must be in it

	if !matchSelector(a.OperationalModes, cc.OperationalMode) {
		return false
	}

	if !matchSelector(a.CampaignPhases, cc.CampaignPhase) {
		return false
	}

	if !matchSelector(a.BuildLayers, cc.BuildLayer) {
		return false
	}

	if !matchSelector(a.InitPhases, cc.InitPhase) {
		return false
	}

	if !matchSelector(a.NorthstarPhases, cc.NorthstarPhase) {
		return false
	}

	if !matchSelector(a.OuroborosStages, cc.OuroborosStage) {
		return false
	}

	if !matchSelector(a.IntentVerbs, cc.IntentVerb) {
		return false
	}

	if !matchSelector(a.ShardTypes, cc.ShardType) {
		return false
	}

	if !matchSelector(a.Languages, cc.Language) {
		return false
	}

	// Frameworks: match if ANY framework in context matches ANY in selector
	if len(a.Frameworks) > 0 && len(cc.Frameworks) > 0 {
		found := false
		for _, af := range a.Frameworks {
			for _, cf := range cc.Frameworks {
				if af == cf {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// WorldStates: match if ANY required world state is present
	if len(a.WorldStates) > 0 {
		contextStates := cc.WorldStates()
		found := false
		for _, ws := range a.WorldStates {
			for _, cs := range contextStates {
				if ws == cs {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// matchSelector checks if a value matches a selector list.
// Empty selector list means "match any". Empty value matches empty list only.
func matchSelector(selector []string, value string) bool {
	if len(selector) == 0 {
		return true // No constraint = match all
	}
	if value == "" {
		return false // Has constraint but no value = no match
	}
	// Normalize to allow legacy selector values without leading "/" to match
	// canonical context values (and vice versa). This preserves backward
	// compatibility while encouraging "/"-prefixed tags going forward.
	normalizedValue := strings.TrimPrefix(value, "/")
	for _, s := range selector {
		if s == value {
			return true
		}
		if strings.TrimPrefix(s, "/") == normalizedValue {
			return true
		}
	}
	return false
}

// ToFact converts this atom to a Mangle fact for kernel queries.
// Format: prompt_atom(ID, Category, TokenCount, Priority, IsMandatory).
func (a *PromptAtom) ToFact() core.Fact {
	mandatory := "/false"
	if a.IsMandatory {
		mandatory = "/true"
	}

	return core.Fact{
		Predicate: "prompt_atom",
		Args: []interface{}{
			a.ID,
			"/" + string(a.Category),
			a.TokenCount,
			a.Priority,
			core.MangleAtom(mandatory),
		},
	}
}

// ToSelectorFact generates a selector fact for Mangle matching.
// Format: atom_selector(ID, Dimension, Value).
func (a *PromptAtom) ToSelectorFacts() []core.Fact {
	var facts []core.Fact

	addSelectorFacts := func(dimension string, values []string) {
		for _, v := range values {
			facts = append(facts, core.Fact{
				Predicate: "atom_selector",
				Args:      []interface{}{a.ID, "/" + dimension, v},
			})
		}
	}

	addSelectorFacts("operational_mode", a.OperationalModes)
	addSelectorFacts("campaign_phase", a.CampaignPhases)
	addSelectorFacts("build_layer", a.BuildLayers)
	addSelectorFacts("init_phase", a.InitPhases)
	addSelectorFacts("northstar_phase", a.NorthstarPhases)
	addSelectorFacts("ouroboros_stage", a.OuroborosStages)
	addSelectorFacts("intent_verb", a.IntentVerbs)
	addSelectorFacts("shard_type", a.ShardTypes)
	addSelectorFacts("language", a.Languages)
	addSelectorFacts("framework", a.Frameworks)
	addSelectorFacts("world_state", a.WorldStates)

	return facts
}

// ToDependencyFacts generates dependency facts for Mangle.
// Format: atom_depends(ID, DependencyID).
func (a *PromptAtom) ToDependencyFacts() []core.Fact {
	var facts []core.Fact

	for _, dep := range a.DependsOn {
		facts = append(facts, core.Fact{
			Predicate: "atom_depends",
			Args:      []interface{}{a.ID, dep},
		})
	}

	return facts
}

// ToConflictFacts generates conflict facts for Mangle.
// Format: atom_conflicts(ID1, ID2).
func (a *PromptAtom) ToConflictFacts() []core.Fact {
	var facts []core.Fact

	for _, conflict := range a.ConflictsWith {
		facts = append(facts, core.Fact{
			Predicate: "atom_conflicts",
			Args:      []interface{}{a.ID, conflict},
		})
	}

	return facts
}

// ToExclusionFact generates an exclusion group fact if applicable.
// Format: atom_exclusive(ID, GroupID).
func (a *PromptAtom) ToExclusionFact() *core.Fact {
	if a.IsExclusive == "" {
		return nil
	}

	return &core.Fact{
		Predicate: "atom_exclusive",
		Args:      []interface{}{a.ID, a.IsExclusive},
	}
}

// Validate checks the atom for consistency errors.
func (a *PromptAtom) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("atom ID is required")
	}

	if a.Content == "" {
		return fmt.Errorf("atom content is required for atom %q", a.ID)
	}

	if a.Category == "" {
		return fmt.Errorf("atom category is required for atom %q", a.ID)
	}

	// Validate category is known
	validCategory := false
	for _, cat := range AllCategories() {
		if cat == a.Category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		return fmt.Errorf("unknown category %q for atom %q", a.Category, a.ID)
	}

	// Check for self-dependency
	for _, dep := range a.DependsOn {
		if dep == a.ID {
			return fmt.Errorf("atom %q cannot depend on itself", a.ID)
		}
	}

	// Check for self-conflict
	for _, conflict := range a.ConflictsWith {
		if conflict == a.ID {
			return fmt.Errorf("atom %q cannot conflict with itself", a.ID)
		}
	}

	return nil
}

// Clone creates a deep copy of the atom.
func (a *PromptAtom) Clone() *PromptAtom {
	clone := *a

	// Deep copy slices
	clone.OperationalModes = copyStringSlice(a.OperationalModes)
	clone.CampaignPhases = copyStringSlice(a.CampaignPhases)
	clone.BuildLayers = copyStringSlice(a.BuildLayers)
	clone.InitPhases = copyStringSlice(a.InitPhases)
	clone.NorthstarPhases = copyStringSlice(a.NorthstarPhases)
	clone.OuroborosStages = copyStringSlice(a.OuroborosStages)
	clone.IntentVerbs = copyStringSlice(a.IntentVerbs)
	clone.ShardTypes = copyStringSlice(a.ShardTypes)
	clone.Languages = copyStringSlice(a.Languages)
	clone.Frameworks = copyStringSlice(a.Frameworks)
	clone.WorldStates = copyStringSlice(a.WorldStates)
	clone.DependsOn = copyStringSlice(a.DependsOn)
	clone.ConflictsWith = copyStringSlice(a.ConflictsWith)

	// Deep copy embedding if present
	if a.Embedding != nil {
		clone.Embedding = make([]float32, len(a.Embedding))
		copy(clone.Embedding, a.Embedding)
	}

	return &clone
}

// copyStringSlice creates a deep copy of a string slice.
func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}

// EmbeddedCorpus holds the embedded (baked-in) prompt atoms.
// These are loaded at compile time and cannot be modified.
type EmbeddedCorpus struct {
	atoms map[string]*PromptAtom
}

// NewEmbeddedCorpus creates a new embedded corpus from a slice of atoms.
func NewEmbeddedCorpus(atoms []*PromptAtom) *EmbeddedCorpus {
	corpus := &EmbeddedCorpus{
		atoms: make(map[string]*PromptAtom, len(atoms)),
	}
	for _, atom := range atoms {
		corpus.atoms[atom.ID] = atom
	}
	return corpus
}

// Get retrieves an atom by ID.
func (c *EmbeddedCorpus) Get(id string) (*PromptAtom, bool) {
	atom, ok := c.atoms[id]
	return atom, ok
}

// GetByCategory returns all atoms in a category.
func (c *EmbeddedCorpus) GetByCategory(category AtomCategory) []*PromptAtom {
	var result []*PromptAtom
	for _, atom := range c.atoms {
		if atom.Category == category {
			result = append(result, atom)
		}
	}
	return result
}

// All returns all atoms in the corpus.
func (c *EmbeddedCorpus) All() []*PromptAtom {
	result := make([]*PromptAtom, 0, len(c.atoms))
	for _, atom := range c.atoms {
		result = append(result, atom)
	}
	return result
}

// Count returns the number of atoms in the corpus.
func (c *EmbeddedCorpus) Count() int {
	return len(c.atoms)
}

// AtomStore defines the interface for loading atoms from various sources.
type AtomStore interface {
	// LoadAtoms loads all atoms from this store.
	LoadAtoms(ctx context.Context) ([]*PromptAtom, error)

	// GetAtom retrieves a specific atom by ID.
	GetAtom(ctx context.Context, id string) (*PromptAtom, error)

	// SaveAtom persists an atom to the store.
	SaveAtom(ctx context.Context, atom *PromptAtom) error

	// DeleteAtom removes an atom from the store.
	DeleteAtom(ctx context.Context, id string) error
}

// LogAtomSelection logs atom selection decisions for debugging.
func LogAtomSelection(atomID string, selected bool, reason string) {
	if selected {
		logging.Get(logging.CategoryContext).Debug("Atom selected: %s (%s)", atomID, reason)
	} else {
		logging.Get(logging.CategoryContext).Debug("Atom rejected: %s (%s)", atomID, reason)
	}
}
