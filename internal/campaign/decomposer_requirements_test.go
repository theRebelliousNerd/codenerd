package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRequirementsSmartBatchesDiscoveryQuestions(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "knowledge.db")
	docPath := filepath.Join(t.TempDir(), "spec.md")

	kbStore, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	content := "UNIQUE_SENTINEL functional requirements security compliance integration API contracts UI UX branding key specifications edge cases non-functional immutable pre-task workspace ownership baseline"
	if err := kbStore.StoreVectorWithEmbedding(ctx, content, map[string]any{"path": docPath}); err != nil {
		_ = kbStore.Close()
		t.Fatal(err)
	}
	if err := kbStore.Close(); err != nil {
		t.Fatal(err)
	}

	generated := make([]map[string]string, 25)
	for i := range generated {
		generated[i] = map[string]string{
			"description": fmt.Sprintf("direct requirement %02d", i+1),
			"priority":    "/high",
			"source":      docPath,
		}
	}
	payload, err := json.Marshal(map[string]any{"requirements": generated})
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	seenPrompt := ""
	client := &mockLLMClient{completeFunc: func(_ context.Context, prompt string) (string, error) {
		calls++
		seenPrompt = prompt
		return string(payload), nil
	}}
	kernel := &MockKernel{Facts: []core.Fact{{Predicate: "is_relevant", Args: []any{docPath}}}}
	d := NewDecomposer(kernel, client, t.TempDir())

	requirements, err := d.extractRequirementsSmart(ctx, "/campaign_batch", "immutable pre-task workspace ownership baseline", dbPath, []FileMetadata{{Path: docPath}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("LLM calls = %d, want exactly 1 for all discovery questions", calls)
	}
	if got := strings.Count(seenPrompt, "UNIQUE_SENTINEL"); got != 1 {
		t.Fatalf("duplicate snippet occurrences = %d, want 1", got)
	}
	if len(requirements) != 20 {
		t.Fatalf("requirements = %d, want hard cap of 20", len(requirements))
	}
	if requirements[0].ID != "/req_campaign_batch_0001" || requirements[19].ID != "/req_campaign_batch_0020" {
		t.Fatalf("requirement IDs are not dense and deterministic: first=%s last=%s", requirements[0].ID, requirements[19].ID)
	}
}
