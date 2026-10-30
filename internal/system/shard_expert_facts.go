package system

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/experts"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/orient"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Shard experts: the roster side of expert_needed (policy/shard_experts.mg).
//
// Boot measures which shard agents exist and what each is expert in, and
// asserts it; each task's side is asserted per selection by internal/experts.
// The kernel then decides which agent a task needs.
//
//	shard_agent(Agent)                       a ready agent in .nerd/agents.json
//	shard_agent_recorded(Agent)              its entry has a domains field
//	shard_agent_domain(Agent, Tech, Source)  /registry: its recorded domains;
//	                                         /knowledge_base: languages:/frameworks:
//	                                         tags on its KB atoms (which is where
//	                                         its prompts.yaml selectors land);
//	                                         /catalog: the init catalog's key for
//	                                         it, from the saved orientation
//	                                         (orient.AgentDomains), for an entry
//	                                         with no domains yet
//	technology_import_exact(Import, Tech)    init's technology vocabulary as
//	technology_import_prefix(Prefix, Tech)   import spellings (Go, npm, Python, Rust)
//	shard_agent_name_part(Agent, Part)       the words of each agent's name; the
//	                                         kernel derives /agent_name domains
//	                                         from them (a concept agent such as
//	                                         mangle-logic-architect has no
//	                                         registry domains)
//	domain_concept_term(Term, Domain)        init's concept vocabulary
//	domain_concept_verb(Domain, Verb)        (nerdinit.ConceptDomains)
//
// An entry written before agents.json had a domains field gets its domains
// derived at boot and recorded (shard_agent_domain_to_record), so an existing
// workspace heals on its next boot without re-running init, and pays for the
// derivation once.

// shardRoster is boot's measurement of the roster.
type shardRoster struct {
	facts []types.Fact
	// unrecorded are the roster agents (by key) whose entry has no domains.
	unrecorded []string
	// complete is true when every source of an unrecorded agent's domains
	// was read, so what the kernel derives for it may be recorded.
	complete bool
	// kbUnread are the agents (by atom) whose knowledge base could not be
	// read, and catalogUnread is the catalog's read failing: a source that
	// was not read measured nothing, so its last published support stands.
	kbUnread      map[types.MangleAtom]bool
	catalogUnread bool
}

// measureShardRoster builds the roster facts for a workspace. compiler, when
// non-nil, is asked for each agent's KB technologies (its registered DB). A
// source that cannot be read is reported beside the facts, never by dropping
// the agent.
func measureShardRoster(ctx context.Context, workspace string, compiler *prompt.JITPromptCompiler) (shardRoster, error) {
	roster := shardRoster{complete: true, kbUnread: map[types.MangleAtom]bool{}}
	var failures []string

	for _, root := range nerdinit.TechnologyImportRoots() {
		tech, ok := technologyAtom(root.Technology)
		if !ok {
			continue
		}
		roster.facts = append(roster.facts,
			types.Fact{Predicate: "technology_import_exact", Args: []any{root.Exact, tech}},
			types.Fact{Predicate: "technology_import_prefix", Args: []any{root.Prefix, tech}})
	}

	// The concept vocabulary: words that name a concept domain and the verbs
	// whose tasks need its experts.
	for _, concept := range nerdinit.ConceptDomains() {
		domain, ok := technologyAtom(concept.Key)
		if !ok {
			continue
		}
		for _, term := range concept.Terms {
			if t, ok := technologyAtom(term); ok {
				roster.facts = append(roster.facts, types.Fact{Predicate: "domain_concept_term", Args: []any{t, domain}})
			}
		}
		for _, verb := range concept.Verbs {
			if v, ok := technologyAtom(verb); ok {
				roster.facts = append(roster.facts, types.Fact{Predicate: "domain_concept_verb", Args: []any{domain, v}})
			}
		}
	}

	if strings.TrimSpace(workspace) == "" {
		return roster, nil
	}
	reg, _, err := loadAgentRegistry(filepath.Join(workspace, ".nerd", "agents.json"))
	if err != nil {
		roster.complete = false
		return roster, fmt.Errorf("read agents.json: %w", err)
	}
	for _, agent := range reg.Agents {
		if !rosterAgent(agent) {
			continue
		}
		atom := experts.AgentAtom(agent.Name)
		roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent", Args: []any{atom}})
		// What the user granted the agent beyond advice; the policy decides
		// what each grant admits (shard_agent_authority).
		for _, grant := range agent.Authority {
			if g := strings.ToLower(strings.TrimSpace(grant)); g != "" && experts.AgentKey(g) == g && !strings.ContainsAny(g, " /\t") {
				roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_authority", Args: []any{atom, types.MangleAtom("/" + g)}})
			}
		}
		// The words of the agent's name: the kernel derives from them the
		// languages, technologies and concepts the name states.
		for _, part := range nerdinit.AgentNameParts(agent.Name) {
			if p, ok := technologyAtom(part); ok {
				roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_name_part", Args: []any{atom, p}})
			}
		}
		if agent.Domains != nil {
			roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_recorded", Args: []any{atom}})
			for _, domain := range agent.Domains {
				if tech, ok := technologyAtom(domain); ok {
					roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_domain", Args: []any{atom, tech, types.MangleAtom("/registry")}})
				}
			}
		} else {
			roster.unrecorded = append(roster.unrecorded, experts.AgentKey(agent.Name))
		}
		if compiler == nil {
			continue
		}
		if _, registered := compiler.LookupShardDB(agent.Name); !registered {
			continue
		}
		techs, err := compiler.ShardTechnologies(ctx, agent.Name)
		if err != nil {
			roster.kbUnread[atom] = true
			failures = append(failures, err.Error())
			if agent.Domains == nil {
				roster.complete = false
			}
			continue
		}
		for _, raw := range techs {
			if tech, ok := technologyAtom(raw); ok {
				roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_domain", Args: []any{atom, tech, types.MangleAtom("/knowledge_base")}})
			}
		}
	}

	if len(roster.unrecorded) > 0 {
		catalog, err := orient.AgentDomains(ctx, workspace)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// Never oriented: no catalog created these agents, so the
			// catalog has nothing to say about them. What the KB claims is
			// still recorded.
		case err != nil:
			roster.complete = false
			roster.catalogUnread = true
			failures = append(failures, fmt.Sprintf("catalog domains: %v", err))
		default:
			byKey := make(map[string][]string, len(catalog))
			for name, keys := range catalog {
				byKey[experts.AgentKey(name)] = append(byKey[experts.AgentKey(name)], keys...)
			}
			for _, key := range roster.unrecorded {
				for _, raw := range byKey[key] {
					if tech, ok := technologyAtom(raw); ok {
						roster.facts = append(roster.facts, types.Fact{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom(key), tech, types.MangleAtom("/catalog")}})
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		return roster, fmt.Errorf("shard agent domains not fully read: %s", strings.Join(failures, "; "))
	}
	return roster, nil
}

// rosterAgent is an agent a task can need: a persistent (or user-defined)
// agent whose knowledge DB exists ("ready", as SyncAgentRegistryFromDisk
// measures it).
func rosterAgent(agent agentRegistryAgent) bool {
	if strings.TrimSpace(agent.Name) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(agent.Type)) {
	case "persistent", "user", "":
	default:
		return false
	}
	return strings.EqualFold(strings.TrimSpace(agent.Status), "ready")
}

// technologyAtom is a technology key as a name constant: lower-case, one
// segment, no whitespace ("go" -> /go). Anything else is not a key.
func technologyAtom(raw string) (types.MangleAtom, bool) {
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "/"))
	if key == "" || strings.ContainsAny(key, " \t\n\"/") {
		return "", false
	}
	return types.MangleAtom("/" + key), true
}

// refreshShardRoster measures the roster, asserts it, and records the domains
// the kernel derived for agents whose entry had none
// (shard_agent_domain_to_record). Asserting is idempotent, so it runs at boot
// and again whenever an agent is created mid-session (RefreshShardRoster).
func refreshShardRoster(ctx context.Context, kernel types.Kernel, workspace string, compiler *prompt.JITPromptCompiler) error {
	roster, measureErr := measureShardRoster(ctx, workspace, compiler)
	if kernel == nil {
		return measureErr
	}
	if err := publishRoster(kernel, roster); err != nil {
		return errors.Join(measureErr, err)
	}
	if len(roster.unrecorded) == 0 || !roster.complete {
		return measureErr
	}
	rows, err := kernel.Query("shard_agent_domain_to_record")
	if err != nil {
		return errors.Join(measureErr, fmt.Errorf("query shard_agent_domain_to_record: %w", err))
	}
	derived := make(map[string][]string, len(roster.unrecorded))
	for _, key := range roster.unrecorded {
		derived[key] = []string{}
	}
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		key := experts.AgentKey(types.ExtractString(row.Args[0]))
		if _, ok := derived[key]; !ok {
			continue
		}
		tech := strings.TrimPrefix(types.ExtractString(row.Args[1]), "/")
		if tech != "" && !hasString(derived[key], tech) {
			derived[key] = append(derived[key], tech)
		}
	}
	written, err := recordAgentDomains(workspace, derived)
	if err != nil {
		return errors.Join(measureErr, fmt.Errorf("record derived domains: %w", err))
	}
	if written {
		logging.Get(logging.CategoryContext).Info("Recorded derived domains for %d shard agent(s) in agents.json: %v", len(derived), derived)
	}
	return measureErr
}

// RefreshShardRoster re-asserts the shard-agent roster after an agent is
// created or its definition changes mid-session, so the next task can need
// it. Callers pass the session kernel and JIT compiler (whose agent DBs they
// have registered).
func RefreshShardRoster(ctx context.Context, kernel types.Kernel, workspace string, compiler *prompt.JITPromptCompiler) error {
	return refreshShardRoster(ctx, kernel, workspace, compiler)
}

// assertShardExpertFacts is the boot step: it runs after every discovered
// agent's DB is registered with the JIT compiler, so the KB side can be read.
func assertShardExpertFacts(bctx *bootContext) {
	if err := refreshShardRoster(bctx.ctx, bctx.kernel, bctx.workspace, bctx.jitCompiler); err != nil {
		logging.Get(logging.CategoryContext).Warn("shard expert roster incomplete: %v", err)
	}
}

func hasString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// rosterPublications are the roster facts last published into each kernel:
// what a refresh owns, and so what it may retire. Refreshes of one kernel are
// serialized by the mutex, so an older measurement never lands after a newer
// one has been published.
var rosterPublications = struct {
	mu        sync.Mutex
	published map[types.Kernel][]types.Fact
}{published: map[types.Kernel][]types.Fact{}}

// publishRoster makes the kernel's roster facts the new measurement's: it
// asserts what was measured and retracts what the last publication asserted
// and this one did not, in one transaction. Until 2026-10-30 a refresh only
// asserted, so a domain an agent's knowledge base no longer tagged stayed
// active in the running kernel and kept routing tasks to the agent; a fresh
// boot over the same files would not have it. Facts other producers asserted
// are untouched (only this producer's own publication is retired), and the
// policy still decides everything from the observations
// (policy/shard_experts.mg).
func publishRoster(kernel types.Kernel, roster shardRoster) error {
	rosterPublications.mu.Lock()
	defer rosterPublications.mu.Unlock()
	next, obsolete := reconcileRoster(rosterPublications.published[kernel], roster)
	if len(next) == 0 && len(obsolete) == 0 {
		return nil
	}
	// One transaction: selection and the knowledge refresh see the old
	// roster or the new one, never a half-retired mix.
	tx, ok := types.TransactorOf(kernel)
	if !ok {
		return fmt.Errorf("publish shard roster: kernel %T cannot commit the retirement and the new observations atomically", kernel)
	}
	t := tx.Transaction()
	for _, f := range obsolete {
		t.RetractExactFact(f)
	}
	for _, f := range next {
		t.Assert(f)
	}
	if err := t.Commit(); err != nil {
		return fmt.Errorf("publish shard roster: %w", err)
	}
	if len(obsolete) > 0 {
		logging.Get(logging.CategoryContext).Info("shard roster refresh retired %d observation(s) no source supports any more", len(obsolete))
	}
	rosterPublications.published[kernel] = next
	return nil
}

// reconcileRoster is the publication that replaces prev: the measured facts,
// plus prev's support from a source this measurement could not read (an
// agent's unreadable knowledge base, an unreadable catalog) -- an unread
// source measured nothing, which is not the same as measuring no support --
// and the facts of prev the new publication drops.
func reconcileRoster(prev []types.Fact, roster shardRoster) (next, obsolete []types.Fact) {
	seen := make(map[string]bool, len(roster.facts))
	key := func(f types.Fact) string { return f.String() }
	add := func(f types.Fact) {
		if k := key(f); !seen[k] {
			seen[k] = true
			next = append(next, f)
		}
	}
	for _, f := range roster.facts {
		add(f)
	}
	for _, f := range prev {
		if f.Predicate != "shard_agent_domain" || len(f.Args) <= 2 {
			continue
		}
		agent := types.MangleAtom(types.ArgName(f, 0))
		switch types.ExtractString(f.Args[2]) {
		case "/knowledge_base":
			if roster.kbUnread[agent] {
				add(f)
			}
		case "/catalog":
			if roster.catalogUnread {
				add(f)
			}
		}
	}
	for _, f := range prev {
		if !seen[key(f)] {
			obsolete = append(obsolete, f)
		}
	}
	return next, obsolete
}
