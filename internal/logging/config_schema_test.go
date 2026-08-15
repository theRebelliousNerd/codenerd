package logging

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The package parsed `json_format` while config.LoggingConfig — the struct the
// rest of the app loads from the same .nerd/config.json — writes `format`.
// A config produced by the app could therefore never enable structured logging.

func TestLoadConfig_WhenFormatIsJSON_ShouldEnableStructuredOutput(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "format": "json"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !IsJSONFormat() {
		t.Fatal(`format: "json" did not enable structured output`)
	}

	Get(CategoryKernel).Info("structured line")
	CloseAll()

	line := firstJSONLine(t, readLog(t, ws, "kernel"))
	if line["msg"] != "structured line" {
		t.Errorf("expected the message in the JSON entry, got %v", line)
	}
}

func TestLoadConfig_WhenLegacyJSONFormatFlag_ShouldStillEnableStructuredOutput(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "json_format": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !IsJSONFormat() {
		t.Fatal("legacy json_format alias stopped working")
	}
}

func TestLoadConfig_WhenFormatIsText_ShouldStayTextual(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "format": "TEXT"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if IsJSONFormat() {
		t.Fatal(`format: "text" must not produce JSON`)
	}
	Get(CategoryKernel).Info("plain line")
	CloseAll()
	if !strings.Contains(readLog(t, ws, "kernel"), "[INFO] plain line") {
		t.Error("expected the text format to be unchanged")
	}
}

// TestLoggingConfigSchema_WhenComparedToAppConfig_ShouldParseEveryKey pins the
// two schemas together. config.LoggingConfig cannot be imported here (it
// imports this package), so its JSON keys are listed literally; if that struct
// gains or renames a key without this package following, this fails.
func TestLoggingConfigSchema_WhenComparedToAppConfig_ShouldParseEveryKey(t *testing.T) {
	// Keys as spelled by config.LoggingConfig's json tags.
	appConfigJSON := `{
		"level": "warn",
		"format": "json",
		"file": "codenerd.log",
		"debug_mode": true,
		"trace_llm_io": true,
		"categories": {"kernel": true},
		"performance_sampling": 0.25,
		"performance_thresholds_ms": {"default": 100}
	}`

	var parsed loggingConfig
	if err := json.Unmarshal([]byte(appConfigJSON), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Level != "warn" {
		t.Errorf("level not parsed: %q", parsed.Level)
	}
	if parsed.Format != "json" {
		t.Errorf("format not parsed: %q", parsed.Format)
	}
	if !parsed.DebugMode || !parsed.TraceLLMIO {
		t.Error("debug_mode / trace_llm_io not parsed")
	}
	if !reflect.DeepEqual(parsed.Categories, map[string]bool{"kernel": true}) {
		t.Errorf("categories not parsed: %v", parsed.Categories)
	}
	if parsed.PerformanceSampling != 0.25 {
		t.Errorf("performance_sampling not parsed: %v", parsed.PerformanceSampling)
	}
	if parsed.PerformanceThresholdsMs["default"] != 100 {
		t.Errorf("performance_thresholds_ms not parsed: %v", parsed.PerformanceThresholdsMs)
	}

	// `file` (legacy single-file logging) is deliberately unsupported here: this
	// package writes one file per category and has no single-file mode. It is
	// listed above so an accidental future dependency on it is visible.
}

func firstJSONLine(t *testing.T, content string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		idx := strings.Index(line, "{")
		if idx < 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line[idx:]), &entry); err == nil {
			return entry
		}
	}
	t.Fatalf("no JSON entry found in:\n%s", content)
	return nil
}
