// campaign_runner.go implements the Campaign Runner system shard.
//
// The Campaign Runner is a Type S supervisor that ensures long-horizon
// campaigns keep progressing hands-off. It watches persisted campaigns
// on disk and automatically resumes any active/paused campaign until
// completion, including across process restarts.
package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codenerd/internal/campaign"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// CampaignRunnerConfig controls polling and supervision cadence.
type CampaignRunnerConfig struct {
	TickInterval time.Duration // How often to scan for runnable campaigns
}

// DefaultCampaignRunnerConfig returns sensible defaults.
func DefaultCampaignRunnerConfig() CampaignRunnerConfig {
	return CampaignRunnerConfig{
		TickInterval: 5 * time.Second,
	}
}

// CampaignRunnerShard supervises campaign orchestrators for durability.
type CampaignRunnerShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	config CampaignRunnerConfig

	workspace string
	shardMgr  *coreshards.ShardManager

	activeOrch        *campaign.Orchestrator
	activeCampaignID  string
	activeOrchDone    chan error
	lastStartAttempt  time.Time
	restartBackoffSec int

	running bool
}

// NewCampaignRunnerShard creates a new Campaign Runner shard.
func NewCampaignRunnerShard() *CampaignRunnerShard {
	return NewCampaignRunnerShardWithConfig(DefaultCampaignRunnerConfig())
}

// NewCampaignRunnerShardWithConfig creates a Campaign Runner with custom config.
func NewCampaignRunnerShardWithConfig(cfg CampaignRunnerConfig) *CampaignRunnerShard {
	logging.SystemShards("[CampaignRunner] Initializing campaign runner shard")
	base := NewBaseSystemShard("campaign_runner", StartupAuto)

	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
		types.PermissionWriteFile,
		types.PermissionExecCmd,
	}

	return &CampaignRunnerShard{
		BaseSystemShard:   base,
		config:            cfg,
		restartBackoffSec: 5,
	}
}

// SetWorkspaceRoot sets the workspace for campaign discovery.
func (s *CampaignRunnerShard) SetWorkspaceRoot(workspace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspace = workspace
}

// SetShardManager injects the shared ShardManager.
func (s *CampaignRunnerShard) SetShardManager(sm *coreshards.ShardManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shardMgr = sm
}

// Execute runs the Campaign Runner supervision loop.
func (s *CampaignRunnerShard) Execute(ctx context.Context, task string) (string, error) {
	logging.SystemShards("[CampaignRunner] Starting supervision loop")
	s.SetState(types.ShardStateRunning)
	s.mu.Lock()
	s.running = true
	s.StartTime = time.Now()
	s.mu.Unlock()

	defer func() {
		s.SetState(types.ShardStateCompleted)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		logging.SystemShards("[CampaignRunner] Supervision loop terminated")
	}()

	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "campaign runner stopped: context cancelled", ctx.Err()
		case <-s.StopCh:
			return "campaign runner stopped", nil
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *CampaignRunnerShard) tick(ctx context.Context) {
	// Emit heartbeat fact for policy observability (best-effort).
	if s.Kernel != nil {
		_ = s.Kernel.Assert(types.Fact{
			Predicate: "campaign_runner_heartbeat",
			Args:      []interface{}{time.Now().Unix()},
		})
	}

	// Check for orchestrator completion.
	s.mu.Lock()
	if s.activeOrchDone != nil {
		select {
		case err := <-s.activeOrchDone:
			campaignID := s.activeCampaignID
			s.activeOrch = nil
			s.activeOrchDone = nil
			s.activeCampaignID = ""
			s.lastStartAttempt = time.Time{}
			s.restartBackoffSec = 5

			if err != nil && err != context.Canceled {
				logging.Get(logging.CategorySystemShards).Warn("[CampaignRunner] Campaign %s exited with error: %v", campaignID, err)
				_ = s.Kernel.Assert(types.Fact{
					Predicate: "campaign_runner_failure",
					Args:      []interface{}{campaignID, err.Error(), time.Now().Unix()},
				})
			} else {
				logging.SystemShards("[CampaignRunner] Campaign %s completed or paused", campaignID)
				_ = s.Kernel.Assert(types.Fact{
					Predicate: "campaign_runner_success",
					Args:      []interface{}{campaignID, time.Now().Unix()},
				})
			}
		default:
		}
	}

	// If already supervising a campaign, nothing else to do.
	if s.activeOrch != nil {
		s.mu.Unlock()
		return
	}

	workspace := s.workspace
	shardMgr := s.shardMgr
	s.mu.Unlock()

	if workspace == "" {
		return
	}
	if shardMgr == nil || s.Kernel == nil || s.LLMClient == nil {
		logging.SystemShardsDebug("[CampaignRunner] Missing dependencies (shardMgr/kernel/llm), skipping tick")
		return
	}

	camp, err := s.findLatestRunnableCampaign(workspace)
	if err != nil || camp == nil {
		return
	}

	// Simple restart backoff to avoid tight loops on repeated failures.
	s.mu.Lock()
	if !s.lastStartAttempt.IsZero() && time.Since(s.lastStartAttempt) < time.Duration(s.restartBackoffSec)*time.Second {
		s.mu.Unlock()
		return
	}
	s.lastStartAttempt = time.Now()
	s.mu.Unlock()

	s.startCampaign(ctx, camp.ID, workspace, shardMgr)
}

func (s *CampaignRunnerShard) startCampaign(ctx context.Context, campaignID, workspace string, shardMgr *coreshards.ShardManager) {
	logging.SystemShards("[CampaignRunner] Resuming campaign: %s", campaignID)

	executor := tactile.NewSafeExecutor()
	orch := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:        workspace,
		Kernel:           s.Kernel,
		LLMClient:        s.LLMClient,
		ShardManager:     shardMgr,
		Executor:         executor,
		VirtualStore:     s.VirtualStore,
		AutoReplan:       true,
		CheckpointOnFail: true,
		DisableTimeouts:  true,
	})

	if err := orch.LoadCampaign(campaignID); err != nil {
		logging.Get(logging.CategorySystemShards).Error("[CampaignRunner] Failed to load campaign %s: %v", campaignID, err)
		// Exponential backoff on repeated load failures.
		s.mu.Lock()
		if s.restartBackoffSec < 300 {
			s.restartBackoffSec *= 2
		}
		s.mu.Unlock()
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	s.mu.Lock()
	s.activeOrch = orch
	s.activeOrchDone = done
	s.activeCampaignID = campaignID
	s.mu.Unlock()

	_ = s.Kernel.Assert(types.Fact{
		Predicate: "campaign_runner_active",
		Args:      []interface{}{campaignID, time.Now().Unix()},
	})
}

// findLatestRunnableCampaign returns the most recently updated active/paused campaign.
func (s *CampaignRunnerShard) findLatestRunnableCampaign(workspace string) (*campaign.Campaign, error) {
	campaignsDir := filepath.Join(workspace, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		return nil, nil
	}

	type candidate struct {
		campaign *campaign.Campaign
		updated  time.Time
	}
	var candidates []candidate

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(campaignsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var c campaign.Campaign
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}

		if c.Status != campaign.StatusActive && c.Status != campaign.StatusPaused {
			continue
		}

		updated := c.UpdatedAt
		if updated.IsZero() {
			if info, statErr := os.Stat(path); statErr == nil {
				updated = info.ModTime()
			}
		}

		candidates = append(candidates, candidate{campaign: &c, updated: updated})
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].updated.After(candidates[j].updated)
	})

	// Prefer active over paused if timestamps are close.
	best := candidates[0]
	for _, cand := range candidates {
		if cand.campaign.Status == campaign.StatusActive {
			best = cand
			break
		}
	}

	logging.SystemShardsDebug("[CampaignRunner] Found runnable campaign: %s (%s)", best.campaign.ID, best.campaign.Status)
	return best.campaign, nil
}
