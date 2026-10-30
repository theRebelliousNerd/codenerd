package config

// DevConfig is for working on codeNERD from its own source tree.
type DevConfig struct {
	// PolicyHotReload, when the workspace is codeNERD's source tree (its
	// go.mod names module codenerd and internal/core/defaults holds the
	// corpus), boots the kernels on the .mg corpus in that tree instead of
	// the one compiled into the binary, and swaps an edited file into the
	// running kernels once the whole program compiles. Off by default; in
	// any other workspace it has no effect and boot says so.
	PolicyHotReload bool `json:"policy_hot_reload"`
}

// GetDev returns the dev section, never nil.
func (c *UserConfig) GetDev() DevConfig {
	if c == nil || c.Dev == nil {
		return DevConfig{}
	}
	return *c.Dev
}
