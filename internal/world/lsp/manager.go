package lsp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

// ============================================================================
// LSP Manager - Multi-Language Code Intelligence for World Model
// ============================================================================
// This manager wraps language servers and projects their data into World Model facts.
// Architecture: LSP data → World Model EDB → Spreading Activation/Shards

// Manager coordinates LSP servers and fact projection.
type Manager struct {
	mu            sync.RWMutex
	mangleServer  *mangle.LSPServer
	mangleEngine  *mangle.Engine
	workspaceRoot string
	indexed       bool

	// External language servers (gopls, rust-analyzer, ...) keyed by language
	// atom. The Mangle server stays a special case because it runs in-process.
	servers map[string]*Client
}

// NewManager creates a new LSP manager.
func NewManager(workspaceRoot string) *Manager {
	logging.WorldDebug("Creating LSP Manager for workspace: %s", workspaceRoot)
	return &Manager{
		workspaceRoot: workspaceRoot,
		indexed:       false,
		servers:       make(map[string]*Client),
	}
}

// AddLanguageServer registers an already-connected language server under a
// language atom ("/go", "/rust"). Its diagnostics join the Mangle server's in
// ProjectToFacts, so policy sees one code_diagnostic relation regardless of
// which server produced a row.
func (m *Manager) AddLanguageServer(lang string, client *Client) {
	if client == nil {
		return
	}
	if !strings.HasPrefix(lang, "/") {
		lang = "/" + lang
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.servers == nil {
		m.servers = make(map[string]*Client)
	}
	if prev, ok := m.servers[lang]; ok && prev != client {
		// Replacing a live server without closing it leaks the subprocess and
		// leaves a reader goroutine parked on its stdout forever.
		_ = prev.Close()
	}
	m.servers[lang] = client
	logging.World("LSP Manager: registered language server for %s", lang)
}

// StartLanguageServer launches a language server binary and registers it.
// A missing binary is not an error state for the session: code intelligence
// degrades to the AST/tree-sitter layer, so the caller gets a plain error it
// can log and continue past.
func (m *Manager) StartLanguageServer(ctx context.Context, lang, binary string, args ...string) error {
	client, err := StartServer(ctx, lang, binary, args...)
	if err != nil {
		return err
	}
	if err := client.Initialize(ctx, m.workspaceRoot); err != nil {
		_ = client.Close()
		return fmt.Errorf("initialize %s: %w", binary, err)
	}
	m.AddLanguageServer(lang, client)
	return nil
}

// LanguageServer returns a registered server, if any.
func (m *Manager) LanguageServer(lang string) (*Client, bool) {
	if !strings.HasPrefix(lang, "/") {
		lang = "/" + lang
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.servers[lang]
	return c, ok
}

// CloseLanguageServers shuts every registered external server down.
func (m *Manager) CloseLanguageServers(ctx context.Context) {
	m.mu.Lock()
	servers := m.servers
	m.servers = make(map[string]*Client)
	m.mu.Unlock()
	for lang, c := range servers {
		if err := c.Shutdown(ctx); err != nil {
			logging.WorldWarn("LSP Manager: shutting down %s server: %v", lang, err)
		}
	}
}

// Initialize initializes the Mangle LSP server and indexes the workspace.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logging.World("Initializing LSP Manager")

	// Create Mangle engine for LSP server
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to create Mangle engine for LSP: %v", err)
		return fmt.Errorf("failed to create Mangle engine: %w", err)
	}
	m.mangleEngine = engine

	// Create Mangle LSP server
	m.mangleServer = mangle.NewLSPServer(engine)

	// Index workspace
	if err := m.mangleServer.IndexWorkspace(ctx, m.workspaceRoot); err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to index workspace for LSP: %v", err)
		return fmt.Errorf("failed to index workspace: %w", err)
	}

	m.indexed = true
	logging.World("LSP Manager initialized successfully")
	return nil
}

// ============================================================================
// Fact Projection - Convert LSP Data to World Model Facts
// ============================================================================

// ProjectToFacts converts all LSP data (definitions, references, diagnostics)
// into Mangle facts for the World Model.
func (m *Manager) ProjectToFacts() ([]core.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.indexed {
		return nil, fmt.Errorf("LSP manager not initialized")
	}

	var facts []core.Fact

	// Project symbol definitions
	definitionFacts := m.projectDefinitions()
	facts = append(facts, definitionFacts...)

	// Project symbol references
	referenceFacts := m.projectReferences()
	facts = append(facts, referenceFacts...)

	// Project diagnostics
	diagnosticFacts := m.projectDiagnostics()
	facts = append(facts, diagnosticFacts...)

	// External servers contribute to the same relations.
	for _, c := range m.servers {
		facts = append(facts, c.DiagnosticFacts(m.workspaceRoot)...)
	}

	logging.WorldDebug("LSP projected %d facts (%d definitions, %d references, %d diagnostics)",
		len(facts),
		len(definitionFacts),
		len(referenceFacts),
		len(diagnosticFacts))

	return facts, nil
}

// projectDefinitions converts symbol definitions to facts.
func (m *Manager) projectDefinitions() []core.Fact {
	var facts []core.Fact

	if m.mangleServer == nil {
		return facts
	}

	// Get all definitions from Mangle LSP server
	allDefs := m.mangleServer.GetAllDefinitions()

	for symbol, defs := range allDefs {
		for _, def := range defs {
			facts = append(facts, core.Fact{
				Predicate: "symbol_defined",
				Args: []any{
					"/mangle",
					symbol,
					def.FilePath,
					def.Line,
					def.Column,
				},
			})
		}
	}

	return facts
}

// projectReferences converts symbol references to facts.
func (m *Manager) projectReferences() []core.Fact {
	var facts []core.Fact

	if m.mangleServer == nil {
		return facts
	}

	// Get all references from Mangle LSP server
	allRefs := m.mangleServer.GetAllReferences()

	for symbol, refs := range allRefs {
		for _, ref := range refs {
			kind := referenceKindToAtom(ref.Kind)
			facts = append(facts, core.Fact{
				Predicate: "symbol_referenced",
				Args: []any{
					"/mangle",
					symbol,
					ref.FilePath,
					ref.Line,
					ref.Column,
					kind,
				},
			})
		}
	}

	return facts
}

// projectDiagnostics converts diagnostics to facts.
func (m *Manager) projectDiagnostics() []core.Fact {
	var facts []core.Fact

	if m.mangleServer == nil {
		return facts
	}

	// Get all diagnostics from Mangle LSP server
	allDiags := m.mangleServer.GetAllDiagnostics()

	for _, diags := range allDiags {
		for _, diag := range diags {
			severity := diagnosticSeverityToAtom(diag.Severity)
			facts = append(facts, core.Fact{
				Predicate: "code_diagnostic",
				Args: []any{
					diag.FilePath,
					diag.Line,
					severity,
					diag.Message,
				},
			})
		}
	}

	return facts
}

// ============================================================================
// Batch Query API for Shards
// ============================================================================

// GetDefinitions returns all definitions for a symbol.
// Used by shards for batch queries (non-interactive).
func (m *Manager) GetDefinitions(symbol string) ([]core.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mangleServer == nil {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	// Query LSP server
	defs := m.mangleServer.GetDefinitions(symbol)

	// Convert to facts
	var facts []core.Fact
	for _, def := range defs {
		facts = append(facts, core.Fact{
			Predicate: "symbol_defined",
			Args: []any{
				"/mangle",
				def.Symbol,
				def.FilePath,
				def.Line,
				def.Column,
			},
		})
	}

	return facts, nil
}

// GetReferences returns all references to a symbol.
func (m *Manager) GetReferences(symbol string) ([]core.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mangleServer == nil {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	refs := m.mangleServer.GetReferences(symbol)

	var facts []core.Fact
	for _, ref := range refs {
		kind := referenceKindToAtom(ref.Kind)
		facts = append(facts, core.Fact{
			Predicate: "symbol_referenced",
			Args: []any{
				"/mangle",
				ref.Symbol,
				ref.FilePath,
				ref.Line,
				ref.Column,
				kind,
			},
		})
	}

	return facts, nil
}

// ValidateCode validates Mangle code and returns diagnostic facts.
// Used by CoderShard and LegislatorShard to validate generated code.
func (m *Manager) ValidateCode(filePath, content string) ([]core.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mangleServer == nil {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	uri := pathToURI(filePath)
	diags := m.mangleServer.ValidateCode(uri, content)

	var facts []core.Fact
	for _, diag := range diags {
		severity := diagnosticSeverityToAtom(diag.Severity)
		facts = append(facts, core.Fact{
			Predicate: "code_diagnostic",
			Args: []any{
				diag.FilePath,
				diag.Line,
				severity,
				diag.Message,
			},
		})
	}

	return facts, nil
}

// ============================================================================
// Stdio Server for External Editors
// ============================================================================

// ServeStdio starts the LSP server on stdin/stdout for editor integration.
// This is the entry point for `nerd mangle-lsp` CLI command.
func (m *Manager) ServeStdio(ctx context.Context) error {
	m.mu.RLock()
	server := m.mangleServer
	m.mu.RUnlock()

	if server == nil {
		return fmt.Errorf("LSP server not initialized - call Initialize() first")
	}

	logging.World("Starting LSP stdio server for external editors")
	return server.ServeStdio(ctx)
}

// ============================================================================
// Utility Functions
// ============================================================================

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	// This should match the implementation in mangle.LSPServer
	// For now, simple implementation
	return "file://" + path
}

// referenceKindToAtom converts ReferenceKind to Mangle atom.
func referenceKindToAtom(kind mangle.ReferenceKind) string {
	switch kind {
	case mangle.RefInHead:
		return "/head"
	case mangle.RefInBody:
		return "/body"
	case mangle.RefInFact:
		return "/fact"
	case mangle.RefInQuery:
		return "/query"
	default:
		return "/unknown"
	}
}

// diagnosticSeverityToAtom converts DiagnosticSeverity to Mangle atom.
func diagnosticSeverityToAtom(severity mangle.DiagnosticSeverity) string {
	switch severity {
	case mangle.DiagError:
		return "/error"
	case mangle.DiagWarning:
		return "/warning"
	case mangle.DiagInformation:
		return "/info"
	case mangle.DiagHint:
		return "/hint"
	default:
		return "/unknown"
	}
}
