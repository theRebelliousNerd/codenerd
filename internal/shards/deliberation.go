package shards

import (
	"context"
	"fmt"
	"sync"

	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/deliberation"
	"codenerd/internal/types"
)

// DeliberationShard is a read-only, ephemeral role. The conductor supplies the
// round frame; identity and output contracts are compiled from JIT atoms.
type DeliberationShard struct {
	*coreshards.BaseShardAgent
	mu      sync.RWMutex
	role    string
	client  types.LLMClient
	prompts types.RolePromptCompiler
	kernel  types.Kernel
	read    deliberation.ReadTools
}

func newDeliberationShard(role string) *DeliberationShard {
	cfg := types.ShardConfig{Name: role, Type: types.ShardTypeEphemeral,
		Permissions: []types.ShardPermission{types.PermissionReadFile, types.PermissionCodeGraph},
		Model:       types.ModelConfig{Capability: types.CapabilityHighReasoning}}
	return &DeliberationShard{BaseShardAgent: coreshards.NewBaseShardAgent(role, cfg), role: role}
}

func (s *DeliberationShard) SetLLMClient(c types.LLMClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = c
	s.BaseShardAgent.SetLLMClient(c)
}

func (s *DeliberationShard) configure(k types.Kernel, prompts types.RolePromptCompiler, vs *core.VirtualStore, bytes int) {
	s.kernel, s.prompts = k, prompts
	s.SetParentKernel(k)
	s.read = deliberation.ReadTools{Kernel: k, ContextBytes: bytes}
	if vs != nil {
		s.read.Registry = vs.GetModularTools()
	}
}

func (s *DeliberationShard) Execute(ctx context.Context, task string) (result string, err error) {
	s.SetState(types.ShardStateRunning)
	defer func() {
		if err != nil {
			s.SetState(types.ShardStateFailed)
		} else {
			s.SetState(types.ShardStateCompleted)
		}
	}()
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if s.kernel == nil {
		return "", fmt.Errorf("deliberation kernel unavailable")
	}
	return deliberation.Agent(ctx, s.kernel, client, s.prompts, s.role, task, s.read)
}
