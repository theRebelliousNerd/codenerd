package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The registry's "authority" grant reaches the kernel at boot as
// shard_agent_authority, which mandate admission reads, and survives the
// registry's rewrite.
func TestBootAssertsTheRegistryAuthorityGrant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "WidgetExpert")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(widgetExpertPromptsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "WidgetExpert", "type": "persistent", "status": "ready",
		"domains": []string{}, "authority": []string{"mandate"},
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	rows, err := cortex.Kernel.Query("shard_agent_authority")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		found = found || (types.ExtractString(r.Args[0]) == "/widgetexpert" && types.ExtractString(r.Args[1]) == "/mandate")
	}
	if !found {
		t.Fatalf("shard_agent_authority = %v, want widgetexpert's /mandate grant", rows)
	}
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Agents []struct {
			Authority []string `json:"authority"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	if len(reg.Agents) != 1 || !slices.Contains(reg.Agents[0].Authority, "mandate") {
		t.Fatalf("the registry rewrite dropped the grant: %s", data)
	}
}
