package chat

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/world"
	"context"
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
