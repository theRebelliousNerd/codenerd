// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and helper functions.
package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/store"
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

func articulateWithContext(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string) (string, error) {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString("System Instructions:\n")
		sb.WriteString(systemPrompt)
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
	sb.WriteString("You MUST respond only with JSON (no extra text). Schema:\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "surface_response": "text visible to user",` + "\n")
	sb.WriteString(`  "control_packet": {` + "\n")
	sb.WriteString(`    "intent_classification": { "category": "mutation|query|instruction", "verb": "...", "target": "...", "confidence": 0.0 },` + "\n")
	sb.WriteString(`    "reasoning_trace": "optional",` + "\n")
	sb.WriteString(`    "mangle_updates": [ "atom(...)" ],` + "\n")
	sb.WriteString(`    "memory_operations": [ { "op": "promote_to_long_term|forget|note", "key": "k", "value": "v" } ],` + "\n")
	sb.WriteString(`    "self_correction": { "triggered": false, "hypothesis": "" }` + "\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")
	sb.WriteString("Use only the context facts above. Do not invent filesystem access or knowledge not present in the facts. Output JSON only.")

	raw, err := client.CompleteWithSystem(ctx, systemPrompt, sb.String())
	if err != nil {
		return "", fmt.Errorf("articulation failed: %w", err)
	}

	type llmPayload struct {
		SurfaceResponse string `json:"surface_response"`
		ControlPacket   struct {
			IntentClassification articulation.IntentClassification `json:"intent_classification"`
			MangleUpdates        []string                          `json:"mangle_updates"`
			MemoryOperations     []articulation.MemoryOperation    `json:"memory_operations"`
			SelfCorrection       map[string]interface{}            `json:"self_correction"`
			ReasoningTrace       string                            `json:"reasoning_trace"`
		} `json:"control_packet"`
	}

	var parsed llmPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil || parsed.SurfaceResponse == "" {
		return "", fmt.Errorf("piggyback JSON invalid: %w (raw=%s)", err, raw)
	}

	// Apply control data from LLM
	if parsed.ControlPacket.IntentClassification.Category != "" {
		payload.Control.IntentClassification = parsed.ControlPacket.IntentClassification
	}
	if len(parsed.ControlPacket.MangleUpdates) > 0 {
		payload.Control.MangleUpdates = parsed.ControlPacket.MangleUpdates
	}
	if len(parsed.ControlPacket.MemoryOperations) > 0 {
		payload.Control.MemoryOperations = parsed.ControlPacket.MemoryOperations
	}

	payload.Surface = parsed.SurfaceResponse
	return formatResponse(intent, payload), nil
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
		factsPath := nerdDir + "/profile.gl"
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

		// Also reload profile.gl if it exists
		factsPath := filepath.Join(m.workspace, ".nerd", "profile.gl")
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
