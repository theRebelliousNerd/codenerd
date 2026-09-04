package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The direct-LLM file fallback overwrote a 251-line schema with generated
// content through a raw os.WriteFile (campaign 149c512d, 2026-09-04). It is a
// creator, never an editor, and repository writes go through the VirtualStore.

func fallbackFixture(t *testing.T) (*Orchestrator, string) {
	t.Helper()
	ws := t.TempDir()
	o := &Orchestrator{
		workspace: ws,
		campaign:  &Campaign{ID: "/campaign_fb"},
		llmClient: &MockLLMClient{},
	}
	return o, ws
}

func TestFileFallback_RefusesExistingTarget(t *testing.T) {
	o, ws := fallbackFixture(t)
	rel := "internal/x/existing.mg"
	full := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "Decl a(X).\nDecl b(Y).\nDecl c(Z).\n"
	if err := os.WriteFile(full, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	task := &Task{ID: "/task_fb_1", Type: TaskTypeFileModify, Description: "Modify " + rel, Artifacts: []TaskArtifact{{Type: "/file", Path: rel}}}

	_, err := o.executeFileTaskFallback(context.Background(), task, rel)
	if err == nil || !strings.Contains(err.Error(), "exists") {
		t.Fatalf("expected refusal naming the existing target, got %v", err)
	}
	after, _ := os.ReadFile(full)
	if string(after) != original {
		t.Fatalf("existing file was modified by the fallback: %q", string(after))
	}
}

func TestFileFallback_RefusesCreateOverExisting(t *testing.T) {
	o, ws := fallbackFixture(t)
	rel := "docs/report.md"
	full := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := &Task{ID: "/task_fb_2", Type: TaskTypeFileCreate, Description: "Create " + rel, Artifacts: []TaskArtifact{{Type: "/doc", Path: rel}}}

	_, err := o.executeFileTaskFallback(context.Background(), task, rel)
	if err == nil || !strings.Contains(err.Error(), "exists") {
		t.Fatalf("expected refusal for create over an existing file, got %v", err)
	}
	after, _ := os.ReadFile(full)
	if string(after) != "keep me" {
		t.Fatalf("existing file was modified: %q", string(after))
	}
}

func TestFileFallback_NilVirtualStoreRefuses(t *testing.T) {
	o, ws := fallbackFixture(t)
	rel := "docs/new_report.md"
	task := &Task{ID: "/task_fb_3", Type: TaskTypeFileCreate, Description: "Create " + rel, Artifacts: []TaskArtifact{{Type: "/doc", Path: rel}}}

	_, err := o.executeFileTaskFallback(context.Background(), task, rel)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "virtualstore") {
		t.Fatalf("expected refusal naming the missing VirtualStore, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, rel)); statErr == nil {
		t.Fatal("no file may be written around the VirtualStore")
	}
}

func TestInsideCampaignArtifacts(t *testing.T) {
	o, ws := fallbackFixture(t)
	slug := campaignSlug(o.campaign.ID)
	inside := filepath.Join(ws, ".nerd", "campaigns", slug, "artifacts", "task_1.md")
	nested := filepath.Join(ws, ".nerd", "campaigns", slug, "artifacts", "sub", "task_2.md")
	outsideRepo := filepath.Join(ws, "internal", "core", "defaults", "schemas_execution.mg")
	otherCampaign := filepath.Join(ws, ".nerd", "campaigns", "other", "artifacts", "x.md")
	escape := filepath.Join(ws, ".nerd", "campaigns", slug, "artifacts", "..", "..", "escape.md")

	if !o.insideCampaignArtifacts(inside) || !o.insideCampaignArtifacts(nested) {
		t.Fatal("paths under the campaign artifacts dir must be allowed")
	}
	for _, p := range []string{outsideRepo, otherCampaign, escape} {
		if o.insideCampaignArtifacts(p) {
			t.Fatalf("path outside the campaign artifacts dir must be refused: %s", p)
		}
	}
}
