package config

import "testing"

// ToLoggingConfig is the only bridge from the parsed file into the logger's
// injected config. `format` must cross it unchanged: it is the one key that
// enables structured output, and there is no second spelling left to carry
// the intent if this mapping drops it.
func TestToLoggingConfig_WhenFormatSet_ShouldCarryItUnchanged(t *testing.T) {
	for _, format := range []string{"json", "text", "JSON", ""} {
		if got := (&LoggingConfig{DebugMode: true, Level: "debug", Format: format}).ToLoggingConfig(); got.Format != format {
			t.Errorf("format %q became %q across ToLoggingConfig", format, got.Format)
		}
	}
}
