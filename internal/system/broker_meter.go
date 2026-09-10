package system

import (
	"path/filepath"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/jsonl"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
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
func configureBrokerMeter(appCfg *config.UserConfig, workspace string) {
	sink := openReceiptLog(workspace)

	if appCfg == nil {
		// Still install the receipt log. Metering without a configured window
		// cannot enforce a budget, but it can still record what was spent, and
		// a readout that goes blank because config was missing looks identical
		// to one that goes blank because nothing spent.
		if sink != nil {
			broker.Configure(broker.MeterConfig{ExtraSink: sink})
		}
		return
	}

	ctxCfg := appCfg.GetContextWindowConfig()
	if ctxCfg.MaxTokens <= 0 {
		if sink != nil {
			broker.Configure(broker.MeterConfig{ExtraSink: sink})
		}
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
		ExtraSink:     sink,
	})

	logging.Get(logging.CategoryAPI).Info(
		"broker: metering window=%d output_reserve=%d (output=%d thinking=%d tool_buffer=%d)",
		ctxCfg.MaxTokens, reserve, ctxCfg.OutputReserve, ctxCfg.ThinkingReserve, ctxCfg.ToolUseBuffer)
}

// openReceiptLog installs the workspace receipt log, or returns nil when it
// cannot be opened.
//
// Failure here is logged and tolerated: a workspace on a read-only mount, or
// one whose .nerd directory is not writable, should still be able to run the
// agent. Metering degrades to in-process only, which is exactly what it was
// before the log existed.
func openReceiptLog(workspace string) broker.ReceiptSink {
	if workspace == "" {
		return nil
	}

	path := filepath.Join(workspace, ".nerd", broker.DefaultReceiptLogName)
	sink, err := broker.NewFileSink(path)
	if err != nil {
		logging.Get(logging.CategoryAPI).Warn(
			"broker: receipt log unavailable at %s (%v); metering stays in-process only", path, err)
		return nil
	}

	logging.Get(logging.CategoryAPI).Debug("broker: receipts logging to %s", path)
	return sink
}

// configureCoUseLog installs the prompt-atom selection log.
//
// Same rationale as the receipt log, same failure posture: a workspace that
// cannot be written to still runs the agent, with measurement degrading to
// in-process only.
func configureCoUseLog(workspace string) {
	if workspace == "" {
		return
	}

	path := filepath.Join(workspace, ".nerd", prompt.DefaultSelectionLogName)
	log, err := jsonl.Open(path)
	if err != nil {
		logging.Get(logging.CategoryJIT).Warn(
			"prompt: atom selection log unavailable at %s (%v); co-use analysis stays in-process only", path, err)
		return
	}

	if err := prompt.CoUse().SetLog(log); err != nil {
		// The new log is installed either way; this says the previous one did
		// not flush cleanly, which is a lost tail of measurement rather than a
		// reason to run without one.
		logging.Get(logging.CategoryJIT).Warn(
			"prompt: previous atom selection log did not close cleanly: %v", err)
	}
	logging.Get(logging.CategoryJIT).Debug("prompt: atom selections logging to %s", path)
}
