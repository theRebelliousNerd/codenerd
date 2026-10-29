package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/types"
)

func seedToolLedger(t *testing.T, dir string) {
	t.Helper()
	ledger := autopoiesis.NewProfileStore(autopoiesis.ProfilesDir(filepath.Join(dir, ".nerd", "tools")))
	// grep: 30 offers across 12 sessions, 25 measured calls that each scanned
	// 200 KB to return 400 bytes.
	for s := 0; s < 12; s++ {
		ledger.NoteToolOffers([]types.ToolOffer{
			{Tool: "grep", Origin: types.ToolOriginBuiltin, SessionID: string(rune('a' + s)), CatalogTokens: 100},
			{Tool: "bash", Origin: types.ToolOriginBuiltin, SessionID: string(rune('a' + s)), CatalogTokens: 300},
		})
	}
	for i := 0; i < 25; i++ {
		ledger.NoteToolCall(types.ToolCallMeasure{
			Tool: "grep", Origin: types.ToolOriginBuiltin, Channel: types.ToolChannelNative, SessionID: "a", Success: true,
			CallTokens: 15, ResultTokens: 100, ResultBytes: 400,
			Consumed: types.ToolConsumption{Known: true, Bytes: 200_000, Items: 40},
		})
	}
}

func TestToolsValue_ReportsClassCostAndCondensationFromTheLedger(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)
	seedToolLedger(t, dir)

	out, err := captureStdout(t, func() error { return runToolsValue(toolsValueCmd, nil) })
	if err != nil {
		t.Fatalf("runToolsValue: %v", err)
	}
	var grepLine, bashLine, unusedLine string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "grep "):
			grepLine = line
		case strings.HasPrefix(line, "bash "):
			bashLine = line
		case strings.HasPrefix(line, "glob "):
			unusedLine = line
		}
	}
	if !strings.Contains(grepLine, "earning") || !strings.Contains(grepLine, "49995%") {
		t.Errorf("grep should earn at ~50000%% condensation: %q", grepLine)
	}
	if !strings.Contains(bashLine, "dead") || !strings.Contains(bashLine, "never_called") {
		t.Errorf("bash was offered in 12 sessions and never called: %q", bashLine)
	}
	// glob has no ledger at all: it is listed, and its measures are unknown.
	if !strings.Contains(unusedLine, "unknown") {
		t.Errorf("a tool nobody offered should be unknown: %q", unusedLine)
	}
	if !strings.Contains(out, "bash") || !strings.Contains(out, "flagged (built-in: never removed)") {
		t.Errorf("a dead built-in is flagged, never removed:\n%s", out)
	}
}

func TestToolsValue_JSONCarriesNullForAnUnreportedScan(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)
	seedToolLedger(t, dir)
	toolsValueJSON = true
	t.Cleanup(func() { toolsValueJSON = false })

	out, err := captureStdout(t, func() error { return runToolsValue(toolsValueCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	var report autopoiesis.ToolValueReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	for _, row := range report.Rows {
		if row.Tool == "bash" && (row.CondensationPercent != nil || row.LeveragePercent != nil) {
			t.Errorf("bash reported no scan, but its condensation is %v / %v rather than null", row.CondensationPercent, row.LeveragePercent)
		}
		if row.Tool == "grep" && (row.CondensationPercent == nil || *row.CondensationPercent != 49995) {
			t.Errorf("grep condensation %v, want 49995", row.CondensationPercent)
		}
	}
}
