package system

import (
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
)

// WorkspaceStaticToolDefs loads the workspace's .nerd/available_tools.json
// (written by nerd init) as the static tool definitions boot registers.
// Boot (initExecutionLayer) and `nerd config check` both read it here, so
// the command that reports an unregistered agent tool sees exactly the
// tools boot registers.
func WorkspaceStaticToolDefs(nerdDir string) ([]core.StaticToolDef, error) {
	defs, err := nerdinit.LoadToolsFromFile(nerdDir)
	if err != nil {
		return nil, err
	}
	static := make([]core.StaticToolDef, 0, len(defs))
	for _, d := range defs {
		static = append(static, core.StaticToolDef{
			Name:          d.Name,
			Category:      d.Category,
			Description:   d.Description,
			Command:       d.Command,
			ShardAffinity: d.ShardAffinity,
		})
	}
	return static, nil
}
