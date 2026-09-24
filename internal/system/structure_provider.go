package system

import (
	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// wireStructureProvider registers the structure index for this Cortex. The
// index is empty until the first structural query, so boot pays nothing.
func wireStructureProvider(workspace string) {
	if workspace == "" {
		codedom.RegisterStructureProvider(nil)
		logging.Get(logging.CategoryBoot).Debug("structure provider not registered: no workspace")
		return
	}
	// The workspace's one index: the campaign's brief preload reads the same
	// one (world.SharedStructureIndex), where a second instance cost ~73 MB
	// and parsed every changed file twice.
	codedom.RegisterStructureProvider(world.SharedStructureIndex(workspace).Provider())
	logging.Get(logging.CategoryBoot).Debug("structure provider registered for %s", workspace)
}
