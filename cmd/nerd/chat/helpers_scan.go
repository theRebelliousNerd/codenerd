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
		if closeErr := initializer.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close initializer: %w", closeErr)
		}

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
			if err := m.virtualStore.PersistLinkFacts(res.NewFacts, "scan"); err != nil {
				logging.Routing("[helpers] failed to persist knowledge graph links: %v", err)
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
		// Query readback renders a Mangle /name as a plain string, never a
		// core.MangleAtom — so this type assertion was always false and the
		// filter matched nothing. /scan --deep reported zero Go files in a Go
		// repository. types.ExtractString accepts both forms.
		if types.ExtractString(f.Args[2]) == "/go" {
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

	res, err := world.EnsureDeepFactsInRoot(context.Background(), m.workspace, goFiles, m.localDB, deepWorkers)
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
		if err := m.virtualStore.PersistLinkFacts(res.NewFacts, "scan-deep"); err != nil {
			logging.Routing("[helpers] failed to persist deep knowledge graph links: %v", err)
		}
	}

	return nil
}

// runPartialScan scans specific file paths (non-recursive) and persists facts.
func (m Model) runPartialScan(paths []string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		m.ReportStatus(fmt.Sprintf("Scanning %d paths...", len(paths)))
		files := make([]string, 0, len(paths))
		for _, raw := range paths {
			p := strings.TrimSpace(raw)
			if p == "" {
				continue
			}
			files = append(files, world.ResolveWorkspacePath(m.workspace, p))
		}
		total := m.scanFilesIntoWorld(files, "scan-path")
		m.ReportStatus("Scan complete")
		return scanCompleteMsg{
			fileCount:      len(files),
			directoryCount: 0,
			factCount:      total,
			duration:       time.Since(start),
		}
	}
}

// runDirScan scans a directory recursively and persists facts (lighter than full init).
func (m Model) runDirScan(dir string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		dir = world.ResolveWorkspacePath(m.workspace, strings.TrimSpace(dir))
		m.ReportStatus(fmt.Sprintf("Scanning directory: %s", dir))
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return scanCompleteMsg{err: fmt.Errorf("invalid directory: %s", dir)}
		}

		var files []string
		dirCount := 0
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
			files = append(files, path)
			if len(files)%10 == 0 {
				m.ReportStatus(fmt.Sprintf("Scanning... (%d files)", len(files)))
			}
			return nil
		}); walkDirErr != nil {
			logging.Routing("[helpers] directory walk error: %v", walkDirErr)
		}

		total := m.scanFilesIntoWorld(files, "scan-dir")
		m.ReportStatus("Scan complete")
		return scanCompleteMsg{
			fileCount:      len(files),
			directoryCount: dirCount,
			factCount:      total,
			duration:       time.Since(start),
		}
	}
}

// scanFilesIntoWorld parses the files at fsPaths (absolute, openable) and
// installs their facts in the kernel, the world cache and the knowledge graph
// under each file's CANONICAL identity (workspace-relative, forward-slash; see
// world.CanonicalPath). It is the shared body of /scan-path and /scan-dir and
// returns the number of facts loaded.
//
// Before this, both commands keyed every fact by the absolute path they had
// opened. The file_topology, symbol_graph and dependency_link rows they
// produced therefore named a file the full and incremental scanners had
// already recorded under its relative path, so nothing joined (impact,
// activation, test_file_for), nothing was ever retracted, and a moved
// checkout matched none of it. They also left import facts as raw "pkg:..."
// tokens where every other scanner resolves them into file->file edges, and
// wrote nothing to the world cache, so the next incremental scan could not
// retract what they had loaded either.
func (m Model) scanFilesIntoWorld(fsPaths []string, source string) int {
	if m.kernel == nil || len(fsPaths) == 0 {
		return 0
	}
	parser := world.NewASTParser()
	defer parser.Close()

	facts := make([]core.Fact, 0, len(fsPaths)*4)
	scanned := make([]string, 0, len(fsPaths))
	var retract []core.Fact
	for _, fsPath := range fsPaths {
		info, err := os.Stat(fsPath)
		if err != nil || info.IsDir() {
			continue
		}
		canonical := world.CanonicalPath(m.workspace, fsPath)
		scanned = append(scanned, canonical)
		if m.localDB != nil {
			// The cached rows are what a later incremental scan retracts by;
			// retracting them here keeps the kernel at one generation per file.
			if old, _, loadErr := m.localDB.LoadWorldFactsForFile(canonical, "fast"); loadErr == nil {
				for _, in := range old {
					retract = append(retract, core.Fact{Predicate: in.Predicate, Args: in.Args})
				}
			}
		}
		facts = append(facts, buildFileTopologyFact(fsPath, canonical, info))
		astFacts, parseErr := parser.ParseAs(fsPath, canonical)
		if parseErr != nil {
			logging.Routing("[helpers] %s: AST parse of %s failed: %v", source, canonical, parseErr)
			continue
		}
		facts = append(facts, astFacts...)
	}
	if len(facts) == 0 {
		return 0
	}

	// Import edges resolve against the whole workspace, not just the files in
	// hand: a scanned file's import of an untouched package still has to land.
	facts = append(facts, world.ResolveDependencyLinksAgainst(m.workspace, m.knownWorkspaceFiles(scanned), facts)...)

	if len(retract) > 0 {
		if err := m.kernel.RetractExactFactsBatch(retract); err != nil {
			logging.Kernel("[helpers] %s: failed to retract superseded facts: %v", source, err)
		}
	}
	if err := m.kernel.LoadFacts(facts); err != nil {
		logging.Kernel("[helpers] %s: failed to load facts: %v", source, err)
	}
	if m.localDB != nil {
		if err := world.PersistFastSnapshotToDBInRoot(m.localDB, m.workspace, facts); err != nil {
			logging.Routing("[helpers] %s: failed to cache world facts: %v", source, err)
		}
	}
	if m.virtualStore != nil {
		if err := m.virtualStore.PersistFactsToKnowledge(facts, "fact", 5); err != nil {
			logging.Routing("[helpers] %s: failed to persist facts to knowledge: %v", source, err)
		}
		if err := m.virtualStore.PersistLinkFacts(facts, source); err != nil {
			logging.Routing("[helpers] %s: failed to persist knowledge graph links: %v", source, err)
		}
	}
	return len(facts)
}

// knownWorkspaceFiles returns the canonical paths of every file the kernel
// currently knows, plus extra, for import resolution over a partial scan.
func (m Model) knownWorkspaceFiles(extra []string) []string {
	seen := make(map[string]struct{}, len(extra))
	files := make([]string, 0, len(extra))
	add := func(p string) {
		if p == "" {
			return
		}
		if _, dup := seen[p]; dup {
			return
		}
		seen[p] = struct{}{}
		files = append(files, p)
	}
	for _, p := range extra {
		add(p)
	}
	if m.kernel != nil {
		if rows, err := m.kernel.Query("file_topology"); err == nil {
			for _, f := range rows {
				if len(f.Args) > 0 {
					add(types.ExtractString(f.Args[0]))
				}
			}
		}
	}
	return files
}
