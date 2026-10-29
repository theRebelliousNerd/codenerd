package system

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/experts"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Shard experts: the roster side of expert_needed (policy/shard_experts.mg).
//
// Boot measures which shard agents exist and what each is expert in, and
// asserts it once; each task's side is asserted per selection by
// internal/experts. The kernel then decides which agent a task needs.
//
//	shard_agent(Agent)                       a ready agent in .nerd/agents.json
//	shard_agent_domain(Agent, Tech, Source)  /registry: its `domains` there;
//	                                         /knowledge_base: languages:/frameworks:
//	                                         tags on its KB atoms
//	technology_import_root(Root, Tech)       init's dependency vocabulary

// shardExpertFacts builds the roster facts for a workspace. compiler, when
// non-nil, is asked for each agent's KB technologies (its registered DB); an
// agent whose KB cannot be read still contributes its registry domains, and
// the failure is returned beside the facts rather than dropping the agent.
func shardExpertFacts(ctx context.Context, workspace string, compiler *prompt.JITPromptCompiler) ([]types.Fact, error) {
	var facts []types.Fact
	var failures []string

	roots := nerdinit.TechnologyImportRoots()
	paths := make([]string, 0, len(roots))
	for path := range roots {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		tech, ok := technologyAtom(roots[path])
		if !ok {
			continue
		}
		facts = append(facts, types.Fact{Predicate: "technology_import_root", Args: []any{path, tech}})
	}

	if strings.TrimSpace(workspace) == "" {
		return facts, nil
	}
	reg, _, err := loadAgentRegistry(filepath.Join(workspace, ".nerd", "agents.json"))
	if err != nil {
		return facts, fmt.Errorf("read agents.json: %w", err)
	}
	for _, agent := range reg.Agents {
		if !rosterAgent(agent) {
			continue
		}
		atom := experts.AgentAtom(agent.Name)
		facts = append(facts, types.Fact{Predicate: "shard_agent", Args: []any{atom}})
		for _, domain := range agent.Domains {
			if tech, ok := technologyAtom(domain); ok {
				facts = append(facts, types.Fact{Predicate: "shard_agent_domain", Args: []any{atom, tech, types.MangleAtom("/registry")}})
			}
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
			continue
		}
		for _, raw := range techs {
			if tech, ok := technologyAtom(raw); ok {
				facts = append(facts, types.Fact{Predicate: "shard_agent_domain", Args: []any{atom, tech, types.MangleAtom("/knowledge_base")}})
			}
		}
	}
	if len(failures) > 0 {
		return facts, fmt.Errorf("shard agent knowledge bases unreadable: %s", strings.Join(failures, "; "))
	}
	return facts, nil
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

// assertShardExpertFacts is the boot step: it runs after every discovered
// agent's DB is registered with the JIT compiler, so the KB side can be read.
func assertShardExpertFacts(bctx *bootContext) {
	facts, err := shardExpertFacts(bctx.ctx, bctx.workspace, bctx.jitCompiler)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("shard expert roster incomplete: %v", err)
	}
	if len(facts) == 0 || bctx.kernel == nil {
		return
	}
	if err := bctx.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategoryContext).Warn("shard expert roster not asserted: %v", err)
		return
	}
	logging.Get(logging.CategoryContext).Info("Asserted %d shard expert roster facts", len(facts))
}
