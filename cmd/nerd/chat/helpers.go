// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and helper functions.
package chat

import (
	"bufio"
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

func countFileLines(workspace, path string) (int64, error) {
	full := resolvePath(workspace, path)
	f, err := os.Open(full)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 256*1024)
	var lines int64
	var sawAnyByte bool
	lastByteWasNewline := false

	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			sawAnyByte = true
			lastByteWasNewline = chunk[len(chunk)-1] == '\n'
			if lastByteWasNewline {
				lines++
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	if !sawAnyByte {
		return 0, nil
	}
	if !lastByteWasNewline {
		lines++
	}
	return lines, nil
}

func (m *Model) handleStatsIntent(ctx context.Context, intent perception.Intent) (string, error) {
	target := strings.TrimSpace(intent.Target)
	if target == "" || strings.EqualFold(target, "none") {
		return "", fmt.Errorf("stats requires a file or directory target")
	}

	full := resolvePath(m.workspace, target)
	info, err := os.Stat(full)
	if err != nil {
		// If the target looks like a path, surface a clear error; otherwise the /stats key isn't wired yet.
		if strings.ContainsAny(target, `/\`) || filepath.Ext(target) != "" {
			return "", fmt.Errorf("file not found: %s", full)
		}
		return "", fmt.Errorf("unsupported stats target %q (try a file path)", target)
	}

	if info.IsDir() {
		// Directory LOC: count lines across code-ish files under the directory.
		allowedExt := map[string]bool{
			".go":     true,
			".mg":     true,
			".mangle": true,
			".dl":     true,
			".py":     true,
			".js":     true,
			".jsx":    true,
			".ts":     true,
			".tsx":    true,
			".rs":     true,
			".java":   true,
			".kt":     true,
			".c":      true,
			".cc":     true,
			".cpp":    true,
			".h":      true,
			".hpp":    true,
			".cs":     true,
			".sh":     true,
			".ps1":    true,
		}

		var totalLines int64
		var countedFiles int64
		var skippedFiles int64
		const maxFileSize = 5 * 1024 * 1024 // 5MB

		walkErr := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			name := d.Name()
			if d.IsDir() {
				// Skip hidden and dependency/cache directories.
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				if name == "bin" || name == "build" || name == "tmp" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(name))
			if !allowedExt[ext] {
				return nil
			}

			if st, statErr := os.Stat(path); statErr == nil && st.Size() > maxFileSize {
				skippedFiles++
				return nil
			}

			lines, err := countFileLines(m.workspace, path)
			if err != nil {
				skippedFiles++
				return nil
			}
			totalLines += lines
			countedFiles++
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}

		resp := fmt.Sprintf("%s: %d total lines across %d files", target, totalLines, countedFiles)
		if skippedFiles > 0 {
			resp += fmt.Sprintf(" (%d skipped)", skippedFiles)
		}
		return resp, nil
	}

	lines, err := countFileLines(m.workspace, target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: %d lines", target, lines), nil
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

func (m Model) runInitialization(force bool) tea.Cmd {
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
		progressCh := make(chan nerdinit.InitProgress, 10)

		// Forward progress to status bar
		go func() {
			for p := range progressCh {
				m.ReportStatus(p.Message)
			}
		}()

		initConfig := nerdinit.InitConfig{
			Workspace:       m.workspace,
			LLMClient:       m.client,
			ShardManager:    m.shardMgr,
			Timeout:         10 * time.Minute,
			Interactive:     false, // Non-interactive in chat mode
			SkipResearch:    false, // Do full research
			SkipAgentCreate: false, // Create Type 3 agents
			Context7APIKey:  context7Key,
			ProgressChan:    progressCh,
		}

		// Ensure .nerd directory exists
		if err := createDirIfNotExists(m.workspace + "/.nerd"); err != nil {
			return errorMsg(fmt.Errorf("failed to create .nerd directory: %w", err))
		}

		initializer, err := nerdinit.NewInitializer(initConfig)
		if err != nil {
			close(progressCh)
			return errorMsg(fmt.Errorf("failed to create initializer: %w", err))
		}

		// Run the comprehensive initialization
		result, err := initializer.Initialize(ctx)
		close(progressCh) // Stop progress forwarder

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

			// Also scan workspace to load fresh AST facts (supplemental, incremental)
			if m.scanner != nil {
				res, scanErr := m.scanner.ScanWorkspaceIncremental(ctx, m.workspace, m.localDB, world.IncrementalOptions{SkipWhenUnchanged: false})
				if scanErr == nil && res != nil && !res.Unchanged {
					_ = world.ApplyIncrementalResult(m.kernel, res)
				}
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

// runScan performs a codebase rescan without full reinitialization.
// If deep is true, it also ensures deep (Cartographer) facts are hydrated.
func (m Model) runScan(deep bool) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		m.ReportStatus("Scanning workspace...")

		// Incremental fast scan
		res, err := m.scanner.ScanWorkspaceIncremental(context.Background(), m.workspace, m.localDB, world.IncrementalOptions{SkipWhenUnchanged: true})
		if err != nil {
			return scanCompleteMsg{err: err}
		}

		if res != nil && res.Unchanged {
			m.ReportStatus("Workspace unchanged")
			return scanCompleteMsg{
				fileCount:      res.FileCount,
				directoryCount: res.DirectoryCount,
				factCount:      0,
				duration:       res.Duration,
			}
		}

		m.ReportStatus("Updating kernel...")
		if applyErr := world.ApplyIncrementalResult(m.kernel, res); applyErr != nil {
			return scanCompleteMsg{err: applyErr}
		}

		// Persist delta facts to knowledge DB and KG links
		if m.virtualStore != nil && res != nil && len(res.NewFacts) > 0 {
			_ = m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5)
			for _, f := range res.NewFacts {
				switch f.Predicate {
				case "dependency_link":
					if len(f.Args) >= 2 {
						a := fmt.Sprintf("%v", f.Args[0])
						b := fmt.Sprintf("%v", f.Args[1])
						rel := "depends_on"
						if len(f.Args) >= 3 {
							rel = "depends_on:" + fmt.Sprintf("%v", f.Args[2])
						}
						_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "scan"})
					}
				case "symbol_graph":
					if len(f.Args) >= 4 {
						sid := fmt.Sprintf("%v", f.Args[0])
						file := fmt.Sprintf("%v", f.Args[3])
						_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]interface{}{"source": "scan"})
					}
				}
			}
		}

		// Reload profile facts if present
		factsPath := filepath.Join(m.workspace, ".nerd", "profile.mg")
		if _, statErr := os.Stat(factsPath); statErr == nil {
			_ = m.kernel.LoadFactsFromFile(factsPath)
		}

		// Optional deep scan (on-demand)
		if deep {
			_ = m.ensureDeepWorldFacts()
		}

		m.ReportStatus("Scan complete")
		fileCount := 0
		dirCount := 0
		if res != nil {
			fileCount = res.FileCount
			dirCount = res.DirectoryCount
		}
		factCount := 0
		if res != nil {
			factCount = len(res.NewFacts)
		}
		return scanCompleteMsg{
			fileCount:      fileCount,
			directoryCount: dirCount,
			factCount:      factCount,
			duration:       time.Since(startTime),
		}
	}
}

// docRefreshCompleteMsg signals completion of document refresh.
type docRefreshCompleteMsg struct {
	docsDiscovered int
	docsProcessed  int
	atomsStored    int
	duration       time.Duration
	err            error
}

// runDocRefresh scans for new/changed documentation and updates the knowledge base.
// Uses Mangle tracking to only process documents that have changed since last run.
func (m Model) runDocRefresh(force bool) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		m.ReportStatus("Discovering documentation files...")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Create initializer for doc processing (reuses init infrastructure)
		initConfig := nerdinit.InitConfig{
			Workspace:    m.workspace,
			LLMClient:    m.client,
			ShardManager: m.shardMgr,
			Timeout:      15 * time.Minute,
			Interactive:  false,
		}

		initializer, err := nerdinit.NewInitializer(initConfig)
		if err != nil {
			return docRefreshCompleteMsg{err: fmt.Errorf("failed to create initializer: %w", err)}
		}

		// Gather all documentation
		allDocs := initializer.GatherProjectDocumentation()
		if len(allDocs) == 0 {
			return docRefreshCompleteMsg{
				docsDiscovered: 0,
				duration:       time.Since(startTime),
			}
		}

		m.ReportStatus(fmt.Sprintf("Found %d docs, processing with Mangle tracking...", len(allDocs)))

		// Process with tracking (handles resumption, change detection, incremental storage)
		state, err := initializer.ProcessDocumentsWithTracking(ctx, allDocs, m.localDB, m.kernel)
		if err != nil {
			return docRefreshCompleteMsg{err: fmt.Errorf("document processing failed: %w", err)}
		}

		// If synthesis is ready and we have stored docs, run synthesis
		if state.SynthesisReady && state.TotalStored > 0 {
			m.ReportStatus("Synthesizing strategic knowledge from stored atoms...")
			knowledge, synthErr := initializer.SynthesizeFromStoredAtoms(ctx, m.localDB, state)
			if synthErr != nil {
				// Log but don't fail - we still stored the individual atoms
				m.ReportStatus(fmt.Sprintf("Synthesis warning: %v", synthErr))
			} else if knowledge != nil {
				// Persist the synthesized knowledge
				if _, persistErr := initializer.PersistStrategicKnowledge(ctx, knowledge, m.localDB); persistErr != nil {
					m.ReportStatus(fmt.Sprintf("Persist warning: %v", persistErr))
				}
			}
		}

		m.ReportStatus("Document refresh complete")
		return docRefreshCompleteMsg{
			docsDiscovered: state.TotalDiscovered,
			docsProcessed:  state.TotalProcessed,
			atomsStored:    state.TotalStored,
			duration:       time.Since(startTime),
		}
	}
}

// ensureDeepWorldFacts hydrates deep Cartographer facts for Go files.
// This is on-demand only (e.g., `/scan --deep`).
func (m *Model) ensureDeepWorldFacts() error {
	if m.kernel == nil || m.scanner == nil {
		return nil
	}

	fileFacts, _ := m.kernel.Query("file_topology")
	goFiles := make([]string, 0)
	for _, f := range fileFacts {
		if len(f.Args) < 3 {
			continue
		}
		path, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		langAtom, ok := f.Args[2].(core.MangleAtom)
		if !ok {
			continue
		}
		if string(langAtom) == "/go" {
			goFiles = append(goFiles, path)
		}
	}
	if len(goFiles) == 0 {
		return nil
	}

	deepWorkers := 0
	if m.Config != nil {
		deepWorkers = m.Config.GetWorldConfig().DeepWorkers
	}

	res, err := world.EnsureDeepFacts(context.Background(), goFiles, m.localDB, deepWorkers)
	if err != nil || res == nil || len(res.NewFacts) == 0 {
		return err
	}

	if len(res.RetractFacts) > 0 {
		_ = m.kernel.RetractExactFactsBatch(res.RetractFacts)
	}
	if loadErr := m.kernel.LoadFacts(res.NewFacts); loadErr != nil {
		return loadErr
	}

	if m.virtualStore != nil {
		_ = m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 6)
		for _, f := range res.NewFacts {
			switch f.Predicate {
			case "dependency_link":
				if len(f.Args) >= 2 {
					a := fmt.Sprintf("%v", f.Args[0])
					b := fmt.Sprintf("%v", f.Args[1])
					rel := "depends_on"
					if len(f.Args) >= 3 {
						rel = "depends_on:" + fmt.Sprintf("%v", f.Args[2])
					}
					_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "scan-deep"})
				}
			case "symbol_graph":
				if len(f.Args) >= 4 {
					sid := fmt.Sprintf("%v", f.Args[0])
					file := fmt.Sprintf("%v", f.Args[3])
					_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]interface{}{"source": "scan-deep"})
				}
			}
		}
	}

	return nil
}

// runPartialScan scans specific file paths (non-recursive) and persists facts.
func (m Model) runPartialScan(paths []string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		m.ReportStatus(fmt.Sprintf("Scanning %d paths...", len(paths)))
		parser := world.NewASTParser()
		defer parser.Close()

		var totalFacts int
		for _, raw := range paths {
			path := strings.TrimSpace(raw)
			if path == "" {
				continue
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(m.workspace, path)
			}
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}

			ft := buildFileTopologyFact(path, info)
			_ = m.kernel.LoadFacts([]core.Fact{ft})
			if m.virtualStore != nil {
				_ = m.virtualStore.PersistFactsToKnowledge([]core.Fact{ft}, "fact", 5)
			}
			totalFacts++

			astFacts, parseErr := parser.Parse(path)
			if parseErr == nil && len(astFacts) > 0 {
				_ = m.kernel.LoadFacts(astFacts)
				totalFacts += len(astFacts)
				if m.virtualStore != nil {
					_ = m.virtualStore.PersistFactsToKnowledge(astFacts, "fact", 6)
					for _, f := range astFacts {
						switch f.Predicate {
						case "dependency_link":
							if len(f.Args) >= 2 {
								a := fmt.Sprintf("%v", f.Args[0])
								b := fmt.Sprintf("%v", f.Args[1])
								rel := "depends_on"
								if len(f.Args) >= 3 {
									rel = "depends_on:" + fmt.Sprintf("%v", f.Args[2])
								}
								_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "scan-path"})
							}
						case "symbol_graph":
							if len(f.Args) >= 4 {
								sid := fmt.Sprintf("%v", f.Args[0])
								file := fmt.Sprintf("%v", f.Args[3])
								_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]interface{}{"source": "scan-path"})
							}
						}
					}
				}
			}
		}

		m.ReportStatus("Scan complete")
		return scanCompleteMsg{
			fileCount:      len(paths),
			directoryCount: 0,
			factCount:      totalFacts,
			duration:       time.Since(start),
		}
	}
}

// runDirScan scans a directory recursively and persists facts (lighter than full init).
func (m Model) runDirScan(dir string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(m.workspace, dir)
		}
		m.ReportStatus(fmt.Sprintf("Scanning directory: %s", dir))
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return scanCompleteMsg{err: fmt.Errorf("invalid directory: %s", dir)}
		}

		parser := world.NewASTParser()
		defer parser.Close()

		fileCount := 0
		dirCount := 0
		factCount := 0

		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				dirCount++
				// skip hidden dirs
				if strings.HasPrefix(d.Name(), ".") && path != dir {
					return filepath.SkipDir
				}
				return nil
			}
			fileCount++
			if fileCount%10 == 0 {
				m.ReportStatus(fmt.Sprintf("Scanning... (%d files)", fileCount))
			}
			info, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			ft := buildFileTopologyFact(path, info)
			_ = m.kernel.LoadFacts([]core.Fact{ft})
			if m.virtualStore != nil {
				_ = m.virtualStore.PersistFactsToKnowledge([]core.Fact{ft}, "fact", 5)
			}
			factCount++

			astFacts, parseErr := parser.Parse(path)
			if parseErr == nil && len(astFacts) > 0 {
				_ = m.kernel.LoadFacts(astFacts)
				factCount += len(astFacts)
				if m.virtualStore != nil {
					_ = m.virtualStore.PersistFactsToKnowledge(astFacts, "fact", 6)
					for _, f := range astFacts {
						switch f.Predicate {
						case "dependency_link":
							if len(f.Args) >= 2 {
								a := fmt.Sprintf("%v", f.Args[0])
								b := fmt.Sprintf("%v", f.Args[1])
								rel := "depends_on"
								if len(f.Args) >= 3 {
									rel = "depends_on:" + fmt.Sprintf("%v", f.Args[2])
								}
								_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "scan-dir"})
							}
						case "symbol_graph":
							if len(f.Args) >= 4 {
								sid := fmt.Sprintf("%v", f.Args[0])
								file := fmt.Sprintf("%v", f.Args[3])
								_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]interface{}{"source": "scan-dir"})
							}
						}
					}
				}
			}
			return nil
		})

		m.ReportStatus("Scan complete")
		return scanCompleteMsg{
			fileCount:      fileCount,
			directoryCount: dirCount,
			factCount:      factCount,
			duration:       time.Since(start),
		}
	}
}

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
		Args: []interface{}{
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

func (a *learningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]core.ShardLearning, error) {
	learnings, err := a.store.LoadByPredicate(shardType, predicate)
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
	sb.WriteString(fmt.Sprintf("| Metric | Count |\n|--------|-------|\n"))
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
// help requests, general questions, stats) rather than requiring code actions or shard work.
// These intents can use the perception response directly without articulation.
func isConversationalIntent(intent perception.Intent) bool {
	// Verbs that are ALWAYS conversational and don't require shard execution
	alwaysConversational := map[string]bool{
		"/greet":     true, // Greetings: hello, hi, hey
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
		"/explain": true, // General explanations (but not code-specific)
		"/read":    true, // Simple file reads (when target is "none" or empty)
	}

	// Check if it's a conditional verb
	if !conditionalVerbs[intent.Verb] {
		return false
	}

	// For /explain, it's conversational if target is generic/conceptual (not a specific file/function)
	// Concept explanations can use perception response directly; code explanations need articulation
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
		// Conceptual/system explanations (not file paths or code symbols)
		// These don't need workspace context, just general knowledge
		if !strings.Contains(target, ".") && !strings.Contains(target, "/") &&
			!strings.Contains(target, "::") && !strings.Contains(target, "(") {
			// No file extension, path separator, scope operator, or parens = likely conceptual
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Extended for LLM-based tools
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		m.ReportStatus(fmt.Sprintf("Executing tool: %s...", toolName))

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

		m.ReportStatus("Tool execution complete")
		return responseMsg(sb.String())
	}
}

// generateTool generates a new tool using the Ouroboros Loop
func (m Model) generateTool(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // Ouroboros has multiple LLM stages
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		m.ReportStatus("Detecting tool requirements...")

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
		m.ReportStatus(fmt.Sprintf("Generating tool: %s...", toolNeed.Name))
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
		} else if result.Error != "" {
			sb.WriteString(fmt.Sprintf("### Generation Failed\n\n**Error**: %s\n", result.Error))
		}

		m.ReportStatus("Tool generation complete")
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
	sb.WriteString(result)

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
