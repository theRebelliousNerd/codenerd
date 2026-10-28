package browser

import "codenerd/internal/config"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

func DefaultProcessQuerier() ProcessQuerier {
	return newProcessQuerier(config.DefaultBrowserReaperConfig())
}
