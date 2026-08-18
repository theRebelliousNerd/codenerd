package marathon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"

	"gopkg.in/yaml.v3"
)

// Optimizer rewrites one atom at a time against a researched model profile.
type Optimizer struct {
	client  types.LLMClient
	profile *ModelDocProfile
}

// NewOptimizer returns an Optimizer bound to a researched profile.
// A profile without citations is refused: that is the state the research gate
// exists to reject, and accepting it here would route around the gate.
func NewOptimizer(client types.LLMClient, profile *ModelDocProfile) (*Optimizer, error) {
	if client == nil {
		return nil, fmt.Errorf("marathon: optimizer requires an LLM client")
	}
	if !profile.HasDocs() {
		return nil, fmt.Errorf("%w: refusing to optimize against an unsourced profile", ErrNoModelDocs)
	}
	return &Optimizer{client: client, profile: profile}, nil
}

// optimizerSystemPrompt is deliberately conservative. The corpus is the agent's
// own instructions; a rewrite that "improves style" while dropping a constraint
// is a regression that no test will catch, because the atom still reads well.
var optimizerSystemPrompt = `You adapt a single prompt atom for one specific LLM.

You are given documentation for the target model and the current text of one atom.
Rewrite the atom so it works better on THAT model specifically.

## Hard rules

1. PRESERVE MEANING. Every instruction, constraint, prohibition and edge case in
   the original must survive. You are changing how it is expressed, not what it
   requires. Dropping a constraint is the worst possible outcome.
2. GROUND EVERY CHANGE. Only make changes the supplied documentation supports.
   If the documentation says nothing relevant to this atom, return it unchanged.
3. NO PADDING. Do not add preamble, restatements, or motivational text. The atom
   competes for a token budget against every other atom.
4. KEEP THE VOICE. These are imperative instructions to an agent, not prose
   about an agent.

## When to return unchanged

Returning the original is the correct answer far more often than not. Say so
plainly via "changed: false". Rewriting an atom that needed no rewriting spends
budget and risks meaning drift for nothing.

## Output

Output ONLY valid YAML, no other text:

changed: true|false
rationale: "one line: what changed and which part of the documentation motivated it"
content: |
  the rewritten atom text (omit entirely when changed is false)`

// optimizeResponse is the YAML contract above.
type optimizeResponse struct {
	Changed   bool   `yaml:"changed"`
	Rationale string `yaml:"rationale"`
	Content   string `yaml:"content"`
}

// Optimize rewrites one atom, or reports that it should stay as it is.
// A nil OptimizedAtom with a nil error means "no change warranted".
func (o *Optimizer) Optimize(ctx context.Context, base *prompt.PromptAtom) (*OptimizedAtom, error) {
	if base == nil || strings.TrimSpace(base.Content) == "" {
		return nil, nil
	}

	response, err := o.client.CompleteWithSystem(ctx, optimizerSystemPrompt, o.buildUserPrompt(base))
	if err != nil {
		return nil, fmt.Errorf("optimize %s: %w", base.ID, err)
	}

	parsed, err := parseOptimizeResponse(response)
	if err != nil {
		return nil, fmt.Errorf("optimize %s: %w", base.ID, err)
	}
	if !parsed.Changed || strings.TrimSpace(parsed.Content) == "" {
		return nil, nil
	}

	variant := o.buildVariant(base, parsed.Content)

	return &OptimizedAtom{
		Atom:            variant,
		BaseID:          base.ID,
		BaseContentHash: base.ContentHash,
		Rationale:       strings.TrimSpace(parsed.Rationale),
		Citations:       o.profile.Citations,
		OptimizedAt:     time.Now(),
	}, nil
}

// buildVariant produces the pinned, superseding atom.
//
// Everything about the base is carried across unchanged except three things:
// the ID gains the model suffix, the pin selectors are set, and the variant
// declares supersession over the base. In particular the selector dimensions,
// category, priority and mandatory flag are inherited verbatim -- the variant
// must be selected in exactly the same situations as the atom it replaces, or
// it is not a replacement.
func (o *Optimizer) buildVariant(base *prompt.PromptAtom, content string) *prompt.PromptAtom {
	variant := base.Clone()
	variant.ID = OverlayAtomID(base.ID, o.profile.ModelPin)
	variant.Content = strings.TrimSpace(content)
	variant.ContentHash = prompt.HashContent(variant.Content)
	variant.TokenCount = prompt.EstimateTokens(variant.Content)
	variant.CreatedAt = time.Now()

	// Concise/min bodies were written for the original text and no longer
	// describe this one. Carrying them over would let the budget fall back to a
	// rendering of an atom that is no longer in the prompt.
	variant.ContentConcise = ""
	variant.ContentMin = ""

	// The pins. Provider may be empty for a local model with no vendor name; the
	// model pin alone is still enough to scope the variant.
	if o.profile.ProviderPin != "" {
		variant.Providers = []string{o.profile.ProviderPin}
	}
	if o.profile.ModelPin != "" {
		variant.Models = []string{o.profile.ModelPin}
	}

	// Supersession, one direction only: the variant replaces the base, never
	// the reverse. A mutual pair cancels both atoms in the kernel and deletes
	// the guidance outright (see TestSupersession_MutualConflictCancelsBoth).
	variant.ConflictsWith = append([]string(nil), base.ConflictsWith...)
	variant.ConflictsWith = append(variant.ConflictsWith, base.ID)

	// Priority is bumped so the variant also wins on the vector/flesh path,
	// where supersession does not apply and precedence is decided by score.
	if variant.Priority < 100 {
		variant.Priority++
	}

	return variant
}

func (o *Optimizer) buildUserPrompt(base *prompt.PromptAtom) string {
	var sb strings.Builder

	sb.WriteString("## Target model\n\n")
	sb.WriteString(fmt.Sprintf("- Provider: %s\n", o.profile.Provider))
	sb.WriteString(fmt.Sprintf("- Model: %s\n\n", o.profile.Model))

	sb.WriteString("## Documentation for this model\n\n")
	sb.WriteString(o.profile.Guidance)
	sb.WriteString("\n\n")

	sb.WriteString("## Atom to adapt\n\n")
	sb.WriteString(fmt.Sprintf("- ID: %s\n", base.ID))
	sb.WriteString(fmt.Sprintf("- Category: %s\n", base.Category))
	if base.IsMandatory {
		sb.WriteString("- Mandatory: yes (this atom is in every prompt it matches; " +
			"it must stay tight)\n")
	}
	sb.WriteString("\n```\n")
	sb.WriteString(base.Content)
	sb.WriteString("\n```\n")

	return sb.String()
}

// parseOptimizeResponse extracts the YAML contract from a model response,
// tolerating a fenced block.
func parseOptimizeResponse(response string) (*optimizeResponse, error) {
	content := strings.TrimSpace(response)

	if start := strings.Index(content, "```"); start >= 0 {
		rest := content[start+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.LastIndex(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		content = strings.TrimSpace(rest)
	}

	if content == "" {
		return nil, fmt.Errorf("empty optimizer response")
	}

	var parsed optimizeResponse
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("unparseable optimizer response: %w", err)
	}
	return &parsed, nil
}

// logOptimization records one decision at debug level.
func logOptimization(base *prompt.PromptAtom, optimized *OptimizedAtom) {
	if optimized == nil {
		logging.Get(logging.CategoryJIT).Debug("Marathon: %s unchanged", base.ID)
		return
	}
	logging.Get(logging.CategoryJIT).Debug("Marathon: %s -> %s (%s)",
		base.ID, optimized.Atom.ID, optimized.Rationale)
}
