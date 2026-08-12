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
func TestPlanResponseSchema_DocumentsIndexSpaces(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(planResponseSchema), &schema); err != nil {
		t.Fatalf("planResponseSchema is not valid JSON: %v", err)
	}

	// Navigate: properties.phases.items.properties.tasks.items.properties.[depends_on,context_from]
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing top-level properties")
	}
	phases, ok := props["phases"].(map[string]any)
	if !ok {
		t.Fatal("schema missing phases property")
	}
	phaseItems, ok := phases["items"].(map[string]any)
	if !ok {
		t.Fatal("schema missing phases.items")
	}
	phaseProps, ok := phaseItems["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing phases.items.properties")
	}
	tasks, ok := phaseProps["tasks"].(map[string]any)
	if !ok {
		t.Fatal("schema missing tasks property")
	}
	taskItems, ok := tasks["items"].(map[string]any)
	if !ok {
		t.Fatal("schema missing tasks.items")
	}
	taskProps, ok := taskItems["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing tasks.items.properties")
	}

	// depends_on must have non-empty description documenting phase-local index space
	dependsOn, ok := taskProps["depends_on"].(map[string]any)
	if !ok {
		t.Fatal("schema missing tasks.items.properties.depends_on")
	}
	dependsDesc, ok := dependsOn["description"].(string)
	if !ok || strings.TrimSpace(dependsDesc) == "" {
		t.Fatal("depends_on in planResponseSchema must carry a non-empty description documenting its phase-local index space")
	}
	if !strings.Contains(dependsDesc, "WITHIN THE SAME PHASE") {
		t.Fatalf("depends_on description must mention phase-local index space (WITHIN THE SAME PHASE), got %q", dependsDesc)
	}
	if !strings.Contains(dependsDesc, "first task of that phase") {
		t.Fatalf("depends_on description must mention counted from 0 at the first task of that phase, got %q", dependsDesc)
	}

	// context_from must have non-empty description documenting global index space
	contextFrom, ok := taskProps["context_from"].(map[string]any)
	if !ok {
		t.Fatal("schema missing tasks.items.properties.context_from")
	}
	contextDesc, ok := contextFrom["description"].(string)
	if !ok || strings.TrimSpace(contextDesc) == "" {
		t.Fatal("context_from in planResponseSchema must carry a non-empty description documenting its global index space")
	}
	if !strings.Contains(contextDesc, "GLOBAL") {
		t.Fatalf("context_from description must mention GLOBAL index space, got %q", contextDesc)
	}
	if !strings.Contains(contextDesc, "FIRST phase") {
		t.Fatalf("context_from description must mention FIRST phase, got %q", contextDesc)
	}
	if !strings.Contains(contextDesc, "across phase boundaries") {
		t.Fatalf("context_from description must mention continuing across phase boundaries, got %q", contextDesc)
	}
	if !strings.Contains(contextDesc, "depends_on") {
		t.Fatalf("context_from description must mention deliberately different numbering from depends_on, got %q", contextDesc)
	}

	if dependsDesc == contextDesc {
		t.Fatal("depends_on and context_from descriptions must not be identical; they describe different index spaces")
	}
}

