package shards

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"fmt"
	"strings"
	"sync"
	"time"
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

	// spawnCounter ensures shard IDs are unique even when time resolution is coarse.
	spawnCounter int64

	// Core dependencies to inject into shards
	kernel    types.Kernel
	llmClient types.LLMClient // default worker/main client for most shards
	// imageLLMClient is Gemini Nano Banana 2 for image_generator shards only.
	// Never the Ollama worker — image models are Gemini-only.
	imageLLMClient types.LLMClient
	virtualStore   any
	// tracingClient TracingClient // Optional: set when llmClient implements TracingClient
	// transparencyManager receives shard lifecycle phases. It was stored as
	// `any` with a "to be added later" comment, which meant the spawn path
	// could not call it: `/transparency` rendered Active Operations from a
	// ShardObserver that nothing fed, while Glass Box showed the same shards.
	transparencyManager types.TransparencyManager
	learningStore       types.LearningStore
	reviewerFeedback    ReviewerFeedbackProvider

	// PostSpawnHook is called on every newly created shard immediately after core
	// dependency injection (kernel, llmClient, virtualStore). It lets the chat layer
	// inject chat-specific dependencies (GlassBox, ToolEventBus, ToolStore, etc.)
	// without introducing import cycles from shards → transparency/store.
	postSpawnHook func(agent types.ShardAgent)

	// taskDelegator routes shard types that have no registered in-process
	// factory into the JIT clean loop (session.TaskExecutor). The JIT migration
	// deleted the coder/tester/reviewer/researcher factories, so without this
	// every domain persona and every user-defined agent fell through to a
	// BaseShardAgent that returned a placeholder string with a nil error.
	// See SpawnAsyncWithContext for the resolution order.
	taskDelegator TaskDelegator

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

	// Optional Glass Box event bus for TUI activity overlay. When set,
	// the manager emits shard lifecycle events (spawn / completion) so
	// the chat can show "⚡ Spawned reviewer (3.2s)" inline.
	glassBoxBus *transparency.GlassBoxEventBus
}

// SetGlassBoxBus attaches the optional Glass Box event bus. Safe to
// call before or after shards spawn; nil is also safe (events become
// no-ops).
func (sm *ShardManager) SetGlassBoxBus(bus *transparency.GlassBoxEventBus) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.glassBoxBus = bus
}

// truncateForEvent shortens a value for inclusion in a Glass Box
// event detail. Long shard outputs would otherwise dominate the
// scrollback line.
func truncateForEvent(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// emitShardEvent fires a CategoryShard Glass Box event immediately so
// spawn/completion lines hit chat scrollback without batch delay.
// Cheap to call when the bus is nil (returns immediately).
func (sm *ShardManager) emitShardEvent(summary, details, source string, dur time.Duration) {
	if sm.glassBoxBus == nil {
		return
	}
	sm.glassBoxBus.EmitImmediate(transparency.GlassBoxEvent{
		Timestamp: time.Now(),
		Category:  transparency.CategoryShard,
		Summary:   summary,
		Details:   details,
		Source:    source,
		Duration:  dur,
	})
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

// SetTransparencyManager attaches the operator-visibility manager. Shard
// spawn/complete now report phases through it (see SpawnAsyncWithContext).
// nil is safe; every call site is nil-guarded.
func (sm *ShardManager) SetTransparencyManager(tm types.TransparencyManager) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transparencyManager = tm
	logging.ShardsDebug("TransparencyManager attached to ShardManager")
}

// TransparencyManager returns the attached manager, or nil.
func (sm *ShardManager) TransparencyManager() types.TransparencyManager {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.transparencyManager
}

func (sm *ShardManager) SetLLMClient(client types.LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
	// Tracing support would check interface here
}

// SetImageLLMClient sets the dedicated image-generation client (Gemini Nano
// Banana 2 / gemini-3.1-flash-image). Image shards use this instead of the
// worker Ollama client.
func (sm *ShardManager) SetImageLLMClient(client types.LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.imageLLMClient = client
	logging.ShardsDebug("Image LLM client attached to ShardManager")
}

// clientForShardType picks LLM for a shard: image family → imageLLMClient
// (Gemini Nano Banana 2), everything else → default llmClient (worker Ollama).
// Image shard types never fall back to the worker/main client — that would
// silently send Nano Banana work to Ollama (FM15). When the image client is
// unset, returns nil so spawn leaves the agent without a mis-wired client.
func (sm *ShardManager) clientForShardType(typeName string) types.LLMClient {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.clientForShardTypeLocked(typeName)
}

// clientForShardTypeLocked is the lock-free body of clientForShardType.
// Caller must hold sm.mu (R or W). Used from SpawnAsyncWithContext which
// already holds the write lock — nested RLock would deadlock.
func (sm *ShardManager) clientForShardTypeLocked(typeName string) types.LLMClient {
	if config.IsImageShardType(typeName) {
		return sm.imageLLMClient
	}
	return sm.llmClient
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

// SetPostSpawnHook registers a callback that is invoked on every newly spawned
// shard right after core dependency injection. This lets the chat layer inject
// chat-specific dependencies (GlassBox, ToolEventBus, ToolStore, etc.) into
// on-demand shards without creating import cycles.
func (sm *ShardManager) SetPostSpawnHook(hook func(types.ShardAgent)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.postSpawnHook = hook
	logging.ShardsDebug("PostSpawnHook registered on ShardManager")
}

func (sm *ShardManager) RegisterShard(typeName string, factory types.ShardFactory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.factories[typeName] = factory
	logging.Shards("Registered shard factory: %s", typeName)
}

// HasShardFactory reports whether a factory is registered for typeName.
// It distinguishes factory registration from profile definition: profiles
// alone (DefineProfile/GetProfile) do not create shards.
func (sm *ShardManager) HasShardFactory(typeName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.factories[typeName]
	return ok
}

// SetTaskDelegator attaches the JIT clean-loop executor used for shard types
// with no registered factory. Without it, spawning such a type is a hard error
// rather than a silent no-op — see SpawnAsyncWithContext.
func (sm *ShardManager) SetTaskDelegator(d TaskDelegator) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.taskDelegator = d
	logging.ShardsDebug("TaskDelegator attached to ShardManager")
}

// HasTaskDelegator reports whether a JIT clean-loop fallback is wired.
func (sm *ShardManager) HasTaskDelegator() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.taskDelegator != nil
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

// legacySystemShardNames is the pre-profile hardcoded system-shard list, kept
// only as a fallback for a factory registered without a profile. It is
// deliberately not extended: new system shards get their type from the profile
// they register, which is what stopped this list from drifting again.
var legacySystemShardNames = map[string]bool{
	"perception_firewall":  true,
	"executive_policy":     true,
	"constitution_gate":    true,
	"tactile_router":       true,
	"session_planner":      true,
	"world_model_ingestor": true,
}

func (sm *ShardManager) ListAvailableShards() []ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var shards []ShardInfo

	for name := range sm.factories {
		// The registered profile is the authority on shard type. This loop used
		// to carry a hardcoded list of six system-shard names, which drifted:
		// mangle_repair, campaign_runner and legislator are all registered
		// `system` (internal/shards/registration.go) but were reported
		// `ephemeral` here. Callers that filter on ShardTypeSystem — dream and
		// shadow — therefore consulted them. campaign_runner's Execute is an
		// infinite supervision loop that only returns on cancellation, so
		// `nerd dream` blocked on it until its own 25-minute budget expired and
		// emitted no output at all. A name list cannot stay in sync with a
		// registry; the profile can.
		shardType := types.ShardTypeEphemeral
		if profile, ok := sm.profiles[name]; ok {
			shardType = profile.Type
		} else if legacySystemShardNames[name] {
			// Fallback only for a factory registered without a profile.
			// Production registration always does both, so this is unreachable
			// there; it keeps a profile-less registration from being reported
			// as ephemeral, which is the dangerous direction.
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
			Args:      []any{name, string(cfg.Type)},
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
//
// With no provider installed — which is every production process, since nothing
// calls SetReviewerFeedbackProvider and no type implements the interface — this
// used to be a silent no-op. The user told the system a finding was right and
// the system discarded it.
//
// It now falls back to the kernel, the same way CheckReviewNeedsValidation and
// GetReviewSuspectReasons already read from it. reviewer.mg's whole
// self-correction section is built on user_accepted_finding/4 and
// user_rejected_finding/5 (schemas_tools.mg:339,343) and nothing produced
// either, so review_suspect, review_rejection_count and
// reviewer_needs_validation could never fire — a learning loop complete on the
// logic side and starved on the Go side.
func (sm *ShardManager) AcceptReviewFinding(reviewID, file string, line int) {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	kernel := sm.kernel
	sm.mu.RUnlock()

	if provider != nil {
		provider.AcceptFinding(reviewID, file, line)
		return
	}
	sm.assertReviewFeedback(kernel, types.Fact{
		Predicate: "user_accepted_finding",
		Args:      []any{reviewID, file, int64(line), time.Now().Unix()},
	}, reviewID)
}

// RejectReviewFinding marks a finding as rejected by the user.
//
// The rejection is the higher-value half: reviewer.mg derives
// review_suspect(ReviewID, "multiple_rejections") from two of these, and
// reviewer_needs_validation from that — which the chat already reads back
// through CheckReviewNeedsValidation's kernel fallback. Asserting the fact
// closes that loop end to end.
func (sm *ShardManager) RejectReviewFinding(reviewID, file string, line int, reason string) {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	kernel := sm.kernel
	sm.mu.RUnlock()

	if provider != nil {
		provider.RejectFinding(reviewID, file, line, reason)
		return
	}
	if strings.TrimSpace(reason) == "" {
		reason = "unspecified"
	}
	sm.assertReviewFeedback(kernel, types.Fact{
		Predicate: "user_rejected_finding",
		Args:      []any{reviewID, file, int64(line), reason, time.Now().Unix()},
	}, reviewID)
}

// assertReviewFeedback writes one feedback fact and refreshes review_accuracy.
func (sm *ShardManager) assertReviewFeedback(kernel types.Kernel, fact types.Fact, reviewID string) {
	if kernel == nil {
		logging.Get(logging.CategoryShards).Warn(
			"Review feedback for %s discarded: no provider and no kernel", reviewID)
		return
	}
	if err := kernel.Assert(fact); err != nil {
		logging.Get(logging.CategoryShards).Warn(
			"Failed to record %s for review %s: %v", fact.Predicate, reviewID, err)
		return
	}
	sm.refreshReviewAccuracy(kernel, reviewID)
}

// reviewFeedbackCounts totals a review's accepted and rejected findings.
func reviewFeedbackCounts(kernel types.Kernel, reviewID string) (accepted, rejected int) {
	if kernel == nil {
		return 0, 0
	}
	count := func(predicate string) int {
		facts, err := kernel.Query(predicate)
		if err != nil {
			logging.Get(logging.CategoryShards).Warn("review feedback query %q failed: %v", predicate, err)
			return 0
		}
		n := 0
		for _, f := range facts {
			if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == reviewID {
				n++
			}
		}
		return n
	}
	return count("user_accepted_finding"), count("user_rejected_finding")
}

// refreshReviewAccuracy recomputes review_accuracy/5 for one review.
//
// review_accuracy is an EDB with a Decl and no rule (schemas_tools.mg:347), and
// reviewer.mg's high_rejection_rate suspicion joins on it. Recomputed rather
// than incremented because Mangle facts are a set: asserting a second
// review_accuracy row for the same review would leave both visible and make
// every rule reading it non-deterministic.
//
// Score is an integer percentage. Every numeric Mangle slot in this kernel is
// int64 — one float64 fact aborts the whole fixpoint — so the ratio goes
// through types.PercentFromRatio rather than being cast.
func (sm *ShardManager) refreshReviewAccuracy(kernel types.Kernel, reviewID string) {
	accepted, rejected := reviewFeedbackCounts(kernel, reviewID)
	total := accepted + rejected
	if total == 0 {
		return
	}

	// Retract only this review's row. types.Kernel.Retract takes a predicate
	// and drops the whole relation, which would erase every other review's
	// accuracy in the same session; RetractFact is the scoped one.
	if existing, err := kernel.Query("review_accuracy"); err == nil {
		for _, f := range existing {
			if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == reviewID {
				if err := kernel.RetractFact(f); err != nil {
					logging.ShardsDebug("review_accuracy retract before refresh failed: %v", err)
				}
			}
		}
	}
	score := types.PercentFromRatio(float64(accepted) / float64(total))
	if err := kernel.Assert(types.Fact{
		Predicate: "review_accuracy",
		Args:      []any{reviewID, int64(total), int64(accepted), int64(rejected), score},
	}); err != nil {
		logging.Get(logging.CategoryShards).Warn("Failed to record review_accuracy for %s: %v", reviewID, err)
	}
}

// GetReviewAccuracyReport returns accuracy statistics for a review session.
//
// It used to return the fixed string "Review feedback provider not available"
// in every production process, because no provider exists. It now reports what
// the kernel actually holds.
func (sm *ShardManager) GetReviewAccuracyReport(reviewID string) string {
	sm.mu.RLock()
	provider := sm.reviewerFeedback
	kernel := sm.kernel
	sm.mu.RUnlock()

	if provider != nil {
		return provider.GetAccuracyReport(reviewID)
	}
	if kernel == nil {
		return "Review feedback unavailable: no kernel attached"
	}

	accepted, rejected := reviewFeedbackCounts(kernel, reviewID)
	total := accepted + rejected
	if total == 0 {
		return fmt.Sprintf("Review %s: no findings accepted or rejected yet", reviewID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Review %s: %d finding(s) judged — %d accepted, %d rejected (%d%% accepted)",
		reviewID, total, accepted, rejected, types.PercentFromRatio(float64(accepted)/float64(total)))
	if reasons := sm.GetReviewSuspectReasons(reviewID); len(reasons) > 0 {
		fmt.Fprintf(&b, "\nFlagged suspect: %s", strings.Join(reasons, ", "))
	}
	return b.String()
}
