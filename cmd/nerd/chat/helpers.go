// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and helper functions.
package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/verification"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// ARTICULATION HELPERS
// =============================================================================

func formatResponse(intent perception.Intent, payload articulation.PiggybackEnvelope) string {
	// Keep logic artifacts internal; return only the conversational surface text.
	// Log intent for debugging if needed
	_ = intent.Verb // Mark as used
	return strings.TrimSpace(payload.Surface)
}

func payloadForArticulation(intent perception.Intent, mangleUpdates []string) articulation.PiggybackEnvelope {
	return articulation.PiggybackEnvelope{
		Surface: "",
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   intent.Category,
				Verb:       intent.Verb,
				Target:     intent.Target,
				Constraint: intent.Constraint,
				Confidence: intent.Confidence,
			},
			MangleUpdates: mangleUpdates,
		},
	}
}

// ArticulationOutput contains the full output from the articulation layer.
// This is used to pass control data back to the compressor.
type ArticulationOutput struct {
	Surface          string
	Envelope         articulation.PiggybackEnvelope
	MemoryOperations []articulation.MemoryOperation
	MangleUpdates    []string
	SelfCorrection   *articulation.SelfCorrection
	ParseMethod      string
	Warnings         []string
}

// articulateWithContext performs the articulation phase and returns the surface response.
// This is the original signature for backward compatibility.
func articulateWithContext(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string) (string, error) {
	output, err := articulateWithContextFull(ctx, client, intent, payload, contextFacts, warnings, systemPrompt)
	if err != nil {
		return "", err
	}
	return output.Surface, nil
}

// ConversationContext holds recent conversation history for LLM context injection.
// This enables fluid conversation by providing the LLM with recent turns.
// Implements the Blackboard Pattern for cross-shard and cross-turn context propagation.
type ConversationContext struct {
	RecentTurns     []Message      // Last N conversation turns
	LastShardResult *ShardResult   // Most recent shard output for follow-ups
	TurnNumber      int            // Current turn number
	ShardHistory    []*ShardResult // Sliding window of past shard results (blackboard)
	CompressedCtx   string         // Semantically compressed session context from compressor
}

// articulateWithContextFull performs articulation and returns full structured output.
// This enhanced version properly extracts all control packet data for the compressor.
func articulateWithContextFull(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string) (*ArticulationOutput, error) {
	// Use the new version with nil conversation context for backward compatibility
	return articulateWithConversation(ctx, client, intent, payload, contextFacts, warnings, systemPrompt, nil)
}

// articulateWithConversation performs articulation with full conversation context.
// This is the new entry point that enables fluid conversational follow-ups.
func articulateWithConversation(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string, convCtx *ConversationContext) (*ArticulationOutput, error) {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString("System Instructions:\n")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}

	// =========================================================================
	// CONVERSATION HISTORY INJECTION (Critical for fluid chat)
	// =========================================================================
	// Include recent conversation turns so the LLM understands context.
	// This enables follow-up questions like "what are the other suggestions?"
	if convCtx != nil && len(convCtx.RecentTurns) > 0 {
		sb.WriteString("## Recent Conversation History\n")
		sb.WriteString("(Use this context to understand follow-up questions)\n\n")
		for _, turn := range convCtx.RecentTurns {
			if turn.Role == "user" {
				sb.WriteString(fmt.Sprintf("**User**: %s\n", turn.Content))
			} else {
				// Truncate long assistant responses
				content := turn.Content
				if len(content) > 500 {
					content = content[:500] + "\n... (truncated)"
				}
				sb.WriteString(fmt.Sprintf("**Assistant**: %s\n", content))
			}
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// LAST SHARD RESULT INJECTION (Critical for follow-up queries)
	// =========================================================================
	// If there's a recent shard result, include it so follow-ups work.
	// This enables "what are the other warnings?" after a review.
	if convCtx != nil && convCtx.LastShardResult != nil {
		sr := convCtx.LastShardResult
		sb.WriteString("## Last Shard Execution Result\n")
		sb.WriteString(fmt.Sprintf("**Type**: %s\n", sr.ShardType))
		sb.WriteString(fmt.Sprintf("**Task**: %s\n", sr.Task))
		sb.WriteString(fmt.Sprintf("**Turn**: %d\n\n", sr.TurnNumber))

		// Include structured findings if available (for reviewer)
		if len(sr.Findings) > 0 {
			sb.WriteString("### All Findings (use for follow-up queries)\n")
			for i, finding := range sr.Findings {
				sb.WriteString(fmt.Sprintf("%d. ", i+1))
				for k, v := range finding {
					sb.WriteString(fmt.Sprintf("%s=%v ", k, v))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		// Include metrics if available
		if len(sr.Metrics) > 0 {
			sb.WriteString("### Metrics\n")
			for k, v := range sr.Metrics {
				sb.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
			}
			sb.WriteString("\n")
		}
	}

	// =========================================================================
	// SHARD HISTORY INJECTION (Blackboard Pattern)
	// =========================================================================
	// Include summarized history of recent shard executions for cross-shard context.
	// This enables flows like: reviewer→coder, tester→debugger, coder→tester.
	if convCtx != nil && len(convCtx.ShardHistory) > 1 {
		sb.WriteString("## Shard Execution History (Blackboard)\n")
		sb.WriteString("(Previous shard results for cross-shard context)\n\n")
		// Skip the last one since it's already shown above as LastShardResult
		for i := 0; i < len(convCtx.ShardHistory)-1; i++ {
			sr := convCtx.ShardHistory[i]
			sb.WriteString(fmt.Sprintf("- **Turn %d [%s]**: %s", sr.TurnNumber, sr.ShardType, truncateForContext(sr.Task, 50)))
			if len(sr.Findings) > 0 {
				sb.WriteString(fmt.Sprintf(" → %d findings", len(sr.Findings)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// COMPRESSED SESSION CONTEXT INJECTION (Infinite Context)
	// =========================================================================
	// Inject the semantically compressed context from the compressor.
	// This provides >100:1 compressed session history as Mangle atoms.
	if convCtx != nil && convCtx.CompressedCtx != "" {
		sb.WriteString("## Compressed Session Context\n")
		sb.WriteString("(Semantic compression of prior turns - use for long-range context)\n\n")
		sb.WriteString(convCtx.CompressedCtx)
		sb.WriteString("\n\n")
	}

	if len(contextFacts) > 0 {
		sb.WriteString("Context Facts:\n")
		for _, f := range contextFacts {
			sb.WriteString("- " + f.String() + "\n")
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range warnings {
			sb.WriteString("- " + w + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Intent: %s -> %s\n\n", intent.Verb, intent.Target))
	sb.WriteString("CRITICAL: You MUST output JSON in this EXACT order to prevent lies!\n")
	sb.WriteString("If generation fails mid-stream, control_packet must be written FIRST.\n\n")
	sb.WriteString("Required JSON Schema (THOUGHT-FIRST ordering):\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "control_packet": {` + "\n")
	sb.WriteString(`    "intent_classification": { "category": "mutation|query|instruction", "verb": "...", "target": "...", "confidence": 0.0 },` + "\n")
	sb.WriteString(`    "reasoning_trace": "optional internal notes",` + "\n")
	sb.WriteString(`    "mangle_updates": [ "predicate(arg1, arg2)." ],` + "\n")
	sb.WriteString(`    "memory_operations": [ { "op": "promote_to_long_term|forget|note|store_vector", "key": "preference_name", "value": "value" } ],` + "\n")
	sb.WriteString(`    "self_correction": { "triggered": false, "hypothesis": "" }` + "\n")
	sb.WriteString(`  },` + "\n")
	sb.WriteString(`  "surface_response": "text visible to user ONLY after control_packet is written"` + "\n")
	sb.WriteString("}\n\n")
	sb.WriteString("DO NOT speak to the user until AFTER you have written the complete control_packet!\n\n")
	sb.WriteString("MEMORY OPERATIONS:\n")
	sb.WriteString("- promote_to_long_term: Store user preferences or learned patterns\n")
	sb.WriteString("- forget: Remove outdated facts\n")
	sb.WriteString("- note: Add temporary session notes\n")
	sb.WriteString("- store_vector: Store for semantic search\n\n")
	sb.WriteString("Use only the context facts above. Do not invent filesystem access or knowledge not present in the facts. Output JSON only.")

	raw, err := client.CompleteWithSystem(ctx, systemPrompt, sb.String())
	if err != nil {
		return nil, fmt.Errorf("articulation failed: %w", err)
	}

	// Use the enhanced ResponseProcessor from articulation package
	processor := articulation.NewResponseProcessor()
	result, err := processor.Process(raw)
	if err != nil {
		return nil, fmt.Errorf("piggyback JSON invalid: %w (raw=%s)", err, raw)
	}

	// Build output
	output := &ArticulationOutput{
		Surface:          result.Surface,
		MemoryOperations: result.Control.MemoryOperations,
		MangleUpdates:    result.Control.MangleUpdates,
		ParseMethod:      result.ParseMethod,
		Warnings:         result.Warnings,
	}

	// Check for self-correction
	if result.Control.SelfCorrection != nil && result.Control.SelfCorrection.Triggered {
		output.SelfCorrection = result.Control.SelfCorrection
		output.Warnings = append(output.Warnings,
			fmt.Sprintf("Self-correction: %s", result.Control.SelfCorrection.Hypothesis))
	}

	// Build the envelope with merged data
	output.Envelope = articulation.PiggybackEnvelope{
		Surface: result.Surface,
		Control: result.Control,
	}

	// Override with any pre-existing payload data if LLM didn't provide it
	if result.Control.IntentClassification.Category == "" {
		output.Envelope.Control.IntentClassification = payload.Control.IntentClassification
	}
	if len(result.Control.MangleUpdates) == 0 && len(payload.Control.MangleUpdates) > 0 {
		output.Envelope.Control.MangleUpdates = payload.Control.MangleUpdates
		output.MangleUpdates = payload.Control.MangleUpdates
	}

	return output, nil
}

func appendFileContent(workspace, path, content string) error {
	full := resolvePath(workspace, path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func applyPatchResult(workspace, patch string) string {
	fullPatch := patch
	if !strings.HasPrefix(strings.TrimSpace(patch), "*** Begin Patch") {
		fullPatch = "*** Begin Patch\n" + patch + "\n*** End Patch\n"
	}
	tmpPath := filepath.Join(workspace, ".nerd", "last_patch.txt")
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0755); err == nil {
		_ = os.WriteFile(tmpPath, []byte(fullPatch), 0644)
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Content -Path '"+filepath.Join(workspace, ".nerd", "patch.ps1")+"' -Value $args[0]", fullPatch)
	_ = cmd.Run()
	if err := runApplyPatch(fullPatch); err != nil {
		return fmt.Sprintf("Patch failed: %v", err)
	}
	return "Patch applied."
}

func runApplyPatch(patch string) error {
	// Try git apply first, fallback to 'patch' if available
	cmd := exec.Command("git", "apply", "--whitespace=nowarn")
	cmd.Stdin = strings.NewReader(patch)
	if err := cmd.Run(); err == nil {
		return nil
	}
	if _, err := exec.LookPath("patch"); err == nil {
		cmd = exec.Command("patch", "-p0")
		cmd.Stdin = strings.NewReader(patch)
		return cmd.Run()
	}
	return fmt.Errorf("git apply and patch both unavailable")
}

func resolvePath(workspace, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

func readFileContent(workspace, path string, maxBytes int) (string, error) {
	full := resolvePath(workspace, path)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data), nil
}

func writeFileContent(workspace, path, content string) error {
	full := resolvePath(workspace, path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0644)
}

func makeDir(workspace, path string) error {
	full := resolvePath(workspace, path)
	return os.MkdirAll(full, 0755)
}

func searchInFiles(root, pattern string, maxHits int) ([]string, error) {
	matches := make([]string, 0)
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= maxHits {
			return filepath.SkipDir
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), pattern) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func (m Model) runInit() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Detect project type for profile
		projectInfo := detectProjectType(m.workspace)

		// Get Context7 API key from config or environment
		context7Key := m.Config.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}

		// Create the comprehensive initializer with all components
		initConfig := nerdinit.InitConfig{
			Workspace:       m.workspace,
			LLMClient:       m.client,
			ShardManager:    m.shardMgr,
			Timeout:         10 * time.Minute,
			Interactive:     false, // Non-interactive in chat mode
			SkipResearch:    false, // Do full research
			SkipAgentCreate: false, // Create Type 3 agents
			Context7APIKey:  context7Key,
		}

		// Ensure .nerd directory exists
		if err := createDirIfNotExists(m.workspace + "/.nerd"); err != nil {
			return errorMsg(fmt.Errorf("failed to create .nerd directory: %w", err))
		}

		initializer := nerdinit.NewInitializer(initConfig)

		// Run the comprehensive initialization
		result, err := initializer.Initialize(ctx)
		if err != nil {
			return errorMsg(fmt.Errorf("initialization failed: %w", err))
		}

		// Update profile with detected info if missing
		if result.Profile.Language == "unknown" {
			result.Profile.Language = projectInfo.Language
		}
		if result.Profile.Framework == "unknown" {
			result.Profile.Framework = projectInfo.Framework
		}
		if result.Profile.Architecture == "unknown" {
			result.Profile.Architecture = projectInfo.Architecture
		}

		// Load all generated facts into the kernel
		nerdDir := m.workspace + "/.nerd"
		factsPath := nerdDir + "/profile.mg"
		if _, statErr := os.Stat(factsPath); statErr == nil {
			// Load Mangle facts from file
			if err := m.kernel.LoadFactsFromFile(factsPath); err != nil {
				return errorMsg(fmt.Errorf("failed to load profile facts: %w", err))
			}

			// Also scan workspace to load fresh AST facts (supplemental)
			facts, scanErr := m.scanner.ScanWorkspace(m.workspace)
			if scanErr == nil {
				_ = m.kernel.LoadFacts(facts)
			}
		}

		// Initialize learning store for Autopoiesis (§8.3)
		shardsDir := nerdDir + "/shards"
		learningStore, lsErr := store.NewLearningStore(shardsDir)
		if lsErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Learning store init failed: %v", lsErr))
		}

		// Return init result with learning store (may be nil if failed)
		return initCompleteMsg{
			result:        result,
			learningStore: learningStore,
		}
	}
}

// scanCompleteMsg is sent when scan completes
type scanCompleteMsg struct {
	fileCount      int
	directoryCount int
	factCount      int
	duration       time.Duration
	err            error
}

// runScan performs a codebase rescan without full reinitialization
func (m Model) runScan() tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()

		// Scan the workspace
		facts, err := m.scanner.ScanWorkspace(m.workspace)
		if err != nil {
			return scanCompleteMsg{err: err}
		}

		// Load facts into kernel
		if loadErr := m.kernel.LoadFacts(facts); loadErr != nil {
			return scanCompleteMsg{err: loadErr}
		}

		// Also reload profile.mg if it exists
		factsPath := filepath.Join(m.workspace, ".nerd", "profile.mg")
		if _, statErr := os.Stat(factsPath); statErr == nil {
			_ = m.kernel.LoadFactsFromFile(factsPath)
		}

		// Count files and directories from facts
		fileCount := 0
		dirCount := 0
		for _, f := range facts {
			switch f.Predicate {
			case "file_topology":
				fileCount++
			case "directory":
				dirCount++
			}
		}

		return scanCompleteMsg{
			fileCount:      fileCount,
			directoryCount: dirCount,
			factCount:      len(facts),
			duration:       time.Since(startTime),
		}
	}
}

// learningStoreAdapter wraps store.LearningStore to implement core.LearningStore interface.
type learningStoreAdapter struct {
	store *store.LearningStore
}

func (a *learningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *learningStoreAdapter) Load(shardType string) ([]core.ShardLearning, error) {
	learnings, err := a.store.Load(shardType)
	if err != nil {
		return nil, err
	}
	// Convert store.Learning to core.ShardLearning
	result := make([]core.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = core.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
}

func (a *learningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *learningStoreAdapter) Close() error {
	return a.store.Close()
}

// renderInitComplete builds a summary message for initialization completion.
func (m Model) renderInitComplete(result *nerdinit.InitResult) string {
	var sb strings.Builder
	sb.WriteString("## Initialization Complete\n\n")

	sb.WriteString(fmt.Sprintf("**Project**: %s\n", result.Profile.Name))
	sb.WriteString(fmt.Sprintf("**Language**: %s\n", result.Profile.Language))
	if result.Profile.Framework != "" {
		sb.WriteString(fmt.Sprintf("**Framework**: %s\n", result.Profile.Framework))
	}
	sb.WriteString(fmt.Sprintf("**Architecture**: %s\n", result.Profile.Architecture))
	sb.WriteString(fmt.Sprintf("**Files Analyzed**: %d\n", result.Profile.FileCount))
	sb.WriteString(fmt.Sprintf("**Directories**: %d\n", result.Profile.DirectoryCount))
	sb.WriteString(fmt.Sprintf("**Facts Generated**: %d\n\n", result.FactsGenerated))

	// Show detected technologies
	if len(result.Profile.Dependencies) > 0 {
		sb.WriteString("### Detected Technologies\n\n")

		// Group dependencies by type
		var mainDeps, devDeps []string
		for _, dep := range result.Profile.Dependencies {
			depStr := dep.Name
			if dep.Version != "" {
				depStr += fmt.Sprintf(" (%s)", dep.Version)
			}

			if dep.Type == "dev" {
				devDeps = append(devDeps, depStr)
			} else {
				mainDeps = append(mainDeps, depStr)
			}
		}

		if len(mainDeps) > 0 {
			sb.WriteString("**Dependencies**:\n")
			for i, dep := range mainDeps {
				if i >= 10 {
					sb.WriteString(fmt.Sprintf("... and %d more\n", len(mainDeps)-10))
					break
				}
				sb.WriteString(fmt.Sprintf("- %s\n", dep))
			}
			sb.WriteString("\n")
		}

		if len(devDeps) > 0 && len(devDeps) <= 5 {
			sb.WriteString("**Dev Dependencies**:\n")
			for _, dep := range devDeps {
				sb.WriteString(fmt.Sprintf("- %s\n", dep))
			}
			sb.WriteString("\n")
		}
	}

	// Show created agents
	if len(result.CreatedAgents) > 0 {
		sb.WriteString("### Type 3 Agents Created\n\n")
		sb.WriteString("| Agent | Knowledge Atoms | Status |\n")
		sb.WriteString("|-------|-----------------|--------|\n")
		for _, agent := range result.CreatedAgents {
			sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", agent.Name, agent.KBSize, agent.Status))
		}
		sb.WriteString("\n")
	}

	// Tool capabilities note
	sb.WriteString("### Tool Generation\n\n")
	sb.WriteString("codeNERD can generate custom tools on-demand via the Ouroboros Loop:\n")
	sb.WriteString("- Tools are created automatically when capabilities are missing\n")
	sb.WriteString("- Each tool is compiled, safety-checked, and registered for use\n")
	sb.WriteString("- Use `/tool list` to see generated tools\n")
	sb.WriteString("- Use `/tool generate <description>` to create new tools\n\n")

	// Show warnings if any
	if len(result.Warnings) > 0 {
		sb.WriteString("### Warnings\n\n")
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", w))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("**Duration**: %.2fs\n\n", result.Duration.Seconds()))

	sb.WriteString("### Next Steps\n\n")
	sb.WriteString("- View agents: `/agents`\n")
	sb.WriteString("- Spawn an agent: `/spawn <agent> <task>`\n")
	sb.WriteString("- Define custom agents: `/define-agent <name>`\n")
	sb.WriteString("- View available tools: `/tool list`\n")
	sb.WriteString("- Query the codebase: Just ask questions!\n")

	return sb.String()
}

// getDefinedProfiles returns user-defined agent profiles
func (m Model) getDefinedProfiles() map[string]core.ShardConfig {
	profiles := make(map[string]core.ShardConfig)

	// Get profiles from shard manager
	// Note: We need to iterate through known profile names
	// For now, we'll check some common ones and any that were defined this session
	knownProfiles := []string{
		"RustExpert", "SecurityAuditor", "K8sArchitect",
		"PythonExpert", "GoExpert", "ReactExpert",
	}

	for _, name := range knownProfiles {
		if cfg, ok := m.shardMgr.GetProfile(name); ok {
			profiles[name] = cfg
		}
	}

	return profiles
}

// loadType3Agents loads Type 3 agents from the agents.json registry
func (m Model) loadType3Agents() []nerdinit.CreatedAgent {
	agents := make([]nerdinit.CreatedAgent, 0)

	// Try to load from agents.json registry
	registryPath := m.workspace + "/.nerd/agents.json"
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return agents
	}

	// Parse the registry
	var registry struct {
		Version   string                  `json:"version"`
		CreatedAt string                  `json:"created_at"`
		Agents    []nerdinit.CreatedAgent `json:"agents"`
	}

	if err := json.Unmarshal(data, &registry); err != nil {
		return agents
	}

	return registry.Agents
}

// isConversationalIntent returns true if the intent is conversational (greetings,
// help requests, general questions) rather than requiring code actions or shard work.
// These intents can use the perception response directly without articulation.
func isConversationalIntent(intent perception.Intent) bool {
	// Verbs that are conversational and don't require shard execution
	conversationalVerbs := map[string]bool{
		"/explain": true, // General explanations, greetings, capability questions
		"/read":    true, // Simple file reads (when target is "none" or empty)
	}

	// Check if it's a conversational verb
	if !conversationalVerbs[intent.Verb] {
		return false
	}

	// For /explain, it's conversational if target is generic/none
	// (actual code explanation with a specific target should go through articulation)
	if intent.Verb == "/explain" {
		target := strings.ToLower(intent.Target)
		// Generic targets indicate conversational intent
		if target == "" || target == "none" || target == "codebase" ||
			target == "hi" || target == "hello" || target == "help" ||
			strings.Contains(target, "what can you") ||
			strings.Contains(target, "what do you") ||
			strings.Contains(target, "capabilities") {
			return true
		}
	}

	// For /read with no specific target, it's conversational
	if intent.Verb == "/read" {
		target := strings.ToLower(intent.Target)
		if target == "" || target == "none" {
			return true
		}
	}

	return false
}

// =============================================================================
// TOOL HELPERS - Autopoiesis Tool Management
// =============================================================================

// renderToolList renders a list of all registered tools
func (m Model) renderToolList() string {
	var sb strings.Builder
	sb.WriteString("## Generated Tools\n\n")

	if m.autopoiesis == nil {
		sb.WriteString("*Autopoiesis not initialized*\n")
		return sb.String()
	}

	// Get the ouroboros loop's tool list via the orchestrator
	tools := m.autopoiesis.ListTools()

	if len(tools) == 0 {
		sb.WriteString("*No tools have been generated yet.*\n\n")
		sb.WriteString("Use `/tool generate <description>` to create a new tool.\n")
		return sb.String()
	}

	sb.WriteString("| Name | Description | Executions | Registered |\n")
	sb.WriteString("|------|-------------|------------|------------|\n")

	for _, tool := range tools {
		desc := tool.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %s |\n",
			tool.Name, desc, tool.ExecuteCount, tool.RegisteredAt.Format("2006-01-02")))
	}

	sb.WriteString("\n*Use `/tool run <name> <input>` to execute a tool*\n")
	sb.WriteString("*Use `/tool info <name>` for details*\n")

	return sb.String()
}

// renderToolInfo renders detailed information about a specific tool
func (m Model) renderToolInfo(toolName string) string {
	var sb strings.Builder

	if m.autopoiesis == nil {
		sb.WriteString("*Autopoiesis not initialized*\n")
		return sb.String()
	}

	info, exists := m.autopoiesis.GetToolInfo(toolName)
	if !exists {
		sb.WriteString(fmt.Sprintf("Tool `%s` not found.\n\n", toolName))
		sb.WriteString("Use `/tool list` to see available tools.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Tool: %s\n\n", info.Name))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", info.Description))
	sb.WriteString("### Details\n\n")
	sb.WriteString(fmt.Sprintf("- **Binary Path**: `%s`\n", info.BinaryPath))
	sb.WriteString(fmt.Sprintf("- **Hash**: `%s`\n", info.Hash[:16]+"..."))
	sb.WriteString(fmt.Sprintf("- **Registered**: %s\n", info.RegisteredAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Execution Count**: %d\n\n", info.ExecuteCount))

	// Get quality profile if available
	if profile := m.autopoiesis.GetToolProfile(toolName); profile != nil {
		sb.WriteString("### Quality Profile\n\n")
		sb.WriteString(fmt.Sprintf("- **Type**: %s\n", profile.ToolType))
		sb.WriteString(fmt.Sprintf("- **Expected Duration**: %v - %v\n",
			profile.Performance.ExpectedDurationMin, profile.Performance.ExpectedDurationMax))
		if profile.Output.ExpectsPagination {
			sb.WriteString("- **Expects Pagination**: Yes\n")
		}
	}

	sb.WriteString("\n*Use `/tool run " + toolName + " <input>` to execute*\n")

	return sb.String()
}

// runTool executes a generated tool asynchronously
func (m Model) runTool(toolName, input string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		// Execute the tool with quality evaluation
		output, assessment, err := m.autopoiesis.ExecuteAndEvaluateWithProfile(ctx, toolName, input)
		if err != nil {
			return errorMsg(fmt.Errorf("tool execution failed: %w", err))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Tool Execution: %s\n\n", toolName))

		if input != "" {
			sb.WriteString(fmt.Sprintf("**Input**: `%s`\n\n", input))
		}

		sb.WriteString("### Output\n\n")
		sb.WriteString("```\n")
		sb.WriteString(output)
		sb.WriteString("\n```\n\n")

		if assessment != nil {
			sb.WriteString("### Quality Assessment\n\n")
			sb.WriteString(fmt.Sprintf("- **Score**: %.2f\n", assessment.Score))
			sb.WriteString(fmt.Sprintf("- **Completeness**: %.2f\n", assessment.Completeness))
			sb.WriteString(fmt.Sprintf("- **Accuracy**: %.2f\n", assessment.Accuracy))
			if len(assessment.Issues) > 0 {
				sb.WriteString("- **Issues**: ")
				for i, issue := range assessment.Issues {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(string(issue.Type))
				}
				sb.WriteString("\n")
			}
		}

		return responseMsg(sb.String())
	}
}

// generateTool generates a new tool using the Ouroboros Loop
func (m Model) generateTool(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		var sb strings.Builder
		sb.WriteString("## Tool Generation\n\n")
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", description))

		// Detect tool need from description
		toolNeed, err := m.autopoiesis.DetectToolNeed(ctx, description)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to analyze tool need: %w", err))
		}

		if toolNeed == nil {
			sb.WriteString("Could not determine tool requirements from description.\n")
			sb.WriteString("Try being more specific about what the tool should do.\n")
			return responseMsg(sb.String())
		}

		sb.WriteString(fmt.Sprintf("**Detected Need**: %s\n", toolNeed.Name))
		sb.WriteString(fmt.Sprintf("**Purpose**: %s\n", toolNeed.Purpose))
		sb.WriteString(fmt.Sprintf("**Confidence**: %.2f\n\n", toolNeed.Confidence))

		// Execute the Ouroboros Loop
		sb.WriteString("### Ouroboros Loop Execution\n\n")

		result := m.autopoiesis.ExecuteOuroborosLoop(ctx, toolNeed)

		sb.WriteString(fmt.Sprintf("- **Stage Reached**: %s\n", result.Stage))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n\n", result.Duration))

		if result.SafetyReport != nil {
			sb.WriteString("**Safety Check**:\n")
			if result.SafetyReport.Safe {
				sb.WriteString(fmt.Sprintf("- Score: %.2f (PASSED)\n", result.SafetyReport.Score))
			} else {
				sb.WriteString("- FAILED\n")
				for _, v := range result.SafetyReport.Violations {
					sb.WriteString(fmt.Sprintf("  - %s: %s\n", v.Type, v.Description))
				}
			}
			sb.WriteString("\n")
		}

		if result.CompileResult != nil && result.CompileResult.Success {
			sb.WriteString("**Compilation**: SUCCESS\n")
			sb.WriteString(fmt.Sprintf("- Binary: `%s`\n", result.CompileResult.OutputPath))
			sb.WriteString(fmt.Sprintf("- Compile Time: %v\n\n", result.CompileResult.CompileTime))
		}

		if result.Success {
			sb.WriteString("### Tool Registered Successfully!\n\n")
			sb.WriteString(fmt.Sprintf("Tool `%s` is now available.\n\n", result.ToolName))
			sb.WriteString(fmt.Sprintf("*Use `/tool run %s <input>` to execute*\n", result.ToolName))
		} else if result.Error != nil {
			sb.WriteString(fmt.Sprintf("### Generation Failed\n\n**Error**: %v\n", result.Error))
		}

		return responseMsg(sb.String())
	}
}

// =============================================================================
// VERIFICATION HELPERS
// =============================================================================

// formatVerifiedResponse formats a response that passed verification.
func formatVerifiedResponse(
	intent perception.Intent,
	shardType string,
	task string,
	result string,
	verificationResult *verification.VerificationResult,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s Result\n\n", strings.Title(shardType)))

	if verificationResult != nil {
		sb.WriteString(fmt.Sprintf("**Verification**: ✅ Passed (confidence: %.0f%%)\n\n",
			verificationResult.Confidence*100))
	}

	sb.WriteString("### Output\n\n")
	// Truncate very long results
	if len(result) > 2000 {
		sb.WriteString(result[:2000])
		sb.WriteString("\n\n... (truncated)\n")
	} else {
		sb.WriteString(result)
	}

	return sb.String()
}

// formatVerificationEscalation formats a response when verification fails after max retries.
func formatVerificationEscalation(
	task string,
	shardType string,
	verificationResult *verification.VerificationResult,
) string {
	var sb strings.Builder

	sb.WriteString("## ⚠️ Task Escalation Required\n\n")
	sb.WriteString("The task could not be completed to quality standards after multiple attempts.\n\n")

	sb.WriteString("### Task\n")
	sb.WriteString(task)
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("### Shard Used: %s\n\n", shardType))

	if verificationResult != nil {
		sb.WriteString("### Last Verification Result\n\n")
		sb.WriteString(fmt.Sprintf("**Reason**: %s\n\n", verificationResult.Reason))

		if len(verificationResult.QualityViolations) > 0 {
			sb.WriteString("**Quality Violations Detected**:\n")
			for _, v := range verificationResult.QualityViolations {
				sb.WriteString(fmt.Sprintf("- %s\n", v))
			}
			sb.WriteString("\n")
		}

		if len(verificationResult.Evidence) > 0 {
			sb.WriteString("**Evidence**:\n")
			for _, e := range verificationResult.Evidence {
				sb.WriteString(fmt.Sprintf("- %s\n", e))
			}
			sb.WriteString("\n")
		}

		if len(verificationResult.Suggestions) > 0 {
			sb.WriteString("**Suggestions**:\n")
			for _, s := range verificationResult.Suggestions {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("### Next Steps\n\n")
	sb.WriteString("- Provide more specific requirements\n")
	sb.WriteString("- Break the task into smaller steps\n")
	sb.WriteString("- Try a different approach or shard\n")

	return sb.String()
}

// truncateForContext truncates a string for inclusion in context prompts.
// Removes newlines and truncates to maxLen characters.
func truncateForContext(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
