// Package prompt provides the JIT Predicate Selector for context-aware predicate injection.
// It selects the most relevant predicates from the corpus based on current compilation context.
package prompt

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// PredicateSelector selects predicates for JIT prompt injection.
// Instead of injecting all 799 predicates, it selects ~50-100 based on:
// - Compilation context (shard type, intent, phase)
// - Domain relevance
// - Semantic similarity to the task
type PredicateSelector struct {
	corpus             *core.PredicateCorpus
	vectorStore        *store.LocalStore
	vectorIndexReady   atomic.Bool
	vectorIndexRunning atomic.Bool
	maxPredicates      int
	vectorLimit        int
}

const (
	defaultPredicateLimit    = 100
	defaultPredicateVecLimit = 200
	predicateVectorMetaKey   = "kind"
	predicateVectorMetaValue = "predicate"
)

// SelectionContext provides dimensions for predicate selection.
type SelectionContext struct {
	ShardType       string   // /coder, /tester, /reviewer
	IntentVerb      string   // /fix, /debug, /implement
	CampaignPhase   string   // /planning, /executing, /validating
	Language        string   // /go, /python, /typescript
	TaskDescription string   // Free-text for semantic matching
	Domains         []string // Explicit domain requests
	MaxPredicates   int      // Limit (default: 100)
}

// SelectedPredicate contains a predicate with its selection metadata.
type SelectedPredicate struct {
	Name        string
	Arity       int
	Domain      string
	Category    string
	Description string
	Relevance   float64 // 0.0 - 1.0 relevance score
}

// NewPredicateSelector creates a new selector with the given corpus.
func NewPredicateSelector(corpus *core.PredicateCorpus) *PredicateSelector {
	return &PredicateSelector{
		corpus:        corpus,
		maxPredicates: defaultPredicateLimit,
		vectorLimit:   defaultPredicateVecLimit,
	}
}

// SetVectorStore attaches a LocalStore for vector-based predicate selection.
func (ps *PredicateSelector) SetVectorStore(store *store.LocalStore) {
	ps.vectorStore = store
	ps.ensurePredicateVectorIndex()
}

// SetMaxPredicates overrides the default predicate cap for selections.
func (ps *PredicateSelector) SetMaxPredicates(limit int) {
	if limit > 0 {
		ps.maxPredicates = limit
	}
}

// Select chooses predicates based on the compilation context.
func (ps *PredicateSelector) Select(ctx SelectionContext) ([]SelectedPredicate, error) {
	if ps.corpus == nil {
		return nil, fmt.Errorf("no predicate corpus configured")
	}

	// Set default max
	if ctx.MaxPredicates <= 0 {
		ctx.MaxPredicates = ps.maxPredicates
	}

	var selected []SelectedPredicate
	seen := make(map[string]bool)

	// 1. Always include core predicates (highest priority)
	corePredicates, _ := ps.corpus.GetByDomain("core")
	for _, p := range corePredicates {
		if !seen[p.Name] {
			seen[p.Name] = true
			selected = append(selected, SelectedPredicate{
				Name:        p.Name,
				Arity:       p.Arity,
				Domain:      p.Domain,
				Category:    p.Category,
				Description: p.Description,
				Relevance:   1.0, // Core = highest relevance
			})
		}
	}

	// 2. Add domain-specific predicates based on shard type
	if ctx.ShardType != "" {
		shardDomains := ps.shardTypeToDomains(ctx.ShardType)
		for _, domain := range shardDomains {
			domainPredicates, _ := ps.corpus.GetByDomain(domain)
			for _, p := range domainPredicates {
				if !seen[p.Name] {
					seen[p.Name] = true
					selected = append(selected, SelectedPredicate{
						Name:        p.Name,
						Arity:       p.Arity,
						Domain:      p.Domain,
						Category:    p.Category,
						Description: p.Description,
						Relevance:   0.9, // Shard-specific = high relevance
					})
				}
			}
		}
	}

	// 3. Add intent-specific predicates
	if ctx.IntentVerb != "" {
		intentDomains := ps.intentVerbToDomains(ctx.IntentVerb)
		for _, domain := range intentDomains {
			domainPredicates, _ := ps.corpus.GetByDomain(domain)
			for _, p := range domainPredicates {
				if !seen[p.Name] {
					seen[p.Name] = true
					selected = append(selected, SelectedPredicate{
						Name:        p.Name,
						Arity:       p.Arity,
						Domain:      p.Domain,
						Category:    p.Category,
						Description: p.Description,
						Relevance:   0.85,
					})
				}
			}
		}
	}

	// 4. Add campaign/phase predicates if active
	if ctx.CampaignPhase != "" {
		campaignPredicates, _ := ps.corpus.GetByDomain("campaign")
		for _, p := range campaignPredicates {
			if !seen[p.Name] {
				seen[p.Name] = true
				selected = append(selected, SelectedPredicate{
					Name:        p.Name,
					Arity:       p.Arity,
					Domain:      p.Domain,
					Category:    p.Category,
					Description: p.Description,
					Relevance:   0.8,
				})
			}
		}
	}

	// 5. Add explicitly requested domains
	for _, domain := range ctx.Domains {
		domainPredicates, _ := ps.corpus.GetByDomain(domain)
		for _, p := range domainPredicates {
			if !seen[p.Name] {
				seen[p.Name] = true
				selected = append(selected, SelectedPredicate{
					Name:        p.Name,
					Arity:       p.Arity,
					Domain:      p.Domain,
					Category:    p.Category,
					Description: p.Description,
					Relevance:   0.75,
				})
			}
		}
	}

	// 6. Add safety predicates (always important)
	safetyPredicates, _ := ps.corpus.GetByDomain("safety")
	for _, p := range safetyPredicates {
		if !seen[p.Name] {
			seen[p.Name] = true
			selected = append(selected, SelectedPredicate{
				Name:        p.Name,
				Arity:       p.Arity,
				Domain:      p.Domain,
				Category:    p.Category,
				Description: p.Description,
				Relevance:   0.95, // Safety = very high
			})
		}
	}

	// 7. Add routing predicates (needed for action dispatch)
	routingPredicates, _ := ps.corpus.GetByDomain("routing")
	for _, p := range routingPredicates {
		if !seen[p.Name] {
			seen[p.Name] = true
			selected = append(selected, SelectedPredicate{
				Name:        p.Name,
				Arity:       p.Arity,
				Domain:      p.Domain,
				Category:    p.Category,
				Description: p.Description,
				Relevance:   0.7,
			})
		}
	}

	// Sort by relevance (highest first)
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Relevance > selected[j].Relevance
	})

	// Limit to max
	if len(selected) > ctx.MaxPredicates {
		selected = selected[:ctx.MaxPredicates]
	}

	return selected, nil
}

// shardTypeToDomains maps shard types to relevant domains.
func (ps *PredicateSelector) shardTypeToDomains(shardType string) []string {
	shardType = strings.TrimPrefix(shardType, "/")
	switch shardType {
	case "coder":
		return []string{"shard_lifecycle", "shard_coder", "world_model", "diagnostic"}
	case "tester":
		return []string{"shard_lifecycle", "shard_tester", "diagnostic"}
	case "reviewer":
		return []string{"shard_lifecycle", "shard_reviewer", "diagnostic"}
	case "researcher":
		return []string{"shard_lifecycle", "memory"}
	case "tool_generator":
		return []string{"shard_lifecycle", "tool"}
	default:
		return []string{"shard_lifecycle"}
	}
}

// intentVerbToDomains maps intent verbs to relevant domains.
func (ps *PredicateSelector) intentVerbToDomains(verb string) []string {
	verb = strings.TrimPrefix(verb, "/")
	switch verb {
	case "fix", "debug":
		return []string{"diagnostic", "world_model"}
	case "test":
		return []string{"shard_tester"}
	case "review":
		return []string{"shard_reviewer"}
	case "implement", "scaffold", "generate":
		return []string{"shard_coder", "world_model"}
	case "research", "explore":
		return []string{"memory"}
	default:
		return nil
	}
}

// FormatForPrompt formats selected predicates for prompt injection.
func (ps *PredicateSelector) FormatForPrompt(predicates []SelectedPredicate) string {
	var sb strings.Builder

	sb.WriteString("## Available Mangle Predicates\n\n")
	sb.WriteString("Use ONLY these predicates in your rules:\n\n")

	// Group by domain
	byDomain := make(map[string][]SelectedPredicate)
	for _, p := range predicates {
		byDomain[p.Domain] = append(byDomain[p.Domain], p)
	}

	// Get sorted domain names
	var domains []string
	for d := range byDomain {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		preds := byDomain[domain]
		sb.WriteString(fmt.Sprintf("### %s\n", domain))
		for _, p := range preds {
			sb.WriteString(fmt.Sprintf("- `%s/%d`", p.Name, p.Arity))
			if p.Description != "" {
				// Truncate long descriptions
				desc := p.Description
				if len(desc) > 60 {
					desc = desc[:60] + "..."
				}
				sb.WriteString(fmt.Sprintf(" - %s", desc))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SelectForMangleGeneration is a convenience method for Mangle rule generation contexts.
// It selects predicates appropriate for generating new Mangle rules.
func (ps *PredicateSelector) SelectForMangleGeneration(shardType, intentVerb string) ([]SelectedPredicate, error) {
	ctx := SelectionContext{
		ShardType:     shardType,
		IntentVerb:    intentVerb,
		MaxPredicates: 80, // Smaller set for generation
		Domains:       []string{"shard_lifecycle", "routing"},
	}
	return ps.Select(ctx)
}

// SelectForRepair selects predicates appropriate for rule repair contexts.
// Includes error-prone predicates and their correct alternatives.
func (ps *PredicateSelector) SelectForRepair(errorTypes []string) ([]SelectedPredicate, error) {
	limit := ps.maxPredicates
	if limit <= 0 {
		limit = defaultPredicateLimit
	}
	ctx := SelectionContext{
		MaxPredicates: limit,
		Domains:       []string{"core", "safety", "routing"},
	}

	// Add domains based on error types
	for _, errType := range errorTypes {
		switch {
		case strings.Contains(errType, "shard"):
			ctx.Domains = append(ctx.Domains, "shard_lifecycle")
		case strings.Contains(errType, "campaign"):
			ctx.Domains = append(ctx.Domains, "campaign")
		case strings.Contains(errType, "tool"):
			ctx.Domains = append(ctx.Domains, "tool")
		}
	}

	base, err := ps.Select(ctx)
	if err != nil {
		return nil, err
	}

	query := "mangle repair"
	if len(errorTypes) > 0 {
		query = query + " " + strings.Join(errorTypes, " ")
	}

	return ps.mergePredicatesWithVector(context.Background(), base, query, limit), nil
}

// GetPredicateSignatures returns formatted signatures for a list of predicate names.
func (ps *PredicateSelector) GetPredicateSignatures(names []string) []string {
	var signatures []string
	for _, name := range names {
		if sig, err := ps.corpus.FormatPredicateSignature(name, true); err == nil {
			signatures = append(signatures, sig)
		}
	}
	return signatures
}

// SelectForContext implements PredicateSelectorInterface for FeedbackLoop integration.
// Returns predicate signatures relevant to the given context.
func (ps *PredicateSelector) SelectForContext(ctx context.Context, shardType, intentVerb, domain, query string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	maxPredicates := ps.maxPredicates
	if maxPredicates <= 0 {
		maxPredicates = defaultPredicateLimit
	}
	selection := SelectionContext{
		ShardType:     shardType,
		IntentVerb:    intentVerb,
		MaxPredicates: maxPredicates,
	}

	// Map domain to relevant predicate domains
	if domain != "" {
		cleanDomain := strings.TrimSpace(strings.TrimPrefix(domain, "/"))
		switch cleanDomain {
		case "legislator", "mangle", "policy":
			selection.Domains = []string{"core", "routing", "safety", "shard_lifecycle"}
		case "executive", "action":
			selection.Domains = []string{"core", "routing", "shard_lifecycle"}
		case "constitution", "safety":
			selection.Domains = []string{"core", "safety", "routing"}
		case "campaign":
			selection.Domains = []string{"core", "campaign", "shard_lifecycle"}
		default:
			selection.Domains = []string{cleanDomain}
		}
	}

	base, err := ps.Select(selection)
	if err != nil {
		return nil, err
	}

	merged := ps.mergePredicatesWithVector(ctx, base, query, maxPredicates)

	// Convert to signature strings
	signatures := make([]string, 0, len(merged))
	for _, p := range merged {
		sig := fmt.Sprintf("%s/%d", p.Name, p.Arity)
		if p.Description != "" {
			// Truncate description for readability
			desc := p.Description
			if len(desc) > 50 {
				desc = desc[:50] + "..."
			}
			sig += " - " + desc
		}
		signatures = append(signatures, sig)
	}

	if len(signatures) > maxPredicates {
		signatures = signatures[:maxPredicates]
	}
	return signatures, nil
}

func (ps *PredicateSelector) mergePredicatesWithVector(ctx context.Context, base []SelectedPredicate, query string, limit int) []SelectedPredicate {
	combined := make(map[string]SelectedPredicate)
	for _, p := range base {
		combined[predicateKey(p.Name, p.Arity)] = p
	}

	for _, p := range ps.selectVectorPredicates(ctx, query) {
		key := predicateKey(p.Name, p.Arity)
		if existing, ok := combined[key]; !ok || p.Relevance > existing.Relevance {
			combined[key] = p
		}
	}

	merged := make([]SelectedPredicate, 0, len(combined))
	for _, p := range combined {
		merged = append(merged, p)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Relevance == merged[j].Relevance {
			return merged[i].Name < merged[j].Name
		}
		return merged[i].Relevance > merged[j].Relevance
	})

	if limit <= 0 {
		limit = defaultPredicateLimit
	}
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

func (ps *PredicateSelector) selectVectorPredicates(ctx context.Context, query string) []SelectedPredicate {
	query = strings.TrimSpace(query)
	if query == "" || ps.vectorStore == nil || ps.corpus == nil {
		return nil
	}

	ps.ensurePredicateVectorIndex()

	limit := ps.vectorLimit
	if limit <= 0 {
		limit = defaultPredicateVecLimit
	}
	results, err := ps.vectorStore.VectorRecallSemanticFiltered(ctx, query, limit, predicateVectorMetaKey, predicateVectorMetaValue)
	if err != nil {
		logging.Get(logging.CategoryKernel).Debug("PredicateSelector: vector recall failed: %v", err)
		return nil
	}

	predicates := make([]SelectedPredicate, 0, len(results))
	for _, entry := range results {
		name, arity, ok := parsePredicateMeta(entry.Metadata, entry.Content)
		if !ok {
			continue
		}
		info, err := ps.corpus.GetPredicate(name)
		if err != nil || info == nil {
			continue
		}
		relevance := similarityFromMetadata(entry.Metadata)
		selectedArity := info.Arity
		if arity > 0 {
			selectedArity = arity
		}
		predicates = append(predicates, SelectedPredicate{
			Name:        info.Name,
			Arity:       selectedArity,
			Domain:      info.Domain,
			Category:    info.Category,
			Description: info.Description,
			Relevance:   relevance,
		})
	}
	return predicates
}

func (ps *PredicateSelector) ensurePredicateVectorIndex() {
	if ps.vectorStore == nil || ps.corpus == nil {
		return
	}
	if ps.vectorIndexReady.Load() {
		return
	}
	if !ps.vectorIndexRunning.CompareAndSwap(false, true) {
		return
	}
	defer ps.vectorIndexRunning.Store(false)
	if ps.indexPredicateVectors() {
		ps.vectorIndexReady.Store(true)
	}
}

func (ps *PredicateSelector) indexPredicateVectors() bool {
	if ps.vectorStore == nil || ps.corpus == nil {
		return false
	}

	stats, err := ps.vectorStore.GetVectorStats()
	if err == nil {
		if engine, ok := stats["embedding_engine"].(string); ok && strings.HasPrefix(engine, "none") {
			return false
		}
	}

	corpusStats, err := ps.corpus.Stats()
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("PredicateSelector: failed to read corpus stats: %v", err)
	}
	totalPredicates := corpusStats["total_predicates"]

	existing, err := ps.vectorStore.CountVectorsByMetadata(predicateVectorMetaKey, predicateVectorMetaValue)
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("PredicateSelector: failed to count predicate vectors: %v", err)
	}

	if totalPredicates > 0 && existing >= totalPredicates {
		return true
	}

	predicates, err := ps.corpus.GetAllPredicates()
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("PredicateSelector: failed to load predicates for vector index: %v", err)
		return false
	}

	logging.Get(logging.CategoryKernel).Debug("PredicateSelector: indexing %d predicates for vector search", len(predicates))

	var existingContents map[string]struct{}
	if existing > 0 && totalPredicates > 0 && existing < totalPredicates {
		contents, err := ps.vectorStore.VectorContentsByMetadata(predicateVectorMetaKey, predicateVectorMetaValue)
		if err != nil {
			logging.Get(logging.CategoryKernel).Warn("PredicateSelector: failed to load existing predicate vectors: %v", err)
		} else {
			existingContents = contents
		}
	}

	ctx := context.Background()
	contents := make([]string, 0, len(predicates))
	metadata := make([]map[string]interface{}, 0, len(predicates))
	for _, p := range predicates {
		content := predicateVectorContent(p)
		if existingContents != nil {
			if _, ok := existingContents[content]; ok {
				continue
			}
		}
		contents = append(contents, content)
		metadata = append(metadata, map[string]interface{}{
			predicateVectorMetaKey: predicateVectorMetaValue,
			"name":                 p.Name,
			"arity":                p.Arity,
			"domain":               p.Domain,
			"category":             p.Category,
			"content_type":         "knowledge_atom",
		})
	}
	if len(contents) == 0 {
		return false
	}
	if _, err := ps.vectorStore.StoreVectorBatchWithEmbedding(ctx, contents, metadata); err != nil {
		logging.Get(logging.CategoryKernel).Warn("PredicateSelector: batch index failed: %v", err)
		return false
	}

	if totalPredicates <= 0 {
		totalPredicates = len(predicates)
	}
	existing, err = ps.vectorStore.CountVectorsByMetadata(predicateVectorMetaKey, predicateVectorMetaValue)
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("PredicateSelector: failed to count predicate vectors after index: %v", err)
		return false
	}
	return totalPredicates > 0 && existing >= totalPredicates
}

func predicateVectorContent(p core.PredicateInfo) string {
	var sb strings.Builder
	sb.WriteString("predicate ")
	sb.WriteString(p.Name)
	sb.WriteString("/")
	sb.WriteString(fmt.Sprintf("%d", p.Arity))
	if p.Domain != "" {
		sb.WriteString(" domain:")
		sb.WriteString(p.Domain)
	}
	if p.Category != "" {
		sb.WriteString(" category:")
		sb.WriteString(p.Category)
	}
	if p.Description != "" {
		sb.WriteString(" ")
		sb.WriteString(p.Description)
	}
	return sb.String()
}

func parsePredicateMeta(meta map[string]interface{}, content string) (string, int, bool) {
	var name string
	var arity int

	if v, ok := meta["name"].(string); ok {
		name = v
	}
	switch v := meta["arity"].(type) {
	case float64:
		arity = int(v)
	case int:
		arity = v
	case int64:
		arity = int(v)
	}

	if name == "" && content != "" {
		fields := strings.Fields(content)
		if len(fields) > 1 {
			parts := strings.SplitN(fields[1], "/", 2)
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) == 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil {
					arity = n
				}
			}
		}
	}

	if name == "" || arity <= 0 {
		return "", 0, false
	}
	return name, arity, true
}

func similarityFromMetadata(meta map[string]interface{}) float64 {
	switch v := meta["similarity"].(type) {
	case float64:
		return clampSimilarity(v)
	case float32:
		return clampSimilarity(float64(v))
	case int:
		return clampSimilarity(float64(v))
	default:
		return 0.6
	}
}

func clampSimilarity(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func predicateKey(name string, arity int) string {
	return fmt.Sprintf("%s/%d", name, arity)
}

// AllPredicateSignatures returns every predicate in the corpus as name/arity.
func (ps *PredicateSelector) AllPredicateSignatures() ([]string, error) {
	if ps.corpus == nil {
		return nil, fmt.Errorf("no predicate corpus configured")
	}
	return ps.corpus.GetAllPredicateSignatures()
}
