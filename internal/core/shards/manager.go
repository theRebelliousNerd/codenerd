package shards

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"sync"
)

// =============================================================================
// SHARD MANAGER
// =============================================================================

// VirtualStoreConsumer interface for agents that need file system access.
type VirtualStoreConsumer interface {
	SetVirtualStore(vs any)
}

// ReviewerFeedbackProvider defines the interface for reviewer validation.
type ReviewerFeedbackProvider interface {
	NeedsValidation(reviewID string) bool
	GetSuspectReasons(reviewID string) []string
	AcceptFinding(reviewID, file string, line int)
	RejectFinding(reviewID, file string, line int, reason string)
	GetAccuracyReport(reviewID string) string
}

// ShardManager orchestrates all shard agents.
type ShardManager struct {
	shards    map[string]types.ShardAgent
	results   map[string]types.ShardResult
	profiles  map[string]types.ShardConfig
	factories map[string]types.ShardFactory
	disabled  map[string]struct{}
	mu        sync.RWMutex

	// Core dependencies to inject into shards
	kernel       types.Kernel
	llmClient    types.LLMClient
	virtualStore any
	// tracingClient TracingClient // Optional: set when llmClient implements TracingClient
	transparencyManager any // types.TransparencyManager to be added later
	learningStore       types.LearningStore
	reviewerFeedback    ReviewerFeedbackProvider

	// Resource limits enforcement
	limitsEnforcer types.LimitsEnforcer

	// SpawnQueue for backpressure management (optional)
	spawnQueue *SpawnQueue

	// Session context for tracing
	sessionID string

	// Workspace paths and callbacks for prompt loading
	nerdDir        string                 // Path to .nerd directory
	promptLoader   types.PromptLoaderFunc // Callback to load agent prompts (avoids import cycle)
	jitRegistrar   types.JITDBRegistrar   // Callback to register agent DBs with JIT compiler
	jitUnregistrar types.JITDBUnregistrar // Callback to unregister agent DBs when shard deactivates
	activeJITDBs   map[string]string      // Tracks which shards have registered JIT DBs (shardID -> typeName)
}

func NewShardManager() *ShardManager {
	logging.Shards("Creating new ShardManager")
	sm := &ShardManager{
		shards:       make(map[string]types.ShardAgent),
		results:      make(map[string]types.ShardResult),
		profiles:     make(map[string]types.ShardConfig),
		factories:    make(map[string]types.ShardFactory),
		activeJITDBs: make(map[string]string),
	}
	logging.ShardsDebug("ShardManager initialized with empty maps")
	return sm
}

func (sm *ShardManager) SetReviewerFeedbackProvider(provider ReviewerFeedbackProvider) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.reviewerFeedback = provider
}

func (sm *ShardManager) SetParentKernel(k types.Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kernel = k
	logging.ShardsDebug("Parent kernel attached to ShardManager")
}

func (sm *ShardManager) SetVirtualStore(vs any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.virtualStore = vs
	logging.ShardsDebug("VirtualStore attached to ShardManager")
}

func (sm *ShardManager) SetTransparencyManager(tm any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transparencyManager = tm
	logging.ShardsDebug("TransparencyManager attached to ShardManager")
}

func (sm *ShardManager) SetLLMClient(client types.LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
	// Tracing support would check interface here
}

func (sm *ShardManager) SetSessionID(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionID = sessionID
	logging.ShardsDebug("Session ID set: %s", sessionID)
}

func (sm *ShardManager) SetNerdDir(nerdDir string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.nerdDir = nerdDir
	logging.ShardsDebug("Nerd directory set: %s", nerdDir)
}

func (sm *ShardManager) SetPromptLoader(loader types.PromptLoaderFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.promptLoader = loader
	logging.ShardsDebug("Prompt loader callback set")
}

func (sm *ShardManager) SetJITRegistrar(registrar types.JITDBRegistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitRegistrar = registrar
	logging.ShardsDebug("JIT registrar callback set")
}

func (sm *ShardManager) SetJITUnregistrar(unregistrar types.JITDBUnregistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitUnregistrar = unregistrar
	logging.ShardsDebug("JIT unregistrar callback set")
}

func (sm *ShardManager) categorizeShardType(typeName string, shardType types.ShardType) string {
	// System shards (built-in, always-on)
	systemShards := map[string]bool{
		"perception_firewall":  true,
		"constitution_gate":    true,
		"executive_policy":     true,
		"cost_guard":           true,
		"tactile_router":       true,
		"session_planner":      true,
		"world_model_ingestor": true,
	}
	if systemShards[typeName] || shardType == types.ShardTypeSystem {
		return "system"
	}

	// Ephemeral shards (built-in factories)
	ephemeralShards := map[string]bool{
		"coder":      true,
		"tester":     true,
		"reviewer":   true,
		"researcher": true,
	}
	if ephemeralShards[typeName] || shardType == types.ShardTypeEphemeral {
		return "ephemeral"
	}

	// Everything else is a specialist (LLM-created or user-created)
	return "specialist"
}

func (sm *ShardManager) SetLearningStore(store types.LearningStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.learningStore = store
	logging.ShardsDebug("LearningStore attached to ShardManager")
}

func (sm *ShardManager) RegisterShard(typeName string, factory types.ShardFactory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.factories[typeName] = factory
	logging.Shards("Registered shard factory: %s", typeName)
}

func (sm *ShardManager) DefineProfile(name string, config types.ShardConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.profiles[name] = config
	logging.Shards("Defined shard profile: %s (type: %s)", name, config.Type)
}

func (sm *ShardManager) GetProfile(name string) (types.ShardConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	cfg, ok := sm.profiles[name]
	return cfg, ok
}

// ShardInfo is an alias to types.ShardInfo for shard discovery.
type ShardInfo = types.ShardInfo

func (sm *ShardManager) ListAvailableShards() []ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var shards []ShardInfo

	for name := range sm.factories {
		shardType := types.ShardTypeEphemeral
		if name == "perception_firewall" || name == "executive_policy" ||
			name == "constitution_gate" || name == "tactile_router" ||
			name == "session_planner" || name == "world_model_ingestor" {
			shardType = types.ShardTypeSystem
		}
		shards = append(shards, ShardInfo{
			Name: name,
			Type: shardType,
		})
	}

	for name, profile := range sm.profiles {
		alreadyAdded := false
		for _, s := range shards {
			if s.Name == name {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		shards = append(shards, ShardInfo{
			Name:         name,
			Type:         profile.Type,
			HasKnowledge: profile.KnowledgePath != "",
		})
	}

	return shards
}

func (sm *ShardManager) GetRunningShardByConfigName(name string) (types.ShardAgent, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, shard := range sm.shards {
		if shard == nil {
			continue
		}
		cfg := shard.GetConfig()
		if cfg.Name == name {
			return shard, true
		}
	}
	return nil, false
}

func (sm *ShardManager) ToFacts() []types.Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	facts := make([]types.Fact, 0)

	for name, cfg := range sm.profiles {
		facts = append(facts, types.Fact{
			Predicate: "shard_profile",
			Args:      []interface{}{name, string(cfg.Type)},
		})
	}

	return facts
}

func (sm *ShardManager) CheckReviewNeedsValidation(reviewID string) bool {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	sm.mu.RUnlock()

	if provider == nil {
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("reviewer_needs_validation")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 && fact.Args[0] == reviewID {
						return true
					}
				}
			}
		}
		return false
	}
	return provider.NeedsValidation(reviewID)
}

// GetReviewSuspectReasons returns reasons why a review is flagged as suspect.
func (sm *ShardManager) GetReviewSuspectReasons(reviewID string) []string {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	sm.mu.RUnlock()

	if provider == nil {
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("review_suspect")
			if err == nil {
				var reasons []string
				for _, fact := range facts {
					if len(fact.Args) >= 2 && fact.Args[0] == reviewID {
						if reason, ok := fact.Args[1].(string); ok {
							reasons = append(reasons, reason)
						}
					}
				}
				return reasons
			}
		}
		return nil
	}
	return provider.GetSuspectReasons(reviewID)
}

// AcceptReviewFinding marks a finding as accepted by the user.
func (sm *ShardManager) AcceptReviewFinding(reviewID, file string, line int) {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	sm.mu.RUnlock()

	if provider != nil {
		provider.AcceptFinding(reviewID, file, line)
	}
}

// RejectReviewFinding marks a finding as rejected by the user.
func (sm *ShardManager) RejectReviewFinding(reviewID, file string, line int, reason string) {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	sm.mu.RUnlock()

	if provider != nil {
		provider.RejectFinding(reviewID, file, line, reason)
	}
}

// GetReviewAccuracyReport returns accuracy statistics for a review session.
func (sm *ShardManager) GetReviewAccuracyReport(reviewID string) string {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	sm.mu.RUnlock()

	if provider == nil {
		return "Review feedback provider not available"
	}
	return provider.GetAccuracyReport(reviewID)
}
