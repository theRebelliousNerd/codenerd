package system

import (
	"codenerd/internal/config"
	"codenerd/internal/usage"
)

// UsageOptions applies the config's usage section to this process and returns
// the options the workspace's shared usage tracker is acquired with.
//
// Price overrides go into the usage price table, which is process-wide like the
// built-in table it extends; costs are estimated when a call is metered, so
// this runs before the tracker records anything. usage.event_log turns on the
// tracker's bounded event ring. Every host that acquires the shared tracker
// (the Cortex boot, the chat model) passes these options: the first acquirer's
// win, and both read the same config.
//
// Before this, usage.RegisterPrice ("for operators whose negotiated rates
// differ from list price") and usage.WithEventLog had no caller, so neither
// could be switched on by anyone.
func UsageOptions(cfg *config.UserConfig) []usage.Option {
	uc := cfg.GetUsageConfig()
	for prefix, p := range uc.Prices {
		usage.RegisterPrice(prefix, usage.Price{InputPerMTok: p.InputPerMTok, OutputPerMTok: p.OutputPerMTok})
	}
	var opts []usage.Option
	if uc.EventLog {
		opts = append(opts, usage.WithEventLog())
	}
	return opts
}
