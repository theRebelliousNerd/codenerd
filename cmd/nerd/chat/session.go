// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains session management, initialization, and state persistence.
package chat

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/config"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"

	"codenerd/internal/transparency"
	"codenerd/internal/usage"
	"codenerd/internal/ux"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================
// Functions for initializing the chat, loading/saving session state, and
// managing persistent configuration.

// InitChat initializes the interactive chat model (Lightweight UI only)
func InitChat(cfg Config) Model {
	// Load configuration from unified .nerd/config.json
	appCfg, _ := config.GlobalConfig()
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}

	// Initialize styles
	styles := ui.DefaultStyles()
	if appCfg.Theme == "dark" {
		styles = ui.NewStyles(ui.DarkTheme())
	}

	// Initialize textarea for input
	ta := textarea.New()
	ta.Placeholder = "System initializing..."
	ta.Prompt = "┃ "
	ta.CharLimit = 100_000 // Hard cap: prevents OOM in token counter and regex engine on massive paste
	ta.SetWidth(80)
	ta.SetHeight(3) // 3 lines default
	ta.ShowLineNumbers = false

	// Initialize spinner — MiniDot reads as a continuous pulse rather than
	// a single blinking cursor, which sells the "still working" feel.
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = styles.Spinner

	// Initialize viewport for chat history
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Initialize viewport for error panel (small + scrollable)
	errVP := viewport.New(76, errorPanelViewportHeight)
	errVP.SetContent("")

	// Initialize list (empty by default)
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Past Sessions"
	l.SetShowHelp(false)

	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // All files
	fp.CurrentDirectory, _ = os.Getwd()

	// Initialize markdown renderer
	var renderer *glamour.TermRenderer
	if styles.Theme.IsDark {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(80),
		)
	} else {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("light"),
			glamour.WithWordWrap(80),
		)
	}

	// Resolve workspace. Anchor on the canonical project root (topmost
	// ancestor with .nerd/ or go.mod) rather than raw os.Getwd(): if the
	// user (or a test) invokes from inside a subpackage, raw cwd would
	// cause every .nerd/* path (config, knowledge.db, session.json,
	// sessions/) to materialize in that subdirectory instead of the
	// project root — producing exactly the stray cmd/nerd/chat/.nerd
	// pollution we hit before. FindWorkspaceRoot enforces config-is-boss:
	// one workspace per project, regardless of cwd.
	workspace, err := config.FindWorkspaceRoot()
	if err != nil || workspace == "" {
		workspace, _ = os.Getwd()
	}

	// Note: API key parsing is handled by perception.NewClientFromEnv() during boot
	// The perception package supports multiple providers (zai, anthropic, openai, gemini, xai, openrouter)
	// and reads configuration from .nerd/config.json or environment variables

	// Initialize Usage Tracker (lightweight).
	//
	// Shared, not NewTracker: chat and the Cortex it boots meter the same
	// workspace, and two trackers over one usage.json each hold their own
	// aggregates and overwrite the file on flush — whichever saved last erased
	// the other's tokens.
	tracker, err := usage.Shared(workspace)
	if err != nil {
		fmt.Printf("⚠ Usage tracking init failed: %v\n", err)
	}

	// Initialize split-pane view
	splitPaneView := ui.NewSplitPaneView(styles, 80, 24)

	// Create shutdown context for coordinating background goroutine lifecycle
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	// Initialize Preferences Manager
	prefsMgr := ux.NewPreferencesManager(workspace)
	if err := prefsMgr.Load(); err != nil {
		fmt.Printf("⚠ Failed to load preferences: %v\n", err)
	}

	// Initialize Transparency Manager
	transparencyCfg := appCfg.GetTransparencyConfig()
	transparencyMgr := transparency.NewTransparencyManager(transparencyCfg)

	// Return the model in "Booting" state
	return Model{
		textarea:     ta,
		viewport:     vp,
		errorVP:      errVP,
		spinner:      sp,
		list:         l,
		filepicker:   fp,
		styles:       styles,
		renderer:     renderer,
		usageTracker: tracker,
		usagePage:    ui.NewUsagePageModel(tracker, styles),
		jitPage:      ui.NewJITPageModel(),
		autoPage:     ui.NewAutopoiesisPageModel(),
		shardPage:    ui.NewShardPageModel(),
		splitPane:    &splitPaneView,
		logicPane:    splitPaneView.RightPane,
		showLogic:    false,
		paneMode:     ui.ModeSinglePane,
		showError:    true,
		focusError:   false,
		// System action summaries are noisy; default to showing them only in debug mode.
		showSystemActions: appCfg != nil && appCfg.Logging != nil && appCfg.Logging.DebugMode,
		history:           []Message{},
		Config:            appCfg,
		// Rendering cache for performance
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0, // All messages need rendering initially
		// Backend components start nil
		kernel:              nil,
		shardMgr:            nil,
		client:              nil,              // Will be set in boot
		isBooting:           true,             // Flag for UI
		bootStage:           BootStageBooting, // Startup phase
		statusChan:          make(chan string, 10),
		workspace:           workspace,
		DisableSystemShards: cfg.DisableSystemShards,
		// Mouse capture enabled by default (Alt+M to toggle for text selection)
		mouseEnabled: true,
		// Shutdown coordination (pointer to sync.Once to allow Model copy without noCopy violation)
		shutdownOnce:   &sync.Once{},
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		goroutineWg:    &sync.WaitGroup{},
		// UX components
		preferencesMgr:  prefsMgr,
		transparencyMgr: transparencyMgr,
	}
}
