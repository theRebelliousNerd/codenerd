package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/executable"
	"codenerd/internal/logging"
)

// discardBuiltExecutables removes the compiled executables an attempt left
// as new untracked files and returns changed without them.
//
// An attempt that runs `go build ./cmd/x` without -o writes x at the
// workspace root. It is the attempt's by-product, not its change: the
// committed-binary audit only reads tracked files, so it passed, and the
// ratchet then committed every new path. One kept cycle committed a 2.98 MB
// binary that way (2026-11-07). Discarding it before the gates re-measure
// means the verdict, the gates and the commit all see the tree without it,
// and an attempt whose only change was a binary is no change. A tracked
// executable is left alone: that is the audit's finding to raise.
func (r *recurseRun) discardBuiltExecutables(ctx context.Context, changed []string, untrackedBefore map[string]bool) ([]string, error) {
	if len(changed) == 0 {
		return changed, nil
	}
	now, err := r.git.untracked(ctx)
	if err != nil {
		return nil, err
	}
	kept := changed[:0]
	var dropped []string
	for _, p := range changed {
		if now[p] && !untrackedBefore[p] {
			abs := filepath.Join(r.root, filepath.FromSlash(p))
			if kind, ok := executable.Kind(abs); ok {
				if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				dropped = append(dropped, p+" ("+kind+")")
				continue
			}
		}
		kept = append(kept, p)
	}
	if len(dropped) > 0 {
		logging.Campaign("recurse: discarded %d executable(s) the attempt built, not committed: %s", len(dropped), strings.Join(dropped, ", "))
	}
	return kept, nil
}
