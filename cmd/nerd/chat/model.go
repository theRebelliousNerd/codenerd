// Package chat provides the interactive TUI chat interface for codeNERD.
// The chat functionality is split across multiple files for maintainability:
//   - model_types.go: Type definitions, constants, message types
//   - model_lifecycle.go: Init, Shutdown, lifecycle methods
//   - model_update.go: Main Update loop and message routing
//   - model_handlers.go: Input handlers (submit, clarification, dream learning)
//   - model_session_context.go: Session context building and kernel queries
//   - model_helpers.go: Utility functions (extractFindings, hardWrap, etc.)
//   - commands.go: /command handling
//   - process.go: Natural language input processing
//   - view.go: Rendering functions
//   - session.go: Session management
//   - campaign.go: Campaign orchestration
//   - delegation.go: Shard spawning
//   - shadow.go: Shadow mode
//   - helpers.go: General utility functions
package chat
