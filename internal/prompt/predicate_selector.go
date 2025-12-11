// Package prompt provides the JIT Predicate Selector for context-aware predicate injection.
// It selects the most relevant predicates from the corpus based on current compilation context.
package prompt

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/core"
)

// PredicateSelector selects predicates for JIT prompt injection.
// Instead of injecting all 799 predicates, it selects ~50-100 based on:
// - Compilation context (shard type, intent, phase)
// - Domain relevance
// - Semantic similarity to the task
type PredicateSelector struct {
	corpus *core.PredicateCorpus
}

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
	return &PredicateSelector{corpus: corpus}
}

// Select chooses predicates based on the compilation context.
func (ps *PredicateSelector) Select(ctx SelectionContext) ([]SelectedPredicate, error) {
	if ps.corpus == nil {
		return nil, fmt.Errorf("no predicate corpus configured")
	}

	// Set default max
	if ctx.MaxPredicates <= 0 {
		ctx.MaxPredicates = 100
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
	ctx := SelectionContext{
		MaxPredicates: 60,
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

	return ps.Select(ctx)
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
func (ps *PredicateSelector) SelectForContext(shardType, intentVerb, domain string) ([]string, error) {
	ctx := SelectionContext{
		ShardType:     shardType,
		IntentVerb:    intentVerb,
		MaxPredicates: 100,
	}

	// Map domain to relevant predicate domains
	if domain != "" {
		switch domain {
		case "executive", "action":
			ctx.Domains = []string{"core", "routing", "shard_lifecycle"}
		case "constitution", "safety":
			ctx.Domains = []string{"core", "safety", "routing"}
		case "campaign":
			ctx.Domains = []string{"core", "campaign", "shard_lifecycle"}
		default:
			ctx.Domains = []string{domain}
		}
	}

	predicates, err := ps.Select(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to signature strings
	signatures := make([]string, 0, len(predicates))
	for _, p := range predicates {
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

	return signatures, nil
}
