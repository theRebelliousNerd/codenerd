package system

import (
	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// configureBrokerMeter points the process meter at the workspace's configured
// context window.
//
// This is the moment the single budget authority replaces the several that used
// to exist. Before the broker, ContextWindow.MaxTokens was read independently by
// the context compressor and the JIT prompt compiler, neither of which
// subtracted the other, alongside four hard-coded literals elsewhere that read
// nothing at all. It is now read once, here, and everything that spends tokens
// is charged against the result.
func configureBrokerMeter(appCfg *config.UserConfig) {
	if appCfg == nil {
		return
	}

	ctxCfg := appCfg.GetContextWindowConfig()
	if ctxCfg.MaxTokens <= 0 {
		return
	}

	reserve := ctxCfg.OutputReserve
	// Thinking tokens are billed as output and occupy the same reserve. A model
	// configured for extended thinking with no allowance for it produces a turn
	// that is admitted and then truncated, which reads as a model failure rather
	// than a budgeting one.
	if ctxCfg.ThinkingReserve > 0 {
		reserve += ctxCfg.ThinkingReserve
	}
	// Multi-turn tool cycles append results into the same window after
	// admission. Holding the configured buffer back keeps a long tool loop from
	// walking off the end of a request that was legitimately admitted.
	if ctxCfg.ToolUseBuffer > 0 {
		reserve += ctxCfg.ToolUseBuffer
	}

	broker.Configure(broker.MeterConfig{
		Window:        ctxCfg.MaxTokens,
		OutputReserve: reserve,
	})

	logging.Get(logging.CategoryAPI).Info(
		"broker: metering window=%d output_reserve=%d (output=%d thinking=%d tool_buffer=%d)",
		ctxCfg.MaxTokens, reserve, ctxCfg.OutputReserve, ctxCfg.ThinkingReserve, ctxCfg.ToolUseBuffer)
}
