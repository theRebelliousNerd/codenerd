package session

import (
	"context"

	"codenerd/internal/prompt"
	"codenerd/internal/sessionlog"
)

// NoteCompile hands the session log the compiler's own manifest for the
// prompt the agent under ctx is about to send, so the exchange file shows
// which atoms were selected (in render order) and which were dropped, and why.
// Nothing is recomputed: the manifest is the compiler's. A no-op unless debug
// mode is on.
func NoteCompile(ctx context.Context, res *prompt.CompilationResult) {
	if res == nil || res.Manifest == nil {
		return
	}
	sessionlog.NoteJIT(ctx, res.Manifest)
}
