package system

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

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
//	technology_import_root(Root, Tech)       init's dependency vocabulary
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
}

// measureShardRoster builds the roster facts for a workspace. compiler, when
// non-nil, is asked for each agent's KB technologies (its registered DB). A
// source that cannot be read is reported beside the facts, never by dropping
// the agent.
func measureShardRoster(ctx context.Context, workspace string, compiler *prompt.JITPromptCompiler) (shardRoster, error) {
	roster := shardRoster{complete: true}
	var failures []string

	roots := nerdinit.TechnologyImportRoots()
	paths := make([]string, 0, len(roots))
	for path := range roots {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if tech, ok := technologyAtom(roots[path]); ok {
			roster.facts = append(roster.facts, types.Fact{Predicate: "technology_import_root", Args: []any{path, tech}})
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
	if kernel == nil || len(roster.facts) == 0 {
		return measureErr
	}
	if err := kernel.AssertBatch(roster.facts); err != nil {
		return errors.Join(measureErr, fmt.Errorf("assert shard roster: %w", err))
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
