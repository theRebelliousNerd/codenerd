package marathon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Researcher gathers the prompting documentation for one specific model.
//
// It is deliberately narrow. It does not search for prompt engineering advice
// in general, best-practice listicles, or techniques that worked on some other
// model -- the corpus is already full of that, and the marathon exists to
// replace generic guidance with vendor-specific guidance. The queries name the
// model and ask for its own documentation.
type Researcher struct {
	searcher types.GroundedWebSearcher
}

// NewResearcher returns a Researcher over the given client.
//
// The client must implement types.GroundedWebSearcher AND report that grounded
// search is actually available; a client that satisfies the interface but
// returns false from SupportsGroundedWebSearch is rejected here rather than
// producing an empty profile later. grounded_web_search is the only web route
// with a safe_action entry in the constitution, so there is no fallback to
// offer.
func NewResearcher(client any) (*Researcher, error) {
	searcher, ok := client.(types.GroundedWebSearcher)
	if !ok {
		return nil, fmt.Errorf("%w (client type %T)", ErrNoGroundedSearch, client)
	}
	if !searcher.SupportsGroundedWebSearch() {
		return nil, fmt.Errorf("%w (client reports grounding unavailable)", ErrNoGroundedSearch)
	}
	return &Researcher{searcher: searcher}, nil
}

// researchQueries are run in order and their results concatenated. They are
// separate queries rather than one broad one because grounded search returns
// citations per call, and a single omnibus query reliably grounds on whichever
// facet the retriever liked best while silently dropping the others.
func researchQueries(provider, model string) []string {
	id := strings.TrimSpace(model)
	if id == "" {
		id = provider
	}
	return []string{
		fmt.Sprintf("%s %s official prompt engineering guide and documentation", provider, id),
		fmt.Sprintf("%s system prompt best practices and instruction following characteristics", id),
		fmt.Sprintf("%s known prompting failure modes, formatting requirements, and tool use conventions", id),
	}
}

// Research builds the documentation profile for the serving model.
//
// It fails hard in three cases: unknown serving identity, unavailable grounded
// search (both handled by the caller and NewResearcher), and -- here -- a search
// that produced no citations. The last one is the subtle one: grounded search
// answers happily from the model's own priors when retrieval finds nothing, so
// a non-empty response text is NOT evidence that documentation was found.
// Citations are. A profile with prose and no sources is exactly the fabricated
// authority this whole gate exists to prevent.
func (r *Researcher) Research(ctx context.Context, provider, model string) (*ModelDocProfile, error) {
	if strings.TrimSpace(provider) == "" && strings.TrimSpace(model) == "" {
		return nil, ErrNoModelIdentity
	}

	timer := logging.StartTimer(logging.CategoryJIT, "Marathon.Research")
	defer timer.Stop()

	profile := &ModelDocProfile{
		Provider:     provider,
		Model:        model,
		ProviderPin:  prompt.NormalizeProviderToken(provider),
		ModelPin:     modelPinToken(model),
		ResearchedAt: time.Now(),
	}

	var (
		sections []string
		seen     = map[string]struct{}{}
	)

	for _, query := range researchQueries(provider, model) {
		result, err := r.searcher.GroundedWebSearch(ctx, query)
		if err != nil {
			// One failed query is survivable; the citation check below decides
			// whether enough was found overall.
			logging.Get(logging.CategoryJIT).Warn("Marathon research query failed (%q): %v", query, err)
			continue
		}
		if result == nil {
			continue
		}

		if text := strings.TrimSpace(result.Text); text != "" {
			sections = append(sections, text)
		}
		for _, c := range result.Citations {
			url := strings.TrimSpace(c.URL)
			if url == "" {
				continue
			}
			if _, dup := seen[url]; dup {
				continue
			}
			seen[url] = struct{}{}
			profile.Citations = append(profile.Citations, Citation{URL: url, Title: c.Title})
		}
	}

	profile.Guidance = strings.Join(sections, "\n\n---\n\n")

	if !profile.HasDocs() {
		return nil, fmt.Errorf("%w: provider=%q model=%q (%d citations, %d chars of guidance)",
			ErrNoModelDocs, provider, model, len(profile.Citations), len(profile.Guidance))
	}

	logging.Get(logging.CategoryJIT).Info(
		"Marathon research complete: model=%s citations=%d guidance=%d chars",
		model, len(profile.Citations), len(profile.Guidance))

	return profile, nil
}

// modelPinToken returns the family token for the serving model, falling back to
// the exact token when no distinct family exists.
//
// Family granularity is deliberate: an optimization researched from a model's
// documentation describes the model, not one dated snapshot of it, and pinning
// to the snapshot would retire the entire overlay on the vendor's next release.
func modelPinToken(model string) string {
	tokens := prompt.ModelPinTokens(model)
	if len(tokens) == 0 {
		return ""
	}
	// ModelPinTokens returns [exact] or [exact, family]; prefer the family.
	return tokens[len(tokens)-1]
}

// ServingIdentity resolves the provider and model a client will serve, or fails.
func ServingIdentity(client any) (provider, model string, err error) {
	identifier, ok := client.(types.ModelIdentifier)
	if !ok {
		return "", "", fmt.Errorf("%w (client type %T)", ErrNoModelIdentity, client)
	}
	provider, model = identifier.ModelIdentity()
	if strings.TrimSpace(provider) == "" && strings.TrimSpace(model) == "" {
		return "", "", ErrNoModelIdentity
	}
	return provider, model, nil
}
