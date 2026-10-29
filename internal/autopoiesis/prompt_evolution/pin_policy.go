package prompt_evolution

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/prompt"
)

// ErrUnpinnedEvidence is the sentinel behind UnpinnedEvidenceError: the
// failures cannot be pinned to the model and role that produced them, so no
// atom is generated from them.
var ErrUnpinnedEvidence = errors.New("evidence cannot be pinned to a model and role")

// UnpinnedEvidenceError refuses an atom whose evidence lacks the provider,
// model or role its pin scope needs. An atom with a missing selector would be
// served to every model (or every role), which is the failure pinning exists
// to prevent, so the atom is not produced at all.
type UnpinnedEvidenceError struct {
	Scope     PinScope
	ShardType string
	Pin       ServingPin
	Missing   string // "provider", "model" or "role"
}

func (e *UnpinnedEvidenceError) Error() string {
	return fmt.Sprintf("no pin, no atom: %s missing (pin_scope=%s, role=%q, serving=%s)",
		e.Missing, e.Scope, e.ShardType, e.Pin)
}

// Unwrap lets errors.Is(err, ErrUnpinnedEvidence) match.
func (e *UnpinnedEvidenceError) Unwrap() error { return ErrUnpinnedEvidence }

// requirePin reports whether the evidence carries everything scope needs to
// pin an atom: the source role always, the provider always, and the model for
// every scope narrower than provider.
func requirePin(scope PinScope, shardType string, pin ServingPin) error {
	refuse := func(missing string) error {
		return &UnpinnedEvidenceError{Scope: scope, ShardType: shardType, Pin: pin, Missing: missing}
	}
	if strings.TrimSpace(shardType) == "" {
		return refuse("role")
	}
	if prompt.NormalizeProviderToken(pin.Provider) == "" {
		return refuse("provider")
	}
	if scope != PinScopeProvider && prompt.NormalizeModelToken(pin.Model) == "" {
		return refuse("model")
	}
	return nil
}

// applyPin stamps the atom's role, provider and model selectors. The role is
// exactly the source shard type: whatever extra roles the generating model
// suggested are ignored, because widening to another role needs evidence from
// that role. Selectors are overwritten, not merged.
func applyPin(atom *prompt.PromptAtom, scope PinScope, shardType string, pin ServingPin) {
	atom.ShardTypes = []string{strings.TrimSpace(shardType)}
	atom.Providers, atom.Models = pinSelectors(scope, pin)
	atom.NormalizeSelectors()
}

// pinnedSelectors is the selector triple applyPin controls, for comparison.
type pinnedSelectors struct {
	roles, providers, models []string
}

func selectorsOf(atom *prompt.PromptAtom) pinnedSelectors {
	clone := *atom
	clone.ShardTypes = slices.Clone(atom.ShardTypes)
	clone.Providers = slices.Clone(atom.Providers)
	clone.Models = slices.Clone(atom.Models)
	clone.NormalizeSelectors()
	return pinnedSelectors{roles: clone.ShardTypes, providers: clone.Providers, models: clone.Models}
}

func (s pinnedSelectors) equal(o pinnedSelectors) bool {
	return slices.Equal(s.roles, o.roles) && slices.Equal(s.providers, o.providers) && slices.Equal(s.models, o.models)
}
