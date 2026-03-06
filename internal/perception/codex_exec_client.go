package perception

import "codenerd/internal/config"

// CodexExecClient is the explicit codex exec backend implementation used by
// codeNERD's `codex-cli` engine.
//
// The engine name remains `codex-cli` for compatibility, but the subprocess
// transport is specifically `codex exec`.
type CodexExecClient = CodexCLIClient

// NewCodexExecClient creates the explicit codex exec backend client.
func NewCodexExecClient(cfg *config.CodexCLIConfig) *CodexExecClient {
	return NewCodexCLIClient(cfg)
}
