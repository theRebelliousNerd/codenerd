// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and helper functions.
package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/verification"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildFileTopologyFact constructs a file_topology fact with hash/lang/test flag.
func buildFileTopologyFact(path string, info os.FileInfo) core.Fact {
	data, _ := os.ReadFile(path)
	hash := sha256.Sum256(data)
	lang := detectLanguage(path)
	isTest := "/false"
	if isTestFile(path) {
		isTest = "/true"
	}
	return core.Fact{
		Predicate: "file_topology",
		Args: []any{
			path,
			hex.EncodeToString(hash[:]),
			"/" + lang,
			info.ModTime().Unix(),
			isTest,
		},
	}
}

// detectLanguage is a lightweight extension-based detector.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".cs":
		return "csharp"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return "unknown"
	}
}

// isTestFile determines if a path is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	if strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".test.tsx") {
		return true
	}
	if strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".spec.tsx") {
		return true
	}
	if strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "_test.rs") {
		return true
	}
	return false
}

// learningStoreAdapter wraps store.LearningStore to implement core.LearningStore interface.
type learningStoreAdapter struct {
	store *store.LearningStore
}

func (a *learningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *learningStoreAdapter) Load(shardType string) ([]types.ShardLearning, error) {
	learnings, err := a.store.Load(shardType)
	if err != nil {
		return nil, err
	}
	// Convert store.Learning to core.ShardLearning
	result := make([]types.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = types.ShardLearning{
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

func (a *learningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	learnings, err := a.store.LoadByPredicate(shardType, predicate)
	if err != nil {
		return nil, err
	}
	// Convert store.Learning to core.ShardLearning
	result := make([]types.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = types.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
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

// renderWorkspaceSummary generates a friendly, experience-level-appropriate summary.
// This is shown after scan completes to give users immediate context about their project.
func (m Model) renderWorkspaceSummary(fileCount, dirCount, factCount int, experienceLevel string) string {
	var sb strings.Builder

	// Get project context from kernel facts
	var projectName, mainLang, framework string
	if m.kernel != nil {
		// Try to get project profile facts
		if facts, _ := m.kernel.Query("project_profile"); len(facts) > 0 {
			if len(facts[0].Args) > 0 {
				projectName, _ = facts[0].Args[0].(string)
			}
			if len(facts[0].Args) > 1 {
				if atom, ok := facts[0].Args[1].(core.MangleAtom); ok {
					mainLang = strings.TrimPrefix(string(atom), "/")
				}
			}
			if len(facts[0].Args) > 2 {
				if atom, ok := facts[0].Args[2].(core.MangleAtom); ok {
					framework = strings.TrimPrefix(string(atom), "/")
				}
			}
		}
	}

	// Friendly header based on experience level
	switch experienceLevel {
	case "beginner":
		sb.WriteString("## Your Workspace is Ready!\n\n")
		sb.WriteString("I've analyzed your codebase and I'm ready to help.\n\n")
	case "expert":
		sb.WriteString("## Scan Complete\n\n")
	default:
		sb.WriteString("## Workspace Indexed\n\n")
	}

	// Show project info if detected
	if projectName != "" || mainLang != "" {
		sb.WriteString("**Project**: ")
		if projectName != "" {
			sb.WriteString(projectName)
		} else {
			sb.WriteString("(unnamed)")
		}
		if mainLang != "" {
			sb.WriteString(fmt.Sprintf(" • %s", mainLang))
		}
		if framework != "" {
			sb.WriteString(fmt.Sprintf(" • %s", framework))
		}
		sb.WriteString("\n\n")
	}

	// Show stats
	sb.WriteString("| Metric | Count |\n|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Files | %d |\n", fileCount))
	sb.WriteString(fmt.Sprintf("| Directories | %d |\n", dirCount))
	sb.WriteString(fmt.Sprintf("| Facts | %d |\n\n", factCount))

	// Experience-level specific tips
	switch experienceLevel {
	case "beginner":
		sb.WriteString("### Quick Start\n\n")
		sb.WriteString("Here are some things you can try:\n\n")
		sb.WriteString("- **Ask questions**: Just type naturally, like \"What does the main function do?\"\n")
		sb.WriteString("- **Get a code review**: Type `/review`\n")
		sb.WriteString("- **Run tests**: Type `/test`\n")
		sb.WriteString("- **Get help**: Type `/help` anytime\n")
	case "intermediate":
		sb.WriteString("### Suggested Commands\n\n")
		sb.WriteString("| Command | Description |\n|---------|-------------|\n")
		sb.WriteString("| `/review` | Code review + security scan |\n")
		sb.WriteString("| `/test` | Run and analyze tests |\n")
		sb.WriteString("| `/research <topic>` | Deep-dive into a topic |\n")
		sb.WriteString("| `/query <predicate>` | Query Mangle facts |\n")
	case "advanced", "expert":
		sb.WriteString("### Available Queries\n\n")
		sb.WriteString("```\n")
		sb.WriteString("/query file_topology      # All files\n")
		sb.WriteString("/query symbol_graph       # Functions/classes\n")
		sb.WriteString("/query dependency_link    # Dependencies\n")
		sb.WriteString("/why next_action          # Derivation trace\n")
		sb.WriteString("```\n")
	default:
		sb.WriteString("Type `/help` for available commands.\n")
	}

	return sb.String()
}

// getDefinedProfiles returns user-defined agent profiles
func (m Model) getDefinedProfiles() map[string]types.ShardConfig {
	profiles := make(map[string]types.ShardConfig)

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
// help requests, general questions, stats) rather than requiring code actions or shard work.
// These intents can use the perception response directly without articulation.
func isConversationalIntent(intent perception.Intent) bool {
	// Verbs that are ALWAYS conversational and don't require shard execution
	alwaysConversational := map[string]bool{
		"/greet":     true, // Greetings: hello, hi, hey
		"/converse":  true, // Casual chat: mapped from action_type "chat"
		"/help":      true, // Capability questions: what can you do?
		"/knowledge": true, // Memory queries: what do you remember?
		"/shadow":    true, // What-if queries: what would happen if?
		"/dream":     true, // Dream mode queries: hypothetical scenarios
		"/configure": true, // Configuration instructions: preferences, settings
	}

	// If it's an always-conversational verb, return true immediately
	if alwaysConversational[intent.Verb] {
		return true
	}

	// Verbs that are conditionally conversational based on target
	conditionalVerbs := map[string]bool{
		"/read":    true, // Simple file reads (when target is "none" or empty)
		"/explain": true, // Meta-questions about the agent itself are conversational
	}

	// Check if it's a conditional verb
	if !conditionalVerbs[intent.Verb] {
		return false
	}

	// For /read with no specific target, it's conversational
	if intent.Verb == "/read" {
		target := strings.ToLower(intent.Target)
		if target == "" || target == "none" {
			return true
		}
	}

	// For /explain: meta-questions about the agent itself are conversational.
	// Codebase explanations (target = specific file/symbol) need articulation
	// so the LLM can emit knowledge_requests for unknown topics.
	if intent.Verb == "/explain" {
		target := strings.ToLower(intent.Target)
		return target == "capabilities" || target == "session"
	}

	return false
}

// =============================================================================
// VERIFICATION HELPERS
// =============================================================================

// formatVerifiedResponse formats a response that passed verification.
//
// A shard's raw `result` may be either plain text or a piggyback envelope
// (JSON with control_packet + surface_response) — for example when a
// downstream shard delegates back to articulation. Dumping the envelope
// directly produces the noisy "### Output\n\n{control_packet:..., ...}"
// blob users have been seeing. Extract surface_response when present so
// the display stays clean. If parsing fails (genuine plain text), we
// preserve the raw result unchanged.
func formatVerifiedResponse(
	intent perception.Intent,
	shardType string,
	task string,
	result string,
	verificationResult *verification.VerificationResult,
) string {
	displayResult := strings.TrimSpace(result)
	if looksLikeEnvelope(displayResult) {
		if processed := articulation.ProcessLLMResponseAllowPlain(displayResult); processed != nil &&
			processed.ParseMethod != "fallback" && strings.TrimSpace(processed.Surface) != "" {
			displayResult = strings.TrimSpace(processed.Surface)
		}
	}

	var sb strings.Builder

	// Include intent/task in header for traceability
	if task != "" {
		sb.WriteString(fmt.Sprintf("<!-- Task: %s (%s) -->\n", task, intent.Verb))
	}

	sb.WriteString(fmt.Sprintf("## %s Result\n\n", strings.Title(shardType)))

	if verificationResult != nil {
		sb.WriteString(fmt.Sprintf("**Verification**: ✅ Passed (confidence: %.0f%%)\n\n",
			verificationResult.Confidence*100))
	}

	// Include the LLM's surface response if meaningful
	if intent.Response != "" && len(intent.Response) < 500 {
		sb.WriteString(fmt.Sprintf("> %s\n\n", intent.Response))
	}

	sb.WriteString("### Output\n\n")
	sb.WriteString(displayResult)

	return sb.String()
}

// looksLikeEnvelope reports whether a string is plausibly a piggyback
// envelope worth running through the response processor. Used to avoid
// invoking the JSON parser on every plain-text shard result.
func looksLikeEnvelope(s string) bool {
	if len(s) < 20 {
		return false
	}
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "{") {
		return strings.Contains(trimmed, `"surface_response"`) ||
			strings.Contains(trimmed, `"control_packet"`)
	}
	return false
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
		// Synthesize a Reason from QualityViolations + Evidence when the
		// verifier LLM returns success=false without filling in `reason`
		// in the JSON response (observed against Gemini 3.5-flash). The
		// previous template printed "**Reason**: " with a blank tail and
		// gave the user no useful signal.
		reason := strings.TrimSpace(verificationResult.Reason)
		if reason == "" {
			parts := make([]string, 0, 2)
			if len(verificationResult.QualityViolations) > 0 {
				viols := make([]string, 0, len(verificationResult.QualityViolations))
				for _, v := range verificationResult.QualityViolations {
					viols = append(viols, string(v))
				}
				parts = append(parts, "violations="+strings.Join(viols, ","))
			}
			if len(verificationResult.Evidence) > 0 {
				parts = append(parts, "evidence="+verificationResult.Evidence[0])
			}
			if len(parts) == 0 {
				reason = "(verifier returned no reason — see logs)"
			} else {
				reason = strings.Join(parts, "; ")
			}
		}
		sb.WriteString(fmt.Sprintf("**Reason**: %s\n\n", reason))

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

// =============================================================================
// JIT PROMPT COMPILER HELPERS
// =============================================================================

// renderJITStatus renders the JIT Prompt Compiler status and last compilation result.
func (m Model) renderJITStatus() string {
	var sb strings.Builder

	sb.WriteString("## JIT Prompt Compiler Status\n\n")

	if m.jitCompiler == nil {
		sb.WriteString("**Status**: ❌ Not initialized\n\n")
		sb.WriteString("The JIT Prompt Compiler is not available. This may indicate:\n")
		sb.WriteString("- Initialization failure during boot\n")
		sb.WriteString("- Missing embedded corpus\n")
		sb.WriteString("- Configuration issue\n")
		return sb.String()
	}

	sb.WriteString("**Status**: ✅ Active\n\n")

	// Get compiler stats
	stats := m.jitCompiler.GetStats()
	sb.WriteString("### Compiler Stats\n\n")
	sb.WriteString(fmt.Sprintf("- Embedded Atom Count: %d\n", stats.EmbeddedAtomCount))
	sb.WriteString(fmt.Sprintf("- Shard DBs Loaded: %d\n", stats.ShardDBCount))
	sb.WriteString("\n")

	// Get last compilation result
	result := m.jitCompiler.GetLastResult()
	if result == nil {
		sb.WriteString("### Last Compilation\n\n")
		sb.WriteString("_No compilations yet this session._\n")
		return sb.String()
	}

	sb.WriteString("### Last Compilation Result\n\n")
	sb.WriteString(fmt.Sprintf("- **Tokens Used**: %d (%.1f%% of budget)\n",
		result.TotalTokens, result.BudgetUsed*100))
	sb.WriteString(fmt.Sprintf("- **Atoms Included**: %d\n", result.AtomsIncluded))

	// Show timing breakdown
	if result.Stats != nil {
		sb.WriteString("\n### Timing Breakdown\n\n")
		sb.WriteString(fmt.Sprintf("- Collect Atoms: %dms\n", result.Stats.CollectAtomsMs))
		sb.WriteString(fmt.Sprintf("- Select Atoms: %dms (vector: %dms)\n",
			result.Stats.SelectAtomsMs, result.Stats.VectorQueryMs))
		sb.WriteString(fmt.Sprintf("- Resolve Deps: %dms\n", result.Stats.ResolveDepsMs))
		sb.WriteString(fmt.Sprintf("- Fit Budget: %dms\n", result.Stats.FitBudgetMs))
		sb.WriteString(fmt.Sprintf("- Assemble: %dms\n", result.Stats.AssembleMs))
		sb.WriteString(fmt.Sprintf("- **Total**: %dms\n", result.Stats.Duration.Milliseconds()))
	}

	// Show included atoms
	if len(result.IncludedAtoms) > 0 {
		sb.WriteString("\n### Included Atoms\n\n")
		sb.WriteString("| Category | ID | Tokens |\n")
		sb.WriteString("|----------|----|---------|\n")
		shown := 0
		for _, atom := range result.IncludedAtoms {
			if shown >= 10 {
				sb.WriteString(fmt.Sprintf("| ... | _+%d more_ | |\n", len(result.IncludedAtoms)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", atom.Category, atom.ID, atom.TokenCount))
			shown++
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("_Use Alt+P to toggle the Prompt Inspector view._\n")

	return sb.String()
}
