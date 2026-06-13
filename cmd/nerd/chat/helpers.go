// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and helper functions.
package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) runInitialization(force bool) tea.Cmd {
	return func() tea.Msg {
		if force {
			m.ReportStatus("Forcing full initialization...")
		}
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
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
					if err := world.ApplyIncrementalResult(m.kernel, res); err != nil {
						logging.Routing("[helpers] failed to apply incremental result: %v", err)
					}
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

		if m.scanner == nil {
			return scanCompleteMsg{err: fmt.Errorf("scanner not initialized")}
		}

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
			if err := m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5); err != nil {
				logging.Routing("[helpers] failed to persist facts to knowledge: %v", err)
			}
			for _, f := range res.NewFacts {
				switch f.Predicate {
				case "dependency_link":
					if len(f.Args) >= 2 {
						a := types.ExtractString(f.Args[0])
						b := types.ExtractString(f.Args[1])
						rel := "depends_on"
						if len(f.Args) >= 3 {
							rel = "depends_on:" + types.ExtractString(f.Args[2])
						}
						if err := m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]any{"source": "scan"}); err != nil {
							logging.Routing("[helpers] failed to persist dependency link: %v", err)
						}
					}
				case "symbol_graph":
					if len(f.Args) >= 4 {
						sid := types.ExtractString(f.Args[0])
						file := types.ExtractString(f.Args[3])
						if err := m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]any{"source": "scan"}); err != nil {
							logging.Routing("[helpers] failed to persist symbol link: %v", err)
						}
					}
				}
			}
		}

		// Reload profile facts if present
		factsPath := filepath.Join(m.workspace, ".nerd", "profile.mg")
		if _, statErr := os.Stat(factsPath); statErr == nil {
			if err := m.kernel.LoadFactsFromFile(factsPath); err != nil {
				logging.Kernel("[helpers] failed to load profile facts from file: %v", err)
			}
		}

		// Optional deep scan (on-demand)
		if deep {
			if err := m.ensureDeepWorldFacts(); err != nil {
				logging.Routing("[helpers] failed to ensure deep world facts: %v", err)
			}
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
		if force {
			m.ReportStatus("Forcing document refresh...")
		}
		startTime := time.Now()
		m.ReportStatus("Discovering documentation files...")

		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().DocumentProcessingTimeout)
		defer cancel()

		// Create initializer for doc processing (reuses init infrastructure)
		initConfig := nerdinit.InitConfig{
			Workspace:    m.workspace,
			LLMClient:    m.client,
			ShardManager: m.shardMgr,
			Timeout:      config.GetLLMTimeouts().DocumentProcessingTimeout,
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
		if err := m.kernel.RetractExactFactsBatch(res.RetractFacts); err != nil {
			logging.Kernel("[helpers] failed to retract facts batch: %v", err)
		}
	}
	if loadErr := m.kernel.LoadFacts(res.NewFacts); loadErr != nil {
		return loadErr
	}

	if m.virtualStore != nil {
		if err := m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 6); err != nil {
			logging.Routing("[helpers] failed to persist deep facts to knowledge: %v", err)
		}
		for _, f := range res.NewFacts {
			switch f.Predicate {
			case "dependency_link":
				if len(f.Args) >= 2 {
					a := types.ExtractString(f.Args[0])
					b := types.ExtractString(f.Args[1])
					rel := "depends_on"
					if len(f.Args) >= 3 {
						rel = "depends_on:" + types.ExtractString(f.Args[2])
					}
					if err := m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]any{"source": "scan-deep"}); err != nil {
						logging.Routing("[helpers] failed to persist deep dependency link: %v", err)
					}
				}
			case "symbol_graph":
				if len(f.Args) >= 4 {
					sid := types.ExtractString(f.Args[0])
					file := types.ExtractString(f.Args[3])
					if err := m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]any{"source": "scan-deep"}); err != nil {
						logging.Routing("[helpers] failed to persist deep symbol link: %v", err)
					}
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
			if err := m.kernel.LoadFacts([]core.Fact{ft}); err != nil {
				logging.Kernel("[helpers] failed to load file topology fact: %v", err)
			}
			if m.virtualStore != nil {
				if err := m.virtualStore.PersistFactsToKnowledge([]core.Fact{ft}, "fact", 5); err != nil {
					logging.Routing("[helpers] failed to persist file topology: %v", err)
				}
			}
			totalFacts++

			astFacts, parseErr := parser.Parse(path)
			if parseErr == nil && len(astFacts) > 0 {
				if err := m.kernel.LoadFacts(astFacts); err != nil {
					logging.Kernel("[helpers] failed to load AST facts: %v", err)
				}
				totalFacts += len(astFacts)
				if m.virtualStore != nil {
					if err := m.virtualStore.PersistFactsToKnowledge(astFacts, "fact", 6); err != nil {
						logging.Routing("[helpers] failed to persist AST facts: %v", err)
					}
					for _, f := range astFacts {
						switch f.Predicate {
						case "dependency_link":
							if len(f.Args) >= 2 {
								a := types.ExtractString(f.Args[0])
								b := types.ExtractString(f.Args[1])
								rel := "depends_on"
								if len(f.Args) >= 3 {
									rel = "depends_on:" + types.ExtractString(f.Args[2])
								}
								if err := m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]any{"source": "scan-path"}); err != nil {
									logging.Routing("[helpers] failed to persist path dependency link: %v", err)
								}
							}
						case "symbol_graph":
							if len(f.Args) >= 4 {
								sid := types.ExtractString(f.Args[0])
								file := types.ExtractString(f.Args[3])
								if err := m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]any{"source": "scan-path"}); err != nil {
									logging.Routing("[helpers] failed to persist path symbol link: %v", err)
								}
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

		if walkDirErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
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
			if err := m.kernel.LoadFacts([]core.Fact{ft}); err != nil {
				logging.Kernel("[helpers] failed to load dir file topology fact: %v", err)
			}
			if m.virtualStore != nil {
				if err := m.virtualStore.PersistFactsToKnowledge([]core.Fact{ft}, "fact", 5); err != nil {
					logging.Routing("[helpers] failed to persist dir file topology: %v", err)
				}
			}
			factCount++

			astFacts, parseErr := parser.Parse(path)
			if parseErr == nil && len(astFacts) > 0 {
				if err := m.kernel.LoadFacts(astFacts); err != nil {
					logging.Kernel("[helpers] failed to load dir AST facts: %v", err)
				}
				factCount += len(astFacts)
				if m.virtualStore != nil {
					if err := m.virtualStore.PersistFactsToKnowledge(astFacts, "fact", 6); err != nil {
						logging.Routing("[helpers] failed to persist dir AST facts: %v", err)
					}
					for _, f := range astFacts {
						switch f.Predicate {
						case "dependency_link":
							if len(f.Args) >= 2 {
								a := types.ExtractString(f.Args[0])
								b := types.ExtractString(f.Args[1])
								rel := "depends_on"
								if len(f.Args) >= 3 {
									rel = "depends_on:" + types.ExtractString(f.Args[2])
								}
								if err := m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]any{"source": "scan-dir"}); err != nil {
									logging.Routing("[helpers] failed to persist dir dependency link: %v", err)
								}
							}
						case "symbol_graph":
							if len(f.Args) >= 4 {
								sid := types.ExtractString(f.Args[0])
								file := types.ExtractString(f.Args[3])
								if err := m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]any{"source": "scan-dir"}); err != nil {
									logging.Routing("[helpers] failed to persist dir symbol link: %v", err)
								}
							}
						}
					}
				}
			}
			return nil
		}); walkDirErr != nil {
			logging.Routing("[helpers] directory walk error: %v", walkDirErr)
		}

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
