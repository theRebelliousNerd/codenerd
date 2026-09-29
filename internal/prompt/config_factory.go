package prompt

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ConfigAtom represents a configuration fragment associated with an intent.
type ConfigAtom struct {
	Tools    []string
	Policies []string
	Priority int
}

// Merge combines two ConfigAtoms.
// Tools and Policies are merged and deduplicated.
// The higher priority is kept.
func (c ConfigAtom) Merge(other ConfigAtom) ConfigAtom {
	merged := ConfigAtom{
		Tools:    uniqueStrings(append(c.Tools, other.Tools...)),
		Policies: uniqueStrings(append(c.Policies, other.Policies...)),
		Priority: c.Priority,
	}

	if other.Priority > c.Priority {
		merged.Priority = other.Priority
	}

	return merged
}

func uniqueStrings(input []string) []string {
	const MaxItems = 1000 // Prevent massive DoS

	keys := make(map[string]bool)
	capacity := len(input)
	if capacity > MaxItems {
		capacity = MaxItems
	}
	list := make([]string, 0, capacity) // pre-allocate capacity

	for _, entry := range input {
		if len(list) >= MaxItems {
			break
		}
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry) // copies the string reference, safe
		}
	}
	return list
}

// Clone creates a deep copy of the ConfigAtom to prevent data races.
func (c ConfigAtom) Clone() ConfigAtom {
	tools := make([]string, len(c.Tools))
	copy(tools, c.Tools)

	policies := make([]string, len(c.Policies))
	copy(policies, c.Policies)

	return ConfigAtom{
		Tools:    tools,
		Policies: policies,
		Priority: c.Priority,
	}
}

// ConfigAtomProvider defines the interface for retrieving config atoms.
type ConfigAtomProvider interface {
	GetAtom(intent string) (ConfigAtom, bool)
}

// ConfigFactory generates EffectiveAgentRuntimeConfig objects.
type ConfigFactory struct {
	provider ConfigAtomProvider
}

// NewConfigFactory creates a new ConfigFactory.
func NewConfigFactory(provider ConfigAtomProvider) *ConfigFactory {
	return &ConfigFactory{
		provider: provider,
	}
}

// Generate creates an EffectiveAgentRuntimeConfig based on the intents and compilation result.
// It merges config atoms for all provided intents.
func (f *ConfigFactory) Generate(ctx context.Context, result *CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if f.provider == nil {
		return nil, fmt.Errorf("config provider cannot be nil")
	}
	if result == nil {
		return nil, fmt.Errorf("compilation result cannot be nil")
	}
	if len(intents) == 0 {
		return nil, fmt.Errorf("no intents provided")
	}
	var finalAtom ConfigAtom
	found := false

	for _, rawIntent := range intents {
		intent := strings.TrimSpace(rawIntent)
		if atom, ok := f.provider.GetAtom(intent); ok {
			finalAtom = finalAtom.Merge(atom)
			found = true
			continue
		}
		// An unregistered intent still merges the /general atom so the turn
		// has a policy anchor. The tool floor is turn_tool_allowed: an
		// unknown verb derives the /general envelope, and an empty
		// derivation fail-closes. This atom is not that catalog.
		//
		// The fallback used to apply only to "/consult/<persona>" specialists,
		// so every other unregistered verb produced AllowedTools == nil. The
		// caller logged a WARN, kept the empty config, and answered with no
		// way to read a file. `nerd explain <file>` said it was reading the
		// file and exited 0. Degrade loudly, don't silently disarm.
		if atom, ok := f.provider.GetAtom("/general"); ok {
			logging.Get(logging.CategoryContext).Warn(
				"No config atom for intent %q; falling back to /general policies. "+
					"The tool envelope is turn_tool_allowed, not this atom. "+
					"Canonical verbs belong in NewDefaultConfigAtomProvider.", intent)
			finalAtom = finalAtom.Merge(atom)
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("no config atoms found for intents: %v", intents)
	}

	// Determine primary intent for the config
	primaryIntent := intents[0]

	// AllowedTools is whatever the atom itself declared. The default atoms
	// declare none: the turn catalog is DeriveTurnTools, and the session
	// executor and spawner overlay that derivation onto this config. A
	// custom atom's Tools are not a second built-in catalog.
	cfg := &config.EffectiveAgentRuntimeConfig{
		IdentityPrompt: result.Prompt,
		IntentVerb:     primaryIntent,
		AllowedTools:   finalAtom.Tools,
		Policies:       finalAtom.Policies,
	}

	// The factory is the main-turn producer of runtime configs and was the one
	// path that never validated: an empty Policies slice passed silently while
	// specialist YAML is rejected for the same defect. The single tool budget
	// lives on the session ExecutorConfig (fed from core_limits in factory.go),
	// so this config carries no loop limits of its own.
	if err := cfg.Validate(); err != nil {
		logging.Get(logging.CategoryContext).Warn(
			"Generated config for intent %q failed validation: %v", primaryIntent, err)
		return nil, err
	}

	return cfg, nil
}

// DeriveTurnTools is the turn's tool envelope: whatever
// turn_tool_allowed(Verb, Tool) derives (policy/intent_routing_rules.mg),
// before a turn's withholdings narrow it. The session executor, the
// spawner and the prompt tests all call this so they name the same verb
// the policy does. Prompt cannot import session, which is why it lives
// here rather than next to the executor.
//
// Only the verb naming happens here, and it mirrors the factory's lookup.
// An empty verb is /general. /consult/<name> resolves to /<name> (GetAtom).
// /generate-tool is the one hyphenated alias, rewritten to /generate_tool
// because a Mangle atom cannot spell a hyphen and no other registered verb
// contains one. Anything not shaped like a Mangle atom falls back to
// /general. Which tools a verb gets is the kernel's answer.
//
// A verb with no verb_persona fact gets the /general floor. The policy
// derives that floor from user_intent when a turn has asserted one; the
// spawner compiles a config without asserting user_intent, so the same
// floor is read here when verb_has_persona is false. A persona-bearing
// verb that derives nothing is a broken projection: an empty catalog is
// not "all tools", and the caller fail-closes.
func DeriveTurnTools(kernel types.Kernel, verb string) ([]string, error) {
	if kernel == nil {
		return nil, fmt.Errorf("turn catalog: no kernel to derive the tool envelope from")
	}
	verb = canonicalTurnVerb(verb)
	derived, err := queryTurnTools(kernel, verb)
	if err != nil {
		return nil, err
	}
	if len(derived) == 0 {
		has, herr := kernel.Query(fmt.Sprintf("verb_has_persona(%s)", verb))
		if herr != nil {
			return nil, fmt.Errorf("turn catalog: verb_has_persona(%s) failed: %w", verb, herr)
		}
		if len(has) == 0 && verb != "/general" {
			derived, err = queryTurnTools(kernel, "/general")
			if err != nil {
				return nil, err
			}
		}
		if len(derived) == 0 {
			return nil, fmt.Errorf("turn catalog: turn_tool_allowed(%s, Tool) derived no tools; the policy projection is missing or broken", verb)
		}
	}
	slices.Sort(derived)
	return derived, nil
}

// canonicalTurnVerb is the atom DeriveTurnTools queries. It is the same
// normalization GetAtom and validMangleVerb apply at the two call sites:
// empty and non-atoms become /general, a consult prefix resolves to the
// remainder, and the one hyphenated alias is spelled with an underscore.
func canonicalTurnVerb(verb string) string {
	verb = strings.TrimSpace(verb)
	if verb == "" {
		return "/general"
	}
	if after, found := strings.CutPrefix(verb, "/consult/"); found {
		verb = "/" + strings.ToLower(strings.TrimSpace(after))
	}
	verb = strings.ReplaceAll(verb, "-", "_")
	if !queryableIntentVerb(verb) {
		logging.Get(logging.CategorySession).Warn(
			"turn catalog: %q is not a queryable intent verb; falling back to /general read-only tools", verb)
		return "/general"
	}
	return verb
}

// queryableIntentVerb admits only the atom shape the policy corpus uses.
// The verb is interpolated into a query, so a second slash, whitespace or
// an uppercase letter would turn caller state into a query fragment.
// Same predicate as session.validMangleVerb.
func queryableIntentVerb(verb string) bool {
	if len(verb) < 2 || verb[0] != '/' {
		return false
	}
	for i := 1; i < len(verb); i++ {
		c := verb[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

func queryTurnTools(kernel types.Kernel, verb string) ([]string, error) {
	facts, err := kernel.Query(fmt.Sprintf("turn_tool_allowed(%s, Tool)", verb))
	if err != nil {
		return nil, fmt.Errorf("turn catalog: turn_tool_allowed(%s, Tool) failed: %w", verb, err)
	}
	seen := make(map[string]struct{}, len(facts))
	derived := make([]string, 0, len(facts))
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		tool := strings.TrimPrefix(types.ExtractString(f.Args[len(f.Args)-1]), "/")
		if tool == "" {
			continue
		}
		if _, dup := seen[tool]; dup {
			continue
		}
		seen[tool] = struct{}{}
		derived = append(derived, tool)
	}
	return derived, nil
}

// GenerateFallback creates a minimal config for when JIT compilation fails.
func (f *ConfigFactory) GenerateFallback(ctx context.Context, intent string, fallbackIdentity string) *config.EffectiveAgentRuntimeConfig {
	// Prevent OOM from massive fallback strings
	const MaxFallbackLength = 1024 * 1024 // 1MB limit
	if len(fallbackIdentity) > MaxFallbackLength {
		// Truncating by bytes can slice a multibyte UTF-8 character in half, resulting in invalid UTF-8.
		// It should truncate on rune boundaries.
		truncateIdx := MaxFallbackLength
		for truncateIdx > 0 && !utf8.RuneStart(fallbackIdentity[truncateIdx]) {
			truncateIdx--
		}
		fallbackIdentity = fallbackIdentity[:truncateIdx]
	}

	intent = strings.TrimSpace(intent)
	var finalAtom ConfigAtom
	if f.provider != nil {
		if atom, ok := f.provider.GetAtom(intent); ok {
			finalAtom = atom
		} else if atom, ok := f.provider.GetAtom("/general"); ok {
			finalAtom = atom
		}
	}

	return &config.EffectiveAgentRuntimeConfig{
		IdentityPrompt: fallbackIdentity,
		IntentVerb:     intent,
		AllowedTools:   finalAtom.Tools,
		Policies:       finalAtom.Policies,
	}
}

// =============================================================================
// DEFAULT CONFIG ATOM PROVIDER
// =============================================================================
// Provides built-in config atoms for common intents. This maps intent verbs
// to allowed tools and policies.

// DefaultConfigAtomProvider provides built-in config atoms.
type DefaultConfigAtomProvider struct {
	atoms map[string]ConfigAtom
	mu    sync.RWMutex
}

func mustDefaultPolicySet(setID string) []string {
	files, ok := core.DefaultAgentPolicySetFiles(setID)
	if !ok {
		panic(fmt.Sprintf("unknown default agent policy set %q", setID))
	}
	return files
}

func copyPolicySet(setID string) []string {
	return append([]string(nil), mustDefaultPolicySet(setID)...)
}

// NewDefaultConfigAtomProvider creates a new default config provider.
func NewDefaultConfigAtomProvider() *DefaultConfigAtomProvider {
	provider := &DefaultConfigAtomProvider{
		atoms: make(map[string]ConfigAtom),
	}

	// These atoms carry policies and priority. The tool envelope is
	// turn_tool_allowed (policy/intent_routing_rules.mg), read by
	// DeriveTurnTools. A Tools slice here would be a second catalog.
	//
	// The verb lists must still cover every verb in
	// perception.DefaultTaxonomyData, each on the policy set its ShardType
	// names. They drifted once: the taxonomy grew to 36 verbs while this
	// file still listed 9, an unregistered verb had no atom, and
	// `nerd explain <file>` had nothing to anchor its policies to. The
	// tool half of that failure (an empty catalog, "reading the file now",
	// exit 0) is the kernel floor for /explain and for an unknown verb.
	// Non-canonical aliases stay: perception has historically emitted them.

	// Register coder intents
	for _, intent := range []string{
		"/fix", "/refactor", "/create", "/write", "/delete", "/debug",
		"/campaign", "/git", "/migrate", "/optimize", "/document",
		"/scaffold", "/format", "/deploy",
		// non-canonical aliases
		"/implement", "/modify", "/add", "/update",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetCoder),
			Priority: 100,
		}
	}

	// Register tester intents (ordinary)
	for _, intent := range []string{
		"/test", "/benchmark", "/profile",
		// non-canonical aliases
		"/cover",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetTester),
			Priority: 90,
		}
	}

	// Register verification intents (tester policies; grounded search is a
	// turn_tool_allowed fact, not a second tool list).
	for _, intent := range []string{
		"/verify", "/validate",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetTester),
			Priority: 90,
		}
	}

	// Register reviewer intents
	for _, intent := range []string{
		"/review", "/review_enhance", "/security", "/analyze", "/audit", "/lint",
		// non-canonical aliases
		"/check", "/inspect",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetReviewer),
			Priority: 80,
		}
	}

	// Register researcher intents
	for _, intent := range []string{
		"/explore", "/search", "/research", "/init",
		// non-canonical aliases
		"/learn", "/understand", "/find",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetResearcher),
			Priority: 70,
		}
	}

	// Nemesis/adversarial intents (attack persona)
	for _, intent := range []string{"/attack", "/break", "/exploit", "/fuzz", "/pentest", "/nemesis"} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetNemesis),
			Priority: 85, // Higher priority than reviewer
		}
	}

	// Tool generator intents. /generate-tool is the one hyphenated alias;
	// DeriveTurnTools rewrites it to /generate_tool before querying.
	for _, intent := range []string{
		"/generate_tool",
		// non-canonical aliases
		"/generate", "/generate-tool", "/tool_generator", "/create_tool",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetToolGenerator),
			Priority: 75,
		}
	}

	// General/fallback intent. Policies only; the core tool floor is the
	// /general persona in the kernel.
	provider.atoms["/general"] = ConfigAtom{
		Policies: copyPolicySet(core.PolicySetBase),
		Priority: 50,
	}

	// Taxonomy verbs whose declared ShardType is /none. They route no
	// persona. The atom exists so the base policies attach and /consult/<name>
	// does not log a missing-atom warning; the read-only tools are the
	// kernel's /general envelope.
	for _, intent := range []string{
		"/explain", "/read", "/stats", "/knowledge", "/help", "/greet",
		"/configure", "/dream", "/shadow", "/assault",
		"/converse", "/forget", "/remember",
		// Built-in consultable system shards. User agents get the same pair
		// from registerUserAgentConfigAtoms; without these, /consult/<name>
		// falls back to /general and logs a warning on every /clarify.
		"/requirements_interrogator", "/consult/requirements_interrogator",
	} {
		provider.atoms[intent] = ConfigAtom{
			Policies: copyPolicySet(core.PolicySetBase),
			Priority: 50,
		}
	}

	return provider
}

// GetAtom returns the config atom for an intent.
func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	atom, ok := p.atoms[intent]
	if ok {
		return atom.Clone(), true
	}
	// Chat delegation uses /consult/<name> while spawn uses /<name>.
	if after, found := strings.CutPrefix(intent, "/consult/"); found {
		if atom, ok = p.atoms["/"+strings.ToLower(strings.TrimSpace(after))]; ok {
			return atom.Clone(), true
		}
	}
	return atom, false
}

// RegisterAtom adds or updates a config atom for an intent.
func (p *DefaultConfigAtomProvider) RegisterAtom(intent string, atom ConfigAtom) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.atoms[intent] = atom.Clone()
}

// RegisteredIntents returns the sorted list of intent verbs the provider knows.
// Tests iterate this so every registered verb — not a hardcoded subset — is
// proven to generate a config that passes Validate.
func (p *DefaultConfigAtomProvider) RegisteredIntents() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	intents := make([]string, 0, len(p.atoms))
	for intent := range p.atoms {
		intents = append(intents, intent)
	}
	sort.Strings(intents)
	return intents
}
