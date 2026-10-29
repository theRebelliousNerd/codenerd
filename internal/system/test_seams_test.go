package system

import (
	"codenerd/internal/config"
	"context"
	"fmt"
)

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// getOrBootCortex contains the cache transaction independently of the heavy
// production bootstrap. Keeping the boot function injectable lets tests prove
// failure and identity behavior without starting the entire runtime.
func getOrBootCortex(
	ctx context.Context,
	workspace string,
	apiKey string,
	disableSystemShards []string,
	boot cortexBootFunc,
) (*Cortex, error) {
	if boot == nil {
		return nil, fmt.Errorf("cortex boot function is nil")
	}
	return getOrBootCortexWithConfig(ctx, workspace, apiKey, disableSystemShards, func(ctx context.Context, workspace string, apiKey string, disabled []string, _ *config.UserConfig) (*Cortex, error) {
		return boot(ctx, workspace, apiKey, disabled)
	})
}
