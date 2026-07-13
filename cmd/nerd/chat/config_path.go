package chat

import (
	"path/filepath"
	"strings"

	"codenerd/internal/config"
)

// userConfigPath binds configuration writes to the Model's active workspace.
// Falling back is reserved for pre-workspace initialization; live chat and its
// tests must never mutate whichever repository happens to be the process cwd.
func (m Model) userConfigPath() string {
	if workspace := strings.TrimSpace(m.workspace); workspace != "" {
		return filepath.Join(workspace, ".nerd", "config.json")
	}
	return config.DefaultUserConfigPath()
}
