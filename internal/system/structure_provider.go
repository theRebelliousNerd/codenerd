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
	codedom.RegisterStructureProvider(world.NewStructureIndex(workspace).Provider())
	logging.Get(logging.CategoryBoot).Debug("structure provider registered for %s", workspace)
}
