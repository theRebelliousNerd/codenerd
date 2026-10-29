package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/store"
)

// /cleanup-tools previews the budget boot and maintenance enforce, and says
// why --smart is not offered instead of calling it "not yet implemented"
// while store.ToolStore.CleanupIntelligent sat unwired: which executions to
// delete is a retention decision, and retention is the harness's.
func TestCleanupTools_PreviewsTheEnforcedBudgetAndDeclinesSmart(t *testing.T) {
	ts, err := store.NewToolStore(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	m := NewTestModel()
	m.toolStore = ts

	runtime := m.handleCleanupToolsCommand([]string{"--runtime"})
	if want := fmt.Sprintf("%.0f runtime hours", store.DefaultCleanupConfig().MaxRuntimeHours); !strings.Contains(runtime, want) {
		t.Errorf("--runtime preview does not name the enforced budget (%q):\n%s", want, runtime)
	}
	smart := m.handleCleanupToolsCommand([]string{"--smart"})
	if strings.Contains(smart, "not yet implemented") || !strings.Contains(smart, "not offered") {
		t.Errorf("--smart does not state the decision:\n%s", smart)
	}
}
