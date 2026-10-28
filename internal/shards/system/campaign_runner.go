// campaign_runner.go implements the Campaign Runner system shard.
//
// The Campaign Runner is a Type S supervisor that ensures long-horizon
// campaigns keep progressing hands-off. It watches persisted campaigns
// on disk and resumes active/paused campaigns when explicitly started.
// NOTE: Uses StartupOnDemand to prevent automatic campaign execution on boot.
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/consultation"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
	"codenerd/internal/world"
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

	// taskExecutor drives the verification checkpoints. Without it, shard
	// validation and the Nemesis gauntlet cannot run at all, so campaigns this
	// shard supervises would be unverifiable. It is set after boot rather than
	// at construction because the executor does not exist yet when the shard
	// factory is registered; the factory closure runs on demand, by which time
	// it does.
	taskExecutor session.TaskExecutor

	// planningLLM serves the supervised campaigns' planning (replanning,
	// rolling-wave refinement, assault remediation planning); the shard's own
	// LLMClient serves their execution-side calls. prompts compiles their
	// planning prompts from the JIT corpus for that client's model; nil leaves
	// the static fallback. Both are set by the boot closure (SetPlanning).
	planningLLM types.LLMClient
	prompts     campaign.PromptProvider

	activeOrch        *campaign.Orchestrator
	activeCampaignID  string
	activeOrchDone    chan error
	lastStartAttempt  time.Time
	restartBackoffSec int

	running bool
}

// SetTaskExecutor supplies the executor the orchestrator's verification
// checkpoints run on. Call it from the shard factory closure, which runs on
// demand and therefore after the executor has been built.
func (s *CampaignRunnerShard) SetTaskExecutor(exec session.TaskExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskExecutor = exec
}

// TaskExecutor returns the configured executor, or nil if none was wired.
func (s *CampaignRunnerShard) TaskExecutor() session.TaskExecutor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.taskExecutor
}

// SetPlanning supplies the client supervised campaigns plan on (the planner
// slot when config.json configures one, else main) and the assembler their
// planning prompts compile from. Without the client no campaign is resumed:
// one that fails could not replan. Call it from the shard factory closure,
// like SetTaskExecutor.
func (s *CampaignRunnerShard) SetPlanning(client types.LLMClient, assembler *articulation.PromptAssembler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.planningLLM = client
	s.prompts = campaign.NewJITPromptProvider(assembler)
}

// NewCampaignRunnerShard creates a new Campaign Runner shard.
func NewCampaignRunnerShard() *CampaignRunnerShard {
	return NewCampaignRunnerShardWithConfig(DefaultCampaignRunnerConfig())
}

// NewCampaignRunnerShardWithConfig creates a Campaign Runner with custom config.
func NewCampaignRunnerShardWithConfig(cfg CampaignRunnerConfig) *CampaignRunnerShard {
	logging.SystemShards("[CampaignRunner] Initializing campaign runner shard")
	base := NewBaseSystemShard("campaign_runner", StartupOnDemand)

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
			Args:      []any{time.Now().Unix()},
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
			if err != nil && err != context.Canceled {
				logging.Get(logging.CategorySystemShards).Warn("[CampaignRunner] Campaign %s exited with error: %v", campaignID, err)
				if s.Kernel != nil {
					_ = s.Kernel.Assert(types.Fact{
						Predicate: "campaign_runner_failure",
						Args:      []any{campaignID, err.Error(), time.Now().Unix()},
					})
				}
				// Exponential backoff on repeated Run failures, capped at 300s.
				if s.restartBackoffSec < 300 {
					s.restartBackoffSec *= 2
					if s.restartBackoffSec > 300 {
						s.restartBackoffSec = 300
					}
				}
				s.lastStartAttempt = time.Now()
			} else {
				logging.SystemShards("[CampaignRunner] Campaign %s completed or paused", campaignID)
				if s.Kernel != nil {
					_ = s.Kernel.Assert(types.Fact{
						Predicate: "campaign_runner_success",
						Args:      []any{campaignID, time.Now().Unix()},
					})
				}
				s.lastStartAttempt = time.Time{}
				s.restartBackoffSec = 5
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
	planning := s.planningLLM
	s.mu.Unlock()

	if workspace == "" {
		return
	}
	if shardMgr == nil || s.Kernel == nil || s.LLMClient == nil || planning == nil {
		logging.SystemShardsDebug("[CampaignRunner] Missing dependencies (shardMgr/kernel/llm/planning llm), skipping tick")
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

	orchCfg, err := s.orchestratorConfig(workspace, shardMgr)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[CampaignRunner] Campaign policy for %s unreadable: %v", campaignID, err)
		return
	}
	orch, err := campaign.NewOrchestrator(orchCfg)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[CampaignRunner] Invalid orchestrator config for %s: %v", campaignID, err)
		return
	}
	s.mu.RLock()
	prompts := s.prompts
	s.mu.RUnlock()
	if prompts != nil {
		orch.SetPromptProvider(prompts)
	}

	// The same admission every resume door uses: the generation must be one
	// the journal vouches for (or a provenance the kernel admits), and another
	// owner — a CLI resume, the chat — keeps it. This automated door never
	// adopts a snapshot the journal does not vouch for; only an operator can.
	if report, err := orch.ResumeCampaign(campaignID, campaign.AdmissionPolicy{}); err != nil {
		logging.Get(logging.CategorySystemShards).Error("[CampaignRunner] Campaign %s not admitted: %v", campaignID, err)
		if report != nil {
			logging.SystemShardsDebug("[CampaignRunner] %s", campaign.RenderRecoveryReport(report))
		}
		// The orchestrator owns its northstar observer: a retry every few
		// seconds that dropped it unclosed leaked one guardian per attempt.
		_ = orch.Close()
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
		runErr := orch.Run(ctx)
		_ = orch.Close()
		done <- runErr
	}()

	s.mu.Lock()
	s.activeOrch = orch
	s.activeOrchDone = done
	s.activeCampaignID = campaignID
	s.mu.Unlock()

	_ = s.Kernel.Assert(types.Fact{
		Predicate: "campaign_runner_active",
		Args:      []any{campaignID, time.Now().Unix()},
	})
}

// orchestratorConfig is the configuration a supervised campaign resumes
// under: its execution-side calls on the shard's LLM client, its planning on
// the planning client, its tasks on the task executor, the risk preflight's
// sensors over the workspace, and the policy of the workspace config's
// campaign section, as on every other door; a config that does not load runs
// nothing.
func (s *CampaignRunnerShard) orchestratorConfig(workspace string, shardMgr *coreshards.ShardManager) (campaign.OrchestratorConfig, error) {
	appCfg, err := config.LoadUserConfig(filepath.Join(workspace, ".nerd", "config.json"))
	if err != nil {
		return campaign.OrchestratorConfig{}, err
	}

	s.mu.RLock()
	planning := s.planningLLM
	s.mu.RUnlock()

	// A registered direct bypass (tactile.DirectBypassRegistry): the resumed
	// campaign runs its commands here, so the shard kernel is its audit sink.
	var executor tactile.Executor = tactile.NewDirectExecutor()
	if s.Kernel != nil {
		kernel := s.Kernel
		executor = tactile.NewFactAuditedExecutor(executor, func(f tactile.Fact) error {
			return kernel.Assert(types.Fact{Predicate: f.Predicate, Args: append([]any(nil), f.Args...)})
		})
	}
	worldScanner := world.NewScanner()
	// The same consultation engine every door uses: a failed advisor is a
	// typed missing review, not a silently shorter panel.
	consultationMgr := consultation.New(&campaignRunnerShardManagerConsultationSpawner{shardMgr: shardMgr})
	holographic := world.NewHolographicProvider(s.Kernel, workspace)
	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		workspace,
		s.Kernel,
		worldScanner,
		holographic,
		nil,
		nil,
		nil,
		nil,
		consultationMgr,
	)
	return campaign.OrchestratorConfig{
		Workspace:            workspace,
		Kernel:               s.Kernel,
		LLMClient:            s.LLMClient,
		PlanningLLMClient:    planning,
		ShardManager:         shardMgr,
		Executor:             executor,
		VirtualStore:         s.VirtualStore,
		Campaign:             appCfg.GetCampaignConfig(),
		IntelligenceGatherer: intelligenceGatherer,
		AdvisoryBoard:        campaign.NewShardAdvisoryBoard(consultationMgr),
		EdgeCaseDetector:     campaign.NewEdgeCaseDetector(s.Kernel, worldScanner),
		NorthstarObserver:    northstar.BuildCampaignObserver(workspace, s.LLMClient, s.Kernel),
		TaskExecutor:         s.TaskExecutor(),
	}, nil
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
		if candidates[i].updated.Equal(candidates[j].updated) {
			return candidates[i].campaign.ID < candidates[j].campaign.ID
		}
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

// campaignRunnerShardManagerConsultationSpawner runs a supervised campaign's
// consultations on the shard manager.
type campaignRunnerShardManagerConsultationSpawner struct {
	shardMgr *coreshards.ShardManager
}

func (s *campaignRunnerShardManagerConsultationSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if s == nil || s.shardMgr == nil {
		return "", fmt.Errorf("shard manager not available")
	}
	return s.shardMgr.Spawn(ctx, specialistName, task)
}
