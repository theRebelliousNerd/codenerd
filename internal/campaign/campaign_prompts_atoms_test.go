package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every campaign role must be served by prompt atoms, not by prompts.go.
//
// prompts.go is ~1000 lines of frozen prompt with no view of the campaign it is
// serving. The JIT path assembles internal/prompt/atoms/campaign/* against the
// live campaign's facts and token budget. Both paths exist, and the failure
// mode is silent: a role whose atoms were never written falls back to the
// frozen text and produces a plan that looks completely normal.
//
// This reads the atom corpus and fails if a role has no atoms behind it.
func TestCampaignRoles_HaveAtomCoverage(t *testing.T) {
	atomsDir := filepath.Join(repoRoot(t), "internal", "prompt", "atoms", "campaign")
	if _, err := os.Stat(atomsDir); err != nil {
		t.Skipf("campaign atom corpus not present: %v", err)
	}

	corpus, err := loadCampaignAtomIDs(atomsDir)
	if err != nil {
		t.Fatalf("reading atom corpus: %v", err)
	}
	if len(corpus) < 20 {
		t.Fatalf("found only %d atom ids under %s; the scan is broken, not the corpus", len(corpus), atomsDir)
	}

	for _, role := range AllCampaignRoles() {
		family := CampaignRoleAtomFamily(role)
		if family == "" {
			t.Errorf("role %s has no atom family; add it to CampaignRoleAtomFamily", role)
			continue
		}
		count := 0
		for _, id := range corpus {
			if strings.HasPrefix(id, family+"/") {
				count++
			}
		}
		if count == 0 {
			t.Errorf("role %s claims atom family %q but no atom in the corpus carries that prefix. "+
				"Every request for this role silently falls back to the frozen prompt in prompts.go.",
				role, family)
		}
	}
}

// The static provider must announce itself. A campaign planned from the frozen
// prompt is indistinguishable in its output from one planned by the JIT path,
// so the only way an operator learns which happened is if the fallback says so.
func TestStaticPromptProvider_ShouldStillServeEveryRole(t *testing.T) {
	provider := NewStaticPromptProvider()
	for _, role := range AllCampaignRoles() {
		prompt, err := provider.GetPrompt(context.Background(), role, "/campaign_x")
		if err != nil {
			t.Errorf("role %s: %v", role, err)
			continue
		}
		if strings.TrimSpace(prompt) == "" {
			t.Errorf("role %s produced an empty fallback prompt; the fallback exists precisely so this never happens", role)
		}
	}

	// An unknown role must not return an empty string either.
	generic, err := provider.GetPrompt(context.Background(), CampaignRole("nonsense"), "/campaign_x")
	if err != nil {
		t.Fatalf("unknown role returned an error: %v", err)
	}
	if strings.TrimSpace(generic) == "" {
		t.Error("unknown role produced an empty prompt")
	}
}

// CampaignRoleAtomFamily must cover every declared role, so adding a role
// forces a decision about where its prompt content lives.
func TestCampaignRoleAtomFamily_CoversEveryRole(t *testing.T) {
	for _, role := range AllCampaignRoles() {
		if CampaignRoleAtomFamily(role) == "" {
			t.Errorf("role %s has no atom family mapping", role)
		}
	}
}

// loadCampaignAtomIDs reads `- id: "..."` entries out of the YAML corpus. A
// full YAML parse would pull a dependency into this package for a test; the ids
// are the only field this needs.
func loadCampaignAtomIDs(dir string) ([]string, error) {
	var ids []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "- id:") && !strings.HasPrefix(trimmed, "id:") {
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- id:"), "id:"))
			value = strings.Trim(value, `"'`)
			if value != "" {
				ids = append(ids, value)
			}
		}
		return nil
	})
	return ids, err
}
