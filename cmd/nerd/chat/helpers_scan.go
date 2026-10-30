package chat

import (
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
		ctx, cancel := m.sessionOperationContext()
		defer cancel()

		// Detect project type for profile
		projectInfo := detectProjectType(m.workspace)

		// Get Context7 API key from config or environment
		context7Key := m.Config.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}

		// Ensure .nerd directory exists
		if err := createDirIfNotExists(m.workspace + "/.nerd"); err != nil {
			return errorMsg(fmt.Errorf("failed to create .nerd directory: %w", err))
		}

		// Progress goes to the status bar. The forwarder exits when the
		// channel closes, which the deferred close guarantees on every
		// return path, after Initialize (the only sender) has returned.
		progressCh := make(chan nerdinit.InitProgress, 32)
		forwarded := make(chan struct{})
		go func() {
			defer close(forwarded)
			for p := range progressCh {
				m.ReportStatus(initProgressStatus(p))
			}
		}()
		defer func() {
			close(progressCh)
			<-forwarded
		}()

		// The session's own clients, embedding engine and shard manager: the
		// planner slot Cortex booted, never a second client. Timeout stays
		// zero so each request reads llm_timeouts.per_call_timeout. Output
		// stays nil: init must not write into the TUI's screen; its progress
		// arrives on progressCh.
		initConfig := nerdinit.InitConfig{
			Workspace:       m.workspace,
			LLMClient:       m.client,
			PlannerClient:   m.cortex.PlannerLLM(),
			ShardManager:    m.shardMgr,
			EmbeddingEngine: m.embeddingEngine,
			Interactive:     false, // Non-interactive in chat mode
			SkipResearch:    false, // Do full research
			SkipAgentCreate: false, // Create Type 3 agents
			Context7APIKey:  context7Key,
			ProgressChan:    progressCh,
		}

		initializer, err := nerdinit.NewInitializer(initConfig)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to create initializer: %w", err))
		}

		// Run the comprehensive initialization
		result, err := initializer.Initialize(ctx)
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
				_, scanErr := m.scanner.Refresh(ctx, m.workspace, m.localDB, m.kernel, world.IncrementalOptions{SkipWhenUnchanged: false})
				if scanErr != nil {
					logging.Routing("[helpers] incremental workspace scan failed: %v", scanErr)
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

// initProgressStatus is one init progress update as a status line: the phase
// position and elapsed time are measured, so they are shown; init's remaining
// time is a guess from fixed per-phase estimates, so it is not.
func initProgressStatus(p nerdinit.InitProgress) string {
	if p.CurrentPhaseNo <= 0 || p.TotalPhases <= 0 {
		return p.Message
	}
	return fmt.Sprintf("[init %d/%d, %s] %s", p.CurrentPhaseNo, p.TotalPhases, p.ElapsedTime.Round(time.Second), p.Message)
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
		res, err := m.scanner.Refresh(context.Background(), m.workspace, m.localDB, m.kernel, world.IncrementalOptions{SkipWhenUnchanged: true})
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

// partialDocsNote says what a partial extraction means for the user.
func partialDocsNote(partial int) string {
	if partial == 0 {
		return ""
	}
	return fmt.Sprintf(" %d document(s) had a chunk that failed to extract; their other atoms are stored and the next /refresh-docs extracts them again.", partial)
}

// docRefreshCompleteMsg signals completion of document refresh.
type docRefreshCompleteMsg struct {
	docsDiscovered int
	docsProcessed  int
	atomsStored    int
	// docsPartial were extracted only in part (a chunk failed); the next
	// refresh extracts them again.
	docsPartial int
	duration    time.Duration
	err         error
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

		ctx, cancel := m.sessionOperationContext()
		defer cancel()

		// Create initializer for doc processing (reuses init infrastructure):
		// extraction runs on the session's main client and synthesis on its
		// planner slot, as in init.
		initConfig := nerdinit.InitConfig{
			Workspace:       m.workspace,
			LLMClient:       m.client,
			PlannerClient:   m.cortex.PlannerLLM(),
			EmbeddingEngine: m.embeddingEngine,
			Interactive:     false,
		}

		initializer, err := nerdinit.NewInitializer(initConfig)
		if err != nil {
			return docRefreshCompleteMsg{err: fmt.Errorf("failed to create initializer: %w", err)}
		}
		defer func() {
			if closeErr := initializer.Close(); closeErr != nil {
				logging.Routing("[helpers] close document refresh initializer: %v", closeErr)
			}
		}()

		// The documents are the orientation read set, refreshed against HEAD
		// first; the changed projection is published to the running kernel.
		allDocs, refresh, err := initializer.OrientationDocuments(ctx)
		if err != nil && len(allDocs) == 0 {
			return docRefreshCompleteMsg{err: err}
		}
		if err != nil {
			m.ReportStatus(fmt.Sprintf("Orientation warning: %v", err))
		}
		if tx, ok := m.kernel.(types.KernelTransactor); ok {
			if applyErr := refresh.Apply(tx); applyErr != nil {
				return docRefreshCompleteMsg{err: fmt.Errorf("publish orientation refresh: %w", applyErr)}
			}
		} else if refresh != nil && refresh.Refreshed {
			m.ReportStatus("Orientation refreshed on disk; this kernel takes no transactions, so it sees the change at next boot")
		}
		if len(allDocs) == 0 {
			return docRefreshCompleteMsg{
				docsDiscovered: 0,
				duration:       time.Since(startTime),
			}
		}

		m.ReportStatus(fmt.Sprintf("Found %d docs, processing with Mangle tracking...", len(allDocs)))

		// Process with tracking (handles resumption, change detection, incremental storage)
		state, err := initializer.ProcessDocumentsWithTracking(ctx, allDocs, m.localDB, m.kernel, force)
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
			atomsStored:    state.AtomsStored,
			docsPartial:    state.TotalPartial,
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
	return m.scanner.WithRefreshLock(context.Background(), m.ensureDeepWorldFactsLocked)
}

func (m *Model) ensureDeepWorldFactsLocked() error {

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
		if err := m.virtualStore.ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan-deep"); err != nil {
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
			files = append(files, types.ResolveWorkspacePath(m.workspace, p))
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
		dir = types.ResolveWorkspacePath(m.workspace, strings.TrimSpace(dir))
		m.ReportStatus(fmt.Sprintf("Scanning directory: %s", dir))
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return scanCompleteMsg{err: fmt.Errorf("invalid directory: %s", dir)}
		}

		memRoot := m.workspace
		if strings.TrimSpace(memRoot) == "" {
			memRoot = dir
		}
		mem, memErr := openMembership(memRoot)
		if memErr != nil {
			return scanCompleteMsg{err: memErr}
		}

		var files []string
		dirCount := 0
		if walkDirErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			member, admErr := mem.Admit(path, d.IsDir())
			if admErr != nil {
				return admErr
			}
			if !member {
				return nil
			}
			if d.IsDir() {
				dirCount++
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
	if m.scanner == nil {
		logging.Routing("[helpers] %s: scanner not initialized", source)
		return 0
	}
	var total int
	if err := m.scanner.WithRefreshLock(context.Background(), func() error {
		total = m.scanFilesIntoWorldLocked(fsPaths, source)
		return nil
	}); err != nil {
		logging.Routing("[helpers] %s: world update failed: %v", source, err)
	}
	return total
}

func (m Model) scanFilesIntoWorldLocked(fsPaths []string, source string) int {
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
		canonical := types.CanonicalPath(m.workspace, fsPath)
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
		if err := m.virtualStore.ReplaceLinkFacts(retract, facts, source); err != nil {
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
