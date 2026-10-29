package system

import (
	"context"
	"errors"
	"fmt"
	"strings"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/types"
)

// agentActivator makes an agent defined mid-session usable by the running
// session, the way boot makes the agents on disk usable: its prompts.yaml is
// synced into its KB, the KB is registered with the JIT compiler, a shard
// profile is defined, the registry is synced, and the shard-agent roster is
// refreshed (refreshShardRoster) so the next task can need it. Without this
// an agent written by autopoiesis or the chat wizard existed on disk and was
// invisible to expert_needed until the next boot.
type agentActivator struct {
	workspace string
	kernel    types.Kernel
	compiler  *prompt.JITPromptCompiler
	shards    *coreshards.ShardManager
	loader    *prompt.AtomLoader
}

// Activate brings the named agent into the running session.
func (a agentActivator) Activate(ctx context.Context, name string) error {
	if strings.TrimSpace(a.workspace) == "" {
		return fmt.Errorf("activate agent %q: no workspace", name)
	}
	synchronizer := prsync.NewAgentSynchronizer(a.workspace, a.loader)
	var errs []error
	if err := synchronizer.SyncAll(ctx); err != nil {
		errs = append(errs, fmt.Errorf("sync agent prompts: %w", err))
	}
	discovered := synchronizer.GetDiscoveredAgents()
	found := false
	onDisk := make([]AgentOnDisk, 0, len(discovered))
	for _, agent := range discovered {
		onDisk = append(onDisk, AgentOnDisk{ID: agent.ID, DBPath: agent.DBPath})
		if !strings.EqualFold(agent.ID, strings.TrimSpace(name)) {
			continue
		}
		found = true
		if a.compiler != nil {
			if err := prompt.RegisterAgentDBWithJIT(a.compiler, agent.ID, agent.DBPath); err != nil {
				errs = append(errs, fmt.Errorf("register %s with the JIT compiler: %w", agent.ID, err))
			}
		}
		if a.shards != nil {
			cfg := coreshards.DefaultSpecialistConfig(agent.ID, agent.DBPath)
			cfg.Type = types.ShardTypeUser
			a.shards.DefineProfile(agent.ID, cfg)
		}
	}
	if !found {
		errs = append(errs, fmt.Errorf("agent %q has no prompts.yaml under .nerd/agents", name))
	}
	if _, err := SyncAgentRegistryFromDiscovered(a.workspace, onDisk); err != nil {
		errs = append(errs, fmt.Errorf("sync agents.json: %w", err))
	}
	if err := refreshShardRoster(ctx, a.kernel, a.workspace, a.compiler); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("activate agent %q: %w", name, err)
	}
	logging.Get(logging.CategoryContext).Info("Activated shard agent %q in the running session", name)
	return nil
}

// activator is the boot context's agent activator. It reads the boot
// context's fields when called, so a writer installed before the JIT
// compiler exists (the autopoiesis writer) activates against the finished
// session.
func (bctx *bootContext) activator() agentActivator {
	return agentActivator{
		workspace: bctx.workspace,
		kernel:    bctx.kernel,
		compiler:  bctx.jitCompiler,
		shards:    bctx.shardManager,
		loader:    bctx.atomLoader,
	}
}

// ActivateAgent brings an agent defined during this session (the chat agent
// wizard, autopoiesis) into it: KB synced and registered, shard profile
// defined, roster refreshed so tasks on its files need it.
func (c *Cortex) ActivateAgent(ctx context.Context, name string) error {
	if c == nil {
		return fmt.Errorf("activate agent %q: no cortex", name)
	}
	return c.agents.Activate(ctx, name)
}
